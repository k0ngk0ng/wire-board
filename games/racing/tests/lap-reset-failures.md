# Lap and reset failure inventory

Original inventory was written before the reported UK three-lap change. The
following extension precedes the continuous-topology implementation after the
user reported all four production routes still losing laps:

- Any lateral-distance cutoff, including the previous 3 m margin or a larger
  replacement, can permanently strand a real excursion. Actual ordinary drive
  must exceed the paved edge by 10 m, 20 m and farther in open physical areas,
  return and continue completing laps on all four original routes.
- Dense VROAD samples must not be mandatory ordered gates. No hidden sector
  certificate, minimum driven-distance ratio or anti-shortcut rule may replace
  the withdrawn lateral limitation. Physical terrain/collisions bound driving.
- Continuous signed route winding must survive a distant return and must not
  award extra laps for stationary/reverse finish-line oscillation.
- Authoritative collision separation can exceed a guessed per-step distance;
  it must remain valid driving. Detect externally relocated snapshot positions
  from discontinuous physical-step origins, not an arbitrary travel budget.
- An old v3 snapshot with stale gate/checkpoint must resume from its current
  physical position without retroactively inventing unknown completed laps.
- Reset must choose the current same-layer road projection, not stale progress;
  preserve completed laps and exclude its positional relocation from winding.
- New tracking state is persisted privately and omitted from dynamic View,
  retaining the six-car/24-event/six-horn 8 KiB budget.
- Off-road driving must not receive artificial grip/drag from lateral distance;
  later material behavior must use actual frozen terrain-mesh contact. Unknown
  material is not automatically grass.

Original cases retained, with shortcut restrictions superseded by the user's
explicit decision to let physical course boundaries govern driving:

- A normal human line, wheel on the shoulder or brief excursion misses one of hundreds of dense road samples; later forward finish crossings remain permanently at lap 0.
- Curved or banked roads cross neighbouring gate planes in a different order even when the car remains in the local road corridor.
- Reset uses a stale gate hundreds of metres behind the actual car, changes earned laps/progress, or leaves the next checkpoint unreachable.
- Reset near start/finish grants a lap, loses an earned lap, or permits stationary/reverse repeated finish crossings.
- A nonphysical coordinate jump or bridge at a different elevation is credited
  as a driven circuit. Ordinary physically driven detours/shortcuts are allowed.
- Restore/reclaim resets progress, revives a missed-gate deadlock, or changes race identity/deadlines.
- A car completes the physical circuit but the authoritative lap, HUD and lap event disagree; final lap fails to finish exactly once.
- Legacy physics=0 saves or ordinary computer 1/3/5-lap races regress.

Primary reproduction must use production Module Start/Apply/OnTimer and ordinary driving/reset, with retained route and authority traces. Geometry-only probes are explicitly isolated and cannot substitute for a driven lap. No production race state is modified by investigation.
