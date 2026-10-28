"""Record original C3D1 graphics requests before the independent queue exists.

Failure cases: wrong mapped tileset; stale CPU-buffer cursor; treating current
used<64 as a post-append capacity cap; skipped palette flag; half-supertile
attribute masks; horizontal carry/240-pixel vertical wrap; map/collision changed
too early; and mode23 incorrectly treated as a lock by this request routine.
Natural execution and controlled function-boundary setup are labeled separately.
"""
import gzip,hashlib,json,os,shutil,sys
from pathlib import Path

ROOT=Path(__file__).resolve().parents[2]
BASE=ROOT/'.local/contra-enemies-terrain/oracle'
os.environ['CONTRA_REFERENCE_ROOT']=str(BASE)
from capture_reference import run

HOOK=r'''
local records=assert(io.open(out..'/graphics-requests.jsonl','w'))
local pending=nil;local returns={};local ordinal=0
CASES
local function read(a)return memory.readbyte(a)end
local function word(a)return read(a)+256*read(a+1)end
local function hex(a,n)local t={};for k=a,a+n-1 do t[#t+1]=string.format('%02x',read(k))end;return table.concat(t)end
local function state()
 return string.format('{"used":%d,"mode":%d,"scrollX":%d,"scrollY":%d,"control":%d,"level":%d,"sceneType":%d,"mapAttributePointer":%d,"address":%d,"queue":"%s","supertiles":"%s","collision":"%s"}',read(0x21),read(0x23),read(0xfd),read(0xfc),read(0xff),read(0x30),read(0x40),word(0x46),word(0x12),hex(0x700,256),hex(0x600,128),hex(0x680,128))
end
local function arm(pc)
 if returns[pc]then return end;returns[pc]=true
 memory.registerexec(pc,1,function()
  if not pending then return end
  local p=pending
  if p.pc~=pc or p.sp~=memory.getregister('s')then return end
  records:write(string.format('{"index":%d,"frame":%d,"x":%d,"y":%d,"code":%d,"carry":%d,"before":%s,"after":%s}\n',p.index,p.frame,p.x,p.y,p.code,AND(memory.getregister('p'),1),p.before,state()))
  if p.restore then for a,v in pairs(p.restore)do memory.writebyte(a,v)end end
  pending=nil
 end)
end
memory.registerexec(0xc3d1,1,function()
 assert(not pending,'recursive graphics request')
 assert(read(0x8000)==0x16,'graphics request did not map bank6/7')
 ordinal=ordinal+1;local restore=nil
 if cases then
  local c=cases[ordinal];assert(c,'too many controlled requests')
  restore={}
  for _,a in ipairs({0x21,0x23,0xfc,0xfd,0xff})do restore[a]=read(a)end
  for a=0x700,0x77f do restore[a]=read(a);memory.writebyte(a,AND(a*7+3,255))end
  memory.writebyte(0x21,c.used);memory.writebyte(0x23,c.mode)
  memory.writebyte(0xfc,c.sy);memory.writebyte(0xfd,c.sx);memory.writebyte(0xff,c.control)
  memory.writebyte(0x10,c.code);memory.setregister('a',c.x);memory.setregister('y',c.y)
 end
 local sp=memory.getregister('s');local pc=word(0x100+AND(sp+1,255))+1
 pending={index=ordinal,frame=emu.framecount(),x=memory.getregister('a'),y=memory.getregister('y'),code=read(0x10),pc=pc,sp=AND(sp+2,255),before=state(),restore=restore}
 arm(pc)
end)
if cases then
 memory.registerexec(0xe54f,1,function()
  -- One native weapon-box drawing request per frame. Setup is explicit and
  -- occurs before its original AI; no requested output bytes are injected.
  for i=0,15 do memory.writebyte(0x4b8+i,0)end
  local i=15
  memory.writebyte(0x528+i,2);memory.writebyte(0x4b8+i,3);memory.writebyte(0x568+i,0)
  memory.writebyte(0x538+i,1);memory.writebyte(0x5c8+i,0)
  memory.writebyte(0x33e+i,180);memory.writebyte(0x324+i,160)
  memory.writebyte(0x358+i,0);memory.writebyte(0x598+i,0x8f)
  memory.writebyte(0x578+i,0xf0);memory.writebyte(0x30a+i,1)
 end)
end
function finish_probe()assert(not pending,'unfinished graphics request');records:close();if cases then assert(ordinal==#cases,'missing controlled request')end end
'''

