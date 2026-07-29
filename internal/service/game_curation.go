package service

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"foliospace-reader/internal/domain"
	"foliospace-reader/internal/launchcatalog"
)

const defaultMAMEPlatforms = "arcade,mame,model2,cps,cps1,cps2,cps3,neogeo"

func (s *Service) defaultGameCatalogSettings() domain.GameCatalogSettings {
	policyDir := filepath.Join(s.configDir, "policies")
	return domain.GameCatalogSettings{
		AutoAnalyzeAfterScan: true,
		EnableLibretroCovers: true,
		FBNeoDATPath:         filepath.Join(policyDir, "fbneo-arcade.dat"),
		MAMEListXMLPath:      filepath.Join(policyDir, "mame0288lx.zip"),
		LaunchTargetsPath:    filepath.Join(policyDir, "targets.json"),
		MAMEPlatforms:        defaultMAMEPlatforms,
		MetadataProvider:     "local",
	}
}

func (s *Service) GameCatalogSettings() domain.GameCatalogSettings {
	settings := s.defaultGameCatalogSettings()
	raw, err := s.store.Setting(gameCatalogSettingsSetting)
	if err != nil {
		return settings
	}
	var saved domain.GameCatalogSettings
	if json.Unmarshal([]byte(raw), &saved) != nil {
		return settings
	}
	return normalizeGameCatalogSettings(saved, settings)
}

func normalizeGameCatalogSettings(settings, defaults domain.GameCatalogSettings) domain.GameCatalogSettings {
	if strings.TrimSpace(settings.FBNeoDATPath) == "" {
		settings.FBNeoDATPath = defaults.FBNeoDATPath
	}
	if strings.TrimSpace(settings.MAMEListXMLPath) == "" {
		settings.MAMEListXMLPath = defaults.MAMEListXMLPath
	}
	if strings.TrimSpace(settings.LaunchTargetsPath) == "" {
		settings.LaunchTargetsPath = defaults.LaunchTargetsPath
	}
	if strings.TrimSpace(settings.MAMEPlatforms) == "" {
		settings.MAMEPlatforms = defaults.MAMEPlatforms
	}
	switch strings.ToLower(strings.TrimSpace(settings.MetadataProvider)) {
	case "local", "hasheous", "disabled":
		settings.MetadataProvider = strings.ToLower(strings.TrimSpace(settings.MetadataProvider))
	default:
		settings.MetadataProvider = defaults.MetadataProvider
	}
	return settings
}

func (s *Service) SaveGameCatalogSettings(settings domain.GameCatalogSettings) error {
	settings = normalizeGameCatalogSettings(settings, s.defaultGameCatalogSettings())
	data, err := json.Marshal(settings)
	if err != nil {
		return err
	}
	return s.store.UpsertSetting(gameCatalogSettingsSetting, string(data))
}

func (s *Service) GameCurationSummary() (domain.GameCurationSummary, error) {
	var summary domain.GameCurationSummary
	var err error
	summary.Total, summary.Ready, summary.NeedsCuration, summary.Dependencies, err = s.store.GameCatalogRoleCounts()
	if err != nil {
		return summary, err
	}
	summary.MetadataReady, summary.ArtworkReady, err = s.store.GameCatalogEnrichmentCounts()
	if err != nil {
		return summary, err
	}
	summary.Policies = s.gameCatalogPolicyStatuses()
	summary.LastTask = s.GameCatalogTaskStatus()
	return summary, nil
}

func (s *Service) gameCatalogPolicyStatuses() []domain.GameCatalogPolicyStatus {
	settings := s.GameCatalogSettings()
	return []domain.GameCatalogPolicyStatus{
		gameCatalogPolicyStatus("fbneo", settings.FBNeoDATPath),
		gameCatalogPolicyStatus("mame", settings.MAMEListXMLPath),
		gameCatalogPolicyStatus("targets", settings.LaunchTargetsPath),
	}
}

func gameCatalogPolicyStatus(id, path string) domain.GameCatalogPolicyStatus {
	status := domain.GameCatalogPolicyStatus{ID: id, Path: path}
	info, err := os.Stat(path)
	status.Available = err == nil && !info.IsDir()
	if !status.Available {
		status.Message = "File is not installed. Copy a compatible policy file into /config/policies or choose another path."
	}
	return status
}

