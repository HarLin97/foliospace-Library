package httpapi

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"foliospace-reader/internal/db"
	"foliospace-reader/internal/domain"
	"foliospace-reader/internal/service"
	"foliospace-reader/internal/store"
)

func TestAPIResolvesAuditedMAMEProfilesWithoutChangingLegacyManifest(t *testing.T) {
	ts, vstriker, segabill, tekken := launchProfileTestServer(t)
	defer ts.Close()
	info := authGet(t, ts.URL+"/api/client/info", "secret")
	if !strings.Contains(info, `"gameLaunchResolver":true`) {
		t.Fatalf("client info missing resolver capability: %s", info)
	}

	request := auditedMAMERequest("1.302")
	response := postLaunchResolve(t, ts.URL, vstriker.ID, "secret", request, map[string]string{
		"X-FolioSpace-Client": "untrusted-header", "X-FolioSpace-Runtime": "mame-9.999",
	})
	if response.StatusCode != http.StatusOK {
		t.Fatalf("vstriker resolve status=%d body=%s", response.StatusCode, response.Body)
	}
	var resolved clientGameLaunchResolutionResponse
	if err := json.Unmarshal(response.Body, &resolved); err != nil {
		t.Fatal(err)
	}
	if resolved.LaunchProfileID != "vstriker-windows-mame0288-v1" || resolved.ProfileRevision != 1 {
		t.Fatalf("profile=%q revision=%d", resolved.LaunchProfileID, resolved.ProfileRevision)
	}
	if resolved.Runtime.ID != "mame" || resolved.Runtime.Version != "0.288" || resolved.Runtime.ContentSet != "mame-0.288" {
		t.Fatalf("runtime=%+v", resolved.Runtime)
	}
	if resolved.Manifest.Game.ROMSetName != "vstriker" || resolved.Manifest.Game.FileName != "vstriker.zip" || resolved.Manifest.Game.Size != 10316803 {
		t.Fatalf("resolved game=%+v", resolved.Manifest.Game)
	}
	if resolved.Manifest.EntryFile == nil || *resolved.Manifest.EntryFile != "vstriker.zip" || len(resolved.Manifest.Files) != 2 {
		t.Fatalf("resolved manifest=%+v", resolved.Manifest)
	}
	entry, dependency := resolved.Manifest.Files[0], resolved.Manifest.Files[1]
	if entry.Role != "entry" || entry.URL != "/api/client/games/"+itoa(vstriker.ID)+"/file" || entry.Checksum != "sha1:8e3518318eeb157ab299b2f284faef176d3f49dd" {
		t.Fatalf("entry=%+v", entry)
	}
	if dependency.Name != "segabill.zip" || dependency.Role != "dependency" || dependency.URL != "/api/client/games/"+itoa(segabill.ID)+"/file" || dependency.Checksum != "sha1:4631db7f7f5160a3a6591d3102722be869710f66" {
		t.Fatalf("dependency=%+v", dependency)
	}

	legacy := authGet(t, ts.URL+"/api/client/games/"+itoa(vstriker.ID)+"/manifest", "secret")
	if !strings.Contains(legacy, `"romSetName":"Model2ROMs"`) || strings.Contains(legacy, "launchProfileId") || strings.Contains(legacy, "segabill.zip") {
		t.Fatalf("legacy manifest changed: %s", legacy)
	}

	tekkenResponse := postLaunchResolve(t, ts.URL, tekken.ID, "secret", request, nil)
	if tekkenResponse.StatusCode != http.StatusOK {
		t.Fatalf("tekken resolve status=%d body=%s", tekkenResponse.StatusCode, tekkenResponse.Body)
	}
	var tekkenResolved clientGameLaunchResolutionResponse
	if err := json.Unmarshal(tekkenResponse.Body, &tekkenResolved); err != nil {
		t.Fatal(err)
	}
	if tekkenResolved.Manifest.Game.Title != "Tekken Tag Tournament (World, TEG2/VER.C1, set 2)" || tekkenResolved.Manifest.Game.ROMSetName != "tektagtc1a" || tekkenResolved.Manifest.Game.FileName != "tektagtc1a.zip" {
		t.Fatalf("tekken game=%+v", tekkenResolved.Manifest.Game)
	}
	if len(tekkenResolved.Manifest.Files) != 1 || tekkenResolved.Manifest.Files[0].Name != "tektagtc1a.zip" || tekkenResolved.Manifest.Files[0].URL != "/api/client/games/"+itoa(tekken.ID)+"/file" {
		t.Fatalf("tekken files=%+v", tekkenResolved.Manifest.Files)
	}
	legacyTekken := authGet(t, ts.URL+"/api/client/games/"+itoa(tekken.ID)+"/manifest", "secret")
	if !strings.Contains(legacyTekken, `"fileName":"tektagtac1.zip"`) || strings.Contains(legacyTekken, "tektagtc1a.zip") {
		t.Fatalf("legacy Tekken manifest exposed alias: %s", legacyTekken)
	}
}

