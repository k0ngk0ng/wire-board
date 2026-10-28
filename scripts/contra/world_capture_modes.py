"""Record complete indoor/vertical player-projectile phases for integration.

Starts are the previously recorded controlled stage entries. Boss setup repeats
the declared last-room/clear/invincibility setup, then lets the original perform
its scene transition. No output states are fed back into independent rules.
"""
import gzip,hashlib,json,os,shutil,sys
from pathlib import Path
ROOT=Path(__file__).resolve().parents[2]
os.environ.setdefault('CONTRA_REFERENCE_ROOT',str(ROOT/'.local/contra-world/modes-oracle'))
from capture_reference import BASE,run
from world_capture import HOOK

# Per-function staged boundary cases are intentionally not whole-world runs.
# Pin this integration corpus rather than accidentally replaying future records
# which restore modified state at a function return and have no full RAM trace.
CASE_NAMES=('base2-controls','base2-progress','base4-controls','waterfall-climb','waterfall-drop','base2-boss')

def main():
 label=sys.argv[1] if len(sys.argv)>1 else 'v1'
 source_path=ROOT/'games/contra/engine/testdata/original-player-modes.json.gz'
 source=json.loads(gzip.decompress(source_path.read_bytes()))
 pfields=source['player_fields'];wfields=list(source['world_fields'])
 for name,address in [('weaponStrength','0x2f'),('vulnerablePlayer','0x8e'),('indoorScroll','0x73'),('screenCleared','0x6a'),('solidCollision','0x59')]:
  if not any(f['name']==name for f in wfields):wfields.append({'name':name,'address':address})
 lua=HOOK.replace('PLAYER_FIELDS','{'+','.join(f['address'] for f in pfields)+'}').replace('WORLD_FIELDS','{'+','.join(f['address'] for f in wfields)+'}')
 (BASE/'states').mkdir(parents=True,exist_ok=True)
 results=[]
 cases={case['name']:case for case in source['scenarios']}
 assert all(name in cases and 'ram_sha256' in cases[name] for name in CASE_NAMES)
 for name in CASE_NAMES:
  case=cases[name]
  original=case['scenario'];state=original['load_state']
  src=ROOT/'.local/contra-static/modes-oracle/states'/(state+'.fcs');dst=BASE/'states'/(state+'.fcs')
  assert hashlib.sha256(src.read_bytes()).hexdigest()==case['source']['initial_state']['sha256']
  if dst.exists():assert dst.read_bytes()==src.read_bytes()
  else:shutil.copyfile(src,dst)
  intervention=''
  if case['name']=='base2-boss':
   intervention='memory.writebyte(0x64,memory.readbyte(0x58)-1);memory.writebyte(0x6a,128);memory.writebyte(0xb0,255);memory.writebyte(0xb1,255)\n'
  spec={'name':f'frame-modes-{label}-{case["name"]}','load_state':state,'frames':original['frames'],
        'segments':original['segments'],'captures':[1,original['frames']], 'hooks_lua':intervention+lua,'timeout_seconds':90}
  path=BASE/(spec['name']+'.json');path.write_text(json.dumps(spec,indent=2)+'\n');observed=run(path)
  records=[json.loads(line) for line in (observed/'frame-phase.jsonl').read_text().splitlines()]
  assert records
  assert hashlib.sha256((observed/'ram.bin').read_bytes()).hexdigest()==case['ram_sha256'], 'phase hooks changed the original player-mode run'
  results.append({'name':case['name'],'source':json.loads((observed/'source.json').read_text()),'scenario':spec,
                  'initial_stage_source':case['initial_stage_source'],'additional_intervention':case['additional_intervention'],
                  'records':records,'ramSHA256':hashlib.sha256((observed/'ram.bin').read_bytes()).hexdigest()})
 data={'source_is_original_execution':True,'scope':'Controlled original stage starts plus declared Boss setup; complete D296 phases, not natural campaign completion.',
       'metadataSourceSHA256':hashlib.sha256(source_path.read_bytes()).hexdigest(),'player_fields':pfields,'world_fields':wfields,'scenarios':results}
 target=ROOT/'docs/research/nes-contra/evidence/world-modes-original.json.gz'
 if target.exists():
  old=json.loads(gzip.decompress(target.read_bytes()))
  for a,b in zip(old['scenarios'],results,strict=True):assert a['records']==b['records'] and a['ramSHA256']==b['ramSHA256']
 else:target.write_bytes(gzip.compress(json.dumps(data,separators=(',',':')).encode(),mtime=0))
 print(json.dumps({'frames':sum(len(s['records']) for s in results),'scenes':len(results),'sha256':hashlib.sha256(target.read_bytes()).hexdigest()}))

if __name__=='__main__':main()
