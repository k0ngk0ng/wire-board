"""Original palette selection, queue and NMI commit observations.
Only the existing isolated court choice at D1B5 is retained. All subsequent
inputs are ordinary controller inputs; instrumentation only reads state.
"""
import json,csv
from pathlib import Path
from capture_reference import ROOT,BASE,run
HOOK=r'''
local log=assert(io.open(out.."/palette.jsonl","w"));local queued_id=nil;local committing=false;local nmi=0
memory.registerexec(0xfff1,1,function()nmi=nmi+1 end)
local function colors(addr,reader,n)local p={};for i=0,n-1 do p[#p+1]=reader(addr+i)end;return '['..table.concat(p,',')..']'end
local function observe(kind,value)
 log:write(string.format('{"kind":"%s","frame":%d,"cycles":%d,"nmi":%d,"pc":%d,"court":%d,"scene":%d,"phase":%d,"selection":%d,"flags":%d,"delay":%d,"queueHigh":%d,"queueCount":%d,"queuedID":%s,"value":%d,"ppu":%s,"buffer":%s}\n',kind,emu.framecount(),debugger.getcyclescount(),nmi,memory.getregister('pc'),memory.readbyte(0x58b),memory.readbyte(0x588),memory.readbyte(0x589),memory.readbyte(0x304),memory.readbyte(0x301),memory.readbyte(0x302),memory.readbyte(0x374),memory.readbyte(0x376),queued_id and tostring(queued_id)or'null',value or 0,colors(0x3f00,ppu.readbyte,32),colors(0x377,memory.readbyte,32)))
end
memory.registerwrite(0x304,1,function(a,z,v)observe('selection',v)end)
memory.registerwrite(0x374,1,function(a,z,v)
 if v==0x3f then queued_id=memory.readbyte(0x304);observe('queue',v)end
end)
memory.registerexec(0x94e0,1,function()if memory.readbyte(0xbfff)==4 then observe('build')end end)
memory.registerexec(0xcd12,1,function()if memory.readbyte(0x374)==0x3f and memory.readbyte(0x375)==0 then committing=true;observe('commit-before')end end)
memory.registerexec(0xcd21,1,function()if committing then observe('commit-after');committing=false end end)
memory.registerexec(0xd231,1,function()observe('opening-base')end)
memory.registerexec(0xd420,1,function()observe('period-reset')end)
memory.registerexec(0xba40,1,function()if memory.readbyte(0xbfff)==4 and memory.readbyte(0x589)==6 then observe('result-entry')end end)
memory.registerexec(0xbb3a,1,function()if memory.readbyte(0xbfff)==4 then observe('result-restored')end end)
memory.registerwrite(0x589,1,function(a,z,v)observe('phase',v)end)
memory.registerexec(0xdbf8,1,function()observe('background',memory.readbyte(0x3a0))end)
function finish_probe()log:close()end
'''
def main():
 base=json.loads((BASE/'runs/court-4-reference/scenario.json').read_text());specs=[base]
 pause={**base,'name':'court-4-palette-pause-reference','frames':1800,'captures':[1448,1455,1475,1488,1750,1800],'segments':base['segments']+[{'start':1450,'end':1451,'keys':['start']},{'start':1480,'end':1481,'keys':['start']}]};specs.append(pause)
 complete={**base,'name':'court-4-palette-complete-reference','frames':25000,'captures':[12000,12400,24000,24500,25000],'segments':base['segments']+[{'start':12320,'end':12321,'keys':['start']},{'start':12330,'end':12331,'keys':['A']}]+[{'start':n,'end':n+1,'keys':['A']} for n in range(12360,12800,30)]};specs.append(complete)
 records=[]
 for reference_spec in specs:
  reference=BASE/'runs'/reference_spec['name'];path=BASE/(reference_spec['name']+'.json');path.write_text(json.dumps(reference_spec,indent=2)+'\n')
  if not (reference/'capture-complete.json').exists():run(path)
  spec={**reference_spec,'name':reference_spec['name'].replace('-reference','-palette-nmi-observed'),'hooks_lua':reference_spec['hooks_lua']+'\n'+HOOK};path=BASE/(spec['name']+'.json');path.write_text(json.dumps(spec,indent=2)+'\n');observed=BASE/'runs'/spec['name']
  if not (observed/'capture-complete.json').exists():run(path)
  assert (reference/'ram.bin').read_bytes()==(observed/'ram.bin').read_bytes()
  for n in spec['captures']:assert (reference/f'frame-{n:05d}.png').read_bytes()==(observed/f'frame-{n:05d}.png').read_bytes()
  records.append({'reference':reference_spec['name'],'observed':spec['name'],'source':json.loads((observed/'source.json').read_text()),'ram_and_png_equal':True})
 out=ROOT/'.local/basketball/palette-timing';out.mkdir(parents=True,exist_ok=True);(out/'capture.json').write_text(json.dumps({'scope':'original configured court4 palette selection/queue/video writes; no Go runtime modifications','records':records},indent=2)+'\n')
if __name__=='__main__':main()
