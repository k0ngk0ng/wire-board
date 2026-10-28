"""Original machine dispatcher oracle with declared per-call inputs.

Failure scenarios before implementation: queue refusal and camera clipping;
byte underflow; kind-specific properties; nearest-player state/tie behavior;
growth/shrink timing; shared-slot allocation; globals/sounds lost at return.
Only the input is staged; all expected enemy/queue output is original execution.
"""
import gzip,hashlib,json,os,shutil,sys,zipfile
from pathlib import Path
ROOT=Path(__file__).resolve().parents[2]
BASE=ROOT/'.local/contra-enemies-machines/cases-oracle'
os.environ['CONTRA_REFERENCE_ROOT']=str(BASE)
from capture_reference import run
from enemies_machines_capture import HOOK,ENEMY,PLAYER

def lua(v):
    if isinstance(v,dict):return '{'+','.join(str(k)+'='+lua(x)for k,x in v.items())+'}'
    if isinstance(v,list):return '{'+','.join(lua(x)for x in v)+'}'
    if isinstance(v,str):return json.dumps(v)
    return str(v)

def cases(group='flame'):
    base=dict(level=5,kind=16,routine=2,x=160,y=80,attributes=0x80,animation=0,attack=8,frame=4,variable=32,data=[0,2,0,0],used=0,scroll=0,clock=32,random=15,
              players=[dict(state=1,x=150,y=112),dict(state=1,x=170,y=112)])
    result=[]
    if group=='mortar-vertical':
        base.update(level=2,kind=11,frame=0,attributes=1,animation=1,data=[0,0,0,0],vertical=1,vy=256)
        for routine in range(1,10):
            for scroll in [0,1,3]:
                for y in [192,231]:
                    for ground in [0,255]:result.append(dict(base,routine=routine,scroll=scroll,y=y,ground=ground))
        return result
    if group=='spawn-variants':
        base.update(level=6,frame=0,attributes=0,animation=1,data=[0,0,0,0])
        result.append(dict(base,kind=18,routine=1,attributes=2))
        for part in range(4):result.append(dict(base,kind=18,routine=4,animation=4-part,data=[0,0,0,part]))
        for attr in range(4,16):result.append(dict(base,kind=16,routine=1,attributes=attr))
        result.append(dict(base,kind=23,routine=1,attributes=1))
        return result
    if group=='edges':
        for kind in [16,17,18]:
            for timer in [1,9,17,25,33]:result.append(dict(base,kind=kind,routine=2,frame=4 if kind==16 else 0,attack=timer,animation=2))
        return result
    if group=='giant-audio':
        base.update(kind=19,routine=5,frame=0,x=160,y=155,clock=127,random=0,animation=2,hp=64,data=[0,0,0,0])
        for blocker in [0,0x62,0x77]:
            for effect in [0,0x2b,0x77]:
                result.append(dict(base,audio=[0,blocker,0,0,effect,0]))
        return result
    if group=='giant':
        base.update(level=5,kind=19,frame=0,attributes=0,animation=1,data=[0,176,8,0],attack=1,y=155)
        for routine in range(1,11):
            for delay in [0,1,2]:
                for used in [0,79,80]:result.append(dict(base,routine=routine,animation=delay,used=used))
        for action in range(4):
            for random in range(4):
                for x in [100,180]:result.append(dict(base,routine=3,data=[action,1,0,3],random=random,x=x))
        for routine in [4,5,6]:
            for hp in [15,16,31,32]:
                for clock in [3,7,127]:
                    for random in [0,1,127,128]:result.append(dict(base,routine=routine,hp=hp,clock=clock,random=random,animation=2))
        for routine in [5,6]:
            for x in [30,32,33,191,192,193]:
                for y in [150,155,156,200]:result.append(dict(base,routine=routine,x=x,y=y,vx=128,vy=256))
        for mode in [0,1,2]:
            for used in [0,63,64,79,80]:result.append(dict(base,routine=10,data=[mode,176,1,0],used=used))
        for explosion in range(5):result.append(dict(base,routine=8,data=[explosion,0,0,0]))
        for free in [0,1,15]:
            for flip in [0,64]:result.append(dict(base,routine=4,free=free,flip=flip))
        for gameover in [0,1]:
            result.append(dict(base,routine=3,players=[dict(state=1,x=96,y=112,gameover=gameover),dict(state=1,x=220,y=112,gameover=1-gameover)]))
        return result
    if group=='walls':
        base.update(level=6,frame=0,attributes=0,animation=1,data=[0,0,48,0],attack=2)
        for kind,total in [(17,6),(18,4),(22,7)]:
            for routine in range(1,total+1):
                for delay in [0,1,3]:
                    for used in [0,63,64]:
                        d=[0,6 if kind==17 and routine==3 else 0,48,4 if kind==18 else 0]
                        result.append(dict(base,kind=kind,routine=routine,animation=delay,used=used,data=d))
        for attrs in range(16):result.append(dict(base,kind=17,routine=1,attributes=attrs))
        for index in range(7):
            for used in [0,64]:
                result.append(dict(base,kind=17,routine=3,animation=0,used=used,data=[0,index,48,0]))
                result.append(dict(base,kind=17 if index<4 else 18,routine=6 if index<4 else 4,used=used,data=[0,0,48,index]))
        for hp in [0,4,5,240]:result.append(dict(base,kind=22,routine=2,hp=hp))
        for offset in [0,1]:
            for used in [0,63,64]:result.append(dict(base,kind=22,routine=7,used=used,data=[0,offset,0,0]))
        return result
    if group=='mechanisms':
        base.update(level=6,frame=0,attributes=0,animation=1,data=[4,0,0,0],attack=1)
        for kind,total in [(19,2),(20,6),(21,6),(23,8),(24,5)]:
            for routine in range(1,total+1):
                for delay in [0,1,2]:
                    for used in [0,63,64,79,80]:
                        result.append(dict(base,kind=kind,routine=routine,animation=delay,used=used))
        for kind in [19,23,24]:
            for free in [0,1,15]:
                for parts in [0,1,2]:
                    for frame in [0,2,128]:
                        result.append(dict(base,kind=kind,routine=2 if kind==19 else 3,free=free,parts=parts,frame=frame))
        for kind in [20,21]:
            for ground in [0,255]:
                for direction in [0,2]:
                    for x in [4,8,16,248,255]:
                        result.append(dict(base,kind=kind,routine=3,ground=ground,data=[0,0,0,direction],x=x,vx=256 if direction==0 else-256,attributes=128))
        for kind in [23,24]:
            for ready in [0,1]:
                for parts in [0,2]:
                    for frame in [0,2]:result.append(dict(base,kind=kind,routine=2,ready=ready,parts=parts,frame=frame))
        return result
    if group=='common':
        base.update(level=5,frame=0,attributes=0,animation=1,data=[0,0,0,0])
        for kind,total in [(11,9),(14,6),(15,3),(20,3)]:
            for routine in range(1,total+1):
                for delay in [0,1,2]:
                    for attrs in ([0,1,2,3]if kind==11 else[0,1,3]):
                        result.append(dict(base,kind=kind,routine=routine,animation=delay,attributes=attrs))
        for variant in range(5):result.append(dict(base,kind=11,routine=1,data=[variant,0,0,0]))
        for kind in [11,15,20]:
            for x,y in [(7,80),(8,80),(239,80),(240,80),(255,80),(160,47),(160,48),(160,120),(160,224),(160,231)]:
                for vy in [-256,-16,0,256]:
                    for ground in [0,255]:result.append(dict(base,kind=kind,routine=2,x=x,y=y,vy=vy,vx=-640,ground=ground,attributes=1))
        for free in [0,1,2,15]:
            for kind in [11,14]:result.append(dict(base,kind=kind,routine=3,free=free))
        return result
    for kind in [16,17,18]:
        for routine in range(1,5):
            for delay in [0,1,2]:
                for used in [0,79,80]:
                    for length in [0,8]:
                        data=[0,2,0,0]
                        data[3 if kind==16 else 2]=length if kind!=17 else(-length)&255
                        result.append(dict(base,kind=kind,routine=routine,attributes={16:128,17:64,18:0}[kind]|1,animation=delay,used=used,data=data))
        for clock in [0,31,32,63,127,128,192,255]:result.append(dict(base,kind=kind,clock=clock,attributes={16:128,17:64,18:0}[kind]))
        for x in [6,7,8,11,43,47,48,240,255]:
            for routine in [2,3,4]:result.append(dict(base,kind=kind,x=x,routine=routine,scroll=1,attributes={16:128,17:64,18:0}[kind],data=[0,1,248 if kind==17 else 0,0]))
        for player in [dict(state=0,x=150,y=112),dict(state=1,x=128,y=112),dict(state=1,x=129,y=112)]:result.append(dict(base,kind=kind,players=[player,dict(state=0,x=160,y=112)]))
        for attrs in range(16):result.append(dict(base,kind=kind,routine=1,attributes=attrs))
    for routine in range(1,5):
        for attrs in range(4):
            for used in [0,79,80]:
                for delay in [0,1,2]:
                    result.append(dict(base,level=6,kind=16,routine=routine,attributes=attrs,frame=32,data=[0,2,0,attrs*2+(routine==4)],animation=delay,used=used))
    for x in [6,7,8,43,44,240]:
        for routine in range(1,5):result.append(dict(base,level=6,kind=16,routine=routine,x=x,scroll=1,frame=32,attributes=3,data=[0,1,0,6+(routine==4)]))
    return result

