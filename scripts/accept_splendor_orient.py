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
import urllib.request
import http.cookiejar
root = Path(__file__).resolve().parents[1]
parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument('--width', type=int, default=390)
args = parser.parse_args()
if args.width < 320:
    parser.error('width must be at least 320')
conf = json.loads((root / '.local/orient-browser.json').read_text())
mode = conf.get('mode', 'chain')
run = uuid.uuid4().hex[:8]
out = root / '.local' / f'orient-{mode}-{args.width}-{run}'
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
    if want != 'turn' and mode == 'chain':
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
    js('document.querySelector(' + json.dumps(selector) + ').scrollIntoView({block:"center",inline:"nearest"})')
    b('click', selector)
    time.sleep(0.08)

def confirm(selector):
    js('scrollTo(0,0);document.querySelector(".players-strip").scrollLeft=0')
    click(selector)

def run_chain():
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
    return p

def show_orient():
    if js('document.querySelector(".gem-market-switch").getBoundingClientRect().width > 0'):
        click('.gem-market-switch button:nth-child(2)')

def private_views():
    for name in conf['observers']:
        client = urllib.request.build_opener(urllib.request.HTTPCookieProcessor(http.cookiejar.CookieJar()))
        request = urllib.request.Request(conf['url'] + '/api/login', data=json.dumps({'name': name, 'password': conf['password']}).encode(), headers={'Content-Type': 'application/json'})
        with client.open(request, timeout=10) as response:
            response.read()
        with client.open(conf['url'] + '/api/state', timeout=10) as response:
            sg = json.load(response)['room']['game']['splendor']
        assert 'reserveChoice' not in sg and 'reserved' not in sg['players'][0], 'Private candidates disclosed'
        assert all((e.get('card') is None for e in sg.get('cardEvents', []))), 'Blind event disclosed a card face'

def run_reserve():
    show_orient()
    before = state()
    deck = '.market-row[data-splendor-tier="1"][data-splendor-orient="true"] .deck'
    click(deck)
    click('.modal .form-grid button.outline')
    assert state()['version'] == before['version'], 'Cancelling reservation changed state'
    click(deck)
    click('.modal .form-grid button.primary')
    r = phase('gem_reserve')
    choices = r['game']['splendor']['reserveChoice']
    assert len(choices) == 2
    private_views()
    b('screenshot', str(out / 'private-choices.png'))
    b('open', conf['url'])
    assert state()['game']['splendor']['reserveChoice'] == choices, 'Reload changed private choices'
    click('.gem-effect-cards > div:nth-child(2) button.primary')
    r = phase('turn')
    sg = r['game']['splendor']
    assert sg['players'][0]['reserved'][0]['id'] == choices[1]['id']
    assert sg['remaining'][3] == initial_remaining[3] - 1
    assert sg['players'][0]['tokens'][5] == 1 and sg['bank'][5] == 4
    private_views()
    assert not b('errors')['errors']
    b('screenshot', str(out / 'reserved.png'))
    report['chosenCard'] = choices[1]['id']
    report['privateViewsChecked'] = len(conf['observers'])
    return sg['players'][0]

def run_gold():
    show_orient()
    click('.market [data-card-id="1021"]')
    click('.gem-payment-cards [data-card-id="1001"]')
    b('select', '.gem-purchase-panel select[aria-label="支付祖母绿数量"]', '0')
    b('select', '.gem-purchase-panel select[aria-label="支付红宝石数量"]', '0')
    assert not js('document.querySelector(".gem-purchase-panel button[data-card-acquire]").disabled')
    b('screenshot', str(out / 'gold-payment.png'))
    confirm('.gem-purchase-panel button[data-card-acquire="1021"]')
    r = phase('gem_free_card')
    sg = r['game']['splendor']
    p = sg['players'][0]
    assert p['tokens'] == [1, 0, 0, 0, 1, 0] and sg['bank'][5] == 5
    assert all((c['id'] != 1001 for c in p['cards']))
    assert any((c['id'] == 1021 for c in p['cards']))
    b('open', conf['url'])
    assert state()['game']['phase'] == 'gem_free_card'
    assert not b('errors')['errors']
    report['virtualGoldSpent'] = 2
    return p

def run_double_gold():
    show_orient()
    click('.market [data-card-id="1021"]')
    b('select', '.gem-purchase-panel select[aria-label="支付祖母绿数量"]', '0')
    b('select', '.gem-purchase-panel select[aria-label="支付红宝石数量"]', '0')
    assert js('document.querySelector(".gem-purchase-panel button[data-card-acquire]").disabled')
    b('select', '.gem-purchase-panel select[aria-label="支付祖母绿数量"]', '1')
    b('select', '.gem-purchase-panel select[aria-label="支付红宝石数量"]', '1')
    report['splitColorGoldRejected'] = True
    b('select', '.gem-purchase-panel select[aria-label="支付蓝宝石数量"]', '3')
    assert not js('document.querySelector(".gem-purchase-panel button[data-card-acquire]").disabled')
    b('screenshot', str(out / 'double-gold.png'))
    confirm('.gem-purchase-panel button[data-card-acquire="1021"]')
    r = phase('gem_free_card')
    sg = r['game']['splendor']
    p = sg['players'][0]
    assert p['tokens'] == [0, 0, 2, 0, 0, 0] and sg['bank'][5] == 5, p
    report['physicalGoldSpent'] = 1
    report['sameColorDeficitPaid'] = 2
    return p

