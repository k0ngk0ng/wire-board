import argparse
import hashlib
import json
from pathlib import Path
import subprocess
import tempfile

ROOT = Path(__file__).resolve().parents[3]
LOCAL = ROOT / '.local/pal'


def guard(path):
    path = path.absolute()
    if path.is_symlink() or not path.resolve().is_relative_to(ROOT) or any(parent.is_symlink() for parent in path.parents if parent.is_relative_to(ROOT)):
        raise ValueError('server package path escapes workspace or contains a symlink')


def sha(data):
    return hashlib.sha256(data).hexdigest()


def inventory(folder):
    result = {}
    for path in folder.rglob('*'):
        guard(path)
        if path.is_file():
            result[str(path.relative_to(folder))] = sha(path.read_bytes())
    return result


def verify(folder):
    result = subprocess.run(['go', 'run', './games/pal/build/cmd/verify-production', '--package', str(folder)], cwd=ROOT, check=True, capture_output=True, text=True)
    report = json.loads(result.stdout)
    if not report['passed'] or not report['singlePlayer'] or not report['supportsSave'] or not report['supportsCheckpoint']:
        raise ValueError('server package does not expose the required PAL capabilities')
    return report


def build(core, archive, retained, output):
    output = output.absolute()
    guard(output)
    if output.parent != LOCAL or not output.name.startswith('server-package'):
        raise ValueError('server package output must stay in a dedicated .local/pal/server-package* directory')
    catalog = json.loads((ROOT / 'games/pal/assets/authority-release.json').read_text())
    guard(archive)
    authority = archive.read_bytes()
    if len(authority) != catalog['archiveBytes'] or sha(authority) != catalog['archiveSHA256']:
        raise ValueError('server authority archive differs from its accepted source-bound bytes')
    programs = {}
    ids = []
    for path in [core, *retained]:
        guard(path)
        program = path.read_bytes()
        identity = sha(program)
        if identity in programs:
            raise ValueError('server package repeats a native core')
        programs[identity] = program
        ids.append(identity)
    manifest = {'format': 1, 'resourceID': catalog['resourceID'], 'authoritySHA256': catalog['archiveSHA256'], 'current': ids[0], 'retained': ids[1:]}
    raw = (json.dumps(manifest, indent=2, sort_keys=True) + '\n').encode()
    identity = sha(raw)
    output.mkdir(parents=True, exist_ok=True)
    target = output / identity
    guard(target)
    with tempfile.TemporaryDirectory(prefix='staging-', dir=output) as temporary:
        folder = Path(temporary)
        (folder / 'cores').mkdir()
        (folder / 'manifest.json').write_bytes(raw)
        (folder / 'authority.zip').write_bytes(authority)
        for program_id, program in programs.items():
            (folder / 'cores' / (program_id + '.wasm')).write_bytes(program)
        report = verify(folder)
        expected = inventory(folder)
        if target.exists():
            if inventory(target) != expected:
                raise ValueError('existing immutable server package differs or contains unexpected payloads')
            verify(target)
        else:
            folder.rename(target)
    return {**report, 'package': str(target.relative_to(ROOT)), 'manifestSHA256': identity, 'files': expected}


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--core', type=Path, default=ROOT / '.local/pal/runtime-candidates/audio/original/rules.wasm')
    parser.add_argument('--authority', type=Path, default=ROOT / '.local/pal/authority-prepared/authority.zip')
    parser.add_argument('--retain', type=Path, action='append', default=[])
    parser.add_argument('--out', type=Path, default=LOCAL / 'server-packages')
    args = parser.parse_args()
    print(json.dumps(build(args.core, args.authority, args.retain, args.out)))


if __name__ == '__main__':
    main()
