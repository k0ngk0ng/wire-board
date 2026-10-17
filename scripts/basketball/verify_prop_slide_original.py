"""Original B3EA isolated action oracle, synthetic starts then consecutive calls."""
import hashlib, itertools, json
from verify_pass_floor_original import ROOT, Memory, MPU, put
from extract_world_actions import decode_world_action_state

def main():
    rom=(ROOT/'.local/nes-basket/basketball.nes').read_bytes()
    source=ROOT/'.local/nes-basket/runs/ball-pass/ram.bin'
    ram=source.read_bytes()[:2048]
    rows=[]
    for case,(actor,facing,x,entered,vx) in enumerate(itertools.product([4,5],[0,128],[48,200,464],[False,True],[-257,0,257])):
        r=bytearray(ram)
        for a,v in {0x512:13|(128 if entered else 0),0x46b:facing,0x437:0,0x444:0,0x451:0,0x52c:0,0x539:255,0x642:64,0x56b:255,0x51f:128,0x5f7:0}.items():r[a+actor]=v
        put(r,x*256+123,0x68+actor,0x75+actor,0x82+actor)
        put(r,70*256+39,0x8f+actor,0x9c+actor)
        put(r,0,0xa9+actor,0xb6+actor,0xc3+actor)
        for addresses,value in [((0x478,0x485),vx),((0x492,0x49f),0),((0x4ac,0x4b9),0),((0x4c6,0x4d3),0)]:put(r,value,*(a+actor for a in addresses))
        for tick in range(80):
            memory=Memory(rom[16:16+131072],r)
            cpu=MPU(memory=memory);cpu.sp=255;cpu.stPushWord(0x6fff);cpu.pc=0xb3ea;cpu.x=actor;cpu.p=0x20
            before=decode_world_action_state(r)['auxiliary'][actor-4]
            for steps in range(20000):
                if cpu.pc==0x7000:break
                cpu.step()
            assert cpu.pc==0x7000,hex(cpu.pc)
            r=bytearray(memory[:2048]);after=decode_world_action_state(r)['auxiliary'][actor-4]
            rows.append(dict(case=case,tick=tick,actor=actor,before=before,after=after))
            if after['action']!=13:break
    out=ROOT/'games/basketball/engine/testdata/prop-slide-isolated.json'
    out.write_text(json.dumps(rows,separators=(',',':'))+'\n')
    out.with_name('prop-slide-isolated-source.json').write_text(json.dumps(dict(natural_gameplay=False,isolated_original_function_execution=True,entry='bank6:B3EA',unchanged_ROM_instructions=True,records=len(rows),rom_sha256=hashlib.sha256(rom).hexdigest(),ram_source=str(source.relative_to(ROOT)),ram_sha256=hashlib.sha256(ram).hexdigest(),sha256=hashlib.sha256(out.read_bytes()).hexdigest(),device_model='MMC3 PRG registers 6/7; other device writes rejected'),indent=2)+'\n')
    print('original slide calls',len(rows))

if __name__=='__main__':main()
