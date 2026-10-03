#!/usr/bin/env python3
"""Extract board component facts from a local thoun/tickettoride reference checkout.

Reference revision: d0fad9d18f0d92152d443dcff26de221897a5047.
Only city/route/ticket facts are extracted; no PHP game logic is executed or copied.
Artwork is prepared separately and never bundled into the application image.
"""
import argparse
import ast
import json
import re
from pathlib import Path

MAPS = ('europe', 'india', 'switzerland', 'nordiccountries', 'legendaryasia')
COLORS = {'PINK': 0, 'PURPLE': 0, 'WHITE': 1, 'BLUE': 2, 'YELLOW': 3,
          'ORANGE': 4, 'BLACK': 5, 'RED': 6, 'GREEN': 7, 'GRAY': -1}


def extract(source, name):
    folder = source / 'modules/maps' / name
    city_matches = re.findall(r"(-?\d+)\s*=>\s*new City\('((?:\\.|[^'])+)',\s*(\d+),\s*(\d+)\)", (folder/'cities.php').read_text())
    ids = {int(old): i for i, (old, *_rest) in enumerate(city_matches)}
    cities = []
    for old, label, x, y in city_matches:
        old = int(old)
        c = dict(id=ids[old], name=label.replace("\\'", "'"), x=int(x), y=int(y))
        if name == 'switzerland' and (old < 0 or old >= 1000):
            c.update(kind='country' if old < 0 else 'border', country=label)
        cities.append(c)
    routes = []
    raw_routes = (folder/'routes.php').read_text()
    for m in re.finditer(r'(\d+)\s*=>\s*new Route\((\d+),\s*(\d+),\s*(\w+),\s*\[(.*?)\]\s*([^)]*)\)', raw_routes, re.S):
        rid, a, b, color, spaces, options = m.groups()
        segments = [dict(x=int(x),y=int(y),angle=int(angle)) for x,y,angle in re.findall(r'new RouteSpace\((-?\d+),\s*(-?\d+),\s*(-?\d+)', spaces)]
        r = dict(id=int(rid), a=ids[int(a)], b=ids[int(b)], length=len(segments), color=COLORS[color], segments=segments)
        if re.search(r'\btrue\b', options): r['tunnel'] = True
        for field, key in [('locomotives','ferry'), ('mountain','mountain'), ('canPayWithAnySetOfCards','substitute')]:
            match = re.search(field+r':\s*(\d+)', options)
            if match: r[key] = int(match[1])
        # Nordic ferries permit three arbitrary cards in place of a locomotive.
        if name == 'nordiccountries' and r.get('ferry'): r['substitute'] = 3
        routes.append(r)
    assert len(routes) == len(re.findall(r'\d+\s*=>\s*new Route\(', raw_routes)), name
    raw_tickets = (folder/'destinations.php').read_text()
    tickets = []
    for method, is_long in [('getBaseDestinations', False), ('getBaseSmallDestinations', False), ('getBaseBigDestinations', True)]:
        section = re.search(r'function '+method+r'\(\)\s*\{(.*?)\n\}', raw_tickets, re.S)
        if not section: continue
        for art, args in re.findall(r'(\d+)\s*=>\s*new DestinationCard\(([^)]+)\)', section[1]):
            a,b,points = ast.literal_eval('['+args+']')
            t = dict(id=len(tickets)+1, a=ids[a], b=ids[b[0] if isinstance(b,list) else b], points=min(points) if isinstance(points,list) else points, art=int(art))
            if is_long: t['long'] = True
            if isinstance(b,list): t['options'] = [dict(to=ids[to], points=value) for to,value in zip(b,points)]
            tickets.append(t)
            if name == 'switzerland' and int(art) <= 4:
                tickets.append(dict(t,id=len(tickets)+1))
    labels=json.loads(Path(__file__).with_name('rail_map_labels.json').read_text()).get(name,{})
    for c in cities:
        if c.get('kind')=='country':
            c['label']=[c['x']-50,c['y']-15,100,30]
        elif c.get('kind')!='border' and c['name'] in labels:
            x,y,w,h=labels[c['name']]
            c['label']=[x,y-2,w,h+4]
    return dict(cities=cities,routes=routes,tickets=tickets)


def main():
    parser=argparse.ArgumentParser(description=__doc__)
    parser.add_argument('source',type=Path)
    parser.add_argument('--output',type=Path,default=Path('internal/game/rail_maps'))
    args=parser.parse_args()
    args.output.mkdir(parents=True,exist_ok=True)
    for name in MAPS:
        data=extract(args.source,name)
        (args.output/(name+'.json')).write_text(json.dumps(data,ensure_ascii=False,separators=(',',':'))+'\n')
        print(name, {k:len(v) for k,v in data.items()})


if __name__ == '__main__': main()
