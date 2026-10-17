"""Original 805C controller samples in held-prop contexts, plus action chains."""
import collections,gzip,hashlib,itertools,json
from verify_pass_floor_original import ROOT,Memory,MPU,put
from extract_world_inputs import decode_world_input_state
from capture_world_human_input import context

ROM=(ROOT/'.local/nes-basket/basketball.nes').read_bytes();PRG=ROM[16:16+131072]
SOURCE=ROOT/'.local/nes-basket/runs/court-3-reference/ram.bin';BASE=SOURCE.read_bytes()[1500*2048:1501*2048]

def state(r):
    value=decode_world_input_state(r)
    for slot in range(2):value['world']['auxiliary'][slot]['heading']=8 if r[0x4f0+slot]==255 else r[0x4f0+slot]//32
    return value

def machine(actor,slot,action=0x20,air=False,mirror=False,attached=0,held=True):
    r=bytearray(BASE);obj=slot+4
    for i in range(4):r[0x4e8+i]=i;r[0x539+i]=128;r[0x550+i]=255
    r[0x539+actor]=actor;r[0x53d]=r[0x53e]=255;r[0x53f]=6;r[0x569]=r[0x56b]=255
    for i in (actor,obj):
        put(r,300*256+37,0x68+i,0x75+i,0x82+i);put(r,40*256+17,0x8f+i,0x9c+i);put(r,20*256 if air else 0,0xa9+i,0xb6+i,0xc3+i)
        for a,b in [(0x478,0x485),(0x492,0x49f),(0x4ac,0x4b9),(0x4c6,0x4d3)]:put(r,0,a+i,b+i)
        for a,v in {0x512:128,0x51f:128,0x52c:255,0x437:0,0x444:0,0x451:0,0x46b:0,0x5f7:128 if air else 0,0x642:64,0x5b:128 if mirror else 0}.items():r[a+i]=v
    r[0x512+actor]=128|action;r[0x46b+actor]=0 if mirror else 128;r[0x4ec+actor]=192;r[0x576+actor]=0
    r[0x4f0]=32;r[0x4f1]=255
    if held:r[0x539+obj]=actor;r[0x512+obj]=129;r[0x642+obj]=128|actor
    if attached==1:r[0x642+actor]=128|(actor^1)
    if attached in (2,3,4):
        parent=(obj^1);r[0x642+actor]=128|parent;r[0x3d6+parent]=12;r[0x642+parent]=64;r[0x5f7+parent]=128 if attached==3 else 0
        if attached==4:r[0x569]=actor
    if attached==5:r[0x550+actor]=actor^1
    if attached==6:r[0x53d]=r[0x53e]=actor
    for a in (0x4f9,0x4fd,0x501,0x505,0x509,0x50d):r[a+actor]=0
    return MPU(memory=Memory(PRG,r))

def call(cpu,address,actor):
    cpu.memory[0x8000:0xc000]=PRG[6*16384:7*16384];cpu.pc=address;cpu.x=actor;cpu.sp=255;cpu.p=0x20;cpu.stPushWord(0x6fff)
    for n in range(100000):
        if cpu.pc==0x7000:return
        if cpu.pc in (0xc000,0xfbcc):
            cpu.memory[0x7ff]=cpu.a
            cpu.pc=(cpu.stPopWord()+1)&65535
        else:cpu.step()
    raise RuntimeError(('did not return',hex(cpu.pc)))

def input_call(cpu,actor,held,last,label,tick):
    cpu.memory[:8]=[0]*8;cpu.memory[actor]=held;cpu.memory[actor+4]=held&~last
    before=bytes(cpu.memory[:2048]);call(cpu,0x805c,actor);after=bytes(cpu.memory[:2048])
    return dict(name=label,tick=tick,actor=actor,before=state(before),after=state(after),context=context(before),pickupBefore=list(before[0x634:0x638]),pickupAfter=list(after[0x634:0x638]),jumpEnhancements=[before[0x6e6+3*before[0x3e3+i]] for i in range(4)])

