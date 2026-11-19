import hashlib
import json
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest

import cdn_delivery as cdn
import cdn_local_ready as ready
import asset_reuse as reuse

ROOT = Path(__file__).resolve().parents[1]
WORK = ROOT / '.local/resource-cleanup/canonical-tests'
WORK.mkdir(parents=True, exist_ok=True)


class CanonicalPublicationE2E(unittest.TestCase):
    def test_real_cli_places_every_game_and_shared_object_under_its_owner(self):
        with tempfile.TemporaryDirectory(dir=WORK) as name:
            dist = Path(name)
            (dist / 'assets').mkdir()
            (dist / 'splendor').mkdir()
            code = b'export const ready=true;'
            image = b'generated pixels'
            (dist / 'assets/app-abcdefgh.js').write_bytes(code)
            (dist / 'splendor/cards.webp').write_bytes(image)
            base = 'https://cdn.ichenj.com/bga/'
            (dist / 'index.html').write_text('<script src="' + base + 'assets/app-abcdefgh.js"></script>')
            (dist / 'game-load-manifest.json').write_text(json.dumps({'sourceSHA': 'a' * 40, 'assetBase': base, 'entry': base + 'assets/app-abcdefgh.js', 'games': {'splendor': [{'url': base + 'assets/app-abcdefgh.js', 'bytes': len(code)}]}, 'owners': {'assets/app-abcdefgh.js': '_common'}, 'external': {'/splendor/cards.webp': {'url': base + 'splendor/' + 'c' * 64 + '.webp', 'bytes': 9, 'sha256': 'c' * 64, 'mime': 'image/webp', 'encoding': ''}, '/splendor/nobles.webp': {'url': base + 'splendor/' + 'b' * 64 + '.webp', 'bytes': 7, 'sha256': 'b' * 64, 'mime': 'image/webp', 'encoding': ''}}, 'media': {'/splendor/cards.webp': len(image)}}))
            baseline = dist / 'baseline.json'
            baseline.write_text(json.dumps({'version': 1, 'files': {}}))
            baseline_copy = WORK / ('baseline-' + dist.name + '.json')
            baseline.replace(baseline_copy)
            try:
                command = [sys.executable, '-B', 'scripts/asset_reuse.py', '--dist', str(dist), '--baseline', str(baseline_copy)]
                subprocess.run(command, cwd=ROOT, check=True, capture_output=True)
                manifest = json.loads((dist / 'cdn-assets-manifest.json').read_text())
                self.assertEqual(manifest['files']['/splendor/cards.webp']['url'], base + 'splendor/' + hashlib.sha256(image).hexdigest() + '.webp')
                self.assertEqual(manifest['files']['/assets/app-abcdefgh.js']['url'], base + '_common/app-abcdefgh.js')
                self.assertNotIn('/splendor/cards.webp', manifest['external'])
                self.assertEqual(manifest['external']['/splendor/nobles.webp']['url'], base + 'splendor/' + 'b' * 64 + '.webp')
                self.assertEqual(json.loads((dist / 'game-load-manifest.json').read_text())['sourceSHA'], 'a' * 40)
                before = {str(path.relative_to(dist)): path.read_bytes() for path in dist.rglob('*') if path.is_file()}
                subprocess.run(command, cwd=ROOT, check=True, capture_output=True)
                self.assertEqual(before, {str(path.relative_to(dist)): path.read_bytes() for path in dist.rglob('*') if path.is_file()})
                rows = cdn.inventory(dist, base)
                self.assertTrue(all('/bga/_common/' in row['url'] or '/bga/splendor/' in row['url'] for row in rows))
            finally:
                baseline_copy.unlink(missing_ok=True)


    def test_local_readback_binds_source_metadata_independently_of_resource_directory(self):
        with tempfile.TemporaryDirectory(dir=WORK) as name:
            dist = Path(name)
            (dist / 'splendor').mkdir()
            (dist / 'splendor/card.webp').write_bytes(b'generated resource')
            manifest = {'assetBase': 'https://cdn.ichenj.com/bga/', 'sourceSHA': 'a' * 40}
            (dist / 'game-load-manifest.json').write_text(json.dumps(manifest))
            (dist / 'index.html').write_text('entry')
            report = {'passed': True, 'stage': 'verified', 'cdnVerified': True, 'verificationLocation': 'local', 'files': cdn.inventory(dist, manifest['assetBase']), 'indexSHA256': cdn.sha((dist / 'index.html').read_bytes()), 'manifestSHA256': cdn.sha((dist / 'game-load-manifest.json').read_bytes())}
            proof = ready.local_readback_proof(dist, report, 'a' * 40, 123)
            self.assertEqual(proof['sourceSHA'], 'a' * 40)
            with self.assertRaises(ValueError):
                ready.local_readback_proof(dist, report, 'b' * 40, 123)


    def test_source_registry_identity_encoding_is_not_inferred_from_json_extension(self):
        value = {'key': 'bga/splendor/' + 'a' * 64 + '.json', 'url': 'https://cdn.ichenj.com/bga/splendor/' + 'a' * 64 + '.json', 'bytes': 10, 'sha256': 'a' * 64, 'mime': 'application/json'}
        self.assertEqual(reuse.normalized(value)['encoding'], '')


if __name__ == '__main__':
    result = unittest.TextTestRunner(verbosity=2).run(unittest.defaultTestLoader.loadTestsFromTestCase(CanonicalPublicationE2E))
    (WORK / 'result.json').write_text(json.dumps({'passed': result.wasSuccessful(), 'tests': result.testsRun}) + '\n')
    raise SystemExit(0 if result.wasSuccessful() else 1)
