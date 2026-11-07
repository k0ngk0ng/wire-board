"""Continue the established original Boss route through the second-stage entry.

Only the inherited first-C001 reserve-lives=30 intervention is retained. Boss
HP, death, scenes, reset and progression are never patched. Observation uses
matching C001/F8DB stack depth so nested NMI cannot truncate direct PPU work.
"""
import os,gzip,hashlib,json,shutil,argparse
from pathlib import Path
ROOT=Path(__file__).resolve().parents[2]
os.environ['CONTRA_REFERENCE_ROOT']=str(ROOT/'.local/contra-world-interlude/oracle')
from capture_reference import BASE,run

def main():
 p=argparse.ArgumentParser();p.add_argument('--label',required=True);p.add_argument('--export',action='store_true');a=p.parse_args()
 original=json.loads(gzip.decompress((ROOT/'docs/research/nes-contra/evidence/world-boss-original.json.gz').read_bytes()))
 spec=dict(original['scenario']);spec['name']='world-interlude-'+a.label;spec['timeout_seconds']=180;spec['captures']=[1,6000,6475,6800,7200,7800,8200,9000]
 hook=spec['hooks_lua'];old='if memory.readbyte(0x18)~=5 or memory.readbyte(0x30)~=0 or (routine~=4 and routine~=8 and routine~=9 and routine~=11) then return end';assert old in hook
 hook=hook.replace(old,"if memory.readbyte(0x18)~=5 or memory.readbyte(0x30)>1 or secondBattle>=120 then return end\n if memory.readbyte(0x30)==1 and routine==4 then secondBattle=secondBattle+1 end")
 hook=hook.replace('local pending=nil;local ordinal=0',r'''
local ppuout=assert(io.open(out..'/world-ppu.bin','wb'))
local function ppustate()
 local bytes={};for a=0x2000,0x27ff do bytes[#bytes+1]=string.char(ppu.readbyte(a))end
 for a=0x3f00,0x3f1f do bytes[#bytes+1]=string.char(ppu.readbyte(a))end
 return table.concat(bytes)
end
local secondBattle=0
local pending=nil;local ordinal=0''')
 hook=hook.replace('before=memory.readbyterange(0,2048),commands={}','before=memory.readbyterange(0,2048),beforePPU=ppustate(),sp=AND(memory.getregister("s")+2,255),commands={}')
 hook=hook.replace('if not pending then return end','if not pending or memory.getregister("s")~=pending.sp then return end')
 hook=hook.replace('local p=pending;pending=nil','local p=pending;pending=nil;ppuout:write(p.beforePPU);ppuout:write(ppustate())')
 hook=hook.replace('records:close();state:close()','records:close();state:close();ppuout:close();assert(secondBattle==120,"second battle was not reached")')
 spec['hooks_lua']=hook
 (BASE/'states').mkdir(parents=True,exist_ok=True);src=ROOT/'.local/contra/states/coop-play.fcs';dst=BASE/'states/coop-play.fcs'
 if not dst.exists():shutil.copyfile(src,dst)
 assert src.read_bytes()==dst.read_bytes();path=BASE/(spec['name']+'.json');path.write_text(json.dumps(spec,indent=2)+'\n');out=BASE/'runs'/spec['name'] if a.export else run(path)
 assert json.loads((out/'scenario.json').read_text())==spec;assert(out/'capture-complete.json').is_file()
 rows=[json.loads(l)for l in (out/'world-frames.jsonl').read_text().splitlines()];ram=(out/'world-frames.bin').read_bytes();ppu=(out/'world-ppu.bin').read_bytes();assert len(ram)==len(rows)*4096 and len(ppu)==len(rows)*4160
 phases={}
 for i,row in enumerate(rows):
  b=ram[i*4096:i*4096+2048];after=ram[i*4096+2048:(i+1)*4096];key=f'{b[0x30]+1}:{b[0x2c]}';phases[key]=phases.get(key,0)+1
  row.update(before=b.hex(),after=after.hex(),beforePPU=ppu[i*4160:i*4160+2080].hex(),afterPPU=ppu[i*4160+2080:(i+1)*4160].hex())
 assert phases.get('1:10',0)>0 and phases.get('1:5',0)==1 and phases.get('2:0',0)==1 and phases.get('2:1',0)>0 and phases.get('2:2',0)==1 and phases.get('2:3',0)==32 and phases.get('2:4',0)==120
 # The new passive hooks must leave the entire established original route intact.
 unchanged=hashlib.sha256((out/'ram.bin').read_bytes()).hexdigest()==original['ramSHA256'];assert unchanged,'observation changed the established original route'
 assert [{k:r[k]for k in ('before','after','commands')}for r in rows[:len(original['frames'])]]==[{k:r[k]for k in ('before','after','commands')}for r in original['frames']]
 payload={'source_is_original_execution':True,'scope':'Inherited one-time reserve30 start, then original Boss defeat, exit, interlude, reset and second-stage entry using identical controller route; no after-state or outcome intervention.','source':json.loads((out/'source.json').read_text()),'scenario':spec,'interventions':json.loads((out/'interventions.json').read_text()),'ramSHA256':hashlib.sha256((out/'ram.bin').read_bytes()).hexdigest(),'inputTraceSHA256':hashlib.sha256((out/'inputs.csv').read_bytes()).hexdigest(),'originalBossRouteUnchanged':unchanged,'originalBossPrefixFrames':len(original['frames']),'phases':phases,'frames':rows}
 target=ROOT/'docs/research/nes-contra/evidence/world-interlude-original.json.gz'
 if target.exists():
  previous=json.loads(gzip.decompress(target.read_bytes()));assert previous['frames']==rows and previous['ramSHA256']==payload['ramSHA256']
 else:target.write_bytes(gzip.compress(json.dumps(payload,separators=(',',':')).encode(),mtime=0))
 print(json.dumps({'frames':len(rows),'phases':phases,'sourceUnchanged':unchanged,'fixtureSHA256':hashlib.sha256(target.read_bytes()).hexdigest()}))
if __name__=='__main__':main()
