"""Offline provenance for fixed PSG store/leaf architecture costs."""
import argparse
import json
from pathlib import Path
import re
from reference import ROOT, sha

def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--out', type=Path, required=True)
    args = parser.parse_args()
    out = args.out.resolve()
    assert out.is_relative_to(ROOT)
    base = ROOT / '.local/md-yuyuhakusho/tools/Genesis-Plus-GX-60bee4c724859710ce05672239e5b601ad01382e/core'
    table = base / 'm68k/m68kops.h'
    rompath = ROOT / '.local/md-yuyuhakusho/downloads/yuyuhakusho-jp.bin'
    rom = rompath.read_bytes()
    entries = re.findall(r'\{(m68k_op_\w+)\s*,\s*(0x[0-9a-f]+),\s*(0x[0-9a-f]+),\s*(\d+)\}', table.read_text())
    declared = [('pitch-latch-store', 0x3a0c6, 16), ('pitch-data-store', 0x3a0cc, 16), ('pitch-return', 0x3a0d2, 16),
                ('volume-store', 0x3a140, 16), ('volume-return', 0x3a146, 16), ('keyoff-store', 0x3a186, 16), ('keyoff-return', 0x3a18c, 16),
                ('mute-select-port', 0x3a18e, 12), ('mute-tone0', 0x3a194, 12), ('mute-tone1', 0x3a198, 12),
                ('mute-tone2', 0x3a19c, 12), ('mute-noise', 0x3a1a0, 12), ('mute-return', 0x3a1a4, 16)]
    operations = []
    for name, pc, expected in declared:
        opcode = int.from_bytes(rom[pc:pc + 2], 'big')
        matches = [entry for entry in entries if opcode & int(entry[1], 16) == int(entry[2], 16)]
        assert matches, (name, hex(opcode))
        selected = matches[-1]
        assert int(selected[3]) == expected, (name, selected, expected)
        operations.append({'semantic_operation': name, 'source_pc': pc, 'source_opcode': opcode,
                           'native_handler': selected[0], 'cpu_cycles': expected})
    memory = (base / 'mem68k.c').read_text()
    psg_path = memory[memory.index('void vdp_write_byte('):memory.index('void vdp_write_word(')]
    assert 'psg_write(m68k.cycles, data);' in psg_path
    assert not re.search(r'm68k.cycles\s*\+=', psg_path)
    report = {'pass': True, 'scope': 'Offline source binding for named fixed PSG operations; never an executable runtime opcode/cycle program',
              'operations': operations, 'psg_store_extra_master': 0,
              'no_refresh_master': {'raw-store': 112, 'single': 224, 'pitch-pair': 336, 'mute-all': 532},
              'refresh_master': {'deadline_after_current_boundary': 896, 'delay': 14},
              'source_sha256': {str(path.relative_to(ROOT)): sha(path.read_bytes()) for path in [table, base / 'm68k/m68kcpu.c', base / 'mem68k.c', base / 'sound/psg.c', rompath, ROOT / 'docs/research/md-yuyuhakusho/static/audio-loop-sequencer.txt']},
              'producer_sha256': sha(Path(__file__).read_bytes()), 'production_uses_source_pc_or_opcodes': False}
    out.parent.mkdir(parents=True, exist_ok=True)
    out.write_text(json.dumps(report, indent=2) + '\n')
    print(json.dumps({'pass': True, 'operations': len(operations), 'psg_store_extra_master': 0}))

if __name__ == '__main__':
    main()
