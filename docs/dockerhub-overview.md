# FolioSpace Library

FolioSpace Library is a self-hosted personal digital asset library for NAS, Docker, and local servers. It provides a unified indexing layer and client API for books, comics, PDFs, game ROM libraries, videos, and future spatial media clients.

It is not a cloud media service and does not distribute books, comics, ROMs, movies, or other media content. It indexes user-owned local files and exposes stable service URLs to web and native clients without leaking real NAS paths.

## 0.978 Release: Game Play-Time Sync

Release `0.978` adds profile-scoped game play-time synchronization for GameEMU and other native clients.

- Clients report cumulative active emulation time through idempotent launch-session heartbeats, so retries and out-of-order reports never double-count time.
- `GET` and `PUT /api/client/games/{gameId}/play-stats` provide total play seconds, launch count, and first/last played timestamps.
- MCP adds `foliospace.get_game_play_stats` and `foliospace.report_game_play_session` for trusted local agents.
- `/api/client/info` advertises `gamePlayStats: true` for capability detection.
- Service and MCP metadata report version `0.978`.

## 0.977 Release: Expanded Console and Arcade ROM Support

Release `0.977` adds canonical scanning, filtering, artwork, and complete launch manifests for six additional platforms.

- Dreamcast GDI/CDI/CHD packages retain every required track under one launchable game.
- Sega Saturn CUE/BIN and ISO games are counted by disc rather than by physical track file.
- NEC PC-FX supports CUE, CCD, TOC, CHD, M3U, multi-disc grouping, Pegasus metadata, and local cover folders.
- Nintendo 64 validates `.z64`, `.v64`, and `.n64` byte order and can stream the raw ROM from a supported single-ROM ZIP.
- NEC PC-98 adds validated floppy/hard-disk formats, CP932 title decoding, duplicate merging, multi-disk manifests, and artwork sidecars.
- Sega Model 2 preserves MAME ZIP shortnames and bytes, adds friendly titles and compatibility states, and keeps BIOS dependencies outside ordinary platform counts.
- Client API facets, manifests, MCP tools, web filters, and authenticated downloads use stable canonical platform metadata.
- Service and MCP metadata now report version `0.977`.

## 0.976 Release: Bounded Memory and Reader Layouts

Release `0.976` addresses a T0 memory-exhaustion risk on large NAS libraries and expands comic reader display controls.

- Image decoding and thumbnail transforms enforce source-size, pixel-count, output-size, and concurrency limits.
- PDF thumbnail rendering is handled only by the bounded background worker.
- Cover-wall cache misses use a bounded queue and batched SQLite writes, keeping the API responsive under thumbnail bursts.
- Docker deployments receive a 768 MiB Go memory budget and a 1.5 GiB Compose container limit by default.
- Comic single-page reading supports contain, fit-width, and fit-height modes with left- and right-handed controls.
- Service and MCP metadata now report version `0.976`.

## 0.975 Release: Large Game Library Stability

Release `0.975` is a stability and performance hotfix for large game libraries and NAS deployments.

- The web game catalog now loads a smaller first page to reduce initial cover-wall pressure.
- The Client Home API avoids concurrent SQLite section reads that could queue up on single-connection deployments.
- Game and collection private-state lookups now only read state for the current page of items.
- Game list sorting and filtering add SQLite expression indexes for title and platform-heavy browsing.
- Service and MCP metadata now report version `0.975`.

## 0.970 Release: Manual Collections and Game Library Controls

Release `0.970` adds a more flexible personal-library layer on top of indexed assets.

- User-defined manual collections can group books, games, and videos without moving files on disk.
- Game assets can now be marked favorite or liked from the web UI, Client API, and MCP.
- Game catalog browsing can filter by platform groups derived from indexed game collections.
- Game metadata helpers include provider discovery and `gamelist.xml` export for launcher-style integrations.
- Service and MCP metadata now report version `0.970`.

## 0.969 Release: PDF Metadata and Scan Excludes

Release `0.969` improves scan results for PDF-heavy libraries and mixed folders that contain generated artwork.

- PDF scans now read lightweight embedded Info metadata when available, mapping title, author, and subject to FolioSpace title, creator, and description fields.
- Libraries can define scan exclude directories from the web UI, API, or MCP.
- The scanner skips common generated folders such as `media`, `thumbnails`, `covers`, `__MACOSX`, and `@eaDir`, preventing artwork and sidecar folders from being indexed as books.
- Service and MCP metadata now report version `0.969`.

## 0.968 Release: Sortable Library Views

Release `0.968` improves large-library browsing in the web UI and keeps the Client API version aligned.

- Collection pages can now sort by title, recently added time, item count, or primary type, with ascending/descending direction controls.
- Game and video catalog pages now expose simple sort controls for title, recently added time, and platform where applicable.
- Collection API responses include `addedAt`, and paginated `/api/collections` supports type-based sorting for client integrations.
- Game cover lookup continues to support local `media/<rom-name>/boxFront.*` artwork beside ROM files, so curated arcade/console covers can be displayed without remote scraping.
- Service and MCP metadata now report version `0.968`.

