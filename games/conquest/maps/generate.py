#!/usr/bin/env python3
"""Build both geographic maps from the checked-in public-domain Natural Earth data.

No network, third-party Python packages or machine-specific paths are required.
"""
import argparse
from collections import defaultdict, deque
import hashlib
import json
import math
from pathlib import Path

HERE = Path(__file__).resolve().parent
SOURCE = HERE / "source/ne_110m_admin_0_countries.geojson"
SOURCE_SHA = "6866c877d39cba9c357620878839b336d569f8c662d3cfab4cb1dbe2d39c977f"

# Large countries are divided into plainly named geographic areas, not claimed
# administrative subdivisions. Each rectangle clips real land boundaries.
SPLITS = {
    "CAN": [("west", "加拿大西部", (-180, -90, -110, 90)), ("central", "加拿大中部", (-110, -90, -85, 90)), ("east", "加拿大东部", (-85, -90, 180, 90))],
    "USA": [("alaska", "美国阿拉斯加", (-180, 50, -130, 90)), ("west", "美国西部", (-130, -90, -110, 90)), ("central", "美国中部", (-110, -90, -95, 90)), ("east", "美国东部", (-95, -90, 180, 90))],
    "RUS": [("west", "俄罗斯西部", (-180, -90, 50, 90)), ("ural", "俄罗斯乌拉尔地区", (50, -90, 80, 90)), ("siberia", "俄罗斯中西伯利亚", (80, -90, 110, 90)), ("east", "俄罗斯东西伯利亚", (110, -90, 140, 90)), ("fareast", "俄罗斯远东", (140, -90, 180, 90))],
    "CHN": [("west", "中国西部", (-180, -90, 100, 90)), ("north", "中国北部", (100, 35, 180, 90)), ("south", "中国南部", (100, -90, 180, 35))],
    "AUS": [("west", "澳大利亚西部", (-180, -90, 129, 90)), ("north", "澳大利亚北部", (129, -26, 180, 90)), ("south", "澳大利亚南部", (129, -90, 141, -26)), ("east", "澳大利亚东部", (141, -90, 180, -26))],
    "BRA": [("west", "巴西西部", (-180, -90, -55, 90)), ("north", "巴西北部", (-55, -12, 180, 90)), ("south", "巴西南部", (-55, -90, 180, -12))],
    "IND": [("north", "印度北部", (-180, 23, 180, 90)), ("south", "印度南部", (-180, -90, 180, 23))],
    "ARG": [("north", "阿根廷北部", (-180, -38, 180, 90)), ("south", "阿根廷南部", (-180, -90, 180, -38))],
    "DZA": [("north", "阿尔及利亚北部", (-180, 30, 180, 90)), ("south", "阿尔及利亚南部", (-180, -90, 180, 30))],
    "SAU": [("west", "沙特阿拉伯西部", (-180, -90, 44, 90)), ("east", "沙特阿拉伯东部", (44, -90, 180, 90))],
    "COD": [("west", "刚果民主共和国西部", (-180, -90, 24, 90)), ("east", "刚果民主共和国东部", (24, -90, 180, 90))],
    "KAZ": [("west", "哈萨克斯坦西部", (-180, -90, 65, 90)), ("east", "哈萨克斯坦东部", (65, -90, 180, 90))],
    "FRA": [("west", "法国西部", (-180, -90, 2, 90)), ("east", "法国东部", (2, -90, 180, 90))],
    "DEU": [("west", "德国西部", (-180, -90, 10, 90)), ("east", "德国东部", (10, -90, 180, 90))],
    "ESP": [("west", "西班牙西部", (-180, -90, -3, 90)), ("east", "西班牙东部", (-3, -90, 180, 90))],
    "ITA": [("north", "意大利北部", (-180, 42, 180, 90)), ("south", "意大利南部", (-180, -90, 180, 42))],
    "GBR": [("north", "英国北部", (-180, 55, 180, 90)), ("south", "英国南部", (-180, -90, 180, 55))],
    "TUR": [("west", "土耳其西部", (-180, -90, 35, 90)), ("east", "土耳其东部", (35, -90, 180, 90))],
    "NOR": [("south", "挪威南部", (-180, -90, 180, 65)), ("north", "挪威北部", (-180, 65, 180, 90))],
    "SWE": [("south", "瑞典南部", (-180, -90, 180, 62)), ("north", "瑞典北部", (-180, 62, 180, 90))],
}

