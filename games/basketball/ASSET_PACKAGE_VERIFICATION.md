# Basketball asset package and immutable publication

Failure inventory before implementation:

- Copying whole research directories publishes ROM/NSF, music WAV masters, raw HTML, CPU fixtures, logs or credentials.
- Trusting filenames instead of manifest hashes; mixed source ROM identities; stale nested manifest references; an unsigned runtime file reference.
- Traversal, absolute/URL/backslash/encoded paths; nested symlinks; output symlinks or output outside the worktree; clobbering existing output.
- Nondeterministic timestamps/order/modes in archive or manifests; source changing during collection; missing local fallback paths.
- Audio list diverges from the actual catalog plus playback override merge; future music rendition omitted despite a valid manifest entry.
- Export provenance accidentally made public; release/listening acceptance flags silently changed.
- Dry-run reads secrets or accesses network; immutable existing OSS bytes overwritten; collision detected only after earlier uploads; incomplete OSS/CDN readback or wrong MIME.
- Packaging edits production registry, deployment, workflow, or game availability.

This preparation task creates local artifacts and publication tooling without uploading. The existing session authorization covers final publication after all required acceptance checks; this task does not add another permission step.

## Current candidate: 67-page atlas and music76

The current pinned package is `.local/basketball-asset-package/final-76-a/`; `final-76-b/` is an independently generated identical copy. It includes **448 files, 266,164,513 public bytes** and a **259,604,624-byte archive**. The three generated archive/manifest/provenance artifacts compare byte-for-byte. The earlier candidate1/candidate2 figures below remain historical evidence of the initial packaging step.

- Content identity: `2d05c1115cfc05aa3f87ae6ad7a49bad9bec7357ec0dda3917480739a573d68d`.
- Archive SHA-256: `3e442cdaac3e0e273ebf607bb4aa07a86ba3439cfc9d9ebd3c646034ac5ef0ab`.
- Package manifest SHA-256: `6a6017967444b0f5d061249c15f42fa62dfbd159738488f8decdd9fb7e8e627d`.
- Provenance SHA-256: `8942ac2cfac3ea7bc3d2f9c3e297bf27b0e931b4065f22314689381e42646bb8`.
- Compact release pin:82,718 bytes; SHA-256 `b190cbadf868b853a67e8fcab2a7c904204af142da9629900ec0d2574f5249c8`.

The packaged postresult atlas manifest contains67 pages. Music76 is exactly `music-076-intro-loop.mp3`, SHA-256 `7e0eb8fcaa8cbc9c426b93fc19d63ac06451874cd9008aecd11d4cb99e733469`. Its public playback metadata preserves sample_rate44100, loop_start_sample5465255, loop_end_sample32515748, loop_samples27050493, decoded_samples32515748 and loop=true from the verified central entry. The WAV master and delivery/source evidence manifests remain private.

The Node build allowlist had a retained RED for this actual filename before adding an exact76 exception. It still rejects `music-076.mp3`, `music-076-loop.mp3`, music76 WAV masters and arbitrary other `music-###-intro-loop.mp3` names. The music worker's8 packaging tests and the8 Node build/base cases pass. The real Node prepare hook and existing public collector verify all448 local fallbacks under the new pin. No upload occurred; `.local/basketball-asset-package/final-76-publication-plan.json` is a zero-write plan.

Use `BASKETBALL_ASSET_PACKAGE=.local/basketball-asset-package/final-76-a` for the next local application build. The old candidate1 package is intentionally rejected after repinning. Full evidence is `.local/basketball-asset-package/final-result.json`, with `music76-build-red.log`, `music76-build-after.log`, `music76-package-tests.log`, `final-76-node-tests.log`, `final-76-build-acceptance.log` and `final-76-local-verify.json`.

## Initial prepared result

`scripts/basketball/package-assets.py` creates a new worktree-local directory containing:

- `public/basketball/v1/graphics/`: court, courts, static-atlas, static-objects, raster, results and postresult runtime image/JSON exports.
- `public/basketball/v1/selection/`: manifest-selected court/player images and sanitized selection metadata.
- `public/basketball/v1/audio/`: explicit runtime effect WAV files and music MP3 delivery renditions, plus a reduced playback catalog.
- `asset-package.tar.gz`: these public files at their unchanged `basketball/v1/...` fallback paths, in sorted order with fixed timestamps, ownership and modes.
- `package-manifest.json`: every public path, MIME, byte length and SHA-256; archive/provenance hashes; ROM identity and content-addressed OSS target.
- `provenance.json`: separate source manifest hashes, generation/source/license notes and original acceptance/listening flags. This file is excluded from the public tree and archive.

The snapshot in `.local/basketball-asset-package/candidate-1/` has **447 public objects, 254,366,026 bytes**, with a **247,900,678-byte archive**. Candidate2 is a separate identical build. Music59..66 and75 have explicit delivery entries in this snapshot; music76 is intentionally not guessed from files lying in the export directory. A later valid music76 playback/encoding entry is automatically included on the next build; the synthetic fixture already checks this path.

| Group | Public files |
|---|---:|
| Graphics: court | 10 |
| Graphics: courts | 85 |
| Graphics: static-atlas | 129 |
| Graphics: static-objects | 33 |
| Graphics: raster | 2 |
| Graphics: results | 61 |
| Graphics: postresult | 10 |
| Selection | 73 |
| Audio | 44 |

The audio count comprises34 runtime effect WAVs,9 MP3 music renditions and one reduced playback JSON. The original music renditions dominate the size; packaging does not trim, transcode or replace music.

Content identity: `82b77475390f10555d1bcd7de67447eb0d21ddc6d7e69116ab7f14859e1bb367`.

