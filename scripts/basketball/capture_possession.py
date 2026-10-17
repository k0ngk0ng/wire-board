"""Capture possession actions reached by normal input, without changing RAM.

This is an original-game oracle, not a claim of Go gameplay support. Each case
must have identical full RAM traces with and without the observational hooks.
"""
from pathlib import Path
import hashlib
import json

from capture_reference import BASE, ROOT, run


HOOK = r'''
local trace=assert(io.open(out.."/possession.jsonl","w"))
local pending={}
local function word(a,b) return memory.readbyte(a)+256*memory.readbyte(b) end
local function wide(a,b,c) return word(a,b)+65536*memory.readbyte(c) end
local function body(i)
 local fields={action=0x512,request=0x51f,pose=0x5b,facing=0x46b,clock=0x437,cursor=0x444,repeatCount=0x451,nextAction=0x52c,air=0x5f7,attachment=0x642,boost=0x576}
 local parts={}
 for name,a in pairs(fields) do parts[#parts+1]=string.format('"%s":%d',name,memory.readbyte(a+i)) end
 parts[#parts+1]=string.format('"x":%d,"depth":%d,"height":%d,"vx":%d,"vy":%d,"vz":%d,"gravity":%d',wide(0x68+i,0x75+i,0x82+i),word(0x8f+i,0x9c+i),wide(0xa9+i,0xb6+i,0xc3+i),word(0x478+i,0x485+i),word(0x492+i,0x49f+i),word(0x4ac+i,0x4b9+i),word(0x4c6+i,0x4d3+i))
 return '{'..table.concat(parts,',')..'}'
end
local function state()
 local pad=memory.readbyte(0x4e8)
 return string.format('{"holder":%d,"held":%d,"pressed":%d,"heading":%d,"player":%s,"teammate":%s,"ball":%s,"specialShot":%d,"inputMemory":{"tapTimer":%d,"lastTap":%d,"attackTapTimer":%d,"lastAttackTap":%d,"buttonDelay":%d,"pendingButtons":%d}}',memory.readbyte(0x53f),memory.readbyte(pad),memory.readbyte(pad+4),memory.readbyte(0x4ec),body(0),body(1),body(6),memory.readbyte(0x572),memory.readbyte(0x4f9),memory.readbyte(0x509),memory.readbyte(0x4fd),memory.readbyte(0x505),memory.readbyte(0x501),memory.readbyte(0x50d))
end
local function enter(kind)
 assert(not pending[kind],"nested "..kind)
 pending[kind]=string.format('{"kind":"%s","frame":%d,"before":%s,',kind,emu.framecount(),state())
end
local function leave(kind)
 assert(pending[kind],"unmatched "..kind)
 trace:write(pending[kind]..'"after":'..state()..'}\n');pending[kind]=nil
end
memory.registerexec(0x805c,1,function()
 if memory.readbyte(0xbfff)==6 and memory.getregister("x")==0 then enter("input") end
end)
memory.registerexec(0x8085,1,function()
 if memory.readbyte(0xbfff)==6 and memory.getregister("x")==0 then leave("input") end
end)
memory.registerexec(0xd345,1,function() enter("world") end)
memory.registerexec(0xd348,1,function() leave("world") end)
memory.registerexec(0xe149,1,function() enter("ball") end)
memory.registerexec(0xe14c,1,function() leave("ball") end)
memory.registerexec(0xe165,1,function()
 if memory.getregister("x")==0 then enter("player") end
end)
memory.registerexec(0xe168,1,function()
 if memory.getregister("x")==0 then leave("player") end
end)
function finish_probe()
 for kind,entry in pairs(pending) do writefile("incomplete-"..kind..".json",entry..'"incomplete":true}') end
 trace:close()
end
'''


def capture(spec):
    path = BASE / (spec['name'] + '.json')
    path.write_text(json.dumps(spec, indent=2) + '\n')
    run(path)
    return BASE / 'runs' / spec['name']


def main():
    scenarios = json.loads((ROOT / 'scripts/basketball/possession_scenarios.json').read_text())
    baseline = capture(scenarios['baseline'])
    ram = (baseline / 'ram.bin').read_bytes()
    assert ram[-2048 + 0x53f] == 0, 'Normal inputs did not give human player possession'
    baseline_end = ram[-2048:]
    report = []
    records = []
    for name, segments in scenarios['cases'].items():
        spec = dict(name='possession-control-' + name, load_slot=7, frames=120,
                    captures=[1, 30, 60, 120], segments=segments)
        reference = capture(spec)
        assert (reference / 'initial-ram.bin').read_bytes() == baseline_end
        observed = capture(dict(spec, name='possession-' + name, hooks_lua=HOOK))
        actual_ram = (observed / 'ram.bin').read_bytes()
        assert actual_ram == (reference / 'ram.bin').read_bytes(), 'Probe changed original behavior'
        rows = [json.loads(line) for line in (observed / 'possession.jsonl').read_text().splitlines()]
        records.extend(dict(scenario=name, **row) for row in rows)
        counts = {kind: sum(row['kind'] == kind for row in rows) for kind in ('input', 'world', 'ball', 'player')}
        assert all(counts.values()), 'Missing action phase'
        world = [row for row in rows if row['kind'] == 'world']
        report.append(dict(scenario=name, calls=counts,
                           player_actions=sorted({r['after']['player']['action'] & 127 for r in world}),
                           ball_actions=sorted({r['after']['ball']['action'] & 127 for r in world}),
                           holders=sorted({r['after']['holder'] for r in world}),
                           ram_sha256=hashlib.sha256(actual_ram).hexdigest(),
                           trace_sha256=hashlib.sha256((observed / 'possession.jsonl').read_bytes()).hexdigest(),
                           ram_matches_uninstrumented=True))
    output = ROOT / '.local/basketball/possession'
    output.mkdir(parents=True, exist_ok=True)
    result = dict(status='passed', baseline='normal controller inputs from slot 5; no RAM writes',
                  baseline_ram_sha256=hashlib.sha256(baseline_end).hexdigest(),
                  scope='original-game observations only; possession gameplay not implemented in Go',
                  cases=report)
    (output / 'verification.json').write_text(json.dumps(result, indent=2) + '\n')
    fixture = ROOT / 'games/basketball/engine/testdata/possession.json'
    fixture.write_text(json.dumps(records, separators=(',', ':')) + '\n')
    manifest = dict(rom_sha256=json.loads((baseline / 'source.json').read_text())['rom_sha256'],
                    sha256=hashlib.sha256(fixture.read_bytes()).hexdigest(), records=len(records),
                    source_is_original_execution=True, scenarios=list(scenarios['cases']),
                    source='scripts/basketball/capture_possession.py; human action E165/E168 and ball E149/E14C')
    fixture.with_name('possession-source.json').write_text(json.dumps(manifest, indent=2) + '\n')
    print(json.dumps(result))


if __name__ == '__main__':
    main()
