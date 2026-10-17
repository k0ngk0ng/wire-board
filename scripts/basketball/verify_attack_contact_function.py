"""Isolated original 8318/82E4 geometry oracle, plus cold selector/request audit."""
import gzip,hashlib,json,sys
from pathlib import Path
ROOT=Path(__file__).resolve().parents[2]
sys.path.insert(0,str(ROOT/'.local/basketball/python'))
from py65.devices.mpu6502 import MPU


def main():
 rom=(ROOT/'.local/nes-basket/basketball.nes').read_bytes();digest=hashlib.sha256(rom).hexdigest()
 assert digest=='0fa28f60abb8d2ccbbeee90e79e3fa160a5cf32feb90f47ebf1a89168cff569b'
 prg=rom[16:16+131072];bank6=prg[6*16384:7*16384]
 word6=lambda a:int.from_bytes(bank6[a-0x8000:a-0x8000+2],'little')
 variants=set()
 for action in range(9):
  p=word6(0xa022+action*2)
  variants.update(bank6[p-0x8000:p-0x8000+64:2])
 counts={}
 for variant in sorted(variants):
  p=word6(0xac8f+variant*2);count=0
  while bank6[p+count*2-0x8000]<253:count+=1;assert count<128
  counts[variant]=count
 cpu=MPU();cpu.memory[0x8000:0xc000]=prg[5*16384:6*16384];cpu.memory[0xc000:]=prg[-16384:]
 records=[]
 for variant,count in counts.items():
  for frame in range(count):
   cpu.memory[:2048]=[0]*2048;cpu.sp=255;cpu.pc=0x8318;cpu.x=1;cpu.p=0x20
   cpu.memory[0x513]=0xe7;cpu.memory[0x57b]=variant;cpu.memory[0x445]=frame;cpu.stPushWord(0x6fff)
   for steps in range(200):
    if cpu.pc==0x7000:break
    cpu.step()
   assert cpu.pc==0x7000
   pointer=cpu.WordAt(0x2c)
   records.append(dict(variant=variant,frame=frame,pointer=pointer,shape=list(cpu.memory[pointer:pointer+8])))
 flags=list(cpu.memory[0x9321+96:0x9321+106])
 # The source request dispatcher and selection are separate from geometry.
 cpu.memory[0x8000:0xc000]=bank6;cpu.memory[:2048]=[0]*2048;cpu.x=1;cpu.pc=0xe18b
 cpu.memory[0x513]=103;cpu.memory[0x520]=29;cpu.memory[0x3e4]=1
 for n in range(200):
  if cpu.pc==0xe1ed:break
  cpu.step()
 assert cpu.pc==0xe1ed and cpu.memory[0x513]==103 and cpu.memory[0x520]==29
 cpu.pc=0x9fa7
 for n in range(200):
  if cpu.pc==0xa01f:break
  cpu.step()
 assert cpu.pc==0xa01f and cpu.memory[0x57b]==93
 selector=dict(action=cpu.memory[0x513],request=cpu.memory[0x520],variant=cpu.memory[0x57b],handler=cpu.WordAt(0x2a))
 payload=(json.dumps(dict(records=records,flags=flags,selector=selector),separators=(',',':'))+'\n').encode();packed=gzip.compress(payload,mtime=0)
 out=ROOT/'games/basketball/engine/testdata';(out/'contact-attack-function.json.gz').write_bytes(packed)
 summary=dict(source_is_natural_execution=False,isolated_original_function_execution=True,geometry_entry='bank5 8318 -> 82E4',variant_selection_source='bank6 A022; animation frame counts AC8F',request_audit='fixed E18B..E1ED; bank6 9FA7..A01F',variants=len(variants),records=len(records),selector=selector,rom_sha256=digest,sha256=hashlib.sha256(packed).hexdigest(),uncompressed_sha256=hashlib.sha256(payload).hexdigest())
 (out/'contact-attack-function-source.json').write_text(json.dumps(summary,indent=2)+'\n');print(json.dumps(summary))


if __name__=='__main__':main()
