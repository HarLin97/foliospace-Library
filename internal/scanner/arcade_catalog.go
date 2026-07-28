package scanner

import (
	"path/filepath"
	"strings"
)

// CPS families are pinned to the MAME 0.288 driver classification for the
// currently managed mixed CPS catalog. Unknown sets remain ordinary arcade
// entries until they are explicitly classified.
var cpsROMSetPlatforms = map[string]string{
	// CPS-1
	"1941": "cps1", "3wonders": "cps1", "captcomm": "cps1", "cawing": "cps1",
	"cworld2j": "cps1", "dino": "cps1", "dynwarj": "cps1", "ffight": "cps1",
	"forgottn": "cps1", "ghouls": "cps1", "knights": "cps1", "kod": "cps1",
	"mbombrd": "cps1", "megaman": "cps1", "mercs": "cps1", "msword": "cps1",
	"mtwins": "cps1", "nemo": "cps1", "pang3": "cps1", "pnickj": "cps1",
	"punisher": "cps1", "qad": "cps1", "qtono2j": "cps1", "sf2": "cps1",
	"sf2ce": "cps1", "sf2hf": "cps1", "slammast": "cps1", "strider": "cps1",
	"unsquad": "cps1", "varth": "cps1", "willow": "cps1", "wofa": "cps1",

	// CPS-2
	"1944": "cps2", "19xx": "cps2", "armwar": "cps2", "avsp": "cps2",
	"batcir": "cps2", "csclub": "cps2", "cybots": "cps2", "ddsom": "cps2",
	"ddtod": "cps2", "dimahoo": "cps2", "dstlk": "cps2", "ecofghtr": "cps2",
	"gigawing": "cps2", "hsf2": "cps2", "jyangoku": "cps2", "megaman2": "cps2",
	"mmatrix": "cps2", "mpang": "cps2", "msh": "cps2", "mshvsf": "cps2",
	"mvsc": "cps2", "nwarr": "cps2", "progear": "cps2", "pzloop2": "cps2",
	"ringdest": "cps2", "sfa": "cps2", "sfa2": "cps2", "sfa3": "cps2",
	"sgemf": "cps2", "spf2t": "cps2", "ssf2": "cps2", "ssf2t": "cps2",
	"vhunt2": "cps2", "vsav": "cps2", "vsav2": "cps2", "xmcota": "cps2",
	"xmvsf": "cps2",

	// CPS-3
	"jojo": "cps3", "jojoba": "cps3", "redearth": "cps3", "sfiii": "cps3",
	"sfiii2": "cps3", "sfiii3": "cps3",
}

func cpsPlatformForROMSet(path string, ext string) string {
	if ext != ".zip" && ext != ".7z" {
		return ""
	}
	return cpsROMSetPlatforms[gameROMSetStem(path, ext)]
}

func gameROMSetStem(path string, ext string) string {
	name := filepath.Base(path)
	if ext == "" {
		ext = filepath.Ext(name)
	}
	return strings.ToLower(strings.TrimSpace(strings.TrimSuffix(name, ext)))
}

func canonicalArcadeCatalogMetadata(platform string, path string, ext string) (romSetName string, emulatorHint string, catalogRole string, ok bool) {
	platform = strings.ToLower(strings.TrimSpace(platform))
	switch platform {
	case "cps1", "cps2", "cps3":
		return gameROMSetStem(path, ext), "fbneo", "game", true
	case "mame":
		romSetName = gameROMSetStem(path, ext)
		catalogRole = "game"
		if romSetName == "ym2413_instruments" {
			catalogRole = "dependency"
		}
		return romSetName, "mame", catalogRole, true
	default:
		return "", "", "", false
	}
}
