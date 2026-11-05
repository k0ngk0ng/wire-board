import json,subprocess,sys
from pathlib import Path
root=Path.cwd();folder=root/'.local/md-yuyuhakusho/fighter6-ai-r4-search'
base=json.loads((root/'games/yuyuhakusho/engine/testdata/fighter6-ai-inputs.json').read_text())
def e(start,end,buttons):return {'start':start,'end':end,'buttons':buttons}
patterns={
 'walk-jump':[e(2250,2290,[7])]+[e(s,s+4,[4]) for s in [2350,2500,2650,2800,2950,3100,3250,3400]],
 'rise-repeat':[e(2250,2280,[7])]+[e(s+d,s+d+3,[4]) for s in [2350,2650,2950,3250] for d in [0,12]],
 'air-throw':[e(2250,2290,[7])]+[v for s in [2350,2600,2850,3100,3350] for v in [e(s,s+4,[4]),e(s+18,s+22,[7,1])]],
 'air-b':[e(2250,2290,[7])]+[v for s in [2350,2600,2850,3100,3350] for v in [e(s,s+4,[4]),e(s+18,s+22,[7,0])]],
 'guard': [e(2300,3550,[8])],
 'jump-retreat':[e(2250,2290,[7])]+[v for s in [2350,2650,2950,3250] for v in [e(s,s+30,[6]),e(s,s+4,[4]),e(s+60,s+90,[7])]],
 'sweep-pulses':[e(2250,2290,[7])]+[e(s,s+3,[5,0]) for s in range(2350,3500,45)],
 'throw-pulses':[e(2250,2290,[7])]+[e(s,s+3,[7,1]) for s in range(2350,3500,55)],
}
summary=[]
for name,extra in patterns.items():
 out=folder/name;out.mkdir(exist_ok=True);inputs=out/'inputs.json'
 inputs.write_text(json.dumps(sorted(base+extra,key=lambda x:x['start']),indent=2)+'\n')
 with (out/'stdout.log').open('w') as log:
  rc=subprocess.run([sys.executable,'scripts/yuyuhakusho/capture_fighter6_natural_ai.py','--inputs',str(inputs),'--out',str(out)],stdout=log,stderr=subprocess.STDOUT).returncode
 if rc:raise RuntimeError((name,rc,(out/'stdout.log').read_text()[-2000:]))
 r=json.loads((out/'report.json').read_text())
 row={k:r[k] for k in ['input','complete_ai_returns','source6_character_decisions','complete_air_special_returns','computer6_actions','natural34_complete_selections','natural34_frames','errors']};row['pattern']=name;row['extra_events']=extra;summary.append(row)
 (folder/'summary.json').write_text(json.dumps(summary,indent=2)+'\n')
 print(name,row['complete_ai_returns'],row['natural34_complete_selections'],row['natural34_frames'],flush=True)
 if row['natural34_complete_selections']:break
