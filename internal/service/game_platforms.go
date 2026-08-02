package service

import (
	"sort"
	"strings"

	"foliospace-reader/internal/domain"
	"foliospace-reader/internal/store"
)

var supportedGamePlatforms = []domain.GamePlatformDefinition{
	{Platform: "nes", Title: "NES", Aliases: []string{"famicom"}},
	{Platform: "snes", Title: "SNES", Aliases: []string{"sfc", "super-nintendo"}},
	{Platform: "virtualboy", Title: "Virtual Boy", Aliases: []string{"virtual-boy", "virtual boy"}},
	{Platform: "gb", Title: "GB", Aliases: []string{"game-boy"}},
	{Platform: "gbc", Title: "GBC", Aliases: []string{"game-boy-color"}},
	{Platform: "gba", Title: "GBA", Aliases: []string{"game-boy-advance"}},
	{Platform: "nds", Title: "Nintendo DS", Aliases: []string{"nintendo-ds"}},
	{Platform: "3ds", Title: "Nintendo 3DS", Aliases: []string{"nintendo-3ds"}},
	{Platform: "md", Title: "Mega Drive", Aliases: []string{"genesis", "mega-drive", "megadrive"}},
	{Platform: "32x", Title: "32X", Aliases: []string{"sega-32x"}},
	{Platform: "saturn", Title: "Saturn", Aliases: []string{"ss", "sega-saturn"}},
	{Platform: "n64", Title: "Nintendo 64", Aliases: []string{"nintendo-64"}},
	{Platform: "ps1", Title: "PlayStation", Aliases: []string{"ps", "psx", "playstation-1"}},
	{Platform: "psp", Title: "PSP", Aliases: []string{"playstation-portable"}},
	{Platform: "ps2", Title: "PlayStation 2", Aliases: []string{"playstation-2"}},
	{Platform: "ngc", Title: "Nintendo GameCube", Aliases: []string{"gamecube", "game-cube"}},
	{Platform: "dreamcast", Title: "Dreamcast", Aliases: []string{"dc", "sega-dreamcast"}},
	{Platform: "pc-fx", Title: "PC-FX", Aliases: []string{"pcfx", "nec-pc-fx"}},
	{Platform: "pc98", Title: "NEC PC-98", Aliases: []string{"pc-98", "pc9801", "pc9821"}},
	{Platform: "dos", Title: "DOS", Aliases: []string{"ms-dos", "msdos"}},
	{Platform: "neogeo", Title: "Neo Geo", Aliases: []string{"neo-geo"}},
	{Platform: "cps1", Title: "CPS-1", Aliases: []string{"cps-1"}},
	{Platform: "cps2", Title: "CPS-2", Aliases: []string{"cps-2"}},
	{Platform: "cps3", Title: "CPS-3", Aliases: []string{"cps-3"}},
	{Platform: "model2", Title: "Model 2", Aliases: []string{"model-2", "sega-model-2"}},
	{Platform: "model3", Title: "Model 3", Aliases: []string{"model-3", "sega-model-3"}},
	{Platform: "naomi", Title: "NAOMI", Aliases: []string{"sega-naomi"}},
	{Platform: "naomi2", Title: "NAOMI 2", Aliases: []string{"naomi-2", "sega-naomi-2"}},
	{Platform: "arcade", Title: "Arcade"},
	{Platform: "mame", Title: "MAME"},
}

func (s *Service) ListGamePlatforms() (domain.GamePlatformCatalog, error) {
	facets, err := s.ListGameFacets(domain.GameListOptions{ClientVisibleOnly: true})
	if err != nil {
		return domain.GamePlatformCatalog{}, err
	}

	counts := make(map[string]int64, len(facets.Platforms))
	for _, facet := range facets.Platforms {
		counts[strings.ToLower(strings.TrimSpace(facet.Platform))] = facet.Count
	}

	items := make([]domain.GamePlatformDefinition, 0, len(supportedGamePlatforms)+len(facets.Platforms))
	declared := make(map[string]bool, len(supportedGamePlatforms))
	for _, definition := range supportedGamePlatforms {
		definition.Count = counts[definition.Platform]
		definition.Available = definition.Count > 0
		items = append(items, definition)
		declared[definition.Platform] = true
	}

	unknown := make([]domain.GamePlatformDefinition, 0)
	for platform, count := range counts {
		if platform == "" || declared[platform] {
			continue
		}
		unknown = append(unknown, domain.GamePlatformDefinition{
			Platform:  platform,
			Title:     store.GamePlatformLabel(platform),
			Count:     count,
			Available: true,
		})
	}
	sort.Slice(unknown, func(i, j int) bool {
		return unknown[i].Platform < unknown[j].Platform
	})
	items = append(items, unknown...)

	return domain.GamePlatformCatalog{Items: items, Total: len(items)}, nil
}
