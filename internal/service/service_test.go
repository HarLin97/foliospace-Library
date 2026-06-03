package service

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"foliospace-reader/internal/db"
	"foliospace-reader/internal/domain"
	"foliospace-reader/internal/store"
)

func TestListDirectoriesRestrictsToConfiguredRoots(t *testing.T) {
	allowed := t.TempDir()
	libraryRoot := t.TempDir()
	blocked := t.TempDir()
	if err := mkdir(filepath.Join(allowed, "Books")); err != nil {
		t.Fatal(err)
	}
	if err := mkdir(filepath.Join(libraryRoot, "Comics")); err != nil {
		t.Fatal(err)
	}
	if err := mkdir(filepath.Join(blocked, "Private")); err != nil {
		t.Fatal(err)
	}
	t.Setenv("FOLIOSPACE_DIRECTORY_ROOTS", allowed)
	t.Setenv("FOLIOSPACE_LIBRARY_DIR", "")

	conn, err := db.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	st := store.New(conn)
	if _, err := st.CreateLibrary("Existing", libraryRoot); err != nil {
		t.Fatal(err)
	}

	svc := New(st)
	rootListing, err := svc.ListDirectories("/")
	if err != nil {
		t.Fatal(err)
	}
	if !hasDirectory(rootListing.Entries, allowed) || !hasDirectory(rootListing.Entries, libraryRoot) {
		t.Fatalf("root entries = %#v, want configured and existing library roots", rootListing.Entries)
	}
	if hasDirectory(rootListing.Entries, blocked) {
		t.Fatalf("root entries = %#v, blocked directory should not be exposed", rootListing.Entries)
	}

	allowedListing, err := svc.ListDirectories(allowed)
	if err != nil {
		t.Fatal(err)
	}
	if allowedListing.Parent != "/" {
		t.Fatalf("allowed root parent = %q, want virtual root", allowedListing.Parent)
	}
	if !hasDirectory(allowedListing.Entries, filepath.Join(allowed, "Books")) {
		t.Fatalf("allowed entries = %#v, want Books child", allowedListing.Entries)
	}

	if _, err := svc.ListDirectories(blocked); err == nil {
		t.Fatal("blocked directory listing succeeded, want error")
	}
}

