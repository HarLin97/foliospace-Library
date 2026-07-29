# Game Catalog Curation

FolioSpace separates file discovery from client publication. Scanning proves that a file exists and records stable identity where possible. Catalog curation then decides whether it is a launchable game, a dependency, or an item that still needs review. Native clients receive only launchable `game` records.

## 1. Automatic Post-Scan Analysis

Enable **Analyze after game scans** during first-run setup or under **Game Curation**. After a game or mixed library scan completes, FolioSpace starts one background catalog task. A second task is not created while the first one is running.

The base pass identifies ordinary launch entries and known dependency files. Strict arcade families remain `needs-curation` until an installed compatibility policy verifies the ROM set and creates at least one target-specific Launch Profile. This avoids advertising a game that every client would reject with `409 runtime-profile-not-available`.

Automatic analysis is saved only after setup explicitly writes game settings. Upgraded installations therefore do not unexpectedly start a full arcade audit on their first scan.

## 2. Curation Center

Open **Game Curation** in the web sidebar. The summary shows:

- total indexed records;
- ready games published to clients;
- records needing curation;
- dependencies hidden from clients;
- metadata and cover coverage;
- compatibility policy availability;
- the latest background task and its progress.

The record list can filter by state, platform, or search text. Typical issue codes are:

- `identity-missing`: rescan the single file so a stable hash can be recorded;
- `policy-pack-missing`: install the matching FBNeo DAT or MAME listxml package;
- `launch-profile-missing`: the archive failed the installed audit, usually because the set version is wrong or a parent, BIOS, or device archive is absent;
- `dependency`: the file is intentionally retained for another game's manifest and is not a standalone game.

Use **Analyze Catalog** after adding policies or repairing ROM dependencies. The operation changes database classification and profiles only; it does not rename, move, rewrite, or delete source ROM files.

## 3. Compatibility Policies

The default container paths are:

```text
/config/policies/fbneo-arcade.dat
/config/policies/mame0288lx.zip
/config/policies/targets.json
```

The FBNeo DAT and MAME listxml are not bundled with FolioSpace. Administrators must supply policy files that match the exact runtime/core versions used by their clients. `targets.json` supplies client/runtime fingerprints for target-specific profile generation. Paths are configurable in setup and Game Curation for installations with a different `/config` layout.

If policy files are absent, ordinary console and disc games can still be indexed. Strict arcade records remain visible to administrators as `needs-curation` and stay out of native-client facets until audited.

## 4. Cover Matching

Use **Match Local Covers** first. FolioSpace checks, in order, controlled same-name sidecars and common artwork layouts such as:

```text
Game.iso
Game.jpg
covers/Game.png
boxarts/Game.webp
media/Game/boxFront.jpg
```

The source library may be read-only; selected artwork can point at a readable local sidecar without changing the ROM. If **Libretro cover matching** is enabled, the separate network-assisted action can fall back to the existing Libretro thumbnail matcher for still-unmatched games. The job is bounded and sequential to avoid a cover-wall task exhausting NAS memory.

## 5. Metadata And Manual Corrections

Local metadata is the default. It preserves scanner-derived title, platform, format, region, hashes, and existing sidecar metadata without contacting an external service.

Hasheous can be enabled explicitly as an optional hash provider. FolioSpace sends CRC/SHA identity, stores returned candidates as metadata sources, and fills only missing display title, year, and publisher fields when the result is unambiguous. Ambiguous results require an administrator to select a candidate. Hasheous downtime or no match never blocks scanning, covers, manifests, or client launch.

The metadata editor remains the final authority. Administrators can correct display title, description, genres, developers, publishers, release date, player count, rating, and external links. Manual corrections are stored separately from source files and continue to work with all online providers disabled.

## API And Safety

All curation routes are protected by the normal FolioSpace API token. They intentionally expose policy status and administrative records, so they are not part of `/api/client`. Native clients continue using the stable client catalog and never receive NAS file paths.

Background tasks persist their last status. If the service restarts during a task, the previous task is reported as `interrupted`; it is not silently resumed or duplicated. Run the action again after confirming the policy files and free storage.
