#!/usr/bin/env python3
"""End-to-end checks for conversion of the verified original NFS4 demo."""

import hashlib
import json
import math
import os
from pathlib import Path
import shutil
import struct
import subprocess
import sys
import unittest

ROOT = Path(__file__).resolve().parents[2]
EXTENDED = '--extended' in sys.argv
if EXTENDED:
    sys.argv.remove('--extended')
GAME = ROOT / ('.local/racing-nfs-reference/additional/game-root' if EXTENDED else '.local/racing-nfs-reference/wineprefix/drive_c/NFSHSD')
WORK = ROOT / ('.local/racing-nfs-reference/asset-e2e-full' if EXTENDED else '.local/racing-nfs-reference/asset-e2e')
OUT = WORK / 'output'
CARS = ['P911','FM50','LDIA','MCF1','CCOR'] if EXTENDED else ['P911','FM50']
TRACKS = ['France','Coastal','UK','Park'] if EXTENDED else ['France']


def glb(path):
    data = path.read_bytes()
    magic, version, size = struct.unpack_from('<4sII', data)
    if (magic, version, size) != (b'glTF', 2, len(data)):
        raise AssertionError(f'Invalid GLB container: {path}')
    count, kind = struct.unpack_from('<II', data, 12)
    if kind != 0x4E4F534A:
        raise AssertionError('Missing GLB JSON chunk')
    doc = json.loads(data[20:20+count])
    cursor = 20 + count
    bin_size, bin_kind = struct.unpack_from('<II', data, cursor)
    if bin_kind != 0x004E4942:
        raise AssertionError('Missing GLB binary chunk')
    blob = data[cursor+8:cursor+8+bin_size]
    return doc, blob


