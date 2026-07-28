package launchprofile

import (
	"archive/zip"
	"bytes"
	"fmt"
	"hash/crc32"
	"os"
	"path/filepath"
	"testing"
)

func TestParseFBNeoDATAndValidateSplitClone(t *testing.T) {
	cloneBytes := []byte("clone-rom")
	parentBytes := []byte("parent-rom")
	dat := fmt.Sprintf(`<?xml version="1.0"?><datafile><header><name>FinalBurn Neo - Arcade Games</name><version>1.0.0.03</version></header>
		<game name="clone" cloneof="parent" romof="parent" sourcefile="capcom/d_cps1.cpp">
			<description>Clone Game</description>
			<rom name="clone.bin" size="%d" crc="%08x"/>
			<rom name="parent.bin" merge="parent.bin" size="%d" crc="%08x"/>
		</game>
		<game name="parent" sourcefile="capcom/d_cps1.cpp">
			<description>Parent Game</description>
			<rom name="parent.bin" size="%d" crc="%08x"/>
		</game></datafile>`, len(cloneBytes), crc32.ChecksumIEEE(cloneBytes), len(parentBytes), crc32.ChecksumIEEE(parentBytes), len(parentBytes), crc32.ChecksumIEEE(parentBytes))
	catalog, err := ParseFBNeoDAT(bytes.NewBufferString(dat))
	if err != nil {
		t.Fatal(err)
	}
	if catalog.Version != "1.0.0.03" || catalog.Revision <= 0 || len(catalog.Games) != 2 {
		t.Fatalf("unexpected catalog: %+v", catalog)
	}
	clone := catalog.Games["clone"]
	if clone.Platform() != "cps1" || len(clone.RequiredROMs()) != 1 {
		t.Fatalf("unexpected clone: %+v", clone)
	}
	dependencies, err := catalog.Dependencies("clone")
	if err != nil || len(dependencies) != 1 || dependencies[0].Name != "parent" {
		t.Fatalf("unexpected dependencies: %+v, %v", dependencies, err)
	}

	dir := t.TempDir()
	clonePath := filepath.Join(dir, "clone.zip")
	writeTestZip(t, clonePath, map[string][]byte{"clone.bin": cloneBytes})
	if err := ValidateFBNeoArchive(clonePath, clone); err != nil {
		t.Fatal(err)
	}
	parentPath := filepath.Join(dir, "parent.zip")
	writeTestZip(t, parentPath, map[string][]byte{"parent.bin": parentBytes})
	if err := ValidateFBNeoArchive(parentPath, dependencies[0]); err != nil {
		t.Fatal(err)
	}
}

func TestValidateFBNeoArchiveRejectsWrongCRC(t *testing.T) {
	game := FBNeoGame{Name: "bad", ROMs: []FBNeoROM{{Name: "rom.bin", Size: 3, CRC: "12345678"}}}
	path := filepath.Join(t.TempDir(), "bad.zip")
	writeTestZip(t, path, map[string][]byte{"rom.bin": []byte("bad")})
	if err := ValidateFBNeoArchive(path, game); err == nil {
		t.Fatal("expected CRC mismatch")
	}
}

func writeTestZip(t *testing.T, path string, files map[string][]byte) {
	t.Helper()
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(file)
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
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}
