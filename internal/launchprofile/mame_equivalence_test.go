package launchprofile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMAMEContentDefinitionComparesCompleteClosure(t *testing.T) {
	xml := `<mame build="0.288 (release)"><machine name="clone" cloneof="parent" romof="parent"><description>Old title</description><rom name="a" size="4" crc="11111111" sha1="aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" region="main" offset="0"/><device_ref name="sound"/></machine><machine name="parent"><rom name="p" size="4" crc="22222222"/></machine><machine name="sound" isdevice="yes"><rom name="s" size="4" crc="33333333"/></machine></mame>`
	makeDefinition := func(s string) MAMEContentDefinition {
		t.Helper()
		p := filepath.Join(t.TempDir(), "list.xml")
		if err := os.WriteFile(p, []byte(s), 0600); err != nil {
			t.Fatal(err)
		}
		c, err := ParseMAMEListXMLFile(p, []string{"clone"})
		if err != nil {
			t.Fatal(err)
		}
		d, err := c.ContentDefinition("clone")
		if err != nil {
			t.Fatal(err)
		}
		return d
	}
	a := makeDefinition(xml)
	equivalent := strings.ReplaceAll(strings.ReplaceAll(xml, "0.288", "0.289"), "Old title", "New localized title")
	b := makeDefinition(equivalent)
	if a.ListXMLSHA256 == b.ListXMLSHA256 || a.DefinitionSHA256 != b.DefinitionSHA256 {
		t.Fatal("build and descriptive changes must preserve content")
	}
	for _, test := range []struct{ name, from, to string }{
		{"entry CRC", "11111111", "11111112"}, {"parent ROM", "22222222", "22222223"}, {"device ROM", "33333333", "33333334"},
		{"SHA1", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"},
		{"placement", `offset="0"`, `offset="4"`}, {"device closure", `<device_ref name="sound"/>`, ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			d := makeDefinition(strings.ReplaceAll(equivalent, test.from, test.to))
			if a.DefinitionSHA256 == d.DefinitionSHA256 {
				t.Fatal("content change accepted as equivalent")
			}
		})
	}
}

func TestMAMEContentRegistryRejectsAmbiguousEvidence(t *testing.T) {
	d := MAMEContentDefinition{Set: "clone", Version: "0.288", ListXMLSHA256: strings.Repeat("a", 64), DefinitionSHA256: strings.Repeat("b", 64)}
	r := MAMEContentRegistry{SchemaVersion: 1, Definitions: []MAMEContentDefinition{d, d}}
	if r.Validate() == nil {
		t.Fatal("duplicate evidence accepted")
	}
	r.Definitions = r.Definitions[:1]
	r.Definitions[0].Version = "*"
	if r.Validate() == nil {
		t.Fatal("wildcard accepted")
	}
}
