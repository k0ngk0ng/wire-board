"""Unmodified 9949 / 9417 player-hoop calls and a complete action stage from the incident.

Controlled replay only: initial RAM is encoded from the saved semantic state.
The instruction stream and MMC3 mappings remain original; this is not a ROM
runtime dependency or evidence that every case arose in a natural match.
"""
import gzip, hashlib, itertools, json
from verify_ball18 import ROOT, ROM, PRG, SoundMemory, MPU, put
from encode_world_probe import encode_world_probe
from extract_world_actions import decode_world_action_state
from extract_world_preparation import context


def world(r):
    value=decode_world_action_state(r)
    for i in range(2):value['auxiliary'][i]['kind']=r[0x3da+i]
    return value


def run(r, actor, entry=0xe165, exit=0xe168):
    m=SoundMemory(PRG,r);cpu=MPU(memory=m,pc=entry);cpu.x=actor
    calls=[];sounds=[];prepared=None;shot_source=None
    for steps in range(500000):
        if cpu.pc==exit:break
        if cpu.pc==0xe18b:calls.append(cpu.x)
        if cpu.pc==0xfbcc:sounds.append(cpu.a)
        if cpu.pc==0x9448:shot_source=cpu.x
        if cpu.pc==0x9986 and prepared is None:prepared=bytes(m[:2048])
        cpu.step()
    else:raise RuntimeError(('original did not return',hex(cpu.pc)))
    return bytes(m[:2048]),calls,sounds,prepared,shot_source


def row(r, actor):
    final,calls,sounds,prepared,shot_source=run(r,actor)
    after=world(final)
    ctx=context(r)['actions'];ctx['actorHeldButtons']=list(r[0x4e0:0x4e4])
    return dict(actor=actor,backboardWrite=shot_source is not None,backboardStrength=final[0x56d]>>6,held=list(r[0x4e0:0x4e4]),before=world(r),after=after,prepared=world(prepared) if prepared else None,context=ctx,calls=calls,sounds=sounds)


def main():
    assert hashlib.sha256(ROM).hexdigest()=='0fa28f60abb8d2ccbbeee90e79e3fa160a5cf32feb90f47ebf1a89168cff569b'
    origin=ROOT/'games/basketball/engine/testdata/hoop-air-throw-runtime.json.gz'
    snapshot=json.loads(gzip.decompress(origin.read_bytes()))
    basefile=ROOT/'.local/nes-basket/runs/court-4-reference/ram.bin'
    base=basefile.read_bytes()[1500*2048:1501*2048];assert len(base)==2048
    patch,_=encode_world_probe(snapshot);ram=bytearray(base)
    for a,v in patch.items():ram[a]=v
    ram[:8]=bytes(8)
    prepared,_,_,_,_=run(ram,0,0xd314,0xd345)
    final,calls,sounds,_,_=run(prepared,0,0xd345,0xd348)
    stage=dict(before=world(prepared),after=world(final),context=context(prepared)['actions'],calls=calls,sounds=sounds)
    stages=[stage]
    for held in [0,8]:
        r=bytearray(prepared);r[0x512]=0x3c;r[0x51f]=128;r[0x4e0]=held;r[0x56d]=0xc0
        final,phase_calls,phase_sounds,_,source=run(r,0,0xd345,0xd348)
        stages.append(dict(before=world(r),after=world(final),held=list(r[0x4e0:0x4e4]),backboardWrite=source is not None,backboardStrength=final[0x56d]>>6,context=context(r)['actions'],calls=phase_calls,sounds=phase_sounds))
    rows=[]
    for actor,entered,left,owner,air,action,held,court in itertools.product(range(4),[False,True],[False,True],range(7),[False,True],[0x3a,0x3c],[0,8],[1,4]):
        r=bytearray(prepared)
        for a,v in {0x512:action|(128 if entered else 0),0x51f:128,0x437:254 if entered else 3,0x444:0,0x451:0,0x52c:255,0x46b:128 if left else 0,0x5f7:128 if air else 0,0x642:64}.items():r[a+actor]=v
        r[0x4e0+actor]=held;r[0x56d]=0xc0;r[0x58b]=court
        r[0x569]=255;r[0x540:0x546]=bytes([15]*6)
        if owner<6:r[0x540+owner]=actor
        put(r,300*256+123,0x68+actor,0x75+actor,0x82+actor)
        put(r,45*256+71,0x8f+actor,0x9c+actor)
        put(r,(60 if air else 0)*256,0xa9+actor,0xb6+actor,0xc3+actor)
        rows.append(row(r,actor))
    for actor,entered,left,support_air,action,held,court in itertools.product(range(4),[False,True],[False,True],[False,True],[0x3a,0x3c],[0,8],[1,4]):
        r=bytearray(prepared);support=actor^1
        for a,v in {0x512:action|(128 if entered else 0),0x51f:128,0x437:254 if entered else 3,0x444:0,0x451:0,0x52c:255,0x46b:128 if left else 0,0x5f7:128,0x642:128|support}.items():r[a+actor]=v
        r[0x4e0+actor]=held;r[0x56d]=0xc0;r[0x58b]=court
        r[0x569]=actor;r[0x540:0x546]=bytes([15]*6);r[0x545]=actor
        r[0x642+support]=64;r[0x5f7+support]=128 if support_air else 0
        for i in [actor,support]:
            put(r,300*256+71,0x68+i,0x75+i,0x82+i);put(r,45*256+19,0x8f+i,0x9c+i)
            put(r,90*256 if i==actor else 60*256 if support_air else 0,0xa9+i,0xb6+i,0xc3+i)
        rows.append(row(r,actor))
    out=ROOT/'games/basketball/engine/testdata';packed=gzip.compress((json.dumps(dict(calls=rows,stages=stages),separators=(',',':'))+'\n').encode(),mtime=0)
    (out/'hoop-air-throw-original.json.gz').write_bytes(packed)
    metadata=dict(romSHA256=hashlib.sha256(ROM).hexdigest(),sourceRuntimeSHA256=hashlib.sha256(origin.read_bytes()).hexdigest(),baseRAMSHA256=hashlib.sha256(base).hexdigest(),fixtureSHA256=hashlib.sha256(packed).hexdigest(),entries=['bank6:9949 and 9417 via fixed:E165..E168','fixed:D314..D345 preparation','fixed:D345..D348 full action phase'],scope='isolated original action calls from controlled initial RAM plus original whole action stage at the incident boundary',naturalGameplay=False,calls=len(rows),wholeStages=len(stages),stageCalls=calls)
    (out/'hoop-air-throw-source.json').write_text(json.dumps(metadata,indent=2)+'\n');print(json.dumps(metadata))

if __name__=='__main__':main()
