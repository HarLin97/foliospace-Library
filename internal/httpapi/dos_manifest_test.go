package httpapi

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"foliospace-reader/internal/domain"
)

func TestClientDOSManifestPublishesArchiveChecksumAndNullableEntry(t *testing.T) {
	game := domain.GameAsset{ID: 42, Title: "DOS Game", Platform: "dos", Format: "zip", Size: 123, SHA1: strings.Repeat("a", 40), EmulatorHint: "dosbox-staging"}
	files := []domain.GameFile{{Name: "game.zip", Size: 123, Role: "entry", Position: 0}}
	launch := domain.DOSLaunch{EntrySource: "unknown", Arguments: []string{}, Candidates: []domain.DOSLaunchCandidate{{Path: "GAME/PLAY.EXE", Kind: "exe"}}}
	manifest := clientGameManifest(game, files, &launch)
	if manifest.EntryFile != nil {
		t.Fatalf("entryFile = %#v, want nil", manifest.EntryFile)
	}
	if manifest.DOSLaunch == nil || manifest.DOSLaunch.EntrySource != "unknown" || len(manifest.DOSLaunch.Candidates) != 1 {
		t.Fatalf("dosLaunch = %#v", manifest.DOSLaunch)
	}
	if len(manifest.Files) != 1 || manifest.Files[0].Checksum != game.SHA1 {
		t.Fatalf("files = %#v", manifest.Files)
	}
	encoded, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(encoded), `"entryFile":null`) {
		t.Fatalf("manifest JSON = %s, want explicit null entryFile", encoded)
	}
}

func TestClientDOSManifestUsesCuratedInnerEntry(t *testing.T) {
	updatedAt := time.Date(2026, 7, 26, 8, 0, 0, 0, time.UTC)
	game := domain.GameAsset{ID: 43, Title: "DOS Game", Platform: "dos", Format: "zip", Size: 123, SHA1: strings.Repeat("b", 40), EmulatorHint: "dosbox-staging", UpdatedAt: updatedAt}
	files := []domain.GameFile{{Name: "game.zip", Size: 123, Role: "entry", Position: 0}}
	launch := domain.DOSLaunch{EntryFile: "GAME/PLAY.EXE", EntrySource: "curated", InstallDirectory: "DRA4", WorkingDirectory: "GAME", Arguments: []string{"2"}, Candidates: []domain.DOSLaunchCandidate{}}
	manifest := clientGameManifest(game, files, &launch)
	if manifest.EntryFile == nil || *manifest.EntryFile != "GAME/PLAY.EXE" {
		t.Fatalf("entryFile = %#v", manifest.EntryFile)
	}
	if manifest.DOSLaunch.WorkingDirectory == nil || *manifest.DOSLaunch.WorkingDirectory != "GAME" {
		t.Fatalf("dosLaunch = %#v", manifest.DOSLaunch)
	}
	if manifest.DOSLaunch.InstallDirectory == nil || *manifest.DOSLaunch.InstallDirectory != "DRA4" || manifest.UpdatedAt != updatedAt.Format(time.RFC3339Nano) {
		t.Fatalf("manifest = %#v", manifest)
	}
}
