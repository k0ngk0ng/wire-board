"""Original power-on initialization with declared stage/dirty/demo inputs only.

Entry and return hooks multiplex callbacks and validate stack depth. Full RAM,
nametables and audio requests are retained. No return state is ever injected.
"""
import os
from pathlib import Path
ROOT=Path(__file__).resolve().parents[2]
os.environ.setdefault('CONTRA_REFERENCE_ROOT',str(ROOT/'.local/contra-campaign-initialize/oracle'))
from capture_reference import BASE,run
import argparse,gzip,json,hashlib
HOOK=r'''
local f=assert(io.open(out..'/initialize.jsonl','w'));local pending={};local hooks={};local exits={};local staged=false
local function on(pc,callback)
 if not hooks[pc]then hooks[pc]={};memory.registerexec(pc,1,function()for _,fn in ipairs(hooks[pc])do fn()end end)end
 hooks[pc][#hooks[pc]+1]=callback
end
local function hex(first,n,reader)local v={};for a=first,first+n-1 do v[#v+1]=string.format('%02x',reader(a))end;return table.concat(v)end
local function ram()return hex(0,2048,memory.readbyte)end
local function ppuhex()return hex(0x2000,4096,ppu.readbyte)end
local function retpc()local sp=memory.getregister('s');return memory.readbyte(0x100+AND(sp+1,255))+256*memory.readbyte(0x100+AND(sp+2,255))+1 end
local function finish(pc)
 local names={};for name,p in pairs(pending)do if p.pc==pc and memory.getregister('s')==AND(p.sp+2,255)then names[#names+1]=name end end;table.sort(names)
 for _,name in ipairs(names)do local p=pending[name];pending[name]=nil;f:write(string.format('{"kind":"%s","frame":%d,"before":"%s","after":"%s","ppuBefore":"%s","ppuAfter":"%s","sounds":[%s]}\n',name,p.frame,p.before,ram(),p.ppu,ppuhex(),table.concat(p.sounds,',')))end
end
local function enter(name)
 if memory.readbyte(0x18)~=5 then return end
 assert(not pending[name],name..' nested');local pc=retpc()
 if not exits[pc]then exits[pc]=true;on(pc,function()finish(pc)end)end
 pending[name]={pc=pc,sp=memory.getregister('s'),frame=emu.framecount(),before=ram(),ppu=ppuhex(),sounds={}}
end
on(0xf9bc,function()for _,p in pairs(pending)do p.sounds[#p.sounds+1]=tostring(memory.getregister('a'))end end)
on(0xf9d4,function()for _,p in pairs(pending)do p.sounds[#p.sounds+1]='-1'end end)
on(0xcc5f,function()if not staged then staged=true;memory.writebyte(0x30,STAGE)end;enter('initialize')end)
on(0xcc7d,function()
 if DIRTY then for a=0x300,0x5ff do memory.writebyte(a,AND(a*37+11,255))end end
 if PRESERVE then
  local values={[0x32]=5,[0x33]=4,[0x94]=91,[0x95]=93,[0x96]=89,[0x97]=87,[0xaa]=19,[0xab]=2,[0xba]=255,[0xbb]=255,[0x5a]=1,[0x5b]=3,[0x5c]=9,[0x5d]=2,[0x60]=17,[0x61]=9,[0x62]=55,[0x63]=39,[0x64]=3,[0x65]=3,[0x66]=207,[0x67]=39,[0x69]=64,[0x74]=3,[0x82]=7,[0x194]=2}
  for a,v in pairs(values)do memory.writebyte(a,v)end
  for a=0x600,0x6ff do memory.writebyte(a,AND(a*13+7,255))end
 end
 if DEMO then memory.writebyte(0x1c,1)end
 enter('prepare')
end)
on(0xc328,function()if pending.prepare then enter('clear')end end)
on(0xa816,function()if pending.prepare then enter('header')end end)
on(0xdad3,function()if pending.prepare then enter('map')end end)
function finish_probe()assert(next(pending)==nil,'unfinished initialization');f:close()end
'''
def main():
 p=argparse.ArgumentParser();p.add_argument('--name',required=True);p.add_argument('--stage',type=int,default=0,choices=range(8));p.add_argument('--players',type=int,choices=[1,2],default=2);p.add_argument('--dirty',action='store_true');p.add_argument('--preserve',action='store_true');p.add_argument('--demo',action='store_true');p.add_argument('--verify-against');a=p.parse_args()
 BASE.mkdir(parents=True,exist_ok=True)
 segments=[{'start':120,'end':122,'keys':['start']},{'start':720,'end':722,'keys':['start']}]
 if a.players==2:segments.insert(0,{'start':100,'end':102,'keys':['select']})
 spec={'name':a.name,'frames':1800,'segments':segments,'captures':[1700],'timeout_seconds':120,'hooks_lua':HOOK.replace('STAGE',str(a.stage)).replace('DIRTY',str(a.dirty).lower()).replace('PRESERVE',str(a.preserve).lower()).replace('DEMO',str(a.demo).lower())}
 path=BASE/(a.name+'.json');path.write_text(json.dumps(spec,indent=2)+'\n');out=run(path)
 records=[json.loads(line)for line in(out/'initialize.jsonl').read_text().splitlines()]
 assert {r['kind']for r in records}=={'initialize','prepare','clear','header','map'}
 assert len(records)==5
 payload={'scope':'Original power-on and genuine controller flow. Declared interventions only: stage index at CC5F; optional dirty transient pages, preservation sentinels and demo flag at CC7D, precisely listed in scenario hooks. No return state injection.','stage':a.stage,'players':a.players,'dirty':a.dirty,'preserve':a.preserve,'demo':a.demo,'source':json.loads((out/'source.json').read_text()),'scenario':spec,'records':records}
 if a.verify_against:
  assert Path(a.verify_against).name==a.verify_against
  reference=ROOT/'docs/research/nes-contra/evidence'/a.verify_against
  prior=json.loads(gzip.decompress(reference.read_bytes()))
  assert prior['records']==records,'independent original record differs'
  report={'reference':str(reference.relative_to(ROOT)),'record_count':len(records),'records_equal':True,'records_sha256':hashlib.sha256(json.dumps(records,sort_keys=True,separators=(',',':')).encode()).hexdigest()}
  (out/'repeat-verification.json').write_text(json.dumps(report,indent=2)+'\n')
  print(json.dumps(report));return
 data=gzip.compress(json.dumps(payload,separators=(',',':')).encode(),mtime=0)
 dest=ROOT/'docs/research/nes-contra/evidence'/('campaign-initialize-'+a.name+'-original.json.gz');dest.write_bytes(data)
 print(json.dumps({'name':a.name,'records':len(records),'sha256':hashlib.sha256(data).hexdigest()}))
if __name__=='__main__':main()
