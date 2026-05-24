package archive

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestListPagesSortsImagesAndSkipsNonImages(t *testing.T) {
	path := makeZip(t, map[string]string{
		"002.jpg":   "two",
		"001.png":   "one",
		"notes.txt": "skip",
	})

	pages, err := ListPages(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(pages) != 2 {
		t.Fatalf("pages len = %d, want 2", len(pages))
	}
	if pages[0].Name != "001.png" || pages[1].Name != "002.jpg" {
		t.Fatalf("pages = %#v, want sorted image pages", pages)
	}
}

func TestOpenPageStreamsExpectedBytes(t *testing.T) {
	path := makeZip(t, map[string]string{
		"002.jpg": "two",
		"001.png": "one",
	})

	page, contentType, err := OpenPage(path, 1)
	if err != nil {
		t.Fatal(err)
	}
	defer page.Close()

	data, err := io.ReadAll(page)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "two" {
		t.Fatalf("page bytes = %q, want two", string(data))
	}
	if contentType != "image/jpeg" {
		t.Fatalf("content type = %q, want image/jpeg", contentType)
	}
}

func makeZip(t *testing.T, entries map[string]string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "book.cbz")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(file)
	for name, body := range entries {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}
