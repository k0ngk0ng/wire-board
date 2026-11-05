#!/usr/bin/env python3
"""Read-only research extraction of typed Yu Yu Hakusho score parameters.

Only the read_* functions consume source locations. Returned to_data() values
contain musical/hardware semantics, with source identity retained separately by
source_provenance(). This module is neither an original-bytecode runtime nor a
CPU/DSP implementation. Fixed source table boundaries are research evidence.
"""
from dataclasses import asdict, dataclass
import hashlib
from typing import Literal

ROM_SHA = "c39d52b2e2298b3784ada3de70d64b26b67e172114c7d18494faad70fc4d7c06"
Stereo = Literal["left", "right", "both"]
Domain = Literal["music", "effect"]


def _s8(value: int) -> int:
    return (value & 127) - (value & 128)


def _integer(value: int, minimum: int, maximum: int, label: str) -> int:
    if type(value) is not int or not minimum <= value <= maximum:
        raise ValueError(f"{label} must be an integer in {minimum}..{maximum}")
    return value


def _read(data: bytes, start: int, size: int) -> bytes:
    if not isinstance(data, bytes):
        raise TypeError("source must be immutable bytes")
    _integer(start, 0, len(data), "source start")
    _integer(size, 0, len(data), "source size")
    if start + size > len(data):
        raise ValueError("truncated parameter source")
    return data[start:start + size]


def _data(value):
    if hasattr(value, "__dataclass_fields__"):
        return _data(asdict(value))
    if isinstance(value, dict):
        return {key: _data(child) for key, child in value.items()}
    if isinstance(value, (list, tuple)):
        return [_data(child) for child in value]
    return value


@dataclass(frozen=True)
class FMOperator:
    """One operator in ascending hardware register-slot order, not source order."""
    detune: int
    multiple: int
    total_level: int
    rate_scale: int
    attack_rate: int
    am_enabled: bool
    decay_rate: int
    sustain_rate: int
    sustain_level: int
    release_rate: int
    carrier: bool
    volume_carry_bias: bool = False

    def to_data(self) -> dict:
        """Frozen YM-effective view; use to_score_data() for driver behavior."""
        data = _data(self)
        del data["volume_carry_bias"]
        return data

    def to_score_data(self) -> dict:
        """Complete immutable driver and hardware semantics, without raw TL."""
        return _data(self)


@dataclass(frozen=True)
class FMPatch:
    algorithm: int
    feedback: int
    operators: tuple[FMOperator, FMOperator, FMOperator, FMOperator]

    def to_data(self) -> dict:
        """Frozen YM-effective view, retained for the original source contract."""
        return {"algorithm": self.algorithm, "feedback": self.feedback,
                "operators": [operator.to_data() for operator in self.operators]}

    def to_score_data(self) -> dict:
        return {"algorithm": self.algorithm, "feedback": self.feedback,
                "operators": [operator.to_score_data() for operator in self.operators]}


def read_fm_patch(rom: bytes, bank_base: int, instrument_index: int) -> FMPatch:
    """Read precisely one known patch; bank extent is never inferred."""
    _integer(instrument_index, 0, 255, "instrument index")
    _integer(bank_base, 0, len(rom), "instrument bank")
    source = _read(rom, bank_base + instrument_index * 25, 25)
    algorithm = source[0] & 7
    source_carriers = (8, 8, 8, 8, 10, 14, 14, 15)[algorithm]
    operators = []
    # The driver writes each source group to 0,8,4,12. Hardware register slots
    # are defined here as 0,4,8,12, so the middle two source elements swap.
    for source_slot in (0, 2, 1, 3):
        dt_mul, rs_ar, am_dr, sr, sl_rr, tl = (
            source[1 + group * 4 + source_slot] for group in range(6))
        operators.append(FMOperator(
            detune=(dt_mul >> 4) & 7, multiple=dt_mul & 15, total_level=tl & 127,
            rate_scale=rs_ar >> 6, attack_rate=rs_ar & 31,
            am_enabled=bool(am_dr & 128), decay_rate=am_dr & 31,
            sustain_rate=sr & 31, sustain_level=sl_rr >> 4,
            release_rate=sl_rr & 15, carrier=bool(source_carriers & (1 << source_slot)),
            volume_carry_bias=bool(tl & 128)))
    return FMPatch(algorithm, (source[0] >> 3) & 7, tuple(operators))


