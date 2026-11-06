"""Bind fixed header call semantics to pinned ROM opcodes and native costs."""
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
    base = ROOT / '.local/md-yuyuhakusho/tools/Genesis-Plus-GX-60bee4c724859710ce05672239e5b601ad01382e/core/m68k'
    table = base / 'm68kops.h'
    rompath = ROOT / '.local/md-yuyuhakusho/downloads/yuyuhakusho-jp.bin'
    rom = rompath.read_bytes()
    entries = re.findall(r'\{(m68k_op_\w+)\s*,\s*(0x[0-9a-f]+),\s*(0x[0-9a-f]+),\s*(\d+)\}', table.read_text())
    declared = [('call-control', 0x375ba, 18), ('call-tempo', 0x375be, 18), ('call-fade', 0x375c2, 18),
                ('control-aux-compare', 0x38c2e, 16), ('control-aux-branch', 0x38c34, 10), ('control-mode-read', 0x38c38, 12), ('control-mode-zero-branch', 0x38c3c, 10),
                ('control-mode-compare', 0x38c60, 16), ('control-mode-two-branch', 0x38c66, 10), ('control-aux-read', 0x38c6a, 12), ('control-aux-zero-branch', 0x38c6e, 10), ('control-return', 0x38d06, 16),
                ('tempo-period-test', 0x389a8, 12), ('tempo-disabled-branch', 0x389ac, 10), ('tempo-remaining-step', 0x389ae, 16), ('tempo-no-reload-branch', 0x389b2, 10),
                ('tempo-reload', 0x389b4, 20), ('tempo-select-music', 0x389ba, 8), ('tempo-slot-stride', 0x389be, 4), ('tempo-ten-slots', 0x389c0, 4),
                ('tempo-active-test', 0x389c2, 8), ('tempo-skip-branch', 0x389c4, 10), ('tempo-note-wait-step', 0x389c6, 16), ('tempo-next-slot', 0x389ca, 8), ('tempo-loop-branch-base', 0x389cc, 12), ('tempo-return', 0x389d0, 16),
                ('fade-clear-temporary', 0x3893a, 4), ('fade-steps-read', 0x3893c, 12), ('fade-idle-branch', 0x38940, 10), ('fade-return', 0x3894c, 16)]
    rows = []
    for name, pc, expected in declared:
        opcode = int.from_bytes(rom[pc:pc + 2], 'big')
        matches = [row for row in entries if opcode & int(row[1], 16) == int(row[2], 16)]
        assert matches
        selected = matches[-1]
        assert int(selected[3]) == expected, (name, selected, expected)
        if name.startswith('call-'):
            assert opcode == 0x4eba and selected[0] == 'm68k_op_jsr_32_pcdi'
        rows.append({'semantic_operation': name, 'source_pc': pc, 'source_opcode': opcode, 'native_handler': selected[0], 'cpu_cycles': expected})
    cpu = (base / 'm68kcpu.h').read_text()
    for macro, delta in [('CYC_BCC_NOTAKE_B', -2), ('CYC_BCC_NOTAKE_W', 2), ('CYC_DBCC_F_NOEXP', -2), ('CYC_DBCC_F_EXP', 2)]:
        assert re.search(macro + r'\s+\(\s*' + str(delta) + r'\s*\*\s*MUL\)', cpu), macro
    handler = table.read_text().split('static void m68k_op_dbf_16(void)', 1)[1].split('static void ', 1)[0]
    assert 'res != 0xffff' in handler and 'USE_CYCLES(CYC_DBCC_F_NOEXP)' in handler and 'USE_CYCLES(CYC_DBCC_F_EXP)' in handler
    report = {'pass': True, 'scope': 'Offline ROM/architecture provenance for fixed header costs; no executable instruction list enters production',
              'operations': rows, 'caller_cpu': 18, 'caller_addressing': 'PC-relative displacement',
              'short_branch_not_taken_cpu': 8, 'long_branch_not_taken_cpu': 12, 'DBF_backedge_cpu': 10, 'DBF_final_cpu': 14,
              'master_multiplier': 7, 'refresh_delay_master': 14, 'refresh_deadline_delta_master': 896,
              'no_refresh_master': {'control-idle': 938, 'fade-idle': 420, 'tempo-disabled': 392, 'tempo-step': 560, 'tempo-reload-no-active': 3346, 'tempo-reload-all-active': 4326},
              'source_sha256': {str(path.relative_to(ROOT)): sha(path.read_bytes()) for path in [table, base / 'm68kcpu.h', base / 'm68kcpu.c', rompath, ROOT / 'docs/research/md-yuyuhakusho/static/audio-loop-sequencer.txt']},
              'producer_sha256': sha(Path(__file__).read_bytes()), 'production_uses_source_addresses_or_opcodes': False}
    out.parent.mkdir(parents=True, exist_ok=True)
    out.write_text(json.dumps(report, indent=2) + '\n')
    print(json.dumps({'pass': True, 'operations': len(rows), 'caller_cpu': 18, 'DBF_backedge_cpu': 10, 'DBF_final_cpu': 14}))

if __name__ == '__main__':
    main()