func TestAPIResolverRejectsUnauthorizedInvalidAndUnmatchedRequests(t *testing.T) {
	ts, vstriker, _, _ := launchProfileTestServer(t)
	defer ts.Close()

	unauthorized := postLaunchResolve(t, ts.URL, vstriker.ID, "", auditedMAMERequest("1.302"), nil)
	if unauthorized.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unauthorized status=%d body=%s", unauthorized.StatusCode, unauthorized.Body)
	}

	unmatched := auditedMAMERequest("1.302")
	unmatched.Runtimes[0].Version = "0.289"
	unmatched.Runtimes[0].ContentSet = "mame-0.289"
	conflict := postLaunchResolve(t, ts.URL, vstriker.ID, "secret", unmatched, nil)
	if conflict.StatusCode != http.StatusConflict || !strings.Contains(string(conflict.Body), `"code":"runtime-profile-not-available"`) || !strings.Contains(string(conflict.Body), "MAME 0.289") {
		t.Fatalf("unmatched status=%d body=%s", conflict.StatusCode, conflict.Body)
	}

	oldClient := postLaunchResolve(t, ts.URL, vstriker.ID, "secret", auditedMAMERequest("1.301"), nil)
	if oldClient.StatusCode != http.StatusConflict {
		t.Fatalf("old client status=%d body=%s", oldClient.StatusCode, oldClient.Body)
	}

	invalid := auditedMAMERequest("1.302")
	invalid.Runtimes[0].CoreSHA256 = "ABC"
	badRequest := postLaunchResolve(t, ts.URL, vstriker.ID, "secret", invalid, nil)
	if badRequest.StatusCode != http.StatusBadRequest {
		t.Fatalf("invalid request status=%d body=%s", badRequest.StatusCode, badRequest.Body)
	}

	req, err := http.NewRequest(http.MethodGet, ts.URL+"/api/client/games/"+itoa(vstriker.ID)+"/resolve", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer secret")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("GET resolver status=%d", resp.StatusCode)
	}
}

func TestAPIResolvesAuditedCPSAndMAMECatalogProfiles(t *testing.T) {
	ts, games := catalogLaunchProfileTestServer(t)
	defer ts.Close()

	cpsRequest := domain.GameLaunchResolveRequest{
		Client: domain.GameLaunchClient{Name: "SpatialEMU.Windows", Version: "1.302", Platform: "windows-x64", Architecture: "x64"},
		Runtimes: []domain.GameRuntimeDescriptor{{
			ID: "libretro", CoreID: "fbneo", CoreSHA256: "6ebc2675c272c8d654935647ac336d45bbd97452c4d5943290d5ffc75678d9f1",
		}},
	}
	for _, test := range []struct {
		name      string
		platform  string
		profileID string
	}{
		{name: "sf2", platform: "cps1", profileID: "sf2-windows-fbneo-v1"},
		{name: "sfa", platform: "cps2", profileID: "sfa-windows-fbneo-v1"},
		{name: "sfiii", platform: "cps3", profileID: "sfiii-windows-fbneo-v1"},
	} {
		response := postLaunchResolve(t, ts.URL, games[test.name].ID, "secret", cpsRequest, nil)
		if response.StatusCode != http.StatusOK {
			t.Fatalf("%s resolve status=%d body=%s", test.name, response.StatusCode, response.Body)
		}
		var resolved clientGameLaunchResolutionResponse
		if err := json.Unmarshal(response.Body, &resolved); err != nil {
			t.Fatal(err)
		}
		if resolved.LaunchProfileID != test.profileID || resolved.Runtime.ID != "libretro" || resolved.Runtime.CoreID != "fbneo" || resolved.Manifest.Game.Platform != test.platform || resolved.Manifest.Game.ROMSetName != test.name || resolved.Manifest.Game.FileName != test.name+".zip" {
			t.Fatalf("%s resolution=%+v", test.name, resolved)
		}
	}

	wrongCore := cpsRequest
	wrongCore.Runtimes = append([]domain.GameRuntimeDescriptor{}, cpsRequest.Runtimes...)
	wrongCore.Runtimes[0].CoreSHA256 = strings.Repeat("0", 64)
	conflict := postLaunchResolve(t, ts.URL, games["sf2"].ID, "secret", wrongCore, nil)
	if conflict.StatusCode != http.StatusConflict || !strings.Contains(string(conflict.Body), `"code":"runtime-profile-not-available"`) {
		t.Fatalf("wrong CPS core status=%d body=%s", conflict.StatusCode, conflict.Body)
	}

	mameRequest := auditedMAMERequest("1.302")
	for _, test := range []struct {
		name      string
		profileID string
	}{
		{name: "hypreact", profileID: "hypreact-windows-mame0288-v1"},
		{name: "hypreac2", profileID: "hypreac2-windows-mame0288-v1"},
		{name: "srmp4", profileID: "srmp4-windows-mame0288-v1"},
		{name: "fromancr", profileID: "fromancr-windows-mame0288-v1"},
		{name: "fromanc4", profileID: "fromanc4-windows-mame0288-v1"},
		{name: "mcnpshnt", profileID: "mcnpshnt-windows-mame0288-v1"},
	} {
		response := postLaunchResolve(t, ts.URL, games[test.name].ID, "secret", mameRequest, nil)
		if response.StatusCode != http.StatusOK {
			t.Fatalf("%s resolve status=%d body=%s", test.name, response.StatusCode, response.Body)
		}
		var resolved clientGameLaunchResolutionResponse
		if err := json.Unmarshal(response.Body, &resolved); err != nil {
			t.Fatal(err)
		}
		if resolved.LaunchProfileID != test.profileID || resolved.Manifest.Game.ROMSetName != test.name || resolved.Manifest.Game.FileName != test.name+".zip" {
			t.Fatalf("%s resolution=%+v", test.name, resolved)
		}
		if test.name == "mcnpshnt" {
			if len(resolved.Manifest.Files) != 2 || resolved.Manifest.Files[1].Name != "ym2413.zip" || resolved.Manifest.Files[1].Role != "dependency" || resolved.Manifest.Files[1].URL != "/api/client/games/"+itoa(games["ym2413_instruments"].ID)+"/file" {
				t.Fatalf("mcnpshnt files=%+v", resolved.Manifest.Files)
			}
		}
	}
}

