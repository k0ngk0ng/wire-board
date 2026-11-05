#!/usr/bin/env python3
"""Independent source-contract checks for read-only typed score parameters.

The raw interpreters below follow saved 68000 instruction effects and never call
the helper's envelope compiler or pan stepper to obtain expected values.
"""
import argparse
import hashlib
import importlib
import json
from pathlib import Path
import re
import sys

sys.dont_write_bytecode = True
ROOT = Path(__file__).resolve().parents[2]
ROM_SHA = "c39d52b2e2298b3784ada3de70d64b26b67e172114c7d18494faad70fc4d7c06"


def s8(value):
    return (value & 127) - (value & 128)


def must_reject(call):
    try:
        call()
    except (ValueError, TypeError, IndexError):
        return
    raise AssertionError("invalid parameter accepted")


def expanded(envelope, count):
    result = list(envelope.prefix)
    if envelope.cycle:
        while len(result) < count:
            result.extend(envelope.cycle)
    return result[:count]


def raw_pitch(data, count):
    cursor = 0
    result = []
    for _ in range(count):
        delta = 0
        seen = set()
        while True:
            if cursor in seen:
                raise ValueError("pitch same-tick source loop")
            seen.add(cursor)
            if not 0 <= cursor < len(data):
                raise ValueError("pitch source out of range")
            index = cursor
            value = data[index]
            cursor = (cursor + 1) & 255
            if value == 128:
                cursor = 0
            elif value == 129:
                cursor = (cursor - 2) & 255
            elif value == 130:
                if index + 1 >= len(data):
                    raise ValueError("short pitch jump")
                cursor = data[index + 1]
            elif value == 132:
                if index + 1 >= len(data):
                    raise ValueError("short pitch scale-add")
                delta = (delta + data[index + 1]) & 255
                cursor = (cursor + 1) & 255
            elif value == 131:
                result.append({"offset": None, "scale_add": s8(delta), "key_off": True, "fixed_pitch_word": 131})
                return result
            else:
                result.append({"offset": s8(value), "scale_add": s8(delta), "key_off": False, "fixed_pitch_word": None})
                break
    return result


def raw_volume(data, count):
    cursor = 0
    result = []
    for _ in range(count):
        seen = set()
        while True:
            if cursor in seen:
                raise ValueError("volume same-tick source loop")
            seen.add(cursor)
            if not 0 <= cursor < len(data):
                raise ValueError("volume source out of range")
            index = cursor
            value = data[index]
            cursor = (cursor + 1) & 255
            if value == 128:
                cursor = 0
                continue
            if value == 130:
                if index + 1 >= len(data):
                    raise ValueError("short volume jump")
                cursor = data[index + 1]
                continue
            if value in (129, 131):
                cursor = (cursor - 2) & 255
                result.append({"level": None, "writes_level": False, "key_off": value == 131})
            else:
                result.append({"level": value, "writes_level": True, "key_off": False})
            break
    return result


def raw_pan(mode, pattern, initial_step, limit, period, calls):
    patterns = (("right", "left"), ("right", "both", "left"), ("both", "left", "both", "right"))
    step, elapsed = initial_step, period
    output = []
    for on_note, key_off in calls:
        pan = None
        enabled = mode != 0 and not key_off and (on_note or mode >= 2)
        if enabled:
            if (on_note and mode in (2, 3, 4)) or (not on_note and mode == 4):
                step, elapsed = 0, period
            halted = False
            if elapsed == period:
                if s8(limit - step) < 0:
                    if mode == 2:
                        halted = True
                    else:
                        step = 0
                if not halted:
                    elapsed = 0
                    step = (step + 1) & 255
            if not halted:
                pan = patterns[pattern - 1][step - 1]
                elapsed = (elapsed + 1) & 255
        output.append((step, elapsed, pan))
    return output


def no_source_data(value):
    forbidden = {"address", "source", "source_address", "source_bytes", "raw", "hex", "opcode", "pc", "cursor", "bank_base"}
    if isinstance(value, dict):
        assert not (forbidden & value.keys()), value.keys()
        for child in value.values():
            no_source_data(child)
    elif isinstance(value, (tuple, list)):
        for child in value:
            no_source_data(child)
    else:
        assert not isinstance(value, bytes)


