#!/usr/bin/env python3
"""Summarize repeatable natural-resource simulations; this is not human winrate."""
import argparse
import json
from pathlib import Path
import statistics

parser=argparse.ArgumentParser(description=__doc__)
parser.add_argument('directory',type=Path)
parser.add_argument('--output',type=Path,required=True)
args=parser.parse_args()
runs=[json.loads(p.read_text()) for p in sorted(args.directory.glob('*.json'))]
runs=[r for r in runs if 'scene' in r and 'rounds' in r]
assert runs,'no completed runs'
groups=[]
for scene in ['day','night','pool','fog','roof','roofnight']:
 for players in [2,4]:
  for defender in sorted({r.get('defender','default') for r in runs}):
   selected=[r for r in runs if r['scene']==scene and r['players']==players and r.get('defender','default')==defender]
   if not selected:continue
   rounds=[q for r in selected for q in r['rounds']]
   breaches=[q['elapsedMS']/1000 for q in rounds if q['breached']]
   samples={}
   for seconds in [0,15,30,60,90,120,150]:
    rows=[q for r in selected for q in r['samples'] if q['seconds']==seconds]
    if not rows:continue
    def mean(fn):return round(statistics.mean(fn(q) for q in rows),2)
    samples[seconds]={'count':len(rows),'shooterRows':mean(lambda q:q['covered']),'producers':mean(lambda q:q['producers']),'mowersUsed':mean(lambda q:q['mowersUsed']),'plantEarned':mean(lambda q:q['earned'][(q['round']-1)%2]),'zombieEarned':mean(lambda q:q['earned'][1-(q['round']-1)%2]),'plantMoney':mean(lambda q:q['money'][(q['round']-1)%2])}
   groups.append({'scene':scene,'players':players,'defender':defender,'rounds':len(rounds),'breached':sum(q['breached'] for q in rounds),'defenseHeld':sum(not q['breached'] for q in rounds),'medianBreachSeconds':round(statistics.median(breaches),2) if breaches else None,'samples':samples})
report={'scope':'deterministic whole matches; normal visible-information actions and natural resources; two defensive policies and three attacking policies; physical shooter-row count excludes cross-row coverage; not human winrate','matches':len(runs),'rounds':sum(len(r['rounds']) for r in runs),'profiles':sorted({r['profile'] for r in runs}),'groups':groups}
args.output.parent.mkdir(parents=True,exist_ok=True)
args.output.write_text(json.dumps(report,ensure_ascii=False,indent=2)+'\n')
print(json.dumps({'matches':report['matches'],'rounds':report['rounds'],'held':sum(q['defenseHeld'] for q in groups),'output':str(args.output)}))
