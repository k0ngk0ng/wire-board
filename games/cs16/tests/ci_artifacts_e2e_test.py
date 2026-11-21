import importlib.util
import json
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path
from types import SimpleNamespace
from unittest.mock import patch


ROOT = Path(__file__).resolve().parents[3]
WORK = ROOT / '.local/cs16/verification/ci-artifacts'
WORK.mkdir(parents=True, exist_ok=True)
SPEC = importlib.util.spec_from_file_location('cs16_ci_scope', ROOT / 'scripts/ci/scope.py')
SCOPE = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(SCOPE)


class Cs16ArtifactE2E(unittest.TestCase):
    def replay(self, game, status, evidence=True):
        with tempfile.TemporaryDirectory(dir=WORK, prefix='fixture-') as name:
            root = Path(name)
            program = '''import json,sys
from pathlib import Path
if sys.argv[2] == 'yes':
    output=Path.cwd()/'.local/cs16/verification/server'
    output.mkdir(parents=True,exist_ok=True)
    (output/'checkpoint').mkdir(parents=True,exist_ok=True)
    (output/'natural.json').write_text(json.dumps({'passed':False,'initialSeed':'18446744073709551615','privateCheckpoint':{'world':{'seed':9007199254740993}}}))
    (output/'checkpoint'/'fy_iceworld.json').write_text('checkpoint-bytes\\n')
    (output/'ignored.pprof').write_bytes(b'profile')
raise SystemExit(int(sys.argv[1]))
'''
            command = [sys.executable, '-B', '-c', program, str(status), 'yes' if evidence else 'no']
            args = SimpleNamespace(game=game, backend=True, shard=0, count=1)
            with patch.object(SCOPE, 'ROOT', root), patch.object(SCOPE, 'frontend_tests', return_value=[]), patch.object(SCOPE, 'go_commands', return_value=[command]), patch.object(SCOPE, 'build_commands', return_value=[]):
                if status:
                    with self.assertRaises(subprocess.CalledProcessError) as failure:
                        SCOPE.run(args)
                    self.assertEqual(failure.exception.returncode, status)
                else:
                    SCOPE.run(args)
            output = root / '.local/ci' / game
            result = json.loads((output / 'result-0.json').read_text())
            self.assertEqual(result['passed'], status == 0)
            source = root / '.local/cs16/verification/server'
            archived = output / 'server'
            if game == 'cs16' and evidence:
                self.assertEqual((archived / 'natural.json').read_bytes(), (source / 'natural.json').read_bytes())
                self.assertEqual((archived / 'checkpoint/fy_iceworld.json').read_bytes(), (source / 'checkpoint/fy_iceworld.json').read_bytes())
                self.assertFalse((archived / 'ignored.pprof').exists())
            else:
                self.assertFalse(archived.exists())

    def test_failed_cs16_command_retains_original_failure_and_checkpoints(self):
        self.replay('cs16', 7)

    def test_successful_cs16_command_collects_the_same_json_boundary(self):
        self.replay('cs16', 0)

    def test_missing_cs16_artifacts_do_not_create_false_failure(self):
        self.replay('cs16', 0, evidence=False)

    def test_non_cs16_lane_does_not_collect_cs16_artifacts(self):
        self.replay('shared', 7)


if __name__ == '__main__':
    result = unittest.TextTestRunner(verbosity=2).run(unittest.defaultTestLoader.loadTestsFromTestCase(Cs16ArtifactE2E))
    (WORK / 'summary.json').write_text(json.dumps({'passed': result.wasSuccessful(), 'tests': result.testsRun, 'failures': len(result.failures), 'errors': len(result.errors)}, indent=2) + '\n')
    raise SystemExit(0 if result.wasSuccessful() else 1)
