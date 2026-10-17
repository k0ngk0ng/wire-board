"""Record natural human controller calls with active movement/button sequences."""
import csv,gzip,hashlib,json,sys
from pathlib import Path
from capture_reference import BASE,ROOT,run
from extract_world_actions import decode_world_action_state
HOOK=r'''
local f=assert(io.open(out..'/human-input-ram.bin','wb'))
local c=assert(io.open(out..'/human-input.csv','w'));c:write('frame,actor\n')
local pending=nil
local function ram() local p={};for i=0,2047 do p[#p+1]=string.char(memory.readbyte(i)) end;return table.concat(p) end
memory.registerexec(0x805c,1,function()
 if memory.readbyte(0xbfff)~=6 then return end
 local i=memory.getregister('x');if i>=4 then return end
 assert(pending==nil);pending=i;f:write(ram());c:write(string.format('%d,%d\n',emu.framecount(),i))
end)
memory.registerexec(0x8085,1,function()
 if memory.readbyte(0xbfff)~=6 or pending==nil then return end
 f:write(ram());pending=nil
end)
function finish_probe() assert(pending==nil);f:close();c:close() end
'''
def signed(v):return v-256 if v>=128 else v
def state(r):
 def control(i):return dict(mode=r[0x672+i]&127,entered=bool(r[0x672+i]&128),plan=r[0x676+i],counter=signed(r[0x66e+i]),planParameter=r[0x682+i],trackedObject=signed(r[0x67a+i]),destination=r[0x67e+i])
 return dict(world=decode_world_action_state(r),team=dict(teams=[dict(mode=r[0x663+i]&127,entered=bool(r[0x663+i]&128),counter=signed(r[0x668+i])) for i in range(2)],players=[control(i) for i in range(4)]),memory=[dict(tapWindow=r[0x4f9+i],lastTap=r[0x509+i],attackTapWindow=r[0x4fd+i],lastAttackTap=r[0x505+i],buttonDelay=r[0x501+i],pendingButtons=r[0x50d+i]) for i in range(4)],held=list(r[0x4e0:0x4e4]),pressed=list(r[0x4e4:0x4e8]),human=[dict(request=r[0x653+i],status=signed(r[0x658+i]),buttonWindows=list(r[0x65b+i:0x65d+i]),timers=[signed(v) for v in r[0x65f+i:0x661+i]],messages=list(r[0x6c6+i:0x6c8+i]),overrides=list(r[0x68e+i:0x690+i])) for i in (0,2)],pickupFlags=list(r[0x634:0x638]))
def context(r):
 world=dict(courtID=r[0x58b],controls=list(r[0x539:0x53d]),characters=list(r[0x3e3:0x3e7]),teams=list(r[0x586:0x588]),runGrades=list(r[0x615:0x619]),jumpGrades=list(r[0x619:0x61d]),shotSkills=list(r[0x61d:0x621]),recoveryGrades=list(r[0x621:0x625]),equipment=r[0x6e4],otherObjectArcFlags=list(r[0x6e9:0x6f0]),enhancementEnabled=r[0x58d]<128,enhancements=[r[0x6e5+3*r[0x3e3+j]] for j in range(4)])
 frames=[]
 for i in range(4):
  slot=r[0x4e8+i];frames.append(dict(held=r[slot],pressed=r[slot+4]) if slot<4 else dict(held=0,pressed=0))
 return dict(world=world,frames=frames,difficulty=r[0x58c],scoreDifference=r[0x696],cachedZones=list(r[0x66a:0x66e]),objectKinds=list(r[0x3d6:0x3e3]),hoopLinks=list(r[0x649:0x64f]),humanTactics=list(r[0x666:0x668]),jumpEnhancements=[r[0x6e6+3*r[0x3e3+j]] for j in range(4)])
def main():
 records=[];sources=[]
 for name,slot,frames in [('free',5,1800),('carrier',7,1200),('air-carrier',7,360)]:
  segments=[]
  for base in range(0,frames,90):
   direction='right' if base//90%2==0 else 'left'
   for a,b,keys in [(1,12,[direction]),(16,40,[direction]),(22,24,['A']),(28,30,['B']),(35,37,['A','B']),(43,45,['A','B']),(49,51,['A']),(56,58,['B']),(65,66,['up']),(69,70,['up']),(75,76,['down']),(79,80,['down'])]:segments.append(dict(start=base+a,end=min(base+b,frames),keys=keys))
  if name=='air-carrier':
   segments=[dict(start=a,end=b,keys=k) for a,b,k in [(1,35,['right']),(5,7,['A','B']),(16,18,['A','B']),(25,40,['B']),(65,67,['A']),(110,112,['A','B']),(125,140,['B'])]]
  spec=dict(name='human-active-'+name,load_slot=slot,frames=frames,segments=segments,captures=[frames])
  path=BASE/(spec['name']+'.json');path.write_text(json.dumps(spec,indent=2)+'\n')
  if '--extract' not in sys.argv:run(path)
  reference=BASE/'runs'/spec['name'];spec.update(name='input-'+spec['name'],hooks_lua=HOOK);path=BASE/(spec['name']+'.json');path.write_text(json.dumps(spec,indent=2)+'\n')
  if '--extract' not in sys.argv:run(path)
  observed=BASE/'runs'/spec['name'];assert (observed/'ram.bin').read_bytes()==(reference/'ram.bin').read_bytes()
  raw=(observed/'human-input-ram.bin').read_bytes();rows=list(csv.DictReader((observed/'human-input.csv').open()));assert len(raw)==len(rows)*4096
  for i,row in enumerate(rows):
   before=raw[i*4096:i*4096+2048];after=raw[i*4096+2048:(i+1)*4096]
   records.append(dict(scenario=name,frame=int(row['frame']),actor=int(row['actor']),before=state(before),after=state(after),context=context(before),pickupBefore=list(before[0x634:0x638]),pickupAfter=list(after[0x634:0x638]),jumpEnhancements=[before[0x6e6+3*before[0x3e3+j]] for j in range(4)]))
  sources.append(dict(scenario=name,trace_sha256=hashlib.sha256(raw).hexdigest(),source=json.loads((observed/'source.json').read_text())))
 payload=(json.dumps(records,separators=(',',':'))+'\n').encode();packed=gzip.compress(payload,mtime=0);out=ROOT/'games/basketball/engine/testdata/world-human-input.json.gz';out.write_bytes(packed)
 out.with_name('world-human-input-source.json').write_text(json.dumps(dict(source_is_original_execution=True,records=len(records),entry='bank6:805c',exit='bank6:8085',all_records_retained=True,sha256=hashlib.sha256(packed).hexdigest(),sources=sources),indent=2)+'\n');print('human calls',len(records))
if __name__=='__main__':main()
