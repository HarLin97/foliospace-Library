package service

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"foliospace-reader/internal/domain"
	"foliospace-reader/internal/launchcatalog"
)

type RuntimeProfileNotAvailableError struct {
	GameID         int64
	RuntimeID      string
	RuntimeVersion string
}

func (e *RuntimeProfileNotAvailableError) Error() string {
	runtimeName := strings.ToUpper(strings.TrimSpace(e.RuntimeID))
	if runtimeName == "" {
		runtimeName = "compatible runtime"
	}
	if strings.TrimSpace(e.RuntimeVersion) != "" {
		runtimeName += " " + strings.TrimSpace(e.RuntimeVersion)
	}
	return fmt.Sprintf("No %s profile is available for game %d.", runtimeName, e.GameID)
}

type auditedGameLaunchProfile struct {
	ID               string
	Revision         int
	ClientName       string
	MinClientVersion string
	ClientPlatform   string
	Architecture     string
	Runtime          domain.GameRuntimeDescriptor
	EntrySHA1        string
	EntrySourceName  string
	Title            string
	ROMSetName       string
	Files            []auditedGameLaunchFile
}

type auditedGameLaunchFile struct {
	SourceSHA1 string
	SourceName string
	Name       string
	Size       int64
	Role       string
}

var sha1Pattern = regexp.MustCompile(`^[0-9a-f]{40}$`)
var sha256Pattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

