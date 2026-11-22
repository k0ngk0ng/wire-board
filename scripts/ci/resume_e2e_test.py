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
    def replay(self, change='ci', changed_command=False, failed_shared=False, ambiguous=False, recovered=False, newer_failure=False, newer_shared_failure=False, verified_baseline=False, skipped_shared=False):
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
def plan(paths,games):return {'has_shared':any(p.startswith('internal/') for p in paths),'frontend_checks':any(p.startswith('web/') for p in paths),'games':[{'game':g,'backend':any(p.startswith('games/'+g+'/') and p.endswith('.go') for p in paths)} for g in games if any(p.startswith('games/'+g+'/') for p in paths)]}
'''
            write('scripts/ci/scope.py', fixture)
            write('.github/workflows/build.yml', 'name: CI\njobs:\n  frontend:\n    run: frontend\n  shared:\n    run: shared\n  games:\n    run: games\n  verify:\n    needs: [games]\n    steps:\n      - run: build\n  complete:\n    run: complete\n')
            write('games/rail/server/rules.go', 'package rail\n')
            write('games/catan/server/rules.go', 'package catan\n')
            baseline = ''
            if verified_baseline:
                fixture = fixture.replace("['catan','rail']", "['catan','rail','untouched']")
                write('scripts/ci/scope.py', fixture)
                write('games/untouched/server/rules.go', 'package untouched\n')
                run('git', 'add', '.')
                run('git', 'commit', '-qm', 'verified ancestor')
                baseline = run('git', 'rev-parse', 'HEAD')
                write('games/rail/server/rules.go', 'package rail\nconst first=1\n')
                write('games/catan/server/rules.go', 'package catan\nconst first=1\n')
            run('git', 'add', '.')
            run('git', 'commit', '-qm', 'source validation')
            base = run('git', 'rev-parse', 'HEAD')
            write('scripts/ci/resume.py', (ROOT / 'scripts/ci/resume.py').read_text())
            write('scripts/ci/scope.py', fixture if recovered else fixture.replace("return [['go','test',game,'old']]", "return [['go','test',game,'new' if game=='catan' else 'old']]" if not changed_command else "return [['go','test',game,'old' if game=='shared' else 'new']]"))
            if change == 'business':write('games/rail/server/rules.go', 'package rail\nconst value=1\n')
            if change == 'frontend':write('games/rail/web/Board.tsx', 'export const Board=()=>null;\n')
            if change == 'frontend-test':write('web/tests/browser.test.mjs', 'export const fixture=1;\n')
            if change == 'public-frontend':write('web/src/ui/shared.tsx', 'export const fixture=1;\n')
            if change == 'shared':write('internal/server/rules.go', 'package server\n')
            if change == 'workflow':write('.github/workflows/build.yml', (root / '.github/workflows/build.yml').read_text().replace('run: games', 'run: changed'))
            run('git', 'add', '.')
            run('git', 'commit', '-qm', 'policy change')
            head = run('git', 'rev-parse', 'HEAD')
            jobs = [{'name': 'Build application assets', 'conclusion': 'success', 'id': 1}, {'name': 'Shared platform HTTP, persistence and race checks', 'conclusion': 'failure' if failed_shared else 'success', 'id': 2}, {'name': 'Game rail', 'conclusion': 'success', 'id': 3}, {'name': 'Game catan', 'conclusion': 'cancelled', 'id': 4}]
            if skipped_shared:jobs[1]['conclusion'] = 'skipped'
            if ambiguous:jobs.append(dict(jobs[2], id=5))
            data = {'workflow_runs': [{'id': 123, 'head_sha': base, 'head_branch': 'main', 'event': 'push', 'path': '.github/workflows/build.yml', 'status': 'completed', 'conclusion': 'cancelled'}], 'jobs': {'123': jobs}}
            intermediate = head
            if recovered:
                jobs[-1]['conclusion'] = 'success'
                latest = [{'name': 'Build application assets', 'conclusion': 'success', 'id': 10}, {'name': 'Shared platform HTTP, persistence and race checks', 'conclusion': 'failure' if newer_shared_failure else 'skipped', 'id': 11}, {'name': 'Game catan', 'conclusion': 'failure' if newer_failure else 'success', 'id': 12}]
                data['workflow_runs'].insert(0, dict(data['workflow_runs'][0], id=124, head_sha=intermediate, conclusion='failure'))
                data['jobs']['124'] = latest
                write('scripts/ci/fix.txt', 'candidate repair\n')
                if change == 'build-step':write('.github/workflows/build.yml', (root / '.github/workflows/build.yml').read_text().replace('run: build', 'run: fixed-build'))
                if change == 'build-gate':write('.github/workflows/build.yml', (root / '.github/workflows/build.yml').read_text().replace('needs: [games]', 'needs: []'))
                run('git', 'add', '.')
                run('git', 'commit', '-qm', 'repair publication only')
                head = run('git', 'rev-parse', 'HEAD')
            write('.local/fixture.json', json.dumps(data))
            binary = root / '.local/bin'
            binary.mkdir()
            driver = binary / 'gh'
            driver.write_text('''#!/usr/bin/env python3
