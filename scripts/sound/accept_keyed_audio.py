#!/usr/bin/env python3
"""Native Chromium media acceptance for the shared keyed-track API.

Failure scenarios precede implementation in platform-audio-tracks.test.mjs.
This fixture uses the production GameAudio and preference reader with explicit
host lifecycle hooks; it does not run a game or claim physical-device audibility.
"""
import argparse
import functools
import hashlib
import http.server
import json
import math
import os
from pathlib import Path
import shutil
import struct
import subprocess
import threading
import traceback
import uuid
import wave

parser = argparse.ArgumentParser()
parser.add_argument('--out', required=True)
args = parser.parse_args()
root = Path(__file__).resolve().parents[2]
out = (root / args.out).resolve()
assert out.is_relative_to(root) and not out.exists(), out
out.mkdir(parents=True)
temporary = out / 'temporary'
temporary.mkdir()
run = 'ka' + uuid.uuid4().hex[:6]
sockets = root / '.s' / run
env = dict(os.environ, AGENT_BROWSER_SESSION=run,
           AGENT_BROWSER_SOCKET_DIR=str(sockets),
           AGENT_BROWSER_PROFILE=str(temporary / 'profile'),
           AGENT_BROWSER_ARGS='--mute-audio,--disable-breakpad,--disable-crash-reporter',
           TMPDIR=str(temporary / 'tmp'))
Path(env['TMPDIR']).mkdir()
sockets.mkdir(parents=True)
report = {'passed': False, 'checks': [], 'scope': 'Production GameAudio with native Chromium HTMLAudioElement; explicit fixture storage/visibility hooks. No game rules, no physical iOS or audio-output capture.'}
opened = False
server = None

def browser(*args):
    result = subprocess.run(['agent-browser', *args, '--json'], cwd=root, env=env,
                            capture_output=True, text=True, timeout=45)
    assert result.returncode == 0, result.stdout + result.stderr
    value = json.loads(result.stdout)
    assert value['success'], value
    return value['data']

def js(code):
    return browser('eval', code)['result']

def check(name, expression):
    value = js(expression)
    assert value.get('passed'), (name, value)
    report['checks'].append({'name': name, **value})
    return value