var auditedGameLaunchProfiles = []auditedGameLaunchProfile{
	{
		ID:               "vstriker-windows-mame0288-v1",
		Revision:         1,
		ClientName:       "SpatialEMU.Windows",
		MinClientVersion: "1.302",
		ClientPlatform:   "windows-x64",
		Architecture:     "x64",
		Runtime:          domain.GameRuntimeDescriptor{ID: "mame", Version: "0.288", ContentSet: "mame-0.288"},
		EntrySHA1:        "8e3518318eeb157ab299b2f284faef176d3f49dd",
		EntrySourceName:  "vstriker.zip",
		Title:            "Virtua Striker",
		ROMSetName:       "vstriker",
		Files: []auditedGameLaunchFile{
			{SourceSHA1: "8e3518318eeb157ab299b2f284faef176d3f49dd", SourceName: "vstriker.zip", Name: "vstriker.zip", Size: 10313686, Role: "entry"},
			{SourceSHA1: "4631db7f7f5160a3a6591d3102722be869710f66", SourceName: "segabill.zip", Name: "segabill.zip", Size: 3117, Role: "dependency"},
		},
	},
	{
		ID:               "tektagtc1a-windows-mame0288-v1",
		Revision:         1,
		ClientName:       "SpatialEMU.Windows",
		MinClientVersion: "1.302",
		ClientPlatform:   "windows-x64",
		Architecture:     "x64",
		Runtime:          domain.GameRuntimeDescriptor{ID: "mame", Version: "0.288", ContentSet: "mame-0.288"},
		EntrySHA1:        "d6615a3a70ea9941b61ccd608054a0044d3d6ab3",
		EntrySourceName:  "tektagtac1.zip",
		Title:            "Tekken Tag Tournament (World, TEG2/VER.C1, set 2)",
		ROMSetName:       "tektagtc1a",
		Files: []auditedGameLaunchFile{
			{SourceSHA1: "d6615a3a70ea9941b61ccd608054a0044d3d6ab3", SourceName: "tektagtac1.zip", Name: "tektagtc1a.zip", Size: 120980600, Role: "entry"},
		},
	},
	{
		ID:               "sf2-windows-fbneo-v1",
		Revision:         1,
		ClientName:       "SpatialEMU.Windows",
		MinClientVersion: "1.302",
		ClientPlatform:   "windows-x64",
		Architecture:     "x64",
		Runtime:          domain.GameRuntimeDescriptor{ID: "libretro", CoreID: "fbneo", CoreSHA256: "6ebc2675c272c8d654935647ac336d45bbd97452c4d5943290d5ffc75678d9f1"},
		EntrySHA1:        "bd59872a57f14dc492e2fb387727a9402f3d4f97",
		EntrySourceName:  "sf2.zip",
		ROMSetName:       "sf2",
		Files: []auditedGameLaunchFile{
			{SourceSHA1: "bd59872a57f14dc492e2fb387727a9402f3d4f97", SourceName: "sf2.zip", Name: "sf2.zip", Size: 3551819, Role: "entry"},
		},
	},
	{
		ID:               "sfa-windows-fbneo-v1",
		Revision:         1,
		ClientName:       "SpatialEMU.Windows",
		MinClientVersion: "1.302",
		ClientPlatform:   "windows-x64",
		Architecture:     "x64",
		Runtime:          domain.GameRuntimeDescriptor{ID: "libretro", CoreID: "fbneo", CoreSHA256: "6ebc2675c272c8d654935647ac336d45bbd97452c4d5943290d5ffc75678d9f1"},
		EntrySHA1:        "61dece364b8d2f2ff15391505168be334ebb371a",
		EntrySourceName:  "sfa.zip",
		ROMSetName:       "sfa",
		Files: []auditedGameLaunchFile{
			{SourceSHA1: "61dece364b8d2f2ff15391505168be334ebb371a", SourceName: "sfa.zip", Name: "sfa.zip", Size: 7365582, Role: "entry"},
		},
	},
	{
		ID:               "sfiii-windows-fbneo-v1",
		Revision:         1,
		ClientName:       "SpatialEMU.Windows",
		MinClientVersion: "1.302",
		ClientPlatform:   "windows-x64",
		Architecture:     "x64",
		Runtime:          domain.GameRuntimeDescriptor{ID: "libretro", CoreID: "fbneo", CoreSHA256: "6ebc2675c272c8d654935647ac336d45bbd97452c4d5943290d5ffc75678d9f1"},
		EntrySHA1:        "7aae0cfc4ef8911f19d2e986cee63807deebf1b6",
		EntrySourceName:  "sfiii.zip",
		ROMSetName:       "sfiii",
		Files: []auditedGameLaunchFile{
			{SourceSHA1: "7aae0cfc4ef8911f19d2e986cee63807deebf1b6", SourceName: "sfiii.zip", Name: "sfiii.zip", Size: 38868517, Role: "entry"},
		},
	},
	{
		ID:               "hypreact-windows-mame0288-v1",
		Revision:         1,
		ClientName:       "SpatialEMU.Windows",
		MinClientVersion: "1.302",
		ClientPlatform:   "windows-x64",
		Architecture:     "x64",
		Runtime:          domain.GameRuntimeDescriptor{ID: "mame", Version: "0.288", ContentSet: "mame-0.288"},
		EntrySHA1:        "e0940f848884c9d53bbc41bb947d584e06cc1845",
		EntrySourceName:  "hypreact.zip",
		ROMSetName:       "hypreact",
		Files: []auditedGameLaunchFile{
			{SourceSHA1: "e0940f848884c9d53bbc41bb947d584e06cc1845", SourceName: "hypreact.zip", Name: "hypreact.zip", Size: 8052342, Role: "entry"},
		},
	},
	{
		ID:               "hypreac2-windows-mame0288-v1",
		Revision:         1,
		ClientName:       "SpatialEMU.Windows",
		MinClientVersion: "1.302",
		ClientPlatform:   "windows-x64",
		Architecture:     "x64",
		Runtime:          domain.GameRuntimeDescriptor{ID: "mame", Version: "0.288", ContentSet: "mame-0.288"},
		EntrySHA1:        "7fe73cc7ee40a49225a4616106e538c084ef4364",
		EntrySourceName:  "hypreac2.zip",
		ROMSetName:       "hypreac2",
		Files: []auditedGameLaunchFile{
			{SourceSHA1: "7fe73cc7ee40a49225a4616106e538c084ef4364", SourceName: "hypreac2.zip", Name: "hypreac2.zip", Size: 18291541, Role: "entry"},
		},
	},
	{
		ID:               "srmp4-windows-mame0288-v1",
		Revision:         1,
		ClientName:       "SpatialEMU.Windows",
		MinClientVersion: "1.302",
		ClientPlatform:   "windows-x64",
		Architecture:     "x64",
		Runtime:          domain.GameRuntimeDescriptor{ID: "mame", Version: "0.288", ContentSet: "mame-0.288"},
		EntrySHA1:        "cfcf2cdf61ebca862a84473a8bf75fbe8d76cb7b",
		EntrySourceName:  "srmp4.zip",
		ROMSetName:       "srmp4",
		Files: []auditedGameLaunchFile{
			{SourceSHA1: "cfcf2cdf61ebca862a84473a8bf75fbe8d76cb7b", SourceName: "srmp4.zip", Name: "srmp4.zip", Size: 7697767, Role: "entry"},
		},
	},
	{
		ID:               "fromancr-windows-mame0288-v1",
		Revision:         1,
		ClientName:       "SpatialEMU.Windows",
		MinClientVersion: "1.302",
		ClientPlatform:   "windows-x64",
		Architecture:     "x64",
		Runtime:          domain.GameRuntimeDescriptor{ID: "mame", Version: "0.288", ContentSet: "mame-0.288"},
		EntrySHA1:        "137e4949d7e204ff10e33372528cc1e9481b962c",
		EntrySourceName:  "fromancr.zip",
		ROMSetName:       "fromancr",
		Files: []auditedGameLaunchFile{
			{SourceSHA1: "137e4949d7e204ff10e33372528cc1e9481b962c", SourceName: "fromancr.zip", Name: "fromancr.zip", Size: 14121810, Role: "entry"},
		},
	},
	{
		ID:               "fromanc4-windows-mame0288-v1",
		Revision:         1,
		ClientName:       "SpatialEMU.Windows",
		MinClientVersion: "1.302",
		ClientPlatform:   "windows-x64",
		Architecture:     "x64",
		Runtime:          domain.GameRuntimeDescriptor{ID: "mame", Version: "0.288", ContentSet: "mame-0.288"},
		EntrySHA1:        "ff478f3350d9703e8647f659ce169ee234082249",
		EntrySourceName:  "fromanc4.zip",
		ROMSetName:       "fromanc4",
		Files: []auditedGameLaunchFile{
			{SourceSHA1: "ff478f3350d9703e8647f659ce169ee234082249", SourceName: "fromanc4.zip", Name: "fromanc4.zip", Size: 21443327, Role: "entry"},
		},
	},
	{
		ID:               "mcnpshnt-windows-mame0288-v1",
		Revision:         1,
		ClientName:       "SpatialEMU.Windows",
		MinClientVersion: "1.302",
		ClientPlatform:   "windows-x64",
		Architecture:     "x64",
		Runtime:          domain.GameRuntimeDescriptor{ID: "mame", Version: "0.288", ContentSet: "mame-0.288"},
		EntrySHA1:        "24a714371a867db1709798a95a171778e0940021",
		EntrySourceName:  "mcnpshnt.zip",
		ROMSetName:       "mcnpshnt",
		Files: []auditedGameLaunchFile{
			{SourceSHA1: "24a714371a867db1709798a95a171778e0940021", SourceName: "mcnpshnt.zip", Name: "mcnpshnt.zip", Size: 1205007, Role: "entry"},
			{SourceSHA1: "cbcd6e0698026452bb2bb6a6e6f7f5a3667a675c", SourceName: "ym2413_instruments.zip", Name: "ym2413.zip", Size: 322, Role: "dependency"},
		},
	},
}

