#!/usr/bin/env python3
"""Offline USA/China maps from the reviewed public-domain regional source."""
import argparse
from collections import deque
import hashlib
import json
from pathlib import Path
from generate import canonical,area,inside,interior_label
HERE=Path(__file__).resolve().parent

def rings(geometry):
    polygons=geometry['coordinates'] if geometry['type']=='MultiPolygon' else [geometry['coordinates']]
    return [[tuple(p) for p in ring[:-1]] for polygon in polygons for ring in polygon if len(ring)>=4]

def bounds(polys):
    points=[p for r in polys for p in r];return min(p[0] for p in points),min(p[1] for p in points),max(p[0] for p in points),max(p[1] for p in points)

def project(region,map_id):
    polys=rings(region['geometry']);ident=region['id']
    # Like a conventional US atlas inset, show the main Hawaiian island chain;
    # distant tiny atolls must not shrink the inhabited islands into dots.
    if ident=='us-hi':polys=[r for r in polys if max(x for x,y in r)>-161.5 and min(y for x,y in r)<23.5]
    if map_id=='usa':
        source=(-125,24,-66,50);rect=(80,35,1120,600)
        if ident=='us-ak':source=bounds(polys);rect=(75,680,350,165)
        if ident=='us-hi':source=bounds(polys);rect=(510,655,205,105)
    else:
        source=(73,15.5,136,54);rect=(30,35,1030,820)
        if ident=='cn-hk':source=bounds(polys);rect=(985,775,85,75)
        if ident=='cn-mo':source=bounds(polys);rect=(920,785,45,65)
    lo,la,hi,ha=source;x,y,w,h=rect
    return [[(round(x+(a-lo)/(hi-lo)*w,2),round(y+(ha-b)/(ha-la)*h,2)) for a,b in ring] for ring in polys]

def safe_poly(ring):
    result=[]
    for p in ring:
        if not result or p!=result[-1]:result.append(p)
    if len(result)>1 and result[0]==result[-1]:result.pop()
    return result if len(result)>=3 and area(result)>.001 else []

def starts(regions,edges,first):
    adj={p['id']:set() for p in regions}
    for a,b in edges:adj[a].add(b);adj[b].add(a)
    ds={}
    for start in adj:
        d={start:0};todo=deque([start])
        while todo:
            a=todo.popleft()
            for b in sorted(adj[a]):
                if b not in d:d[b]=d[a]+1;todo.append(b)
        assert len(d)==len(adj),('disconnected',start,sorted(set(adj)-set(d)));ds[start]=d
    candidates=[p['id'] for p in regions if len(adj[p['id']])>=2 and p['id'] not in ['us-dc','cn-hk','cn-mo']]
    def search(selected):
        if len(selected)==8:return selected
        possible=[x for x in candidates if x not in selected and all(ds[x][y]>=2 for y in selected)]
        if len(possible)<8-len(selected):return None
        possible.sort(key=lambda x:(min(ds[x][y] for y in selected),len(adj[x]),x),reverse=True)
        for p in possible:
            answer=search(selected+[p])
            if answer:return answer
        return None
    selected=search([first]);assert selected,'no eight independent starts'
    return {str(n):selected[:n] for n in range(2,9)}

def svg(ring):return 'M'+'L'.join(f'{x:g},{y:g}' for x,y in ring)+'Z'
def route(points):return 'M'+'L'.join(f'{x:g},{y:g}' for x,y in points)

def closest_water(a,b,all_rings):
    # Principal land fragments avoid visually routing the Taiwan Strait through
    # a tiny offshore island instead of showing the sea passage to the island.
    aa=max(a,key=area);bb=max(b,key=area)
    pairs=sorted(((x,y) for x in aa for y in bb),key=lambda p:(p[0][0]-p[1][0])**2+(p[0][1]-p[1][1])**2)
    for x,y in pairs[:2000]:
        if all(not any(sum(inside((x[0]+(y[0]-x[0])*i/20,x[1]+(y[1]-x[1])*i/20),r) for r in pr)%2 for pr in all_rings) for i in range(1,20)):
            return [x,y]
    raise ValueError('no clear sea route')

def build(map_id):
    manifest=json.loads((HERE/'regional-sources.json').read_text());source=HERE/manifest['file'];raw=source.read_bytes();assert hashlib.sha256(raw).hexdigest()==manifest['sha256']
    data=json.loads(raw)[map_id];items=data['regions'];land={tuple(sorted(x)) for x in data['landBorders']}
    # Natural Earth's Macau/Guangdong seam is about 0.00079 degrees wide after
    # generalisation; restore this known real land boundary explicitly, never
    # add arbitrary nearest-neighbour connections to repair a graph.
    if map_id=='china':land.add(('cn-gd','cn-mo'))
    sea={('us-ak','us-wa'),('us-ca','us-hi')} if map_id=='usa' else {('cn-gd','cn-hi'),('cn-fj','cn-tw')}
    shapes={p['id']:[q for r in project(p,map_id) if (q:=safe_poly(r))] for p in items}
    rendered=[]
    for p in items:
        polys=shapes[p['id']];assert polys,p['id'];label=interior_label(max(polys,key=area))
        assert sum(inside((label['x'],label['y']),r) for r in polys)%2==1,(p['id'],'label in a hole')
        rendered.append({'id':p['id'],'name':p['name'],'path':''.join(svg(r) for r in polys),'label':label,'neighbors':sorted(b if a==p['id'] else a for a,b in land if p['id'] in (a,b)),'seaNeighbors':sorted(b if a==p['id'] else a for a,b in sea if p['id'] in(a,b))})
    routes=[]
    for a,b in sorted(sea):
        if map_id=='usa':
            if a=='us-ak':
                ak=min((p for r in shapes[a] for p in r),key=lambda p:p[0]);wa=min((p for r in shapes[b] for p in r),key=lambda p:p[0]);points=[ak,(35,ak[1]),(35,wa[1]),wa]
            else:
                ca=min((p for r in shapes[a] for p in r),key=lambda p:p[0]);hi=max((p for r in shapes[b] for p in r),key=lambda p:p[1]);points=[ca,(35,ca[1]),(35,880),(hi[0],880),hi]
        else:points=closest_water(shapes[a],shapes[b],list(shapes.values()))
        routes.append({'from':a,'to':b,'path':route(points)})
    insets=[{'title':'阿拉斯加','x':60,'y':650,'width':390,'height':215},{'title':'夏威夷','x':490,'y':630,'width':245,'height':140}] if map_id=='usa' else [{'title':'港澳放大','x':900,'y':745,'width':185,'height':125}]
    result={'id':map_id,'version':1,'name':'美国' if map_id=='usa' else '中国','description':'按州与华盛顿特区划分，阿拉斯加、夏威夷以插图展示。' if map_id=='usa' else '按省级区域展开争夺，港澳提供放大示意。','width':1280 if map_id=='usa' else 1100,'height':900 if map_id=='usa' else 920,'provinces':rendered,'starts':starts(items,land|sea,'us-tx' if map_id=='usa' else 'cn-sc'),'seaRoutes':routes,'insets':insets}
    result['sha256']=hashlib.sha256(canonical(result)).hexdigest();return result

def main():
    p=argparse.ArgumentParser();p.add_argument('--check',action='store_true');args=p.parse_args()
    for name in ['usa','china']:
        value=build(name);text=json.dumps(value,ensure_ascii=False,indent=2)+'\n';path=HERE/(name+'.json')
        if args.check:assert path.exists() and path.read_text()==text,name+' not reproducible'
        else:path.write_text(text)
        print(name,len(value['provinces']),value['sha256'])
if __name__=='__main__':main()
