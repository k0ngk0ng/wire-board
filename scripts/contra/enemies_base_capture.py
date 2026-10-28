"""Original base-room enemy calls, room loads and shared-state observations.

Each run starts from a source-bound original stage entry. Optional Boss setup is
declared once and then uses the original room transition. The recorder never
generates expected state or patches enemy movement/damage after initialization.
"""
from pathlib import Path
import os,gzip,hashlib,json,shutil,argparse
ROOT=Path(__file__).resolve().parents[2];BASE=ROOT/'.local/contra-enemies-base/oracle'
os.environ['CONTRA_REFERENCE_ROOT']=str(BASE)
from capture_reference import run
FIELDS=[0x30a,0x324,0x33e,0x358,0x4b8,0x4c8,0x4d8,0x4e8,0x4f8,0x508,0x518,0x528,0x538,0x548,0x558,0x568,0x578,0x588,0x598,0x5a8,0x5b8,0x5c8,0x5d8,0x5e8]
WORLD=[('frame',0x1a),('weaponStrength',0x2f),('level',0x30),('gameLoop',0x31),('random',0x34),('boss',0x3b),('sceneType',0x40),('vertical',0x41),('screen',0x64),('roomCleared',0x6a),('frameScroll',0x68),('indoorScroll',0x73),('cores',0x86),('plates',0x87),('rounds',0x88),('redCreated',0x89),('grenadeLauncher',0x8a),('attackEnabled',0x8e),('readOffset',0x82),('delayLow',0x2a),('delayHigh',0x2b),('scrollY',0xfc),('scrollX',0xfd),('control',0xff)]

HOOK=r'''
local f=assert(io.open(out..'/base-enemies.jsonl','w'));local fields=FIELDS;local worldFields=WORLD;local pending={};local returns={};local hooks={};local count=0
local function read(a)return memory.readbyte(a)end
local function ret()local sp=memory.getregister('s');return read(0x100+AND(sp+1,255))+256*read(0x100+AND(sp+2,255))+1,AND(sp+2,255)end
local function hex(a,n)local t={};for i=a,a+n-1 do t[#t+1]=string.format('%02x',read(i))end;return table.concat(t)end
local function array(addresses,offset)local t={};for _,a in ipairs(addresses)do t[#t+1]=tostring(read(a+offset))end;return '['..table.concat(t,',')..']'end
local function snapshot()
 local pool={};for i=0,15 do pool[#pool+1]=array(fields,i)end
 local players={};for i=0,1 do players[#players+1]=string.format('{"state":%d,"x":%d,"y":%d,"gameOver":%d}',read(0x90+i),read(0x334+i),read(0x31a+i),read(0x38+i))end
 return '{"pool":['..table.concat(pool,',')..'],"world":'..array(worldFields,0)..',"players":['..table.concat(players,',')..'],"queueUsed":'..read(0x21)..',"queue":"'..hex(0x700,128)..'","supertiles":"'..hex(0x600,128)..'","collision":"'..hex(0x680,128)..'"}'
end
local function arm(pc)
 if returns[pc]then return end;returns[pc]=true
 memory.registerexec(pc,1,function()
  local sp=memory.getregister('s')
  for i=#pending,1,-1 do
   local p=pending[i]
   if p.pc==pc and p.sp==sp then
    f:write(string.format('{"id":%d,"name":"%s","frame":%d,"slot":%d,"type":%d,"before":%s,"after":%s,"audio":[%s],"requests":[%s]}\n',p.id,p.name,p.frame,p.slot,p.kind,p.before,snapshot(),table.concat(p.audio,','),table.concat(p.requests,',')))
    table.remove(pending,i)
   end
  end
  if hooks[pc]then hooks[pc]()end
 end)
end
local function hook(pc,name,bank)
 hooks[pc]=function()
  if bank and read(0x8000)~=bank then return end
  local level=read(0x30);if level~=1 and level~=3 then return end
  local slot=memory.getregister('x');count=count+1;local returnPC,sp=ret()
  pending[#pending+1]={id=count,name=name,pc=returnPC,sp=sp,frame=emu.framecount(),slot=slot,kind=slot<16 and read(0x528+slot) or 255,before=snapshot(),audio={},requests={}}
  arm(returnPC)
 end;arm(pc)
end
hook(0xe58a,'step',0x10)
hook(0xaa11,'room-load',0x14)
memory.registerexec(0xf9bc,1,function()for _,p in ipairs(pending)do p.audio[#p.audio+1]=string.format('{"kind":"play","code":%d}',memory.getregister('a'))end end)
memory.registerexec(0xf9d4,1,function()for _,p in ipairs(pending)do p.audio[#p.audio+1]='{"kind":"reset"}'end end)
memory.registerexec(0xc3d1,1,function()for _,p in ipairs(pending)do p.requests[#p.requests+1]=string.format('{"kind":"supertile","x":%d,"y":%d,"code":%d,"used":%d}',memory.getregister('a'),memory.getregister('y'),read(0x10),read(0x21))end end)
memory.registerexec(0xc593,1,function()for _,p in ipairs(pending)do p.requests[#p.requests+1]=string.format('{"kind":"tiles","x":%d,"y":%d,"code":%d,"used":%d}',memory.getregister('a'),memory.getregister('y'),read(0x10),read(0x21))end end)
SETUP
function control_frame(n,pads)
 for i=0,1 do
  pads[i+1].B=n%4==1
  if read(0x40)==1 and read(0x6a)~=0 then pads[i+1].up=true end
  if read(0x90+i)==3 and n%30==1 then pads[i+1].A=true end
 end
end
function finish_probe()assert(#pending==0,'unfinished base call');f:close()end
'''

