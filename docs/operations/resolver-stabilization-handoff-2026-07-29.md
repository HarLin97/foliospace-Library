# FolioSpace Resolver Stabilization Handoff

Date: 2026-07-29

## Purpose

Restore game launch availability on existing macOS and iPadOS clients, then
finish the launch resolver without coupling every client release to a server
release or a full profile migration.

The immediate recovery must not require a client update. A later client update
should introduce a stable emulator-core identity while the server continues to
support existing clients.

## Production Incident Snapshot

The production instance inspected during this incident reported service version
`0.991` and advertised the game launch resolver as available. The catalog itself
was reachable, but representative macOS and iPadOS FBNeo resolve requests failed
with `409 launch-profile-missing`.

The production curation summary showed:

- `total`: approximately 20,268 administrative game records
- `ready`: approximately 19,567 client-ready records
- `needsCuration`: approximately 671 records
- `checksumPending`: 2,625 files
- the latest checksum task was completed but processed only 2 files
- the legacy `targets.json` policy was unavailable

The resolver was therefore exposed before its target catalog, checksum backfill,
and mobile launch-profile migration were complete.

## Root Cause

The previous manifest flow returned a canonical game package and allowed the
client to select its own runtime. Client application updates did not affect the
server.

The new resolver requires a compatible persisted launch profile. Strict FBNeo
and MAME profiles are target-specific, and current Apple clients report a
build-specific SHA-256 derived from the whole statically linked application
executable. Rebuilding an application for an unrelated UI or product change can
therefore change the reported runtime fingerprint even when the emulator core
did not change.

This created two regressions:

1. A normal Apple client update can invalidate otherwise compatible launch
   profiles.
2. Resolver readiness and catalog publication became too tightly coupled, so an
   incomplete profile migration can make a large catalog unusable.

## Required Compatibility Position

### Existing clients

Existing macOS, iPadOS, iOS, visionOS, and tvOS clients must continue to work
without an immediate client update.

While the new resolver is not ready, the server must not advertise it as an
available capability. The resolve route must return a status that existing
clients treat as resolver unavailable (`404`, `405`, or `501`), allowing them to
use the established manifest flow.

The server must preserve all existing manifest, download, authentication, save
sync, and progress APIs.

### Future clients

A later client release should stop using the whole application executable hash
as the emulator-core identity. It should report:

- `clientVersion`: API and protocol compatibility only
- `runtimeId`: runtime family, such as `libretro` or `mame`
- `coreId`: emulator core family, such as `fbneo` or `bsnes`
- `coreBuildId`: stable core source revision, build configuration, and ABI
- `coreSha256`: hash of the core binary itself when it is independently
  addressable
- `contentSet`: the ROM-set contract, such as `mame-0.288`

For statically linked cores, the client should embed an official signed runtime
manifest. A UI-only application rebuild must not change `coreBuildId`.

The server must support both generations during migration:

- old clients continue using their existing request shape or legacy manifest
  fallback;
- new clients prefer the stable runtime identity;
- existing users are not forced to upgrade before the resolver is re-enabled.

## Resolver Scope

Do not apply strict per-build fingerprinting to every emulator platform.

- Ordinary console and computer platforms should resolve by validated platform,
  runtime family, core family, ABI, and canonical manifest.
- FBNeo and MAME remain strict because the executable/runtime version, content
  set, parent ROMs, BIOS archives, and device dependencies are coupled.
- A missing profile for one runtime must not hide unrelated platforms or games.
- Catalog visibility and runtime certification must be separate states.
- A visible game without a compatible profile may expose an unavailable reason,
  but it must not be falsely marked as launchable.

## Runtime Target Catalog

Persist runtime targets independently from application releases. A target should
include at least:

```text
clientName
clientPlatform
architecture
runtimeId
coreId
coreBuildId
coreSha256
contentSet
trustSource
status
firstSeenAt
lastSeenAt
```

Suggested statuses:

```text
official
approved
pending
rejected
```

An official signed runtime catalog may be trusted automatically. Unknown
third-party targets should be deduplicated, rate-limited, and shown as pending
for administrator review. Unknown runtime reports must not automatically create
unbounded launch-profile records.

## T0 Recovery

1. Add or restore an independent resolver capability switch.
2. Default the capability to disabled until profile readiness checks pass.
3. When disabled, return `404` or `501` from the resolve route.
4. Verify existing clients use the legacy manifest and can start games again.
5. Test the recovery build on port `18081` before replacing port `18080`.
6. Preserve the production API token, SQLite database, policy directory, media
   mounts, artwork cache, private state, and play statistics.

