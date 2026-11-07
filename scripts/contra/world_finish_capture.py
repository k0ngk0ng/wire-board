"""Original final transition boundary, explicitly controlled before C001.

This does not claim a campaign clear: it verifies the final caller contract
after the already-recovered exit/interlude have selected the transition.
"""
import gzip,hashlib,json,os,shutil,sys
from pathlib import Path
ROOT=Path(__file__).resolve().parents[2]
os.environ['CONTRA_REFERENCE_ROOT']=str(ROOT/'.local/contra-world/finish-oracle')
from world_capture_complete import HOOK
from capture_reference import BASE,run

def main():
 label=sys.argv[1] if len(sys.argv)>1 else 'v1'
 (BASE/'states').mkdir(parents=True,exist_ok=True)
 src=ROOT/'.local/contra-campaign/states/stage-8-v2.fcs';dst=BASE/'states/stage-eight.fcs'
 if dst.exists():assert dst.read_bytes()==src.read_bytes()
 else:shutil.copyfile(src,dst)
 hook=HOOK.replace('local pending=nil;local ordinal=0','local pending=nil;local ordinal=0;local staged=false')
 hook=hook.replace(' if memory.readbyte(0x18)~=5 or memory.readbyte(0x2c)~=4 then return end',r'''
 if staged then return end;staged=true
 local values={[0x2c]=5,[0x2d]=0,[0x3b]=2,[0x23]=0,[0x7eb]=0}
 local edits={};for a,v in pairs(values)do edits[#edits+1]=string.format('{"address":%d,"before":%d,"after":%d}',a,memory.readbyte(a),v);memory.writebyte(a,v)end
 writefile('interventions.json','['..table.concat(edits,',')..']')''')
 hook=hook.replace('before=memory.readbyterange(0,2048),commands={}','before=memory.readbyterange(0,2048),sp=AND(memory.getregister("s")+2,255),commands={}')
 hook=hook.replace('if not pending then return end','if not pending or memory.getregister("s")~=pending.sp then return end')
 spec={'name':'world-finish-'+label,'load_state':'stage-eight','frames':3,'segments':[],'captures':[1,3],'hooks_lua':hook,'timeout_seconds':90}
 path=BASE/(spec['name']+'.json');path.write_text(json.dumps(spec,indent=2)+'\n');out=run(path)
 rows=[json.loads(line)for line in(out/'world-frames.jsonl').read_text().splitlines()];states=(out/'world-frames.bin').read_bytes();assert len(rows)==1 and len(states)==4096
 rows[0]['before']=states[:2048].hex();rows[0]['after']=states[2048:].hex()
 assert states[2048+0x18]==6 and states[2048+0x30]==8
 data={'source_is_original_execution':True,'scope':__doc__,'source':json.loads((out/'source.json').read_text()),'scenario':spec,'initialStateSHA256':hashlib.sha256(src.read_bytes()).hexdigest(),'interventions':json.loads((out/'interventions.json').read_text()),'frames':rows}
 target=ROOT/'docs/research/nes-contra/evidence/world-finish-original.json.gz'
 if target.exists():assert json.loads(gzip.decompress(target.read_bytes()))['frames']==rows
 else:target.write_bytes(gzip.compress(json.dumps(data,separators=(',',':')).encode(),mtime=0))
 print(json.dumps({'frames':len(rows),'sha256':hashlib.sha256(target.read_bytes()).hexdigest()}))
if __name__=='__main__':main()
