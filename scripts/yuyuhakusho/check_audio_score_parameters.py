#!/usr/bin/env python3
"""Actual-read and raw-request evidence for typed immutable score parameters."""
import argparse
import collections
import gzip
import hashlib
import json
from pathlib import Path
import sys

sys.dont_write_bytecode = True
ROOT = Path(__file__).resolve().parents[2]
sys.path.insert(0, str(ROOT / "scripts/yuyuhakusho"))
import score_parameter_tables as helper

ROM_SHA = "c39d52b2e2298b3784ada3de70d64b26b67e172114c7d18494faad70fc4d7c06"


def sha(raw):
    return hashlib.sha256(raw).hexdigest()


def require(condition, message, event=None):
    if not condition:
        raise AssertionError((message, event))


def bind(path):
    path = path.resolve()
    require(path.is_relative_to(ROOT), "input/output outside worktree")
    return {"path": str(path.relative_to(ROOT)), "sha256": sha(path.read_bytes())}


def next_requests(events, index, kind, limit):
    origin = events[index]
    result = []
    for event in events[index + 1:index + 120]:
        if event["tick"] != origin["tick"]:
            break
        if event.get("row") != origin["row"]:
            continue
        if event["kind"] == kind:
            result.append(event)
            if len(result) == limit:
                break
        elif event["kind"] in ("control-after", "fm-patch-read"):
            break
    return result


def next_volume_write(events, index):
    origin = events[index]
    for event in events[index + 1:index + 120]:
        if event["tick"] != origin["tick"]:
            break
        if event.get("row") != origin["row"]:
            continue
        if event["kind"] == "volume-envelope-read":
            break
        if event["kind"] == "psg-request" and event["pc"] in (0x3a140, 0x3a186):
            return event
    return None


def scope_of(row):
    return "effect" if row >= 0xfa20 else "music"


def raw_pan_calls(rom, values, calls):
    mode, pattern, step, limit, period = values
    elapsed = period
    output = []
    base = int.from_bytes(rom[0x37b1e + (pattern - 1) * 4:0x37b22 + (pattern - 1) * 4], "big") if mode else 0
    for note, off in calls:
        pan = None
        if mode and not off and (note or mode >= 2):
            if mode == 4 or (note and mode in (2, 3)):
                step, elapsed = 0, period
            stopped = False
            if elapsed == period:
                difference = (limit - step) & 255
                if difference & 128:
                    if mode == 2:
                        stopped = True
                    else:
                        step = 0
                if not stopped:
                    elapsed = 0
                    step = (step + 1) & 255
            if not stopped:
                pan = {64: "right", 128: "left", 192: "both"}[rom[base + step - 1]]
                elapsed = (elapsed + 1) & 255
        output.append((step, elapsed, pan))
    return output


def synthetic_contract():
    source_slots = (0, 2, 1, 3)
    for total in range(256):
        for algorithm in range(8):
            raw = bytes([algorithm] + [0] * 20 + [total] * 4)
            patch = helper.read_fm_patch(raw, 0, 0)
            data = patch.to_score_data()
            require(all(op["total_level"] == total & 127 and op["volume_carry_bias"] == bool(total & 128) for op in data["operators"]), "TL driver bitfield loss")
            mask = (8, 8, 8, 8, 10, 14, 14, 15)[algorithm]
            for level in (0, 1, 127 - (total & 127), 128 - (total & 127), 127, 128, 255):
                for initial in (False, True):
                    expected = []
                    for source_slot, slot in enumerate(source_slots):
                        carrier = bool(mask & (1 << source_slot))
                        if initial:
                            expected.append((slot, (total + (level if carrier else 0)) & 127))
                        elif level < 128 and carrier and total + level <= 255:
                            expected.append((slot, (total + level) & 127))
                    actual = [(update.slot, update.total_level) for update in helper.fm_level_updates(patch, level, initial=initial)]
                    require(actual == expected, "TL signed/carry boundary differs", (total, algorithm, level, initial))
    rom = (ROOT / ".local/md-yuyuhakusho/downloads/yuyuhakusho-jp.bin").read_bytes()
    require([list(pattern.steps) for pattern in helper.read_score_pan_patterns(rom)] == [["right", "left", "right", "both"], ["right", "both", "left", "both"], ["both", "left", "both", "right"]], "shared pan window not fully represented")
    calls = [(tick % 7 == 0, tick % 13 == 12) for tick in range(600)]
    for values in ((3, 3, 3, 1, 10), (1, 2, 1, 3, 2), (1, 1, 3, 1, 2), (2, 1, 0, 3, 2)):
        definition = helper.decode_score_pan_automation(*values)
        state = helper.initial_pan_state(definition)
        actual = []
        for note, off in calls:
            result = helper.advance_pan_automation(definition, state, note, off)
            state = result.state
            actual.append((state.step, state.elapsed, result.pan))
        require(actual == raw_pan_calls(rom, values, calls), "actual pan argument boundary differs", values)
    return 2048


