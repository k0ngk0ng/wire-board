# Post-result text verification

Failure cases recorded before implementation:

- Duplicating authoritative actor06AB/06AD or ending06AC flags, losing phase/actor text requests, or consuming them in the wrong scene order.
- Wrong cursor byte order/carry; resetting retained address/buffer state on a new text request.
- Wrong two-call default delay, Scene2 one-call delay, zero-glyph one-call delay, pause wrap or clear-row progression.
- Losing F0/F1 repeated-buffer modes, F2/F3/F9 pauses, F4 clears, F5 completion, F6 no-op, F7 line advance or F8 mode clear.
- Treating command bytes as glyphs, ignoring dakuten/handakuten lookup/decrement, or overwriting unused buffer bytes.
- Missing repeated same-value upload requests; confusing pending header/data writes with completed NMI writes.
- Losing the source's28 cleared bytes with declared upload length 27, address carries, or blank/terminator behavior.
- C036 choosing wrong scene/phase address or changing byte layout/order of its two tile rows; skipping its explicit next-NMI boundary without documenting it.
- Inventing timing/NMI mutations, using source expected snapshots to force transitions, or claiming controlled text probes are an earned campaign.
- Invalid stream/cursor/message input mutates state or returns partially queued writes.

Scope: new standalone text progress/message components only. Runtime/server/display integration and complete movie acceptance are separate.

## Source and scope

REA `ev_3ba34f52bc8520fc29bd1d7ec8a342a72172e090d4e73917cf6a81458ab55640` covers bank4 A496, its text command handlers, C036/BE6E, C13F glyph decomposition and C1A4 buffer preparation. Complete bundle: `.local/postresult-text/analysis/evidence.json`. The analysis session exported and closed normally. ROM SHA-256 remains `0fa28f60abb8d2ccbbeee90e79e3fa160a5cf32feb90f47ebf1a89168cff569b`.

`verify_postresult_text.py` executes unchanged original instructions. All FD0A/A496 cases return normally without skipping calls. C036/BE6E cases execute both original glyph/buffer helpers and stop at BEBC before the explicit NMI wait. Fixtures retain full raw input/output RAM, instruction counts and pending-header stores. These are controlled inputs, not invented earned histories.

The fixture contains 10,141 text calls, including all 37 complete sequential streams and controlled flag/countdown/address cases; ten explicit command-handler observations; and124 message preparations covering all 31 message IDs across reward/other scene/phase combinations. Fixture SHA-256: `ee5d305d54f2d33e3ce5e5fbc4cc0bbf15c13f1cddf603e339b7802795c71bb8`.

F0/F1/F2/F6/F8 do not occur in the bounded37-stream inventory. Their handler behavior is retained from explicitly isolated original handler probes, while regular stream playback exercises F3/F4/F5/F7/F9. This does not establish natural callers for unused commands. High-bit text-ID aliases use the original low-seven-bit pointer selection. The generated tables contain semantic glyph/command bytes and the original 256-entry glyph-mark map; runtime reads neither ROM nor CPU instructions.

## Authoritative state and API

`StepOriginalSceneText(scene, actorContext, textActive, progress, uploads)` is the complete FD0A logical call. Its inputs/outputs deliberately avoid a second persistent copy of phase/actor fields:

| Source | Owner / representation |
| --- | --- |
| 06AB,06AD | Existing `ResultActorContext.TextRequestFlags` / `TextID` |
| 06AC | Existing `EndingPhaseContext.TextActive`, passed separately and returned as `SceneTextStep.Active` |
| 06AE..06B7 | New `SceneTextProgress`: address, cursor, retained repeat byte, clock, pause, delay and line address |
| 030B..032D | Existing `EndingPhaseContext.Upload`, supplied as `uploads[0]` |
| 032E..0350 | Second `EndingUpload`, supplied as `uploads[1]` and retained by the caller |
| 0307..030A | `ClearPaletteOverrides=true` records the original four zero stores on every FD0A call |

The returned `QueueMask` identifies buffers whose header was written even when bytes repeat. It denotes pending uploads, not completed transfers. The caller owns NMI consumption, video timing and draw composition. Pass returned Actor/Active/first Upload back into the existing ending context, retain returned progress/second Upload separately, then run the ending phase and actor tail. Phase/actor requests consequently reach FD0A on the next source iteration.

Source behavior:

