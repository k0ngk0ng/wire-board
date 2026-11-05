"""Export semantic IO outcomes from native service traces, without source PCs."""
import argparse,gzip,json
from pathlib import Path
from reference import ROOT,sha
def main():
 p=argparse.ArgumentParser(description=__doc__);p.add_argument('--source',type=Path,required=True);p.add_argument('--out',type=Path,required=True);a=p.parse_args()
 out=a.out.resolve();assert out.is_relative_to(ROOT);out.mkdir(parents=True,exist_ok=True)
 raw=a.source.read_bytes();native=json.loads(gzip.decompress(raw));cases=[]
 for row in native['cases']:
  events=[];cache=None;pending=None
  for event in row['events']:
   kind=event['kind'];at=event['at']
   if kind==4:
    assert pending is None;pending=event;continue
   if kind==5:
    assert pending is not None
    value=pending['value']>>8 if pending['width']==2 else pending['value']
    events.append({'kind':'request','at':pending['at'],'after':event['after'],'asserted':bool(value&1)});pending=None
   elif kind==6:events.append({'kind':'ack','at':at,'granted':not bool(event['value']&1)})
   elif kind==3:events.append({'kind':'mailbox','at':event['after'],'value':event['value']})
   elif kind in[1,2]:events.append({'kind':'status'if kind==1 else'write','at':at,'port':event['address'],'value':event['value']})
   elif kind==7:
    assert cache is None;cache={'at':at,'bank':row['input']['bank'],'register':row['input']['register'],'value':event['value']}
   else:raise AssertionError(('Unknown native event',event))
  assert pending is None
  cases.append({'input':row['input'],'events':events,'cache':cache,'end':row['end']})
 fixture={'schema':1,'scope':'Canonical direct YM service, bus already granted, mailbox clear; given origin and seed busy, real per-access timings and final refresh','cases':cases}
 content=(json.dumps(fixture,separators=(',',':'))+'\n').encode();target=out/'native-services.json.gz';target.write_bytes(gzip.compress(content,mtime=0))
 manifest={'source':{'path':str(a.source.resolve().relative_to(ROOT)),'sha256':sha(raw)},'output':{'path':str(target.relative_to(ROOT)),'gzip_sha256':sha(target.read_bytes()),'json_sha256':sha(content)},'cases':len(cases),'accesses':sum(len(x['events'])for x in cases),'cache_effects':sum(x['cache']is not None for x in cases),'producer_sha256':sha(Path(__file__).read_bytes()),'source_addresses_in_runtime_fixture':False}
 (out/'native-manifest.json').write_text(json.dumps(manifest,indent=2)+'\n');print(json.dumps(manifest))
if __name__=='__main__':main()
