from pathlib import Path
import tempfile
import unittest

from local_paths import original_path

ROOT = Path(__file__).resolve().parents[2]


class OriginalPaths(unittest.TestCase):
    def test_recorded_relative_and_absolute_paths_resolve_without_rewriting_evidence(self):
        source = Path('.local') / 'nes-basket' / 'runs' / 'selection-names-court-0'
        expected = ROOT / '.local/basketball/research/original/runs/selection-names-court-0'
        self.assertEqual(original_path(ROOT, source), expected)
        self.assertEqual(original_path(ROOT, ROOT / source), expected)
        self.assertEqual(original_path(ROOT, expected), expected)

    def test_outside_paths_and_traversal_are_rejected(self):
        for source in [ROOT.parent / 'outside', Path('..') / 'outside']:
            with self.subTest(source=source), self.assertRaises(ValueError):
                original_path(ROOT, source)

    def test_symlink_cannot_leave_the_workspace(self):
        parent = ROOT / '.local/basketball/tmp/path-tests'
        parent.mkdir(parents=True, exist_ok=True)
        with tempfile.TemporaryDirectory(dir=parent) as directory:
            link = Path(directory) / 'outside'
            link.symlink_to(ROOT.parent, target_is_directory=True)
            with self.assertRaises(ValueError):
                original_path(ROOT, link / 'outside')


if __name__ == '__main__':
    unittest.main()
