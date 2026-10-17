"""Extract immutable original character parameters, not emulator snapshots."""
from pathlib import Path
import argparse,hashlib,json
ROOT=Path(__file__).resolve().parents[2]
p=argparse.ArgumentParser();p.add_argument('--write-go',action='store_true');args=p.parse_args()
rom=(ROOT/'.local/nes-basket/basketball.nes').read_bytes();assert hashlib.sha256(rom).hexdigest()=='0fa28f60abb8d2ccbbeee90e79e3fa160a5cf32feb90f47ebf1a89168cff569b'
fixed=rom[16+7*16384:16+8*16384]
read=lambda a:fixed[a-0xc000]
word=lambda a:read(a)+256*read(a+1)
rows=[];lines=[]
for team in range(8):
 slots=[]
 for character in range(4):
  addr=word(word(0xc1d8+2*team)+2*character)
  recovery,attack,run,jump,shot=[read(addr+i) for i in range(5)]
  grade=dict(Run=run,Jump=jump,Shot=shot,Recovery=recovery,Attack=attack)
  rows.append(dict(team=team,character=character,address=hex(addr),grades=grade));slots.append('{Run:%d, Jump:%d, Shot:%d, Recovery:%d, Attack:%d}'%(run,jump,shot,recovery,attack))
 lines.append(' {'+', '.join(slots)+'},')
out=ROOT/'games/basketball/engine/testdata';fixture=out/'character-grades.json';fixture.write_text(json.dumps(rows,separators=(',',':'))+'\n')
(out/'character-grades-source.json').write_text(json.dumps(dict(rom_sha256=hashlib.sha256(rom).hexdigest(),table='fixed:C1D8',rows=len(rows),sha256=hashlib.sha256(fixture.read_bytes()).hexdigest(),source='ROM parameter bytes, not runtime observations'),indent=2)+'\n')
if args.write_go:
 (ROOT/'games/basketball/engine/character_grade_data.go').write_text('package engine\n\n// Original team/character parameters from fixed C1D8, decoded by\n// scripts/basketball/extract_character_grades.py. Runtime needs no ROM.\nvar originalCharacterGrades = [8][4]CharacterGrades{\n'+'\n'.join(lines)+'\n}\n')
print('decoded',len(rows),'original character profiles')
