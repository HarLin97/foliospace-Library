package launchprofile

import (
	"archive/zip"
	"fmt"
	"hash/crc32"
	"os"
	"path/filepath"
	"testing"
)

func TestParseMAMEListXMLAndValidateDependencyClosure(t *testing.T) {
	cloneBytes := []byte("clone-rom")
	parentBytes := []byte("parent-rom")
	billBytes := []byte("bill-rom")
	xmlText := fmt.Sprintf(`<?xml version="1.0"?><mame build="0.288 (mame0288)" mameconfig="10">
		<machine name="parent" sourcefile="sega/model2.cpp"><description>Parent</description>
			<rom name="parent.bin" size="%d" crc="%08x" status="good" optional="no"/>
		</machine>
		<machine name="segabill" sourcefile="sega/segabill.cpp" isdevice="yes" runnable="no"><description>Bill Board</description>
			<rom name="bill.bin" size="%d" crc="%08x" status="good" optional="no"/>
		</machine>
		<machine name="clone" sourcefile="sega/model2.cpp" cloneof="parent" romof="parent"><description>Clone</description>
			<rom name="clone.bin" size="%d" crc="%08x" status="good" optional="no"/>
			<rom name="parent.bin" merge="parent.bin" size="%d" crc="%08x" status="good" optional="no"/>
			<device_ref name="segabill"/>
		</machine></mame>`, len(parentBytes), crc32.ChecksumIEEE(parentBytes), len(billBytes), crc32.ChecksumIEEE(billBytes),
		len(cloneBytes), crc32.ChecksumIEEE(cloneBytes), len(parentBytes), crc32.ChecksumIEEE(parentBytes))

	dir := t.TempDir()
	listPath := filepath.Join(dir, "mame0288lx.zip")
	writeMAMEXMLArchive(t, listPath, xmlText)
	catalog, err := ParseMAMEListXMLFile(listPath, []string{"clone"})
	if err != nil {
		t.Fatal(err)
	}
	if catalog.Build != "0.288 (mame0288)" || catalog.Revision <= 0 || len(catalog.Machines) != 3 {
		t.Fatalf("unexpected catalog: %+v", catalog)
	}
	clone := catalog.Machines["clone"]
	if len(clone.RequiredROMs()) != 1 || clone.RequiredROMs()[0].Name != "clone.bin" {
		t.Fatalf("unexpected clone ROMs: %+v", clone.RequiredROMs())
	}
	dependencies, err := catalog.Dependencies("clone")
	if err != nil {
		t.Fatal(err)
	}
	if len(dependencies) != 2 || dependencies[0].Name != "parent" || dependencies[1].Name != "segabill" {
		t.Fatalf("unexpected dependencies: %+v", dependencies)
	}

	clonePath := filepath.Join(dir, "clone.zip")
	writeTestZip(t, clonePath, map[string][]byte{"clone.bin": cloneBytes})
	if err := ValidateMAMEArchive(clonePath, clone); err != nil {
		t.Fatal(err)
	}
	parentPath := filepath.Join(dir, "parent.zip")
	writeTestZip(t, parentPath, map[string][]byte{"parent.bin": parentBytes})
	if err := ValidateMAMEArchive(parentPath, dependencies[0]); err != nil {
		t.Fatal(err)
	}
	billPath := filepath.Join(dir, "segabill.zip")
	writeTestZip(t, billPath, map[string][]byte{"bill.bin": billBytes})
	if err := ValidateMAMEArchive(billPath, dependencies[1]); err != nil {
		t.Fatal(err)
	}
}

func TestMAMERequiredROMsUsesDefaultBIOSAndRejectsWrongCRC(t *testing.T) {
	machine := MAMEMachine{
		Name:     "biosgame",
		BIOSSets: []MAMEBIOSSet{{Name: "new", Default: true}, {Name: "old"}},
		ROMs: []MAMEROM{
			{Name: "new.bin", BIOS: "new", Size: 3, CRC: fmt.Sprintf("%08x", crc32.ChecksumIEEE([]byte("new")))},
			{Name: "old.bin", BIOS: "old", Size: 3, CRC: fmt.Sprintf("%08x", crc32.ChecksumIEEE([]byte("old")))},
		},
	}
	required := machine.RequiredROMs()
	if len(required) != 1 || required[0].Name != "new.bin" {
		t.Fatalf("unexpected BIOS selection: %+v", required)
	}
	path := filepath.Join(t.TempDir(), "biosgame.zip")
	writeTestZip(t, path, map[string][]byte{"new.bin": []byte("bad")})
	if err := ValidateMAMEArchive(path, machine); err == nil {
		t.Fatal("expected CRC mismatch")
	}
}

func TestValidateMAMEArchiveAcceptsCRCMatchedHistoricalAlias(t *testing.T) {
	data := []byte("renamed-rom")
	machine := MAMEMachine{
		Name: "vstriker",
		ROMs: []MAMEROM{{
			Name: "epr-18068a.15", Size: int64(len(data)), CRC: fmt.Sprintf("%08x", crc32.ChecksumIEEE(data)),
		}},
	}
	path := filepath.Join(t.TempDir(), "vstriker.zip")
	writeTestZip(t, path, map[string][]byte{"ep18068a.15": data})
	if err := ValidateMAMEArchive(path, machine); err != nil {
		t.Fatal(err)
	}
}

func writeMAMEXMLArchive(t *testing.T, path, xmlText string) {
	t.Helper()
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(file)
	entry, err := writer.Create("mame0288.xml")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := entry.Write([]byte(xmlText)); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}
