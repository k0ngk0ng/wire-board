"""Before implementation: malformed/duplicate build stamps; changed profile;
bad artifact hash/size; immutable-version overwrite; concurrent retention;
path traversal/symlink escape; partial/corrupt retained artifacts. All temporary
files remain in the current workspace. No native statistics are changed.
"""
import concurrent.futures
import hashlib
import json
from pathlib import Path
import tempfile
import unittest

from runtime_archive import stamp_wasm, read_stamp, output_directory, retain_runtime

ROOT = Path(__file__).resolve().parents[3]
STAMP = {"format": 1, "profile": "original-98", "purpose": "campaign", "sourceCommit": "170accc3e9bd69c22eaffe27f4ac2288980b2869"}


class RuntimeArchiveTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(dir=ROOT / '.local/tmp')
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)

    def artifact(self, **changes):
        stamp = dict(STAMP, **changes)
        program = stamp_wasm(b'\0asm\1\0\0\0', stamp)
        folder = self.root / 'candidate'
        folder.mkdir(exist_ok=True)
        wasm, meta = folder / 'rules.wasm', folder / 'build.json'
        wasm.write_bytes(program)
        metadata = dict(runtimeSHA256=hashlib.sha256(program).hexdigest(), runtimeBytes=len(program), sourceCommit=stamp['sourceCommit'], testBattleHP=9999 if stamp['profile'] == 'development-health:9999' else None, rngFixture=None, battleFixture=None, referenceFont=False)
        meta.write_text(json.dumps(metadata))
        return wasm, meta

    def test_bounded_stamp_and_duplicate_rejected(self):
        wasm = stamp_wasm(b'\0asm\1\0\0\0', STAMP)
        self.assertEqual(read_stamp(wasm), STAMP)
        self.assertIsNone(read_stamp(b'\0asm\1\0\0\0'))
        with self.assertRaises(ValueError): stamp_wasm(wasm, STAMP)
        for value in [b'bad', wasm[:-1], b'\0asm\1\0\0\0\0\xff\xff\xff\xff\xff\xff']:
            with self.subTest(value=value[:20]):
                with self.assertRaises(ValueError): read_stamp(value)
        for changed in [dict(STAMP, profile='development-health:0'), dict(STAMP, profile='other-game'), dict(STAMP, purpose='unknown'), dict(STAMP, sourceCommit='bad'), dict(STAMP, unexpected=1)]:
            with self.subTest(stamp=changed):
                with self.assertRaises(ValueError): stamp_wasm(b'\0asm\1\0\0\0', changed)

    def test_output_stays_inside_workspace(self):
        self.assertEqual(output_directory(self.root, 'a/b'), self.root / 'a/b')
        for value in ['../outside', self.root.parent / 'outside']:
            with self.assertRaises(ValueError): output_directory(self.root, value)
        (self.root / 'escape').symlink_to(self.root.parent, target_is_directory=True)
        with self.assertRaises(ValueError): output_directory(self.root, 'escape/outside')

    def test_retains_exact_program_and_never_overwrites(self):
        wasm, meta = self.artifact()
        expected = wasm.read_bytes()
        target = retain_runtime(self.root, wasm, meta)
        self.assertEqual(target.name, hashlib.sha256(expected).hexdigest())
        self.assertEqual((target / 'rules.wasm').read_bytes(), expected)
        original_meta = (target / 'build.json').read_bytes()
        changed = json.loads(meta.read_text()); changed['commands'] = ['another identical rebuild']
        meta.write_text(json.dumps(changed))
        self.assertEqual(retain_runtime(self.root, wasm, meta), target)
        self.assertEqual((target / 'build.json').read_bytes(), original_meta)
        (target / 'rules.wasm').write_bytes(b'corrupt')
        with self.assertRaises(ValueError): retain_runtime(self.root, wasm, meta)
        self.assertEqual((target / 'rules.wasm').read_bytes(), b'corrupt')

    def test_wrong_identity_profile_and_partial_archive_rejected(self):
        for change in [dict(runtimeSHA256='0'*64), dict(runtimeBytes=1), dict(testBattleHP=9999), dict(sourceCommit='a'*40)]:
            with self.subTest(change=change):
                wasm, meta = self.artifact(); doc = json.loads(meta.read_text()); doc.update(change); meta.write_text(json.dumps(doc))
                with self.assertRaises(ValueError): retain_runtime(self.root, wasm, meta)
        wasm, meta = self.artifact(); target = self.root / '.local/pal/runtime-versions' / hashlib.sha256(wasm.read_bytes()).hexdigest()
        target.mkdir(parents=True)
        (target / 'rules.wasm').write_bytes(wasm.read_bytes())
        with self.assertRaises(ValueError): retain_runtime(self.root, wasm, meta)

    def test_concurrent_retention_is_one_immutable_version(self):
        wasm, meta = self.artifact(profile='development-health:9999')
        with concurrent.futures.ThreadPoolExecutor(max_workers=4) as pool:
            paths = list(pool.map(lambda _: retain_runtime(self.root, wasm, meta), range(12)))
        self.assertEqual(len(set(paths)), 1)
        self.assertEqual((paths[0] / 'rules.wasm').read_bytes(), wasm.read_bytes())
        self.assertEqual(list(paths[0].parent.glob('.staging-*')), [])

    def test_digest_directory_cannot_alias_another_workspace_path(self):
        wasm, meta = self.artifact()
        store = self.root / '.local/pal/runtime-versions'
        store.mkdir(parents=True)
        other = self.root / 'unrelated-folder'
        (store / hashlib.sha256(wasm.read_bytes()).hexdigest()).symlink_to(other, target_is_directory=True)
        with self.assertRaises(ValueError): retain_runtime(self.root, wasm, meta)
        self.assertFalse(other.exists())


if __name__ == '__main__':
    unittest.main()
