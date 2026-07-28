package service

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"foliospace-reader/internal/db"
	"foliospace-reader/internal/domain"
	"foliospace-reader/internal/store"
)

func TestAuditedGameLaunchProfilesAreValid(t *testing.T) {
	if err := validateAuditedGameLaunchProfiles(); err != nil {
		t.Fatal(err)
	}
}

func TestLogicalLaunchNamesRejectUnsafeAndCollidingPaths(t *testing.T) {
	for _, name := range []string{"", ".", "..", "../rom.zip", `folder\\rom.zip`, `/rom.zip`, `C:\\rom.zip`, "bad\x00name.zip"} {
		if validLogicalLaunchName(name) {
			t.Fatalf("validLogicalLaunchName(%q) = true, want false", name)
		}
	}
	if !validLogicalLaunchName("tektagtc1a.zip") {
		t.Fatal("expected audited ROM alias to be valid")
	}
}

func TestValidateGameLaunchResolveRequestRejectsInvalidCoreHash(t *testing.T) {
	req := domain.GameLaunchResolveRequest{
		Client:   domain.GameLaunchClient{Name: "SpatialEMU.Windows", Version: "1.302", Platform: "windows-x64", Architecture: "x64"},
		Runtimes: []domain.GameRuntimeDescriptor{{ID: "libretro", CoreID: "fbneo", CoreSHA256: "ABC"}},
	}
	if err := ValidateGameLaunchResolveRequest(req); err == nil {
		t.Fatal("expected invalid core hash to be rejected")
	}
	req.Runtimes[0].CoreSHA256 = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	if err := ValidateGameLaunchResolveRequest(req); err != nil {
		t.Fatalf("valid request rejected: %v", err)
	}
}

func TestLaunchProfileClientVersionFloor(t *testing.T) {
	if versionAtLeast("1.301", "1.302") {
		t.Fatal("1.301 must not match a 1.302 profile")
	}
	if !versionAtLeast("1.302", "1.302") || !versionAtLeast("1.303", "1.302") {
		t.Fatal("1.302 and later should match the profile floor")
	}
}

func TestSFCSupportsWindowsBSNES(t *testing.T) {
	runtime := domain.GameRuntimeDescriptor{ID: "libretro", CoreID: "bsnes"}
	if !pragmaticRuntimeSupportsPlatform(runtime, "snes") {
		t.Fatal("expected SFC/SNES to accept the Windows bsnes core")
	}
}

func TestPersistedLaunchProfileResolvesFromSQLite(t *testing.T) {
	conn, err := db.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	st := store.New(conn)
	root := t.TempDir()
	library, err := st.CreateLibrary("Games", root)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "clone.zip")
	if err := os.WriteFile(path, []byte("verified-container"), 0o644); err != nil {
		t.Fatal(err)
	}
	const sourceSHA1 = "0123456789abcdef0123456789abcdef01234567"
	game, err := st.UpsertGame(domain.GameAsset{
		LibraryID: library.ID, Title: "Clone", Platform: "cps1", ROMSetName: "clone", Format: "zip",
		FilePath: path, RelPath: "clone.zip", Size: int64(len("verified-container")), MTime: time.Unix(1, 0),
		SHA1: sourceSHA1, EmulatorHint: "fbneo", CatalogRole: "needs-curation",
	})
	if err != nil {
		t.Fatal(err)
	}
	runtime := domain.GameRuntimeDescriptor{ID: "libretro", CoreID: "fbneo", CoreSHA256: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"}
	profile := domain.GameLaunchProfile{
		GameID: game.ID, ID: "clone-windows-fbneo-test", Revision: 42, Priority: 200, Policy: "test",
		ClientName: "SpatialEMU.Windows", MinClientVersion: "1.302", ClientPlatform: "windows-x64", Architecture: "x64",
		Runtime: runtime, EntryFile: "clone.zip", CanonicalSet: "clone", Status: "ready",
		Files: []domain.GameLaunchProfileFile{{Position: 0, SourceGameID: game.ID, SourceSHA1: sourceSHA1, SourceName: "clone.zip", Name: "clone.zip", Size: game.Size, Role: "entry"}},
	}
	if _, err := st.ReplaceGameLaunchProfiles("test", []domain.GameLaunchProfile{profile}, []domain.GameLaunchCatalogUpdate{{
		GameID: game.ID, Platform: "cps1", ROMSetName: "clone", EmulatorHint: "fbneo", CatalogRole: "game",
	}}); err != nil {
		t.Fatal(err)
	}
	request := domain.GameLaunchResolveRequest{
		Client:   domain.GameLaunchClient{Name: "SpatialEMU.Windows", Version: "1.302", Platform: "windows-x64", Architecture: "x64"},
		Runtimes: []domain.GameRuntimeDescriptor{runtime},
	}
	resolved, err := New(st).ResolveGameLaunchProfile(game.ID, request)
	if err != nil {
		t.Fatal(err)
	}
	if resolved.LaunchProfileID != profile.ID || resolved.ProfileRevision != 42 || len(resolved.Files) != 1 {
		t.Fatalf("resolution=%+v", resolved)
	}
}

