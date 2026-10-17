"""Original ordinary 09/0B teammate-assist calls; carried-body branch excluded."""
import gzip, hashlib, itertools, json
from verify_attack_handlers import ROOT, ROM, initial, machine, PRG, call
from verify_pass_floor_original import put
from capture_player_attack import decode

def execute(cpu,actor,name):
    cpu.memory[0x8000:0xc000]=PRG[6*16384:7*16384]
    cpu.pc,cpu.x,cpu.sp,cpu.p=0x85e9,actor,255,0x20;cpu.stPushWord(0x6fff)
    before=bytes(cpu.memory[:2048])
    for steps in range(20000):
        if cpu.pc==0x7000:
            return dict(name=name,actor=actor,before=decode(before,actor),after=decode(bytes(cpu.memory[:2048]),actor),instructions=steps)
        cpu.step()
    raise AssertionError(hex(cpu.pc))

rows=[]
for actor,action,entered,facing,motion,speed in itertools.product(range(4),(9,11),(False,True),(False,True),((0,False,0),(8192,True,513),(1,True,-513)),(-257,-1,0,1,257)):
    r=initial(actor,(action,5,1,0),entered,facing,motion,0)
    r[0x569]=255;r[0x550+actor]=actor^1;r[0x4ec+actor]=192 if facing else 64
    r[0x52c+actor]=11 if action==9 else 0
    r[0x437+actor]=3 if action==9 else 7
    r[0x444+actor]=0;r[0x451+actor]=0
    put(r,speed,0x478+actor,0x485+actor);put(r,speed,0x492+actor,0x49f+actor)
    put(r,0,0x4c6+actor,0x4d3+actor)
    rows.append(execute(machine(r),actor,f'matrix-{len(rows)}'))
for actor,facing in itertools.product(range(4),(False,True)):
    r=initial(actor,(9,5,1,0),False,facing,(8192,True,0),0)
    r[0x569]=255;r[0x550+actor]=actor^1;r[0x4ec+actor]=192 if facing else 64
    put(r,0,0x4c6+actor,0x4d3+actor)
    cpu=machine(r)
    for tick in range(80):
        row=execute(cpu,actor,f'chain-{actor}-{facing}-{tick}');rows.append(row)
        if row['after']['action']&127 not in (9,11):break
    else:raise AssertionError('assist did not finish')
p=ROOT/'games/basketball/engine/testdata/player-assist.json.gz';packed=gzip.compress((json.dumps(rows,separators=(',',':'))+'\n').encode(),mtime=0);p.write_bytes(packed)
p.with_name('player-assist-source.json').write_text(json.dumps(dict(records=len(rows),entries=['bank6:9852','bank6:8E5E'],scope='ordinary source09/0B calls and chains; 0569 carried-body branch excluded explicitly',romSHA256=hashlib.sha256(ROM).hexdigest(),sha256=hashlib.sha256(packed).hexdigest()),indent=2)+'\n')
print(len(rows))
