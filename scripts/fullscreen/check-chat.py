#!/usr/bin/env python3
"""Production-component browser acceptance; failure scenarios in CHAT-SCENARIOS.md."""
import argparse
import functools
import hashlib
import http.server
import json
import os
from pathlib import Path
import shutil
import subprocess
import threading
import time
import uuid

parser = argparse.ArgumentParser()
parser.add_argument('--out', required=True)
args = parser.parse_args()
root = Path(__file__).resolve().parents[2]
out = (root / args.out).resolve()
assert out.is_relative_to(root) and not out.exists()
out.mkdir(parents=True)
temp = out / 'temporary'
session = 'chat' + uuid.uuid4().hex[:5]
# A relative socket directory keeps macOS Unix socket paths below its limit.
socket_dir = Path('.s') / session
env = dict(os.environ)
for key, directory in {
    'TMPDIR': temp / 'tmp',
    'AGENT_BROWSER_PROFILE': temp / 'profile',
    'AGENT_BROWSER_SOCKET_DIR': socket_dir,
}.items():
    (root / directory).mkdir(parents=True, exist_ok=True)
    env[key] = str(directory)
env.update(AGENT_BROWSER_SESSION=session,
           AGENT_BROWSER_ARGS='--mute-audio,--disable-breakpad,--disable-crash-reporter')
report = {'passed': False, 'cases': [], 'command': 'python3 -B scripts/fullscreen/check-chat.py --out ' + args.out}
server = None


def browser(*arguments):
    result = subprocess.run(['agent-browser', *arguments, '--json'], cwd=root,
                            env=env, text=True, capture_output=True, timeout=45)
    assert result.returncode == 0, result.stdout + result.stderr
    data = json.loads(result.stdout)
    assert data['success'], data
    return data['data']


def js(code):
    return browser('eval', code)['result']


def wait(predicate):
    until = time.monotonic() + 12
    while time.monotonic() < until:
        if predicate():
            return
        time.sleep(.05)
    raise AssertionError('Timed out waiting for component state')


def native_click(selector, base, mobile):
    result = subprocess.run(['node', 'scripts/fullscreen/tap.mjs',
                             browser('get', 'cdp-url')['cdpUrl'], base, selector,
                             'touch' if mobile else 'mouse'], cwd=root, env=env,
                            text=True, capture_output=True, timeout=20)
    assert result.returncode == 0, result.stderr
    return json.loads(result.stdout)


