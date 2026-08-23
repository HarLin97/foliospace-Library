package scanner

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInspectThreeDSCIARequiresStructuralHeader(t *testing.T) {
	path := filepath.Join(t.TempDir(), "Install Package.cia")
	if err := os.WriteFile(path, []byte("cia-install-package"), 0o644); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := inspectThreeDSImage(path, info, ".cia"); err == nil || !strings.Contains(err.Error(), "CIA") {
		t.Fatalf("inspect random CIA error = %v, want structural rejection", err)
	}
}

func TestInspectThreeDSCIAAcceptsBoundedLayoutAndChecksumsBytes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "Install Package.cia")
	body := syntheticCIAForTest()
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	image, err := inspectThreeDSImage(path, info, ".cia")
	if err != nil {
		t.Fatal(err)
	}
	if image.name != "Install Package.cia" || image.format != "cia" || image.size != int64(len(body)) || !image.install {
		t.Fatalf("CIA image = %#v", image)
	}
	if len(image.checksums.sha1) != 40 || len(image.checksums.sha256) != 64 || len(image.checksums.crc32) != 8 {
		t.Fatalf("CIA checksums = %#v", image.checksums)
	}
}

func TestInspectThreeDSCIARejectsTruncatedDeclaredSections(t *testing.T) {
	body := syntheticCIAForTest()
	binary.LittleEndian.PutUint64(body[0x18:0x20], uint64(len(body)))
	path := filepath.Join(t.TempDir(), "Truncated.cia")
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := inspectThreeDSImage(path, info, ".cia"); err == nil || !strings.Contains(err.Error(), "exceed file size") {
		t.Fatalf("inspect truncated CIA error = %v", err)
	}
}

func syntheticCIAForTest() []byte {
	const (
		headerSize  = 0x2020
		certSize    = 0x80
		ticketSize  = 0x80
		tmdSize     = 0x80
		contentSize = 0x100
	)
	header := make([]byte, headerSize)
	binary.LittleEndian.PutUint32(header[0:4], headerSize)
	binary.LittleEndian.PutUint32(header[8:12], certSize)
	binary.LittleEndian.PutUint32(header[12:16], ticketSize)
	binary.LittleEndian.PutUint32(header[16:20], tmdSize)
	binary.LittleEndian.PutUint64(header[24:32], contentSize)
	header[32] = 0x80

	result := append([]byte(nil), header...)
	result = appendAlignedCIASectionForTest(result, certSize, 0x11)
	result = appendAlignedCIASectionForTest(result, ticketSize, 0x22)
	result = appendAlignedCIASectionForTest(result, tmdSize, 0x33)
	result = append(result, make([]byte, contentSize)...)
	return result
}

func appendAlignedCIASectionForTest(body []byte, size int, fill byte) []byte {
	for len(body)%0x40 != 0 {
		body = append(body, 0)
	}
	section := make([]byte, size)
	for index := range section {
		section[index] = fill
	}
	return append(body, section...)
}
