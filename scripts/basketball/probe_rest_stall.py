"""Probe original ball dispatch against actual Go team5 suspension approach.

These are isolated original calls initialized from retained Go world values,
not claims of a native ROM full match. Also run a no-reinjection ball chain.
"""
import argparse,json,hashlib
from pathlib import Path
from verify_ball18 import ROOT,ROM,PRG,BASE,SoundMemory
from verify_pass_floor_original import MPU,put
from extract_world_actions import decode_world_action_state

def encode(s):
 r=bytearray(BASE);a=s['playing']['world']['Preparation']['Actions'];b=a['ball'];p=b['body']['position'];v=b['body']['velocity'];anim=b['animation']
 fields={0x58b:s['config']['match']['Court'],0x3dc:8,0x518:b['action']|(128 if b['entered'] else 0),0x525:b['request']['action']|(0 if b['request']['pending'] else 128),0x5fd:128 if b['airborne'] else 0,0x648:a['ballLink'],0x53f:a['Holder']&255,0x471:128 if b['facingLeft'] else 0,0x61:b['pose']['frame']|(128 if b['pose']['mirrored'] else 0),0x43d:anim['elapsed'],0x44a:anim['cursor'],0x457:anim['loops'],0x532:anim['nextAction'] if anim['hasNextAction'] else 255,0x511:b['bouncesRemaining'],0x564:b['deflection'],0x6d5:s['playing']['world']['Camera']['shake']}
 for x,value in fields.items():r[x]=value
 for value,addresses in [(p['x'],[0x6e,0x7b,0x88]),(p['depth'],[0x95,0xa2]),(p['height'],[0xaf,0xbc,0xc9]),(v['x'],[0x47e,0x48b]),(v['depth'],[0x498,0x4a5]),(v['height'],[0x4b2,0x4bf]),(b['body']['gravity'],[0x4cc,0x4d9])]:put(r,value,*addresses)
 return r

def run(m):
 cpu=MPU(memory=m,pc=0xe149);cpu.x=6;sounds=[]
 for step in range(10000):
  if cpu.pc==0xe14c:break
  if cpu.pc==0xfbcc:sounds.append(cpu.a)
  cpu.step()
 else:raise RuntimeError(hex(cpu.pc))
 return decode_world_action_state(bytes(m[:2048]))['ball'],sounds

def main():
 parser=argparse.ArgumentParser();parser.add_argument('--directory',default='.local/basketball/rea/ball-rest-stall');args=parser.parse_args()
 out=(ROOT/args.directory).resolve();assert out.is_relative_to(ROOT);rows=json.loads((out/'first-entry/history.json').read_text());calls=[]
 for row in rows:
  m=SoundMemory(PRG,encode(row['Before']));actual,sounds=run(m);want=row['After']['playing']['world']['Preparation']['Actions']['ball']
  calls.append(dict(video=row['Video'],match=actual==want,actual=actual,want=want,sounds=sounds))
 chain=[];m=SoundMemory(PRG,encode(rows[0]['Before']))
 for row in rows:
  actual,sounds=run(m);want=row['After']['playing']['world']['Preparation']['Actions']['ball'];chain.append(dict(video=row['Video'],match=actual==want,actual=actual,want=want,sounds=sounds))
 result=dict(scope='controlled original E149 calls from real Go world samples; continuous ball-only chain, no frame injection',romSHA256=hashlib.sha256(ROM).hexdigest(),calls=calls,chain=chain)
 (out/'original-approach.json').write_text(json.dumps(result,indent=2)+'\n');print('calls',len(calls),'matches',sum(x['match'] for x in calls),'chain matches',sum(x['match'] for x in chain))
 for r in calls:
  if not r['match']:print(r['video'],r['actual'],r['want']);break
if __name__=='__main__':main()
