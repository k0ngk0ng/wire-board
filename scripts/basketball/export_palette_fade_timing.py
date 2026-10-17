"""Derive video-boundary fade expectations from retained original observations."""
from pathlib import Path
import hashlib,json
ROOT=Path(__file__).resolve().parents[2]
source=ROOT/'.local/nes-basket/runs/court-4-palette-complete-palette-nmi-observed/palette.jsonl'
events=[json.loads(line) for line in source.read_text().splitlines()]
cases=[]
for name,start,end,out in [('period-one-out',14362,14370,True),('second-opening-in',14554,14562,False),('period-two-out',25508,25516,True),('result-in',25530,25538,False)]:
 queues=[e for e in events if start<=e['frame']<end and e['kind']=='queue']
 commits=[e for e in events if start<=e['frame']<end and e['kind']=='commit-after']
 assert len(queues)==len(commits)==4
 levels=[3,2,1,0] if out else [1,2,3,4]
 base=queues[0]['ppu'] if out else commits[-1]['ppu']
 for i,(q,c,level) in enumerate(zip(queues,commits,levels)):
  assert q['flags']==(128 if out else 0) and q['delay']==1
  assert c['nmi']==q['nmi']+1 and q['nmi']==queues[0]['nmi']+i*2
  # Independent original source $9526 threshold clamp, all32 court colors.
  threshold=level*16
  expected=[15 if not level else ((v&15)+threshold-16 if v>=threshold else v) for v in base]
  assert c['ppu']==expected
 cases.append(dict(name=name,out=out,palette=queues[0]['queuedID'],initialLevel=4 if out else 0,levels=[x for level in levels for x in [level]*2],queueNMIs=[e['nmi'] for e in queues],commitNMIs=[e['nmi'] for e in commits],colors=[e['ppu'] for e in commits]))
fixture=ROOT/'games/basketball/engine/testdata/palette-fade-video.json'
fixture.write_text(json.dumps(dict(cases=cases),indent=2)+'\n')
(ROOT/'games/basketball/engine/testdata/palette-fade-video-source.json').write_text(json.dumps(dict(source=str(source.relative_to(ROOT)),sourceSHA256=hashlib.sha256(source.read_bytes()).hexdigest(),fixtureSHA256=hashlib.sha256(fixture.read_bytes()).hexdigest(),generatorSHA256=hashlib.sha256(Path(__file__).read_bytes()).hexdigest(),scope='Four original court/result fades, actual queue/NMI intervals and color buffers; not full Go runtime timing equivalence'),indent=2)+'\n')
print('PASS: four original fades,16 color commits, next-NMI and two-video level spacing')