func (s *Service) ListGameCurationPage(options domain.GameListOptions) (domain.GameCurationPage, error) {
	options.IncludeDependencies = true
	page, err := s.store.ListGamesPage(options)
	if err != nil {
		return domain.GameCurationPage{}, err
	}
	items := make([]domain.GameCurationItem, 0, len(page.Items))
	policies := s.gameCatalogPolicyStatuses()
	for _, game := range page.Items {
		details, detailsErr := s.store.GameDetails(game.ID)
		profiles, profileErr := s.store.GameLaunchProfiles(game.ID)
		item := domain.GameCurationItem{Game: game, MetadataStatus: "missing", ArtworkStatus: "missing"}
		if detailsErr == nil {
			item.MetadataStatus = details.MetadataStatus
			for _, artwork := range details.Artwork {
				if artwork.Kind == "cover" && artwork.Selected {
					item.ArtworkStatus = "ready"
					break
				}
			}
		}
		if profileErr == nil {
			item.ReadyProfiles = len(profiles)
		}
		item.IssueCode, item.IssueMessage = gameCurationIssue(game, profiles, policies)
		items = append(items, item)
	}
	return domain.GameCurationPage{Items: items, Total: page.Total, Limit: page.Limit, Offset: page.Offset, HasMore: page.HasMore}, nil
}

func gameCurationIssue(game domain.GameAsset, profiles []domain.GameLaunchProfile, policies []domain.GameCatalogPolicyStatus) (string, string) {
	switch strings.ToLower(strings.TrimSpace(game.CatalogRole)) {
	case "dependency":
		return "dependency", "This file is a BIOS, device, parent, or track dependency and is intentionally hidden from clients."
	case "needs-curation":
		if strings.TrimSpace(game.SHA1) == "" && strings.EqualFold(game.Format, "zip") {
			return "identity-missing", "The archive has no stable SHA-1 identity and cannot be audited. Rescan the file."
		}
		available := false
		for _, policy := range policies {
			if policy.ID != "targets" && policy.Available {
				available = true
			}
		}
		if !available {
			return "policy-pack-missing", "No FBNeo or MAME compatibility policy is installed."
		}
		return "launch-profile-missing", "The ROM did not pass an installed compatibility policy, or a required parent/BIOS archive is missing."
	default:
		if len(profiles) == 0 && launchcatalog.IsStrictArcadePlatform(game.Platform) {
			return "launch-profile-missing", "The catalog entry is visible but has no audited runtime profile. Re-run compatibility analysis."
		}
	}
	return "", ""
}

func (s *Service) handleCompletedScan(library domain.Library, _ domain.ScanJob) {
	if library.AssetType != "game" && library.AssetType != "mixed" {
		return
	}
	if _, err := s.store.Setting(gameCatalogSettingsSetting); err != nil {
		return
	}
	if s.GameCatalogSettings().AutoAnalyzeAfterScan {
		_, _ = s.StartGameCatalogAnalysis()
	}
}

func (s *Service) GameCatalogTaskStatus() domain.GameCatalogTaskStatus {
	s.gameCatalogMu.Lock()
	defer s.gameCatalogMu.Unlock()
	if s.gameCatalogTask.Status != "" {
		return s.gameCatalogTask
	}
	raw, err := s.store.Setting(gameCatalogTaskSetting)
	if err == nil {
		_ = json.Unmarshal([]byte(raw), &s.gameCatalogTask)
		if s.gameCatalogTask.Status == "running" {
			now := time.Now()
			s.gameCatalogTask.Status = "interrupted"
			s.gameCatalogTask.EndedAt = &now
			s.gameCatalogTask.Message = "The previous background task was interrupted by a service restart."
		}
	}
	return s.gameCatalogTask
}

func (s *Service) beginGameCatalogTask(action string) (domain.GameCatalogTaskStatus, error) {
	s.gameCatalogMu.Lock()
	defer s.gameCatalogMu.Unlock()
	if s.gameCatalogTask.Status == "running" {
		return s.gameCatalogTask, fmt.Errorf("game catalog task %s is already running", s.gameCatalogTask.ID)
	}
	now := time.Now()
	s.gameCatalogTask = domain.GameCatalogTaskStatus{
		ID: fmt.Sprintf("%s-%d", action, now.Unix()), Action: action, Status: "running",
		Message: "Background task started.", StartedAt: &now, Details: map[string]any{},
	}
	s.persistGameCatalogTaskLocked()
	return s.gameCatalogTask, nil
}

