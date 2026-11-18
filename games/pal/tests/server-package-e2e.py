import hashlib
import json
from pathlib import Path
import subprocess
import tempfile

ROOT = Path(__file__).resolve().parents[3]
command = ['python3', '-B', 'games/pal/build/server-package.py']
with tempfile.TemporaryDirectory(prefix='server-packages-e2e-', dir=ROOT / '.local/pal') as temporary:
    folder = Path(temporary)
    args = [*command, '--out', str(folder)]
    first = subprocess.run(args, cwd=ROOT, text=True, capture_output=True)
    assert first.returncode == 0, first.stdout + first.stderr
    receipt = json.loads(first.stdout)
    package = ROOT / receipt['package']
    files = {str(path.relative_to(package)): hashlib.sha256(path.read_bytes()).hexdigest() for path in package.rglob('*') if path.is_file()}
    assert set(files) == {'manifest.json', 'authority.zip', 'cores/' + receipt['identity']['buildID'] + '.wasm'}
    assert receipt['identity']['profile'] == 'original-98' and receipt['identity']['purpose'] == 'campaign'
    repeated = subprocess.run(args, cwd=ROOT, text=True, capture_output=True)
    assert repeated.returncode == 0, repeated.stdout + repeated.stderr
    assert json.loads(repeated.stdout)['package'] == receipt['package']
    assert files == {str(path.relative_to(package)): hashlib.sha256(path.read_bytes()).hexdigest() for path in package.rglob('*') if path.is_file()}
    bad = subprocess.run([*args, '--core', '.local/pal/runtime-candidates/audio/hp-9999/rules.wasm'], cwd=ROOT, text=True, capture_output=True)
    assert bad.returncode != 0
    (package / 'music.flac').write_bytes(b'forbidden client media')
    rejected = subprocess.run(args, cwd=ROOT, text=True, capture_output=True)
    assert rejected.returncode != 0
report = {'passed': True, 'scope': 'actual original-profile physical server package, archive/current-core identity validation, same-directory immutable repeat, development-core and unexpected client-media rejection; no complete application build or production registry claim', 'identity': receipt['identity'], 'files': files, 'developmentRejected': True, 'unexpectedClientMediaRejected': True}
(ROOT / '.local/pal/verification/server-package-e2e.json').write_text(json.dumps(report, indent=2) + '\n')
print(json.dumps(report))
