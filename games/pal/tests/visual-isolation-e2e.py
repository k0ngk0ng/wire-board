import hashlib
import json
import os
from pathlib import Path
import subprocess

ROOT = Path(__file__).resolve().parents[3]
BASE = ROOT / '.local/pal/visual/pal/v1'
assert Path(os.environ['GOTMPDIR']).resolve().is_relative_to(ROOT)
index = json.loads((BASE / 'index.json').read_text())
assert 'fonts.json' in index['files'] and 'fonts/glyphs.bin.gz' in index['files']
paths = [*index['files'], 'index.json']
before = {name: hashlib.sha256((BASE / name).read_bytes()).hexdigest() for name in paths}
subprocess.run(['go', 'test', '-v', './games/pal/resources', '-run', '^TestOriginalVisualExport$', '-count=1', '-timeout=3m'], cwd=ROOT, check=True)
after = {name: hashlib.sha256((BASE / name).read_bytes()).hexdigest() for name in paths}
assert before == after, 'isolated export E2E changed current client-release input'
report = {'passed': True, 'scope': 'real original resource-export E2E in an independent temporary directory, all current release-input hashes unchanged including original font registration; no CDN or gameplay claim', 'unchangedFiles': len(paths), 'indexSHA256': before['index.json']}
(ROOT / '.local/pal/verification/visual-export-isolation.json').write_text(json.dumps(report, indent=2) + '\n')
print(json.dumps(report))
