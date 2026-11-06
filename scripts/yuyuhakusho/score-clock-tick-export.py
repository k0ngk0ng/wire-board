"""Export source-free fixed tick boundary fixtures from native observations."""
import argparse,gzip,json
from pathlib import Path
from reference import ROOT,sha
def main():
 p=argparse.ArgumentParser(description=__doc__);p.add_argument('--source',type=Path,required=True);p.add_argument('--out',type=Path,required=True);a=p.parse_args();out=a.out.resolve();assert out.is_relative_to(ROOT);out.mkdir(parents=True,exist_ok=True)
 raw=a.source.read_bytes();s=json.loads(gzip.decompress(raw));rows=[]
 for row in s['cases']:
  trace=[];domain=None
  for e in row['events']:
   if e['kind']==14:domain=e['at'];continue
   kind={13:'pause-read',10:'pause-write',9:'priority-write'}.get(e['kind']);assert kind is not None,e
   trace.append({'kind':kind,'at':e['at'],'value':e['value']&255})
  rows.append({'input':row['input'],'route':row['route'],'end':row['end'],'domain_cleared_at':domain,'trace':trace})
 natural_path=ROOT/'games/yuyuhakusho/verification/score-clock/natural-boundaries.json';n=json.loads(natural_path.read_bytes())
 natural=[{'frame':r['frame'],'origin':r['driver_entry']['at'],'refresh':r['driver_entry']['refresh'],'pause':r['driver_entry']['value'],'write_at':r['actual_pause_field_store']['at'],'value':r['actual_pause_field_store']['value']}for r in n['pause_transitions']]
 payload={'schema':1,'scope':'Fixed private tick gate/return/priority-store outcomes; natural pause field-store checkpoints, no full tick costs','cases':rows,'natural_pause_stores':natural};body=(json.dumps(payload,separators=(',',':'))+'\n').encode();target=out/'tick-native-boundaries.json.gz';target.write_bytes(gzip.compress(body,mtime=0))
 manifest={'pass':True,'cases':len(rows),'raw_source':{'path':str(a.source.resolve().relative_to(ROOT)),'sha256':sha(raw)},'natural_source':{'path':str(natural_path.relative_to(ROOT)),'sha256':sha(natural_path.read_bytes())},'fixture':{'path':str(target.relative_to(ROOT)),'gzip_sha256':sha(target.read_bytes()),'json_sha256':sha(body)},'producer_sha256':sha(Path(__file__).read_bytes()),'runtime_fixture_has_source_addresses':False};(out/'tick-native-manifest.json').write_text(json.dumps(manifest,indent=2)+'\n');print(json.dumps(manifest))
if __name__=='__main__':main()