func TestAPISelectsMAMEOrFBNeoFromDualArcadeCapabilities(t *testing.T) {
	ts, games := catalogLaunchProfileTestServer(t)
	defer ts.Close()

	mame := domain.GameRuntimeDescriptor{ID: "mame", Version: "0.288", ContentSet: "mame-0.288"}
	fbneo := domain.GameRuntimeDescriptor{ID: "libretro", CoreID: "fbneo", CoreSHA256: "6ebc2675c272c8d654935647ac336d45bbd97452c4d5943290d5ffc75678d9f1"}
	client := domain.GameLaunchClient{Name: "SpatialEMU.Windows", Version: "1.302", Platform: "windows-x64", Architecture: "x64"}

	for _, runtimes := range [][]domain.GameRuntimeDescriptor{{mame, fbneo}, {fbneo, mame}} {
		request := domain.GameLaunchResolveRequest{Client: client, Runtimes: runtimes}
		for _, test := range []struct {
			gameID      int64
			wantRuntime string
			wantCore    string
		}{
			{gameID: games["sf2"].ID, wantRuntime: "libretro", wantCore: "fbneo"},
			{gameID: games["hypreact"].ID, wantRuntime: "mame"},
		} {
			response := postLaunchResolve(t, ts.URL, test.gameID, "secret", request, nil)
			if response.StatusCode != http.StatusOK {
				t.Fatalf("resolve game=%d status=%d body=%s", test.gameID, response.StatusCode, response.Body)
			}
			var resolved clientGameLaunchResolutionResponse
			if err := json.Unmarshal(response.Body, &resolved); err != nil {
				t.Fatal(err)
			}
			if resolved.Runtime.ID != test.wantRuntime || resolved.Runtime.CoreID != test.wantCore {
				t.Fatalf("resolve game=%d runtime=%+v, want id=%q core=%q", test.gameID, resolved.Runtime, test.wantRuntime, test.wantCore)
			}
		}
	}
}

