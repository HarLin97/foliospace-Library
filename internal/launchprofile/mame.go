package launchprofile

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"hash"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

const MAMEPolicy = "mame-0.288-listxml"

func MAMEPolicyForVersion(version string) string {
	version = strings.ToLower(strings.TrimSpace(strings.TrimPrefix(version, "v")))
	if version == "0.288" {
		return MAMEPolicy
	}
	return "mame-" + version + "-listxml"
}

type MAMECatalog struct {
	Build    string
	SHA256   string
	Revision int
	Machines map[string]MAMEMachine
}

type MAMEMachine struct {
	Name        string
	SourceFile  string
	CloneOf     string
	ROMOf       string
	Description string
	IsBIOS      bool
	IsDevice    bool
	Runnable    bool
	BIOSSets    []MAMEBIOSSet
	ROMs        []MAMEROM
	Disks       []MAMEDisk
	DeviceRefs  []string
}

type MAMEBIOSSet struct {
	Name    string
	Default bool
}

type MAMEROM struct {
	Name     string
	BIOS     string
	Merge    string
	Size     int64
	CRC      string
	Status   string
	Optional bool
}

type MAMEDisk struct {
	Name     string
	Merge    string
	SHA1     string
	Status   string
	Optional bool
}

type mameMachineXML struct {
	Name        string          `xml:"name,attr"`
	SourceFile  string          `xml:"sourcefile,attr"`
	CloneOf     string          `xml:"cloneof,attr"`
	ROMOf       string          `xml:"romof,attr"`
	IsBIOS      string          `xml:"isbios,attr"`
	IsDevice    string          `xml:"isdevice,attr"`
	Runnable    string          `xml:"runnable,attr"`
	Description string          `xml:"description"`
	BIOSSets    []mameBIOSXML   `xml:"biosset"`
	ROMs        []mameROMXML    `xml:"rom"`
	Disks       []mameDiskXML   `xml:"disk"`
	DeviceRefs  []mameDeviceXML `xml:"device_ref"`
}

type mameBIOSXML struct {
	Name    string `xml:"name,attr"`
	Default string `xml:"default,attr"`
}

type mameROMXML struct {
	Name     string `xml:"name,attr"`
	BIOS     string `xml:"bios,attr"`
	Merge    string `xml:"merge,attr"`
	Size     string `xml:"size,attr"`
	CRC      string `xml:"crc,attr"`
	Status   string `xml:"status,attr"`
	Optional string `xml:"optional,attr"`
}

type mameDiskXML struct {
	Name     string `xml:"name,attr"`
	Merge    string `xml:"merge,attr"`
	SHA1     string `xml:"sha1,attr"`
	Status   string `xml:"status,attr"`
	Optional string `xml:"optional,attr"`
}

type mameDeviceXML struct {
	Name string `xml:"name,attr"`
}

// ParseMAMEListXMLFile retains only requested machines and their recursive
// dependency closure. The official listxml is over 300 MB, so retaining the
// complete catalog would be inappropriate on small NAS systems.
func ParseMAMEListXMLFile(path string, requested []string) (MAMECatalog, error) {
	wanted := make(map[string]bool, len(requested))
	for _, name := range requested {
		name = normalizeMAMESet(name)
		if name != "" {
			wanted[name] = true
		}
	}
	if len(wanted) == 0 {
		return MAMECatalog{}, fmt.Errorf("MAME listxml selection is empty")
	}

	catalog := MAMECatalog{Machines: make(map[string]MAMEMachine)}
	pending := cloneMAMESet(wanted)
	firstPass := true
	for len(pending) > 0 {
		build, digest, machines, err := parseMAMEListXMLPass(path, pending, firstPass)
		if err != nil {
			return MAMECatalog{}, err
		}
		if firstPass {
			catalog.Build = build
			catalog.SHA256 = digest
			digestBytes, err := hex.DecodeString(digest)
			if err != nil {
				return MAMECatalog{}, fmt.Errorf("decode MAME listxml digest: %w", err)
			}
			catalog.Revision = revisionFromDigest(digestBytes)
			firstPass = false
		}
		for name, machine := range machines {
			catalog.Machines[name] = machine
		}

		next := make(map[string]bool)
		for _, machine := range machines {
			for _, name := range machine.dependencyReferences() {
				if _, loaded := catalog.Machines[name]; !loaded {
					next[name] = true
				}
			}
		}
		pending = next
	}
	if len(catalog.Machines) == 0 {
		return MAMECatalog{}, fmt.Errorf("MAME listxml contains none of the requested machines")
	}
	return catalog, nil
}

