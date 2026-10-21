# Music76 source timeline and integration proposal

Source investigation and original proposal, 2026-10-18. The parent subsequently approved implementation. See `MUSIC76_ASSET_VERIFICATION.md` for the selected passing7449 anchor and `MUSIC76_RUNTIME_VERIFICATION.md` for the implemented platform option and measured browser results. The observations and initial proposal below preserve the pre-implementation decision record; no install or upload occurred.

The existing76 preview is too short to contain one complete repeating driver phase. The original looping section has **576 music ticks per musical sequence, 36,864 source frames per complete phase period, and35 musical sequences per period**. Repeating one approximately17.5-second sequence would reset the tempo phase; repeating the332.768-second preview would replay the introduction.

## Source observations

ROM SHA-256: `0fa28f60abb8d2ccbbeee90e79e3fa160a5cf32feb90f47ebf1a89168cff569b`.

Retained source is `.local/basketball/audio/export/court-music-076-20000-run-{1,2}/`. Both captures' PCM, complete231-byte driver RAM, APU writes, call log, sample cursors and jump records are byte-identical. The existing WAV/preview SHA-256 is `570cbe70d5f3daa2ff849a3e74e5d1f37f10feef72ba0de28a71ceb47d050d84`; full driver capture SHA-256 is `0778ba69a3ec926d594586e55132c997cc0639e2cf36ba5204f664afbe0b7eed`.

Reusable REA sources:

- Original driver request/update: `ev_b7dcc4d2199f88363f0de9659e2a278ac02a6d6435cfd7d88be1dde49fc632ec`, `ev_88919b8f185184e91d70fe32056d61eed05f0a14407c07fda56b62bf97ceb4f0`.
- Sequence jumps/DPCM: `ev_9a9854fcb3e8501642256c4655e7dcc84f0b20b4ad4adc958db214e9a990f5c1`, `ev_6849ad7405eccfae03f61c6956ebadc11b8210bdae5778b9073128dc6a2268ef`.
- Scene music requests: `ev_33bb6f8ee44e775ab1856de49120b96ebace7f235894fef4fb2cd6684a2b56de`; current ending dispatch is also established in `POSTRESULT_ANIMATION_SOURCE.md`.

Sound76 header is bank0 BC5E. Channel streams begin at BC6C/BD89/BE5D/BF0C. Channel0 command `CC81` at BC6C selects tempo129; `CC8C` at BC98 selects tempo140. Repeated D0 jumps for channels0..3 occur together at BCCF/BDFA/BEA2/BF4C and target BCA3/BDDC/BE7F/BF34.

| Boundary | Exact observation / limit |
| --- | --- |
| First active capture state | Source step4, tempo129, phase254; prior three retained states are capture startup. |
| Intro-to-loop tempo command | First tempo140 state is source step4289, phase59. The command occurs inside this emulated frame; existing PCM checkpoints only bound it between samples3,145,740 and3,146,474 (71.331972789..71.348616780 seconds). No sample-exact onset claim is made. |
| First main-channel D0 loop | Step5342, followed by6396; phases23 and127. The576-tick count follows `((6396-5342)*140 +23-127)/256`. |
| Complete tempo-phase period | `576*256/gcd(576*256,140) = 36,864` source frames;35 sequence loops. Approximately613.39 seconds is an estimate, not a derivative sample count. |
| First full driver-state recurrence | Source steps4304→41168. All bytes0700..07E6 match. The earlier tempo-change state4289 does not yet match its period-shifted state. |
| First possible delivery-prefix anchor | Existing sample cursor at step4304 is3,157,480 (71.598185941 seconds). This is after the musical tempo change: it is a delivery-prefix boundary, not a renamed intro onset. |
| Existing PCM coverage | Ends at step20,000 / sample14,675,073. It cannot establish PCM at end step41168 or a complete loop derivative. |

`probe_music76_timeline.py` executes unchanged original bank0 instructions through step45,000. All19,997 states overlapping the retained post-init capture match byte-for-byte. It verifies3,833 full-state recurrence pairs separated by36,864 frames. No reseeding, sequence editing, synthetic note generation or PCM synthesis is involved. First equal state SHA-256: `82400af63f5a31212293c728de001c83481a46748dcc25fb1c5eabad939b172a`; extended raw state SHA-256: `2c2e9f7077439cb675259daf9e2d1c90936ccee79b3dbe97a8956b6cf752bafc`.