func (s *Service) updateGameCatalogTask(update func(*domain.GameCatalogTaskStatus)) {
	s.gameCatalogMu.Lock()
	defer s.gameCatalogMu.Unlock()
	update(&s.gameCatalogTask)
	s.persistGameCatalogTaskLocked()
}

func (s *Service) persistGameCatalogTaskLocked() {
	data, err := json.Marshal(s.gameCatalogTask)
	if err == nil {
		_ = s.store.UpsertSetting(gameCatalogTaskSetting, string(data))
	}
}

func (s *Service) finishGameCatalogTask(err error, message string) {
	s.updateGameCatalogTask(func(task *domain.GameCatalogTaskStatus) {
		now := time.Now()
		task.EndedAt = &now
		task.Message = message
		if err != nil {
			task.Status = "failed"
			task.Failed++
		} else {
			task.Status = "completed"
		}
	})
}

func (s *Service) StartGameCatalogAnalysis() (domain.GameCatalogTaskStatus, error) {
	task, err := s.beginGameCatalogTask("analyze")
	if err != nil {
		return task, err
	}
	go s.runGameCatalogAnalysis()
	return task, nil
}

func (s *Service) runGameCatalogAnalysis() {
	processed, changed, err := s.normalizeGameCatalogRoles()
	if err != nil {
		s.finishGameCatalogTask(err, err.Error())
		return
	}
	s.updateGameCatalogTask(func(task *domain.GameCatalogTaskStatus) {
		task.Processed = processed
		task.Matched = changed
		task.Message = "Base catalog classification completed; compatibility policies are being evaluated."
	})

	settings := s.GameCatalogSettings()
	policyRuns := 0
	for _, run := range []struct {
		id   string
		path string
		args []string
	}{
		{id: "fbneo", path: settings.FBNeoDATPath, args: []string{"-policy", "fbneo", "-dat", settings.FBNeoDATPath}},
		{id: "mame", path: settings.MAMEListXMLPath, args: []string{"-policy", "mame", "-mame-listxml", settings.MAMEListXMLPath, "-platforms", settings.MAMEPlatforms}},
	} {
		if info, statErr := os.Stat(run.path); statErr != nil || info.IsDir() {
			continue
		}
		args := append([]string{}, run.args...)
		if targetInfo, targetErr := os.Stat(settings.LaunchTargetsPath); targetErr == nil && !targetInfo.IsDir() {
			args = append(args, "-targets", settings.LaunchTargetsPath)
		}
		output, runErr := s.runLaunchProfileRebuild(args)
		if runErr != nil {
			s.finishGameCatalogTask(runErr, fmt.Sprintf("%s compatibility analysis failed: %v", run.id, runErr))
			return
		}
		policyRuns++
		s.updateGameCatalogTask(func(task *domain.GameCatalogTaskStatus) {
			task.Details[run.id] = output
		})
	}
	message := fmt.Sprintf("Catalog analysis completed; %d compatibility policy run(s) applied.", policyRuns)
	if policyRuns == 0 {
		message = "Base classification completed. Install an FBNeo or MAME policy file to publish audited arcade ROMs."
	}
	s.finishGameCatalogTask(nil, message)
}

func (s *Service) normalizeGameCatalogRoles() (processed, changed int64, err error) {
	for offset := 0; ; offset += 200 {
		page, pageErr := s.store.ListGamesPage(domain.GameListOptions{Limit: 200, Offset: offset, Sort: "title"})
		if pageErr != nil {
			return processed, changed, pageErr
		}
		for _, game := range page.Items {
			processed++
			var dosLaunch *domain.DOSLaunch
			if strings.EqualFold(game.Platform, "dos") {
				if launch, launchErr := s.store.DOSLaunch(game.ID); launchErr == nil {
					dosLaunch = &launch
				}
			}
			expected := launchcatalog.CatalogRole(game, dosLaunch)
			if launchcatalog.IsStrictArcadePlatform(game.Platform) {
				if profiles, profileErr := s.store.GameLaunchProfiles(game.ID); profileErr == nil && len(profiles) > 0 {
					expected = launchcatalog.RoleGame
				}
			}
			if expected != strings.ToLower(strings.TrimSpace(game.CatalogRole)) {
				if updateErr := s.store.UpdateGameCatalogRole(game.ID, expected); updateErr != nil {
					return processed, changed, updateErr
				}
				changed++
			}
		}
		if !page.HasMore {
			break
		}
	}
	return processed, changed, nil
}