@dataclass(frozen=True)
class FMLevelUpdate:
    slot: int
    total_level: int

    def to_data(self) -> dict:
        return _data(self)


def fm_level_updates(patch: FMPatch, base_level: int,
                     *, initial: bool = False) -> tuple[FMLevelUpdate, ...]:
    """Candidate effective TL writes, in original operator-write order.

    Initial patch load writes every operator with byte-wrapped carrier volume.
    Later volume updates return early for a negative signed base level and skip
    each carrier whose driver TL plus base level carries. The immutable bias is
    the high arithmetic bit; the hardware total level remains seven bits.
    Claim/override suppression belongs to the caller's voice owner.
    """
    _integer(base_level, 0, 255, "FM base attenuation")
    if type(initial) is not bool:
        raise ValueError("initial FM level update must be boolean")
    if not initial and base_level >= 128:
        return ()
    result = []
    for slot in (0, 2, 1, 3):
        operator = patch.operators[slot]
        if initial:
            result.append(FMLevelUpdate(slot, (operator.total_level + (base_level if operator.carrier else 0)) & 127))
        elif operator.carrier:
            driver_input = operator.total_level + (128 if operator.volume_carry_bias else 0)
            if driver_input + base_level <= 255:
                result.append(FMLevelUpdate(slot, (operator.total_level + base_level) & 127))
    return tuple(result)


@dataclass(frozen=True)
class PitchTick:
    """One calculation tick; a terminal keyoff bypass has a literal word result."""
    offset: int | None
    scale_add: int = 0
    key_off: bool = False
    fixed_pitch_word: int | None = None

    def to_data(self) -> dict:
        return _data(self)


@dataclass(frozen=True)
class PitchEnvelope:
    id: int
    prefix: tuple[PitchTick, ...]
    cycle: tuple[PitchTick, ...]
    ended: bool

    def to_data(self) -> dict:
        return _data(self)


@dataclass(frozen=True)
class VolumeTick:
    """A candidate level write or a no-write/keyoff action before voice gating."""
    level: int | None
    writes_level: bool
    key_off: bool = False

    def to_data(self) -> dict:
        return _data(self)


@dataclass(frozen=True)
class VolumeEnvelope:
    id: int
    prefix: tuple[VolumeTick, ...]
    cycle: tuple[VolumeTick, ...]

    def to_data(self) -> dict:
        return _data(self)


def _value(source: bytes, position: int) -> int:
    if not 0 <= position < len(source):
        raise ValueError("envelope reads outside its bounded source")
    return source[position]


def compile_pitch_envelope(data: bytes, envelope_id: int) -> PitchEnvelope:
    """Eliminate source control instructions into typed ticks and one true cycle."""
    if not isinstance(data, bytes) or not data:
        raise ValueError("pitch envelope source must be nonempty immutable bytes")
    _integer(envelope_id, 1, 65535, "pitch envelope id")
    position = 0
    steps: list[PitchTick] = []
    boundary = {}
    while position not in boundary:
        boundary[position] = len(steps)
        visited = set()
        scale_add = 0
        while True:
            if position in visited:
                raise ValueError("pitch envelope has a same-tick source cycle")
            visited.add(position)
            at = position
            sample = _value(data, at)
            position = (position + 1) & 255
            if sample == 128:
                position = 0
                continue
            if sample == 129:
                position = (position - 2) & 255
                continue
            if sample == 130:
                position = _value(data, at + 1)
                continue
            if sample == 132:
                scale_add = (scale_add + _value(data, at + 1)) & 255
                position = (position + 1) & 255
                continue
            if sample == 131:
                steps.append(PitchTick(None, _s8(scale_add), True, 131))
                return PitchEnvelope(envelope_id, tuple(steps), (), True)
            steps.append(PitchTick(_s8(sample), _s8(scale_add)))
            break
    first = boundary[position]
    return PitchEnvelope(envelope_id, tuple(steps[:first]), tuple(steps[first:]), False)


