"""Observe a complete door/tunnel continuation after one declared setup.

The clear sequence begins at defeated-door state3. The contention sequence
begins at tunnel state7 alongside surviving animated weapon boxes. Only that
initial setup is written; all later state, graphics and timing are original.
"""
from pathlib import Path
import os,gzip,hashlib,json,shutil,sys
ROOT=Path(__file__).resolve().parents[2];BASE=ROOT/'.local/contra-enemies-shooters/bossdoor-sequence'
os.environ['CONTRA_REFERENCE_ROOT']=str(BASE)
from capture_reference import run
FIELDS=[0x30a,0x324,0x33e,0x358,0x4b8,0x4c8,0x4d8,0x4e8,0x4f8,0x508,0x518,0x528,0x538,0x548,0x558,0x568,0x578,0x588,0x598,0x5a8,0x5b8,0x5c8,0x5d8,0x5e8]
HOOK=r'''
local f=assert(io.open(out..'/door-sequence.jsonl','w'));local fields=FIELDS;local mode=MODE;local setup=false;local pending=nil;local returns={}
local function read(a)return memory.readbyte(a)end
local function ret()local sp=memory.getregister('s');return read(0x100+AND(sp+1,255))+256*read(0x100+AND(sp+2,255))+1 end
local function hex(a,n)local t={};for i=a,a+n-1 do t[#t+1]=string.format('%02x',read(i))end;return table.concat(t)end
local function pool()local result={};for slot=0,15 do local values={};for _,a in ipairs(fields)do values[#values+1]=tostring(read(a+slot))end;result[#result+1]='['..table.concat(values,',')..']'end;return '['..table.concat(result,',')..']'end
local function globals()return string.format('{"delayLow":%d,"delayHigh":%d,"boss":%d}',read(0x2a),read(0x2b),read(0x3b))end
local function arm(pc)
 if returns[pc]then return end;returns[pc]=true
 memory.registerexec(pc,1,function()
  if not pending or pending.pc~=pc then return end
  local p=pending;pending=nil
  f:write(string.format('{"frame":%d,"slot":%d,"scroll":%d,"scrollX":%d,"scrollY":%d,"control":%d,"before":%s,"after":%s,"globalBefore":%s,"globalAfter":%s,"audio":[%s],"requests":[%s],"collisionBefore":"%s","collisionAfter":"%s","supertiles":"%s","usedBefore":%d,"usedAfter":%d,"queueBefore":"%s","queueAfter":"%s"}\n',p.frame,p.slot,p.scroll,p.sx,p.sy,p.control,p.before,pool(),p.globalBefore,globals(),table.concat(p.audio,','),table.concat(p.requests,','),p.collision,hex(0x680,128),p.supertiles,p.used,read(0x21),p.queue,hex(0x700,128)))
 end)
end
memory.registerexec(0xf9d4,1,function()if pending then pending.audio[#pending.audio+1]='{"kind":"reset"}'end end)
memory.registerexec(0xf9bc,1,function()if pending then pending.audio[#pending.audio+1]=string.format('{"kind":"play","code":%d}',memory.getregister('a'))end end)
memory.registerexec(0xc3d1,1,function()if pending then pending.requests[#pending.requests+1]=string.format('{"x":%d,"y":%d,"code":%d,"used":%d}',memory.getregister('a'),memory.getregister('y'),read(0x10),read(0x21))end end)
memory.registerexec(0xe54f,1,function()
 if setup then return end;setup=true
 for slot=0,15 do for _,a in ipairs(fields)do memory.writebyte(a+slot,0)end end
 local slot=mode==0 and 15 or 0
 local values={mode==0 and 1 or 0,mode==0 and 176 or 160,mode==0 and 160 or 144,0,mode==0 and 3 or 7,0,0,0,0,0,0,17,mode==0 and 8 or 1,0,0,mode==0 and 0 or 4,0,0x82,mode==0 and 0x0c or 0x8d,0,0,0,0,0}
 for i,a in ipairs(fields)do memory.writebyte(a+slot,values[i])end
 memory.writebyte(0x2a,0xff);memory.writebyte(0x2b,0);memory.writebyte(0x3b,mode)
 if mode==1 then
  for box=13,15 do
   memory.writebyte(0x528+box,2);memory.writebyte(0x4b8+box,3);memory.writebyte(0x30a+box,1)
   memory.writebyte(0x578+box,0xf0);memory.writebyte(0x598+box,0x0f);memory.writebyte(0x538+box,1)
   memory.writebyte(0x33e+box,80+(box-13)*32);memory.writebyte(0x324+box,160)
  end
 end
 writefile('setup.json',string.format('{"frame":%d,"mode":%d,"slot":%d,"intervention":"single defeated-door or tunnel plus surviving-box setup; subsequent execution unmodified"}',emu.framecount(),mode,slot))
end)
memory.registerexec(0xe58a,1,function()
 local slot=memory.getregister('x')
 if read(0x30)~=0 or read(0x528+slot)~=17 then return end
 assert(not pending)
 pending={pc=ret(),frame=emu.framecount(),slot=slot,scroll=read(0x68),sx=read(0xfd),sy=read(0xfc),control=read(0xff),before=pool(),globalBefore=globals(),audio={},requests={},collision=hex(0x680,128),supertiles=hex(0x600,128),used=read(0x21),queue=hex(0x700,128)};arm(pending.pc)
end)
function finish_probe()assert(setup and not pending);f:close()end
'''

