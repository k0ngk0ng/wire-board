#!/usr/bin/env python3
"""Play the controlled Orient/copy/trading-post/stronghold chain in a browser.
Start TestSplendorOrientBrowserFixture with its opt-in environment first.
This verifies a conserved midgame fixture, not a natural-game distribution.
"""
import argparse
import json
import os
from pathlib import Path
import shutil
import subprocess
import time
import uuid
root = Path(__file__).resolve().parents[1]
parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument('--width', type=int, default=390)
args = parser.parse_args()
if args.width < 320:
    parser.error('width must be at least 320')
conf = json.loads((root / '.local/orient-browser.json').read_text())
run = uuid.uuid4().hex[:8]
out = root / '.local' / f'orient-chain-{args.width}-{run}'
out.mkdir(parents=True)
env = dict(os.environ, AGENT_BROWSER_SOCKET_DIR=str(root / '.local/ab3'), AGENT_BROWSER_SESSION='oc-' + run, AGENT_BROWSER_PROFILE=str(out / 'profile'), AGENT_BROWSER_ARGS='--mute-audio,--disable-breakpad', TMPDIR=str(root / '.local/tmp'))

def b(*args):
    p = subprocess.run(['agent-browser', *args, '--json'], env=env, text=True, capture_output=True, timeout=30)
    if p.returncode:
        raise RuntimeError(p.stderr or p.stdout)
    r = json.loads(p.stdout)
    if not r['success']:
        raise RuntimeError(r)
    return r['data']

def js(code):
    return b('eval', code)['result']

def state():
    return js('fetch("/api/state").then(r=>r.json())')['room']
report = {'passed': False, 'checkpoints': []}
results = report['checkpoints']
initial_remaining = None

def phase(want):
    for _ in range(30):
        r = state()
        if r['game']['phase'] == want:
            break
        time.sleep(0.1)
    else:
        raise AssertionError((want, r['game']['phase']))
    assert r['game']['turn'] == 0 or want == 'turn', r['game']['turn']
    sg = r['game']['splendor']
    if want != 'turn':
        assert sg['remaining'] == initial_remaining, 'Deck refilled during the chain'
        for ev in sg.get('cardEvents', []):
            if ev['source'] == 'market':
                row = ev['tier'] - 1 + (3 if ev.get('orient') else 0)
                assert sg['market'][row][ev['slot']]['id'] == 0, ('Premature refill', ev)
    metrics = js('({width:innerWidth,scroll:document.documentElement.scrollWidth,errors:document.querySelector(".gem-payment-error")?.textContent,flights:[...document.querySelectorAll("[data-card-flight]")].map(e=>({id:e.dataset.cardFlight,action:e.dataset.flightAction,player:e.dataset.flightPlayer,visible:e.querySelector(".splendor-card-flight").style.visibility}))})')
    assert metrics['scroll'] <= metrics['width'], metrics
    results.append({'phase': want, 'version': r['version'], 'metrics': metrics, 'events': r['game']['splendor'].get('cardEvents', [])})
    (out / 'result.json').write_text(json.dumps(report, ensure_ascii=False, indent=2))
    print('phase', want, 'cards', [(e['action'], (e.get('card') or {}).get('id')) for e in r['game']['splendor'].get('cardEvents', [])], flush=True)
    return r

def click(selector):
    js('document.querySelector(' + json.dumps(selector) + ').scrollIntoView({block:"nearest",inline:"nearest"})')
    b('click', selector)
    time.sleep(0.08)

def confirm(selector):
    js('scrollTo(0,0);document.querySelector(".players-strip").scrollLeft=0')
    click(selector)
try:
    b('open', conf['url'])
    js('fetch("/api/login",{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify(' + json.dumps({'name': conf['name'], 'password': conf['password']}) + ')}).then(r=>r.ok)')
    b('set', 'viewport', str(args.width), '844')
    b('open', conf['url'])
    time.sleep(0.3)
    initial_remaining = state()['game']['splendor']['remaining']
    if state()['game']['phase'] == 'turn':
        if js('document.querySelector(".gem-market-switch").getBoundingClientRect().width > 0'):
            click('.gem-market-switch button:nth-child(2)')
        click('.market [data-card-id="1021"]')
        confirm('.gem-purchase-panel button[data-card-acquire="1021"]')
    phase('gem_free_card')
    click('.gem-effect-panel [data-card-id="1016"]')
    confirm('.gem-effect-panel button[data-card-acquire="1016"]')
    phase('gem_copy')
    b('screenshot', str(out / 'first-copy.png'))
    b('open', conf['url'])
    time.sleep(0.15)
    assert state()['game']['phase'] == 'gem_copy'
    assert js('document.querySelectorAll("[data-card-flight]").length') == 0, 'Reload replayed acquisition'
    click('.gem-effect-panel [data-card-id="1013"]')
    confirm('.gem-effect-panel button.primary.wide')
    phase('gem_free_card')
    click('.gem-effect-panel [data-card-id="1006"]')
    confirm('.gem-effect-panel button[data-card-acquire="1006"]')
    phase('gem_copy')
    click('.gem-effect-panel [data-card-id="1016"]')
    confirm('.gem-effect-panel button.primary.wide')
    r = phase('gem_token')
    assert r['game']['splendor']['players'][0]['bonus'] == [6, 1, 1, 0, 0]
    click('.gem-effect-panel button[aria-label^="领取蓝宝石"]')
    phase('gem_stronghold')
    click('.gem-effect-panel [data-card-id="1026"]')
    click('.gem-effect-actions button')
    phase('gem_conquest')
    click('.gem-effect-panel .form-grid button.primary')
    click('.gem-payment-cards [data-card-id="1006"]')
    click('.gem-payment-cards [data-card-id="1016"]')
    confirm('.gem-purchase-panel button[data-card-acquire="1026"]')
    phase('gem_token')
    click('.gem-effect-panel button[aria-label^="领取红宝石"]')
    r = phase('gem_stronghold')
    blocked = r['game']['splendor']['market'][0][0]['id']
    click('.gem-effect-panel [data-card-id="' + str(blocked) + '"]')
    click('.gem-effect-actions button')
    r = phase('turn')
    p = r['game']['splendor']['players'][0]
    assert p['bonus'] == [2, 2, 1, 0, 0] and p['score'] == 5 and (p['tokens'] == [0, 0, 1, 0, 1, 0]), p
    assert [e['action'] for e in r['game']['splendor']['cardEvents']] == ['buy', 'free', 'free', 'buy']
    assert r['game']['turn'] == 1
    assert not b('errors')['errors']
    b('screenshot', str(out / 'finished-chain.png'))
    report['passed'] = True
    report['finalPlayer'] = p
    print('PASS Orient chain')
finally:
    (out / 'result.json').write_text(json.dumps(report, ensure_ascii=False, indent=2))
    (root / '.local/orient-browser-stop').write_text(json.dumps({'passed': report['passed'], 'artifact': str(out / 'result.json')}))
    b('close')
    if (out / 'profile').exists():
        shutil.rmtree(out / 'profile')
    print('Artifact:', out / 'result.json')
