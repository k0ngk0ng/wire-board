"""Original 8C23 signed-FF receiver-height domain, independent of Go."""
import argparse,gzip,hashlib,json
from probe_rest_stall import ROOT,ROM,PRG
from verify_pass_floor_original import Memory,MPU,put
from extract_world_actions import decode_world_action_state

SOURCE=ROOT/'.local/nes-basket/runs/negative-pass-height-observed/pass-ram.bin'
BASE=SOURCE.read_bytes()[:2048]


def execute(raw,limit=30000):
 m=Memory(PRG,raw);m[0x8000:0xc000]=PRG[5*16384:6*16384]
 c=MPU(memory=m,pc=0x8c23);c.x=6;c.y=0;c.sp=255;c.p=0x20;c.stPushWord(0x6fff)
 normalization=0
 for n in range(limit):
  if c.pc==0x7000:break
  if c.pc==0x8c6d:normalization+=1
  c.step()
 else:return dict(returned=False,pc=f'{c.pc:04X}',normalization=normalization,instructions=limit)
 before=decode_world_action_state(raw)['ball']['body'];after=decode_world_action_state(bytes(m[:2048]))['ball']['body']
 signed=lambda v:v if v<32768 else v-65536
 target=dict(x=(raw[0x69f]+256*raw[0x6a0])*256,depth=raw[0x6a1]*256,height=signed(raw[0x6a2]+256*raw[0x6a3])*256)
 assert raw[0x48:0x4c]==bytes(m[0x48:0x4c])
 return dict(returned=True,position=before['position'],target=target,arc=raw[0x6a4]*256,secondPlayerX=raw[0x69]+256*raw[0x76]+65536*raw[0x83],expected=after,instructions=n,normalization=normalization)


def main():
 parser=argparse.ArgumentParser();parser.add_argument('--check',action='store_true');args=parser.parse_args();rows=[];excluded=0
 observed=execute(BASE);assert observed['returned'];rows.append(dict(name='retained-original-complete-world-call',**observed))
 for target in range(-256,0):
  for height in [0,1,30,62,255,511]:
   for arc in [0,24,255]:
    if height==0 and target+arc<0:excluded+=1;continue
    raw=bytearray(BASE);put(raw,height,0xbc,0xc9);put(raw,target,0x6a2,0x6a3);raw[0x6a4]=arc
    row=execute(raw);assert row['returned'],(target,height,arc,row)
    rows.append(dict(name=f'height{height}-target{target}-arc{arc}',**row))
 # Fractional target input is quantized by the original caller's byte read.
 for target in [-1,-255,-256,-257,-65535]:
  raw=bytearray(BASE);put(raw,target>>8,0x6a2,0x6a3);row=execute(raw);assert row['returned'];row['target']['height']=target
  rows.append(dict(name=f'fractional-target{target}',**row))
 traps=[]
 for target,arc in [(-256,0),(-1,0),(-256,255)]:
  raw=bytearray(BASE);put(raw,0,0xbc,0xc9);put(raw,target,0x6a2,0x6a3);raw[0x6a4]=arc
  row=execute(raw,2000);assert not row['returned'] and row['normalization']>50
  traps.append(dict(target=target,arc=arc,**row))
 data=gzip.compress((json.dumps(rows,separators=(',',':'))+'\n').encode(),mtime=0);out=ROOT/'games/basketball/engine/testdata';p=out/'negative-pass-height.json.gz'
 if args.check:assert p.read_bytes()==data
 else:p.write_bytes(data)
 meta=dict(romSHA256=hashlib.sha256(ROM).hexdigest(),fixtureSHA256=hashlib.sha256(data).hexdigest(),records=len(rows),excludedZeroSourceNegativeRise=excluded,boundedSourceTraps=traps,source=str(SOURCE.relative_to(ROOT)),sourceSHA256=hashlib.sha256(SOURCE.read_bytes()).hexdigest(),entry='bank5:8C23 complete unmodified launch',scope='one controlled whole-world producer call plus isolated signed-FF receiver matrix; no Go output as oracle, no full-match acceptance',originalInstructionsUnmodified=True)
 if not args.check:(out/'negative-pass-height-source.json').write_text(json.dumps(meta,indent=2)+'\n')
 print(json.dumps(meta))
if __name__=='__main__':main()
