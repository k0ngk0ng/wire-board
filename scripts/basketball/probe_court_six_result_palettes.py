"""Isolated execution of original bank4 $9526 for palette/level/group coverage."""
import json,hashlib,sys
from pathlib import Path
ROOT=Path(__file__).resolve().parents[2];sys.path.insert(0,str(ROOT/'.local/basketball/python'))
from py65.devices.mpu6502 import MPU
from static_sprites import SpriteROM,DIGEST
rom=SpriteROM((ROOT/'.local/nes-basket/basketball.nes').read_bytes());cases=[]
for palette in [7,11,12,13,14,15]:
 for teams in [(0,1),(7,6)]:
  for flags in [0,128]:
   for level in range(5):
    cpu=MPU();cpu.memory[0x8000:0xc000]=rom.prg[4*16384:5*16384];cpu.memory[0xc000:]=rom.prg[-16384:];cpu.memory[0x301]=flags;cpu.memory[0x303]=level*16;cpu.memory[0x304]=palette;cpu.memory[0x305]=teams[0]+2;cpu.memory[0x306]=teams[1]+2;cpu.memory[0x588]=1;cpu.memory[0x589]=1;cpu.sp=0xfd;cpu.memory[0x1fe]=0xff;cpu.memory[0x1ff]=0x3f;cpu.pc=0x9526
    for ticks in range(10000):
     if cpu.pc==0x4000:break
     cpu.step()
    else:raise AssertionError('Original palette builder did not return')
    assert cpu.memory[0x374]==0x3f and cpu.memory[0x376]==32
    cases.append({'paletteID':palette,'teams':teams,'flags':flags,'level':level,'colors':list(cpu.memory[0x377:0x397])})
out=ROOT/'.local/basketball/rea/court-six';out.mkdir(parents=True,exist_ok=True);data={'scope':'120 isolated original $9526 executions; controlled threshold and selectors, not natural frames','source_rom_sha256':DIGEST,'original_source_sha256':hashlib.sha256(rom.prg[4*16384+0x1526:4*16384+0x162f]).hexdigest(),'cases':cases};(out/'result-palette-oracle.json').write_text(json.dumps(data,indent=2)+'\n');print(json.dumps({'cases':len(cases),'scope':data['scope']}))