def compile_psg_volume_envelope(data: bytes, envelope_id: int) -> VolumeEnvelope:
    """Preserve volume hold/keyoff no-write cadence, including repeated keyoff."""
    if not isinstance(data, bytes) or not data:
        raise ValueError("volume envelope source must be nonempty immutable bytes")
    _integer(envelope_id, 1, 65535, "volume envelope id")
    position = 0
    steps: list[VolumeTick] = []
    boundary = {}
    while position not in boundary:
        boundary[position] = len(steps)
        visited = set()
        while True:
            if position in visited:
                raise ValueError("volume envelope has a same-tick source cycle")
            visited.add(position)
            at = position
            sample = _value(data, at)
            position = (position + 1) & 255
            if sample == 128:
                position = 0
                continue
            if sample == 130:
                position = _value(data, at + 1)
                continue
            if sample in (129, 131):
                position = (position - 2) & 255
                steps.append(VolumeTick(None, False, sample == 131))
            else:
                # Unlike pitch, this path does not sign-extend the table byte.
                steps.append(VolumeTick(sample, True))
            break
    first = boundary[position]
    return VolumeEnvelope(envelope_id, tuple(steps[:first]), tuple(steps[first:]))


def _read_envelopes(rom: bytes, table: int, count: int, end: int, compiler):
    pointers = _read(rom, table, count * 4)
    _read(rom, end - 1, 1)
    result = []
    for index in range(count):
        start = int.from_bytes(pointers[index * 4:index * 4 + 4], "big")
        if not table + count * 4 <= start < end:
            raise ValueError("envelope pointer outside the proved parameter range")
        # Some entries point into another envelope's suffix. Another pointer is
        # not a terminator; actual same-tick semantics determine the boundary.
        result.append(compiler(rom[start:end], index + 1))
    return tuple(result)


def read_pitch_envelopes(rom: bytes) -> tuple[PitchEnvelope, ...]:
    return _read_envelopes(rom, 0x3a256, 8, 0x3a31a, compile_pitch_envelope)


def read_psg_volume_envelopes(rom: bytes) -> tuple[VolumeEnvelope, ...]:
    return _read_envelopes(rom, 0x3a31a, 11, 0x3a492, compile_psg_volume_envelope)


def pitch_word(step: PitchTick, scale: int, base_pitch: int,
               detune: int = 0, modulation: int = 0) -> tuple[int, int]:
    _integer(scale, -128, 127, "pitch envelope scale")
    _integer(base_pitch, 0, 65535, "base pitch")
    _integer(detune, -128, 127, "detune")
    _integer(modulation, -32768, 65535, "modulation")
    next_scale = _s8(scale + step.scale_add)
    if step.fixed_pitch_word is not None:
        return step.fixed_pitch_word, next_scale
    if step.offset is None:
        raise ValueError("pitch tick lacks a resolved calculation")
    return (step.offset * next_scale + base_pitch + detune + modulation) & 65535, next_scale


def psg_attenuation(step: VolumeTick, base_level: int) -> int | None:
    _integer(base_level, 0, 255, "PSG base attenuation")
    if not step.writes_level:
        return None
    if step.level is None:
        raise ValueError("volume write lacks a level")
    attenuation = (base_level + step.level) & 255
    return attenuation if attenuation < 16 else 15


@dataclass(frozen=True)
class PanPattern:
    id: str
    steps: tuple[Stereo, ...]

    def to_data(self) -> dict:
        return _data(self)


_PAN_PATTERNS = (
    PanPattern("alternating", ("right", "left")),
    PanPattern("through-center", ("right", "both", "left")),
    PanPattern("center-return", ("both", "left", "both", "right")),
)
_SCORE_PAN_PATTERNS = (
    PanPattern("right-left-right-center", ("right", "left", "right", "both")),
    PanPattern("right-center-left-center", ("right", "both", "left", "both")),
    PanPattern("center-left-center-right", ("both", "left", "both", "right")),
)
_PAN_MODES = ("disabled", "advance-on-note", "restart-on-note-once",
              "restart-on-note-loop", "restart-every-driver-tick")


def read_pan_patterns(rom: bytes) -> tuple[PanPattern, ...]:
    """Frozen nominal source-table views; score extraction uses shared windows."""
    expected_pointers = (0x37b2a, 0x37b2c, 0x37b2f)
    pointers = _read(rom, 0x37b1e, 12)
    actual_pointers = tuple(int.from_bytes(pointers[index:index + 4], "big") for index in range(0, 12, 4))
    if actual_pointers != expected_pointers:
        raise ValueError("pan pattern pointers differ from the proved source")
    encode = {"right": 64, "left": 128, "both": 192}
    for start, pattern in zip(actual_pointers, _PAN_PATTERNS):
        if _read(rom, start, len(pattern.steps)) != bytes(encode[step] for step in pattern.steps):
            raise ValueError("pan pattern differs from the proved source")
    return _PAN_PATTERNS


