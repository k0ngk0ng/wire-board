import argparse
import importlib.util
import json
import os
from pathlib import Path
import subprocess

ROOT = Path(__file__).resolve().parents[3]
spec = importlib.util.spec_from_file_location('pal_server_distribution', ROOT / 'games/pal/build/server-distribution.py')
distribution = importlib.util.module_from_spec(spec)
spec.loader.exec_module(distribution)


def write(path, value):
    distribution.guard(path)
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(value, ensure_ascii=False, indent=2) + '\n')


def attest(folder, runtime):
    distribution.guard(runtime)
    before = distribution.sources()
    manifest, _ = distribution.package_files(folder)
    saved = {name: (runtime / name).read_bytes() for name in ['rules.wasm', 'build.json'] if (runtime / name).exists()}
    log = ROOT / '.local/pal/verification/server-source-rebuild-run.txt'
    try:
        with log.open('w') as output:
            subprocess.run(['python3', '-B', 'games/pal/build/native.py', '--out', str(runtime.relative_to(ROOT))], cwd=ROOT, check=True, stdout=output, stderr=subprocess.STDOUT)
        digest = distribution.sha((runtime / 'rules.wasm').read_bytes())
        if digest != manifest['current'] or before != distribution.sources():
            raise ValueError('native source rebuild differs from the accepted core')
    except BaseException:
        for name, data in saved.items():
            (runtime / name).write_bytes(data)
        raise
    proof = {'passed': True, 'runtimeSHA256': digest, 'sources': before, 'runtimeDirectory': str(runtime.relative_to(ROOT)), 'scope': 'actual original-profile campaign C source reproduction at its original build path; no native snapshot identity rewrite'}
    write(ROOT / '.local/pal/verification/server-core-source.json', proof)
    return proof


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--package', type=Path)
    parser.add_argument('--source-runtime', type=Path, default=ROOT / '.local/pal/runtime-candidates/audio/original')
    parser.add_argument('--report', type=Path, default=ROOT / '.local/pal/verification/server-publication.json')
    modes = parser.add_mutually_exclusive_group()
    modes.add_argument('--publish', action='store_true')
    modes.add_argument('--verify-only', action='store_true')
    args = parser.parse_args()
    if os.environ.get('GITHUB_ACTIONS') == 'true':
        parser.error('server core publication and CDN read-back must run locally')
    selected = distribution.retained_release().get('current', 'a9234deb9d91a84e0d273f0d0ef3e90e81a7fbdf052c41ed4fcb0c91a35566ac')
    folder = (args.package or ROOT / '.local/pal/server-packages' / selected).absolute()
    runtime = args.source_runtime.absolute()
    distribution.guard(folder)
    distribution.verify(folder)
    proof = attest(folder, runtime)
    pin, rows = distribution.plan(folder, proof)
    report = {'passed': False, 'pin': pin, 'files': rows, 'stage': 'preflight', 'remoteWritesRequested': args.publish}
    write(args.report, report)
    if not args.publish and not args.verify_only:
        report.update(passed=True, remoteWritesPerformed=0)
        write(args.report, report)
        print(json.dumps({key: value for key, value in pin.items() if key not in ['files', 'sources']}))
        return
    fresh = [row for row in distribution.audio.unique_objects(rows) if not row['reused']]
    if args.publish and fresh:
        distribution.cdn.publish(folder, fresh, distribution.audio.BUCKET, distribution.audio.ENDPOINT)
    else:
        report['remoteWritesPerformed'] = 0
    verified = []
    for row in rows:
        distribution.download(row)
        verified.append({**row, 'cdnVerified': True})
    if distribution.plan(folder, proof) != (pin, rows):
        raise ValueError('server publication inputs changed during local read-back')
    report.update(passed=True, files=verified, stage='verified', verificationLocation='local')
    write(args.report, report)
    release = distribution.retained_release()
    published = {key: value for key, value in pin.items() if key not in ['reusedObjects', 'newObjects', 'uploadBytes']}
    published['cdnVerified'] = True
    old = release['catalogs'].get(pin['assetID'])
    if old and old != published:
        raise ValueError('retained server distribution record changed')
    release['catalogs'][pin['assetID']] = published
    release['current'] = pin['assetID']
    write(distribution.RELEASE, release)
    print(json.dumps({'passed': True, **{key: value for key, value in pin.items() if key not in ['files', 'sources']}}))


if __name__ == '__main__':
    main()
