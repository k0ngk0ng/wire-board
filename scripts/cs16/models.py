import argparse
import gzip
import hashlib
import json
import math
import re
from pathlib import Path
import struct
import zlib


ROOT = Path(__file__).resolve().parents[2]
MODEL_LIMIT = 32 * 1024 * 1024
WEAPONS = ("p228", "scout", "xm1014", "mac10", "aug", "elite", "fiveseven", "ump45", "sg550", "galil", "famas", "usp", "glock18", "awp", "mp5", "m249", "m3", "m4a1", "tmp", "g3sg1", "deagle", "sg552", "ak47", "p90", "knife", "hegrenade", "flashbang", "smokegrenade", "c4")
PLAYERS = ("terror", "leet", "arctic", "guerilla", "urban", "gsg9", "sas", "gign")
WEAPON_EXTENSIONS = ("ak47", "rifle", "carbine", "mp5", "onehanded", "dualpistols", "shotgun", "m249", "knife", "grenade", "c4")


def read_values(raw, format_string, offset):
    size = struct.calcsize(format_string)
    if offset < 0 or offset + size > len(raw):
        raise ValueError("MDL field exceeds its source byte range")
    values = struct.unpack_from(format_string, raw, offset)
    if any(isinstance(value, float) and not math.isfinite(value) for value in values):
        raise ValueError("MDL field contains non-finite values")
    return values


def read_array(raw, format_string, offset, count, limit=1000000):
    size = struct.calcsize(format_string)
    if count < 0 or count > limit or offset < 0 or offset + size * count > len(raw):
        raise ValueError("MDL table exceeds its source byte range")
    return [read_values(raw, format_string, offset + size * index) for index in range(count)]


def text_name(raw):
    return raw.split(b"\0", 1)[0].decode("latin1")


def source_bytes(path):
    path = Path(path)
    if path.is_symlink() or any(parent.is_symlink() for parent in path.parents):
        raise ValueError("MDL source cannot traverse a symlink")
    raw = path.read_bytes()
    if len(raw) > MODEL_LIMIT:
        raise ValueError("MDL source exceeds the model budget")
    return raw


def decode_channel(raw, offset, frame_count):
    if frame_count < 1 or frame_count > 10000:
        raise ValueError("MDL sequence frame count is invalid")
    result = []
    position = offset
    while len(result) < frame_count:
        valid, total = read_values(raw, "<BB", position)
        if valid < 1 or total < valid:
            raise ValueError("MDL animation RLE counts are invalid")
        values = [item[0] for item in read_array(raw, "<h", position + 2, valid, 255)]
        result.extend(values)
        result.extend([values[-1]] * (total - valid))
        position += 2 + valid * 2
    return result[:frame_count]


def quaternion(angles):
    sx, sy, sz = [math.sin(value / 2) for value in angles]
    cx, cy, cz = [math.cos(value / 2) for value in angles]
    return [sx * cy * cz - cx * sy * sz, cx * sy * cz + sx * cy * sz, cx * cy * sz - sx * sy * cz, cx * cy * cz + sx * sy * sz]


def matrix(translation, rotation):
    x, y, z, w = rotation
    return [1 - 2 * y * y - 2 * z * z, 2 * x * y - 2 * z * w, 2 * x * z + 2 * y * w, translation[0], 2 * x * y + 2 * z * w, 1 - 2 * x * x - 2 * z * z, 2 * y * z - 2 * x * w, translation[1], 2 * x * z - 2 * y * w, 2 * y * z + 2 * x * w, 1 - 2 * x * x - 2 * y * y, translation[2], 0, 0, 0, 1]


def multiply_matrix(a, b):
    return [sum(a[row * 4 + step] * b[step * 4 + column] for step in range(4)) for row in range(4) for column in range(4)]


def transform_vector(values, transform, translate):
    return [sum(transform[row * 4 + column] * values[column] for column in range(3)) + (transform[row * 4 + 3] if translate else 0) for row in range(3)]


def inverse_matrix(values):
    result = [values[column * 4 + row] if row < 3 and column < 3 else 0 for row in range(4) for column in range(4)]
    for row in range(3):
        result[row * 4 + 3] = -sum(result[row * 4 + column] * values[column * 4 + 3] for column in range(3))
    result[15] = 1
    return [result[row * 4 + column] for column in range(4) for row in range(4)]


