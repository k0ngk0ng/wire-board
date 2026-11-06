"""Original native mailbox values sampled after each actual bus grant."""
from pathlib import Path
import argparse,gzip,json,hashlib
ROOT=Path(__file__).resolve().parents[2];p=argparse.ArgumentParser();p.add_argument('--capture',required=True,type=Path);p.add_argument('--out',required=True,type=Path);a=p.parse_args();capture=a.capture.resolve();out=a.out.resolve();assert capture.is_relative_to(ROOT)and out.is_relative_to(ROOT);cases={}
for path in sorted(capture.glob('*.json.gz')):
 b=json.loads(gzip.decompress(path.read_bytes()));cases[b['case']['name']]={'Values':[x['busy']for x in b['transactions']],'SourceSHA256':hashlib.sha256(path.read_bytes()).hexdigest()}
raw=(json.dumps({'Version':1,'Cases':cases},separators=(',',':'))+'\n').encode();out.write_bytes(gzip.compress(raw,mtime=0));print(json.dumps({'cases':len(cases),'sha256':hashlib.sha256(out.read_bytes()).hexdigest()}))
