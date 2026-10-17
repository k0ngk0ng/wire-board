"""Initial team5/court1/all-computer configuration, then original untouched play.

No ball state or gameplay result is injected. These initial choices replace the
platform's preparation; this is not original menu-selection coverage.
"""
import hashlib,json
from capture_reference import ROOT,BASE,run
CONFIG=r'''
local teams=false;local court=false
memory.registerexec(0xacbb,1,function()
 if not teams and memory.readbyte(0xbfff)==3 then
  memory.writebyte(0x586,0);memory.writebyte(0x587,5)
  for i=0,3 do memory.writebyte(0x3e3+i,i%2) end
  for i=0,11 do memory.writebyte(0x6e4+i,255) end
  memory.writebyte(0x58d,0)
  teams=true;writefile('configured-teams.json',string.format('{"frame":%d,"teams":[0,5],"characters":[0,1,0,1],"equipment":"none","mode":0}',emu.framecount()))
 end
end)
memory.registerexec(0xd1b5,1,function()
 if not court and memory.readbyte(0x589)<128 then
  memory.writebyte(0x58b,1)
  for i=0,3 do memory.writebyte(0x4e8+i,255) end
  local seed={0x12,0x34,0x56,0x78};for i=0,3 do memory.writebyte(0x48+i,seed[i+1]) end
  court=true;writefile('configured-court.json',string.format('{"frame":%d,"court":1,"controllers":[-1,-1,-1,-1],"seed":305419896}',emu.framecount()))
 end
end)
'''
HOOK=r'''
local f=assert(io.open(out..'/ball-dispatches.jsonl','w'));local raw=assert(io.open(out..'/ball-dispatches.ram','wb'));local pending=nil
local function snap()local t={};for i=0,2047 do t[#t+1]=string.char(memory.readbyte(i)) end;raw:write(table.concat(t))end
memory.registerexec(0xe149,1,function()assert(not pending);pending=emu.framecount();snap()end)
memory.registerexec(0xe14c,1,function()if pending then snap();f:write(string.format('{"frame":%d}\n',pending));pending=nil end end)
function finish_probe()assert(not pending);f:close();raw:close()end
'''
def main():
 out=ROOT/'.local/basketball/rea/ball-rest-stall';base=json.loads((BASE/'opening-from-menu.json').read_text());base.pop('save_slot',None)
 base.update(frames=15000,captures=[1500,6000,8000,12000,15000])
 paths=[]
 for label,hook in [('reference',CONFIG),('observed',CONFIG+'\n'+HOOK)]:
  spec=dict(base,name='ball-rest-team5-'+label,hooks_lua=hook);path=out/(label+'-spec.json');path.write_text(json.dumps(spec,indent=2)+'\n');run(path);paths.append(BASE/'runs'/spec['name'])
 a,b=paths;assert (a/'ram.bin').read_bytes()==(b/'ram.bin').read_bytes()
 for n in base['captures']:assert (a/f'frame-{n:05d}.png').read_bytes()==(b/f'frame-{n:05d}.png').read_bytes()
 raw=(b/'ram.bin').read_bytes();stalls=[];rest=[];phases={}
 for i in range(len(raw)//2048):
  r=raw[i*2048:(i+1)*2048];phase=r[0x589]&127;phases[phase]=phases.get(phase,0)+1
  if r[0x53f]==6 and r[0x518]&127==0 and r[0x5fd]&128 and (r[0xbc] or r[0xc9]) and not any(r[x] for x in [0x47e,0x48b,0x498,0x4a5,0x4b2,0x4bf,0x4cc,0x4d9]):
   stalls.append(dict(frame=i+1,x=r[0x7b]+256*r[0x88],depth=r[0xa2],height=r[0xbc]+256*r[0xc9]))
 result=dict(scope='initial choices only, unchanged original gameplay; not menu choice or Go trajectory equality',frames=base['frames'],ramMatchesReference=True,screenshotsMatchReference=True,phases=phases,stalledFrames=len(stalls),firstStalls=stalls[:10],lastStalls=stalls[-10:],source=json.loads((b/'source.json').read_text()),ramSHA256=hashlib.sha256(raw).hexdigest(),observed=str(b.relative_to(ROOT)))
 (out/'original-configured-result.json').write_text(json.dumps(result,indent=2)+'\n');print(json.dumps(result))
if __name__=='__main__':main()
