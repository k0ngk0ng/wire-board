"""Read-only $06 producer trace on the retained real Boss-fight controller route.

The existing scenario's sole initial reserve-life intervention is preserved.
This adds observation only, with a private capture root and a new run name.
"""
from pathlib import Path
import os,json,shutil,sys
ROOT=Path(__file__).resolve().parents[2];BASE=ROOT/'.local/contra-enemies-world-bias'
os.environ['CONTRA_REFERENCE_ROOT']=str(BASE)
from capture_reference import run

def main():
    label=sys.argv[1]if len(sys.argv)>1 else'v1'
    source=ROOT/'.local/contra-world/campaign-oracle/runs/campaign-world-boss-v1/scenario.json'
    spec=json.loads(source.read_text());spec['name']='world-bias-1736-'+label;spec['frames']=1740;spec['captures']=[]
    (BASE/'states').mkdir(parents=True,exist_ok=True);start=ROOT/'.local/contra/states/coop-play.fcs';dest=BASE/'states/coop-play.fcs'
    if dest.exists():assert start.read_bytes()==dest.read_bytes()
    else:shutil.copyfile(start,dest)
    spec['hooks_lua']+=r'''
local bias=assert(io.open(out..'/bias.jsonl','w'))
local calls=assert(io.open(out..'/fire-calls.jsonl','w'));local fire=nil;local fireReturns={}
local function fireEvent(kind,pc)
 calls:write(string.format('{"kind":"%s","index":%d,"pc":%d,"bias":%d,"a":%d,"x":%d,"y":%d,"rapid":%d,"aim":%d,"shot":%d}\n',kind,ordinal,pc,memory.readbyte(6),memory.getregister('a'),memory.getregister('x'),memory.getregister('y'),memory.readbyte(9),memory.readbyte(0x0b),memory.readbyte(0x17)))
end
memory.registerexec(0x8001,1,function()
 if not pending or ordinal<1734 or ordinal>1738 or memory.readbyte(0x8000)~=0x1c then return end
 assert(not fire);local sp=memory.getregister('s')
 local pc=memory.readbyte(0x100+AND(sp+1,255))+256*memory.readbyte(0x100+AND(sp+2,255))+1
 fire={pc=pc,sp=AND(sp+2,255)};fireEvent('start',0x8001)
 if not fireReturns[pc]then fireReturns[pc]=true;memory.registerexec(pc,1,function()
  if fire and fire.pc==pc and fire.sp==memory.getregister('s')then fireEvent('end',pc);fire=nil end
 end)end
end)
memory.registerexec(0x848a,1,function()
 if fire then fireEvent('spread-velocity',0x848a)end
end)
memory.registerwrite(6,1,function(address,size,value)
 if pending and ordinal>=1734 and ordinal<=1738 then
  bias:write(string.format('{"index":%d,"frame":%d,"pc":%d,"bank":%d,"enemy":%d,"value":%d,"high":%d,"a":%d,"x":%d,"y":%d}\n',ordinal,emu.framecount(),memory.getregister('pc'),memory.readbyte(0x8000),memory.readbyte(0x83),value,memory.readbyte(7),memory.getregister('a'),memory.getregister('x'),memory.getregister('y')))
 end
end)
local prior_finish=finish_probe
function finish_probe()assert(not fire);prior_finish();bias:close();calls:close()end
'''
    path=BASE/(spec['name']+'.json');path.write_text(json.dumps(spec,indent=2)+'\n');out=run(path)
    rows=[json.loads(l)for l in(out/'bias.jsonl').read_text().splitlines()]
    for index in range(1734,1739):print(json.dumps({'index':index,'writes':[x for x in rows if x['index']==index]}))

if __name__=='__main__':main()
