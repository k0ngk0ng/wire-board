import json
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[2]
WORK = ROOT / '.local/ci-traffic'
WORK.mkdir(parents=True, exist_ok=True)


class TrafficLintE2E(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory(dir=WORK)
        self.addCleanup(self.temporary.cleanup)
        self.root = Path(self.temporary.name)
        shutil.copytree(ROOT / '.github', self.root / '.github')
        for name in ['scripts/ci/resources.py', 'scripts/ci/traffic-policy.json', 'scripts/cdn_delivery.py', 'scripts/resource_store.py']:
            source = ROOT / name
            if source.exists():
                target = self.root / name
                target.parent.mkdir(parents=True, exist_ok=True)
                shutil.copyfile(source, target)

    def invoke(self):
        return subprocess.run([sys.executable, '-B', str(ROOT / 'scripts/ci/traffic_lint.py'), '--root', str(self.root)], cwd=ROOT, capture_output=True, text=True)

    def test_real_workflows_pass_the_enforced_policy(self):
        result = self.invoke()
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)

    def test_normal_workflow_cannot_reintroduce_direct_hydration(self):
        path = self.root / '.github/workflows/build.yml'
        path.write_text(path.read_text() + '\n  expensive:\n    runs-on: ubuntu-latest\n    steps:\n      - run: python3 -B scripts/resource_store.py hydrate --oss\n')
        result = self.invoke()
        self.assertNotEqual(result.returncode, 0)
        self.assertIn('direct-resource-download', result.stdout)

    def test_paid_downloads_cannot_be_moved_to_cdn_curl(self):
        path = self.root / '.github/workflows/build.yml'
        path.write_text(path.read_text() + '\n  expensive:\n    runs-on: ubuntu-latest\n    steps:\n      - run: curl --fail "$CDN_ORIGIN/bga/example.bin"\n')
        result = self.invoke()
        self.assertNotEqual(result.returncode, 0)
        self.assertIn('direct-resource-download', result.stdout)

    def test_cache_miss_cannot_be_ignored_or_given_paid_credentials(self):
        path = self.root / '.github/actions/restore-resources/action.yml'
        path.write_text(path.read_text().replace('shell: bash', 'continue-on-error: true\n      shell: bash', 1))
        result = self.invoke()
        self.assertNotEqual(result.returncode, 0)
        self.assertIn('cache-failure', result.stdout)

    def test_cache_preparation_cannot_be_triggered_automatically(self):
        path = self.root / '.github/workflows/resource-cache.yml'
        path.write_text(path.read_text().replace('  workflow_dispatch:', '  push:\n  workflow_dispatch:', 1))
        result = self.invoke()
        self.assertNotEqual(result.returncode, 0)
        self.assertIn('manual-cache-only', result.stdout)

    def test_nonzero_routine_budget_and_publisher_get_are_rejected(self):
        path = self.root / 'scripts/ci/traffic-policy.json'
        policy = json.loads(path.read_text())
        policy['normalDownloadBytes'] = 1
        path.write_text(json.dumps(policy))
        result = self.invoke()
        self.assertNotEqual(result.returncode, 0)
        self.assertIn('zero-routine-budget', result.stdout)
        policy['normalDownloadBytes'] = 0
        path.write_text(json.dumps(policy))
        path = self.root / 'scripts/cdn_delivery.py'
        path.write_text(path.read_text().replace("canonical=f'HEAD", "canonical=f'GET", 1))
        result = self.invoke()
        self.assertNotEqual(result.returncode, 0)
        self.assertIn('publication-get', result.stdout)


if __name__ == '__main__':
    result = unittest.TextTestRunner(verbosity=2).run(unittest.defaultTestLoader.loadTestsFromTestCase(TrafficLintE2E))
    (WORK / 'lint-e2e.json').write_text(json.dumps({'passed': result.wasSuccessful(), 'tests': result.testsRun, 'failures': len(result.failures), 'errors': len(result.errors)}, indent=2) + '\n')
    raise SystemExit(0 if result.wasSuccessful() else 1)
