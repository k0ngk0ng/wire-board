"""Require direct native before/after-ack and execution-batch evidence."""
from pathlib import Path
import argparse,gzip,json
ROOT=Path(__file__).resolve().parents[2]
p=argparse.ArgumentParser();p.add_argument('--capture',type=Path,default=ROOT/'games/yuyuhakusho/verification/world-video/native');a=p.parse_args()
j=json.loads(gzip.decompress((a.capture/'events.json.gz').read_bytes()));fields=j['fields'];rows=[dict(zip(fields,r))for r in j['events']]
before=[r for r in rows if r['kind']==7];after=[r for r in rows if r['kind']==10];batch=[r for r in rows if r['kind']==11]
assert before and len(before)==len(after), 'Missing direct original IRQ acknowledge before/after pairs'
assert batch,'Missing original CPU execution target/call-order evidence'
for b,c in zip(before,after):
 assert b['frame']==c['frame'] and b['value']==c['value'] and b['cpu_cycle']==c['cpu_cycle']
 assert b['value']in[4,6]
 assert b['hint_pending']or b['vint_pending']
print(json.dumps({'pass':True,'ack_pairs':len(before),'batch_changes':len(batch)}))
