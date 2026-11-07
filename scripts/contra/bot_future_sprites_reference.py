"""Independent static-ROM pixel/source audit of bounded late-stage sprite art.

Reads the already pinned local research ROM as data only; no emulator executes,
no captured after-state drives game logic, and no current World/slot is queried.
"""
import base64,gzip,hashlib,json,re
from pathlib import Path
from sprites_decode import SpriteBank,render_oam
ROOT=Path(__file__).resolve().parents[2];E=ROOT/'docs/research/nes-contra/evidence';ROM=ROOT/'.local/contra-raster/oracle/contra-j.nes';SHA='6ba53139fa88b8de1ae527c438bda6f1541d1ee7df26d63dec5164a32d166bfe'
def sha(p):return hashlib.sha256(p.read_bytes()).hexdigest()
def main():
 raw=ROM.read_bytes();assert hashlib.sha256(raw).hexdigest()==SHA;prg=raw[16:131088];chr=raw[131088:];decoder=SpriteBank(prg);rgb=(ROOT/'.local/contra-background/runs/background-v1/background-colors.rgb').read_bytes();cases=[]
 def art(level,alternate=False):
  d=json.loads((ROOT/f'games/contra/engine/leveldata/stage-{level+1}.json').read_text());info=d['stages'][0];presets={p['id']:p['pages']for p in d['chrPresets']};desc=info['alternate']if alternate else info['initialPaletteState'];preset=info['alternate']['chrPreset']if alternate else info['chrPreset'];return list(presets[preset]),list(desc['paletteIndices'])
 def add(level,mapping,indices,codes,attrs,kind,label='enemy'):
  palette=bytes(v for n in indices for v in [15,*prg[0x1d0e3+n*3:0x1d0e6+n*3]])
  assert len(palette)==32
  for code in codes:
   for attr in attrs:
    before=bytearray(256)
    for i in range(0,256,4):before[i]=244
    oam,_,_=decoder.compose({'code':code,'x':128,'y':112,'flags':attr,'offset':0,'remaining':63,'stride':4},before)
    ppu=bytearray(0x4000)
    for slot,page in enumerate(mapping):ppu[slot*1024:(slot+1)*1024]=chr[page*1024:(page+1)*1024]
    ppu[0x3f00:0x3f20]=palette;img,_=render_oam(oam,ppu,rgb,0xb0,0x14);box=img.getbbox();assert box
    left,top,right,bottom=box;data=img.crop(box).tobytes()
    cases.append({'levelResource':level,'mapping':mapping[:],'paletteIndices':indices[:],'palette':list(palette),'code':code,'attributes':attr,'class':label,'kind':kind,'width':right-left,'height':bottom-top,'anchorX':128-left,'anchorY':112-top,'rgba':base64.b64encode(data).decode(),'rgbaSHA256':hashlib.sha256(data).hexdigest()})
 for level in [4,5,6]:
  mapping,pal=art(level);add(level,mapping,pal,[189,190],[0],14);add(level,mapping,pal,[31],[0],0,'ambiguous')
  if level==6:add(level,mapping,pal,[32],[6],0,'ambiguous');add(level,mapping,pal,[42,43],[0],253)
 mapping,pal=art(5,True);add(5,mapping,pal,[187,188],[0],20)
 for group in [86,81,82]:
  colors=pal[:];colors[7]=group;add(5,mapping,colors,[184,185,186,195],[0,64],19)
 assert len(cases)==38
 mapping,pal=art(4);add(4,mapping,pal,[116,117,118,119],[0,32],17);add(4,mapping,pal,[196],[0],0,'scenery')
 mapping,pal=art(4,True);add(4,mapping,pal,[124,125],[0],21)
 for page in [81,108,109,110]:
  variant=mapping[:];variant[5]=page;add(4,variant,pal,[126],[0],21)
 add(4,mapping,pal,[34],[0],0,'ambiguous')
 mapping,pal=art(7);add(7,mapping,pal,[172,173,174,175],[0,192],17);add(7,mapping,pal,[176,177,178],[0],19);add(7,mapping,pal,[179,180,181,182],[0,128],20)
 assert len(cases)==73
 # Keep source-domain evidence separate from legal artwork enumeration: some
 # controlled native records intentionally seed impossible prior Sprite/attr.
 native=json.loads(gzip.decompress((E/'enemies-machines-rea.json.gz').read_bytes()));addresses={'0xf02f','0xf041','0xf052','0xf085','0xf13c','0xae6d','0xaeaf','0xb1ab','0xb20a','0xb212','0xedad','0xacb8'};refs=[]
 for row in native['records']:
  ev=row['evidence']
  if ev.get('parameters',{}).get('procedure')in addresses:refs.append({'procedure':ev['parameters']['procedure'],'id':ev['evidence_id']})
 sources=['games/contra/engine/enemies_machines_common.go','games/contra/engine/enemies_machines_giant.go','games/contra/engine/enemies_machines_hangar.go','games/contra/engine/enemies_snow_small.go','games/contra/engine/enemies_snow_carrier.go','games/contra/engine/enemies_snow_tables.go','games/contra/engine/enemies_aliens_fetus.go','games/contra/engine/enemies_aliens_blob.go','games/contra/engine/enemies_aliens_spider.go','games/contra/engine/enemies_all_tables.go','games/contra/engine/enemies_shooters_tables.go','games/contra/engine/presentation_sprites_data.json']
 sources += [f'games/contra/engine/leveldata/stage-{i}.json'for i in [5,6,7,8]]
 payload={'romSHA256':SHA,'scope':'Bounded static original art; original execution separately verifies numeric state transitions. No natural campaign-completion claim.','cases':cases,'rea':refs,'sourceHashes':{p:sha(ROOT/p)for p in sources},'ambiguities':['Mortar32 attr6 vs friendly spread32 attr0: compact attr bit2 is not rendered.','Turret bullet31 attr0 vs friendly M/S31 attr0.','Snow bomb34 attr0 vs friendly F34 attr0.','Moving and initially stationary carts share42/43 attr0; use one visible cart label.'],'controlledCaseLimit':'Do not promote preserved artificial Sprite1/attr1 values in staged noninitial routines into legal sprite/palette combinations.'}
 target=E/'bot-future-sprites-original-art.json.gz';target.write_bytes(gzip.compress(json.dumps(payload,separators=(',',':')).encode(),mtime=0));print(json.dumps({'cases':len(cases),'sha256':sha(target),'reaReferences':len(refs)}))
if __name__=='__main__':main()