If the current code cannot safely disable the resolver, use the preserved
pre-mobile-resolver service as a temporary recovery path. Do not run production
with a resolver capability that is known to be incomplete.

The recovery branch implements this switch as
`FOLIOSPACE_DISABLE_GAME_LAUNCH_RESOLVER=true`. While enabled,
`/api/client/info` advertises `gameLaunchResolver: false` and
`POST /api/client/games/{gameId}/resolve` returns `404`. The legacy manifest
endpoint remains unchanged.

## Game Curation UI

Extend Game Curation with:

- recognized runtime targets;
- pending runtime targets with approve and reject actions;
- official catalog trust status;
- per-game, per-platform, per-runtime, and whole-library rebuild scopes;
- `missingOnly` as the default operation;
- a separately confirmed force rebuild;
- dry-run counts and failure previews;
- processed, matched, skipped, failed, and elapsed task metrics;
- single-flight protection for overlapping rebuild tasks;
- explicit missing-policy, checksum, dependency, and runtime-fingerprint errors.

The UI must not report a successful full migration when a bounded task processed
only a small subset of the pending files.

## Suggested Administration APIs

Keep the existing client resolve contract backward compatible. Add or extend
administration endpoints along these lines:

```http
GET  /api/games/runtime-targets
POST /api/games/runtime-targets/{id}/approve
POST /api/games/runtime-targets/{id}/reject
POST /api/games/curation/rebuild-profiles
GET  /api/games/curation/task
```

The rebuild request should support:

```json
{
  "gameId": 0,
  "platform": "",
  "runtimeTargetId": "",
  "missingOnly": true,
  "dryRun": false
}
```

Only include nonzero/nonempty scope fields. A request without a scope represents
the whole catalog and requires explicit confirmation in the Web UI.

## Migration Procedure

Keep the resolver capability disabled throughout migration.

1. Install and validate the deployment-supplied FBNeo DAT and MAME listxml.
2. Install or generate the runtime target catalog for every supported physical
   Apple and desktop client target.
3. Complete bounded checksum backfill until `checksumPending` is zero or every
   remaining failure has a documented file-level reason.
4. Rebuild ordinary platform mappings without per-application hashes.
5. Rebuild audited FBNeo and MAME profiles for every approved runtime target.
6. Verify that the deterministic `profileRevision` changes.
7. Confirm the active resolver reads the same SQLite database that received the
   migration.
8. Run API and physical-device acceptance tests on port `18081`.
9. Enable the resolver capability only after the readiness gate passes.
10. Replace the production service on port `18080` and remove temporary
    containers, images, and verification copies.

Profile rebuilds are expected to be idempotent. They must not move, rename,
rewrite, or delete source ROM files.

## Readiness Gate

The server may advertise the resolver only when all required conditions are
true:

- required policy files are available;
- required canonical files have checksums;
- approved production runtime targets exist;
- representative ordinary-platform profiles resolve;
- representative FBNeo and MAME profiles resolve for each enabled strict target;
- the resolver and migration command use the same database;
- no exclusive migration task is running;
- the latest migration did not terminate with unreviewed failures.

Readiness must be computed from persisted state, not inferred from a successful
process exit alone.

## Acceptance Tests

The release is not ready until all of the following pass:

1. An existing macOS client starts a legacy-manifest game while the resolver is
   disabled.
2. An existing iPadOS client starts a legacy-manifest game while the resolver is
   disabled.
3. Ordinary NES/SNES/PS1/N64-style games resolve without a per-App SHA profile.
4. A representative macOS FBNeo/CPS game resolves and starts with an approved
   strict target.
5. A representative iPadOS FBNeo/CPS game resolves and starts with an approved
   strict target.
6. A representative MAME game selects only its audited content set.
7. An unknown runtime affects only its own launch request and does not empty the
   catalog.
8. A UI-only client rebuild retains the same `coreBuildId` and does not require
   a server rebuild.
9. A real emulator-core update produces a new target and only an incremental
   profile rebuild.
10. Older clients continue to use the existing manifest contract.

## Resource Safety

Checksum and profile work must remain bounded and observable. Limit concurrency,
avoid loading complete archives or policy catalogs into memory when streaming is
possible, and monitor NAS CPU and memory during migration. A migration must be
paused or failed safely before it can exhaust system memory.

Temporary containers and database copies must be named, tracked, and removed
after acceptance or rollback.

## Non-Goals

- Do not weaken authentication or allow a runtime report to grant content access.
- Do not silently substitute a nearby MAME or FBNeo content set.
- Do not require every existing client to update before service recovery.
- Do not publish unverified strict arcade combinations as guaranteed playable.
- Do not rescan or rewrite the entire media library merely to update runtime
  compatibility metadata.
