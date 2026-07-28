package launchcatalog

import (
	"path/filepath"
	"strings"

	"foliospace-reader/internal/domain"
)

const (
	RoleGame          = "game"
	RoleDependency    = "dependency"
	RoleNeedsCuration = "needs-curation"
)

type auditedEntry struct {
	name string
	size int64
	sha1 string
}

// These entry fingerprints are the assets backed by an exact runtime profile.
// Keep this list aligned with service.auditedGameLaunchProfiles.
var auditedEntries = []auditedEntry{
	{name: "vstriker.zip", size: 10313686, sha1: "8e3518318eeb157ab299b2f284faef176d3f49dd"},
	{name: "tektagtac1.zip", size: 120980600, sha1: "d6615a3a70ea9941b61ccd608054a0044d3d6ab3"},
	{name: "sf2.zip", size: 3551819, sha1: "bd59872a57f14dc492e2fb387727a9402f3d4f97"},
	{name: "sfa.zip", size: 7365582, sha1: "61dece364b8d2f2ff15391505168be334ebb371a"},
	{name: "sfiii.zip", size: 38868517, sha1: "7aae0cfc4ef8911f19d2e986cee63807deebf1b6"},
	{name: "hypreact.zip", size: 8052342, sha1: "e0940f848884c9d53bbc41bb947d584e06cc1845"},
	{name: "hypreac2.zip", size: 18291541, sha1: "7fe73cc7ee40a49225a4616106e538c084ef4364"},
	{name: "srmp4.zip", size: 7697767, sha1: "cfcf2cdf61ebca862a84473a8bf75fbe8d76cb7b"},
	{name: "fromancr.zip", size: 14121810, sha1: "137e4949d7e204ff10e33372528cc1e9481b962c"},
	{name: "fromanc4.zip", size: 21443327, sha1: "ff478f3350d9703e8647f659ce169ee234082249"},
	{name: "mcnpshnt.zip", size: 1205007, sha1: "24a714371a867db1709798a95a171778e0940021"},
}

func IsStrictArcadePlatform(platform string) bool {
	switch strings.ToLower(strings.TrimSpace(platform)) {
	case "arcade", "mame", "model2", "cps", "cps1", "cps2", "cps3", "neogeo":
		return true
	default:
		return false
	}
}

func IsAuditedEntry(game domain.GameAsset) bool {
	name := strings.ToLower(filepath.Base(strings.TrimSpace(game.FilePath)))
	sha1 := strings.ToLower(strings.TrimSpace(game.SHA1))
	for _, entry := range auditedEntries {
		if name == entry.name && game.Size == entry.size && sha1 == entry.sha1 {
			return true
		}
	}
	return false
}

func IsAuditedEntryIdentity(name string, size int64, sha1 string) bool {
	return IsAuditedEntry(domain.GameAsset{FilePath: name, Size: size, SHA1: sha1})
}

func IsDOSReady(launch domain.DOSLaunch) bool {
	source := strings.ToLower(strings.TrimSpace(launch.EntrySource))
	if source != "curated" && source != "dosboxconfig" {
		return false
	}
	switch strings.ToLower(filepath.Ext(strings.TrimSpace(launch.EntryFile))) {
	case ".bat", ".com", ".exe":
		return true
	default:
		return false
	}
}

func CatalogRole(game domain.GameAsset, dosLaunch *domain.DOSLaunch) string {
	role := strings.ToLower(strings.TrimSpace(game.CatalogRole))
	if role == RoleDependency || isKnownDependency(game.FilePath) {
		return RoleDependency
	}
	platform := strings.ToLower(strings.TrimSpace(game.Platform))
	if platform == "dos" {
		if dosLaunch != nil && IsDOSReady(*dosLaunch) {
			return RoleGame
		}
		return RoleNeedsCuration
	}
	if IsStrictArcadePlatform(platform) {
		if IsAuditedEntry(game) {
			return RoleGame
		}
		return RoleNeedsCuration
	}
	return RoleGame
}

func isKnownDependency(path string) bool {
	switch strings.ToLower(filepath.Base(strings.TrimSpace(path))) {
	case "neogeo.zip", "segabill.zip", "ym2413_instruments.zip":
		return true
	default:
		return false
	}
}