func parseMAMEListXMLPass(path string, wanted map[string]bool, hashContents bool) (string, string, map[string]MAMEMachine, error) {
	reader, err := openMAMEListXML(path)
	if err != nil {
		return "", "", nil, err
	}
	defer reader.Close()

	var source io.Reader = reader
	var hasher hash.Hash
	if hashContents {
		hasher = sha256.New()
		source = io.TeeReader(reader, hasher)
	}
	decoder := xml.NewDecoder(source)
	machines := make(map[string]MAMEMachine)
	build := ""
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", "", nil, fmt.Errorf("parse MAME listxml: %w", err)
		}
		start, ok := token.(xml.StartElement)
		if !ok {
			continue
		}
		switch strings.ToLower(start.Name.Local) {
		case "mame":
			for _, attribute := range start.Attr {
				if strings.EqualFold(attribute.Name.Local, "build") {
					build = strings.TrimSpace(attribute.Value)
				}
			}
		case "machine":
			name := ""
			for _, attribute := range start.Attr {
				if strings.EqualFold(attribute.Name.Local, "name") {
					name = normalizeMAMESet(attribute.Value)
					break
				}
			}
			if !wanted[name] {
				if err := decoder.Skip(); err != nil {
					return "", "", nil, fmt.Errorf("skip MAME machine %q: %w", name, err)
				}
				continue
			}
			var parsed mameMachineXML
			if err := decoder.DecodeElement(&parsed, &start); err != nil {
				return "", "", nil, fmt.Errorf("parse MAME machine %q: %w", name, err)
			}
			machine, err := normalizeMAMEMachine(parsed)
			if err != nil {
				return "", "", nil, err
			}
			machines[machine.Name] = machine
		}
	}
	digest := ""
	if hashContents {
		digest = hex.EncodeToString(hasher.Sum(nil))
	}
	return build, digest, machines, nil
}

type mameXMLReadCloser struct {
	io.Reader
	closers []io.Closer
}

func (r *mameXMLReadCloser) Close() error {
	var first error
	for _, closer := range r.closers {
		if err := closer.Close(); err != nil && first == nil {
			first = err
		}
	}
	return first
}

func openMAMEListXML(path string) (io.ReadCloser, error) {
	if !strings.EqualFold(filepath.Ext(path), ".zip") {
		file, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		return file, nil
	}
	archive, err := zip.OpenReader(path)
	if err != nil {
		return nil, fmt.Errorf("open MAME listxml archive: %w", err)
	}
	for _, file := range archive.File {
		if !file.FileInfo().IsDir() && strings.EqualFold(filepath.Ext(file.Name), ".xml") {
			entry, err := file.Open()
			if err != nil {
				_ = archive.Close()
				return nil, fmt.Errorf("open %s from MAME listxml archive: %w", file.Name, err)
			}
			return &mameXMLReadCloser{Reader: entry, closers: []io.Closer{entry, archive}}, nil
		}
	}
	_ = archive.Close()
	return nil, fmt.Errorf("MAME listxml archive contains no XML file")
}

