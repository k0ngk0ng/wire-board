# Music 76 asset verification

**Current outcome: offline asset verification passed using the approved source-boundary follow-up, at source frames 7,449→44,313.** The first rejected anchor and its unchanged manifest are retained below. Delivery artifact: `.local/music76-source/assets/music-076-intro-loop.mp3`; delivery metadata: `music76-delivery-manifest.json` in the same directory. This does not establish browser, human-listening or device acceptance.

## Predeclared failure inventory

Before writing capture/derivation code, the following failures are acceptance blockers:

- Source ROM, immutable NSF, renderer or bridge identity differs; a capture process/lock is active; any renderer output escapes the workspace.
- A capture is incomplete, times out, has wrong format/settings, or its 45,000 driver states/sample cursors are incomplete.
- The two independently launched captures differ in PCM, driver RAM, APU writes, calls, jumps or sample cursors; the retained 20,000-frame prefix differs.
- The approved driver boundaries 4,304 and 41,168 do not match all 231 bytes, or the prefix is not exactly 3,157,480 samples. No anchor search or substitution is authorized.
- The compound WAV changes any source sample before its final 1,411 samples, changes duration, or changes the full 36,864-frame/35-sequence period.
- The fixed raised-cosine tail join fails original-step boundary error ≤ 1 sample unit, final 16-sample error ≤ 2, no clipping, local RMS deviation ≤ 2 dB (353-sample windows at 88-sample steps), or transition-step limit 1.05× original + 2.
- Independent encodes differ, the MP3 lacks usable Xing/Info and delay/padding metadata, decode count differs, SI-SDR is below 24 dB, decoded peak clips, edge RMS differs by more than 1.5 dB, or EOF→decoded loop-start step exceeds 1.05× adjacent range + 2. Only the predeclared 128/160/192 kbps sequence is allowed.
- Existing previews/masters/manifests are modified, browser acceptance is inferred from offline checks, or listening/device acceptance is incorrectly reported complete.

## Results

**The approved anchor is rejected. No derivative WAV or MP3 was produced.** The original source captures pass; the fixed 1,411-sample raised-cosine join fails the predeclared local RMS requirement. The anchor and processing parameters were not changed.

Two independently launched 45,000-frame FCEUX captures each contain 33,019,863 PCM samples. They are byte-identical in all six artifacts: WAV, full 231-byte driver states, sample cursors, sequence jumps, APU writes and call log. Both retained 20,000-frame recordings match the new recordings' PCM and event/state prefixes. Capture wall times were 18.526 and 18.499 seconds.

The exact source cursor metadata is:

| Field | Value |
| --- | ---: |
| Sample rate | 44,100 Hz |
| Loop start sample | 3,157,480 |
| Loop end sample / proposed compound sample count | 30,207,974 |
| Full period samples | 27,050,494 |
| Prefix seconds | 71.59818594104308 |
| Full period seconds | 613.3898866213152 |
| Proposed compound seconds | 684.9880725623583 |
| Source period | 36,864 frames / 35 musical sequences |

All 231 original driver bytes match at source boundaries 4,304 and 41,168. These are observed PCM checkpoints, not frame-count/60 estimates. Decoded MP3 sample count remains null because encoding was correctly stopped before producing a derivative.

The join's original-step boundary error is 0, its final 16-sample error is 2, peak absolute amplitude is 26,975 and maximum transition step is 4,150 (limit 4,734.35). Three 353-sample windows violate the ±2 dB RMS limit:

| Offset within tail blend | RMS difference |
| --- | ---: |
| 528 samples | −2.115724491312293 dB |
| 616 samples | −2.31551850820799 dB |
| 704 samples | −2.466291747790596 dB |

The verification was executed twice and reproduced the rejected manifest byte-for-byte: SHA-256 `3a1f0ff79cb17d675dd4eb8593b95e40968eace0ef504ec0059289b1e968b524`. Expected verifier exit status is 2. A later authorized revision must preserve this failed-anchor evidence.

