# FolioSpace Reader MVP Design

## Goal

FolioSpace Reader is a lightweight self-hosted reader service for a NAS-hosted manga and book library. The first version must replace the parts of Komga that matter for private use: fast startup, observable scanning, explainable file failures, persistent reading progress, and CBZ/ZIP reading without whole-library preprocessing.

## Product Boundary

Version 1 is intentionally narrow.

- Scan one or more configured library roots.
- Index files into libraries, series, books, and files.
- Browse series and books from a web UI.
- Read CBZ/ZIP books page by page through the backend.
- Extract and cache one cover per book from the first readable page.
- Record file-level errors with real paths and categorized causes.
- Keep running when one file is broken.
- Preserve SQLite data and cached covers under `/config`.
- Treat `/library` as a read-only mounted content root.

Version 1 does not import Komga data, implement OPDS, edit metadata, run advanced media analysis for every file, or support multi-user administration. These are later extensions.

## Recommended Architecture

Use a single Go service that owns the HTTP API, SQLite database, scan worker, archive reader, and static frontend hosting. Build the React/Vite frontend into static assets and serve them from the Go binary or container filesystem.

This keeps deployment to one Docker image:

- `/config`: SQLite database, cover cache, runtime config.
- `/library`: read-only manga/book library root.
- Port `8080`: web UI and API.

The Go service is split into application services before HTTP handlers are written. Scan, book lookup, page streaming, cover generation, job reporting, and error reporting must be callable from the HTTP layer and later from an MCP layer without duplicating business logic.

## MCP Position

MCP is a first-class future integration point, but not the main client protocol for version 1. The core protocol remains HTTP because the web UI, future mobile clients, and simple NAS debugging should not depend on an agent runtime.

The V1 codebase must still reserve a clean MCP path:

- Domain and application services live outside HTTP handlers.
- Job and error APIs return structured data, not rendered text.
- File diagnostics expose stable error codes and paths.
- Scan and analyze operations accept explicit input structs and return job IDs.

The first MCP server can later expose tools such as `list_libraries`, `scan_library`, `get_scan_jobs`, `get_file_errors`, `explain_file_error`, `analyze_book`, `search_books`, and `get_book_diagnostics`.

## Series Derivation

For MVP, series is derived from the immediate parent directory of each supported file. This is deliberately conservative:

- `/library/Series A/book.cbz` maps to series `Series A`.
- `/library/Author/Series B/book.cbz` maps to series `Series B`.
- Files directly under the library root map to a synthetic `Unsorted` series.

This avoids hard-coding a single NAS directory convention. Later metadata enrichment can merge, split, or rename series without changing the file index.

## Data Model

SQLite tables:

- `libraries`: configured roots and display names.
- `series`: parent-directory grouping inside a library.
- `books`: display title, series link, format, page count, cover status, analyzed status.
- `files`: absolute or root-relative file path, size, mtime, extension, hash status.
- `pages`: page index and archive entry name for analyzed books.
- `scan_jobs`: job status, counters, timestamps, cancellation state.
- `job_events`: append-only scan/analyze events for observability.
- `read_progress`: book ID, page index, updated timestamp.
- `file_errors`: file path, error code, message, job ID, timestamps.

Paths stored in the database should include both the library-relative path and the resolved absolute path used at scan time. UI and diagnostics can show the real path, while future moves can use relative paths to recover.

## Error Taxonomy

File errors must be categorized rather than stored as opaque strings.

Initial codes:

- `file_missing`
- `permission_denied`
- `empty_file`
- `unsupported_format`
- `archive_open_failed`
- `archive_empty`
- `archive_page_decode_failed`
- `path_encoding_error`
- `case_conflict`
- `mount_missing`
- `unknown_io_error`

Every error row stores path, library ID, optional book/file ID, job ID, code, human-readable message, and first/last seen timestamps.

## Scan Behavior

Scanning is incremental and resumable enough for MVP:

- Directory traversal discovers candidate files without blocking on media analysis.
- Supported P0 file extensions are `.cbz` and `.zip`.
- Unsupported files can be ignored by default or recorded as `unsupported_format` when they look like book/archive inputs.
- The scanner upserts libraries, series, books, and files based on path and file stat.
- A bad file records a `file_errors` row and does not abort the job.
- Job counters track discovered files, indexed files, skipped files, errors, and analyzed books.
- `scan_jobs` and `job_events` make progress visible in the UI and API.

Hashing is not required for the first closed loop. The data model should allow adding `hash` and `hash_status` later.

## CBZ/ZIP Reading

The backend reads pages on demand:

- Opening a book loads the ZIP central directory.
- Page entries are sorted by normalized archive entry name.
- Non-image entries are skipped.
- `GET /api/books/:id/pages` returns page count and page metadata.
- `GET /api/books/:id/pages/:page` streams one image response.
- The service does not extract all pages to disk.
- The service does not pre-generate full-library page caches.

Cover generation uses the first readable image entry and writes a cached cover under `/config/cache/covers`. Failure records a file error and keeps the book visible.

## HTTP API

Initial endpoints:

- `GET /api/libraries`
- `POST /api/libraries`
- `POST /api/libraries/:id/scan`
- `GET /api/series`
- `GET /api/series/:id/books`
- `GET /api/books/:id`
- `GET /api/books/:id/cover`
- `GET /api/books/:id/pages`
- `GET /api/books/:id/pages/:page`
- `PUT /api/books/:id/progress`
- `GET /api/jobs`
- `GET /api/jobs/:id/events`
- `GET /api/errors`
- `POST /api/books/:id/analyze`

The API returns JSON for metadata and direct image bytes for covers/pages.

## Web UI

The first UI is a compact operational reader, not a marketing site.

Primary views:

- Library status: configured libraries, scan button, latest job state.
- Series list: searchable list with counts and latest activity.
- Book grid/list: cover, title, format, page count, error state.
- Reader: page image, previous/next controls, page index, progress persistence.
- Jobs: active/recent jobs and event stream.
- Errors: filterable table of file errors with code, path, message, and timestamp.

The UI should favor dense, readable NAS-admin ergonomics over decorative layout.

## Testing Strategy

Backend tests lead the implementation:

- Scanner indexes a temporary library with nested CBZ files.
- Scanner records an empty file error and continues.
- ZIP page analyzer sorts image entries and ignores non-image entries.
- Page endpoint streams the expected image bytes.
- Cover extraction caches the first page or records a categorized error.
- Progress API persists and reloads page position.

Frontend tests can stay lightweight for MVP. Build verification and a browser smoke test are enough after the API-backed flow exists.

## First-Phase Acceptance

The MVP is complete when:

- Docker starts and the homepage responds within 5 seconds on normal NAS-class hardware.
- A mounted library such as `/volume2/ComicCenter` can be scanned without all files being pre-analyzed.
- Series and books appear in the web UI.
- A CBZ opens and pages can be turned through backend streaming.
- Scan errors appear in the UI and map to real file paths.
- One broken file does not block the rest of the scan.
- Restarting the container preserves database state and reading progress under `/config`.

