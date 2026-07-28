package service

import (
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"foliospace-reader/internal/domain"
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
		resolvedGame.Title = profile.Title
		resolvedGame.ROMSetName = profile.ROMSetName
		resolvedGame.Size = totalSize
		return domain.GameLaunchResolution{
			LaunchProfileID: profile.ID, ProfileRevision: profile.Revision, Runtime: runtime,
			Game: resolvedGame, EntryFile: profile.Files[0].Name, Files: resolvedFiles,
		}, nil
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
			return profile.Runtime, true
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