- A pending request is accepted only when active bit80 is clear. It clears06AB, sets cursorFFFF, installs per-scene delay `[2,2,1,2,2,2,2,2]`, ORs active80, and reads the first token immediately. Other retained address/buffer state is preserved.
- Active flags60 take precedence: decrement the pause byte with 8-bit wrap; reaching0 clears those two bits. Flag10 clears successive row pairs; low address83 resets address/line address to2303 and clears flag10.
- Normal calls decrement the clock. Reaching0 reads the next token, except modes04/08 requeue retained glyph data. Zero glyphs set clock1; ordinary glyphs use the stored scene delay.
- Glyph marks come from fixed C03F. Mark0C decrements the displayed lower tile by 1; mark0D decrements it by 2. Top/bottom tile writes use separate original buffers. C036's helper applies its own original mark categories and FF terminator padding.
- Text upload length is1 per glyph. The clear command sets length 27 but stores 28 zero bytes; the extra byte and all unaffected trailing bytes are preserved exactly.

| Token | Exact logical effect |
| --- | --- |
| F0/F1 | Active=84/88, retained repeat byte=FF |
| F2 | Active OR20; pause byte retained |
| F3/F9 | Pause=64/128, Active OR40 |
| F4 | Active OR10; address2303; queue initial clear rows |
| F5 | Active=0; the normal FD0A fetch has set scratch24=0, so no final glyph upload |
| F6 | Return without further mutation |
| F7 | Line address +=64; copy to current address |
| F8 | Active ANDF3 |

`PrepareOriginalPostResultMessage(scene, phase, messageID, uploads)` accepts the0351/0374 buffers, preserving unused data bytes. It returns both pending rows, `QueueMask=3`, and `WaitNextNMI=true`. Scene1 phase7 uses address page20; other contexts use23. Each row's low address follows BFD6, with the second row offset32. The function stops at the original wait boundary rather than guessing how many scheduler ticks it takes.

## Whole-console corroboration

The art/source worker supplied controlled full-console runs `postresult-ending-observed` and `postresult-reward-observed`. Their initial BA40 score/final-period injections are explicitly declared in that worker's capture report; they are not natural earned victories. `verify_postresult_text_capture.py` reexecutes original source from each captured call's entry and compares its persistent text/context/buffer fields against the captured return:

- Ending run: 14,170 complete FD0A calls match exactly, including 13,370 Scene7 calls and 800 Scene4 calls after reset. No modeled buffer mutation from NMI occurred inside those calls.
- Reward run: 110 Scene2 FD0A calls match exactly. All 111 completed C036 calls match buffer data, length and low addresses; their two pending header high bytes were observed written toFF by NMI before return. Those header-consumption differences are retained separately, never patched into expected source execution.
- One final C036 call begins at emulator frame 26649 but the capture ends before its return. It is explicitly excluded and listed as incomplete.
- Reward messages 28,30,29 are observed at frames 26185,26186,26187 while reward timer remains 40. Each completes its own next-NMI wait. Combining them into one instantaneous frame would lose this source behavior.

Exact capture event/RAM hashes, call counts and NMI-consumption records are in `.local/postresult-text/whole-console-corroboration.json`. The comparator reports zero unexplained differences. This corroborates the source primitive; the new logical composition still requires its own Go-to-console and display checks.

## Tests and reproduction

Tests and fixture generation preceded implementation. `.local/postresult-text/before.log` records failures for the first original text stream, control handler and C036 message. The completed tests compare all 37 streams continuously from a single initial state per stream, all fields/queued buffers, command branches, original message layouts, JSON restoration and atomic error behavior. Native, race and Wasm pass. Source fixture and generated table reproduction also pass.

```sh
PYTHONDONTWRITEBYTECODE=1 .local/board-art-env/bin/python scripts/basketball/verify_postresult_text.py --check
PYTHONDONTWRITEBYTECODE=1 .local/board-art-env/bin/python scripts/basketball/verify_postresult_text.py --check --emit-tables
PYTHONDONTWRITEBYTECODE=1 .local/board-art-env/bin/python scripts/basketball/verify_postresult_text_capture.py
. scripts/env.sh
go test -race ./games/basketball/engine -run '^(TestOriginalSceneText.*|TestOriginalPostResultMessageSources|TestOriginalResultActor.*)$' -count=1 -v
GOOS=js GOARCH=wasm go test ./games/basketball/engine -exec="$(go env GOROOT)/lib/wasm/go_js_wasm_exec" -run '^(TestOriginalSceneText.*|TestOriginalPostResultMessageSources|TestOriginalResultActor.*)$' -count=1 -v
```

Logs: `.local/postresult-text/{before,after,race,wasm,oracle-repeat,tables-repeat,whole-console-corroboration}.log`. No runtime, server, shared display or lifecycle implementation was modified by this task. Original upload timing, text/phase/actor composition, rendering, audio76 and a naturally completed campaign remain separate acceptance requirements.