def check(helper, rom):
    totals = {"pitch_source_ticks": 0, "volume_source_ticks": 0, "pan_source_calls": 0}
    source = helper.source_provenance(rom)
    assert source["rom_sha256"] == ROM_SHA
    chip_path = ROOT / ".local/md-yuyuhakusho/tools/Genesis-Plus-GX-60bee4c724859710ce05672239e5b601ad01382e/core/sound/ym2612.c"
    chip_bytes = chip_path.read_bytes()
    assert hashlib.sha256(chip_bytes).hexdigest() == "67bca180dd13c61a1e68901035bba871bb0124fe09a4b79192d6242013672a04"
    chip = chip_bytes.decode()
    slots = {int(number): int(slot) for number, slot in re.findall(r"#define SLOT(\d) (\d)", chip)}
    special = {int(port): slots[int(number)] for number, port in re.findall(r"refresh_fc_eg_slot\(&ym2612.CH\[2\].SLOT\[SLOT(\d)\]\s*, ym2612.OPN.SL3.fc\[(\d)\]", chip)}
    ports = list(rom[0x37a83:0x37a8a:2])
    routing = [special[port & 3] if port >= 0xa8 else slots[4] for port in ports]
    assert routing == [0, 1, 2, 3]
    assert source["fm3_frequency_slot_order"] == routing
    must_reject(lambda: helper.source_provenance(bytes(len(rom))))
    must_reject(lambda: helper.read_fm_patch(rom, len(rom) - 24, 0))
    must_reject(lambda: helper.read_fm_patch(rom, 0, -1))
    must_reject(lambda: helper.read_fm_patch(rom, 0, 256))
    must_reject(lambda: helper.read_fm_patch(rom, 0, True))
    # Every bit in every meaningful field, all carrier arrangements, and source
    # slot permutation are independently reconstructed through register writes.
    for algorithm in range(8):
        raw = bytes([algorithm | 5 << 3] + [((group * 19 + slot * 7 + 3) & 255) for group in range(6) for slot in range(4)])
        patch = helper.read_fm_patch(raw, 0, 0)
        data = patch.to_data()
        assert data["algorithm"] == algorithm and data["feedback"] == 5
        mask = (8, 8, 8, 8, 10, 14, 14, 15)[algorithm]
        for slot, source_slot in enumerate((0, 2, 1, 3)):
            fields = [raw[1 + group * 4 + source_slot] for group in range(6)]
            expected = {"detune": (fields[0] >> 4) & 7, "multiple": fields[0] & 15,
                        "rate_scale": fields[1] >> 6, "attack_rate": fields[1] & 31,
                        "am_enabled": bool(fields[2] & 128), "decay_rate": fields[2] & 31,
                        "sustain_rate": fields[3] & 31, "sustain_level": fields[4] >> 4,
                        "release_rate": fields[4] & 15, "total_level": fields[5] & 127,
                        "carrier": bool(mask & (1 << source_slot))}
            assert data["operators"][slot] == expected, (slot, data["operators"][slot], expected)
        no_source_data(data)
    # Extra leading patch verifies index is zero-based and exactly 25 bytes.
    assert helper.read_fm_patch(bytes(25) + raw, 0, 1).to_data() == patch.to_data()
    for kind, reader, raw_model, pointer_base, count in (
            ("pitch", helper.read_pitch_envelopes, raw_pitch, 0x3a256, 8),
            ("volume", helper.read_psg_volume_envelopes, raw_volume, 0x3a31a, 11)):
        envelopes = reader(rom)
        assert len(envelopes) == count
        pointers = [int.from_bytes(rom[pointer_base + 4*i:pointer_base + 4*i + 4], "big") for i in range(count)]
        end = 0x3a31a if kind == "pitch" else 0x3a492
        for index, (envelope, start) in enumerate(zip(envelopes, pointers), 1):
            expected = raw_model(rom[start:end], 2048)
            actual = [tick.to_data() for tick in expanded(envelope, 2048)]
            assert actual == expected, (kind, index, actual[:4], expected[:4])
            totals[kind + "_source_ticks"] += len(actual)
            no_source_data(envelope.to_data())
    pitch_cases = (bytes([3, 129]), bytes([255, 130, 0]), bytes([132, 255, 2, 128]), bytes([132, 127, 132, 2, 131]))
    volume_cases = (bytes([3, 129]), bytes([3, 131]), bytes([255, 130, 0]), bytes([1, 2, 128]))
    for compiler, raw_model, cases in ((helper.compile_pitch_envelope, raw_pitch, pitch_cases), (helper.compile_psg_volume_envelope, raw_volume, volume_cases)):
        for data in cases:
            result = compiler(data, 99)
            assert [tick.to_data() for tick in expanded(result, 40)] == raw_model(data, 40)
        for data in (b"", bytes([128]), bytes([130]), bytes([130, 200]), bytes([129])):
            must_reject(lambda data=data: compiler(data, 99))
    for data in (bytes([132]), bytes([132, 1, 128])):
        must_reject(lambda data=data: helper.compile_pitch_envelope(data, 99))
    example = helper.compile_pitch_envelope(bytes([132, 2, 255, 128]), 99)
    tick = expanded(example, 1)[0]
    assert helper.pitch_word(tick, 127, 65530, -2, 9) == (128, -127)
    bypass = expanded(helper.compile_pitch_envelope(bytes([131]), 99), 1)[0]
    assert helper.pitch_word(bypass, 7, 1234, 3, 4) == (131, 7)
    for level in (0, 1, 14, 15, 16, 127, 240, 255):
        value = expanded(helper.compile_psg_volume_envelope(bytes([level, 128]), 99), 1)[0]
        for base in (0, 1, 15, 127, 255):
            expected = (base + level) & 255
            expected = expected if expected < 16 else 15
            assert helper.psg_attenuation(value, base) == expected
    patterns = helper.read_pan_patterns(rom)
    assert [list(pattern.steps) for pattern in patterns] == [["right", "left"], ["right", "both", "left"], ["both", "left", "both", "right"]]
    calls = [(index % 7 == 0, index % 13 == 12) for index in range(600)]
    for mode in range(5):
        for pattern, length in enumerate((2, 3, 4), 1):
            for period in (0, 1, 3, 255):
                definition = helper.decode_pan_automation(mode, pattern, 0, length - 1, period)
                state = helper.initial_pan_state(definition)
                actual = []
                for note, key_off in calls:
                    outcome = helper.advance_pan_automation(definition, state, note, key_off)
                    state = outcome.state
                    actual.append((state.step, state.elapsed, outcome.pan))
                assert actual == raw_pan(mode, pattern, 0, length - 1, period, calls), (mode, pattern, period)
                totals["pan_source_calls"] += len(calls)
                no_source_data(definition.to_data())
    for args in ((5, 1, 0, 1, 1), (1, 0, 0, 1, 1), (1, 4, 0, 1, 1), (1, 1, 0, 2, 1), (1, 1, 0, 1, 256)):
        must_reject(lambda args=args: helper.decode_pan_automation(*args))
    for first in range(256):
        second = (first * 7) & 255
        result = helper.decode_e9(first, second)
        d = result.to_data()
        assert d["am_enable_slots"] == [slot for slot, source_slot in enumerate((0, 2, 1, 3)) if first & (128 >> source_slot)]
        assert d["lfo_enabled"] == bool(first & 8) and d["lfo_rate"] == first & 7
        assert d["ams"] == (second >> 4) & 3 and d["fms"] == second & 7
        assert d["enable_left"] == bool(second & 128) and d["enable_right"] == bool(second & 64)
        updates = helper.e9_am_updates(bytes([0x17, 0x2e, 0x45, 0x6c]), 0, result)
        assert [(update.slot, update.decay_rate, update.am_enabled) for update in updates] == [(slot, [0x17, 0x45, 0x2e, 0x6c][slot] & 31, True) for slot in (0, 2, 1, 3) if slot in d["am_enable_slots"]]
        no_source_data(d)
    for domain in ("music", "effect"):
        fm3 = helper.read_fm3_offsets(rom, (0, 1, 2, 3), domain)
        assert fm3.frequency_offsets == (0, 384, 500, 608) and fm3.domain == domain
        no_source_data(fm3.to_data())
    for args in (((0, 1, 2), "music"), ((0, 1, 2, 4), "music"), ((0, 1, 2, 3), "both")):
        must_reject(lambda args=args: helper.read_fm3_offsets(rom, *args))
    return {"passed": True, "scope": "source-table and typed-helper isolation; native reads and playback remain separate", "totals": totals}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--out", type=Path)
    args = parser.parse_args()
    rom = (ROOT / ".local/md-yuyuhakusho/downloads/yuyuhakusho-jp.bin").read_bytes()
    assert hashlib.sha256(rom).hexdigest() == ROM_SHA
    helper = importlib.import_module("score_parameter_tables")
    result = check(helper, rom)
    result["rom_sha256"] = ROM_SHA
    result["checker_sha256"] = hashlib.sha256(Path(__file__).read_bytes()).hexdigest()
    result["helper_sha256"] = hashlib.sha256(Path(helper.__file__).read_bytes()).hexdigest()
    encoded = json.dumps(result, indent=2) + "\n"
    if args.out:
        output = args.out.resolve()
        if not output.is_relative_to(ROOT):
            raise ValueError("output outside worktree")
        output.parent.mkdir(parents=True, exist_ok=True)
        output.write_text(encoded)
    print(encoded, end="")


if __name__ == "__main__":
    main()
