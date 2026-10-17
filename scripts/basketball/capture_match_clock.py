"""Observe original match clock, HUD scheduling, pause and period-end checks."""
import gzip,hashlib,json
from capture_reference import ROOT,BASE,run
HOOK=r'''
local f=assert(io.open(out..'/match-clock.jsonl','w'));local pending={}
local function arr(a,n) local p={};for i=0,n-1 do p[#p+1]=memory.readbyte(a+i) end;return '['..table.concat(p,',')..']' end
local function state()
 return string.format('{"clock":{"minutes":%d,"tens":%d,"ones":%d,"videoRemainder":%d,"displayedSecond":%d,"hudTurn":%d,"scoreLowCache":%s},"phase":%d,"entered":%s,"period":%d,"overtimeFlags":%d,"displayedScores":[%s,%s]}',memory.readbyte(0x6c2),memory.readbyte(0x6c3),memory.readbyte(0x6c4),memory.readbyte(0x6c5),memory.readbyte(0x3cd),memory.readbyte(0x3ce),arr(0x3cb,2),memory.readbyte(0x589)%128,tostring(memory.readbyte(0x589)>=128),memory.readbyte(0x58c),memory.readbyte(0x6f7),arr(0x160,3),arr(0x163,3))
end
local function context()
 return string.format('{"frameParity":%d,"buttons":%d,"holder":%d,"ballAirborne":%s,"scores":[%d,%d],"tactics":%s,"tacticCache":%s}',memory.readbyte(0x300)%2,memory.readbyte(4),memory.readbyte(0x53f),tostring(memory.readbyte(0x5fd)>=128),memory.readbyte(0x166)+256*memory.readbyte(0x168),memory.readbyte(0x167)+256*memory.readbyte(0x169),arr(0x6c6,4),arr(0x3c7,4))
end
local function begin(id)
 assert(not pending[id]);pending[id]={frame=emu.framecount(),before=state(),ctx=context(),pre=0,post=0,sounds={},clockDone=false}
end
local function finish(id)
 local p=pending[id];assert(p)
 f:write(string.format('{"kind":"%s","frame":%d,"before":%s,"after":%s,"context":%s,"videoBeforeClock":%d,"videoAfterClock":%d,"sounds":[%s]}\n',id,p.frame,p.before,state(),p.ctx,p.pre,p.post,table.concat(p.sounds,',')));pending[id]=nil
end
memory.registerexec(0xf25c,1,function()
 if memory.readbyte(0x589)~=2 then for _,p in pairs(pending) do if p.clockDone then p.post=p.post+1 else p.pre=p.pre+1 end end end
 begin('video')
end)
memory.registerexec(0xf266,1,function() finish('video') end)
memory.registerexec(0xd35a,1,function()begin('pause')end)
memory.registerexec(0xd36b,1,function()finish('pause')end)
memory.registerexec(0xd36c,1,function()begin('end')end)
memory.registerexec(0xd3c3,1,function()if pending['end'] then finish('end')end end)
memory.registerexec(0xd3eb,1,function()begin('resume')end)
memory.registerexec(0xd3f8,1,function()finish('resume')end)
memory.registerexec(0xaff3,1,function()if memory.readbyte(0xbfff)==4 then begin('hud')end end)
memory.registerexec(0xfd25,1,function()finish('hud')end)
memory.registerexec(0xb1ca,1,function()if memory.readbyte(0xbfff)==4 then begin('clock')end end)
memory.registerexec(0xb226,1,function()if memory.readbyte(0xbfff)==4 and pending['clock'] then finish('clock');if pending['hud'] then pending['hud'].clockDone=true end end end)
memory.registerexec(0xfbcc,1,function()for _,p in pairs(pending)do p.sounds[#p.sounds+1]=memory.getregister('a') end end)
function finish_probe() assert(next(pending)==nil);f:close()end
'''
def main():
 import argparse
 parser=argparse.ArgumentParser();parser.add_argument('--reuse-existing',action='store_true');args=parser.parse_args()
 records=[];sources=[]
 for reference_name in ['match-clock-complete','match-clock-pause']:
  reference=BASE/'runs'/reference_name
  if reference_name=='match-clock-complete':
   expected=dict(name=reference_name,load_slot=5,frames=24000,captures=[12000,22500,24000],segments=[dict(start=11000,end=11001,keys=['start']),dict(start=11010,end=11011,keys=['A'])]+[dict(start=n,end=n+1,keys=['A']) for n in range(11040,11500,30)])
  else:
   expected=dict(name=reference_name,load_slot=5,frames=300,captures=[65,95,300],segments=[dict(start=60,end=66,keys=['start']),dict(start=90,end=96,keys=['start'])])
  if not (reference/'scenario.json').exists():
   baseline=BASE/(reference_name+'.json');baseline.write_text(json.dumps(expected,indent=2)+'\n');run(baseline)
  spec=json.loads((reference/'scenario.json').read_text());assert spec==expected,'reference inputs differ from declared clock scenario'
  name='match-clock-observation' if reference_name=='match-clock-complete' else 'match-clock-pause-observation'
  spec.update(name=name,hooks_lua=HOOK)
  path=BASE/(spec['name']+'.json');path.write_text(json.dumps(spec,indent=2)+'\n')
  out=BASE/'runs'/spec['name']
  reusable=args.reuse_existing and (out/'scenario.json').exists() and json.loads((out/'scenario.json').read_text())==spec and (out/'match-clock.jsonl').exists()
  if not reusable:run(path)
  assert (out/'ram.bin').read_bytes()==(reference/'ram.bin').read_bytes()
  for p in reference.glob('frame-*.png'):assert p.read_bytes()==(out/p.name).read_bytes()
  records.extend(dict(scenario=reference_name,**json.loads(l)) for l in (out/'match-clock.jsonl').read_text().splitlines())
  sources.append(json.loads((out/'source.json').read_text()))
 target=ROOT/'games/basketball/engine/testdata/match-clock.json.gz';data=json.dumps(records,separators=(',',':')).encode()
 target.write_bytes(gzip.compress(data,mtime=0))
 counts={k:sum(r['kind']==k for r in records) for k in sorted({r['kind'] for r in records})}
 metadata=dict(records=len(records),kinds=counts,source_is_original_execution=True,ram_matches_uninstrumented=True,sha256=hashlib.sha256(target.read_bytes()).hexdigest(),sources=sources)
 target.with_name('match-clock-source.json').write_text(json.dumps(metadata,indent=2)+'\n');print(json.dumps(metadata))
if __name__=='__main__':main()
