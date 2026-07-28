package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"path/filepath"
	"sort"
	"strings"

	"foliospace-reader/internal/config"
	"foliospace-reader/internal/db"
	"foliospace-reader/internal/domain"
	"foliospace-reader/internal/launchcatalog"
	"foliospace-reader/internal/launchprofile"
	"foliospace-reader/internal/store"
)

const fbNeoCoreSHA256 = "6ebc2675c272c8d654935647ac336d45bbd97452c4d5943290d5ffc75678d9f1"

type rebuildOutput struct {
	Policy          string                                `json:"policy"`
	DATName         string                                `json:"datName"`
	DATVersion      string                                `json:"datVersion"`
	DATSHA256       string                                `json:"datSha256"`
	ProfileRevision int                                   `json:"profileRevision"`
	Candidates      int                                   `json:"candidates"`
	Matched         int                                   `json:"matched"`
	Result          domain.GameLaunchProfileRebuildResult `json:"result"`
	Failures        []string                              `json:"failures,omitempty"`
	DryRun          bool                                  `json:"dryRun"`
}

func main() {
	cfg := config.Load()
	datPath := flag.String("dat", filepath.Join(cfg.ConfigDir, "policies", "fbneo-arcade.dat"), "official FBNeo arcade DAT path")
	dryRun := flag.Bool("dry-run", false, "audit without writing SQLite")
	failureLimit := flag.Int("failure-limit", 50, "maximum failure details to print")
	flag.Parse()

	catalog, err := launchprofile.ParseFBNeoDATFile(*datPath)
	if err != nil {
		log.Fatal(err)
	}
	conn, err := db.Open(cfg.ConfigDir)
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()
	appStore := store.New(conn)
	candidates, err := appStore.ListGameLaunchAuditCandidates()
	if err != nil {
		log.Fatal(err)
	}

	bySet := make(map[string][]domain.GameAsset)
	for _, candidate := range candidates {
		setName := canonicalSetName(candidate.FilePath)
		if setName != "" {
			bySet[setName] = append(bySet[setName], candidate)
		}
	}
	profiles := make([]domain.GameLaunchProfile, 0, len(candidates))
	updates := make([]domain.GameLaunchCatalogUpdate, 0, len(candidates))
	failures := make([]string, 0)
	matched := 0
	for _, candidate := range candidates {
		if !eligibleFBNeoCandidate(candidate) || isKnownDependency(candidate) {
			continue
		}
		setName := canonicalSetName(candidate.FilePath)
		datGame, ok := catalog.Games[setName]
		if !ok {
			continue
		}
		matched++
		update := domain.GameLaunchCatalogUpdate{
			GameID: candidate.ID, Platform: datGame.Platform(), ROMSetName: setName,
			EmulatorHint: "fbneo", CatalogRole: launchcatalog.RoleNeedsCuration,
		}
		profile, auditErr := buildFBNeoProfile(catalog, datGame, candidate, bySet)
		if auditErr == nil {
			profiles = append(profiles, profile)
			update.CatalogRole = launchcatalog.RoleGame
		} else {
			failures = append(failures, fmt.Sprintf("game=%d set=%s: %v", candidate.ID, setName, auditErr))
		}
		updates = append(updates, update)
	}
	sort.Slice(profiles, func(i, j int) bool { return profiles[i].GameID < profiles[j].GameID })
	sort.Slice(updates, func(i, j int) bool { return updates[i].GameID < updates[j].GameID })

	result := domain.GameLaunchProfileRebuildResult{ProfilesWritten: len(profiles)}
	for _, profile := range profiles {
		result.FilesWritten += len(profile.Files)
		result.GamesReady++
	}
	result.GamesRejected = len(updates) - result.GamesReady
	if !*dryRun {
		result, err = appStore.ReplaceGameLaunchProfiles(launchprofile.FBNeoPolicy, profiles, updates)
		if err != nil {
			log.Fatal(err)
		}
	}
	if *failureLimit >= 0 && len(failures) > *failureLimit {
		failures = failures[:*failureLimit]
	}
	output := rebuildOutput{
		Policy: launchprofile.FBNeoPolicy, DATName: catalog.Name, DATVersion: catalog.Version,
		DATSHA256: catalog.SHA256, ProfileRevision: catalog.Revision,
		Candidates: len(candidates), Matched: matched, Result: result, Failures: failures, DryRun: *dryRun,
	}
	encoded, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(string(encoded))
}

