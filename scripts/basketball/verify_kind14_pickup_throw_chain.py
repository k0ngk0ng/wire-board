"""Original pickup/A-throw chain from an explicit isolated Court3 setup.

Only the starting positions/state and subsequent physical controller samples are
supplied. Input, request acceptance, pickup, carry, release, motion and shape
selection execute unchanged original functions. This is not a natural match.
"""
import gzip,hashlib,json
from verify_pass_floor_original import ROOT,Memory,MPU,put
from extract_world_actions import decode_world_action_state

def main():
    rom=(ROOT/'.local/nes-basket/basketball.nes').read_bytes();prg=rom[16:16+131072]
    source=ROOT/'.local/nes-basket/runs/court-3-reference/ram.bin';source_ram=source.read_bytes()[1500*2048:1501*2048]
    records=[];chains=[]
    for spare_buttons in (0,0x41):
        r=bytearray(source_ram);assert r[0x58b]==3 and r[0x3db]==14
        for actor in (0,5):
            put(r,300*256+37,0x68+actor,0x75+actor,0x82+actor);put(r,40*256+17,0x8f+actor,0x9c+actor);put(r,0,0xa9+actor,0xb6+actor,0xc3+actor)
            for a,b in [(0x478,0x485),(0x492,0x49f),(0x4ac,0x4b9),(0x4c6,0x4d3)]:put(r,0,a+actor,b+actor)
            for a,v in {0x512:128,0x51f:128,0x52c:255,0x437:0,0x444:0,0x451:0,0x46b:0,0x5f7:0,0x642:64,0x5b:0}.items():r[a+actor]=v
        r[0x53d]=255;r[0x53e]=255;r[0x53f]=6;r[0x569]=255;r[0x56b]=255;r[0x550]=255;r[0x539]=0;r[0x4e8]=0;r[0x4ec]=255
        for a in (0x4f9,0x4fd,0x501,0x505,0x509,0x50d):r[a]=0
        memory=Memory(prg,r);cpu=MPU(memory=memory);sounds=[];trace=[]
        def call(address,actor,bank=6):
            memory[0x8000:0xc000]=prg[bank*16384:(bank+1)*16384]
            cpu.sp=255;cpu.stPushWord(0x6fff);cpu.pc=address;cpu.x=actor;cpu.p=0x20
            for steps in range(100000):
                if cpu.pc==0x7000:return
                if cpu.pc==0xc000:sounds.append(cpu.a);cpu.pc=(cpu.stPopWord()+1)&65535
                else:cpu.step()
            raise RuntimeError(('did not return',hex(cpu.pc)))
        last=0;second_press=None;seen_pickup=False;seen_release=False
        for tick in range(100):
            if second_press is None and memory[0x53e]==0 and memory[0x512]&127==0x20:
                second_press=tick+3
            held=128 if tick<4 or second_press is not None and second_press<=tick<second_press+4 else 0
            memory[0:8]=[held,0,0,spare_buttons,held&~last,0,0,spare_buttons if tick==0 else 0];last=held
            if memory[0x517]&127==3:
                before=bytes(memory[:2048]);call(0x8318,5,5);shape=memory[0x2c]+256*memory[0x2d]
                records.append(dict(chain=spare_buttons,tick=tick,world=decode_world_action_state(before),controllers=[dict(held=before[i],pressed=before[i+4]) for i in range(4)],kind=14,action=3,frame=before[0x449],shapeAddress=shape,shape=list(memory[shape:shape+8]),flags=0x28))
                seen_release=True
            call(0x805c,0);request=memory[0x51f]
            call(0xe18b,0);call(0xe18b,5)
            seen_pickup|=memory[0x53e]==0
            trace.append(dict(tick=tick,held=held,request=request,playerAction=memory[0x512],auxAction=memory[0x517],auxOwner=memory[0x53e],auxCursor=memory[0x449],poseSource=memory[0x56a]))
            if seen_release and memory[0x517]&127==0:break
        assert seen_pickup and seen_release,trace
        chains.append(dict(spareControllerButtons=spare_buttons,secondPress=second_press,trace=trace,sounds=sounds))
    out=ROOT/'games/basketball/engine/testdata/kind14-pickup-throw-contact.json.gz';out.write_bytes(gzip.compress((json.dumps(records,separators=(',',':'))+'\n').encode(),mtime=0))
    out.with_name('kind14-pickup-throw-chain.json').write_text(json.dumps(chains,indent=2)+'\n')
    out.with_name('kind14-pickup-throw-source.json').write_text(json.dumps(dict(records=len(records),chains=len(chains),isolated_original_function_chain=True,natural_gameplay=False,entries=['bank6:805C','fixed:E18B','bank5:8318'],method='Explicit Court3 start; only physical controller samples change after start; original input/request/action/motion functions execute; C000 sound sink only',ram_source=str(source.relative_to(ROOT)),ram_frame=1501,ram_sha256=hashlib.sha256(source_ram).hexdigest(),rom_sha256=hashlib.sha256(rom).hexdigest(),sha256=hashlib.sha256(out.read_bytes()).hexdigest()),indent=2)+'\n')
    print('original Kind14 pickup/throw contacts',len(records),'chains',len(chains))

if __name__=='__main__':main()
