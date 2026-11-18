import argparse
import json
import os
from pathlib import Path
import shutil
import signal
import subprocess
import time
import uuid

ROOT = Path(__file__).resolve().parents[3]
parser = argparse.ArgumentParser()
parser.add_argument('--baseline', action='store_true')
args = parser.parse_args()
out = ROOT / '.local/pal/verification/local-first' / ('baseline-browser' if args.baseline else 'candidate-browser')
out.mkdir(parents=True, exist_ok=True)
socket_root = Path(subprocess.check_output(['git', 'rev-parse', '--git-common-dir'], cwd=ROOT, text=True).strip()).resolve().parent/'.local/pal-lf-ab'
env = dict(os.environ, AGENT_BROWSER_SOCKET_DIR=str(socket_root), AGENT_BROWSER_PROFILE=str(out/'profile'), TMPDIR=str(ROOT/'.local/tmp'), AGENT_BROWSER_NO_VERSION_CHECK='1', AGENT_BROWSER_ARGS='--mute-audio,--disable-breakpad,--disable-crash-reporter')
Path(env['AGENT_BROWSER_SOCKET_DIR']).mkdir(exist_ok=True)
session = subprocess.check_output(['agent-browser', 'session', 'id', '--scope', 'worktree', '--prefix', 'lf'], cwd=ROOT, env=env, text=True).strip()
base = (ROOT/'.local/pal/cdn-performance-board-browser/url').read_text().strip()
report = {'passed': False, 'budgets': {'p50': 33, 'p95': 50, 'max': 100}, 'cases': [], 'scope': 'real original inn, native keyboard and canvas pixels, full menu opening/confirmation/back, normal and 200 ms RTT'}
def browser(*argv):
    result = subprocess.run(['agent-browser', '--session', session, *map(str, argv), '--json'], cwd=ROOT, env=env, text=True, capture_output=True, timeout=50)
    assert result.returncode == 0, result.stdout + result.stderr
    value = json.loads(result.stdout)
    assert value['success'], value
    return value['data']
def js(source):
    return browser('eval', source)['result']
def wait(fn, label, seconds=60):
    until = time.monotonic()+seconds
    while time.monotonic() < until:
        value = fn()
        if value:
            return value
        time.sleep(.05)
    raise AssertionError(label)
def api(path, body=None):
    options = '' if body is None else ',{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify('+json.dumps(body)+')}'
    value = js('(async()=>{const r=await fetch('+json.dumps('/api'+path)+options+');return {status:r.status,body:await r.json()}})()')
    assert 200 <= value['status'] < 300, value
    return value['body']
def room():
    return api('/state')['room']
def command(kind):
    value = room()
    return api('/rooms/'+value['id'], {'type': kind, 'version': value['version'], 'nonce': uuid.uuid4().hex})
proxy = subprocess.Popen(['node', 'games/pal/tests/performance-proxy.mjs', base], cwd=ROOT, env=env, stdin=subprocess.PIPE, stdout=subprocess.PIPE, text=True)
url = json.loads(proxy.stdout.readline())['url']
observer = None
device = None
def trace():
    observer.send_signal(signal.SIGUSR2)
    return json.loads(observer.stdout.readline())['events']
def quantile(values, p):
    values = sorted(values)
    return values[round((len(values)-1)*p)]