func TestAPIResolvesStrictMobileArcadeProfilesWithExactRuntimeIdentity(t *testing.T) {
	ts, games := catalogLaunchProfileTestServer(t)
	defer ts.Close()

	requests := []struct {
		name    string
		gameID  int64
		request domain.GameLaunchResolveRequest
	}{
		{
			name: "iPadOS FBNeo", gameID: games["sf2"].ID,
			request: domain.GameLaunchResolveRequest{
				Client:   domain.GameLaunchClient{Name: "SpatialEMU.iPadOS", Version: "1.300", Platform: "ipados-arm64", Architecture: "arm64"},
				Runtimes: []domain.GameRuntimeDescriptor{{ID: "libretro", CoreID: "fbneo", CoreSHA256: strings.Repeat("1", 64)}},
			},
		},
		{
			name: "visionOS MAME", gameID: games["hypreact"].ID,
			request: domain.GameLaunchResolveRequest{
				Client:   domain.GameLaunchClient{Name: "SpatialEMU.visionOS", Version: "1.300", Platform: "visionos-arm64", Architecture: "arm64"},
				Runtimes: []domain.GameRuntimeDescriptor{{ID: "mame", Version: "0.287", ContentSet: "mame-0.287"}},
			},
		},
	}
	for _, test := range requests {
		response := postLaunchResolve(t, ts.URL, test.gameID, "secret", test.request, nil)
		if response.StatusCode != http.StatusOK {
			t.Fatalf("%s status=%d body=%s", test.name, response.StatusCode, response.Body)
		}
	}

	wrongHash := requests[0].request
	wrongHash.Runtimes = append([]domain.GameRuntimeDescriptor{}, wrongHash.Runtimes...)
	wrongHash.Runtimes[0].CoreSHA256 = strings.Repeat("2", 64)
	response := postLaunchResolve(t, ts.URL, games["sf2"].ID, "secret", wrongHash, nil)
	if response.StatusCode != http.StatusConflict || !strings.Contains(string(response.Body), `"code":"runtime-profile-not-available"`) {
		t.Fatalf("wrong mobile FBNeo hash status=%d body=%s", response.StatusCode, response.Body)
	}
}

func TestAPIResolvesPragmaticConsoleDiscAndDOSProfiles(t *testing.T) {
	ts, games := pragmaticLaunchProfileTestServer(t)
	defer ts.Close()

	tests := []struct {
		name      string
		runtime   domain.GameRuntimeDescriptor
		entryFile string
		fileCount int
	}{
		{name: "nes", runtime: domain.GameRuntimeDescriptor{ID: "libretro", CoreID: "nestopia"}, entryFile: "mario.nes", fileCount: 1},
		{name: "ps1", runtime: domain.GameRuntimeDescriptor{ID: "libretro", CoreID: "swanstation"}, entryFile: "ridge.cue", fileCount: 2},
		{name: "n64", runtime: domain.GameRuntimeDescriptor{ID: "libretro", CoreID: "mupen64plus-next"}, entryFile: "zelda.z64", fileCount: 1},
		{name: "saturn", runtime: domain.GameRuntimeDescriptor{ID: "libretro", CoreID: "beetle-saturn"}, entryFile: "nights.cue", fileCount: 2},
		{name: "pc98", runtime: domain.GameRuntimeDescriptor{ID: "libretro", CoreID: "np2kai"}, entryFile: "game-disk1.fdi", fileCount: 2},
		{name: "ps2", runtime: domain.GameRuntimeDescriptor{ID: "pcsx2", Version: "2.6.3.0"}, entryFile: "mgs2.iso", fileCount: 1},
		{name: "psp", runtime: domain.GameRuntimeDescriptor{ID: "ppsspp", Version: "1.20.4"}, entryFile: "mgs.iso", fileCount: 1},
		{name: "ngc", runtime: domain.GameRuntimeDescriptor{ID: "dolphin"}, entryFile: "twin-snakes.iso", fileCount: 1},
		{name: "dreamcast", runtime: domain.GameRuntimeDescriptor{ID: "flycast", Version: "2.6"}, entryFile: "crazy-taxi.chd", fileCount: 1},
		{name: "dos", runtime: domain.GameRuntimeDescriptor{ID: "dosbox-staging", Version: "0.82.2.0"}, entryFile: "GAME/START.BAT", fileCount: 1},
	}
	client := domain.GameLaunchClient{Name: "SpatialEMU.Windows", Version: "1.302", Platform: "windows-x64", Architecture: "x64"}
	for _, test := range tests {
		request := domain.GameLaunchResolveRequest{Client: client, Runtimes: []domain.GameRuntimeDescriptor{test.runtime}}
		response := postLaunchResolve(t, ts.URL, games[test.name].ID, "secret", request, nil)
		if response.StatusCode != http.StatusOK {
			t.Fatalf("%s resolve status=%d body=%s", test.name, response.StatusCode, response.Body)
		}
		var resolved clientGameLaunchResolutionResponse
		if err := json.Unmarshal(response.Body, &resolved); err != nil {
			t.Fatal(err)
		}
		if resolved.Runtime != test.runtime {
			t.Fatalf("%s runtime=%+v, want exact request tuple %+v", test.name, resolved.Runtime, test.runtime)
		}
		if !strings.HasPrefix(resolved.LaunchProfileID, "auto-") || resolved.ProfileRevision <= 0 {
			t.Fatalf("%s profile=%q revision=%d", test.name, resolved.LaunchProfileID, resolved.ProfileRevision)
		}
		if resolved.Manifest.EntryFile == nil || *resolved.Manifest.EntryFile != test.entryFile || len(resolved.Manifest.Files) != test.fileCount {
			t.Fatalf("%s manifest=%+v", test.name, resolved.Manifest)
		}
		for position, file := range resolved.Manifest.Files {
			expectedURL := "/api/client/games/" + itoa(games[test.name].ID) + "/files/" + itoa(int64(position))
			if file.URL != expectedURL {
				t.Fatalf("%s file %d URL=%q, want %q", test.name, position, file.URL, expectedURL)
			}
		}

		repeated := postLaunchResolve(t, ts.URL, games[test.name].ID, "secret", request, nil)
		var repeatedResolved clientGameLaunchResolutionResponse
		if repeated.StatusCode != http.StatusOK || json.Unmarshal(repeated.Body, &repeatedResolved) != nil || repeatedResolved.LaunchProfileID != resolved.LaunchProfileID || repeatedResolved.ProfileRevision != resolved.ProfileRevision {
			t.Fatalf("%s repeated resolution is not stable: %s", test.name, repeated.Body)
		}
	}

	dosRequest := domain.GameLaunchResolveRequest{Client: client, Runtimes: []domain.GameRuntimeDescriptor{{ID: "dosbox-staging", Version: "0.82.2.0"}}}
	dosResponse := postLaunchResolve(t, ts.URL, games["dos"].ID, "secret", dosRequest, nil)
	var dosResolved clientGameLaunchResolutionResponse
	if err := json.Unmarshal(dosResponse.Body, &dosResolved); err != nil {
		t.Fatal(err)
	}
	if dosResolved.Manifest.Game.FileName != "dos-game.zip" || dosResolved.Manifest.DOSLaunch == nil || dosResolved.Manifest.DOSLaunch.EntrySource != "curated" || dosResolved.Manifest.DOSLaunch.WorkingDirectory == nil || *dosResolved.Manifest.DOSLaunch.WorkingDirectory != "GAME" {
		t.Fatalf("DOS resolution=%+v", dosResolved)
	}

	pc98Request := domain.GameLaunchResolveRequest{Client: client, Runtimes: []domain.GameRuntimeDescriptor{{ID: "libretro", CoreID: "np2kai"}}}
	pc98Response := postLaunchResolve(t, ts.URL, games["pc98"].ID, "secret", pc98Request, nil)
	var pc98Resolved clientGameLaunchResolutionResponse
	if err := json.Unmarshal(pc98Response.Body, &pc98Resolved); err != nil {
		t.Fatal(err)
	}
	if len(pc98Resolved.Manifest.Files) != 2 || pc98Resolved.Manifest.Files[0].Role != "entry" || pc98Resolved.Manifest.Files[0].DiskIndex == nil || *pc98Resolved.Manifest.Files[0].DiskIndex != 0 || pc98Resolved.Manifest.Files[0].DriveHint != "FDD1" || pc98Resolved.Manifest.Files[1].Role != "disk" || pc98Resolved.Manifest.Files[1].DiskIndex == nil || *pc98Resolved.Manifest.Files[1].DiskIndex != 1 {
		t.Fatalf("PC-98 disk metadata=%+v", pc98Resolved.Manifest.Files)
	}
}

