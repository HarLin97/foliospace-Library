package httpapi

import (
	"encoding/json"
	"strings"
	"testing"

	"foliospace-reader/internal/domain"
)

func TestClientBookContentIdentityIsNullableAndStable(t *testing.T) {
	book := domain.Book{ID: 42, Title: "Pending", Format: "epub", FilePath: "/library/pending.epub", FileSize: 123}
	var pending map[string]any
	encoded, err := json.Marshal(clientBookItem(book))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(encoded, &pending); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"contentHash", "contentHashAlgorithm", "contentRevision"} {
		if value, ok := pending[key]; !ok || value != nil {
			t.Fatalf("pending %s = %#v, want explicit null", key, value)
		}
	}
	if pending["fileSize"] != float64(123) {
		t.Fatalf("pending fileSize = %#v, want 123", pending["fileSize"])
	}

	hash := strings.Repeat("a", 64)
	revision := "sha256:" + strings.Repeat("b", 64)
	book.ContentHash = &hash
	algorithm := "sha256"
	book.ContentHashAlgorithm = &algorithm
	book.ContentRevision = &revision
	var ready map[string]any
	encoded, err = json.Marshal(clientBookItem(book))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(encoded, &ready); err != nil {
		t.Fatal(err)
	}
	if ready["contentHash"] != hash || ready["contentHashAlgorithm"] != algorithm || ready["contentRevision"] != revision {
		t.Fatalf("ready content identity = %#v", ready)
	}
}