import json,os,re,sys
from pathlib import Path
data=json.loads(Path(os.environ['RESUME_FIXTURE']).read_text())
print(json.dumps({'jobs':data['jobs'][re.search(r'/runs/([0-9]+)/jobs',sys.argv[-1]).group(1)]} if '/jobs' in sys.argv[-1] else {'workflow_runs':data['workflow_runs']}))
''')
            driver.chmod(0o755)
            env = dict(os.environ, PATH=str(binary) + os.pathsep + os.environ['PATH'], RESUME_FIXTURE=str(root / '.local/fixture.json'))
            arguments = ['python3', '-B', 'scripts/ci/resume.py', '--repo', 'fixture/repo', '--head', head, '--out', '.local/resume.json']
            if baseline: arguments += ['--baseline', baseline]
            result = subprocess.run(arguments, cwd=root, env=env, capture_output=True, text=True)
            self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
            report = json.loads((root / '.local/resume.json').read_text())
            eligible = change in ['ci', 'business', 'frontend', 'frontend-test', 'build-step'] and not failed_shared and not ambiguous and not newer_shared_failure and (not skipped_shared or verified_baseline)
            self.assertEqual(report['eligible'], eligible)
            if eligible:
                self.assertEqual(report['head'], head)
                self.assertEqual(report['base'], intermediate if recovered else base)
                self.assertEqual(report['sourceRun'], 124 if recovered else 123)
                if verified_baseline:
                    self.assertEqual(report['verifiedBase'], baseline)
                    self.assertEqual(report['requiredGames'], ['catan', 'rail'])
                    import importlib.util
                    selection = importlib.util.spec_from_file_location('verified_selection', ROOT / 'scripts/ci/scope.py')
                    selected = importlib.util.module_from_spec(selection)
                    selection.loader.exec_module(selected)
                    selected.ROOT = root
                    self.assertEqual(selected.resume_selection(report, head, base, ['catan', 'rail', 'untouched']), ['catan', 'rail'] if change == 'frontend' and changed_command else ['catan'])
                    broken = dict(report, requiredGames=['catan'])
                    with self.assertRaises(ValueError):
                        selected.resume_selection(broken, head, base, ['catan', 'rail', 'untouched'])
                if recovered:
                    expected = ['rail'] if change == 'business' else ['catan'] if newer_failure else []
                    self.assertEqual(report['pendingGames'], expected)
                    sources = {row['game']: row['sourceRun'] for row in report['reusedGames']}
                    if 'catan' not in expected:self.assertEqual(sources['catan'], 124)
                    if 'rail' not in expected:self.assertEqual(sources['rail'], 123)
                    return
                all_pending = changed_command or change in ['business', 'frontend']
                self.assertEqual(report['pendingGames'], ['catan', 'rail'] if all_pending else ['catan'])
                self.assertEqual([r['game'] for r in report['reusedGames']], [] if all_pending else ['rail'])

    def test_reuses_only_successful_games_with_identical_inputs(self):
        self.replay()

    def test_changed_commands_remain_required(self):
        self.replay(changed_command=True)

    def test_public_test_changes_preserve_games_but_public_source_changes_do_not(self):
        self.replay(change='frontend-test')
        self.replay(change='public-frontend')

    def test_changed_game_remains_required_and_shared_changes_cannot_reuse(self):
        self.replay(change='business')
        self.replay(change='shared')
        self.replay(change='workflow')

    def test_missing_success_or_ambiguous_job_fails_closed(self):
        self.replay(failed_shared=True)
        self.replay(ambiguous=True)

    def test_completed_games_from_later_failed_publication_are_bound_individually(self):
        self.replay(recovered=True)
        self.replay(recovered=True, change='business')

    def test_publication_steps_can_change_but_build_dependencies_cannot(self):
        self.replay(recovered=True, change='build-step')
        self.replay(recovered=True, change='build-gate')

    def test_a_newer_failure_cannot_be_hidden_by_an_older_success(self):
        self.replay(recovered=True, newer_failure=True)
        self.replay(recovered=True, newer_shared_failure=True)

    def test_verified_baseline_reuses_passed_games_without_scheduling_unaffected_games(self):
        self.replay(verified_baseline=True)

    def test_verified_baseline_allows_unselected_shared_checks_but_never_hides_a_shared_failure(self):
        self.replay(verified_baseline=True, skipped_shared=True)
        self.replay(skipped_shared=True)
        self.replay(verified_baseline=True, failed_shared=True)

    def test_changed_frontend_keeps_frontend_scope_when_its_previous_game_result_cannot_be_reused(self):
        self.replay(verified_baseline=True, skipped_shared=True, change='frontend')
        self.replay(verified_baseline=True, skipped_shared=True, change='frontend', changed_command=True)

    def test_workflow_attempts_partial_reuse_when_a_verified_ancestor_exists(self):
        workflow = (ROOT / '.github/workflows/build.yml').read_text()
        step = workflow.split('      - name: Verify reusable unchanged game inputs', 1)[1].split('      - name:', 1)[0]
        self.assertNotIn("steps.baseline.outputs.sha == ''", step)
        self.assertIn('--baseline', step)


if __name__ == '__main__':
    result = unittest.TextTestRunner(verbosity=2).run(unittest.defaultTestLoader.loadTestsFromTestCase(ResumeE2E))
    (WORK / 'result.json').write_text(json.dumps({'passed': result.wasSuccessful(), 'tests': result.testsRun, 'failures': len(result.failures), 'errors': len(result.errors)}, indent=2) + '\n')
    raise SystemExit(0 if result.wasSuccessful() else 1)