## Artifacts and reproduction

All artifacts are under `.local/music76-source/assets/`:

- `capture-45000-run-1/` and `capture-45000-run-2/`: original audio/state evidence, settings, exact commands, tool hashes and capture logs.
- `capture-verification.json`: independent-capture and old-prefix validation.
- `music76-asset-manifest.json`: exact source timing, checks, all failed windows, hashes and explicit absent derivative/decoded metadata.
- `anchor-verification.log` and `anchor-reproducibility.json`: reproducible failure result.

```sh
PYTHONDONTWRITEBYTECODE=1 .local/board-art-env/bin/python scripts/basketball/capture_music76_assets.py
PYTHONDONTWRITEBYTECODE=1 .local/board-art-env/bin/python scripts/basketball/verify_music76_asset_anchor.py
```

The capture command reuses only completed captures whose source/settings/file hashes still match. The verifier never writes a derivative or searches other anchors. Planned public filename `music-076-intro-loop.mp3` is reserved only; that file does not exist in this evidence directory. Existing masters, previews, delivery manifests and player/runtime code were not modified.

Source WAV SHA-256: `87f2f6319491d3280ea1be0918b5f76198be2d3f2d4b656374144ef81bc36b94`. Source driver SHA-256: `cde71567d011cff56419c521e61b3f7543219954abc8482afd3203a4177e52d2`. The new capture reuses the original NSF/ROM and existing FCEUX/bridge identities listed in the manifest. It corroborates the isolated source driver timeline already recorded by `MUSIC76_INTEGRATION_SOURCE.md`; it is not earned campaign evidence.

Parent/root review is required before selecting a different anchor or processing method. MP3 quality/delay/padding/seam checks, human listening, native browser boundary timing, device acceptance and release acceptance remain pending. Offline waveform checks cannot establish gapless browser playback.

## Authorized source-boundary follow-up

Root subsequently approved testing source-equivalent anchors with the unchanged 36,864-frame period, complete 231-byte state equality, original prefix and all existing thresholds. The bounded candidate set is the original synchronized channel D0 jumps whose full periods fit inside the existing 45,000-frame captures: 5,342, 6,396 and 7,449. This is an event-defined candidate set, not a waveform search. The original rejected manifest/report section above is retained as evidence.

Additional failure cases declared before the follow-up scripts: a proposed boundary is absent from the four original channel jump records; its period endpoint is not the corresponding 35th later jump; either 231-byte state differs; fewer/more source samples are emitted than the chosen end cursor; any original prefix or loop sample before the fixed tail window changes; chosen-source identity differs between derivation runs; encoding or decoded-loop-seam validation fails any original threshold. No candidate may alter sample rate, shorten the period, stretch samples, reseed the source, add notes or relax a threshold.

## Accepted compound source and MP3

The bounded original D0-jump candidates produce these results under the same fixed thresholds:

| Source frames | Source sample cursors | Maximum absolute local RMS deviation | Result |
| --- | --- | ---: | --- |
| 5,342→42,206 | 3,919,156→30,969,649 | 7.6436426645 dB | Rejected |
| 6,396→43,260 | 4,692,572→31,743,066 | 12.3641755022 dB | Rejected |
| **7,449→44,313** | **5,465,255→32,515,748** | **0.0187149994 dB** | **Passed** |

All three are observed simultaneous four-channel D0 jumps, with equal complete 231-byte driver states at their paired endpoints and exactly 35 musical sequences / 36,864 source frames between them. The accepted boundary is the earliest passing event in this bounded source-event set. The unchanged prefix now contains the original introduction plus the first three loop traversals; no original notes or timing are changed. The one-sample differences between measured candidate periods arise from sample-cursor alignment, not trimming or a changed frame period.

