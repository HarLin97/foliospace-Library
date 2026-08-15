package scanner

import (
	stdzip "archive/zip"
	"bytes"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash/crc32"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

const (
	threeDSImageMaxBytes     = uint64(8 << 30)
	threeDSArchiveMaxEntries = 4096
	threeDSArchiveMaxRatio   = uint64(200)
	threeDSHeaderOffset      = 0x100
	threeDSHeaderSize        = 4
)

type threeDSImageInfo struct {
	name      string
	format    string
	size      int64
	checksums checksumPair
	install   bool
}

func inspectThreeDSImage(path string, info fs.FileInfo, ext string) (threeDSImageInfo, error) {
	ext = strings.ToLower(strings.TrimSpace(ext))
	if info.Size() <= 0 || uint64(info.Size()) > threeDSImageMaxBytes {
		return threeDSImageInfo{}, fmt.Errorf("invalid Nintendo 3DS image size %d", info.Size())
	}
	if ext == ".cia" {
		checksums, err := fileChecksums(path)
		if err != nil {
			return threeDSImageInfo{}, err
		}
		return threeDSImageInfo{
			name: filepath.Base(path), format: "cia", size: info.Size(), checksums: checksums, install: true,
		}, nil
	}
	if ext == ".zip" {
		return inspectThreeDSZIP(path)
	}
	if !isThreeDSDirectImageExt(ext) {
		return threeDSImageInfo{}, fmt.Errorf("unsupported Nintendo 3DS format %s", ext)
	}

	file, err := os.Open(path)
	if err != nil {
		return threeDSImageInfo{}, err
	}
	defer file.Close()
	checksums, err := validateAndChecksumThreeDSImage(file, uint64(info.Size()), ext)
	if err != nil {
		return threeDSImageInfo{}, err
	}
	return threeDSImageInfo{
		name: filepath.Base(path), format: strings.TrimPrefix(ext, "."), size: info.Size(), checksums: checksums,
	}, nil
}

func inspectThreeDSZIP(path string) (threeDSImageInfo, error) {
	reader, err := stdzip.OpenReader(path)
	if err != nil {
		return threeDSImageInfo{}, fmt.Errorf("open Nintendo 3DS zip: %w", err)
	}
	defer reader.Close()
	if len(reader.File) > threeDSArchiveMaxEntries {
		return threeDSImageInfo{}, fmt.Errorf("Nintendo 3DS zip has %d entries, limit is %d", len(reader.File), threeDSArchiveMaxEntries)
	}

	seen := make(map[string]struct{}, len(reader.File))
	var total uint64
	var candidate *stdzip.File
	for _, file := range reader.File {
		key, err := validateThreeDSArchiveEntry(file)
		if err != nil {
			return threeDSImageInfo{}, err
		}
		if _, exists := seen[key]; exists {
			return threeDSImageInfo{}, fmt.Errorf("Nintendo 3DS zip contains duplicate path %q", file.Name)
		}
		seen[key] = struct{}{}

		if file.UncompressedSize64 > threeDSImageMaxBytes || total > threeDSImageMaxBytes-file.UncompressedSize64 {
			return threeDSImageInfo{}, fmt.Errorf("Nintendo 3DS zip uncompressed size exceeds %d bytes", threeDSImageMaxBytes)
		}
		total += file.UncompressedSize64
		if file.UncompressedSize64 > 0 {
			if file.CompressedSize64 == 0 || file.UncompressedSize64/file.CompressedSize64 > threeDSArchiveMaxRatio {
				return threeDSImageInfo{}, fmt.Errorf("Nintendo 3DS zip entry %q exceeds compression ratio limit", file.Name)
			}
		}
		if file.FileInfo().IsDir() || isIgnoredArchiveEntry(file.Name) {
			continue
		}
		if !isThreeDSDirectImageExt(strings.ToLower(filepath.Ext(file.Name))) {
			continue
		}
		if candidate != nil {
			return threeDSImageInfo{}, fmt.Errorf("Nintendo 3DS zip requires exactly one image candidate")
		}
		candidate = file
	}
	if candidate == nil {
		return threeDSImageInfo{}, fmt.Errorf("Nintendo 3DS zip contains no launchable image")
	}
	if candidate.UncompressedSize64 == 0 || candidate.UncompressedSize64 > threeDSImageMaxBytes {
		return threeDSImageInfo{}, fmt.Errorf("invalid Nintendo 3DS image size %d", candidate.UncompressedSize64)
	}
	body, err := candidate.Open()
	if err != nil {
		return threeDSImageInfo{}, fmt.Errorf("open Nintendo 3DS image %q: %w", candidate.Name, err)
	}
	defer body.Close()
	imageExt := strings.ToLower(filepath.Ext(candidate.Name))
	checksums, err := validateAndChecksumThreeDSImage(body, candidate.UncompressedSize64, imageExt)
	if err != nil {
		return threeDSImageInfo{}, fmt.Errorf("validate Nintendo 3DS image %q: %w", candidate.Name, err)
	}
	return threeDSImageInfo{
		name:   filepath.Base(filepath.Clean(strings.ReplaceAll(candidate.Name, `\`, "/"))),
		format: "zip", size: int64(candidate.UncompressedSize64), checksums: checksums,
	}, nil
}

func validateThreeDSArchiveEntry(file *stdzip.File) (string, error) {
	if strings.ContainsRune(file.Name, '\x00') || strings.Contains(file.Name, `\`) {
		return "", fmt.Errorf("unsafe Nintendo 3DS zip entry %q", file.Name)
	}
	if err := validateArchiveEntryName(file.Name); err != nil {
		return "", err
	}
	if file.Mode()&os.ModeSymlink != 0 || file.Flags&0x1 != 0 {
		return "", fmt.Errorf("unsupported Nintendo 3DS zip entry %q", file.Name)
	}
	normalized := strings.Trim(strings.ReplaceAll(file.Name, `\`, "/"), "/")
	return strings.ToLower(filepath.ToSlash(filepath.Clean(normalized))), nil
}

func validateAndChecksumThreeDSImage(reader io.Reader, declaredSize uint64, ext string) (checksumPair, error) {
	if declaredSize < threeDSHeaderOffset+threeDSHeaderSize || declaredSize > threeDSImageMaxBytes {
		return checksumPair{}, fmt.Errorf("invalid Nintendo 3DS image size %d", declaredSize)
	}
	expected, ok := threeDSMagicForExt(ext)
	if !ok {
		return checksumPair{}, fmt.Errorf("unsupported Nintendo 3DS image extension %s", ext)
	}

	crc := crc32.NewIEEE()
	sha := sha1.New()
	sha256Hash := sha256.New()
	stream := io.TeeReader(io.LimitReader(reader, int64(declaredSize)+1), io.MultiWriter(crc, sha, sha256Hash))
	header := make([]byte, threeDSHeaderOffset+threeDSHeaderSize)
	if _, err := io.ReadFull(stream, header); err != nil {
		return checksumPair{}, fmt.Errorf("read Nintendo 3DS image header: %w", err)
	}
	actual := header[threeDSHeaderOffset : threeDSHeaderOffset+threeDSHeaderSize]
	if !bytes.Equal(actual, expected) {
		return checksumPair{}, fmt.Errorf("Nintendo 3DS image header is %q, expected %q", actual, expected)
	}
	written, err := io.Copy(io.Discard, stream)
	if err != nil {
		return checksumPair{}, err
	}
	if uint64(written)+uint64(len(header)) != declaredSize {
		return checksumPair{}, fmt.Errorf("Nintendo 3DS image size mismatch: read %d, expected %d", written+int64(len(header)), declaredSize)
	}
	return checksumPair{
		crc32: fmt.Sprintf("%08x", crc.Sum32()), sha1: hex.EncodeToString(sha.Sum(nil)),
		sha256: hex.EncodeToString(sha256Hash.Sum(nil)),
	}, nil
}

func isThreeDSDirectImageExt(ext string) bool {
	switch strings.ToLower(strings.TrimSpace(ext)) {
	case ".3ds", ".cci", ".cxi":
		return true
	default:
		return false
	}
}

func threeDSMagicForExt(ext string) ([]byte, bool) {
	switch strings.ToLower(strings.TrimSpace(ext)) {
	case ".3ds", ".cci":
		return []byte("NCSD"), true
	case ".cxi":
		return []byte("NCCH"), true
	default:
		return nil, false
	}
}