func TestAPIResolvesPragmaticProfilesForAppleMobileClients(t *testing.T) {
	ts, games := pragmaticLaunchProfileTestServer(t)
	defer ts.Close()

	clients := []domain.GameLaunchClient{
		{Name: "SpatialEMU.iOS", Version: "1.40", Platform: "ios-arm64", Architecture: "arm64"},
		{Name: "SpatialEMU.iPadOS", Version: "1.40", Platform: "ipados-arm64", Architecture: "arm64"},
		{Name: "SpatialEMU.visionOS", Version: "1.40", Platform: "visionos-arm64", Architecture: "arm64"},
		{Name: "SpatialEMU.tvOS", Version: "1.40", Platform: "tvos-arm64", Architecture: "arm64"},
	}
	runtime := domain.GameRuntimeDescriptor{ID: "libretro", CoreID: "nestopia"}
	profileIDs := make(map[string]struct{}, len(clients))
	for _, client := range clients {
		t.Run(client.Platform, func(t *testing.T) {
			request := domain.GameLaunchResolveRequest{Client: client, Runtimes: []domain.GameRuntimeDescriptor{runtime}}
			response := postLaunchResolve(t, ts.URL, games["nes"].ID, "secret", request, nil)
			if response.StatusCode != http.StatusOK {
				t.Fatalf("resolve status=%d body=%s", response.StatusCode, response.Body)
			}
			var resolved clientGameLaunchResolutionResponse
			if err := json.Unmarshal(response.Body, &resolved); err != nil {
				t.Fatal(err)
			}
			if resolved.Runtime != runtime || !strings.HasPrefix(resolved.LaunchProfileID, "auto-") || resolved.ProfileRevision <= 0 {
				t.Fatalf("resolution=%+v", resolved)
			}
			if resolved.Manifest.EntryFile == nil || *resolved.Manifest.EntryFile != "mario.nes" || len(resolved.Manifest.Files) != 1 {
				t.Fatalf("manifest=%+v", resolved.Manifest)
			}
			if _, exists := profileIDs[resolved.LaunchProfileID]; exists {
				t.Fatalf("profile id %q was reused across client platforms", resolved.LaunchProfileID)
			}
			profileIDs[resolved.LaunchProfileID] = struct{}{}
		})
	}
}

