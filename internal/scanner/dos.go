package scanner

import (
	stdzip "archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"foliospace-reader/internal/domain"
	"foliospace-reader/internal/launchcatalog"
)

const (
	dosArchiveMaxEntries           = 10000
	dosArchiveMaxPathBytes         = 512
	dosArchiveMaxDeclaredBytes     = int64(16 << 30)
	dosArchiveMaxConfigReadBytes   = int64(256 << 10)
	dosArchiveMaxMetadataPathBytes = int64(2 << 20)
)

type dosCatalogFile struct {
	Games map[string]dosCatalogEntry `json:"games"`
}

type dosCatalogEntry struct {
	Identifier       string            `json:"identifier"`
	Name             map[string]string `json:"name"`
	ReleaseYear      int               `json:"releaseYear"`
	Executable       string            `json:"executable"`
	InstallDirectory string            `json:"installDirectory"`
	Keymaps          map[string]string `json:"keymaps"`
	Links            map[string]string `json:"links"`
	CoverFilename    string            `json:"coverFilename"`
	SHA256           string            `json:"sha256"`
	Filesize         int64             `json:"filesize"`
}

type dosCatalogIndex struct {
	root     string
	revision string
	byDigest map[string][]dosCatalogEntry
}

type dosArchiveInspection struct {
	members    map[string]string
	candidates []domain.DOSLaunchCandidate
	configs    map[string][]byte
}

func (s *Scanner) indexDOSGameFile(library domain.Library, filePath string, info fs.FileInfo, ext string, relPath string) error {
	checksums, err := fileChecksums(filePath)
	if err != nil {
		return err
	}
	catalog, _ := s.dosCatalogForPath(library.RootPath, filePath)
	launch := domain.DOSLaunch{
		EntrySource:     "unknown",
		Arguments:       []string{},
		Candidates:      []domain.DOSLaunchCandidate{},
		KeymapHints:     map[string]string{},
		SourceSHA256:    checksums.sha256,
		CatalogRevision: catalog.revision,
		AuditStatus:     "unmatched",
	}
	title := gameTitle(filePath)
	var matched *dosCatalogEntry
	if entries := catalog.byDigest[dosDigestKey(checksums.sha256, info.Size())]; len(entries) == 1 {
		entry := entries[0]
		matched = &entry
		launch.SourceIdentifier = entry.Identifier
		launch.KeymapHints = entry.Keymaps
		launch.AuditStatus = "matched"
		if installDirectory, ok := normalizeDOSInstallDirectory(entry.InstallDirectory); ok {
			launch.InstallDirectory = installDirectory
		} else if strings.TrimSpace(entry.InstallDirectory) != "" {
			launch.AuditStatus = "invalid_install_directory"
		}
		if localized := dosLocalizedTitle(entry); localized != "" {
			title = localized
		}
	} else if len(entries) > 1 {
		launch.AuditStatus = "ambiguous_catalog_match"
	}

	if ext == ".zip" || ext == ".dosz" {
		inspection, inspectErr := inspectDOSArchive(filePath)
		if inspectErr == nil {
			launch.Candidates = inspection.candidates
			if matched != nil {
				if entry, args, ok := resolveDOSCommand(matched.Executable, inspection.members); ok {
					launch.EntryFile = entry
					launch.EntrySource = "curated"
					launch.Arguments = args
					launch.WorkingDirectory = dosEntryDirectory(entry)
				} else {
					launch.AuditStatus = "curated_entry_missing"
				}
			}
			if launch.EntryFile == "" {
				if entry, args, _, ok := resolveDOSBoxConfig(inspection); ok {
					launch.EntryFile = entry
					launch.EntrySource = "dosboxConfig"
					launch.Arguments = args
					launch.WorkingDirectory = dosEntryDirectory(entry)
				}
			}
		} else {
			launch.AuditStatus = "archive_inventory_unavailable"
		}
	} else {
		launch.EntryFile = filepath.Base(filePath)
		launch.EntrySource = "curated"
		launch.Candidates = []domain.DOSLaunchCandidate{{Path: filepath.Base(filePath), Kind: strings.TrimPrefix(ext, ".")}}
	}

	gameAsset := domain.GameAsset{
		LibraryID: library.ID, Title: title, Platform: "dos", ROMSetName: "DOS", Format: strings.TrimPrefix(ext, "."),
		FilePath: filePath, RelPath: filepath.ToSlash(relPath), Size: info.Size(), MTime: info.ModTime(),
		CRC32: checksums.crc32, SHA1: checksums.sha1, EmulatorHint: "dosbox-staging", Compatibility: "unknown", CatalogRole: "game",
	}
	gameAsset.CatalogRole = launchcatalog.CatalogRole(gameAsset, &launch)
	game, err := s.store.UpsertGame(gameAsset)
	if err != nil {
		return err
	}
	if err := s.store.ReplaceGameFiles(game.ID, []domain.GameFile{{
		GameID: game.ID, Name: filepath.Base(filePath), FilePath: filePath, Size: info.Size(), MTime: info.ModTime(), SHA1: checksums.sha1, Role: "entry", Position: 0,
	}}); err != nil {
		return err
	}
	launch.GameID = game.ID
	if err := s.store.UpsertDOSLaunch(launch); err != nil {
		return err
	}
	if matched == nil {
		return nil
	}
	if err := s.store.UpsertGameMetadata(domain.GameMetadata{
		GameID: game.ID, DisplayTitle: title, ReleaseDate: dosReleaseDate(matched.ReleaseYear), ExternalLinks: dosSortedLinkValues(matched.Links),
	}); err != nil {
		return err
	}
	raw, _ := json.Marshal(matched)
	if _, err := s.store.UpsertGameMetadataSource(domain.GameMetadataSource{
		GameID: game.ID, Source: "dos-games-json", SourceID: matched.Identifier, MatchedBy: "sha256+filesize", Confidence: 1, RawJSON: string(raw),
	}); err != nil {
		return err
	}
	if coverPath, ok := dosCatalogCoverPath(catalog.root, *matched); ok {
		_, err = s.store.UpsertGameArtwork(domain.GameArtwork{
			GameID: game.ID, Source: "dos-games-json", Kind: "cover", CachePath: coverPath, Selected: true, Confidence: 1,
		})
		return err
	}
	return nil
}

