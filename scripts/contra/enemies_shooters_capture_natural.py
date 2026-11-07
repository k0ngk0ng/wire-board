"""Observe continuous original shooter routines from ordinary controller play.

The opening reuses a verified natural machine-gun pickup input timeline. Later
inputs are a reproducible controller-only traversal policy, logged per frame.
No memory, enemy, life, weapon, result or progression value is patched.
"""
from pathlib import Path
import os,gzip,hashlib,json,shutil,sys
ROOT=Path(__file__).resolve().parents[2];BASE=ROOT/'.local/contra-enemies-shooters/natural'
os.environ['CONTRA_REFERENCE_ROOT']=str(BASE)
from capture_reference import run
FIELDS=[0x30a,0x324,0x33e,0x358,0x4b8,0x4c8,0x4d8,0x4e8,0x4f8,0x508,0x518,0x528,0x538,0x548,0x558,0x568,0x578,0x588,0x598,0x5a8,0x5b8,0x5c8,0x5d8,0x5e8]
HOOK=r'''
local f=assert(io.open(out..'/natural-shooters.jsonl','w'));local input=assert(io.open(out..'/input-policy.jsonl','w'))
local fields=FIELDS;local sequence=0;local pending=nil;local returns={};local kinds={[4]=true,[6]=true,[7]=true,[16]=true}
local function read(a)return memory.readbyte(a)end
local function ret()local sp=memory.getregister('s');return read(0x100+AND(sp+1,255))+256*read(0x100+AND(sp+2,255))+1 end
local function hex(a,n)local t={};for i=a,a+n-1 do t[#t+1]=string.format('%02x',read(i))end;return table.concat(t)end
local function pool()local result={};for slot=0,15 do local values={};for _,a in ipairs(fields)do values[#values+1]=tostring(read(a+slot))end;result[#result+1]='['..table.concat(values,',')..']'end;return '['..table.concat(result,',')..']'end
local function players()local t={};for i=0,1 do t[#t+1]=string.format('{"state":%d,"x":%d,"y":%d}',read(0x90+i),read(0x334+i),read(0x31a+i))end;return '['..table.concat(t,',')..']'end
local function arm(pc)
 if returns[pc]then return end;returns[pc]=true
 memory.registerexec(pc,1,function()
  if not pending or pending.pc~=pc then return end
  local p=pending;pending=nil
  f:write(string.format('{"frame":%d,"slot":%d,"kind":%d,"world":%s,"before":%s,"after":%s,"sounds":[%s],"requests":[%s],"supertiles":"%s","usedBefore":%d,"usedAfter":%d,"queueBefore":"%s","queueAfter":"%s"}\n',p.frame,p.slot,p.kind,p.world,p.before,pool(),table.concat(p.sounds,','),table.concat(p.requests,','),p.supertiles,p.used,read(0x21),p.queue,hex(0x700,128)))
 end)
end
memory.registerexec(0xf9bc,1,function()if pending then pending.sounds[#pending.sounds+1]=tostring(memory.getregister('a'))end end)
memory.registerexec(0xc3d1,1,function()if pending then pending.requests[#pending.requests+1]=string.format('{"x":%d,"y":%d,"code":%d,"used":%d}',memory.getregister('a'),memory.getregister('y'),read(0x10),read(0x21))end end)
memory.registerexec(0xe58a,1,function()
 if read(0x8000)~=0x10 then return end
 local slot=memory.getregister('x');local kind=read(0x528+slot)
 if not kinds[kind]then return end
 assert(not pending)
 local world=string.format('{"level":%d,"sceneType":%d,"frame":%d,"scroll":%d,"vertical":%d,"enabled":%d,"strength":%d,"loop":%d,"random":%d,"scrollX":%d,"scrollY":%d,"control":%d,"aliasX":%d,"aliasY":%d,"players":%s}',read(0x30),read(0x40),read(0x1a),read(0x68),read(0x41),read(0x8e),read(0x2f),read(0x31),read(0x34),read(0xfd),read(0xfc),read(0xff),read(0x3ba),read(0x3a0),players())
 pending={pc=ret(),frame=emu.framecount(),slot=slot,kind=kind,world=world,before=pool(),sounds={},requests={},supertiles=hex(0x600,128),used=read(0x21),queue=hex(0x700,128)};arm(pending.pc)
end)
function control_frame(n,pads)
 sequence=n
 if sequence<=350 then return end
 for i=0,1 do
  local keys={A=false,B=false,up=false,down=false,left=false,right=false,start=false,select=false}
  keys.right=true
  local weapon=AND(read(0xaa+i),15)
  keys.B=(weapon==1 or weapon==4 or sequence%4==1)
  if read(0x90+i)==1 and read(0xa0+i)==0 and read(0xa4+i)==0 and read(0xb2+i)==0 then keys.A=true end
  if read(0x90+i)==3 and sequence%30==1 then keys.A=true end
  pads[i+1]=keys
  input:write(string.format('{"frame":%d,"player":%d,"right":true,"A":%s,"B":%s}\n',sequence,i+1,tostring(keys.A),tostring(keys.B)))
 end
end
function finish_probe()assert(not pending);f:close();input:close()end
'''

