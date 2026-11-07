# Controller normalization work

Frozen before production implementation. Target: pinned Japan ROM SHA-256
`c39d52b2e2298b3784ada3de70d64b26b67e172114c7d18494faad70fc4d7c06`.
Reuse the existing Ghidra `68000:BE:32:default` evidence in
`docs/research/md-yuyuhakusho/static/{input-detail,logical-input,world-frame-driver}.txt`;
REA raw M68k is already unsupported. Do not install a provider or alter the core.

The native fixture must be captured and the missing-API test must fail before
`controller_normalization_work.go` exists. Each native case executes all of
original `2C9E` to its actual caller return, first by stepping and then by whole
native calls. Compare both unchanged pinned cores. Raw packets remain evidence,
never production state.

Failure scenarios frozen before implementation:

- Both port layouts: direct MD3/MD6, or four MD6 TeamPlayer pads; header/empty
  records must consume physical records without consuming logical channels.
- Desired channel count 0 through 8; stop before reading the next physical
  device; clear only active trailing channels below the requested count;
  preserve channels outside that count.
- MD6 to MD3 clears all five extension history fields only when old device
  history says MD6, even when its old held extension buttons were zero.
- Held/previous/changed/pressed/released, first press, repeat hold, release,
  A/C swap disabled/enabled and all A/C branches.
- Preserve the last normalized active/device history independently of held
  buttons. Never infer an old MD3/MD6 type from a zero-button frame.
- A completed new scan between device-type dispatch and individual nibble
  reads: dispatch uses its old latched type, each subsequent nibble uses the
  latest physical record at the same physical location. A topology change
  must not silently switch to compressed logical-channel indexing.
- Refresh at entry, exact deadline and near deadline; every semantic work
  operation must match native cycle/refresh boundaries, not just total time.
- All sixteen trusted caller reserved-carry nibbles; carry survives MD3 and
  adapter/empty records and is consumed by the first MD6 channel only.
- Hook failure at every emitted operation: stop all later hooks/getters and
  return original state. Invalid input/state and getter failures must also
  retain original state.
- Reject non-button physical bits, unsupported devices, MD3 extension input,
  contradictory physical directions, nonempty unused direct pads, TeamPlayer
  MD3, counts above 8, and invalid prior normalized device history.

The independent physical pad input uses twelve actual controls. Original `2DCE`
carries caller D1 bits8..11 into the four unused storage bits; this is observed
in the existing natural input oracle and remains supported by the unchanged
trusted `SampleController` API. A physical pad cannot supply those caller bits.
Whole-call equality therefore takes an explicit trusted transient
`ReservedCarry` nibble, preserving all sixteen-bit storage fields. It is not a
physical input and is never inferred from pad state. No CPU register shadow or
persistent carry state is allowed. The world caller must derive it from its
proved semantic entry path; unproved callers remain explicit gaps.

The live-input fixture describes an already completed scan between normalization
operations. Actual IRQ acceptance, scanner duration and return scheduling are
owned by the shared main-clock integration and are a separate E2E obligation.
