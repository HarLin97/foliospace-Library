package launchprofile

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const FBNeoPolicy = "fbneo-arcade-dat"

type FBNeoCatalog struct {
	Name     string
	Version  string
	SHA256   string
	Revision int
	Games    map[string]FBNeoGame
}

type FBNeoGame struct {
	Name        string
	CloneOf     string
	ROMOf       string
	SourceFile  string
	Description string
	ROMs        []FBNeoROM
}

type FBNeoROM struct {
	Name   string
	Merge  string
	Size   int64
	CRC    string
	Status string
}

type fbNeoHeader struct {
	Name    string `xml:"name"`
	Version string `xml:"version"`
}

type fbNeoGameXML struct {
	Name        string     `xml:"name,attr"`
	CloneOf     string     `xml:"cloneof,attr"`
	ROMOf       string     `xml:"romof,attr"`
	SourceFile  string     `xml:"sourcefile,attr"`
	Description string     `xml:"description"`
	ROMs        []FBNeoROM `xml:"rom"`
}

func (r *FBNeoROM) UnmarshalXML(decoder *xml.Decoder, start xml.StartElement) error {
	for _, attribute := range start.Attr {
		switch strings.ToLower(attribute.Name.Local) {
		case "name":
			r.Name = attribute.Value
		case "merge":
			r.Merge = attribute.Value
		case "size":
			size, err := strconv.ParseInt(attribute.Value, 10, 64)
			if err != nil {
				return fmt.Errorf("parse ROM size %q: %w", attribute.Value, err)
			}
			r.Size = size
		case "crc":
			r.CRC = strings.ToLower(attribute.Value)
		case "status":
			r.Status = strings.ToLower(attribute.Value)
		}
	}
	return decoder.Skip()
}

func ParseFBNeoDATFile(path string) (FBNeoCatalog, error) {
	file, err := os.Open(path)
	if err != nil {
		return FBNeoCatalog{}, err
	}
	defer file.Close()
	return ParseFBNeoDAT(file)
}

// ParseFBNeoDAT streams the XML because the official arcade DAT is large and
// this command is intended to remain safe on small NAS systems.
func ParseFBNeoDAT(reader io.Reader) (FBNeoCatalog, error) {
	hasher := sha256.New()
	decoder := xml.NewDecoder(io.TeeReader(reader, hasher))
	catalog := FBNeoCatalog{Games: make(map[string]FBNeoGame)}
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return FBNeoCatalog{}, fmt.Errorf("parse FBNeo DAT: %w", err)
		}
		start, ok := token.(xml.StartElement)
		if !ok {
			continue
		}
		switch strings.ToLower(start.Name.Local) {
		case "header":
			var header fbNeoHeader
			if err := decoder.DecodeElement(&header, &start); err != nil {
				return FBNeoCatalog{}, fmt.Errorf("parse FBNeo DAT header: %w", err)
			}
			catalog.Name = strings.TrimSpace(header.Name)
			catalog.Version = strings.TrimSpace(header.Version)
		case "game", "machine":
			var parsed fbNeoGameXML
			if err := decoder.DecodeElement(&parsed, &start); err != nil {
				return FBNeoCatalog{}, fmt.Errorf("parse FBNeo DAT game: %w", err)
			}
			name := strings.ToLower(strings.TrimSpace(parsed.Name))
			if name == "" {
				continue
			}
			catalog.Games[name] = FBNeoGame{
				Name: name, CloneOf: strings.ToLower(strings.TrimSpace(parsed.CloneOf)),
				ROMOf:       strings.ToLower(strings.TrimSpace(parsed.ROMOf)),
				SourceFile:  strings.ToLower(strings.TrimSpace(parsed.SourceFile)),
				Description: strings.TrimSpace(parsed.Description), ROMs: parsed.ROMs,
			}
		}
	}
	digest := hasher.Sum(nil)
	catalog.SHA256 = hex.EncodeToString(digest)
	catalog.Revision = revisionFromDigest(digest)
	if len(catalog.Games) == 0 {
		return FBNeoCatalog{}, fmt.Errorf("FBNeo DAT contains no games")
	}
	return catalog, nil
}

func revisionFromDigest(digest []byte) int {
	if len(digest) < 4 {
		return 1
	}
	revision := int(binary.BigEndian.Uint32(digest[:4]) & 0x7fffffff)
	if revision == 0 {
		return 1
	}
	return revision
}

