"""Unmodified A8DE entered-mode26 direct plans; random table is untouched."""
import gzip,hashlib,itertools,json,sys
from pathlib import Path
ROOT=Path(__file__).resolve().parents[2];sys.path.insert(0,str(ROOT/'.local/basketball/python'))
from py65.devices.mpu6502 import MPU
ROM=(ROOT/'.local/nes-basket/basketball.nes').read_bytes();PRG=ROM[16:16+131072]

def execute(case,retained=None):
 fresh=retained is None or retained[0] is None
 c=MPU() if fresh else retained[0]
 if retained is not None:retained[0]=c
 m=c.memory;a=case['actor']
 if fresh:
  m[0x8000:]=PRG[5*16384:6*16384]+PRG[-16384:]
  for addr,key in [(0x682,'parameter'),(0x66e,'counter'),(0x66a,'zone'),(0x512,'action'),(0x4ec,'heading'),(0x576,'jumpPhase'),(0x4b9,'verticalSpeed')]:m[addr+a]=case[key]
  m[0x672+a]=case.get('mode',154);m[0x676+a]=case.get('plan',8);m[0x58b]=1;m[0x48:0x4c]=case['random'].to_bytes(4,'big')
 c.pc=0xa8de;c.x=a;c.sp=255;c.stPushWord(0x6fff)
 for _ in range(1000):
  if c.pc==0x7000:break
  c.step()
 else:raise RuntimeError(hex(c.pc))
 fields={key:m[addr+a] for addr,key in [(0x682,'parameter'),(0x66e,'counter'),(0x512,'action'),(0x4ec,'heading'),(0x576,'jumpPhase'),(0x672,'mode'),(0x676,'plan')]};fields['random']=int.from_bytes(bytes(m[0x48:0x4c]),'big');return dict(case,after=fields)

def main():
 rows=[]
 for plan,a,counter,zone,action in itertools.product([6,10,12,13],range(4),[0,1,7,8,31,32,127,128,255],[0x30,0x31,0x32,0x33,0x34,0x35,0x70],[16,144,17,145,18,146,20,148,46,174]):
  rows.append(execute(dict(actor=a,parameter=plan,counter=counter,zone=zone,action=action,heading=32,jumpPhase=0,verticalSpeed=1,random=0x12345678)))
 for plan,a,phase,speed in itertools.product([7,8,9],range(4),[0,1,127,128,255],[0,1,2,127,128,255]):
  rows.append(execute(dict(actor=a,parameter=plan,counter=7,zone=0x32,action=146,heading=64,jumpPhase=phase,verticalSpeed=speed,random=0x12345678)))
 for a,zone,seed in itertools.product(range(4),range(6),[0x00345678,0x40345678,0x80345678,0xc0345678,0xff345678]):
  rows.append(execute(dict(actor=a,parameter=6,counter=0,zone=0x30|zone,action=174,heading=128,jumpPhase=255,verticalSpeed=255,random=seed)))
 for a,heading,counter in itertools.product(range(4),range(0,256,32),[6,7,15,31,127,255]):
  rows.append(execute(dict(actor=a,parameter=12,counter=counter,zone=0x32,action=146,heading=heading,jumpPhase=0,verticalSpeed=0,random=0x12345678)))
 invalid=[execute(dict(actor=a,parameter=12,counter=7,zone=0x32,action=146,heading=255,jumpPhase=0,verticalSpeed=0,random=0x12345678)) for a in range(4)]
 chains=[]
 for plan,a in itertools.product([6,7,8,9,10,12,13],range(4)):
  case=dict(actor=a,parameter=plan,counter=0,zone=0x32,action=146,heading=64,jumpPhase=0,verticalSpeed=1,random=0x12345678);records=[];retained=[None]
  for _ in range(40):
   record=execute(case,retained);records.append(record);after=record['after']
   if after['mode']&127!=26:break
   case=dict(case,**after)
  chains.append(dict(actor=a,parameter=plan,records=records))
 out=ROOT/'games/basketball/engine/testdata';packed=gzip.compress((json.dumps(dict(calls=rows,unsupportedHeading=invalid,chains=chains),separators=(',',':'))+'\n').encode(),mtime=0);(out/'ai-evasion-direct.json.gz').write_bytes(packed)
 meta=dict(scope='controlled entered-mode26 original A8DE calls; does not create random-table entries or establish natural reachability',records=len(rows),chainSource='one RAM initialization per chain; same MPU retained; no semantic field reinjection',chains=len(chains),chainSteps=[len(c['records']) for c in chains],unsupportedHeading=len(invalid),romSHA256=hashlib.sha256(ROM).hexdigest(),fixtureSHA256=hashlib.sha256(packed).hexdigest());(out/'ai-evasion-direct-source.json').write_text(json.dumps(meta,indent=2)+'\n');print(json.dumps(meta))
if __name__=='__main__':main()