func ValidateGameLaunchResolveRequest(req domain.GameLaunchResolveRequest) error {
	for name, value := range map[string]string{
		"client.name": req.Client.Name, "client.version": req.Client.Version,
		"client.platform": req.Client.Platform, "client.architecture": req.Client.Architecture,
	} {
		value = strings.TrimSpace(value)
		if value == "" || len(value) > 128 {
			return fmt.Errorf("%s must contain 1 to 128 characters", name)
		}
	}
	if len(req.Runtimes) == 0 || len(req.Runtimes) > 16 {
		return errors.New("runtimes must contain 1 to 16 descriptors")
	}
	for index, runtime := range req.Runtimes {
		if strings.TrimSpace(runtime.ID) == "" || len(strings.TrimSpace(runtime.ID)) > 128 {
			return fmt.Errorf("runtimes[%d].id must contain 1 to 128 characters", index)
		}
		for name, value := range map[string]string{
			"version": runtime.Version, "contentSet": runtime.ContentSet,
			"coreId": runtime.CoreID, "coreSha256": runtime.CoreSHA256,
		} {
			if len(strings.TrimSpace(value)) > 128 {
				return fmt.Errorf("runtimes[%d].%s must not exceed 128 characters", index, name)
			}
		}
		if runtime.CoreSHA256 != "" && !sha256Pattern.MatchString(runtime.CoreSHA256) {
			return fmt.Errorf("runtimes[%d].coreSha256 must be 64 lowercase hexadecimal characters", index)
		}
	}
	return nil
}

