package archive

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"path/filepath"
	"sort"
	"strings"

	"foliospace-reader/internal/domain"
	_ "golang.org/x/image/webp"
)

const embeddedMetadataMaxBytes int64 = 1024 * 1024

var imageExts = map[string]string{
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".png":  "image/png",
	".webp": "image/webp",
	".gif":  "image/gif",
}

func ListPages(path string) ([]domain.Page, error) {
	reader, err := zip.OpenReader(path)
	if err != nil {
		return nil, fmt.Errorf("open zip: %w", err)
	}
	defer reader.Close()

	files := imageFiles(reader.File)
	if len(files) == 0 {
		return nil, fmt.Errorf("archive has no image pages")
	}

	pages := make([]domain.Page, 0, len(files))
	for i, file := range files {
		pages = append(pages, domain.Page{Index: i, Name: file.Name})
	}
	return pages, nil
}

type EmbeddedComicMetadata struct {
	Title       string
	Creator     string
	Description string
	Tags        []string
}

func ReadEmbeddedComicMetadata(path string) (EmbeddedComicMetadata, bool, error) {
	reader, err := zip.OpenReader(path)
	if err != nil {
		return EmbeddedComicMetadata{}, false, fmt.Errorf("open zip: %w", err)
	}
	defer reader.Close()

	file := selectEmbeddedMetadataFile(reader.File)
	if file == nil {
		return EmbeddedComicMetadata{}, false, nil
	}
	if file.UncompressedSize64 > uint64(embeddedMetadataMaxBytes) {
		return EmbeddedComicMetadata{}, false, fmt.Errorf("embedded metadata too large: %s", file.Name)
	}
	body, err := file.Open()
	if err != nil {
		return EmbeddedComicMetadata{}, false, fmt.Errorf("open embedded metadata: %w", err)
	}
	defer body.Close()
	limited := io.LimitReader(body, embeddedMetadataMaxBytes+1)
	var payload comicMetadataPayload
	if err := json.NewDecoder(limited).Decode(&payload); err != nil {
		return EmbeddedComicMetadata{}, false, fmt.Errorf("decode embedded metadata: %w", err)
	}
	metadata := EmbeddedComicMetadata{
		Title:       strings.TrimSpace(payload.Name),
		Creator:     strings.Join(cleanStringList(payload.Author), ", "),
		Description: strings.TrimSpace(payload.Description),
		Tags:        cleanStringList(payload.Tags),
	}
	if metadata.Title == "" && metadata.Creator == "" && metadata.Description == "" && len(metadata.Tags) == 0 {
		return EmbeddedComicMetadata{}, false, nil
	}
	return metadata, true, nil
}

type comicMetadataPayload struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Author      []string `json:"author"`
	Tags        []string `json:"tags"`
}

func selectEmbeddedMetadataFile(files []*zip.File) *zip.File {
	var fallback *zip.File
	for _, file := range files {
		if file.FileInfo().IsDir() || isMacOSResourceForkEntry(file.Name) || strings.ToLower(filepath.Ext(file.Name)) != ".json" {
			continue
		}
		if fallback == nil || pathDepth(file.Name) < pathDepth(fallback.Name) {
			fallback = file
		}
		base := strings.ToLower(filepath.Base(normalizeEntryName(file.Name)))
		if base == "metadata.json" || base == "info.json" || base == "comicinfo.json" || base == "元数据.json" {
			return file
		}
	}
	return fallback
}

func pathDepth(name string) int {
	clean := strings.Trim(strings.ReplaceAll(name, "\\", "/"), "/")
	if clean == "" {
		return 0
	}
	return strings.Count(clean, "/")
}

