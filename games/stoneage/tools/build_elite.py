#!/usr/bin/env python3
"""Index strong native pets and highest-tier crafted gear; images stay on CDN."""
import argparse,hashlib,json,re,struct
from pathlib import Path
p=argparse.ArgumentParser();p.add_argument('--source',required=True);root=Path(p.parse_args().source).resolve()
base=Path(__file__).resolve().parents[1];assets=root/'client/web/assets/original';data=root/'server/legacy/source/2.5/gmsv/data'
sprites=json.loads((assets/'sprites.json').read_text())['sprites'];manifest=json.loads((assets/'manifest.json').read_text())
def rows(name):return [l.decode('gb18030').split(',') for l in (data/name).read_bytes().splitlines() if l.strip() and not l.lstrip().startswith(b'#')]
def complete(g):
 available={(a['direction'],a['action']) for a in sprites.get(str(g),{}).get('actions',[]) if a['frames']}
 return all((d,a) in available for d in [3,7] for a in [0,1,2,3,4,10])
best={}
for r in rows('enemybase.txt'):
 g=int(r[36]);w=[int(float(x)) for x in r[9:13]]
 if r[37].strip()!='1' or sum(w)<100 or not complete(g):continue
 if g not in best or sum(w)>sum(best[g]['weights']):best[g]=dict(id=int(r[6]),name=r[0].strip(),graphic=g,weights=w,elements=[int(x) for x in r[15:19]])
classic=json.loads((base/'server/catalog_source.json').read_text());rides=[r for r in classic['rides'] if sum(r['weights'])>=100]
art={'basePath':'assets','sprites':{},'resources':{}}
def resource(file):
 payload=(assets/file).read_bytes();art['resources'][file]={'bytes':len(payload),'sha256':hashlib.sha256(payload).hexdigest()}
for g in sorted(best):
 actions=[]
 for a in sprites[str(g)]['actions']:
  if a['direction'] not in [3,7] or a['action'] not in [0,1,2,3,4,10]:continue
  frames=[]
  for f in a['frames']:
   resource(f['file']);payload=(assets/f['file']).read_bytes();width,height=struct.unpack('>II',payload[16:24]);frames.append(dict(file=f['file'],x=f['xoffset'],y=f['yoffset'],width=width,height=height,sound=f.get('sound',0)))
  actions.append(dict(direction=a['direction'],action=a['action'],ticks=a['frame_ms'],frames=frames))
 art['sprites'][str(g)]=actions
groups={}
for r in rows('itemset.txt'):
 name=r[0].strip()
 if not name.startswith('合成') or '[' in name or '［' in name:continue
 m=re.search(r'([0-9０-９]+)',name)
 if not m:continue
 family=re.sub(r'\s+','',name[:m.start()]).replace('耳环','耳饰');tier=int(m.group(1));kind=int(r[19])
 if kind not in [0,1,2,3,4,17,18,19,6,7,8,9,10,11,13]:continue
 groups.setdefault(family,[]).append((tier,r))
gear=[]
def mid(r,i):return (int(r[i] or 0)+int(r[i+1] or 0))//2
for family,entries in sorted(groups.items()):
 top=max(t for t,r in entries)
 for variant,(tier,r) in enumerate((t,r) for t,r in entries if t==top):
  kind=int(r[19]);slot='weapon' if kind in [0,1,2,3,4,17,18,19] else 'hat' if kind==6 else 'clothes' if kind==7 else 'accessory'
  if slot!='accessory' and tier!=20:continue
  bitmap=manifest['bitmaps'].get(str(int(r[17]))) or manifest['bitmaps'][str(manifest['bitmap_aliases'][str(int(r[17]))])];resource(bitmap['file'])
  item=dict(id=int(r[16]),name=r[0].strip(),slot=slot,weaponType=kind if slot=='weapon' else -1,icon=bitmap['file'],variant=variant,tier=tier,maxTier=top,family=family,sourceDescription=r[2].strip(),attack=mid(r,37),defense=mid(r,39),speed=mid(r,41),hp=mid(r,43),mp=mid(r,45),minHits=int(r[35] or 0),maxHits=int(r[36] or 0))
  description=r[2].strip()
  if '会心' in description:item['critical']=10
  if '回避' in description:item['evade']=mid(r,51)
  if '耐性' in description:
   item['resistances']={key:100 if '+100' in description else 85 for label,key in [('毒','poison'),('石化','stone'),('睡','sleep'),('混','confusion'),('酒','drunk')] if label in description}
  match=re.search(r'(\S*精灵)\s*Lv\s*(\d+)',description)
  if match:
   name,level=match.group(1),int(match.group(2));mp=max(8,int(r[57] or 0));magic=None
   for token,status in [('毒','poison'),('石化','stone'),('混迷','confusion'),('混乱','confusion'),('酩酊','drunk'),('昏睡','sleep'),('睡眠','sleep')]:
    if token in name:magic=dict(kind='control',status=status,power=70);break
   if any(token in name for token in ['净化','淨化']):magic=dict(kind='cure',power=0)
   elif any(token in name for token in ['治愈','滋润','恩惠']):magic=dict(kind='heal',power=220+level*15,group='滋润' in name or '恩惠' in name)
   elif any(token in name for token in ['大地','水的','火的','风的']):
    element=next(value for token,value in [('大地','earth'),('水的','water'),('火的','fire'),('风的','wind')] if token in name);magic=dict(kind='damage',power=200+level*20,status=element)
   if magic:item['magic']=dict(id=200000+int(r[55] or r[16]),name=f'{name} Lv{level}',mp=mp,**magic)
  if slot=='accessory' and not any(item.get(k) for k in ['attack','defense','speed','hp','mp','critical','evade','resistances','magic']):continue
  gear.append(item)
needed={f['file'] for actions in art['sprites'].values() for action in actions for f in action['frames']}|{g['icon'] for g in gear}
art['resources']={file:meta for file,meta in art['resources'].items() if file in needed}
catalog={'pets':sorted(best.values(),key=lambda p:p['graphic']),'rides':rides,'equipment':gear,'sourceSHA256':hashlib.sha256((data/'itemset.txt').read_bytes()).hexdigest()}
(base/'server/elite_source.json').write_text(json.dumps(catalog,ensure_ascii=False,separators=(',',':'))+'\n');(base/'web/elite-art.json').write_text(json.dumps(art,ensure_ascii=False,separators=(',',':'))+'\n')
print(json.dumps({'pets':len(best),'rides':len(rides),'equipment':len(gear),'images':len(art['resources'])}))
