# FolioSpace Reader MVP Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the first runnable FolioSpace Reader service: Go API, SQLite persistence, CBZ/ZIP scanning and reading, React UI, Docker packaging, and MCP-ready service boundaries.

**Architecture:** A single Go service owns SQLite, scanner, archive reader, HTTP API, and static frontend hosting. Business logic lives in focused `internal/*` packages so HTTP handlers are thin and a future MCP server can reuse the same services. The React/Vite frontend consumes the HTTP API and is served by the Go service in production.

**Tech Stack:** Go 1.22+, SQLite via `modernc.org/sqlite`, `net/http`, React, TypeScript, Vite, Docker multi-stage build.

---

## File Structure

- `go.mod`: Go module definition.
- `cmd/foliospace-reader/main.go`: process entrypoint, config loading, DB opening, route setup.
- `internal/config/config.go`: environment/default config for `/config`, `/library`, and listen address.
- `internal/db/db.go`: SQLite connection and migrations.
- `internal/domain/models.go`: shared domain structs and error codes.
- `internal/store/store.go`: persistence methods for libraries, series, books, files, pages, jobs, errors, and progress.
- `internal/archive/zip.go`: CBZ/ZIP page listing and page streaming helpers.
- `internal/scanner/scanner.go`: library scan service.
- `internal/service/service.go`: application service facade used by HTTP and future MCP.
- `internal/httpapi/server.go`: route registration and JSON/image handlers.
- `web/package.json`: frontend package manifest.
- `web/src/App.tsx`: operational UI shell and API-backed views.
- `web/src/api.ts`: frontend API client.
- `web/src/styles.css`: compact reader/admin styling.
- `Dockerfile`: single image build.
- `docker-compose.yml`: local/NAS deployment example.
- `README.md`: setup, run, and acceptance instructions.

## Task 1: Project Skeleton and Configuration

**Files:**
- Create: `go.mod`
- Create: `cmd/foliospace-reader/main.go`
- Create: `internal/config/config.go`
- Create: `README.md`

- [ ] **Step 1: Write failing config test**

Create `internal/config/config_test.go`:

```go
package config

import "testing"

func TestLoadUsesNASDefaults(t *testing.T) {
	t.Setenv("FOLIOSPACE_CONFIG_DIR", "")
	t.Setenv("FOLIOSPACE_LIBRARY_DIR", "")
	t.Setenv("FOLIOSPACE_ADDR", "")

	cfg := Load()

	if cfg.ConfigDir != "/config" {
		t.Fatalf("ConfigDir = %q, want /config", cfg.ConfigDir)
	}
	if cfg.LibraryDir != "/library" {
		t.Fatalf("LibraryDir = %q, want /library", cfg.LibraryDir)
	}
	if cfg.Addr != ":8080" {
		t.Fatalf("Addr = %q, want :8080", cfg.Addr)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/config`

Expected: FAIL because the package does not exist or `Load` is undefined.

- [ ] **Step 3: Implement config package and entrypoint**

Implement `internal/config/config.go` with:

```go
package config

import "os"

type Config struct {
	ConfigDir  string
	LibraryDir string
	Addr       string
}

func Load() Config {
	return Config{
		ConfigDir:  envOr("FOLIOSPACE_CONFIG_DIR", "/config"),
		LibraryDir: envOr("FOLIOSPACE_LIBRARY_DIR", "/library"),
		Addr:       envOr("FOLIOSPACE_ADDR", ":8080"),
	}
}

func envOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
```

Create a minimal `cmd/foliospace-reader/main.go` that loads config and starts a placeholder HTTP server returning `FolioSpace Reader`.

- [ ] **Step 4: Verify**

Run: `go test ./...`

Expected: PASS.

- [ ] **Step 5: Commit**

Run:

```bash
git add go.mod cmd internal README.md
git commit -m "chore: scaffold FolioSpace Reader service"
```

## Task 2: SQLite Schema and Store

**Files:**
- Create: `internal/domain/models.go`
- Create: `internal/db/db.go`
- Create: `internal/store/store.go`
- Test: `internal/store/store_test.go`

- [ ] **Step 1: Write failing store test**

Test that opening a temporary database runs migrations, creates a library, upserts a series/book/file, records progress, and records a categorized file error.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/store`

Expected: FAIL because database/store packages are missing.

- [ ] **Step 3: Implement migrations and store methods**

Create tables from the design spec. Implement:

- `CreateLibrary(name, rootPath string)`.
- `ListLibraries()`.
- `UpsertSeries(libraryID int64, title string)`.
- `UpsertBook(seriesID int64, title, format string)`.
- `UpsertFile(bookID, libraryID int64, absPath, relPath string, size int64, mtime time.Time, ext string)`.
- `SaveProgress(bookID int64, pageIndex int)`.
- `RecordFileError(input domain.FileErrorInput)`.
- `ListFileErrors()`.

- [ ] **Step 4: Verify**

Run: `go test ./internal/store`

Expected: PASS.

- [ ] **Step 5: Commit**

Run:

```bash
git add internal/domain internal/db internal/store
git commit -m "feat: add SQLite schema and store"
```

## Task 3: ZIP Archive Reader

**Files:**
- Create: `internal/archive/zip.go`
- Test: `internal/archive/zip_test.go`

- [ ] **Step 1: Write failing archive tests**

Create tests that build a temporary ZIP with entries `002.jpg`, `001.png`, `notes.txt`, then assert image pages are returned as `001.png`, `002.jpg` and page bytes stream correctly.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/archive`

Expected: FAIL because archive reader is missing.

- [ ] **Step 3: Implement ZIP reader**

Implement:

