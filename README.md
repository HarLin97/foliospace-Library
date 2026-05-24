# FolioSpace Reader

FolioSpace Reader is a lightweight self-hosted manga/book reader service for a NAS library. The first implementation targets CBZ/ZIP scanning and reading with SQLite persistence, observable scan jobs, categorized file errors, and a compact web UI.

## Runtime Layout

- `/config`: SQLite database, generated covers, runtime cache.
- `/library`: read-only mounted manga/book library.
- `8080`: web UI and HTTP API.

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
FOLIOSPACE_ADDR=:8080
```

## Docker

For local verification:

```bash
mkdir -p data/config data/library
docker compose up --build
```

For a NAS deployment, mount your real library as read-only:

```bash
docker run -p 8080:8080 \
  -v /volume1/docker/foliospace-reader/config:/config \
  -v /volume2/ComicCenter:/library:ro \
  foliospace-reader:dev
```

Open `http://localhost:8080`, scan the configured library, then browse series and books.

## Current MVP Support

- P0 formats: `.cbz`, `.zip`.
- Series derivation: immediate parent directory, with root-level files grouped under `Unsorted`.
- Reading: backend streams one ZIP image entry at a time.
- Errors: empty files, archive open failures, walk errors, and unsupported future categories are recorded as structured rows.

## Git Remote

The project remote is:

```bash
git remote add origin http://192.168.10.158:8418/funland/FolioSpaceReader.git
```
