"""Original mode5/plan11 calls plus controller-selection and obstacle chains.

Inputs are explicit isolated configurations; all choice, RNG and state updates
execute original ROM instructions. This is not a natural full-match trace.
"""
import gzip,hashlib,itertools,json
from verify_pass_floor_original import ROOT,Memory,MPU,put
from extract_world_inputs import decode_world_input_state
from capture_world_human_input import context

ROM=(ROOT/'.local/nes-basket/basketball.nes').read_bytes();PRG=ROM[16:16+131072]
SOURCE=ROOT/'.local/nes-basket/runs/ball-pass/ram.bin';BASE=SOURCE.read_bytes()[:2048]

def start(actor):
    r=bytearray(BASE);r[0x48:0x4c]=(0x12345678).to_bytes(4,'big');r[0x539:0x53d]=bytes([128]*4)
    r[0x642:0x646]=bytes([64]*4);r[0x550:0x554]=bytes([255]*4);r[0x58b]=1;r[0x58c]=0;r[0x586:0x588]=bytes([0,1])
    for i,x in enumerate([140,180,350,390]):
        put(r,x*256+37,0x68+i,0x75+i,0x82+i);put(r,40*256+17,0x8f+i,0x9c+i);put(r,0,0xa9+i,0xb6+i,0xc3+i)
        r[0x512+i]=130;r[0x51f+i]=128;r[0x66a+i]=0x32;r[0x5f7+i]=0;r[0x4ec+i]=64;r[0x46b+i]=128 if i>=2 else 0
    r[0x53f]=6;r[0x546]=6;r[0x518]=128;r[0x525]=128
    put(r,144*256+127,0x6e,0x7b,0x88);put(r,40*256+17,0x95,0xa2);put(r,0,0xaf,0xbc,0xc9)
    r[0x672+actor]=5;r[0x676+actor]=8;r[0x66e+actor]=9;r[0x682+actor]=4;r[0x67a+actor]=actor^2;r[0x67e+actor]=0x71
    return r

def machine(r):return MPU(memory=Memory(PRG,r))
def call(cpu,address,actor,bank=5,stop=0x7000):
    cpu.memory[0x8000:0xc000]=PRG[bank*16384:(bank+1)*16384]
    cpu.pc=address;cpu.x=actor;cpu.sp=255;cpu.p=0x20;cpu.stPushWord(0x6fff)
    history=[]
    for steps in range(20000):
        if cpu.pc==stop:return
        if cpu.pc==0xc000:cpu.pc=(cpu.stPopWord()+1)&65535
        else:
            history=(history+[cpu.pc])[-20:]
            try:cpu.step()
            except Exception as e:raise RuntimeError((hex(address),actor,hex(cpu.pc),[hex(p) for p in history])) from e
    raise RuntimeError(('did not return',hex(address),hex(cpu.pc)))

def record(cpu,actor,label):
    before=bytes(cpu.memory[:2048]);call(cpu,0xa05d,actor)
    return dict(name=label,actor=actor,before=decode_world_input_state(before),after=decode_world_input_state(bytes(cpu.memory[:2048])),context=context(before))

def team_context(r):
    w=decode_world_input_state(r)['world'];positions=[a['player']['body']['position'] for a in w['actors']]+[a['body']['position'] for a in w['auxiliary']]+[w['ball']['body']['position']]
    return dict(teams=list(r[0x586:0x588]),hasHuman=[r[0x539+i]&r[0x53a+i]&128==0 for i in (0,2)],humanTactics=list(r[0x666:0x668]),holder=r[0x53f],previousHolder=r[0x546],courtID=r[0x58b],scoreDifference=r[0x696],objects=positions,airborne=[bool(r[0x5f7+i]&128) for i in range(4)],attachments=list(r[0x642:0x646]),hoops=[dict(position=h['position'],action=h['action'],owner=h['owner']) for h in w['hoops']],hoopEntered=[h['entered'] for h in w['hoops']])