# Connections are deliberate sea passages, not a nearest-neighbour repair of an
# incomplete graph. Endpoints name coastal regions; examples include the English
# Channel, Strait of Gibraltar, Mozambique Channel and the North Atlantic.
SEA = [
    ("gbr-south", "fra-west", "英吉利海峡"), ("gbr-south", "bel", "北海"),
    ("gbr-north", "irl", "北海峡"), ("gbr-south", "irl", "爱尔兰海"),
    ("gbr-north", "nor-south", "北海"), ("dnk", "swe-south", "厄勒海峡"),
    ("dnk", "nor-south", "斯卡格拉克海峡"), ("fin", "est", "芬兰湾"),
    ("swe-south", "pol", "波罗的海"), ("isl", "gbr-north", "北大西洋"),
    ("isl", "nor-south", "挪威海"), ("isl", "grl", "丹麦海峡"),
    ("grl", "can-east", "戴维斯海峡"), ("esp-west", "mar", "直布罗陀海峡"),
    ("ita-south", "tun", "西西里海峡"), ("ita-south", "grc", "爱奥尼亚海"),
    ("grc", "tur-west", "爱琴海"), ("cyp", "grc", "地中海"),
    ("cyp", "lbn", "地中海东部"), ("cyn", "tur-west", "塞浦路斯海峡"),
    ("egy", "cyp", "地中海东部"), ("yem", "dji", "曼德海峡"),
    ("mdg", "moz", "莫桑比克海峡"), ("lka", "ind-south", "保克海峡"),
    ("twn", "chn-south", "台湾海峡"), ("twn", "phl", "吕宋海峡"),
    ("jpn", "kor", "朝鲜海峡"), ("jpn", "rus-fareast", "日本海"),
    ("phl", "mys", "苏禄海"), ("phl", "idn", "苏拉威西海"),
    ("mys", "idn", "马六甲海峡"), ("tls", "aus-north", "帝汶海"),
    ("idn", "aus-west", "印度洋东部"), ("png", "aus-north", "托雷斯海峡"),
    ("png", "slb", "所罗门海"), ("slb", "vut", "珊瑚海"),
    ("vut", "ncl", "珊瑚海"), ("vut", "fji", "南太平洋"),
    ("ncl", "aus-east", "珊瑚海"), ("nzl", "aus-east", "塔斯曼海"),
    ("nzl", "fji", "南太平洋"), ("usa-east", "bhs", "佛罗里达海峡"),
    ("usa-east", "cub", "佛罗里达海峡"), ("cub", "hti", "向风海峡"),
    ("cub", "jam", "加勒比海"), ("jam", "hti", "牙买加海峡"),
    ("dom", "pri", "莫纳海峡"), ("cub", "mex", "尤卡坦海峡"),
    ("tto", "ven", "帕里亚湾"), ("pri", "tto", "小安的列斯群岛航路"),
    ("flk", "arg-south", "南大西洋"), ("usa-alaska", "rus-fareast", "白令海峡"),
]


def canonical(value):
    return json.dumps(value, ensure_ascii=False, sort_keys=True, separators=(",", ":")).encode()


def area(poly):
    return abs(sum(a[0] * b[1] - b[0] * a[1] for a, b in zip(poly, poly[1:] + poly[:1]))) / 2


