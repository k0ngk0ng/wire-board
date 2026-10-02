#!/usr/bin/env python3
"""Exercise update failure paths without Docker or root; all fixtures stay local."""
import os
import json
from pathlib import Path
import shutil
import sqlite3
import subprocess
import tarfile
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[1]
DIGEST = "ghcr.io/k0ngk0ng/wire-board@sha256:" + "a" * 64
MOCK_DOCKER = r'''#!/usr/bin/env python3
import json, os, pathlib, subprocess, sys
p = pathlib.Path(os.environ['FIXTURE'])
a = sys.argv[1:]
with (p / 'calls').open('a') as f: f.write(json.dumps(a) + '\n')
scenario = os.environ['SCENARIO']
if scenario == 'clean':
    repo = 'ghcr.io/k0ngk0ng/wire-board'
    if a == ['ps', '-aq']: print('container-id'); sys.exit(0)
    if a[:2] == ['image', 'ls']:
        print('sha256:current\nsha256:rollback\nsha256:old\nsha256:dangling\nsha256:foreign\nsha256:old-image'); sys.exit(0)
    if a[:2] == ['image', 'inspect']:
        fmt, ref = a[3], a[4]
        if fmt == '{{.Id}}': print('sha256:current')
        elif 'RepoDigests' in fmt: pass
        elif ref == 'sha256:foreign': print('other-service:latest')
        elif ref == 'sha256:old': print(repo + ':v0.0.1')
        sys.exit(0)
    if a[:2] == ['image', 'rm']: sys.exit(0)
if a[0] == 'pull':
    sys.exit(1 if scenario == 'pull-failure' else 0)
if a[:2] == ['image', 'inspect']:
    print(os.environ['DIGEST']); sys.exit(0)
if a[0] == 'run':
    mounts = [a[i+1] for i, value in enumerate(a) if value == '--mount']
    source = pathlib.Path(next(x.split('source=')[1].split(',target=')[0] for x in mounts if 'target=/source,' in x))
    snapshot = pathlib.Path(next(x.split('source=')[1].split(',target=')[0] for x in mounts if 'target=/snapshot' in x))
    if scenario == 'backup-failure':
        (snapshot / 'wire-board.db').write_text('incomplete')
        sys.exit(1)
    # Exercise the same real SQLite CLI backup API with WAL, without Docker.
    subprocess.run(['sqlite3', '-readonly', str(source / 'wire-board.db'), '.timeout 5000', '.backup ' + str(snapshot / 'wire-board.db')], check=True)
    subprocess.run(['sqlite3', str(snapshot / 'wire-board.db'), 'PRAGMA journal_mode=DELETE;'], check=True, stdout=subprocess.DEVNULL)
    check = subprocess.check_output(['sqlite3', '-readonly', str(snapshot / 'wire-board.db'), 'PRAGMA quick_check;'], text=True)
    sys.exit(0 if check.strip() == 'ok' else 1)
if a[0] == 'inspect':
    fmt = a[a.index('--format') + 1]
    if '.Mounts' in fmt: print(p / 'volume')
    elif fmt == '{{.Image}}': print('sha256:new-image' if (p / 'started').exists() else 'sha256:old-image')
    else: print('healthy')
    sys.exit(0)
if a[0] == 'compose':
    if 'version' in a: sys.exit(0)
    if 'config' in a: sys.exit(1 if scenario == 'config-failure' else 0)
    if 'ps' in a:
        if scenario != 'initial' or (p / 'started').exists(): print('container-id')
    if 'up' in a:
        if scenario == 'upgrade':
            assert list((p / 'backups').glob('*.tar.gz')), 'compression was not completed before switch'
            subprocess.run(['sqlite3', str(p / 'volume/wire-board.db'), "INSERT INTO state VALUES ('last action before switching')"], check=True)
        image_file = a[a.index('-f') - 1]
        image = pathlib.Path(image_file).read_text().strip()
        with (p / 'images').open('a') as f: f.write(image + '\n')
        if scenario == 'startup-failure' and 'old-image' not in image: sys.exit(1)
        (p / 'started').touch()
    sys.exit(0)
sys.exit(2)
'''