func (s *Service) ResolveGameLaunchProfile(gameID int64, req domain.GameLaunchResolveRequest) (domain.GameLaunchResolution, error) {
	if err := ValidateGameLaunchResolveRequest(req); err != nil {
		return domain.GameLaunchResolution{}, err
	}
	game, err := s.store.GameByID(gameID)
	if err != nil {
		return domain.GameLaunchResolution{}, err
	}
	if err := validateAuditedGameLaunchProfiles(); err != nil {
		return domain.GameLaunchResolution{}, err
	}

	var requestedRuntime domain.GameRuntimeDescriptor
	if len(req.Runtimes) > 0 {
		requestedRuntime = req.Runtimes[0]
	}
	for _, profile := range auditedGameLaunchProfiles {
		runtime, ok := matchingRuntime(profile, req)
		if !ok || !strings.EqualFold(strings.TrimSpace(game.SHA1), profile.EntrySHA1) || game.Size != profile.Files[0].Size || !strings.EqualFold(filepath.Base(game.FilePath), profile.EntrySourceName) {
			continue
		}
		resolvedFiles := make([]domain.GameLaunchResolvedFile, 0, len(profile.Files))
		var totalSize int64
		for _, file := range profile.Files {
			source, sourceErr := s.resolveAuditedLaunchSource(file)
			if sourceErr != nil {
				return domain.GameLaunchResolution{}, sourceErr
			}
			resolvedFiles = append(resolvedFiles, domain.GameLaunchResolvedFile{
				SourceGameID: source.ID, Name: file.Name, Size: file.Size, Role: file.Role, SHA1: file.SourceSHA1,
			})
			totalSize += file.Size
		}
		resolvedGame := game
		if strings.TrimSpace(profile.Title) != "" {
			resolvedGame.Title = profile.Title
		}
		resolvedGame.ROMSetName = profile.ROMSetName
		resolvedGame.Size = totalSize
		return domain.GameLaunchResolution{
			LaunchProfileID: profile.ID, ProfileRevision: profile.Revision, Runtime: runtime,
			Game: resolvedGame, EntryFile: profile.Files[0].Name, Files: resolvedFiles,
		}, nil
	}
	if runtime, ok := matchingPragmaticRuntime(game, req); ok {
		return s.resolvePragmaticGameLaunch(game, runtime, req.Client)
	}
	for _, runtime := range req.Runtimes {
		if strings.EqualFold(runtime.ID, "mame") {
			requestedRuntime = runtime
			break
		}
	}
	return domain.GameLaunchResolution{}, &RuntimeProfileNotAvailableError{
		GameID: gameID, RuntimeID: requestedRuntime.ID, RuntimeVersion: requestedRuntime.Version,
	}
}

func (s *Service) resolvePragmaticGameLaunch(game domain.GameAsset, runtime domain.GameRuntimeDescriptor, client domain.GameLaunchClient) (domain.GameLaunchResolution, error) {
	files, err := s.GameFiles(game.ID)
	if err != nil {
		return domain.GameLaunchResolution{}, err
	}
	entryFile, err := validatePragmaticManifest(game, files)
	if err != nil {
		return domain.GameLaunchResolution{}, &RuntimeProfileNotAvailableError{
			GameID: game.ID, RuntimeID: runtime.ID, RuntimeVersion: runtime.Version,
		}
	}

	var dosLaunch *domain.DOSLaunch
	if strings.EqualFold(game.Platform, "dos") {
		launch, launchErr := s.store.DOSLaunch(game.ID)
		if launchErr != nil || validatePragmaticDOSLaunch(launch) != nil {
			return domain.GameLaunchResolution{}, &RuntimeProfileNotAvailableError{
				GameID: game.ID, RuntimeID: runtime.ID, RuntimeVersion: runtime.Version,
			}
		}
		entryFile = launch.EntryFile
		dosLaunch = &launch
	}

	resolvedFiles := make([]domain.GameLaunchResolvedFile, 0, len(files))
	for _, file := range files {
		position := file.Position
		sha1 := ""
		if file.Position == 0 && sha1Pattern.MatchString(strings.ToLower(strings.TrimSpace(game.SHA1))) {
			sha1 = strings.ToLower(strings.TrimSpace(game.SHA1))
		}
		resolvedFiles = append(resolvedFiles, domain.GameLaunchResolvedFile{
			SourceGameID: game.ID, Position: &position, Name: file.Name,
			Size: file.Size, Role: file.Role, SHA1: sha1,
		})
	}

	return domain.GameLaunchResolution{
		LaunchProfileID: pragmaticLaunchProfileID(game, runtime, client),
		ProfileRevision: pragmaticProfileRevision(game),
		Runtime:         runtime, Game: game, EntryFile: entryFile, Files: resolvedFiles, DOSLaunch: dosLaunch,
	}, nil
}

