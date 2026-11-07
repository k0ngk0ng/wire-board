"""Rebuild semantic indoor wave tables from the pinned original ROM."""
from pathlib import Path
import hashlib,zipfile,subprocess
ROOT=Path(__file__).resolve().parents[2]
with zipfile.ZipFile('/Users/jason/Documents/nes/roms/03/魂斗罗日版(完全版).zip')as z:rom=z.read('Contra (J).nes')
assert hashlib.sha256(rom).hexdigest()=='6ba53139fa88b8de1ae527c438bda6f1541d1ee7df26d63dec5164a32d166bfe'
p=rom[16:131088];get=lambda a,n:p[a-0x8000:a-0x8000+n];word=lambda a:int.from_bytes(get(a,2),'little')
lines=['package contra\n\n// Japanese bank0/1, pinned ROM; tables are semantic spawn programs, not executable code.\nvar baseSoldierWaves = [2][][]uint8{']
for i,count in [(0,5),(1,8)]:
 table=word(0x8dde+i*2);lines.append('{')
 for room in range(count):
  start=word(table+room*2);at=start
  while not get(at+1,1)[0]&128:at+=2;assert at-start<100
  lines.append('{'+','.join(hex(x)for x in get(start,at-start+2))+'},')
 lines.append('},')
lines.append('}\nvar baseRollerWaves = [][]uint8{')
for i in range(2):
 start=word(0x9634+i*2);at=start
 while get(at,1)[0]!=255:at+=2;assert at-start<100
 lines.append('{'+','.join(hex(x)for x in get(start,at-start+1))+'},')
lines.append('}\nvar baseRedBlueWaves = [...]uint8{'+','.join(hex(x)for x in get(0xa383,28))+'}\n')
path=ROOT/'games/contra/engine/enemies_base_tables.go';path.write_text('\n'.join(lines));subprocess.run(['gofmt','-w',str(path)],check=True)
print(hashlib.sha256(path.read_bytes()).hexdigest())