def main():
    label=sys.argv[1]if len(sys.argv)>1 else'v1'
    group=sys.argv[2]if len(sys.argv)>2 else'flame'
    cs=cases(group)
    with zipfile.ZipFile('/Users/jason/Documents/nes/roms/03/魂斗罗日版(完全版).zip')as z:raw=z.read('Contra (J).nes')
    assert hashlib.sha256(raw).hexdigest()=='6ba53139fa88b8de1ae527c438bda6f1541d1ee7df26d63dec5164a32d166bfe'
    prg=raw[16:131088]
    for c in cs:
        c['attr']=int.from_bytes(prg[0xa87b+c['level']*32+6:0xa87b+c['level']*32+8],'little')
        c['dispatch']=int.from_bytes(prg[0x1e5c9+c['level']*2:0x1e5cb+c['level']*2],'little')
        c.setdefault('free',15);c.setdefault('ground',0);c.setdefault('parts',0);c.setdefault('ready',1);c.setdefault('vertical',0)
        c['palette']=list(prg[0xa87b+c['level']*32+9:0xa87b+c['level']*32+24])
        vx,vy=c.get('vx',0)&65535,c.get('vy',0)&65535
        c['values']=[1,c['y'],c['x'],c.get('flip',0),c['routine'],0,0,vy>>8,vy&255,vx>>8,vx&255,c['kind'],c['animation'],c['variable'],c['attack'],c['frame'],c.get('hp',0xf0),0x11,0x81,c['attributes'],*c['data']]
    hook=HOOK.replace('local ordinal=0','local ordinal=0;local cases=CASES;local caseIndex=0')
    hook=hook.replace("local p=pending;pending=nil","local p=pending;pending=nil")
    old="p.before,state(),table.concat(p.audio,',')))"
    hook=hook.replace(old,old+"\n  for a,v in pairs(p.restore)do memory.writebyte(a,v)end")
    hook=hook.replace(" local slot=memory.getregister('x');local kind=read(0x528+slot)",''' local slot=memory.getregister('x');if slot~=15 then return end
 caseIndex=caseIndex+1;local c=cases[caseIndex];assert(c,'unexpected native dispatch')
 local restore={};for a=0,0x7ff do if a<0x100 or a>=0x200 then restore[a]=read(a)end end
 for i=0,15 do for _,a in ipairs(ef)do memory.writebyte(a+i,0)end end
 for i=0,14-c.free do memory.writebyte(0x4b8+i,1)end
 for i,a in ipairs(ef)do memory.writebyte(a+slot,c.values[i])end
 memory.writebyte(4,c.routine*2);memory.writebyte(0x30,c.level);memory.writebyte(0x40,0);memory.writebyte(0x41,c.vertical);memory.writebyte(0x68,c.scroll);memory.writebyte(0x1a,c.clock);memory.writebyte(0x34,c.random);memory.writebyte(0x8e,1)
 memory.writebyte(0x80,c.dispatch%256);memory.writebyte(0x81,math.floor(c.dispatch/256))
 memory.writebyte(0x84,c.ready);memory.writebyte(0x85,c.parts)
 memory.writebyte(0x46,c.attr%256);memory.writebyte(0x47,math.floor(c.attr/256));memory.writebyte(0xfc,224);memory.writebyte(0xfd,0);memory.writebyte(0xff,0xb0)
 memory.writebyte(0x21,c.used);for a=0x600,0x77f do memory.writebyte(a,0)end
 for a=0x680,0x6ff do memory.writebyte(a,c.ground)end
 for i=0,1 do local p=c.players[i+1];memory.writebyte(0x90+i,p.state);memory.writebyte(0x334+i,p.x);memory.writebyte(0x31a+i,p.y);memory.writebyte(0x38+i,0)end
 local kind=read(0x528+slot)''')
    hook=hook.replace('before=state(),audio={}','before=state(),audio={},restore=restore')
    if group in ['giant','giant-audio']:
        hook=hook.replace('memory.writebyte(0x84,c.ready);memory.writebyte(0x85,c.parts)','memory.writebyte(0x84,c.ready);memory.writebyte(0x85,c.parts);for i,v in ipairs(c.palette)do memory.writebyte(0x48+i,v)end')
        hook=hook.replace("'\"}'\nend", "'\",\"palette\":\"'..hx(0x7c0,32)..'\"}'\nend")
        hook=hook.replace("'\"}'\nend", "'\",\"audioSlots\":\"'..hx(0x106,6)..'\"}'\nend")
        hook=hook.replace('memory.writebyte(0x38+i,0)','memory.writebyte(0x38+i,p.gameover or 0)')
        hook=hook.replace('local kind=read(0x528+slot)', 'if c.audio then for i,v in ipairs(c.audio)do restore[0x105+i]=read(0x105+i);memory.writebyte(0x105+i,v)end end\n local kind=read(0x528+slot)')
    hook=hook.replace(' if kind<16 then return end','')
    hook=hook.replace('function finish_probe()', '''memory.registerexec(0xe54f,1,function()
 for i=0,15 do memory.writebyte(0x4b8+i,0)end
 memory.writebyte(0x528+15,16);memory.writebyte(0x4b8+15,1)
end)
function finish_probe()assert(caseIndex==#cases,'missing native calls');''')
    hook=hook.replace('CASES',lua(cs)).replace('ENEMY',lua([a for _,a in ENEMY])).replace('PLAYER',lua([a for _,a in PLAYER]))
    source=ROOT/'.local/contra-campaign/states/stage-7-v2.fcs';target=BASE/'states/stage-7.fcs';target.parent.mkdir(parents=True,exist_ok=True)
    if target.exists():assert source.read_bytes()==target.read_bytes()
    else:shutil.copyfile(source,target)
    spec={'name':'machines-cases-'+group+'-'+label,'load_state':'stage-7','frames':len(cs),'captures':[],'segments':[],'hooks_lua':hook,'timeout_seconds':180}
    path=BASE/(spec['name']+'.json');path.write_text(json.dumps(spec,indent=2)+'\n');out=BASE/'runs'/spec['name']
    if out.exists():
        assert json.loads((out/'capture-complete.json').read_text())=={'status':'complete'}
        assert json.loads((out/'scenario.json').read_text())==spec
    else:out=run(path)
    rows=[json.loads(s)for s in(out/'machines.jsonl').read_text().splitlines()];assert len(rows)==len(cs)
    data={'source_is_original_execution':True,'interventions':'Explicit entity/world/player/queue input at real E58A dispatch, original result captured before restoring input. Not natural progression.',
          'source':json.loads((out/'source.json').read_text()),'scenario':spec,'cases':cs,'records':rows}
    dest=ROOT/('games/contra/engine/testdata/original-enemy-machines-'+('cases'if group=='flame'else group)+'.json.gz')
    if dest.exists():assert json.loads(gzip.decompress(dest.read_bytes()))['records']==rows
    else:dest.write_bytes(gzip.compress(json.dumps(data,separators=(',',':')).encode(),mtime=0))
    print(json.dumps({'calls':len(rows),'sha256':hashlib.sha256(dest.read_bytes()).hexdigest()}))

if __name__=='__main__':main()
