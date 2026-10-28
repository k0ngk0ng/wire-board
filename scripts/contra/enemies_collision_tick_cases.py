"""Construct explicit whole-phase ordering inputs; original execution decides outcomes."""
import argparse,gzip,json
from pathlib import Path
from enemies_collision_tick_capture import ROOT,PLAYER,ENEMY

def main():
    parser=argparse.ArgumentParser();parser.add_argument('--boss',action='store_true');parser.add_argument('--bias',action='store_true');args=parser.parse_args()
    paddr=dict(PLAYER);eaddr=dict(ENEMY);cases=[]
    def base(name):
        writes={}
        for _,a in PLAYER:
            for i in range(2):writes[a+i]=0
        for _,a in ENEMY:
            for i in range(16):writes[a+i]=0
        for a in range(0x368,0x4b8):writes[a]=0
        for a in range(0x7e0,0x7e6):writes[a]=0
        for a in range(0x3c,0x40):writes[a]=0
        for a in range(0x700,0x780):writes[a]=0
        for a in range(0x680,0x700):writes[a]=0x55
        writes.update({0x1a:8,0x1c:0,0x2f:0,0x30:0,0x31:0,0x34:0,0x40:0,0x41:0,0x59:0,0x68:0,0x8e:1,0xfc:224,0xfd:0,0xff:0xb0,0x7d:0,0x21:0,0x3c:200,0x3e:200,0x7e0:200})
        for i in range(2):writes.update({paddr['state']+i:1,paddr['x']+i:32+i*16,paddr['y']+i:100,paddr['sprite']+i:15,paddr['lives']+i:2,paddr['onEnemy']+i:i+1})
        return {'name':name,'writes':writes}
    def enemy(c,slot,**fields):
        defaults=dict(sprite=1,routine=1,x=180,y=116,kind=1,hp=2,scoreCollision=0x10,stateWidth=0)
        defaults.update(fields)
        for name,value in defaults.items():c['writes'][eaddr[name]+slot]=value
    def player(c,i,**fields):
        for name,value in fields.items():c['writes'][paddr[name]+i]=value
    def projectile(c,i,**fields):
        addresses={'sprite':0x368,'kind':0x388,'routine':0x438,'x':0x3c8,'y':0x3b8,'owner':0x448,'yFraction':0x398}
        for name,value in fields.items():c['writes'][addresses[name]+i]=value
    cases.append(base('empty-clears-on-enemy'))
    for explosion_slot,damage_slot in [(15,14),(14,15)]:
        c=base(f'ordered-sounds-explosion-{explosion_slot}')
        enemy(c,explosion_slot,kind=5,routine=7,stateWidth=2,x=200)
        enemy(c,damage_slot,kind=6,routine=1,stateWidth=4,x=120,y=116,hp=3)
        projectile(c,15,sprite=30,kind=1,routine=1,x=120,y=116,owner=0)
        cases.append(c)
    for first_state in [1,2]:
        c=base(f'target-refresh-p0-state-{first_state}')
        player(c,0,state=first_state,x=32,y=100);player(c,1,x=80,y=100)
        enemy(c,15,kind=1,routine=2,sprite=30,x=80,y=100,scoreCollision=1,stateWidth=0x80)
        enemy(c,14,kind=6,routine=3,x=128,y=100,attackDelay=1,variable4=1,stateWidth=0x83)
        cases.append(c)
    c=base('falcon-changes-future-slot');player(c,1,x=120,y=100)
    enemy(c,15,kind=0,routine=3,x=120,y=100,attributes=6,stateWidth=0x80,scoreCollision=0x22,hp=1)
    enemy(c,14,kind=5,routine=3,x=180,y=116,hp=1,stateWidth=2)
    cases.append(c)
    for used in [0,63,64]:
        c=base(f'box-create-live-lower-slot-used-{used}');c['writes'][0x21]=used
        enemy(c,15,kind=2,routine=5,x=120,y=120,hp=0,scoreCollision=0x32,attributes=1)
        cases.append(c)
    data=json.loads(gzip.decompress((ROOT/'games/contra/engine/testdata/original-enemy-shooters-turrets.json.gz').read_bytes()))
    row=next(r for r in data['records']if any(t['Index']==134 for t in r.get('Targets',[]))) if 'Targets'in data['records'][0]else next(r for r in data['records']if any(t['index']==134 for t in r.get('targets',[])))
    source=data['cases'][row['case']-1]
    for used in [0,64]:
        c=base(f'red-turret-real-weapon-alias-used-{used}');w=c['writes']
        for i,(_,address)in enumerate(ENEMY):w[address+15]=source['values'][i]
        w.update({0x21:used,0x68:source['scroll'],0x1a:source['frameCounter'],0x34:source['random'],0x31:source['loop'],0x2f:source['strength'],0x8e:source['enabled'],0x3ba:source['aliasX'],0x3a0:source['aliasY']})
        for i,p in enumerate(source['players']):player(c,i,**p)
        cases.append(c)
    filename='tick-cases.json'
    if args.boss:
        cases=[];filename='tick-boss-cases.json'
        c=base('boss-reset-discards-prior-and-retains-later-audio')
        c['writes'].update({0x2a:17,0x2b:3,0x3b:0})
        enemy(c,15,kind=5,routine=7,stateWidth=2,x=200)
        enemy(c,14,kind=17,routine=3,hp=0,stateWidth=12,scoreCollision=2,x=160,y=176)
        enemy(c,13,kind=2,routine=9,hp=1,stateWidth=2,x=200)
        cases.append(c)
        c=base('boss-tunnel-finished-preserves-boss-marker')
        c['writes'].update({0x2a:17,0x2b:3,0x3b:1})
        enemy(c,15,kind=17,routine=7,sprite=0,hp=0,stateWidth=0x8d,scoreCollision=2,x=160,y=176,animationDelay=2,variable1=8)
        cases.append(c)
        for used in [0,64]:
            c=base(f'boss-tunnel-request-used-{used}')
            c['writes'].update({0x2a:17,0x2b:3,0x3b:1,0x21:used})
            enemy(c,15,kind=17,routine=7,sprite=0,hp=0,stateWidth=0x8d,scoreCollision=2,x=160,y=176,animationDelay=1,variable1=0)
            cases.append(c)
        c=base('two-boss-resets-in-one-phase')
        c['writes'].update({0x2a:17,0x2b:3,0x3b:0})
        for slot in [15,14]:enemy(c,slot,kind=17,routine=3,hp=0,stateWidth=12,scoreCollision=2,x=160,y=176)
        cases.append(c)
    if args.bias:
        filename=filename.replace('.json','-bias.json')
        if not args.boss:cases[0]['writes'][6]=0xa7
    out=ROOT/'.local/contra-enemies-collision'/filename;out.parent.mkdir(parents=True,exist_ok=True);out.write_text(json.dumps(cases,separators=(',',':'))+'\n')
    print(json.dumps({'cases':len(cases),'output':str(out)}))

if __name__=='__main__':main()