def capture(stage,mode,label,frames):
    (BASE/'states').mkdir(parents=True,exist_ok=True)
    source=ROOT/f'.local/contra-campaign/states/stage-{stage}-v2.fcs';target=BASE/f'states/stage-{stage}.fcs'
    if not target.exists():shutil.copyfile(source,target)
    assert target.read_bytes()==source.read_bytes()
    setup=''
    if mode=='boss':setup="local old=memory.readbyte(0x64);local target=memory.readbyte(0x58)-1;memory.writebyte(0x64,target);memory.writebyte(0x6a,128);memory.writebyte(0xb0,255);memory.writebyte(0xb1,255);writefile('setup.json',string.format('{\"screenBefore\":%d,\"screenAfter\":%d,\"cleared\":128,\"barrier\":[255,255],\"scope\":\"controlled setup followed by original room transition\"}',old,target))"
    spec={'name':f'base-{stage}-{mode}-{label}','load_state':f'stage-{stage}','frames':frames,'captures':[1,200,600,frames],
          'segments':[],'hooks_lua':HOOK.replace('FIELDS','{'+','.join(hex(a)for a in FIELDS)+'}').replace('WORLD','{'+','.join(hex(a)for _,a in WORLD)+'}').replace('SETUP',setup),'timeout_seconds':120}
    path=BASE/(spec['name']+'.json');path.write_text(json.dumps(spec,indent=2)+'\n');out=run(path)
    rows=[json.loads(line)for line in (out/'base-enemies.jsonl').read_text().splitlines()]
    worldIndex={name:i for i,(name,_)in enumerate(WORLD)};counts={};phases={}
    for row in rows:
        if row['name']=='step':counts[str(row['type'])]=counts.get(str(row['type']),0)+1;phases.setdefault(str(row['type']),set()).add(row['before']['pool'][row['slot']][4])
    result={'source_is_original_execution':True,'stage':stage,'mode':mode,'scope':'controller-only continuation from a controlled original stage-entry snapshot'if mode=='room'else'one controlled room/clear/barrier setup; original transition and subsequent enemy execution',
            'source':json.loads((out/'source.json').read_text()),'scenario':spec,'enemy_fields':FIELDS,'world_fields':[{'name':name,'address':hex(a)}for name,a in WORLD],
            'setup':json.loads((out/'setup.json').read_text())if mode=='boss'else None,'records':rows,'counts':counts,'phases':{k:sorted(v)for k,v in phases.items()},
            'actual_inputs_sha256':hashlib.sha256((out/'inputs.csv').read_bytes()).hexdigest(),'ram_sha256':hashlib.sha256((out/'ram.bin').read_bytes()).hexdigest()}
    packed=gzip.compress(json.dumps(result,separators=(',',':')).encode(),mtime=0);dest=ROOT/f'docs/research/nes-contra/evidence/enemies-base-{stage}-{mode}.json.gz'
    if dest.exists():
        old=json.loads(gzip.decompress(dest.read_bytes()));assert old['records']==rows,'repeat differs'
    else:dest.write_bytes(packed)
    print(json.dumps({'stage':stage,'mode':mode,'records':len(rows),'counts':counts,'phases':result['phases'],'scene_types':sorted({r['before']['world'][worldIndex['sceneType']]for r in rows})}))

def main():
    parser=argparse.ArgumentParser();parser.add_argument('--stage',type=int,choices=[2,4],required=True);parser.add_argument('--mode',choices=['room','boss'],default='room');parser.add_argument('--label',default='v1');parser.add_argument('--frames',type=int,default=1200);args=parser.parse_args();capture(args.stage,args.mode,args.label,args.frames)

if __name__=='__main__':main()
