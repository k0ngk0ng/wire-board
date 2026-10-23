"""Original shared hoop-player handlers and complete lob action chains.

Offline controlled oracle only: initial semantic state is explicitly supplied;
original instructions/MMC3 execute unchanged. Not a runtime ROM dependency.
"""
import gzip, hashlib, itertools, json
from verify_ball18 import ROOT, ROM, PRG, BASE, SoundMemory, MPU, put
from encode_world_probe import encode_world_probe
from extract_world_actions import decode_world_action_state
from extract_world_preparation import context

def world(r):
    value=decode_world_action_state(r)
    for i in range(2):value['auxiliary'][i]['kind']=r[0x3da+i]
    value['hoopHeadings']=[r[0x4f3+i]//32 if r[0x4f3+i]!=255 else 8 for i in range(6)]
    return value

def run(r, actor=0, whole=False):
    reset_before=world(r)
    if whole:
        reset=SoundMemory(PRG,r);reset[0x8000:0xc000]=PRG[5*16384:6*16384];cpu=MPU(memory=reset,pc=0x8469)
        for _ in range(1000):
            if cpu.pc==0x8498:break
            cpu.step()
        else:raise RuntimeError('contact reset did not return')
        r=bytes(reset[:2048])
    m=SoundMemory(PRG,r);cpu=MPU(memory=m,pc=0xd345 if whole else 0xe165);cpu.x=actor
    end=0xd348 if whole else 0xe168;calls=[];sounds=[];sound_objects=[]
    for _ in range(500000):
        if cpu.pc==end:break
        if cpu.pc==0xe18b:calls.append(cpu.x)
        if cpu.pc==0xfbcc:sounds.append(cpu.a);sound_objects.append(m[0x3d3])
        cpu.step()
    else:raise RuntimeError(hex(cpu.pc))
    after=bytes(m[:2048])
    return after,dict(held=list(r[0x4e0:0x4e4]),pressed=list(r[0x4e4:0x4e8]),resetBefore=reset_before,actor=actor,before=world(r),after=world(after),context=context(r)['actions'],calls=calls,sounds=sounds,soundObjects=sound_objects)

def main():
    assert hashlib.sha256(ROM).hexdigest()=='0fa28f60abb8d2ccbbeee90e79e3fa160a5cf32feb90f47ebf1a89168cff569b'
    source=ROOT/'games/basketball/engine/testdata/hoop-air-throw-runtime.json.gz'
    snapshot=json.loads(gzip.decompress(source.read_bytes()))
    base=(ROOT/'.local/nes-basket/runs/court-4-reference/ram.bin').read_bytes()[1500*2048:1501*2048]
    patch,_=encode_world_probe(snapshot);initial=bytearray(base)
    for a,v in patch.items():initial[a]=v
    initial[:8]=bytes(8);initial[0x569]=255
    rows=[]
    def setup(action,actor,entered,left,air,owner):
        r=bytearray(initial)
        for a,v in {0x512:action|(128 if entered else 0),0x51f:128,0x437:3 if entered else 0,0x444:0,0x451:0,0x52c:255,0x46b:128 if left else 0,0x5f7:128 if air else 0,0x642:64,0x4ec:192 if left else 64}.items():r[a+actor]=v
        r[0x540:0x546]=bytes([15]*6)
        if owner<6:r[0x540+owner]=actor
        put(r,300*256+123,0x68+actor,0x75+actor,0x82+actor);put(r,45*256+71,0x8f+actor,0x9c+actor)
        put(r,60*256 if air else 0,0xa9+actor,0xb6+actor,0xc3+actor)
        return r
    for action,actor,entered,left,air,owner in itertools.product([0x31,0x33,0x37,0x38],range(4),[False,True],[False,True],[False,True],range(7)):
        r=setup(action,actor,entered,left,air,owner);_,record=run(r,actor);rows.append(record)
    for action,actor,heading,computer,cursor in itertools.product([0x31,0x33,0x37,0x38],range(4),range(9),[False,True],[0,1]):
        r=setup(action,actor,True,False,False,0);r[0x4ec+actor]=255 if heading==8 else heading*32;r[0x539+actor]=128 if computer else actor;r[0x444+actor]=cursor if action in [0x37,0x38] else 0;r[0x437+actor]=0
        _,record=run(r,actor);rows.append(record)
    for action,actor,support_air in itertools.product([0x31,0x33,0x37,0x38],range(4),[False,True]):
        r=setup(action,actor,True,False,True,5);support=actor^1;r[0x569]=actor;r[0x642+actor]=128|support;r[0x642+support]=64;r[0x5f7+support]=128 if support_air else 0
        put(r,300*256+71,0x68+support,0x75+support,0x82+support);put(r,45*256+19,0x8f+support,0x9c+support);put(r,60*256 if support_air else 0,0xa9+support,0xb6+support,0xc3+support)
        _,record=run(r,actor);rows.append(record)
    def other_hoops(r,index):
        # Independently isolate the selected release; otherwise the online
        # incident's old carried basket would retain action5 with no owner.
        for a,v in {0x518:128,0x525:128,0x43d:0,0x44a:0,0x457:0,0x532:255,0x5fd:0,0x53f:6,0x511:0}.items():r[a]=v
        put(r,0,0xaf,0xbc,0xc9)
        for a,b in [(0x47e,0x48b),(0x498,0x4a5),(0x4b2,0x4bf),(0x4cc,0x4d9)]:put(r,0,a,b)
        for j in range(6):
            if j==index:continue
            i=j+7;r[0x512+i]=128;r[0x51f+i]=128;r[0x539+i]=15;r[0x5f7+i]=0
            put(r,0,0xa9+i,0xb6+i,0xc3+i)
            for a,b in [(0x478,0x485),(0x492,0x49f),(0x4ac,0x4b9),(0x4c6,0x4d3)]:put(r,0,a+i,b+i)
    for actor,entered,vx,vy,left in itertools.product([4,5],[False,True],[-96,0,32,128],[-96,0,32,128],[False,True]):
        r=setup(0x30,0,True,False,False,0)
        for a,v in {0x512:4|(128 if entered else 0),0x51f:128,0x437:0,0x444:0,0x451:0,0x52c:255,0x46b:128 if left else 0,0x539:255,0x642:64,0x5f7:0}.items():r[a+actor]=v
        put(r,300*256+71,0x68+actor,0x75+actor,0x82+actor);put(r,45*256+19,0x8f+actor,0x9c+actor);put(r,0,0xa9+actor,0xb6+actor,0xc3+actor)
        put(r,vx,0x478+actor,0x485+actor);put(r,vy,0x492+actor,0x49f+actor);put(r,0,0x4ac+actor,0x4b9+actor);put(r,0,0x4c6+actor,0x4d3+actor)
        _,record=run(r,actor);rows.append(record)
    phases=[]
    for index,entered,left,heading,air in itertools.product(range(6),[False,True],[False,True],range(9),[False,True]):
        r=setup(0x37,0,True,False,False,index);r[0x444]=2;r[0x437]=0;r[0x52c]=0;i=index+7;other_hoops(r,index)
        for a,v in {0x512:7|(128 if entered else 0),0x51f:128,0x437:0,0x444:0,0x451:0,0x52c:255,0x46b:128 if left else 0,0x5f7:128 if air else 0,0x4ec:255 if heading==8 else heading*32}.items():r[a+i]=v
        put(r,300*256+71,0x68+i,0x75+i,0x82+i);put(r,45*256+19,0x8f+i,0x9c+i);put(r,60*256 if air else 0,0xa9+i,0xb6+i,0xc3+i)
        _,record=run(r,whole=True);phases.append(record)
    for action,support_air in itertools.product([0x31,0x33,0x37,0x38],[False,True]):
        r=bytearray(BASE);assert r[0x58b]==1
        other_hoops(r,0)
        for actor in range(4):
            for a,v in {0x512:128,0x51f:128,0x437:0,0x444:0,0x451:0,0x52c:255,0x642:0,0x5f7:0}.items():r[a+actor]=v
            put(r,(200+actor)*256,0x68+actor,0x75+actor,0x82+actor);put(r,40*256,0x8f+actor,0x9c+actor);put(r,0,0xa9+actor,0xb6+actor,0xc3+actor)
            for a,b in [(0x478,0x485),(0x492,0x49f),(0x4ac,0x4b9),(0x4c6,0x4d3)]:put(r,0,a+actor,b+actor)
        r[0x512]=action;r[0x642]=129;r[0x5f7]=128;r[0x569]=0;r[0x4ec]=64;r[0x539]=0
        r[0x5f8]=128 if support_air else 0;put(r,60*256 if support_air else 0,0xaa,0xb7,0xc4)
        put(r,90*256,0xa9,0xb6,0xc3);r[0x540]=0;r[0x519]=133;r[0x526]=128
        _,record=run(r,whole=True);phases.append(record)
    chains=[]
    for action,left,index in itertools.product([0x37,0x38],[False,True],range(6)):
        r=setup(action,0,False,left,action==0x38,index);other_hoops(r,index);records=[];seen=False
        # Player and owned basket start together; all subsequent writes come
        # from complete original action phases, with no state repair/reset.
        i=index+7;put(r,300*256+71,0x68+i,0x75+i,0x82+i);put(r,45*256+19,0x8f+i,0x9c+i);put(r,(90 if action==0x38 else 30)*256,0xa9+i,0xb6+i,0xc3+i)
        r[0x512+i]=5;r[0x51f+i]=128
        for tick in range(140):
            r,record=run(r,whole=True);records.append(record);seen|=(r[0x512+i]&127)==7
            if seen and r[0x512+i]&127==0:break
        assert seen and r[0x512+i]&127==0,(action,left,index)
        chains.append(dict(action=action,left=left,index=index,records=records))
    payload=gzip.compress((json.dumps(dict(calls=rows,phases=phases,chains=chains),separators=(',',':'))+'\n').encode(),mtime=0)
    out=ROOT/'games/basketball/engine/testdata';(out/'hoop-shared-original.json.gz').write_bytes(payload)
    report=dict(romSHA256=hashlib.sha256(ROM).hexdigest(),initialRAMSHA256=hashlib.sha256(initial).hexdigest(),sourceRuntimeSHA256=hashlib.sha256(source.read_bytes()).hexdigest(),fixtureSHA256=hashlib.sha256(payload).hexdigest(),calls=len(rows),phases=len(phases),reaEvidence='ev_1ad622c6f6d8c5c1804e844fa9f245547533fd4359788aab647902a76e77679a',chains=[len(c['records']) for c in chains],entries=['bank6:8810,88E7,92B2,9363,B117 via E165..E168','fixed:DF39 via bank5:8469..8498 reset and complete D345..D348'],naturalGameplay=False,scope='controlled original calls, complete action phases and uninterrupted original release-to-landing chains')
    (out/'hoop-shared-source.json').write_text(json.dumps(report,indent=2)+'\n');print(json.dumps(report))

if __name__=='__main__':main()
