#!/usr/bin/env python3
"""Full catalog/terrain E2E specified before the expanded production UI."""
import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess
import time
import uuid

ROOT = Path(__file__).resolve().parents[3]
OUT = ROOT / '.local/pvz/frontend-full'
OUT.mkdir(parents=True, exist_ok=True)
BASE = os.environ.get('PVZ_BROWSER_URL', '').rstrip('/')
ENV = dict(os.environ, AGENT_BROWSER_SESSION='f' + uuid.uuid4().hex[:3],
           AGENT_BROWSER_SOCKET_DIR=str(ROOT / '.local/ab'),
           AGENT_BROWSER_PROFILE=str(OUT / 'profile'), TMPDIR=str(OUT / 'tmp'),
           AGENT_BROWSER_ARGS='--mute-audio,--disable-breakpad,--disable-crash-reporter')
for key in ['AGENT_BROWSER_SOCKET_DIR', 'AGENT_BROWSER_PROFILE', 'TMPDIR']:
    Path(ENV[key]).mkdir(parents=True, exist_ok=True)
REPORT = {'passed': False, 'command': 'PVZ_BROWSER_URL=<local URL> python3 -B games/pvz/tests/frontend-full-browser.py',
          'scope': 'production catalog and terrains; authenticated normal rooms; native keyboard/mouse; mobile viewport simulation',
          'musicSkipped': bool(os.environ.get('PVZ_BROWSER_SKIP_MUSIC')), 'cases': [], 'sourceSHA256': {str(p.relative_to(ROOT)): hashlib.sha256(p.read_bytes()).hexdigest()
                                     for p in (ROOT / 'games/pvz/web').glob('*') if p.is_file()}}
OPENED = False

def b(*args):
    result = subprocess.run(['agent-browser', *map(str, args), '--json'], cwd=ROOT, env=ENV,
                            text=True, capture_output=True, timeout=60)
    assert result.returncode == 0, result.stdout + result.stderr
    record = json.loads(result.stdout); assert record['success'], record
    return record['data']

def js(code): return b('eval', code)['result']

def api(path, body=None):
    options = '' if body is None else ',{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify(' + json.dumps(body) + ')}'
    result = js('(async()=>{const r=await fetch(' + json.dumps('/api' + path) + options + ');return {status:r.status,body:await r.json()}})()')
    assert 200 <= result['status'] < 300, result
    return result['body']

def room(): return api('/state')['room']

def command(kind, **extra):
    state = room()
    return api('/rooms/' + state['id'], dict(type=kind, version=state['version'], nonce=uuid.uuid4().hex, **extra))

def wait(fn, seconds=40):
    until = time.monotonic() + seconds
    while time.monotonic() < until:
        if fn(): return
        time.sleep(.1)
    raise AssertionError('condition timed out')

def audit_text():
    result = js('''(()=>{const rgb=s=>(s.match(/[\\d.]+/g)||[]).map(Number),lum=c=>c.slice(0,3).map(v=>v/255).map(v=>v<=.04045?v/12.92:((v+.055)/1.055)**2.4).reduce((sum,v,i)=>sum+v*[.2126,.7152,.0722][i],0);return [...document.querySelectorAll('[data-pvz-clock],[data-pvz-money],.pvz-seed-name,.pvz-seed-cost,.pvz-book-grid>button>strong,.pvz-book-grid>button>span,.pvz-book-grid>button>p,.pvz-book-grid>button>small,.pvz-book-grid>button>em,.pvz-book-slots button,.pvz-book-footer button')].filter(e=>e.textContent.trim()&&e.getBoundingClientRect().width>0).map(e=>{let bg=[246,245,239],opacity=1;const chain=[];for(let n=e;n;n=n.parentElement)chain.unshift(n);for(const n of chain){const s=getComputedStyle(n),a=rgb(s.backgroundColor),o=a[3]??1;bg=a.slice(0,3).map((v,i)=>v*o+bg[i]*(1-o));opacity*=parseFloat(s.opacity)}const fg=lum(rgb(getComputedStyle(e).color)),back=lum(bg);return{text:e.textContent,contrast:(Math.max(fg,back)+.05)/(Math.min(fg,back)+.05),opacity}})})()''')
    assert all(item['contrast'] >= 4.5 and item['opacity'] >= .99 for item in result), result
    return result

