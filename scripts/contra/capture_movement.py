"""Capture original controller sampling and vertical motion before Go implementation.

The oracle records natural execution of fixed-bank routines. No RAM patches,
cheats, generated expected physics or independently invented jump constants.
"""
import gzip
import hashlib
import json
from pathlib import Path
import sys

from capture_reference import ROOT, BASE, run

HOOKS = r'''
local f=assert(io.open(out.."/movement.jsonl","w"))
local pending={}
local function pair(a) return string.format('[%d,%d]',memory.readbyte(a),memory.readbyte(a+1)) end
local function state(i)
 local pos=memory.readbyte(0x94+i)+256*memory.readbyte(0x31a+i)+65536*memory.readbyte(0xba+i)
 if pos>=0x800000 then pos=pos-0x1000000 end
 local vel=memory.readbyte(0xc4+i)+256*memory.readbyte(0xc6+i)
 if vel>=0x8000 then vel=vel-0x10000 end
 return string.format('{"position":%d,"velocity":%d,"jump":%d,"animationTimer":%d,"animationFrame":%d,"aim":%d}',pos,vel,memory.readbyte(0xa0+i),memory.readbyte(0x9e+i),memory.readbyte(0xa6+i),memory.readbyte(0xc2+i))
end
local function routine(name,entry,exit)
 memory.registerexec(entry,1,function()
  local i=memory.getregister("x");assert(i<2);assert(not pending[name])
  pending[name]={actor=i,before=state(i),stageType=memory.readbyte(0x40)}
 end)
 memory.registerexec(exit,1,function()
  local p=pending[name];if not p then return end
  assert(memory.getregister("x")==p.actor)
  f:write(string.format('{"kind":"%s","frame":%d,"actor":%d,"stageType":%d,"before":%s,"after":%s}\n',name,emu.framecount(),p.actor,p.stageType,p.before,state(p.actor)))
  pending[name]=nil
 end)
end
routine('beginJump',0xd5fd,0xd625)
routine('gravity',0xd922,0xd92f)
routine('advanceY',0xd901,0xd921)
local sample=nil
memory.registerexec(0xc1bd,1,function() sample={previous=pair(0xf9)} end)
memory.registerexec(0xc1c0,1,function() sample.first=pair(4) end)
memory.registerexec(0xc1cb,1,function() sample.second=pair(4) end)
memory.registerexec(0xc1f8,1,function()
 assert(sample and sample.first and sample.second)
 f:write(string.format('{"kind":"input","frame":%d,"flags":%d,"previous":%s,"first":%s,"second":%s,"held":%s,"pressed":%s}\n',emu.framecount(),memory.readbyte(0x1d),sample.previous,sample.first,sample.second,pair(0xf1),pair(0xf5)))
 sample=nil
end)
function finish_probe()
 assert(next(pending)==nil);assert(not sample);f:close()
end
'''


def main():
    label = sys.argv[1] if len(sys.argv) > 1 else 'v1'
    scenarios = [
        ('solo-jump', 'solo', [(1, 1, 1, ['A']), (100, 100, 1, ['A'])]),
        ('solo-jump-right', 'solo', [(1, 85, 1, ['right']), (4, 30, 1, ['A'])]),
        ('solo-jump-left', 'solo', [(1, 90, 1, ['left']), (3, 3, 1, ['A'])]),
        ('solo-down-jump', 'solo', [(1, 8, 1, ['down', 'A']), (60, 90, 1, ['right', 'A'])]),
        ('solo-port2', 'solo', [(1, 20, 2, ['right']), (3, 3, 2, ['A']), (100, 101, 1, ['B'])]),
        ('coop-opposite', 'coop', [(1, 30, 1, ['right', 'B']), (1, 30, 2, ['left', 'B']),
                                  (3, 3, 1, ['A']), (20, 20, 2, ['A'])]),
        ('coop-pad2-jump', 'coop', [(2, 10, 2, ['A', 'right']), (60, 70, 1, ['A', 'right'])]),
        ('coop-held-jump', 'coop', [(1, 120, 1, ['A']), (1, 120, 2, ['B']),
                                  (130, 130, 1, ['A']), (130, 130, 2, ['A'])]),
    ]
    records, sources, counts = [], [], {}
    for name, mode, timeline in scenarios:
        spec = {'name': f'movement-{label}-{name}', 'load_state': mode+'-play', 'frames': 180,
                'segments': [{'start': a, 'end': b, 'player': p, 'keys': keys} for a,b,p,keys in timeline],
                'captures': [1, 20, 40, 80, 180], 'hooks_lua': HOOKS}
        p = BASE/'scenarios'/(spec['name']+'.json')
        p.parent.mkdir(parents=True, exist_ok=True)
        p.write_text(json.dumps(spec, indent=2)+'\n')
        result = run(p)
        for line in (result/'movement.jsonl').read_text().splitlines():
            record = json.loads(line)
            record['scenario'] = name
            counts[record['kind']] = counts.get(record['kind'], 0) + 1
            records.append(record)
        sources.append({'scenario': name, **json.loads((result/'source.json').read_text()),
                        'trace_sha256': hashlib.sha256((result/'movement.jsonl').read_bytes()).hexdigest(),
                        'initial_snapshot': mode+'-play', 'timeline': spec['segments']})
    assert counts['beginJump'] >= 8 and counts['gravity'] >= 100 and counts['advanceY'] >= 100
    assert all(r['first'] == r['second'] for r in records if r['kind'] == 'input')
    data = json.dumps(records, separators=(',', ':')).encode()
    out = ROOT/'games/contra/engine/testdata'
    out.mkdir(parents=True, exist_ok=True)
    fixture = out/'original-movement.json.gz'
    compressed = gzip.compress(data, mtime=0)
    if fixture.exists():
        compressed = fixture.read_bytes()
        assert gzip.decompress(compressed) == data, 'new capture differs from retained original oracle'
    else:
        fixture.write_bytes(compressed)
    source = {'rom_sha256': sources[0]['rom_sha256'], 'source_is_original_execution': True,
              'interventions': 'none; controller input only', 'counts': counts,
              'fixture_sha256': hashlib.sha256(compressed).hexdigest(), 'sources': sources,
              'routines': {'beginJump': ['0xd5fd', '0xd625'], 'gravity': ['0xd922', '0xd92f'],
                           'advanceY': ['0xd901', '0xd921'], 'input': ['0xc1bd', '0xc1f8']},
              'limitations': ['Stage 1 outdoor natural execution only; indoor/boundary arithmetic needs separate coverage.',
                              'Routine differential oracle does not prove complete game behavior.']}
    source_path = out/'original-movement-source.json'
    if not source_path.exists():
        source_path.write_text(json.dumps(source, indent=2)+'\n')
    # Rechecks retain their own invocation provenance without rewriting the
    # original fixture's source record (scenario labels may differ).
    (BASE/'movement-source.json').write_text(json.dumps(source, indent=2)+'\n')
    print(json.dumps({'fixture': str(fixture.relative_to(ROOT)), 'counts': counts}))


if __name__ == '__main__':
    main()
