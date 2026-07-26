# FolioSpace DOS client launch handoff

Status: deployed on `192.168.10.175:18080`

Backend baseline: FolioSpace Library commit `17c50e3`

Consumer: SpatialEMU Windows and other FolioSpace clients

Platform: IBM-compatible DOS only (`platform=dos`)

## 1. Client goal

The client downloads one authenticated DOS archive, verifies the exact archive bytes, extracts it into a private writable workspace, resolves a program inside that workspace, and launches it through DOSBox Staging.

The service never extracts or executes DOS content. PC-98 remains a separate `pc98` platform and must not enter this flow.

## 2. Capability discovery

Call:

```http
GET /api/client/info
Authorization: Bearer <token>
```

Enable archive-backed DOS launch only when:

```text
capabilities.dosArchiveLaunchV1 == true
```

The deployed service returns `capabilities` as an object. Do not expect the capability to be an array item.

## 3. Catalog discovery

DOS games are available through the normal paged game API:

```http
GET /api/client/games?platform=dos&limit=100&offset=0
Authorization: Bearer <token>
```

Canonical fields:

```json
{
  "id": 18728,
  "assetType": "game",
  "title": "仙剑奇侠传",
  "platform": "dos",
  "romSetName": "DOS",
  "format": "zip",
  "fileName": "仙剑奇侠传.zip",
  "size": 20272189,
  "sha1": "a6030fe6bc5d0b2f47a2c5174c6700183206527f",
  "emulatorHint": "dosbox-staging",
  "inputProfile": "standard",
  "coverUrl": "/api/games/18728/cover?v=game-cover-refresh-20260714",
  "manifestUrl": "/api/client/games/18728/manifest",
  "downloadUrl": "/api/client/games/18728/file"
}
```

Use `coverUrl` for the game shelf. The cover request also requires the same Bearer token.

The platform filter can be built from:

```http
GET /api/client/games/facets?platform=dos
Authorization: Bearer <token>
```

## 4. Manifest contract

Fetch the manifest before downloading:

```http
GET /api/client/games/{gameId}/manifest
Authorization: Bearer <token>
```

Resolved curated example:

```json
{
  "game": {
    "id": 18728,
    "title": "仙剑奇侠传",
    "platform": "dos",
    "format": "zip",
    "fileName": "仙剑奇侠传.zip",
    "size": 20272189,
    "sha1": "a6030fe6bc5d0b2f47a2c5174c6700183206527f",
    "emulatorHint": "dosbox-staging"
  },
  "fileUrl": "/api/client/games/18728/file",
  "entryFile": "PAL!.EXE",
  "updatedAt": "2026-07-26T08:00:00Z",
  "files": [
    {
      "name": "仙剑奇侠传.zip",
      "size": 20272189,
      "role": "entry",
      "url": "/api/client/games/18728/files/0",
      "checksum": "a6030fe6bc5d0b2f47a2c5174c6700183206527f"
    }
  ],
  "dosLaunch": {
    "entrySource": "curated",
    "installDirectory": null,
    "workingDirectory": null,
    "dosboxConfig": null,
    "arguments": [],
    "candidates": [
      { "path": "PAL!.EXE", "kind": "exe" },
      { "path": "PLAY.BAT", "kind": "bat" },
      { "path": "INSTALL.EXE", "kind": "exe" }
    ],
    "keymapHints": {
      "Enter/Ctrl/Space": "对话、调查、菜单选择"
    }
  }
}
```

Unresolved games return an explicit `entryFile: null`. This is a valid result, not a backend error:

```json
{
  "entryFile": null,
  "dosLaunch": {
    "entrySource": "unknown",
    "installDirectory": null,
    "workingDirectory": null,
    "dosboxConfig": null,
    "arguments": [],
    "candidates": [
      { "path": "GAME/PLAY.EXE", "kind": "exe" },
      { "path": "GAME/SETUP.EXE", "kind": "exe" }
    ]
  }
}
```

## 5. Download and cache

Download `fileUrl`, `downloadUrl`, or the entry in `files[]`. All are authenticated.

Required client behavior:

1. Send the Bearer token on the archive request.
2. Stream the response to a temporary file instead of buffering it in memory.
3. Require a successful HTTP status and verify `Content-Length` when present.
4. Compute SHA-1 while downloading and compare it with both `game.sha1` and `files[0].checksum`.
5. On mismatch, delete the temporary file and do not extract or launch it.
6. Move the verified archive into a revision-specific cache only after verification succeeds.

Recommended cache key:

```text
DOS/{gameId}/{sha1}/archive.zip
DOS/{gameId}/{sha1}/workspace/
```

Changing SHA-1 means a new archive revision. Do not reuse an old extracted workspace or saved entry selection for a different SHA-1.

## 6. Safe extraction

Extract only on the client into the private per-game workspace.

Reject an archive member when it:

