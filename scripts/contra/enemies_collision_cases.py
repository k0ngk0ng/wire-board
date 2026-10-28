"""Generate declared boundary inputs, never expected collision outcomes."""
import json,argparse
import zipfile
from pathlib import Path

ROOT=Path(__file__).resolve().parents[2]
from enemies_collision_capture import PLAYER,ENEMY

def main():
    parser=argparse.ArgumentParser();parser.add_argument('--special-only',action='store_true');args=parser.parse_args()
    with zipfile.ZipFile('/Users/jason/Documents/nes/roms/03/魂斗罗日版(完全版).zip')as z:prg=z.read('Contra (J).nes')[16:131088]
    word=lambda a:int.from_bytes(prg[a+0x10000:a+0x10002],'little')
    tables=[word(0xe419+i*2)for i in range(5)]
    boxes=[[list(prg[a+0x10000+j*4:a+0x10004+j*4])for j in range(15)]for a in tables]
    paddr=dict(PLAYER);eaddr=dict(ENEMY)
    cases=[]
    def base(kind='players'):
        w={}
        for _,a in PLAYER:
            for i in range(2):w[a+i]=0
        for _,a in ENEMY:
            for i in range(16):w[a+i]=0
        for a in range(0x368,0x4b8):w[a]=0
        for a in range(0x7e0,0x7e6):w[a]=0
        for a in range(0x3c,0x40):w[a]=0
        w.update({0x1c:0,0x30:0,0x40:0,0x7d:0,0x3c:200,0x3e:200})
        for i in range(2):w.update({paddr['state']+i:1,paddr['x']+i:16,paddr['y']+i:32,paddr['lives']+i:2,paddr['sprite']+i:15})
        for name,value in dict(sprite=59,x=120,y=120,kind=5,routine=3,hp=2,scoreCollision=0x10,stateWidth=0).items():w[eaddr[name]+15]=value
        return {'kind':kind,'writes':w}
    def player(c,actor=0,**fields):
        for name,value in fields.items():c['writes'][paddr[name]+actor]=value&255
    def enemy(c,slot=15,**fields):
        for name,value in fields.items():c['writes'][eaddr[name]+slot]=value&255
    def shot(c,slot=15,**fields):
        addresses={'sprite':0x368,'attributes':0x378,'kind':0x388,'y':0x3b8,'x':0x3c8,'timer':0x418,'routine':0x438,'owner':0x448}
        values=dict(sprite=30,kind=1,y=120,x=120,routine=1,owner=0);values.update(fields)
        for name,value in values.items():c['writes'][addresses[name]+slot]=value&255
    def add(c,name):c['name']=name;cases.append(c)
    poses=[('water',0,dict(water=4)),('jump',1,dict(jump=17)),('crouch',2,dict(sprite=23)),('normal',3,{})]
    for pose,profile,fields in poses:
        for code in range(15):
            dy,dx,height,width=boxes[profile][code]
            top,left=(120+dy)&255,(120+dx)&255
            points=[(left,top),(left-1,top),(left,top-1),(left+width-1,top+height-1),(left+width,top+height-1),(left+width-1,top+height),(120,120)]
            for index,(x,y) in enumerate(points):
                c=base();player(c,x=x,y=y,**fields);enemy(c,scoreCollision=0x10|code)
                add(c,f'pose-{pose}-shape-{code}-edge-{index}')
    for flags in (0,64,128,192):
        for length in (0,8,31,128,255):
            for pose,_,fields in poses:
                for x,y in ((120,120),(112,112),(128,128),(96,96),(144,144)):
                    c=base();player(c,x=x,y=y,**fields);enemy(c,scoreCollision=31,attributes=flags,variable1=length)
                    add(c,f'variable-{flags}-{length}-{pose}-{x}-{y}')
    for guard in (dict(state=0),dict(state=2),dict(state=3),dict(spawnInvincible=128),dict(invincible=128),dict(water=4,held=4)):
        c=base();player(c,x=120,y=120,**guard);add(c,'player-guard-'+str(guard))
    for level in (0,6):
        for item in range(8):
            for weapon in (0,1,17,19,32):
                c=base();player(c,x=120,y=120,weapon=weapon);enemy(c,kind=0,attributes=item,scoreCollision=0x22,hp=1);c['writes'][0x30]=level
                add(c,f'pickup-{item}-weapon-{weapon}-level-{level}')
    for hp in (0,1,2,239,240,241,255):
        for protect in (0,1):
            c=base();player(c,x=120,y=120,invincible=128,spawnInvincible=protect);enemy(c,hp=hp)
            add(c,f'shield-hp-{hp}-spawn-{protect}')
    for scene in (0,1,128):
        for flags in (0,64,96,112):
            for vy in (-256,0,256):
                c=base();c['writes'][0x40]=scene
                enemy(c,stateWidth=flags,velocityX=1,velocityXFraction=240,xFraction=32)
                player(c,x=120,y=100,jump=17,vyLo=vy&255,vyHi=(vy>>8)&255,fallLockY=0)
                add(c,f'platform-scene-{scene}-flags-{flags}-vy-{vy}')
    for second in (1,2):
        c=base();player(c,0,x=120,y=120);player(c,1,state=second,x=120,y=120);add(c,f'two-players-state2-{second}')
    for pscore,nextlife,lives,demo in [(190,200,2,0),(195,200,98,0),(195,200,99,0),(65530,200,2,0),(30000,0x7500,2,0),(65535,65530,2,0),(190,200,2,1)]:
        c=base();player(c,x=120,y=120,lives=lives);enemy(c,kind=0,attributes=1,hp=1,scoreCollision=0x22)
        c['writes'].update({0x7e2:pscore&255,0x7e3:pscore>>8,0x3c:nextlife&255,0x3d:nextlife>>8,0x1c:demo})
        add(c,f'score-{pscore}-{nextlife}-{lives}-{demo}')
    # Falcon modifies the whole enemy pool, skipping boxes/capsules and HP F0.
    c=base();player(c,x=120,y=120);enemy(c,kind=0,attributes=6,hp=1,scoreCollision=0x22)
    for i,(kind,hp,sprite)in enumerate([(1,0,1),(2,1,1),(3,1,1),(5,1,1),(6,240,1),(7,241,1),(11,1,0)]):enemy(c,i,kind=kind,hp=hp,sprite=sprite,routine=1)
    add(c,'falcon-pool')
    for scene in (0,1,128):
        for flags in (0,16,32,48):
            for kind in (1,5,129,133):
                for timer in (0,1,2):
                    c=base('bullets');c['writes'][0x40]=scene;enemy(c,stateWidth=flags);shot(c,kind=kind,timer=timer)
                    add(c,f'bullet-filter-{scene}-{flags}-{kind}-{timer}')
    for code in range(15):
        dy,dx,height,width=boxes[4][code];top,left=(120+dy)&255,(120+dx)&255
        for x,y in ((left,top),(left+1,top+1),(left+width,top+height),(left+width+1,top+height),(120,120)):
            c=base('bullets');enemy(c,scoreCollision=0x10|code);shot(c,x=x,y=y);add(c,f'bullet-shape-{code}-{x}-{y}')
    for hp in (0,1,2,239,240,241,255):
        for sound in range(5):
            c=base('bullets');enemy(c,hp=hp,stateWidth=4,variableA=sound);shot(c);add(c,f'bullet-hp-{hp}-sound-{sound}')
    for level in range(8):
        for kind in range(16):
            c=base('bullets');c['writes'][0x30]=level;enemy(c,kind=kind,hp=1,scoreCollision=0xa0);shot(c);add(c,f'kill-level-{level}-kind-{kind}')
    for slot in (8,14,15):
        for early in (0,1,2):
            c=base('bullets');shot(c,slot,kind=5);baseSlot=0 if slot<10 else 10
            shot(c,baseSlot,routine=early,sprite=0,x=0,y=0,owner=1)
            add(c,f'laser-slot-{slot}-early-{early}')
    for edge in (0,1,7,8,127,128,240,248,255):
        c=base('bullets');enemy(c,x=edge);shot(c,x=(edge+128)&255);add(c,f'byte-wrap-{edge}')
    c=base('bullets');shot(c,0);shot(c,15,owner=1);add(c,'two-bullets-one-hit')
    filename='cases.json'
    if args.special_only:
        cases=[];filename='cases-special.json'
        for level,count in enumerate([3,17,6,17,7,5,9,7]):
            for kind in range(16,16+count):
                c=base('bullets');c['writes'][0x30]=level;enemy(c,kind=kind,hp=1,scoreCollision=0xa0);shot(c)
                add(c,f'special-kill-level-{level}-kind-{kind}')
        for pscore,nextlife,lives,demo in [(0,200,2,0),(65530,65530,99,0),(65535,65535,255,0),(1000,0x7500,2,0),(0,200,2,1)]:
            c=base('bullets');enemy(c,kind=16,hp=1,scoreCollision=0xa0);shot(c)
            player(c,lives=lives)
            c['writes'].update({0x7e2:pscore&255,0x7e3:pscore>>8,0x3c:nextlife&255,0x3d:nextlife>>8,0x1c:demo})
            add(c,f'special-score-{pscore}-{nextlife}-{lives}-{demo}')
    out=ROOT/'.local/contra-enemies-collision'/filename;out.parent.mkdir(parents=True,exist_ok=True);out.write_text(json.dumps(cases,separators=(',',':'))+'\n')
    print(json.dumps({'cases':len(cases),'path':str(out)}))

if __name__=='__main__':main()