func TestAuditedLaunchCandidatePriorityDoesNotDependOnRuntimeOrder(t *testing.T) {
	const sha1 = "0123456789abcdef0123456789abcdef01234567"
	client := domain.GameLaunchClient{Name: "SpatialEMU.Windows", Version: "1.302", Platform: "windows-x64", Architecture: "x64"}
	mame := domain.GameRuntimeDescriptor{ID: "mame", Version: "0.288", ContentSet: "mame-0.288"}
	fbneo := domain.GameRuntimeDescriptor{ID: "libretro", CoreID: "fbneo", CoreSHA256: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"}
	profile := func(id string, priority int, runtime domain.GameRuntimeDescriptor) auditedGameLaunchProfile {
		return auditedGameLaunchProfile{
			ID: id, Revision: 1, Priority: priority,
			ClientName: client.Name, MinClientVersion: "1.302", ClientPlatform: client.Platform, Architecture: client.Architecture,
			Runtime: runtime, EntrySHA1: sha1, EntrySourceName: "shared.zip",
			Files: []auditedGameLaunchFile{{SourceSHA1: sha1, SourceName: "shared.zip", Name: "shared.zip", Size: 1024, Role: "entry"}},
		}
	}
	profiles := []auditedGameLaunchProfile{
		profile("shared-windows-mame-v1", 100, mame),
		profile("shared-windows-fbneo-v1", 200, fbneo),
	}
	game := domain.GameAsset{FilePath: "/games/shared.zip", Size: 1024, SHA1: sha1}

	for _, runtimes := range [][]domain.GameRuntimeDescriptor{{mame, fbneo}, {fbneo, mame}} {
		candidates := matchingAuditedLaunchCandidates(profiles, game, domain.GameLaunchResolveRequest{Client: client, Runtimes: runtimes})
		if len(candidates) != 2 || candidates[0].Profile.ID != "shared-windows-fbneo-v1" || candidates[0].Runtime.CoreID != "fbneo" {
			t.Fatalf("candidates=%+v, want higher-priority FBNeo profile first", candidates)
		}
	}
}

func TestAuditedLaunchSelectionFallsBackWhenPreferredDependenciesAreMissing(t *testing.T) {
	originalProfiles := auditedGameLaunchProfiles
	t.Cleanup(func() { auditedGameLaunchProfiles = originalProfiles })

	preferred := originalProfiles[0]
	preferred.Priority = 200
	fallback := preferred
	fallback.ID = "vstriker-windows-fbneo-fallback-test-v1"
	fallback.Priority = 100
	fallback.Runtime = domain.GameRuntimeDescriptor{
		ID: "libretro", CoreID: "fbneo", CoreSHA256: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
	}
	fallback.Files = append([]auditedGameLaunchFile(nil), preferred.Files[0])
	auditedGameLaunchProfiles = []auditedGameLaunchProfile{preferred, fallback}

	conn, err := db.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	st := store.New(conn)
	root := t.TempDir()
	library, err := st.CreateLibrary("Games", root)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, preferred.EntrySourceName)
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Truncate(preferred.Files[0].Size); err != nil {
		_ = file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	game, err := st.UpsertGame(domain.GameAsset{
		LibraryID: library.ID, Title: "Virtua Striker", Platform: "model2", ROMSetName: "vstriker", Format: "zip",
		FilePath: path, RelPath: preferred.EntrySourceName, Size: preferred.Files[0].Size, MTime: time.Unix(1, 0),
		SHA1: preferred.EntrySHA1, EmulatorHint: "model2", Compatibility: "unknown", CatalogRole: "game",
	})
	if err != nil {
		t.Fatal(err)
	}

	request := domain.GameLaunchResolveRequest{
		Client:   domain.GameLaunchClient{Name: preferred.ClientName, Version: preferred.MinClientVersion, Platform: preferred.ClientPlatform, Architecture: preferred.Architecture},
		Runtimes: []domain.GameRuntimeDescriptor{preferred.Runtime, fallback.Runtime},
	}
	resolved, err := New(st).ResolveGameLaunchProfile(game.ID, request)
	if err != nil {
		t.Fatal(err)
	}
	if resolved.LaunchProfileID != fallback.ID || resolved.Runtime.CoreID != "fbneo" || len(resolved.Files) != 1 {
		t.Fatalf("resolution=%+v, want dependency-free FBNeo fallback", resolved)
	}
}

func TestCanonicalPragmaticVersionNormalizesWindowsVersions(t *testing.T) {
	tests := map[string]string{
		"  v0.82.2.0  ": "0.82.2",
		"0, 82, 2, 0":   "0.82.2",
		"2.6.3.0":       "2.6.3",
		"1.20.4":        "1.20.4",
		"2.6":           "2.6",
		"unknown":       "",
	}
	for input, expected := range tests {
		if actual := canonicalPragmaticVersion(input); actual != expected {
			t.Fatalf("canonicalPragmaticVersion(%q)=%q, want %q", input, actual, expected)
		}
	}
}

func TestValidatePragmaticDOSLaunchRejectsUnsafePaths(t *testing.T) {
	valid := domain.DOSLaunch{
		EntrySource: "curated", EntryFile: "GAME/START.BAT", WorkingDirectory: "GAME",
		Candidates: []domain.DOSLaunchCandidate{{Path: "GAME/START.BAT", Kind: "bat"}},
	}
	if err := validatePragmaticDOSLaunch(valid); err != nil {
		t.Fatalf("valid DOS launch rejected: %v", err)
	}
	valid.Candidates = []domain.DOSLaunchCandidate{
		{Path: "C&C107.EXE", Kind: "exe"},
		{Path: "(O)_(-).EXE", Kind: "exe"},
	}
	if err := validatePragmaticDOSLaunch(valid); err != nil {
		t.Fatalf("valid DOS candidate filenames rejected: %v", err)
	}

	unsafe := valid
	unsafe.EntryFile = "GAME/START.BAT|FORMAT"
	if err := validatePragmaticDOSLaunch(unsafe); err == nil {
		t.Fatal("expected shell metacharacter to be rejected")
	}
	unsafe = valid
	unsafe.Candidates = []domain.DOSLaunchCandidate{{Path: "../START.BAT", Kind: "bat"}}
	if err := validatePragmaticDOSLaunch(unsafe); err == nil {
		t.Fatal("expected unsafe candidate path to be rejected")
	}
}
