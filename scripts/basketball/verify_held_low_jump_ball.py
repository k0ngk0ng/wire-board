"""Original B585 held-ball animation for the owner's 5A/5C follow-through."""
import gzip,hashlib,itertools,json,sys,shutil
from verify_ball18 import ROOT,ROM,PRG,BASE,SoundMemory,initial as ball_initial
from verify_pass_floor_original import MPU,put
from extract_world_actions import decode_world_action_state
from extract_world_preparation import context

def run(r):
    m=SoundMemory(PRG,r);cpu=MPU(memory=m,pc=0xe149);cpu.x=6;header=None;motion=None
    for _ in range(30000):
        if cpu.pc==0xb5ba:header=bytes(m[:2048])
        if cpu.pc==0xb5c0:motion=bytes(m[:2048])
        if cpu.pc==0xe14c:break
        cpu.step()
    else:raise RuntimeError(hex(cpu.pc))
    return dict(before=decode_world_action_state(r),header=decode_world_action_state(header),afterMotion=decode_world_action_state(motion),after=decode_world_action_state(bytes(m[:2048])),context=context(r)['actions'])

def initial(actor,action,cursor,entered,elapsed,pose,left):
    r=ball_initial(left,entered);r[0x518]=2|(128 if entered else 0);r[0x53f]=actor;r[0x43d]=elapsed
    r[0x512+actor]=action|128;r[0x444+actor]=cursor;r[0x5b+actor]=pose|(128 if left else 0)
    put(r,(140+actor*40)*256+17,0x68+actor,0x75+actor,0x82+actor)
    put(r,(45+actor*2)*256+33,0x8f+actor,0x9c+actor)
    put(r,(48+actor*8)*256+77,0xa9+actor,0xb6+actor,0xc3+actor)
    return r

def main():
    rows=[]
    for actor,action,entered,elapsed,pose,left in itertools.product(range(4),[0x5a,0x5c],[False,True],[0,254,255],[4,30,58,59,62,63],[False,True]):
        for cursor in range(6 if action==0x5a else 4):
            r=initial(actor,action,cursor,entered,elapsed,pose,left);rows.append(dict(label=f'{actor}-{action}-{cursor}-{entered}-{elapsed}-{pose}-{left}',**run(r)))
    for actor,cursor,elapsed,left in itertools.product(range(4),[0,2,3,4,5,6,7,9,10,11,12],[0,254,255],[False,True]):
        r=initial(actor,0x16,0,True,elapsed,4,left);r[0x44a]=cursor;rows.append(dict(label=f'continued-{actor}-{cursor}-{elapsed}-{left}',**run(r)))
    for cursor,elapsed,left in itertools.product([0,2,3,4,5,6,7,9,10,11,12],[0,254,255],[False,True]):
        r=initial(6,2,cursor,True,elapsed,0,left);rows.append(dict(label=f'self-tail-{cursor}-{elapsed}-{left}',**run(r)))
    out=ROOT/'.local/basketball/rea/held-low-jump';out.mkdir(parents=True,exist_ok=True)
    packed=gzip.compress((json.dumps(rows,separators=(',',':'))+'\n').encode(),mtime=0);(out/'held-low-jump-ball.json.gz').write_bytes(packed)
    meta=dict(scope='controlled original B585 full calls; low-jump owner action/cursor header plus persistent animation after owner changes action; not a natural match',entry='bank6 B585 through original E149 action dispatcher',calls=len(rows),romSHA256=hashlib.sha256(ROM).hexdigest(),baseSHA256=hashlib.sha256(BASE).hexdigest(),fixtureSHA256=hashlib.sha256(packed).hexdigest())
    (out/'source.json').write_text(json.dumps(meta,indent=2)+'\n')
    if '--write-fixture' in sys.argv:
        dest=ROOT/'games/basketball/engine/testdata';shutil.copy2(out/'held-low-jump-ball.json.gz',dest/'held-low-jump-ball.json.gz');shutil.copy2(out/'source.json',dest/'held-low-jump-ball-source.json')
    print(json.dumps(meta))
if __name__=='__main__':main()
