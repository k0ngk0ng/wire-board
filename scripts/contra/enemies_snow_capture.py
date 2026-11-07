"""Original snow-field enemy calls, with explicit natural/controlled scopes.

Failures considered before Go rules: wrong per-stage dispatch/properties,
same-slot palette/scroll ordering, fractional high-byte tank visibility,
spawned child slots omitted, queue refusal changes delayed phases, and Boss
clear/delay/audio reset mistaken for instant final results.
"""
import gzip,hashlib,json,os,shutil,sys
from pathlib import Path

ROOT=Path(__file__).resolve().parents[2]
BASE=ROOT/'.local/contra-enemies-snow/oracle'
os.environ['CONTRA_REFERENCE_ROOT']=str(BASE)
from capture_reference import run

FIELDS=[0x30a,0x324,0x33e,0x358,0x4b8,0x4c8,0x4d8,0x4e8,0x4f8,0x508,0x518,0x528,0x538,0x548,0x558,0x568,0x578,0x588,0x598,0x5a8,0x5b8,0x5c8,0x5d8,0x5e8]
WORLD=[('frame',0x1a),('weaponStrength',0x2f),('level',0x30),('loop',0x31),('random',0x34),('boss',0x3b),('sceneType',0x40),('scrollType',0x41),('screen',0x64),('screenScroll',0x65),('frameScroll',0x68),('fade',0x74),('autoScrollA',0x75),('autoScrollB',0x76),('tankScroll',0x77),('palettePause',0x78),('jointScroll',0x7f),('bossScrollComplete',0x84),('bossCount',0x85),('attackEnabled',0x8e),('activePlayers',0xdf),('scrollY',0xfc),('scrollX',0xfd),('control',0xff),('delayLow',0x2a),('delayHigh',0x2b),('levelRoutine',0x2c),('generationBias',0x0b)]

HOOK=r'''
local f=assert(io.open(out..'/snow.jsonl','w'));local pending=nil;local requests=nil;local returns={};local serial=0
local fields=FIELDS;local worldFields=WORLD
CONTROLLED
local function read(a)return memory.readbyte(a)end
local function hex(a,n)local t={};for k=a,a+n-1 do t[#t+1]=string.format('%02x',read(k))end;return table.concat(t)end
local function ret()local sp=memory.getregister('s');return read(0x100+AND(sp+1,255))+256*read(0x100+AND(sp+2,255))+1,AND(sp+2,255)end
local function array(fields,offset)local t={};for _,a in ipairs(fields)do t[#t+1]=tostring(read(a+offset))end;return '['..table.concat(t,',')..']'end
local function state()
 local pool={};for i=0,15 do pool[#pool+1]=array(fields,i)end
 local players={};for i=0,1 do players[#players+1]=string.format('{"state":%d,"x":%d,"y":%d,"gameOver":%d}',read(0x90+i),read(0x334+i),read(0x31a+i),read(0x38+i))end
 return string.format('{"enemies":[%s],"world":%s,"players":[%s],"indices":"%s","palette":"%s","supertiles":"%s","terrain":"%s","used":%d,"queue":"%s"}',table.concat(pool,','),array(worldFields,0),table.concat(players,','),hex(0x50,8),hex(0x7c0,32),hex(0x600,128),hex(0x680,128),read(0x21),hex(0x700,128))
end
local function arm(pc)
 if returns[pc]then return end;returns[pc]=true
 memory.registerexec(pc,1,function()
  if requests and requests.pc==pc and requests.sp==memory.getregister('s') then
   local q=requests;requests=nil
   pending.events[#pending.events+1]=string.format('{"kind":"terrain","x":%d,"y":%d,"code":%d,"used":%d,"afterUsed":%d,"carry":%d,"address":%d,"attributeLookup":%d}',q.x,q.y,q.code,q.used,read(0x21),AND(memory.getregister('p'),1),read(0x12)+256*read(0x13),read(0x0f))
  end
  if not pending or pending.pc~=pc or pending.sp~=memory.getregister('s')then return end
  assert(not requests,'unfinished terrain call')
  local p=pending;pending=nil
  f:write(string.format('{"id":%d,"frame":%d,"slot":%d,"before":%s,"after":%s,"events":[%s]}\n',p.id,p.frame,p.slot,p.before,state(),table.concat(p.events,',')))
  if p.restore then for a,v in pairs(p.restore)do memory.writebyte(a,v)end end
 end)
end
memory.registerexec(0xf9bc,1,function()if pending then pending.events[#pending.events+1]=string.format('{"kind":"sound","code":%d}',memory.getregister('a'))end end)
memory.registerexec(0xf9d4,1,function()if pending then pending.events[#pending.events+1]='{"kind":"audioReset"}'end end)
memory.registerexec(0xcee3,1,function()if pending then pending.events[#pending.events+1]=string.format('{"kind":"palette","count":%d,"indices":"%s","fade":%d}',memory.getregister('a'),hex(0x50,8),read(0x74))end end)
memory.registerexec(0xc3d1,1,function()
 if not pending then return end;assert(not requests)
 local pc,sp=ret();requests={pc=pc,sp=sp,x=memory.getregister('a'),y=memory.getregister('y'),code=read(0x10),used=read(0x21)};arm(pc)
end)
memory.registerexec(0xe58a,1,function()
 local slot=memory.getregister('x');local kind=read(0x528+slot)
 if read(0x30)~=4 or kind<16 or kind>22 then return end
 assert(not pending);serial=serial+1
 local restore=nil
 if controlSetup and slot==15 then restore=controlSetup()end
 local pc,sp=ret();pending={pc=pc,sp=sp,id=serial,frame=emu.framecount(),slot=slot,before=state(),events={},restore=restore};arm(pc)
end)
function finish_probe()assert(not pending and not requests);f:close()end
'''

