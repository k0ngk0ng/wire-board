# Public-module automatic campaign exploration

This is a bounded diagnostic, **not final fourteen-win or real-time HTTP acceptance**. The engine's postresult video/RNG integration was still in progress during this run.

The driver calls public `Module.Configure`, `Start` and `OnTimer`, with one platform seat in its ordinary automated state. The host clock advances100ms per call; the existing timer produces normal60Hz video ticks and full match clocks. Automatic intermission, Continue and opponent/menu choices come from the existing module. No game state, score, clock, roster, inventory or RNG value is rewritten. Start generates one initial seed, and the exact initial public Match is retained; later matches continue the original RNG stream. There is no seed search or forced win.

The first transcript reached Court5/period16 after **19 actual completed matches:8 wins and11 losses**, and stopped at the start of match20. Its original summary incorrectly counted20 because the observer reused a JSON decode target and retained the preceding optional Outcome pointer. The public Match passed to OnTimer was always fresh and untouched; only observation/counters were affected. The original directory remains immutable. Court7 included repeated ordinary losses/Continue before a win.

Artifacts are under `.local/basketball/campaign-autoplay-audit-20261018/`: `initial.json`, `events.jsonl`, `final.json`, `result.json`; stdout is `.local/basketball/rea/suspended-repair-proposal/campaign-autoplay-audit.log`. Event `period`/`wins` identify the immutable match-entry progress; the next playing round shows committed progression. The final stage can still be `playing` when the bounded stop catches an already confirmed match before its remaining postresult/menu transition.

```sh
source scripts/env.sh
go run ./scripts/basketball/audit_campaign_autoplay --output .local/basketball/campaign-autoplay-audit-new --matches 20
# Replay the same initial public Match rather than generating another seed:
go run ./scripts/basketball/audit_campaign_autoplay --initial .local/basketball/campaign-autoplay-audit-20261018/initial.json --output .local/basketball/campaign-autoplay-audit-repeat --matches 20
```

On the first module error the driver writes the exact pre-transaction Match, typed failing RuntimeState and held inputs, and stops without retrying. A bounded stop is recorded separately from campaign completion. Once postresult lifecycle integration is complete, rerun from the retained initial Match and obtain separate final campaign/browser/HTTP evidence; do not reuse this exploratory result as that acceptance.


## Continued original random stream and observer correction

The continuation reads the exact original `final.json`, retains its stored ServerAt and does not invoke Start, reset seeds or choose new equipment/roster strategies. It produced **six additional wins**, through Court5, homeCourt0, Court2, homeCourt0, Court6 and final homeCourt0. The authoritative final state is round25, finalvideo22379, score130:62, campaign stagecomplete, settlement period28 and winBits[127,127]. Together the two segments contain **25 real matches,14 wins and11 losses** without a game-rule error.

The corrected driver zeroes its observer state before each JSON decode. Repeating the continuation from the exact same saved origin produces byte-identical final public Match SHA-256 `0ba52ae5450bb0ec1fdbfd68068feb81e50f7b5825164b76ad4d7a309e33ad7d`. Thus the observer defect did not alter gameplay or the continuation's outcome. The corrected continuation records6 actual completed matches and6 wins. `observer-correction-comparison.json` retains the correction and full parsed-Match comparison.

Continuation artifacts: `.local/basketball/campaign-autoplay-audit-20261018-continuation/` and the separate corrected replay `.local/basketball/campaign-autoplay-audit-20261018-continuation-verified/`. Each keeps the actual origin, source SHA, copied executable/driver SHA, complete stage transcript, final Match and bounded/complete/error classification. The initial source Match SHA is `cf947095654191e95a4261c7c0e33be1198627328a0ec4ffb0e002aa6106ee64`.

```sh
source scripts/env.sh
go run ./scripts/basketball/audit_campaign_autoplay --resume .local/basketball/campaign-autoplay-audit-20261018/final.json --output .local/basketball/campaign-autoplay-continuation-new --matches 40
```

This establishes exploratory original-AI/menu progression through all14 wins and late periods24–27. It still does **not** replace final14-win acceptance after original postresult animation/RNG scheduling is integrated, real-time authenticated HTTP progression, or multiplayer/browser coverage. That later integration changes the random stream and must be assessed separately rather than inheriting this trajectory's green status.
