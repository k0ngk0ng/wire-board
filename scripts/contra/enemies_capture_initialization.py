"""Execute real EDAD for all Japanese level/kind properties under declared setup."""
from pathlib import Path
import gzip,hashlib,json,os,shutil,sys
from enemies_all_data import ROOT,COUNTS,inventory

BASE=ROOT/'.local/contra-enemies-all'
os.environ['CONTRA_REFERENCE_ROOT']=str(BASE)
from capture_reference import run
FIELDS=[0x30a,0x324,0x33e,0x358,0x4b8,0x4c8,0x4d8,0x4e8,0x4f8,0x508,0x518,0x528,0x538,0x548,0x558,0x568,0x578,0x588,0x598,0x5a8,0x5b8,0x5c8,0x5d8,0x5e8]
HOOK=r'''
local f=assert(io.open(out..'/initializations.jsonl','w'));local fields=FIELDS;local cases=CASES
local index=0;local pending=nil
local function read(a)return memory.readbyte(a)end
local function ret()local s=memory.getregister('s');return read(0x100+AND(s+1,255))+256*read(0x100+AND(s+2,255))+1 end
local function pool()local t={};for slot=0,15 do local v={};for _,a in ipairs(fields)do v[#v+1]=tostring(read(a+slot))end;t[#t+1]='['..table.concat(v,',')..']'end;return '['..table.concat(t,',')..']'end
memory.registerexec(0xe54f,1,function()
 index=index+1;assert(cases[index])
 for slot=0,15 do for _,a in ipairs(fields)do memory.writebyte(a+slot,0)end end
 local s=15;memory.writebyte(0x528+s,6);memory.writebyte(0x4b8+s,3);memory.writebyte(0x30a+s,0x43)
 memory.writebyte(0x33e+s,180);memory.writebyte(0x324+s,100);memory.writebyte(0x558+s,1);memory.writebyte(0x568+s,3);memory.writebyte(0x5e8+s,1);memory.writebyte(0x598+s,0x80)
 memory.writebyte(0x30,0);memory.writebyte(0x40,0);memory.writebyte(0x41,0);memory.writebyte(0x68,0);memory.writebyte(0x8e,1)
 for p=0,1 do memory.writebyte(0x90+p,1);memory.writebyte(0xae+p,255);memory.writebyte(0x38+p,0)end
end)
memory.registerexec(0xedad,1,function()
 if ret()~=0xf257 then return end
 assert(not pending);local c=cases[index];local restore={}
 for slot=0,15 do for _,a in ipairs(fields)do restore[a+slot]=read(a+slot)end end
 restore[0x30]=read(0x30)
 local actor=memory.getregister('x')
 for slot=0,15 do for n,a in ipairs(fields)do memory.writebyte(a+slot,AND(slot*17+n*13+index*7,255))end end
 memory.writebyte(0x528+c.slot,c.kind);memory.writebyte(0x30,c.level);memory.setregister('x',c.slot)
 pending={restore=restore,actor=actor,before=pool(),case=index}
end)
memory.registerexec(0xf257,1,function()
 if not pending then return end
 local p=pending;pending=nil
 f:write(string.format('{"case":%d,"before":%s,"after":%s}\n',p.case,p.before,pool()))
 for a,v in pairs(p.restore)do memory.writebyte(a,v)end
 memory.setregister('x',p.actor)
end)
function finish_probe()assert(not pending);f:close()end
'''

def main():
    label=sys.argv[1]if len(sys.argv)>1 else 'v1'
    cases=[{'level':level,'kind':kind,'slot':slot}for level,count in enumerate(COUNTS)for kind in range(16+count)for slot in [0,7,15]]
    (BASE/'states').mkdir(parents=True,exist_ok=True);source=ROOT/'.local/contra/states/coop-play.fcs';target=BASE/'states/coop-play.fcs'
    if target.exists():assert target.read_bytes()==source.read_bytes()
    else:shutil.copyfile(source,target)
    hook=HOOK.replace('FIELDS','{'+','.join(hex(a)for a in FIELDS)+'}').replace('CASES','{'+','.join('{'+','.join(f'{k}={v}'for k,v in c.items())+'}'for c in cases)+'}')
    spec={'name':'all-enemy-initialization-'+label,'load_state':'coop-play','frames':len(cases),'segments':[],'captures':[],'hooks_lua':hook,'timeout_seconds':90}
    path=BASE/(spec['name']+'.json')
    if '--reuse' in sys.argv[2:]:
        out=BASE/'runs'/spec['name']
        assert json.loads((out/'scenario.json').read_text())==spec
        assert json.loads((out/'capture-complete.json').read_text())=={'status':'complete'}
    else:
        path.write_text(json.dumps(spec,indent=2)+'\n');out=run(path)
    rows=[json.loads(l)for l in (out/'initializations.jsonl').read_text().splitlines()];assert len(rows)==len(cases)
    data={'source_is_original_execution':True,'interventions':'At the actual F225->EDAD boundary, only level/kind/slot and poisoned entity fields are set; output is recorded then original caller state restored. Not natural gameplay.',
          'source':json.loads((out/'source.json').read_text()),'scenario':spec,'cases':cases,'records':rows,'inventory':inventory()}
    packed=gzip.compress(json.dumps(data,separators=(',',':')).encode(),mtime=0);dest=ROOT/'games/contra/engine/testdata/original-enemy-all-initialization.json.gz'
    if dest.exists():old=json.loads(gzip.decompress(dest.read_bytes()));assert old['cases']==cases and old['records']==rows
    else:dest.write_bytes(packed)
    print(json.dumps({'cases':len(rows),'sha256':hashlib.sha256(dest.read_bytes()).hexdigest()}))
if __name__=='__main__':main()