func TestAPIPragmaticResolverRejectsUnknownCoreAndUncuratedDOS(t *testing.T) {
	ts, games := pragmaticLaunchProfileTestServer(t)
	defer ts.Close()
	client := domain.GameLaunchClient{Name: "SpatialEMU.Windows", Version: "1.302", Platform: "windows-x64", Architecture: "x64"}

	unknownCore := domain.GameLaunchResolveRequest{Client: client, Runtimes: []domain.GameRuntimeDescriptor{{ID: "libretro", CoreID: "unknown-core"}}}
	response := postLaunchResolve(t, ts.URL, games["nes"].ID, "secret", unknownCore, nil)
	if response.StatusCode != http.StatusConflict {
		t.Fatalf("unknown core status=%d body=%s", response.StatusCode, response.Body)
	}

	uncurated := domain.GameLaunchResolveRequest{Client: client, Runtimes: []domain.GameRuntimeDescriptor{{ID: "dosbox-staging", Version: "0.82.2.0"}}}
	response = postLaunchResolve(t, ts.URL, games["dos-unknown"].ID, "secret", uncurated, nil)
	if response.StatusCode != http.StatusConflict {
		t.Fatalf("uncurated DOS status=%d body=%s", response.StatusCode, response.Body)
	}
}

func pragmaticLaunchProfileTestServer(t *testing.T) (*httptest.Server, map[string]domain.GameAsset) {
	t.Helper()
	root := t.TempDir()
	conn, err := db.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	st := store.New(conn)
	lib, err := st.CreateLibrary("Games", root)
	if err != nil {
		t.Fatal(err)
	}
	games := map[string]domain.GameAsset{}
	createFile := func(name, contents string) string {
		path := filepath.Join(root, name)
		if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
		return path
	}
	addGame := func(key, platform, format, entryName string, dependencies ...string) domain.GameAsset {
		entryContents := "game"
		if format == "cue" && len(dependencies) > 0 {
			entryContents = "FILE \"" + dependencies[0] + "\" BINARY\n  TRACK 01 MODE1/2352\n    INDEX 01 00:00:00\n"
		}
		entryPath := createFile(entryName, entryContents)
		files := []domain.GameFile{{Name: entryName, FilePath: entryPath, Size: int64(len(entryContents)), MTime: time.Unix(1, 0), Role: "entry", Position: 0}}
		totalSize := int64(len(entryContents))
		for index, dependency := range dependencies {
			contents := "track-data-" + dependency
			path := createFile(dependency, contents)
			files = append(files, domain.GameFile{Name: dependency, FilePath: path, Size: int64(len(contents)), MTime: time.Unix(1, 0), Role: "dependency", Position: index + 1})
			totalSize += int64(len(contents))
		}
		game, err := st.UpsertGame(domain.GameAsset{
			LibraryID: lib.ID, Title: key, Platform: platform, ROMSetName: strings.ToUpper(platform), Format: format,
			FilePath: entryPath, RelPath: entryName, Size: totalSize, MTime: time.Unix(1, 0),
			SHA1: strings.Repeat("a", 40), EmulatorHint: platform, Compatibility: "unknown", CatalogRole: "game",
		})
		if err != nil {
			t.Fatal(err)
		}
		if err := st.ReplaceGameFiles(game.ID, files); err != nil {
			t.Fatal(err)
		}
		games[key] = game
		return game
	}

	addGame("nes", "nes", "nes", "mario.nes")
	addGame("ps1", "ps1", "cue", "ridge.cue", "ridge.bin")
	addGame("n64", "n64", "z64", "zelda.z64")
	addGame("saturn", "saturn", "cue", "nights.cue", "nights.bin")
	pc98 := addGame("pc98", "pc98", "fdi", "game-disk1.fdi", "game-disk2.fdi")
	pc98Files, err := st.GameFiles(pc98.ID)
	if err != nil {
		t.Fatal(err)
	}
	pc98Files[1].Role = "disk"
	if err := st.ReplaceGameFiles(pc98.ID, pc98Files); err != nil {
		t.Fatal(err)
	}
	addGame("ps2", "ps2", "iso", "mgs2.iso")
	addGame("psp", "psp", "iso", "mgs.iso")
	addGame("ngc", "ngc", "iso", "twin-snakes.iso")
	addGame("dreamcast", "dreamcast", "chd", "crazy-taxi.chd")
	dos := addGame("dos", "dos", "zip", "dos-game.zip")
	if err := st.UpsertDOSLaunch(domain.DOSLaunch{
		GameID: dos.ID, EntryFile: "GAME/START.BAT", EntrySource: "curated", WorkingDirectory: "GAME",
		Arguments: []string{"-fast"}, Candidates: []domain.DOSLaunchCandidate{{Path: "GAME/START.BAT", Kind: "bat"}},
	}); err != nil {
		t.Fatal(err)
	}
	unknown := addGame("dos-unknown", "dos", "zip", "unknown-dos.zip")
	if err := st.UpsertDOSLaunch(domain.DOSLaunch{
		GameID: unknown.ID, EntrySource: "unknown", Candidates: []domain.DOSLaunchCandidate{{Path: "GAME.EXE", Kind: "exe"}},
	}); err != nil {
		t.Fatal(err)
	}

	ts := httptest.NewServer(NewWithOptions(service.New(st), nil, Options{APIToken: "secret"}).Routes())
	return ts, games
}