def main():
    label=sys.argv[1]if len(sys.argv)>1 else 'natural-v1'
    (BASE/'states').mkdir(parents=True,exist_ok=True);origin=ROOT/'.local/contra-campaign/states/stage-5-v2.fcs';target=BASE/'states/stage-5.fcs'
    if target.exists():assert target.read_bytes()==origin.read_bytes()
    else:shutil.copyfile(origin,target)
    spec={'name':'snow-'+label,'load_state':'stage-5','frames':3000,'captures':[1,600,1200,1800,2400,3000],'timeout_seconds':90,
       'segments':[dict(start=1,end=3000,player=p,keys=['right'])for p in [1,2]]+
       [dict(start=n,end=n,player=p,keys=['B'])for p in [1,2]for n in range(1,3001,4)]+
       [dict(start=n,end=n+1,player=p,keys=['A'])for p in [1,2]for n in range(30,3000,70)]}
    hook=HOOK.replace('FIELDS','{'+','.join(hex(a)for a in FIELDS)+'}').replace('WORLD','{'+','.join(hex(a)for _,a in WORLD)+'}').replace('CONTROLLED','local controlSetup=nil')
    spec['hooks_lua']=hook
    path=BASE/(spec['name']+'.json');path.write_text(json.dumps(spec,indent=2)+'\n');out=run(path)
    records=[json.loads(line)for line in (out/'snow.jsonl').read_text().splitlines()];assert records
    counts={}
    for row in records:
        e=row['before']['enemies'][row['slot']];key=f'{e[11]:02x}:{e[4]}'
        counts[key]=counts.get(key,0)+1
    fixture={'source_is_original_execution':True,'scope':'natural-input-after-controlled-stage-entry','interventions':'The source stage-5 entry has a single declared stage-index initialization override. This run uses ordinary controller input and observation only.',
             'source':json.loads((out/'source.json').read_text()),'scenario':spec,'world_fields':[{'name':n,'address':hex(a)}for n,a in WORLD],
             'enemy_fields':FIELDS,'records':records,'counts':counts}
    packed=gzip.compress(json.dumps(fixture,separators=(',',':')).encode(),mtime=0);target=ROOT/'games/contra/engine/testdata/original-enemies-snow-natural.json.gz'
    if target.exists():assert json.loads(gzip.decompress(target.read_bytes()))['records']==records,'repeat differs'
    else:target.write_bytes(packed)
    summary={'passed':True,'records':len(records),'counts':counts,'fixture_sha256':hashlib.sha256(target.read_bytes()).hexdigest(),'ram_sha256':hashlib.sha256((out/'ram.bin').read_bytes()).hexdigest()}
    (out/'snow-source.json').write_text(json.dumps(summary,indent=2)+'\n');print(json.dumps(summary))

if __name__=='__main__':main()
