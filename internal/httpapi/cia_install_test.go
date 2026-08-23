package httpapi

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
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

func TestAPICIAInstallCapabilityCatalogManifestNegotiationAndRange(t *testing.T) {
	ts, cia, direct, ciaBody := ciaInstallTestServer(t)
	defer ts.Close()

	info := authGet(t, ts.URL+"/api/client/info", "secret")
	if !strings.Contains(info, `"ciaInstallV1":true`) {
		t.Fatalf("client info = %s, want ciaInstallV1", info)
	}

	catalog := authGet(t, ts.URL+"/api/client/games?platform=3ds", "secret")
	for _, want := range []string{
		`"platform":"3ds"`, `"format":"cia"`, `"contentMode":"install"`,
		`"validation":"client"`, `"installUrl":"/api/client/games/` + itoa(cia.ID) + `/install"`,
		`"fileName":"Install Package.cia"`, `"size":` + itoa(int64(len(ciaBody))),
		`"sha1":"` + cia.SHA1 + `"`,
	} {
		if !strings.Contains(catalog, want) {
			t.Fatalf("CIA catalog = %s, missing %s", catalog, want)
		}
	}

	manifest := authGet(t, ts.URL+"/api/client/games/"+itoa(cia.ID)+"/manifest", "secret")
	for _, want := range []string{
		`"contentMode":"install"`, `"validation":"client"`,
		`"entryFile":"Install Package.cia"`, `"name":"Install Package.cia"`,
		`"size":` + itoa(int64(len(ciaBody))), `"checksum":"sha1:` + cia.SHA1 + `"`,
	} {
		if !strings.Contains(manifest, want) {
			t.Fatalf("CIA manifest = %s, missing %s", manifest, want)
		}
	}

	unsupported := postGameContentAction(t, ts.URL, cia.ID, "install", map[string]any{
		"client":       apple3DSInstallClient(),
		"capabilities": []string{},
	})
	if unsupported.StatusCode != http.StatusConflict || !bytes.Contains(unsupported.Body, []byte(`"code":"content-mode-unsupported"`)) {
		t.Fatalf("unsupported install status=%d body=%s", unsupported.StatusCode, unsupported.Body)
	}

	installed := postGameContentAction(t, ts.URL, cia.ID, "install", map[string]any{
		"client":       apple3DSInstallClient(),
		"capabilities": []string{"cia-install-v1"},
	})
	if installed.StatusCode != http.StatusOK {
		t.Fatalf("install status=%d body=%s", installed.StatusCode, installed.Body)
	}
	for _, want := range [][]byte{
		[]byte(`"action":"install"`), []byte(`"contentMode":"install"`), []byte(`"validation":"client"`),
		[]byte(`"entryFile":"Install Package.cia"`), []byte(`"checksum":"sha1:` + cia.SHA1 + `"`),
	} {
		if !bytes.Contains(installed.Body, want) {
			t.Fatalf("install response = %s, missing %s", installed.Body, want)
		}
	}

	wrongAction := postGameContentAction(t, ts.URL, direct.ID, "install", map[string]any{
		"client":       apple3DSInstallClient(),
		"capabilities": []string{"cia-install-v1"},
	})
	if wrongAction.StatusCode != http.StatusConflict ||
		!bytes.Contains(wrongAction.Body, []byte(`"code":"content-mode-unsupported"`)) ||
		!bytes.Contains(wrongAction.Body, []byte("uses launch mode and does not support the install action")) {
		t.Fatalf("direct image install status=%d body=%s", wrongAction.StatusCode, wrongAction.Body)
	}

	rangeReq, err := http.NewRequest(http.MethodGet, ts.URL+"/api/client/games/"+itoa(cia.ID)+"/file", nil)
	if err != nil {
		t.Fatal(err)
	}
	rangeReq.Header.Set("Authorization", "Bearer secret")
	rangeReq.Header.Set("Range", "bytes=4-11")
	rangeResp, err := http.DefaultClient.Do(rangeReq)
	if err != nil {
		t.Fatal(err)
	}
	defer rangeResp.Body.Close()
	rangeBody, err := io.ReadAll(rangeResp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if rangeResp.StatusCode != http.StatusPartialContent || !bytes.Equal(rangeBody, ciaBody[4:12]) ||
		rangeResp.Header.Get("Content-Range") != "bytes 4-11/"+itoa(int64(len(ciaBody))) ||
		rangeResp.Header.Get("Content-Length") != "8" || rangeResp.Header.Get("Accept-Ranges") != "bytes" ||
		rangeResp.Header.Get("Content-Disposition") != `attachment; filename="Install Package.cia"` ||
		rangeResp.Header.Get("X-FolioSpace-Content-Mode") != "install" ||
		rangeResp.Header.Get("X-FolioSpace-Validation") != "client" {
		t.Fatalf("CIA range status=%d body=%x headers=%v", rangeResp.StatusCode, rangeBody, rangeResp.Header)
	}

	directManifest := authGet(t, ts.URL+"/api/client/games/"+itoa(direct.ID)+"/manifest", "secret")
	if !strings.Contains(directManifest, `"contentMode":"launch"`) || strings.Contains(directManifest, `"installUrl"`) || strings.Contains(directManifest, `"validation":"client"`) {
		t.Fatalf("direct 3DS manifest changed = %s", directManifest)
	}
}

func TestAPICIARejectsOrdinaryLaunchResolverAndInvalidIndexedIntegrity(t *testing.T) {
	ts, cia, _, _ := ciaInstallTestServer(t)
	defer ts.Close()

	resolve := postLaunchResolve(t, ts.URL, cia.ID, "secret", domain.GameLaunchResolveRequest{
		Client:   domain.GameLaunchClient{Name: "SpatialEMU.visionOS", Version: "1.320", Platform: "visionos-arm64", Architecture: "arm64"},
		Runtimes: []domain.GameRuntimeDescriptor{{ID: "libretro", CoreID: "azahar"}},
	}, nil)
	if resolve.StatusCode != http.StatusConflict || !bytes.Contains(resolve.Body, []byte(`"code":"content-mode-unsupported"`)) || bytes.Contains(resolve.Body, []byte("launch-profile-missing")) {
		t.Fatalf("CIA resolver status=%d body=%s", resolve.StatusCode, resolve.Body)
	}

	conn, err := db.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	st := store.New(conn)
	root := t.TempDir()
	body := []byte("indexed-but-changed")
	checksumPath := filepath.Join(root, "Broken Checksum.cia")
	if err := os.WriteFile(checksumPath, body, 0o644); err != nil {
		t.Fatal(err)
	}
	lib, err := st.CreateLibraryWithType("3DS", root, "game")
	if err != nil {
		t.Fatal(err)
	}
	brokenChecksum, err := st.UpsertGame(domain.GameAsset{
		LibraryID: lib.ID, Title: "Broken Checksum", Platform: "3ds", ROMSetName: "Nintendo 3DS", Format: "cia",
		FilePath: checksumPath, RelPath: "Broken Checksum.cia", Size: int64(len(body)), MTime: time.Now(), CRC32: "12345678",
		SHA1: strings.Repeat("a", 40), EmulatorHint: "spatialemu-3ds-companion", Compatibility: "untested", CatalogRole: "needs-curation",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := st.ReplaceGameFiles(brokenChecksum.ID, []domain.GameFile{{
		Name: "Broken Checksum.cia", FilePath: checksumPath, Size: int64(len(body)), MTime: time.Now(), SHA1: strings.Repeat("b", 40), Role: "entry", Position: 0,
	}}); err != nil {
		t.Fatal(err)
	}

	sizePath := filepath.Join(root, "Broken Size.cia")
	if err := os.WriteFile(sizePath, body, 0o644); err != nil {
		t.Fatal(err)
	}
	brokenSize, err := st.UpsertGame(domain.GameAsset{
		LibraryID: lib.ID, Title: "Broken Size", Platform: "3ds", ROMSetName: "Nintendo 3DS", Format: "cia",
		FilePath: sizePath, RelPath: "Broken Size.cia", Size: 999, MTime: time.Now(), CRC32: "87654321",
		SHA1: strings.Repeat("c", 40), EmulatorHint: "spatialemu-3ds-companion", Compatibility: "untested", CatalogRole: "needs-curation",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := st.ReplaceGameFiles(brokenSize.ID, []domain.GameFile{{
		Name: "Broken Size.cia", FilePath: sizePath, Size: 999, MTime: time.Now(), SHA1: strings.Repeat("c", 40), Role: "entry", Position: 0,
	}}); err != nil {
		t.Fatal(err)
	}

	brokenServer := httptest.NewServer(NewWithOptions(service.New(st), nil, Options{APIToken: "secret"}).Routes())
	defer brokenServer.Close()
	for _, gameID := range []int64{brokenChecksum.ID, brokenSize.ID} {
		response := postGameContentAction(t, brokenServer.URL, gameID, "install", map[string]any{
			"client":       apple3DSInstallClient(),
			"capabilities": []string{"cia-install-v1"},
		})
		if response.StatusCode != http.StatusConflict || !bytes.Contains(response.Body, []byte(`"code":"content-integrity-invalid"`)) {
			t.Fatalf("invalid integrity game=%d status=%d body=%s", gameID, response.StatusCode, response.Body)
		}
	}
}

func ciaInstallTestServer(t *testing.T) (*httptest.Server, domain.GameAsset, domain.GameAsset, []byte) {
	t.Helper()
	conn, err := db.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	st := store.New(conn)
	root := t.TempDir()
	lib, err := st.CreateLibraryWithType("3DS", root, "game")
	if err != nil {
		t.Fatal(err)
	}

	ciaBody := []byte("synthetic-cia-transfer-body")
	ciaPath := filepath.Join(root, "Install Package.cia")
	if err := os.WriteFile(ciaPath, ciaBody, 0o644); err != nil {
		t.Fatal(err)
	}
	ciaHash := sha1.Sum(ciaBody)
	ciaSHA1 := hex.EncodeToString(ciaHash[:])
	cia, err := st.UpsertGame(domain.GameAsset{
		LibraryID: lib.ID, Title: "Install Package", Platform: "3ds", ROMSetName: "Nintendo 3DS", Format: "cia",
		FilePath: ciaPath, RelPath: "Install Package.cia", Size: int64(len(ciaBody)), MTime: time.Now(),
		CRC32: "12345678", SHA1: ciaSHA1, EmulatorHint: "spatialemu-3ds-companion", Compatibility: "untested", CatalogRole: "needs-curation",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := st.ReplaceGameFiles(cia.ID, []domain.GameFile{{
		Name: "Install Package.cia", FilePath: ciaPath, Size: int64(len(ciaBody)), MTime: time.Now(), SHA1: ciaSHA1, Role: "entry", Position: 0,
	}}); err != nil {
		t.Fatal(err)
	}

	directBody := append(make([]byte, 0x100), []byte("NCSDdirect-image")...)
	directPath := filepath.Join(root, "Direct.3ds")
	if err := os.WriteFile(directPath, directBody, 0o644); err != nil {
		t.Fatal(err)
	}
	directHash := sha1.Sum(directBody)
	directSHA1 := hex.EncodeToString(directHash[:])
	direct, err := st.UpsertGame(domain.GameAsset{
		LibraryID: lib.ID, Title: "Direct", Platform: "3ds", ROMSetName: "Nintendo 3DS", Format: "3ds",
		FilePath: directPath, RelPath: "Direct.3ds", Size: int64(len(directBody)), MTime: time.Now(),
		CRC32: "87654321", SHA1: directSHA1, EmulatorHint: "spatialemu-3ds-companion", Compatibility: "untested", CatalogRole: "game",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := st.ReplaceGameFiles(direct.ID, []domain.GameFile{{
		Name: "Direct.3ds", FilePath: directPath, Size: int64(len(directBody)), MTime: time.Now(), SHA1: directSHA1, Role: "entry", Position: 0,
	}}); err != nil {
		t.Fatal(err)
	}

	return httptest.NewServer(NewWithOptions(service.New(st), nil, Options{APIToken: "secret"}).Routes()), cia, direct, ciaBody
}

func apple3DSInstallClient() map[string]string {
	return map[string]string{
		"name": "SpatialEMU.visionOS", "version": "1.320", "platform": "visionos-arm64", "architecture": "arm64",
	}
}

func postGameContentAction(t *testing.T, baseURL string, gameID int64, action string, value any) launchResolveHTTPResponse {
	t.Helper()
	body, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest(http.MethodPost, baseURL+"/api/client/games/"+itoa(gameID)+"/"+action, bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer secret")
	req.Header.Set("Content-Type", "application/json")
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
