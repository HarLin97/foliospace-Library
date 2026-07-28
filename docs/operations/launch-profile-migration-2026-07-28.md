# Launch Profile Migration Report - 2026-07-28

Production target: `192.168.10.175:18080`

Policy: `arcade-strict-v1`

This migration changed SQLite catalog metadata and repaired missing single-file manifests. It did not rename, rewrite, move, or delete ROM, BIOS, CHD, archive, or cover files. Catalog-role reconciliation preserves each game's original `updated_at`, so migration does not reorder recent-game shelves.

## Result

| Scope | Processed | Ready | Dependency | Needs curation | Failed |
| --- | ---: | ---: | ---: | ---: | ---: |
| Strict arcade families (`arcade`, `mame`, `model2`, `cps1`, `cps2`, `cps3`, `neogeo`) | 8,175 | 14 | 3 | 8,158 | 0 |
| DOS | 1,897 | 1,859 | 0 | 38 | 0 |

The playable client catalog and facets include only `catalogRole=game` records. `needs-curation` games remain in SQLite and administrative views for repair, but are no longer advertised to native clients when strict launch resolution is known to reject them.

Exact audited fingerprints also repair stale catalog metadata. For example, a
verified `sf2.zip` formerly stored as `platform=arcade, romSetName=FBNeo` is
reclassified as `platform=cps1, romSetName=sf2, emulatorHint=fbneo`. This keeps
the platform filter and the runtime selected by the client aligned with the
published Resolver profile.

No migration statement failed. `Needs curation` is an intentional quarantine state rather than an execution failure; full per-game identity remains in the production database and is not copied into this public report because it includes private library titles and paths.

## Manifest Repair

Legacy single-file Model 2, Model 3, and NAOMI records missing `game_files` rows were backfilled only when the physical file still existed as a regular file and its byte size matched the indexed record. The migration is idempotent and does not add duplicate manifest rows.

## Resolver Verification

- 14 of 14 published strict arcade entries resolved successfully for the Windows 1.302 MAME 0.288 or pinned FBNeo runtime tuple.
- 199 of 199 Model 3, NAOMI, and NAOMI 2 entries resolved successfully with their pragmatic Supermodel/Flycast runtime policies.
- 1,859 of 1,859 published DOS entries resolved successfully with their deterministic curated launch contracts.
- 10 of 10 ordinary-platform control entries continued to resolve successfully: NES, PS1, N64, Saturn, PC-98, PS2, PSP, GameCube, Dreamcast, and curated DOS.
- Direct checks of quarantined records continued to return `409 runtime-profile-not-available`, confirming that the resolver did not manufacture compatibility.

## Explicitly Quarantined Samples

| Game ID | Title | Reason |
| ---: | --- | --- |
| 3089 | `hypreac2` | Indexed asset fingerprint no longer matches the audited MAME 0.288 asset. A separate matching `hypreac2` record remains ready. |
| 4658 | `neogeo` | BIOS package; stored as a dependency, not a launchable game. |
| 20623 | `龙霸天下` | DOS entry is still ambiguous and has no curated executable contract. |

## Remaining Audit Work

The original migration deliberately did not claim that all historical arcade archives were compatible. The FBNeo addendum below now audits eligible archives against an exact DAT and core fingerprint; unmatched or rejected archives stay in `needs-curation`. Remaining MAME-only rows still need a fixed MAME 0.288 audit, dependency-closure verification, and an exact runtime profile before promotion. macOS MAME/FBNeo binaries must likewise be audited against their actual version or core hash rather than inheriting the Windows allow-list.

## FBNeo Profile Rebuild Addendum

The server now persists audited launch profiles in `game_launch_profiles` and their immutable source-container closure in `game_launch_profile_files`. Resolution reads these tables from the same SQLite database on every request; there is no separate in-memory profile index to become stale after a rebuild.

The official FBNeo Arcade DAT is deployment-supplied rather than bundled in the image. Place it at `/config/policies/fbneo-arcade.dat`, then run:

```sh
/app/foliospace-rebuild-launch-profiles \
  --dat=/config/policies/fbneo-arcade.dat
```

The command is explicit and idempotent. It streams the DAT and opens one ZIP central directory at a time, so it does not perform a full-library audit during normal service startup. It never renames, rewrites, moves, or deletes ROM files. Before production writeback, the 2026-07-28 isolated audit matched 8,322 indexed candidates: 7,714 passed exact ROM and dependency verification, while 608 remained `needs-curation`. The rebuild emitted 13,978 logical entry/dependency file rows and a deterministic profile revision derived from the DAT SHA-256.