func matchingPragmaticRuntime(game domain.GameAsset, req domain.GameLaunchResolveRequest) (domain.GameRuntimeDescriptor, bool) {
	if !supportedPragmaticClient(req.Client) || isStrictArcadePlatform(game.Platform) || !isLaunchableCatalogGame(game) {
		return domain.GameRuntimeDescriptor{}, false
	}
	platform := normalizeLaunchPlatform(game.Platform)
	for _, runtime := range req.Runtimes {
		if pragmaticRuntimeSupportsPlatform(runtime, platform) {
			// Windows 1.302 compares every field with the selected request tuple.
			return runtime, true
		}
	}
	return domain.GameRuntimeDescriptor{}, false
}

func supportedPragmaticClient(client domain.GameLaunchClient) bool {
	name := strings.ToLower(strings.TrimSpace(client.Name))
	platform := strings.ToLower(strings.TrimSpace(client.Platform))
	architecture := strings.ToLower(strings.TrimSpace(client.Architecture))
	switch name {
	case "spatialemu.windows":
		return platform == "windows-x64" && architecture == "x64" && versionAtLeast(client.Version, "1.302")
	case "spatialemu.macos":
		return (platform == "macos-arm64" && architecture == "arm64") ||
			(platform == "macos-x64" && architecture == "x64")
	default:
		return false
	}
}

func pragmaticRuntimeSupportsPlatform(runtime domain.GameRuntimeDescriptor, platform string) bool {
	runtimeID := strings.ToLower(strings.TrimSpace(runtime.ID))
	version := canonicalPragmaticVersion(runtime.Version)
	switch platform {
	case "dreamcast", "naomi", "naomi2":
		return runtimeID == "flycast" && version != "" && versionAtLeast(version, "2.6")
	case "model3":
		return runtimeID == "supermodel" && optionalNumericVersion(version)
	case "ngc":
		return runtimeID == "dolphin" && optionalNumericVersion(version)
	case "ps2":
		return runtimeID == "pcsx2" && versionAtLeast(version, "2.6.3")
	case "psp":
		return runtimeID == "ppsspp" && versionAtLeast(version, "1.20.4")
	case "dos":
		return runtimeID == "dosbox-staging" && versionAtLeast(version, "0.82.2")
	default:
		return runtimeID == "libretro" && ordinaryLibretroCoreSupportsPlatform(runtime.CoreID, platform)
	}
}

func ordinaryLibretroCoreSupportsPlatform(coreID string, platform string) bool {
	core := normalizeLaunchCoreID(coreID)
	if core == "" || core == "fbneo" {
		return false
	}
	allowed := map[string]map[string]bool{
		"nes": {
			"nestopia": true, "nestopiaue": true, "mesen": true, "fceumm": true,
		},
		"snes": {
			"snes9x": true, "snes9xcurrent": true, "mesens": true,
		},
		"md": {
			"genesisplusgx": true, "picodrive": true,
		},
		"32x": {
			"picodrive": true,
		},
		"gb": {
			"gambatte": true, "sameboy": true, "mesens": true,
		},
		"gbc": {
			"gambatte": true, "sameboy": true, "mesens": true,
		},
		"gba": {
			"mgba": true, "vbanext": true,
		},
		"ps1": {
			"swanstation": true, "beetlepsx": true, "beetlepsxhw": true, "pcsxrearmed": true,
		},
		"n64": {
			"mupen64plusnext": true, "mupen64plus": true, "paralleln64": true,
		},
		"saturn": {
			"beetlesaturn": true, "mednafensaturn": true, "yabasanshiro": true, "yabasanshiro2": true,
		},
		"pc-fx": {
			"beetlepcfx": true, "mednafenpcfx": true,
		},
		"pc98": {
			"np2kai": true, "nekoprojectiikai": true,
		},
	}
	return allowed[platform][core]
}