def clip(poly, bounds):
    """Clip a geographic ring to the specified rectangle (Sutherland-Hodgman)."""
    xmin, ymin, xmax, ymax = bounds
    for axis, bound, sign in [(0, xmin, 1), (0, xmax, -1), (1, ymin, 1), (1, ymax, -1)]:
        if not poly:
            return []
        out = []
        for a, b in zip(poly[-1:] + poly[:-1], poly):
            ina, inb = sign * (a[axis] - bound) >= -1e-9, sign * (b[axis] - bound) >= -1e-9
            if ina != inb:
                ratio = (bound - a[axis]) / (b[axis] - a[axis])
                out.append((round(a[0] + ratio * (b[0] - a[0]), 6), round(a[1] + ratio * (b[1] - a[1]), 6)))
            if inb:
                out.append(tuple(b))
        poly = []
        for p in out:
            if not poly or p != poly[-1]:
                poly.append(p)
        if poly and poly[0] == poly[-1]:
            poly.pop()
    return poly if len(poly) >= 3 and area(poly) > 1e-6 else []


def inside(point, ring):
    x, y = point
    odd = False
    for a, b in zip(ring, ring[1:] + ring[:1]):
        if (a[1] > y) != (b[1] > y) and x < (b[0] - a[0]) * (y - a[1]) / (b[1] - a[1]) + a[0]:
            odd = not odd
    return odd


def distance_to_segment(p, a, b):
    dx, dy = b[0] - a[0], b[1] - a[1]
    t = max(0, min(1, ((p[0] - a[0]) * dx + (p[1] - a[1]) * dy) / (dx * dx + dy * dy))) if dx or dy else 0
    return math.hypot(p[0] - a[0] - t * dx, p[1] - a[1] - t * dy)


def interior_label(poly):
    """Choose a point well inside the largest land fragment using a bounded grid."""
    minx, miny = min(p[0] for p in poly), min(p[1] for p in poly)
    maxx, maxy = max(p[0] for p in poly), max(p[1] for p in poly)
    best, score = None, -1
    for _ in range(3):
        step_x, step_y = (maxx - minx) / 16, (maxy - miny) / 16
        for ix in range(16):
            for iy in range(16):
                p = (minx + (ix + 0.5) * step_x, miny + (iy + 0.5) * step_y)
                if inside(p, poly):
                    d = min(distance_to_segment(p, a, b) for a, b in zip(poly, poly[1:] + poly[:1]))
                    if d > score:
                        best, score = p, d
        if best is None:
            raise ValueError("No interior point in polygon")
        minx, maxx = best[0] - step_x, best[0] + step_x
        miny, maxy = best[1] - step_y, best[1] + step_y
    return {"x": round(best[0], 2), "y": round(best[1], 2)}


def country_parts():
    raw = SOURCE.read_bytes()
    if hashlib.sha256(raw).hexdigest() != SOURCE_SHA:
        raise ValueError("Source digest changed; review and version the geography before rebuilding")
    for feature in json.loads(raw)["features"]:
        props, geometry = feature["properties"], feature["geometry"]
        code = props["ADM0_A3"]
        if code in {"ATA", "ATF"}:
            continue
        polygons = geometry["coordinates"] if geometry["type"] == "MultiPolygon" else [geometry["coordinates"]]
        # Natural Earth is 1:110m generalised geography. Interior country holes
        # remain SVG subpaths (use evenodd); enclave vertices still form
        # adjacency segments, as seen in Lesotho / South Africa.
        outers = [[tuple(round(v, 6) for v in p) for p in polygon[0][:-1]] for polygon in polygons]
        holes = [[tuple(round(v, 6) for v in p) for p in hole[:-1]] for polygon in polygons for hole in polygon[1:]]
        if code == "RUS":
            # Keep the negative-longitude Chukotka fragments with the far east,
            # rather than accidentally assigning them to European Russia.
            west = [p for p in outers if sum(x for x, y in p) / len(p) > 0]
            dateline = [p for p in outers if p not in west]
            outers = west
        else:
            dateline = []
        if code == "FRA":
            # French Guiana is a distinct province, preventing instant movement
            # between European France and South America via one province ID.
            guiana = [p for p in outers if max(x for x, y in p) < -20]
            if guiana:
                yield {"id": "guf", "name": "法属圭亚那", "rings": guiana, "holes": []}
            outers = [p for p in outers if p not in guiana]
        names = {"CHN": "中国", "TWN": "台湾", "PRK": "朝鲜", "KOR": "韩国", "MNG": "蒙古", "CYN": "塞浦路斯北部"}
        for suffix, name, bounds in SPLITS.get(code, [(None, names.get(code, props["NAME_ZH"]), (-180, -90, 180, 90))]):
            pieces = [p for poly in outers if (p := clip(poly, bounds))]
            if code == "RUS" and suffix == "fareast":
                pieces += dateline
            if not pieces:
                continue
            yield {"id": code.lower() + ("-" + suffix if suffix else ""), "name": name, "rings": pieces, "holes": [p for poly in holes if (p := clip(poly, bounds))]}


