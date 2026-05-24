# FolioSpace Reader

FolioSpace Reader is a lightweight self-hosted manga/book reader service for a NAS library. The first implementation targets CBZ/ZIP scanning and reading with SQLite persistence, observable scan jobs, categorized file errors, and a compact web UI.

## Runtime Layout

- `/config`: SQLite database, generated covers, runtime cache.
- `/library`: read-only mounted manga/book library.
- `8080`: web UI and HTTP API.

## Local Development

The backend requires Go 1.22 or newer. The frontend requires Node.js 20 or newer.

```bash
go test ./...
go run ./cmd/foliospace-reader
```

## Environment

```bash
FOLIOSPACE_CONFIG_DIR=/config
FOLIOSPACE_LIBRARY_DIR=/library
FOLIOSPACE_ADDR=:8080
```

## Git Remote

The project remote is:

```bash
git remote add origin http://192.168.10.158:8418/funland/FolioSpaceReader.git
```
