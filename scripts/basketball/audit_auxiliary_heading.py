"""Verify source zeros and the lossless Heading-only carry-fixture migration."""
import collections,gzip,hashlib,json
from pathlib import Path
ROOT=Path(__file__).resolve().parents[2]
OLD_SHA='2541ae0b3cedd024400aab6dd7fc29dcffabb7fd308504125e7b2588db57ccd4'

def main():
    fixture=ROOT/'games/basketball/engine/testdata/auxiliary-carry-chain.json.gz'
    packed=fixture.read_bytes();rows=json.loads(gzip.decompress(packed));values=set()
    for row in rows:
        for phase in ('before','after'):
            for aux in row[phase]['auxiliary']:values.add(aux.pop('heading'))
    legacy=gzip.compress((json.dumps(rows,separators=(',',':'))+'\n').encode(),mtime=0)
    assert hashlib.sha256(legacy).hexdigest()==OLD_SHA,'a pre-existing fixture field changed'
    audits=[]
    for name in ['world-actions-cpu-long/world-actions-ram.bin','world-inputs-cpu-long/world-inputs-ram.bin.gz','world-preparation-cpu-long/world-preparation-ram.bin','ai-shot-observation/ai-ram.bin']:
        path=ROOT/'.local/nes-basket/runs'/name;source=path.read_bytes();raw=gzip.decompress(source) if path.suffix=='.gz' else source
        counts=collections.Counter((raw[i+0x4f0],raw[i+0x4f1]) for i in range(0,len(raw),2048))
        assert list(counts)==[(0,0)],(name,counts)
        audits.append(dict(source=str(path.relative_to(ROOT)),sha256=hashlib.sha256(source).hexdigest(),snapshots=len(raw)//2048,all_auxiliary_headings_raw_zero=True))
    out=ROOT/'.local/basketball/rea/held-auxiliary-input';out.mkdir(parents=True,exist_ok=True)
    report=dict(old_fixture_sha256=OLD_SHA,new_fixture_sha256=hashlib.sha256(packed).hexdigest(),records=len(rows),all_old_fields_identical=True,new_heading_values=sorted(values),legacy_zero_audit=audits)
    (out/'heading-migration.json').write_text(json.dumps(report,indent=2)+'\n')
    print('Heading migration preserved all old fields; audited',sum(r['snapshots'] for r in audits),'original snapshots')

if __name__=='__main__':main()
