package launchcatalog

import (
	"testing"

	"foliospace-reader/internal/domain"
)

func TestAuditedSF2OverridesCandidateMetadata(t *testing.T) {
	game := domain.GameAsset{
		FilePath:     "/games/FBNeo/arcade/sf2.zip",
		Size:         3551819,
		SHA1:         "bd59872a57f14dc492e2fb387727a9402f3d4f97",
		Platform:     "arcade",
		ROMSetName:   "sf2",
		EmulatorHint: "mame",
		CatalogRole:  RoleNeedsCuration,
	}
	game = CanonicalizeAuditedGame(game)
	if game.Platform != "cps1" || game.ROMSetName != "sf2" || game.EmulatorHint != "fbneo" || CatalogRole(game, nil) != RoleGame {
		t.Fatalf("game = %#v, want audited CPS1/FBNeo entry", game)
	}
}

func TestExplicitNeedsCurationSurvivesForNonArcadeContent(t *testing.T) {
	game := domain.GameAsset{Platform: "unknown", CatalogRole: RoleNeedsCuration}
	if got := CatalogRole(game, nil); got != RoleNeedsCuration {
		t.Fatalf("CatalogRole = %q, want %q", got, RoleNeedsCuration)
	}
}

func TestCapcomAudioAndZNBIOSFilesAreDependencies(t *testing.T) {
	for _, path := range []string{
		"/games/Capcom BIOS/qsound.zip",
		"/games/Capcom BIOS/qsound_hle.zip",
		"/games/Capcom BIOS/dl-1425.bin",
		"/games/Capcom BIOS/coh1000c.zip",
		"/games/Capcom BIOS/coh3002c.zip",
	} {
		game := domain.GameAsset{FilePath: path, Platform: "mame", CatalogRole: RoleNeedsCuration}
		if got := CatalogRole(game, nil); got != RoleDependency {
			t.Fatalf("CatalogRole(%q) = %q, want %q", path, got, RoleDependency)
		}
	}
}