func (c FBNeoCatalog) Dependencies(setName string) ([]FBNeoGame, error) {
	setName = strings.ToLower(strings.TrimSpace(setName))
	seen := map[string]bool{setName: true}
	dependencies := make([]FBNeoGame, 0, 2)
	current, ok := c.Games[setName]
	if !ok {
		return nil, fmt.Errorf("set %q is absent from FBNeo DAT", setName)
	}
	for strings.TrimSpace(current.ROMOf) != "" {
		name := strings.ToLower(strings.TrimSpace(current.ROMOf))
		if seen[name] {
			return nil, fmt.Errorf("cyclic romof chain at %q", name)
		}
		dependency, ok := c.Games[name]
		if !ok {
			return nil, fmt.Errorf("romof dependency %q is absent from FBNeo DAT", name)
		}
		seen[name] = true
		dependencies = append(dependencies, dependency)
		current = dependency
	}
	return dependencies, nil
}

func (g FBNeoGame) Platform() string {
	source := strings.ToLower(strings.TrimSpace(g.SourceFile))
	switch {
	case strings.Contains(source, "d_cps1"):
		return "cps1"
	case strings.Contains(source, "d_cps2"):
		return "cps2"
	case strings.Contains(source, "d_cps3"):
		return "cps3"
	case strings.Contains(source, "neogeo"):
		return "neogeo"
	default:
		return "arcade"
	}
}

func (g FBNeoGame) RequiredROMs() []FBNeoROM {
	required := make([]FBNeoROM, 0, len(g.ROMs))
	for _, rom := range g.ROMs {
		if strings.TrimSpace(rom.Merge) != "" || strings.EqualFold(rom.Status, "nodump") ||
			strings.TrimSpace(rom.Name) == "" || rom.Size <= 0 || strings.TrimSpace(rom.CRC) == "" {
			continue
		}
		required = append(required, rom)
	}
	return required
}

func ValidateFBNeoArchive(path string, game FBNeoGame) error {
	reader, err := zip.OpenReader(path)
	if err != nil {
		return fmt.Errorf("open %s: %w", filepath.Base(path), err)
	}
	defer reader.Close()
	entries := make(map[string]*zip.File, len(reader.File))
	entriesByFingerprint := make(map[string][]*zip.File, len(reader.File))
	for _, file := range reader.File {
		entries[strings.ToLower(strings.ReplaceAll(file.Name, "\\", "/"))] = file
		fingerprint := fbNeoROMFingerprint(int64(file.UncompressedSize64), file.CRC32)
		entriesByFingerprint[fingerprint] = append(entriesByFingerprint[fingerprint], file)
	}
	usedEntries := make(map[*zip.File]bool, len(reader.File))
	for _, rom := range game.RequiredROMs() {
		entry := entries[strings.ToLower(strings.ReplaceAll(rom.Name, "\\", "/"))]
		if entry == nil {
			for _, candidate := range entriesByFingerprint[fbNeoROMFingerprint(rom.Size, parseFBNeoCRC(rom.CRC))] {
				if !usedEntries[candidate] {
					entry = candidate
					break
				}
			}
		}
		if entry == nil {
			return fmt.Errorf("%s is missing ROM %s", filepath.Base(path), rom.Name)
		}
		if usedEntries[entry] {
			return fmt.Errorf("%s is missing a distinct ROM for %s", filepath.Base(path), rom.Name)
		}
		if !matchesFBNeoROM(game.Name, rom, int64(entry.UncompressedSize64), entry.CRC32) {
			return fmt.Errorf("%s ROM %s does not match DAT size/CRC", filepath.Base(path), rom.Name)
		}
		usedEntries[entry] = true
	}
	return nil
}

func fbNeoROMFingerprint(size int64, crc uint32) string {
	return fmt.Sprintf("%d:%08x", size, crc)
}

func parseFBNeoCRC(value string) uint32 {
	parsed, err := strconv.ParseUint(strings.TrimSpace(value), 16, 32)
	if err != nil {
		return 0
	}
	return uint32(parsed)
}

func matchesFBNeoROM(gameName string, rom FBNeoROM, actualSize int64, actualCRC uint32) bool {
	if actualSize == rom.Size && actualCRC == parseFBNeoCRC(rom.CRC) {
		return true
	}

	// Compatibility registrations cover field-proven historical ROM content
	// that FBNeo can run but the installed DAT no longer lists. Keep these
	// content-based so renamed/repacked archives remain usable without allowing
	// unrelated or damaged ROM data through the audit.
	switch strings.ToLower(strings.TrimSpace(gameName)) + "/" + strings.ToLower(strings.TrimSpace(rom.Name)) {
	case "captcomm/ioc1.ic7":
		return actualSize == 279 && actualCRC == 0x0d182081
	default:
		return false
	}
}
