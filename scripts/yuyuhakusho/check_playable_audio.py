"""Real Board, Go/Wasm rules and native FLAC media; reproducible acceptance."""
from pathlib import Path
import argparse, functools, hashlib, http.server, json, os, statistics, subprocess, threading, urllib.request

ROOT = Path(__file__).resolve().parents[2]
parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument('--label', required=True)
parser.add_argument('--red', action='store_true')
args = parser.parse_args()
OUT = ROOT/'games/yuyuhakusho/verification/playable-audio'/args.label
TMP = ROOT/'.local/md-yuyuhakusho/platform-web'
MEDIA = ROOT/'.local/md-yuyuhakusho/playable-audio/media'
OUT.mkdir(parents=True, exist_ok=False)
socket = (ROOT.parents[1] if ROOT.parent.name == 'worktrees' else ROOT)/'.local/au'
socket.mkdir(parents=True, exist_ok=True)
env = dict(os.environ, TMPDIR=str(TMP/'tmp'), GOTMPDIR=str(TMP/'tmp'), GOCACHE=str(ROOT/'.local/go-cache'), GOMODCACHE=str(ROOT/'.local/go-mod'), TEST_TELEMETRY_DIR=str(ROOT/'.local/go-telemetry'), AGENT_BROWSER_SOCKET_DIR=str(socket), AGENT_BROWSER_SESSION='aud'+hashlib.sha256(args.label.encode()).hexdigest()[:6])
report = {'pass': False, 'scope': 'Actual Board + real independent Go module/Wasm + platform native audio. Authentication/socket acceptance remains separate.'}
def browser(*command):
    r = subprocess.run(['agent-browser', *command, '--json'], cwd=ROOT, env=env, text=True, capture_output=True, timeout=55)
    if r.returncode: raise RuntimeError(r.stdout+r.stderr)
    x = json.loads(r.stdout)
    if not x['success']: raise RuntimeError(x)
    return x['data']
def js(code): return browser('eval', code)['result']
def wait(code): return browser('wait', '--fn', code)
def native_start(name):
    endpoint = browser('get', 'cdp-url')['cdpUrl']
    actions = [{'type':'keyDown','key':' ','code':'Space','windowsVirtualKeyCode':32,'wait':100}, {'type':'keyUp','key':' ','code':'Space','windowsVirtualKeyCode':32,'wait':150}]
    subprocess.run(['node', str(ROOT/'scripts/yuyuhakusho/native_platform_input.mjs'), endpoint, report['url'].split('?')[0], json.dumps(actions)], cwd=ROOT, env=env, check=True, stdout=(OUT/(name+'.json')).open('w'))
