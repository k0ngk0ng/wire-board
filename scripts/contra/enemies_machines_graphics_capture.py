"""Source C593 oracle, before independent tile-animation implementation.

Failure inventory: wrong level/bank/table; reading exactly four tiles for a
three-row claw; gate 0x40 instead of 0x50; projected-size gate; masking the
palette header; discarding skipped-palette lookup; accidental 06/23/map writes;
240-pixel wrap and horizontal carry; mutation beyond the appended queue range.
Controlled cases alter only declared input at the native function boundary,
and restore it after recording the actual original return.
"""
import gzip,hashlib,json,os,shutil,sys,zipfile
from pathlib import Path
ROOT=Path(__file__).resolve().parents[2]
BASE=ROOT/'.local/contra-enemies-machines/graphics-oracle'
os.environ['CONTRA_REFERENCE_ROOT']=str(BASE)
from capture_reference import run
from enemies_terrain_capture import HOOK as BASE_HOOK

def main():
    mode=sys.argv[1]if len(sys.argv)>1 else'controlled'
    label=sys.argv[2]if len(sys.argv)>2 else'v1'
    with zipfile.ZipFile('/Users/jason/Documents/nes/roms/03/魂斗罗日版(完全版).zip')as z:rom=z.read('Contra (J).nes')
    assert hashlib.sha256(rom).hexdigest()=='6ba53139fa88b8de1ae527c438bda6f1541d1ee7df26d63dec5164a32d166bfe'
    prg=rom[16:131088]
    definitions={}
    for level,count in [(1,11),(3,11),(5,14),(6,12)]:
        at=int.from_bytes(prg[0x1c63a+level*2:0x1c63c+level*2],'little')
        raw=prg[at+0x4000:at+0x4000+count*5]
        definitions[level]={'cpu':at,'prgOffset':at+0x4000,'raw':raw.hex(),'sha256':hashlib.sha256(raw).hexdigest()}
    source=ROOT/'.local/contra-campaign/states/stage-7-v2.fcs'
    target=BASE/'states/stage-7.fcs';target.parent.mkdir(parents=True,exist_ok=True)
    if target.exists():assert target.read_bytes()==source.read_bytes()
    else:shutil.copyfile(source,target)
    hook=BASE_HOOK.replace('0xc3d1','0xc593')
    hook=hook.replace('"address":%d,"queue"','"address":%d,"attributeLookup":%d,"offset06":%d,"queue"')
    hook=hook.replace('word(0x12),hex(0x700','word(0x12),read(0xf),read(6),hex(0x700')
    hook=hook.replace('{0x21,0x23,0xfc,0xfd,0xff}','{0x21,0x23,0xfc,0xfd,0xff,0x30,0x46,0x47,0xf,6}')
    hook=hook.replace('for a=0x700,0x77f', 'for a=0x700,0x77f')
    hook=hook.replace('memory.writebyte(0x10,c.code);', '''memory.writebyte(0x30,c.level);memory.writebyte(0x46,c.attr%256);memory.writebyte(0x47,math.floor(c.attr/256));memory.writebyte(0xf,c.lookup);memory.writebyte(6,c.offset06)
  for a=0x600,0x67f do restore[a]=read(a);memory.writebyte(a,0)end
  memory.writebyte(0x10,c.code);''')
    start=hook.index('  -- One native weapon-box')
    end=hook.index('\n end)\nend\nfunction finish_probe',start)
    hook=hook[:start]+'''  -- One original claw draw per frame, before its native dispatcher.
  for i=0,15 do memory.writebyte(0x4b8+i,0)end
  local i=15
  memory.writebyte(0x528+i,16);memory.writebyte(0x4b8+i,3)
  memory.writebyte(0x538+i,0);memory.writebyte(0x5c8+i,2)
  memory.writebyte(0x5d8+i,0);memory.writebyte(0x5e8+i,0)
  memory.writebyte(0x33e+i,160);memory.writebyte(0x324+i,80)
  memory.writebyte(0x30a+i,1)
'''+hook[end:]
    cases=[]
    if mode=='controlled':
        for lv,definition in definitions.items():
            attr=int.from_bytes(prg[0xa87b+lv*32+6:0xa87b+lv*32+8],'little')
            for code in range(len(bytes.fromhex(definition['raw']))//5):
                for skip in (0,128):
                    for x,y,sx,sy in [(0,16,0,224),(16,16,0,224),(0,32,0,224),(16,32,0,224),(32,32,244,224)]:
                        for used in (0,79,80,255):
                            cases.append(dict(level=lv,code=code|skip,x=x,y=y,sx=sx,sy=sy,used=used,mode=255,control=0xb0,attr=attr,lookup=(code+used+17)&255,offset06=(code*7+used)&255))
        spec={'name':'tile-animation-'+label,'load_state':'stage-7','frames':len(cases),'captures':[],'segments':[],'timeout_seconds':180}
        lua='local cases={'+','.join('{'+','.join(f'{k}={v}'for k,v in c.items())+'}'for c in cases)+'}'
    else:
        original=ROOT/'.local/contra-enemies-machines/oracle/runs/machines-natural-v1-7/scenario.json'
        spec={k:v for k,v in json.loads(original.read_text()).items()if k!='hooks_lua'}
        spec.update(name='tile-animation-natural-'+label,load_state='stage-7')
        lua='local cases=nil'
    spec['hooks_lua']=hook.replace('CASES',lua)
    path=BASE/(spec['name']+'.json');path.write_text(json.dumps(spec,indent=2)+'\n')
    out=BASE/'runs'/spec['name']
    if out.exists():
        assert json.loads((out/'capture-complete.json').read_text())=={'status':'complete'}
        assert json.loads((out/'scenario.json').read_text())==spec
    else:out=run(path)
    records=[json.loads(line)for line in(out/'graphics-requests.jsonl').read_text().splitlines()]
    assert records
    for r in records:
        for key in ['supertiles','collision','mode','offset06']:assert r['before'][key]==r['after'][key],key
    data={'source_is_original_execution':True,'scope':mode,'interventions':'Controlled C593 input only, original return captured; restored before continuing. Natural variant uses controller-only continuation of declared stage-7 entry.',
          'source':json.loads((out/'source.json').read_text()),'scenario':spec,'definitions':definitions,'cases':cases,'records':records}
    dest=ROOT/f'games/contra/engine/testdata/original-enemy-machines-graphics-{mode}.json.gz'
    if dest.exists():assert json.loads(gzip.decompress(dest.read_bytes()))['records']==records
    else:dest.write_bytes(gzip.compress(json.dumps(data,separators=(',',':')).encode(),mtime=0))
    print(json.dumps({'records':len(records),'accepted':sum(r['carry']==0 for r in records),'sha256':hashlib.sha256(dest.read_bytes()).hexdigest()}))

if __name__=='__main__':main()
