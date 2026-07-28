package main

import (
	"archive/zip"
	"fmt"
	"hash/crc32"
	"os"
	"path/filepath"
	"testing"

	"foliospace-reader/internal/domain"
	"foliospace-reader/internal/launchprofile"
)

func TestBuildMAMEProfileUsesAuditedModel2Set(t *testing.T) {
	rom := []byte("model2-rom")
	path := filepath.Join(t.TempDir(), "vf2.zip")
	writeMAMETestArchive(t, path, map[string][]byte{"epr-test.ic1": rom})
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	machine := launchprofile.MAMEMachine{
		Name:     "vf2",
		Runnable: true,
		ROMs: []launchprofile.MAMEROM{{
			Name: "epr-test.ic1", Size: int64(len(rom)), CRC: fmt.Sprintf("%08x", crc32.ChecksumIEEE(rom)),
		}},
	}
	catalog := launchprofile.MAMECatalog{
		Build: "0.288 (mame0288)", SHA256: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		Revision: 288, Machines: map[string]launchprofile.MAMEMachine{"vf2": machine},
	}
	entry := domain.GameAsset{
		ID: 42, Platform: "model2", FilePath: path, Size: info.Size(),
		SHA1: "0123456789abcdef0123456789abcdef01234567",
	}

	profile, err := buildMAMEProfile(catalog, machine, entry, map[string][]domain.GameAsset{"vf2": {entry}})
	if err != nil {
		t.Fatal(err)
	}
	if profile.Policy != launchprofile.MAMEPolicy || profile.Runtime.ID != "mame" || profile.Runtime.Version != "0.288" || profile.Runtime.ContentSet != "mame-0.288" {
		t.Fatalf("unexpected runtime profile: %+v", profile)
	}
	if profile.CanonicalSet != "vf2" || profile.EntryFile != "vf2.zip" || len(profile.Files) != 1 || profile.Files[0].Role != "entry" {
		t.Fatalf("unexpected profile files: %+v", profile)
	}
}

func writeMAMETestArchive(t *testing.T, path string, files map[string][]byte) {
	t.Helper()
	archive, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(archive)
	for name, data := range files {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
}
