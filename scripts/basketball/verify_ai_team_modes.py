"""Original team5/team7 selection and complete personal modes33/34/35.

These are explicit isolated configurations, not full natural-match evidence.
Only original AI/team functions run; actor physics is outside this scope.
"""
import gzip,hashlib,itertools,json
from verify_ai_mode5_plan11 import ROOT,ROM,PRG,SOURCE,BASE,start,machine,call,record,team_context
from verify_pass_floor_original import put
from extract_world_inputs import decode_world_input_state

def setup(mode,actor,entered):
    r=start(actor);r[0x672+actor]=mode|(128 if entered else 0);r[0x676+actor]=9;r[0x66e+actor]=17;r[0x682+actor]=7;r[0x67e+actor]=4;r[0x67a+actor]=actor^1
    r[0x53f]=actor;r[0x518]=130;r[0x46b+actor]=128 if actor<2 else 0
    put(r,123,0x478+actor,0x485+actor);put(r,-99,0x492+actor,0x49f+actor)
    r[0x586+actor//2]=5 if mode==34 else 7
    return r

def main():
    groups={'ai-mode33-isolated':[],'ai-mode34-isolated':[],'ai-mode35-isolated':[]}
    for n,(actor,entered,char,air,zone,height) in enumerate(itertools.product(range(4),[False,True],range(4),[False,True],[0x22,0x25],[7,8,255,256,263,264,511,512,-1])):
        r=setup(33,actor,entered);r[0x3e3+actor]=char;r[0x5f7+actor]=128 if air else 0;r[0x66a+actor]=zone;r[0x512+actor]=159
        put(r,height*256+127,0xa9+actor,0xb6+actor,0xc3+actor)
        groups['ai-mode33-isolated'].append(record(machine(r),actor,f'mode33-{n}'))
    for n,(actor,seed,peer_mode,peer_entered,peer_air,near) in enumerate(itertools.product(range(4),[0,0x12345678,0xc48d159e],[2,40],[False,True],[False,True],[False,True])):
        r=setup(34,actor,False);peer=actor^1;r[0x48:0x4c]=seed.to_bytes(4,'big');r[0x672+peer]=peer_mode|(128 if peer_entered else 0);r[0x676+peer]=9;r[0x5f7+peer]=128 if peer_air else 0;r[0x512+actor]=146
        x=int.from_bytes(bytes([r[0x75+actor],r[0x82+actor]]),'little')
        put(r,(x+(2 if near else 30))*256+11,0x68+peer,0x75+peer,0x82+peer)
        groups['ai-mode34-isolated'].append(record(machine(r),actor,f'mode34-entry-{n}'))
    for n,(actor,grade,phase,speed,air) in enumerate(itertools.product(range(4),[0,3,4,7],[-128,-2,-1,0,1],[-1,0,1,2],[False,True])):
        r=setup(34,actor,True);r[0x512+actor]=148;r[0x619+actor]=grade;r[0x576+actor]=phase&255;r[0x4b9+actor]=speed&255;r[0x5f7+actor]=128 if air else 0
        groups['ai-mode34-isolated'].append(record(machine(r),actor,f'mode34-jump-{n}'))
    for n,(actor,action,air,delta,phase) in enumerate(itertools.product(range(4),[0,18,61,62],[False,True],[(0,0),(3,7),(4,7),(15,8),(16,8),(-3,-7),(-4,-8),(256,0)],[-2,-1,0])):
        r=setup(34,actor,True);r[0x512+actor]=128|action;r[0x5f7+actor]=128 if air else 0;r[0x576+actor]=phase&255;peer=actor^1
        x=r[0x75+actor]+256*r[0x82+actor];put(r,(x+delta[0])*256+13,0x68+peer,0x75+peer,0x82+peer);put(r,(40+delta[1])*256+23,0x8f+peer,0x9c+peer)
        groups['ai-mode34-isolated'].append(record(machine(r),actor,f'mode34-approach-{n}'))
    for n,(actor,char,ball_action,difficulty,seed) in enumerate(itertools.product(range(4),[0,1],[0,1,2,7,8,13,16,23],[0,12,24,28],[0,0x12345678,0xffffffff])):
        r=setup(35,actor,False);r[0x3e3+actor]=char;r[0x518]=128|ball_action;r[0x58c]=difficulty;r[0x48:0x4c]=seed.to_bytes(4,'big');r[0x53f]=6
        groups['ai-mode35-isolated'].append(record(machine(r),actor,f'mode35-entry-{n}'))
    for n,(actor,parameter,action,entered,air,speed) in enumerate(itertools.product(range(4),[0,1,3,7,10,11],[0,4,6,5],[False,True],[False,True],[2,3,-1])):
        r=setup(35,actor,True);r[0x682+actor]=parameter;r[0x67a+actor]=6;r[0x512+actor]=action|(128 if entered else 0);r[0x5f7+actor]=128 if air else 0;r[0x4b9+actor]=speed&255
        r[0x53f]=actor if n%2 else 6;r[0x642+(actor^1)]=128|actor if n%3==0 else 64
        put(r,16*256,0xa9+actor,0xb6+actor,0xc3+actor)
        groups['ai-mode35-isolated'].append(record(machine(r),actor,f'mode35-continuation-{n}'))
    chains=[]
    for team,side,difference,seed,holder_kind,air in itertools.product([5,7],[0,1],[0,1,2,3,0x81,0x82,0x83],[0,0x12345678],[0,1],[False,True]):
        actor=side*2+1;r=setup(33 if team==7 else 34,actor,False);r[0x586+side]=team;r[0x696]=difference;r[0x48:0x4c]=seed.to_bytes(4,'big');r[0x53f]=actor if holder_kind==0 else (actor^2);r[0x546]=6
        r[0x663:0x665]=bytes([12,12]);r[0x672:0x676]=bytes([0]*4);r[0x3e3+actor]=1;r[0x512+actor]=159 if air and holder_kind==0 else 146;r[0x5f7+actor]=128 if air else 0
        put(r,32*256,0xa9+actor,0xb6+actor,0xc3+actor)
        cpu=machine(r);before=bytes(cpu.memory[:2048]);call(cpu,0x99f0,0);after=bytes(cpu.memory[:2048]);calls=[]
        selected=cpu.memory[0x672+actor]&127;plan=cpu.memory[0x676+actor]
        # Keep every original team selection, but execute only the selected
        # requested mode or its real mode1->33 catch handoff in this chain.
        if selected in (33,34,35) or selected==1 and plan==33:
            calls.append(record(cpu,actor,f'selection-{len(chains)}-first'))
            if selected==1 and plan==33 and cpu.memory[0x672+actor]&127==33:calls.append(record(cpu,actor,f'selection-{len(chains)}-mode33'))
        chains.append(dict(team=team,side=side,difference=difference,seed=seed,actor=actor,selectedMode=selected,before=decode_world_input_state(before)['team'],after=decode_world_input_state(after)['team'],context=team_context(before),calls=calls))
    # A successful AAC7 write is visible to the later teammate's input call.
    r=setup(34,2,False);r[0x48:0x4c]=bytes(4);r[0x672+3]=2;r[0x5f7+3]=0;r[0x512+2]=146
    cpu=machine(r);ordered=[record(cpu,2,'teammate-command'),record(cpu,3,'later-teammate')]
    out=ROOT/'games/basketball/engine/testdata';sources=[]
    for name,rows in [*groups.items(),('ai-team-special-selection-chains',chains),('ai-mode34-teammate-order',ordered)]:
        p=out/(name+'.json.gz');p.write_bytes(gzip.compress((json.dumps(rows,separators=(',',':'))+'\n').encode(),mtime=0));sources.append(dict(path=p.name,records=len(rows),sha256=hashlib.sha256(p.read_bytes()).hexdigest()))
    source=dict(natural_gameplay=False,isolated_original_execution=True,original_instructions_unchanged=True,entry='bank5:A05D dispatcher and complete99F0 selection',mode_entries={'33':'ABEF','34':'AAC7','35':'AC3A'},mode35_thresholds=list(PRG[5*16384+0xac7f-0x8000:5*16384+0xac87-0x8000]),selection_scope='All224 team5/team7 selections retained.80 immediately enter target modes or mode1->33 catch handoff and continue through original AI calls; remaining selections/airborne recovery waits are not silently removed or claimed as complete target-mode chains.',sound_scope='C000 returns without audio-driver execution',ram_source=str(SOURCE.relative_to(ROOT)),ram_sha256=hashlib.sha256(BASE).hexdigest(),rom_sha256=hashlib.sha256(ROM).hexdigest(),outputs=sources)
    (out/'ai-team-modes-source.json').write_text(json.dumps(source,indent=2)+'\n');print(json.dumps(source,indent=2))

if __name__=='__main__':main()