def main():
    label=sys.argv[1]if len(sys.argv)>1 else 'v1'
    (BASE/'states').mkdir(parents=True,exist_ok=True);origin=ROOT/'.local/contra/states/coop-play.fcs';target=BASE/'states/coop-play.fcs'
    if not target.exists():shutil.copyfile(origin,target)
    assert target.read_bytes()==origin.read_bytes()
    sequences=[]
    for mode in (0,1):
        spec={'name':f'door-sequence-{label}-{mode}','load_state':'coop-play','frames':500,'segments':[],'captures':[1,60,120,240,500],
              'hooks_lua':HOOK.replace('FIELDS','{'+','.join(hex(a)for a in FIELDS)+'}').replace('MODE',str(mode))}
        path=BASE/(spec['name']+'.json');path.write_text(json.dumps(spec,indent=2)+'\n');out=run(path)
        rows=[json.loads(line)for line in (out/'door-sequence.jsonl').read_text().splitlines()];assert rows
        last=None
        for row in rows:
            actor=row['slot']
            if last is not None:assert row['before'][actor]==last,'door changed outside its step'
            last=row['after'][actor]
        assert last[4]==0,'door did not complete'
        if mode==1:assert any(q['used']>=64 for row in rows for q in row['requests']),'contention did not reject a real request'
        sequences.append({'mode':mode,'source':json.loads((out/'source.json').read_text()),'scenario':spec,'setup':json.loads((out/'setup.json').read_text()),'records':rows})
    data={'source_is_original_execution':True,'interventions':'One explicit initial enemy/progress setup per sequence; no later patches; not a natural Boss fight.','sequences':sequences}
    packed=gzip.compress(json.dumps(data,separators=(',',':')).encode(),mtime=0);dest=ROOT/'games/contra/engine/testdata/original-enemy-shooters-bossdoor-sequence.json.gz'
    if dest.exists():
        old=json.loads(gzip.decompress(dest.read_bytes()));assert [s['records']for s in old['sequences']]==[s['records']for s in sequences],'repeat differs'
    else:dest.write_bytes(packed)
    print(json.dumps({'sequences':[(s['mode'],len(s['records']))for s in sequences],'requests':sum(len(r['requests'])for s in sequences for r in s['records']),'sha256':hashlib.sha256(dest.read_bytes()).hexdigest()}))

if __name__=='__main__':main()