def png_image(width, height, indices, palette, masked):
    if width < 1 or height < 1 or width > 4096 or height > 4096 or len(indices) != width * height or len(palette) != 768:
        raise ValueError("MDL texture dimensions or palette are invalid")
    def chunk(kind, data):
        return struct.pack(">I", len(data)) + kind + data + struct.pack(">I", zlib.crc32(kind + data))
    raw = b"".join(b"\0" + indices[row * width:(row + 1) * width] for row in range(height))
    result = b"\x89PNG\r\n\x1a\n" + chunk(b"IHDR", struct.pack(">IIBBBBB", width, height, 8, 3, 0, 0, 0)) + chunk(b"PLTE", palette)
    if masked:
        result += chunk(b"tRNS", bytes([255]) * 255 + bytes([0]))
    return result + chunk(b"IDAT", zlib.compress(raw, 9)) + chunk(b"IEND", b"")


class StudioModel:
    def __init__(self, path):
        self.path = Path(path)
        self.raw = source_bytes(path)
        self.dependencies = {str(self.path.resolve()): hashlib.sha256(self.raw).hexdigest()}
        self.group_cache = {}
        if len(self.raw) < 244 or self.raw[:4] != b"IDST" or read_values(self.raw, "<i", 4)[0] != 10:
            raise ValueError("Expected a complete GoldSrc MDL10 model")
        declared = read_values(self.raw, "<i", 72)[0]
        if declared != len(self.raw):
            raise ValueError("MDL declared size does not match its carrier")
        self.header = read_values(self.raw, "<26i", 140)
        self.name = text_name(self.raw[8:72])
        self.bones = []
        for index, value in enumerate(read_array(self.raw, "<32s8i12f", self.header[1], self.header[0], 256)):
            if value[1] < -1 or value[1] >= index:
                raise ValueError("MDL bone parents must precede their children")
            self.bones.append({"name": text_name(value[0]), "parent": value[1], "controllers": list(value[3:9]), "value": list(value[9:15]), "scale": list(value[15:21])})
        if not self.bones:
            raise ValueError("MDL requires a nonempty skeleton")
        self.groups = [text_name(value[1]) for value in read_array(self.raw, "<32s64s2i", self.header[9], self.header[8], 32)]
        self.sequences = self.read_sequences()
        self.hitboxes = [{"bone": value[0], "group": value[1], "min": list(value[2:5]), "max": list(value[5:8])} for value in read_array(self.raw, "<2i6f", self.header[5], self.header[4], 512)]
        self.attachments = [{"name": text_name(value[0]), "type": value[1], "bone": value[2], "origin": list(value[3:6]), "vectors": list(value[6:])} for value in read_array(self.raw, "<32s2i12f", self.header[19], self.header[18], 64)]
        self.controllers = [{"bone": value[0], "type": value[1], "start": value[2], "end": value[3], "rest": value[4], "index": value[5]} for value in read_array(self.raw, "<2i2f2i", self.header[3], self.header[2], 64)]
        self.textures = self.read_textures()
        self.skins = [list(row) for row in read_array(self.texture_raw, "<" + "h" * self.texture_header[13], self.texture_header[15], self.texture_header[14], 256)]
        self.global_bind = []
        for bone in self.bones:
            local = matrix(bone["value"][:3], quaternion(bone["value"][3:]))
            self.global_bind.append(local if bone["parent"] < 0 else multiply_matrix(self.global_bind[bone["parent"]], local))

    def read_sequences(self):
        result = []
        for index in range(self.header[6]):
            offset = self.header[7] + index * 176
            label = text_name(read_values(self.raw, "<32s", offset)[0])
            fps = read_values(self.raw, "<f", offset + 32)[0]
            flags, activity, weight, events, event_index, frames = read_values(self.raw, "<6i", offset + 36)
            motion_type, motion_bone = read_values(self.raw, "<2i", offset + 68)
            blends, animation = read_values(self.raw, "<2i", offset + 120)
            group = read_values(self.raw, "<i", offset + 156)[0]
            if fps <= 0 or fps > 1000 or frames < 1 or frames > 10000 or blends < 1 or blends > 16 or group < 0 or group >= len(self.groups):
                raise ValueError("MDL sequence metadata is invalid")
            event_data = [{"frame": item[0], "event": item[1], "type": item[2], "options": text_name(item[3])} for item in read_array(self.raw, "<3i64s", event_index, events, 1024)]
            result.append({"index": index, "name": label, "fps": fps, "flags": flags, "activity": activity, "weight": weight, "events": event_data, "frames": frames, "motionType": motion_type, "motionBone": motion_bone, "linearMovement": list(read_values(self.raw, "<3f", offset + 76)), "blends": blends, "animation": animation, "group": group})
        return result

    def read_textures(self):
        self.texture_raw = self.raw
        self.texture_header = self.header
        if not self.header[10]:
            companion = self.path.with_name(self.path.stem + "T.mdl")
            if not companion.is_file():
                companion = self.path.with_name(self.path.stem + "t.mdl")
            self.texture_raw = source_bytes(companion)
            self.dependencies[str(companion.resolve())] = hashlib.sha256(self.texture_raw).hexdigest()
            if self.texture_raw[:4] != b"IDST" or read_values(self.texture_raw, "<i", 4)[0] != 10:
                raise ValueError("MDL external texture carrier is invalid")
            self.texture_header = read_values(self.texture_raw, "<26i", 140)
        result = []
        for value in read_array(self.texture_raw, "<64s4i", self.texture_header[11], self.texture_header[10], 512):
            name, flags, width, height, offset = value
            count = width * height
            if width < 1 or height < 1 or width > 4096 or height > 4096 or offset < 0 or offset + count + 768 > len(self.texture_raw):
                raise ValueError("MDL indexed texture exceeds its carrier")
            png = png_image(width, height, self.texture_raw[offset:offset + count], self.texture_raw[offset + count:offset + count + 768], bool(flags & 64))
            result.append({"name": text_name(name), "flags": flags, "width": width, "height": height, "png": png})
        if not result:
            raise ValueError("MDL has no decoded textures")
        return result

    def sequence_bytes(self, sequence):
        if sequence["group"] == 0:
            return self.raw
        if sequence["group"] in self.group_cache:
            return self.group_cache[sequence["group"]]
        group_name = Path(self.groups[sequence["group"]].replace("\\", "/")).name
        candidate = self.path.parent / group_name
        if not group_name or not candidate.is_file():
            candidate = self.path.with_name(self.path.stem + f"{sequence['group']:02d}.mdl")
        raw = source_bytes(candidate)
        if raw[:4] != b"IDSQ" or read_values(raw, "<i", 4)[0] != 10 or read_values(raw, "<i", 72)[0] != len(raw):
            raise ValueError("MDL external sequence group carrier is invalid")
        self.dependencies[str(candidate.resolve())] = hashlib.sha256(raw).hexdigest()
        self.group_cache[sequence["group"]] = raw
        return raw


