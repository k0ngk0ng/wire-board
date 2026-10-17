"""Controlled first-world origin matching the platform's cold semantics.

One initial alignment only, then unmodified original whole-machine gameplay.
Not a natural menu path. All injected fields and before/after RAM are retained.
"""
import json,hashlib
from capture_reference import ROOT,BASE,run
from capture_rest_stall_original import CONFIG
from extract_world_loop import loop_state

def main():
 out=ROOT/'.local/basketball/rea/ball-rest-stall';go=json.loads((out/'go-cold-first-playing.json').read_text());a=go['playing']['world']['Preparation']['Actions'];c=go['playing']['match']['Clock']
 patch={0x8f:a['actors'][0]['player']['body']['position']['depth']&255,0x69:a['actors'][1]['player']['body']['position']['x']&255,0x300:go['playing']['world']['LogicFrame'],0x6c2:c['Minutes'],0x6c3:c['Tens'],0x6c4:c['Ones'],0x6c5:c['VideoRemainder'],0x3cd:c['DisplayedSecond'],0x3ce:c['HUDTurn'],0x3cb:c['ScoreLowCache'][0],0x3cc:c['ScoreLowCache'][1]}
 for i,b in enumerate(a['Random'].to_bytes(4,'big')):patch[0x48+i]=b
 initial=';'.join(f'memory.writebyte({k},{v})' for k,v in sorted(patch.items()))
 hook=r'''
local aligned=false;local count=0
local f=assert(io.open(out..'/world-after.ram','wb'));local index=assert(io.open(out..'/world-after.jsonl','w'))
local function raw()local t={};for i=0,2047 do t[#t+1]=string.char(memory.readbyte(i)) end;return table.concat(t)end
memory.registerexec(0xd355,1,function()
 if not aligned and memory.readbyte(0x589)==129 then
  writefile('alignment-before.ram',raw());PATCH
  aligned=true;writefile('alignment-after.ram',raw());writefile('alignment-frame.json',string.format('{"frame":%d}',emu.framecount()))
 end
 if aligned then f:write(raw());index:write(string.format('{"frame":%d,"index":%d}\n',emu.framecount(),count));count=count+1 end
end)
function finish_probe()assert(aligned);f:close();index:close()end
'''.replace('PATCH',initial)
 (out/'aligned-patch.json').write_text(json.dumps(dict(scope='single controlled first complete world origin; not natural menu path',goSource='go-cold-first-playing.json',fields={hex(k):v for k,v in patch.items()}),indent=2)+'\n')
 spec=json.loads((BASE/'opening-from-menu.json').read_text());spec.pop('save_slot',None);spec.update(name='ball-rest-team5-aligned',frames=15000,captures=[1500,6000,8000,12000,15000],hooks_lua=CONFIG+'\n'+hook)
 path=out/'aligned-spec.json';path.write_text(json.dumps(spec,indent=2)+'\n');run(path)
 observed=BASE/'runs'/spec['name']
 reference_spec=dict(spec,name='ball-rest-team5-aligned-reference',hooks_lua=spec['hooks_lua'].replace("if aligned then f:write(raw());index:write(string.format('{\"frame\":%d,\"index\":%d}\\n',emu.framecount(),count));count=count+1 end",''))
 assert reference_spec['hooks_lua']!=spec['hooks_lua']
 reference_path=out/'aligned-reference-spec.json';reference_path.write_text(json.dumps(reference_spec,indent=2)+'\n');run(reference_path)
 reference=BASE/'runs'/reference_spec['name']
 assert (reference/'ram.bin').read_bytes()==(observed/'ram.bin').read_bytes()
 for n in spec['captures']:assert (reference/f'frame-{n:05d}.png').read_bytes()==(observed/f'frame-{n:05d}.png').read_bytes()
 raw=(observed/'world-after.ram').read_bytes();rows=[json.loads(x) for x in (observed/'world-after.jsonl').read_text().splitlines()];assert len(raw)==len(rows)*2048
 stalls=[]
 for i,row in enumerate(rows):
  r=raw[i*2048:(i+1)*2048]
  if r[0x589]&127==1 and r[0x53f]==6 and r[0x518]&127==0 and r[0x5fd]&128 and (r[0xbc] or r[0xc9]) and not any(r[x] for x in [0x47e,0x48b,0x498,0x4a5,0x4b2,0x4bf,0x4cc,0x4d9]):stalls.append(dict(**row,x=r[0x7b]+256*r[0x88],depth=r[0xa2],height=r[0xbc]+256*r[0xc9]))
 result=dict(referenceRAMAndScreenshotsMatch=True,scope='controlled first-world semantic alignment followed by unmodified original full machine',steps=len(rows),stalledSteps=len(stalls),firstStalls=stalls[:5],lastStalls=stalls[-5:],source=json.loads((observed/'source.json').read_text()),traceSHA256=hashlib.sha256(raw).hexdigest())
 (out/'aligned-result.json').write_text(json.dumps(result,indent=2)+'\n');print(json.dumps(result))
 # Retain typed original worlds to compare normal Go advancement from the same origin.
 decoded=[loop_state(raw[i*2048:(i+1)*2048],i+1) for i in range(min(50,len(rows)))];(out/'aligned-first-worlds.json').write_text(json.dumps(decoded,separators=(',',':'))+'\n')
if __name__=='__main__':main()
