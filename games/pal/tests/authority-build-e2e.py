import hashlib
import json
from pathlib import Path
import subprocess
import tempfile
import zipfile

ROOT = Path(__file__).resolve().parents[3]
CATALOG = json.loads((ROOT / 'games/pal/assets/authority-release.json').read_text())
SOURCE = ROOT / '.local/pal/original-98'
MANIFEST = json.loads((ROOT / 'games/pal/assets/source-manifest.json').read_text())
before = {item['path']: hashlib.sha256((SOURCE / item['path']).read_bytes()).hexdigest() for item in MANIFEST['files']}
command = ['python3', '-B', 'games/pal/build/authority.py']
with tempfile.TemporaryDirectory(prefix='authority-e2e-', dir=ROOT / '.local/pal') as temporary:
    folder = Path(temporary)
    args = [*command, '--out', str(folder)]
    subprocess.run(args, cwd=ROOT, check=True)
    first = (folder / 'authority.zip').read_bytes()
    assert len(first) == CATALOG['archiveBytes'] and hashlib.sha256(first).hexdigest() == CATALOG['archiveSHA256']
    with zipfile.ZipFile(folder / 'authority.zip') as archive:
        assert set(archive.namelist()) == set(CATALOG['files'])
        for name, expected in CATALOG['files'].items():
            data = archive.read(name)
            assert len(data) == expected['bytes'] and hashlib.sha256(data).hexdigest() == expected['sha256']
    subprocess.run(args, cwd=ROOT, check=True)
    assert (folder / 'authority.zip').read_bytes() == first
    for bad in ['games/pal/assets', '.local/pal/original-98']:
        result = subprocess.run([*command, '--out', bad], cwd=ROOT, capture_output=True)
        assert result.returncode != 0, bad
    (folder / 'authority.zip').unlink()
    (folder / 'authority.zip').symlink_to(ROOT / 'rules/common.md')
    protected = (ROOT / 'rules/common.md').read_bytes()
    result = subprocess.run(args, cwd=ROOT, capture_output=True)
    assert result.returncode != 0 and (ROOT / 'rules/common.md').read_bytes() == protected
for name, expected in before.items():
    assert hashlib.sha256((SOURCE / name).read_bytes()).hexdigest() == expected
report = {'passed': True, 'scope': 'source-pinned server-only archive byte-for-byte regeneration, all fifteen file hashes, repeated generation, protected-output and symlink rejection, all104 source files unchanged; no formal registration or server-image claim', 'archiveSHA256': CATALOG['archiveSHA256'], 'resourceID': CATALOG['resourceID'], 'bytes': len(first), 'files': len(CATALOG['files']), 'sourceFilesUnchanged': len(before)}
(ROOT / '.local/pal/verification/authority-build.json').write_text(json.dumps(report, indent=2) + '\n')
print(json.dumps(report))
