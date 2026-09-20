package service

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMAMEContentRegistryDefaultsAndAdministratorOverride(t *testing.T) {
	s := &Service{}
	if _, _, ok := s.mameContentRegistry().Equivalent("strider2u", "0.288", "0.289"); !ok {
		t.Fatal("bundled equivalent content missing")
	}
	s.configDir = t.TempDir()
	if len(s.mameContentRegistry().Definitions) != 1889 {
		t.Fatal("missing file should select bundled registry")
	}
	path := filepath.Join(s.configDir, "policies", "mame-content-equivalence.json")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	for _, data := range []string{`{"schemaVersion":1,"definitions":[]}`, `invalid`} {
		if err := os.WriteFile(path, []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
		if len(s.mameContentRegistry().Definitions) != 0 {
			t.Fatal("explicit empty or invalid operator registry must not fall back")
		}
	}
}
