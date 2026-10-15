#!/usr/bin/env python3
"""Publish verified racing media immutably; verify both OSS and public CDN bytes."""
import base64, hashlib, hmac, json, subprocess, urllib.error, urllib.request
from email.utils import formatdate
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
subprocess.run(['python3', str(ROOT / 'scripts/racing/verify-assets.py'), '--decode-audio'], check=True)
m = json.loads((ROOT / 'games/racing/assets/manifest.json').read_text())
bucket = 'stoneage-assets'; endpoint = f'https://{bucket}.oss-cn-shanghai.aliyuncs.com'
prefix = 'bga/v20261002-1'
assert m['assetsBaseURL'] == 'https://cdn.ichenj.com/' + prefix
key = (ROOT / 'config/secrets/oss-access-key-id').read_text().strip()
secret = (ROOT / 'config/secrets/oss-access-key-secret').read_text().strip()
sha = lambda b: hashlib.sha256(b).hexdigest()
def request(method, path, body=None, mime=''):
    date = formatdate(usegmt=True)
    md5 = base64.b64encode(hashlib.md5(body).digest()).decode() if body else ''
    extra = 'x-oss-forbid-overwrite:true\n' if method == 'PUT' else ''
    canonical = f'{method}\n{md5}\n{mime}\n{date}\n{extra}/{bucket}/{path}'
    signature = base64.b64encode(hmac.new(secret.encode(), canonical.encode(), hashlib.sha1).digest()).decode()
    headers = {'Date': date, 'Authorization': f'OSS {key}:{signature}'}
    if method == 'PUT':
        headers.update({'Content-MD5': md5, 'Content-Type': mime, 'x-oss-forbid-overwrite': 'true', 'Cache-Control': 'public, max-age=31536000, immutable'})
    return urllib.request.urlopen(urllib.request.Request(endpoint + '/' + path, data=body, method=method, headers=headers), timeout=40)
# Bind publication to the known platform bucket/prefix before creating any objects.
known = json.loads((ROOT / 'games/snooker/assets/manifest.json').read_text())['entries'][0]
with request('GET', prefix + known['urlPath']) as response:
    assert sha(response.read()) == known['sha256'], 'wrong platform bucket or prefix'
results = []
for e in m['entries']:
    path = prefix + e['urlPath']; data = (ROOT / e['output']).read_bytes()
    assert sha(data) == e['sha256']
    try:
        with request('GET', path) as response:
            assert sha(response.read()) == e['sha256'], 'immutable object differs: ' + path
    except urllib.error.HTTPError as error:
        if error.code != 404: raise
        with request('PUT', path, data, e['mime']) as response: assert response.status == 200
    with request('GET', path) as response: assert sha(response.read()) == e['sha256']
    with urllib.request.urlopen(m['assetsBaseURL'] + e['urlPath'], timeout=40) as response:
        assert sha(response.read()) == e['sha256']
        assert response.headers.get_content_type() == e['mime']
    results.append({'path': path, 'sha256': e['sha256'], 'bytes': len(data), 'cdnVerified': True})
    print('Verified', e['urlPath'], flush=True)
target = ROOT / '.local/racing/verification/oss-publish.json'
target.write_text(json.dumps({'passed': True, 'objects': results}, indent=2) + '\n')
print('Published and read back', len(results), 'immutable objects:', target)
