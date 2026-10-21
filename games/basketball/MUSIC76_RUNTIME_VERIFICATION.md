# Music76 delivery and native-media verification

Root reviewed MUSIC76_INTEGRATION_SOURCE.md and authorized the fixed 4304→41168 source anchor, a compound intro/prefix plus complete36,864-frame period, and platform option `loopStartSeconds`. No upload is authorized.

Failure inventory before implementation:

- Intro repeats at EOF or restore; complete source period shortened; source anchor silently moved.
- Incorrect offset at prefix/loop/end boundaries, long elapsed offsets or metadata delay.
- Stable snapshot epochs restart; a new same-track request incorrectly deduplicates.
- Paused ended music resumes at0; pending restart plays while paused/hidden/muted.
- Old ended/metadata/play callbacks seek, pause or revive a new owner's media.
- Invalid loop start interrupts a valid owner; invalid metadata leaves partial playback.
- Source fallback loses the mapped offset; playback failure disrupts game input.
- Existing whole-resource loops, finite75 and effect owner lifetimes regress.
- Browser time or mute drives original audio IRQ/work state; server restore uses video frames instead of media wall time.
- MP3 decoder padding or wrong seam reference shifts/corrupts the suffix; end-to-zero metrics used instead of end-to-loopStart.
- Claimed native gaplessness without measured ended/seek/play timing; shortened or accelerated file used for acceptance.

Tests and asset validation must precede the corresponding implementation/registration. Browser evidence must use the actual verified MP3 and retain lifecycle/offset/network/timing records.

## Verified delivery and source boundary

The initial 4304 anchor failed the existing −2 dB RMS limit and remains recorded. Root approved analysis of documented source D0 boundaries with the same complete36,864-frame period and unchanged limits. Boundaries 5342 and 6396 also failed;7449→44313 passed. No clock, note sequence, source seed or quality threshold was changed.

The registered MP3 is `music-076-intro-loop.mp3`, SHA-256 `7e0eb8fcaa8cbc9c426b93fc19d63ac06451874cd9008aecd11d4cb99e733469`. Central `audio-playback.json` contains:

| Field | Value |
| --- | --- |
| sample_rate | 44100 |
| loop_start_sample | 5465255 |
| loop_end_sample / decoded_samples | 32515748 |
| loop_samples | 27050493 |
| loopStartSeconds derived by adapter | 123.92868480725623 |
| full decoded duration | 737.3185487528344 seconds |

The prefix includes the original tempo129 introduction and the original first loop passages before the chosen third D0 boundary. It is not a claim that the musical intro lasts 123.9 seconds. The suffix retains35 musical sequences and the full tempo-phase period. The first 32,514,337 PCM samples are unchanged; only the existing 1,411-sample tail join is applied. Dual capture/master/MP3 identity, exact decoded count, SI-SDR 28.3341 dB, clipping, sample seam and RMS results are in `MUSIC76_ASSET_VERIFICATION.md` and `.local/music76-source/assets/music76-delivery-manifest.json`.

Only verified master/MP3/manifest files are copied to the existing local export directory. The preview and prior masters remain unchanged. `register_music76_delivery.py` refuses to overwrite conflicting existing 76 artifacts or metadata. No upload occurred; listening and release flags remain false.

## Platform and restoration behavior

`GameAudioPort.music` now accepts optional `loopStartSeconds`. Missing or zero keeps native whole-resource looping. A positive anchor uses the same loaded native media resource with `media.loop=false`; `ended` seeks to the anchor and resumes. Intro-to-body playback does not switch sources. Cumulative offsets map into the prefix once and thereafter into the suffix; metadata validation happens before audible playback.

A pending loop-boundary restart remains attached to the platform task during pause and is cancelled by stop/mute/hidden cleanup/disposal/owner replacement. Generation/task checks reject stale callbacks and play completions. Playback continues through native HTML media and the existing playback audio session; no game-owned player or AudioContext was introduced.

