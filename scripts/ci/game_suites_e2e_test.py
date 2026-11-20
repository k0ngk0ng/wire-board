from collections import Counter
import importlib.util
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[2]
WORK = ROOT / '.local/resource-cleanup/ci-optimization'
WORK.mkdir(parents=True, exist_ok=True)
spec = importlib.util.spec_from_file_location('scope', Path(__file__).with_name('scope.py'))
scope = importlib.util.module_from_spec(spec)
spec.loader.exec_module(scope)


def identities(commands):
    found = Counter()
    for command in commands:
        pattern = command[command.index('-run') + 1]
        names = pattern[4:-2].split('|') if pattern.startswith('^(?:') else [pattern.split('/')[0].strip('^$')]
        found.update((command[-1], name) for name in names)
    return found


class GameSuitesE2E(unittest.TestCase):
    def test_only_declared_batches_leave_standard_and_full_restores_them(self):
        for game in ['basketball', 'starfort', 'yuyuhakusho']:
            deferred = set(scope.GAME_EXTENDED_TESTS[game])
            self.assertEqual(len(deferred), len(scope.GAME_EXTENDED_TESTS[game]))
            for race in [False, True]:
                full = identities(scope.go_commands(ROOT, game, race=race, extended=True))
                standard = identities(scope.go_commands(ROOT, game, race=race))
                self.assertEqual(standard, Counter({key: value for key, value in full.items() if key not in deferred}))
                if not race:
                    self.assertEqual(set(full) - set(standard), deferred)
            self.assertTrue(all('-timeout=20m' in command for command in scope.go_commands(ROOT, game)))
        basketball = identities(scope.go_commands(ROOT, 'basketball'))
        starfort = identities(scope.go_commands(ROOT, 'starfort'))
        self.assertIn(('./games/basketball/server', 'TestBasketballHTTPCompletePreparedMatch'), basketball)
        self.assertIn(('./games/starfort/tests', 'TestStarfortPublicNatural'), starfort)
        self.assertIn(('./games/starfort/tests', 'TestStarfortWorldEndlessForty'), starfort)
        for name in ['TestBasketballRealtimeStreamAndRestore', 'TestCampaignEndingConfirmationDeadline', 'TestRuntimePresentationDisplayLifecycle']:
            self.assertIn(name, ' '.join(' '.join(command) for command in scope.go_commands(ROOT, 'basketball', race=True)))

    def test_basketball_inherited_matrix_flags_require_full_but_full_http_stays_required(self):
        inherited = dict(BASKETBALL_PREPARATION_MATRIX='1', BASKETBALL_SELECTION_ADMISSION='1', BASKETBALL_FULL_HTTP='1')
        standard = scope.verification_env('basketball', True, inherited)
        self.assertEqual(standard['BASKETBALL_FULL_HTTP'], '1')
        self.assertNotIn('BASKETBALL_PREPARATION_MATRIX', standard)
        self.assertNotIn('BASKETBALL_SELECTION_ADMISSION', standard)
        full = scope.verification_env('basketball', True, inherited, extended=True)
        self.assertTrue(all(full.get(key) == '1' for key in inherited))

    def test_trajectory_subsets_cover_modes_and_unknown_cases_default_to_required(self):
        fixture = ROOT / 'games/yuyuhakusho/renderstate/testdata/bridge-reference.json'
        rows = json.loads(fixture.read_text())['rows']
        all_names = {row['configuration']['name'] for row in rows}
        deferred = set(scope.YUYU_EXTENDED_TRAJECTORIES)
        self.assertEqual(len(deferred), len(scope.YUYU_EXTENDED_TRAJECTORIES))
        for extended in [False, True]:
            commands = scope.go_commands(ROOT, 'yuyuhakusho', extended=extended)
            for parent in ['TestRenderFrozenBridgeTrajectories', 'TestSchema2FrozenWholeTrajectories']:
                command = next(c for c in commands if c[c.index('-run') + 1].startswith('^' + parent + '$'))
                pattern = command[command.index('-run') + 1]
                chosen = all_names if '/' not in pattern else {name for name in all_names if re.fullmatch(pattern.split('/', 1)[1], name)}
                self.assertEqual(chosen, all_names if extended else all_names - deferred)
                if not extended:
                    self.assertEqual(chosen, {'legacy-itsuki-chu', 'legacy-three', 'legacy-team', 'arena-a-pan', 'arena-e-four'})
        with tempfile.TemporaryDirectory(dir=WORK) as folder:
            root = Path(folder)
            target = root / fixture.relative_to(ROOT)
            target.parent.mkdir(parents=True)
            target.write_text(json.dumps({'rows': [*rows, {'configuration': {'name': 'new-unclassified-case'}}]}))
            self.assertIn('new-unclassified-case', scope.trajectory_cases(root, extended=False))

    def test_yuyuhakusho_actual_cli_partitions_preserve_failures_and_run_binding_once(self):
        for fail in [False, True]:
            with tempfile.TemporaryDirectory(dir=WORK) as folder:
                root = Path(folder)
                (root / 'scripts/ci').mkdir(parents=True)
                shutil.copyfile(Path(scope.__file__), root / 'scripts/ci/scope.py')
                source = root / 'games/yuyuhakusho/server/rules_test.go'
                source.parent.mkdir(parents=True)
                source.write_text('package server\nfunc TestConcurrentUnknownYuyuhakusho() {}\n')
                frontend = root / 'games/yuyuhakusho/tests/input.test.mjs'
                frontend.parent.mkdir(parents=True)
                frontend.write_text('')
                calls = root / 'calls'
                calls.mkdir()
                binary = root / 'bin'
                binary.mkdir()
                driver = '''#!/usr/bin/env python3
import json,os,sys,uuid
from pathlib import Path
(Path(os.environ['GAME_CALLS'])/(uuid.uuid4().hex+'.json')).write_text(json.dumps({'tool':Path(sys.argv[0]).name,'args':sys.argv[1:]}))
if os.environ.get('GAME_FAIL') and 'TestConcurrentUnknownYuyuhakusho' in ' '.join(sys.argv[1:]):sys.exit(1)
'''
                for name in ['go', 'node']:
                    path = binary / name
                    path.write_text(driver)
                    path.chmod(0o755)
                env = dict(os.environ, PATH=str(binary) + os.pathsep + os.environ['PATH'], GAME_CALLS=str(calls), GAME_FAIL='1' if fail else '')
                result = subprocess.run(['python3', '-B', 'scripts/ci/scope.py', 'run', '--game', 'yuyuhakusho', '--backend'], cwd=root, env=env, capture_output=True, text=True)
                self.assertEqual(result.returncode == 0, not fail, result.stdout + result.stderr)
                aggregate = json.loads((root / '.local/ci/yuyuhakusho/partitions.json').read_text())
                self.assertEqual(aggregate['suite'], 'standard')
                self.assertEqual(aggregate['partitions'], 4)
                self.assertEqual(aggregate['workers'], 2)
                self.assertEqual(aggregate['passed'], not fail)
                if not fail:
                    observed = [json.loads(path.read_text()) for path in calls.glob('*.json')]
                    self.assertEqual(sum(row['tool'] == 'node' for row in observed), 2)
                    self.assertEqual(sum('runtime-delivery.test.mjs' in ' '.join(row['args']) for row in observed), 1)
                    self.assertEqual(sum(row['tool'] == 'go' for row in observed), 2)

    def test_full_entry_and_standard_budget_are_explicit(self):
        workflow = (ROOT / '.github/workflows/games-extended.yml').read_text()
        self.assertIn('workflow_dispatch:', workflow)
        self.assertNotIn('continue-on-error', workflow)
        for game in ['basketball', 'catan', 'starfort', 'yuyuhakusho']:
            self.assertIn('- ' + game, workflow)
        self.assertIn('scope.py run --game "$GAME_ID" --backend --extended', workflow)
        main = (ROOT / '.github/workflows/build.yml').read_text()
        self.assertIn('args+=(--extended)', main)
        self.assertIn('&& 30 || 180', main)


if __name__ == '__main__':
    result = unittest.TextTestRunner(verbosity=2).run(unittest.defaultTestLoader.loadTestsFromTestCase(GameSuitesE2E))
    (WORK / 'suites-e2e.json').write_text(json.dumps({'passed': result.wasSuccessful(), 'tests': result.testsRun, 'failures': len(result.failures), 'errors': len(result.errors)}, indent=2) + '\n')
    raise SystemExit(0 if result.wasSuccessful() else 1)
