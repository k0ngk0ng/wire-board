"""Reject the unsupported claim that ordered score intents already carry time."""
import argparse,gzip,hashlib,json
from pathlib import Path
ROOT=Path(__file__).resolve().parents[2]
def main():
 p=argparse.ArgumentParser(description=__doc__);p.add_argument('--source',type=Path,required=True);p.add_argument('--out',type=Path,required=True);a=p.parse_args()
 source=a.source.resolve();out=a.out.resolve();assert out.is_relative_to(ROOT)
 data=source.read_bytes();rows=[json.loads(x)for x in gzip.decompress(data).splitlines()]
 intents=[i for row in rows for i in row.get('intents',[])];missing=[{'frame_index':k,'order':i.get('order'),'kind':i.get('kind')}for k,row in enumerate(rows)for i in row.get('intents',[])if i.get('kind')in['ym','psg']and 'cycle'not in i]
 r={'pass':bool(intents)and not missing,'scope':'Ordered typed intents do not establish driver service times','source':str(source.relative_to(ROOT)),'source_sha256':hashlib.sha256(data).hexdigest(),'intents':len(intents),'missing_timed_chip_intents':len(missing),'first_missing':missing[:8]}
 out.parent.mkdir(parents=True,exist_ok=True);out.write_text(json.dumps(r,indent=2)+'\n');print(json.dumps(r))
 if not r['pass']:raise SystemExit(1)
if __name__=='__main__':main()