def main():
    rows=[]
    scenarios=[('A',0x20,False,[128,128,0],0),('B-neutral',0x20,False,[64,64,0],0),('B-right',0x20,False,[65,65,1],0),('B-left',0x20,False,[66,66,2],0),('run-B',0x22,False,[64,64,0],0),('run-A',0x22,False,[128,128,0],0),('air-A',0x24,True,[128,128,0],0),('air-B-right',0x24,True,[65,65,1],0),('air-B-neutral',0x24,True,[64,64,0],0),('release-aim',0x29,False,[64,0,0],64),('release-air-aim',0x2a,True,[64,0,0],64),('walk-stop',0x20,False,[1,1,0],0),('double-run',0x20,False,[1,0,1,1,0],0),('jump',0x20,False,[192,192,0],0),('run-jump',0x22,False,[192,192,0],0),('combine-AB',0x20,False,[128,192,0],0),('short-B',0x20,False,[64,0,0],0),('repeat-A',0x20,False,[128,0,128,0],0)]
    for actor,slot,mirror,spec in itertools.product(range(4),range(2),[False,True],scenarios):
        name,action,air,sequence,last=spec;cpu=machine(actor,slot,action,air,mirror)
        for tick,buttons in enumerate(sequence):
            rows.append(input_call(cpu,actor,buttons,last,f'{name}-{actor}-{slot}-{mirror}',tick));last=buttons
    for actor,slot,attached,buttons in itertools.product(range(4),range(2),range(1,7),[64,65,128,192]):
        cpu=machine(actor,slot,0x3d,True,False,attached);last=0
        for tick in range(3):rows.append(input_call(cpu,actor,buttons if tick<2 else 0,last,f'attached-{actor}-{slot}-{attached}-{buttons}',tick));last=buttons if tick<2 else 0
    # Actual controller pickup followed by second A/B requests and original
    # E18B actions; no requests or ownership are patched after initialization.
    chains=[]
    for kind in ['A-ground','B-ground','A-air','B-air','run-jump']:
        cpu=machine(0,1,0,False,False,held=False);last=0;ready=None;jumped=False;thrown=False;trace=[]
        for tick in range(100):
            if ready is None and cpu.memory[0x53e]==0 and cpu.memory[0x512]&127==0x20:ready=tick+3
            held=128 if tick<4 else 0
            if ready is not None:
                dt=tick-ready
                if kind=='A-ground' and 0<=dt<4:held=128
                if kind=='B-ground' and 0<=dt<4:held=65
                if kind in ('A-air','B-air'):
                    if 0<=dt<3:held=192
                    if 7<=dt<11:held=128 if kind=='A-air' else 65
                if kind=='run-jump':
                    if dt in (0,2,3,4):held=1
                    if 5<=dt<8:held=193
            row=input_call(cpu,0,held,last,f'chain-{kind}',tick);last=held
            rows.append(row)
            try:call(cpu,0xe18b,0);call(cpu,0xe18b,5)
            except Exception as e:raise RuntimeError((kind,tick,cpu.memory[0x512],cpu.memory[0x51f],hex(cpu.pc),cpu.memory[0xbfff],trace[-4:])) from e
            row['afterActions']=state(bytes(cpu.memory[:2048]))
            action=cpu.memory[0x512]&127;aux=cpu.memory[0x517]&127
            jumped|=action in (0x24,0x25);thrown|=cpu.memory[0x53e]==255 and ready is not None
            trace.append(dict(tick=tick,held=held,request=row['after']['world']['actors'][0]['player']['request'],playerAction=action,auxAction=aux,owner=cpu.memory[0x53e]))
            if ready is not None and (thrown and aux==0 or kind=='run-jump' and tick>ready+25):break
        assert ready is not None,(kind,trace)
        if kind!='run-jump':assert thrown,(kind,trace)
        chains.append(dict(name=kind,jumped=jumped,thrown=thrown,trace=trace))
    out=ROOT/'games/basketball/engine/testdata';p=out/'held-auxiliary-human-input.json.gz';p.write_bytes(gzip.compress((json.dumps(rows,separators=(',',':'))+'\n').encode(),mtime=0))
    q=out/'held-auxiliary-controller-chains.json';q.write_text(json.dumps(chains,indent=2)+'\n')
    p.with_name('held-auxiliary-human-input-source.json').write_text(json.dumps(dict(records=len(rows),original_function='bank6:805C',original_instructions_unchanged=True,natural_gameplay=False,scope='Explicit held contexts and controller chains; first A picks up via original805C/E18B; only controller samples change during each chain',sound_scope='C000/FBCC preserve the07FF sound latch and return without audio-driver execution; gameplay instructions unchanged',ram_source=str(SOURCE.relative_to(ROOT)),ram_frame=1501,ram_sha256=hashlib.sha256(BASE).hexdigest(),rom_sha256=hashlib.sha256(ROM).hexdigest(),sha256=hashlib.sha256(p.read_bytes()).hexdigest(),chains_sha256=hashlib.sha256(q.read_bytes()).hexdigest()),indent=2)+'\n')
    print('original held-prop input calls',len(rows),'chains',[(c['name'],len(c['trace']),c['jumped'],c['thrown']) for c in chains])

if __name__=='__main__':main()
