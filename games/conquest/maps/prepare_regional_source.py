#!/usr/bin/env python3
"""Prepare reviewed public-domain administrative polygons for offline map builds.

Optional source-refresh tool; needs Shapely 2.1.2/GEOS 3.13.1. Application and
normal CI use the checked-in result and require no GIS dependency or network.
"""
import argparse
from collections import defaultdict
import hashlib
import json
from pathlib import Path
import shapely
from shapely.geometry import shape, mapping
from shapely.ops import unary_union

HERE=Path(__file__).resolve().parent
UPSTREAM='https://raw.githubusercontent.com/nvkelso/natural-earth-vector/117488dc884bad03366ff727eca013e434615127/geojson/ne_10m_admin_1_states_provinces.geojson'
UPSTREAM_SHA='22d0e3ad85eb3e27f17cabf8ba2d50e554fbc27a87796ff891d958185da62fb5'

def rounded(v):
    if isinstance(v,(list,tuple)):return [rounded(x) for x in v]
    return round(v,6) if isinstance(v,float) else v

def unwrap(v):
    if len(v)>=2 and isinstance(v[0],(float,int)):return [v[0]-360 if v[0]>0 else v[0],v[1]]
    return [unwrap(x) for x in v]

def main():
    p=argparse.ArgumentParser();p.add_argument('source',type=Path);args=p.parse_args()
    raw=args.source.read_bytes();assert hashlib.sha256(raw).hexdigest()==UPSTREAM_SHA,'upstream identity changed'
    assert shapely.__version__=='2.1.2' and shapely.geos_version_string=='3.13.1','review new GIS toolchain before regenerating'
    groups=defaultdict(list);names={};source_ids=defaultdict(list)
    for f in json.loads(raw)['features']:
        a=f['properties'];country=a['adm0_a3'];geom=f['geometry']
        if country not in ('USA','CHN','HKG','MAC','TWN'):continue
        if country=='USA':
            ident=a['iso_3166_2'].lower();name={'US-DC':'华盛顿特区','US-WV':'西弗吉尼亚州','US-NE':'内布拉斯加州'}.get(a['iso_3166_2'],a['name_zh'])
            if ident=='us-ak':geom={**geom,'coordinates':unwrap(geom['coordinates'])}
        elif country in ('HKG','MAC','TWN'):
            ident,name={'HKG':('cn-hk','香港'),'MAC':('cn-mo','澳门'),'TWN':('cn-tw','台湾')}[country]
        else:
            ident=a['iso_3166_2'].lower();name=a['name_zh']
            if ident=='cn-x01~':ident,name='cn-hi','海南'
            for suffix in ['维吾尔自治区','壮族自治区','回族自治区','自治区','省','市']:
                if name.endswith(suffix):name=name[:-len(suffix)];break
        names[ident]=name;groups[ident].append(shapely.make_valid(shape(geom)));source_ids[ident].append(a['adm1_code'])
    results={}
    for map_id,prefix,count in [('usa','us-',51),('china','cn-',34)]:
        ids=sorted(k for k in groups if k.startswith(prefix));assert len(ids)==count
        geoms=[shapely.set_precision(unary_union(groups[k]),1e-6) for k in ids]
        assert all(g.is_valid and not g.is_empty for g in geoms)
        # Keep exact source-border adjacency before simplifying display paths.
        borders=[]
        for i,a in enumerate(geoms):
            for j in range(i):
                b=geoms[j]
                if a.boundary.intersection(b.boundary).length>1e-5:borders.append([ids[j],ids[i]])
        if not shapely.coverage_is_valid(geoms):
            bad=shapely.coverage_invalid_edges(geoms)
            print(map_id,'coverage diagnostic',[(ids[i],round(g.length,7)) for i,g in enumerate(bad) if not g.is_empty])
        simple=shapely.coverage_simplify(geoms,.035,simplify_boundary=True)
        features=[]
        for ident,g in zip(ids,simple):
            assert g.is_valid and not g.is_empty,ident
            obj=mapping(g);features.append({'id':ident,'name':names[ident],'sourceIDs':sorted(source_ids[ident]),'geometry':{'type':obj['type'],'coordinates':rounded(obj['coordinates'])}})
        results[map_id]={'regions':features,'landBorders':borders}
        print(map_id,'regions',len(features),'borders',len(borders),'vertices',sum(shapely.get_num_coordinates(g) for g in simple))
    destination=HERE/'source/admin1-regional.json';destination.write_text(json.dumps(results,ensure_ascii=False,separators=(',',':'))+'\n')
    old=json.loads((HERE/'sources.json').read_text())
    manifest={'title':'Natural Earth 1:10m administrative regions, USA and China gameplay subsets','file':'source/admin1-regional.json','license':'Public domain','licenseUrl':old['licenseUrl'],'licenseEvidence':old['licenseEvidence'],'upstreamUrl':UPSTREAM,'upstreamSHA256':UPSTREAM_SHA,'upstreamBytes':len(raw),'sha256':hashlib.sha256(destination.read_bytes()).hexdigest(),'preparer':'prepare_regional_source.py','preparerSHA256':hashlib.sha256(Path(__file__).read_bytes()).hexdigest(),'toolchain':{'shapely':shapely.__version__,'geos':shapely.geos_version_string},'changes':['Select 50 US states and Washington DC','Group China gameplay regions at province level, merge Hong Kong and Taiwan district polygons, attach Paracel polygons to Hainan','Unwrap Alaska longitude across the dateline','Coverage-preserving display simplification at 0.035 geographic degrees; exact original shared-border graph retained'],'retrieved':'2026-10-24'}
    (HERE/'regional-sources.json').write_text(json.dumps(manifest,ensure_ascii=False,indent=2)+'\n')
if __name__=='__main__':main()
