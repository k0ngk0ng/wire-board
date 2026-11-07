"""Retain original palette oracle before implementing the semantic Go phases."""
from pathlib import Path
import argparse,gzip,hashlib,json,re
ROOT=Path(__file__).resolve().parents[2]
BASE=ROOT/'.local/contra-palette'


def main():
 p=argparse.ArgumentParser();p.add_argument('--first-label',default='v1');p.add_argument('--natural-label',default='v2');p.add_argument('--controlled-label',default='v2');a=p.parse_args()
 assert all(re.fullmatch(r'[a-z0-9-]+',s)for s in [a.first_label,a.natural_label,a.controlled_label])
 def load(mode,label):return json.loads(gzip.decompress((BASE/'runtime'/f'palette-{mode}-{label}.json.gz').read_bytes()))
 first=load('natural',a.first_label);natural=load('natural',a.natural_label);controlled=load('controlled',a.controlled_label)
 assert natural['records']==6400 and controlled['records']==137
 for old,new in zip(first['traces'],natural['traces']):
  assert old['ramSHA256']==new['ramSHA256'] and old['records']==new['records']
 tables=json.loads((ROOT/'docs/research/nes-contra/evidence/palette-additional-colors.json').read_text())
 result={'romSHA256':tables['romSHA256'],'natural':natural,'controlled':controlled,'tables':tables}
 raw=gzip.compress(json.dumps(result,separators=(',',':')).encode(),mtime=0)
 out=ROOT/'games/contra/engine/testdata';out.mkdir(parents=True,exist_ok=True)
 (out/'original-palette.json.gz').write_bytes(raw)
 source={'romSHA256':tables['romSHA256'],'fixtureSHA256':hashlib.sha256(raw).hexdigest(),
         'naturalRecords':6400,'controlledRecords':137,'naturalRepeatRAMIdentical':True,
         'scope':'Original CEE3/CF6D/CC02/CE59 calls. Natural gameplay and controlled function inputs remain separate.',
         'fadeInterpretation':'CF64 byte BB belongs to the preceding BNE operand; valid indexed fade values use CF65..CF6C for timer 1..8.'}
 (out/'original-palette-source.json').write_text(json.dumps(source,indent=2)+'\n');print(json.dumps(source))


if __name__=='__main__':main()
