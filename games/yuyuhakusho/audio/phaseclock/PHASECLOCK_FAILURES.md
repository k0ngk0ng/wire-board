# Reference bus arithmetic: failures declared before implementation

- Dropping the DMA wait, applying 70/77 before the wait, or clearing an existing wait flag.
- Computing the phase from absolute master time rather than the current reference frame.
- Counting bank reads once per decoded byte instead of every prefetch/dummy read supplied by the player.
- Treating DMA fill/copy modes 2/3 as ROM-bus hold, or accepting an unknown mode.
- Wrapping the unsigned resume cycle at uint32 maximum, or returning a partial effect on error.
- Claiming this pure effect resolves the timing or number of bank reads, the 68k world clock, IRQ or real hardware.

The native corpus is exported solely from original before/after observations.
The first test run must fail on the absent public helper, before implementation.
