# MAME content compatibility

MAME runtime releases may reuse an audited game's launch profile when trusted
listxml definitions prove that its content is equivalent. The resolver first
prefers an exact runtime profile. Cross-version reuse preserves that profile's
ID, revision, client/platform checks and files, echoes the requested runtime
tuple, and returns an optional `contentAudit` with the source runtime, policy,
both listxml SHA-256 values and the shared content-definition SHA-256.

Release 1.01 embeds reviewed evidence for 631 sets across MAME 0.287, 0.288 and
0.289 (628, 630 and 631 definitions respectively). These defaults apply when
`/config/policies/mame-content-equivalence.json` does not exist. They are hash
metadata, not ROMs, BIOS files, listxml archives, or private catalog records.

An administrator-owned file at that path replaces the bundled registry in full.
It is read on each resolution; updated evidence needs no restart. An explicit
empty registry disables cross-version reuse. Invalid or unreadable custom files
also disable cross-version reuse rather than silently falling back. Exact
profiles remain available. Clients cannot provide or override evidence.

## Updating a core release

1. Obtain official MAME listxml ZIPs and verify their published release digests.
2. Generate definitions for the affected games. The generator reads only the
   requested machines and recursively follows every parent/device dependency.
3. Review the output. Equal definition hashes permit reuse; unequal hashes do
   not. Do not manually change hashes to force equivalence.
4. Atomically install the reviewed registry in the configuration directory.
5. Test the new request tuple and the previous runtime, download checksums and a
   request with an unknown content set.

Example (paths are administrator-selected):

```sh
/app/foliospace-mame-content-registry \
  --listxml /config/policies/mame0288lx.zip,/config/policies/mame0289lx.zip \
  --sets strider2u
```

The JSON is written to stdout for review, not installed automatically. Pass
`--existing /config/policies/mame-content-equivalence.json` to retain other games
and versions; conflicting evidence for an existing set/version is rejected.
Future releases follow the same process without copying profiles or changing
server source. No version wildcard or numeric "newer means compatible" rule is
used. Only the selected games gain evidence; this is not a full-library rebuild.

The definition includes the complete parent/device closure, ROM names, sizes,
CRC/SHA-1, placement, merge semantics, BIOS order/defaults, disks and runnable
flags. Translated descriptions and build strings do not affect equivalence.
This is deliberately conservative; a genuinely different definition needs a
fresh ROM audit even if an emulator might still tolerate the older package.

Cross-version resolution additionally hashes the selected container files and
checks identity/size/mtime before and after reading. Missing dependencies remain
`dependency-missing`; changed bytes return `content-checksum-mismatch` (HTTP 409).
Unknown or unequal content definitions retain `content-set-mismatch` (HTTP 409).
This adds file I/O only to cross-version reuse; exact profile behavior is retained.

Content equivalence proves the required media contract, not emulator behavior,
graphics, sound, input, or performance. Client gameplay acceptance is separate.

## Bundled evidence provenance

The release registry is `internal/launchprofile/policies/mame-content-equivalence.json`.
Its SHA-256 is `6068f92b8e8babd76e323c7be8f72c3a4f9cc5e04fa197d624af1383ae76f47b`.
It was generated from the official MAME listxml archives:

| Version | Release archive | Archive SHA-256 |
| --- | --- | --- |
| 0.287 | [mame0287lx.zip](https://github.com/mamedev/mame/releases/download/mame0287/mame0287lx.zip) | `60444261c6f2378ddf026237b91db4f01a56e83f0597d076c5a6ccc8f734eecd` |
| 0.288 | [mame0288lx.zip](https://github.com/mamedev/mame/releases/download/mame0288/mame0288lx.zip) | `e7ef2186adde7aa029f8a1edabc3d248189da59bf1c156845d8c0340723b10d7` |
| 0.289 | [mame0289lx.zip](https://github.com/mamedev/mame/releases/download/mame0289/mame0289lx.zip) | `d5d8f40223aef8afddbb846c003402d4e348d07d3a867e6686267895ddbd2817` |

Each record also preserves the uncompressed listxml SHA-256. To reproduce, group
the registry's short names by version, generate each version with the tool, and
compare each complete record. Missing sets in older releases are not invented.
Coverage is bounded; this is not a claim that every MAME set or future release
is equivalent or playable.

## Upgrade to 1.01

Back up persistent `/config` data and retain the previous image for rollback.
Pull the 1.01 image using your existing Compose configuration and mounts. No
automatic catalog rebuild, ROM modification, or custom-policy replacement is
performed. Existing audited profiles are required; bundled evidence cannot make
an unaudited game ready. An existing custom registry keeps its previous scope;
review it before deciding whether to expand it or use the release defaults.

FBNeo retains the public 1.00 compatibility behavior: `coreSha256` and
`coreBuildId` are diagnostic for supported official clients, not App-build gates.
Client/platform/architecture, runtime family, ROM and dependency checks remain.

简体中文：缺少自定义配置时使用内置证据；已有自定义配置整体优先，错误或空配置不会回退放行。升级不会改写游戏、管理员配置或自动重建资料库。内容等价不代表已完成客户端实机游玩验收。
