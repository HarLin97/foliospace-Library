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