def main():
    mode5=[];plan11=[]
    cases=itertools.product(range(4),[False,True],[0x12,0x21,0x22,0x23,0x25,0x30],range(5),[0,2,3],[0x45,0x50])
    for index,(actor,entered,zone,holder_kind,ball_action,opponent_zone) in enumerate(cases):
        r=start(actor);r[0x672+actor]=5|(128 if entered else 0);r[0x66a+actor]=zone;r[0x53f]=[actor,actor^1,(actor&2)^2,((actor&2)^2)+1,6][holder_kind]
        r[0x518]=128|ball_action;r[0x66e+actor]=[0,1,2,255,9][index%5]
        for i in (((actor&2)^2),((actor&2)^2)+1):r[0x66a+i]=opponent_zone
        mode5.append(record(machine(r),actor,f'matrix-{index}'))
    for actor,entered,x,ball_action in itertools.product(range(4),[False,True],[159,160,255,256,335,336],[0,2,3]):
        r=start(actor);r[0x672+actor]=5|(128 if entered else 0);r[0x66a+actor]=0x12;r[0x518]=128|ball_action
        put(r,x*256+127,0x6e,0x7b,0x88);put(r,(x+1)*256+1,0x68+actor,0x75+actor,0x82+actor)
        mode5.append(record(machine(r),actor,f'loose-{actor}-{entered}-{x}-{ball_action}'))
    for actor,action,entered,counter,zone in itertools.product(range(4),[16,17,18],[False,True],[0,1,7,8,127,128,255],range(6)):
        r=start(actor);r[0x672+actor]=154;r[0x682+actor]=11;r[0x66e+actor]=counter;r[0x66a+actor]=0x30|zone;r[0x512+actor]=action|(128 if entered else 0);r[0x53f]=actor;r[0x518]=130
        plan11.append(record(machine(r),actor,f'plan11-{actor}-{action}-{entered}-{counter}-{zone}'))
    for actor,human,action,prior_parameter,zone in itertools.product(range(4),[False,True],[17,18],[0,255],range(6)):
        r=start(actor);r[0x586:0x588]=bytes([2,2]);r[0x672+actor]=26;r[0x682+actor]=prior_parameter;r[0x66a+actor]=0x30|zone;r[0x512+actor]=128|action;r[0x53f]=actor;r[0x518]=130
        if human:r[0x539+(actor^1)]=0
        cpu=machine(r);call(cpu,0xc018,actor);call(cpu,0xc018,actor)
        plan11.append(record(cpu,actor,f'entry-{actor}-{human}-{action}-{prior_parameter}-{zone}'))
    # Original tactic-selection directional input -> team selection -> mode5.
    r=start(1);r[0x539]=0;r[0x672:0x676]=bytes([0]*4);r[0x666:0x668]=bytes([4,0]);r[0x6dc]=3;r[0x23]=0;r[0x1f]=0;r[0x3c3]=0;r[0]=1;r[4]=1;r[0x663:0x665]=bytes([13,12]);r[0x653]=r[0x655]=r[0x658]=r[0x65a]=0
    cpu=machine(r);cpu.y=0;call(cpu,0xb4b9,0,3,0xb501);assert cpu.memory[0x666]==5
    team_before=bytes(cpu.memory[:2048]);call(cpu,0x99f0,0);team_after=bytes(cpu.memory[:2048]);assert cpu.memory[0x673]&127==5
    selection=dict(pressedRight=True,cursorBefore=3,cursorAfter=cpu.memory[0x6dc],tacticBefore=4,tacticAfter=cpu.memory[0x666],before=decode_world_input_state(team_before)['team'],after=decode_world_input_state(team_after)['team'],context=team_context(team_before),call=record(cpu,1,'selected-tactic5'))
    # Mode17 consumes the first two real RNG draws; a forward opponent then
    # selects mode26, whose third draw selects team2 table index6 -> plan11.
    r=start(2);r[0x586:0x588]=bytes([0,2]);r[0x672:0x676]=bytes([0]*4);r[0x663:0x665]=bytes([12,12]);r[0x53f]=2;r[0x546]=6;r[0x518]=130;r[0x512+2]=146;r[0x66a+2]=0x32;r[0x46b+2]=128
    put(r,334*256+37,0x68,0x75,0x82);put(r,40*256+17,0x8f,0x9c)
    cpu=machine(r);escape=[]
    evasion_team_before=decode_world_input_state(r)['team'];call(cpu,0x99f0,0)
    evasion_team_after=decode_world_input_state(bytes(cpu.memory[:2048]))['team'];assert cpu.memory[0x674]==17
    for tick in range(2):
        escape.append(record(cpu,2,f'entry-chain-{tick}'))
        if tick==0:assert cpu.memory[0x674]==26 and cpu.memory[0x684]==0
        if tick==1:assert cpu.memory[0x684]==11 and cpu.memory[0x514]&127==17
    out=ROOT/'games/basketball/engine/testdata';sources=[]
    for name,rows in [('ai-mode5-isolated',mode5),('ai-plan11-isolated',plan11),('ai-plan11-entry-chain',escape)]:
        p=out/(name+'.json.gz');p.write_bytes(gzip.compress((json.dumps(rows,separators=(',',':'))+'\n').encode(),mtime=0));sources.append(dict(path=p.name,records=len(rows),sha256=hashlib.sha256(p.read_bytes()).hexdigest()))
    p=out/'ai-mode5-selection-chain.json';p.write_text(json.dumps(selection,separators=(',',':'))+'\n');sources.append(dict(path=p.name,sha256=hashlib.sha256(p.read_bytes()).hexdigest()))
    source=dict(natural_gameplay=False,isolated_original_execution=True,original_instructions_unchanged=True,entries=['bank5:A05D/A229/A8DE/B97B','bank3:B4B9..B501','bank5:99F0'],selection_scope='Original directional selection update only; stops before rendering/wait loop',evasion_scope='Original99F0 selects team2 carrier mode17 from possession; mode17 chooses26 from a configured forward opponent; next A8DE selects plan11. Physics is outside this chain.',evasion_team_before=evasion_team_before,evasion_team_after=evasion_team_after,sound_scope='C000 returns without audio-driver execution; gameplay instructions are unchanged',initial_rng='12345678',ram_source=str(SOURCE.relative_to(ROOT)),ram_sha256=hashlib.sha256(BASE).hexdigest(),rom_sha256=hashlib.sha256(ROM).hexdigest(),bank5_sha256=hashlib.sha256(PRG[5*16384:6*16384]+PRG[-16384:]).hexdigest(),outputs=sources)
    (out/'ai-mode5-plan11-source.json').write_text(json.dumps(source,indent=2)+'\n');print(json.dumps(source,indent=2))

if __name__=='__main__':main()