class BinaryBuilder:
    def __init__(self):
        self.raw = bytearray()
        self.views = []
        self.accessors = []
        self.view_cache = {}
        self.accessor_cache = {}

    def view(self, raw, target=None):
        signature = (raw, target)
        if signature in self.view_cache:
            return self.view_cache[signature]
        while len(self.raw) % 4:
            self.raw.append(0)
        value = {"buffer": 0, "byteOffset": len(self.raw), "byteLength": len(raw)}
        if target:
            value["target"] = target
        self.raw.extend(raw)
        index = len(self.views)
        self.views.append(value)
        self.view_cache[signature] = index
        return index

    def accessor(self, values, width, kind="f", bounds=False, target=None):
        if not values or len(values) % width:
            raise ValueError("GLB accessor has invalid scalar count")
        raw = struct.pack("<" + kind * len(values), *values)
        signature = (raw, width, kind, bounds, target)
        if signature in self.accessor_cache:
            return self.accessor_cache[signature]
        value = {"bufferView": self.view(raw, target), "componentType": {"f": 5126, "H": 5123, "I": 5125}[kind], "count": len(values) // width, "type": {1: "SCALAR", 2: "VEC2", 3: "VEC3", 4: "VEC4", 16: "MAT4"}[width]}
        if bounds:
            value["min"] = [min(values[component::width]) for component in range(width)]
            value["max"] = [max(values[component::width]) for component in range(width)]
        index = len(self.accessors)
        self.accessors.append(value)
        self.accessor_cache[signature] = index
        return index


def mesh_primitives(model, model_offset, binary):
    value = read_values(model.raw, "<64sif10i", model_offset)
    mesh_count, mesh_index, vertex_count, vertex_bones, vertex_index, normal_count, normal_bones, normal_index = value[3:11]
    vertices = read_array(model.raw, "<3f", vertex_index, vertex_count, 100000)
    normals = read_array(model.raw, "<3f", normal_index, normal_count, 100000)
    vertex_joints = [item[0] for item in read_array(model.raw, "<B", vertex_bones, vertex_count, 100000)]
    normal_joints = [item[0] for item in read_array(model.raw, "<B", normal_bones, normal_count, 100000)]
    if any(joint >= len(model.bones) for joint in vertex_joints + normal_joints):
        raise ValueError("MDL mesh refers to an invalid bone")
    primitives = []
    triangle_count = 0
    for mesh in read_array(model.raw, "<5i", mesh_index, mesh_count, 10000):
        declared_triangles, command, skin_ref = mesh[:3]
        if not model.skins or skin_ref < 0 or skin_ref >= len(model.skins[0]):
            raise ValueError("MDL mesh skin reference is invalid")
        texture_index = model.skins[0][skin_ref]
        if texture_index < 0 or texture_index >= len(model.textures):
            raise ValueError("MDL mesh texture reference is invalid")
        texture = model.textures[texture_index]
        points = []
        normal_values = []
        uvs = []
        joints = []
        weights = []
        indices = []
        unique = {}
        commands = 0
        while True:
            count = read_values(model.raw, "<h", command)[0]
            command += 2
            if count == 0:
                break
            commands += 1
            if commands > 100000 or abs(count) < 3:
                raise ValueError("MDL triangle command is invalid")
            strip = []
            for entry in read_array(model.raw, "<4h", command, abs(count), 32767):
                vertex, normal, s, t = entry
                if vertex < 0 or vertex >= len(vertices) or normal < 0 or normal >= len(normals):
                    raise ValueError("MDL triangle vertex reference is invalid")
                if entry not in unique:
                    unique[entry] = len(points) // 3
                    points.extend(transform_vector(vertices[vertex], model.global_bind[vertex_joints[vertex]], True))
                    normal_values.extend(transform_vector(normals[normal], model.global_bind[normal_joints[normal]], False))
                    uvs.extend([s / texture["width"], t / texture["height"]])
                    joints.extend([vertex_joints[vertex], 0, 0, 0])
                    weights.extend([1, 0, 0, 0])
                strip.append(unique[entry])
            command += abs(count) * 8
            for index in range(abs(count) - 2):
                if count < 0:
                    triangle = [strip[0], strip[index + 1], strip[index + 2]]
                elif index % 2:
                    triangle = [strip[index + 1], strip[index], strip[index + 2]]
                else:
                    triangle = [strip[index], strip[index + 1], strip[index + 2]]
                indices.extend([triangle[0], triangle[2], triangle[1]])
        if len(indices) // 3 != declared_triangles:
            raise ValueError("MDL triangle count differs from its command list")
        if not indices:
            continue
        triangle_count += len(indices) // 3
        attributes = {"POSITION": binary.accessor(points, 3, bounds=True, target=34962), "NORMAL": binary.accessor(normal_values, 3, target=34962), "TEXCOORD_0": binary.accessor(uvs, 2, target=34962), "JOINTS_0": binary.accessor(joints, 4, kind="H", target=34962), "WEIGHTS_0": binary.accessor(weights, 4, target=34962)}
        primitives.append({"attributes": attributes, "indices": binary.accessor(indices, 1, kind="I", target=34963), "material": texture_index, "extras": {"skinRef": skin_ref}})
    return primitives, triangle_count


def model_animations(model, binary):
    animations = []
    used_names = set()
    for sequence in model.sequences:
        raw = model.sequence_bytes(sequence)
        count = sequence["frames"]
        times = [frame / sequence["fps"] for frame in range(count)]
        time_accessor = binary.accessor(times, 1, bounds=True)
        for blend in range(sequence["blends"]):
            base_name = sequence["name"] or f"sequence-{sequence['index']}"
            name = base_name if blend == 0 else base_name + f"-blend-{blend}"
            if name in used_names:
                name += f"-sequence-{sequence['index']}"
            used_names.add(name)
            samplers = []
            channels = []
            for bone_index, bone in enumerate(model.bones):
                offset = sequence["animation"] + (blend * len(model.bones) + bone_index) * 12
                offsets = read_values(raw, "<6H", offset)
                values = []
                for component, relative in enumerate(offsets):
                    channel = decode_channel(raw, offset + relative, count) if relative else [0] * count
                    values.append([bone["value"][component] + value * bone["scale"][component] for value in channel])
                if bone_index == sequence["motionBone"]:
                    for component in range(3):
                        if sequence["motionType"] & (1 << component):
                            values[component] = [0] * count
                positions = [[values[component][frame] for component in range(3)] for frame in range(count)]
                rotations = [quaternion([values[component + 3][frame] for component in range(3)]) for frame in range(count)]
                for frame in range(1, count):
                    if sum(a * b for a, b in zip(rotations[frame - 1], rotations[frame])) < 0:
                        rotations[frame] = [-value for value in rotations[frame]]
                for path, keyframes, rest in [("translation", positions, bone["value"][:3]), ("rotation", rotations, quaternion(bone["value"][3:]))]:
                    if all(keyframe == rest for keyframe in keyframes):
                        continue
                    constant = all(keyframe == keyframes[0] for keyframe in keyframes)
                    selected = keyframes[:1] if constant else keyframes
                    input_accessor = binary.accessor([0], 1, bounds=True) if constant else time_accessor
                    output_accessor = binary.accessor([value for keyframe in selected for value in keyframe], len(rest))
                    channels.append({"sampler": len(samplers), "target": {"node": bone_index + 1, "path": path}})
                    samplers.append({"input": input_accessor, "output": output_accessor, "interpolation": "LINEAR"})
            if channels:
                animations.append({"name": name, "samplers": samplers, "channels": channels, "extras": {"sequence": sequence["index"], "blend": blend, "fps": sequence["fps"], "frames": count, "loop": bool(sequence["flags"] & 1), "activity": sequence["activity"], "events": sequence["events"]}})
    return animations


def convert_model(source, target):
    source = Path(source)
    target = Path(target)
    owned = ROOT / ".local/cs16"
    if not target.resolve().is_relative_to(owned.resolve()) or target.is_symlink() or any(parent.is_symlink() for parent in target.parents):
        raise ValueError("Model output must remain inside .local/cs16")
    model = StudioModel(source)
    binary = BinaryBuilder()
    metadata = {"format": "GoldSrc-MDL10", "sourceSHA256": hashlib.sha256(model.raw).hexdigest(), "coordinates": "GoldSrc X/Y horizontal Z up; original units", "sequences": model.sequences, "hitboxes": model.hitboxes, "attachments": model.attachments, "controllers": model.controllers, "skins": model.skins}
    nodes = [{"name": "GoldSrcModel", "children": [], "extras": metadata}]
    for bone in model.bones:
        nodes.append({"name": bone["name"], "translation": bone["value"][:3], "rotation": quaternion(bone["value"][3:]), "children": []})
    for index, bone in enumerate(model.bones):
        nodes[0 if bone["parent"] < 0 else bone["parent"] + 1]["children"].append(index + 1)
    meshes = []
    bodygroups = []
    triangles = 0
    for body in read_array(model.raw, "<64s3i", model.header[17], model.header[16], 128):
        name, count, base, offset = body
        variants = []
        if count < 1 or count > 256:
            raise ValueError("MDL bodygroup count is invalid")
        for index in range(count):
            primitives, current_triangles = mesh_primitives(model, offset + index * 112, binary)
            triangles += current_triangles
            if not primitives:
                variants.append(None)
                continue
            mesh_index = len(meshes)
            meshes.append({"name": text_name(name) + f"-{index}", "primitives": primitives})
            variants.append(len(nodes))
            nodes.append({"name": text_name(name) + f"-{index}", "mesh": mesh_index, "skin": 0})
            if index == 0:
                nodes[0]["children"].append(len(nodes) - 1)
        bodygroups.append({"name": text_name(name), "base": base, "variants": variants})
    if not meshes:
        raise ValueError("MDL has no renderable mesh")
    metadata["bodygroups"] = bodygroups
    inverse = binary.accessor([value for transform in model.global_bind for value in inverse_matrix(transform)], 16)
    images = []
    materials = []
    textures = []
    for index, texture in enumerate(model.textures):
        images.append({"name": texture["name"], "bufferView": binary.view(texture["png"]), "mimeType": "image/png"})
        textures.append({"sampler": 0, "source": index})
        material = {"name": texture["name"], "pbrMetallicRoughness": {"baseColorTexture": {"index": index}, "metallicFactor": 0, "roughnessFactor": 1}, "doubleSided": False, "extras": {"studioFlags": texture["flags"], "width": texture["width"], "height": texture["height"]}}
        if texture["flags"] & 64:
            material["alphaMode"] = "MASK"
            material["alphaCutoff"] = 0.5
        if texture["flags"] & 32:
            material["alphaMode"] = "BLEND"
        if texture["flags"] & 4:
            material["extensions"] = {"KHR_materials_unlit": {}}
        materials.append(material)
    animations = model_animations(model, binary)
    for attachment in model.attachments:
        if attachment["bone"] < 0 or attachment["bone"] >= len(model.bones):
            raise ValueError("MDL attachment refers to an invalid bone")
        index = len(nodes)
        nodes.append({"name": attachment["name"] or f"attachment-{index}", "translation": attachment["origin"], "extras": {"attachment": True}})
        nodes[attachment["bone"] + 1]["children"].append(index)
    while len(binary.raw) % 4:
        binary.raw.append(0)
    document = {"asset": {"version": "2.0", "generator": "wire-board GoldSrc MDL10 converter"}, "scene": 0, "scenes": [{"nodes": [0]}], "nodes": nodes, "meshes": meshes, "skins": [{"joints": list(range(1, len(model.bones) + 1)), "inverseBindMatrices": inverse}], "materials": materials, "images": images, "textures": textures, "samplers": [{"magFilter": 9729, "minFilter": 9987, "wrapS": 10497, "wrapT": 10497}], "animations": animations, "buffers": [{"byteLength": len(binary.raw)}], "bufferViews": binary.views, "accessors": binary.accessors}
    if any("extensions" in material for material in materials):
        document["extensionsUsed"] = ["KHR_materials_unlit"]
    raw_json = json.dumps(document, separators=(",", ":"), ensure_ascii=True, allow_nan=False).encode()
    raw_json += b" " * (-len(raw_json) % 4)
    raw_glb = struct.pack("<III", 0x46546C67, 2, 12 + 8 + len(raw_json) + 8 + len(binary.raw)) + struct.pack("<II", len(raw_json), 0x4E4F534A) + raw_json + struct.pack("<II", len(binary.raw), 0x004E4942) + binary.raw
    target.parent.mkdir(parents=True, exist_ok=True)
    target.write_bytes(raw_glb)
    return {"file": "models/" + target.name, "bytes": len(raw_glb), "sha256": hashlib.sha256(raw_glb).hexdigest(), "mime": "model/gltf-binary", "sourceSHA256": metadata["sourceSHA256"], "bones": len(model.bones), "sequences": len(model.sequences), "clips": len(animations), "meshes": len(meshes), "triangles": triangles, "_dependencies": model.dependencies}


def build_model_asset(source, key, output_root):
    output_root = Path(output_root)
    source = Path(source)
    if not re.fullmatch(r"[a-z][a-z0-9-]{1,63}", key) or not output_root.resolve().is_relative_to((ROOT / ".local/cs16").resolve()):
        raise ValueError("Model asset key or output location is invalid")
    converter = hashlib.sha256(Path(__file__).read_bytes()).hexdigest()
    cache_directory = ROOT / ".local/cs16/cache/models"
    cache_directory.mkdir(parents=True, exist_ok=True)
    cache_key = hashlib.sha256((str(output_root.resolve()) + "/" + key).encode()).hexdigest()
    cache_path = cache_directory / (cache_key + ".json")
    try:
        cache = json.loads(cache_path.read_text())
        resource = cache["resource"]
        if cache["converter"] != converter or cache["source"] != str(source.resolve()) or not cache["dependencies"]:
            raise ValueError("Model cache identity changed")
        for file, digest in cache["dependencies"].items():
            if hashlib.sha256(source_bytes(Path(file))).hexdigest() != digest:
                raise ValueError("Model cache dependency changed")
        carrier_path = output_root / resource["file"]
        if not carrier_path.resolve().is_relative_to(output_root.resolve()) or carrier_path.is_symlink():
            raise ValueError("Model cache carrier escaped its output")
        carrier = carrier_path.read_bytes()
        if len(carrier) != resource["bytes"] or hashlib.sha256(carrier).hexdigest() != resource["sha256"]:
            raise ValueError("Model cache carrier changed")
        return dict(sorted(resource.items()))
    except (OSError, ValueError, KeyError, TypeError):
        pass
    resource = convert_model(source, output_root / "models" / (key + ".glb"))
    dependencies = resource.pop("_dependencies")
    raw = (output_root / resource["file"]).read_bytes()
    compressed = gzip.compress(raw, compresslevel=9, mtime=0)
    carrier_path = output_root / (resource["file"] + ".gz")
    if carrier_path.is_symlink():
        raise ValueError("Model gzip carrier cannot be a symlink")
    carrier_path.write_bytes(compressed)
    resource.update({"file": resource["file"] + ".gz", "encoding": "gzip", "decodedBytes": len(raw), "decodedSHA256": resource["sha256"], "bytes": len(compressed), "sha256": hashlib.sha256(compressed).hexdigest()})
    cache_path.write_text(json.dumps({"converter": converter, "source": str(source.resolve()), "dependencies": dependencies, "resource": resource}, sort_keys=True, separators=(",", ":")) + "\n")
    return dict(sorted(resource.items()))


def required_models(source_root):
    source_root = Path(source_root)
    if (source_root / "cstrike/models").is_dir():
        source_root = source_root / "cstrike/models"
    elif (source_root / "models").is_dir():
        source_root = source_root / "models"
    result = {"player-" + player: source_root / "player" / player / (player + ".mdl") for player in PLAYERS}
    for weapon in WEAPONS:
        for prefix in ("v", "p", "w"):
            result[prefix + "-" + weapon] = source_root / (prefix + "_" + weapon + ".mdl")
    result["hostage"] = source_root / "hostage.mdl"
    for name in ("w_backpack", "w_thighpack"):
        if (source_root / (name + ".mdl")).is_file():
            result[name.replace("_", "-")] = source_root / (name + ".mdl")
    for name in ("pred_plant", "abone_template1", "skeleton"):
        candidate = source_root / (name + ".mdl")
        if not candidate.is_file():
            candidate = source_root.parent.parent / "valve/models" / (name + ".mdl")
        result["prop-" + name.replace("_", "-")] = candidate
    return result


def build_models(source_root, output_root):
    output_root = Path(output_root)
    required = required_models(source_root)
    missing = [key for key, path in required.items() if not path.is_file()]
    if missing:
        raise ValueError("Missing original models: " + ", ".join(missing))
    result = {}
    for key, path in required.items():
        result[key] = build_model_asset(path, key, output_root)
        print(json.dumps({"model": key, **result[key]}, ensure_ascii=False), flush=True)
    for prefix in ("v", "p", "w"):
        result[prefix + "-mp5navy"] = result[prefix + "-mp5"]
    return {"models": result}


def model_rules(source, identity):
    model = StudioModel(source)
    poses = {}
    sequence_names = {"stand": "idle1", "crouch": "idle1" if identity == "hostage" else "crouch_idle"}
    pose_specs = {key: {"sequence": name, "blend": 0, "frame": 0} for key, name in sequence_names.items()}
    if identity != "hostage":
        for extension in WEAPON_EXTENSIONS:
            for suffix, prefix in [("Stand", "ref_aim_"), ("Crouch", "crouch_aim_")]:
                name = prefix + extension
                sequence = next((item for item in model.sequences if item["name"] == name), None)
                if sequence is None:
                    raise ValueError("Required original weapon pose is missing: " + name)
                pose_specs[extension + suffix] = {"sequence": name, "blend": sequence["blends"] // 2, "frame": 0}
    for pose, specification in pose_specs.items():
        name = specification["sequence"]
        sequence = next((item for item in model.sequences if item["name"] == name), None)
        if sequence is None:
            raise ValueError("Required original player pose is missing: " + name)
        raw = model.sequence_bytes(sequence)
        transforms = []
        for bone_index, bone in enumerate(model.bones):
            offset = sequence["animation"] + (specification["blend"] * len(model.bones) + bone_index) * 12
            offsets = read_values(raw, "<6H", offset)
            values = [bone["value"][component] + (decode_channel(raw, offset + relative, 1)[0] if relative else 0) * bone["scale"][component] for component, relative in enumerate(offsets)]
            if bone_index == sequence["motionBone"]:
                for component in range(3):
                    if sequence["motionType"] & (1 << component):
                        values[component] = 0
            local = matrix(values[:3], quaternion(values[3:]))
            transforms.append(local if bone["parent"] < 0 else multiply_matrix(transforms[bone["parent"]], local))
        matrices = []
        for transform in transforms:
            values = {axis: round(transform[index * 4 + 3] * 256) for index, axis in enumerate("xyz")}
            values.update({row_axis + column_axis: round(transform[row * 4 + column] * 1048576) for row, row_axis in enumerate("xyz") for column, column_axis in enumerate("xyz")})
            matrices.append(values)
        poses[pose] = matrices
    hitboxes = [{"bone": box["bone"], "group": box["group"], "min": {axis: round(value * 256) for axis, value in zip("xyz", box["min"])}, "max": {axis: round(value * 256) for axis, value in zip("xyz", box["max"])}} for box in model.hitboxes]
    return {"id": identity, "sourceSHA256": hashlib.sha256(model.raw).hexdigest(), "origin": "feet" if identity == "hostage" else "center", "hitboxes": hitboxes, "poses": poses, "sequences": sequence_names, "poseSources": pose_specs}


def build_model_rules(source_root):
    sources = required_models(source_root)
    result = [model_rules(sources["player-" + player], player) for player in PLAYERS]
    result.append(model_rules(sources["hostage"], "hostage"))
    encoded = json.dumps(result, separators=(",", ":"), ensure_ascii=True, allow_nan=False)
    target = ROOT / "games/cs16/data/model_rules.go"
    target.parent.mkdir(parents=True, exist_ok=True)
    if target.is_symlink():
        raise ValueError("Model rules output cannot be a symlink")
    target.write_text("package data\n\nconst ModelRulesJSON = `" + encoded + "`\n")
    return result


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--source", type=Path, default=ROOT / ".local/cs16/research/original")
    parser.add_argument("--output", type=Path, default=ROOT / ".local/cs16/assets")
    args = parser.parse_args()
    report_directory = ROOT / ".local/cs16/verification"
    report_directory.mkdir(parents=True, exist_ok=True)
    manifest = build_models(args.source, args.output)
    rules = build_model_rules(args.source)
    (report_directory / "models.json").write_text(json.dumps(manifest, ensure_ascii=False, indent=2) + "\n")
    (args.output / "models.json").write_text(json.dumps(manifest, ensure_ascii=False, separators=(",", ":")) + "\n")
    (report_directory / "model-rules.json").write_text(json.dumps({"characters": len(rules), "sourceSHA256": {item["id"]: item["sourceSHA256"] for item in rules}, "file": "games/cs16/data/model_rules.go", "sha256": hashlib.sha256((ROOT / "games/cs16/data/model_rules.go").read_bytes()).hexdigest()}, indent=2) + "\n")


if __name__ == "__main__":
    main()
