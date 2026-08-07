package service

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"foliospace-reader/internal/db"
	"foliospace-reader/internal/domain"
	"foliospace-reader/internal/store"
)

func TestContentHashWorkerHashesAndInvalidatesChangedBook(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "book.cbz")
	if err := os.WriteFile(path, []byte("first content"), 0o644); err != nil {
		t.Fatal(err)
	}

	conn, err := db.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	st := store.New(conn)
	lib, err := st.CreateLibraryWithType("Books", root, "comic")
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	book, err := st.UpsertBasicBookFile(lib.ID, "Series", root, "Book", "cbz", path, "book.cbz", info.Size(), info.ModTime(), ".cbz")
	if err != nil {
		t.Fatal(err)
	}

	pending, err := st.BookByID(book.ID)
	if err != nil {
		t.Fatal(err)
	}
	if pending.ContentHash != nil || pending.ContentHashAlgorithm != nil || pending.ContentRevision != nil {
		t.Fatalf("pending content identity = %#v, want nil fields", pending)
	}
	if pending.FileSize != int64(len("first content")) {
		t.Fatalf("pending file size = %d, want %d", pending.FileSize, len("first content"))
	}

	svc := NewWithConfig(st, t.TempDir())
	processed, err := svc.processNextContentHash()
	if err != nil || !processed {
		t.Fatalf("processNextContentHash() = processed %v, err %v", processed, err)
	}

	first, err := st.BookByID(book.ID)
	if err != nil {
		t.Fatal(err)
	}
	firstBytes := []byte("first content")
	firstSum := sha256.Sum256(firstBytes)
	wantHash := hex.EncodeToString(firstSum[:])
	if first.ContentHash == nil || *first.ContentHash != wantHash {
		t.Fatalf("content hash = %v, want %s", first.ContentHash, wantHash)
	}
	if first.ContentHashAlgorithm == nil || *first.ContentHashAlgorithm != "sha256" {
		t.Fatalf("content hash algorithm = %v, want sha256", first.ContentHashAlgorithm)
	}
	if first.ContentRevision == nil || *first.ContentRevision != store.ContentRevisionForHash(wantHash, "cbz", 0) {
		t.Fatalf("content revision = %v, want revision for initial page set", first.ContentRevision)
	}

	if err := st.ReplacePages(book.ID, []domain.Page{{Index: 0, Name: "001.jpg"}}); err != nil {
		t.Fatal(err)
	}
	withPage, err := st.BookByID(book.ID)
	if err != nil {
		t.Fatal(err)
	}
	if withPage.ContentRevision == nil || *withPage.ContentRevision != store.ContentRevisionForPages(wantHash, "cbz", []domain.Page{{Index: 0, Name: "001.jpg"}}) {
		t.Fatalf("page-aware revision = %v, want revision for one page", withPage.ContentRevision)
	}
	if *withPage.ContentRevision == *first.ContentRevision {
		t.Fatal("content revision did not change after page collection changed")
	}

	if err := st.ReplacePages(book.ID, []domain.Page{{Index: 0, Name: "002.jpg"}}); err != nil {
		t.Fatal(err)
	}
	withRenamedPage, err := st.BookByID(book.ID)
	if err != nil {
		t.Fatal(err)
	}
	if withRenamedPage.ContentRevision == nil || *withRenamedPage.ContentRevision == *withPage.ContentRevision {
		t.Fatalf("same-size page collection kept revision: before=%v after=%v", withPage.ContentRevision, withRenamedPage.ContentRevision)
	}

	secondBytes := []byte("second content with a changed byte")
	if err := os.WriteFile(path, secondBytes, 0o644); err != nil {
		t.Fatal(err)
	}
	secondInfo, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.UpsertBasicBookFile(lib.ID, "Series", root, "Book", "cbz", path, "book.cbz", secondInfo.Size(), secondInfo.ModTime(), ".cbz"); err != nil {
		t.Fatal(err)
	}
	invalidated, err := st.BookByID(book.ID)
	if err != nil {
		t.Fatal(err)
	}
	if invalidated.ContentHash != nil || invalidated.ContentRevision != nil {
		t.Fatalf("changed file retained content identity: %#v", invalidated)
	}

	processed, err = svc.processNextContentHash()
	if err != nil || !processed {
		t.Fatalf("reprocess changed file = processed %v, err %v", processed, err)
	}
	second, err := st.BookByID(book.ID)
	if err != nil {
		t.Fatal(err)
	}
	secondSum := sha256.Sum256(secondBytes)
	secondHash := hex.EncodeToString(secondSum[:])
	if second.ContentHash == nil || *second.ContentHash != secondHash || *second.ContentHash == wantHash {
		t.Fatalf("changed file hash = %v, want %s and not %s", second.ContentHash, secondHash, wantHash)
	}
	if second.ContentRevision == nil || *second.ContentRevision != store.ContentRevisionForPages(secondHash, "cbz", []domain.Page{{Index: 0, Name: "002.jpg"}}) {
		t.Fatalf("changed file revision = %v, want revision for new content", second.ContentRevision)
	}
}

func TestContentHashFailureCanBeRetried(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "later.epub")
	conn, err := db.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	st := store.New(conn)
	lib, err := st.CreateLibraryWithType("Books", root, "book")
	if err != nil {
		t.Fatal(err)
	}
	book, err := st.UpsertBasicBookFile(lib.ID, "Series", root, "Later", "epub", path, "later.epub", 0, time.Now(), ".epub")
	if err != nil {
		t.Fatal(err)
	}
	svc := NewWithConfig(st, t.TempDir())
	if processed, err := svc.processNextContentHash(); err != nil || !processed {
		t.Fatalf("missing file hash = processed %v, err %v", processed, err)
	}
	if err := svc.RetryFailedContentHashes(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("now available"), 0o644); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.UpsertBasicBookFile(lib.ID, "Series", root, "Later", "epub", path, "later.epub", info.Size(), info.ModTime(), ".epub"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.processNextContentHash(); err != nil {
		t.Fatal(err)
	}
	got, err := st.BookByID(book.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.ContentHash == nil || len(strings.TrimSpace(*got.ContentHash)) != 64 {
		t.Fatalf("retried content hash = %v, want 64 hex characters", got.ContentHash)
	}
}
