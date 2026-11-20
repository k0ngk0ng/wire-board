import json
import struct
from collections import deque
from pathlib import Path


ROOT = Path(__file__).resolve().parents[2]
ORIGINAL = ROOT / ".local/hotlinemiami/research/original/gog-linux-2.0.0.4"
METADATA = ORIGINAL / "metadata"
SELECTION = [("no-talk", "不许交谈", 17, "rmApartment1Upstairs"), ("full-house", "满屋", 31, "rmHouse2Downstairs"), ("push-it", "夜店", 33, "rmHouse3Downstairs")]


def read_rooms(data):
    cursor = 0

    def read(fmt):
        nonlocal cursor
        size = struct.calcsize(fmt)
        if cursor + size > len(data):
            raise ValueError("truncated native room data")
        values = struct.unpack_from(fmt, data, cursor)
        cursor += size
        return values

    def count():
        value = read("<I")[0]
        if value > 100000:
            raise ValueError("native room count exceeds bound")
        return value

    read("<" + str(count()) + "i")
    read("<" + str(count()) + "i")
    total = count()
    rooms = {}
    for _ in range(total):
        source_offset = cursor
        header = read("<117i")
        if header[1] < 1 or header[2] < 1 or header[-1] < 0 or header[-1] > 100000:
            raise ValueError("native room header invalid")
        tiles = [read("<8i") for _ in range(header[-1])]
        objects = [read("<5i") for _ in range(count())]
        backgrounds = [read("<10i") for _ in range(count())]
        rooms[header[0]] = {"id": header[0], "width": header[1], "height": header[2], "tiles": tiles, "objects": objects, "backgrounds": backgrounds, "sourceOffset": source_offset}
    if cursor != len(data):
        raise ValueError("native room data has trailing bytes")
    return rooms


def read_records(data, record_size):
    if len(data) < 8:
        raise ValueError("truncated metadata")
    lookup_count = struct.unpack_from("<I", data)[0]
    position = 4 + lookup_count * 4
    if position + 4 > len(data):
        raise ValueError("metadata lookup exceeds source")
    count = struct.unpack_from("<I", data, position)[0]
    position += 4
    if position + count * record_size != len(data):
        raise ValueError("metadata record size mismatch")
    return [data[position + index * record_size:position + (index + 1) * record_size] for index in range(count)]


def read_sprite_data():
    records = read_records((METADATA / "hotline_sprites.bin").read_bytes(), 148)
    return {struct.unpack_from("<i", record)[0]: {"fields": struct.unpack_from("<14i", record), "name": record[56:].split(b"\0")[0].decode("utf-8")} for record in records}


def read_object_data():
    records = read_records((METADATA / "hotline_objects.bin").read_bytes(), 40)
    return {struct.unpack_from("<i", record)[0]: struct.unpack_from("<10i", record) for record in records}


def walkable(x, y, walls, radius):
    return all(x + radius <= wall["x"] or x - radius >= wall["x"] + wall["w"] or y + radius <= wall["y"] or y - radius >= wall["y"] + wall["h"] for wall in walls)


def select_spawns(room, walls, doors):
    floor = {(tile[0] + tile[5] // 2, tile[1] + tile[6] // 2) for tile in room["tiles"] if tile[5] == 16 and tile[6] == 16 and tile[7] >= 1000}
    points = {(x, y) for x, y in floor if 16 <= x < room["width"] - 16 and 16 <= y < room["height"] - 16 and walkable(x, y, walls, 10)}
    groups = []
    while points:
        first = min(points)
        queue = deque([first])
        points.remove(first)
        group = []
        while queue:
            x, y = queue.popleft()
            group.append((x, y))
            for point in [(x - 16, y), (x + 16, y), (x, y - 16), (x, y + 16)]:
                if point in points:
                    points.remove(point)
                    queue.append(point)
        groups.append(group)
    if not groups:
        raise ValueError("map contains no safe original floor")
    candidates = sorted(point for point in max(groups, key=len) if walkable(point[0], point[1], doors, 12))
    if len(candidates) < 8:
        raise ValueError("map cannot support eight distinct spawn candidates")
    selected = [candidates[len(candidates) // 2]]
    for _ in range(7):
        point = max((point for point in candidates if point not in selected), key=lambda point: (min((point[0] - chosen[0]) ** 2 + (point[1] - chosen[1]) ** 2 for chosen in selected), point))
        selected.append(point)
    return [{"x": x, "y": y} for x, y in selected]


def recover_maps():
    rooms = read_rooms((METADATA / "hotline_rooms.bin").read_bytes())
    objects = read_object_data()
    sprites = read_sprite_data()
    arenas = []
    for identity, name, room_id, source_name in SELECTION:
        room = rooms[room_id]
        walls = []
        doors = []
        for instance, object_id, x, y, create in room["objects"]:
            if create != 0:
                raise ValueError("selected room requires unknown instance create transform")
            object_data = objects[object_id]
            sprite = sprites.get(object_data[1])
            if not sprite:
                continue
            path = sprite["name"]
            ancestors = []
            ancestor = object_id
            while ancestor in objects and ancestor not in ancestors:
                ancestors.append(ancestor)
                ancestor = objects[ancestor][3]
            door = "/Walls/sprDoor" in path
            wall = "/Walls/" in path and not any(word in path for word in ["Broken", "Destroyed", "Open"])
            furniture = 50 in ancestors and object_data[5] == 1
            corner = 9 in ancestors and "/Collision_Masks/sprCorner" in path
            if not (door or wall or furniture or corner):
                continue
            mask = sprites.get(object_data[4], sprite)
            fields = mask["fields"]
            bounds = {"x": x + fields[5] - fields[3], "y": y + fields[7] - fields[4], "w": fields[6] - fields[5] + 1, "h": fields[8] - fields[7] + 1}
            if bounds["w"] < 1 or bounds["h"] < 1:
                continue
            bounds.update({"sourceInstance": instance, "sourceObject": object_id, "sourceSprite": path})
            if door:
                bounds["id"] = len(doors)
                doors.append(bounds)
            else:
                bounds["kind"] = "glass" if any(word in path for word in ["GlassPanel", "Window"]) else "furniture" if furniture else "wall"
                walls.append(bounds)
        spawns = select_spawns(room, walls, doors)
        weapons = ["pistol", "uzi", "m16", "shotgun", "knife", "bat", "katana"]
        weapon_spawns = [{**point, "weapon": weapons[index % len(weapons)]} for index, point in enumerate(spawns)]
        arenas.append({"id": identity, "name": name, "width": room["width"], "height": room["height"], "walls": walls, "doors": doors, "spawns": spawns, "weaponSpawns": weapon_spawns, "sourceRoomId": room_id, "sourceRoomName": source_name, "sourceRoomOffset": room["sourceOffset"], "coordinateScale": 1})
    return arenas


def main():
    output = ROOT / "games/hotlinemiami/data/maps.json"
    output.parent.mkdir(parents=True, exist_ok=True)
    arenas = recover_maps()
    output.write_text(json.dumps(arenas, ensure_ascii=False, indent=2) + "\n")
    print(json.dumps({"maps": [{"id": arena["id"], "sourceRoomId": arena["sourceRoomId"], "walls": len(arena["walls"]), "doors": len(arena["doors"])} for arena in arenas]}, ensure_ascii=False))


if __name__ == "__main__":
    main()
