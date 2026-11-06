"""Research action windows from actual Root-control call boundaries."""
import argparse,gzip,json
from pathlib import Path
from reference import ROOT,sha
def main():
 p=argparse.ArgumentParser(description=__doc__);p.add_argument('--capture',type=Path,required=True);p.add_argument('--out',type=Path,required=True);a=p.parse_args();out=a.out.resolve();assert out.is_relative_to(ROOT);out.mkdir(parents=True,exist_ok=True)
 reports=[]
 for path in sorted(a.capture.resolve().glob('*.json.gz')):
  if path.name.endswith('-control.json.gz'):continue
  raw=path.read_bytes();body=json.loads(gzip.decompress(raw));events=body['events'];segments=[];fade=None;reset=None
  for i,event in enumerate(events):
   pc=event['pc']
   if pc==0x3893a:fade=(i,event)
   if pc in[0x3894c,0x389a6,0x38a2c]and fade is not None:
    first,start=fade;assert 'driver_hex'in start and 'driver_hex'in event
    kind='fade-step'if pc==0x389a6 else'request-full-clear'if pc==0x38a2c else'counter-or-idle'
    segments.append({'kind':kind,'step':event['step'],'before_driver_hex':start['driver_hex'],'after_driver_hex':event['driver_hex'],'before_root':start['root'],'after_root':event['root'],'events':events[first:i+1]});fade=None
   if pc==0x38a2c:reset=(i,event)
   if pc==0x3a1a4 and reset is not None:
    first,start=reset;segments.append({'kind':'full-clear04','step':event['step'],'before_driver_hex':start['driver_hex'],'after_driver_hex':event['driver_hex'],'before_root':start['root'],'after_root':body['frames'][event['step']]['root'],'events':events[first:i+1]});reset=None
   if pc in[0x38916,0x38928]:
    before=bytes.fromhex(event['driver_hex']);after=bytes.fromhex(body['frames'][event['step']]['driver_hex']);assert before[65:112]==after[65:112]and after[64]==0,'DAC stop modified params or flags were not fully cleared'
    segments.append({'kind':'stop-dac-flags','step':event['step'],'control':1 if pc==0x38916 else 5,'before_dac_hex':before[64:112].hex(),'after_dac_hex':after[64:112].hex(),'source_event':event})
  assert fade is None and reset is None,(path.name,'unclosed control window')
  target=out/(body['case']['id']+'-actions.json.gz');encoded=(json.dumps({'schema':1,'scope':'Original control-only action windows, no model-generated expected values; source pointers are research-only provenance','case':body['case'],'segments':segments},separators=(',',':'))+'\n').encode();target.write_bytes(gzip.compress(encoded,mtime=0))
  reports.append({'case':body['case']['id'],'source_path':str(path.relative_to(ROOT)),'source_sha256':sha(raw),'segments':len(segments),'kinds':{kind:sum(x['kind']==kind for x in segments)for kind in sorted({x['kind']for x in segments})},'output_path':str(target.relative_to(ROOT)),'output_sha256':sha(target.read_bytes())})
 (out/'index.json').write_text(json.dumps({'pass':True,'scope':'Original action provenance for score owner, not a Go Root/control acceptance','cases':reports,'producer_sha256':sha(Path(__file__).read_bytes())},indent=2)+'\n');print(json.dumps(reports))
if __name__=='__main__':main()