func normalizeMAMEMachine(parsed mameMachineXML) (MAMEMachine, error) {
	machine := MAMEMachine{
		Name: normalizeMAMESet(parsed.Name), SourceFile: strings.ToLower(strings.TrimSpace(parsed.SourceFile)),
		CloneOf: normalizeMAMESet(parsed.CloneOf), ROMOf: normalizeMAMESet(parsed.ROMOf),
		Description: strings.TrimSpace(parsed.Description), IsBIOS: yesValue(parsed.IsBIOS),
		IsDevice: yesValue(parsed.IsDevice), Runnable: !strings.EqualFold(strings.TrimSpace(parsed.Runnable), "no"),
	}
	for _, item := range parsed.BIOSSets {
		machine.BIOSSets = append(machine.BIOSSets, MAMEBIOSSet{Name: strings.ToLower(strings.TrimSpace(item.Name)), Default: yesValue(item.Default)})
	}
	for _, item := range parsed.ROMs {
		size, err := strconv.ParseInt(strings.TrimSpace(item.Size), 10, 64)
		if err != nil {
			return MAMEMachine{}, fmt.Errorf("parse MAME machine %s ROM %s size: %w", machine.Name, item.Name, err)
		}
		machine.ROMs = append(machine.ROMs, MAMEROM{
			Name: item.Name, BIOS: strings.ToLower(strings.TrimSpace(item.BIOS)), Merge: item.Merge,
			Size: size, CRC: strings.ToLower(strings.TrimSpace(item.CRC)), Status: strings.ToLower(strings.TrimSpace(item.Status)),
			Optional: yesValue(item.Optional),
		})
	}
	for _, item := range parsed.Disks {
		machine.Disks = append(machine.Disks, MAMEDisk{
			Name: item.Name, Merge: item.Merge, SHA1: strings.ToLower(strings.TrimSpace(item.SHA1)),
			Status: strings.ToLower(strings.TrimSpace(item.Status)), Optional: yesValue(item.Optional),
		})
	}
	for _, item := range parsed.DeviceRefs {
		if name := normalizeMAMESet(item.Name); name != "" {
			machine.DeviceRefs = append(machine.DeviceRefs, name)
		}
	}
	return machine, nil
}

func (m MAMEMachine) RequiredROMs() []MAMEROM {
	return m.requiredROMs(false)
}

// RequiredSelfContainedROMs includes merged parent ROMs. A clone archive that
// satisfies this set can run without shipping the parent archive separately.
func (m MAMEMachine) RequiredSelfContainedROMs() []MAMEROM {
	return m.requiredROMs(true)
}

func (m MAMEMachine) requiredROMs(includeMerged bool) []MAMEROM {
	defaultBIOS := m.defaultBIOS()
	required := make([]MAMEROM, 0, len(m.ROMs))
	for _, rom := range m.ROMs {
		if (!includeMerged && strings.TrimSpace(rom.Merge) != "") || rom.Optional || strings.EqualFold(rom.Status, "nodump") ||
			strings.TrimSpace(rom.Name) == "" || rom.Size <= 0 || strings.TrimSpace(rom.CRC) == "" {
			continue
		}
		if rom.BIOS != "" && defaultBIOS != "" && rom.BIOS != defaultBIOS {
			continue
		}
		required = append(required, rom)
	}
	return required
}

func (m MAMEMachine) RequiredDisks() []MAMEDisk {
	required := make([]MAMEDisk, 0, len(m.Disks))
	for _, disk := range m.Disks {
		if strings.TrimSpace(disk.Merge) == "" && !disk.Optional && !strings.EqualFold(disk.Status, "nodump") && strings.TrimSpace(disk.Name) != "" {
			required = append(required, disk)
		}
	}
	return required
}

func (m MAMEMachine) defaultBIOS() string {
	for _, bios := range m.BIOSSets {
		if bios.Default {
			return bios.Name
		}
	}
	if len(m.BIOSSets) > 0 {
		return m.BIOSSets[0].Name
	}
	return ""
}

func (m MAMEMachine) dependencyReferences() []string {
	refs := make([]string, 0, len(m.DeviceRefs)+1)
	if m.ROMOf != "" {
		refs = append(refs, m.ROMOf)
	}
	refs = append(refs, m.DeviceRefs...)
	return uniqueMAMESets(refs)
}

