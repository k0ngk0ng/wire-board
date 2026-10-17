"""Unmodified bank 6 BBFD oracle, through the original action dispatcher."""
import gzip, hashlib, itertools, json
from verify_ball18 import ROOT, ROM, PRG, BASE, SoundMemory, initial as base_initial
from verify_pass_floor_original import MPU, put
from extract_world_actions import decode_world_action_state
from extract_world_preparation import context

def initial(left=False, entered=False, variant='normal', cursor=0, loops=12):
    r=base_initial(left,entered)
    r[0x518]=17|(128 if entered else 0)
    r[0x44a]=cursor;r[0x457]=loops;r[0x532]=12 if entered else 255
    put(r,(-6 if left else 6)*256,0x47e,0x48b)
    if variant=='ground':
        put(r,0,0xaf,0xbc,0xc9);put(r,0,0x4b2,0x4bf);put(r,0,0x4cc,0x4d9)
    elif variant=='descending':put(r,-3000,0x4b2,0x4bf)
    elif variant=='wrap':put(r,32700,0x4b2,0x4bf);put(r,32700,0x4cc,0x4d9)
    elif variant=='moving-depth':put(r,-537,0x498,0x4a5)
    elif variant=='special':r[0x554]=128
    elif variant=='owner-six':r[0x53f]=6
    return r

def run(m):
    before=bytes(m[:2048]);cpu=MPU(memory=m,pc=0xe149);cpu.x=6;sounds=[];prepared=None;motion=None
    for _ in range(30000):
        if cpu.pc==0xbc29 and prepared is None:prepared=bytes(m[:2048])
        if cpu.pc==0xbc2f and motion is None:motion=bytes(m[:2048])
        if cpu.pc==0xfbcc:sounds.append(cpu.a)
        if cpu.pc==0xe14c:break
        cpu.step()
    else:raise RuntimeError(hex(cpu.pc))
    return dict(before=decode_world_action_state(before),prepared=decode_world_action_state(prepared) if prepared else None,afterMotion=decode_world_action_state(motion) if motion else None,after=decode_world_action_state(bytes(m[:2048])),context=context(before)['actions'],sounds=sounds)

def main():
    calls=[]
    for left,entered,variant in itertools.product([False,True],[False,True],['normal','ground','descending','wrap','moving-depth','special','owner-six']):
        calls.append(dict(label=f'{left}-{entered}-{variant}',**run(SoundMemory(PRG,initial(left,entered,variant)))))
    for left,cursor,loops in itertools.product([False,True],range(4),[0,1,12,255]):
        calls.append(dict(label=f'animation-{left}-{cursor}-{loops}',**run(SoundMemory(PRG,initial(left,True,cursor=cursor,loops=loops)))))
    chains=[]
    for left,variant in itertools.product([False,True],['normal','moving-depth','ground']):
        m=SoundMemory(PRG,initial(left,variant=variant));rows=[]
        for tick in range(500):
            rows.append(run(m))
            if m[0x518]&127==6:break
        else:raise RuntimeError(('chain never bounced',left,variant))
        chains.append(dict(left=left,variant=variant,records=rows))
    out=ROOT/'games/basketball/engine/testdata'
    packed=gzip.compress((json.dumps(dict(calls=calls,chains=chains),separators=(',',':'))+'\n').encode(),mtime=0)
    (out/'ball17.json.gz').write_bytes(packed)
    meta=dict(entry='bank6 BBFD through original E149..E14C action dispatch',scope='controlled original calls and continuous original action chains; not natural menu gameplay',romSHA256=hashlib.sha256(ROM).hexdigest(),baseSHA256=hashlib.sha256(BASE).hexdigest(),fixtureSHA256=hashlib.sha256(packed).hexdigest(),calls=len(calls),chains=[len(c['records']) for c in chains])
    (out/'ball17-source.json').write_text(json.dumps(meta,indent=2)+'\n');print(json.dumps(meta))
if __name__=='__main__':main()
