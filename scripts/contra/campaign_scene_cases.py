"""Explicit original exit-function input matrix, never natural campaign evidence."""
import os
from pathlib import Path
ROOT=Path(__file__).resolve().parents[2]
os.environ.setdefault('CONTRA_REFERENCE_ROOT',str(ROOT/'.local/contra-campaign-scene/oracle'))
from capture_reference import BASE,run
from campaign_scene_capture import HOOK
import argparse,gzip,json,shutil

def cases():
 result=[]
 for stage in range(8):
  base={0x18:5,0x2c:9,0x2d:0,0x30:stage,0x3b:1,0x190:0,0x191:240,0x192:1,0x193:1,0x194:2,0x25:0,0x38:0,0x39:0,0x90:1,0x91:1,0xa0:0,0xa1:0,0xc6:0,0xc7:0,0x334:128,0x335:128,0x31a:160,0x31b:160,0xf1:255,0xf2:255,0xf5:255,0xf6:255}
  def add(v):r=dict(base);r.update(v);result.append(r)
  for dead in range(4):
   for jump in [0,1]:add({0x38:dead&1,0x39:dead>>1,0xa0:jump})
  for delay in [0,1]:
   for frame in [20,21]:
    for budget in [0,1,2]:add({0x2d:1,0x190:delay,0x1a:frame,0x191:budget})
  for script in [1,2,3,4]:
   for x,y,jump,vy in [(3,160,0,0),(4,160,0,0),(12,160,0,0),(13,160,0,0),(127,160,0,0),(128,160,0,0),(136,160,0,0),(144,160,0,0),(152,160,0,0),(184,160,0,0),(208,160,0,0),(248,160,0,0),(128,7,0,0),(128,176,1,0),(128,176,1,255)]:add({0x2d:1,0x192:script,0x193:script,0x334:x,0x335:x,0x31a:y,0x31b:y,0xa0:jump,0xa1:jump,0xc6:vy,0xc7:vy})
  for delay in [0,1,2]:add({0x2d:2,0x190:delay})
  for frame in [0,15,16]:
   for fade in [0,1,6]:add({0x2d:3,0x1a:frame,0xfb:fade})
 return result
def intro_cases():
 result=[]
 for stage in range(8):
  for mode in [0,1]:
   for lives,dead in [(0,0),(0,1),(2,0),(99,0),(128,0),(255,0)]:
    for score in [0,1,65535]:
     result.append({0x18:5,0x2c:1,0x2d:1,0x30:stage,0x22:mode,0x23:0,0x1c:0,0x25:0,0x32:lives,0x33:lives,0x38:dead,0x39:dead,0x7e0:score&255,0x7e1:score>>8,0x7e2:score&255,0x7e3:score>>8,0x7e4:score&255,0x7e5:score>>8,0xf1:0,0xf2:0,0xf5:0,0xf6:0})
 for stage in [0,5]:
  for phase in range(11):
   for demo,held,pressed in [(0,16,16),(1,0,0)]:result.append({0x18:5,0x2c:1,0x2d:phase,0x30:stage,0x23:0,0x1c:demo,0x25:0,0xf1:held,0xf5:pressed})
 return result

STAGING=r'''
local cases=CASES;local current=0
on(0xcc42,function()
 current=current+1
 if current>#cases then memory.writebyte(0x2c,4);memory.writebyte(0x25,1);return end
 for a,v in pairs(cases[current])do memory.writebyte(a,v)end
 for i=0,15 do memory.writebyte(0x4b8+i,0)end
end)
'''
def main():
 p=argparse.ArgumentParser();p.add_argument('--name',required=True);p.add_argument('--kind',choices=['exit','intro'],default='exit');p.add_argument('--export',action='store_true');a=p.parse_args();rows=cases() if a.kind=='exit' else intro_cases()
 (BASE/'states').mkdir(parents=True,exist_ok=True);src=ROOT/'.local/contra/states/coop-play.fcs';target=BASE/'states/coop-play.fcs'
 if not target.exists():shutil.copyfile(src,target)
 assert src.read_bytes()==target.read_bytes()
 lua='{'+','.join('{'+','.join(f'[{a}]={v}'for a,v in r.items())+'}'for r in rows)+'}'
 spec={'name':a.name,'frames':len(rows)*(3 if a.kind=='intro' else 1)+4,'load_state':'coop-play','segments':[],'captures':[],'timeout_seconds':120,'hooks_lua':HOOK.replace('STAGING',STAGING.replace('CASES',lua))}
 path=BASE/(a.name+'.json');path.write_text(json.dumps(spec,indent=2)+'\n');out=BASE/'runs'/a.name if a.export else run(path);assert json.loads((out/'scenario.json').read_text())==spec;assert (out/'capture-complete.json').is_file();stream=out/'scenes.jsonl';stream_text=stream.read_text() if stream.exists() else gzip.decompress(stream.with_suffix('.jsonl.gz').read_bytes()).decode();records=[json.loads(l)for l in stream_text.splitlines()];assert len(records)==len(rows)
 payload={'scope':'Each original call receives explicitly declared test input; no after-state is injected. Not continuous traversal or a natural Boss victory.','source':json.loads((out/'source.json').read_text()),'cases':rows,'scenario':spec,'records':records}
 (ROOT/('docs/research/nes-contra/evidence/campaign-scene-'+a.kind+'-cases-original.json.gz')).write_bytes(gzip.compress(json.dumps(payload,separators=(',',':')).encode(),mtime=0));print(json.dumps({'cases':len(rows),'records':len(records)}))
if __name__=='__main__':main()