def main():
    name=sys.argv[1]if len(sys.argv)>1 else 'shooters-natural-v1'
    retained=ROOT/'games/contra/engine/testdata/original-enemy-shooters-natural.json.gz'
    if retained.exists():
        opening=json.loads(gzip.decompress(retained.read_bytes()))['scenario']
    else:
        opening=json.loads((ROOT/'.local/contra-weapons/pickup/runs/natural-machinegun-01/scenario.json').read_text())
    segments=[dict(s,end=min(350,s['end']))for s in opening['segments']if s['start']<=350]
    (BASE/'states').mkdir(parents=True,exist_ok=True);origin=ROOT/'.local/contra/states/coop-play.fcs';target=BASE/'states/coop-play.fcs'
    if not target.exists():shutil.copyfile(origin,target)
    assert target.read_bytes()==origin.read_bytes()
    spec={'name':name,'load_state':'coop-play','frames':5000,'segments':segments,'captures':[350,1000,2000,3000,4000,5000],
          'hooks_lua':HOOK.replace('FIELDS','{'+','.join(hex(a)for a in FIELDS)+'}'),'timeout_seconds':90}
    path=BASE/(name+'.json');path.write_text(json.dumps(spec,indent=2)+'\n');out=run(path)
    records=[json.loads(line)for line in (out/'natural-shooters.jsonl').read_text().splitlines()]
    counts={};states={}
    for r in records:
        counts[r['kind']]=counts.get(r['kind'],0)+1;states.setdefault(r['kind'],set()).add(r['before'][r['slot']][4])
    ram=(out/'ram.bin').read_bytes();screens=sorted({ram[i+0x64]for i in range(0,len(ram),2048)if ram[i+0x18]==5})
    data={'source_is_original_execution':True,'interventions':'none; only actual controller inputs, including a logged traversal policy after frame350',
          'source':json.loads((out/'source.json').read_text()),'scenario':spec,'input_policy_sha256':hashlib.sha256((out/'input-policy.jsonl').read_bytes()).hexdigest(),
          'actual_inputs_sha256':hashlib.sha256((out/'inputs.csv').read_bytes()).hexdigest(),
          'records':records,'counts':counts,'states':{str(k):sorted(v)for k,v in states.items()},'screens':screens}
    packed=gzip.compress(json.dumps(data,separators=(',',':')).encode(),mtime=0)
    path=ROOT/'games/contra/engine/testdata/original-enemy-shooters-natural.json.gz'
    if path.exists():
        old=json.loads(gzip.decompress(path.read_bytes()));assert old['records']==records,'repeat differs'
    else:path.write_bytes(packed)
    print(json.dumps({'counts':counts,'states':data['states'],'screens':screens,'sha256':hashlib.sha256(path.read_bytes()).hexdigest()}))

if __name__=='__main__':main()