def overlapping(a, b, c, d):
    dx, dy = b[0] - a[0], b[1] - a[1]
    length = math.hypot(dx, dy)
    if length < 1e-8:
        return False
    if abs(dx * (c[1] - a[1]) - dy * (c[0] - a[0])) > length * 0.000003 or abs(dx * (d[1] - a[1]) - dy * (d[0] - a[0])) > length * 0.000003:
        return False
    tc = ((c[0] - a[0]) * dx + (c[1] - a[1]) * dy) / length
    td = ((d[0] - a[0]) * dx + (d[1] - a[1]) * dy) / length
    return min(length, max(tc, td)) - max(0, min(tc, td)) > 0.00001


def land_edges(provinces):
    cells, segments, edges = defaultdict(list), [], set()
    for p in provinces:
        for poly in p["rings"] + p["holes"]:
            for a, b in zip(poly, poly[1:] + poly[:1]):
                segment_id = len(segments)
                segments.append((p["id"], a, b))
                keys = [(x, y) for x in range(math.floor(min(a[0], b[0]) / 5), math.floor(max(a[0], b[0]) / 5) + 1) for y in range(math.floor(min(a[1], b[1]) / 5), math.floor(max(a[1], b[1]) / 5) + 1)]
                candidates = {i for key in keys for i in cells[key]}
                for i in candidates:
                    other, c, d = segments[i]
                    edge = tuple(sorted((p["id"], other)))
                    if other != p["id"] and edge not in edges and overlapping(a, b, c, d):
                        edges.add(edge)
                for key in keys:
                    cells[key].append(segment_id)
    return edges


def starts_for(provinces, all_edges):
    adj = {p["id"]: set() for p in provinces}
    for a, b in all_edges:
        adj[a].add(b)
        adj[b].add(a)
    candidates = [p["id"] for p in provinces if len(adj[p["id"]]) >= 2 and sum(area(r) for r in p["rings"]) >= 2]
    first = "deu-west" if len(provinces) < 100 else "chn-north"
    selected = [first]
    def distances(start):
        result, todo = {start: 0}, deque([start])
        while todo:
            a = todo.popleft()
            for b in adj[a]:
                if b not in result:
                    result[b] = result[a] + 1
                    todo.append(b)
        if len(result) != len(adj):
            raise ValueError(f"Unreachable regions from {start}: {sorted(set(adj) - set(result))}")
        return result
    scores = distances(first)
    while len(selected) < 8:
        options = [p for p in candidates if p not in selected and scores[p] >= 2]
        if not options:
            raise ValueError("Cannot provide eight non-adjacent starting regions")
        nxt = max(options, key=lambda p: (scores[p], len(adj[p]), p))
        selected.append(nxt)
        new = distances(nxt)
        scores = {p: min(v, new[p]) for p, v in scores.items()}
    return {str(n): selected[:n] for n in range(2, 9)}


