# Controller normalization work evidence

`NormalizeControllersWork` implements original complete `2C9E` normalization as
independent semantic Go, using typed transient direct/TeamPlayer port samples.
It emits each `MainWorkOperation` synchronously. It stores neither original
80-byte packets, instructions, CPU registers nor an operation history.

The input contains a desired channel count (0..8), per-channel A/C swap mask,
physical ports, and an explicit trusted `ReservedCarry` nibble. Saved state
contains eight normalized frames and last normalized device types (0 inactive,
3 MD3, 6 MD6). Device history is independent of held buttons. All five frame
fields preserve sixteen-bit original storage semantics; external pad buttons
remain twelve-bit. Existing `SampleController` is unchanged.

Original `2DCE` propagates the caller's D1 bits8..11 into bits12..15 of the first
MD6 channel. MD3 and empty/adapter records retain this carry; the first MD6
consumes it. The API names that transient semantic nibble explicitly instead of
persisting a register file or guessing it from a previous button frame. The
existing natural input oracle has 3,066 MD6 calls from battle frame2068 onward,
all with zero carry. Earlier callers include nonzero carry and remain covered
by the unchanged trusted input oracle and all sixteen new native carry cases.

`test-first.json` binds the complete native oracle, tests, failure-scenario plan,
generator, and missing-API red result before production implementation existed.
The original frozen files and expectations are unchanged. `native-first/`
retains 689 whole original calls and 147,216 source and portable operation
boundaries. Both pinned cores agree at every boundary; uninterrupted original
native execution agrees with stepping. Physical MD3 devices were scanned by
the original scanner; other inputs reuse the 576 retained settled/cold scanner
packets. Sixteen cases replace a completed scan after an actual type or nibble
read. Desired counts0..8 and carry nibbles0..15 are all present.

The work-hook contract is **operation complete after latch**: a read first
captures its device/nibble, then its hook advances to that operation's completed
cycle/refresh boundary. The native replacement fixture changes ports after the
specified completed operation. A shared-clock coordinator that services IRQs
before pricing an operation must reuse its preceding-batch synchronization/IRQ
prelude inside `LivePorts`, before returning the latest ports. This prelude
does not charge the current read. The work hook then advances the read itself;
its repeated prelude has no pending work to consume. An old type remains
latched across an IRQ while each later nibble reads the latest physical record
at that same physical location.

`final-native/validation.json` records the native race checks, vet, and a second
complete native capture byte-identical to the frozen fixture and source steps.
`final-wasm/validation.json` records the same bounded Go checks and vet on Wasm.
The first implementation passed the frozen expectations. Hook errors at every
operation and getter failures at32 read points preserve the original state and
suppress subsequent hooks/reads. Legacy input and full scanner regressions pass.

Reproduce from the worktree root with new, nonexistent output directories:

```sh
python3 -B scripts/yuyuhakusho/verify_controller_normalization_work.py --out games/yuyuhakusho/verification/controller-normalization-work/recheck-native --recapture
python3 -B scripts/yuyuhakusho/verify_controller_normalization_work.py --out games/yuyuhakusho/verification/controller-normalization-work/recheck-wasm --wasm
```

The verifier directs Go telemetry/cache/module/temp output into the worktree,
records commands, results and source SHA-256 values, and checks frozen hashes
before running. No tools are installed and pinned ROM/core sources are read only.

Scope limits: completed-scan replacement establishes normalization latch
semantics; actual IRQ acceptance, scanner duration and return scheduling belong
to the shared main-clock integration. Inactive flags with bit6 set but bit7
clear, and lower reserved device-flag bits, are outside the typed device-history
domain. No full-world timing or unrelated-game acceptance is claimed here.