func launchProfileTestServer(t *testing.T) (*httptest.Server, domain.GameAsset, domain.GameAsset, domain.GameAsset) {
	t.Helper()
	root := t.TempDir()
	conn, err := db.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	st := store.New(conn)
	lib, err := st.CreateLibrary("Games", root)
	if err != nil {
		t.Fatal(err)
	}
	createSparse := func(name string, size int64) string {
		path := filepath.Join(root, name)
		file, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := file.Truncate(size); err != nil {
			_ = file.Close()
			t.Fatal(err)
		}
		if err := file.Close(); err != nil {
			t.Fatal(err)
		}
		return path
	}
	upsert := func(title, platform, romSet, name string, size int64, sha1, role string) domain.GameAsset {
		game, err := st.UpsertGame(domain.GameAsset{
			LibraryID: lib.ID, Title: title, Platform: platform, ROMSetName: romSet, Format: "zip",
			FilePath: createSparse(name, size), RelPath: name, Size: size, MTime: time.Unix(1, 0),
			SHA1: sha1, EmulatorHint: platform, Compatibility: "unknown", CatalogRole: role,
		})
		if err != nil {
			t.Fatal(err)
		}
		return game
	}
	vstriker := upsert("vstriker", "model2", "Model2ROMs", "vstriker.zip", 10313686, "8e3518318eeb157ab299b2f284faef176d3f49dd", "game")
	segabill := upsert("segabill", "model2", "Model2ROMs", "segabill.zip", 3117, "4631db7f7f5160a3a6591d3102722be869710f66", "dependency")
	tekken := upsert("tektagtac1", "arcade", "Namco System 12", "tektagtac1.zip", 120980600, "d6615a3a70ea9941b61ccd608054a0044d3d6ab3", "game")
	ts := httptest.NewServer(NewWithOptions(service.New(st), nil, Options{APIToken: "secret"}).Routes())
	return ts, vstriker, segabill, tekken
}