func buildFBNeoProfile(catalog launchprofile.FBNeoCatalog, datGame launchprofile.FBNeoGame, entry domain.GameAsset, bySet map[string][]domain.GameAsset) (domain.GameLaunchProfile, error) {
	if !validContainerIdentity(entry) {
		return domain.GameLaunchProfile{}, fmt.Errorf("entry container has no stable SHA-1 identity")
	}
	if err := launchprofile.ValidateFBNeoArchive(entry.FilePath, datGame); err != nil {
		return domain.GameLaunchProfile{}, err
	}
	files := []domain.GameLaunchProfileFile{{
		Position: 0, SourceGameID: entry.ID, SourceSHA1: strings.ToLower(entry.SHA1),
		SourceName: filepath.Base(entry.FilePath), Name: datGame.Name + ".zip", Size: entry.Size, Role: "entry",
	}}
	dependencies, err := catalog.Dependencies(datGame.Name)
	if err != nil {
		return domain.GameLaunchProfile{}, err
	}
	for _, dependency := range dependencies {
		source, err := selectDependencySource(entry, dependency, bySet[dependency.Name])
		if err != nil {
			return domain.GameLaunchProfile{}, err
		}
		files = append(files, domain.GameLaunchProfileFile{
			Position: len(files), SourceGameID: source.ID, SourceSHA1: strings.ToLower(source.SHA1),
			SourceName: filepath.Base(source.FilePath), Name: dependency.Name + ".zip", Size: source.Size, Role: "dependency",
		})
	}
	version := profileVersion(catalog.Version)
	return domain.GameLaunchProfile{
		GameID: entry.ID, ID: fmt.Sprintf("%s-windows-fbneo-%s-%s", datGame.Name, version, catalog.SHA256[:8]),
		Revision: catalog.Revision, Priority: 200, Policy: launchprofile.FBNeoPolicy,
		ClientName: "SpatialEMU.Windows", MinClientVersion: "1.302", ClientPlatform: "windows-x64", Architecture: "x64",
		Runtime:   domain.GameRuntimeDescriptor{ID: "libretro", CoreID: "fbneo", CoreSHA256: fbNeoCoreSHA256},
		EntryFile: datGame.Name + ".zip", CanonicalSet: datGame.Name, Status: "ready", Files: files,
	}, nil
}

func selectDependencySource(entry domain.GameAsset, dependency launchprofile.FBNeoGame, candidates []domain.GameAsset) (domain.GameAsset, error) {
	sort.SliceStable(candidates, func(i, j int) bool {
		leftSameDir := filepath.Dir(candidates[i].FilePath) == filepath.Dir(entry.FilePath)
		rightSameDir := filepath.Dir(candidates[j].FilePath) == filepath.Dir(entry.FilePath)
		return leftSameDir && !rightSameDir
	})
	for _, candidate := range candidates {
		if !validContainerIdentity(candidate) {
			continue
		}
		if err := launchprofile.ValidateFBNeoArchive(candidate.FilePath, dependency); err == nil {
			return candidate, nil
		}
	}
	return domain.GameAsset{}, fmt.Errorf("verified dependency %s.zip is unavailable", dependency.Name)
}

func canonicalSetName(path string) string {
	return strings.ToLower(strings.TrimSpace(strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))))
}

func eligibleFBNeoCandidate(game domain.GameAsset) bool {
	if launchcatalog.IsStrictArcadePlatform(game.Platform) {
		return true
	}
	path := strings.ToLower(filepath.ToSlash(game.FilePath))
	return strings.Contains(path, "/fbneo/")
}

func isKnownDependency(game domain.GameAsset) bool {
	return strings.EqualFold(strings.TrimSpace(game.CatalogRole), launchcatalog.RoleDependency) ||
		canonicalSetName(game.FilePath) == "neogeo" || canonicalSetName(game.FilePath) == "ym2413_instruments"
}

func validContainerIdentity(game domain.GameAsset) bool {
	sha1 := strings.ToLower(strings.TrimSpace(game.SHA1))
	if len(sha1) != 40 || game.Size <= 0 {
		return false
	}
	for _, char := range sha1 {
		if !(char >= '0' && char <= '9' || char >= 'a' && char <= 'f') {
			return false
		}
	}
	return true
}

func profileVersion(version string) string {
	var builder strings.Builder
	for _, char := range strings.ToLower(strings.TrimSpace(version)) {
		if char >= 'a' && char <= 'z' || char >= '0' && char <= '9' {
			builder.WriteRune(char)
		}
	}
	if builder.Len() == 0 {
		return "dat"
	}
	return builder.String()
}
