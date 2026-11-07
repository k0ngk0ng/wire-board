"""Observe complete enemy phases around the parent route's real Boss kill.

The retained controller policy and sole initial reserve-life override are
unchanged. Neither Boss HP, enemy state nor outcome is injected here.
"""
from pathlib import Path
import ast,gzip,hashlib,json,os,shutil,sys
ROOT=Path(__file__).resolve().parents[2];BASE=ROOT/'.local/contra-enemies-natural-boss'
os.environ['CONTRA_REFERENCE_ROOT']=str(BASE)
from capture_reference import run

def literal(path,name):
    for n in ast.parse(path.read_text()).body:
        if isinstance(n,ast.Assign)and any(isinstance(t,ast.Name)and t.id==name for t in n.targets):return ast.literal_eval(n.value)
    raise ValueError(name)

def main():
    label=sys.argv[1]if len(sys.argv)>1 else'v1'
    source=ROOT/'.local/contra-world/campaign-oracle/runs/campaign-world-boss-v1/scenario.json'
    spec=json.loads(source.read_text());spec['name']='enemy-real-boss-'+label;spec['frames']=6030;spec['captures']=[]
    pf=literal(ROOT/'scripts/contra/static_capture_playerstep.py','PLAYER_FIELDS');ef=literal(ROOT/'scripts/contra/enemies_capture.py','FIELDS')
    path=ROOT/'scripts/contra/enemies_collision_tick_capture.py';wf=literal(path,'WORLD')+[('delayLow',0x2a),('delayHigh',0x2b),('generationBias',6)]
    hook=literal(path,'HOOK').replace('PLAYERS','{'+','.join(hex(a)for _,a in pf)+'}').replace('ENEMIES','{'+','.join(hex(a)for _,a in ef)+'}').replace('WORLD','{'+','.join(hex(a)for _,a in wf)+'}').replace('CAPTURE_BIAS','true')
    hook=hook.replace("memory.registerexec(0xe54f,1,function()\n", "memory.registerexec(0xe54f,1,function()\n if emu.framecount()<7378 or emu.framecount()>7420 then return end\n")
    old=spec['hooks_lua']
    # The second recorder owns shared F9BC/F9D4 observers in this private copy;
    # only E54F fixture output is consumed, not the copied world's sound log.
    spec['hooks_lua']=old+'\nlocal finish_world_probe=finish_probe\n'+hook+'\nlocal finish_enemy_probe=finish_probe\nfunction finish_probe()finish_enemy_probe();finish_world_probe()end\n'
    (BASE/'states').mkdir(parents=True,exist_ok=True);start=ROOT/'.local/contra/states/coop-play.fcs';dest=BASE/'states/coop-play.fcs'
    if dest.exists():assert start.read_bytes()==dest.read_bytes()
    else:shutil.copyfile(start,dest)
    path=BASE/(spec['name']+'.json');path.write_text(json.dumps(spec,indent=2)+'\n');out=run(path)
    rows=[json.loads(l)for l in(out/'enemy-ticks.jsonl').read_text().splitlines()];assert rows
    assert any(r['frame']==7385 and r.get('audioReset')for r in rows)
    data={'source_is_original_execution':True,'scope':'natural-boss','interventions':__doc__,'source':json.loads((out/'source.json').read_text()),'scenario':spec,
          'player_fields':[{'name':n,'address':hex(a)}for n,a in pf],'enemy_fields':[{'name':n,'address':hex(a)}for n,a in ef],'world_fields':[{'name':n,'address':hex(a)}for n,a in wf],'records':rows}
    target=ROOT/'games/contra/engine/testdata/original-enemy-collision-tick-natural-boss.json.gz'
    if target.exists():assert json.loads(gzip.decompress(target.read_bytes()))['records']==rows
    else:target.write_bytes(gzip.compress(json.dumps(data,separators=(',',':')).encode(),mtime=0))
    print(json.dumps({'phases':len(rows),'entityCalls':sum(len(r['slots'])for r in rows),'sha256':hashlib.sha256(target.read_bytes()).hexdigest()}))

if __name__=='__main__':main()
