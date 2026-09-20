package launchprofile

import "testing"

func TestBundledMAMEEvidenceCoverage(t *testing.T) {
	r := DefaultMAMEContentRegistry()
	if err := r.Validate(); err != nil {
		t.Fatal(err)
	}
	counts := map[string]int{}
	for _, d := range r.Definitions {
		counts[d.Version]++
	}
	if counts["0.287"] != 628 || counts["0.288"] != 630 || counts["0.289"] != 631 || len(counts) != 3 {
		t.Fatalf("unexpected bundled coverage: %v", counts)
	}
	for _, set := range []string{"strider2u", "timecris", "timecrs2", "vcop", "vcop2", "vf2"} {
		_, _, from287 := r.Equivalent(set, "0.287", "0.289")
		_, _, from288 := r.Equivalent(set, "0.288", "0.289")
		if !from287 && !from288 {
			t.Errorf("missing equivalent source for %s", set)
		}
	}
	if _, _, ok := r.Equivalent("strider2u", "0.288", "0.999"); ok {
		t.Fatal("unlisted version cannot claim equivalence")
	}
}
