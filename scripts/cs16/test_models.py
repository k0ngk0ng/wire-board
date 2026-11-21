import importlib.util
import gzip
import hashlib
import json
import math
from pathlib import Path
import struct
import sys
import unittest
from unittest import mock


ROOT = Path(__file__).resolve().parents[2]
MODULE = ROOT / "scripts/cs16/models.py"
TEMP = ROOT / ".local/cs16/tmp/model-tests"
SOURCE = ROOT / ".local/cs16/research/original/cstrike/models"
TEMP.mkdir(parents=True, exist_ok=True)
SPEC = importlib.util.spec_from_file_location("cs16_models", MODULE)
models = importlib.util.module_from_spec(SPEC)
sys.modules[SPEC.name] = models
SPEC.loader.exec_module(models)


def glb_parts(path):
    raw = path.read_bytes()
    magic, version, length = struct.unpack_from("<III", raw)
    if magic != 0x46546C67 or version != 2 or length != len(raw):
        raise AssertionError("Invalid GLB carrier")
    json_size, json_type = struct.unpack_from("<II", raw, 12)
    if json_type != 0x4E4F534A:
        raise AssertionError("Missing JSON chunk")
    document = json.loads(raw[20:20 + json_size])
    position = 20 + json_size
    bin_size, bin_type = struct.unpack_from("<II", raw, position)
    if bin_type != 0x004E4942:
        raise AssertionError("Missing BIN chunk")
    return document, raw[position + 8:position + 8 + bin_size]


