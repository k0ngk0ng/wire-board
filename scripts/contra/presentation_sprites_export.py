"""Export original whole-scene oracles and source-derived semantic sprite data."""
import argparse,gzip,hashlib,json,re,zipfile
from pathlib import Path

ROOT=Path(__file__).resolve().parents[2]
BASE=ROOT/'.local/contra-presentation-sprites'
ROM_SHA='6ba53139fa88b8de1ae527c438bda6f1541d1ee7df26d63dec5164a32d166bfe'


def sha(data):return hashlib.sha256(data).hexdigest()


def main():
 parser=argparse.ArgumentParser();parser.add_argument('--runtime-data',action='store_true')
 parser.add_argument('--first-label',default='v1');parser.add_argument('--natural-label',default='v2');parser.add_argument('--controlled-label',default='v3');args=parser.parse_args()
 assert all(re.fullmatch(r'[a-z0-9-]+',v)for v in [args.first_label,args.natural_label,args.controlled_label])
 with zipfile.ZipFile('/Users/jason/Documents/nes/roms/03/魂斗罗日版(完全版).zip')as z:rom=z.read('Contra (J).nes')
 assert sha(rom)==ROM_SHA;prg=rom[16:16+131072]
 old=json.loads(gzip.decompress((ROOT/'docs/research/nes-contra/evidence/sprites-catalog.json.gz').read_bytes()))
 frames=[]
 for source in old['sprites']:
  code=source['code'];pointer=int.from_bytes(prg[0x93da+(code-1)*2:0x93dc+(code-1)*2],'little')
  assert pointer==source['address'];header=prg[pointer];parts=[]
  if header==0xfe:
   parts=[dict(source['parts'][0])]
  elif header:
   cursor=pointer+1;recoil=True
   for _ in range(header):
    bias=None
    if prg[cursor]==0x80:
     bias=prg[cursor+1];cursor=int.from_bytes(prg[cursor+1:cursor+3],'little');recoil=False
    y,tile,attr,x=prg[cursor:cursor+4];cursor+=4
    part={'dy':y-256 if y&128 else y,'tile':tile,'attr':attr,'dx':x-256 if x&128 else x,'recoil':recoil}
    if bias is not None:part['generationBias']=bias
    parts.append(part)
  assert [{k:v for k,v in p.items()if k!='generationBias'}for p in parts]==source['parts']
  frames.append({'code':code,'kind':source['kind'],'parts':parts})
 initial=[{'x':prg[0xa85b+15-i],'y':prg[0xa84b+15-i],'flags':prg[0xa86b+15-i]}for i in range(16)]
 data={'schema':1,'sourceRomSHA256':ROM_SHA,'frames':frames,'initialSnow':initial,
       'snowVelocityY':list(prg[0x9280:0x9282]),'snowVelocityX':list(prg[0x9282:0x9284]),
       'hudTiles':list(prg[0x93ca:0x93d2]),'hudX':list(prg[0x93d2:0x93da])}
 raw=json.dumps(data,separators=(',',':')).encode()+b'\n'
 if args.runtime_data:
  (ROOT/'games/contra/engine/presentation_sprites_data.json').write_bytes(raw)
  print(json.dumps({'runtimeDataSHA256':sha(raw),'frames':len(frames),'generationWrites':sum('generationBias'in p for f in frames for p in f['parts'])}))
  return
 natural=json.loads(gzip.decompress((BASE/'runtime'/f'composition-natural-{args.natural_label}.json.gz').read_bytes()))
 controlled=json.loads(gzip.decompress((BASE/'runtime'/f'composition-controlled-{args.controlled_label}.json.gz').read_bytes()))
 first=json.loads(gzip.decompress((BASE/'runtime'/f'composition-natural-{args.first_label}.json.gz').read_bytes()))
 assert natural['records']==4868 and controlled['records']==39
 for a,b in zip(first['traces'],natural['traces']):
  assert a['ramSHA256']==b['ramSHA256']
  assert [{k:v for k,v in r.items()if k!='endFrame'}for r in a['records']]==[{k:v for k,v in r.items()if k!='endFrame'}for r in b['records']]
 observed={'sourceRomSHA256':ROM_SHA,'natural':natural,'controlled':controlled,'expectedInitialSnow':initial,
           'expectedRuntimeDataSHA256':sha(raw)}
 out=ROOT/'games/contra/engine/testdata';out.mkdir(parents=True,exist_ok=True)
 packed=gzip.compress(json.dumps(observed,separators=(',',':')).encode(),mtime=0)
 (out/'original-presentation-sprites.json.gz').write_bytes(packed)
 proof={'sourceRomSHA256':ROM_SHA,'fixtureSHA256':sha(packed),'runtimeDataSHA256':sha(raw),
        'naturalRecords':4868,'controlledRecords':39,'naturalTraces':10,'controlledCases':20,
        'naturalRepeatRAMIdentical':True,'source':'Original complete 919F execution, no output substitution.',
        'generationBiasSource':'92D9 shared component pointer low byte; write events retained even when value unchanged.',
        'limitations':['Controlled overloaded linear mode crosses video frames; capture drains only the already-entered call.',
                       'Pixel submission remains the next World presentation boundary; tests here validate composition output.']}
 (out/'original-presentation-sprites-source.json').write_text(json.dumps(proof,indent=2)+'\n')
 print(json.dumps(proof))


if __name__=='__main__':main()