The basketball adapter derives the anchor from the five central integer sample fields, validates their relationships, and passes the option for new requests and snapshot restoration. Backend revision lookup validates the same manifest. New original 76 requests bind the verified MP3 hash and create a fresh intro epoch. Legacy 76 saves with empty revisions remain unbound and silent; effects do not guess a migration. Server positions remain unbounded media-wall-clock milliseconds, including original 253/255 pause/resume semantics, independent of world/audio-IRQ simulation.

## Test-first evidence and native media results

`platform-before.log`, `adapter-before.log` and `music-state-before.log` retain failures before their implementation/registration. All 81 common platform-audio and basketball adapter contracts pass, including 12 new suffix cases. Targeted asset/server native and race checks pass for revision metadata, wall-clock JSON restore, long elapsed positions, pause/resume, fresh same-ID epochs and legacy unbound saves. TypeScript checking passes.

Actual Chromium media verification uses the complete verified MP3 at 1x and explicit seeks near the original boundaries, without shortening or accelerating the source. `.local/music76-source/browser/result.json` records 11 checks: initial native source/duration, continuous prefix-to-body transition, two real suffix wraps, source253/255, long wall-clock restore, paused snapshot restore, mute/no stale restart, native tab-hidden cleanup, owner replacement, disposal, and simulated CDN failure with the same local MP3. No WAV was requested.

The two measured native `ended`→`playing` intervals were **4.4 ms and 1.2 ms**. These are media-event latencies, not speaker-output silence measurements or a guarantee of gapless playback. Native MediaRecorder output files are retained, but decode to approximately 0.53 s and 0.49 s EOF-tail recordings; they do not prove uninterrupted audio across restart. Browser/device and human listening scope remains explicit.

Current-player native regressions also pass independently: 66 performs two actual whole-resource wraps, and 75 reaches its finite native end and clears its source on disposal. Results are in `.local/music76-source/whole-loop-66/` and `finite-75/`. The existing nine-track distribution manifest and earlier browser artifacts were not overwritten. Other old browser cases remain historical evidence tied to their own bundles.

Audio packaging tests were written first and pass eight methods, including 16 malformed76 metadata/source-binding cases. Real collection publishes all five sample integers and the MP3, never the music WAV. Ending/distribution validators and the historical/current browser aggregate preserve their distinct evidence scopes.

## Reproduction

```sh
. scripts/env.sh
node --experimental-strip-types --test web/tests/*audio*.test.mjs games/basketball/tests/audio.test.mjs
go test -race ./games/basketball/assets ./games/basketball/server -run '^(TestMusic76.*|TestMusicOriginalCommandsAndMediaClock|TestHostMusicPersistsWithoutReplayingOldEffects)$' -count=1 -v
web/node_modules/.bin/tsc -p web/tsconfig.json --noEmit
PYTHONDONTWRITEBYTECODE=1 .local/board-art-env/bin/python scripts/basketball/accept_music76.py
BASKETBALL_AUDIO_TEST_TRACK=66 BASKETBALL_AUDIO_EVIDENCE_DIR=.local/music76-source/whole-loop-66 PYTHONDONTWRITEBYTECODE=1 .local/board-art-env/bin/python scripts/basketball/accept_audio_loop.py
BASKETBALL_AUDIO_TEST_TRACK=75 BASKETBALL_AUDIO_EVIDENCE_DIR=.local/music76-source/finite-75 PYTHONDONTWRITEBYTECODE=1 .local/board-art-env/bin/python scripts/basketball/accept_audio_loop.py
PYTHONDONTWRITEBYTECODE=1 .local/board-art-env/bin/python scripts/basketball/verify_ending_music.py
PYTHONDONTWRITEBYTECODE=1 .local/board-art-env/bin/python scripts/basketball/verify_music_distribution.py
PYTHONDONTWRITEBYTECODE=1 .local/board-art-env/bin/python scripts/basketball/verify_music_browser.py
```

The native harness uses fresh isolated workspace browser profiles and short workspace socket paths. Its earlier tab-ID invocation error is retained in `browser/first-run-failure.json`; stable tab IDs are now selected from the actual browser tab listing. No shared player change was made to bypass a browser assertion.

Full ending/campaign rendering, real Safari/iOS/Android, speaker-output gap measurement, listening acceptance and release remain outside these local audio results.