def build(name):
    bounds = (-180, -58, 180, 84) if name == "world" else (-25, 27, 55, 72)
    width, height = (1440, 720) if name == "world" else (1120, 980)
    xmin, ymin, xmax, ymax = bounds
    provinces = []
    for p in country_parts():
        rings = [r for poly in p["rings"] if (r := clip(poly, bounds))]
        if not rings or sum(area(r) for r in rings) < 0.015:
            continue
        provinces.append({**p, "rings": rings, "holes": [r for poly in p["holes"] if (r := clip(poly, bounds))]})
    provinces.sort(key=lambda p: p["id"])
    by_id = {p["id"]: p for p in provinces}
    land = land_edges(provinces)
    sea = {tuple(sorted((a, b))) for a, b, label in SEA if a in by_id and b in by_id} - land
    def project(point):
        x, y = point
        return round((x - xmin) / (xmax - xmin) * width, 2), round((ymax - y) / (ymax - ymin) * height, 2)
    def svg(poly):
        return "M" + "L".join(f"{x:g},{y:g}" for x, y in map(project, poly)) + "Z"
    rendered = []
    for p in provinces:
        projected = [list(map(project, poly)) for poly in p["rings"]]
        rendered.append({"id": p["id"], "name": p["name"], "path": "".join(svg(poly) for poly in p["rings"] + p["holes"]), "label": interior_label(max(projected, key=area)), "neighbors": sorted(b if a == p["id"] else a for a, b in land if p["id"] in (a, b)), "seaNeighbors": sorted(b if a == p["id"] else a for a, b in sea if p["id"] in (a, b))})
    routes = []
    for a, b in sorted(sea):
        def separation(pair):
            pa, pb = pair
            dx = min(abs(pa[0] - pb[0]), 360 - abs(pa[0] - pb[0]))
            return (dx * math.cos(math.radians((pa[1] + pb[1]) / 2))) ** 2 + (pa[1] - pb[1]) ** 2
        pair = min(((pa, pb) for ra in by_id[a]["rings"] for rb in by_id[b]["rings"] for pa in ra for pb in rb), key=separation)
        xy_a, xy_b = project(pair[0]), project(pair[1])
        pa, pb = {"x": xy_a[0], "y": xy_a[1]}, {"x": xy_b[0], "y": xy_b[1]}
        if abs(pa["x"] - pb["x"]) > width * 0.6:
            # Split Bering Strait at the world wrap instead of drawing a route
            # over every continent. IDs still form one bidirectional edge.
            left, right = (pa, pb) if pa["x"] < pb["x"] else (pb, pa)
            path = f'M{left["x"]},{left["y"]}L0,{left["y"]}M{width},{right["y"]}L{right["x"]},{right["y"]}'
        else:
            path = f'M{pa["x"]},{pa["y"]}L{pb["x"]},{pb["y"]}'
        routes.append({"from": a, "to": b, "path": path})
    result = {"id": name, "version": 1, "name": "世界" if name == "world" else "欧洲", "description": "跨越大陆与海峡，争夺世界各地的领土。" if name == "world" else "从北海到地中海，经营领地并统一欧洲战场。", "width": width, "height": height, "provinces": rendered, "starts": starts_for(provinces, land | sea), "seaRoutes": routes}
    result["sha256"] = hashlib.sha256(canonical(result)).hexdigest()
    return result


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--check", action="store_true")
    args = parser.parse_args()
    for name in ["world", "europe"]:
        result = build(name)
        encoded = json.dumps(result, ensure_ascii=False, indent=2) + "\n"
        target = HERE / f"{name}.json"
        if args.check:
            if not target.exists() or target.read_text() != encoded:
                raise SystemExit(f"{target.name} is not reproducible; regenerate and review")
        else:
            target.write_text(encoded)
        print(f'{name}: {len(result["provinces"])} regions, {len(result["seaRoutes"])} sea routes, SHA256 {result["sha256"]}')


if __name__ == "__main__":
    main()