try:
    subprocess.run(['node', 'scripts/fullscreen/build-chat-harness.mjs', str(temp / 'web')],
                   cwd=root, env=env, check=True, capture_output=True, text=True)
    class Handler(http.server.SimpleHTTPRequestHandler):
        def log_message(self, *_args):
            pass
    server = http.server.ThreadingHTTPServer(('127.0.0.1', 0),
             functools.partial(Handler, directory=str(temp / 'web')))
    threading.Thread(target=server.serve_forever, daemon=True).start()
    base = 'http://127.0.0.1:' + str(server.server_port)
    for mobile in [False, True]:
        for mode in ['native', 'page']:
            browser('open', base)
            browser('set', 'viewport', '320' if mobile else '1280', '700' if mobile else '900')
            browser('wait', '.chat-toggle')
            assert not js('!!document.querySelector("[data-enter]")')
            browser('click', '.chat-toggle')
            browser('fill', 'textarea', '跨全屏保留的草稿')
            js('window.chatInput=document.querySelector("textarea"); window.chatNode=document.querySelector(".table-chat"); true')
            browser('click', '.chat-toggle')
            js('chatFixture.incoming("未读消息"); true')
            wait(lambda: js('document.querySelector(".chat-toggle b")?.textContent === "1"'))
            browser('click', '[data-start]')
            browser('wait', '[data-enter]:not([disabled])')
            if mode == 'page':
                js('document.querySelector("[data-game-fullscreen-surface]").requestFullscreen=()=>Promise.reject(Error("test denied")); true')
            pointer = native_click('[data-enter]', base, mobile)
            wait(lambda: js('document.querySelector("[data-game-fullscreen-surface]").dataset.roomFullscreen') == mode)
            assert js('''(()=>{const root=document.querySelector('[data-room-fullscreen]');
              return root.contains(document.querySelector('.table-chat')) &&
                !root.querySelector('.topbar,.room-heading,.players-strip,.room-tools') &&
                document.querySelectorAll('.table-chat').length===1 &&
                window.chatNode===document.querySelector('.table-chat') &&
                !document.querySelector('.table-chat').closest('[inert]') &&
                document.querySelector('.chat-toggle b')?.textContent==='1'})()'''), 'chat entry must move inside fullscreen without losing unread state'
            if mode == 'native':
                assert js('document.fullscreenElement===document.querySelector("[data-room-fullscreen]")')
            native_click('.chat-toggle', base, mobile)
            assert js('document.querySelector("textarea").value === "跨全屏保留的草稿"')
            js('window.chatInput=document.querySelector("textarea"); chatFixture.gameKeys.length=0; chatFixture.releaseKeys.length=0; true')
            browser('focus', 'textarea')
            browser('press', 'a')
            assert js('chatFixture.gameKeys.length===0 && chatFixture.releaseKeys.includes("a")'), 'chat typing must not reach game keydown; key releases still propagate'
            measurements = js('''(()=>{const p=document.querySelector('.chat-panel'),b=document.querySelector('.chat-toggle');
              return [p,b].map(e=>{const r=e.getBoundingClientRect(); const x=r.x+r.width/2,y=r.y+r.height/2;
                return {x:r.x,y:r.y,w:r.width,h:r.height,visible:r.left>=0&&r.top>=0&&r.right<=innerWidth&&r.bottom<=innerHeight,
                  hit:e.contains(document.elementFromPoint(x,y))}})})()''')
            assert all(item['visible'] and item['hit'] for item in measurements), measurements
            browser('screenshot', str(out / f'{mode}-{"mobile" if mobile else "desktop"}.png'))
            draft_before_profile = js('document.querySelector("textarea").value')
            native_click('.chat-message .player-name', base, mobile)
            wait(lambda: js('!!document.querySelector("[role=dialog][aria-label=玩家页面]")'))
            profile = js('''(()=>{const dialog=document.querySelector('[role=dialog][aria-label=玩家页面]'),
              button=dialog.querySelector('button'),r=button.getBoundingClientRect();
              return {inside:document.querySelector('[data-room-fullscreen]').contains(dialog),
                hit:button.contains(document.elementFromPoint(r.x+r.width/2,r.y+r.height/2)),
                notInert:!dialog.closest('[inert]'),text:dialog.innerText}})()''')
            assert profile['inside'] and profile['hit'] and profile['notInert'], profile
            assert '同桌玩家' in profile['text']
            wait(lambda: js('document.querySelector("[role=dialog][aria-label=玩家页面]").getAnimations().every(a=>a.playState!=="running")'))
            browser('screenshot', str(out / f'profile-{mode}-{"mobile" if mobile else "desktop"}.png'))
            native_click('[role=dialog][aria-label=玩家页面] [aria-label=关闭]', base, mobile)
            wait(lambda: js('!document.querySelector("[role=dialog][aria-label=玩家页面]")'))
            assert js('document.querySelector("textarea").value') == draft_before_profile
            assert js('window.chatInput===document.querySelector("textarea") && !!document.querySelector("[data-room-fullscreen]")')
            browser('fill', 'textarea', '一次发送')
            browser('press', 'Enter')
            wait(lambda: js('chatFixture.sent.length===1'))
            native_click('[data-room-fullscreen] [data-fullscreen-toggle]', base, mobile)
            wait(lambda: js('!document.querySelector("[data-room-fullscreen]")'))
            assert js('window.chatInput===document.querySelector("textarea") && document.querySelector("textarea").value==="一次发送"')
            assert js('!document.querySelector("[inert]") && document.documentElement.style.overflow!=="hidden"')
            browser('focus', 'textarea')
            browser('press', 'Enter')
            assert js('chatFixture.sent.length===1'), 'pending send must survive fullscreen exit'
            js('chatFixture.finishSend(); true')
            wait(lambda: js('document.querySelector("textarea").value===""'))
            assert js('document.querySelectorAll(".chat-message.mine").length===1')
            browser('click', '.chat-toggle')
            js('chatFixture.incoming("退出后未读"); true')
            wait(lambda: js('document.querySelector(".chat-toggle b")?.textContent==="1"'))
            native_click('[data-enter]', base, mobile)
            wait(lambda: js('!!document.querySelector("[data-room-fullscreen]")'))
            browser('press', 'Escape')
            wait(lambda: js('!document.querySelector("[data-room-fullscreen]")'))
            assert js('document.querySelector(".chat-toggle b")?.textContent==="1" && !document.querySelector("[inert]")')
            assert js('getComputedStyle(document.querySelector(".table-chat")).position') == ('relative' if mobile else 'fixed')
            native_click('[data-enter]', base, mobile)
            wait(lambda: js('!!document.querySelector("[data-room-fullscreen]")'))
            js('chatFixture.leave(); true')
            wait(lambda: js('!document.querySelector("[data-room-fullscreen],.table-chat,[inert]") && !document.fullscreenElement'))
            assert js('document.documentElement.style.overflow!=="hidden"')
            report['cases'].append({'mode': mode, 'mobile': mobile, 'pointer': pointer,
                                    'geometry': measurements, 'profile': profile, 'passed': True})
            print('PASS', mode, 'mobile' if mobile else 'desktop', flush=True)
    report['passed'] = True
finally:
    try:
        browser('close')
    except Exception:
        pass
    if server:
        server.shutdown()
        server.server_close()
    for owned in [temp, root / socket_dir]:
        if owned.exists():
            shutil.rmtree(owned)
    report['sourceSHA256'] = {str(p.relative_to(root)): hashlib.sha256(p.read_bytes()).hexdigest()
        for p in [root / 'web/src/chat.tsx', root / 'web/src/ui/room-fullscreen.tsx',
                  root / 'web/src/ui/room-fullscreen.css', Path(__file__),
                  root / 'scripts/fullscreen/chat-harness.tsx', root / 'web/src/main.tsx',
                  root / 'web/src/ui/game-overlay-root.ts']}
    (out / 'result.json').write_text(json.dumps(report, ensure_ascii=False, indent=2) + '\n')
