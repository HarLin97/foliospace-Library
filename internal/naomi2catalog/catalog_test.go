package naomi2catalog

import "testing"

func TestCatalogSplitCloneParents(t *testing.T) {
	want := map[string]string{
		"clubkrto":  "clubkrt",
		"clubkrta":  "clubkrt",
		"clubkrtc":  "clubkrt",
		"kingrt66p": "kingrt66",
		"vstrik3co": "vstrik3c",
	}
	for clone, parent := range want {
		entry, ok := Lookup(clone)
		if !ok || entry.SetName != clone || entry.Parent != parent {
			t.Errorf("Lookup(%q) = %#v, %v; want parent %q", clone, entry, ok, parent)
		}
	}
	if got := Parent("VF4"); got != "" {
		t.Fatalf("Parent(VF4) = %q, want empty", got)
	}
}

func TestCatalogEntriesHaveStableIdentity(t *testing.T) {
	entries := Entries()
	if len(entries) != 43 {
		t.Fatalf("len(Entries()) = %d, want 43", len(entries))
	}
	for _, entry := range entries {
		if entry.SetName == "" || entry.Title == "" || entry.Region == "" {
			t.Fatalf("incomplete entry: %#v", entry)
		}
		if entry.Parent != "" {
			if _, ok := Lookup(entry.Parent); !ok {
				t.Fatalf("entry %q references unknown parent %q", entry.SetName, entry.Parent)
			}
		}
	}
}