Archive SHA-256: `e3e3d47de15d155ce19129eb602b3f2ffcfd351e9137b02d8402aaddd76af1c4`.

Package manifest SHA-256: `d0dd1613c313101993067aa6ba927d2a7251058a15429e490161aea8cb4b7f99`.

Provenance SHA-256: `00e3a021b9c1031ca1e829ea31197b574f90edce7a346d92b29f5c4cf150c4b2`.

These values identify the recorded snapshot, not all future exports. Any changed public bytes yield a new OSS prefix. Rebuild after an additional rendition or artwork export is accepted into its input manifest.

## Validation and inclusion rules

The packager does not glob-copy research directories. It follows explicit image/JSON references from the eight approved manifest groups and verifies the hashes before copying. Nested court manifests are checked against their original parent hashes; because public metadata removes private provenance, the public parent hashes are then recalculated against the exact sanitized child bytes. Selection's declared source atlas/image/manifest hashes are also checked. All encountered source ROM declarations must match `0fa28f60abb8d2ccbbeee90e79e3fa160a5cf32feb90f47ebf1a89168cff569b`.

Audio selection matches the production merge: `audio-catalog.json` base effects plus `audio-playback.json` effect overrides and explicit music delivery entries. Base effects require original export-manifest hashes; overrides require their source-manifest record; music requires an explicit `audio/mpeg` file, delivery SHA-256 and matching encoding-manifest record. Filename patterns do not grant inclusion by themselves. Track/effect IDs must agree with their delivery filenames and source/encoding records. A mismatched-track red test was retained before adding this check. MP3/WAV/PNG signatures and public MIME declarations are checked. Runtime data JSON is restricted to the known manifest/atlas and court map/ring/column names. The export directory's ROM, NSF, music WAV masters, raw HTML, CPU fixtures, logs and unused render files are never copied.

Relative paths reject traversal, absolute paths, URL/query/fragment characters, backslashes and percent-encoded spellings. Nested file/directory symlinks are rejected. The one existing worktree `.local/basketball` alias is allowed only when it resolves to the repository's established shared `.local/basketball` directory. Outputs require an explicit new directory inside the worktree; existing outputs and output symlinks are rejected. Verification rejects unlisted public files, altered bytes, changed provenance/archive hashes and a mismatched OSS target.

Public JSON contains the runtime graphic/layout data and minimal ROM identity, without local source paths, capture manifests or source logs. Original `accepted_for_release:false` / `listening_accepted:false` values remain in the separate provenance ledger; they are not converted to acceptance by this script. This work proves packaging integrity and reproducibility, not new listening, gameplay or distribution approval.

## Immutable publisher

`scripts/basketball/publish-assets.py` is read-only by default. It validates the package and prints a plan without reading credentials, opening network connections or writing remote objects. Its target uses the established `stoneage-assets` bucket, Shanghai OSS endpoint and `cdn.ichenj.com`, with a new immutable prefix:

```text
bga/basketball/v1-<full-content-SHA256>
```

The package's `assetsBaseURL` is this prefix under the CDN. Existing runtime URL suffixes and the local fallback tree remain unchanged. Production asset-base wiring, static-file deployment, registry, workflows and availability settings are outside this change; the game remains off by default.

Only an explicit future `--execute` invocation reads the existing `config/secrets/oss-access-key-id` and `oss-access-key-secret` files. Before any PUT, it verifies the existing platform snooker anchor and checks every prospective object's current bytes/MIME. A conflicting immutable object stops publication before writes begin. Missing objects use signed `x-oss-forbid-overwrite:true` PUTs and immutable cache headers. Every object is read back from both OSS and the public CDN with SHA-256/MIME verification; existing identical objects are reusable. A concurrent creator cannot trigger an overwrite. Credentials, signed headers and remote response bodies are not printed.

No upload was performed. Actual OSS credentials, bucket access and CDN availability therefore remain untested by this task.

## Tests and reproduction

The failure inventory and six test methods were written before implementation; initial missing-script failures are retained. Tests cover deterministic manifests/archive, runtime allowlisting, private provenance, source/hash corruption, path traversal, nested symlinks, output boundaries, tampering, future music76 inclusion, network-free publication planning, preflight collisions, successful mocked OSS/CDN readback, zero-write idempotent publication and corrupted CDN rejection. Temporary test files stay under `.local/asset-package-tests/`.

```sh
PYTHONDONTWRITEBYTECODE=1 python3 scripts/basketball/test_package_assets.py
PYTHONDONTWRITEBYTECODE=1 python3 scripts/basketball/package-assets.py --output .local/basketball-asset-package/new-candidate
PYTHONDONTWRITEBYTECODE=1 python3 scripts/basketball/package-assets.py --verify .local/basketball-asset-package/new-candidate
PYTHONDONTWRITEBYTECODE=1 python3 scripts/basketball/publish-assets.py --package .local/basketball-asset-package/new-candidate
```

After all required acceptance checks pass, publication can proceed under the existing session authorization using:

```sh
PYTHONDONTWRITEBYTECODE=1 python3 scripts/basketball/publish-assets.py --package .local/basketball-asset-package/new-candidate --execute --report .local/basketball-asset-package/oss-readback.json
```

Two real package builds produced byte-identical `package-manifest.json`, `provenance.json` and `asset-package.tar.gz`. The real publication plan reports `writesPerformed:0`. `.local/basketball-asset-package/result.json`, `verify.json`, `publication-plan.json`, `test-red.log`, `track-binding-red.log`, `source-revalidation.json` and `test.log` preserve the checks. No game production code, registry, deployment script or workflow was changed.