The transient difference through step4303 includes channel8 flags/cursor/wait and envelope/register state. Therefore tempo change alone is insufficient for the complete-driver equality assertion. Full driver equality also does not prove raw APU oscillator/noise/resampler PCM phase equality; the established source-only tail-join validation is still required.

The old `export_ending_music.py` metadata labels BD27 as phase11. Current B995 dispatch proves BD27 is **Scene7 phase8**; phase11 is defeat/Continue. Correct that source label when the76 manifest is next deliberately updated. The sound request remains BD68/BD6A.

## Current platform and game contracts

`GameAudioPort.music` currently accepts only whole-resource `loop`, `startOffset` and stable `playbackID`. `web/src/audio.ts` owns one native HTMLAudioElement for background music, uses native `media.loop`, and wraps offsets by the full decoded media duration. It cannot repeat a suffix while playing an intro only once. Its general clip-list helper is not an exposed gapless music playlist; replacing source URLs after a finite intro would add a load/start boundary.

The platform already owns gesture unlock, `audioSession.type="playback"` when available, media volume, pause/resume, mute, owner cleanup, stale metadata handling and cancellation. Page hiding calls `audio.stop()` in `web/src/main.tsx`; showing the page requests a fresh snapshot. The basketball adapter restores only current music from server media wall time, checks the asset hash and watermark, and does not replay historical effects. These contracts should be extended rather than replaced.

Server `MusicState` already recognizes76 but intentionally gives it no asset revision; the frontend omits76 because no compressed playback entry exists. `MusicRevision` reads the same embedded delivery manifest as the frontend. Enabling76 therefore requires a verified delivery entry, removing the special unversioned76 creation path, and validating/restoring the same revision. Existing unversioned76 saves must remain explicitly unplayable or receive a deliberate migration; silently treating an empty revision as the new asset would bypass source binding.

The source music state machine `audiostate` remains separate from media playback. Browser time, media seek, mute and volume must not drive its original IRQ state or work byte.

## Recommended minimal delivery/player change

Use **one compound MP3 containing the untouched source prefix followed by one complete verified phase period**, and add an optional **platform-owned `loopStart`** to music options. The media file plays through the prefix into its loop passage naturally, with no source switch at the intro boundary. At native `ended`, the platform seeks the same loaded resource to `loopStart` and resumes its suffix; it does not replay the prefix. Existing music without the option keeps its current native whole-resource loop behavior.

For this suffix case, native `media.loop` must be false: otherwise the browser immediately returns to byte/time zero. The platform's owner/task/generation guards must govern the `ended` callback and any resulting play promise. The game continues to provide only manifest selection/metadata through `BoardProps.audio`; it creates no Audio/AudioContext, timer, media element or mute control.

Let `I` be the verified loop-start sample divided by44,100 and `D` the verified decoded duration; `L=D-I`. A cumulative media-wall-clock offset `T` maps to:

```text
T < I:  mediaPosition = T
T >= I: mediaPosition = I + ((T-I) mod L)
```

Apply the mapping before audible playback when metadata is available; validate finite nonnegative inputs, `0 <= I < D`, and the manifest's sample/duration identity. Include `loopStart` in the stable music key. Repeated snapshots in the same epoch must not restart; a new original76 request creates a new epoch and legitimately plays its intro again. Source253 pauses the same position;255 resumes;0 cancels the entire task and any pending loop restart.

A pause that races with `ended` needs an explicit pending-loop-boundary state. Resuming an ended element without first restoring the suffix position can restart at zero and replay the intro. Muting, hiding, disposal, owner replacement and a later track must cancel that pending state and stale callbacks. After showing/reconnecting, restore the current authoritative elapsed offset; do not revive an old local queue. Unmuting alone must not resurrect a cancelled task.

This is a small platform-native-media extension, but `ended`→seek→play is **not guaranteed sample-gapless**. The native event/seek latency must be measured in real browsers. Keeping the same loaded resource avoids a new network/source load, and the complete phase period makes this boundary infrequent; neither fact is proof of an inaudible seam. If measured behavior fails the agreed audible-gap limit, do not declare76 seamless or shorten the source loop to hide it. A more involved native-media streaming approach would require a separate compatibility design.