func normalizeLaunchCoreID(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.TrimSuffix(value, "_libretro")
	value = strings.TrimSuffix(value, "-libretro")
	var builder strings.Builder
	for _, char := range value {
		if char >= 'a' && char <= 'z' || char >= '0' && char <= '9' {
			builder.WriteRune(char)
		}
	}
	return builder.String()
}

func normalizeLaunchPlatform(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "sfc":
		return "snes"
	case "megadrive", "mega-drive", "genesis":
		return "md"
	case "psx":
		return "ps1"
	case "gamecube", "game-cube":
		return "ngc"
	case "dc":
		return "dreamcast"
	case "pcfx":
		return "pc-fx"
	case "pc-98":
		return "pc98"
	default:
		return strings.ToLower(strings.TrimSpace(value))
	}
}

func isStrictArcadePlatform(value string) bool {
	switch normalizeLaunchPlatform(value) {
	case "mame", "arcade", "model2", "cps", "cps1", "cps2", "cps3", "neogeo":
		return true
	default:
		return false
	}
}

func isLaunchableCatalogGame(game domain.GameAsset) bool {
	role := strings.ToLower(strings.TrimSpace(game.CatalogRole))
	return role == "" || role == "game"
}

func optionalNumericVersion(value string) bool {
	if value == "" {
		return true
	}
	_, ok := numericVersion(value)
	return ok
}

func canonicalPragmaticVersion(value string) string {
	value = strings.TrimSpace(value)
	if strings.HasPrefix(value, "v") || strings.HasPrefix(value, "V") {
		value = value[1:]
	}
	value = strings.ReplaceAll(value, ",", ".")
	value = strings.ReplaceAll(value, " ", "")
	parts, ok := numericVersion(value)
	if !ok {
		return ""
	}
	for len(parts) > 3 && parts[len(parts)-1] == 0 {
		parts = parts[:len(parts)-1]
	}
	text := make([]string, 0, len(parts))
	for _, part := range parts {
		text = append(text, strconv.Itoa(part))
	}
	return strings.Join(text, ".")
}

func validatePragmaticManifest(game domain.GameAsset, files []domain.GameFile) (string, error) {
	if len(files) == 0 {
		return "", errors.New("canonical manifest contains no files")
	}
	entryFile := ""
	entries := 0
	names := make(map[string]struct{}, len(files))
	positions := make(map[int]struct{}, len(files))
	for _, file := range files {
		if !validLaunchRelativePath(file.Name) || file.Size <= 0 || strings.TrimSpace(file.FilePath) == "" || file.Position < 0 {
			return "", fmt.Errorf("canonical manifest contains invalid file %q", file.Name)
		}
		nameKey := strings.ToLower(file.Name)
		if _, exists := names[nameKey]; exists {
			return "", fmt.Errorf("canonical manifest contains duplicate logical name %q", file.Name)
		}
		names[nameKey] = struct{}{}
		if _, exists := positions[file.Position]; exists {
			return "", fmt.Errorf("canonical manifest contains duplicate position %d", file.Position)
		}
		positions[file.Position] = struct{}{}
		switch strings.ToLower(strings.TrimSpace(file.Role)) {
		case "entry":
			entries++
			entryFile = file.Name
		case "dependency", "disk", "font":
		default:
			return "", fmt.Errorf("canonical manifest contains unsupported role %q", file.Role)
		}
	}
	if entries != 1 {
		return "", fmt.Errorf("canonical manifest requires exactly one entry, got %d", entries)
	}
	if game.Size <= 0 {
		return "", errors.New("canonical manifest has invalid aggregate size")
	}
	return entryFile, nil
}