def main():
    mode=sys.argv[1] if len(sys.argv)>1 else 'natural'
    label=sys.argv[2] if len(sys.argv)>2 else 'v1'
    abi=len(sys.argv)>3 and sys.argv[3]=='abi'
    source=ROOT/'.local/contra/states/coop-play.fcs';target=BASE/'states/coop-play.fcs';target.parent.mkdir(parents=True,exist_ok=True)
    if target.exists():assert target.read_bytes()==source.read_bytes()
    else:shutil.copyfile(source,target)
    cases=[]
    if mode=='controlled':
        for code in [0,1,5,37,47,0x80,0xa5]:
            for x,y in [(0,16),(16,16),(0,32),(16,32)]:
                for sx in [0,9,244]:
                    for used in [0,31,63,64,255]:
                        cases.append(dict(code=code,x=x,y=y,sx=sx,sy=224,used=used,mode=255 if used==31 else 1,control=0xb0))
        spec={'name':'graphics-controlled-'+label,'load_state':'coop-play','frames':len(cases),'captures':[],'segments':[],'timeout_seconds':90}
        if abi:
            for c in cases:c['lookup']=(c['code']+c['used']+17)&255
        lua='local cases={'+','.join('{'+','.join(f'{k}={v}'for k,v in c.items())+'}'for c in cases)+'}'
    elif mode=='natural':
        original=ROOT/'.local/contra-weapons/pickup/runs/natural-machinegun-01/scenario.json'
        observed=json.loads(original.read_text())
        spec={k:observed[k]for k in ['load_state','frames','segments']}
        spec.update(name='graphics-natural-'+label,captures=[1,250,350,700],timeout_seconds=90)
        lua='local cases=nil'
    else:raise ValueError('mode must be natural or controlled')
    hook=HOOK
    if abi:
        hook=hook.replace('"address":%d,"queue"','"address":%d,"attributeLookup":%d,"queue"')
        hook=hook.replace('word(0x12),hex(0x700','word(0x12),read(0x0f),hex(0x700')
        hook=hook.replace('{0x21,0x23,0xfc,0xfd,0xff}','{0x21,0x23,0xfc,0xfd,0xff,0x0f}')
        hook=hook.replace('memory.writebyte(0x10,c.code);','memory.writebyte(0x0f,c.lookup);memory.writebyte(0x10,c.code);')
    spec['hooks_lua']=hook.replace('CASES',lua)
    path=BASE/(spec['name']+'.json');path.write_text(json.dumps(spec,indent=2)+'\n');out=run(path)
    records=[json.loads(line)for line in (out/'graphics-requests.jsonl').read_text().splitlines()]
    assert records
    for row in records:
        assert row['before']['supertiles']==row['after']['supertiles']
        assert row['before']['collision']==row['after']['collision']
        assert row['before']['mode']==row['after']['mode']
    result={'source_is_original_execution':True,'scope':mode,'cases':cases,
      'interventions':('At C3D1 boundary: explicit x/y/code/scroll/cursor setup; original function executes; queue/scroll setup restored after recording return. Weapon-box dispatcher is staged once per frame. Not natural gameplay.' if mode=='controlled' else 'none; natural dual-player input with M pickup'),
      'source':json.loads((out/'source.json').read_text()),'scenario':spec,'records':records}
    raw=json.dumps(result,separators=(',',':')).encode();packed=gzip.compress(raw,mtime=0)
    suffix='-abi' if abi else ''
    target=ROOT/'games/contra/engine/testdata'/f'original-graphics-requests-{mode}{suffix}.json.gz'
    if target.exists():
        old=json.loads(gzip.decompress(target.read_bytes()));assert old['records']==records,'repeat changed original results'
    else:target.write_bytes(packed)
    summary={'passed':True,'scope':mode,'records':len(records),'accepted':sum(r['carry']==0 for r in records),'rejected':sum(r['carry']!=0 for r in records),'fixture_sha256':hashlib.sha256(target.read_bytes()).hexdigest(),'ram_sha256':hashlib.sha256((out/'ram.bin').read_bytes()).hexdigest()}
    (out/'graphics-source.json').write_text(json.dumps(summary,indent=2)+'\n');print(json.dumps(summary))

if __name__=='__main__':main()
