"""Capture attached-player calls in the existing natural jump-on-teammate scene."""
import csv
import hashlib
import json
import sys
from capture_reference import BASE, ROOT, run
from capture_player_attack import decode
from extract_world_actions import decode_world_action_state

HOOK = r'''
local f=assert(io.open(out..'/rider-actions-ram.bin','wb'))
local c=assert(io.open(out..'/rider-actions.csv','w'));c:write('frame,actor\n')
local pending=nil
local function ram()
 local parts={};for i=0,2047 do parts[#parts+1]=string.char(memory.readbyte(i)) end
 return table.concat(parts)
end
memory.registerexec(0xe165,1,function()
 local i=memory.getregister('x');if i>3 then return end
 local a=AND(memory.readbyte(0x512+i),127)
 if a<0x3d or a>0x46 then return end
 assert(not pending);pending=i;f:write(ram());c:write(string.format('%d,%d\n',emu.framecount(),i))
end)
memory.registerexec(0xe168,1,function()
 if pending==nil then return end
 f:write(ram());pending=nil
end)
function finish_probe() assert(pending==nil);f:close();c:close() end
'''


def main():
    reference = BASE / 'runs/rider-normal-reference'
    spec = json.loads((reference / 'scenario.json').read_text())
    receives = '--receives' in sys.argv
    name = 'rider-receive-natural' if receives else 'rider-actions-natural'
    hook = HOOK.replace('if a<0x3d or a>0x46 then return end', 'if a~=0x1e or AND(memory.readbyte(0x642+i),128)==0 then return end') if receives else HOOK
    spec.update(name=name, hooks_lua=hook)
    path = BASE / (name+'.json')
    path.write_text(json.dumps(spec, indent=2) + '\n')
    if '--extract' not in sys.argv:
        run(path)
    observed = BASE / 'runs' / spec['name']
    assert (observed / 'ram.bin').read_bytes() == (reference / 'ram.bin').read_bytes()
    for image in reference.glob('frame-*.png'):
        assert image.read_bytes() == (observed / image.name).read_bytes()
    rows = list(csv.DictReader((observed / 'rider-actions.csv').open()))
    raw = (observed / 'rider-actions-ram.bin').read_bytes()
    assert len(raw) == len(rows) * 4096 and rows
    result = []
    for j, row in enumerate(rows):
        actor = int(row['actor'])
        states = []
        worlds = []
        for offset in (0, 2048):
            r = raw[j*4096+offset:j*4096+offset+2048]
            s = decode(r, actor)
            s['objects'] = [dict(
                x=r[0x68+i]+256*r[0x75+i]+65536*r[0x82+i],
                depth=r[0x8f+i]+256*r[0x9c+i],
                height=r[0xa9+i]+256*r[0xb6+i]+65536*r[0xc3+i],
                vx=r[0x478+i]+256*r[0x485+i], vy=r[0x492+i]+256*r[0x49f+i],
                vz=r[0x4ac+i]+256*r[0x4b9+i], gravity=r[0x4c6+i]+256*r[0x4d3+i],
                pose=r[0x5b+i],
                action=r[0x512+i], kind=r[0x3d6+i], air=r[0x5f7+i],
                linkedActor=r[0x550+i],
            ) for i in range(6)]
            s['allLinks'] = list(r[0x642:0x64f])
            s['linkedActor'] = r[0x550+actor]
            states.append(s)
            worlds.append(decode_world_action_state(r))
        result.append(dict(frame=int(row['frame']), actor=actor, before=states[0], after=states[1], worldBefore=worlds[0], worldAfter=worlds[1]))
    fixture = ROOT / ('games/basketball/engine/testdata/'+name+'.json')
    fixture.write_text(json.dumps(result, separators=(',', ':')) + '\n')
    fixture.with_name(name+'-source.json').write_text(json.dumps(dict(
        source_is_original_execution=True, records=len(result),
        ram_and_screenshots_match_reference=True,
        sha256=hashlib.sha256(fixture.read_bytes()).hexdigest(),
        trace_sha256=hashlib.sha256(raw).hexdigest(),
        source=json.loads((observed / 'source.json').read_text()),
    ), indent=2) + '\n')
    print(json.dumps(dict(records=len(result), actions=sorted(set(r['before']['action'] & 127 for r in result)))))


if __name__ == '__main__':
    main()