Two separately derived WAVs are byte-identical. Every source sample in `[0, 32,514,337)` is unchanged, including the complete prefix and almost all of the loop passage. Only the final 1,411 samples use the original raised-cosine convex join with source samples `[5,463,844, 5,465,255)`. The output remains 32,515,748 samples. The accepted join has boundary error 0, final 16-sample error 0, maximum step 2 (limit 3.05), and no clipping.

| Delivery metadata | Exact value |
| --- | ---: |
| `sample_rate` | 44,100 |
| `loop_start_sample` | 5,465,255 |
| `loop_end_sample` | 32,515,748 |
| `loop_samples` | 27,050,493 |
| `decoded_samples` | 32,515,748 |
| Prefix / `loopStartSeconds` | 123.92868480725623 seconds |
| Loop period | 613.3898639455782 seconds |
| Total decoded duration | 737.3185487528344 seconds |

The existing pinned FFmpeg/libmp3lame profile produced a passing 128 kbps, 44.1 kHz mono MP3; 160/192 kbps fallback was unnecessary. Two independent MP3 encodes are byte-identical, and both full decodes have the exact source sample count and matching PCM hash. The Info header and LAME-compatible extension record 28,227 MPEG audio frames, 576 encoder-delay samples and 1,180 padding samples: `28227*1152 - 576 - 1180 = 32515748`.

Measured SI-SDR is 28.3341 dB against the ≥24 dB requirement. Decoded peak is −2.099752 dB. The decoded EOF→decoded loop-start step is 10, below the adjacent-range limit of 5,015.75; this comparison uses sample 5,465,255, not sample zero. Edge RMS differences are −0.4257145 dB at the prefix start, −0.4313540 dB at the loop start and −0.4453720 dB at EOF, all within ±1.5 dB.

| Artifact | SHA-256 |
| --- | --- |
| `music-076-intro-loop.wav` | `cc6a513c5246031c63e71a04190760f226241ce20ddf90029dd5930050721fac` |
| `music-076-intro-loop.mp3` | `7e0eb8fcaa8cbc9c426b93fc19d63ac06451874cd9008aecd11d4cb99e733469` |
| Decoded MP3 PCM | `4f6f79fbc461fa9173c07a3c8a7708ee2242272126f9f0bb4dccc6cc55ae52f9` |

The MP3 is 11,798,194 bytes. The independent source captures, two derived masters, two encode candidates, decode logs, exact commands, source/tool/script hashes, candidate failure records and all numeric checks remain in `.local/music76-source/assets/`. `music76-delivery-manifest.json` is the accepted offline delivery manifest; `music76-asset-manifest.json` remains the unchanged rejected-anchor record. The source preview and existing masters/manifests are preserved.

```sh
PYTHONDONTWRITEBYTECODE=1 .local/board-art-env/bin/python scripts/basketball/verify_music76_sequence_anchors.py
PYTHONDONTWRITEBYTECODE=1 .local/board-art-env/bin/python scripts/basketball/derive_music76_assets.py
```

The follow-up does not rerun capture. It independently derives from the two existing verified captures and encodes twice. It writes only music76-specific evidence, not playback registration or runtime code. Listening, native ended→seek→play boundary timing, real-device behavior and release acceptance remain pending; the offline seam check is not a sample-gapless browser-playback claim.

## Audio packaging and validator integration

After the parent staged the compound WAV, MP3 and delivery manifest and registered the five integer timing fields, audio-specific packaging and validators were updated. No capture or encoding was repeated. The original nine-track `music-distribution-manifest.json` remains a separate historical batch; music76 uses `music76-delivery-manifest.json`.

Before implementation, package tests added a synthetic compound76 success case and 16 malformed/binding failures: missing, boolean, fractional, negative or zero sample fields; wrong rate/end/period/decoded count; finite playback; wrong filename; wrong master hash; mismatched source timing; missing ROM identity; wrong source master; and rejected source status. The first run failed on the missing filename support. The final eight test methods pass, including all new cases and existing packaging/publication safety checks. Logs: `package-tests-before.log` and `package-tests-after.log` under this asset evidence directory.

