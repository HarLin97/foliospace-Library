package httpapi

import (
	"encoding/json"
	"strings"
	"testing"

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
	game := domain.GameAsset{ID: 43, Title: "DOS Game", Platform: "dos", Format: "zip", Size: 123, SHA1: strings.Repeat("b", 40), EmulatorHint: "dosbox-staging"}
	files := []domain.GameFile{{Name: "game.zip", Size: 123, Role: "entry", Position: 0}}
	launch := domain.DOSLaunch{EntryFile: "GAME/PLAY.EXE", EntrySource: "curated", WorkingDirectory: "GAME", Arguments: []string{"2"}, Candidates: []domain.DOSLaunchCandidate{}}
	manifest := clientGameManifest(game, files, &launch)
	if manifest.EntryFile == nil || *manifest.EntryFile != "GAME/PLAY.EXE" {
		t.Fatalf("entryFile = %#v", manifest.EntryFile)
	}
	if manifest.DOSLaunch.WorkingDirectory == nil || *manifest.DOSLaunch.WorkingDirectory != "GAME" {
		t.Fatalf("dosLaunch = %#v", manifest.DOSLaunch)
	}
}
