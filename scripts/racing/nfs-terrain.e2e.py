#!/usr/bin/env python3
"""Real original collision-material catalog conversion, no synthetic terrain."""
import hashlib
import json
from pathlib import Path
import subprocess
import sys
import unittest

ROOT = Path(__file__).resolve().parents[2]
WORK = ROOT / '.local/racing-followup/terrain-e2e'

def digest(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()

class TerrainConversion(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        WORK.mkdir(parents=True, exist_ok=True)
        cls.index = json.loads((ROOT/'games/racing/server/data/index.json').read_text())
        cls.before = {str(path): digest(path) for folder in ['games/racing/assets/public/racing/v3', 'games/racing/server/data/rules'] for path in (ROOT/folder).rglob('*') if path.is_file()}
        cls.outputs = []
        for name in ['first', 'repeat']:
            out=WORK/name
            result=subprocess.run([sys.executable,'-B',str(ROOT/'scripts/racing/nfs-terrain.py'),'--out',str(out)],cwd=ROOT,text=True,capture_output=True)
            (WORK/(name+'.log')).write_text(result.stdout+result.stderr)
            if result.returncode:raise AssertionError(result.stdout+result.stderr)
            cls.outputs.append(out)

    def test_all_source_material_polygons_and_coordinates_are_preserved(self):
        index=json.loads((self.outputs[0]/'terrain-index.json').read_text())
        self.assertEqual(len(index),4)
        for spec in self.index:
            with self.subTest(track=spec['id']):
                path=ROOT/f"games/racing/assets/public/racing/v3/tracks/{spec['id']}.collision.json"
                original=json.loads(path.read_text());entry=next(row for row in index if row['id']==spec['id'])
                self.assertEqual(entry['geometry'],spec['geometry']);self.assertEqual(entry['sourceSha256'],digest(path))
                target=self.outputs[0]/'terrain'/(spec['geometry']+'.json');data=json.loads(target.read_text())
                self.assertEqual(digest(target),entry['sha256'])
                self.assertEqual((data['version'],data['unit'],data['id'],data['geometry'],data['sourceSha256']),(1,'decimetres',spec['id'],spec['geometry'],digest(path)))
                self.assertEqual(len(data['surfaces']),len(original['surfaces']))
                for actual,source in zip(data['surfaces'],original['surfaces'],strict=True):
                    self.assertEqual((actual['segment'],actual['surface'],actual['faces']),(source['segment'],source['surface'],source['faces']))
                    self.assertEqual(len(actual['vertices']),len(source['vertices']))
                    for a,b in zip(actual['vertices'],source['vertices'],strict=True):
                        for x,y in zip(a,b,strict=True):self.assertAlmostEqual(x,y*10,places=5)

    def test_repeated_conversion_is_identical(self):
        inventory=lambda p:{str(f.relative_to(p)):digest(f)for f in p.rglob('*')if f.is_file()}
        self.assertEqual(inventory(self.outputs[0]),inventory(self.outputs[1]))

    def test_original_wheel_centres_are_shared_without_estimated_track_width(self):
        source=ROOT/'games/racing/assets/public/racing/v3/cars.json'
        cars=json.loads(source.read_text())['cars'];wheels=json.loads((self.outputs[0]/'terrain-wheels.json').read_text())
        self.assertEqual(wheels['sourceSha256'],digest(source));self.assertEqual(wheels['unit'],'decimetres')
        self.assertEqual(set(wheels['vehicles']),{name.lower()for name in cars})
        for name,car in cars.items():
            actual=wheels['vehicles'][name.lower()]
            self.assertAlmostEqual(actual['groundOffset'],car['groundOffset']*10,places=5)
            for wheel in actual['wheels']:
                original=car['wheels'][wheel['name']]
                self.assertEqual(wheel['front'],wheel['name'].startswith('front'))
                for x,y in zip(wheel['position'],original['position'],strict=True):self.assertAlmostEqual(x,y*10,places=5)
                self.assertAlmostEqual(wheel['radius'],original['radius']*10,places=5)

    def test_material_parameters_are_copied_from_the_audited_native_numeric_record(self):
        source=ROOT/'games/racing/assets/source/nfs4/terrain-materials.json'
        self.assertEqual((self.outputs[0]/'terrain-materials.json').read_bytes(),source.read_bytes())

    def test_existing_client_assets_and_immutable_road_rules_are_untouched(self):
        for name,before in self.before.items():self.assertEqual(digest(Path(name)),before,name)

if __name__=='__main__':
    unittest.main(verbosity=2)