func (s *Service) runLaunchProfileRebuild(args []string) (map[string]any, error) {
	binary := strings.TrimSpace(os.Getenv("FOLIOSPACE_PROFILE_REBUILD_BIN"))
	if binary == "" {
		binary = "/app/foliospace-rebuild-launch-profiles"
	}
	if _, err := os.Stat(binary); err != nil {
		if path, lookupErr := exec.LookPath("foliospace-rebuild-launch-profiles"); lookupErr == nil {
			binary = path
		} else {
			return nil, fmt.Errorf("launch profile rebuild tool is not installed")
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Hour)
	defer cancel()
	command := exec.CommandContext(ctx, binary, args...)
	command.Env = append(os.Environ(), "FOLIOSPACE_CONFIG_DIR="+s.configDir)
	output, err := command.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("%w: %s", err, strings.TrimSpace(string(output)))
	}
	var result map[string]any
	if err := json.Unmarshal(output, &result); err != nil {
		return nil, fmt.Errorf("invalid rebuild output: %w", err)
	}
	return result, nil
}

func (s *Service) StartGameCoverMatch(includeNetwork bool) (domain.GameCatalogTaskStatus, error) {
	if includeNetwork && !s.GameCatalogSettings().EnableLibretroCovers {
		return domain.GameCatalogTaskStatus{}, fmt.Errorf("Libretro cover matching is disabled in game catalog settings")
	}
	action := "covers-local"
	if includeNetwork {
		action = "covers-libretro"
	}
	task, err := s.beginGameCatalogTask(action)
	if err != nil {
		return task, err
	}
	go s.runGameCoverMatch(includeNetwork)
	return task, nil
}

func (s *Service) runGameCoverMatch(includeNetwork bool) {
	var processed, matched, failed int64
	for offset := 0; ; offset += 200 {
		page, err := s.store.ListGamesPage(domain.GameListOptions{Limit: 200, Offset: offset})
		if err != nil {
			s.finishGameCatalogTask(err, err.Error())
			return
		}
		for _, game := range page.Items {
			processed++
			if stream, ok := s.openSelectedGameCover(game.ID); ok {
				_ = stream.Body.Close()
				continue
			}
			localPath := ""
			for _, candidate := range localGameCoverCandidates(game.FilePath) {
				if info, statErr := os.Stat(candidate); statErr == nil && !info.IsDir() {
					localPath = candidate
					break
				}
			}
			if localPath != "" {
				_, err = s.store.UpsertGameArtwork(domain.GameArtwork{GameID: game.ID, Source: "local", Kind: "cover", CachePath: localPath, Selected: true, Confidence: 1})
				if err == nil {
					matched++
				} else {
					failed++
				}
			} else if includeNetwork {
				stream, coverErr := s.OpenGameCover(game.ID)
				if coverErr == nil {
					_ = stream.Body.Close()
					matched++
				} else {
					failed++
				}
			}
			if processed%10 == 0 {
				s.updateGameCatalogTask(func(task *domain.GameCatalogTaskStatus) {
					task.Processed, task.Matched, task.Failed = processed, matched, failed
					task.Message = fmt.Sprintf("Checked %d games; matched %d covers.", processed, matched)
				})
			}
		}
		if !page.HasMore {
			break
		}
	}
	s.updateGameCatalogTask(func(task *domain.GameCatalogTaskStatus) {
		task.Processed, task.Matched, task.Failed = processed, matched, failed
	})
	s.finishGameCatalogTask(nil, fmt.Sprintf("Cover matching completed: %d matched, %d unavailable.", matched, failed))
}

func (s *Service) UpdateGameMetadata(id int64, metadata domain.GameMetadata) (domain.GameDetails, error) {
	if _, err := s.store.GameByID(id); err != nil {
		return domain.GameDetails{}, err
	}
	metadata.GameID = id
	if metadata.Rating < 0 || metadata.Rating > 5 {
		return domain.GameDetails{}, fmt.Errorf("rating must be between 0 and 5")
	}
	if err := s.store.UpsertGameMetadata(metadata); err != nil {
		return domain.GameDetails{}, err
	}
	_, err := s.store.UpsertGameMetadataSource(domain.GameMetadataSource{
		GameID: id, Source: "manual", SourceID: fmt.Sprintf("game:%d", id), MatchedBy: "manual", Confidence: 1,
	})
	if err != nil {
		return domain.GameDetails{}, err
	}
	return s.store.GameDetails(id)
}