def run_extra_token():
    click('.gem-bank [data-bank-color="1"]')
    click('.gem-bank [data-bank-color="1"]')
    click('.bank-actions button.primary')
    phase('gem_token')
    assert js('document.querySelector(`.gem-effect-panel button[aria-label^="领取钻石"]`).disabled')
    assert js('document.querySelector(`.gem-effect-panel button[aria-label^="领取蓝宝石"]`).disabled')
    b('screenshot', str(out / 'extra-token.png'))
    click('.gem-effect-panel button[aria-label^="领取缟玛瑙"]')
    r = phase('turn')
    p = r['game']['splendor']['players'][0]
    assert p['tokens'] == [1, 2, 5, 1, 1, 0] and r['game']['turn'] == 1, p
    return p

def run_posts():
    for color in [0, 1, 3]:
        click(f'.gem-bank [data-bank-color="{color}"]')
    click('.bank-actions button.primary')
    r = phase('gem_post')
    assert r['game']['splendor']['postChoices'] == [3, 5]
    b('screenshot', str(out / 'post-choices.png'))
    b('open', conf['url'])
    assert state()['game']['splendor']['postChoices'] == [3, 5]
    click('.gem-post-choices button:nth-child(2)')
    r = phase('turn')
    p = r['game']['splendor']['players'][0]
    assert p['tradingPosts'] == [1, 2, 5] and p['score'] == 4 and (r['game']['turn'] == 1), p
    assert '贸易声望' in js('document.querySelector(`[data-player-seat="0"] .player-gem-posts`).textContent')
    assert js('document.querySelector(`[data-player-seat="0"] .player-score`).textContent').strip() == '4分'
    b('screenshot', str(out / 'prestige.png'))
    return p

def run_cities():
    before = state()
    cities = before['game']['splendor']['cities']
    assert not before['game']['splendor'].get('lastRound')
    assert not before['game']['splendor'].get('cityEligibility', [[], [], []])[0]
    if js('document.querySelector(".gem-city-expand").getBoundingClientRect().width > 0'):
        click('.gem-city-expand')
    b('screenshot', str(out / 'city-before.png'))
    card = conf['finishCard']
    click(f'.market [data-card-id="{card}"]')
    confirm(f'.gem-purchase-panel button[data-card-acquire="{card}"]')
    r = phase('turn')
    assert r['game']['turn'] == 1 and (not r['game']['finished'])
    assert r['game']['splendor']['lastRound']
    assert r['game']['splendor']['cityEligibility'][0]
    assert js('document.querySelectorAll(".gem-city.is-eligible").length') > 0
    assert '最后一轮' in js('document.body.innerText')
    b('screenshot', str(out / 'city-qualified.png'))
    b('open', conf['url'])
    assert state()['game']['splendor']['cityEligibility'][0]
    for actor in [1, 2]:
        client = urllib.request.build_opener(urllib.request.HTTPCookieProcessor(http.cookiejar.CookieJar()))

        def call(path, body=None):
            request = urllib.request.Request(conf['url'] + path, data=json.dumps(body).encode() if body is not None else None, headers={'Content-Type': 'application/json'})
            with client.open(request, timeout=10) as response:
                return json.load(response)
        call('/api/login', {'name': conf['players'][actor], 'password': conf['password']})
        r = call('/api/state')['room']
        assert r['game']['turn'] == actor and (not r['game']['finished'])
        call('/api/rooms/' + conf['room'], {'type': 'action', 'version': r['version'], 'nonce': uuid.uuid4().hex, 'action': {'type': 'take', 'tokens': [1, 1, 1, 0, 0, 0]}})
    r = state()
    assert r['game']['finished'] and r['game']['winners'] == [0]
    assert r['game']['splendor']['cities'] == cities
    b('open', conf['url'])
    assert '获胜' in js('document.querySelector(".results h2").textContent')
    assert '城市' in js('document.querySelector(".results p").textContent')
    assert js('document.documentElement.scrollWidth <= innerWidth')
    assert not b('errors')['errors']
    b('screenshot', str(out / 'city-results.png'))
    report['winners'] = r['game']['winners']
    report['citiesUnchanged'] = True
    return r['game']['splendor']['players'][0]
try:
    b('open', conf['url'])
    js('fetch("/api/login",{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify(' + json.dumps({'name': conf['name'], 'password': conf['password']}) + ')}).then(r=>r.ok)')
    b('set', 'viewport', str(args.width), '844')
    b('open', conf['url'])
    time.sleep(0.3)
    initial_remaining = state()['game']['splendor']['remaining']
    p = {'chain': run_chain, 'gold': run_gold, 'reserve': run_reserve, 'double_gold': run_double_gold, 'extra_token': run_extra_token, 'posts': run_posts, 'cities': run_cities}[mode]()
    report['passed'] = True
    report['finalPlayer'] = p
    print('PASS Orient ' + mode)
finally:
    (out / 'result.json').write_text(json.dumps(report, ensure_ascii=False, indent=2))
    (root / '.local/orient-browser-stop').write_text(json.dumps({'passed': report['passed'], 'artifact': str(out / 'result.json')}))
    b('close')
    if (out / 'profile').exists():
        shutil.rmtree(out / 'profile')
    print('Artifact:', out / 'result.json')