func catalogLaunchProfileTestServer(t *testing.T) (*httptest.Server, map[string]domain.GameAsset) {
	t.Helper()
	root := t.TempDir()
	conn, err := db.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	st := store.New(conn)
	lib, err := st.CreateLibrary("Games", root)
	if err != nil {
		t.Fatal(err)
	}
	type asset struct {
		name     string
		platform string
		emulator string
		size     int64
		sha1     string
		role     string
	}
	assets := []asset{
		{name: "sf2", platform: "cps1", emulator: "fbneo", size: 3551819, sha1: "bd59872a57f14dc492e2fb387727a9402f3d4f97", role: "game"},
		{name: "sfa", platform: "cps2", emulator: "fbneo", size: 7365582, sha1: "61dece364b8d2f2ff15391505168be334ebb371a", role: "game"},
		{name: "sfiii", platform: "cps3", emulator: "fbneo", size: 38868517, sha1: "7aae0cfc4ef8911f19d2e986cee63807deebf1b6", role: "game"},
		{name: "hypreact", platform: "mame", emulator: "mame", size: 8052342, sha1: "e0940f848884c9d53bbc41bb947d584e06cc1845", role: "game"},
		{name: "hypreac2", platform: "mame", emulator: "mame", size: 18291541, sha1: "7fe73cc7ee40a49225a4616106e538c084ef4364", role: "game"},
		{name: "srmp4", platform: "mame", emulator: "mame", size: 7697767, sha1: "cfcf2cdf61ebca862a84473a8bf75fbe8d76cb7b", role: "game"},
		{name: "fromancr", platform: "mame", emulator: "mame", size: 14121810, sha1: "137e4949d7e204ff10e33372528cc1e9481b962c", role: "game"},
		{name: "fromanc4", platform: "mame", emulator: "mame", size: 21443327, sha1: "ff478f3350d9703e8647f659ce169ee234082249", role: "game"},
		{name: "mcnpshnt", platform: "mame", emulator: "mame", size: 1205007, sha1: "24a714371a867db1709798a95a171778e0940021", role: "game"},
		{name: "ym2413_instruments", platform: "mame", emulator: "mame", size: 322, sha1: "cbcd6e0698026452bb2bb6a6e6f7f5a3667a675c", role: "dependency"},
	}
	games := make(map[string]domain.GameAsset, len(assets))
	for _, item := range assets {
		path := filepath.Join(root, item.name+".zip")
		file, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := file.Truncate(item.size); err != nil {
			_ = file.Close()
			t.Fatal(err)
		}
		if err := file.Close(); err != nil {
			t.Fatal(err)
		}
		game, err := st.UpsertGame(domain.GameAsset{
			LibraryID: lib.ID, Title: "Catalog " + item.name, Platform: item.platform, ROMSetName: item.name,
			Format: "zip", FilePath: path, RelPath: item.name + ".zip", Size: item.size, MTime: time.Unix(1, 0),
			SHA1: item.sha1, EmulatorHint: item.emulator, Compatibility: "unknown", CatalogRole: item.role,
		})
		if err != nil {
			t.Fatal(err)
		}
		games[item.name] = game
	}
	mobileProfiles := []domain.GameLaunchProfile{
		{
			GameID: games["sf2"].ID, ID: "sf2-ipados-fbneo-test", Revision: 1, Priority: 200,
			ClientName: "SpatialEMU.iPadOS", MinClientVersion: "1.300", ClientPlatform: "ipados-arm64", Architecture: "arm64",
			Runtime:   domain.GameRuntimeDescriptor{ID: "libretro", CoreID: "fbneo", CoreSHA256: strings.Repeat("1", 64)},
			EntryFile: "sf2.zip", CanonicalSet: "sf2", Status: "ready",
			Files: []domain.GameLaunchProfileFile{{
				Position: 0, SourceGameID: games["sf2"].ID, SourceSHA1: games["sf2"].SHA1,
				SourceName: "sf2.zip", Name: "sf2.zip", Size: games["sf2"].Size, Role: "entry",
			}},
		},
		{
			GameID: games["hypreact"].ID, ID: "hypreact-visionos-mame0287-test", Revision: 1, Priority: 200,
			ClientName: "SpatialEMU.visionOS", MinClientVersion: "1.300", ClientPlatform: "visionos-arm64", Architecture: "arm64",
			Runtime:   domain.GameRuntimeDescriptor{ID: "mame", Version: "0.287", ContentSet: "mame-0.287"},
			EntryFile: "hypreact.zip", CanonicalSet: "hypreact", Status: "ready",
			Files: []domain.GameLaunchProfileFile{{
				Position: 0, SourceGameID: games["hypreact"].ID, SourceSHA1: games["hypreact"].SHA1,
				SourceName: "hypreact.zip", Name: "hypreact.zip", Size: games["hypreact"].Size, Role: "entry",
			}},
		},
	}
	mobileUpdates := []domain.GameLaunchCatalogUpdate{
		{GameID: games["sf2"].ID, Platform: "cps1", ROMSetName: "sf2", EmulatorHint: "fbneo", CatalogRole: "game"},
		{GameID: games["hypreact"].ID, Platform: "mame", ROMSetName: "hypreact", EmulatorHint: "mame", CatalogRole: "game"},
	}
	if _, err := st.ReplaceGameLaunchProfiles("test-mobile-arcade", mobileProfiles, mobileUpdates); err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(NewWithOptions(service.New(st), nil, Options{APIToken: "secret"}).Routes())
	return ts, games
}

func auditedMAMERequest(version string) domain.GameLaunchResolveRequest {
	return domain.GameLaunchResolveRequest{
		Client:   domain.GameLaunchClient{Name: "SpatialEMU.Windows", Version: version, Platform: "windows-x64", Architecture: "x64"},
		Runtimes: []domain.GameRuntimeDescriptor{{ID: "mame", Version: "0.288", ContentSet: "mame-0.288"}},
	}
}

type launchResolveHTTPResponse struct {
	StatusCode int
	Body       []byte
}

func postLaunchResolve(t *testing.T, baseURL string, gameID int64, token string, body domain.GameLaunchResolveRequest, headers map[string]string) launchResolveHTTPResponse {
	t.Helper()
	encoded, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest(http.MethodPost, baseURL+"/api/client/games/"+itoa(gameID)+"/resolve", bytes.NewReader(encoded))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	for name, value := range headers {
		req.Header.Set(name, value)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return launchResolveHTTPResponse{StatusCode: resp.StatusCode, Body: data}
}
