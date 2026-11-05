"""Export all original bus input/outcome tuples; no model-generated outcomes."""
import argparse,gzip,json,struct
from pathlib import Path
from reference import ROOT,sha

def main():
 p=argparse.ArgumentParser(description=__doc__);p.add_argument('--capture',type=Path,required=True);p.add_argument('--out',type=Path,required=True);args=p.parse_args()
 out=args.out.resolve();assert out.is_relative_to(ROOT);out.mkdir(parents=True,exist_ok=True)
 rows=[];sources=[]
 for scenario in ['baseline','pause']:
  path=args.capture.resolve()/f'phase-clock-{scenario}.json.gz';data=path.read_bytes();source=json.loads(gzip.decompress(data));fields={name:i for i,name in enumerate(source['bus_fields'])}
  for event in source['bus_events']:
   def v(name):return event[fields[name]]
   assert v('kind')==13 and v('guest_address')>=0x8000,'Fixture scope is original ROM bank reads'
   rows.append(struct.pack('>IIBIIBBB',v('z80_before'),v('dma_end'),v('dma_type'),v('dma_length'),v('z80_after'),v('m68k_after')-v('m68k_before'),v('zstate_before'),v('zstate_after')))
  sources.append({'path':str(path.relative_to(ROOT)),'sha256':sha(data),'count':len(source['bus_events'])})
 raw=b'YYHBUS1\n'+struct.pack('>I',len(rows))+b''.join(rows);packed=gzip.compress(raw,mtime=0)
 target=out/'native-bus.bin.gz';target.write_bytes(packed)
 manifest={'schema':1,'scope':'Every original input and observed output from both complete native captures; no predicted or fitted expected values','count':len(rows),'record_bytes':20,'byte_order':'big','header':'YYHBUS1 newline + uint32 record count','fields':['frame_relative_z80_before:u32','dma_end:u32','dma_type:u8','dma_length:u32','observed_z80_after:u32','observed_m68k_delta:u8','zstate_before:u8','zstate_after:u8'],'sources':sources,'output':{'path':str(target.relative_to(ROOT)),'jsonless_raw_sha256':sha(raw),'gzip_sha256':sha(packed),'raw_bytes':len(raw),'gzip_bytes':len(packed)},'producer_sha256':sha(Path(__file__).read_bytes())}
 (out/'native-bus-manifest.json').write_text(json.dumps(manifest,indent=2)+'\n');print(json.dumps(manifest['output']))
if __name__=='__main__':main()
