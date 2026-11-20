import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[2]
WORK = ROOT / '.local/resource-cleanup/ci-resume'
WORK.mkdir(parents=True, exist_ok=True)


class ResumeE2E(unittest.TestCase):
    def replay(self, change='ci', changed_command=False, failed_shared=False, ambiguous=False):
        with tempfile.TemporaryDirectory(dir=WORK) as folder:
            root = Path(folder)
            def run(*args):
                return subprocess.check_output(args, cwd=root, text=True).strip()
            def write(name, text):
                path = root / name
                path.parent.mkdir(parents=True, exist_ok=True)
                path.write_text(text)
            run('git', 'init', '-q')
            run('git', 'config', 'user.name', 'CI fixture')
            run('git', 'config', 'user.email', 'ci@example.invalid')
            write('.gitignore', '.local/\n')
            fixture = '''from pathlib import Path
ROOT=Path(__file__).resolve().parents[2]
GAME_PARTITIONS={'catan':(4,20)}
def game_names(root):return ['catan','rail']
def ci_path(path):return path.startswith(('scripts/ci/','.github/workflows/'))
def go_commands(root,game,**kwargs):return [['go','test',game,'old']]
def frontend_tests(root,game):return []
def build_commands(root,game,index=0):return []
def verification_env(game,backend,base=None):return {}
def plan(paths,games):return {'has_shared':any(p.startswith('internal/') for p in paths),'frontend_checks':False,'games':[{'game':g,'backend':True} for g in games if any(p.startswith('games/'+g+'/') for p in paths)]}
'''
            write('scripts/ci/scope.py', fixture)
            write('.github/workflows/build.yml', 'name: CI\njobs:\n  frontend:\n    run: frontend\n  shared:\n    run: shared\n  games:\n    run: games\n  verify:\n    run: build\n  complete:\n    run: complete\n')
            write('games/rail/server/rules.go', 'package rail\n')
            write('games/catan/server/rules.go', 'package catan\n')
            run('git', 'add', '.')
            run('git', 'commit', '-qm', 'source validation')
            base = run('git', 'rev-parse', 'HEAD')
            write('scripts/ci/resume.py', (ROOT / 'scripts/ci/resume.py').read_text())
            write('scripts/ci/scope.py', fixture.replace("return [['go','test',game,'old']]", "return [['go','test',game,'new' if game=='catan' else 'old']]" if not changed_command else "return [['go','test',game,'old' if game=='shared' else 'new']]"))
            if change == 'business':write('games/rail/server/rules.go', 'package rail\nconst value=1\n')
            if change == 'shared':write('internal/server/rules.go', 'package server\n')
            if change == 'workflow':write('.github/workflows/build.yml', (root / '.github/workflows/build.yml').read_text().replace('run: games', 'run: changed'))
            run('git', 'add', '.')
            run('git', 'commit', '-qm', 'policy change')
            head = run('git', 'rev-parse', 'HEAD')
            jobs = [{'name': 'Build application assets', 'conclusion': 'success', 'id': 1}, {'name': 'Shared platform HTTP, persistence and race checks', 'conclusion': 'failure' if failed_shared else 'success', 'id': 2}, {'name': 'Game rail', 'conclusion': 'success', 'id': 3}, {'name': 'Game catan', 'conclusion': 'cancelled', 'id': 4}]
            if ambiguous:jobs.append(dict(jobs[2], id=5))
            data = {'workflow_runs': [{'id': 123, 'head_sha': base, 'head_branch': 'main', 'event': 'push', 'path': '.github/workflows/build.yml', 'status': 'completed', 'conclusion': 'cancelled'}], 'jobs': jobs}
            write('.local/fixture.json', json.dumps(data))
            binary = root / '.local/bin'
            binary.mkdir()
            driver = binary / 'gh'
            driver.write_text('''#!/usr/bin/env python3
import json,os,sys
from pathlib import Path
data=json.loads(Path(os.environ['RESUME_FIXTURE']).read_text())
print(json.dumps({'jobs':data['jobs']} if '/jobs' in sys.argv[-1] else {'workflow_runs':data['workflow_runs']}))
''')
            driver.chmod(0o755)
            env = dict(os.environ, PATH=str(binary) + os.pathsep + os.environ['PATH'], RESUME_FIXTURE=str(root / '.local/fixture.json'))
            result = subprocess.run(['python3', '-B', 'scripts/ci/resume.py', '--repo', 'fixture/repo', '--head', head, '--out', '.local/resume.json'], cwd=root, env=env, capture_output=True, text=True)
            self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
            report = json.loads((root / '.local/resume.json').read_text())
            eligible = change in ['ci', 'business'] and not failed_shared and not ambiguous
            self.assertEqual(report['eligible'], eligible)
            if eligible:
                self.assertEqual(report['head'], head)
                self.assertEqual(report['base'], base)
                self.assertEqual(report['sourceRun'], 123)
                all_pending = changed_command or change == 'business'
                self.assertEqual(report['pendingGames'], ['catan', 'rail'] if all_pending else ['catan'])
                self.assertEqual([r['game'] for r in report['reusedGames']], [] if all_pending else ['rail'])

    def test_reuses_only_successful_games_with_identical_inputs(self):
        self.replay()

    def test_changed_commands_remain_required(self):
        self.replay(changed_command=True)

    def test_changed_game_remains_required_and_shared_changes_cannot_reuse(self):
        self.replay(change='business')
        self.replay(change='shared')
        self.replay(change='workflow')

    def test_missing_success_or_ambiguous_job_fails_closed(self):
        self.replay(failed_shared=True)
        self.replay(ambiguous=True)


if __name__ == '__main__':
    result = unittest.TextTestRunner(verbosity=2).run(unittest.defaultTestLoader.loadTestsFromTestCase(ResumeE2E))
    (WORK / 'result.json').write_text(json.dumps({'passed': result.wasSuccessful(), 'tests': result.testsRun, 'failures': len(result.failures), 'errors': len(result.errors)}, indent=2) + '\n')
    raise SystemExit(0 if result.wasSuccessful() else 1)
