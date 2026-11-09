#!/usr/bin/env python3
"""Verify uniform model choices and native-clock round handoff in a real browser."""
import json
import os
from pathlib import Path
import subprocess
import time
import uuid

ROOT = Path(__file__).resolve().parents[3]
OUT = ROOT / '.local/stoneage/round-handoff/browser'
OUT.mkdir(parents=True, exist_ok=True)
BASE = (ROOT / '.local/stoneage/browser/url').read_text().strip()
ENV = dict(os.environ, AGENT_BROWSER_SOCKET_DIR=str(ROOT / '.local/rhb'),
           AGENT_BROWSER_PROFILE=str(OUT / 'profile'), TMPDIR=str(OUT / 'tmp'),
           AGENT_BROWSER_SCREENSHOT_DIR=str(OUT), AGENT_BROWSER_RESTORE_SAVE='never',
           AGENT_BROWSER_ARGS='--mute-audio,--disable-breakpad,--disable-crash-reporter')
for key in ['AGENT_BROWSER_SOCKET_DIR', 'AGENT_BROWSER_PROFILE', 'TMPDIR']:
    Path(ENV[key]).mkdir(parents=True, exist_ok=True)
SESSION = subprocess.check_output(['agent-browser', 'session', 'id', '--scope', 'worktree',
                                   '--prefix', 'stoneage-handoff'], cwd=ROOT, env=ENV, text=True).strip()


def b(*args):
    p = subprocess.run(['agent-browser', '--session', SESSION, *map(str, args), '--json'],
                       cwd=ROOT, env=ENV, text=True, capture_output=True, timeout=60)
    assert p.returncode == 0, p.stdout + p.stderr
    r = json.loads(p.stdout)
    assert r['success'], r
    return r['data']


def js(code):
    return b('eval', code)['result']


def api(path, body=None):
    options = '' if body is None else ',{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify(' + json.dumps(body) + ')}'
    r = js('(async()=>{const r=await fetch(' + json.dumps('/api' + path) + options + ');return {status:r.status,body:await r.json()}})()')
    assert 200 <= r['status'] < 300, r
    return r['body']


def room():
    return api('/state')['room']


def command(kind, **extra):
    nonce = uuid.uuid4().hex
    for attempt in range(12):
        r = room()
        try:
            return api('/rooms/' + r['id'], dict(type=kind, version=r['version'], nonce=nonce, **extra))
        except AssertionError as e:
            value = e.args[0]
            if not isinstance(value, dict) or value.get('status') != 409 or value.get('body', {}).get('error') != '局面已更新，请根据最新画面重试' or attempt == 11:
                raise


def wait(fn, seconds=45):
    until = time.monotonic() + seconds
    while time.monotonic() < until:
        if fn():
            return
        time.sleep(.08)
    raise AssertionError('condition timed out')


def click(selector):
    point = js('(()=>{const e=document.querySelector(' + json.dumps(selector) + ');e.scrollIntoView({block:"center",behavior:"instant"});const r=e.getBoundingClientRect();return [Math.round(r.x+r.width/2),Math.round(r.y+r.height/2)]})()')
    b('mouse', 'move', *point)
    b('mouse', 'down')
    b('mouse', 'up')


def tap(selector):
    point = js('(()=>{const e=document.querySelector(' + json.dumps(selector) + ');e.scrollIntoView({block:"center",behavior:"instant"});const r=e.getBoundingClientRect();return [Math.round(r.x+r.width/2),Math.round(r.y+r.height/2)]})()')
    p = subprocess.run(['node', 'scripts/browser_touch.mjs', b('get', 'cdp-url')['cdpUrl'], BASE,
                        'tap', *map(str, point + point)], cwd=ROOT, env=ENV, text=True, capture_output=True, timeout=20)
    assert p.returncode == 0, p.stdout + p.stderr


