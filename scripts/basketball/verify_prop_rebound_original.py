"""Run B1BD with synthetic starts and unmodified original MMC3-backed physics."""
import gzip, hashlib, itertools, json
from verify_pass_floor_original import ROOT, Memory, MPU, put
from extract_world_actions import decode_world_action_state

def main():
    rom=(ROOT/'.local/nes-basket/basketball.nes').read_bytes();source=ROOT/'.local/nes-basket/runs/ball-pass/ram.bin';ram=source.read_bytes()[:2048]
    rows=[]
    for case,(actor,facing,x,entered,height,depth) in enumerate(itertools.product([4,5],[0,128],[48,200,464],[False,True],[0,127,512],[71,72])):
        r=bytearray(ram)
        for a,v in {0x512:7|(128 if entered else 0),0x46b:facing,0x437:0,0x444:0,0x451:0,0x52c:255,0x539:255 if entered else 2,0x642:64 if entered else 128,0x56b:2,0x51f:128,0x5f7:0}.items():r[a+actor]=v
        r[0x56b]=3
        put(r,x*256+123,0x68+actor,0x75+actor,0x82+actor);put(r,depth*256+39,0x8f+actor,0x9c+actor);put(r,height,0xa9+actor,0xb6+actor,0xc3+actor)
        for addresses,value in [((0x478,0x485),-257 if facing else 257),((0x492,0x49f),33),((0x4ac,0x4b9),-128),((0x4c6,0x4d3),37)]:put(r,value,*(a+actor for a in addresses))
        for tick in range(80):
            memory=Memory(rom[16:16+131072],r);cpu=MPU(memory=memory);cpu.sp=255;cpu.stPushWord(0x6fff);cpu.pc=0xb1bd;cpu.x=actor;cpu.p=0x20
            before=decode_world_action_state(r)['auxiliary'][actor-4];holder=r[0x56b]
            for steps in range(20000):
                if cpu.pc==0x7000:break
                cpu.step()
            assert cpu.pc==0x7000,hex(cpu.pc)
            r=bytearray(memory[:2048]);after=decode_world_action_state(r)['auxiliary'][actor-4]
            rows.append(dict(case=case,tick=tick,actor=actor,before=before,after=after,specialHolder=holder if holder<128 else holder-256,afterSpecialHolder=r[0x56b] if r[0x56b]<128 else r[0x56b]-256))
            if after['action']!=7:break
    p=ROOT/'games/basketball/engine/testdata/prop-rebound-isolated.json.gz';p.write_bytes(gzip.compress((json.dumps(rows,separators=(',',':'))+'\n').encode(),mtime=0))
    p.with_name('prop-rebound-isolated-source.json').write_text(json.dumps(dict(records=len(rows),natural_gameplay=False,isolated_original_function_execution=True,entry='bank6:B1BD',original_instructions_unchanged=True,rom_sha256=hashlib.sha256(rom).hexdigest(),ram_source=str(source.relative_to(ROOT)),ram_sha256=hashlib.sha256(ram).hexdigest(),sha256=hashlib.sha256(p.read_bytes()).hexdigest()),indent=2)+'\n')
    print('original prop rebound calls',len(rows))

if __name__=='__main__':main()