func TestOpenVideoThumbnailUsesLocalSidecarImage(t *testing.T) {
	root := t.TempDir()
	videoPath := filepath.Join(root, "Demo Movie.mp4")
	coverPath := filepath.Join(root, "Demo Movie.jpg")
	if err := os.WriteFile(videoPath, []byte("video"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(coverPath, []byte{0xff, 0xd8, 0xff, 0xdb, 0x00, 0x43, 0x00}, 0o644); err != nil {
		t.Fatal(err)
	}

	conn, err := db.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	st := store.New(conn)
	lib, err := st.CreateLibraryWithType("Movies", root, "video")
	if err != nil {
		t.Fatal(err)
	}
	video, err := st.UpsertVideo(domain.VideoAsset{
		LibraryID:       lib.ID,
		Title:           "Demo Movie",
		Format:          "mp4",
		FilePath:        videoPath,
		RelPath:         "Demo Movie.mp4",
		Size:            5,
		MTime:           time.Unix(1, 0),
		ThumbnailStatus: "placeholder",
	})
	if err != nil {
		t.Fatal(err)
	}

	stream, err := NewWithConfig(st, t.TempDir()).OpenVideoThumbnail(video.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Body.Close()
	data, err := io.ReadAll(stream.Body)
	if err != nil {
		t.Fatal(err)
	}
	if stream.ContentType != "image/jpeg" || len(data) == 0 {
		t.Fatalf("stream contentType=%q len=%d, want local jpeg", stream.ContentType, len(data))
	}
}

func TestBookThumbnailQueuesAndWorkerGeneratesCachedImage(t *testing.T) {
	root := t.TempDir()
	bookPath := filepath.Join(root, "Series A", "book1.cbz")
	if err := os.MkdirAll(filepath.Dir(bookPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := makeImageZipSized(bookPath, 1200, 1680); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(bookPath)
	if err != nil {
		t.Fatal(err)
	}

	conn, err := db.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	st := store.New(conn)
	lib, err := st.CreateLibrary("Comics", root)
	if err != nil {
		t.Fatal(err)
	}
	series, err := st.UpsertSeries(lib.ID, "Series A", "Series A")
	if err != nil {
		t.Fatal(err)
	}
	book, err := st.UpsertBook(series.ID, "book1", "cbz")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.UpsertFile(book.ID, lib.ID, bookPath, "Series A/book1.cbz", info.Size(), info.ModTime(), ".cbz"); err != nil {
		t.Fatal(err)
	}

	svc := NewWithConfig(st, t.TempDir())
	svc.PauseThumbnailWorker()
	stream, err := svc.OpenBookThumbnail(book.ID, "small")
	if err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(stream.Body)
	stream.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	if stream.ContentType != "image/jpeg" || stream.CacheHit || len(data) == 0 {
		t.Fatalf("first thumbnail contentType=%q cacheHit=%v len=%d, want original cover jpeg while thumbnail is queued", stream.ContentType, stream.CacheHit, len(data))
	}
	originalImage, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if originalImage.Bounds().Dx() != 1200 {
		t.Fatalf("first thumbnail fallback width = %d, want original cover width before cached thumbnail is ready", originalImage.Bounds().Dx())
	}
	status, err := svc.ThumbnailWorkerStatus()
	if err != nil {
		t.Fatal(err)
	}
	if !status.Paused || status.Queued != 1 || status.Ready != 0 {
		t.Fatalf("paused status = %#v, want paused with one queued job", status)
	}

	svc.ResumeThumbnailWorker()
	if err := svc.ProcessNextThumbnailJobForTest(); err != nil {
		t.Fatal(err)
	}
	status, err = svc.ThumbnailWorkerStatus()
	if err != nil {
		t.Fatal(err)
	}
	if status.Ready != 1 || status.Queued != 0 || status.Failed != 0 {
		t.Fatalf("status after processing = %#v, want one ready job", status)
	}
	cached, err := svc.OpenBookThumbnail(book.ID, "small")
	if err != nil {
		t.Fatal(err)
	}
	defer cached.Body.Close()
	cachedData, err := io.ReadAll(cached.Body)
	if err != nil {
		t.Fatal(err)
	}
	if cached.ContentType != "image/jpeg" || len(cachedData) == 0 || !cached.CacheHit {
		t.Fatalf("cached thumbnail contentType=%q len=%d cacheHit=%v, want cached jpeg", cached.ContentType, len(cachedData), cached.CacheHit)
	}
	cachedImage, _, err := image.Decode(bytes.NewReader(cachedData))
	if err != nil {
		t.Fatal(err)
	}
	if cachedImage.Bounds().Dx() != 320 {
		t.Fatalf("small thumbnail width = %d, want 320 for 1x cover wall rendering", cachedImage.Bounds().Dx())
	}
	if ThumbnailCacheVersion() != "v1" {
		t.Fatalf("thumbnail cache version = %q, want v1", ThumbnailCacheVersion())
	}
	bookWithPath, err := st.BookByID(book.ID)
	if err != nil {
		t.Fatal(err)
	}
	currentKey, err := svc.bookThumbnailCacheKey(bookWithPath, "small")
	if err != nil {
		t.Fatal(err)
	}
	legacyKey, err := legacyThumbnailV1CacheKey(bookWithPath, "small")
	if err != nil {
		t.Fatal(err)
	}
	if currentKey == legacyKey {
		t.Fatal("current v1 cache key reused legacy uncropped v1 cache key")
	}
}

func TestBookThumbnailServesStaleCacheWhileRegenerating(t *testing.T) {
	root := t.TempDir()
	bookPath := filepath.Join(root, "Series A", "book1.cbz")
	if err := os.MkdirAll(filepath.Dir(bookPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := makeImageZipSized(bookPath, 1200, 1680); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(bookPath)
	if err != nil {
		t.Fatal(err)
	}

	conn, err := db.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	st := store.New(conn)
	lib, err := st.CreateLibrary("Comics", root)
	if err != nil {
		t.Fatal(err)
	}
	series, err := st.UpsertSeries(lib.ID, "Series A", "Series A")
	if err != nil {
		t.Fatal(err)
	}
	book, err := st.UpsertBook(series.ID, "book1", "cbz")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.UpsertFile(book.ID, lib.ID, bookPath, "Series A/book1.cbz", info.Size(), info.ModTime(), ".cbz"); err != nil {
		t.Fatal(err)
	}

	svc := NewWithConfig(st, t.TempDir())
	svc.PauseThumbnailWorker()
	stalePath, err := svc.bookThumbnailCachePath(book.ID, "small", "legacy-fallback")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(stalePath), 0o755); err != nil {
		t.Fatal(err)
	}
	staleBytes := makeTestJPEGBytes(t, 32, 44, color.RGBA{R: 120, G: 90, B: 180, A: 255})
	if err := os.WriteFile(stalePath, staleBytes, 0o644); err != nil {
		t.Fatal(err)
	}

	stream, err := svc.OpenBookThumbnail(book.ID, "small")
	if err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(stream.Body)
	_ = stream.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	if stream.ContentType != "image/jpeg" || !stream.CacheHit || stream.CachePath != stalePath || !bytes.Equal(data, staleBytes) {
		t.Fatalf("stale stream type=%q cacheHit=%v path=%q len=%d, want stale jpeg fallback from %q", stream.ContentType, stream.CacheHit, stream.CachePath, len(data), stalePath)
	}
	status, err := svc.ThumbnailWorkerStatus()
	if err != nil {
		t.Fatal(err)
	}
	if status.Queued != 1 || status.Ready != 0 || !status.Paused {
		t.Fatalf("thumbnail worker status = %#v, want queued regeneration while serving stale cache", status)
	}
}

func TestBookThumbnailCropsLandscapeCoverToPortraitRatio(t *testing.T) {
	root := t.TempDir()
	bookPath := filepath.Join(root, "Series A", "landscape.cbz")
	if err := os.MkdirAll(filepath.Dir(bookPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := makeImageZipSized(bookPath, 2400, 1200); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(bookPath)
	if err != nil {
		t.Fatal(err)
	}

	conn, err := db.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	st := store.New(conn)
	lib, err := st.CreateLibrary("Comics", root)
	if err != nil {
		t.Fatal(err)
	}
	series, err := st.UpsertSeries(lib.ID, "Series A", "Series A")
	if err != nil {
		t.Fatal(err)
	}
	book, err := st.UpsertBook(series.ID, "landscape", "cbz")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.UpsertFile(book.ID, lib.ID, bookPath, "Series A/landscape.cbz", info.Size(), info.ModTime(), ".cbz"); err != nil {
		t.Fatal(err)
	}

	svc := NewWithConfig(st, t.TempDir())
	svc.PauseThumbnailWorker()
	stream, err := svc.OpenBookThumbnail(book.ID, "small")
	if err != nil {
		t.Fatal(err)
	}
	_ = stream.Body.Close()
	svc.ResumeThumbnailWorker()
	if err := svc.ProcessNextThumbnailJobForTest(); err != nil {
		t.Fatal(err)
	}
	cached, err := svc.OpenBookThumbnail(book.ID, "small")
	if err != nil {
		t.Fatal(err)
	}
	defer cached.Body.Close()
	cachedData, err := io.ReadAll(cached.Body)
	if err != nil {
		t.Fatal(err)
	}
	cachedImage, _, err := image.Decode(bytes.NewReader(cachedData))
	if err != nil {
		t.Fatal(err)
	}
	bounds := cachedImage.Bounds()
	if bounds.Dx() != 320 {
		t.Fatalf("landscape thumbnail width = %d, want 320", bounds.Dx())
	}
	ratio := float64(bounds.Dx()) / float64(bounds.Dy())
	target := 3.0 / 4.15
	if math.Abs(ratio-target) > 0.01 {
		t.Fatalf("landscape thumbnail ratio = %.3f (%dx%d), want portrait ratio %.3f", ratio, bounds.Dx(), bounds.Dy(), target)
	}
}

func TestBookThumbnailUsesGenericCoverWhenSourceFails(t *testing.T) {
	root := t.TempDir()
	bookPath := filepath.Join(root, "Series A", "broken.cbz")
	if err := os.MkdirAll(filepath.Dir(bookPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bookPath, []byte("not a zip archive"), 0o644); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(bookPath)
	if err != nil {
		t.Fatal(err)
	}

	conn, err := db.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	st := store.New(conn)
	lib, err := st.CreateLibrary("Comics", root)
	if err != nil {
		t.Fatal(err)
	}
	series, err := st.UpsertSeries(lib.ID, "Series A", "Series A")
	if err != nil {
		t.Fatal(err)
	}
	book, err := st.UpsertBook(series.ID, "broken", "cbz")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.UpsertFile(book.ID, lib.ID, bookPath, "Series A/broken.cbz", info.Size(), info.ModTime(), ".cbz"); err != nil {
		t.Fatal(err)
	}

	svc := NewWithConfig(st, t.TempDir())
	svc.PauseThumbnailWorker()
	stream, err := svc.OpenBookThumbnail(book.ID, "small")
	if err != nil {
		t.Fatal(err)
	}
	_ = stream.Body.Close()
	svc.ResumeThumbnailWorker()
	if err := svc.ProcessNextThumbnailJobForTest(); err != nil {
		t.Fatal(err)
	}

	status, err := svc.ThumbnailWorkerStatus()
	if err != nil {
		t.Fatal(err)
	}
	if status.Ready != 1 || status.Failed != 0 {
		t.Fatalf("thumbnail status = %#v, want generic cover stored as ready without failed job", status)
	}
	cached, err := svc.OpenBookThumbnail(book.ID, "small")
	if err != nil {
		t.Fatal(err)
	}
	defer cached.Body.Close()
	cachedData, err := io.ReadAll(cached.Body)
	if err != nil {
		t.Fatal(err)
	}
	if cached.ContentType != "image/jpeg" || !cached.CacheHit {
		t.Fatalf("fallback thumbnail contentType=%q cacheHit=%v, want cached generic jpeg", cached.ContentType, cached.CacheHit)
	}
	cachedImage, _, err := image.Decode(bytes.NewReader(cachedData))
	if err != nil {
		t.Fatal(err)
	}
	if cachedImage.Bounds().Dx() != 320 {
		t.Fatalf("generic thumbnail width = %d, want 320", cachedImage.Bounds().Dx())
	}
}

func TestThumbnailWorkerContinuesAfterFailedJob(t *testing.T) {
	root := t.TempDir()
	badPath := filepath.Join(root, "Series A", "bad.cbz")
	goodPath := filepath.Join(root, "Series A", "good.cbz")
	if err := os.MkdirAll(filepath.Dir(badPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(badPath, []byte("not a zip archive"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := makeImageZip(goodPath); err != nil {
		t.Fatal(err)
	}
	badInfo, err := os.Stat(badPath)
	if err != nil {
		t.Fatal(err)
	}
	goodInfo, err := os.Stat(goodPath)
	if err != nil {
		t.Fatal(err)
	}

	conn, err := db.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	st := store.New(conn)
	lib, err := st.CreateLibrary("Comics", root)
	if err != nil {
		t.Fatal(err)
	}
	series, err := st.UpsertSeries(lib.ID, "Series A", "Series A")
	if err != nil {
		t.Fatal(err)
	}
	badBook, err := st.UpsertBook(series.ID, "bad", "cbz")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.UpsertFile(badBook.ID, lib.ID, badPath, "Series A/bad.cbz", badInfo.Size(), badInfo.ModTime(), ".cbz"); err != nil {
		t.Fatal(err)
	}
	badBook, err = st.BookByID(badBook.ID)
	if err != nil {
		t.Fatal(err)
	}
	goodBook, err := st.UpsertBook(series.ID, "good", "cbz")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.UpsertFile(goodBook.ID, lib.ID, goodPath, "Series A/good.cbz", goodInfo.Size(), goodInfo.ModTime(), ".cbz"); err != nil {
		t.Fatal(err)
	}
	goodBook, err = st.BookByID(goodBook.ID)
	if err != nil {
		t.Fatal(err)
	}

	svc := NewWithConfig(st, t.TempDir())
	svc.PauseThumbnailWorker()
	badKey, err := svc.bookThumbnailCacheKey(badBook, "small")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.EnqueueThumbnailJob(domain.ThumbnailJobInput{
		BookID:   badBook.ID,
		Size:     "small",
		CacheKey: badKey,
		Priority: 100,
	}); err != nil {
		t.Fatal(err)
	}
	goodKey, err := svc.bookThumbnailCacheKey(goodBook, "small")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.EnqueueThumbnailJob(domain.ThumbnailJobInput{
		BookID:   goodBook.ID,
		Size:     "small",
		CacheKey: goodKey,
		Priority: 100,
	}); err != nil {
		t.Fatal(err)
	}
	svc.ResumeThumbnailWorker()

	var status domain.ThumbnailQueueStatus
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		status, err = svc.ThumbnailWorkerStatus()
		if err != nil {
			t.Fatal(err)
		}
		if status.Failed == 0 && status.Ready == 2 && status.Queued == 0 {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("thumbnail worker status = %#v, want generic bad job and good job both ready", status)
}

func TestThumbnailWorkerStatusReportsCacheStats(t *testing.T) {
	root := t.TempDir()
	configDir := t.TempDir()
	bookPath := filepath.Join(root, "Series A", "book1.cbz")
	if err := os.MkdirAll(filepath.Dir(bookPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := makeImageZipSized(bookPath, 1200, 1680); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(bookPath)
	if err != nil {
		t.Fatal(err)
	}

	conn, err := db.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	st := store.New(conn)
	lib, err := st.CreateLibrary("Comics", root)
	if err != nil {
		t.Fatal(err)
	}
	series, err := st.UpsertSeries(lib.ID, "Series A", "Series A")
	if err != nil {
		t.Fatal(err)
	}
	book, err := st.UpsertBook(series.ID, "book1", "cbz")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.UpsertFile(book.ID, lib.ID, bookPath, "Series A/book1.cbz", info.Size(), info.ModTime(), ".cbz"); err != nil {
		t.Fatal(err)
	}

	svc := NewWithConfig(st, configDir)
	svc.PauseThumbnailWorker()
	stream, err := svc.OpenBookThumbnail(book.ID, "small")
	if err != nil {
		t.Fatal(err)
	}
	_ = stream.Body.Close()
	svc.ResumeThumbnailWorker()
	if err := svc.ProcessNextThumbnailJobForTest(); err != nil {
		t.Fatal(err)
	}

	orphanPath := filepath.Join(configDir, "cache", "book-thumbnails", "small", "orphan.jpg")
	if err := os.WriteFile(orphanPath, []byte("orphan-cache"), 0o644); err != nil {
		t.Fatal(err)
	}
	status, err := svc.ThumbnailWorkerStatus()
	if err != nil {
		t.Fatal(err)
	}
	if status.Cache.ReadyFiles != 1 || status.Cache.OrphanFiles != 1 || status.Cache.Files != 2 {
		t.Fatalf("cache status = %#v, want one ready file and one orphan file", status.Cache)
	}
	if status.Cache.Bytes <= int64(len("orphan-cache")) || status.Cache.AlgorithmVersion == "" {
		t.Fatalf("cache status = %#v, want bytes and algorithm version", status.Cache)
	}
	if status.Cache.SmallWidth != 320 || status.Cache.MediumWidth != 640 {
		t.Fatalf("cache dimensions = small %d medium %d, want 320/640", status.Cache.SmallWidth, status.Cache.MediumWidth)
	}
	cleanup, err := svc.CleanupThumbnailOrphanCache()
	if err != nil {
		t.Fatal(err)
	}
	if cleanup.DeletedFiles != 1 || cleanup.DeletedBytes != int64(len("orphan-cache")) || cleanup.FailedFiles != 0 {
		t.Fatalf("cleanup result = %#v, want one deleted orphan", cleanup)
	}
	if _, err := os.Stat(orphanPath); !os.IsNotExist(err) {
		t.Fatalf("orphan cache file still exists or stat failed unexpectedly: %v", err)
	}
	status, err = svc.ThumbnailWorkerStatus()
	if err != nil {
		t.Fatal(err)
	}
	if status.Cache.OrphanFiles != 0 || status.Cache.Files != 1 || status.Cache.ReadyFiles != 1 {
		t.Fatalf("cache status after cleanup = %#v, want only ready file", status.Cache)
	}
}

func legacyThumbnailV1CacheKey(book domain.Book, size string) (string, error) {
	info, err := os.Stat(book.FilePath)
	if err != nil {
		return "", err
	}
	source := fmt.Sprintf("%d|%s|%s|%d|%s|%s|%s", book.ID, book.FilePath, book.Format, info.Size(), info.ModTime().UTC().Format(time.RFC3339Nano), size, "v1")
	sum := sha256.Sum256([]byte(source))
	return hex.EncodeToString(sum[:])[:20], nil
}

func makeImageZip(path string) error {
	return makeImageZipSized(path, 16, 24)
}

func makeImageZipSized(path string, width int, height int) error {
	var imageBody bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, color.RGBA{R: uint8(20 + x), G: uint8(40 + y), B: 180, A: 255})
		}
	}
	if err := jpeg.Encode(&imageBody, img, &jpeg.Options{Quality: 85}); err != nil {
		return err
	}
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	archive := zip.NewWriter(file)
	writer, err := archive.Create("001.jpg")
	if err != nil {
		_ = archive.Close()
		_ = file.Close()
		return err
	}
	if _, err := writer.Write(imageBody.Bytes()); err != nil {
		_ = archive.Close()
		_ = file.Close()
		return err
	}
	if err := archive.Close(); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}

func makeTestJPEGBytes(t *testing.T, width int, height int, fill color.RGBA) []byte {
	t.Helper()
	var imageBody bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, fill)
		}
	}
	if err := jpeg.Encode(&imageBody, img, &jpeg.Options{Quality: 85}); err != nil {
		t.Fatal(err)
	}
	return imageBody.Bytes()
}

func mkdir(path string) error {
	return os.MkdirAll(path, 0o755)
}

func hasDirectory(entries []domain.DirectoryEntry, path string) bool {
	for _, entry := range entries {
		if entry.Path == path {
			return true
		}
	}
	return false
}
