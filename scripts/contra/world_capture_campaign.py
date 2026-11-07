"""Capture whole-world campaign combat boundaries before their integration.

Loss scenario changes reserve lives once at the first C001 boundary, then uses
only ordinary original execution. It is a controlled zero-reserve challenge,
not a natural full campaign run. Every intervention is retained explicitly.
"""
import gzip, hashlib, json, os, shutil, sys
from pathlib import Path

ROOT=Path(__file__).resolve().parents[2]
os.environ['CONTRA_REFERENCE_ROOT']=str(ROOT/'.local/contra-world/campaign-oracle')
from world_capture_complete import HOOK
from capture_reference import BASE, run

def main():
    label=sys.argv[1] if len(sys.argv)>1 else 'loss-v1'
    mode=sys.argv[2] if len(sys.argv)>2 else 'loss'
    assert mode in ('loss','boss')
    reserves=30 if mode=='boss' else 0
    (BASE/'states').mkdir(parents=True,exist_ok=True)
    src=ROOT/'.local/contra/states/coop-play.fcs';dst=BASE/'states/coop-play.fcs'
    if dst.exists():assert src.read_bytes()==dst.read_bytes()
    else:shutil.copyfile(src,dst)
    hook=HOOK.replace('local pending=nil;local ordinal=0','local pending=nil;local ordinal=0;local staged=false')
    hook=hook.replace(' if memory.readbyte(0x18)~=5 or memory.readbyte(0x2c)~=4 then return end',r'''
 if not staged then
  staged=true
  local l0=memory.readbyte(0x32);local l1=memory.readbyte(0x33)
  memory.writebyte(0x32,RESERVES);memory.writebyte(0x33,RESERVES)
  writefile('interventions.json',string.format('[{"address":50,"before":%d,"after":RESERVES},{"address":51,"before":%d,"after":RESERVES}]',l0,l1))
 end
 local routine=memory.readbyte(0x2c)
 if memory.readbyte(0x18)~=5 or memory.readbyte(0x30)~=0 or (routine~=4 and routine~=8 and routine~=9 and routine~=11) then return end''').replace('RESERVES',str(reserves))
    frames=10000 if mode=='boss' else 3600
    segments=[{'start':1,'end':frames,'player':p,'keys':['right']} for p in (1,2)]
    if mode=='boss':
        opening=json.loads(gzip.decompress((ROOT/'games/contra/engine/testdata/original-enemy-shooters-natural.json.gz').read_bytes()))['scenario']
        segments=[dict(s,end=min(350,s['end'])) for s in opening['segments'] if s['start']<=350]
        hook+=r'''
function control_frame(n,pads)
 if n<=350 then return end
 for i=0,1 do
  local keys={A=false,B=false,up=false,down=false,left=false,right=true,start=false,select=false}
  local weapon=AND(memory.readbyte(0xaa+i),15)
  keys.B=(weapon==1 or weapon==4 or n%4==1)
  if memory.readbyte(0x90+i)==1 and memory.readbyte(0xa0+i)==0 and memory.readbyte(0xa4+i)==0 and memory.readbyte(0xb2+i)==0 then keys.A=true end
  if memory.readbyte(0x90+i)==3 and n%30==1 then keys.A=true end
  pads[i+1]=keys
 end
end
'''
    spec={'name':f'campaign-world-{label}','load_state':'coop-play','frames':frames,
          'captures':[1,600,1200,2400,3600],
          'segments':segments, 'hooks_lua':hook,'timeout_seconds':90}
    path=BASE/(spec['name']+'.json');path.write_text(json.dumps(spec,indent=2)+'\n');out=run(path)
    rows=[json.loads(line) for line in (out/'world-frames.jsonl').read_text().splitlines()]
    states=(out/'world-frames.bin').read_bytes();assert len(states)==len(rows)*4096
    for i,row in enumerate(rows):
        row['before']=states[i*4096:i*4096+2048].hex();row['after']=states[i*4096+2048:(i+1)*4096].hex()
    phases={}
    for row in rows:
        phase=bytes.fromhex(row['before'])[0x2c];phases[phase]=phases.get(phase,0)+1
    print(json.dumps({'observedPhases':phases,'mode':mode}))
    if mode=='loss':assert 11 in phases,'scenario did not reach original shared game-over wait'
    else:assert 8 in phases and 9 in phases,'scenario did not defeat original Boss and reach exit'
    data={'source_is_original_execution':True,'scope':f'Controlled {reserves} reserve lives, then controller-only inputs; no forced death, Boss HP or outcome.',
          'source':json.loads((out/'source.json').read_text()),'scenario':spec,
          'interventions':json.loads((out/'interventions.json').read_text()),
          'inputTraceSHA256':hashlib.sha256((out/'inputs.csv').read_bytes()).hexdigest(),
          'ramSHA256':hashlib.sha256((out/'ram.bin').read_bytes()).hexdigest(),'frames':rows,'phases':phases}
    target=ROOT/f'docs/research/nes-contra/evidence/world-{mode}-original.json.gz';repeated=target.exists()
    if repeated:
        old=json.loads(gzip.decompress(target.read_bytes()));assert old['frames']==rows and old['ramSHA256']==data['ramSHA256']
    else:target.write_bytes(gzip.compress(json.dumps(data,separators=(',',':')).encode(),mtime=0))
    print(json.dumps({'frames':len(rows),'phases':phases,'repeatedIdentically':repeated,'sha256':hashlib.sha256(target.read_bytes()).hexdigest()}))

if __name__=='__main__':main()
