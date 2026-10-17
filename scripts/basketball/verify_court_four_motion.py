"""Configured Court 4 EEFF instruction oracle; not natural-match evidence."""
import gzip,hashlib,itertools,json,sys
from pathlib import Path
ROOT=Path(__file__).resolve().parents[2]
sys.path.insert(0,str(ROOT/'.local/basketball/python'))
from verify_pass_floor_original import Memory,MPU
from verify_court_floor import initial
from verify_court_profiles import image,profiles,EXPECTED


def main():
    rom=(ROOT/'.local/nes-basket/basketball.nes').read_bytes()
    assert hashlib.sha256(rom).hexdigest()==EXPECTED
    base=image();grid=profiles(base)[4];trace=bytearray();rows=[]
    for obj,direction,x,y,rider,seed in itertools.product([4,5],[0,1],[54,55,56,57,200,470,471,472],[55,56,63,71,72],[False,True],[0x12345678,0xabcdef01]):
        height=grid[y//8][x//8]&248
        m=initial(base,4,obj,x,y,x,y,height*256,height,1)
        m[0x48:0x4c]=seed.to_bytes(4,'big');m[0x6d8]=direction
        for i in range(13):
            m[0x642+i]=0;m[0x5f7+i]=0
            for a in [0x478,0x485,0x492,0x49f,0x4ac,0x4b9,0x4c6,0x4d3]:m[a+i]=0
        m[0x512+obj]=128;m[0x539+obj]=255;m[0x3d6+obj]=10 if obj==4 else 11
        if rider:
            m[0x642]=128|obj;m[0x512]=128|66;m[0x478]=37;m[0x485]=2
        mem=Memory(rom[16:16+131072],m[:2048]);before=bytes(mem[:2048])
        cpu=MPU(memory=mem);cpu.pc=0xeeff;cpu.x=obj;cpu.sp=255;cpu.p=0x20;cpu.stPushWord(0x6fff)
        sounds=[]
        for n in range(20000):
            if cpu.pc==0x7000:break
            if cpu.pc==0xc000:
                sounds.append(cpu.a);cpu.pc=(cpu.stPopWord()+1)&65535;continue
            cpu.step()
        assert cpu.pc==0x7000,(hex(cpu.pc),obj,direction,x,y)
        trace.extend(before);trace.extend(bytes(mem[:2048]));rows.append(dict(object=obj,sounds=sounds,direction=direction,rider=rider))
    out=ROOT/'games/basketball/engine/testdata';packed=gzip.compress(trace,mtime=0)
    (out/'court-four-motion.ram.gz').write_bytes(packed)
    (out/'court-four-motion.json').write_text(json.dumps(rows,separators=(',',':'))+'\n')
    source=dict(records=len(rows),source_is_original_execution=True,natural_gameplay=False,entry='fixed EEFF',rom_sha256=EXPECTED,fixture_sha256=hashlib.sha256(packed).hexdigest(),unchanged_ROM_instructions=True,method='configured moving auxiliary boundary/direction/rider inputs; MMC3 PRG bank writes modeled; C000 sound sink only')
    (out/'court-four-motion-source.json').write_text(json.dumps(source,indent=2)+'\n');print(json.dumps(source))


if __name__=='__main__':main()
