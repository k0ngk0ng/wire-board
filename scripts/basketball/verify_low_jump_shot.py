"""Isolated unchanged original 9040/E165 actor calls and continuous chains."""
import gzip, hashlib, itertools, json
from verify_ai_mode5_plan11 import ROOT, ROM, PRG, start, machine
from verify_pass_floor_original import put
from extract_world_actions import decode_world_action_state
from extract_world_preparation import context


def call(cpu, actor):
    cpu.memory[0x8000:0xc000]=PRG[6*16384:7*16384]
    cpu.pc,cpu.x,cpu.sp,cpu.p=0xe165,actor,255,0x20
    before=bytes(cpu.memory[:2048]);sounds=[];prepared=None
    for steps in range(20000):
        if cpu.pc==0xe168:break
        if cpu.pc==0x9064 and prepared is None:prepared=bytes(cpu.memory[:2048])
        if cpu.pc==0xfbcc:
            sounds.append(cpu.a);cpu.pc=cpu.stPopWord()+1
        else:cpu.step()
    else:raise RuntimeError(hex(cpu.pc))
    assert prepared is not None
    return dict(actor=actor,before=decode_world_action_state(before),prepared=decode_world_action_state(prepared),after=decode_world_action_state(bytes(cpu.memory[:2048])),context=context(before)['actions'],held=list(before[0x4e0:0x4e4]),sounds=sounds)


def setup(actor,action,variant,height):
    r=start(actor)
    r[0x512+actor]=action;r[0x53f]=actor;r[0x518]=130;r[0x61]=7
    r[0x642+actor]=64;r[0x539+actor]=128;r[0x58d]=128;r[0x572+actor]=0
    r[0x55d]=7;r[0x6f0:0x6f6]=bytes([1]*6);r[0x586:0x588]=bytes([0,1]);r[0x3e3:0x3e7]=bytes([0,1,0,1])
    r[0x6e4:0x6f0]=bytes([255]*12);r[0x4e0:0x4e4]=bytes([0]*4)
    r[0x46b+actor]=0 if actor>=2 else 128
    r[0x437+actor]=7;r[0x444+actor]=1;r[0x451+actor]=9;r[0x52c+actor]=255
    r[0x554+actor]=3;r[0x5f7+actor]=128 if height else 0
    put(r,220*256+37,0x68+actor,0x75+actor,0x82+actor);put(r,40*256+17,0x8f+actor,0x9c+actor);put(r,height*256+7,0xa9+actor,0xb6+actor,0xc3+actor)
    for addr in [0x478,0x492,0x4ac,0x4c6]:put(r,0,addr+actor,addr+13+actor)
    if variant=='high-arc':r[0x6f0]=3
    if variant=='computer-up':r[0x572+actor]=8
    if variant=='human-up':r[0x539+actor]=0;r[0x4e0+actor]=8
    if variant.startswith('equipment'):
        r[0x586+actor//2]=0;r[0x58d]=0;r[0x6e5+3*r[0x3e3+actor]]=int(variant[9:])
    if variant=='special':r[0x586+actor//2]=0;r[0x58d]=0;r[0x554+actor]=128
    return machine(r)


def main():
    rows=[];chains=[]
    for actor,action,variant,height in itertools.product(range(4),[0x5a,0x5c],['normal','high-arc','computer-up','human-up','equipment2','equipment4','special'],[0,55,56,151,152,256]):
        rows.append(call(setup(actor,action,variant,height),actor))
    for actor,action in itertools.product(range(4),[0x5a,0x5c]):
        cpu=setup(actor,action,'normal',0);chain=[]
        for n in range(160):
            chain.append(call(cpu,actor))
            if cpu.memory[0x512+actor]&127 not in (0x5a,0x5c):break
        else:raise RuntimeError('no landing')
        chains.append(chain)
    out=ROOT/'games/basketball/engine/testdata';packed=gzip.compress(json.dumps(dict(calls=rows,chains=chains),separators=(',',':')).encode(),mtime=0)
    (out/'low-jump-shot.json.gz').write_bytes(packed)
    before=ROOT/'.local/basketball/team-roster-after-attacks-20261018/native/court1-teams0-4-characters0-1-0-1/before.json'
    (out/'low-jump-shot-runtime-before.json.gz').write_bytes(gzip.compress(before.read_bytes(),mtime=0))
    source=dict(calls=len(rows),chains=len(chains),chainCalls=sum(map(len,chains)),originalInstructionsUnchanged=True,naturalGameplay=False,entry='fixed E165..E168, bank6 9040; preparation boundary 9064',soundScope='FBCC sink',ROM_SHA256=hashlib.sha256(ROM).hexdigest(),SHA256=hashlib.sha256(packed).hexdigest(),runtimeBeforeSHA256=hashlib.sha256(before.read_bytes()).hexdigest())
    (out/'low-jump-shot-source.json').write_text(json.dumps(source,indent=2)+'\n');print(json.dumps(source))

if __name__=='__main__':main()
