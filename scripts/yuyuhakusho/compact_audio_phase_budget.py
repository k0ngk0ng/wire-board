#!/usr/bin/env python3
"""Drop only the zero-backlog no-op polling marker; retain every sound write."""
import argparse,gzip,hashlib,json
from collections import Counter
from pathlib import Path
ROOT=Path(__file__).resolve().parents[2]

def sha(p):return hashlib.sha256(p.read_bytes()).hexdigest()
def source(path):
 decoder=json.JSONDecoder()
 with gzip.open(path,'rt')as f:
  buffer=''
  while '"events":['not in buffer:
   chunk=f.read(65536);assert chunk;buffer+=chunk
  prefix,buffer=buffer.split('"events":[',1)
  header=json.loads(prefix+'"events":[]}')
  yield header
  while True:
   buffer=buffer.lstrip(', \n\r\t')
   if buffer.startswith(']'):break
   try:row,end=decoder.raw_decode(buffer)
   except json.JSONDecodeError:
    chunk=f.read(65536);assert chunk;buffer+=chunk;continue
   yield row;buffer=buffer[end:]

def main():
 p=argparse.ArgumentParser(description=__doc__);p.add_argument('--source',type=Path,required=True);p.add_argument('--out',type=Path,required=True);args=p.parse_args()
 out=args.out.resolve();assert out.is_relative_to(ROOT);out.parent.mkdir(parents=True,exist_ok=True)
 it=source(args.source);header=next(it);kept=[];dropped=0;total=0;writer=Counter()
 for index,row in enumerate(it):
  total+=1
  if row[0]==2:writer[hex(row[1]&0xffffff)]+=1
  if row[0]==1 and row[1]&0xffffff==0x21a06:dropped+=1;continue
  kept.append(row+[index])
 payload={'schema':1,'event_fields':header['event_fields']+['source_ordinal'],'owned_fields':header['owned_fields'],'initial':header['initial'],'events':kept}
 raw=(json.dumps(payload,separators=(',',':'))+'\n').encode();packed=gzip.compress(raw,mtime=0);out.write_bytes(packed)
 report={'pass':True,'source':str(args.source.resolve().relative_to(ROOT)),'source_sha256':sha(args.source),'output':str(out.relative_to(ROOT)),'json_sha256':hashlib.sha256(raw).hexdigest(),'gzip_sha256':sha(out),'original_events':total,'retained_events':len(kept),'dropped_noop_polls':dropped,'sound_writes':sum(writer.values()),'all_sound_writes_retained':True,'write_pcs':dict(writer),'filter':'Only kind1 marker21a06 polling return is omitted. Allkind2 writes including unchanged values and all other event classes are retained.'}
 out.with_suffix('').with_suffix('.manifest.json').write_text(json.dumps(report,indent=2)+'\n')
 print(json.dumps({k:v for k,v in report.items()if k not in ['filter','write_pcs']}))
 print(json.dumps(dict(writer)))
if __name__=='__main__':main()
