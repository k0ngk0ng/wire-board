"""Require the intended original pause lifecycle before accepting its oracle."""
import argparse,gzip,json
from pathlib import Path
from reference import ROOT,sha
def main():
 p=argparse.ArgumentParser(description=__doc__);p.add_argument('--source',type=Path,required=True);p.add_argument('--out',type=Path,required=True);a=p.parse_args();out=a.out.resolve();assert out.is_relative_to(ROOT)
 data=a.source.read_bytes();s=json.loads(gzip.decompress(data));frames=s['frames'];events=s['events'];captures=[e for e in events if e['pc']==0x37bd8 and 'levels'in e];errors=[]
 if not captures:errors.append('Intended pause capture point was not reached')
 held=[r['step']for r in frames if r['root']['AudioPause']==255]
 if not held:errors.append('No actual FF-held pause frames')
 restores=[e for e in events if e['pc']==0x37c42]
 if not restores:errors.append('No actual level-restore return')
 result={'pass':not errors,'source':str(a.source.resolve().relative_to(ROOT)),'source_sha256':sha(data),'capture_count':len(captures),'held_steps':held,'restore_count':len(restores),'errors':errors}
 out.parent.mkdir(parents=True,exist_ok=True);out.write_text(json.dumps(result,indent=2)+'\n');print(json.dumps(result))
 if errors:raise SystemExit(1)
if __name__=='__main__':main()
