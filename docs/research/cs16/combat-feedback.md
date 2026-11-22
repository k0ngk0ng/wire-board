# Blood, gunfire and HE feedback

The client consumes the existing authoritative hit/explode stream. The small hit `fx` enum is decided before armor consumption by the server: 1 is blood, 2 is helmet sparks, and an omitted/zero value produces neither. Opponent armor and inventory remain private. Missing coordinates, an invisible target or a different target life suppress the effect.

## Finding ledger

| Finding | Evidence and scope | Result |
| --- | --- | --- |
| Gunfire lines | The user's explicit instruction; the previous `ShotEffects` allocated a `Three.Line` for every pooled shot | Removed all gunfire line geometry. Original model muzzle attachments and existing map-surface impacts remain. Local scene rays only place surface feedback; they do not create blood or decide damage. |
| Armor versus blood | Fixed shipped Linux `cs.so` and Windows `mp.dll` static branches, independently recorded in `.local/cs16/verification/classic-engine/sources/impact-fx/FINDINGS.md`; also ReGameDLL reference `player.cpp:675–826` | The client trusts `fx`; it does not reconstruct armor state from a private or post-damage projection. Generic/leg blood and helmet sparks retain their distinct authoritative paths. The enum remains sufficient when integer health-loss text rounds to zero. |
| Native blood/fire resources | `combat-effect-assets.md`, four SPR identities, all 74 decoded frames and source palette evidence | Native frame dimensions, signed pivots, transparent empty frames, palette alpha and normal/additive blend are retained. Blood uses source palette index 247, RGB 103/0/0, converted from sRGB before Three.js shading. Helmet point color 5 is source RGB 75/75/75. |
| Blood size/timing | Fixed compatible Xash3D source `engine/client/cl_tent.c`, commit `f86ac073668d9d889dc07e25e3428b572f7a8a32`, `R_BloodSprite:872–955` and `TE_BLOODSPRITE:2273–2282` | The message size byte is derived from twice the damage and clamped to 3–16. Spray scale uses size/(25–35), completes in 250 ms, and starts 2–4 units above the contact. Drops use static source frames, size/(15–25), source velocity ranges and a 1–3 second lifetime. These are compatible-engine reference semantics, not an observed original-engine trace. |
| HE sprites | Fixed ReGameDLL reference `ggrenade.cpp:254–325`, `weapons.cpp:391–396`; original fexplo/eexplo resources | One logical HE event creates the two message-level effects: fexplo at Z+20, scale 2.5; eexplo at bounded random XY offsets and Z+30–35, scale 3.0. Both consume the message rate of 30 fps. Flash/smoke events do not acquire an HE fireball. |
| HE audio | Original decoded PCM identities in the asset evidence; ReGameDLL `Explode3` sends two `TE_EXPLOSION` messages with flags zero and debris at volume 0.55; compatible `R_Explosion:1579–1623` dispatches its explosion sound group when NOSOUND is absent | Each HE event selects two existing explode3/4/5 sample layers and one existing debris1/2/3 layer at 550/1000 volume. The event identity and platform spatial audio prevent replay. The previous hegrenade-1 clip is a different, longer sample and is no longer substituted. Smoke selects the already available sg_explode clip. |
| Helmet sparks | The shipped-sample TraceAttack message contains TE_STREAK_SPLASH, palette color 5, count 22, base speed 25 and random speed 65 | A bounded point pool consumes `fx:2`. Point size, direction spread and a 350 ms display lifetime are presentation approximations; the original engine particle integrator was not observed. |
| Bounds and privacy | Targeted module checks plus actual single-map WebGL checks | Sixteen high-priority spray/fire slots, 32 drop slots and eight spark slots are reused. Event history is capped at 256 IDs; a new round/epoch or explicit recovery clears pending and active effects. World effects retain depth testing and do not create a new hit HUD. |

The selected sample authority remains a third-party archive. The original game was not executed. REA session availability was checked; existing REA `ev_0726a0b7fb1eca06bf363dd28abcc8b562ff05beaf8b8eb35c3aafee91b8dcba` establishes PE identity/resources only. The independent native branch audit uses the fixed original bytes and LLVM evidence; it is credited separately from the compatible source and browser observations.

The locally retained Xash source file has SHA-256 `46366d93fee404658dde950140c5977e8a5cb864e7e9b8bf0db5e8b9fa6e3bad`. Source palette SHA-256 is `036b7ea3c8c03be214bb7d22b8d63ac27f63d547e42ca677fef344ddc780961e`.

## Verification

Failure scenarios and RED checks precede implementation in `games/cs16/tests/combat-effects-failures.md` and `.local/cs16/verification/combat-effects/`. The renderer baseline uses the prior committed renderer/shot-effects source, rather than treating an altered screenshot as an original-game capture.

Reproduce module checks:

```sh
node --experimental-strip-types --test games/cs16/tests/web-shot-effects.test.mjs games/cs16/tests/web-combat-effects.test.mjs games/cs16/tests/web-assets.test.mjs games/cs16/tests/classic-hud.test.mjs games/cs16/tests/web-feedback.test.mjs
```

Reproduce the functional renderer check:

```sh
CS16_COMBAT_REPORT=current node games/cs16/tests/combat-effects-browser.mjs
```

The latter verifies actual blood/fire pixels, original atlas identities, hidden-target suppression, duplicate events, helmet separation, expiration, muzzle feedback and zero line objects. It reads the required BSP/MDL/SPR and sound resources without initializing or executing a motion runtime. Audio-resource identities and routing are checked; actual original-engine audio dispatch and listening were not measured. `playability-audit.mjs` also now requires muzzle/surface feedback with zero tracers; its complete live-room entry remains for the unified build verification.

Final reports, screenshots, source hashes, the 30 module results and scoped lint/TypeScript output are retained in `.local/cs16/verification/combat-effects/`.

## Remaining limits

- A hit event contains the player origin and hitgroup, not the original trace end point/direction. Blood is placed on the visible surface of that original animated MDL hitbox group; generic hits use the original non-shield hitboxes. This is a bounded visual estimate and does not alter authoritative coordinates or damage. Aggregated shotgun damage produces one visual contact event rather than a reconstructed per-pellet trace.
- Blood uses the source message's base palette entry. The compatible engine's extra random palette offsets, original blood-stream/decal simulation and complete particle/world collision are not reproduced. Drop motion uses bounded half-gravity presentation; map depth testing hides occluded fragments.
- The two server HE sprite messages are reproduced. Full original-engine dynamic lights, flicker particles, underwater variations and subsequent engine sprite motion remain unverified. Original muzzle texture/decal artwork was not replaced by this change.
- The single-map renderer fixture is a functional observation. It does not certify complete rule matches, network performance, GPU performance, physical mobile hardware or Safari behavior.
