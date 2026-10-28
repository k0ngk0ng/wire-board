"""Retain verified lifecycle analysis and repeatable original boundary records."""
import argparse,gzip,hashlib,json,re
from pathlib import Path
ROOT=Path(__file__).resolve().parents[2]
BASE=ROOT/'.local/contra-campaign-rules'
OUT=ROOT/'docs/research/nes-contra/evidence'
def sha(data):return hashlib.sha256(data).hexdigest()

def main():
    p=argparse.ArgumentParser();p.add_argument('--fixed',default='final-fixed-rea');p.add_argument('--ending',default='ending-control-rea');p.add_argument('--primary',default='campaign-boundary-v8');p.add_argument('--repeat',default='campaign-boundary-v9');a=p.parse_args()
    assert all(re.fullmatch(r'[a-z0-9][a-z0-9-]{0,64}',name) for name in [a.primary,a.repeat])
    functions=[];bundles=[]
    for name,directory in [('fixed',a.fixed),('ending',a.ending)]:
        path=(BASE/directory).resolve();assert path.is_relative_to(BASE)
        data=gzip.compress((path/'evidence.json').read_bytes(),mtime=0);target=OUT/f'campaign-{name}-rea.json.gz';target.write_bytes(data)
        bundles.append({'path':target.name,'sha256':sha(data)})
        for f in sorted(path.glob('function-*.json')):
            d=json.loads(f.read_text());v=d['normalized_result']
            functions.append({'address':v['procedure']['address'],'bank':name,'evidence_id':d['evidence_id'],'subject':d['subject'],'assembly':v['assembly'],'limitations':d['limitations']+v['limitations']})
    verified=[]
    for name in ['ram.bin','campaign.jsonl']:
        first=(BASE/'oracle/runs'/a.primary/name).read_bytes();second=(BASE/'oracle/runs'/a.repeat/name).read_bytes();assert first==second,name
        verified.append({'file':name,'sha256':sha(first),'bytes':len(first)})
    fixture=OUT/'campaign-boundaries-original.json.gz';data=json.loads(gzip.decompress(fixture.read_bytes()))
    assert len(data['records'])==199 and len(data['cases'])==117
    report={'rom_sha256':'6ba53139fa88b8de1ae527c438bda6f1541d1ee7df26d63dec5164a32d166bfe','functions':functions,'bundles':bundles,'oracle_sha256':sha(fixture.read_bytes()),'cases':117,'records':199,'repeat_byte_identical':verified,
            'limitations':['Controlled decision-boundary inputs, not a naturally completed campaign.','End-sequence/interlude/intro scene execution is explicitly delegated, not replaced by a boss flag or a completed boolean.','The original ending has a conditional level-zero loop return at bank8/9 $8498. Standard first-round Campaign freezes before that gameplay loop.','Japanese GameRoutine 7 is sound test, not the normal campaign ending.']}
    (OUT/'campaign-index.json').write_text(json.dumps(report,indent=2)+'\n')
    print(json.dumps({'passed':True,'functions':len(functions),'records':199,'repeat':verified}))

if __name__=='__main__':main()
