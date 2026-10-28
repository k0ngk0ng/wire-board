#!/usr/bin/env python3
"""Natural solo failure and real terminal overlay on a private platform fixture.
Only registration/room API, native keys and read-only authority snapshots are used.
"""
import argparse, hashlib, json, os, re, shutil, subprocess, time, traceback, uuid
from pathlib import Path

root = Path(__file__).resolve().parents[2]
p = argparse.ArgumentParser(); p.add_argument('--fixture', required=True); p.add_argument('--label', required=True); args = p.parse_args()
assert re.fullmatch(r'[a-z0-9-]+', args.label)
fixture = json.loads((root / args.fixture).read_text()); base = fixture['url']
out = root / '.local/contra-browser' / ('terminal-' + args.label); out.mkdir()
temp = out / 'temporary'; temp.mkdir(); (temp / 'tmp').mkdir()
session = 'ct' + uuid.uuid4().hex[:8]; socket = root / '.s' / session; socket.mkdir(parents=True)
env = dict(os.environ, TMPDIR=str(temp / 'tmp'), AGENT_BROWSER_SESSION=session, AGENT_BROWSER_SOCKET_DIR=str(socket), AGENT_BROWSER_PROFILE=str(temp / 'profile'), AGENT_BROWSER_ARGS='--mute-audio,--disable-breakpad,--disable-crash-reporter')
script = Path(__file__).read_bytes(); (out / 'harness-at-run.py').write_bytes(script)
native_source = (root / 'scripts/contra/browser_input.mjs').read_bytes(); (out / 'native-input-at-run.mjs').write_bytes(native_source)
report = {'passed': False, 'scope': 'Natural standard solo failure and terminal presentation; not eight-stage clear, bot or subjective audio acceptance.', 'fixture': {k: fixture.get(k) for k in ['url', 'rulesBuildID', 'webRoot', 'publicRoot', 'webManifestSHA256']}}
def browser(*args):
    result = subprocess.run(['agent-browser', *args, '--json'], cwd=root, env=env, capture_output=True, text=True, timeout=45)
    assert result.returncode == 0, result.stdout + result.stderr
    value = json.loads(result.stdout); assert value['success'], value
    return value['data']
def js(expression): return browser('eval', expression)['result']
def api(path, body=None):
    options = {'method': 'GET'} if body is None else {'method': 'POST', 'headers': {'Content-Type': 'application/json'}, 'body': json.dumps(body)}
    value = js('(async()=>{const r=await fetch(' + json.dumps(path) + ',' + json.dumps(options) + ');return {status:r.status,data:await r.json()}})()')
    assert 200 <= value['status'] < 300, value
    return value['data']
def command(kind):
    state = api('/api/state'); room = state['room']
    return api('/api/rooms/' + room['id'], {'type': kind, 'version': room['version'], 'nonce': uuid.uuid4().hex, 'serverInstance': state['serverInstance']})
def native(options):
    assert (root / 'scripts/contra/browser_input.mjs').read_bytes() == native_source, 'native input helper changed during run'
    endpoint = browser('get', 'cdp-url')['cdpUrl']
    result = subprocess.run(['node', str(out / 'native-input-at-run.mjs'), endpoint, base, json.dumps({'platform': True, **options})], cwd=root, env=env, capture_output=True, text=True, timeout=20)
    assert result.returncode == 0, result.stdout + result.stderr
    return json.loads(result.stdout)