## 0.966 Release: Embedded Comic Metadata

Release `0.966` adds embedded JSON metadata support for comic ZIP/CBZ archives.

- ZIP/CBZ scans now read small embedded metadata JSON files such as `metadata.json`, `info.json`, `comicinfo.json`, and `元数据.json`.
- Metadata fields `name`, `author`, `description`, and `tags` map onto FolioSpace's existing book title, creator, description, and public tag fields without a database migration.
- Search now matches public archive tags and creators, so tagged packs can be found through the web UI, Client API, and MCP-backed search flows.
- Book API responses merge public archive tags with profile-private tags while keeping user private state separate.

## 0.965 Release: Client Catalog APIs

Release `0.965` adds paginated catalog APIs for native iPad, iPhone, and Vision Pro clients.

- `GET /api/client/books` returns a client-safe paginated All Books catalog with `limit`, `offset`, `q`, `sort`, `direction`, and `format`.
- Book catalog responses include `manifestUrl`, cover URLs, thumbnail URLs, profile-scoped progress, favorite state, private status, tags, and ratings without exposing NAS file paths.
- `GET /api/collections` now has an optional paginated mode with `primaryType`, `limit`, `offset`, `sort`, `direction`, and `q`.
- Legacy `GET /api/collections` without query parameters still returns the original array shape for existing web UI compatibility.
- `/api/client/info` advertises `bookCatalog: true` and `collectionCatalog: true` for client capability detection.

## 0.961 Hotfix: Cleaner Shelves and Covers

Release `0.961` is a library cleanup and cover-refresh hotfix on top of `0.96`.

- ZIP/CBZ page listing now ignores macOS resource fork entries such as `__MACOSX/` and `._*`, preventing doubled page counts and broken placeholder pages in affected archives.
- Continue Reading, Favorites, Want to Read, and recent shelves now hide stale entries when the indexed file has been deleted or changed on disk.
- Book thumbnail cache keys were refreshed so corrected books no longer keep old generic placeholder covers after re-analysis.
- The service, Client API, and MCP metadata report version `0.961`.

## 0.96 Release: Fast Recent Scans

Release `0.96` focuses on faster day-to-day imports for very large libraries. When you add several new comics or books to a directory with thousands of existing files, you no longer need to kick off a heavy full-library scan.

- New "scan latest added" action in the Tasks page.
- Selectable recent limits for common import batches, such as 10, 20, 50, 100, or 200 files.
- Recent scans index only new or changed files under a selected library or subdirectory.
- Duplicate running scans for the same library and target path are reused instead of creating overlapping jobs.
- HTTP API supports `POST /api/libraries/:id/scan` with `mode: "recent"`.
- MCP exposes `foliospace.scan_recent`, so local agents can trigger the same fast scan path.
- `/api/client/info` advertises `recentScan: true` for client capability discovery.

Example API request after adding new files under a large manga folder:

```json
{
  "mode": "recent",
  "path": "/library/韩漫",
  "recentLimit": 20
}
```

## Quick Start

```bash
docker pull funland/foliospace-library:0.978
```

```bash
docker run -p 8080:8080 \
  -v /volume1/docker/foliospace-library/config:/config \
  -v /volume2/ComicCenter:/library:ro \
  -v /volume2/Books:/books:ro \
  -v /volume2/GameROMS:/games:ro \
  -e FOLIOSPACE_DIRECTORY_ROOTS=/library,/books,/games \
  funland/foliospace-library:0.978
```

Open `http://localhost:8080`. On a fresh `/config`, FolioSpace Library starts with a setup page for the first access key and first library path.

## Runtime Paths

- `/config`: SQLite database, generated covers/thumbnails, runtime cache.
- `/library`: default read-only mounted asset library root.
- `/books`, `/games`, `/movies`: optional read-only roots.
- `8080`: web UI and HTTP API.

## Key Environment Variables

```bash
FOLIOSPACE_CONFIG_DIR=/config
FOLIOSPACE_LIBRARY_DIR=/library
FOLIOSPACE_DIRECTORY_ROOTS=/library,/books,/games
FOLIOSPACE_ADDR=:8080
FOLIOSPACE_API_TOKEN=
FOLIOSPACE_SCAN_WORKERS=2
```

If `FOLIOSPACE_API_TOKEN` is empty, the web setup page can create the first access token and stores only a SHA-256 token hash in SQLite.

## Supported Areas

- EPUB, CBZ, ZIP, and PDF reading.
- Single-page, double-page, compact mobile, fullscreen, and webtoon-style comic/PDF modes.
- Structured reading progress and private state.
- Game ROM library indexing and client-safe launch manifests.
- Video library indexing and lightweight playback/transcode support.
- Scan jobs with progress, worker settings, errors, pause/cancel/resume, and targeted scan entry points.
- MCP server packages for local agent integration.

## Links

- Website: https://foliospace.app/
- GitHub: https://github.com/funland/foliospace-Library
- Client API docs: https://github.com/funland/foliospace-Library/blob/main/docs/api/client-v1.md
- MCP docs: https://github.com/funland/foliospace-Library/blob/main/docs/mcp/usage.md
