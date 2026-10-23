"""先行地图验收；不依赖浏览器或第三方 Python 包。"""
import hashlib
import json
import math
from pathlib import Path
import re
import subprocess
import sys
import unittest

ROOT = Path(__file__).resolve().parents[3]
MAPS = ROOT / "games/conquest/maps"
ARTIFACT = ROOT / ".local/conquest-maps/validation.json"


def canonical(value):
    return json.dumps(value, ensure_ascii=False, sort_keys=True, separators=(",", ":")).encode()


def rings(path):
    result = []
    for raw in path.split("Z"):
        if raw.strip():
            nums = [float(n) for n in re.findall(r"-?\d+(?:\.\d+)?", raw)]
            result.append(list(zip(nums[::2], nums[1::2])))
    return result


def inside(point, ring):
    x, y = point
    found = False
    for a, b in zip(ring, ring[1:] + ring[:1]):
        if (a[1] > y) != (b[1] > y) and x < (b[0] - a[0]) * (y - a[1]) / (b[1] - a[1]) + a[0]:
            found = not found
    return found


class MapContract(unittest.TestCase):
    reports = []

    def validate(self, name, minimum):
        data = json.loads((MAPS / f"{name}.json").read_text())
        self.assertEqual(data["id"], name)
        self.assertGreaterEqual(data["version"], 1)
        unsigned = {k: v for k, v in data.items() if k != "sha256"}
        self.assertEqual(hashlib.sha256(canonical(unsigned)).hexdigest(), data["sha256"])
        self.assertGreaterEqual(len(data["provinces"]), minimum)
        self.assertLessEqual(len(data["provinces"]), 400)
        self.assertGreater(data["width"], 0)
        self.assertGreater(data["height"], 0)
        provinces = {p["id"]: p for p in data["provinces"]}
        self.assertEqual(len(provinces), len(data["provinces"]))
        edges = set()
        sea = set()
        for pid, p in provinces.items():
            self.assertRegex(pid, r"^[a-z0-9_-]+$")
            self.assertRegex(p["name"], r"[\u3400-\u9fff]", p["name"])
            self.assertRegex(p["path"], r"^M")
            self.assertTrue(p["path"].endswith("Z"))
            polygons = rings(p["path"])
            self.assertTrue(polygons)
            for polygon in polygons:
                self.assertGreaterEqual(len(polygon), 3)
                xs = []
                for x, y in polygon:
                    self.assertTrue(math.isfinite(x) and math.isfinite(y))
                    self.assertGreaterEqual(x, -0.1)
                    self.assertLessEqual(x, data["width"] + 0.1)
                    self.assertGreaterEqual(y, -0.1)
                    self.assertLessEqual(y, data["height"] + 0.1)
                    xs.append(x)
                self.assertLess(max(xs) - min(xs), data["width"] * 0.65, pid)
            point = (p["label"]["x"], p["label"]["y"])
            self.assertEqual(sum(inside(point, polygon) for polygon in polygons) % 2, 1, (pid, point))
            self.assertEqual(len(set(p["neighbors"])), len(p["neighbors"]))
            self.assertEqual(len(set(p["seaNeighbors"])), len(p["seaNeighbors"]))
            self.assertFalse(set(p["neighbors"]) & set(p["seaNeighbors"]), pid)
            self.assertTrue(p["neighbors"] or p["seaNeighbors"], pid)
            for field, collection in [("neighbors", edges), ("seaNeighbors", sea)]:
                for target in p[field]:
                    self.assertIn(target, provinces, (pid, target))
                    self.assertNotEqual(pid, target)
                    self.assertIn(pid, provinces[target][field], (pid, target, field))
                    collection.add(tuple(sorted((pid, target))))
        route_edges = {tuple(sorted((r["from"], r["to"]))) for r in data["seaRoutes"]}
        self.assertEqual(sea, route_edges)
        self.assertEqual(len(route_edges), len(data["seaRoutes"]))
        self.assertGreater(len(sea), 0)
        land_polygons = [(pid, rings(p["path"])) for pid, p in provinces.items()]
        for route in data["seaRoutes"]:
            for raw in route["path"].split("M"):
                if not raw:
                    continue
                nums = [float(n) for n in re.findall(r"-?\d+(?:\.\d+)?", raw)]
                points = list(zip(nums[::2], nums[1::2]))
                for a, b in zip(points, points[1:]):
                    steps = max(2, math.ceil(math.dist(a, b) / 4))
                    for step in range(1, steps):
                        point = (a[0] + (b[0] - a[0]) * step / steps, a[1] + (b[1] - a[1]) * step / steps)
                        for pid, polygons in land_polygons:
                            self.assertEqual(sum(inside(point, polygon) for polygon in polygons) % 2, 0, ("sea route crosses land", route["from"], route["to"], pid, point))
        # Independent known borders catch a connected but geographically wrong
        # graph. They are not inferred again by the generator implementation.
        known_land = [("fra-east", "deu-west"), ("esp-west", "prt"), ("ukr", "pol"), ("deu-east", "pol")]
        known_sea = [("gbr-south", "fra-west"), ("esp-west", "mar")]
        if name == "world":
            known_land += [("usa-alaska", "can-west"), ("chn-north", "mng"), ("zaf", "lso"), ("guf", "bra-north")]
            known_sea += [("usa-alaska", "rus-fareast"), ("aus-east", "nzl"), ("moz", "mdg")]
        for a, b in known_land:
            self.assertIn(tuple(sorted((a, b))), edges)
        for a, b in known_sea:
            self.assertIn(tuple(sorted((a, b))), sea)
        for a, b in [("gbr-south", "fra-west"), ("esp-west", "mar"), ("jpn", "kor")]:
            self.assertNotIn(tuple(sorted((a, b))), edges)
        visited = set()
        todo = [next(iter(provinces))]
        while todo:
            here = todo.pop()
            if here in visited:
                continue
            visited.add(here)
            todo.extend(provinces[here]["neighbors"] + provinces[here]["seaNeighbors"])
        self.assertEqual(visited, set(provinces))
        for count in range(2, 9):
            starts = data["starts"][str(count)]
            self.assertEqual(len(starts), count)
            self.assertEqual(len(set(starts)), count)
            self.assertTrue(set(starts) <= set(provinces))
            for a in starts:
                for b in starts:
                    if a != b:
                        self.assertNotIn(b, provinces[a]["neighbors"], (count, a, b))
        names = " ".join(p["name"] for p in provinces.values())
        if name == "world":
            for region in ["中国", "印度", "巴西", "澳大利亚", "加拿大", "埃及"]:
                self.assertIn(region, names)
        else:
            for region in ["法国", "德国", "西班牙", "英国", "意大利", "希腊"]:
                self.assertIn(region, names)
        self.reports.append({"id": name, "sha256": data["sha256"], "provinces": len(provinces), "landEdges": len(edges), "seaEdges": len(sea), "connected": True, "startCounts": list(range(2, 9))})

    def test_world(self):
        self.validate("world", 170)

    def test_europe(self):
        self.validate("europe", 50)

    def test_source_digest(self):
        manifest = json.loads((MAPS / "sources.json").read_text())
        self.assertEqual(manifest["license"], "Public domain")
        source = MAPS / manifest["file"]
        self.assertEqual(hashlib.sha256(source.read_bytes()).hexdigest(), manifest["sha256"])
        self.assertIn("naturalearthdata.com", manifest["licenseUrl"])
        license_evidence = manifest["licenseEvidence"]
        self.assertEqual(hashlib.sha256((MAPS / license_evidence["file"]).read_bytes()).hexdigest(), license_evidence["sha256"])

    def test_regenerate_byte_identical(self):
        result = subprocess.run([sys.executable, "-B", str(MAPS / "generate.py"), "--check"], cwd=ROOT, text=True, capture_output=True)
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)


if __name__ == "__main__":
    suite = unittest.defaultTestLoader.loadTestsFromTestCase(MapContract)
    result = unittest.TextTestRunner(verbosity=2).run(suite)
    ARTIFACT.parent.mkdir(parents=True, exist_ok=True)
    ARTIFACT.write_text(json.dumps({"passed": result.wasSuccessful(), "checks": result.testsRun, "maps": MapContract.reports, "failures": [str(f[0]) for f in result.failures + result.errors]}, ensure_ascii=False, indent=2) + "\n")
    sys.exit(not result.wasSuccessful())
