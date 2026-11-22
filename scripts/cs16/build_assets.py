import argparse
from collections import defaultdict
import gzip
import hashlib
import importlib.util
import json
import math
from pathlib import Path
import re
import struct
import subprocess
import sys
import wave
import zlib


ROOT = Path(__file__).resolve().parents[2]
MAPS = {"de_dust2": "沙漠 2", "de_inferno": "炼狱小镇", "de_aztec": "阿兹特克", "cs_assault": "仓库突击", "fy_iceworld": "冰雪世界", "cs_bloodstrike": "血战回廊"}
UNIT = 256
NORMAL_UNIT = 1048576
LUMP_SIZES = {1: 20, 3: 12, 5: 24, 6: 40, 7: 20, 9: 8, 10: 28, 11: 2, 12: 4, 13: 4, 14: 64}
SKY_SIDES = ("rt", "lf", "up", "dn", "ft", "bk")
GUN_IDS = ("p228", "scout", "xm1014", "mac10", "aug", "elite", "fiveseven", "ump45", "sg550", "galil", "famas", "usp", "glock18", "awp", "mp5navy", "m249", "m3", "m4a1", "tmp", "g3sg1", "deagle", "sg552", "ak47", "p90", "knife", "hegrenade", "flashbang", "smokegrenade", "c4")
ARMOURY_ITEMS = ("mp5navy", "tmp", "p90", "mac10", "ak47", "sg552", "m4a1", "aug", "scout", "g3sg1", "awp", "m3", "xm1014", "m249", "flashbang", "hegrenade", "vest", "vesthelm", "smokegrenade", "shield", "famas", "sg550", "galil", "ump45", "glock18", "usp", "elite", "fiveseven", "p228", "deagle")


def owned_output(path):
    path = Path(path).resolve()
    owned = (ROOT / ".local/cs16").resolve()
    if path == owned or not path.is_relative_to(owned):
        raise ValueError("CS16 output must be below .local/cs16")
    return path


def digest(raw):
    return hashlib.sha256(raw).hexdigest()


def resource(path, root, mime):
    raw = path.read_bytes()
    return {"file": path.relative_to(root).as_posix(), "bytes": len(raw), "sha256": digest(raw), "mime": mime}


def write_resource(output, relative, raw, mime, compressed=False):
    output = owned_output(output)
    path = output / relative
    if not path.resolve().is_relative_to(output) or path.is_symlink():
        raise ValueError("Resource path escapes output")
    decoded = raw
    if compressed:
        raw = gzip.compress(raw, compresslevel=9, mtime=0)
    path.parent.mkdir(parents=True, exist_ok=True)
    if not path.exists() or path.read_bytes() != raw:
        path.write_bytes(raw)
    result = resource(path, output, mime)
    if compressed:
        result.update({"encoding": "gzip", "decodedBytes": len(decoded), "decodedSHA256": digest(decoded)})
    return result


def json_bytes(value):
    return json.dumps(value, ensure_ascii=False, separators=(",", ":"), allow_nan=False).encode("utf-8")


def material_geometry_sha(rule):
    value = json.loads(json_bytes(rule))
    for brush in value.get("brushes", []):
        brush.pop("material", None)
    return digest(json.dumps(value, ensure_ascii=False, separators=(",", ":"), sort_keys=True, allow_nan=False).encode("utf-8"))


def material_rule_compatibility(previous, current, known):
    old_rules = {rule["id"]: rule for rule in previous}
    transitions = {}
    for rule in current:
        identity = digest(json_bytes(rule))
        geometry = material_geometry_sha(rule)
        old = old_rules.get(rule["id"])
        old_identity = digest(json_bytes(old)) if old else ""
        if old and old_identity != identity and material_geometry_sha(old) == geometry:
            transitions[old_identity] = {"mapId": rule["id"], "from": old_identity, "to": identity, "geometrySHA256": geometry}
        for row in known:
            if row.get("mapId") == rule["id"] and row.get("geometrySHA256") == geometry and row.get("to") in (old_identity, identity) and row.get("from") != identity:
                transitions[row["from"]] = {**row, "to": identity}
    return sorted(transitions.values(), key=lambda row: (row["mapId"], row["from"]))


def go_json_constant(path, name):
    if not path.exists():
        return []
    match = re.search(r"\bconst\s+" + re.escape(name) + r"\s*=\s*`([^`]*)`", path.read_text())
    if not match:
        raise ValueError("Generated Go JSON constant missing")
    return json.loads(match[1])


def png_bytes(width, height, rgba):
    if width < 1 or height < 1 or width > 16384 or height > 16384 or len(rgba) != width * height * 4:
        raise ValueError("Invalid PNG dimensions or pixels")
    def chunk(kind, value):
        return struct.pack(">I", len(value)) + kind + value + struct.pack(">I", zlib.crc32(kind + value) & 0xffffffff)
    rows = b"".join(b"\0" + rgba[row * width * 4:(row + 1) * width * 4] for row in range(height))
    return b"\x89PNG\r\n\x1a\n" + chunk(b"IHDR", struct.pack(">IIBBBBB", width, height, 8, 6, 0, 0, 0)) + chunk(b"IDAT", zlib.compress(rows, 9)) + chunk(b"IEND", b"")


def texture_name(raw):
    return raw.split(b"\0", 1)[0].decode("latin1")


