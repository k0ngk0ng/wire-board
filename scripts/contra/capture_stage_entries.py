"""Observe all Japanese original stage initializers with an explicit intervention.

Failure cases: changing the stage only after CHR/enemy selection; repeated
overrides; wrong bank at header read; menu/cutscene mistaken for gameplay; and
presenting controlled stage entry as a naturally completed campaign.

The single stage-index write occurs at $CC5F before its enemy/CHR/header setup.
No movement, enemy, lives, score or collision results are patched afterward.
"""
import hashlib
import json
import os
from pathlib import Path
import sys
import zipfile

ROOT=Path(__file__).resolve().parents[2]
os.environ.setdefault('CONTRA_REFERENCE_ROOT',str(ROOT/'.local/contra-campaign'))
from capture_reference import BASE,run

HOOK=r'''
local injected=false;local header=false;local initialized=false
local target=TARGET
memory.registerexec(0xcc5f,1,function()
 if injected or memory.readbyte(0x18)~=5 or memory.readbyte(0x2c)~=0 then return end
 local before=memory.readbyte(0x30)
 memory.writebyte(0x30,target);injected=true
 writefile('stage-intervention.json',string.format('{"frame":%d,"pc":"0xcc5f","address":"0x30","before":%d,"after":%d,"reason":"controlled original stage initializer, not a natural campaign clear"}',emu.framecount(),before,target))
end)
memory.registerexec(0xa816,1,function()
 if memory.readbyte(0x8000)~=0x14 or header then return end
 assert(injected and memory.readbyte(0x30)==target,'stage override did not precede header')
 header=true
 writefile('loaded-header.bin',bytes(0xa87b+target*32,32,memory.readbyte))
end)
memory.registerexec(0xa84a,1,function()
 if memory.readbyte(0x8000)~=0x14 or not header or initialized then return end
 writefile('initialized-header.bin',bytes(0x40,32,memory.readbyte));initialized=true
end)
function finish_probe()assert(injected and header and initialized,'missing controlled stage initialization')end
'''


def main():
    label=sys.argv[1] if len(sys.argv)>1 else 'v1'
    results=[]
    source_path=Path('/Users/jason/Documents/nes/roms/03/魂斗罗日版(完全版).zip')
    source_hash=hashlib.sha256(source_path.read_bytes()).hexdigest()
    with zipfile.ZipFile(source_path) as z:rom=z.read('Contra (J).nes')
    assert hashlib.sha256(rom).hexdigest()=='6ba53139fa88b8de1ae527c438bda6f1541d1ee7df26d63dec5164a32d166bfe'
    for stage in range(8):
        name=f'stage-entry-{label}-{stage+1}'
        spec={'name':name,'frames':1420,'captures':[840,1200,1400],
              'segments':[{'start':100,'end':102,'keys':['select']}] +
                         [{'start':f,'end':f+2,'keys':['start']} for f in [120,720,1080]],
              'save_states':{'1400':f'stage-{stage+1}-{label}'},'hooks_lua':HOOK.replace('TARGET',str(stage))}
        BASE.mkdir(parents=True,exist_ok=True)
        path=BASE/(name+'.json');path.write_text(json.dumps(spec,indent=2)+'\n')
        observed=run(path)
        header=(observed/'loaded-header.bin').read_bytes()
        assert header==rom[16+0xa87b+stage*32:16+0xa89b+stage*32]
        assert (observed/'initialized-header.bin').read_bytes()==header
        ram=(observed/'ram.bin').read_bytes()[1399*2048:1400*2048]
        assert ram[0x18]==5 and ram[0x2c]==4 and ram[0x30]==stage,(stage,ram[0x18],ram[0x2c],ram[0x30])
        result={'stage':stage+1,'run':name,'source':json.loads((observed/'source.json').read_text()),
                'intervention':json.loads((observed/'stage-intervention.json').read_text()),
                'header':header.hex(),'mapPointer':hex(int.from_bytes(header[2:4],'little')),
                'runtimeHeader':ram[0x40:0x60].hex(),
                'headerChangesAfterInitialization':[{'address':hex(0x40+i),'initial':a,'runtime':b} for i,(a,b) in enumerate(zip(header,ram[0x40:0x60])) if a!=b],
                'supertilePointer':hex(int.from_bytes(header[4:6],'little')),
                'attributePointer':hex(int.from_bytes(header[6:8],'little')),
                'sceneType':ram[0x40],'scrollType':ram[0x41],
                'players':{'state':list(ram[0x90:0x92]),'x':list(ram[0x334:0x336]),'y':list(ram[0x31a:0x31c])},
                'snapshotSHA256':hashlib.sha256((BASE/'states'/f'stage-{stage+1}-{label}.fcs').read_bytes()).hexdigest(),
                'ramFrame1400SHA256':hashlib.sha256(ram).hexdigest()}
        results.append(result)
    assert hashlib.sha256(source_path.read_bytes()).hexdigest()==source_hash
    out=ROOT/'docs/research/nes-contra/evidence/campaign-controlled-entries.json'
    out.write_text(json.dumps({'passed':True,'scope':'Original stage-initializer execution with one declared index override; no natural campaign completion claim.',
        'sourceArchiveUnchanged':True,'entries':results},indent=2)+'\n')
    print(json.dumps({'passed':True,'stages':len(results),'evidence':str(out.relative_to(ROOT))}))


if __name__=='__main__':main()
