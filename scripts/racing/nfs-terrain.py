#!/usr/bin/env python3
"""Bind immutable original ground polygons to numeric rules; no visual rewrite."""
import argparse
import hashlib
import json
import math
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
PUBLIC = ROOT/'games/racing/assets/public/racing/v3'

def encoded(value):
    return (json.dumps(value,separators=(',',':'),allow_nan=False)+'\n').encode()

def sha(data):
    return hashlib.sha256(data).hexdigest()

def dm(value):
    if not isinstance(value,(int,float)) or not math.isfinite(value):
        raise ValueError('Non-finite original coordinate')
    return round(value*10,6)

def write(path,data):
    if path.is_symlink():raise ValueError('Symlink output')
    path.parent.mkdir(parents=True,exist_ok=True)
    path.write_bytes(data)

def main():
    parser=argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--out',type=Path,required=True)
    parser.add_argument('--client-out',type=Path)
    args=parser.parse_args();out=args.out.resolve()
    if not out.is_relative_to(ROOT) or out==ROOT:raise ValueError('Output must be inside this repository')
    material_bytes=(ROOT/'games/racing/assets/source/nfs4/terrain-materials.json').read_bytes()
    materials=json.loads(material_bytes)
    if materials['version']!=1 or any(len(materials[key])!=32 for key in ['surfaceFactors','collisionFriction']):raise ValueError('Invalid audited material parameters')
    cars_bytes=(PUBLIC/'cars.json').read_bytes();cars=json.loads(cars_bytes)['cars'];vehicles={}
    for ident,car in cars.items():
        if len(car['wheels'])!=4:raise ValueError('Original wheel set is incomplete')
        vehicles[ident.lower()]={'groundOffset':dm(car['groundOffset']),'wheels':[
            {'name':name,'front':name.startswith('front'),'position':[dm(x)for x in wheel['position']],'radius':dm(wheel['radius'])}
            for name,wheel in car['wheels'].items()]}
    wheel_bytes=encoded({'version':1,'unit':'decimetres','sourceSha256':sha(cars_bytes),'vehicles':vehicles})
    write(out/'terrain-materials.json',material_bytes);write(out/'terrain-wheels.json',wheel_bytes)
    specs=json.loads((ROOT/'games/racing/server/data/index.json').read_text())
    manifest={x['path']:x for x in json.loads((PUBLIC/'manifest.json').read_text())['assets']}
    index=[];summary=[]
    for spec in specs:
        ident=spec['id'];source_path=f'tracks/{ident}.collision.json';data=(PUBLIC/source_path).read_bytes();source=json.loads(data)
        route=json.loads((PUBLIC/f'tracks/{ident}.route.json').read_text())
        if source['version']!=1 or source['id']!=ident or route['geometryHash']!=spec['geometry'] or source['origin']!=route['origin'] or source['sourceTransform']!=route['sourceTransform'] or manifest[source_path]['sha256']!=sha(data):raise ValueError('Original collision/route identity differs: '+ident)
        surfaces=[];faces=0;vertices=0
        for surface in source['surfaces']:
            verts=surface['vertices']
            if not isinstance(surface['surface'],int) or not 0<=surface['surface']<=15:raise ValueError('Unknown original effect encoding')
            for face in surface['faces']:
                if len(face)<3 or any(not isinstance(i,int) or i<0 or i>=len(verts)for i in face):raise ValueError('Invalid original face')
            surfaces.append({'segment':surface['segment'],'surface':surface['surface'],'vertices':[[dm(x)for x in v]for v in verts],'faces':surface['faces']})
            faces+=len(surface['faces']);vertices+=len(verts)
        payload=encoded({'version':1,'unit':'decimetres','id':ident,'geometry':spec['geometry'],'sourceSha256':sha(data),'surfaces':surfaces})
        write(out/'terrain'/(spec['geometry']+'.json'),payload)
        index.append({'id':ident,'geometry':spec['geometry'],'sha256':sha(payload),'sourcePath':source_path,'sourceSha256':sha(data),'materialsSha256':sha(material_bytes),'wheelsSha256':sha(wheel_bytes)})
        summary.append({'id':ident,'groups':len(surfaces),'faces':faces,'vertices':vertices,'bytes':len(payload)})
    write(out/'terrain-index.json',encoded(index))
    if args.client_out:
        client=args.client_out.resolve()
        if not client.is_relative_to(ROOT) or client==ROOT:raise ValueError('Client output must be inside this repository')
        write(client/'classic-terrain-materials.json',material_bytes);write(client/'classic-terrain-wheels.json',wheel_bytes)
    print(json.dumps({'passed':True,'tracks':summary,'originalClientAssetsChanged':False}))

if __name__=='__main__':main()
