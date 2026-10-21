# Campaign presentation runtime integration

## Failure inventory before implementation

- Treat confirmed decision as animation completion; auto-start a legacy restored result or a single-match/defeat destination.
- Award twice by passing settlement output instead of pre-award Config.Campaign inventory; replace final video RNG with confirmation RNG.
- Keep duplicate live scenes in Transition and Presentation.Video; mutate immutable Outcome destinations as the ending advances/resets.
- Skip the real outgoing black fade, add an extra three-frame delay or advance the controller on the fade-completion tick.
- Advance gameplay/NMI clocks twice, call court StageDisplay for post-result actors or double-commit display in replay.
- Clear buffered input on every blocked tick; change the controller's retained dispatch sample on cancellation/automation; skip the phase9 confirmation wait.
- Lose pending/active/completed stage on JSON restore; allow corrupted stage/counter/progress combinations or mutate prior snapshots through pointers.
- Read reset Period0 as campaign progress; change score history; settle before/after animation inconsistently or rebuild a new seeded match.
- Break approved unsupported-ball recovery defaults/inheritance while extending runtime stages.

Controlled source shells are distinguished from the separately executed earned campaign match. Tests are authored before production runtime changes.