Packaging permits only `music-076-intro-loop.mp3` for ID76, requires positive safe-integer timing with exact period/end/decoded relationships, matches the verified source/delivery metadata and actual master hash, and publishes only the five timing integers plus the existing file/MIME/hash/loop fields. Master WAVs and research manifests remain excluded from the public payload. The existing recursive source identity check already supports `romSHA256`; the delivery manifest and generator did not need modification.

Targeted verification passed:

```sh
PYTHONDONTWRITEBYTECODE=1 python3 -B scripts/basketball/test_package_assets.py
PYTHONDONTWRITEBYTECODE=1 python3 -B scripts/basketball/verify_ending_music.py
PYTHONDONTWRITEBYTECODE=1 python3 -B scripts/basketball/verify_music_distribution.py
PYTHONDONTWRITEBYTECODE=1 python3 -B scripts/basketball/verify_music_browser.py
```

The ending validator confirms the preserved333-second preview is still unregistered and distinguishes the separately registered compound76 MP3. Distribution validation checks the historical nine-track batch and the separate music76 source/staged hashes, independently derived/encoded artifacts, delay/padding, exact decoded count and EOF→loop-start metrics. The historical browser validator validates only its ten saved cases and saved bundles; it explicitly sets `current_shared_player_verified: false` and links `.local/music76-source/browser/result.json` as separate evidence, pending when absent. It does not infer current-player success from those old bundles.

A real audio-only collection produced 45 audio payload files, including ten music tracks, with all five music76 timing integers intact and no music WAV. Public audio metadata SHA-256: `6659776ac58466429f5b34705dca55e1901e5df75db4ce6625a865094bc210f5`. The sanitized payload and result are retained as `packaged-audio-playback.json` and `package-audio-verification.json`. No graphics collection changes or upload were made in this audio subtask. Current native music76 browser validation remains owned by the parent task.

## Selected native browser evidence bound to the aggregate

The parent subsequently supplied successful native Chromium runs. The aggregate verifier now validates the actual music76 result, saved `harness.js`, registered integer interval, offline delivery metadata and MP3 SHA-256 rather than merely linking an available result. It also separately validates the new native66 whole-resource loop and75 finite-track regression artifacts. The existing nine-track/ten-case batch remains historical; `current_shared_player_verified` stays false as a blanket claim, and only the explicitly named new runs are reported separately.

The music76 result has 11 checks and two actual suffix boundaries. The validator checks the one-resource prefix crossing, same-source suffix restart, native `loop=false`, 1× playback, pause/resume, mapped wall-clock restoration, paused restoration, mute/hidden/disposal cleanup, owner replacement and real source fallback. It recomputes ended→playing/seeked latency from saved events and validates both recording hashes. The event latencies are 4.4000000954 ms and1.1999998093 ms. These are browser event timings, not speaker-output gap measurements. The MediaRecorder files contain approximately0.5-second EOF tails and do not establish continuous playback across the suffix restart or sample-gapless output.

Verified run artifacts:

- `.local/music76-source/browser/result.json` and `harness.js` (11 checks, two suffix boundaries).
- `.local/music76-source/whole-loop-66/result.json` and `loop.js` (two native whole-resource wraps).
- `.local/music76-source/finite-75/result.json` and `loop.js` (finite ended state).

`PYTHONDONTWRITEBYTECODE=1 python3 -B scripts/basketball/verify_music_browser.py` passes; output is retained in `browser-aggregate-current-validation.log`. The aggregate remains `.local/basketball/audio/compression/acceptance-summary.json`. This follow-up reran no captures, encodes or browsers. Human listening, Safari/iOS/Android device coverage, continuous output-gap measurement and release acceptance remain pending.