def surface(): return js('({...document.querySelector("[data-contra-runtime]")?.dataset})')
try:
    browser('open', base); browser('set', 'viewport', '1100', '1000')
    api('/api/register', {'name': '魂斗罗终局-' + uuid.uuid4().hex[:7], 'password': 'contra-terminal-test-only', 'invite': 'contra-browser'})
    created = api('/api/rooms', {'kind': 'contra', 'capacity': 1, 'name': '魂斗罗自然终局验收', 'options': {}}); room_id = created['id']
    command('ready'); command('start'); browser('reload')
    browser('wait', '--fn', 'Number(document.querySelector("[data-contra-runtime]")?.dataset.checked)>5||!!document.querySelector(".contra-board-error")')
    assert not js('document.querySelector(".contra-board-error")?.textContent')
    browser('click', '[data-fullscreen-toggle]')
    native({'keys': ['KeyD'], 'frames': 6, 'hold': True})
    deadline = time.monotonic() + 100
    while time.monotonic() < deadline:
        current = api('/api/rooms/' + room_id + '/sync')['data']
        if current['checkpoint']['state']['World']['Active']['Combat']['World']['LevelRoutine'] == 6:
            break
        assert not js('document.querySelector(".contra-board-error")?.textContent')
        time.sleep(.25)
    else: raise AssertionError('natural movement did not reach original continue menu')
    report['menu'] = current['view']['snapshot']; browser('screenshot', str(out / 'continue-menu.png'))
    native({'keys': ['KeyD'], 'releaseOnly': True})
    native({'keys': ['KeyF'], 'frames': 3})
    js('window.__terminalTrace=[];window.__terminalObserve=true;requestAnimationFrame(function sample(at){const r=document.querySelector("[data-contra-runtime]");window.__terminalTrace.push({at,...r?.dataset,result:!!document.querySelector(".match-result-modal")});if(window.__terminalObserve)requestAnimationFrame(sample)});true')
    browser('press', 'Space')
    browser('wait', '--fn', '!!document.querySelector(".match-result-modal")||!!document.querySelector(".contra-board-error")')
    assert not js('document.querySelector(".contra-board-error")?.textContent')
    report['trace'] = js('window.__terminalObserve=false;window.__terminalTrace')
    assert any(row['result'] for row in report['trace']), 'result overlay was not sampled'
    assert all(row.get('phase') == 'finished' for row in report['trace'] if row['result']), 'result overlay preceded submitted terminal frame'
    report['finalSurface'] = surface(); assert report['finalSurface']['phase'] == 'finished'
    report['finalRoom'] = api('/api/state')['room']; assert report['finalRoom']['status'] == 'finished'
    report['resultText'] = js('document.querySelector(".match-result-modal").textContent')
    browser('wait', '--fn', 'document.querySelector(".match-result-modal").getAnimations({subtree:true}).every(a=>a.playState!=="running")')
    rect = '(()=>{const r=document.querySelector(".contra-runtime canvas").getBoundingClientRect();return {x:r.x,y:r.y,width:r.width,height:r.height}})()'
    report['boardBeforeClose'] = js(rect)
    browser('screenshot', str(out / 'desktop-result.png'))
    browser('click', '.match-result-modal button[aria-label="关闭"]')
    time.sleep(.4); assert not js('!!document.querySelector(".match-result-modal")')
    report['boardAfterClose'] = js(rect)
    assert report['boardBeforeClose'] == report['boardAfterClose'], 'closing results moved final board'
    browser('screenshot', str(out / 'desktop-final-board.png'))
    browser('click', '.result-reopen'); browser('set', 'viewport', '320', '740')
    browser('wait', '--fn', 'document.querySelector(".match-result-modal").getAnimations({subtree:true}).every(a=>a.playState!=="running")')
    browser('screenshot', str(out / 'mobile-result.png'))
    report['mobileGeometry'] = js('(()=>{const e=document.querySelector(".match-result-modal"),r=e.getBoundingClientRect();return {x:r.x,y:r.y,width:r.width,height:r.height,viewport:[innerWidth,innerHeight],scrollWidth:document.documentElement.scrollWidth}})()')
    assert report['mobileGeometry']['scrollWidth'] == 320
    report['passed'] = True
except BaseException:
    report['error'] = traceback.format_exc()
    try: browser('screenshot', str(out / 'failure.png')); report['snapshot'] = browser('snapshot', '-i'); report['errors'] = browser('errors')
    except BaseException: pass
finally:
    try: browser('close')
    except BaseException: report['passed'] = False
    if socket.exists(): shutil.rmtree(socket)
    if temp.exists(): shutil.rmtree(temp)
    report['scriptSHA256'] = hashlib.sha256(script).hexdigest(); report['nativeInputSHA256'] = hashlib.sha256(native_source).hexdigest()
    (out / 'result.json').write_text(json.dumps(report, ensure_ascii=False, indent=2) + '\n')
    print(json.dumps({'passed': report['passed'], 'error': report.get('error'), 'output': str(out)}, ensure_ascii=False))
raise SystemExit(0 if report['passed'] else 1)