func validatePragmaticDOSLaunch(launch domain.DOSLaunch) error {
	source := strings.ToLower(strings.TrimSpace(launch.EntrySource))
	if source != "curated" && source != "dosboxconfig" {
		return fmt.Errorf("DOS launch entry source %q is not deterministic", launch.EntrySource)
	}
	if !validDOSLaunchPath(launch.EntryFile) || !isDOSExecutablePath(launch.EntryFile) {
		return errors.New("DOS launch entry is invalid")
	}
	for _, path := range []string{launch.InstallDirectory, launch.WorkingDirectory, launch.DOSBoxConfig} {
		if strings.TrimSpace(path) != "" && !validDOSLaunchPath(path) {
			return fmt.Errorf("DOS launch path %q is invalid", path)
		}
	}
	for _, candidate := range launch.Candidates {
		kind := strings.ToLower(strings.TrimSpace(candidate.Kind))
		if !validDOSLaunchPath(candidate.Path) || (kind != "bat" && kind != "com" && kind != "exe") {
			return fmt.Errorf("DOS launch candidate %q is invalid", candidate.Path)
		}
	}
	for _, argument := range launch.Arguments {
		if strings.ContainsAny(argument, "\x00\r\n") || len(argument) > 512 {
			return errors.New("DOS launch argument is invalid")
		}
	}
	return nil
}

func validDOSLaunchPath(path string) bool {
	return validLaunchRelativePath(path) && !strings.ContainsAny(path, `|&;<>()`)
}

func isDOSExecutablePath(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".bat", ".com", ".exe":
		return true
	default:
		return false
	}
}

