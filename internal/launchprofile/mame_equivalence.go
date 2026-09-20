package launchprofile

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strings"
)

// MAMEContentRegistry contains administrator-generated evidence, never client claims.
// Each definition covers one game's complete parent/device closure.
type MAMEContentRegistry struct {
	SchemaVersion int                     `json:"schemaVersion"`
	Definitions   []MAMEContentDefinition `json:"definitions"`
}
type MAMEContentDefinition struct {
	Set              string `json:"set"`
	Version          string `json:"version"`
	ListXMLSHA256    string `json:"listxmlSha256"`
	DefinitionSHA256 string `json:"definitionSha256"`
}

var mameVersionPattern = regexp.MustCompile(`^0\.[0-9]{3,4}$`)
var definitionDigestPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

func ReadMAMEContentRegistry(path string) (MAMEContentRegistry, error) {
	f, err := os.Open(path)
	if err != nil {
		return MAMEContentRegistry{}, err
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, (2<<20)+1))
	if err != nil {
		return MAMEContentRegistry{}, err
	}
	if len(b) > 2<<20 {
		return MAMEContentRegistry{}, fmt.Errorf("MAME content registry exceeds 2 MiB")
	}
	var r MAMEContentRegistry
	if err = json.Unmarshal(b, &r); err != nil {
		return r, err
	}
	return r, r.Validate()
}
func (r MAMEContentRegistry) Validate() error {
	if r.SchemaVersion != 1 {
		return fmt.Errorf("unsupported MAME content registry schema")
	}
	seen := map[string]bool{}
	for _, d := range r.Definitions {
		key := d.Set + "/" + d.Version
		if d.Set == "" || strings.ContainsAny(d.Set, "/\\ ") || !mameVersionPattern.MatchString(d.Version) || !definitionDigestPattern.MatchString(d.ListXMLSHA256) || !definitionDigestPattern.MatchString(d.DefinitionSHA256) || seen[key] {
			return fmt.Errorf("invalid or duplicate MAME definition %q", key)
		}
		seen[key] = true
	}
	return nil
}
func (r MAMEContentRegistry) Equivalent(set, from, to string) (MAMEContentDefinition, MAMEContentDefinition, bool) {
	var a, b MAMEContentDefinition
	for _, d := range r.Definitions {
		if d.Set == set {
			if d.Version == from {
				a = d
			}
			if d.Version == to {
				b = d
			}
		}
	}
	return a, b, a.Set != "" && b.Set != "" && a.DefinitionSHA256 == b.DefinitionSHA256
}

// ContentDefinition is intentionally conservative: changes to parent or device
// references, default BIOS, ROM placement/checksums, or disks form a new family.
// Description, year, manufacturer and listxml build strings are not content.
func (c MAMECatalog) ContentDefinition(set string) (MAMEContentDefinition, error) {
	type machineContent struct {
		Name     string
		CloneOf  string
		ROMOf    string
		IsBIOS   bool
		IsDevice bool
		Runnable bool
		BIOS     []MAMEBIOSSet
		ROMs     []MAMEROM
		Disks    []MAMEDisk
		Refs     []string
	}
	version := strings.Fields(c.Build)
	if len(version) == 0 || !mameVersionPattern.MatchString(version[0]) {
		return MAMEContentDefinition{}, fmt.Errorf("invalid MAME build %q", c.Build)
	}
	seen := map[string]bool{}
	active := map[string]bool{}
	content := []machineContent{}
	var visit func(string) error
	visit = func(name string) error {
		if active[name] {
			return fmt.Errorf("cyclic content dependency %q", name)
		}
		if seen[name] {
			return nil
		}
		m, ok := c.Machines[name]
		if !ok {
			return fmt.Errorf("missing content dependency %q", name)
		}
		active[name] = true
		for _, ref := range m.dependencyReferences() {
			if err := visit(ref); err != nil {
				return err
			}
		}
		delete(active, name)
		seen[name] = true
		roms := append([]MAMEROM{}, m.ROMs...)
		disks := append([]MAMEDisk{}, m.Disks...)
		sort.Slice(roms, func(i, j int) bool {
			a, _ := json.Marshal(roms[i])
			b, _ := json.Marshal(roms[j])
			return string(a) < string(b)
		})
		sort.Slice(disks, func(i, j int) bool {
			a, _ := json.Marshal(disks[i])
			b, _ := json.Marshal(disks[j])
			return string(a) < string(b)
		})
		// Keep BIOS order: the first BIOS is the fallback when no explicit default exists.
		content = append(content, machineContent{m.Name, m.CloneOf, m.ROMOf, m.IsBIOS, m.IsDevice, m.Runnable, m.BIOSSets, roms, disks, m.dependencyReferences()})
		return nil
	}
	if err := visit(set); err != nil {
		return MAMEContentDefinition{}, err
	}
	sort.Slice(content, func(i, j int) bool { return content[i].Name < content[j].Name })
	encoded, err := json.Marshal(content)
	if err != nil {
		return MAMEContentDefinition{}, err
	}
	sum := sha256.Sum256(encoded)
	return MAMEContentDefinition{set, version[0], c.SHA256, hex.EncodeToString(sum[:])}, nil
}
