"""Extract semantic column tiles/attributes and independently execute BC74.

Owner FF in auxiliary B07E becomes Y=127 in DFF8; its height reads 0135/0142,
the most recently prepared lower tile row19 and upper attribute row2.
"""
import gzip,hashlib,json,sys
from pathlib import Path
ROOT=Path(__file__).resolve().parents[2];sys.path.insert(0,str(ROOT/'.local/basketball/python'))
from py65.devices.mpu6502 import MPU


def main():
    rom=(ROOT/'.local/nes-basket/basketball.nes').read_bytes();prg=rom[16:16+131072]
    digest=hashlib.sha256(rom).hexdigest();assert digest=='0fa28f60abb8d2ccbbeee90e79e3fa160a5cf32feb90f47ebf1a89168cff569b'
    bank=prg[16384:32768]
    def word(address):return int.from_bytes(bank[address-0x8000:address-0x8000+2],'little')
    def data(address,count):return list(bank[address-0x8000:address-0x8000+count])
    tiles=[];attributes=[]
    for court in range(8):
        t,a=word(0x800c+court*2),word(0x801c+court*2)
        tiles.append(sum([data(word(word(t+4+page*2)+19*2),32) for page in range(2)],[]))
        attributes.append([data(word(a+page*2)+16,8) for page in range(2)])
    rows=[]
    for court in range(8):
        for phase in [0,1]:
            for column in range(64):
                cpu=MPU();cpu.memory[0x8000:0xc000]=bank;cpu.memory[0xc000:]=prg[-16384:];cpu.memory[0x58b]=court;cpu.memory[0x589]=phase;cpu.memory[0x3a1]=column;cpu.pc=0xbc74;cpu.sp=255;cpu.stPushWord(0x6fff)
                for count in range(10000):
                    if cpu.pc==0x7000:break
                    cpu.step()
                assert cpu.pc==0x7000
                rows.append(dict(court=court,phase=phase,column=column,low=cpu.memory[0x135],high=cpu.memory[0x142]))
    out=ROOT/'games/basketball/engine';lines=['// Generated from original bank1 court row/attribute tables. See COURT_FIVE_INTEGRATION.md.','package engine','','var originalLaunchColumnTiles = [8][64]uint8{']
    lines += ['{'+','.join(map(str,row))+'},' for row in tiles];lines+=['}','var originalLaunchColumnAttributes = [8][2][8]uint8{']
    lines += ['{'+','.join('{'+','.join(map(str,page))+'}' for page in row)+'},' for row in attributes];lines+=['}','']
    (out/'background_launch_tables.go').write_text('\n'.join(lines))
    packed=gzip.compress(json.dumps(rows,separators=(',',':')).encode(),mtime=0);(out/'testdata/background-launch-alias.json.gz').write_bytes(packed)
    manifest=dict(records=len(rows),source_is_original_execution=True,natural_gameplay=False,entry='bank1 BC74',rom_sha256=digest,fixture_sha256=hashlib.sha256(packed).hexdigest(),table_sha256=hashlib.sha256((out/'background_launch_tables.go').read_bytes()).hexdigest(),unchanged_ROM_instructions=True)
    (out/'testdata/background-launch-alias-source.json').write_text(json.dumps(manifest,indent=2)+'\n');print(json.dumps(manifest))


if __name__=='__main__':main()
