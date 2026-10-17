"""Observe original pass-launch calculations in normal controller scenarios."""
import hashlib
import json
import argparse
from capture_reference import ROOT, BASE, run

HOOK = r'''
local f=assert(io.open(out.."/pass.jsonl","w"));local pending=nil
local function word(a,b) return memory.readbyte(a)+256*memory.readbyte(b) end
memory.registerexec(0x8c23,1,function()
 if memory.readbyte(0xbfff)~=5 then return end
 assert(not pending)
 local i=memory.getregister("x")
 pending=string.format('{"frame":%d,"actor":%d,"holder":%d,"x":%d,"depth":%d,"height":%d,"targetX":%d,"targetDepth":%d,"targetHeight":%d,"arcHeight":%d,',emu.framecount(),i,memory.readbyte(0x53f),word(0x75+i,0x82+i),memory.readbyte(0x9c+i),word(0xb6+i,0xc3+i),word(0x69f,0x6a0),memory.readbyte(0x6a1),word(0x6a2,0x6a3),memory.readbyte(0x6a4))
end)
memory.registerexec(0x8cf1,1,function()
 if memory.readbyte(0xbfff)~=5 then return end
 assert(pending)
 local i=memory.getregister("x")
 f:write(pending..string.format('"vx":%d,"vy":%d,"vz":%d,"gravity":%d}\n',word(0x478+i,0x485+i),word(0x492+i,0x49f+i),word(0x4ac+i,0x4b9+i),word(0x4c6+i,0x4d3+i)))
 pending=nil
end)
function finish_probe()
 if pending then writefile("incomplete-pass.json",pending..'"incomplete":true}') end
 f:close()
end
'''


def main():
    parser=argparse.ArgumentParser()
    parser.add_argument('--shot',action='store_true',help='record B shots instead of A passes')
    args=parser.parse_args()
    kind='shot' if args.shot else 'pass'
    button='B' if args.shot else 'A'
    cases = {}
    directions = [['up'], ['down'], ['left'], ['right'], ['up','right'], ['down','right'], ['up','left'], ['down','left']]
    for keys in directions:
        for duration in (9,18):
            cases['-'.join(keys)+'-'+str(duration)] = [
                dict(start=1,end=duration,keys=keys),
                dict(start=duration+1,end=duration+9,keys=[button])]
    for start in (10,16,22,28,34):
        cases['jump-'+str(start)] = [dict(start=1,end=6,keys=['A','B']),dict(start=start,end=start+6,keys=[button])]
    records = []
    sources = []
    for name, segments in cases.items():
        spec=dict(name=kind+'-control-'+name,load_slot=7,frames=120,segments=segments,captures=[120])
        path=BASE/(spec['name']+'.json');path.write_text(json.dumps(spec,indent=2)+'\n');run(path)
        reference=BASE/'runs'/spec['name']
        spec.update(name=kind+'-'+name,hooks_lua=HOOK)
        path=BASE/(spec['name']+'.json');path.write_text(json.dumps(spec,indent=2)+'\n');run(path)
        observed=BASE/'runs'/spec['name']
        assert (observed/'ram.bin').read_bytes()==(reference/'ram.bin').read_bytes(), 'Probe changed behavior'
        records.extend(dict(scenario=name,**json.loads(line)) for line in (observed/'pass.jsonl').read_text().splitlines())
        sources.append(dict(scenario=name,**json.loads((observed/'source.json').read_text())))
    assert records, 'No pass launches were observed'
    fixture=ROOT/f'games/basketball/engine/testdata/{kind}-launch.json'
    fixture.write_text(json.dumps(records,separators=(',',':'))+'\n')
    manifest=dict(source_is_original_execution=True,entry='bank5:8c23',exit='bank5:8cf1',records=len(records),sha256=hashlib.sha256(fixture.read_bytes()).hexdigest(),sources=sources)
    fixture.with_name(kind+'-launch-source.json').write_text(json.dumps(manifest,indent=2)+'\n')
    print(json.dumps(dict(records=len(records),scenarios=len(cases),fixture=str(fixture))))


if __name__=='__main__':
    main()
