import argparse
import hashlib
import http.cookiejar
import json
import os
from pathlib import Path
import shutil
import signal
import subprocess
import sys
import time
import urllib.request
import uuid

ROOT = Path(__file__).resolve().parents[2]
sys.path.insert(0, str(ROOT / 'scripts'))
from resource_store import configuration

parser = argparse.ArgumentParser()
parser.add_argument('--dist', type=Path, required=True)
parser.add_argument('--out', type=Path, required=True)
args = parser.parse_args()
dist = args.dist.resolve()
out = args.out.resolve()
if not dist.is_relative_to(ROOT) or not out.is_relative_to(ROOT) or out.exists():
    raise ValueError('Build and new evidence directory must stay inside the repository')
out.mkdir(parents=True)
config = configuration()
temporary = out / 'temporary'
env = dict(os.environ, **{key: str(value) for key, value in config.items()})
for key, path in {'TMPDIR': temporary / 'tmp', 'GOTMPDIR': temporary / 'tmp', 'GOCACHE': ROOT / '.local/go-build', 'GOMODCACHE': ROOT / '.local/go-mod', 'TEST_TELEMETRY_DIR': ROOT / '.local/go-telemetry', 'AGENT_BROWSER_SOCKET_DIR': ROOT / '.s/rsrc', 'AGENT_BROWSER_PROFILE': temporary / 'profile'}.items():
    path.mkdir(parents=True, exist_ok=True)
    env[key] = str(path)
env.update(GOTOOLCHAIN='local', GOTELEMETRY='off', AGENT_BROWSER_NO_VERSION_CHECK='1', AGENT_BROWSER_ARGS='--mute-audio,--disable-breakpad,--disable-crash-reporter')
session = subprocess.check_output(['agent-browser', 'session', 'id', '--scope', 'worktree', '--prefix', 'rsrc'], cwd=ROOT, env=env, text=True).strip()
runner = temporary / 'server.go'
runner.write_text('''package main
import (
 "net"
 "net/http"
 "os"
 "path/filepath"
 "github.com/k0ngk0ng/wire-board/games"
 "github.com/k0ngk0ng/wire-board/internal/server"
)
func main() {
 app, err := server.New(server.Config{DataDir: filepath.Join(os.Args[2],"data"), InviteCode: "resource-browser-local", AdminUsername: "ResourceAdmin", AdminPassword: "resource-local-test-only", AssetsBaseURL: os.Getenv("ASSETS_BASE_URL"), Modules: games.Modules()}, os.DirFS(os.Args[1]))
 if err != nil { panic(err) }
 defer app.Close()
 listener, err := net.Listen("tcp", "127.0.0.1:0")
 if err != nil { panic(err) }
 if err = os.WriteFile(filepath.Join(os.Args[2],"url"), []byte("http://"+listener.Addr().String()), 0600); err != nil { panic(err) }
 if err = http.Serve(listener, app.Handler()); err != nil { panic(err) }
}
''')
log = (out / 'server.log').open('w')
server = subprocess.Popen(['go', 'run', str(runner), str(dist), str(temporary)], cwd=ROOT, env=env, stdout=log, stderr=subprocess.STDOUT, start_new_session=True)
opened = False
report = {'passed': False, 'command': 'python3 -B scripts/resources/check-browser.py --dist ' + str(args.dist) + ' --out ' + str(args.out), 'indexSHA256': hashlib.sha256((dist / 'index.html').read_bytes()).hexdigest(), 'viewports': []}


def browser(*arguments):
    result = subprocess.run(['agent-browser', '--session', session, *arguments, '--json'], cwd=ROOT, env=env, text=True, capture_output=True, timeout=45)
    if result.returncode:
        raise RuntimeError(result.stdout + result.stderr)
    value = json.loads(result.stdout)
    if not value['success']:
        raise RuntimeError(str(value))
    return value['data']


def js(source):
    return browser('eval', source)['result']


def api(path, body=None):
    options = '' if body is None else ',{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify(' + json.dumps(body) + ')}'
    value = js('(async()=>{const r=await fetch(' + json.dumps('/api' + path) + options + ');return {status:r.status,data:await r.json()}})()')
    if value['status'] not in [200, 201]:
        raise ValueError(str(value))
    return value['data']


def peer_api(path, body=None):
    request = urllib.request.Request(base + '/api' + path, data=None if body is None else json.dumps(body).encode(), headers={'Content-Type': 'application/json'})
    with peer.open(request, timeout=20) as response:
        return json.load(response)


def command(kind, peer_actor=False, **extra):
    call = peer_api if peer_actor else api
    room = call('/state')['room']
    return call('/rooms/' + room['id'], dict(type=kind, version=room['version'], nonce=uuid.uuid4().hex, **extra))


