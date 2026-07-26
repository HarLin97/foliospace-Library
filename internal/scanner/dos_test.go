package scanner

import (
	stdzip "archive/zip"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"foliospace-reader/internal/db"
	"foliospace-reader/internal/store"
)

func TestScanDOSArchiveUsesHashMatchedCatalogLaunchAndCover(t *testing.T) {
	root := t.TempDir()
	dosRoot := filepath.Join(root, "DOS")
	archivePath := filepath.Join(dosRoot, "bin", "catalog-name.zip")
	makeZip(t, archivePath, map[string]string{
		"GAME/PLAY.EXE":  "program",
		"GAME/SETUP.EXE": "setup",
		"README.TXT":     "readme",
	})
	checksums, err := fileChecksums(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	coverPath := filepath.Join(dosRoot, "img", "sample-id", "cover.png")
	if err := os.MkdirAll(filepath.Dir(coverPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(coverPath, []byte("cover"), 0o644); err != nil {
		t.Fatal(err)
	}
	catalog := dosCatalogFile{Games: map[string]dosCatalogEntry{
		"sample-id": {
			Identifier: "sample-id", Name: map[string]string{"zh-Hans": "示例 DOS 游戏", "en": "Sample DOS Game"},
			ReleaseYear: 1995, Executable: "game/play.exe 2", Keymaps: map[string]string{"Enter": "确认"},
			CoverFilename: "cover.png", SHA256: checksums.sha256, Filesize: info.Size(),
		},
	}}
	catalogJSON, err := json.Marshal(catalog)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dosRoot, "games.json"), catalogJSON, 0o644); err != nil {
		t.Fatal(err)
	}

	conn, err := db.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	st := store.New(conn)
	library, err := st.CreateLibraryWithType("GameROMS", root, "game")
	if err != nil {
		t.Fatal(err)
	}
	job, err := New(st).ScanLibrary(library)
	if err != nil {
		t.Fatal(err)
	}
	if job.Status != "completed" || job.IndexedFiles != 1 {
		t.Fatalf("job = %#v, want one indexed DOS game", job)
	}
	games, err := st.ListGamesByPlatform("dos")
	if err != nil {
		t.Fatal(err)
	}
	if len(games) != 1 {
		t.Fatalf("DOS games = %#v, want one", games)
	}
	game := games[0]
	if game.Title != "示例 DOS 游戏" || game.ROMSetName != "DOS" || game.EmulatorHint != "dosbox-staging" || game.Format != "zip" {
		t.Fatalf("game = %#v", game)
	}
	launch, err := st.DOSLaunch(game.ID)
	if err != nil {
		t.Fatal(err)
	}
	if launch.EntryFile != "GAME/PLAY.EXE" || launch.EntrySource != "curated" || len(launch.Arguments) != 1 || launch.Arguments[0] != "2" {
		t.Fatalf("launch = %#v", launch)
	}
	if launch.WorkingDirectory != "GAME" || len(launch.Candidates) != 2 || launch.KeymapHints["Enter"] != "确认" {
		t.Fatalf("launch metadata = %#v", launch)
	}
	details, err := st.GameDetails(game.ID)
	if err != nil {
		t.Fatal(err)
	}
	if details.Metadata.DisplayTitle != "示例 DOS 游戏" || details.Metadata.ReleaseDate != "1995" || len(details.Artwork) != 1 || details.Artwork[0].CachePath != coverPath {
		t.Fatalf("details = %#v", details)
	}
	second, err := New(st).ScanLibrary(library)
	if err != nil {
		t.Fatal(err)
	}
	if second.IndexedFiles != 0 || second.SkippedFiles != 1 {
		t.Fatalf("second scan = %#v, want unchanged DOS archive skipped", second)
	}
}

func TestInspectDOSArchiveFiltersUnsafeAndCaseCollidingCandidates(t *testing.T) {
	archivePath := filepath.Join(t.TempDir(), "unsafe.zip")
	file, err := os.Create(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	writer := stdzip.NewWriter(file)
	for _, name := range []string{"GAME/PLAY.EXE", "game/play.exe", "../BAD.EXE", "RUN.BAT", "dosbox.conf"} {
		entry, createErr := writer.Create(name)
		if createErr != nil {
			t.Fatal(createErr)
		}
		content := "x"
		if name == "dosbox.conf" {
			content = "[autoexec]\nRUN.BAT\n"
		}
		if _, writeErr := entry.Write([]byte(content)); writeErr != nil {
			t.Fatal(writeErr)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	inspection, err := inspectDOSArchive(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	if len(inspection.candidates) != 1 || inspection.candidates[0].Path != "RUN.BAT" {
		t.Fatalf("candidates = %#v, want only RUN.BAT", inspection.candidates)
	}
	entry, args, config, ok := resolveDOSBoxConfig(inspection)
	if !ok || entry != "RUN.BAT" || len(args) != 0 || config != "dosbox.conf" {
		t.Fatalf("config resolution = %q %#v %q %v", entry, args, config, ok)
	}
}

func TestResolveDOSCommandLeavesExtensionlessAmbiguityUnresolved(t *testing.T) {
	members := map[string]string{"run.exe": "RUN.EXE", "run.com": "RUN.COM"}
	if entry, args, ok := resolveDOSCommand("run", members); ok || entry != "" || args != nil {
		t.Fatalf("ambiguous command resolved to %q %#v", entry, args)
	}
}

func TestInspectDOSArchiveKeepsNestedCandidatesAndAllowsDataOnlyArchives(t *testing.T) {
	root := t.TempDir()
	nestedPath := filepath.Join(root, "nested.zip")
	makeZip(t, nestedPath, map[string]string{
		"GAME/BIN/PLAY.EXE": "program",
		"GAME/DATA.DAT":     "data",
	})
	inspection, err := inspectDOSArchive(nestedPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(inspection.candidates) != 1 || inspection.candidates[0].Path != "GAME/BIN/PLAY.EXE" {
		t.Fatalf("nested candidates = %#v", inspection.candidates)
	}

	dataOnlyPath := filepath.Join(root, "data-only.zip")
	makeZip(t, dataOnlyPath, map[string]string{"GAME/DATA.DAT": "data"})
	inspection, err = inspectDOSArchive(dataOnlyPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(inspection.candidates) != 0 {
		t.Fatalf("data-only candidates = %#v, want none", inspection.candidates)
	}
}

func TestScanDOSMalformedArchiveKeepsCatalogItemWithoutAuthoritativeEntry(t *testing.T) {
	root := t.TempDir()
	dosRoot := filepath.Join(root, "DOS")
	archivePath := filepath.Join(dosRoot, "bin", "broken.zip")
	if err := os.MkdirAll(filepath.Dir(archivePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(archivePath, []byte("not a zip archive"), 0o644); err != nil {
		t.Fatal(err)
	}
	checksums, err := fileChecksums(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	catalogJSON, err := json.Marshal(dosCatalogFile{Games: map[string]dosCatalogEntry{
		"broken": {
			Identifier: "broken", Name: map[string]string{"en": "Broken DOS Archive"}, Executable: "PLAY.EXE",
			SHA256: checksums.sha256, Filesize: info.Size(),
		},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dosRoot, "games.json"), catalogJSON, 0o644); err != nil {
		t.Fatal(err)
	}

	conn, err := db.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	st := store.New(conn)
	library, err := st.CreateLibraryWithType("GameROMS", root, "game")
	if err != nil {
		t.Fatal(err)
	}
	job, err := New(st).ScanLibrary(library)
	if err != nil {
		t.Fatal(err)
	}
	if job.Status != "completed" || job.IndexedFiles != 1 || job.ErrorCount != 0 {
		t.Fatalf("job = %#v", job)
	}
	games, err := st.ListGamesByPlatform("dos")
	if err != nil || len(games) != 1 {
		t.Fatalf("games = %#v, err = %v", games, err)
	}
	launch, err := st.DOSLaunch(games[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if games[0].Title != "Broken DOS Archive" || launch.EntryFile != "" || launch.EntrySource != "unknown" || launch.AuditStatus != "archive_inventory_unavailable" {
		t.Fatalf("game = %#v, launch = %#v", games[0], launch)
	}
}

func TestScanDirectDOSExecutablePublishesItAsTheCuratedEntry(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "DOS", "PLAY.COM")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("program"), 0o644); err != nil {
		t.Fatal(err)
	}
	conn, err := db.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	st := store.New(conn)
	library, err := st.CreateLibraryWithType("GameROMS", root, "game")
	if err != nil {
		t.Fatal(err)
	}
	job, err := New(st).ScanLibrary(library)
	if err != nil {
		t.Fatal(err)
	}
	if job.Status != "completed" || job.IndexedFiles != 1 {
		t.Fatalf("job = %#v", job)
	}
	games, err := st.ListGamesByPlatform("dos")
	if err != nil || len(games) != 1 {
		t.Fatalf("games = %#v, err = %v", games, err)
	}
	launch, err := st.DOSLaunch(games[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if launch.EntryFile != "PLAY.COM" || launch.EntrySource != "curated" || len(launch.Candidates) != 1 || launch.Candidates[0].Kind != "com" {
		t.Fatalf("launch = %#v", launch)
	}
}