func cleanStringList(values []string) []string {
	out := make([]string, 0, len(values))
	seen := map[string]bool{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func OpenPage(path string, pageIndex int) (io.ReadCloser, string, error) {
	reader, err := zip.OpenReader(path)
	if err != nil {
		return nil, "", fmt.Errorf("open zip: %w", err)
	}

	files := imageFiles(reader.File)
	if pageIndex < 0 || pageIndex >= len(files) {
		_ = reader.Close()
		return nil, "", fmt.Errorf("page index %d out of range", pageIndex)
	}

	page, err := files[pageIndex].Open()
	if err != nil {
		_ = reader.Close()
		return nil, "", fmt.Errorf("open page: %w", err)
	}

	return &zipPageReadCloser{ReadCloser: page, closeReader: reader.Close}, contentType(files[pageIndex].Name), nil
}

type CoverMetadata struct {
	Name        string
	FirstName   string
	ContentType string
}

func CoverInfo(path string) (CoverMetadata, error) {
	reader, err := zip.OpenReader(path)
	if err != nil {
		return CoverMetadata{}, fmt.Errorf("open zip: %w", err)
	}
	defer reader.Close()

	files := imageFiles(reader.File)
	if len(files) == 0 {
		return CoverMetadata{}, fmt.Errorf("archive has no image pages")
	}
	cover := selectCoverFile(files)
	return CoverMetadata{
		Name:        cover.Name,
		FirstName:   files[0].Name,
		ContentType: contentType(cover.Name),
	}, nil
}

func OpenCover(path string) (io.ReadCloser, string, error) {
	reader, err := zip.OpenReader(path)
	if err != nil {
		return nil, "", fmt.Errorf("open zip: %w", err)
	}
	files := imageFiles(reader.File)
	if len(files) == 0 {
		_ = reader.Close()
		return nil, "", fmt.Errorf("archive has no image pages")
	}
	cover := selectCoverFile(files)
	page, err := cover.Open()
	if err != nil {
		_ = reader.Close()
		return nil, "", fmt.Errorf("open cover: %w", err)
	}
	return &zipPageReadCloser{ReadCloser: page, closeReader: reader.Close}, contentType(cover.Name), nil
}

func imageFiles(files []*zip.File) []*zip.File {
	out := make([]*zip.File, 0, len(files))
	for _, file := range files {
		if file.FileInfo().IsDir() {
			continue
		}
		if isMacOSResourceForkEntry(file.Name) {
			continue
		}
		if _, ok := imageExts[strings.ToLower(filepath.Ext(file.Name))]; ok {
			out = append(out, file)
		}
	}
	sort.SliceStable(out, func(i int, j int) bool {
		return normalizeEntryName(out[i].Name) < normalizeEntryName(out[j].Name)
	})
	return out
}

func isMacOSResourceForkEntry(name string) bool {
	clean := strings.Trim(strings.ReplaceAll(name, "\\", "/"), "/")
	if clean == "" {
		return false
	}
	for _, part := range strings.Split(clean, "/") {
		if part == "__MACOSX" || strings.HasPrefix(part, "._") {
			return true
		}
	}
	return false
}

func selectCoverFile(files []*zip.File) *zip.File {
	if len(files) == 0 {
		return nil
	}
	first := files[0]
	firstConfig, firstConfigOK := imageConfig(first)
	firstIsLandscape := firstConfigOK && firstConfig.Width > firstConfig.Height
	firstIsCoverZero := isCoverZeroName(first.Name)
	limit := len(files)
	if limit > 8 {
		limit = 8
	}
	for _, file := range files[1:limit] {
		if !isCoverCandidateName(file.Name) {
			continue
		}
		if firstIsCoverZero && isCoverOneName(file.Name) {
			return file
		}
		config, ok := imageConfig(file)
		if !ok {
			continue
		}
		if config.Height > config.Width && (firstIsLandscape || firstIsCoverZero || !isCoverCandidateName(first.Name)) {
			return file
		}
	}
	return first
}

func imageConfig(file *zip.File) (image.Config, bool) {
	body, err := file.Open()
	if err != nil {
		return image.Config{}, false
	}
	defer body.Close()
	config, _, err := image.DecodeConfig(body)
	if err != nil {
		return image.Config{}, false
	}
	return config, config.Width > 0 && config.Height > 0
}

func isCoverCandidateName(name string) bool {
	base := strings.TrimSuffix(filepath.Base(normalizeEntryName(name)), strings.ToLower(filepath.Ext(name)))
	return strings.Contains(base, "cover") || strings.Contains(base, "front") || strings.Contains(base, "folder")
}

func isCoverZeroName(name string) bool {
	base := strings.TrimSuffix(filepath.Base(normalizeEntryName(name)), strings.ToLower(filepath.Ext(name)))
	return strings.Contains(base, "cover0") || strings.Contains(base, "cover_0") || strings.Contains(base, "cover-0")
}

func isCoverOneName(name string) bool {
	base := strings.TrimSuffix(filepath.Base(normalizeEntryName(name)), strings.ToLower(filepath.Ext(name)))
	return strings.Contains(base, "cover1") || strings.Contains(base, "cover_1") || strings.Contains(base, "cover-1")
}

func contentType(name string) string {
	if value, ok := imageExts[strings.ToLower(filepath.Ext(name))]; ok {
		return value
	}
	return "application/octet-stream"
}

func normalizeEntryName(value string) string {
	return strings.ToLower(strings.ReplaceAll(value, "\\", "/"))
}

type zipPageReadCloser struct {
	io.ReadCloser
	closeReader func() error
}

func (r *zipPageReadCloser) Close() error {
	pageErr := r.ReadCloser.Close()
	readerErr := r.closeReader()
	if pageErr != nil {
		return pageErr
	}
	return readerErr
}
