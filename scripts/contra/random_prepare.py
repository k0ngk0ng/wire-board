"""Prepare source-bound random-work-byte functions for a private REA session."""
import hashlib
import json
from pathlib import Path
import zipfile

ROOT=Path(__file__).resolve().parents[2]
OUT=ROOT/'.local/contra-random'
ROM=Path('/Users/jason/Documents/nes/roms/03/魂斗罗日版(完全版).zip')
SHA='6ba53139fa88b8de1ae527c438bda6f1541d1ee7df26d63dec5164a32d166bfe'

def main():
    source=hashlib.sha256(ROM.read_bytes()).hexdigest()
    with zipfile.ZipFile(ROM)as z:r=z.read('Contra (J).nes')
    assert hashlib.sha256(r).hexdigest()==SHA
    p=r[16:131088];OUT.mkdir(parents=True,exist_ok=True)
    assert p[0x1fa5:0x1fab].hex()=='a534651a8534'
    assert p[0x1f880:0x1f889].hex()=='a51a653485344c80f8'
    data=p[7:0x4000]+p[0x1c000:]
    target=OUT/'random-bank0-fixed.bin';target.write_bytes(data)
    entries={0x9f58:'enemy_state_random_interval',0x9f9e:'random_interval_strength_add',0x9fa5:'random_mutating_add',0xc282:'start_game_reset',0xc307:'clear_low_ram',0xf880:'idle_random_loop'}
    seeds=OUT/'random-seeds.tsv';seeds.write_text(''.join(f'{hex(a)}\tfunction\t{label}\n'for a,label in entries.items()))
    spec={'target':str(target),'entry':'0x8007','seeds':str(seeds),'addresses':[hex(a)for a in entries],
          'file_sha256':hashlib.sha256(data).hexdigest(),'output':str(OUT/'rea-v2'),'tmp':str(ROOT/'dist/cr-tmp'),'cache':str(OUT/'swift-cache')}
    (OUT/'random-spec.json').write_text(json.dumps(spec,indent=2)+'\n')
    reads={0x05:'ORA zp',0x25:'AND zp',0x45:'EOR zp',0x65:'ADC zp',0xa5:'LDA zp',0xa4:'LDY zp',0xa6:'LDX zp',0xc5:'CMP zp',0xe5:'SBC zp',0x24:'BIT zp'}
    writes={0x85:'STA zp',0x86:'STX zp',0x84:'STY zp',0xe6:'INC zp',0xc6:'DEC zp',0x06:'ASL zp',0x46:'LSR zp',0x26:'ROL zp',0x66:'ROR zp'}
    candidates=[]
    for i in range(len(p)-1):
        if p[i+1]==0x34 and p[i]in reads|writes:
            candidates.append({'bank':i//8192,'bankOffset':i%8192,'romOffset':16+i,'instruction':(reads|writes)[p[i]],'writes':p[i]in writes,'context':p[max(0,i-4):i+8].hex()})
    manifest={'romSHA256':SHA,'archiveSHA256':source,'targetSHA256':spec['file_sha256'],'mapping':'PRG bank0/1 at 8000-BFFF and bank14/15 at C000-FFFF; starts 8007. Direct zero-page opcode scans below are candidates, including data false positives; not a complete proof against aliases.','candidates':candidates}
    assert hashlib.sha256(ROM.read_bytes()).hexdigest()==source
    (OUT/'random-static-source.json').write_text(json.dumps(manifest,indent=2)+'\n')
    print(OUT/'random-spec.json')

if __name__=='__main__':main()
