#!/usr/bin/env python3
"""Verify public Catan sea-combination controls on a disposable local server.
Checks all 18 sea options with addons retained, then starts the configured game.
No direct state/React edits: native form actions submit real room commands.
"""
import argparse
import json
import os
from pathlib import Path
import shutil
import subprocess
import time
import urllib.parse
import uuid
root = Path(__file__).resolve().parents[1]
parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument('--base-url', default='http://127.0.0.1:18099')
parser.add_argument('--players', type=int, choices=[2, 3, 6], default=2)
parser.add_argument('--width', type=int, default=390)
args = parser.parse_args()
if urllib.parse.urlsplit(args.base_url).hostname not in ['127.0.0.1', 'localhost', '::1']:
    parser.error('Use a disposable local test server')
base = args.base_url.rstrip('/')
players, width = (args.players, args.width)
run = uuid.uuid4().hex[:8]
out = root / '.local' / f'catan-config-{players}-{width}-{run}'
out.mkdir(parents=True)
env = dict(os.environ, AGENT_BROWSER_SOCKET_DIR=str(root / '.local/ab3'), AGENT_BROWSER_SESSION='cq-' + run, AGENT_BROWSER_PROFILE=str(out / 'profile'), AGENT_BROWSER_ARGS='--mute-audio,--disable-breakpad', TMPDIR=str(root / '.local/tmp'))
report = {'passed': False, 'players': players, 'width': width, 'cases': []}

def b(*args):
    p = subprocess.run(['agent-browser', *args, '--json'], env=env, text=True, capture_output=True, timeout=30)
    if p.returncode:
        raise RuntimeError(p.stderr or p.stdout)
    r = json.loads(p.stdout)
    assert r['success'], r
    return r['data']

def js(code):
    return b('eval', code)['result']

def click(selector):
    js('document.querySelector(' + json.dumps(selector) + ').scrollIntoView({block:"center",inline:"nearest"})')
    b('click', selector)
    time.sleep(0.08)

def state():
    return js('fetch("/api/state").then(r=>r.json())')

def select_label(label, value):
    result = js("((label)=>{const e=[...document.querySelectorAll('.modal select')].find(e=>e.closest('label').innerText.trim().startsWith(label));if(!e)return null;e.dataset.acceptSelect='target';return [...e.options].map(o=>({value:o.value,disabled:o.disabled}));})(" + json.dumps(label) + ')')
    assert result and any((o['value'] == str(value) and (not o['disabled']) for o in result)), (label, value, result)
    b('select', '[data-accept-select="target"]', str(value))
    js('document.querySelectorAll("[data-accept-select]").forEach(e=>delete e.dataset.acceptSelect)')
    time.sleep(0.08)

def check_label(prefix):
    label = js("((prefix)=>{const e=[...document.querySelectorAll('.modal input[type=checkbox]')].find(e=>e.closest('label').innerText.trim().startsWith(prefix));if(!e)return null;e.dataset.acceptCheck='target';return {text:e.closest('label').innerText,disabled:e.matches(':disabled')};})(" + json.dumps(prefix) + ')')
    assert label and (not label['disabled']), (prefix, label)
    js('document.querySelector("[data-accept-check]").scrollIntoView({block:"center"})')
    b('check', '[data-accept-check="target"]')
    js('document.querySelectorAll("[data-accept-check]").forEach(e=>delete e.dataset.acceptCheck)')
    time.sleep(0.08)

def click_text(text):
    found = js("((text)=>{document.querySelectorAll('[data-accept-button]').forEach(e=>delete e.dataset.acceptButton);const e=[...document.querySelectorAll('button')].find(e=>e.textContent.trim()===text&&e.getBoundingClientRect().width>0);if(!e||e.disabled)return false;e.dataset.acceptButton='now';return true})(" + json.dumps(text) + ')')
    assert found, ('button unavailable', text)
    click('[data-accept-button="now"]')