func (s *Scanner) dosCatalogForPath(libraryRoot string, gamePath string) (dosCatalogIndex, bool) {
	root := filepath.Clean(libraryRoot)
	for dir := filepath.Clean(filepath.Dir(gamePath)); ; dir = filepath.Dir(dir) {
		catalogPath := filepath.Join(dir, "games.json")
		if info, err := os.Stat(catalogPath); err == nil && !info.IsDir() {
			revision := fmt.Sprintf("%d:%d", info.Size(), info.ModTime().UnixNano())
			cacheKey := catalogPath + "\x00" + revision
			if cached, ok := s.dosCatalogCache.Load(cacheKey); ok {
				return cached.(dosCatalogIndex), true
			}
			data, err := os.ReadFile(catalogPath)
			if err != nil {
				return dosCatalogIndex{}, false
			}
			var source dosCatalogFile
			if err := json.Unmarshal(data, &source); err != nil {
				return dosCatalogIndex{}, false
			}
			index := dosCatalogIndex{root: dir, revision: revision, byDigest: make(map[string][]dosCatalogEntry)}
			for key, entry := range source.Games {
				if strings.TrimSpace(entry.Identifier) == "" {
					entry.Identifier = key
				}
				entry.SHA256 = strings.ToLower(strings.TrimSpace(entry.SHA256))
				if len(entry.SHA256) != 64 || entry.Filesize <= 0 {
					continue
				}
				digestKey := dosDigestKey(entry.SHA256, entry.Filesize)
				index.byDigest[digestKey] = append(index.byDigest[digestKey], entry)
			}
			s.dosCatalogCache.Store(cacheKey, index)
			return index, true
		}
		if dir == root || filepath.Dir(dir) == dir {
			break
		}
		if rel, err := filepath.Rel(root, dir); err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			break
		}
	}
	return dosCatalogIndex{}, false
}

