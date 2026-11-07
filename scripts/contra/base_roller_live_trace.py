"""Read-only inherited-attribute producer trace on the fixed Base2 world route."""
from pathlib import Path
import json,os,shutil,sys
ROOT=Path(__file__).resolve().parents[2];BASE=ROOT/'.local/contra-base-roller-live'
os.environ['CONTRA_REFERENCE_ROOT']=str(BASE)
from capture_reference import run

def main():
    label=sys.argv[1]if len(sys.argv)>1 else'v1';stage=int(sys.argv[2])if len(sys.argv)>2 else 4
    src=ROOT/f'.local/contra-world-eight-oracle/runs/world-eight-stage-{stage}-v2/scenario.json';spec=json.loads(src.read_text());spec['name']='roller-live-'+label;spec['captures']=[]
    (BASE/'states').mkdir(parents=True,exist_ok=True);start=ROOT/f'.local/contra-campaign/states/stage-{stage}-v2.fcs';dest=BASE/f'states/stage-{stage}.fcs'
    if dest.exists():assert start.read_bytes()==dest.read_bytes()
    else:shutil.copyfile(start,dest)
    spec['hooks_lua']+=r'''
local trace=assert(io.open(out..'/inherited-attribute.jsonl','w'))
memory.registerwrite(0x0a,1,function(a,n,v)
 if pending then trace:write(string.format('{"kind":"write","index":%d,"frame":%d,"pc":%d,"bank":%d,"slot":%d,"value":%d,"a":%d,"x":%d,"y":%d,"code":%d,"used":%d}\n',ordinal,emu.framecount(),memory.getregister('pc'),memory.readbyte(0x8000),memory.readbyte(0x83),v,memory.getregister('a'),memory.getregister('x'),memory.getregister('y'),memory.readbyte(0x10),memory.readbyte(0x21)))end
end)
for _,pc in ipairs({0xe54f,0xe58a,0x971b,0x971e,0xc593})do
 local address=pc
 memory.registerexec(address,1,function()
  if pending then trace:write(string.format('{"kind":"entry","index":%d,"frame":%d,"pc":%d,"bank":%d,"slot":%d,"value":%d,"a":%d,"x":%d,"y":%d,"code":%d,"used":%d}\n',ordinal,emu.framecount(),address,memory.readbyte(0x8000),memory.readbyte(0x83),memory.readbyte(0x0a),memory.getregister('a'),memory.getregister('x'),memory.getregister('y'),memory.readbyte(0x10),memory.readbyte(0x21)))end
 end)
end
local finish_world=finish_probe
function finish_probe()finish_world();trace:close()end
'''
    path=BASE/(spec['name']+'.json');path.write_text(json.dumps(spec,indent=2)+'\n');out=run(path)
    rows=[json.loads(l)for l in(out/'inherited-attribute.jsonl').read_text().splitlines()]
    previous=None;consumers=[];counts={}
    for r in rows:
        if r['kind']=='write':previous=r
        if r['kind']=='entry'and r['pc']==0x971b:
            consumers.append({'index':r['index'],'value':r['value'],'producer':previous})
            key=f'{previous["bank"]:02x}:{previous["pc"]:04x}'if previous else'unknown';counts[key]=counts.get(key,0)+1
    (out/'producer-summary.json').write_text(json.dumps({'stage':stage,'consumers':consumers,'counts':counts},indent=2)+'\n')
    print(json.dumps({'stage':stage,'consumers':len(consumers),'producerCounts':counts}))

if __name__=='__main__':main()
