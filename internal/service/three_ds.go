package service

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const (
	threeDSDownloadMaxBytes     = uint64(8 << 30)
	threeDSDownloadMaxEntries   = 4096
	threeDSDownloadHeaderOffset = 0x100
	threeDSDownloadHeaderSize   = 4
)

func isZippedThreeDSImage(gamePlatform, gamePath, gameFormat string) bool {
	return strings.EqualFold(strings.TrimSpace(gamePlatform), "3ds") &&
		strings.EqualFold(filepath.Ext(gamePath), ".zip") &&
		strings.EqualFold(strings.TrimSpace(gameFormat), "zip")
}

func openThreeDSImageFromZIP(path, expectedName string, expectedSize int64) (io.ReadCloser, error) {
	reader, err := zip.OpenReader(path)
	if err != nil {
		return nil, err
	}
	if len(reader.File) > threeDSDownloadMaxEntries {
		_ = reader.Close()
		return nil, fmt.Errorf("Nintendo 3DS zip has too many entries")
	}

	seen := make(map[string]struct{}, len(reader.File))
	var match *zip.File
	var total uint64
	for _, file := range reader.File {
		name := file.Name
		if strings.ContainsRune(name, '\x00') || strings.Contains(name, `\`) || filepath.IsAbs(name) || filepath.VolumeName(name) != "" {
			_ = reader.Close()
			return nil, fmt.Errorf("unsafe Nintendo 3DS zip entry %q", name)
		}
		clean := filepath.ToSlash(filepath.Clean(name))
		if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
			_ = reader.Close()
			return nil, fmt.Errorf("unsafe Nintendo 3DS zip entry %q", name)
		}
		key := strings.ToLower(clean)
		if _, exists := seen[key]; exists {
			_ = reader.Close()
			return nil, fmt.Errorf("Nintendo 3DS zip contains duplicate path %q", name)
		}
		seen[key] = struct{}{}
		if file.Mode()&os.ModeSymlink != 0 || file.Flags&0x1 != 0 {
			_ = reader.Close()
			return nil, fmt.Errorf("unsupported Nintendo 3DS zip entry %q", name)
		}
		if file.UncompressedSize64 > threeDSDownloadMaxBytes || total > threeDSDownloadMaxBytes-file.UncompressedSize64 {
			_ = reader.Close()
			return nil, fmt.Errorf("Nintendo 3DS zip uncompressed size exceeds %d bytes", threeDSDownloadMaxBytes)
		}
		total += file.UncompressedSize64
		if file.UncompressedSize64 > 0 && (file.CompressedSize64 == 0 || file.UncompressedSize64/file.CompressedSize64 > 200) {
			_ = reader.Close()
			return nil, fmt.Errorf("Nintendo 3DS zip entry %q exceeds compression ratio limit", name)
		}
		if file.FileInfo().IsDir() || isIgnoredGameArchiveEntry(name) || !isThreeDSDownloadImageExt(filepath.Ext(name)) {
			continue
		}
		if match != nil {
			_ = reader.Close()
			return nil, fmt.Errorf("Nintendo 3DS zip contains multiple image entries")
		}
		match = file
	}
	if match == nil || (expectedName != "" && !strings.EqualFold(filepath.Base(match.Name), filepath.Base(expectedName))) {
		_ = reader.Close()
		return nil, fmt.Errorf("Nintendo 3DS image entry %q not found", expectedName)
	}
	if expectedSize <= 0 || uint64(expectedSize) > threeDSDownloadMaxBytes || int64(match.UncompressedSize64) != expectedSize {
		_ = reader.Close()
		return nil, fmt.Errorf("Nintendo 3DS image entry size is %d, expected %d", match.UncompressedSize64, expectedSize)
	}

	body, err := match.Open()
	if err != nil {
		_ = reader.Close()
		return nil, err
	}
	header := make([]byte, threeDSDownloadHeaderOffset+threeDSDownloadHeaderSize)
	if _, err := io.ReadFull(body, header); err != nil {
		_ = body.Close()
		_ = reader.Close()
		return nil, fmt.Errorf("read Nintendo 3DS image header: %w", err)
	}
	expectedMagic, ok := threeDSDownloadMagic(filepath.Ext(match.Name))
	if !ok || !bytes.Equal(header[threeDSDownloadHeaderOffset:], expectedMagic) {
		_ = body.Close()
		_ = reader.Close()
		return nil, fmt.Errorf("Nintendo 3DS image header does not match its extension")
	}
	stream := io.NopCloser(io.MultiReader(bytes.NewReader(header), body))
	return cleanupReadCloser{ReadCloser: stream, cleanup: func() {
		_ = body.Close()
		_ = reader.Close()
	}}, nil
}

func isThreeDSDownloadImageExt(ext string) bool {
	switch strings.ToLower(strings.TrimSpace(ext)) {
	case ".3ds", ".cci", ".cxi":
		return true
	default:
		return false
	}
}

func threeDSDownloadMagic(ext string) ([]byte, bool) {
	switch strings.ToLower(strings.TrimSpace(ext)) {
	case ".3ds", ".cci":
		return []byte("NCSD"), true
	case ".cxi":
		return []byte("NCCH"), true
	default:
		return nil, false
	}
}