func validLaunchRelativePath(name string) bool {
	if strings.Contains(name, `\`) {
		return false
	}
	name = strings.TrimSpace(name)
	if name == "" || strings.HasPrefix(name, "/") || filepath.IsAbs(name) || filepath.VolumeName(name) != "" || strings.ContainsRune(name, '\x00') {
		return false
	}
	if len(name) >= 2 && name[1] == ':' {
		return false
	}
	for _, part := range strings.Split(name, "/") {
		if part == "" || part == "." || part == ".." {
			return false
		}
	}
	return true
}

func pragmaticLaunchProfileID(game domain.GameAsset, runtime domain.GameRuntimeDescriptor, client domain.GameLaunchClient) string {
	key := fmt.Sprintf("%d\x00%s\x00%s\x00%s\x00%s\x00%s", game.ID, strings.ToLower(strings.TrimSpace(runtime.ID)),
		strings.ToLower(strings.TrimSpace(runtime.CoreID)), normalizeLaunchPlatform(game.Platform),
		strings.ToLower(strings.TrimSpace(client.Platform)), strings.ToLower(strings.TrimSpace(client.Architecture)))
	digest := sha256.Sum256([]byte(key))
	return fmt.Sprintf("auto-%x", digest[:10])
}

func pragmaticProfileRevision(game domain.GameAsset) int {
	revision := game.UpdatedAt.Unix()
	if revision <= 0 {
		return 1
	}
	return int(revision)
}

func (s *Service) resolveAuditedLaunchSource(file auditedGameLaunchFile) (domain.GameAsset, error) {
	candidates, err := s.store.GamesBySHA1(file.SourceSHA1)
	if err != nil {
		return domain.GameAsset{}, err
	}
	for _, candidate := range candidates {
		if candidate.Size == file.Size && strings.EqualFold(filepath.Base(candidate.FilePath), file.SourceName) {
			return candidate, nil
		}
	}
	return domain.GameAsset{}, fmt.Errorf("audited launch source %q is unavailable", file.Name)
}

func matchingRuntime(profile auditedGameLaunchProfile, req domain.GameLaunchResolveRequest) (domain.GameRuntimeDescriptor, bool) {
	if !strings.EqualFold(strings.TrimSpace(req.Client.Name), profile.ClientName) ||
		!strings.EqualFold(strings.TrimSpace(req.Client.Platform), profile.ClientPlatform) ||
		!strings.EqualFold(strings.TrimSpace(req.Client.Architecture), profile.Architecture) ||
		!versionAtLeast(req.Client.Version, profile.MinClientVersion) {
		return domain.GameRuntimeDescriptor{}, false
	}
	for _, runtime := range req.Runtimes {
		if strings.EqualFold(strings.TrimSpace(runtime.ID), profile.Runtime.ID) &&
			strings.TrimSpace(runtime.Version) == profile.Runtime.Version &&
			strings.TrimSpace(runtime.ContentSet) == profile.Runtime.ContentSet &&
			strings.TrimSpace(runtime.CoreID) == profile.Runtime.CoreID &&
			strings.TrimSpace(runtime.CoreSHA256) == profile.Runtime.CoreSHA256 {
			// Windows 1.302 verifies the response tuple against the selected
			// request tuple field by field, so preserve the request verbatim.
			return runtime, true
		}
	}
	return domain.GameRuntimeDescriptor{}, false
}

func versionAtLeast(actual string, minimum string) bool {
	actualParts, ok := numericVersion(actual)
	if !ok {
		return false
	}
	minimumParts, ok := numericVersion(minimum)
	if !ok {
		return false
	}
	count := len(actualParts)
	if len(minimumParts) > count {
		count = len(minimumParts)
	}
	for index := 0; index < count; index++ {
		var actualPart, minimumPart int
		if index < len(actualParts) {
			actualPart = actualParts[index]
		}
		if index < len(minimumParts) {
			minimumPart = minimumParts[index]
		}
		if actualPart != minimumPart {
			return actualPart > minimumPart
		}
	}
	return true
}

func numericVersion(value string) ([]int, bool) {
	parts := strings.Split(strings.TrimSpace(value), ".")
	if len(parts) == 0 {
		return nil, false
	}
	out := make([]int, 0, len(parts))
	for _, part := range parts {
		if part == "" {
			return nil, false
		}
		parsed, err := strconv.Atoi(part)
		if err != nil || parsed < 0 {
			return nil, false
		}
		out = append(out, parsed)
	}
	return out, true
}

func validateAuditedGameLaunchProfiles() error {
	profileIDs := map[string]struct{}{}
	for _, profile := range auditedGameLaunchProfiles {
		if strings.TrimSpace(profile.ID) == "" || profile.Revision <= 0 {
			return errors.New("audited launch profile requires an id and positive revision")
		}
		if _, exists := profileIDs[profile.ID]; exists {
			return fmt.Errorf("duplicate audited launch profile id %q", profile.ID)
		}
		profileIDs[profile.ID] = struct{}{}
		if !sha1Pattern.MatchString(profile.EntrySHA1) || len(profile.Files) == 0 {
			return fmt.Errorf("audited launch profile %q has an invalid entry", profile.ID)
		}
		if profile.Files[0].SourceSHA1 != profile.EntrySHA1 || !strings.EqualFold(profile.Files[0].SourceName, profile.EntrySourceName) {
			return fmt.Errorf("audited launch profile %q entry identity does not match its first file", profile.ID)
		}
		if !launchcatalog.IsAuditedEntryIdentity(profile.EntrySourceName, profile.Files[0].Size, profile.EntrySHA1) {
			return fmt.Errorf("audited launch profile %q is missing from the shared launch catalog", profile.ID)
		}
		entryCount := 0
		names := map[string]struct{}{}
		for _, file := range profile.Files {
			if !validLogicalLaunchName(file.Name) || !sha1Pattern.MatchString(file.SourceSHA1) || file.Size <= 0 {
				return fmt.Errorf("audited launch profile %q has an invalid file", profile.ID)
			}
			nameKey := strings.ToLower(file.Name)
			if _, exists := names[nameKey]; exists {
				return fmt.Errorf("audited launch profile %q has a case-insensitive logical name collision", profile.ID)
			}
			names[nameKey] = struct{}{}
			if file.Role == "entry" {
				entryCount++
			} else if file.Role != "dependency" {
				return fmt.Errorf("audited launch profile %q has unsupported role %q", profile.ID, file.Role)
			}
		}
		if entryCount != 1 || profile.Files[0].Role != "entry" {
			return fmt.Errorf("audited launch profile %q must have exactly one leading entry", profile.ID)
		}
	}
	return nil
}

func validLogicalLaunchName(name string) bool {
	trimmed := strings.TrimSpace(name)
	if name != trimmed || name == "" || name == "." || name == ".." || filepath.IsAbs(name) || filepath.VolumeName(name) != "" || strings.ContainsAny(name, `/\\`) {
		return false
	}
	if len(name) >= 2 && ((name[0] >= 'A' && name[0] <= 'Z') || (name[0] >= 'a' && name[0] <= 'z')) && name[1] == ':' {
		return false
	}
	return !strings.ContainsRune(name, '\x00')
}
