# Lap and reset failure inventory

Written before implementation for the reported UK three-lap race:

- A normal human line, wheel on the shoulder or brief excursion misses one of hundreds of dense road samples; later forward finish crossings remain permanently at lap 0.
- Curved or banked roads cross neighbouring gate planes in a different order even when the car remains in the local road corridor.
- Reset uses a stale gate hundreds of metres behind the actual car, changes earned laps/progress, or leaves the next checkpoint unreachable.
- Reset near start/finish grants a lap, loses an earned lap, or permits stationary/reverse repeated finish crossings.
- A detached shortcut, jump to a remote route, bridge at a different elevation, or skipped large course section receives full race credit.
- Restore/reclaim resets progress, revives a missed-gate deadlock, or changes race identity/deadlines.
- A car completes the physical circuit but the authoritative lap, HUD and lap event disagree; final lap fails to finish exactly once.
- Legacy physics=0 saves or ordinary computer 1/3/5-lap races regress.

Primary reproduction must use production Module Start/Apply/OnTimer and ordinary driving/reset, with retained route and authority traces. Geometry-only probes are explicitly isolated and cannot substitute for a driven lap. No production race state is modified by investigation.