def read_score_pan_patterns(rom: bytes) -> tuple[PanPattern, ...]:
    """Four-step shared windows, including all indices actually read by scores.

    Adjacent pointers do not delimit these arrays. The native score parameters
    use index three from the first two pointers and therefore overlap nominal
    table views. No larger unobserved window is inferred.
    """
    read_pan_patterns(rom)
    encode = {"right": 64, "left": 128, "both": 192}
    for start, pattern in zip((0x37b2a, 0x37b2c, 0x37b2f), _SCORE_PAN_PATTERNS):
        if _read(rom, start, len(pattern.steps)) != bytes(encode[step] for step in pattern.steps):
            raise ValueError("score pan window differs from the proved source")
    return _SCORE_PAN_PATTERNS


@dataclass(frozen=True)
class PanAutomation:
    mode: str
    pattern: PanPattern | None
    initial_step: int
    step_limit: int
    period: int

    def to_data(self) -> dict:
        if self.mode == "disabled":
            return {"mode": self.mode}
        return {"mode": self.mode, "pattern": self.pattern.id,
                "initial_step": self.initial_step, "step_limit": self.step_limit,
                "period": self.period}


@dataclass(frozen=True)
class PanState:
    step: int
    elapsed: int


@dataclass(frozen=True)
class PanTick:
    state: PanState
    pan: Stereo | None


def decode_pan_automation(mode: int, pattern: int = 0, initial_step: int = 0,
                          step_limit: int = 0, period: int = 0) -> PanAutomation:
    """Frozen nominal-table contract; use decode_score_pan_automation for scores."""
    _integer(mode, 0, 4, "pan automation mode")
    _integer(initial_step, 0, 255, "pan initial step")
    _integer(step_limit, 0, 255, "pan last step index")
    _integer(period, 0, 255, "pan period")
    if mode == 0:
        return PanAutomation(_PAN_MODES[0], None, initial_step, step_limit, period)
    _integer(pattern, 1, len(_PAN_PATTERNS), "pan pattern")
    selected = _PAN_PATTERNS[pattern - 1]
    if step_limit >= len(selected.steps) or initial_step > step_limit + 1:
        raise ValueError("pan step parameters exceed the proved named pattern")
    return PanAutomation(_PAN_MODES[mode], selected, initial_step, step_limit, period)


def decode_score_pan_automation(mode: int, pattern: int = 0, initial_step: int = 0,
                                step_limit: int = 0, period: int = 0) -> PanAutomation:
    """Decode the actual score domain with shared windows and reset-before-use."""
    _integer(mode, 0, 4, "score pan automation mode")
    if mode == 0:
        return PanAutomation(_PAN_MODES[0], None, 0, 0, 0)
    _integer(pattern, 1, len(_SCORE_PAN_PATTERNS), "score pan pattern")
    _integer(initial_step, 0, 3, "proved score pan initial step")
    _integer(step_limit, 0, 3, "proved score pan last step index")
    _integer(period, 0, 255, "score pan period")
    # Initial step need not lie below the active limit: BPL/reset dispatch can
    # replace it before its first lookup. Every accepted value is in the actual
    # score domain and every emitted lookup remains bounded by the stepper.
    return PanAutomation(_PAN_MODES[mode], _SCORE_PAN_PATTERNS[pattern - 1],
                         initial_step, step_limit, period)


def initial_pan_state(definition: PanAutomation) -> PanState:
    return PanState(definition.initial_step, definition.period)


def advance_pan_automation(definition: PanAutomation, state: PanState,
                           note_boundary: bool, key_off: bool = False) -> PanTick:
    mode = definition.mode
    if mode not in _PAN_MODES:
        raise ValueError("unknown named pan automation mode")
    _integer(state.step, 0, 255, "pan step")
    _integer(state.elapsed, 0, 255, "pan elapsed ticks")
    if key_off or mode == "disabled" or (mode == "advance-on-note" and not note_boundary):
        return PanTick(state, None)
    step, elapsed = state.step, state.elapsed
    if mode == "restart-every-driver-tick" or (note_boundary and mode in ("restart-on-note-once", "restart-on-note-loop")):
        step, elapsed = 0, definition.period
    if elapsed == definition.period:
        # Original BPL tests the signed low-byte difference, rather than an
        # unbounded integer ordering. Bounded named patterns retain that rule.
        if _s8(definition.step_limit - step) < 0:
            if mode == "restart-on-note-once":
                return PanTick(PanState(step, elapsed), None)
            step = 0
        elapsed = 0
        step = (step + 1) & 255
    if definition.pattern is None or not 1 <= step <= len(definition.pattern.steps):
        raise ValueError("pan output exceeds the proved named pattern")
    return PanTick(PanState(step, (elapsed + 1) & 255), definition.pattern.steps[step - 1])


