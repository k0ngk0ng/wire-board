#!/usr/bin/env python3
"""Read native resources; expand appearance and timing metadata without copying art."""
import argparse,hashlib,json,math,re,struct
from pathlib import Path

ROOT=Path(__file__).resolve().parents[3]
BASE=ROOT/'games/stoneage'
ACTIONS=(0,1,2,3,4,10)
def expand(source):
 source=source.resolve();assets=source/'client/web/assets/original'
 sprites=json.loads((assets/'sprites.json').read_text())['sprites']
 manifest=json.loads((assets/'manifest.json').read_text())
 legacy=json.loads((BASE/'web/art.json').read_text())
 catalog=json.loads((BASE/'server/catalog_source.json').read_text())
 def complete(graphic):
  available={(a['direction'],a['action']) for a in sprites.get(str(graphic),{}).get('actions',[]) if a['frames']}
  return all((d,a) in available for d in (3,7) for a in ACTIONS)
 native=source/'server/legacy/source/2.5/gmsv/char/char_base.c'
 table=native.read_bytes().decode('gb18030').split('ridePetTable[296] =',1)[1].split('};',1)[0]
 mappings={}
 for line in table.splitlines():
  if line.strip().startswith('//'):continue
  match=re.search(r'\{\s*(\d+),\s*(\d+),\s*(\d+),\s*(\d+)',line)
  if not match:continue
  graphic,avatar,pet,_=map(int,match.groups())
  if 100000<=avatar<100240 and complete(graphic):mappings.setdefault(pet,{})[avatar]=graphic
 graphics=set()
 for ride in catalog['rides']:
  ride['riders']=[{'avatar':avatar,'graphic':graphic} for avatar,graphic in sorted(mappings[ride['graphic']].items())]
  assert 8<=len(ride['riders'])<=12,ride['name']
  graphics.update(r['graphic'] for r in ride['riders'])
 avatars=[g for g in range(100000,100240,5) if all(complete(g+w) for w in range(5))]
 assert len(avatars)==48
 graphics.update(g+w for g in avatars for w in range(5))
 extra_sprites={};resources={};all_files=set()
 def resource(file):
  if file in legacy['resources'] or file in resources:return
  payload=(assets/file).read_bytes();assert payload[:8]==b'\x89PNG\r\n\x1a\n',file
  width,height=struct.unpack('>II',payload[16:24])
  resources[file]={'bytes':len(payload),'sha256':hashlib.sha256(payload).hexdigest(),'width':width,'height':height}
 for graphic in sorted(graphics-set(map(int,legacy['sprites']))):
  rows=[]
  for a in sprites[str(graphic)]['actions']:
   if a['direction'] not in (3,7) or a['action'] not in ACTIONS:continue
   frames=[]
   for f in a['frames']:
    file=f['file'];resource(file);payload=(assets/file).read_bytes();width,height=struct.unpack('>II',payload[16:24])
    frames.append({'file':file,'x':f['xoffset'],'y':f['yoffset'],'width':width,'height':height,'sound':f.get('sound',0)})
   rows.append({'direction':a['direction'],'action':a['action'],'ticks':a['frame_ms'],'frames':frames})
  extra_sprites[str(graphic)]=rows
 all_sprites={**legacy['sprites'],**extra_sprites}
 people=[]
 for avatar in avatars:
  family=(avatar-100000)//20;color=(avatar-100000)%20//5
  face=manifest['faces'].get(str(30000+family*100+color*25))
  preview=next(a for a in all_sprites[str(avatar)] if a['direction']==3 and a['action']==3)['frames'][0]['file']
  portrait=face['file'] if face else preview
  resource(portrait)
  people.append({'id':avatar,'portrait':portrait,'portraitKind':'face' if face else 'sprite','preview':preview,'family':family,'color':color})
 extras={'version':1,'basePath':'assets','avatars':people,'sprites':extra_sprites,'resources':resources,'source':{'spritesSHA256':hashlib.sha256((assets/'sprites.json').read_bytes()).hexdigest(),'rideTableSHA256':hashlib.sha256(native.read_bytes()).hexdigest(),'faceManifestSHA256':hashlib.sha256((assets/'manifest.json').read_bytes()).hexdigest()},'assetsCopied':0,'assetsUploaded':0}
 (BASE/'web/avatar-art.json').write_text(json.dumps(extras,ensure_ascii=False,separators=(',',':'))+'\n')
 (BASE/'server/catalog_source.json').write_text(json.dumps(catalog,ensure_ascii=False,separators=(',',':'))+'\n')
 # Millisecond metadata only. Keep native 60 Hz SPR frame lengths and first
 # SoundNo contact marker; no image bytes are embedded in the Go package.
 timing={}
 for graphic,rows in sorted(all_sprites.items(),key=lambda p:int(p[0])):
  by_direction={}
  for direction in (3,7):
   values={}
   for name,action in [('attack',0),('hit',1),('dead',2)]:
    a=next(a for a in rows if a['direction']==direction and a['action']==action)
    frame_time=max(1,a['ticks'])*1000/60;frames=a['frames'];contacts=[i for i,f in enumerate(frames) if f.get('sound',0)>=10000]
    contact=contacts[0] if contacts else len(frames)//2
    values[name]={'duration':math.ceil(len(frames)*frame_time),'contactOffset':math.ceil(contact*frame_time),'ticks':a['ticks'],'frames':len(frames)}
   by_direction[str(direction)]=values
  timing[graphic]=by_direction
 points=[[453,432],[485,408],[517,384],[549,360],[581,336],[293,408],[357,360],[421,312],[485,264],[549,216],[37,198],[69,174],[101,150],[133,126],[165,102],[69,298],[133,250],[197,202],[261,154],[325,106]]
 data={'version':1,'fps':60,'contactRadius':64,'speed':8,'normalHoldTicks':8,'heavyHoldTicks':32,'postHoldTicks':20,'points':points,'place':[2,1,3,0,4,7,6,8,5,9,12,11,13,10,14,17,16,18,15,19],'graphics':timing}
 out=BASE/'motion';out.mkdir(exist_ok=True);(out/'metadata.json').write_text(json.dumps(data,separators=(',',':'))+'\n')
 report={'avatars':len(people),'mounts':len(catalog['rides']),'ridingVariants':sum(len(r['riders']) for r in catalog['rides']),'extraSpriteVariants':len(extra_sprites),'additionalOriginalImages':len(resources),'additionalOriginalBytes':sum(r['bytes'] for r in resources.values()),'legacyHDImages':len(legacy['resources']),'modelRequests':0,'assetsCopied':0,'assetsUploaded':0}
 (BASE/'avatar-assets.json').write_text(json.dumps(report,ensure_ascii=False,indent=2)+'\n')
 provenance_path=BASE/'source-manifest.json'
 provenance=json.loads(provenance_path.read_text())
 provenance['counts'].update(avatars=len(people),ridingVariants=report['ridingVariants'],sprites=len(all_sprites),bitmaps=len(legacy['resources'])+len(resources),legacyHDBitmaps=len(legacy['resources']))
 provenance_path.write_text(json.dumps(provenance,ensure_ascii=False,indent=2)+'\n')
 print(json.dumps(report))
 return report
if __name__=='__main__':
 parser=argparse.ArgumentParser();parser.add_argument('--source',type=Path,required=True);args=parser.parse_args();expand(args.source)