- is absolute, drive-prefixed, UNC-prefixed, empty, or contains NUL;
- contains `.` or `..` path segments;
- escapes the workspace after path normalization;
- is a symlink, hard link, junction, or reparse-point escape;
- collides with another path under case-insensitive Windows comparison;
- exceeds client limits for entry count, individual size, total expanded size, or compression ratio.

Preserve subdirectories and original member names. Never flatten the archive.

## 7. Entry selection priority

After extraction, resolve the launch entry in this order:

1. A user-selected local override for the same `gameId + sha1`, if it still exists safely.
2. The service `entryFile` when `entrySource` is `curated` or `dosboxConfig`, if it still resolves to exactly one extracted member.
3. A first-launch chooser built from `dosLaunch.candidates`.

Never infer an authoritative entry only because an archive contains one EXE. Never execute the first candidate automatically.

The chooser may rank likely game programs above tools, but should visibly mark or demote names containing:

```text
SETUP, INSTALL, CONFIG, PATCH, UPDATE, UNINST, UNINSTALL
```

Candidate matching is case-insensitive, but the selected relative path should preserve the manifest/archive spelling.

Suggested local selection record:

```json
{
  "gameId": 18758,
  "archiveSha1": "<current archive SHA-1>",
  "entryFile": "GAME/PLAY.EXE",
  "workingDirectory": "GAME",
  "selectedAt": "<timestamp>"
}
```

Provide a later `Change startup program` action so the user can reopen the chooser.

## 8. Build the DOSBox launch plan

Create a structured launch plan rather than a host-shell command string:

```text
workspaceRoot      = verified extraction directory
entryFile          = selected archive-relative BAT/COM/EXE path
installDirectory   = optional single DOS directory under the generated C-drive root
workingDirectory   = dosLaunch.workingDirectory, otherwise parent(entryFile)
arguments[]        = dosLaunch.arguments only for the authoritative backend entry
dosboxConfig       = optional validated extracted config path
```

Rules:

- Validate every path again immediately before launch.
- If `installDirectory` is null, mount only `workspaceRoot` as the DOS C drive.
- If `installDirectory` is present, create a private C-drive root and expose `workspaceRoot` below that single directory name before mounting the generated root as C.
- Set the DOS working directory before launching the selected program.
- Pass each argument as a literal DOS argument; do not interpret it as host-shell syntax.
- Do not use `cmd.exe /c`, PowerShell, or another host shell to run archive content.
- `keymapHints` are display help only and must not be executed as emulator configuration.
- If the user chooses a different candidate, do not reuse arguments from the backend's previous authoritative entry.
- Fetch the manifest on every launch. Treat `updatedAt` as launch-profile freshness metadata and cache archive bytes separately by SHA-1.

If the client uses an embedded DOSBox core, pass this structured plan directly to the core/session bridge. If it starts DOSBox Staging as a child process, generate a controlled local autoexec/config file and pass that config to DOSBox without interpolating untrusted host-shell commands.

## 9. Error handling

| Condition | Client behavior |
| --- | --- |
| `401` | Ask for a valid FolioSpace token; do not silently retry forever. |
| `404` | Treat the catalog item or revision as removed and clear its stale cache entry. |
| SHA-1 or size mismatch | Delete the download and abort extraction. |
| Unsafe/colliding archive path | Abort extraction and report the offending archive. |
| `entryFile: null` | Open the first-launch chooser. |
| Backend entry missing after extraction | Fall back to the chooser and mark backend metadata stale. |
| Empty candidate list | Allow manual browsing inside the extracted workspace, restricted to safe BAT/COM/EXE files. |
| DOSBox launch failure | Keep the verified cache and allow the user to choose another entry. |

## 10. Compatibility boundary

- This is additive Client API v1 behavior.
- Older clients may ignore `dosLaunch`.
- `platform=pc98` continues to use NP2kai and its existing multi-disk manifest.
- Console, arcade, Dreamcast, Saturn, PC-FX, and other manifests must not enter this extraction/entry chooser flow.

## 11. Acceptance games

Use these production records on the configured test network without embedding a token in source code:

- `gameId=18728`, `仙剑奇侠传`: resolved curated entry `PAL!.EXE`, cover available.
- `gameId=18758`, `F15战斗机`: unresolved curated command, `entryFile: null`, candidates available for chooser testing.
- `gameId=20624`, `龙骑士4`: resolved `PLAY.BAT` with `installDirectory: "DRA4"` so the client mounts the archive at `C:\DRA4`.

Client acceptance checklist:

1. DOS appears as one platform facet.
2. Cover and catalog metadata load with Bearer authentication.
3. Archive downloads incrementally and passes size/SHA-1 verification.
4. `18728` launches through the authoritative entry without showing the chooser.
5. `18758` shows the chooser and remembers the selection for the same SHA-1.
6. Changing the archive SHA-1 invalidates the extracted workspace and saved selection.
7. A traversal or case-collision test archive is rejected before any file can escape the workspace.
8. `20624` starts from `C:\DRA4` and proceeds beyond its absolute `UNIT.DAT` and `FLAG0` lookups.