REPORT = {'passed': False, 'scope': 'native scheduler and browser; controlled HP/loyalty/defenders; real mouse, keyboard, touch, fullscreen; no accelerated clock', 'avatars': [], 'rounds': []}
try:
    b('open', BASE)
    api('/login', {'name': '石器验收', 'password': 'stoneage-browser-only'})
    old = api('/state').get('room')
    if old:
        assert old['kind'] == 'stoneage'
        if old['status'] == 'playing':
            command('close')
        command('leave')
    api('/rooms', {'kind': 'stoneage', 'name': '模型与回合交接验收', 'capacity': 2, 'options': {'turnSeconds': 45, 'preparationSeconds': 300}})
    command('add_bot')
    command('ready')
    command('start')
    b('reload')
    wait(lambda: js('!!document.querySelector("[data-stoneage-selection]")&&!document.querySelector(".game-loading")'), 90)
    pool = room()['game']['data']['pool']
    for width in [1440, 320]:
        b('set', 'viewport', width, 1000 if width == 1440 else 740)
        for ride in pool['rides']:
            click('[data-stoneage-tab="rides"]')
            click('[data-stoneage-pet="' + str(ride['id']) + '"]')
            click('[data-stoneage-tab="avatars"]')
            buttons = js('[...document.querySelectorAll("[data-stoneage-avatar]")].map(e=>({id:Number(e.dataset.stoneageAvatar),canvas:!!e.querySelector("canvas"),image:!!e.querySelector("img")}))')
            assert len(buttons) == len(ride['riders'])
            assert all(v['canvas'] and not v['image'] for v in buttons), ('mixed illustration/model choices', buttons)
            models = js('[...document.querySelectorAll("[data-stoneage-avatar]")].map(e=>{const c=e.querySelector("canvas"),r=c.getBoundingClientRect(),pixels=c.getContext("2d").getImageData(0,0,c.width,c.height).data;return {avatar:Number(e.dataset.stoneageAvatar),width:r.width,height:r.height,visible:[...pixels].filter((v,i)=>i%4===3&&v>100).length,frame:c.dataset.stoneageSpriteFrame}})')
            for model in models:
                assert model['visible'] > 100 and model['frame'], model
                assert model['width'] >= 60 and abs(model['width'] - model['height']) < 1, model
                REPORT['avatars'].append(dict(viewport=width, ride=ride['id'], **model))
            selected = buttons[-1]['id']
            selector = '[data-stoneage-avatar="' + str(selected) + '"]'
            (tap if width == 320 else click)(selector)
            wait(lambda: js('document.querySelector("[data-stoneage-character-preview]").dataset.stoneageCharacterPreview') == str(selected))
            assert js('document.querySelector(' + json.dumps(selector) + ').getAttribute("aria-pressed")') == 'true'
            if width == 1440:
                b('focus', '[data-stoneage-avatar]')
                b('press', 'Enter')
                first = buttons[0]['id']
                wait(lambda: js('document.querySelector("[data-stoneage-character-preview]").dataset.stoneageCharacterPreview') == str(first))
            b('screenshot', OUT / f'avatars-{width}-{ride["id"]}.png', '--full')
    # Lock through the actual UI, then measure several short native-clock rounds.
    click('[data-stoneage-auto-config]')
    wait(lambda: room()['game']['data']['self']['loadout'] is not None)
    click('[data-stoneage-lock]')
    wait(lambda: js('!!document.querySelector("[data-stoneage=planning] [data-stoneage-command=guard]:not(:disabled)")'), 90)
    b('set', 'viewport', 1440, 1000)
    click('[data-fullscreen-toggle]')
    wait(lambda: js('!!document.fullscreenElement||!!document.querySelector(".game-fullscreen-active")'))
    js('window.__handoffFrames=[];window.__handoffStop=false;(function sample(){window.__handoffFrames.push({at:Date.now(),phase:document.querySelector("[data-stoneage]")?.dataset.stoneage,status:document.querySelector(".sa-field-status")?.textContent,canAct:!!document.querySelector("[data-stoneage-command=guard]:not(:disabled)")});if(!window.__handoffStop)requestAnimationFrame(sample)})()')
    for turn in range(4):
        current = room()
        data = current['game']['data']
        click('[data-stoneage-command="guard"]')
        wait(lambda: js('!!document.querySelector(\'[data-stoneage-skill="2"]\')'))
        click('[data-stoneage-skill="2"]')
        wait(lambda: room()['game']['data']['phase'] == 'resolving', 5)
        resolving = room()['game']['data']
        end = resolving['playback']['startedAt'] + resolving['playback']['duration'] + current.get('clockOffsetMs', 0)
        wait(lambda: room()['game']['data']['round'] == data['round'] + 1 and js('!!document.querySelector("[data-stoneage=planning] [data-stoneage-command=guard]:not(:disabled)")'), 10)
        frames = js('window.__handoffFrames')
        reopened = next(f for f in frames if f['at'] >= end and f.get('phase') == 'planning' and f['canAct'])
        lag = reopened['at'] - end
        assert 0 <= lag <= 350, ('round stayed locked after animations', lag)
        REPORT['rounds'].append({'round': data['round'], 'timelineMs': resolving['playback']['duration'], 'handoffLagMs': lag})
    js('window.__handoffStop=true')
    REPORT['frames'] = js('window.__handoffFrames')
    b('screenshot', OUT / 'planning-fullscreen.png', '--full')
    b('press', 'Escape')
    b('set', 'viewport', 320, 740)
    b('screenshot', OUT / 'planning-mobile.png', '--full')
    before = room()['game']['data']
    b('reload')
    wait(lambda: js('!!document.querySelector("[data-stoneage=planning]")'), 90)
    assert room()['game']['data']['round'] == before['round']
    REPORT['errors'] = b('errors')
    assert not REPORT['errors'].get('errors')
    REPORT['passed'] = True
finally:
    if not REPORT['passed']:
        try:
            b('screenshot', OUT / 'failure.png', '--full')
        except Exception:
            pass
    (OUT / 'result.json').write_text(json.dumps(REPORT, ensure_ascii=False, indent=2) + '\n')
    try:
        b('close')
    except Exception:
        pass
print(json.dumps({'passed': REPORT['passed'], 'modelsChecked': len(REPORT['avatars']), 'rounds': REPORT['rounds']}))
