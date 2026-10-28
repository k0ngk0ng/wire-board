"""Controlled original enemy dispatch proving the non-idle $34 writer.

Failure boundaries: wrong mapped bank/type/state, incoming Carry overwritten by
weapon-strength shifts, mutating shared $34 silently treated as a local sample,
and a post-instruction hook observing an intervening NMI instead of the write.
The explicit enemy/world setup is not natural traversal or a complete game.
"""
import gzip,hashlib,json,os,shutil,sys
from pathlib import Path
ROOT=Path(__file__).resolve().parents[2]
os.environ['CONTRA_REFERENCE_ROOT']=str(ROOT/'.local/contra-random/mutator')
from capture_reference import BASE,run

HOOK=r'''
local f=assert(io.open(out..'/mutator.jsonl','w'));local index=0;local cases=CASES;local saved=nil;local returns={};local before=nil
local function rb(a)return memory.readbyte(a)end
local function set(a,v)memory.writebyte(a,v)end
memory.registerexec(0xe54f,1,function()
 if index>=#cases then return end
 assert(not saved);index=index+1;local c=cases[index];saved={}
 for a=0,2047 do if a<256 or a>=512 then saved[a]=rb(a)end end
 for i=0,15 do for _,a in ipairs({0x30a,0x324,0x33e,0x358,0x4b8,0x4c8,0x4d8,0x4e8,0x4f8,0x508,0x518,0x528,0x538,0x548,0x558,0x568,0x578,0x588,0x598,0x5a8,0x5b8,0x5c8,0x5d8,0x5e8})do set(a+i,0)end end
 set(0x30,1);set(0x40,0x80);set(0x80,0xff);set(0x81,0xe5);set(0x1a,c.frame);set(0x34,c.value);set(0x2f,c.strength);set(0x8e,1)
 set(0x4b8+15,3);set(0x528+15,0x1c);set(0x558+15,1);set(0x33e+15,180);set(0x324+15,100)
 local s=memory.getregister('s');local ret=rb(0x100+AND(s+1,255))+256*rb(0x100+AND(s+2,255))+1
 if not returns[ret]then returns[ret]=true;memory.registerexec(ret,1,function()
  if not saved then return end
  assert(before==nil,'mutator was not completed')
  for a,v in pairs(saved)do set(a,v)end;saved=nil
 end)end
end)
memory.registerexec(0x9fa5,1,function()
 if not saved then return end
 assert(rb(0x8000)==0x10 and memory.getregister('x')==15,'wrong original mapping/slot')
 before={value=rb(0x34),frame=rb(0x1a),carry=AND(memory.getregister('p'),1),strength=rb(0x2f),scratch=rb(8),cycles=debugger.getcyclescount()}
end)
memory.registerwrite(0x34,1,function(a,s,v)
 if not before or memory.getregister('pc')~=0x9fab then return end
 local b=before;before=nil
 f:write(string.format('{"case":%d,"frame":%d,"value":%d,"carryIn":%d,"strength":%d,"strengthTimesEight":%d,"after":%d,"carryOut":%d,"writePC":%d,"cyclesBefore":%d,"cyclesAfter":%d}\n',index,b.frame,b.value,b.carry,b.strength,b.scratch,v,AND(memory.getregister('p'),1),memory.getregister('pc'),b.cycles,debugger.getcyclescount()))
end)
function finish_probe()assert(not saved and not before);f:close()end
'''

def main():
    label=sys.argv[1]if len(sys.argv)>1 else'v1'
    cases=[{'frame':f,'value':v,'strength':w}for f in [0,1,127,128,255]for v in [0,1,127,128,254,255]for w in [0,1,31,32,63,255]]
    encoded='{'+','.join('{'+','.join(f'{k}={v}'for k,v in c.items())+'}'for c in cases)+'}'
    (BASE/'states').mkdir(parents=True,exist_ok=True)
    source=ROOT/'.local/contra/states/coop-play.fcs';target=BASE/'states/coop-play.fcs'
    if not target.exists():shutil.copyfile(source,target)
    assert target.read_bytes()==source.read_bytes()
    spec={'name':'random-mutator-'+label,'load_state':'coop-play','frames':len(cases),'segments':[],'captures':[],'hooks_lua':HOOK.replace('CASES',encoded)}
    path=BASE/(spec['name']+'.json');path.write_text(json.dumps(spec,indent=2)+'\n');observed=run(path)
    records=[json.loads(s)for s in (observed/'mutator.jsonl').read_text().splitlines()]
    assert len(records)==len(cases),(len(records),len(cases))
    for a in records:
        assert a['carryIn']==(a['strength']>>5)&1
        total=a['frame']+a['value']+a['carryIn']
        assert (a['after'],a['carryOut'])==(total%256,total//256),a
    output=ROOT/f'docs/research/nes-contra/evidence/random-mutator-{label}.json.gz'
    assert not output.exists()
    result={'scope':'Controlled original enemy dispatch: level index1, enemy kind1C, state3, slot15. Explicit world/slot setup restored after the real enemy loop. Not natural traversal.','source':json.loads((observed/'source.json').read_text()),'scenario':spec,'cases':cases,'records':records,'passed':True}
    output.write_bytes(gzip.compress(json.dumps(result,separators=(',',':')).encode(),mtime=0))
    print(json.dumps({'passed':True,'cases':len(records),'sha256':hashlib.sha256(output.read_bytes()).hexdigest()}))

if __name__=='__main__':main()