class ModelConversionTests(unittest.TestCase):
    def test_rle_run_boundaries(self):
        raw = bytes([2, 4]) + struct.pack("<hh", 10, 20) + bytes([1, 3]) + struct.pack("<h", -5)
        self.assertEqual(models.decode_channel(raw, 0, 7), [10, 20, 20, 20, -5, -5, -5])

    def test_rle_invalid_counts_and_truncation(self):
        for raw in [bytes([0, 0]), bytes([3, 2, 1, 0]), bytes([2, 2, 1, 0])]:
            with self.subTest(raw=raw), self.assertRaises(ValueError):
                models.decode_channel(raw, 0, 3)

    def test_model_header_identity_and_bounds(self):
        raw = (SOURCE / "v_ak47.mdl").read_bytes()
        variants = [b"", b"OTHER" + raw[5:], raw[:200]]
        wrong_version = bytearray(raw)
        struct.pack_into("<i", wrong_version, 4, 11)
        variants.append(bytes(wrong_version))
        wrong_length = bytearray(raw)
        struct.pack_into("<i", wrong_length, 72, len(raw) + 1)
        variants.append(bytes(wrong_length))
        for index, value in enumerate(variants):
            path = TEMP / f"bad-{index}.mdl"
            path.write_bytes(value)
            with self.subTest(index=index), self.assertRaises(ValueError):
                models.StudioModel(path)

    def test_model_rejects_cyclic_bones(self):
        raw = bytearray((SOURCE / "v_ak47.mdl").read_bytes())
        bone_index = struct.unpack_from("<i", raw, 144)[0]
        struct.pack_into("<i", raw, bone_index + 32, 0)
        path = TEMP / "cyclic.mdl"
        path.write_bytes(raw)
        with self.assertRaises(ValueError):
            models.StudioModel(path)

    def test_ak47_glb_has_actual_mesh_skin_and_animation(self):
        result = models.convert_model(SOURCE / "v_ak47.mdl", TEMP / "v-ak47.glb")
        document, binary = glb_parts(TEMP / "v-ak47.glb")
        self.assertEqual(result["bones"], 42)
        self.assertGreater(result["triangles"], 100)
        self.assertEqual(result["sequences"], 6)
        self.assertGreaterEqual(len(document["animations"]), 6)
        self.assertTrue(any(animation["name"] == "reload" for animation in document["animations"]))
        self.assertEqual(len(document["skins"][0]["joints"]), 42)
        self.assertGreater(len(document["images"]), 0)
        self.assertTrue(any(node.get("skin") == 0 for node in document["nodes"]))
        self.assertEqual(document["buffers"][0]["byteLength"], len(binary))
        for accessor in document["accessors"]:
            self.assertGreater(accessor["count"], 0)
        for view in document["bufferViews"]:
            self.assertLessEqual(view.get("byteOffset", 0) + view["byteLength"], len(binary))
        geometry = document["meshes"][0]["primitives"][0]["attributes"]
        positions = document["accessors"][geometry["POSITION"]]
        self.assertTrue(all(math.isfinite(value) for value in positions["min"] + positions["max"]))

    def test_identical_source_is_byte_deterministic(self):
        models.convert_model(SOURCE / "p_ak47.mdl", TEMP / "p-ak47-a.glb")
        models.convert_model(SOURCE / "p_ak47.mdl", TEMP / "p-ak47-b.glb")
        self.assertEqual((TEMP / "p-ak47-a.glb").read_bytes(), (TEMP / "p-ak47-b.glb").read_bytes())

    def test_texture_coordinates_preserve_original_upload_orientation(self):
        model = models.StudioModel(SOURCE / "p_ak47.mdl")
        body = struct.unpack_from("<64s3i", model.raw, model.header[17])
        geometry = struct.unpack_from("<64sif10i", model.raw, body[3])
        mesh = struct.unpack_from("<5i", model.raw, geometry[4])
        entry = struct.unpack_from("<4h", model.raw, mesh[1] + 2)
        texture = model.textures[model.skins[0][mesh[2]]]
        models.convert_model(SOURCE / "p_ak47.mdl", TEMP / "uv.glb")
        document, binary = glb_parts(TEMP / "uv.glb")
        accessor = document["accessors"][document["meshes"][0]["primitives"][0]["attributes"]["TEXCOORD_0"]]
        view = document["bufferViews"][accessor["bufferView"]]
        u, v = struct.unpack_from("<2f", binary, view.get("byteOffset", 0))
        self.assertAlmostEqual(u, entry[2] / texture["width"])
        self.assertAlmostEqual(v, entry[3] / texture["height"])

    def test_output_traversal_is_rejected(self):
        with self.assertRaises(ValueError):
            models.convert_model(SOURCE / "p_ak47.mdl", TEMP / "../../../../escape.glb")

    def test_player_preserves_all_sequence_metadata_and_hitboxes(self):
        model = models.StudioModel(SOURCE / "player/urban/urban.mdl")
        self.assertEqual(len(model.bones), 53)
        self.assertEqual(len(model.sequences), 111)
        self.assertEqual(len(model.hitboxes), 21)
        self.assertEqual(len(model.attachments), 2)
        self.assertGreater(sum(sequence["blends"] for sequence in model.sequences), 111)

    def test_all_required_models_exist(self):
        required = models.required_models(SOURCE)
        for key in ["player-urban", "player-terror", "v-ak47", "v-mp5", "p-m3", "w-awp", "v-knife", "v-hegrenade", "v-flashbang", "v-smokegrenade", "v-c4", "hostage"]:
            self.assertIn(key, required)
        for key, path in required.items():
            with self.subTest(key=key):
                self.assertTrue(path.is_file())

    def test_aztec_original_decorations_convert_to_verified_glb(self):
        required = models.required_models(SOURCE)
        for key in ["prop-pred-plant", "prop-abone-template1", "prop-skeleton"]:
            with self.subTest(key=key):
                self.assertIn(key, required)
                resource = models.build_model_asset(required[key], key, TEMP / "props")
                self.assertGreater(resource["triangles"], 0)
                self.assertGreater(resource["bones"], 0)
                carrier = (TEMP / "props" / resource["file"]).read_bytes()
                raw = gzip.decompress(carrier)
                self.assertEqual(hashlib.sha256(raw).hexdigest(), resource["decodedSHA256"])
                self.assertEqual(raw[:4], b"glTF")

    def test_asset_gzip_roundtrip_and_cache_validation(self):
        output = TEMP / "cached-assets"
        result = models.build_model_asset(SOURCE / "p_ak47.mdl", "p-ak47", output)
        self.assertEqual(result["encoding"], "gzip")
        self.assertTrue(result["file"].endswith(".glb.gz"))
        carrier = (output / result["file"]).read_bytes()
        raw = gzip.decompress(carrier)
        self.assertEqual(result["sha256"], hashlib.sha256(carrier).hexdigest())
        self.assertEqual(result["decodedSHA256"], hashlib.sha256(raw).hexdigest())
        self.assertEqual(result["decodedBytes"], len(raw))
        with mock.patch.object(models, "convert_model", wraps=models.convert_model) as converter:
            cached = models.build_model_asset(SOURCE / "p_ak47.mdl", "p-ak47", output)
            self.assertEqual(result, cached)
            self.assertEqual(json.dumps(result), json.dumps(cached))
            converter.assert_not_called()
        (output / result["file"]).write_bytes(b"broken")
        with mock.patch.object(models, "convert_model", wraps=models.convert_model) as converter:
            rebuilt = models.build_model_asset(SOURCE / "p_ak47.mdl", "p-ak47", output)
            converter.assert_called_once()
            self.assertEqual(result, rebuilt)

    def test_original_hitbox_rules_are_integer_and_complete(self):
        rules = models.model_rules(SOURCE / "player/urban/urban.mdl", "urban")
        self.assertEqual(len(rules["hitboxes"]), 21)
        self.assertEqual(len(rules["poses"]["stand"]), 53)
        self.assertEqual(len(rules["poses"]["crouch"]), 53)
        self.assertEqual(rules["sequences"], {"stand": "idle1", "crouch": "crouch_idle"})
        for pose in rules["poses"].values():
            for transform in pose:
                self.assertEqual(set(transform), {"x", "y", "z", "xx", "xy", "xz", "yx", "yy", "yz", "zx", "zy", "zz"})
                self.assertTrue(all(isinstance(value, int) for value in transform.values()))
        self.assertEqual({box["group"] for box in rules["hitboxes"]}, {1, 2, 3, 4, 5, 6, 7, 8})
        head = next(box for box in rules["hitboxes"] if box["group"] == 1)
        self.assertGreater(rules["poses"]["stand"][head["bone"]]["z"] + 36 * 256, rules["poses"]["crouch"][head["bone"]]["z"] + 18 * 256)

    def test_original_weapon_pose_neutral_blends_cover_every_character(self):
        for identity in models.PLAYERS:
            rules = models.model_rules(SOURCE / "player" / identity / (identity + ".mdl"), identity)
            for extension in ["ak47", "rifle", "carbine", "mp5", "onehanded", "dualpistols", "shotgun", "m249", "knife", "grenade", "c4"]:
                for suffix, prefix in [("Stand", "ref_aim_"), ("Crouch", "crouch_aim_")]:
                    key = extension + suffix
                    with self.subTest(character=identity, pose=key):
                        self.assertEqual(len(rules["poses"][key]), len(rules["poses"]["stand"]))
                        self.assertEqual(rules["poseSources"][key], {"sequence": prefix + extension, "blend": 4, "frame": 0})

    def test_hostage_original_hitboxes_use_idle_pose_and_feet_origin(self):
        rules = models.model_rules(SOURCE / "hostage.mdl", "hostage")
        self.assertEqual(rules["id"], "hostage")
        self.assertEqual(rules["origin"], "feet")
        self.assertEqual(len(rules["hitboxes"]), 19)
        self.assertEqual(len(rules["poses"]["stand"]), 21)
        self.assertEqual(rules["poses"]["crouch"], rules["poses"]["stand"])
        self.assertEqual(rules["poseSources"]["stand"], {"sequence": "idle1", "blend": 0, "frame": 0})

    def test_goldsrc_clockwise_faces_become_gltf_counterclockwise(self):
        models.convert_model(SOURCE / "v_glock18.mdl", TEMP / "winding.glb")
        document, binary = glb_parts(TEMP / "winding.glb")
        self.assertTrue(all(not material.get("doubleSided", False) for material in document["materials"]))
        aligned = 0
        reversed_faces = 0
        for mesh in document["meshes"]:
            for primitive in mesh["primitives"]:
                def values(accessor_index, width, kind):
                    accessor = document["accessors"][accessor_index]
                    view = document["bufferViews"][accessor["bufferView"]]
                    raw = struct.unpack_from("<" + kind * accessor["count"] * width, binary, view.get("byteOffset", 0))
                    return [raw[index:index + width] for index in range(0, len(raw), width)]
                positions = values(primitive["attributes"]["POSITION"], 3, "f")
                normals = values(primitive["attributes"]["NORMAL"], 3, "f")
                indices = [value[0] for value in values(primitive["indices"], 1, "I")]
                self.assertEqual(indices[:3], [0, 2, 1])
                for index in range(0, len(indices), 3):
                    a, b, c = indices[index:index + 3]
                    ab = [positions[b][axis] - positions[a][axis] for axis in range(3)]
                    ac = [positions[c][axis] - positions[a][axis] for axis in range(3)]
                    cross = [ab[1] * ac[2] - ab[2] * ac[1], ab[2] * ac[0] - ab[0] * ac[2], ab[0] * ac[1] - ab[1] * ac[0]]
                    dot = sum(cross[axis] * sum(normals[vertex][axis] for vertex in [a, b, c]) for axis in range(3))
                    aligned += dot > 0
                    reversed_faces += dot < 0
        self.assertGreater(aligned, reversed_faces * 9)

    def test_masked_and_additive_materials_keep_alpha_with_single_sided_faces(self):
        original = bytearray((SOURCE / "v_glock18.mdl").read_bytes())
        texture_index = struct.unpack_from("<i", original, 184)[0]
        for flag, alpha in [(64, "MASK"), (32, "BLEND")]:
            raw = bytearray(original)
            struct.pack_into("<i", raw, texture_index + 64, flag)
            source = TEMP / ("alpha-" + str(flag) + ".mdl")
            target = TEMP / ("alpha-" + str(flag) + ".glb")
            source.write_bytes(raw)
            models.convert_model(source, target)
            document, _ = glb_parts(target)
            self.assertEqual(document["materials"][0]["alphaMode"], alpha)
            self.assertEqual(document["materials"][0]["extras"]["studioFlags"], flag)
            self.assertFalse(document["materials"][0].get("doubleSided", False))


if __name__ == "__main__":
    unittest.main(verbosity=2)
