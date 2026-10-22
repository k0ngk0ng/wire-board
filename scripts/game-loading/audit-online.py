#!/usr/bin/env python3
"""Read-only CDN routing/compression audit; optional cold/warm browser evidence.
Does not log in, create rooms, upload assets or modify the deployed application.
"""
import argparse, concurrent.futures, gzip, json, os, shutil, subprocess, time
from pathlib import Path
from urllib.request import Request, urlopen

ROOT = Path(__file__).resolve().parents[2]
parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument('--browser', action='store_true')
args = parser.parse_args()
origin = 'https://bga.ichenj.com'
out = ROOT / '.local/game-loading-online'
out.mkdir(parents=True, exist_ok=True)

def get(url):
    start = time.monotonic()
    request = Request(url, headers={'Accept-Encoding': 'gzip', 'Origin': origin})
    with urlopen(request, timeout=45) as response:
        first = time.monotonic()
        body, headers = response.read(), dict(response.headers)
    raw = gzip.decompress(body) if headers.get('Content-Encoding') == 'gzip' else body
    return raw, {'url': url, 'ttfbMs': round((first-start)*1000), 'totalMs': round((time.monotonic()-start)*1000), 'wireBytes': len(body), 'bytes': len(raw), 'headers': headers}

version = json.loads(get(origin + '/api/version')[0])
manifest = json.loads(get(origin + '/game-load-manifest.json')[0])
base = manifest['assetBase']
assert base.startswith('https://cdn.ichenj.com/bga/web/')
files = {f['url']: f for group in manifest['games'].values() for f in group}
assert all(url.startswith(base) for url in files)

def check(url):
    _, row = get(url)
    h = row['headers']
    assert row['bytes'] == files[url]['bytes'], url
    assert h.get('Content-Encoding') == 'gzip', url
    assert h.get('Access-Control-Allow-Origin') in ('*', origin), url
    assert 'immutable' in h.get('Cache-Control', ''), url
    assert h.get('Content-Type', '').split(';')[0] in ('text/javascript', 'application/javascript', 'text/css'), url
    return row

with concurrent.futures.ThreadPoolExecutor(max_workers=6) as pool:
    downloads = list(pool.map(check, files))
report = {'passed': True, 'version': version, 'assetBase': base, 'games': len(manifest['games']), 'codeFiles': len(files), 'allGameCodeOnCDN': True, 'requests': downloads, 'limits': 'Measurements from the development network, not the player mobile network. Only public frontend code is transferred to CDN; authoritative server rules remain on the server.'}
(out / 'http.json').write_text(json.dumps(report, ensure_ascii=False, indent=2) + '\n')
if args.browser:
    temp, sockets = out/'temporary', ROOT/'.s/cdna'
    temp.mkdir(exist_ok=True); sockets.mkdir(parents=True, exist_ok=True)
    env = dict(os.environ, AGENT_BROWSER_SESSION='cdn-audit', AGENT_BROWSER_PROFILE=str(temp/'profile'), AGENT_BROWSER_SOCKET_DIR=str(sockets), AGENT_BROWSER_ARGS='--disable-breakpad,--disable-crash-reporter')
    evidence = {'passed': False, 'checks': []}
    def browser(*command):
        p = subprocess.run(['agent-browser', *command, '--json'], env=env, cwd=ROOT, text=True, capture_output=True, timeout=45)
        value = json.loads(p.stdout)
        assert p.returncode == 0 and value['success'], value
        return value['data']
    def js(code): return browser('eval', code)['result']
    try:
        browser('open', origin); browser('wait', 'input[type=password]')
        for mode in ('cold', 'warm'):
            if mode == 'warm': browser('reload'); browser('wait', 'input[type=password]')
            rows = js(r'performance.getEntriesByType("resource").filter(e=>/\.(js|css)(\?|$)/.test(e.name)).map(e=>({url:e.name,ms:e.duration,transfer:e.transferSize,compressed:e.encodedBodySize,decoded:e.decodedBodySize}))')
            assert rows and all(row['url'].startswith(base) for row in rows), rows
            evidence['checks'].append({'mode': mode, 'codeRequests': rows})
        browser('screenshot', str(out/'online-login.png'))
        evidence['errors'] = browser('errors'); assert not evidence['errors']['errors']
        evidence['passed'] = True
    finally:
        browser('close'); shutil.rmtree(temp); shutil.rmtree(sockets)
        (out/'browser.json').write_text(json.dumps(evidence, ensure_ascii=False, indent=2) + '\n')
print(json.dumps({'passed': True, 'version': version['version'], 'games': report['games'], 'codeFiles': len(files), 'artifacts': str(out)}, ensure_ascii=False))
