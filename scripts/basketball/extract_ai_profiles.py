"""Extract the original 8-by-4 pursuit parameter table as numeric evidence."""
from pathlib import Path
import hashlib
import json

ROOT=Path(__file__).resolve().parents[2]

def main():
    rom=(ROOT/'.local/nes-basket/basketball.nes').read_bytes()
    digest=hashlib.sha256(rom).hexdigest()
    assert digest=='0fa28f60abb8d2ccbbeee90e79e3fa160a5cf32feb90f47ebf1a89168cff569b'
    bank=rom[16+5*16384:16+6*16384]
    def word(addr):return int.from_bytes(bank[addr-0x8000:addr-0x8000+2],'little')
    records=[]
    for team in range(8):
        team_ptr=word(0x94af+team*2)
        assert team_ptr==0x94bf+team*8
        for character in range(4):
            pointer=word(team_ptr+character*2)
            records.append(dict(team=team,character=character,options=[dict(destination=bank[pointer-0x8000+i*2],parameter=bank[pointer-0x8000+i*2+1]) for i in range(4)]))
    p=ROOT/'games/basketball/engine/testdata/ai-pursuit-profiles.json';p.write_text(json.dumps(records,indent=2)+'\n')
    p.with_name('ai-pursuit-profiles-source.json').write_text(json.dumps(dict(source_is_original_rom_data=True,rom_sha256=digest,source='bank5:94af team/character tables',records=len(records),sha256=hashlib.sha256(p.read_bytes()).hexdigest()),indent=2)+'\n')
    print(json.dumps(dict(records=len(records))))

if __name__=='__main__':main()