try:
    assert BASE, 'PVZ_BROWSER_URL must point to a local production-module server'
    OPENED = True; b('open', BASE)
    api('/register', {'name': '完整植物验收' + uuid.uuid4().hex[:6], 'password': 'pvz-browser-local', 'invite': 'pvz-browser'})
    b('reload'); b('wait', '--text', '创建房间')
    if os.environ.get('PVZ_BROWSER_LEGACY_ASSETS'):
        assert api('/state')['assetsBaseURL'] == os.environ['PVZ_BROWSER_LEGACY_ASSETS']
    for scene, rows, count, width, height in [('day', 5, 2, 1440, 960), ('night', 5, 2, 320, 760), ('pool', 6, 4, 320, 760), ('fog', 6, 2, 1440, 960), ('roof', 5, 4, 320, 760), ('roofnight', 5, 2, 1440, 960)]:
        b('set', 'viewport', width, height)
        api('/rooms', {'kind': 'pvz', 'name': '完整植物大战僵尸 ' + scene, 'capacity': count,
                       'options': {'mode': 'duel' if count == 2 else 'team', 'teams': [i % 2 for i in range(count)],
                                   'scene': scene, 'roundSeconds': 360, 'prepSeconds': 30}})
        for _ in range(count - 1): command('add_bot')
        command('ready'); command('start')
        b('wait', '[data-pvz-board][data-ready="true"]')
        wait(lambda: not room().get('loading'))
        initial = room()['game']['data']
        assert not js('performance.getEntriesByType("resource").some(entry=>entry.name.includes("legacy-assets.invalid"))'), 'PvZ used legacy asset prefix'
        assert initial['map']['id'] == scene and initial['map']['rows'] == rows, initial['map']
        assert len(initial['catalog']['plants']) == 49
        assert len(initial['catalog']['zombies']) == 33
        assert 1 <= len(initial['players'][0]['deck']) <= 10
        music = []
        if not os.environ.get("PVZ_BROWSER_SKIP_MUSIC"):
            # Setup uses HTTP, so exercise a real game control before checking
            # native media; normal players already gesture on Ready/Start.
            b('click', 'button[aria-label="卡册"]')
            b('click', '[role="dialog"] button[aria-label="关闭"]')
            wait(lambda: js('[...document.querySelectorAll("audio[data-platform-track^=pvz-music-]")].some(media=>!media.paused&&media.readyState>=2)'))
            music = js('[...document.querySelectorAll("audio[data-platform-track^=pvz-music-]")].map(media=>({key:media.dataset.platformTrack,src:media.currentSrc,time:media.currentTime,paused:media.paused}))')
            if scene == 'day':
                js('window.__pvzMusic=document.querySelector("audio[data-platform-track^=pvz-music-]");true')
                time.sleep(1.5)
                assert js('window.__pvzMusic===document.querySelector("audio[data-platform-track^=pvz-music-]")&&window.__pvzMusic.currentTime>1'), 'snapshots restarted music'
                b('click', 'button[aria-label="声音控制"]'); b('click', '[data-sound-mute]')
                wait(lambda: js('[...document.querySelectorAll("audio[data-platform-track^=pvz-music-]")].every(media=>media.paused||media.muted)'))
                b('click', '[data-sound-mute]'); b('press', 'Escape')
                wait(lambda: js('[...document.querySelectorAll("audio[data-platform-track^=pvz-music-]")].some(media=>!media.paused&&media.currentTime>1)'))
        assert js('document.querySelectorAll("[data-pvz-card]").length<=10')
        assert js(f'document.querySelectorAll("[data-pvz-cell]").length==={rows * 9}')
        assert not js('document.documentElement.scrollWidth>innerWidth+1')
        if scene == 'day':
            b('click', '[data-pvz-card="sunflower"]')
            b('scrollintoview', '[data-pvz-cell="0:0"]')
            rectangle = js('document.querySelector("[data-pvz-cell=\\"0:0\\"]").getBoundingClientRect().toJSON()')
            b('mouse', 'move', round(rectangle['left'] + rectangle['width'] / 2), round(rectangle['top'] + rectangle['height'] / 2))
            b('mouse', 'down'); b('mouse', 'up')
            wait(lambda: any(plant['kind'] == 'sunflower' and plant['owner'] == 0 for plant in room()['game']['data']['plants']))
        b('click', 'button[aria-label="卡册"]')
        b('wait', '[role="dialog"][aria-label="植物大战僵尸卡册"]')
        b('wait', '--fn', '(()=>{const dialog=document.querySelector(".pvz-card-book");return !!dialog&&dialog.getAnimations().every(animation=>animation.playState!=="running")})()')
        assert js('document.querySelectorAll("[data-pvz-book-card]").length===49')
        assert js('[...document.querySelectorAll("[data-pvz-book-card]")].every(e=>e.textContent.includes("阳光")&&e.textContent.includes("秒"))')
        text_audit = audit_text()
        first = initial['players'][0]['deck'][0]
        if initial.get('rulesVersion', 0) >= 4:
            b('click', '.pvz-book-modes button:nth-child(2)')
        b('click', '[data-pvz-book-card=' + json.dumps(first) + ']')
        b('click', 'button[aria-label="保存快捷栏"]' if initial.get('rulesVersion', 0) >= 4 else 'button[aria-label="保存卡组"]')
        wait(lambda: first not in room()['game']['data']['players'][0]['deck'])
        b('click', 'button[aria-label="卡册"]')
        b('screenshot', OUT / f'{scene}-{width}-catalog.png')
        b('click', 'button[aria-label="查看僵尸目录"]')
        assert js('document.querySelectorAll("[data-pvz-book-card]").length===33')
        b('press', 'Escape')
        wait(lambda: not js('!!document.querySelector("[role=dialog]")'))
        if scene == 'day':
            before_collect = None
            pickup = None
            deadline = time.monotonic() + 45
            while time.monotonic() < deadline:
                snapshot = room()['game']['data']
                candidates = [item for item in snapshot['pickups'] if item['owner'] == 0 and item['autoAt'] - snapshot['tick'] > 40]
                if candidates:
                    before_collect, pickup = snapshot, candidates[0]
                    break
                time.sleep(.1)
            assert pickup is not None, 'no fresh owned pickup appeared'
            pickup_id = pickup['id']
            b('click', '[data-pvz-pickup="' + str(pickup_id) + '"]')
            wait(lambda: all(item['id'] != pickup_id for item in room()['game']['data']['pickups']))
            assert room()['game']['data']['players'][0]['money'] >= before_collect['players'][0]['money'] + pickup['amount']
        time.sleep(.3)
        assert not js('!!document.querySelector("[role=dialog]")'), 'snapshot reopened catalog'
        b('click', 'button[aria-label="全屏"]')
        b('wait', '[data-room-fullscreen]')
        b('click', 'button[aria-label="卡册"]')
        assert js('document.querySelector("[data-room-fullscreen]").contains(document.querySelector("[role=dialog]"))')
        b('wait', '--fn', '(()=>{const dialog=document.querySelector(".pvz-card-book");return !!dialog&&dialog.getAnimations().every(animation=>animation.playState!=="running")})()')
        b('screenshot', OUT / f'{scene}-{width}-fullscreen-catalog.png')
        b('click', '[role="dialog"] button[aria-label="关闭"]'); b('click', 'button[aria-label="退出全屏"]')
        b('scrollintoview', '[data-pvz-board]')
        b('screenshot', OUT / f'{scene}-{width}-field.png')
        b('screenshot', OUT / f'{scene}-{width}-field-full.png', '--full')
        REPORT['cases'].append({'scene': scene, 'rows': rows, 'players': count, 'viewport': [width, height],
                                'initialDeck': initial['players'][0]['deck'], 'map': initial['map'], 'music': music, 'textAudit': text_audit})
        command('close'); command('leave')
    REPORT['passed'] = True
finally:
    if OPENED:
        if not REPORT['passed']:
            try:
                REPORT['page'] = js('document.body.innerText')
                REPORT['media'] = js('[...document.querySelectorAll("audio")].map(media=>({src:media.currentSrc,track:media.dataset.platformTrack,paused:media.paused,readyState:media.readyState,error:media.error?.message}))')
                REPORT['frame'] = js('document.querySelector("[data-pvz-canvas]")?.__pvzFrame')
                b('screenshot', OUT / 'failure.png', '--full')
            except Exception: pass
            try:
                if room()['status'] == 'playing': command('close')
                command('leave')
            except Exception: pass
        try:
            b('close')
            for suffix in ['config', 'target']:
                (Path(ENV['AGENT_BROWSER_SOCKET_DIR']) / (ENV['AGENT_BROWSER_SESSION'] + '.' + suffix)).unlink(missing_ok=True)
        except Exception: pass
    shutil.rmtree(OUT / 'profile', ignore_errors=True)
    (OUT / 'result.json').write_text(json.dumps(REPORT, ensure_ascii=False, indent=2) + '\n')
    print(json.dumps({'passed': REPORT['passed'], 'cases': len(REPORT['cases'])}))