try:
    for name, duration, frequency in [('long', 8, 330), ('short', .22, 660)]:
        with wave.open(str(temporary / (name + '.wav')), 'wb') as wav:
            wav.setparams((1, 2, 8000, 0, 'NONE', 'not compressed'))
            wav.writeframes(b''.join(struct.pack('<h', round(math.sin(i * 2 * math.pi * frequency / 8000) * 800)) for i in range(round(duration * 8000))))
    bundle = subprocess.run(['node', '--input-type=module', '-e',
        "import{build}from'./web/node_modules/esbuild/lib/main.js';await build({entryPoints:['web/src/audio.ts','web/src/audio-preferences.ts'],outdir:process.argv[1],bundle:true,format:'esm',platform:'browser'});", str(temporary)], cwd=root, env=env, text=True, capture_output=True)
    assert bundle.returncode == 0, bundle.stderr
    (temporary / 'index.html').write_text('''<!doctype html><html lang="zh-CN"><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>平台音频通道验收</title><style>body{font:16px system-ui;color:#172626;background:#f4f3ed;padding:24px}button{font:inherit;margin:6px;padding:12px;color:#172626;background:white;border:1px solid #777}pre{white-space:pre-wrap;overflow-wrap:anywhere}</style><h1>平台音频通道验收</h1><button id="start">播放原生媒体</button><button id="mute">全局静音</button><button id="unmute">取消静音</button><button id="preview">试听</button><pre id="status">就绪</pre><script type="module">
import {GameAudio} from './audio.js';import {readAudioPreferences} from './audio-preferences.js';
const a=new GameAudio(),p=a.channel('room:fixture'),q=a.channel('room:other');a.mount();
const t=window.audioFixture={a,p,q,events:[],url:new URL('long.wav',location.href).href,short:new URL('short.wav',location.href).href};
const prefs=()=>{const v=readAudioPreferences();a.setEnabled(v.enabled);a.setVolume(v.volume);t.preferences=v};prefs();
window.addEventListener('storage',e=>{if(['wb_sound','wb_volume',null].includes(e.key)){prefs();t.events.push('storage')}});
document.addEventListener('visibilitychange',()=>{t.events.push(document.hidden?'hidden':'visible');if(document.hidden)a.stop()});
document.querySelector('#mute').onclick=()=>{localStorage.setItem('wb_sound','off');prefs()};
document.querySelector('#unmute').onclick=()=>{localStorage.setItem('wb_sound','on');prefs()};
document.querySelector('#preview').onclick=async()=>{t.preview=await a.play('preview');document.querySelector('#status').textContent='试听结果 '+t.preview};
document.querySelector('#start').onclick=async()=>{try{await a.unlock();t.started=await Promise.all([p.track('bg-pulse1',[t.url],{loop:true,playbackID:'song-1'}),p.track('bg-noise',[t.url],{loop:true,playbackID:'song-1'}),p.track('slot4',[t.url],{playbackID:'effect-1'})]);document.querySelector('#status').textContent='播放已开始';}catch(e){t.error=String(e);document.querySelector('#status').textContent=t.error}};
window.addEventListener('beforeunload',()=>a.dispose());t.ready=true;
</script></html>''')
    class Handler(http.server.SimpleHTTPRequestHandler):
        def log_message(self, *_):
            pass
    server = http.server.ThreadingHTTPServer(('127.0.0.1', 0), functools.partial(Handler, directory=temporary))
    threading.Thread(target=server.serve_forever, daemon=True).start()
    base = f'http://127.0.0.1:{server.server_port}'
    browser('--profile', str(temporary / 'profile'), 'open', base)
    opened = True
    browser('wait', '--fn', 'window.audioFixture?.ready===true')
    browser('snapshot', '-i')
    browser('click', '#start')
    browser('wait', '--fn', '!!window.audioFixture.started||!!window.audioFixture.error')
    check('native finite and independent looping tracks', '''(()=>{const t=audioFixture,m=[...document.querySelectorAll('audio[data-platform-track]')];return {passed:!t.error&&t.started?.every(Boolean)&&m.length===3&&m.every(x=>x instanceof HTMLAudioElement&&!x.paused),error:t.error,tracks:m.map(x=>({key:x.dataset.platformTrack,loop:x.loop,paused:x.paused}))}})()''')
    browser('wait', '--fn', "[...document.querySelectorAll('audio[data-platform-track]')].every(m=>m.currentTime>.1)")
    check('soft mute keeps time and source', '''(()=>{const t=audioFixture,m=document.querySelector('[data-platform-track="bg-pulse1"]');t.background=m;t.before=m.currentTime;t.p.muteTrack('bg-pulse1',true);return {passed:m.muted&&!m.paused,time:m.currentTime,src:m.currentSrc}})()''')
    browser('wait', '--fn', 'audioFixture.background.currentTime>audioFixture.before+.1')
    check('unmute keeps progressing position', '''(()=>{const t=audioFixture,m=t.background,before=m.currentTime;t.p.muteTrack('bg-pulse1',false);return {passed:!m.muted&&!m.paused&&m.currentTime>=before,time:m.currentTime}})()''')
    check('slot pause and separate pause prompt', '''(async()=>{const t=audioFixture;t.p.pauseTrack('bg-pulse1');t.pausedAt=t.background.currentTime;const ok=await t.p.track('pause-prompt',[t.short],{playbackID:'pause-1'});return {passed:ok&&t.background.paused&&document.querySelector('[data-platform-track="pause-prompt"]').loop===false,time:t.pausedAt}})()''')
    browser('wait', '--fn', '''document.querySelector('[data-platform-track="pause-prompt"]').ended''')
    check('slot resume preserves native position', '''(async()=>{const t=audioFixture,same=t.background.currentTime===t.pausedAt,ok=await t.p.resumeTrack('bg-pulse1');return {passed:same&&ok&&!t.background.paused&&t.background.currentTime>=t.pausedAt,time:t.background.currentTime}})()''')
    check('replacement stops only old slot and old owner cannot stop new owner', '''(async()=>{const t=audioFixture,old=document.querySelector('[data-platform-track="slot4"]');await t.p.track('slot4',[t.short],{playbackID:'effect-2'});await t.q.track('slot4',[t.url],{playbackID:'other-1'});t.p.stopTrack('slot4');const other=[...document.querySelectorAll('[data-platform-track="slot4"]')];return {passed:old.paused&&!old.isConnected&&!t.background.paused&&other.length===1&&!other[0].paused,oldRemoved:!old.isConnected,remaining:other.length}})()''')
    browser('click', '#mute');browser('click', '#preview')
    check('global mute stops all and preview cannot unmute', '''(()=>{const t=audioFixture;return {passed:!t.a.enabled&&t.preview===false&&[...document.querySelectorAll('audio[data-platform-audio]')].every(m=>m.paused),preview:t.preview}})()''')
    browser('click', '#unmute')
    check('unmute and old snapshot IDs cannot restart tracks', '''(async()=>{const t=audioFixture,old=await t.p.track('bg-pulse1',[t.url],{loop:true,playbackID:'song-1'});return {passed:old===false&&[...document.querySelectorAll('audio[data-platform-audio]')].every(m=>m.paused),old}})()''')
    check('fresh accepted event can play; native fallback and finite end work', '''(async()=>{const t=audioFixture,ok=await t.p.track('fresh',['/missing.wav',t.short],{playbackID:'fresh'});return {passed:ok,ok}})()''')
    browser('wait', '--fn', '''document.querySelector('[data-platform-track="fresh"]').ended''')
    check('ended event does not repeat', '''(async()=>{const t=audioFixture;return {passed:!(await t.p.track('fresh',['/missing.wav',t.short],{playbackID:'fresh'}))}})()''')
    # A real second tab generates native visibility and storage events.
    js("audioFixture.p.track('hidden',[audioFixture.url],{playbackID:'hidden-1'})")
    browser('tab', 'new', base)
    browser('wait', '--fn', 'window.audioFixture?.ready===true')
    browser('click', '#mute')
    browser('tab', 't1')
    browser('wait', '--fn', "audioFixture.events.includes('storage')&&!audioFixture.a.enabled")
    check('native cross-tab preference and page hiding cancel voices', '''(()=>{const t=audioFixture;return {passed:t.events.includes('storage')&&t.events.includes('hidden')&&!t.a.enabled&&[...document.querySelectorAll('audio[data-platform-audio]')].every(m=>m.paused),events:t.events,visibility:document.visibilityState}})()''')
    browser('click', '#unmute')
    check('old hidden snapshot does not replay after becoming visible', '''(async()=>{const t=audioFixture;return {passed:!(await t.p.track('hidden',[t.url],{playbackID:'hidden-1'}))}})()''')
    check('owner departure drops tracks and pending resume', '''(async()=>{const t=audioFixture;await t.p.track('leave',[t.url],{playbackID:'leave-1'});t.p.pauseTrack('leave');t.p.stop();return {passed:!(await t.p.resumeTrack('leave'))&&[...document.querySelectorAll('audio[data-platform-track]')].every(m=>m.paused)}})()''')
    report['audioSession'] = js("navigator.audioSession?.type??'not available in this Chromium'")
    js("document.querySelector('#status').textContent='全部通道验收通过：替换、静音时间线、暂停恢复、全局静音、跨标签和离桌清理';true")
    browser('set', 'viewport', '1100', '780')
    browser('screenshot', str(out / 'desktop.png'))
    browser('set', 'viewport', '320', '740')
    browser('screenshot', str(out / 'mobile-320.png'))
    report['passed'] = True
except BaseException:
    report['error'] = traceback.format_exc()
    if opened:
        try:
            report['browserErrors'] = browser('errors')
            report['browserConsole'] = browser('console')
            report['page'] = js('({url:location.href,title:document.title,text:document.body.innerText})')
        except BaseException:
            pass
finally:
    if opened:
        try: browser('close')
        except BaseException: report['passed'] = False
    if server: server.shutdown(); server.server_close()
    for path in [temporary, sockets]:
        if path.exists(): shutil.rmtree(path)
    report['sources'] = {str(p.relative_to(root)): hashlib.sha256(p.read_bytes()).hexdigest() for p in [root / 'web/src/audio.ts', root / 'web/src/audio-preferences.ts', Path(__file__)]}
    (out / 'result.json').write_text(json.dumps(report, ensure_ascii=False, indent=2) + '\n')
    print(json.dumps(report, ensure_ascii=False, indent=2))
raise SystemExit(0 if report['passed'] else 1)