class OriginalConversion(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        if WORK.exists():
            shutil.rmtree(WORK)
        WORK.mkdir(parents=True)
        command = [sys.executable, '-B', str(ROOT/'scripts/racing/nfs-convert.py'),
                   '--game-root', str(GAME), '--out', str(OUT), '--cars', *CARS, '--tracks', *TRACKS]
        result = subprocess.run(command, text=True, capture_output=True, cwd=ROOT)
        (WORK/'conversion.log').write_text(result.stdout + result.stderr)
        if result.returncode:
            raise AssertionError(f'Converter failed ({result.returncode}): {result.stderr}')
        cls.command = command
        cls.manifest = json.loads((OUT/'manifest.json').read_text())

    def test_original_car_shapes_wheels_and_parameters(self):
        cars = json.loads((OUT/'cars.json').read_text())
        self.assertEqual(set(cars['cars']), set(CARS))
        expected = {'P911': (1500, 80.555557, 4.57179, 1127), 'FM50': (1690, 88.962402, 4.54812, 995)}
        if EXTENDED:
            expected.update({'LDIA':(1625,93.055557,4.47000,1077),
                             'MCF1':(915,88.068306,4.92000,907), 'CCOR':(1472,76.445084,4.56,1044)})
        for ident, (mass, speed, length, native_triangles) in expected.items():
            with self.subTest(car=ident):
                car = cars['cars'][ident]
                self.assertEqual(car['defaultVariant'], 'base')
                self.assertEqual(len(car['variants']), 1 if ident == 'MCF1' else 4)
                self.assertEqual(car['variants']['base']['parameters']['mass'], mass)
                self.assertAlmostEqual(car['variants']['base']['parameters']['maximum_velocity'], speed, places=5)
                self.assertAlmostEqual(car['dimensions'][2], length, places=4)
                doc, blob = glb(OUT/f'cars/{ident.lower()}.glb')
                names = {node.get('name') for node in doc['nodes']}
                self.assertTrue({'front_left_whl', 'front_right_whl', 'rear_left_whl', 'rear_right_whl'} <= names)
                exterior = next(node for node in doc['nodes'] if node.get('name') == 'exterior')
                self.assertEqual(sum(doc['accessors'][p['indices']]['count']//3 for node in (doc['nodes'][i] for i in exterior['children']) for p in doc['meshes'][node['mesh']]['primitives']), native_triangles)
                self.assertGreater(len(blob), 10000)
                self.assertGreater(car['groundOffset'], 0.3)
                self.assertLess(car['groundOffset'], 0.9)
                self.assertGreater(car['wheels']['front_left_whl']['position'][0], 0)
                self.assertLess(car['wheels']['front_right_whl']['position'][0], 0)

    def test_original_track_length_height_and_consistent_origin(self):
        route = json.loads((OUT/'tracks/france.route.json').read_text())
        self.assertEqual(route['version'], 1)
        self.assertEqual(route['id'], 'france')
        self.assertEqual(len(route['points']), 973)
        self.assertAlmostEqual(route['length'], 4487.36079, places=2)
        points = route['points']
        self.assertGreater(max(p['y'] for p in points)-min(p['y'] for p in points), 180)
        self.assertGreater(math.dist([points[0][k] for k in ('x','y','z')], [points[-1][k] for k in ('x','y','z')]), 1)
        self.assertGreaterEqual(min(p['x'] for p in points), 30)
        self.assertGreaterEqual(min(p['z'] for p in points), 30)
        self.assertEqual(route['sourceTransform'], [-1,1,1])
        self.assertAlmostEqual(points[0]['x'] + route['origin'][0], -140.77410888671875, places=5)
        self.assertAlmostEqual(points[0]['z'] + route['origin'][2], -887.0740966796875, places=5)
        self.assertGreater(len(route['barriers']), 50)
        self.assertEqual((points[0]['leftWidth'], points[0]['rightWidth']), (6, 12))
        first = points[0]
        right = (first['fy']*first['nz']-first['fz']*first['ny'],
                 first['fz']*first['nx']-first['fx']*first['nz'],
                 first['fx']*first['ny']-first['fy']*first['nx'])
        self.assertGreater(sum(a*b for a,b in zip(right,(first['rx'],first['ry'],first['rz']))), .98)
        for point in points:
            self.assertTrue(all(math.isfinite(point[key]) for key in ('x','y','z','nx','ny','nz','fx','fy','fz','leftWidth','rightWidth')))
            self.assertGreater(point['leftWidth'], 0)
            self.assertGreater(point['rightWidth'], 0)
        for variant in ('france', 'france-night'):
            doc, blob = glb(OUT/f'tracks/{variant}.glb')
            self.assertEqual(doc['extras']['origin'], route['origin'])
            triangles = sum(doc['accessors'][p['indices']]['count']//3 for m in doc['meshes'] for p in m['primitives'])
            self.assertGreater(triangles, 35000)
            self.assertGreater(len(doc['images']), 100)
            self.assertGreater(len(blob), 100000)

    def test_every_image_and_buffer_view_is_valid(self):
        from PIL import Image
        from io import BytesIO
        from speedtools.car_data import CarData
        from speedtools.track_data import TrackData
        from speedtools.utils import create_pil_image
        native_images = set()
        for ident in CARS:
            for resource in CarData(GAME/'Data/Cars'/ident, GAME).materials:
                image = create_pil_image(resource.image).convert('RGBA')
                native_images.add((image.size, hashlib.sha256(image.tobytes()).hexdigest()))
        for ident in TRACKS:
            for night in (False, True):
                track = TrackData(GAME/'Data/Tracks'/ident, GAME, night=night)
                for resource in list(track.track_resources)+list(track.sfx.resources):
                    image = create_pil_image(resource.image).convert('RGBA')
                    native_images.add((image.size, hashlib.sha256(image.tobytes()).hexdigest()))
        names = [f'cars/{ident.lower()}.glb' for ident in CARS]+[f'tracks/{ident.lower()}{suffix}.glb' for ident in TRACKS for suffix in ('','-night')]
        for name in names:
            doc, blob = glb(OUT/name)
            for view in doc['bufferViews']:
                self.assertLessEqual(view.get('byteOffset',0)+view['byteLength'], len(blob))
            for item in doc['images']:
                view = doc['bufferViews'][item['bufferView']]
                image = Image.open(BytesIO(blob[view['byteOffset']:view['byteOffset']+view['byteLength']]))
                image.load()
                self.assertEqual(list(image.size), item['extras']['sourceDimensions'])
                self.assertIn((image.size, hashlib.sha256(image.convert('RGBA').tobytes()).hexdigest()), native_images)
            for texture in doc['textures']:
                source = texture.get('source', texture.get('extensions', {}).get('EXT_texture_webp', {}).get('source'))
                self.assertIsNotNone(source)
                self.assertLess(source, len(doc['images']))
                if 'extensions' in texture:
                    self.assertIn('EXT_texture_webp', doc['extensionsRequired'])
            for material in doc['materials']:
                if 'alphaCutoff' in material:
                    self.assertEqual(material.get('alphaMode'), 'MASK')

    def test_native_ambient_loops_keep_keyframes_and_billboard_centres(self):
        doc, blob = glb(OUT/'tracks/france.glb')
        ambient = [animation for animation in doc['animations'] if animation.get('extras',{}).get('trigger') != 'collision']
        destruction = [animation for animation in doc['animations'] if animation.get('extras',{}).get('trigger') == 'collision']
        self.assertEqual(len(ambient), 2)
        self.assertEqual(len(destruction), 29*2)
        durations = sorted(doc['accessors'][animation['samplers'][0]['input']]['max'][0] for animation in ambient)
        self.assertEqual(durations, [9.28125,37.3125])
        for animation in ambient:
            self.assertEqual({channel['target']['path'] for channel in animation['channels']}, {'translation','rotation'})
            for sampler in animation['samplers']:
                self.assertEqual(sampler['interpolation'], 'LINEAR')
                self.assertGreater(doc['accessors'][sampler['input']]['max'][0], 1)
                for accessor in ('input','output'):
                    view = doc['accessors'][sampler[accessor]]['bufferView']
                    self.assertNotIn('target', doc['bufferViews'][view])
        billboards = [node for node in doc['nodes'] if node.get('extras',{}).get('billboard')]
        self.assertGreater(len(billboards), 0)
        self.assertTrue(all('translation' in node for node in billboards))

    def test_manifest_matches_files_and_repeated_conversion_is_identical(self):
        for item in self.manifest['assets']:
            data = (OUT/item['path']).read_bytes()
            self.assertEqual(item['bytes'], len(data))
            self.assertEqual(item['sha256'], hashlib.sha256(data).hexdigest())
        original = {p.relative_to(OUT).as_posix():hashlib.sha256(p.read_bytes()).hexdigest() for p in OUT.rglob('*') if p.is_file()}
        second = WORK/'repeat'
        cmd = list(self.command)
        cmd[cmd.index('--out')+1] = str(second)
        result = subprocess.run(cmd, text=True, capture_output=True, cwd=ROOT)
        self.assertEqual(result.returncode, 0, result.stderr)
        repeated = {p.relative_to(second).as_posix():hashlib.sha256(p.read_bytes()).hexdigest() for p in second.rglob('*') if p.is_file()}
        self.assertEqual(original, repeated)

    def test_each_route_matches_its_geometry_and_original_point_count(self):
        from speedtools.track_data import TrackData
        for ident in TRACKS:
            route = json.loads((OUT/f'tracks/{ident.lower()}.route.json').read_text())
            native = TrackData(GAME/'Data/Tracks'/ident, GAME)
            self.assertEqual(len(route['points']), native.frd.num_road_blocks)
            self.assertEqual(route['sourceTransform'], [-1,1,1])
            self.assertEqual(route['geometryVersion'], f"nfs4-v1-{route['geometryHash'][:16]}")
            for suffix in ('','-night'):
                doc, _ = glb(OUT/f'tracks/{ident.lower()}{suffix}.glb')
                self.assertEqual(doc['extras']['geometryVersion'], route['geometryVersion'])
                self.assertEqual(doc['extras']['origin'], route['origin'])


if __name__ == '__main__':
    runner = unittest.TextTestRunner(verbosity=2)
    result = runner.run(unittest.defaultTestLoader.loadTestsFromTestCase(OriginalConversion))
    if WORK.exists():
        (WORK/'result.json').write_text(json.dumps({'passed':result.wasSuccessful(), 'tests':result.testsRun,
            'cars':CARS, 'tracks':TRACKS, 'failures':[name for name,_ in result.failures],
            'errors':[name for name,_ in result.errors]}, ensure_ascii=False, indent=2, default=str)+'\n')
    sys.exit(0 if result.wasSuccessful() else 1)