def artwork():
    result = js(r'''(async()=>{
      const selectors=['.market .card-art','.market .deck','.nobles .noble','.gem-bank .token','.wallet-token'];
      const groups=selectors.map(selector=>({selector,images:[...document.querySelectorAll('.splendor-board '+selector)].map(e=>getComputedStyle(e).backgroundImage)}));
      const images=await Promise.all([...new Set(groups.flatMap(group=>group.images))].map(async background=>{
        const requested=background.slice(4,-1).replace(/^["']|["']$/g,'');
        const image=new Image();image.src=requested;await image.decode();
        const response=await fetch(requested);if(!response.ok)throw Error('artwork HTTP '+response.status);
        const bytes=await response.arrayBuffer();const sha256=[...new Uint8Array(await crypto.subtle.digest('SHA-256',bytes))].map(n=>n.toString(16).padStart(2,'0')).join('');
        return {requested,finalURL:response.url,sha256,bytes:bytes.byteLength,width:image.naturalWidth,height:image.naturalHeight};
      }));
      return {groups,images,width:innerWidth,height:innerHeight,overflow:document.documentElement.scrollWidth>innerWidth+1};
    })()''')
    if len(result['images']) != 3 or result['overflow'] or any(not group['images'] for group in result['groups']):
        raise ValueError('Missing or overflowing game artwork')
    registry = json.loads((ROOT / 'assets/external-resources.json').read_text())['files']
    for image in result['images']:
        logical = '/splendor/' + image['requested'].rsplit('/', 1)[1]
        expected = registry[logical]
        if image['sha256'] != expected['sha256'] or image['bytes'] != expected['bytes'] or image['finalURL'] != config['CDN_ORIGIN'] + '/' + expected['key']:
            raise ValueError('Rendered artwork differs from the canonical resource registry')
    return result


try:
    deadline = time.monotonic() + 120
    while not (temporary / 'url').exists():
        if server.poll() is not None or time.monotonic() >= deadline:
            raise RuntimeError('Local browser fixture did not start; inspect server.log')
        time.sleep(0.1)
    base = (temporary / 'url').read_text()
    peer = urllib.request.build_opener(urllib.request.HTTPCookieProcessor(http.cookiejar.CookieJar()))
    peer_api('/register', {'name': '资源同伴' + uuid.uuid4().hex[:8], 'password': 'resource-local-test-only', 'invite': 'resource-browser-local'})
    browser('open', base)
    opened = True
    api('/login', {'name': 'ResourceAdmin', 'password': 'resource-local-test-only'})
    browser('reload')
    browser('wait', '.room-list')
    api('/rooms', {'kind': 'splendor', 'name': '璀璨宝石资源验收', 'capacity': 2})
    room = api('/state')['room']
    peer_api('/rooms/' + room['id'], {'type': 'join', 'version': room['version'], 'nonce': uuid.uuid4().hex})
    command('ready')
    command('ready', peer_actor=True)
    command('start')
    browser('wait', '.splendor-board')
    browser('wait', '--fn', '!document.querySelector(".game-loading")')
    for width, height in [(1440, 1000), (1170, 740), (390, 844), (320, 740)]:
        browser('set', 'viewport', str(width), str(height))
        browser('scrollintoview', '.splendor-board')
        report['viewports'].append(artwork())
        browser('screenshot', '--full', str(out / ('splendor-' + str(width) + '.png')))
    room = api('/state')['room']
    if room['game']['turn'] != room['you']:
        command('action', peer_actor=True, action={'type': 'take', 'tokens': [1, 1, 1, 0, 0, 0]})
    browser('wait', '--fn', '[...document.querySelectorAll(".token-button")].some(e=>!e.disabled)')
    for color in range(3):
        browser('click', '.token-button[data-bank-color="' + str(color) + '"]')
    browser('click', '.bank-actions .primary')
    browser('wait', '--fn', '[...document.querySelectorAll(".wallet-held strong")].reduce((sum,e)=>sum+Number(e.textContent),0)===3')
    report['realTokenAction'] = True
    browser('reload')
    browser('wait', '.splendor-board')
    browser('wait', '--fn', '!document.querySelector(".game-loading")')
    report['reload'] = artwork()
    browser('screenshot', '--full', str(out / 'splendor-after-action-reload.png'))
    report['errors'] = browser('errors')
    if report['errors']['errors']:
        raise ValueError('Browser page errors')
    report['passed'] = True
except Exception as error:
    report['error'] = str(error)
    raise
finally:
    if opened:
        if not report['passed']:
            try:
                report['snapshot'] = browser('snapshot', '-i')
                browser('screenshot', '--full', str(out / 'failure.png'))
            except Exception:
                pass
        browser('close')
    try:
        os.killpg(server.pid, signal.SIGTERM)
    except ProcessLookupError:
        pass
    try:
        server.wait(timeout=10)
    except subprocess.TimeoutExpired:
        os.killpg(server.pid, signal.SIGKILL)
        server.wait()
    log.close()
    (out / 'result.json').write_text(json.dumps(report, ensure_ascii=False, indent=2) + '\n')
    shutil.rmtree(temporary)
