import importlib.util
import json
from pathlib import Path
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[2]
WORK = ROOT / '.local/resource-cleanup/game-tools'
WORK.mkdir(parents=True, exist_ok=True)
spec = importlib.util.spec_from_file_location('game_tools', Path(__file__).with_name('game_tools.py'))
game_tools = importlib.util.module_from_spec(spec)
spec.loader.exec_module(game_tools)


class GameToolsE2E(unittest.TestCase):
    def test_game_specific_commands_live_in_their_game(self):
        self.assertEqual(game_tools.check(ROOT), [])

    def test_platform_registry_is_allowed_and_game_implementation_is_rejected(self):
        with tempfile.TemporaryDirectory(dir=WORK) as folder:
            root = Path(folder)
            platform = root / 'cmd/server/main.go'
            platform.parent.mkdir(parents=True)
            platform.write_text('package main\nimport "github.com/k0ngk0ng/wire-board/games"\n')
            misplaced = root / 'cmd/verify-game/main.go'
            misplaced.parent.mkdir(parents=True)
            misplaced.write_text('package main\nimport (\n game "github.com/k0ngk0ng/wire-board/games/yuyuhakusho/engine"\n)\n')
            owned = root / 'games/yuyuhakusho/cmd/verify/main.go'
            owned.parent.mkdir(parents=True)
            owned.write_text(misplaced.read_text())
            self.assertEqual(game_tools.check(root), [{'file': 'cmd/verify-game/main.go', 'game': 'yuyuhakusho', 'rule': 'game-tool-outside-game'}])


if __name__ == '__main__':
    result = unittest.TextTestRunner(verbosity=2).run(unittest.defaultTestLoader.loadTestsFromTestCase(GameToolsE2E))
    (WORK / 'result.json').write_text(json.dumps({'passed': result.wasSuccessful(), 'tests': result.testsRun, 'failures': len(result.failures), 'errors': len(result.errors)}, indent=2) + '\n')
    raise SystemExit(0 if result.wasSuccessful() else 1)
