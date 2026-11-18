import argparse
import hashlib
import io
import json
from pathlib import Path
import subprocess
import tempfile
import zipfile

ROOT = Path(__file__).resolve().parents[3]
LOCAL = ROOT / '.local/pal'


def digest(data):
    return hashlib.sha256(data).hexdigest()


def guard(path, base=ROOT):
    path = path.absolute()
    if path.is_symlink() or not path.resolve().is_relative_to(base.resolve()) or any(parent.is_symlink() for parent in path.parents if parent.is_relative_to(ROOT)):
        raise ValueError('authority path escapes the workspace or contains a symlink')


def build(source, output):
    source, output = source.absolute(), output.absolute()
    guard(source)
    guard(output)
    if output.parent != LOCAL or not output.name.startswith('authority-'):
        raise ValueError('authority output must be a dedicated .local/pal/authority-* directory')
    for name in ['authority.zip', 'receipt.json', 'authority.zip.tmp', 'receipt.json.tmp']:
        guard(output / name)
    source_manifest = ROOT / 'games/pal/assets/source-manifest.json'
    catalog = json.loads((ROOT / 'games/pal/assets/authority-release.json').read_text())
    if catalog['format'] != 1 or digest(source_manifest.read_bytes()) != catalog['sourceManifestSHA256']:
        raise ValueError('registered original source manifest changed')
    manifest = json.loads(source_manifest.read_text())
    for item in manifest['files']:
        path = source / item['path']
        guard(path, source)
        data = path.read_bytes()
        if len(data) != item['size'] or digest(data) != item['sha256']:
            raise ValueError('original authority source differs: ' + item['path'])
    with tempfile.TemporaryDirectory(prefix='authority-build-', dir=LOCAL) as temporary:
        rules = Path(temporary) / 'rules'
        subprocess.run(['go', 'run', './games/pal/build/cmd/rules', '--source', str(source), '--out', str(rules)], cwd=ROOT, check=True, capture_output=True)
        files = {path.name: path.read_bytes() for path in rules.iterdir() if path.name != 'manifest.json'}
        for name in ['word.dat', 'm.msg']:
            files[name] = (source / name).read_bytes()
        if set(files) != set(catalog['files']):
            raise ValueError('authority file inventory differs from its accepted package')
        for name, data in files.items():
            expected = catalog['files'][name]
            if len(data) != expected['bytes'] or digest(data) != expected['sha256']:
                raise ValueError('headless authority data changed: ' + name)
        container = io.BytesIO()
        with zipfile.ZipFile(container, 'w', compression=zipfile.ZIP_DEFLATED, compresslevel=9) as archive:
            for name, data in sorted(files.items()):
                entry = zipfile.ZipInfo(name, (1980, 1, 1, 0, 0, 0))
                entry.compress_type = zipfile.ZIP_DEFLATED
                entry.create_system = 3
                entry.external_attr = 0o100644 << 16
                archive.writestr(entry, data, compress_type=zipfile.ZIP_DEFLATED, compresslevel=9)
        payload = container.getvalue()
    if len(payload) != catalog['archiveBytes'] or digest(payload) != catalog['archiveSHA256']:
        raise ValueError('authority archive differs from the source-equivalent registered bytes')
    registration = 'package engine\n\nvar authorityResourceIDs = map[string]string{\n\t"' + catalog['archiveSHA256'] + '": "' + catalog['resourceID'] + '",\n}\n'
    if (ROOT / 'games/pal/engine/authority_catalog.go').read_text() != registration:
        raise ValueError('runtime authority registration differs from the accepted package')
    output.mkdir(parents=True, exist_ok=True)
    for name, data in [('authority.zip', payload), ('receipt.json', (json.dumps(catalog, indent=2, sort_keys=True) + '\n').encode())]:
        temporary = output / (name + '.tmp')
        guard(temporary)
        temporary.write_bytes(data)
        temporary.replace(output / name)
    return {'archiveSHA256': catalog['archiveSHA256'], 'resourceID': catalog['resourceID'], 'bytes': len(payload), 'files': len(files), 'output': str(output.relative_to(ROOT))}


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--source', type=Path, default=ROOT / '.local/pal/original-98')
    parser.add_argument('--out', type=Path, default=ROOT / '.local/pal/authority-prepared')
    args = parser.parse_args()
    print(json.dumps(build(args.source, args.out)))


if __name__ == '__main__':
    main()