try:
    b('open', base)
    name = '组合入口' + uuid.uuid4().hex[:7]
    js('fetch("/api/register",{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify(' + json.dumps({'name': name, 'password': 'ui-accept-password', 'invite': os.environ.get('INVITE_CODE', 'ui-accept-code')}) + ')}).then(r=>r.json())')
    b('open', base)
    b('set', 'viewport', str(width), '900')
    b('wait', 'article')
    js("[...document.querySelectorAll('article')].find(e=>e.querySelector('h2')?.textContent.startsWith('卡坦岛')).querySelector('button').dataset.acceptNew='catan' ")
    click('[data-accept-new="catan"]')
    select_label('卡坦剧本', 'rivers-shores')
    select_label('座位上限', players)
    labels = ['渔夫＋', '城市与骑士＋', '启用港口霸主', '启用友善强盗', 'Helpers', '事件牌']
    for label in labels:
        check_label(label)
    scenes = ['rivers-shores', 'rivers-fog', 'rivers-desert', 'rivers-desert-belt', 'rivers-tribe', 'rivers-new-world', 'caravans-shores', 'caravans-islands', 'caravans-desert', 'caravans-tribe', 'caravans-new-world', 'attack-shores', 'attack-desert', 'attack-tribe', 'attack-wonders', 'attack-pirates', 'transport-shores', 'transport-desert']
    for scene in scenes:
        select_label('双人卡坦剧本' if players == 2 else '卡坦剧本', scene)
        flags = js('[...document.querySelectorAll(".modal input[type=checkbox]")].map(e=>({label:e.closest("label").innerText.trim(),checked:e.checked,disabled:e.matches(":disabled")}))')
        for label in labels:
            f = next((x for x in flags if x['label'].startswith(label)), None)
            assert f and f['checked'] and (not f['disabled']), (scene, label, flags)
        report['cases'].append({'scene': scene, 'flags': flags})
    select_label('双人卡坦剧本' if players == 2 else '卡坦剧本', 'attack-pirates')
    click('.modal form > button.primary.wide')
    time.sleep(0.15)
    r = state()['room']
    assert r['capacity'] == players and r['catanScenario'] == 'attack-pirates'
    assert r['catanFishing'] and r['catanCitiesKnights'] is not None and r['catanOptions']['helpers']
    assert r['catanEvents'] == 'wire-board-events-v1' and r['catanHarbors']['enabled'] and r['catanFriendlyRobber']['enabled']
    report['created'] = {k: r.get(k) for k in ['capacity', 'catanScenario', 'catanFishing', 'catanCitiesKnights', 'catanOptions', 'catanEvents']}
    for _ in range(players - 1):
        click_text('添加电脑玩家')
    click_text('我准备好了')
    click_text('开始游戏')
    time.sleep(0.2)
    r = state()['room']
    g = r['game']['catan']
    assert r['status'] == 'playing'
    assert g.get('attack') and g.get('citiesKnights') and g.get('fishing') and g.get('eventDeck')
    assert not b('errors')['errors']
    assert js('document.documentElement.scrollWidth <= innerWidth')
    b('screenshot', str(out / f'matrix-start-{players}-{width}.png'))
    report['startedPhase'] = r['game']['phase']
    report['passed'] = True
    print('PASS', players, width, len(scenes), 'scene switches and configured startup')
finally:
    (out / 'result.json').write_text(json.dumps(report, ensure_ascii=False, indent=2))
    try:
        for _ in range(8):
            r = state().get('room')
            if not r:
                break
            kind = 'close' if r['status'] == 'playing' else 'leave'
            body = {'type': kind, 'version': r['version'], 'nonce': uuid.uuid4().hex}
            js('fetch("/api/rooms/' + r['id'] + '",{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify(' + json.dumps(body) + ')}).then(r=>r.status)')
            time.sleep(0.08)
    finally:
        b('close')
        if (out / 'profile').exists():
            shutil.rmtree(out / 'profile')
    print('Artifact:', out / 'result.json')
