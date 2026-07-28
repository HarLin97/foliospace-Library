# Launch Profile Migration Report - 2026-07-28

Production target: `192.168.10.175:18080`

Policy: `arcade-strict-v1`

This migration changed SQLite catalog metadata and repaired missing single-file manifests. It did not rename, rewrite, move, or delete ROM, BIOS, CHD, archive, or cover files. Catalog-role reconciliation preserves each game's original `updated_at`, so migration does not reorder recent-game shelves.

## Result

| Scope | Processed | Ready | Dependency | Needs curation | Failed |
| --- | ---: | ---: | ---: | ---: | ---: |
| Strict arcade families (`arcade`, `mame`, `model2`, `cps1`, `cps2`, `cps3`, `neogeo`) | 8,175 | 14 | 3 | 8,158 | 0 |
| DOS | 1,897 | 1,859 | 0 | 38 | 0 |

The client catalog, search, recent/played shelves, logical platform collections, and facets now publish only `catalogRole=game` records. Dependency and `needs-curation` rows remain available to backend administration and direct ID-based audit flows.

No migration statement failed. `Needs curation` is an intentional quarantine state rather than an execution failure; full per-game identity remains in the production database and is not copied into this public report because it includes private library titles and paths.

## Manifest Repair

Legacy single-file Model 2, Model 3, and NAOMI records missing `game_files` rows were backfilled only when the physical file still existed as a regular file and its byte size matched the indexed record. The migration is idempotent and does not add duplicate manifest rows.

## Resolver Verification

- 14 of 14 published strict arcade entries resolved successfully for the Windows 1.302 MAME 0.288 or pinned FBNeo runtime tuple.
- 199 of 199 Model 3, NAOMI, and NAOMI 2 entries resolved successfully with their pragmatic Supermodel/Flycast runtime policies.
- 9 of 9 ordinary-platform control entries continued to resolve successfully: NES, PS1, N64, Saturn, PS2, PSP, GameCube, Dreamcast, and curated DOS.
- Direct checks of quarantined records continued to return `409 runtime-profile-not-available`, confirming that the resolver did not manufacture compatibility.

## Explicitly Quarantined Samples

| Game ID | Title | Reason |
| ---: | --- | --- |
| 3089 | `hypreac2` | Indexed asset fingerprint no longer matches the audited MAME 0.288 asset. A separate matching `hypreac2` record remains ready. |
| 4658 | `neogeo` | BIOS package; stored as a dependency, not a launchable game. |
| 20623 | `龙霸天下` | DOS entry is still ambiguous and has no curated executable contract. |

## Remaining Audit Work

This migration deliberately does not claim that all historical arcade archives are compatible. The remaining 8,158 strict-family rows need a fixed MAME 0.288 or FBNeo DAT audit, dependency-closure verification, and exact runtime profile before they can be promoted from `needs-curation`. macOS MAME/FBNeo binaries must likewise be audited against their actual version or core hash rather than inheriting the Windows allow-list.