try:
    browser('open', url)
    browser('set', 'viewport', 1280, 900)
    api('/register', {'name': '仙剑本地响应'+uuid.uuid4().hex[:7], 'password': 'pal-local-fixture-123', 'invite': 'test-invite'})
    observer = subprocess.Popen(['node', 'games/pal/tests/performance-observer.mjs', browser('get','cdp-url')['cdpUrl'], url, 'local-performance-page.js'], cwd=ROOT, env=env, stdout=subprocess.PIPE, text=True)
    assert observer.stdout.readline().strip() == 'ready'
    api('/rooms', {'kind': 'pal', 'name': '本地完整输入', 'capacity': 1})
    command('ready')
    command('start')
    browser('reload')
    browser('wait', '[data-pal-ready=true]')
    wait(lambda: not room().get('loading'), 'loading gate did not release')
    browser('click', '[data-fullscreen-enter]')
    browser('wait', '[data-room-fullscreen]')
    js('document.querySelector("[data-pal-board]").focus()')
    if not args.baseline:
        wait(lambda: js('(()=>{const s=JSON.parse(document.querySelector("[data-pal-board]").dataset.palLocalState||"{}");return s.ready&&s.active&&s.enabled})()'), 'local replica did not complete authenticated loading')
    report['initialLocalState'] = js('document.querySelector("[data-pal-board]").dataset.palLocalState')
    print(json.dumps({'initialLocalState': report['initialLocalState']}), flush=True)
    for delay in [100, 0]:
        proxy.stdin.write(json.dumps({'delayMS': delay})+'\n')
        proxy.stdin.flush()
        assert json.loads(proxy.stdout.readline())['delayMS'] == delay
        samples = []
        for repeat in range(4):
            for code, phase, kind in [('Backspace','menu',1), ('Enter','menu',2), ('Backspace','field',0)]:
                js('window.palLocalPerformance.arm('+json.dumps({'code':code, 'phase':phase, 'kind':kind})+')')
                browser('press', code)
                sample = wait(lambda: js('window.palLocalPerformance.done'), 'native operation did not update original canvas', 8)
                assert sample['trusted']
                samples.append(sample)
                wait(lambda: room()['game']['data']['phase'] == phase and room()['game']['data']['menu']['kind'] == kind, 'authority did not converge')
                time.sleep(.15)
        values = [sample['responseMS'] for sample in samples]
        stats = {'p50':quantile(values,.5), 'p95':quantile(values,.95), 'max':max(values)}
        events = trace()
        case = {'delayMS': delay, 'samples':samples, 'responseMS':stats, 'network':events, 'passed':all(stats[key] <= budget for key,budget in report['budgets'].items())}
        report['cases'].append(case)
        browser('screenshot', out/('rtt-'+str(delay*2)+'.png'))
        print(json.dumps({'delayMS':delay, 'responseMS':stats, 'passed':case['passed']}), flush=True)
    report['passed'] = all(case['passed'] for case in report['cases'])
    proxy.stdin.write(json.dumps({'delayMS':100})+'\n')
    proxy.stdin.flush()
    assert json.loads(proxy.stdout.readline())['delayMS'] == 100
    browser('set', 'viewport', 320, 820)
    endpoint = browser('get','cdp-url')['cdpUrl']
    device = subprocess.Popen(['node','scripts/browser_device.mjs',endpoint,url,'320','820','--keep'],cwd=ROOT,env=env,text=True,stdout=subprocess.PIPE)
    assert device.stdout.readline().strip() == 'ready'
    def touch(mode, x, y):
        px, py = js('(()=>{const r=document.querySelector("[data-pal-board] canvas").getBoundingClientRect();return [r.x+'+str(x)+'*r.width/320,r.y+'+str(y)+'*r.height/200]})()')
        subprocess.run(['node','games/pal/tests/screen-touch.mjs',endpoint,url,mode,*map(str,[px,py,px,py])],cwd=ROOT,env=env,check=True,capture_output=True,text=True,timeout=15)
    before = room()['game']['data']
    cx, cy = before['x']-before['cameraX'], before['y']-before['cameraY']-14
    touch('cancel',cx-12,cy+32)
    time.sleep(.35)
    cancelled = room()['game']['data']
    assert [cancelled['x'],cancelled['y']] == [before['x'],before['y']]
    touch('pinch',cx,cy)
    time.sleep(.2)
    assert room()['game']['data']['phase'] == 'field'
    touch('two',cx,cy)
    wait(lambda: room()['game']['data']['phase'] == 'menu', 'two finger original menu failed')
    js('window.palLocalPerformance.arm('+json.dumps({'phase':'menu','kind':1,'selected':3,'trigger':'pointerup'})+')')
    touch('tap',24,93)
    selection = wait(lambda: js('window.palLocalPerformance.done'), 'touch navigation waited for server confirmation', 5)
    assert selection['responseMS'] <= 100, selection
    wait(lambda: room()['game']['data']['menu']['selected'] == 3, 'touch navigation did not converge')
    browser('press','Backspace')
    wait(lambda: room()['game']['data']['phase'] == 'field', 'phone menu did not close')
    report['phone'] = {'cancelledPreviewPreservedPosition': True, 'pinchNoCommand': True, 'twoFingerMenu': True, 'deepSelection': selection, 'canvas': js('(()=>{const r=document.querySelector("[data-pal-board] canvas").getBoundingClientRect();return [r.width,r.height]})()')}
    assert report['phone']['canvas'] == [320,200]
    assert not js('!!document.querySelector("[data-pal-direction],[data-pal-action],[data-pal-controls] button")')
    proxy.stdin.write(json.dumps({'pauseUp':True,'pauseDown':True})+'\n')
    proxy.stdin.flush()
    assert json.loads(proxy.stdout.readline())['pauseUp']
    browser('press','Backspace')
    wait(lambda:js('document.querySelector("[data-pal-board]").dataset.palPhase==="menu"'), 'local menu did not respond during outage', 3)
    wait(lambda:js('document.querySelector("[data-pal-board]").dataset.palPhase==="field"'), 'prediction did not roll back at timeout', 3)
    proxy.stdin.write(json.dumps({'pauseUp':False,'pauseDown':False})+'\n')
    proxy.stdin.flush()
    assert not json.loads(proxy.stdout.readline())['pauseUp']
    wait(lambda:js('JSON.parse(document.querySelector("[data-pal-board]").dataset.palLocalState).ready'), 'outage did not restore local replica')
    assert room()['game']['data']['phase'] == 'field'
    report['outage'] = {'localResponse':True,'boundedRollback':True,'staleInputDiscarded':True}
    browser('screenshot',out/'phone-recovered.png')
except BaseException:
    report['failure'] = {'room':room(), 'dom':js('document.querySelector("[data-pal-board]")?.outerHTML'), 'probe':js('({state:window.palLocalPerformance.debug(),events:window.palLocalPerformance.events})'), 'events':trace() if observer else []}
    browser('screenshot', out/'failure.png')
    raise
finally:
    (out/'result.json').write_text(json.dumps(report, ensure_ascii=False, indent=2)+'\n')
    if observer:
        observer.terminate()
        observer.communicate(timeout=5)
    if device:
        device.terminate()
        device.wait(timeout=5)
    try:
        browser('close')
    finally:
        proxy.terminate()
        proxy.communicate(timeout=5)
        shutil.rmtree(out/'profile', ignore_errors=True)
if not args.baseline:
    assert report['passed'], 'original full-operation response exceeds frozen budgets'