class UpdateTests(unittest.TestCase):
    def setUp(self):
        scratch = ROOT / '.local' / 'update-tests'
        scratch.mkdir(parents=True, exist_ok=True)
        self.temp = tempfile.TemporaryDirectory(dir=scratch)
        self.addCleanup(self.temp.cleanup)
        self.path = Path(self.temp.name)
        shutil.copy(ROOT / 'scripts/update.sh', self.path / 'update.sh')
        shutil.copy(ROOT / 'compose.yaml', self.path / 'compose.yaml')
        (self.path / '.env').write_text('INVITE_CODE=test-secret\n')
        (self.path / '.image.env').write_text('WIRE_BOARD_IMAGE=previous\n')
        (self.path / 'volume').mkdir()
        self.db = sqlite3.connect(self.path / 'volume' / 'wire-board.db')
        self.addCleanup(self.db.close)
        self.db.execute('PRAGMA journal_mode=WAL')
        self.db.execute('CREATE TABLE state (value TEXT)')
        self.db.execute("INSERT INTO state VALUES ('existing accounts and games')")
        self.db.commit()
        (self.path / 'bin').mkdir()
        for name, source in {
            'docker': MOCK_DOCKER,
            'id': '#!/bin/sh\necho 0\n',
            'flock': '#!/bin/sh\nexit 0\n',
        }.items():
            target = self.path / 'bin' / name
            target.write_text(source)
            target.chmod(0o755)

    def run_update(self, scenario, tag='latest'):
        env = dict(os.environ, FIXTURE=str(self.path), SCENARIO=scenario, DIGEST=DIGEST)
        env['PATH'] = str(self.path / 'bin') + os.pathsep + env['PATH']
        result = subprocess.run(['bash', str(self.path / 'update.sh'), tag], env=env,
                                capture_output=True, text=True)
        self.assertEqual((self.path / '.env').read_text(), 'INVITE_CODE=test-secret\n')
        self.assertFalse([p for p in self.path.glob('.update.*') if p.name != '.update.lock'])
        self.assertFalse(list((self.path / 'backups').glob('*.partial')))
        return result

    def test_clean_preserves_current_rollback_containers_and_foreign_images(self):
        (self.path / '.previous-image').write_text('sha256:rollback\n')
        result = self.run_update('clean', 'clean')
        self.assertEqual(result.returncode, 0, result.stderr)
        calls = [json.loads(line) for line in (self.path / 'calls').read_text().splitlines()]
        removed = [a[2] for a in calls if a[:2] == ['image', 'rm']]
        self.assertCountEqual(removed, ['sha256:dangling', 'ghcr.io/k0ngk0ng/wire-board:v0.0.1'])
        self.assertFalse(any(a[0] == 'pull' or 'prune' in a or '--force' in a for a in calls))
        self.assertEqual(self.db.execute('SELECT value FROM state').fetchone()[0], 'existing accounts and games')

    def test_invalid_source_is_rejected(self):
        result = self.run_update('initial', 'other-registry.example/image:latest')
        self.assertNotEqual(result.returncode, 0)
        self.assertFalse((self.path / 'calls').exists())

    def test_pull_failure_keeps_existing_service(self):
        result = self.run_update('pull-failure')
        self.assertNotEqual(result.returncode, 0)
        self.assertNotIn('"stop"', (self.path / 'calls').read_text())
        self.assertEqual((self.path / '.image.env').read_text(), 'WIRE_BOARD_IMAGE=previous\n')

    def test_initial_deployment_pins_digest(self):
        result = self.run_update('initial', 'v1.0.2')
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual((self.path / '.image.env').read_text(), f'WIRE_BOARD_IMAGE={DIGEST}\n')
        self.assertFalse((self.path / 'backups').exists())

    def test_upgrade_backs_up_committed_wal_before_switch(self):
        self.assertTrue((self.path / 'volume/wire-board.db-wal').exists())
        result = self.run_update('upgrade')
        self.assertEqual(result.returncode, 0, result.stderr)
        with tarfile.open(next((self.path / 'backups').glob('*.tar.gz'))) as archive:
            backup = self.path / 'restored.db'
            backup.write_bytes(archive.extractfile('./wire-board.db').read())
            self.assertFalse(any(n.endswith(('-wal', '-shm')) for n in archive.getnames()))
        with sqlite3.connect(backup) as db:
            self.assertEqual(db.execute('PRAGMA quick_check').fetchone()[0], 'ok')
            self.assertEqual(db.execute('SELECT value FROM state').fetchone()[0], 'existing accounts and games')
        calls = [json.loads(line) for line in (self.path / 'calls').read_text().splitlines()]
        backup_call = next(a for a in calls if a[0] == 'run')
        self.assertIn('none', backup_call)
        self.assertIn('--read-only', backup_call)
        self.assertTrue(any('target=/source,readonly' in a for a in backup_call))
        self.assertLess(next(i for i,a in enumerate(calls) if a[0]=='pull'), calls.index(backup_call))
        self.assertLess(calls.index(backup_call), next(i for i,a in enumerate(calls) if 'up' in a))
        self.assertFalse(any('stop' in a for a in calls))
        self.assertEqual((self.path / '.previous-image').read_text().strip(), 'sha256:old-image')
        self.assertIn(('last action before switching',), self.db.execute('SELECT value FROM state').fetchall())

    def test_preparation_failures_leave_running_service_untouched(self):
        for scenario in ['backup-failure', 'config-failure']:
            with self.subTest(scenario=scenario):
                result = self.run_update(scenario)
                self.assertNotEqual(result.returncode, 0)
                calls = [json.loads(line) for line in (self.path / 'calls').read_text().splitlines()]
                self.assertFalse(any('stop' in a or 'up' in a for a in calls))
                self.assertEqual((self.path / '.image.env').read_text(), 'WIRE_BOARD_IMAGE=previous\n')
                self.assertFalse(list((self.path / 'backups').glob('*.tar.gz')))

    def test_unhealthy_upgrade_restarts_previous_image(self):
        result = self.run_update('startup-failure')
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual((self.path / 'images').read_text().splitlines(),
                         [f'WIRE_BOARD_IMAGE={DIGEST}', 'WIRE_BOARD_IMAGE=sha256:old-image'])
        self.assertEqual((self.path / '.image.env').read_text(), 'WIRE_BOARD_IMAGE=previous\n')
        self.assertTrue(list((self.path / 'backups').glob('*.tar.gz')))


if __name__ == '__main__':
    unittest.main()