@dataclass(frozen=True)
class LFOControl:
    am_enable_slots: tuple[int, ...]
    lfo_enabled: bool
    lfo_rate: int
    ams: int
    fms: int
    enable_left: bool
    enable_right: bool

    def to_data(self) -> dict:
        return _data(self)


@dataclass(frozen=True)
class AMDecayUpdate:
    slot: int
    decay_rate: int
    am_enabled: bool = True

    def to_data(self) -> dict:
        return _data(self)


def decode_e9(first: int, second: int) -> LFOControl:
    _integer(first, 0, 255, "LFO and AM selection")
    _integer(second, 0, 255, "pan, AMS and FMS selection")
    slots = tuple(slot for slot, source_slot in enumerate((0, 2, 1, 3)) if first & (128 >> source_slot))
    return LFOControl(slots, bool(first & 8), first & 7, (second >> 4) & 3,
                      second & 7, bool(second & 128), bool(second & 64))


def e9_am_updates(rom: bytes, bank_base: int, control: LFOControl) -> tuple[AMDecayUpdate, ...]:
    """Return actual source-order AM writes, retaining the original bank-read bug.

    The source increments A0 by nine while reading through A1. A1 therefore
    reads four bytes from the selected bank base, not the active patch's decay
    group. Caller must supply the bank actually observed by the native oracle.
    Only hardware-effective AM/decay fields are represented here.
    """
    values = _read(rom, bank_base, 4)
    return tuple(AMDecayUpdate(slot, values[source_slot] & 31)
                 for source_slot, slot in enumerate((0, 2, 1, 3))
                 if slot in control.am_enable_slots)


@dataclass(frozen=True)
class FM3Offsets:
    """Four frequency offsets in the same register-slot order as FMPatch."""
    frequency_offsets: tuple[int, int, int, int]
    domain: Domain

    def to_data(self) -> dict:
        return {"mode": "independent-frequency", "domain": self.domain,
                "frequency_offsets": list(self.frequency_offsets)}


def read_fm3_offsets(rom: bytes, indices: tuple[int, ...], domain: Domain) -> FM3Offsets:
    if domain not in ("music", "effect"):
        raise ValueError("FM3 domain must be music or effect")
    if not isinstance(indices, tuple) or len(indices) != 4:
        raise ValueError("FM3 requires four frequency-port selections")
    table = _read(rom, 0x39250, 8)
    offsets = []
    for index in indices:
        _integer(index, 0, 3, "proved FM3 offset selection")
        offsets.append(int.from_bytes(table[index * 2:index * 2 + 2], "big"))
    return FM3Offsets(tuple(offsets), domain)


def source_provenance(rom: bytes) -> dict:
    """Research-only identity and ranges; keep separate from every to_data()."""
    digest = hashlib.sha256(rom).hexdigest()
    if digest != ROM_SHA:
        raise ValueError("ROM differs from the pinned Japan source")
    ranges = {
        "fm_register_order": (0x39080, 24), "fm_carrier_masks": (0x39014, 8),
        "pitch_pointers": (0x3a256, 32), "pitch_data": (0x3a276, 164),
        "volume_pointers": (0x3a31a, 44), "volume_data": (0x3a346, 332),
        "pan_dispatch_and_step": (0x37a8a, 148), "pan_pointers": (0x37b1e, 12),
        "pan_patterns": (0x37b2a, 9), "lfo_control": (0x38ed0, 78),
        "fm3_offset_table": (0x39250, 8), "fm3_frequency_ports": (0x37a82, 8),
    }
    return {"rom_sha256": digest, "route": "source bytes and direct Ghidra 68000 listing; no REA raw-68000 support",
            "fm3_frequency_slot_order": [0, 1, 2, 3],
            "ranges": {name: {"start": start, "size": size,
                               "sha256": hashlib.sha256(_read(rom, start, size)).hexdigest()}
                       for name, (start, size) in ranges.items()},
            "unknowns": ["Actual instrument bank extents are not inferred",
                         "Native reads and exact write order require the separate read oracle",
                         "No DSP or audible playback claim"]}
