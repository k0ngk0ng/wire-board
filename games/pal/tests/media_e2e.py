#!/usr/bin/env python3
"""Pinned original media acquisition acceptance, written before --media support.

Failure cases: optional media replaces rule resources; corrupt cached bytes are
trusted; a download escapes the workspace; unknown or empty streams are accepted;
re-running the fetch downloads again or loses the pinned source identity.
"""
import hashlib
import json
from pathlib import Path
import subprocess
import sys

ROOT = Path(__file__).resolve().parents[3]
OUT = ROOT / '.local/pal/original-98-media'
REPORT = ROOT / '.local/pal/verification/media.json'
manifest = json.loads((ROOT / 'games/pal/assets/media-source-manifest.json').read_text())
assert manifest['commit'] == 'ec8dfcb15eb78ce2e851332650b88c723e8eb760'
assert len(manifest['files']) == 14
before = hashlib.sha256((ROOT / '.local/pal/original-98/manifest.json').read_bytes()).hexdigest()
subprocess.run([sys.executable, '-B', 'games/pal/assets/fetch.py', '--media'], cwd=ROOT, check=True)
observed = []
for entry in manifest['files']:
    data = (OUT / entry['path']).read_bytes()
    assert len(data) == entry['size']
    assert hashlib.sha256(data).hexdigest() == entry['sha256']
    assert hashlib.sha1(b'blob '+str(len(data)).encode()+b'\0'+data).hexdigest() == entry['gitBlobSHA1']
    probe = json.loads(subprocess.check_output(['ffprobe', '-v', 'error', '-show_streams', '-show_format', '-of', 'json', str(OUT / entry['path'])], cwd=ROOT))
    streams = probe['streams']
    assert streams and float(probe['format']['duration']) > 0
    if entry['path'].endswith('.avi'):
        assert any(s['codec_type'] == 'video' and s['width'] > 0 and s['height'] > 0 for s in streams)
    else:
        assert all(s['codec_type'] == 'audio' and int(s['sample_rate']) > 0 for s in streams)
    observed.append({'path': entry['path'], 'sha256': entry['sha256'], 'bytes': len(data), 'duration': probe['format']['duration'], 'streams': [{k:s[k] for k in ['codec_name','codec_type','width','height','sample_rate','channels'] if k in s} for s in streams]})
assert before == hashlib.sha256((ROOT / '.local/pal/original-98/manifest.json').read_bytes()).hexdigest()
download = json.loads((ROOT / '.local/pal/verification/media-download.json').read_text())
assert download['passed'] and all(item['cached'] for item in download['results'])
REPORT.write_text(json.dumps({'passed': True, 'scope': 'pinned original AVI/CD data only; acquisition and stream validation, not browser playback or platform audio', 'sourceCommit': manifest['commit'], 'files': observed}, ensure_ascii=False, indent=2)+'\n')
print(json.dumps({'passed': True, 'files': len(observed), 'bytes': sum(item['bytes'] for item in observed), 'report': str(REPORT)}))