func (c MAMECatalog) Dependencies(setName string) ([]MAMEMachine, error) {
	root := normalizeMAMESet(setName)
	if _, ok := c.Machines[root]; !ok {
		return nil, fmt.Errorf("set %q is absent from selected MAME listxml catalog", root)
	}
	seen := map[string]bool{root: true}
	active := make(map[string]bool)
	result := make([]MAMEMachine, 0, 4)
	var visit func(string) error
	visit = func(name string) error {
		if active[name] {
			return fmt.Errorf("cyclic MAME dependency at %q", name)
		}
		if seen[name] {
			return nil
		}
		machine, ok := c.Machines[name]
		if !ok {
			return fmt.Errorf("MAME dependency %q is absent from selected catalog", name)
		}
		active[name] = true
		for _, nested := range machine.dependencyReferences() {
			if err := visit(nested); err != nil {
				return err
			}
		}
		delete(active, name)
		seen[name] = true
		if len(machine.RequiredROMs()) > 0 || len(machine.RequiredDisks()) > 0 {
			result = append(result, machine)
		}
		return nil
	}
	rootMachine := c.Machines[root]
	for _, name := range rootMachine.dependencyReferences() {
		if err := visit(name); err != nil {
			return nil, err
		}
	}
	return result, nil
}

func ValidateMAMEArchive(path string, machine MAMEMachine) error {
	return validateMAMEArchiveROMs(path, machine, machine.RequiredROMs())
}

// ValidateMAMESelfContainedArchive verifies a non-merged clone package. It is
// stricter than the normal split-set audit because merged parent ROMs must also
// be present in the clone archive.
func ValidateMAMESelfContainedArchive(path string, machine MAMEMachine) error {
	return validateMAMEArchiveROMs(path, machine, machine.RequiredSelfContainedROMs())
}

func validateMAMEArchiveROMs(path string, machine MAMEMachine, requiredROMs []MAMEROM) error {
	if disks := machine.RequiredDisks(); len(disks) > 0 {
		return fmt.Errorf("%s requires %d CHD disk image(s), which ZIP-only audit cannot satisfy", machine.Name, len(disks))
	}
	reader, err := zip.OpenReader(path)
	if err != nil {
		return fmt.Errorf("open %s: %w", filepath.Base(path), err)
	}
	defer reader.Close()
	entries := make(map[string]*zip.File, len(reader.File))
	entriesByFingerprint := make(map[string]*zip.File, len(reader.File))
	for _, file := range reader.File {
		entries[strings.ToLower(filepath.Base(strings.ReplaceAll(file.Name, "\\", "/")))] = file
		entriesByFingerprint[mameROMFingerprint(int64(file.UncompressedSize64), file.CRC32)] = file
	}
	for _, rom := range requiredROMs {
		entry := entries[strings.ToLower(filepath.Base(strings.ReplaceAll(rom.Name, "\\", "/")))]
		if entry == nil {
			entry = entriesByFingerprint[mameROMFingerprint(rom.Size, parseMAMECRC(rom.CRC))]
			if entry == nil {
				return fmt.Errorf("%s is missing ROM %s", filepath.Base(path), rom.Name)
			}
		}
		if int64(entry.UncompressedSize64) != rom.Size || entry.CRC32 != parseMAMECRC(rom.CRC) {
			return fmt.Errorf("%s ROM %s does not match MAME listxml size/CRC", filepath.Base(path), rom.Name)
		}
	}
	return nil
}

func mameROMFingerprint(size int64, crc uint32) string {
	return fmt.Sprintf("%d:%08x", size, crc)
}

func parseMAMECRC(value string) uint32 {
	parsed, _ := strconv.ParseUint(strings.TrimSpace(value), 16, 32)
	return uint32(parsed)
}

func yesValue(value string) bool {
	return strings.EqualFold(strings.TrimSpace(value), "yes")
}

func normalizeMAMESet(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func cloneMAMESet(input map[string]bool) map[string]bool {
	result := make(map[string]bool, len(input))
	for name := range input {
		result[name] = true
	}
	return result
}

func uniqueMAMESets(values []string) []string {
	seen := make(map[string]bool, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = normalizeMAMESet(value)
		if value != "" && !seen[value] {
			seen[value] = true
			result = append(result, value)
		}
	}
	sort.Strings(result)
	return result
}
