"""Capture original court animation boundaries under isolated court selection.
Uses the configured graphics references; does not claim natural menu selection.
"""
import csv,gzip,hashlib,json
from capture_reference import BASE,ROOT,run
HOOK=r'''
local raw=assert(io.open(out..'/court-animation-ram.bin','wb'))
local events=assert(io.open(out..'/court-animation.jsonl','w'))
local pending=nil;local targets={}
local function ram() local t={} for i=0,2047 do t[#t+1]=string.char(memory.readbyte(i)) end return table.concat(t) end
local function finish_call()
 if not pending then return end
 raw:write(pending.before);raw:write(ram())
 events:write(string.format('{"frame":%d,"queueWrites":[%s],"palette":%s,"sounds":[%s]}\n',pending.frame,table.concat(pending.queues,','),tostring(pending.palette),table.concat(pending.sounds,',')))
 pending=nil
end
for _,address in ipairs({0x30b,0x32e,0x351}) do
 memory.registerwrite(address,1,function(a,size,value) if pending then pending.queues[#pending.queues+1]=tostring(a) end end)
end
memory.registerwrite(0x304,1,function()if pending then pending.palette=true end end)
memory.registerexec(0xc000,1,function()if pending then pending.sounds[#pending.sounds+1]=tostring(memory.getregister('a')) end end)
memory.registerexec(0x9b95,1,function()
 if memory.readbyte(0xbfff)~=4 then return end
 assert(not pending)
 local s=memory.getregister('s');local addr=memory.readbyte(0x100+(s+1)%256)+256*memory.readbyte(0x100+(s+2)%256)+1
 if not targets[addr] then targets[addr]=true;memory.registerexec(addr,1,finish_call) end
 pending={frame=emu.framecount(),before=ram(),queues={},sounds={},palette=false}
end)
function finish_probe() assert(not pending);raw:close();events:close() end
'''
def state(r,court):
 d={}
 if court==0:d['waterFrame']=r[0x6d7]
 if court in (2,5):d.update(alternatingSide=r[0x6d7],switchFlags=list(r[0x6d8:0x6da]),switchTicks=list(r[0x6da:0x6dc]))
 if court==3:d.update(gateFlags=r[0x6d7],gateTicks=r[0x6d8])
 if court==4:d.update(paletteFrame=r[0x6d7],paletteReverse=r[0x6d8]!=0)
 if court==6:d.update(reelFrames=list(r[0x6d7:0x6dd]),slotProgress=list(r[0x6dd:0x6df]),outcomeFlags=r[0x6df])
 return d

def patches(r,addresses):
 result=[]
 # Commands are submitted in fixed queue order, not the order header bytes
 # happen to be assigned by the producer (water assigns them in reverse).
 for base in [0x30b,0x32e,0x351]:
  if base not in addresses:continue
  address=r[base]*256+r[base+1];step=32 if r[{0x30b:0x307,0x32e:0x308,0x351:0x309}[base]]&4 else 1
  for i in range(r[base+2]):
   a=address+i*step;offset=a&1023
   assert 0x2000<=a<0x3000 and offset<960
   result.append(dict(plane=1 if a&0x800 else 0,row=offset//32,column=offset%32,tiles=[r[base+3+i]]))
 return result

def scores(r):return [r[0x166+i]+256*r[0x168+i] for i in range(2)]

def main():
 rows=[];sources=[]
 for court in range(8):
  reference=BASE/f'runs/court-{court}-reference'
  spec=json.loads((reference/'scenario.json').read_text());spec['name']=f'court-animation-{court}';spec['hooks_lua']+='\n'+HOOK
  path=BASE/(spec['name']+'.json');path.write_text(json.dumps(spec,indent=2)+'\n')
  dest=BASE/'runs'/spec['name']
  if not (dest/'capture-complete.json').exists() or json.loads((dest/'scenario.json').read_text())!=spec:run(path)
  assert (dest/'ram.bin').read_bytes()==(reference/'ram.bin').read_bytes()
  for image in reference.glob('frame-*.png'):assert image.read_bytes()==(dest/image.name).read_bytes()
  raw=(dest/'court-animation-ram.bin').read_bytes();events=[json.loads(v) for v in (dest/'court-animation.jsonl').read_text().splitlines()];assert len(raw)==len(events)*4096
  for i,e in enumerate(events):
   b=raw[i*4096:i*4096+2048];a=raw[i*4096+2048:(i+1)*4096]
   rows.append(dict(court=court,frame=e['frame'],before=state(b,court),context=dict(court=court,active=(b[0x589]&127) not in (0,2),videoFrame=b[0x300],cameraX=b[0x397],random=int.from_bytes(b[0x48:0x4c],'big'),scores=scores(b),hoopOwners=list(b[0x540:0x546]),hoopLocks=list(b[0x62d:0x633])),after=dict(state=state(a,court),patches=patches(a,e['queueWrites']),palette=e['palette'],paletteID=a[0x304] if e['palette'] else 0,sounds=e['sounds'],random=int.from_bytes(a[0x48:0x4c],'big'),scores=scores(a))))
  sources.append(dict(court=court,records=len(events),traceSHA256=hashlib.sha256(raw).hexdigest(),source=json.loads((dest/'source.json').read_text())))
 out=ROOT/'games/basketball/engine/testdata';p=out/'court-animation.json.gz';p.write_bytes(gzip.compress((json.dumps(rows,separators=(',',':'))+'\n').encode(),mtime=0))
 (out/'court-animation-source.json').write_text(json.dumps(dict(scope='original function execution after isolated court configuration, unmodified input/gameplay; no natural-menu claim',records=len(rows),sha256=hashlib.sha256(p.read_bytes()).hexdigest(),sources=sources),indent=2)+'\n')
 print('court animation calls',len(rows))
if __name__=='__main__':main()
