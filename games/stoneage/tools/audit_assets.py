#!/usr/bin/env python3
"""Count unique original PNGs referenced by all supported arena content."""
import hashlib
import json
from pathlib import Path

ROOT=Path(__file__).resolve().parents[3]
BASE=ROOT/'games/stoneage'
art_path=BASE/'web/art.json'
art=json.loads(art_path.read_text())
extra_path=BASE/'web/avatar-art.json'
extra=json.loads(extra_path.read_text()) if extra_path.exists() else {'sprites':{},'resources':{},'avatars':[]}
art={'ui':art['ui'],'sprites':{**art['sprites'],**extra['sprites']},'resources':{**art['resources'],**extra['resources']}}
catalog=json.loads((BASE/'server/catalog_source.json').read_text())
def frames(graphics):
    return {f['file'] for g in graphics for a in art['sprites'].get(str(g),[]) for f in a['frames']}
pets={p['graphic'] for p in catalog['pets']+catalog['rides']}
riders={r['graphic'] for p in catalog['rides'] for r in p['riders']}
people=set(map(int,art['sprites']))-pets-riders
groups=[('宠物与骑宠本体动画',frames(pets)),('人物骑乘动画',frames(riders)),('人物与武器造型动画',frames(people)),('人物头像',{a['portrait'] for a in extra['avatars']}),('装备图标',{g['icon'] for g in catalog['equipment']}),('战斗菜单',{f for pair in art.get('ui',{}).values() for f in pair.values()}),('战斗背景',{'battle/battle_00.png'})]
seen=set();rows=[]
for name,files in groups:
    exclusive=files-seen;seen.update(files)
    rows.append(dict(category=name,uniqueImages=len(files),additionalImages=len(exclusive),bytes=sum(art['resources'][f]['bytes'] for f in exclusive)))
assert seen==set(art['resources']), 'Unclassified original image references'
total=sum(v['bytes'] for v in art['resources'].values())
report=dict(scope='当前竞技场全部候选与可用造型；动画帧按 PNG 文件去重，非每局下载量',root='https://cdn.ichenj.com/stoneage',uniqueImages=len(seen),bytes=total,MiB=round(total/1048576,2),spriteVariants=len(art['sprites']),frameReferences=sum(len(a['frames']) for s in art['sprites'].values() for a in s),categories=rows,manifestSHA256=hashlib.sha256(art_path.read_bytes()).hexdigest(),avatarManifestSHA256=hashlib.sha256(extra_path.read_bytes()).hexdigest() if extra_path.exists() else None,assetsCopied=0,assetsUploaded=0)
(BASE/'asset-audit.json').write_text(json.dumps(report,ensure_ascii=False,indent=2)+'\n')
print(json.dumps(report,ensure_ascii=False,indent=2))
