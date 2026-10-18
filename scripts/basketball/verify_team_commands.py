"""Original bank5 99F0 command lifecycle and direct 9AF6 aliases; no patched code."""
import gzip,hashlib,itertools,json
from pathlib import Path
from verify_pass_floor_original import ROOT,MPU
from extract_world_inputs import decode_world_input_state
ROM=(ROOT/'.local/nes-basket/basketball.nes').read_bytes();PRG=ROM[16:16+131072]
BASE=(ROOT/'.local/nes-basket/runs/team-ai-shot-observation/team-ai-ram.bin').read_bytes()[:2048]

def state(r):
    s=decode_world_input_state(r)
    for t in range(2):s['human'][t]['accepted']=r[0x68a+2*t]
    return dict(state=s['team'],human=s['human'])
def ctx(r):
    s=decode_world_input_state(r);a=s['world'];obj=[x['player']['body']['position'] for x in a['actors']]+[x['body']['position'] for x in a['auxiliary']]+[a['ball']['body']['position']]
    return dict(teams=list(r[0x586:0x588]),hasHuman=[r[0x539+i]&r[0x53a+i]&128==0 for i in (0,2)],humanTactics=list(r[0x666:0x668]),human=state(r)['human'],holder=r[0x53f],previousHolder=r[0x546],scoreDifference=r[0x696],courtID=r[0x58b],objects=obj,airborne=[bool(r[0x5f7+i]&128) for i in range(4)],attachments=list(r[0x642:0x646]),hoops=[dict(position=h['position'],action=h['action'],owner=h['owner']) for h in a['hoops']],hoopEntered=[h['entered'] for h in a['hoops']],playerActions=[p['player']['action'] for p in a['actors']],cachedZones=list(r[0x66a:0x66e]),auxiliary=a['auxiliary'])
def initial(team=0,tactic=0,request=1,holder=6,previous=None,status=0,airborne=False,action=0,mode=2,court=1,zone=4,available=True):
    r=bytearray(BASE);r[0x539:0x53d]=bytes([128]*4);r[0x539+2*team]=0
    for t in range(2):
        for addr,val in [(0x663,13),(0x666,tactic),(0x668,48)]:r[addr+t]=val
        for addr,val in [(0x653,0),(0x657,0),(0x658,0),(0x65b,2),(0x65c,3),(0x65f,5),(0x660,6),(0x68a,2),(0x6c6,3),(0x6c7,4)]:r[addr+2*t]=val
    r[0x653+2*team]=request;r[0x658+2*team]=status&255
    r[0x53f]=holder;r[0x546]=holder if previous is None else previous;r[0x696]=0;r[0x58b]=court
    for i in range(4):r[0x672+i]=mode|128;r[0x512+i]=action|128;r[0x5f7+i]=128 if airborne else 0;r[0x66a+i]=0x30|zone
    for i in (4,5):r[0x539+i]=255;r[0x512+i]=0 if available else 1;r[0xb6+i]=0;r[0xc3+i]=0
    return r

def run(r,label,team=None):
    m=[0]*65536;m[:2048]=r;m[0x8000:0xc000]=PRG[5*16384:6*16384];m[0xc000:]=PRG[-16384:]
    cpu=MPU(memory=m,pc=0x99f0 if team is None else 0x9af6);cpu.sp=255;cpu.x=0 if team is None else team;cpu.stPushWord(0x6fff)
    if team is not None:m[0x22+team]=r[0x653+2*team]
    for _ in range(30000):
        if cpu.pc==0x7000:break
        cpu.step()
    else:raise RuntimeError((label,hex(cpu.pc)))
    if team is not None:m[0x653+2*team]=m[0x22+team]
    after=bytes(m[:2048]);return dict(label=label,before=state(r),after=state(after),context=ctx(r),directTeam=team)

def main():
    rows=[]
    for team,tactic,request,holder,airborne,action,available in itertools.product(range(2),range(6),[1,2],[0,1,2,3,6],[False,True],[0,4],[False,True]):
        r=initial(team,tactic,request,holder,airborne=airborne,action=action,available=available);rows.append(run(r,f'command-{team}-{tactic}-{request}-{holder}-{airborne}-{action}-{available}'))
    for team,mode,court,zone in itertools.product(range(2),range(42),[0,1],[4,5]):
        r=initial(team,mode=mode,court=court,zone=zone);rows.append(run(r,f'eligibility-{team}-{mode}-{court}-{zone}'))
    for team,mode,entered,holder,status,request in itertools.product(range(2),[4,5,6,7],[False,True],[0,1,2,3,6],[-128,-1,0,1],[0,1,2]):
        r=initial(team,request=request,holder=holder,status=status);r[0x663+team]=mode|(128 if entered else 0);rows.append(run(r,f'lifecycle-{team}-{mode}-{entered}-{holder}-{status}-{request}',team))
    for team,holder,previous,status in itertools.product(range(2),[0,1,2,3,6],[0,1,2,3,6],[-128,-1,0,1]):
        r=initial(team,holder=holder,previous=previous,status=status);rows.append(run(r,f'ownership-{team}-{holder}-{previous}-{status}'))
    out=ROOT/'games/basketball/engine/testdata';packed=gzip.compress((json.dumps(rows,separators=(',',':'))+'\n').encode(),mtime=0);(out/'team-commands.json.gz').write_bytes(packed)
    table=PRG[5*16384:6*16384];word=lambda a:table[a-0x8000]+256*table[a-0x8000+1]
    plans=[list(table[word(0x9467+2*t)-0x8000:word(0x9467+2*t)-0x8000+12]) for t in range(6)]
    metadata=dict(scope='controlled unmodified complete99F0 phases and direct9AF6 aliases; not natural menu gameplay',rows=len(rows),romSHA256=hashlib.sha256(ROM).hexdigest(),baseSHA256=hashlib.sha256(BASE).hexdigest(),fixtureSHA256=hashlib.sha256(packed).hexdigest(),commandPlans=plans,modeFlags=list(table[0x95ef-0x8000:0x95ef-0x8000+42]))
    (out/'team-commands-source.json').write_text(json.dumps(metadata,indent=2)+'\n');print(json.dumps(metadata))
if __name__=='__main__':main()
