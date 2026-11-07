"""Read original scene calls. Only exit scenarios stage a declared input once.

No after-state is written. The natural introduction/entrance starts at power-on
using real controller Start. Exit uses a named ordinary savestate plus one
explicit phase-input intervention; it is not a naturally defeated Boss claim.
"""
import os
from pathlib import Path
ROOT=Path(__file__).resolve().parents[2]
os.environ.setdefault('CONTRA_REFERENCE_ROOT',str(ROOT/'.local/contra-campaign-scene/oracle'))
from capture_reference import BASE,run
import argparse,gzip,json,shutil,hashlib
HOOK=r'''
local f=assert(io.open(out..'/scenes.jsonl','w'));local pending={};local hooks={};local exits={}
local function on(pc,callback)
 if not hooks[pc] then hooks[pc]={};memory.registerexec(pc,1,function()for _,fn in ipairs(hooks[pc])do fn()end end)end
 hooks[pc][#hooks[pc]+1]=callback
end
local function hexram()local v={};for a=0,2047 do v[#v+1]=string.format('%02x',memory.readbyte(a))end;return table.concat(v)end
local function retpc()local sp=memory.getregister('s');return memory.readbyte(0x100+AND(sp+1,255))+256*memory.readbyte(0x100+AND(sp+2,255))+1 end
local function finish(pc)
 local names={};for name,p in pairs(pending)do if p.pc==pc then names[#names+1]=name end end;table.sort(names)
 for _,name in ipairs(names)do local p=pending[name];pending[name]=nil;f:write(string.format('{"kind":"%s","frame":%d,"before":"%s","after":"%s","a":%d,"p":%d,"sounds":[%s]}\n',name,p.frame,p.before,hexram(),memory.getregister('a'),memory.getregister('p'),table.concat(p.sounds,',')))end
end
local function enter(name)
 if memory.readbyte(0x18)~=5 then return end
 assert(not pending[name],name..' nested');local pc=retpc()
 if not exits[pc]then exits[pc]=true;on(pc,function()finish(pc)end)end
 pending[name]={pc=pc,frame=emu.framecount(),before=hexram(),sounds={}}
end
on(0xf9bc,function()for _,p in pairs(pending)do p.sounds[#p.sounds+1]=tostring(memory.getregister('a'))end end)
on(0xf9d4,function()for _,p in pairs(pending)do p.sounds[#p.sounds+1]='-1'end end)
STAGING
on(0x888b,function()if memory.readbyte(0x8000)==0x1a then enter('exit')end end)
on(0x82d7,function()if memory.readbyte(0x8000)==0x1a then enter('intro')end end)
on(0xdb1c,function()enter('entrance')end)
on(0x9e22,function()if memory.readbyte(0x8000)==0x18 then enter('interlude')end end)
function finish_probe()assert(next(pending)==nil,'unfinished scene function');f:close()end
'''
STAGING=r'''
local staged=false
on(0xcc42,function()
 if staged then return end;staged=true
 local before=hexram()
 local values={[0x2c]=9,[0x2d]=0,[0x3b]=1,[0x190]=0,[0x191]=0,[0x192]=0,[0x193]=0,[0x194]=2,[0x25]=0}
 for a,v in pairs(values)do memory.writebyte(a,v)end
 -- The original still owns motion, collision, subsequent script stages and exit.
 for i=0,15 do memory.writebyte(0x4b8+i,0)end
 writefile('staging.json',string.format('{"kind":"controlled exit input only","frame":%d,"before":"%s","after":"%s"}',emu.framecount(),before,hexram()))
end)
'''
def main():
 p=argparse.ArgumentParser();p.add_argument('--name',required=True);p.add_argument('--kind',choices=['natural','exit','interlude'],required=True);p.add_argument('--stage',type=int,default=0,choices=range(8));a=p.parse_args()
 BASE.mkdir(parents=True,exist_ok=True)
 spec={'name':a.name,'frames':1800,'segments':[],'captures':[100,400,800,1200,1700],'timeout_seconds':120,'hooks_lua':HOOK.replace('STAGING',(STAGING if a.kind=='exit' else STAGING.replace('[0x2c]=9','[0x2c]=10').replace('[0x3b]=1','[0x3b]=2,[0xfb]=6,[0x30]='+str(a.stage))) if a.kind!='natural' else '')}
 if a.kind=='natural' and a.stage:
  spec['hooks_lua']='local stagedLevel=false;memory.registerexec(0xcc5f,1,function()if not stagedLevel then stagedLevel=true;memory.writebyte(0x30,'+str(a.stage)+')end end)\n'+spec['hooks_lua']
 if a.kind=='natural':
  spec['segments']=[{'start':100,'end':102,'keys':['select']},{'start':120,'end':122,'keys':['start']},{'start':720,'end':722,'keys':['start']}]
 else:
  (BASE/'states').mkdir(exist_ok=True)
  target=BASE/'states/coop-play.fcs';source=ROOT/'.local/contra/states/coop-play.fcs'
  if not target.exists():shutil.copyfile(source,target)
  assert target.read_bytes()==source.read_bytes();spec['load_state']='coop-play'
 path=BASE/(a.name+'.json');path.write_text(json.dumps(spec,indent=2)+'\n');out=run(path)
 records=[json.loads(line) for line in (out/'scenes.jsonl').read_text().splitlines()]
 payload={'scope':('Natural power-on controller-only introduction/entrance.' if not a.stage else 'Power-on with one declared stage-index input at CC5F; no subsequent state intervention.') if a.kind=='natural' else 'Declared one-time controlled exit input; no subsequent state intervention and no natural Boss victory claim.','source':json.loads((out/'source.json').read_text()),'scenario':spec,'records':records}
 if a.kind!='natural':payload['staging']=json.loads((out/'staging.json').read_text())
 data=gzip.compress(json.dumps(payload,separators=(',',':')).encode(),mtime=0)
 dest=ROOT/'docs/research/nes-contra/evidence'/('campaign-scene-'+a.kind+('-stage-'+str(a.stage+1) if a.stage else '')+'-original.json.gz');dest.write_bytes(data)
 print(json.dumps({'records':len(records),'counts':{k:sum(r['kind']==k for r in records)for k in sorted(set(r['kind']for r in records))},'sha256':hashlib.sha256(data).hexdigest()}))
if __name__=='__main__':main()
