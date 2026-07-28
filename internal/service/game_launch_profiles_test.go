package service

import (
	"testing"

	"foliospace-reader/internal/domain"
)

func TestAuditedGameLaunchProfilesAreValid(t *testing.T) {
	if err := validateAuditedGameLaunchProfiles(); err != nil {
		t.Fatal(err)
	}
}

func TestLogicalLaunchNamesRejectUnsafeAndCollidingPaths(t *testing.T) {
	for _, name := range []string{"", ".", "..", "../rom.zip", `folder\\rom.zip`, `/rom.zip`, `C:\\rom.zip`, "bad\x00name.zip"} {
		if validLogicalLaunchName(name) {
			t.Fatalf("validLogicalLaunchName(%q) = true, want false", name)
		}
	}
	if !validLogicalLaunchName("tektagtc1a.zip") {
		t.Fatal("expected audited ROM alias to be valid")
	}
}

func TestValidateGameLaunchResolveRequestRejectsInvalidCoreHash(t *testing.T) {
	req := domain.GameLaunchResolveRequest{
		Client:   domain.GameLaunchClient{Name: "SpatialEMU.Windows", Version: "1.302", Platform: "windows-x64", Architecture: "x64"},
		Runtimes: []domain.GameRuntimeDescriptor{{ID: "libretro", CoreID: "fbneo", CoreSHA256: "ABC"}},
	}
	if err := ValidateGameLaunchResolveRequest(req); err == nil {
		t.Fatal("expected invalid core hash to be rejected")
	}
	req.Runtimes[0].CoreSHA256 = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	if err := ValidateGameLaunchResolveRequest(req); err != nil {
		t.Fatalf("valid request rejected: %v", err)
	}
}

func TestLaunchProfileClientVersionFloor(t *testing.T) {
	if versionAtLeast("1.301", "1.302") {
		t.Fatal("1.301 must not match a 1.302 profile")
	}
	if !versionAtLeast("1.302", "1.302") || !versionAtLeast("1.303", "1.302") {
		t.Fatal("1.302 and later should match the profile floor")
	}
}
