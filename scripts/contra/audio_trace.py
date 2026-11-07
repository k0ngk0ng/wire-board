"""Observe Japanese sound requests/slots without changing gameplay.

Stage entries retain their explicit single stage-index override from the parent
initializer evidence. The machine-gun scene uses ordinary controls only. Hooks
read sound requests and RAM; no request, music state or APU output is patched.
"""
import gzip
import argparse
import hashlib
import json
import os
from pathlib import Path
import shutil

ROOT=Path(__file__).resolve().parents[2]
os.environ.setdefault('CONTRA_REFERENCE_ROOT',str(ROOT/'.local/contra-audio/oracle'))
from capture_reference import BASE,run

HOOK=r'''
local old_finish=finish_probe
local requests=assert(io.open(out..'/sound-requests.jsonl','w'))
local slots=assert(io.open(out..'/sound-slots.bin','wb'))
local function caller()
 local s=memory.getregister('s');return memory.readbyte(0x100+AND(s+1,255))+256*memory.readbyte(0x100+AND(s+2,255))+1
end
memory.registerexec(0xf9bc,1,function()
 requests:write(string.format('{"frame":%d,"id":%d,"caller":%d,"bank":%d,"stage":%d,"gamePhase":%d,"levelPhase":%d,"actor":%d}\n',emu.framecount(),memory.getregister('a'),caller(),memory.readbyte(0x8000),memory.readbyte(0x30),memory.readbyte(0x18),memory.readbyte(0x2c),memory.getregister('x')))
end)
emu.registerafter(function()slots:write(memory.readbyterange(0x106,6))end)
function finish_probe()if old_finish then old_finish() end;requests:close();slots:close()end
'''


def main():
    parser=argparse.ArgumentParser();parser.add_argument('--label',default='v1');args=parser.parse_args()
    BASE.mkdir(parents=True,exist_ok=True)
    results=[]
    def capture(spec):
        path=BASE/(spec['name']+'.json');path.write_text(json.dumps(spec,indent=2)+'\n')
        out=BASE/'runs'/spec['name']
        if out.exists():
            assert json.loads((out/'scenario.json').read_text())==spec
            assert json.loads((out/'capture-complete.json').read_text())=={'status':'complete'}
            return out
        return run(path)
    for stage in range(1,9):
        original=ROOT/f'.local/contra-campaign/runs/stage-entry-v2-{stage}'
        spec=json.loads((original/'scenario.json').read_text())
        spec['name']=f'audio-stage-{stage}-{args.label}';spec['captures']=[];spec.pop('save_states',None)
        spec['hooks_lua']+='\n'+HOOK
        out=capture(spec)
        assert (out/'ram.bin').read_bytes()==(original/'ram.bin').read_bytes()
        requests=[json.loads(line) for line in (out/'sound-requests.jsonl').read_text().splitlines()]
        bgm=[r for r in requests if r['gamePhase']==5 and r['levelPhase']==2]
        assert len(bgm)==1,(stage,bgm)
        results.append({'kind':'stage-initialization','stage':stage,'bgm':bgm[0],
                        'requests':requests,'source':json.loads((out/'source.json').read_text()),
                        'intervention':'one original stage-index selection at CC5F; audio hooks read only',
                        'sameRAMAsInitializerOracle':True,'slotsSHA256':hashlib.sha256((out/'sound-slots.bin').read_bytes()).hexdigest()})
    weapon=json.loads(gzip.decompress((ROOT/'docs/research/nes-contra/evidence/weapons-machine.json.gz').read_bytes()))
    spec=dict(weapon['scenario']);spec['name']=f'audio-machine-{args.label}';spec['hooks_lua']=HOOK;spec['captures']=[]
    (BASE/'states').mkdir(parents=True,exist_ok=True)
    shutil.copyfile(ROOT/'.local/contra/states/coop-play.fcs',BASE/'states/coop-play.fcs')
    out=capture(spec)
    assert hashlib.sha256((out/'ram.bin').read_bytes()).hexdigest()==weapon['pickup']['ram_sha256']
    requests=[json.loads(line) for line in (out/'sound-requests.jsonl').read_text().splitlines()]
    results.append({'kind':'natural-machine-gun-pickup','requests':requests,'source':json.loads((out/'source.json').read_text()),'intervention':'none; controller inputs only','sameRAMAsNaturalPickupOracle':True,
                    'slotsSHA256':hashlib.sha256((out/'sound-slots.bin').read_bytes()).hexdigest()})
    pause={'name':f'audio-pause-{args.label}','load_state':'coop-play','frames':150,'captures':[],
           'segments':[{'start':30,'end':32,'keys':['start']},{'start':90,'end':92,'keys':['start']}],'hooks_lua':HOOK}
    out=capture(pause)
    requests=[json.loads(line) for line in (out/'sound-requests.jsonl').read_text().splitlines()]
    ram=(out/'ram.bin').read_bytes();paused=[n+1 for n in range(150) if ram[n*2048+0x25]]
    assert paused==list(range(30,90)),paused
    results.append({'kind':'natural-pause-resume','requests':requests,'source':json.loads((out/'source.json').read_text()),'intervention':'none; Start presses only','pausedFrames':paused,
                    'slotsSHA256':hashlib.sha256((out/'sound-slots.bin').read_bytes()).hexdigest()})
    target=ROOT/'docs/research/nes-contra/evidence/audio-requests.json'
    target.write_text(json.dumps({'source_is_original_execution':True,'romSHA256':results[0]['source']['rom_sha256'],'runs':results},indent=2)+'\n')
    print(json.dumps({'stageBGM':[r['bgm']['id'] for r in results[:8]],'naturalIDs':sorted({event['id'] for r in results[8:] for event in r['requests']})}))


if __name__=='__main__':main()
