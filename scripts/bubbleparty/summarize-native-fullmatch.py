#!/usr/bin/env python3
"""Summarize retained `go test -json` records without rerunning a match."""
import argparse,hashlib,json
from pathlib import Path
parser=argparse.ArgumentParser(description=__doc__)
parser.add_argument('--out',type=Path,default=Path('.local/bubble-mounts'))
base=parser.parse_args().out
raw=base/'native-fullmatch-go.jsonl'
events=[json.loads(line) for line in raw.read_text().splitlines() if line.strip()]
rows=[]
for event in events:
 output=event.get('Output','')
 if 'NATIVE_FULLMATCH ' in output:rows.append(json.loads(output.split('NATIVE_FULLMATCH ',1)[1]))
failures=[event for event in events if event['Action']=='fail']
package_pass=[event for event in events if event['Action']=='pass' and not event.get('Test')]
assert len(rows)==448 and len({(r['map'],r['players']) for r in rows})==448
summary={
 'passed':not failures and bool(package_pass),
 'matches':len(rows),'maps':len({r['map'] for r in rows}),'playerCounts':sorted({r['players'] for r in rows}),
 'draws':sum(r['draw'] for r in rows),'timeouts':sum(r['timeout'] for r in rows),
 'restores':sum(r['restores'] for r in rows),'bombs':sum(r['bombs'] for r in rows),
 'pickups':sum(r['pickups'] for r in rows),'mountPickups':sum(r['mountPickups'] for r in rows),'dismounts':sum(r['dismounts'] for r in rows),
 'invalidAirborneBombRequests':sum(r['invalidAirborneBombRequests'] for r in rows),
 'partialMovementMatches':sum(r['movedSeats']!=r['players'] for r in rows),
 'maxAirborneIdleMS':max(r['longestAirborneIdleMS'] for r in rows),
 'wallSeconds':package_pass[-1].get('Elapsed') if package_pass else None,
 'rawGoJSON':'native-fullmatch-go.jsonl','rawGoJSONSHA256':hashlib.sha256(raw.read_bytes()).hexdigest(),
 'command':". scripts/env.sh; go test ./games/bubbleparty/server -run '^TestNativeFullmatchAllClassicMaps$' -count=1 -json -timeout 20m > .local/bubble-mounts/native-fullmatch-go.jsonl",
 'scope':'112 source maps; production computer decisions/validated inputs/simulation; 180-second normal rules; no forced result or board mutation; live restore at 0.5 and 60 seconds when match remains active',
 'limitations':['This is the server gameplay E2E matrix, not browser rendering acceptance.','Nine players across five matches placed bombs and died before moving despite initial open exits; neighboring computer bomb placement can close escape paths.'],
}
report={'summary':summary,'matches':rows,'failures':failures}
(base/'native-fullmatch-report.json').write_text(json.dumps(report,ensure_ascii=False,indent=2)+'\n')
print(json.dumps(summary,ensure_ascii=False))
