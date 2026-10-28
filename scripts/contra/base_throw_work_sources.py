"""Verify the narrow creation-work identities against the pinned Japanese ROM."""
from pathlib import Path
import gzip,hashlib,json,re,zipfile
ROOT=Path(__file__).resolve().parents[2]
sha=lambda b:hashlib.sha256(b).hexdigest()
with zipfile.ZipFile('/Users/jason/Documents/nes/roms/03/魂斗罗日版(完全版).zip')as z:raw=z.read('Contra (J).nes')
assert sha(raw)=='6ba53139fa88b8de1ae527c438bda6f1541d1ee7df26d63dec5164a32d166bfe'
prg=raw[16:];tables={}
def table(name,bank,address,n):
 offset=bank*8192+address-(0xc000 if bank==14 else 0x8000)
 b=prg[offset:offset+n];tables[name]={'bank8KiB':bank,'cpu':hex(address),'romOffset':16+offset,'bytes':list(b),'sha256':sha(b)};return b
family=table('projectile-dispatch-families',12,0x85c6,24)
flame=table('indoor-flame-offset-families',12,0x872c,12)
explosions=table('enemy-explosion-families',14,0xe754,8)
code=(ROOT/'games/contra/engine/base_throw_work.go').read_text()
def hexes(text):return [int(x,16)for x in re.findall(r'0x([0-9a-f]+)',text)]
family_code=re.search(r'var projectileFamilyIdentity = \[2\]\[5\]uint8\{(.*?)\n\}',code,re.S).group(1)
assert hexes(family_code)==[family[i*2]for i in [1,2,3,4,5,7,8,9,10,11]]
flame_code=re.search(r'var indoorFlameIdentity = \[3\]uint8\{(.*?)\}',code,re.S).group(1)
assert hexes(flame_code)==[flame[i*4+2]for i in range(3)]
assert list(explosions[::2])==[0x5c,0x5f,0x63,0x66]
inv=json.loads((ROOT/'docs/research/nes-contra/evidence/enemies-all-stage-data.json').read_text());expected=[]
for stage in inv['stages']:
 address=int(stage['screenTable'],16)
 values=[]
 for i,s in enumerate(stage['scripts']):
  pointer=int.from_bytes(prg[address+i*2:address+i*2+2],'little');assert pointer==int(s['pointer'],16);values.append(pointer&255)
 expected.extend(values)
 tables['screen-'+str(stage['level'])]={'prgOffset':address,'identities':values,'pointerTableSHA256':sha(prg[address:address+len(values)*2])}
assert hexes((ROOT/'games/contra/engine/base_throw_tables.go').read_text())==expected
names=[
'.local/contra-weapons/final-rea/function-0x8043.json',
'.local/contra-weapons/final-rea/function-0x859b.json',
'.local/contra-weapons/final-rea/function-0x886a.json',
'.local/contra-static/playerstep-rea/function-0xda5e.json',
'.local/contra-enemies-base/core-rea/function-0x8d37.json',
'.local/contra-enemies-base/core-rea/function-0x95d1.json',
'.local/contra-enemies-base/extra-rea/function-0x8ff7.json',
'.local/contra-enemies-base/core-rea/function-0xe6e1.json',
'.local/contra-enemies-base/extra-rea/function-0x947d.json',
'.local/contra-enemies-base/extra-rea/function-0xea11.json',
'.local/contra-enemies-base/extra-rea/function-0x971e.json',
'.local/contra-enemies-collision/all-rea/function-0xe342.json',
'.local/contra-enemies-shooters/aim-rea/function-0xf225.json',
'.local/contra-enemies-shooters/aim-rea/function-0xf29a.json']
records=[]
for name in names:
 p=ROOT/name;data=p.read_bytes();records.append({'path':name,'sha256':sha(data),'evidence':json.loads(data)})
rea=ROOT/'docs/research/nes-contra/evidence/base-throw-work-rea.json.gz'
rea.write_bytes(gzip.compress(json.dumps({'romSHA256':sha(raw),'records':records},separators=(',',':')).encode(),mtime=0))
result={'passed':True,'romSHA256':sha(raw),'tables':tables,'rea':{'path':str(rea.relative_to(ROOT)),'sha256':sha(rea.read_bytes()),'records':len(records)},'scope':'source identity bytes and complete retained REA function evidence; no runtime ROM interpretation'}
(ROOT/'docs/research/nes-contra/evidence/base-throw-work-sources.json').write_text(json.dumps(result,indent=2)+'\n')
print(json.dumps({'tables':len(tables),'rea':len(records),'passed':True}))
