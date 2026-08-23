package service

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"foliospace-reader/internal/domain"
)

const CIAInstallCapabilityV1 = "cia-install-v1"

func IsCIAInstallContent(game domain.GameAsset) bool {
	return strings.EqualFold(strings.TrimSpace(game.Platform), "3ds") &&
		strings.EqualFold(strings.TrimSpace(game.Format), "cia")
}

func ValidateGameContentActionRequest(req domain.GameContentActionRequest) error {
	for name, value := range map[string]string{
		"client.name": req.Client.Name, "client.version": req.Client.Version,
		"client.platform": req.Client.Platform, "client.architecture": req.Client.Architecture,
	} {
		value = strings.TrimSpace(value)
		if value == "" || len(value) > 128 {
			return fmt.Errorf("%s must contain 1 to 128 characters", name)
		}
	}
	if len(req.Capabilities) > 32 {
		return fmt.Errorf("capabilities must not contain more than 32 entries")
	}
	for index, capability := range req.Capabilities {
		if strings.TrimSpace(capability) == "" || len(strings.TrimSpace(capability)) > 128 {
			return fmt.Errorf("capabilities[%d] must contain 1 to 128 characters", index)
		}
	}
	return nil
}

func (s *Service) ResolveGameContentInstall(gameID int64, req domain.GameContentActionRequest) (domain.GameContentActionResolution, error) {
	if err := ValidateGameContentActionRequest(req); err != nil {
		return domain.GameContentActionResolution{}, err
	}
	game, err := s.store.GameByID(gameID)
	if err != nil {
		return domain.GameContentActionResolution{}, err
	}
	if !IsCIAInstallContent(game) {
		return domain.GameContentActionResolution{}, launchResolveError(
			"content-mode-unsupported",
			"This content uses launch mode and does not support the install action.",
			map[string]any{"gameId": game.ID, "contentMode": contentModeForGame(game)},
		)
	}
	if !hasGameContentCapability(req.Capabilities, CIAInstallCapabilityV1) {
		return domain.GameContentActionResolution{}, launchResolveError(
			"content-mode-unsupported",
			"This content requires the cia-install-v1 client installation action and cannot be launched directly.",
			map[string]any{
				"gameId": game.ID, "contentMode": contentModeForGame(game), "requiredCapability": CIAInstallCapabilityV1,
			},
		)
	}

	files, err := s.store.GameFiles(game.ID)
	if err != nil {
		return domain.GameContentActionResolution{}, err
	}
	if reason := invalidCIAInstallIntegrityReason(game, files); reason != "" {
		return domain.GameContentActionResolution{}, launchResolveError(
			"content-integrity-invalid",
			"The indexed CIA metadata no longer describes a complete downloadable file. Rescan the source before installing.",
			map[string]any{"gameId": game.ID, "reason": reason},
		)
	}

	return domain.GameContentActionResolution{
		Action: "install", ContentMode: "install", Validation: "client",
		Game: game, EntryFile: files[0].Name, Files: files,
	}, nil
}

func invalidCIAInstallIntegrityReason(game domain.GameAsset, files []domain.GameFile) string {
	if game.Size <= 0 || !sha1Pattern.MatchString(strings.ToLower(strings.TrimSpace(game.SHA1))) {
		return "game-metadata"
	}
	if len(files) != 1 {
		return "entry-count"
	}
	file := files[0]
	if file.Position != 0 || !strings.EqualFold(strings.TrimSpace(file.Role), "entry") ||
		filepath.Base(file.Name) != file.Name || !strings.EqualFold(filepath.Ext(file.Name), ".cia") {
		return "entry-metadata"
	}
	fileSHA1 := strings.ToLower(strings.TrimSpace(file.SHA1))
	if file.Size != game.Size || !sha1Pattern.MatchString(fileSHA1) ||
		!strings.EqualFold(fileSHA1, strings.TrimSpace(game.SHA1)) {
		return "entry-checksum"
	}
	if filepath.Clean(file.FilePath) != filepath.Clean(game.FilePath) {
		return "entry-source"
	}
	info, err := os.Stat(game.FilePath)
	if err != nil || !info.Mode().IsRegular() || info.Size() != game.Size {
		return "source-size"
	}
	return ""
}

func hasGameContentCapability(capabilities []string, expected string) bool {
	for _, capability := range capabilities {
		if strings.EqualFold(strings.TrimSpace(capability), expected) {
			return true
		}
	}
	return false
}

func contentModeForGame(game domain.GameAsset) string {
	if IsCIAInstallContent(game) {
		return "install"
	}
	if strings.EqualFold(strings.TrimSpace(game.Platform), "3ds") {
		return "launch"
	}
	return ""
}
