"""Original complete8C23 for source/receiver heights through03FF, no Go oracle.
Observed admission producer: Court3 teams4/7 slot2 at height626px.
"""
import gzip,hashlib,json
from verify_negative_pass_height import ROOT,ROM,BASE,execute,put

def main():
 rows=[]
 cases={(source,target,arc) for target in range(512,1024) for source in [0,30,255,511,512,626,1023] for arc in [0,24,255]}
 cases|={(source,target,arc) for source in range(512,1024) for target in [0,30,255,511,626,1023] for arc in [0,24,255]}
 for source,target,arc in sorted(cases):
  raw=bytearray(BASE);put(raw,source,0xbc,0xc9);put(raw,target,0x6a2,0x6a3);raw[0x6a4]=arc
  row=execute(raw);assert row['returned'],(source,target,arc,row)
  rows.append(dict(name=f'source{source}-target{target}-arc{arc}',**row))
 data=gzip.compress(json.dumps(rows,separators=(',',':')).encode(),mtime=0);out=ROOT/'games/basketball/engine/testdata';(out/'high-pass-height.json.gz').write_bytes(data)
 source=dict(romSHA256=hashlib.sha256(ROM).hexdigest(),fixtureSHA256=hashlib.sha256(data).hexdigest(),records=len(rows),entry='bank5:8C23 unmodified full launch',scope=__doc__,normalization='all calls returned; no instruction skip, score/AI/seed write')
 (out/'high-pass-height-source.json').write_text(json.dumps(source,indent=2)+'\n');print(json.dumps(source))
if __name__=='__main__':main()