def validate(rom, paths, probe):
    source_slots = (0, 2, 1, 3)
    register_order = list(rom[0x39080:0x39098])
    require(len(register_order) == 24, "short register order")
    normalized_slot = [(register & 15) // 4 for register in register_order[:4]]
    require(normalized_slot == list(source_slots), "source register-slot order differs")
    carrier_masks = rom[0x39014:0x3901c]
    effective_masks = [63] + [127] * 4 + [223] * 4 + [159] * 4 + [31] * 4 + [255] * 4 + [127] * 4
    patches = {}
    references = collections.defaultdict(list)
    raw_proof = []
    counts = collections.Counter()
    coverage = {"pitch_envelope_ids": set(), "volume_envelope_ids": set(), "pan_modes": set(), "pan_patterns": set(), "fm3_domains": set()}
    reserved = collections.Counter()
    pan_definitions = set()
    pan_indices = collections.defaultdict(set)

    def patch_read(bank, instrument, address, raw, event=None):
        require(address == bank + 25 * instrument and len(raw) == 25, "patch pointer/index mismatch", event)
        require(rom[address:address + 25] == raw, "actual patch bytes differ from source", event)
        patch = helper.read_fm_patch(rom, bank, instrument)
        data = patch.to_score_data()
        for source_slot, slot in enumerate(source_slots):
            operator = data["operators"][slot]
            require(operator["total_level"] + (128 if operator["volume_carry_bias"] else 0) == raw[21 + source_slot], "TL input is not lossless", event)
        key = (bank, instrument)
        if key in patches:
            require(patches[key] == data, "same patch reference changed semantics", event)
        patches[key] = data
        if event:
            references[key].append({"code": event["code"], "row": event["row"], "step": event["step"], "tick": event["tick"]})
        for offset, (byte, mask) in enumerate(zip(raw, effective_masks)):
            extra = byte & (~mask & 255)
            if extra:
                reserved[str(offset)] += 1
        return patch

    if probe:
        for row in json.loads(probe.read_text()):
            patch_read(row["bank"], row["index"], row["pointer"], bytes.fromhex(row["bytes"]))
            counts["probe_patch_reads"] += 1

    for path in paths:
        trace = json.loads(gzip.decompress(path.read_bytes()))
        require(not trace["errors"], "native observer errors")
        events = trace["events"]
        active = {}
        for index, event in enumerate(events):
            kind = event["kind"]
            if "read_address" in event:
                raw = bytes.fromhex(event["read_hex"])
                require(rom[event["read_address"]:event["read_address"] + len(raw)] == raw, "read value differs from pinned source", event)
            if kind == "fm-patch-read":
                raw = bytes.fromhex(event["read_hex"])
                patch = patch_read(event["bank_address"], event["instrument_index"], event["read_address"], raw, event)
                state = bytes.fromhex(event["row_hex"])
                active[event["row"]] = (patch, raw)
                level, channel = state[9], state[1]
                port, channel_offset = (1 if channel & 4 else 0), channel & ~4
                requests = next_requests(events, index, "ym-request", 25)
                require(len(requests) == 25, "patch did not produce 25 direct YM requests", event)
                mask = carrier_masks[raw[0] & 7]
                values = list(raw)
                for source_slot in range(4):
                    if mask & (1 << source_slot):
                        values[21 + source_slot] = (values[21 + source_slot] + level) & 255
                expected = [(port, (register + channel_offset) & 255, value) for register, value in zip([176] + register_order, values)]
                actual = [(request["port"], request["register"], request["value"]) for request in requests]
                require(actual == expected, "actual full-byte patch request differs", event)
                # Effective fields are compared separately; extra source bits
                # are deliberately retained only in raw research evidence.
                typed_values = [patch.algorithm | patch.feedback << 3]
                groups = [lambda op: op.detune << 4 | op.multiple,
                          lambda op: op.rate_scale << 6 | op.attack_rate,
                          lambda op: (128 if op.am_enabled else 0) | op.decay_rate,
                          lambda op: op.sustain_rate,
                          lambda op: op.sustain_level << 4 | op.release_rate]
                for group in groups:
                    typed_values.extend(group(patch.operators[slot]) for slot in source_slots)
                typed_values.extend(update.total_level for update in helper.fm_level_updates(patch, level, initial=True))
                require(typed_values == [value & mask for value, mask in zip(values, effective_masks)], "typed effective patch output differs", event)
                raw_proof.append({"code": event["code"], "row": event["row"], "step": event["step"], "bank_address": event["bank_address"], "instrument_index": event["instrument_index"], "read_address": event["read_address"], "read_hex": event["read_hex"], "base_level": level, "requests": actual, "effective_values": typed_values})
                counts["native_patch_reads"] += 1
                counts["raw_patch_ym_requests"] += 25
                for base_level in range(256):
                    expected_levels = [(slot, (raw[21 + source_slot] + base_level) & 127) for source_slot, slot in enumerate(source_slots) if base_level < 128 and mask & (1 << source_slot) and raw[21 + source_slot] + base_level <= 255]
                    actual_levels = [(update.slot, update.total_level) for update in helper.fm_level_updates(patch, base_level)]
                    require(actual_levels == expected_levels, "actual patch level/carry arithmetic differs", event)
                counts["actual_patch_level_boundaries"] += 256
            elif kind in ("pitch-envelope-read", "volume-envelope-read"):
                state = bytes.fromhex(event["row_hex"])
                pitch = kind == "pitch-envelope-read"
                envelope_id = state[10] & 127 if pitch else state[11]
                table, count = (0x3a256, 8) if pitch else (0x3a31a, 11)
                require(1 <= envelope_id <= count, "unproved envelope id", event)
                address = int.from_bytes(rom[table + 4 * (envelope_id - 1):table + 4 * envelope_id], "big")
                require(event["base_address"] == address and event["read_address"] == address + event["index"], "envelope pointer/index mismatch", event)
                require(state[38 if pitch else 12] == (event["index"] + 1) & 255, "envelope increment timing differs", event)
                coverage["pitch_envelope_ids" if pitch else "volume_envelope_ids"].add(envelope_id)
                counts[kind] += 1
                sample = rom[event["read_address"]]
                if sample in (128, 129, 130, 131, 132):
                    counts[("pitch_special_" if pitch else "volume_special_") + str(sample)] += 1
                if not pitch and sample not in (128, 130):
                    write = next_volume_write(events, index)
                    expected_value = None
                    if sample == 131:
                        if not state[0] & 4:
                            expected_value = state[1] | 31
                    elif sample != 129:
                        suppressed = state[0] & 6 or (state[0] & 16 and state[19] and not state[18])
                        if not suppressed:
                            tick = helper.VolumeTick(sample, True)
                            expected_value = ((helper.psg_attenuation(tick, state[9]) | state[1]) + 16) & 255
                    require((write["value"] if write else None) == expected_value, "actual PSG level/hold/keyoff write differs", event)
                    counts["volume_ordered_write_outcomes"] += 1
                    counts["volume_actual_writes" if write else "volume_actual_no_write"] += 1
            elif kind == "control-before" and event["command"] == 228:
                values = bytes.fromhex(event["read_hex"])
                automation = helper.decode_score_pan_automation(*values[:5]) if values[0] else helper.decode_score_pan_automation(0)
                require(automation.mode, "pan mode missing", event)
                coverage["pan_modes"].add(values[0])
                counts["pan_control_definitions"] += 1
                values = tuple(values[:5]) if values[0] else (0, 0, 0, 0, 0)
                if values not in pan_definitions:
                    calls = [(tick % 7 == 0, tick % 13 == 12) for tick in range(600)]
                    state = helper.initial_pan_state(automation)
                    actual = []
                    for note, off in calls:
                        result = helper.advance_pan_automation(automation, state, note, off)
                        state = result.state
                        actual.append((state.step, state.elapsed, result.pan))
                    require(actual == raw_pan_calls(rom, values, calls), "actual pan definition projection differs", event)
                    pan_definitions.add(values)
                    counts["pan_actual_definition_projection_calls"] += len(calls)
            elif kind == "pan-pattern-read":
                state = bytes.fromhex(event["row_hex"])
                pattern_id = state[32]
                patterns = helper.read_score_pan_patterns(rom)
                require(1 <= pattern_id <= len(patterns), "unproved pan pattern id", event)
                pattern = patterns[pattern_id - 1]
                require(0 <= event["index"] < len(pattern.steps), "unproved pan pattern index", event)
                source_base = int.from_bytes(rom[0x37b1e + (pattern_id - 1) * 4:0x37b22 + (pattern_id - 1) * 4], "big")
                require(event["base_address"] == source_base and event["read_address"] == source_base + event["index"], "pan pointer/index mismatch", event)
                require(state[33] == event["index"] + 1, "pan step timing differs", event)
                expected_pan = {"right": 64, "left": 128, "both": 192}[pattern.steps[event["index"]]]
                require(bytes.fromhex(event["read_hex"]) == bytes([expected_pan]), "typed pan pattern differs", event)
                requests = next_requests(events, index, "ym-request", 1)
                if not state[0] & 4 and state[1] != 6:
                    channel = state[1]
                    expected = (1 if channel & 4 else 0, 180 + (channel & ~4), expected_pan | (state[39] & 55))
                    require(len(requests) == 1 and (requests[0]["port"], requests[0]["register"], requests[0]["value"]) == expected, "actual pan/LFO merge request differs", event)
                    counts["pan_ordered_write_outcomes"] += 1
                else:
                    counts["pan_DAC_or_claim_write_not_checked"] += 1
                coverage["pan_patterns"].add(pattern_id)
                pan_indices[pattern_id].add(event["index"])
                counts["pan_pattern_reads"] += 1
            elif kind == "fm3-offset-read":
                selection = (event["read_address"] - 0x39250) // 2
                result = helper.read_fm3_offsets(rom, (selection,) * 4, scope_of(event["row"]))
                require(result.frequency_offsets[0] == int.from_bytes(bytes.fromhex(event["read_hex"]), "big"), "typed FM3 offset differs", event)
                coverage["fm3_domains"].add(result.domain)
                counts["fm3_offset_reads"] += 1
            elif kind in ("e9-bank-read", "e9-argument-read"):
                counts[kind] += 1
            elif kind == "voice-init":
                counts["header_voices"] += 1
            elif kind == "tick-before":
                counts["driver_ticks"] += 1
        counts["events_checked"] += len(events)
        counts["definitions_checked"] += 1
    keys = sorted(patches)
    typed = {"patches": [{"id": "fm-patch-" + str(index + 1), **patches[key]} for index, key in enumerate(keys)]}
    provenance = {"references": [{"id": "fm-patch-" + str(index + 1), "bank_address": key[0], "instrument_index": key[1], "read_address": key[0] + 25 * key[1], "uses": references[key]} for index, key in enumerate(keys)], "raw_patch_request_proof": raw_proof, "hardware_ignored_bits_by_patch_offset": dict(reserved), "tl_high_bit_retained_as": "volume_carry_bias", "actual_pan_arguments": [list(values) for values in sorted(pan_definitions)], "actual_pan_indices": {str(key): sorted(values) for key, values in sorted(pan_indices.items())}}
    report = {"pass": True, "scope": "actual source parameter reads and ordered raw/effective patch requests; not a continuous score or DSP proof", "counts": dict(counts), "coverage": {key: sorted(values) for key, values in coverage.items()}, "unique_bank_index_patches": len(keys), "raw_request_full_byte_reconstruction_claimed_from_typed": False, "inputs": [bind(path) for path in paths] + ([bind(probe)] if probe else [])}
    report["not_reached_in_supplied_reads"] = [name for name, counter in (("pitch-envelope", "pitch-envelope-read"), ("E9", "e9-bank-read"), ("FM3", "fm3_offset_reads")) if not counts[counter]]
    report["not_reached_volume_envelope_ids"] = sorted(set(range(1, 12)) - coverage["volume_envelope_ids"])
    return typed, provenance, report


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--reads", nargs="*", type=Path, default=[])
    parser.add_argument("--index", type=Path)
    parser.add_argument("--probe", type=Path)
    parser.add_argument("--out", type=Path, required=True)
    args = parser.parse_args()
    out = args.out.resolve()
    require(out.is_relative_to(ROOT / ".local/md-yuyuhakusho/audio-score-parameters"), "output outside owned private directory")
    rom = (ROOT / ".local/md-yuyuhakusho/downloads/yuyuhakusho-jp.bin").read_bytes()
    require(sha(rom) == ROM_SHA, "ROM identity differs")
    cases = synthetic_contract()
    if args.index:
        require(not args.reads, "select index or explicit traces")
        index = json.loads(args.index.read_text())
        require(index["pass"] and index["scope"] == "native-score-read-oracle" and not index["production_ready"] and index["rom_sha256"] == ROM_SHA, "unaccepted native read index")
        expected_codes = [130, 131, 132, 133, 134, 135, 136, 138, 141, 149, 143] + list(range(160, 172))
        require(index["codes"] == expected_codes and [case["code"] for case in index["cases"]] == expected_codes, "full definition coverage differs")
        for case in index["cases"]:
            binding = case["files"]["events"]
            path = ROOT / binding["path"]
            require(bind(path)["sha256"] == binding["sha256"], "native read trace digest differs")
            args.reads.append(path)
    typed, provenance, report = validate(rom, args.reads, args.probe)
    if args.index:
        require(report["counts"]["header_voices"] == 115 and report["counts"]["definitions_checked"] == 23, "full voice/definition coverage differs")
        report["native_read_index"] = bind(args.index)
    report.update(rom_sha256=ROM_SHA, synthetic_patch_cases=cases, checker=bind(Path(__file__)), helper=bind(Path(helper.__file__)))
    out.mkdir(parents=True, exist_ok=True)
    for name, data in (("typed-patches.json", typed), ("native-read-provenance.json", provenance), ("native-read-result.json", report)):
        (out / name).write_text(json.dumps(data, indent=2) + "\n")
    print(json.dumps(report, indent=2))


if __name__ == "__main__":
    main()