func inspectDOSArchive(path string) (dosArchiveInspection, error) {
	archive, err := stdzip.OpenReader(path)
	if err != nil {
		return dosArchiveInspection{}, err
	}
	defer archive.Close()
	if len(archive.File) > dosArchiveMaxEntries {
		return dosArchiveInspection{}, fmt.Errorf("DOS archive has too many entries: %d", len(archive.File))
	}
	inspection := dosArchiveInspection{members: map[string]string{}, configs: map[string][]byte{}}
	counts := map[string]int{}
	valid := make([]struct{ path, kind string }, 0)
	var declared int64
	var pathBytes int64
	for _, member := range archive.File {
		if member.FileInfo().IsDir() {
			continue
		}
		name, ok := normalizeDOSArchivePath(member.Name)
		if !ok {
			continue
		}
		declared += int64(member.UncompressedSize64)
		pathBytes += int64(len(name))
		if declared > dosArchiveMaxDeclaredBytes || pathBytes > dosArchiveMaxMetadataPathBytes {
			return dosArchiveInspection{}, fmt.Errorf("DOS archive inventory exceeds safety limits")
		}
		lower := strings.ToLower(name)
		counts[lower]++
		inspection.members[lower] = name
		ext := strings.ToLower(filepath.Ext(name))
		if ext == ".bat" || ext == ".com" || ext == ".exe" {
			valid = append(valid, struct{ path, kind string }{name, strings.TrimPrefix(ext, ".")})
		}
		if strings.EqualFold(filepath.Base(name), "dosbox.conf") && member.UncompressedSize64 <= uint64(dosArchiveMaxConfigReadBytes) {
			reader, openErr := member.Open()
			if openErr == nil {
				data, readErr := io.ReadAll(io.LimitReader(reader, dosArchiveMaxConfigReadBytes+1))
				_ = reader.Close()
				if readErr == nil && int64(len(data)) <= dosArchiveMaxConfigReadBytes {
					inspection.configs[lower] = data
				}
			}
		}
	}
	for lower, count := range counts {
		if count > 1 {
			delete(inspection.members, lower)
			delete(inspection.configs, lower)
		}
	}
	for _, candidate := range valid {
		if counts[strings.ToLower(candidate.path)] == 1 {
			inspection.candidates = append(inspection.candidates, domain.DOSLaunchCandidate{Path: candidate.path, Kind: candidate.kind})
		}
	}
	sort.Slice(inspection.candidates, func(i, j int) bool {
		return strings.ToLower(inspection.candidates[i].Path) < strings.ToLower(inspection.candidates[j].Path)
	})
	return inspection, nil
}

