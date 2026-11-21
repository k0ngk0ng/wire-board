import json
from pathlib import Path
import subprocess
import sys
import unittest

ROOT = Path(__file__).resolve().parents[1]
CHECKER = ROOT / 'scripts/check_local_game_paths.py'


class LocalGamePathTests(unittest.TestCase):
    def check_source(self, language, text):
        return subprocess.run([sys.executable, '-B', str(CHECKER), '--stdin', language], input=text, capture_output=True, text=True, cwd=ROOT)

    def test_each_language_accepts_canonical_game_paths(self):
        for language in ['python', 'javascript', 'typescript', 'go', 'shell', 'lua', 'html']:
            with self.subTest(language=language):
                result = self.check_source(language, '".local/basketball/verification/result.json"')
                self.assertEqual(result.returncode, 0, result.stderr)
                self.assertEqual(json.loads(result.stdout)['issues'], [])

    def test_each_language_rejects_alias_and_task_directories(self):
        for language in ['python', 'javascript', 'typescript', 'go', 'shell', 'lua', 'html']:
            for directory in ['basketball-cover', 'basketball-release-v1.26.0', 'nes-basket']:
                with self.subTest(language=language, directory=directory):
                    result = self.check_source(language, '"' + '.local/' + directory + '/result.json"')
                    self.assertEqual(result.returncode, 1, result.stderr)
                    self.assertEqual(len(json.loads(result.stdout)['issues']), 1)

    def test_shared_tools_and_unrelated_games_are_allowed(self):
        for directory in ['tools/nes-research', 'contra/research', 'basketball']:
            result = self.check_source('python', '"' + '.local/' + directory + '"')
            self.assertEqual(result.returncode, 0, result.stderr)

    def test_game_files_require_a_purpose_directory(self):
        for language in ['python', 'javascript', 'typescript', 'go', 'shell', 'lua', 'html', 'css']:
            result = self.check_source(language, '"' + '.local/' + 'basketball/live-browser/result.json"')
            self.assertEqual(result.returncode, 1, language)
            result = self.check_source(language, '".local/basketball/verification/live-browser/result.json"')
            self.assertEqual(result.returncode, 0, language)

    def test_unsupported_language_is_rejected(self):
        result = self.check_source('unknown', '"' + '.local/' + 'basketball/result.json"')
        self.assertEqual(result.returncode, 2)

    def test_split_path_arguments_are_checked(self):
        legacy = 'filepath.Join(' + ', '.join(['root', json.dumps('.local'), json.dumps('basketball'), json.dumps('live-browser')]) + ')'
        canonical = 'filepath.Join(root, ".local", "basketball", "verification", "live-browser")'
        self.assertEqual(self.check_source('go', legacy).returncode, 1)
        self.assertEqual(self.check_source('go', canonical).returncode, 0)


if __name__ == '__main__':
    unittest.main()