def parse_miptex(raw):
    if len(raw) < 40:
        raise ValueError("Truncated miptex")
    name_raw, width, height, *offsets = struct.unpack_from("<16s6I", raw)
    name = texture_name(name_raw)
    if not name or width < 1 or height < 1 or width > 4096 or height > 4096:
        raise ValueError("Invalid miptex dimensions or name")
    if offsets[0] == 0:
        return {"name": name, "width": width, "height": height, "external": True}
    palette_at = offsets[3] + max(1, width // 8) * max(1, height // 8)
    if palette_at + 2 > len(raw) or offsets[0] < 40 or offsets[0] + width * height > len(raw):
        raise ValueError("Miptex data outside file")
    colors = struct.unpack_from("<H", raw, palette_at)[0]
    if colors != 256 or palette_at + 2 + colors * 3 > len(raw):
        raise ValueError("Invalid miptex palette")
    palette = raw[palette_at + 2:palette_at + 2 + 768]
    rgba = bytearray(width * height * 4)
    for index, value in enumerate(raw[offsets[0]:offsets[0] + width * height]):
        rgba[index * 4:index * 4 + 4] = palette[value * 3:value * 3 + 3] + bytes((0 if name.startswith("{") and value == 255 else 255,))
    return {"name": name, "width": width, "height": height, "rgba": bytes(rgba), "external": False}


class Wad:
    def __init__(self, raw):
        if len(raw) < 12 or raw[:4] not in (b"WAD2", b"WAD3"):
            raise ValueError("Invalid WAD header")
        count, directory = struct.unpack_from("<ii", raw, 4)
        if count < 0 or count > 100000 or directory < 12 or directory + count * 32 > len(raw):
            raise ValueError("WAD directory outside file")
        self.textures = {}
        for index in range(count):
            offset, length, size, kind, compression, name_raw = struct.unpack_from("<iiiBB2x16s", raw, directory + index * 32)
            if offset < 12 or length < 0 or offset + length > len(raw):
                raise ValueError("WAD texture outside file")
            if kind not in (67, 68):
                continue
            if compression != 0 or size != length:
                raise ValueError("Compressed WAD texture unsupported")
            self.textures[texture_name(name_raw).casefold()] = raw[offset:offset + length]


class Bsp:
    def __init__(self, raw):
        if len(raw) < 124 or struct.unpack_from("<i", raw)[0] != 30:
            raise ValueError("Expected a BSP30 map")
        self.raw = raw
        self.lumps = []
        for index in range(15):
            offset, length = struct.unpack_from("<ii", raw, 4 + index * 8)
            if offset < 124 or length < 0 or offset + length > len(raw) or index in LUMP_SIZES and length % LUMP_SIZES[index]:
                raise ValueError("Invalid BSP lump bounds or record size")
            self.lumps.append(raw[offset:offset + length])
        if not self.lumps[0] or not self.lumps[1] or not self.lumps[14]:
            raise ValueError("BSP required data missing")
        self.entities = self.parse_entities(self.lumps[0])
        self.planes = list(struct.iter_unpack("<4fi", self.lumps[1]))
        self.vertices = list(struct.iter_unpack("<3f", self.lumps[3]))
        self.nodes = list(struct.iter_unpack("<ihh6hHH", self.lumps[5]))
        self.texinfo = list(struct.iter_unpack("<8fii", self.lumps[6]))
        self.faces = list(struct.iter_unpack("<Hhihh4Bi", self.lumps[7]))
        self.clipnodes = list(struct.iter_unpack("<ihh", self.lumps[9]))
        self.leaf_contents = [struct.unpack_from("<i", self.lumps[10], offset)[0] for offset in range(0, len(self.lumps[10]), 28)]
        self.edges = list(struct.iter_unpack("<HH", self.lumps[12]))
        self.surfedges = [item[0] for item in struct.iter_unpack("<i", self.lumps[13])]
        self.models = list(struct.iter_unpack("<9f7i", self.lumps[14]))
        self.textures = self.parse_textures()

    @staticmethod
    def parse_entities(raw):
        text = raw.rstrip(b"\0").decode("latin1")
        entities = []
        for content in re.findall(r"\{([^{}]*)\}", text):
            tokens = re.findall(r'"([^"\r\n]*)"', content)
            if len(tokens) % 2:
                raise ValueError("Invalid BSP entity key/value")
            entities.append(dict(zip(tokens[::2], tokens[1::2])))
        if not entities or entities[0].get("classname") != "worldspawn":
            raise ValueError("BSP worldspawn missing")
        return entities

    def parse_textures(self):
        raw = self.lumps[2]
        if len(raw) < 4:
            raise ValueError("BSP texture table missing")
        count = struct.unpack_from("<i", raw)[0]
        if count < 1 or count > 100000 or 4 + count * 4 > len(raw):
            raise ValueError("Invalid BSP texture table")
        result = []
        for index in range(count):
            offset = struct.unpack_from("<i", raw, 4 + index * 4)[0]
            if offset == -1:
                result.append(None)
            elif offset < 4 + count * 4 or offset + 40 > len(raw):
                raise ValueError("BSP texture offset outside lump")
            else:
                result.append(parse_miptex(raw[offset:]))
        return result

    def polygon(self, face):
        first, count = face[2:4]
        if count < 3 or first < 0 or first + count > len(self.surfedges):
            raise ValueError("Face edges outside BSP")
        result = []
        for edge_number in self.surfedges[first:first + count]:
            if abs(edge_number) >= len(self.edges):
                raise ValueError("BSP surfedge out of range")
            edge = self.edges[abs(edge_number)]
            vertex = edge[0 if edge_number >= 0 else 1]
            if vertex >= len(self.vertices):
                raise ValueError("BSP vertex out of range")
            result.append(self.vertices[vertex])
        return result

    def contents(self, root, point, clip=True):
        nodes = self.clipnodes if clip else self.nodes
        for _ in range(len(nodes) + 1):
            if root < 0:
                return root if clip else self.leaf_contents[-root - 1]
            if root >= len(nodes):
                raise ValueError("BSP node child out of range")
            node = nodes[root]
            if node[0] < 0 or node[0] >= len(self.planes):
                raise ValueError("BSP plane index out of range")
            plane = self.planes[node[0]]
            side = sum(plane[index] * point[index] for index in range(3)) - plane[3]
            root = node[1 if side >= 0 else 2]
        raise ValueError("BSP cyclic node graph")


def node_graph(bsp, root, clip):
    source = bsp.clipnodes if clip else bsp.nodes
    if root < 0:
        return {"root": root, "nodes": []}
    mapping = {}
    order = []
    pending = [root]
    while pending:
        node_id = pending.pop()
        if node_id < 0 or node_id in mapping:
            continue
        if node_id >= len(source):
            raise ValueError("BSP model root outside node graph")
        mapping[node_id] = len(order)
        order.append(node_id)
        pending.extend((source[node_id][2], source[node_id][1]))
    def child(value):
        if value >= 0:
            return mapping[value]
        return value if clip else bsp.leaf_contents[-value - 1]
    return {"root": mapping[root], "nodes": [{"plane": source[index][0], "front": child(source[index][1]), "back": child(source[index][2])} for index in order]}


def vector(text):
    values = [float(value) for value in text.split()]
    if len(values) != 3 or not all(math.isfinite(value) for value in values):
        raise ValueError("Invalid BSP entity vector")
    return values


def point_object(values):
    return {axis: round(value * UNIT) for axis, value in zip(("x", "y", "z"), values)}


def entity_bounds(bsp, entity):
    match = re.fullmatch(r"\*(\d+)", entity.get("model", ""))
    if not match or int(match[1]) >= len(bsp.models):
        raise ValueError("Entity brush model missing or out of range")
    model = bsp.models[int(match[1])]
    origin = vector(entity.get("origin", "0 0 0"))
    return {"min": point_object([model[index] + origin[index] for index in range(3)]), "max": point_object([model[index + 3] + origin[index] for index in range(3)])}


def spawn(entity):
    values = point_object(vector(entity.get("origin", "0 0 0")))
    angle = float(entity.get("angle", entity.get("angles", "0 0 0").split()[1]))
    values["yaw"] = round(angle * 65536 / 360) % 65536
    return values


def decode_navigation(path, bsp):
    raw = path.read_bytes()
    if len(raw) < 80 or raw[:8] != b"PODWAY!\0":
        raise ValueError("Invalid PODBot waypoint header")
    version, count = struct.unpack_from("<ii", raw, 8)
    if version != 7 or count < 1 or count > 4096 or len(raw) != 80 + count * 204 or texture_name(raw[16:48]) != path.stem:
        raise ValueError("PODBot waypoint version, map, or size mismatch")
    nav = []
    flags = []
    edges = []
    connections = []
    validations = []
    stand_root = bsp.models[0][10]
    crouch_root = bsp.models[0][12]
    for index in range(count):
        at = 80 + index * 204
        number, node_flags, x, y, z = struct.unpack_from("<ii3f", raw, at)
        if number != index or not all(math.isfinite(value) for value in (x, y, z)):
            raise ValueError("Invalid PODBot waypoint position")
        targets = struct.unpack_from("<8h", raw, at + 40)
        link_flags = struct.unpack_from("<8H", raw, at + 56)
        if any(target < -1 or target >= count for target in targets):
            raise ValueError("PODBot connection outside map")
        nav.append(point_object((x, y, z)))
        flags.append(node_flags)
        edges.append([target for target in targets if target >= 0])
        connections.append([value for target, value in zip(targets, link_flags) if target >= 0])
        validations.append({"node": index, "stand": bsp.contents(stand_root, (x, y, z)), "crouch": bsp.contents(crouch_root, (x, y, z))})
    valid_indices = [index for index, value in enumerate(validations) if value["crouch"] != -2]
    mapping = {source: target for target, source in enumerate(valid_indices)}
    return {"nav": [nav[index] for index in valid_indices], "navFlags": [flags[index] | (4 if validations[index]["stand"] == -2 else 0) for index in valid_indices], "navEdges": [[mapping[target] for target in edges[index] if target in mapping] for index in valid_indices], "navConnections": [[flag for target, flag in zip(edges[index], connections[index]) if target in mapping] for index in valid_indices], "navigationSourceSHA256": digest(raw), "navigationSourceIndices": valid_indices, "navigationValidation": validations}


class NavReader:
    def __init__(self, raw):
        self.raw = raw
        self.offset = 0

    def take(self, format_string):
        size = struct.calcsize(format_string)
        if self.offset + size > len(self.raw):
            raise ValueError("Truncated GoldSrc NAV record")
        values = struct.unpack_from(format_string, self.raw, self.offset)
        self.offset += size
        if any(isinstance(value, float) and not math.isfinite(value) for value in values):
            raise ValueError("GoldSrc NAV contains a non-finite coordinate")
        return values

    def skip(self, size):
        if size < 0 or self.offset + size > len(self.raw):
            raise ValueError("GoldSrc NAV record exceeds file bounds")
        self.offset += size


def parse_nav5(raw, bsp_bytes):
    reader = NavReader(raw)
    magic, version, expected_bytes = reader.take("<III")
    if magic != 0xfeedface or version != 5 or expected_bytes != bsp_bytes:
        raise ValueError("GoldSrc NAV identity, version or BSP size mismatch")
    places = reader.take("<H")[0]
    if places > 8192:
        raise ValueError("GoldSrc NAV place count exceeds budget")
    for _ in range(places):
        size = reader.take("<H")[0]
        if size < 1 or size > 256:
            raise ValueError("GoldSrc NAV place name is invalid")
        reader.skip(size)
    count = reader.take("<I")[0]
    if count < 1 or count > 20000:
        raise ValueError("GoldSrc NAV area count exceeds budget")
    areas = []
    identifiers = set()
    for _ in range(count):
        area_id, flags, *extent = reader.take("<IB6f")
        ne_z, sw_z = reader.take("<ff")
        if area_id == 0 or area_id in identifiers or extent[0] > extent[3] or extent[1] > extent[4]:
            raise ValueError("Invalid GoldSrc NAV area identity or extent")
        identifiers.add(area_id)
        connections = []
        for _ in range(4):
            connected = reader.take("<I")[0]
            if connected > count:
                raise ValueError("GoldSrc NAV connection count exceeds area count")
            connections.append(list(reader.take("<" + "I" * connected)))
        hiding = reader.take("<B")[0]
        reader.skip(hiding * 17)
        approaches = reader.take("<B")[0]
        if approaches > 16:
            raise ValueError("GoldSrc NAV approach count exceeds source limit")
        reader.skip(approaches * 14)
        encounters = reader.take("<I")[0]
        if encounters > 100000:
            raise ValueError("GoldSrc NAV encounter count exceeds budget")
        for _ in range(encounters):
            reader.take("<IBIB")
            spots = reader.take("<B")[0]
            reader.skip(spots * 5)
        place = reader.take("<H")[0]
        if place > places:
            raise ValueError("GoldSrc NAV place reference outside directory")
        areas.append({"id": area_id, "flags": flags, "lo": extent[:3], "hi": extent[3:], "neZ": ne_z, "swZ": sw_z, "connections": connections})
    if reader.offset != len(raw):
        raise ValueError("Unconsumed bytes after GoldSrc NAV areas")
    if any(target != 0 and target not in identifiers for area in areas for targets in area["connections"] for target in targets):
        raise ValueError("GoldSrc NAV connection references an unknown area")
    return areas


def nav_height(area, x, y):
    width = area["hi"][0] - area["lo"][0]
    height = area["hi"][1] - area["lo"][1]
    fx = (x - area["lo"][0]) / width if width else 0
    fy = (y - area["lo"][1]) / height if height else 0
    fx = max(0, min(1, fx))
    fy = max(0, min(1, fy))
    top = area["lo"][2] * (1 - fx) + area["neZ"] * fx
    bottom = area["swZ"] * (1 - fx) + area["hi"][2] * fx
    return top * (1 - fy) + bottom * fy


def decode_nav5(path, bsp):
    raw = path.read_bytes()
    areas = parse_nav5(raw, len(bsp.raw))
    source_areas = {area["id"]: area for area in areas}
    nav = []
    flags = []
    edges = []
    connections = []
    validations = []
    source_indices = []
    lookup = {}
    centres = {}
    stand_root = bsp.models[0][10]
    crouch_root = bsp.models[0][12]
    def add_point(x, y, ground, source_id, source_flags):
        standing = point_object((x, y, ground + 36))
        crouching = point_object((x, y, ground + 18))
        stand = bsp.contents(stand_root, tuple(standing[axis] / UNIT for axis in ("x", "y", "z")))
        crouch = bsp.contents(crouch_root, tuple(crouching[axis] / UNIT for axis in ("x", "y", "z")))
        needs_crouch = bool(source_flags & 1) or stand == -2
        if needs_crouch and crouch == -2:
            return None
        point = crouching if needs_crouch else standing
        key = tuple(point[axis] for axis in ("x", "y", "z")) + (needs_crouch,)
        if key in lookup:
            return lookup[key]
        index = len(nav)
        nav.append(point)
        flags.append(4 if needs_crouch else 0)
        edges.append([])
        connections.append([])
        validations.append({"node": index, "sourceArea": source_id, "stand": stand, "crouch": crouch})
        source_indices.append(source_id)
        lookup[key] = index
        return index
    def connect(start, end, jump):
        if start is not None and end is not None and start != end and end not in edges[start]:
            edges[start].append(end)
            connections[start].append(1 if jump else 0)
    for area in areas:
        x = (area["lo"][0] + area["hi"][0]) / 2
        y = (area["lo"][1] + area["hi"][1]) / 2
        centres[area["id"]] = add_point(x, y, nav_height(area, x, y), area["id"], area["flags"])
    for area in areas:
        start = centres[area["id"]]
        for targets in area["connections"]:
            for target in targets:
                if target == 0:
                    continue
                other = source_areas[target]
                end = centres[target]
                lower = [max(area["lo"][axis], other["lo"][axis]) for axis in range(2)]
                upper = [min(area["hi"][axis], other["hi"][axis]) for axis in range(2)]
                jump = bool((area["flags"] | other["flags"]) & 2)
                if lower[0] <= upper[0] and lower[1] <= upper[1]:
                    x, y = [(lower[axis] + upper[axis]) / 2 for axis in range(2)]
                    ground = max(nav_height(area, x, y), nav_height(other, x, y))
                    portal = add_point(x, y, ground, area["id"], area["flags"] | other["flags"])
                    if portal is not None:
                        connect(start, portal, jump)
                        connect(portal, end, jump)
                        continue
                connect(start, end, jump)
    if len(nav) < 2 or not any(edges):
        raise ValueError("GoldSrc NAV has no traversable source-bound graph")
    return {"nav": nav, "navFlags": flags, "navEdges": edges, "navConnections": connections, "navigationFormat": "GoldSrc-NAV5", "navigationSourceSHA256": digest(raw), "navigationSourceIndices": source_indices, "navigationValidation": validations}


def texture_material(name, materials):
    if name.startswith(("+", "-")):
        name = name[2:]
    liquid = name.startswith("!")
    if name.startswith(("{", "!", "~", " ")):
        name = name[1:]
    return materials.get(name[:12].casefold(), "S" if liquid else "C")


def collision_surfaces(bsp, source, brushes):
    material_path = source / "cstrike/sound/materials.txt"
    if not material_path.exists():
        material_path = source / "valve/sound/materials.txt"
    materials = {}
    for line in material_path.read_text(encoding="latin1").splitlines():
        match = re.match(r"\s*([CMDVGTSWPYNX])\s+(\S+)", line, re.IGNORECASE)
        if match:
            materials[match[2][:12].casefold()] = match[1].upper()
    model_brushes = {brush["modelIndex"]: brush["id"] for brush in brushes}
    model_brushes[0] = -1
    surfaces = []
    for model_id, brush_id in model_brushes.items():
        model = bsp.models[model_id]
        for face_id in range(model[14], model[14] + model[15]):
            face = bsp.faces[face_id]
            info = bsp.texinfo[face[4]]
            texture = bsp.textures[info[8]]
            if texture is None:
                raise ValueError("Collision face texture missing")
            surfaces.append({"plane": face[0], "material": texture_material(texture["name"], materials), "texture": texture["name"], "vertices": [point_object(vertex) for vertex in bsp.polygon(face)], "brush": brush_id})
    return surfaces


def rules_for_map(bsp, map_id, source=None):
    source = Path(source) if source is not None else ROOT / ".local/cs16/research/original"
    world = node_graph(bsp, bsp.models[0][9], False)
    kinds = {entity.get("classname") for entity in bsp.entities}
    mode = "bomb" if kinds & {"func_bomb_target", "info_bomb_target"} else "hostage" if "hostage_entity" in kinds else "elimination"
    rule = {"id": map_id, "name": MAPS[map_id], "mode": mode, "sourceSHA256": digest(bsp.raw), "planes": [{"x": round(p[0] * NORMAL_UNIT), "y": round(p[1] * NORMAL_UNIT), "z": round(p[2] * NORMAL_UNIT), "dist": round(p[3] * UNIT)} for p in bsp.planes], "nodes": world["nodes"], "root": world["root"], "hulls": [node_graph(bsp, bsp.models[0][10], True), node_graph(bsp, bsp.models[0][12], True)], "spawnsT": [], "spawnsCT": [], "bombsites": [], "rescueZones": [], "hostages": [], "buyZones": [], "brushes": [], "ladders": []}
    world_weapons = []
    for entity_index, entity in enumerate(bsp.entities):
        kind = entity.get("classname", "")
        if kind in ("info_player_start", "info_player_deathmatch"):
            rule["spawnsCT" if kind == "info_player_start" else "spawnsT"].append(spawn(entity))
        if kind == "hostage_entity":
            rule["hostages"].append(spawn(entity))
        if kind == "armoury_entity":
            item = int(entity.get("item", "0"))
            count = max(1, int(entity.get("count", "1")))
            if item < 0 or item >= len(ARMOURY_ITEMS) or count > 1024:
                raise ValueError("Map armoury item or quantity is unsupported")
            world_weapons.append({"id": entity_index, "item": item, "weapon": ARMOURY_ITEMS[item], **spawn(entity), "count": count})
        if kind in ("func_bomb_target", "func_hostage_rescue", "func_buyzone", "func_ladder"):
            bounds = entity_bounds(bsp, entity)
            if kind == "func_buyzone":
                bounds["team"] = {"1": "t", "2": "ct"}.get(entity.get("team"), "")
            target = {"func_bomb_target": "bombsites", "func_hostage_rescue": "rescueZones", "func_buyzone": "buyZones", "func_ladder": "ladders"}[kind]
            rule[target].append(bounds)
        if kind == "info_bomb_target":
            at = point_object(vector(entity["origin"]))
            rule["bombsites"].append({"min": {axis: value - 128 * UNIT for axis, value in at.items()}, "max": {axis: value + 128 * UNIT for axis, value in at.items()}})
        if kind in ("func_wall", "func_wall_toggle", "func_breakable", "func_door", "func_door_rotating", "func_pushable", "func_rotating", "func_train", "func_button"):
            model_id = int(entity["model"][1:])
            model = bsp.models[model_id]
            angle = float(entity.get("angle", entity.get("angles", "0 0 0").split()[1]))
            direction = [0, 0, 1] if angle == -1 else [0, 0, -1] if angle == -2 else [math.cos(math.radians(angle)), math.sin(math.radians(angle)), 0]
            if kind == "func_door_rotating":
                direction = [0, 0, 0]
            travel = max(0, sum(abs(direction[index]) * (model[index + 3] - model[index]) for index in range(3)) - float(entity.get("lip", "8")))
            flags = int(entity.get("spawnflags", "0"))
            wait = float(entity.get("wait", "3"))
            rule["brushes"].append({"id": entity_index, "kind": kind, "bounds": entity_bounds(bsp, entity), "hulls": [node_graph(bsp, model[9], False), node_graph(bsp, model[10], True), node_graph(bsp, model[12], True)], "origin": point_object(vector(entity.get("origin", "0 0 0"))), "open": bool(flags & 1) if kind in ("func_door", "func_door_rotating") else False, "health": max(1, round(float(entity.get("health", "20")))), "move": point_object([value * travel for value in direction]), "speed": round(float(entity.get("speed", "100")) * UNIT), "wait": round(wait * 100), "target": entity.get("target", ""), "targetName": entity.get("targetname", ""), "flags": flags, "angle": round(float(entity.get("distance", "90")) * 65536 / 360), "modelIndex": model_id})
            if kind == "func_breakable":
                rule["brushes"][-1]["material"] = int(entity.get("material", "0"))
    if len(rule["spawnsT"]) < 5 or len(rule["spawnsCT"]) < 5 or rule["mode"] == "bomb" and not rule["bombsites"] or rule["mode"] == "hostage" and not rule["hostages"]:
        raise ValueError("Map objectives or ten-player spawns missing")
    if not rule["rescueZones"] and rule["mode"] == "hostage":
        rule["rescueLegacy"] = True
        for at in rule["spawnsCT"]:
            rule["rescueZones"].append({"min": {axis: at[axis] - 256 * UNIT for axis in ("x", "y", "z")}, "max": {axis: at[axis] + 256 * UNIT for axis in ("x", "y", "z")}})
    nav_path = ROOT / ".local/cs16/research/navigation" / (map_id + ".pwf")
    if nav_path.is_file():
        rule.update(decode_navigation(nav_path, bsp))
    else:
        rule.update(decode_nav5(nav_path.with_suffix(".nav"), bsp))
    rule["surfaces"] = collision_surfaces(bsp, source, rule["brushes"])
    if world_weapons:
        rule["worldWeapons"] = world_weapons
    return rule


class LightmapAtlas:
    def __init__(self, blocks):
        area = sum((width + 2) * (height + 2) for _, width, height, _ in blocks)
        size = 256
        while size * size < area * 2:
            size *= 2
        while True:
            positions = {}
            x = 0
            y = 0
            shelf = 0
            valid = True
            for face_id, width, height, _ in sorted(blocks, key=lambda item: (-item[2], -item[1], item[0])):
                if x + width + 2 > size:
                    y += shelf
                    x = 0
                    shelf = 0
                if y + height + 2 > size:
                    valid = False
                    break
                positions[face_id] = (x + 1, y + 1, width, height)
                x += width + 2
                shelf = max(shelf, height + 2)
            if valid:
                break
            size *= 2
            if size > 8192:
                raise ValueError("Map lightmap atlas exceeds budget")
        self.size = size
        self.positions = positions
        self.rgba = bytearray(b"\xff\xff\xff\xff" * size * size)
        for face_id, width, height, colors in blocks:
            ax, ay, _, _ = positions[face_id]
            for by in range(-1, height + 1):
                for bx in range(-1, width + 1):
                    source = (max(0, min(height - 1, by)) * width + max(0, min(width - 1, bx))) * 3
                    target = ((ay + by) * size + ax + bx) * 4
                    self.rgba[target:target + 4] = colors[source:source + 3] + b"\xff"


def texture_coordinates(vertex, texinfo):
    values = (sum(vertex[index] * texinfo[index] for index in range(3)) + texinfo[3], sum(vertex[index] * texinfo[index + 4] for index in range(3)) + texinfo[7])
    return tuple(struct.unpack("<f", struct.pack("<f", value))[0] for value in values)


def render_for_map(bsp, map_id, output, wads):
    texture_resources = {}
    textures = []
    for texture in bsp.textures:
        if texture is None:
            textures.append(None)
            continue
        if texture["external"]:
            original = next((wad.textures[texture["name"].casefold()] for wad in wads if texture["name"].casefold() in wad.textures), None)
            if original is None:
                raise ValueError("Missing original WAD texture: " + texture["name"])
            texture = parse_miptex(original)
        textures.append(texture)
        if texture["name"].lower().startswith("sky"):
            continue
        raw = png_bytes(texture["width"], texture["height"], texture["rgba"])
        texture_resources[texture["name"]] = write_resource(output, "textures/" + digest(raw) + ".png", raw, "image/png")
    face_info = {}
    blocks = []
    for face_id, face in enumerate(bsp.faces):
        if face[0] >= len(bsp.planes) or face[4] < 0 or face[4] >= len(bsp.texinfo):
            raise ValueError("BSP face reference out of range")
        texinfo = bsp.texinfo[face[4]]
        if texinfo[8] < 0 or texinfo[8] >= len(textures):
            raise ValueError("BSP texinfo reference out of range")
        texture = textures[texinfo[8]]
        if texture is None or texture["name"].lower().startswith("sky"):
            continue
        vertices = bsp.polygon(face)
        uv = [texture_coordinates(vertex, texinfo) for vertex in vertices]
        mins = [math.floor(min(value[axis] for value in uv) / 16) for axis in range(2)]
        maxs = [math.ceil(max(value[axis] for value in uv) / 16) for axis in range(2)]
        width, height = [maxs[axis] - mins[axis] + 1 for axis in range(2)]
        offset = face[9]
        if texinfo[9] & 1 or offset < 0:
            width = height = 1
            colors = b"\xff\xff\xff"
        else:
            length = width * height * 3
            if width < 1 or height < 1 or width > 1024 or height > 1024 or offset + length > len(bsp.lumps[8]):
                raise ValueError("BSP lightmap outside lighting lump")
            colors = bsp.lumps[8][offset:offset + length]
        blocks.append((face_id, width, height, colors))
        face_info[face_id] = (vertices, uv, mins, texture)
    atlas = LightmapAtlas(blocks)
    lightmap = write_resource(output, "maps/" + map_id + "/lightmap.png", png_bytes(atlas.size, atlas.size, atlas.rgba), "image/png")
    face_entities = {}
    for entity_id, entity in enumerate(bsp.entities):
        match = re.fullmatch(r"\*(\d+)", entity.get("model", ""))
        if not match:
            continue
        model_id = int(match[1])
        if model_id >= len(bsp.models):
            raise ValueError("Render brush model out of range")
        model = bsp.models[model_id]
        for face_id in range(model[14], model[14] + model[15]):
            face_entities[face_id] = (entity_id, entity)
    groups = {}
    for face_id, (vertices, uv, mins, texture) in face_info.items():
        entity_id, entity = face_entities.get(face_id, (-1, {}))
        kind = entity.get("classname", "")
        if kind in ("func_bomb_target", "func_hostage_rescue", "func_buyzone", "func_ladder", "trigger_multiple", "trigger_once", "trigger_push", "trigger_hurt", "trigger_teleport"):
            continue
        mode = int(entity.get("rendermode", "0"))
        alpha = max(0, min(255, int(entity.get("renderamt", "255"))))
        if mode and alpha == 0:
            continue
        dynamic = entity_id if kind in ("func_door", "func_door_rotating", "func_breakable", "func_wall_toggle", "func_button") else -1
        key = (texture["name"], dynamic, mode, alpha)
        mesh = groups.setdefault(key, {"texture": texture["name"], "transparent": texture["name"].startswith("{") or mode in (1, 2, 3, 5), "alpha": alpha, "renderMode": mode, "brush": dynamic, "positions": [], "normals": [], "uv": [], "uv2": []})
        face = bsp.faces[face_id]
        normal = [value * (-1 if face[1] else 1) for value in bsp.planes[face[0]][:3]]
        offset = vector(entity.get("origin", "0 0 0"))
        ax, ay, width, height = atlas.positions[face_id]
        for index in range(1, len(vertices) - 1):
            first = [vertices[index][axis] - vertices[0][axis] for axis in range(3)]
            second = [vertices[index + 1][axis] - vertices[0][axis] for axis in range(3)]
            cross = (first[1] * second[2] - first[2] * second[1], first[2] * second[0] - first[0] * second[2], first[0] * second[1] - first[1] * second[0])
            facing = sum(cross[axis] * normal[axis] for axis in range(3))
            if abs(facing) < 0.00001:
                continue
            vertex_ids = (0, index, index + 1) if facing > 0 else (0, index + 1, index)
            for vertex_id in vertex_ids:
                mesh["positions"].extend(vertices[vertex_id][axis] + offset[axis] for axis in range(3))
                mesh["normals"].extend(normal)
                mesh["uv"].extend((uv[vertex_id][0] / texture["width"], uv[vertex_id][1] / texture["height"]))
                mesh["uv2"].extend(((ax + (uv[vertex_id][0] / 16 - mins[0] if width > 1 else 0) + 0.5) / atlas.size, (ay + (uv[vertex_id][1] / 16 - mins[1] if height > 1 else 0) + 0.5) / atlas.size))
    bounds = {"min": list(bsp.models[0][:3]), "max": list(bsp.models[0][3:6])}
    decorations = []
    for entity in bsp.entities:
        model = entity.get("model", "")
        if entity.get("classname") not in ("cycler", "cycler_sprite", "env_model") or not model.lower().endswith(".mdl"):
            continue
        name = Path(model).stem.replace("_", "-")
        decorations.append({"model": "prop-" + name, "position": vector(entity.get("origin", "0 0 0")), "angles": vector(entity.get("angles", "0 " + entity.get("angle", "0") + " 0")), "sequence": int(entity.get("sequence", "0")), "skin": int(entity.get("skin", "0"))})
    return {"schema": 1, "id": map_id, "skyname": bsp.entities[0].get("skyname", "des"), "bounds": bounds, "meshes": list(groups.values()), "textures": texture_resources, "lightmap": lightmap, "decorations": decorations, "sourceSHA256": digest(bsp.raw)}


def map_preview(render, map_id):
    width = 512
    height = 320
    pixels = bytearray(bytes((28, 32, 35, 255)) * width * height)
    zbuffer = [-math.inf] * (width * height)
    min_x, min_y = render["bounds"]["min"][:2]
    max_x, max_y = render["bounds"]["max"][:2]
    scale = min((width - 24) / (max_x - min_x), (height - 24) / (max_y - min_y))
    offset_x = (width - (max_x - min_x) * scale) / 2
    offset_y = (height - (max_y - min_y) * scale) / 2
    colors = {"de_dust2": (183, 160, 108), "de_inferno": (169, 139, 112), "de_aztec": (125, 145, 108), "cs_assault": (142, 151, 158), "fy_iceworld": (176, 205, 216), "cs_bloodstrike": (176, 126, 101)}
    color = colors[map_id]
    for mesh in render["meshes"]:
        positions = mesh["positions"]
        normals = mesh["normals"]
        for index in range(0, len(positions), 9):
            if normals[index + 2] < 0.6:
                continue
            triangle = [positions[index + step:index + step + 3] for step in (0, 3, 6)]
            projected = [((p[0] - min_x) * scale + offset_x, height - ((p[1] - min_y) * scale + offset_y), p[2]) for p in triangle]
            area = (projected[1][0] - projected[0][0]) * (projected[2][1] - projected[0][1]) - (projected[1][1] - projected[0][1]) * (projected[2][0] - projected[0][0])
            if abs(area) < 0.01:
                continue
            lower_x = max(0, math.floor(min(p[0] for p in projected)))
            upper_x = min(width - 1, math.ceil(max(p[0] for p in projected)))
            lower_y = max(0, math.floor(min(p[1] for p in projected)))
            upper_y = min(height - 1, math.ceil(max(p[1] for p in projected)))
            for y in range(lower_y, upper_y + 1):
                for x in range(lower_x, upper_x + 1):
                    a = ((projected[1][0] - x) * (projected[2][1] - y) - (projected[1][1] - y) * (projected[2][0] - x)) / area
                    b = ((projected[2][0] - x) * (projected[0][1] - y) - (projected[2][1] - y) * (projected[0][0] - x)) / area
                    c = 1 - a - b
                    if min(a, b, c) < 0:
                        continue
                    z = a * projected[0][2] + b * projected[1][2] + c * projected[2][2]
                    pixel = y * width + x
                    if z <= zbuffer[pixel]:
                        continue
                    zbuffer[pixel] = z
                    shade = max(0.4, min(1.2, 0.75 + z / 1600))
                    pixels[pixel * 4:pixel * 4 + 4] = bytes(tuple(min(255, round(channel * shade)) for channel in color) + (255,))
    return png_bytes(width, height, pixels)


def decode_tga(raw):
    if len(raw) < 18:
        raise ValueError("TGA header truncated")
    id_length, color_map, image_type = raw[:3]
    width, height = struct.unpack_from("<HH", raw, 12)
    depth = raw[16]
    if color_map != 0 or image_type not in (2, 10) or depth not in (24, 32) or width < 1 or height < 1 or width * height > 16777216:
        raise ValueError("Unsupported TGA format")
    stride = depth // 8
    at = 18 + id_length
    values = []
    while len(values) < width * height:
        if image_type == 10:
            if at >= len(raw):
                raise ValueError("Truncated TGA RLE")
            packet = raw[at]
            at += 1
            count = (packet & 127) + 1
        else:
            packet = 0
            count = width * height
        if packet & 128:
            if at + stride > len(raw):
                raise ValueError("Truncated TGA pixel")
            values.extend([raw[at:at + stride]] * count)
            at += stride
        else:
            if at + stride * count > len(raw):
                raise ValueError("Truncated TGA pixels")
            values.extend(raw[index:index + stride] for index in range(at, at + stride * count, stride))
            at += stride * count
        if len(values) > width * height:
            raise ValueError("TGA RLE overrun")
    rgba = bytearray(width * height * 4)
    for index, pixel in enumerate(values):
        x = index % width
        y = index // width
        if not raw[17] & 32:
            y = height - 1 - y
        if raw[17] & 16:
            x = width - 1 - x
        rgba[(y * width + x) * 4:(y * width + x) * 4 + 4] = bytes((pixel[2], pixel[1], pixel[0], pixel[3] if stride == 4 else 255))
    return width, height, bytes(rgba)


def build_map(source, output, map_id):
    if map_id not in MAPS:
        raise ValueError("Unknown CS16 map")
    source = Path(source)
    output = owned_output(output)
    output.mkdir(parents=True, exist_ok=True)
    bsp = Bsp((source / "cstrike/maps" / (map_id + ".bsp")).read_bytes())
    wads = [Wad(path.read_bytes()) for path in sorted(source.rglob("*.wad"))]
    rule = rules_for_map(bsp, map_id, source)
    render = render_for_map(bsp, map_id, output, wads)
    metadata = {"name": MAPS[map_id], "mode": rule["mode"], "render": write_resource(output, "maps/" + map_id + "/render.json", json_bytes(render), "application/json"), "rules": write_resource(output, "maps/" + map_id + "/rules.json", json_bytes(rule), "application/json"), "preview": write_resource(output, "maps/" + map_id + "/preview.png", map_preview(render, map_id), "image/png"), "sourceSHA256": digest(bsp.raw), "sky": []}
    skyname = render["skyname"].casefold()
    for side in SKY_SIDES:
        candidates = [path for path in source.rglob("*.tga") if path.stem.casefold() == skyname + side]
        if not candidates:
            raise ValueError("Original sky face missing: " + skyname + side)
        width, height, rgba = decode_tga(sorted(candidates)[0].read_bytes())
        metadata["sky"].append(write_resource(output, "maps/" + map_id + "/sky-" + side + ".png", png_bytes(width, height, rgba), "image/png"))
    metadata["resources"] = list(render["textures"].values()) + [render["lightmap"]]
    provenance = community_map_source(map_id, bsp.raw)
    if provenance is not None:
        metadata["source"] = provenance
    return rule, metadata


def community_map_source(map_id, raw):
    path = ROOT / "games/cs16/assets/community-sources.json"
    if not path.is_file():
        return None
    catalog = json.loads(path.read_text())
    relative = "original/cstrike/maps/" + map_id + ".bsp"
    entries = [entry for entry in catalog["files"] if entry["path"] == relative]
    if not entries:
        return None
    if len(entries) != 1 or entries[0]["bytes"] != len(raw) or entries[0]["sha256"] != digest(raw):
        raise ValueError("Community map differs from its pinned source identity")
    entry = entries[0]
    source = catalog["sources"][entry["source"]]
    return {"repository": source["repository"], "commit": source["commit"], "path": entry["source_path"], "bytes": len(raw), "sha256": digest(raw), "gitBlobSHA1": entry["git_blob_sha1"], "authority": catalog["authority"]}


def build_audio(source, output):
    sounds = {}
    temporary = ROOT / ".local/cs16/tmp"
    temporary.mkdir(parents=True, exist_ok=True)
    sources = {}
    for library in ("valve", "cstrike"):
        for source_path in sorted((source / library / "sound").rglob("*.wav")):
            sources[source_path.relative_to(source / library / "sound").as_posix()] = source_path
    for key, source_path in sorted(sources.items()):
        destination = output / "sounds" / Path(key).with_suffix(".flac")
        destination.parent.mkdir(parents=True, exist_ok=True)
        try:
            with wave.open(str(source_path), "rb") as audio:
                channels = audio.getnchannels()
                rate = audio.getframerate()
                bits = audio.getsampwidth() * 8
                frames = audio.getnframes()
                pcm = audio.readframes(frames)
        except (wave.Error, EOFError) as error:
            raise ValueError("Unsupported source WAV: " + key) from error
        subprocess.run(["flac", "--silent", "--force", "--best", "--no-padding", "--no-seektable", "--output-name=" + str(destination), str(source_path)], check=True, capture_output=True)
        decoded = subprocess.run(["flac", "--silent", "--decode", "--stdout", "--force-raw-format", "--endian=little", "--sign=" + ("unsigned" if bits == 8 else "signed"), str(destination)], check=True, capture_output=True).stdout
        if decoded != pcm:
            raise ValueError("FLAC source PCM mismatch: " + key)
        sounds[key] = {**resource(destination, output, "audio/flac"), "sourceSHA256": digest(source_path.read_bytes()), "pcmSHA256": digest(pcm), "channels": channels, "sampleRate": rate, "bits": bits, "frames": frames}
    if not sounds:
        raise ValueError("CS16 audio sources missing")
    return sounds


def build_assets(source, output, skip_models=False):
    source = Path(source).resolve()
    output = owned_output(output)
    output.mkdir(parents=True, exist_ok=True)
    source_record = ROOT / ".local/cs16/research/original-samples.json"
    provenance = json.loads(source_record.read_text())
    manifest = {"schema": 1, "source": {"repository": provenance["repository"], "commit": provenance["commit"], "sha256": digest(source_record.read_bytes())}, "maps": {}, "models": {}, "players": {}, "weapons": {}, "sounds": {}, "resources": []}
    rules = []
    for map_id in MAPS:
        rule, metadata = build_map(source, output, map_id)
        rules.append(rule)
        metadata["render"] = write_resource(output, "maps/" + map_id + "/render.json.gz", (output / metadata["render"]["file"]).read_bytes(), "application/json", compressed=True)
        metadata["rules"] = write_resource(output, "maps/" + map_id + "/rules.json.gz", (output / metadata["rules"]["file"]).read_bytes(), "application/json", compressed=True)
        manifest["resources"].extend(metadata.pop("resources"))
        manifest["maps"][map_id] = metadata
    if not skip_models:
        spec = importlib.util.spec_from_file_location("cs16_models", Path(__file__).with_name("models.py"))
        models = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(models)
        attach_models(manifest, models.build_models(source, output)["models"])
        models.build_model_rules(source)
    manifest["sounds"] = build_audio(source, output)
    data_directory = ROOT / "games/cs16/data"
    data_directory.mkdir(parents=True, exist_ok=True)
    payload = json_bytes(rules).decode("utf-8")
    (data_directory / "maps.go").write_text("package data\n\nconst MapsJSON = `" + payload + "`\n")
    (output / "manifest.json").write_bytes(json_bytes(manifest) + b"\n")
    metadata_directory = ROOT / "games/cs16/assets"
    metadata_directory.mkdir(parents=True, exist_ok=True)
    (metadata_directory / "source.json").write_bytes(json_bytes({"schema": 1, "source": manifest["source"], "maps": {key: {"sourceSHA256": value["sourceSHA256"], "name": value["name"], "mode": value["mode"]} for key, value in manifest["maps"].items()}}) + b"\n")
    verification = ROOT / ".local/cs16/verification"
    verification.mkdir(parents=True, exist_ok=True)
    (verification / "source-resources.json").write_bytes(json_bytes(resource_records(manifest, output)) + b"\n")
    return manifest


def attach_models(manifest, models):
    manifest["models"] = models
    manifest["players"] = {character: models["player-" + character] for character in ("urban", "gsg9", "sas", "gign", "terror", "leet", "arctic", "guerilla")}
    manifest["weapons"] = {}
    for gun in GUN_IDS:
        model_id = "mp5" if gun == "mp5navy" else gun
        manifest["weapons"][gun] = {role: models[prefix + "-" + model_id] for role, prefix in (("view", "v"), ("player", "p"), ("world", "w"))}


def refresh_models(source, output):
    source = Path(source).resolve()
    output = owned_output(output)
    manifest_path = output / "manifest.json"
    manifest = json.loads(manifest_path.read_text())
    if manifest.get("schema") != 1 or set(manifest.get("maps", {})) != set(MAPS):
        raise ValueError("An existing complete CS16 asset manifest is required")
    spec = importlib.util.spec_from_file_location("cs16_models", Path(__file__).with_name("models.py"))
    models = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(models)
    attach_models(manifest, models.build_models(source, output)["models"])
    manifest_path.write_bytes(json_bytes(manifest) + b"\n")
    verification = ROOT / ".local/cs16/verification"
    verification.mkdir(parents=True, exist_ok=True)
    (verification / "source-resources.json").write_bytes(json_bytes(resource_records(manifest, output)) + b"\n")
    return manifest


def refresh_rules(source, output):
    source = Path(source).resolve()
    output = owned_output(output)
    manifest_path = output / "manifest.json"
    manifest = json.loads(manifest_path.read_text())
    if manifest.get("schema") != 1 or set(manifest.get("maps", {})) != set(MAPS):
        raise ValueError("An existing complete CS16 asset manifest is required")
    data_directory = ROOT / "games/cs16/data"
    previous = go_json_constant(data_directory / "maps.go", "MapsJSON")
    known = go_json_constant(data_directory / "maps_compat.go", "MaterialRulesCompatibilityJSON")
    rules = []
    for map_id in MAPS:
        bsp = Bsp((source / "cstrike/maps" / (map_id + ".bsp")).read_bytes())
        rule = rules_for_map(bsp, map_id, source)
        rules.append(rule)
        raw = json_bytes(rule)
        write_resource(output, "maps/" + map_id + "/rules.json", raw, "application/json")
        manifest["maps"][map_id]["rules"] = write_resource(output, "maps/" + map_id + "/rules.json.gz", raw, "application/json", compressed=True)
    data_directory.mkdir(parents=True, exist_ok=True)
    (data_directory / "maps.go").write_text("package data\n\nconst MapsJSON = `" + json_bytes(rules).decode("utf-8") + "`\n")
    transitions = material_rule_compatibility(previous, rules, known)
    (data_directory / "maps_compat.go").write_text("package data\n\nconst MaterialRulesCompatibilityJSON = `" + json_bytes(transitions).decode("utf-8") + "`\n")
    manifest_path.write_bytes(json_bytes(manifest) + b"\n")
    verification = ROOT / ".local/cs16/verification"
    verification.mkdir(parents=True, exist_ok=True)
    (verification / "source-resources.json").write_bytes(json_bytes(resource_records(manifest, output)) + b"\n")
    return manifest


def extend_maps(source, output, selected):
    source = Path(source).resolve()
    output = owned_output(output)
    manifest_path = output / "manifest.json"
    manifest = json.loads(manifest_path.read_text())
    if manifest.get("schema") != 1 or not isinstance(manifest.get("maps"), dict) or not set(manifest["maps"]).issubset(MAPS):
        raise ValueError("An existing source-bound CS16 manifest is required")
    if not selected or len(set(selected)) != len(selected) or any(map_id not in MAPS for map_id in selected):
        raise ValueError("Choose unique registered CS16 maps to append")
    data_path = ROOT / "games/cs16/data/maps.go"
    existing = data_path.read_text().split("`", 2)
    if len(existing) != 3:
        raise ValueError("Existing source-bound map data is not a raw JSON constant")
    rules = json.loads(existing[1])
    rule_ids = {value["id"] for value in rules}
    if len(rule_ids) != len(rules) or rule_ids != set(manifest["maps"]):
        raise ValueError("Existing public and server map catalogs disagree")
    resources = {value["file"] for value in manifest["resources"]}
    for map_id in selected:
        if map_id in manifest["maps"]:
            raw = (source / "cstrike/maps" / (map_id + ".bsp")).read_bytes()
            if manifest["maps"][map_id].get("sourceSHA256") != digest(raw):
                raise ValueError("An existing map source changed; append cannot replace it")
            continue
        rule, metadata = build_map(source, output, map_id)
        metadata["render"] = write_resource(output, "maps/" + map_id + "/render.json.gz", (output / metadata["render"]["file"]).read_bytes(), "application/json", compressed=True)
        metadata["rules"] = write_resource(output, "maps/" + map_id + "/rules.json.gz", json_bytes(rule), "application/json", compressed=True)
        for descriptor in metadata.pop("resources"):
            if descriptor["file"] not in resources:
                resources.add(descriptor["file"])
                manifest["resources"].append(descriptor)
        manifest["maps"][map_id] = metadata
        rules.append(rule)
    data_path.write_text(existing[0] + "`" + json_bytes(rules).decode("utf-8") + "`" + existing[2])
    manifest_path.write_bytes(json_bytes(manifest) + b"\n")
    metadata_path = ROOT / "games/cs16/assets/source.json"
    provenance = json.loads(metadata_path.read_text())
    for map_id in selected:
        entry = manifest["maps"][map_id]
        provenance["maps"][map_id] = {"sourceSHA256": entry["sourceSHA256"], "name": entry["name"], "mode": entry["mode"], "source": entry.get("source")}
    metadata_path.write_bytes(json_bytes(provenance) + b"\n")
    verification = ROOT / ".local/cs16/verification/community-maps"
    verification.mkdir(parents=True, exist_ok=True)
    (verification / "source-resources.json").write_bytes(json_bytes(resource_records(manifest, output)) + b"\n")
    return manifest


def resource_records(manifest, output):
    output = owned_output(output)
    files = {}
    def visit(value):
        if isinstance(value, dict):
            if all(field in value for field in ("file", "bytes", "sha256", "mime")):
                relative = value["file"]
                path = output / relative
                if not path.resolve().is_relative_to(output) or path.is_symlink():
                    raise ValueError("Manifest resource escapes its output")
                raw = path.read_bytes()
                if len(raw) != value["bytes"] or digest(raw) != value["sha256"]:
                    raise ValueError("Manifest resource digest mismatch: " + relative)
                suffix = Path(relative).suffix
                files["games/cs16/assets/public/cs16/" + relative] = {"bytes": len(raw), "sha256": digest(raw), "mime": "application/gzip" if suffix == ".gz" else value["mime"], "key": "bga/cs16/" + digest(raw) + suffix}
            for child in value.values():
                visit(child)
        elif isinstance(value, list):
            for child in value:
                visit(child)
    visit(manifest)
    raw = (output / "manifest.json").read_bytes()
    files["games/cs16/assets/public/cs16/manifest.json"] = {"bytes": len(raw), "sha256": digest(raw), "mime": "application/json", "key": "bga/cs16/" + digest(raw) + ".json"}
    return {"version": 1, "files": dict(sorted(files.items()))}


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--source", type=Path, default=ROOT / ".local/cs16/research/original")
    parser.add_argument("--output", type=Path, default=ROOT / ".local/cs16/assets")
    mode = parser.add_mutually_exclusive_group()
    mode.add_argument("--skip-models", action="store_true")
    mode.add_argument("--models-only", action="store_true")
    mode.add_argument("--rules-only", action="store_true")
    mode.add_argument("--add-maps", nargs="+")
    arguments = parser.parse_args()
    if arguments.models_only:
        manifest = refresh_models(arguments.source, arguments.output)
    elif arguments.rules_only:
        manifest = refresh_rules(arguments.source, arguments.output)
    elif arguments.add_maps:
        manifest = extend_maps(arguments.source, arguments.output, arguments.add_maps)
    else:
        manifest = build_assets(arguments.source, arguments.output, arguments.skip_models)
    print(json.dumps({"passed": True, "maps": list(manifest["maps"]), "models": len(manifest["models"]), "weapons": len(manifest["weapons"]), "sounds": len(manifest["sounds"])}, ensure_ascii=False))


if __name__ == "__main__":
    main()