- `ListPages(path string) ([]domain.Page, error)`.
- `OpenPage(path string, pageIndex int) (io.ReadCloser, string, error)`.
- Image extension filtering for `.jpg`, `.jpeg`, `.png`, `.webp`, `.gif`.
- Natural-enough stable sorting by normalized entry name.

- [ ] **Step 4: Verify**

Run: `go test ./internal/archive`

Expected: PASS.

- [ ] **Step 5: Commit**

Run:

```bash
git add internal/archive
git commit -m "feat: add CBZ ZIP page reader"
```

## Task 4: Scanner Service

**Files:**
- Create: `internal/scanner/scanner.go`
- Test: `internal/scanner/scanner_test.go`
- Modify: `internal/store/store.go`

- [ ] **Step 1: Write failing scanner tests**

Create a temporary library containing:

- `Series A/book1.cbz` with one small image.
- `Series A/empty.cbz` as a zero-byte file.
- `root-book.zip` with one small image.

Assert scan creates `Series A`, `Unsorted`, two valid books, one `empty_file` error, and does not abort.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/scanner`

Expected: FAIL because scanner is missing.

- [ ] **Step 3: Implement scanner**

Implement directory walking, `.cbz`/`.zip` detection, immediate-parent series derivation, `Unsorted` root grouping, file stat indexing, and categorized error recording for zero-byte files and unreadable inputs.

- [ ] **Step 4: Verify**

Run: `go test ./internal/scanner ./internal/store ./internal/archive`

Expected: PASS.

- [ ] **Step 5: Commit**

Run:

```bash
git add internal/scanner internal/store
git commit -m "feat: add resumable library scanner"
```

## Task 5: Application Service and HTTP API

**Files:**
- Create: `internal/service/service.go`
- Create: `internal/httpapi/server.go`
- Test: `internal/httpapi/server_test.go`
- Modify: `cmd/foliospace-reader/main.go`

- [ ] **Step 1: Write failing HTTP tests**

Use `httptest` with a temp config/database/library and assert:

- `GET /api/libraries` returns JSON.
- `POST /api/libraries/:id/scan` starts/completes a scan.
- `GET /api/series` returns indexed series.
- `GET /api/books/:id/pages` returns page metadata.
- `GET /api/books/:id/pages/0` returns image bytes.
- `GET /api/errors` returns file errors.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/httpapi`

Expected: FAIL because service/API is missing.

- [ ] **Step 3: Implement service facade and routes**

Expose application methods for libraries, scans, series, books, pages, progress, jobs, and errors. Implement HTTP handlers as thin adapters with JSON responses and image streaming.

- [ ] **Step 4: Verify**

Run: `go test ./...`

Expected: PASS.

- [ ] **Step 5: Commit**

Run:

```bash
git add cmd internal/service internal/httpapi
git commit -m "feat: expose reader HTTP API"
```

## Task 6: React/Vite Web UI

**Files:**
- Create: `web/package.json`
- Create: `web/index.html`
- Create: `web/src/main.tsx`
- Create: `web/src/App.tsx`
- Create: `web/src/api.ts`
- Create: `web/src/styles.css`
- Modify: `internal/httpapi/server.go`

- [ ] **Step 1: Create frontend shell**

Add a React/Vite TypeScript app with views for libraries, series/books, reader, jobs, and errors. Keep UI dense and operational.

- [ ] **Step 2: Wire API client**

Implement fetch helpers for the endpoints in the spec. Use local component state and simple loading/error states.

- [ ] **Step 3: Serve frontend from Go**

Configure the Go server to serve `web/dist` for non-API paths in production.

- [ ] **Step 4: Verify**

Run:

```bash
npm --prefix web install
npm --prefix web run build
go test ./...
```

Expected: frontend build succeeds and backend tests pass.

- [ ] **Step 5: Commit**

Run:

```bash
git add web internal/httpapi
git commit -m "feat: add web library reader UI"
```

## Task 7: Docker Packaging and Smoke Verification

**Files:**
- Create: `Dockerfile`
- Create: `docker-compose.yml`
- Modify: `README.md`

- [ ] **Step 1: Add Docker build**

Create a multi-stage Dockerfile that builds the Vite frontend, builds the Go binary, and runs a minimal final image exposing port `8080`.

- [ ] **Step 2: Add compose example**

Map:

- `./data/config:/config`
- `./data/library:/library:ro`
- `8080:8080`

- [ ] **Step 3: Verify**

Run:

```bash
docker build -t foliospace-reader:dev .
docker run --rm -p 8080:8080 -v "$PWD/data/config:/config" -v "$PWD/data/library:/library:ro" foliospace-reader:dev
```

Expected: homepage responds on `http://localhost:8080`.

- [ ] **Step 4: Browser smoke test**

Open the UI, confirm libraries render, scan can be triggered, errors view loads, and reader can display a generated test CBZ.

- [ ] **Step 5: Commit and push**

Run:

```bash
git add Dockerfile docker-compose.yml README.md
git commit -m "chore: add Docker packaging"
git push -u origin main
```

## Plan Self-Review

- Spec coverage: covers the Go single-service architecture, SQLite persistence, scanner, CBZ/ZIP reading, HTTP API, compact UI, Docker deployment, and MCP-ready service boundary.
- Deferred by design: Komga import, OPDS, CBR/RAR, PDF, EPUB, metadata editing, authentication, and full MCP server implementation.
- Placeholder scan: no implementation task depends on unspecified behavior; each deferred feature is explicitly outside MVP.
- Type consistency: package boundaries are stable across tasks: `domain`, `store`, `archive`, `scanner`, `service`, `httpapi`.

