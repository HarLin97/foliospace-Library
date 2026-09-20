package httpapi

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"foliospace-reader/internal/db"
	"foliospace-reader/internal/domain"
	"foliospace-reader/internal/launchprofile"
	"foliospace-reader/internal/service"
	"foliospace-reader/internal/store"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestAPIMAMEContentEquivalencePreservesAuditAndRejectsChangedContent(t *testing.T) {
	config, root := t.TempDir(), t.TempDir()
	conn, err := db.Open(config)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	st := store.New(conn)
	lib, err := st.CreateLibrary("Games", root)
	if err != nil {
		t.Fatal(err)
	}
	add := func(name string) domain.GameAsset {
		t.Helper()
		data := []byte("audited-" + name)
		path := filepath.Join(root, name+".zip")
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
		h := sha1.Sum(data)
		g, err := st.UpsertGame(domain.GameAsset{LibraryID: lib.ID, Title: name, Platform: "mame", ROMSetName: name, Format: "zip", FilePath: path, RelPath: name + ".zip", Size: int64(len(data)), SHA1: hex.EncodeToString(h[:]), MTime: time.Now(), EmulatorHint: "mame", CatalogRole: "game"})
		if err != nil {
			t.Fatal(err)
		}
		return g
	}
	entry, dep := add("clone"), add("bios")
	profile := domain.GameLaunchProfile{GameID: entry.ID, ID: "clone-ipados-mame-0288-11111111", Revision: 0x11111111, Priority: 200, Policy: "mame-0.288-listxml", ClientName: "SpatialEMU.iPadOS", MinClientVersion: "1.300", ClientPlatform: "ipados-arm64", Architecture: "arm64", Runtime: domain.GameRuntimeDescriptor{ID: "mame", Version: "0.288", ContentSet: "mame-0.288"}, EntryFile: "clone.zip", CanonicalSet: "clone", Status: "ready"}
	for i, g := range []domain.GameAsset{entry, dep} {
		role := "entry"
		if i > 0 {
			role = "dependency"
		}
		profile.Files = append(profile.Files, domain.GameLaunchProfileFile{Position: i, SourceGameID: g.ID, SourceSHA1: g.SHA1, SourceName: filepath.Base(g.FilePath), Name: filepath.Base(g.FilePath), Size: g.Size, Role: role})
	}
	if _, err := st.ReplaceGameLaunchProfiles(profile.Policy, []domain.GameLaunchProfile{profile}, nil); err != nil {
		t.Fatal(err)
	}
	defs := []launchprofile.MAMEContentDefinition{
		{Set: "clone", Version: "0.288", ListXMLSHA256: strings.Repeat("1", 64), DefinitionSHA256: strings.Repeat("a", 64)},
		{Set: "clone", Version: "0.289", ListXMLSHA256: strings.Repeat("2", 64), DefinitionSHA256: strings.Repeat("a", 64)},
		{Set: "clone", Version: "0.290", ListXMLSHA256: strings.Repeat("3", 64), DefinitionSHA256: strings.Repeat("b", 64)},
	}
	policyPath := filepath.Join(config, "policies", "mame-content-equivalence.json")
	if err := os.MkdirAll(filepath.Dir(policyPath), 0700); err != nil {
		t.Fatal(err)
	}
	writeRegistry := func() {
		t.Helper()
		b, _ := json.Marshal(launchprofile.MAMEContentRegistry{SchemaVersion: 1, Definitions: defs})
		if err := os.WriteFile(policyPath, b, 0600); err != nil {
			t.Fatal(err)
		}
	}
	writeRegistry()
	ts := httptest.NewServer(NewWithOptions(service.NewWithConfig(st, config), nil, Options{APIToken: "secret"}).Routes())
	defer ts.Close()
	request := domain.GameLaunchResolveRequest{Client: domain.GameLaunchClient{Name: "SpatialEMU.iPadOS", Version: "1.503", Platform: "ipados-arm64", Architecture: "arm64"}}
	probe := func(version, content string, want int) launchResolveHTTPResponse {
		t.Helper()
		request.Runtimes = []domain.GameRuntimeDescriptor{{ID: "mame", Version: version, ContentSet: content}}
		r := postLaunchResolve(t, ts.URL, entry.ID, "secret", request, nil)
		if r.StatusCode != want {
			t.Fatalf("%s %s status=%d body=%s", version, content, r.StatusCode, r.Body)
		}
		return r
	}
	old := probe("0.288", "mame-0.288", 200)
	r := probe("0.289", "mame-0.289", 200)
	var out clientGameLaunchResolutionResponse
	if err := json.Unmarshal(r.Body, &out); err != nil {
		t.Fatal(err)
	}
	if out.LaunchProfileID != profile.ID || out.ProfileRevision != profile.Revision || out.Runtime != request.Runtimes[0] || len(out.Manifest.Files) != 2 || out.ContentAudit == nil || out.ContentAudit.SourceRuntime != profile.Runtime || out.ContentAudit.RequestedListXMLSHA256 != defs[1].ListXMLSHA256 {
		t.Fatalf("unexpected resolution %+v", out)
	}
	probe("0.290", "mame-0.290", 409)
	probe("0.999", "mame-0.999", 409)
	probe("0.289", "mame-0.999", 409)
	request.Client.Architecture = "x64"
	probe("0.289", "mame-0.289", 409)
	request.Client.Architecture = "arm64"
	// Updating verified data makes later equivalent cores work without new profiles or restart.
	defs = append(defs, launchprofile.MAMEContentDefinition{Set: "clone", Version: "0.291", ListXMLSHA256: strings.Repeat("4", 64), DefinitionSHA256: strings.Repeat("a", 64)})
	writeRegistry()
	probe("0.291", "mame-0.291", 200)
	defs[0].ListXMLSHA256 = strings.Repeat("5", 64)
	writeRegistry()
	probe("0.289", "mame-0.289", 409)
	defs[0].ListXMLSHA256 = strings.Repeat("1", 64)
	writeRegistry()
	bytes, err := os.ReadFile(dep.FilePath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(dep.FilePath); err != nil {
		t.Fatal(err)
	}
	missing := probe("0.289", "mame-0.289", 409)
	if !strings.Contains(string(missing.Body), "dependency-missing") {
		t.Fatalf("%s", missing.Body)
	}
	if err := os.WriteFile(dep.FilePath, []byte(strings.Repeat("x", len(bytes))), 0600); err != nil {
		t.Fatal(err)
	}
	changed := probe("0.289", "mame-0.289", 409)
	if !strings.Contains(string(changed.Body), "content-checksum-mismatch") {
		t.Fatalf("%s", changed.Body)
	}
	if err := os.WriteFile(dep.FilePath, bytes, 0600); err != nil {
		t.Fatal(err)
	}
	again := probe("0.288", "mame-0.288", 200)
	if string(old.Body) != string(again.Body) {
		t.Fatal("legacy response changed")
	}
	if err := os.Remove(policyPath); err != nil {
		t.Fatal(err)
	}
	probe("0.288", "mame-0.288", 200)
	probe("0.289", "mame-0.289", 409)
	// A real bundled provenance record resolves without installing an operator file.
	source, _, ok := launchprofile.DefaultMAMEContentRegistry().Equivalent("strider2u", "0.288", "0.289")
	if !ok {
		t.Fatal("missing bundled evidence")
	}
	profile.CanonicalSet = "strider2u"
	profile.ID = "strider2u-ipados-mame-0288-" + source.ListXMLSHA256[:8]
	revision, err := strconv.ParseUint(source.ListXMLSHA256[:8], 16, 32)
	if err != nil {
		t.Fatal(err)
	}
	profile.Revision = int(revision & 0x7fffffff)
	if _, err := st.ReplaceGameLaunchProfiles(profile.Policy, []domain.GameLaunchProfile{profile}, nil); err != nil {
		t.Fatal(err)
	}
	bundled := probe("0.289", "mame-0.289", 200)
	if !strings.Contains(string(bundled.Body), "mame-content-equivalence-v1") {
		t.Fatalf("missing bundled audit: %s", bundled.Body)
	}
	if err := os.WriteFile(policyPath, []byte(`invalid`), 0600); err != nil {
		t.Fatal(err)
	}
	probe("0.289", "mame-0.289", 409)
	probe("0.288", "mame-0.288", 200)
}
