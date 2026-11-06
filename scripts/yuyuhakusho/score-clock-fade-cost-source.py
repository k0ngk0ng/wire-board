"""Offline source binding for fixed fade-start and header-prefix timing."""
import argparse
import json
from pathlib import Path
import re
from reference import ROOT, sha

def main():
    parser=argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--out',type=Path,required=True)
    args=parser.parse_args();out=args.out.resolve();assert out.is_relative_to(ROOT)
    base=ROOT/'.local/md-yuyuhakusho/tools/Genesis-Plus-GX-60bee4c724859710ce05672239e5b601ad01382e/core/m68k'
    table=base/'m68kops.h';rompath=ROOT/'.local/md-yuyuhakusho/downloads/yuyuhakusho-jp.bin';rom=rompath.read_bytes()
    entries=re.findall(r'\{(m68k_op_\w+)\s*,\s*(0x[0-9a-f]+),\s*(0x[0-9a-f]+),\s*(\d+)\}',table.read_text())
    declared=[('start01-remaining',0x38916,16),('start01-steps',0x3891c,16),('start01-DAC-flags',0x38922,16),('start01-return',0x38926,16),
               ('start05-remaining',0x38928,16),('start05-steps',0x3892e,16),('start05-DAC-flags',0x38934,16),('start05-return',0x38938,16),
               ('header-caller',0x375c2,18),('header-clear-temporary',0x3893a,4),('header-read-steps',0x3893c,12),('header-idle-branch',0x38940,10),('header-read-remaining',0x38942,12),('header-step-branch',0x38946,10),('header-remaining-store',0x38948,16),('header-done-return',0x3894c,16),('header-steps-store',0x3894e,16),('header-clear-branch',0x38952,10),('header-remaining-reload',0x38956,16),('header-select-FM',0x3895c,8),('header-six-FM',0x38960,4)]
    rows=[]
    for name,pc,expected in declared:
        opcode=int.from_bytes(rom[pc:pc+2],'big')
        found=[entry for entry in entries if opcode&int(entry[1],16)==int(entry[2],16)]
        assert found and int(found[-1][3])==expected,(name,found[-1] if found else None,expected)
        rows.append({'operation':name,'source_pc':pc,'source_opcode':opcode,'native_handler':found[-1][0],'cpu_cycles':expected})
    cpu=(base/'m68kcpu.h').read_text()
    assert re.search(r'CYC_BCC_NOTAKE_B\s+\(\s*-2\s*\*\s*MUL\)',cpu)
    assert re.search(r'CYC_BCC_NOTAKE_W\s+\(\s*2\s*\*\s*MUL\)',cpu)
    assert rom[0x375c2:0x375c4]==bytes.fromhex('4eba')
    report={'pass':True,'scope':'Fixed source architecture binding only; no runtime program/PC input','operations':rows,
            'short_branch_not_taken_cpu':8,'long_branch_not_taken_cpu':12,'master_multiplier':7,'refresh_delay_master':14,'refresh_deadline_delta_master':896,
            'no_refresh_master':{'start':448,'header-idle':420,'header-timer':658,'header-LevelScan':840,'header-FullClear':630},
            'no_refresh_effect_offsets':{'start':[0,112,224],'header-timer':[434],'header-LevelScan':[448,644],'header-FullClear':[448]},
            'later_dynamic_body_or_full04_charged':False,
            'source_sha256':{str(path.relative_to(ROOT)):sha(path.read_bytes()) for path in [table,base/'m68kcpu.h',base/'m68kcpu.c',rompath,ROOT/'docs/research/md-yuyuhakusho/static/audio-loop-sequencer.txt',ROOT/'docs/research/md-yuyuhakusho/static/audio-control.txt']},'producer_sha256':sha(Path(__file__).read_bytes())}
    out.parent.mkdir(parents=True,exist_ok=True);out.write_text(json.dumps(report,indent=2)+'\n')
    print(json.dumps({'pass':True,'operations':len(rows),'start_master':448,'CLR_absolute_word_cpu':16}))

if __name__=='__main__':main()
