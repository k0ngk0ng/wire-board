"""Selection artifact E2E: source identity, all slots, pixel oracle and repeatability."""
import hashlib
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest
from PIL import Image

ROOT = Path(__file__).resolve().parents[3]
sys.path.insert(0, str(ROOT / 'scripts/basketball'))
from static_sprites import SpriteROM, render_player
from static_court import build_court

EXPORT = ROOT / 'scripts/basketball/selection_assets/export.py'
ART = ROOT / '.local/basketball'
BODY_CROP = (48, 61, 80, 101)
PORTRAIT_CROP = (52, 61, 76, 85)
COURT_CROP = (0, 144, 512, 432)


def digest(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


class SelectionArtifacts(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.workspace = tempfile.TemporaryDirectory(prefix='selection-', dir=ROOT/'.local/basketball/selection-assets-evidence')
        cls.base = Path(cls.workspace.name)
        cls.output = cls.base/'first'
        cls.command(cls.output, check=True)
        cls.manifest = json.loads((cls.output/'manifest.json').read_text())
        cls.rom = SpriteROM((ROOT/'.local/nes-basket/basketball.nes').read_bytes())
        cls.colors = (ROOT/'.local/nes-basket/runs/graphics-right/nes-colors.rgb').read_bytes()

    @classmethod
    def tearDownClass(cls):
        cls.workspace.cleanup()

    @classmethod
    def command(cls, output, *extra, check=False):
        return subprocess.run([sys.executable, str(EXPORT), '--output', str(output), *map(str, extra)], cwd=ROOT,
                              env={**os.environ, 'PYTHONDONTWRITEBYTECODE':'1'}, capture_output=True, text=True, check=check)

    def assertImage(self, asset, expected):
        path = self.output/asset['file']
        self.assertEqual(digest(path), asset['sha256'])
        actual = Image.open(path).convert('RGBA')
        self.assertEqual(actual.size, (asset['width'], asset['height']))
        self.assertEqual(actual.size, expected.size)
        self.assertEqual(actual.tobytes(), expected.tobytes(), asset['file'])

    def test_complete_identity_and_release_separation(self):
        doc = self.manifest
        self.assertEqual(doc['schemaVersion'], 1)
        self.assertIs(doc['accepted_for_release'], False)
        self.assertTrue(doc['requiresAuthoritativePreparationValidation'])
        self.assertEqual([c['id'] for c in doc['courts']], list(range(8)))
        self.assertEqual([t['id'] for t in doc['teams']], list(range(8)))
        rows = [c for t in doc['teams'] for c in t['characters']]
        self.assertEqual([(c['team'], c['character']) for c in rows], [(t,c) for t in range(8) for c in range(4)])
        self.assertEqual(len({c['id'] for c in rows}),32)
        self.assertEqual(len(list(self.output.rglob('*.png'))),72)
        for t in doc['teams']:
            self.assertFalse(t['nameVerified'])
            for c in t['characters']:
                self.assertEqual(c['team'],t['id'])
                self.assertFalse(c['nameVerified'])
                self.assertEqual(c['displayName'],f"球员 {c['character']+1}")
                self.assertNotIn('enabled',c)
        gates = json.loads((self.output/'support-gates.json').read_text())
        self.assertFalse(gates['releaseReady'])
        self.assertEqual(gates['authority'],'evidence-only-not-an-allowlist')
        self.assertEqual(gates['requiredPolicyOwner'],'server preparation validator')

    def test_all_original_player_pixels(self):
        for team in self.manifest['teams']:
            t=team['id']; palette=bytes(16)+self.rom.team_palette(t)*2
            pages=[96,97,98,99]+self.rom.player_chr_pages(t,t)
            for c in team['characters']:
                source=c['source']; self.assertEqual((source['team'],source['character'],source['pose'],source['facing'],source['priority']), (t,c['character'],0,0,'forward'))
                image=render_player(self.rom,t,0,c['character'],0,pages,palette,self.colors,size=(128,128),origin=(64,96))
                bbox=image.getbbox()
                self.assertTrue(BODY_CROP[0]<=bbox[0] and BODY_CROP[1]<=bbox[1] and BODY_CROP[2]>=bbox[2] and BODY_CROP[3]>=bbox[3])
                self.assertImage(c['fullBody'],image.crop(BODY_CROP).resize((128,160),Image.Resampling.NEAREST))
                self.assertImage(c['portrait'],image.crop(PORTRAIT_CROP).resize((96,96),Image.Resampling.NEAREST))

    def test_all_original_court_pixels(self):
        hashes=set()
        for c in self.manifest['courts']:
            self.assertEqual(c['source']['palette'],c['id']+1)
            self.assertEqual(c['source']['crop'],list(COURT_CROP))
            self.assertImage(c['preview'],build_court(self.rom,self.colors,court=c['id']).crop(COURT_CROP))
            hashes.add(c['preview']['sha256'])
        self.assertEqual(len(hashes),8)

    def test_repeatable_all_artifacts(self):
        second=self.base/'second';self.command(second,check=True)
        files=lambda root:{p.relative_to(root).as_posix():digest(p) for p in root.rglob('*') if p.is_file()}
        self.assertEqual(files(self.output),files(second))

    def test_incorrect_source_digest_fails_before_output(self):
        bad=self.base/'bad-atlas';bad.mkdir(exist_ok=True)
        atlas=json.loads((ART/'static-atlas/atlas.json').read_text());atlas['source']['rom_sha256']='0'*64
        (bad/'atlas.json').write_text(json.dumps(atlas))
        out=self.base/'bad-digest-output';result=self.command(out,'--atlas-dir',bad)
        self.assertNotEqual(result.returncode,0)
        self.assertIn('ROM source identity',result.stderr)
        self.assertFalse(out.exists())

    def test_duplicate_character_source_fails_before_output(self):
        bad=self.base/'duplicate-atlas';bad.mkdir(exist_ok=True)
        atlas=json.loads((ART/'static-atlas/atlas.json').read_text());atlas['sheets'].append(atlas['sheets'][0])
        (bad/'atlas.json').write_text(json.dumps(atlas))
        out=self.base/'duplicate-output';result=self.command(out,'--atlas-dir',bad)
        self.assertNotEqual(result.returncode,0)
        self.assertIn('duplicate sprite key',result.stderr)
        self.assertFalse(out.exists())

    def test_missing_source_image_fails_before_output(self):
        bad=self.base/'missing-atlas';bad.mkdir(exist_ok=True)
        (bad/'atlas.json').write_bytes((ART/'static-atlas/atlas.json').read_bytes())
        out=self.base/'missing-output';result=self.command(out,'--atlas-dir',bad)
        self.assertNotEqual(result.returncode,0)
        self.assertFalse(out.exists())


if __name__ == '__main__':
    unittest.main(verbosity=2)
