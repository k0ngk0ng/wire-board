"""Court6 EEFF boundaries and unchanged slot bytes on all13 objects; configured source calls."""
import gzip,hashlib,itertools,json,sys
from pathlib import Path
ROOT=Path(__file__).resolve().parents[2]
sys.path.insert(0,str(ROOT/'.local/basketball/python'))
from verify_pass_floor_original import Memory,MPU
from verify_court_floor import initial
from verify_court_profiles import image,profiles,EXPECTED


def main():
    rom=(ROOT/'.local/nes-basket/basketball.nes').read_bytes();assert hashlib.sha256(rom).hexdigest()==EXPECTED
    base=image();grid=profiles(base)[6];trace=bytearray();rows=[]
    for obj,x,y,variant in itertools.product(range(13),[7,8,39,40,255,256,471,472,503,504],[0,1,70,71],range(4)):
        floor=grid[y//8][x//8]&248
        m=initial(base,6,obj,x,y,x,y,floor*256,floor,1)
        for i in range(13):
            m[0x642+i]=0;m[0x5f7+i]=0;m[0x512+i]=128;m[0x51f+i]=128
            for a in [0x478,0x485,0x492,0x49f,0x4ac,0x4b9,0x4c6,0x4d3]:m[a+i]=0
        m[0x3da:0x3dc]=[12,13];m[0x53d:0x53f]=[255,255];m[0x53f]=6
        m[0x6d7:0x6e4]=[0,2,4,129,132,134,29,61,128,0,0,0,0]
        if variant==1:
            if obj<4:m[0x53f]=obj
            elif obj==6:m[0x53f]=1
            else:m[0x539+obj]=0;m[0x512+obj]=128|5
        elif variant==2:
            m[0x492+obj]=0;m[0x49f+obj]=2
        elif variant==3:
            m[0xb6+obj]=floor+1;m[0x4b9+obj]=254;m[0x5f7+obj]=128
        mem=Memory(rom[16:16+131072],m[:2048]);before=bytes(mem[:2048]);cpu=MPU(memory=mem);cpu.pc=0xeeff;cpu.x=obj;cpu.sp=255;cpu.p=0x20;cpu.stPushWord(0x6fff);sounds=[]
        for n in range(20000):
            if cpu.pc==0x7000:break
            if cpu.pc==0xc000:sounds.append(cpu.a);cpu.pc=(cpu.stPopWord()+1)&65535;continue
            cpu.step()
        assert cpu.pc==0x7000,(hex(cpu.pc),obj,x,y,variant)
        trace.extend(before);trace.extend(bytes(mem[:2048]));rows.append(dict(object=obj,sounds=sounds,variant=variant))
    out=ROOT/'games/basketball/engine/testdata';packed=gzip.compress(trace,mtime=0)
    (out/'court-six-motion.ram.gz').write_bytes(packed);(out/'court-six-motion.json').write_text(json.dumps(rows,separators=(',',':'))+'\n')
    source=dict(records=len(rows),deviceTriggers=sum(21 in row['sounds'] for row in rows),source_is_original_execution=True,natural_gameplay=False,entry='fixed EEFF -> bank4 8249/84D6 (no device)',rom_sha256=EXPECTED,fixture_sha256=hashlib.sha256(packed).hexdigest(),unchanged_ROM_instructions=True,method='all13 object boundary/held/depth-travel/landing cases, retaining slot bytes; MMC3 PRG writes modeled; C000 sound sink only')
    (out/'court-six-motion-source.json').write_text(json.dumps(source,indent=2)+'\n');print(json.dumps(source))


if __name__=='__main__':main()
