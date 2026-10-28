"""Continuous original Boss calls after declared room/plate setup, no rule model."""
from pathlib import Path
import os,json,gzip,hashlib,shutil,sys
ROOT=Path(__file__).resolve().parents[2];BASE=ROOT/'.local/contra-enemies-base/boss-sequence'
os.environ['CONTRA_REFERENCE_ROOT']=str(BASE)
from capture_reference import run
from enemies_base_capture import HOOK,FIELDS,WORLD

def main():
 stage=int(sys.argv[1]);label=sys.argv[2]if len(sys.argv)>2 else 'v1';assert stage in[2,4]
 (BASE/'states').mkdir(parents=True,exist_ok=True);source=ROOT/f'.local/contra-campaign/states/stage-{stage}-v2.fcs';target=BASE/f'states/stage-{stage}.fcs'
 if not target.exists():shutil.copyfile(source,target)
 assert source.read_bytes()==target.read_bytes()
 setup="memory.writebyte(0x64,memory.readbyte(0x58)-1);memory.writebyte(0x6a,128);memory.writebyte(0xb0,255);memory.writebyte(0xb1,255);local activated=false;memory.registerexec(0xe54f,1,function()local found=false;for i=0,15 do if memory.readbyte(0x4b8+i)~=0 and memory.readbyte(0x528+i)==BOSSKIND then found=true end end;if not activated and memory.readbyte(0x40)==128 and found then activated=true;memory.writebyte(0x87,PLATES);writefile('activation.json',string.format('{\"frame\":%d,\"plates\":%d}',emu.framecount(),PLATES))end end)".replace('PLATES',str(4 if stage==2 else 3)).replace('BOSSKIND',str(16 if stage==2 else 28))
 spec=dict(name=f'base-boss-sequence-{stage}-{label}',load_state=f'stage-{stage}',frames=1800,segments=[],captures=[1,200,600,1800],hooks_lua=HOOK.replace('FIELDS','{'+','.join(hex(a)for a in FIELDS)+'}').replace('WORLD','{'+','.join(hex(a)for _,a in WORLD)+'}').replace('SETUP',setup),timeout_seconds=180)
 path=BASE/(spec['name']+'.json');path.write_text(json.dumps(spec,indent=2)+'\n');out=run(path);rows=[json.loads(x)for x in(out/'base-enemies.jsonl').read_text().splitlines()]
 selected=[r for r in rows if r['name']=='step'and r['type']in[16,27,28,29]]
 data=dict(source_is_original_execution=True,scope='one initial room/clear/barrier override and one plate-counter override on Boss entry; thereafter original execution with recorded controller inputs; no enemy or projectile states patched',source=json.loads((out/'source.json').read_text()),scenario=spec,activation=json.loads((out/'activation.json').read_text()),world_fields=[dict(name=k,address=hex(a))for k,a in WORLD],records=selected,input_sha256=hashlib.sha256((out/'inputs.csv').read_bytes()).hexdigest(),ram_sha256=hashlib.sha256((out/'ram.bin').read_bytes()).hexdigest())
 dest=ROOT/f'docs/research/nes-contra/evidence/enemies-base-boss-sequence-{stage}.json.gz';packed=gzip.compress(json.dumps(data,separators=(',',':')).encode(),mtime=0)
 if dest.exists():
  old=json.loads(gzip.decompress(dest.read_bytes()));assert old['records']==selected,'repeat differs'
 else:dest.write_bytes(packed)
 print(json.dumps(dict(stage=stage,records=len(selected),phases={str(k):sorted({r['before']['pool'][r['slot']][4]for r in selected if r['type']==k})for k in[16,27,28,29]})))
if __name__=='__main__':main()
