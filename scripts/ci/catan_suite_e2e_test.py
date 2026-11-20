from collections import Counter
import importlib.util
import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[2]
WORK = ROOT / '.local/catan/ci-tiers'
WORK.mkdir(parents=True, exist_ok=True)
spec = importlib.util.spec_from_file_location('scope', Path(__file__).with_name('scope.py'))
scope = importlib.util.module_from_spec(spec)
spec.loader.exec_module(scope)


def selected(commands):
    result = Counter()
    for command in commands:
        pattern = command[command.index('-run') + 1]
        names = pattern[4:-2].split('|') if pattern.startswith('^(?:') else [pattern]
        result.update((command[-1], name) for name in names)
    return result


class CatanSuiteE2E(unittest.TestCase):
    def test_nested_acceptance_requires_explicit_full_mode(self):
        inherited = {'WIRE_BOARD_NESTING_ACCEPTANCE': '1'}
        self.assertNotIn('WIRE_BOARD_NESTING_ACCEPTANCE', scope.verification_env('catan', True, inherited))
        self.assertEqual(scope.verification_env('catan', True, inherited, extended=True)['WIRE_BOARD_NESTING_ACCEPTANCE'], '1')
        self.assertNotIn('WIRE_BOARD_NESTING_ACCEPTANCE', scope.verification_env('catan', False, inherited, extended=True))

    def test_ci_policy_changes_skip_game_fanout_and_app_packaging(self):
        result = scope.plan(['scripts/ci/catan_suite_e2e_test.py', 'scripts/ci/scope.py', '.github/workflows/catan-extended.yml', 'games/racing/build/asset-reuse-lint.json'], scope.game_names(ROOT))
        self.assertEqual(result['games'], [])
        self.assertEqual(result['quick_components'], ['ci'])
        self.assertFalse(result['build_required'])
        self.assertFalse(result['has_shared'])

    def test_real_inventory_retains_contracts_and_defers_only_registered_batches(self):
        deferred = set(scope.CATAN_EXTENDED_TESTS)
        self.assertEqual(len(deferred), len(scope.CATAN_EXTENDED_TESTS))
        self.assertTrue(deferred)
        for race in [False, True]:
            full = selected(scope.go_commands(ROOT, 'catan', race=race, extended=True))
            standard = selected(scope.go_commands(ROOT, 'catan', race=race))
            expected = Counter({key: count for key, count in full.items() if key not in deferred and (not race or any(word in key[1] for word in ['Concurrent', 'Spectator']))})
            self.assertEqual(standard, expected)
            self.assertTrue(all(value == 1 for value in full.values()))
            if not race:
                self.assertEqual(set(full) - set(standard), deferred)
                names = {key[1] for key in standard}
                for name in ['TestCatanBotsFinishAndPreservePrivacy', 'TestCatanExplorerNaturalMatches', 'TestCatanHelpersBotsCompleteAndConserveResources', 'TestCatanTwoCompleteEngineGames', 'TestCatanTwoKnightsCompleteEngineGames', 'TestCatanPairedSaveRestoresMarkersAndBotsFinish', 'TestCatanFishingNewWorldSetupOrderRestoreAndPrivacy', 'TestCatanExplorerEventsNewYearAlchemyAndPrivateDeck', 'TestCatanExplorerCityBankCardRatesAndConservation', 'TestCatanExplorerCityDepartureReturnsCityAndExplorerHoldings', 'TestCatanFishingHelpersKnightsConfigurations', 'TestCatanTransportSeaModulesAdmission', 'TestCatanExplorerPairedFormalSetupAndRotation', 'TestCatanCaravansAttackEndTimeoutRestart', 'TestCatanTwoCompleteHTTPGames', 'TestCatanExplorerFishNaturalHTTPAutoplayMatch']:
                    self.assertIn(name, names)
                self.assertIn(('./internal/server', 'TestCatanCaravansAttackNaturalHTTP'), deferred)
                self.assertIn(('./internal/server', 'TestCatanAttackShoresHTTP'), deferred)
                self.assertIn(('./internal/server', 'TestCatanSeaNestingsE2E'), deferred)
                self.assertFalse(any(any(word in name for word in ['Timeout', 'Privacy', 'Clock']) for package, name in deferred))
        for game in ['rail', 'shared']:
            self.assertEqual(scope.go_commands(ROOT, game), scope.go_commands(ROOT, game, extended=True))

    def replay(self, fail=False):
        with tempfile.TemporaryDirectory(dir=WORK) as folder:
            root = Path(folder)
            (root / 'scripts/ci').mkdir(parents=True)
            shutil.copyfile(Path(scope.__file__), root / 'scripts/ci/scope.py')
            package, heavy = scope.CATAN_EXTENDED_TESTS[0]
            source = root / package.removeprefix('./') / 'rules_test.go'
            source.parent.mkdir(parents=True, exist_ok=True)
            names = [heavy, 'TestCatanConcurrentUnknownRegression', 'TestCatanRequiredPrivacy']
            source.write_text('package server\n' + '\n'.join('func ' + name + '() {}' for name in names))
            frontend = root / 'games/catan/tests/input.test.mjs'
            frontend.parent.mkdir(parents=True, exist_ok=True)
            frontend.write_text('')
            calls = root / 'calls'
            calls.mkdir()
            tools = root / 'bin'
            tools.mkdir()
            driver = '''#!/usr/bin/env python3
import json,os,sys,uuid
from pathlib import Path
(Path(os.environ['CATAN_CALLS'])/(uuid.uuid4().hex+'.json')).write_text(json.dumps({'tool':Path(sys.argv[0]).name,'args':sys.argv[1:],'nesting':os.environ.get('WIRE_BOARD_NESTING_ACCEPTANCE')}))
if os.environ.get('CATAN_FAIL') and os.environ['CATAN_FAIL'] in ' '.join(sys.argv[1:]):sys.exit(1)
'''
            for name in ['go', 'node']:
                path = tools / name
                path.write_text(driver)
                path.chmod(0o755)
            env = dict(os.environ, PATH=str(tools) + os.pathsep + os.environ['PATH'], CATAN_CALLS=str(calls), CATAN_FAIL=names[1] if fail else '', WIRE_BOARD_NESTING_ACCEPTANCE='1')
            result = subprocess.run(['python3', '-B', 'scripts/ci/scope.py', 'run', '--game', 'catan', '--backend'], cwd=root, env=env, capture_output=True, text=True)
            self.assertEqual(result.returncode == 0, not fail, result.stdout + result.stderr)
            receipt = json.loads((root / '.local/ci/catan/result-0.json').read_text())
            aggregate = json.loads((root / '.local/ci/catan/partitions.json').read_text())
            self.assertEqual(aggregate['partitions'], 4)
            self.assertEqual(aggregate['workers'], 2)
            self.assertEqual(aggregate['suite'], 'standard')
            self.assertEqual(aggregate['passed'], not fail)
            self.assertEqual(receipt['suite'], 'standard')
            self.assertEqual(receipt['deferredTests'], [{'package': package, 'name': heavy}])
            rows = [json.loads(path.read_text()) for path in calls.glob('*.json')]
            self.assertTrue(all(row['nesting'] is None for row in rows))
            self.assertFalse(any(heavy in ' '.join(row['args']) for row in rows))
            self.assertTrue(any(names[1] in ' '.join(row['args']) for row in rows))
            if not fail:
                self.assertEqual(sum(row['tool'] == 'node' for row in rows), 1)
                self.assertTrue(any('-race' in row['args'] for row in rows))

    def test_standard_cli_uses_real_processes_and_records_deferred_scope(self):
        self.replay()

    def test_required_test_failure_stays_failed(self):
        self.replay(fail=True)

    def test_full_workflow_is_explicit_and_preserves_failure_evidence(self):
        workflow = (ROOT / '.github/workflows/catan-extended.yml').read_text()
        self.assertIn('workflow_dispatch:', workflow)
        self.assertNotIn('continue-on-error', workflow)
        self.assertNotIn('schedule:', workflow)
        self.assertIn('scope.py run --game catan --backend --catan-extended', workflow)
        self.assertIn('resource_store.py hydrate --oss --game catan', workflow)
        self.assertIn('if: always()', workflow)
        build = (ROOT / '.github/workflows/build.yml').read_text()
        self.assertIn('FULL_VALIDATION: ${{ inputs.validation == \'full\' }}', build)
        self.assertIn('args+=(--extended)', build)


if __name__ == '__main__':
    result = unittest.TextTestRunner(verbosity=2).run(unittest.defaultTestLoader.loadTestsFromTestCase(CatanSuiteE2E))
    (WORK / 'suite-e2e.json').write_text(json.dumps({'passed': result.wasSuccessful(), 'tests': result.testsRun, 'failures': len(result.failures), 'errors': len(result.errors)}, indent=2) + '\n')
    raise SystemExit(0 if result.wasSuccessful() else 1)
