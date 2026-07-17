# FolioSpace Library

[Website](https://foliospace.app/) · [Docker Hub](https://hub.docker.com/r/funland/foliospace-library) · [Client API](docs/api/client-v1.md) · [MCP](docs/mcp/usage.md)

![FolioSpace Library hero](https://raw.githubusercontent.com/funland/foliospace-Library/main/docs/screenshots/hero.png)

FolioSpace Library is a personal digital asset library that runs on a NAS, Docker host, or local server. It provides a unified indexing layer and stable client service layer for Apple-device experiences across reading, games, spatial media, documents, photos, videos, and related audio collections.

It is not trying to become a complete Plex, Jellyfin, or Immich replacement. The first priority is personal asset indexing: scanning, identifying, covers/thumbnails, classification, search, favorites, recent access, progress, and private state. Dedicated clients such as a reader app, GameEMU, and Vision Pro experiences own the actual consumption UI.

The current implementation still starts from the FolioSpace Reader codebase and keeps the existing reading MVP operational while the model evolves toward `Asset` / `LibraryItem`.

Current release branch: `0.976`.

## Screenshots

![Unified home](https://raw.githubusercontent.com/funland/foliospace-Library/main/docs/screenshots/home.png)

![Reader mode](https://raw.githubusercontent.com/funland/foliospace-Library/main/docs/screenshots/reader.png)

![NAS and API](https://raw.githubusercontent.com/funland/foliospace-Library/main/docs/screenshots/nas-api.png)

![Game and video libraries](https://raw.githubusercontent.com/funland/foliospace-Library/main/docs/screenshots/game-video.png)

## License

Copyright (C) 2026 funland co.,Ltd.

The server and web application source in this repository is released under the GNU Affero General Public License v3.0. See [`LICENSE`](LICENSE).

FolioSpace Library indexes user-owned local files only. It does not distribute books, comics, ROMs, movies, or other media content.

## Runtime Layout

- `/config`: SQLite database, generated covers/thumbnails, runtime cache.
- `/library`: read-only mounted asset library root.
- `/books`, `/games`: optional read-only roots used by the default Docker compose example.
- `8080`: web UI and HTTP API.

Recommended NAS config root:

```text
/volume1/docker/foliospace-library
```

## Local Development

The backend requires Go 1.22 or newer. The frontend requires Node.js 20 or newer.

```bash
npm --prefix web install
npm --prefix web run build
go test ./...
go run ./cmd/foliospace-reader
```

## Environment

```bash
FOLIOSPACE_CONFIG_DIR=/config
FOLIOSPACE_LIBRARY_DIR=/library
FOLIOSPACE_DIRECTORY_ROOTS=/library,/books,/games
FOLIOSPACE_ADDR=:8080
FOLIOSPACE_API_TOKEN=
FOLIOSPACE_SCAN_WORKERS=2
```

Set `FOLIOSPACE_API_TOKEN` to require API authentication from environment variables. If it is empty, release `0.966` can create the first access token from the web setup page and stores only a SHA-256 token hash in SQLite. Native clients can send `Authorization: Bearer <token>`. The web UI stays publicly loadable, then prompts for the access token and receives an HttpOnly cookie so covers, pages, and EPUB iframe resources can load through normal browser requests.

Authentication helpers:

- `GET /api/auth/status`: returns whether token auth is enabled.
- `POST /api/auth/check`: accepts `{"token":"..."}` and returns `{"ok":true}` for a valid token.
- `POST /api/auth/logout`: clears the web auth cookie.

First-run setup helpers:

- `GET /api/setup/status`: returns whether the service has an access token and at least one library.
- `POST /api/setup/initialize`: creates the first access token and first library.
- `GET /api/config/directory-roots`: returns container-visible root directories for the setup picker.

## Client API v1

Detailed client integration docs are in [`docs/api/client-v1.md`](docs/api/client-v1.md).

- `GET /api/client/info`: service metadata, supported formats, and capability flags.
- `GET /api/client/home`: `continueReading`, `recentBooks`, and `collections` in one response.
- `GET /api/client/books`: paginated all-books catalog for native All / Library overview screens.
- `GET /api/collections?primaryType=comic&limit=60&offset=0`: paginated collection catalog for native Home / collection overview screens.
- `GET /api/client/books/:id/manifest`: a client-safe open manifest with `readerModes` and `defaultReaderMode`. CBZ/ZIP books include page URLs; EPUB books include spine, TOC, `resourceBaseUrl`, `coverUrl`, and progress; PDF books expose an opaque Range-capable stream URL for single-page, double-page, or webtoon/vertical-scroll client layouts.
- `GET /api/books/:id/reading-position` and `PUT /api/books/:id/reading-position/webtoon`: structured webtoon progress using a stable page key plus normalized page Y offset, with automatic legacy `/progress` fallback compatibility.
- `GET /api/client/games/:id/manifest`: a client-safe game launch manifest with platform, checksums, emulator hint, and opaque file URLs. Nintendo 64 `.z64`, `.v64`, and `.n64` games and PC-98 disk images are validated before indexing and downloaded as raw media bytes even when stored in a single-media ZIP. Dreamcast GDI, Saturn CUE, and PC-FX CUE/M3U manifests include the descriptor plus every required track in `files[]`.
- `GET/PUT /api/client/books/:id/private-state`: client-safe private status, favorite, rating, tags, and note sync.
- `GET/PUT /api/client/preferences`: client UI language and reader preference sync.
- `GET/PUT /api/settings/scan`: scan worker settings for NAS devices with different CPU and memory budgets.
- `GET /api/client/search`, `/api/client/books/favorites`, and `/api/client/books/private-status/:status`: private-state-aware discovery shelves.
- `POST /api/libraries/:id/scan` supports full scans, targeted `path` scans, and `mode: "recent"` scans for only the latest new or changed files.

Client API book and collection responses omit local NAS file paths. Returned cover, thumbnail, page, EPUB, game, and video URLs are opaque service URLs; clients should preserve query parameters because FolioSpace uses them for cache-compatible media refreshes while keeping older routes valid.

## Compact Mobile Reader

Release `0.91` added a compact mobile reading mode tuned for Safari and small screens:

- Bottom navigation is hidden while reading so books, comics, and PDFs get the full viewport.
- CBZ/ZIP comics support single-page, double-page, and vertical webtoon scrolling.
- Webtoon mode removes page buttons and keeps the long-strip comic body as the only scrollable area.
- PDF and image readers use a fixed three-region mobile layout to avoid Safari viewport reflow issues.
- EPUB gets an in-app fullscreen mode with a softened floating exit control.
- Direct page, cover, PDF, and EPUB resource URLs can carry token auth for browser surfaces that cannot attach headers.

This mode is meant for phone and tablet browsing. Native clients can still use the stable Client API manifests and implement their own reading UI.

## Release 0.93 Highlights

Release `0.93` adds anchored webtoon reading positions for long-strip comics and PDF/image webtoon layouts:

- Webtoon progress is saved as a stable page key plus normalized vertical offset, not just a global scroll percentage.
- Restoring webtoon mode prefers `pageKey` anchors and falls back to page index or document progress when needed.
- Switching between single, double, and webtoon modes no longer immediately overwrites the previous reading position.
- The service exposes `GET /api/books/:id/reading-position` and `PUT /api/books/:id/reading-position/webtoon` for native clients.
- Legacy `/progress` remains compatible through `locator: "webtoon:<fraction>"`, so older clients and MCP progress tools keep working.
- Webtoon image joins are hidden more cleanly to reduce visible seams between adjacent long-strip pages.

## Release 0.931 Hotfix

Release `0.931` is a PDF webtoon stability hotfix:

- PDF webtoon mode no longer renders every PDF page into canvas at once.
- Only the current PDF page and its near neighbors are rendered; distant pages use lightweight placeholders.
- PDF webtoon canvas DPR is capped to reduce memory pressure on Safari, iPadOS, and other mobile browsers.
- The release also includes the latest fullscreen comic reader display fixes merged from GitHub.

## Release 0.932 Hotfix

Release `0.932` is a large-library scan performance hotfix:

- Full-library scans now preload existing file index rows once per job instead of querying SQLite for every unchanged book.
- Unchanged CBZ/ZIP/PDF/7z entries can fast-skip from file metadata without reopening archives or forcing page analysis.
- Existing nested comic collections are not reclassified during normal unchanged scans, avoiding expensive churn on very large libraries.
- Root-level legacy collection migration is still preserved for older `Unsorted`-style imports.
- The change keeps the on-demand analysis model: unchanged comics do not need page metadata populated before they can be skipped.

## Release 0.95

Release `0.95` collects the post-`0.932` reader and library-state fixes:

- Narrow-screen cover cards now keep stable portrait frames, preventing tall or intrinsic cover images from stretching shelves and search results.
- Collection favorite and liked state is preserved during book reclassification, so private collection state follows the active collection instead of being left on old series IDs.
- The favorites page count now matches the visible favorite sections and no longer counts hidden empty collections.
- Image webtoon mode no longer leaves large black gaps in compact or fullscreen layouts after viewport width changes.
- Loaded webtoon images now size from the real image dimensions while unloaded placeholders keep scroll height stable.

## Release 0.96

Release `0.96` adds a fast import path for large libraries:

- Library scans can now run in `mode: "recent"` to index only the newest new or changed files under a library or subdirectory.
- The Tasks page exposes a "scan latest added" action with selectable limits for common manga import batches.
- Duplicate running scans for the same library and target path are reused instead of creating overlapping jobs.
- MCP now exposes `foliospace.scan_recent`, matching the HTTP API and allowing agents to trigger recent-file scans safely.
- Client capability discovery advertises `recentScan: true` from `/api/client/info`.

## Release 0.961 Hotfix

Release `0.961` is a cleanup and cover-refresh hotfix:

- ZIP/CBZ archive listing ignores macOS resource fork files such as `__MACOSX/` and `._*`, fixing doubled page counts and broken pseudo-pages in affected archives.
- Continue Reading, Favorites, Want to Read, and recent shelves hide stale entries when the indexed file has been deleted or changed on disk.
- Thumbnail cache keys were refreshed so re-analyzed books do not keep old generic placeholder covers.
- Service, Client API, and MCP metadata report version `0.961`.

## Release 0.965

Release `0.965` adds paginated catalog APIs for native clients:

- `GET /api/client/books` returns a client-safe paginated All Books catalog with `limit`, `offset`, `q`, `sort`, `direction`, and `format`.
- Book catalog responses include `manifestUrl` so clients can open an item directly through the stable manifest route.
- `GET /api/collections` supports an optional paginated shape for Home / collection overview screens, including `primaryType=book` or `primaryType=comic`.
- Legacy `GET /api/collections` without query parameters remains an array response for existing web UI compatibility.
- `/api/client/info` advertises `bookCatalog: true` and `collectionCatalog: true`.

## Release 0.966

Release `0.966` adds embedded JSON metadata support for comic archives:

- ZIP/CBZ scans now read small embedded metadata JSON files such as `metadata.json`, `info.json`, `comicinfo.json`, and `元数据.json`.
- Archive metadata fields `name`, `author`, `description`, and `tags` map onto the existing book title, creator, description, and tag fields without a database migration.
- Search now matches public archive tags and creators, so tags like `C106`, `中文`, or custom pack tags can find the indexed book.
- Public archive tags are merged with profile-private tags in book API responses while keeping user private state separate.

## Release 0.969

Release `0.969` improves metadata and scan hygiene for mixed book/comic libraries:

- PDF scans now read lightweight embedded Info metadata when available: title, author, and subject are mapped to existing title, creator, and description fields.
- Libraries can define scan exclude directories from the web UI, API, or MCP.
- The scanner skips common generated folders such as `media`, `thumbnails`, `covers`, `__MACOSX`, and `@eaDir`, preventing artwork and sidecar folders from being indexed as books.
- Service, Client API, and MCP metadata report version `0.969`.

## Release 0.970

Release `0.970` improves game library organization and user-curated shelves:

- User-defined manual collections can now group books, games, and videos without moving files on disk.
- Game assets support profile-scoped favorite and liked state in the web UI, Client API, and MCP.
- Game catalog browsing can filter by platform groups derived from indexed game collections.
- Game metadata helpers expose provider information and `gamelist.xml` export for launcher-style integrations.
- Service, Client API, and MCP metadata report version `0.970`.

## Release 0.975

Release `0.975` is a stability and performance hotfix for large game libraries:

- The web game catalog now loads a smaller first page, reducing initial cover-wall pressure on NAS deployments with large ROM collections.
- The Client Home API no longer fans out concurrent SQLite reads for every home section, avoiding single-connection queue pressure during startup.
- Game and collection private-state lookups now only read state for the current page of items.
- Game list sorting and filtering add SQLite expression indexes for title and platform-heavy browsing.
- Service, Client API, and MCP metadata report version `0.975`.

## Release 0.976

Release `0.976` is a T0 memory-safety and reader ergonomics release:

- Image decode and thumbnail transforms now enforce source-size, decoded-pixel, output-pixel, and concurrency limits.
- PDF thumbnail rendering runs only in the bounded background worker instead of spawning synchronous render work on cache misses.
- Thumbnail cache misses use a bounded in-memory queue and batched SQLite writes, preventing cover walls from blocking the single database connection.
- The server applies a 768 MiB Go memory budget by default, while the Docker Compose example adds a 1.5 GiB container hard limit.
- Comic single-page reading adds contain, fit-width, and fit-height display modes plus left- and right-handed control layouts.
- Service, Client API, and MCP metadata report version `0.976`.

## MCP

Agent integration docs are in [`docs/mcp/usage.md`](docs/mcp/usage.md). The MCP server wraps the stable Client API for diagnostics, library lookup, manifests, favorites/private-status shelves, preferences, private reader state, game private state, progress, scan jobs, recent-file scans, scan worker settings, job control, manual collections, and collection access. Heavy media streams still use the HTTP URLs returned by the API.

End users can install the MCP binary on the machine where their agent client runs:

```bash
curl -fsSL https://foliospace.app/install-mcp.sh | sh
```

Release maintainers can build macOS/Linux MCP packages with:

```bash
VERSION=0.976 ./scripts/build-mcp-release.sh
```

## Product Direction

Detailed product direction and the proposed `Asset` / `LibraryItem` model are in [`docs/product/foliospace-library-direction.md`](docs/product/foliospace-library-direction.md).

Core asset types:

- Books and EPUBs.
- Comics and CBZ/ZIP archives.
- Game ROMs and ROM sets.
- PDFs, manuals, art books, guides, and reference documents.
- Photos, videos, Vision Pro spatial photos, and spatial videos.
- OSTs and audio material connected to games, books, and collections.

ROM support is for indexing and launching user-owned local content. FolioSpace Library does not distribute ROMs, provide download sources, or bundle pirated assets.

## Docker

Release `0.976` image tag:

```bash
docker pull funland/foliospace-library:0.976
```

For local verification:

```bash
mkdir -p data/config data/library data/books data/games
docker compose up --build
```

For a NAS deployment, mount your real libraries as read-only:

```bash
docker run -p 8080:8080 \
  -v /volume1/docker/foliospace-library/config:/config \
  -v /volume2/ComicCenter:/library:ro \
  -v /volume2/Books:/books:ro \
  -v /volume2/GameROMS:/games:ro \
  -e FOLIOSPACE_DIRECTORY_ROOTS=/library,/books,/games \
  funland/foliospace-library:0.976
```

Open `http://localhost:8080`. On a fresh `/config`, the setup page asks for an access key and lets you choose a container path such as `/library`, `/books`, or `/games`. If a directory is missing from the setup page, add a Docker volume mapping first; FolioSpace Library can only browse paths visible inside the container.

### Publishing

Docker Hub releases are built by GitHub Actions from Git tags. Configure these repository secrets before publishing:

- `DOCKERHUB_USERNAME`
- `DOCKERHUB_TOKEN`

Then create and push a version tag:

```bash
git tag v0.976
git push github v0.976
```

The workflow builds `linux/amd64` and `linux/arm64` images, then pushes `funland/foliospace-library:0.976` and `funland/foliospace-library:latest`.

## Current MVP Support

- P0 reading formats: `.cbz`, `.zip`, `.epub`.
- P0 game formats: `.nes`, `.sfc`, `.smc`, `.gba`, `.gb`, `.gbc`, `.nds`, `.3ds`, `.cia`, `.gdi`, `.cdi`, `.chd`, `.iso`, `.bin`, `.cue`, plus validated PC-98 floppy and hard-disk image formats. `.zip` and `.7z` are treated as ROM sets only when the library type is `game`; PC-98 ZIP ingestion is limited to one validated media image and does not accept `.7z`, RAR, or TAR containers.
- Series derivation: immediate parent directory, with root-level files grouped under `Unsorted`.
- Reading: backend streams one ZIP image entry or EPUB resource at a time.
- Games: backend indexes local ROM metadata and checksums, exposes client-safe launch manifests without NAS paths, and lazily caches supported Libretro boxart under `/config/cache/game-covers`. Dreamcast `.gdi`, Saturn `.cue`, and PC-FX `.cue`/`.m3u` sets are indexed as one launchable game; referenced track files remain dependencies instead of separate catalog records. PC-FX multi-disc folders are merged into one virtual M3U package, and Pegasus metadata plus local `media/.../boxFront.*` or `PC-FX Covers/* [正面].jpg` artwork is used when available. PC-98 media uses canonical `pc98` / `PC-98` / `np2kai` metadata, rejects firmware/tool/DOS support files, and exposes one byte-exact launch image per catalog record.
- Errors: empty files, archive open failures, walk errors, and unsupported future categories are recorded as structured rows.

Near-term expansion priority:

1. Keep existing EPUB/comic reader APIs stable.
2. Add game asset indexing for local ROMs and ROM sets.
3. Add spatial photo / spatial video indexing.
4. Move data model language from Book/Series toward Asset/LibraryItem after the first non-reading asset type is real.
