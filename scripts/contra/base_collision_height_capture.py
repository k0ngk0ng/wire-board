"""Complete native E54F around the actual Base2 roller creation; no RAM writes."""
from pathlib import Path
import copy,gzip,hashlib,json,os,shutil,sys
ROOT=Path(__file__).resolve().parents[2];BASE=ROOT/'.local/contra-base-roller-live'
os.environ['CONTRA_REFERENCE_ROOT']=str(BASE)
from capture_reference import run
from enemies_all_tick_capture import PLAYER,ENEMY,WORLD,HOOK

def main():
    label=sys.argv[1]if len(sys.argv)>1 else'v1'
    spec=json.loads((ROOT/'.local/contra-world-eight-oracle/runs/world-eight-stage-4-v2/scenario.json').read_text())
    spec['name']='roller-collision-height-'+label;spec['frames']=24;spec['captures']=[]
    spec['segments']=[dict(s,end=min(s['end'],24))for s in spec['segments']if s['start']<=24]
    wf=WORLD+[('inheritedThrowAttribute',0x0a)]
    hook=HOOK.replace('PLAYERS','{'+','.join(hex(a)for _,a in PLAYER)+'}').replace('ENEMIES','{'+','.join(hex(a)for _,a in ENEMY)+'}').replace('WORLD','{'+','.join(hex(a)for _,a in wf)+'}').replace('CAPTURE_BIAS','true')
    hook=hook.replace('memory.registerexec(0xe54f,1,function()\n','memory.registerexec(0xe54f,1,function()\n if emu.framecount()<1408 or emu.framecount()>1412 then return end\n')
    spec['hooks_lua']+='\nlocal finish_world_probe=finish_probe\n'+hook+'\nlocal finish_enemy_probe=finish_probe\nfunction finish_probe()finish_enemy_probe();finish_world_probe()end\n'
    path=BASE/(spec['name']+'.json');path.write_text(json.dumps(spec,indent=2)+'\n');out=run(path)
    rows=[json.loads(l)for l in(out/'enemy-ticks.jsonl').read_text().splitlines()];assert len(rows)==5
    data={'source_is_original_execution':True,'scope':'roller-height','interventions':__doc__,'source':json.loads((out/'source.json').read_text()),'scenario':spec,
          'player_fields':[{'name':n,'address':hex(a)}for n,a in PLAYER],'enemy_fields':[{'name':n,'address':hex(a)}for n,a in ENEMY],'world_fields':[{'name':n,'address':hex(a)}for n,a in wf],'records':rows}
    target=ROOT/'games/contra/engine/testdata/original-enemy-collision-tick-roller-height.json.gz'
    if target.exists():assert json.loads(gzip.decompress(target.read_bytes()))['records']==rows
    else:target.write_bytes(gzip.compress(json.dumps(data,separators=(',',':')).encode(),mtime=0))
    print(json.dumps({'phases':len(rows),'sha256':hashlib.sha256(target.read_bytes()).hexdigest()}))

if __name__=='__main__':main()