def observed(): return js('({events:yuyuAudioProbe.events,calls:yuyuAudioProbe.calls,visibility:yuyuAudioProbe.visibility,media:yuyuAudioProbe.elements.map(m=>({src:m.currentSrc,paused:m.paused,time:m.currentTime,error:m.error?.message})),error:yuyuBoardProbe.error,phase:yuyuBoardProbe.authority.phase,frame:document.querySelector("[data-yuyu-board]").dataset.frame})')
host = None
proxy = None
opened = False
try:
    subprocess.run(['python3', '-B', str(ROOT/'scripts/yuyuhakusho/build_platform_web.py')], cwd=ROOT, env=env, check=True, stdout=(OUT/'build.log').open('w'), stderr=subprocess.STDOUT)
    subprocess.run([str(ROOT/'web/node_modules/.bin/esbuild'), 'scripts/yuyuhakusho/platform_board_harness.tsx', '--bundle', '--format=esm', '--jsx=automatic', '--alias:react=./web/node_modules/react', '--alias:react-dom=./web/node_modules/react-dom', '--loader:.svg=file','--loader:.png=file', '--outfile='+str(TMP/'harness.js')], cwd=ROOT, env=env, check=True, stdout=(OUT/'harness.log').open('w'), stderr=subprocess.STDOUT)
    host = subprocess.Popen([str(TMP/'host'), '-build', str(TMP), '-root', str(ROOT)], cwd=ROOT, env=env, stdout=subprocess.PIPE, stderr=(OUT/'host.log').open('w'), text=True)
    upstream = host.stdout.readline().strip()
    class Proxy(http.server.SimpleHTTPRequestHandler):
        def do_GET(self):
            if self.path.startswith('/yuyuhakusho/audio/'):
                self.path = '/'+self.path.removeprefix('/yuyuhakusho/audio/')
                try: return super().do_GET()
                except (BrokenPipeError, ConnectionResetError): return
            self.forward()
        def do_POST(self): self.forward()
        def forward(self):
            body = self.rfile.read(int(self.headers.get('Content-Length', 0))) if self.command == 'POST' else None
            req = urllib.request.Request(upstream.rstrip('/')+self.path, data=body, method=self.command, headers={'Content-Type': self.headers.get('Content-Type', 'application/json')})
            try:
                with urllib.request.urlopen(req) as response:
                    self.send_response(response.status)
                    for key in ['Content-Type', 'Content-Length', 'Cache-Control']:
                        if key in response.headers: self.send_header(key, response.headers[key])
                    self.end_headers(); self.wfile.write(response.read())
            except Exception as error:
                self.send_error(502, str(error))
        def log_message(self, *_): pass
    proxy = http.server.ThreadingHTTPServer(('127.0.0.1', 0), functools.partial(Proxy, directory=str(MEDIA)))
    threading.Thread(target=proxy.serve_forever, daemon=True).start()
    url = 'http://127.0.0.1:'+str(proxy.server_port)+'/?audio'
    report['url'] = url
    browser('--profile', str(TMP/('audio-profile-'+args.label)), 'open', url); opened = True
    browser('set', 'viewport', '1100', '1000')
    browser('snapshot', '-i')
    browser('click', '#start-board')
    wait('yuyuBoardProbe.ready || !!yuyuBoardProbe.error')
    assert not js('yuyuBoardProbe.error'), observed()
    js('history.replaceState(null,"","/");true')
    wait('yuyuBoardProbe.authority.phase === "playing"')
    wait('yuyuAudioProbe.events.some(e=>e.kind==="playing" && e.src.includes("82.flac"))')
    wait('yuyuAudioProbe.calls.some(c=>c.kind==="effect")')
    report['initial'] = observed()
    assert not any(e['kind'] == 'error' for e in report['initial']['events']), report['initial']
    # A repeated authority replay must never issue the same effect instance twice.
    js('yuyuBoardProbe.syncDelayMS=300;yuyuBoardProbe.gapUpdates=3;true')
    wait('yuyuBoardProbe.syncCount>=2')
    wait('document.querySelector("[data-yuyu-board]").dataset.connection==="connected"')
    report['recovery'] = observed()
    ids = [c['args'][2]['playbackID'] for c in report['recovery']['calls'] if c['kind'] == 'effect']
    assert len(ids) == len(set(ids)), 'correction/reconnect replayed a sound'
    # The uniform platform preference gates native playback and discards old effects.
    browser('click', '#audio-toggle')
    wait('yuyuAudioProbe.elements.every(m=>m.paused)')
    report['muted'] = observed()
    browser('click', '#audio-toggle')
    wait('yuyuAudioProbe.events.filter(e=>e.kind==="playing"&&e.src.includes("82.flac")).length>=2')
    report['unmuted'] = observed()
    # Native Start controls still pause/resume the game and its audio together.
    browser('focus', '[data-yuyu-board]')
    native_start('native-pause')
    wait('yuyuBoardProbe.authority.phase === "paused"')
    wait('yuyuAudioProbe.elements.every(m=>m.paused)')
    report['paused'] = observed()
    native_start('native-resume')
    wait('yuyuBoardProbe.authority.phase === "playing"')
    wait('yuyuAudioProbe.elements.some(m=>m.currentSrc.includes("82.flac")&&!m.paused)')
    report['resumed'] = observed()
    # Real tab visibility, without substituting document.hidden.
    browser('tab', 'new', 'about:blank')
    browser('wait', '100')
    browser('tab', 't1')
    wait('yuyuAudioProbe.visibility.some(e=>e.hidden)&&yuyuAudioProbe.visibility.some(e=>!e.hidden)')
    report['visibleAgain'] = observed()
    hidden = next(v['at'] for v in report['visibleAgain']['visibility'] if v['hidden'])
    visible = next(v['at'] for v in report['visibleAgain']['visibility'] if not v['hidden'] and v['at'] > hidden)
    assert not any(c['kind']=='effect' and hidden <= c['at'] < visible for c in report['visibleAgain']['calls'])
    assert any(c['kind']=='stop' and hidden <= c['at'] <= visible for c in report['visibleAgain']['calls'])
    wait('document.querySelector("[data-yuyu-board]").dataset.connection==="connected"&&yuyuAudioProbe.elements.some(m=>m.currentSrc.includes("82.flac")&&!m.paused)')
    endpoint = browser('get', 'cdp-url')['cdpUrl']
    js('history.replaceState(null,"","/");true')
    started = js('yuyuBoardProbe.paints=[];performance.now()')
    subprocess.run(['node', str(ROOT/'scripts/yuyuhakusho/native_platform_input.mjs'), endpoint, url.split('?')[0], json.dumps([{'type':'keyUp','key':'j','code':'KeyJ','windowsVirtualKeyCode':74,'wait':2500}])], cwd=ROOT, env=env, check=True, stdout=(OUT/'native-performance.json').open('w'))
    measured = js('({paints:yuyuBoardProbe.paints,at:performance.now()})')
    paints = measured['paints']
    intervals = [b['at']-a['at'] for a,b in zip(paints,paints[1:]) if b['frame'] != a['frame']]
    report['performance'] = {'frames':len(paints),'elapsedms':measured['at']-started, 'fps':len(paints)*1000/(measured['at']-started), 'p95ms':sorted(intervals)[int(.95*(len(intervals)-1))]}
    assert report['performance']['fps'] >= 50 and report['performance']['p95ms'] <= 40, report['performance']
    browser('screenshot', str(OUT/'desktop.png'))
    report['final'] = observed()
    assert not report['final']['error']
    report['pass'] = True
except Exception as error:
    report['error'] = repr(error)
    if opened:
        try: report['diagnostic'] = observed(); browser('screenshot', str(OUT/'failure.png'))
        except Exception: pass
    if not args.red: raise
    assert opened and report.get('diagnostic', {}).get('phase') == 'playing' and not report['diagnostic']['calls'], report
    report['expected_missing_playback_red'] = True
finally:
    report['source_sha256'] = {str(p.relative_to(ROOT)):hashlib.sha256(p.read_bytes()).hexdigest() for p in [Path(__file__), ROOT/'scripts/yuyuhakusho/platform_board_harness.tsx']}
    (OUT/'result.json').write_text(json.dumps(report, ensure_ascii=False, indent=2)+'\n')
    if opened: browser('close')
    if proxy: proxy.shutdown(); proxy.server_close()
    if host: host.terminate(); host.wait(timeout=10)
print(json.dumps({'pass':report['pass'], 'red':report.get('expected_missing_playback_red', False), 'result':str(OUT/'result.json')}))