Rejected shortcuts: loop the preview; truncate to one musical sequence; use `timeupdate`/setTimeout to jump early; use undocumented media-fragment looping assumptions; use a game-owned player or Web Audio. These either alter the original timeline or bypass the established platform contract.

## Concrete implementation and test plan for review

1. **Source PCM and derivative evidence.** Reuse the original immutable76 NSF and existing offline renderer. Capture at least45,000 frames twice, at the existing44,100Hz/mono/16-bit/volume192 settings, in a new workspace evidence directory. Verify both outputs and the first20,000-frame driver/PCM/APU prefix against retained records. Do not rerun the existing exporter, which rewrites manifest entries and previews.
2. **Predeclare the delivery anchor search.** Start with full-state recurrence4304→41168; use only boundaries proven equal by the original source, and preserve the complete36,864-frame period. If a nearby boundary fails the existing seam/RMS requirements, retain that failure and select another proven boundary under the same fixed criteria. Record the resulting prefix sample count and exact period sample count from actual PCM checkpoints. Do not round source frames through60fps.
3. **Master verification before delivery encoding.** Preserve every prefix sample and all loop samples outside the established final1,411-sample tail join. Reuse the current raised-cosine join of corresponding original passages only if its fixed seam, clipping and local-RMS criteria pass. Verify unchanged duration, identical driver states and repeated derivation. This is processing of original PCM, not new music. Keep listening/release flags false until their separate acceptance.
4. **MP3 checks.** Independently encode twice with the existing pinned128kbps/44.1kHz/mono libmp3lame profile and Xing/LAME delay/padding metadata. If quality fails, follow the already declared160/192kbps candidate policy. Require identical encoded hashes, exact decoded sample count, at least24dB SI-SDR, no clipping and existing RMS limits. Evaluate the decoded EOF→decoded **loop-start** seam; the current generic validator compares EOF→sample0 and must not be reused unchanged for76. Preserve metadata and exact commands under a new source manifest. Only then add an immutable MP3 playback entry; never fall back to the WAV/preview.
5. **Tests before shared code.** Add failures for prefix playback once; repeated suffix boundaries; offsets before/at/after `I` and `D`; offsets many periods later; metadata validation; stable epoch identity; same-ID new epoch; source fallback retaining the mapped offset; stale metadata/ended/play promises; pause at the exact end; resume after metadata while paused; hidden/mute/dispose/owner replacement at each boundary; error containment. Existing whole-loop, finite75, offset, pause and ended-owner tests remain required regressions. No other games' full gameplay matrices are needed.
6. **Game/server binding.** Manifest validation must bind the chosen prefix/period sample metadata to that derivative's source/file hashes. Add76 to the ordinary revision lookup and adapter mapping only after the asset passes. Preserve unbounded server `positionMS` and its source253/255 accounting; do not change to `VideoFrames/60`. Test save/restore before the intro finishes, while in the repeated suffix, paused at the boundary, and after a long hidden interval. Reject mismatched asset revisions.
7. **Native browser E2E.** Extend the existing platform-backed media harness. Observe the real intro boundary within the same source, then seek near EOF and observe two actual suffix restarts at1x without shortening the file. Record `currentTime`, source identity, network requests, wall-clock timestamps and audio evidence sufficient to measure the gap. Verify no WAV requests, same-file local fallback, current snapshot restore, pause/resume, mute/hidden cleanup and disposal. Chromium evidence alone does not establish Safari/iOS/Android behavior; preserve the actual tested device/browser scope.

## Reproduction and outstanding decisions

```sh
PYTHONDONTWRITEBYTECODE=1 .local/board-art-env/bin/python scripts/basketball/probe_music76_timeline.py
```

Source results, failure inventory, raw extension, hashes and repeat check live in `.local/music76-source/`. The19,997 captured-state matches establish source consistency; the45,000-step extension supplies the previously missing complete control-period witness. It supplies no additional PCM.

At the proposal boundary, the player change and longer PCM derivation required parent review. That review approved `loopStartSeconds`; subsequent source/asset and native-media results are recorded in the two reports linked above. The failed4304 anchor remains retained rather than being relabelled as accepted. Device validation, listening acceptance and any gapless-output claim remain outstanding.
