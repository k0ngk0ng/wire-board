#!/usr/bin/env python3
"""Reproduce the bounded native/independent controller normalization checks."""
import argparse
import json
import os
from pathlib import Path
import shutil
import subprocess

from reference import ROOT, sha


VERIFY = ROOT/'games/yuyuhakusho/verification/controller-normalization-work'
PATTERN = '^(TestControllerNormalizationWork|TestControllerAgainstOriginalCalls|TestControllerBoundaryEdges|TestControllerRejectsUnsupportedInputs|TestBattleControllerScan)'


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument('--out', type=Path, required=True)
    p.add_argument('--wasm', action='store_true')
    p.add_argument('--recapture', action='store_true', help='Execute both pinned cores again and compare the exact frozen fixture')
    a = p.parse_args()
    out = a.out.resolve()
    assert out.is_relative_to(VERIFY) and not out.exists()
    out.mkdir(parents=True)
    frozen = json.loads((VERIFY/'test-first.json').read_bytes())
    assert frozen['implementation_exists'] is False and frozen['native_cases'] == 689
    for path, digest in frozen['files'].items():
        assert sha((ROOT/path).read_bytes()) == digest, ('frozen input changed', path)
    env = dict(os.environ, TEST_TELEMETRY_DIR=str(ROOT/'.local/go-telemetry'),
               GOCACHE=str(ROOT/'.local/go-cache'), GOMODCACHE=str(ROOT/'.local/go-mod'),
               GOTMPDIR=str(ROOT/'.local/md-yuyuhakusho/tmp'),
               TMPDIR=str(ROOT/'.local/md-yuyuhakusho/tmp'))
    commands = []

    def run(argv, logfile):
        with (out/logfile).open('w') as f:
            completed = subprocess.run(argv, cwd=ROOT, env=env, stdout=f, stderr=subprocess.STDOUT)
        commands.append({'argv': argv, 'log': str((out/logfile).relative_to(ROOT)),
                         'log_sha256': sha((out/logfile).read_bytes()), 'returncode': completed.returncode})
        assert completed.returncode == 0, commands[-1]

    if a.recapture:
        assert not a.wasm
        run(['python3', '-B', 'scripts/yuyuhakusho/capture_controller_normalization_work.py',
             '--out', str(out/'recapture')], 'native-recapture.log')
        for name in ['native-controller-normalization.json.gz', 'source-steps.json.gz']:
            assert (out/'recapture'/name).read_bytes() == (VERIFY/'native-first'/name).read_bytes(), name
    go = shutil.which('go')
    assert go
    command = [go, 'test', '-json', '-count=1']
    if a.wasm:
        goroot = subprocess.check_output([go, 'env', 'GOROOT'], cwd=ROOT, env=env, text=True).strip()
        command += ['-exec', str(Path(goroot)/'lib/wasm/go_js_wasm_exec')]
        env.update(GOOS='js', GOARCH='wasm', GOMAXPROCS='1', GODEBUG='gccheckmark=1,clobberfree=1')
    else:
        command += ['-race']
    command += ['-run', PATTERN, './games/yuyuhakusho/engine']
    run(command, 'go-tests.jsonl')
    checks = [json.loads(line) for line in (out/'go-tests.jsonl').read_text().splitlines() if line.startswith('{')]
    assert any(r.get('Action') == 'pass' and r.get('Test') == 'TestControllerNormalizationWorkOriginalWholeCalls' for r in checks)
    assert not any(r.get('Action') in ['fail', 'build-fail'] for r in checks)
    run([go, 'vet', './games/yuyuhakusho/engine'], 'go-vet.log')
    paths = [Path(__file__), ROOT/'scripts/yuyuhakusho/capture_controller_normalization_work.py',
             ROOT/'games/yuyuhakusho/engine/controller_normalization_work.go',
             ROOT/'games/yuyuhakusho/engine/controller_normalization_work_test.go',
             ROOT/'games/yuyuhakusho/engine/input.go', ROOT/'games/yuyuhakusho/engine/controller_scan.go',
             ROOT/'games/yuyuhakusho/engine/main_work.go']
    paths += [ROOT/'docs/research/md-yuyuhakusho/static'/name for name in
              ['input-detail.txt', 'logical-input.txt', 'world-frame-driver.txt']]
    report = {'pass': True, 'platform': 'wasm' if a.wasm else 'native-race',
              'native_fixture': json.loads((VERIFY/'native-first/capture.json').read_bytes()),
              'native_recapture_identical': a.recapture, 'commands': commands,
              'environment': {k: env[k] for k in ['TEST_TELEMETRY_DIR', 'GOCACHE', 'GOMODCACHE', 'GOTMPDIR', 'TMPDIR']},
              'sources': {str(path.relative_to(ROOT)): sha(path.read_bytes()) for path in paths},
              'test_first_sha256': sha((VERIFY/'test-first.json').read_bytes()),
              'scope': 'Exact semantic normalization values and every native main-work boundary, hook/getter error atomicity and unchanged controller/scanner API regressions',
              'limits': ['Live scan replacement is an already completed scan; shared main-clock IRQ acceptance, cost and return integration are separate',
                         'Work observes the completed operation boundary after its read value is latched; a before-operation IRQ coordinator must service pending IRQs in LivePorts before returning the read snapshot']}
    (out/'validation.json').write_text(json.dumps(report, indent=2)+'\n')
    print(json.dumps({'pass': True, 'platform': report['platform'], 'native_recapture_identical': a.recapture,
                      'cases': 689, 'operations': 147216,
                      'validation': str((out/'validation.json').relative_to(ROOT))}))


if __name__ == '__main__':
    main()