func normalizeDOSArchivePath(name string) (string, bool) {
	if !utf8.ValidString(name) || strings.ContainsRune(name, '\x00') || len(name) == 0 || len(name) > dosArchiveMaxPathBytes {
		return "", false
	}
	name = strings.ReplaceAll(name, `\`, "/")
	if strings.HasPrefix(name, "/") || strings.HasPrefix(name, "//") || (len(name) >= 2 && name[1] == ':') {
		return "", false
	}
	parts := strings.Split(name, "/")
	for _, part := range parts {
		if part == "" || part == "." || part == ".." {
			return "", false
		}
	}
	return strings.Join(parts, "/"), true
}

func normalizeDOSInstallDirectory(name string) (string, bool) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", true
	}
	if !utf8.ValidString(name) || strings.ContainsAny(name, "\x00\r\n/\\:;&|<>^") || name == "." || name == ".." || len(name) > 64 {
		return "", false
	}
	return name, true
}

func resolveDOSCommand(command string, members map[string]string) (string, []string, bool) {
	tokens := splitDOSCommandLine(command)
	for split := len(tokens); split >= 1; split-- {
		program := strings.Join(tokens[:split], " ")
		if resolved, ok := resolveDOSProgram(program, members); ok {
			arguments := append([]string{}, tokens[split:]...)
			if !safeDOSArguments(arguments) {
				return "", nil, false
			}
			return resolved, arguments, true
		}
	}
	return "", nil, false
}

func safeDOSArguments(arguments []string) bool {
	for _, argument := range arguments {
		if !utf8.ValidString(argument) || strings.ContainsAny(argument, "\x00\r\n;&|<>^") {
			return false
		}
	}
	return true
}

func resolveDOSProgram(program string, members map[string]string) (string, bool) {
	program = strings.Trim(strings.TrimSpace(program), `"'`)
	if normalized, ok := normalizeDOSArchivePath(program); ok {
		if member, exists := members[strings.ToLower(normalized)]; exists {
			return member, true
		}
	}
	base := strings.ToLower(filepath.Base(strings.ReplaceAll(program, `\`, "/")))
	ext := strings.ToLower(filepath.Ext(base))
	matches := make([]string, 0)
	for _, member := range members {
		memberBase := strings.ToLower(filepath.Base(member))
		if ext == "" {
			memberExt := strings.ToLower(filepath.Ext(memberBase))
			if (memberExt == ".com" || memberExt == ".exe" || memberExt == ".bat") && strings.TrimSuffix(memberBase, memberExt) == base {
				matches = append(matches, member)
			}
		} else if memberBase == base {
			matches = append(matches, member)
		}
	}
	if len(matches) == 1 {
		return matches[0], true
	}
	return "", false
}

func resolveDOSBoxConfig(inspection dosArchiveInspection) (string, []string, string, bool) {
	keys := make([]string, 0, len(inspection.configs))
	for key := range inspection.configs {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		configPath := inspection.members[key]
		inAutoexec := false
		for _, rawLine := range bytes.Split(inspection.configs[key], []byte{'\n'}) {
			line := strings.TrimSpace(strings.TrimSuffix(string(rawLine), "\r"))
			if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
				inAutoexec = strings.EqualFold(line, "[autoexec]")
				continue
			}
			if !inAutoexec || line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") || strings.HasPrefix(line, "@") {
				continue
			}
			fields := splitDOSCommandLine(line)
			if len(fields) == 0 {
				continue
			}
			switch strings.ToLower(fields[0]) {
			case "mount", "imgmount", "cd", "c:", "cls", "echo", "exit", "set", "config":
				continue
			}
			if entry, args, ok := resolveDOSCommand(line, inspection.members); ok {
				return entry, args, configPath, true
			}
		}
	}
	return "", nil, "", false
}

func splitDOSCommandLine(command string) []string {
	var out []string
	var token strings.Builder
	var quote rune
	flush := func() {
		if token.Len() > 0 {
			out = append(out, token.String())
			token.Reset()
		}
	}
	for _, r := range strings.TrimSpace(command) {
		if quote != 0 {
			if r == quote {
				quote = 0
			} else {
				token.WriteRune(r)
			}
			continue
		}
		if r == '"' || r == '\'' {
			quote = r
			continue
		}
		if r == ' ' || r == '\t' {
			flush()
			continue
		}
		token.WriteRune(r)
	}
	flush()
	return out
}

func dosDigestKey(sha256 string, size int64) string {
	return strings.ToLower(strings.TrimSpace(sha256)) + ":" + strconv.FormatInt(size, 10)
}

func dosLocalizedTitle(entry dosCatalogEntry) string {
	for _, locale := range []string{"zh-Hans", "zh-Hant", "en"} {
		if value := strings.TrimSpace(entry.Name[locale]); value != "" {
			return value
		}
	}
	return strings.TrimSpace(entry.Identifier)
}

func dosReleaseDate(year int) string {
	if year <= 0 {
		return ""
	}
	return strconv.Itoa(year)
}

func dosSortedLinkValues(links map[string]string) []string {
	values := make([]string, 0, len(links))
	for _, value := range links {
		if value = strings.TrimSpace(value); value != "" {
			values = append(values, value)
		}
	}
	sort.Strings(values)
	return values
}

func dosEntryDirectory(entry string) string {
	dir := filepath.ToSlash(filepath.Dir(entry))
	if dir == "." {
		return ""
	}
	return dir
}

func dosCatalogCoverPath(root string, entry dosCatalogEntry) (string, bool) {
	identifier, ok := normalizeDOSArchivePath(entry.Identifier)
	if !ok || strings.Contains(identifier, "/") {
		return "", false
	}
	filename, ok := normalizeDOSArchivePath(entry.CoverFilename)
	if !ok || strings.Contains(filename, "/") {
		return "", false
	}
	assetRoot := filepath.Join(root, "img")
	candidate := filepath.Join(assetRoot, filepath.FromSlash(identifier), filepath.FromSlash(filename))
	rel, err := filepath.Rel(assetRoot, candidate)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", false
	}
	info, err := os.Stat(candidate)
	return candidate, err == nil && !info.IsDir()
}
