"""Declared C001 boundary contexts for spatial-owner contracts; no output patches."""
from pathlib import Path
import ast,copy,gzip,hashlib,json
ROOT=Path(__file__).resolve().parents[2];BASE=ROOT/'.local/contra-terrain-work'

def literal(path,name):
    for n in ast.parse(path.read_text()).body:
        if isinstance(n,ast.Assign)and any(isinstance(t,ast.Name)and t.id==name for t in n.targets):return ast.literal_eval(n.value)
    raise ValueError(name)

PF=literal(ROOT/'scripts/contra/static_capture_playerstep.py','PLAYER_FIELDS')
EF=literal(ROOT/'scripts/contra/enemies_capture.py','FIELDS')

def base(name):
    w={}
    for _,a in PF:
        for i in range(2):w[a+i]=0
    for _,a in EF:
        for i in range(16):w[a+i]=0
    for a in range(0x368,0x4b8):w[a]=0
    for a in range(0x680,0x700):w[a]=0x55
    w.update({0x18:5,0x2c:4,0x90:1,0x91:0,0x38:0,0x39:1,0x32:30,0x33:0,0x334:100,0x31a:128,0xd6:15,0xae:255,0xc0:2,
              0x75:0,0x76:0,0x77:0,0x21:0,0x23:0,0x59:0,0x7f:0,0x84:0,0x85:0,0x3b:0,0x7d:0,0x6a:0,0x64:1,0x65:176,0xfc:224,0xfd:0,0xff:0xb0})
    return {'name':name,'writes':w}

def queue_prefix():
    path=BASE/'runs/stage8-v2/ram.bin';raw=path.read_bytes()
    for n in range(len(raw)//2048):
        b=raw[n*2048:(n+1)*2048];q=b[0x21];pos=0
        while pos<q and b[0x700+pos]in [1,2]:
            size=3+b[0x702+pos]*(2+b[0x701+pos])
            if size<3 or pos+size>q:break
            pos+=size
        if pos>=64 and(pos==q or pos+1==q and b[0x700+pos]==0):
            return b[0x700:0x700+pos],{'path':str(path.relative_to(ROOT)),'frame':n+1,'length':pos,'sha256':hashlib.sha256(b[0x700:0x700+pos]).hexdigest()}
    raise AssertionError('no complete original occupied queue')

def main():
    cases=[]
    for y in [16,96,224,239,240,250,255]:
        c=base(f'player-foot-y-{y}');c['writes'][0x31a]=y;cases.append(c)
    for x in [8,100,240]:
        c=base(f'player-side-scan-x-{x}');c['writes'].update({0x334:x,0x31a:112,0xa0:1,0xf1:1,0xf5:1,0x00:1,0x04:1})
        for a in range(0x680,0x700):c['writes'][a]=0
        cases.append(c)
    c=base('player-drop-empty-scan');c['writes'].update({0x31a:96,0xda:1,0xf1:132,0xf5:132,0x00:132,0x04:132})
    for a in range(0x680,0x700):c['writes'][a]=0
    cases.append(c)
    for y in [16,128,223,224,239,250]:
        c=base(f'weapon-query-y-{y}');w=c['writes'];w.update({0x59:128,0x368:30,0x388:1,0x438:1,0x3c8:100,0x3b8:y})
        cases.append(c)
    for address in [0x2000,0x2001,0x201e,0x201f,0x2400,0x2401,0x241e,0x241f]:
        c=base(f'stream-column-{address:04x}');c['writes'].update({0x75:1,0x65:7,0x60:address&31,0x62:address&255,0x63:address>>8,0x69:64 if address&0x400 else 0});cases.append(c)
    prefix,proof=queue_prefix()
    source=json.loads(gzip.decompress((ROOT/'games/contra/engine/testdata/original-enemy-collision-tick-aliens-locator.json.gz').read_bytes()))
    template=source['records'][0]['before']['enemies'][13]
    guarded=[]
    for c in cases:
        for routine in [10,11]:
            g=copy.deepcopy(c);g['name']+=f'-guardian-{routine}';g['guardianRoutine']=routine;g['queuePrefix']=proof;w=g['writes']
            for i,(_,a)in enumerate(EF):w[a+13]=template[i]
            w[0x4b8+13]=routine;w[0x5c8+13]=1
            for i,v in enumerate(prefix):w[0x700+i]=v
            w[0x21]=len(prefix)
            guarded.append(g)
    vertical=[]
    for axis in [0,1,14,15,28,29]:
        c=base(f'stream-row-{axis}');c['writes'].update({0x64:0,0x65:7,0x75:1,0x60:axis,0x62:(0x2000+axis*32)&255,0x63:(0x2000+axis*32)>>8,0x69:0});vertical.append(c)
    indoor=[]
    for x in [8,64,127,128,129,192,248]:
        c=base(f'indoor-projectile-perspective-x-{x}');c['writes'].update({0x64:0,0x65:0,0x334:x,0x31a:176,0xf1:64,0xf5:64,0x00:64,0x04:64});indoor.append(c)
        c=base(f'indoor-player-perspective-x-{x}');c['writes'].update({0x64:0,0x65:0,0x334:x,0x31a:176,0x6a:1,0xf1:8,0xf5:8,0x00:8,0x04:8});indoor.append(c)
        c=base(f'indoor-player-two-perspective-x-{x}');c['writes'].update({0x64:0,0x65:0,0x334:37,0x335:x,0x31b:176,0x6a:1,0x90:0,0x91:1,0x38:1,0x39:0,0x32:0,0x33:30,0xaf:255,0xd7:15,0xf2:8,0xf6:8,0x01:8,0x05:8});indoor.append(c)
    for name,rows in [('guardian',guarded),('vertical',vertical),('indoor',indoor)]:
        (BASE/f'cases-{name}.json').write_text(json.dumps(rows,indent=2)+'\n');print(name,len(rows))

if __name__=='__main__':main()
