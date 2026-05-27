# FolioSpace Library MCP Reference

This MCP server gives agents a safe control surface over FolioSpace Library. It is for lookup, diagnostics, manifests, preferences, private state, progress, and scan operations. It is not the normal transport for page images, EPUB resources, or ROM bytes; agents should use the opaque HTTP URLs returned by the Client API when they need to point a native client at media.

## Build

```bash
go build -o ./bin/foliospace-mcp ./cmd/foliospace-mcp
```

## Runtime Environment

```bash
export FOLIOSPACE_BASE_URL=http://192.168.10.155:18080
export FOLIOSPACE_API_TOKEN=your-token-if-enabled
```

`FOLIOSPACE_BASE_URL` defaults to `http://127.0.0.1:8080` when omitted. `FOLIOSPACE_API_TOKEN` is optional and is forwarded as `Authorization: Bearer <token>`.

## MCP Client Config

Use an absolute path for `command`.

```json
{
  "mcpServers": {
    "foliospace-library": {
      "command": "/Users/deadseafu/Documents/FolioSpaceReader/bin/foliospace-mcp",
      "env": {
        "FOLIOSPACE_BASE_URL": "http://192.168.10.155:18080",
        "FOLIOSPACE_API_TOKEN": "your-token-if-enabled"
      }
    }
  }
}
```

## Tools

- `foliospace.client_info`: service name, version, supported formats, and capability flags.
- `foliospace.home`: continue reading, recent books, and collections.
- `foliospace.search_books`: search indexed books and comics.
- `foliospace.open_book_manifest`: open a CBZ/ZIP/EPUB client manifest by `bookId`.
- `foliospace.list_games`: list paginated client-safe ROM assets with `limit`, `offset`, `q`, `platform`, `format`, and `sort`.
- `foliospace.open_game_manifest`: open a ROM client manifest by `gameId`.
- `foliospace.get_preferences`: read client preferences such as interface language.
- `foliospace.save_preferences`: save client preferences.
- `foliospace.get_private_state`: read per-book private state.
- `foliospace.save_private_state`: save per-book private state.
- `foliospace.get_progress`: read reading progress.
- `foliospace.save_progress`: save reading progress.
- `foliospace.list_collections`: list collections.
- `foliospace.list_collection_assets`: list mixed collection assets by `collectionId`.
- `foliospace.scan_library`: start a library scan by `libraryId`.
- `foliospace.list_jobs`: list scan/import jobs.
- `foliospace.job_events`: list job events by `jobId`.
- `foliospace.list_errors`: list scan/import errors, optionally filtered by `jobId`.
- `foliospace.library_health`: service info plus job and error counts.

## Resources

- `foliospace://client/info`
- `foliospace://client/home`
- `foliospace://client/preferences`
- `foliospace://jobs`
- `foliospace://errors`
- `foliospace://health`

## JSON-RPC Examples

Initialize:

```json
{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"example","version":"0.1.0"}}}
```

List tools:

```json
{"jsonrpc":"2.0","id":2,"method":"tools/list"}
```

Open a game manifest:

```json
{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"foliospace.open_game_manifest","arguments":{"gameId":12}}}
```

Save interface language preference:

```json
{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"foliospace.save_preferences","arguments":{"interfaceLanguage":"zh-Hans"}}}
```

Read current health:

```json
{"jsonrpc":"2.0","id":5,"method":"resources/read","params":{"uri":"foliospace://health"}}
```

## Design Notes

MCP responses intentionally avoid NAS file paths. Book pages, EPUB resources, covers, and game files are exposed as service URLs from the HTTP API. Keep performance-sensitive reader and emulator paths on HTTP; use MCP for agent decisions, setup, troubleshooting, and orchestration.
