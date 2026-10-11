#!/usr/bin/env python3
"""Run a local rail/3v3 match through normal bots and autoplay, recording UI results.

Failure cases: mobile overflow/missing controls, broken images, JS errors,
missing rail draw effects, stalled match, or results changed by page reload.
No game-state injection; only normal registration and room commands are used.
"""
import argparse
import json
import os
from pathlib import Path
import shutil
import subprocess
import time
from urllib.parse import urlsplit
import uuid

root = Path(__file__).resolve().parents[1]
parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument('--kind', choices=['rail', 'sanguosha'], required=True)
parser.add_argument('--base-url', default='http://127.0.0.1:18099')
parser.add_argument('--width', type=int, default=390)
args = parser.parse_args()
assert urlsplit(args.base_url).hostname in ('127.0.0.1', 'localhost', '::1')
run = uuid.uuid4().hex[:8]
out = root / '.local' / f'{args.kind}-browser-match-{run}'
out.mkdir(parents=True)
env = dict(os.environ, AGENT_BROWSER_SOCKET_DIR=str(root / '.local/ab3'),
           AGENT_BROWSER_SESSION='match-' + run, AGENT_BROWSER_PROFILE=str(out / 'profile'),
           AGENT_BROWSER_ARGS='--mute-audio,--disable-breakpad', TMPDIR=str(root / '.local/tmp'))
report = dict(passed=False, kind=args.kind, width=args.width, trace=[])


def b(*command):
    p = subprocess.run(['agent-browser', *command, '--json'], env=env,
                       text=True, capture_output=True, timeout=30)
    if p.returncode:
        raise RuntimeError(p.stderr or p.stdout)
    response = json.loads(p.stdout)
    assert response['success'], response
    return response['data']


def js(code):
    return b('eval', code)['result']


def api(path, body=None):
    options = '' if body is None else ',{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify(' + json.dumps(body) + ')}'
    response = js('fetch(' + json.dumps(path) + options + ').then(async r=>({ok:r.ok,body:await r.json()}))')
    assert response['ok'], response
    return response['body']


def room():
    return api('/api/state')['room']


def command(kind, **extra):
    for _ in range(8):
        r = room()
        body = dict(type=kind, version=r['version'], nonce=uuid.uuid4().hex, **extra)
        response = js('fetch(' + json.dumps('/api/rooms/' + r['id']) + ',{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify(' + json.dumps(body) + ')}).then(async r=>({status:r.status,body:await r.json()}))')
        if response['status'] == 409:
            continue
        assert response['status'] == 200, response
        return response['body']
    raise AssertionError('Repeated room version conflict')


def checkpoint(label):
    checks = js('({overflow:document.documentElement.scrollWidth>innerWidth,broken:[...document.images].filter(i=>i.complete&&!i.naturalWidth).map(i=>i.src),text:document.body.innerText})')
    assert not checks['overflow'] and not checks['broken'], checks
    assert not b('errors')['errors']
    (out / (label + '.json')).write_text(json.dumps(checks, ensure_ascii=False, indent=2))
    b('screenshot', str(out / (label + '.png')))


try:
    b('open', args.base_url)
    api('/api/register', dict(name='整局验收' + run, password=uuid.uuid4().hex,
                            invite=os.environ.get('INVITE_CODE', 'ui-accept-code')))
    create = dict(kind=args.kind, name='本地完整局验收', capacity=3 if args.kind == 'rail' else 6)
    if args.kind == 'rail':
        create['railMap'] = 'europe'
    else:
        create['sanguoshaOptions'] = dict(mode='3v3')
    api('/api/rooms', create)
    for _ in range(create['capacity'] - 1):
        command('add_bot')
    command('ready')
    b('set', 'viewport', str(args.width), '844')
    b('open', args.base_url)
    b('wait', '.waiting-layout')
    command('start')
    time.sleep(.7)
    checkpoint('opening')
    if args.kind == 'rail':
        # Track DOM animation creation independently of the server event list.
        js('''window.drawFlights=[];window.flightObserver=new MutationObserver(records=>{
          for(const r of records)for(const n of r.addedNodes)if(n instanceof Element){
            const es=[...(n.matches('.hidden-train-flight')?[n]:[]),...n.querySelectorAll('.hidden-train-flight')];
            for(const e of es)window.drawFlights.push({name:e.textContent,public:e.classList.contains('public-train-flight')});
          }});window.flightObserver.observe(document.body,{childList:true,subtree:true});''')
        r = room()
        assert r['game']['rail']['setup'] and r['game']['rail']['pending']
        assert js('document.querySelector(".ticket-choice").innerText.includes("所有人同时选择")')
        b('click', '.ticket-choice header button')
        assert js('document.querySelector(".ticket-choice").classList.contains("is-collapsed")')
        b('click', '.ticket-choice header button')
        command('action', action=dict(type='keep', keep=[t['id'] for t in r['game']['rail']['pending'][:2]]))
        time.sleep(.5)
        js('document.querySelector(".map-frame").scrollIntoView({block:"start"})')
        geometry = js('''(()=>{const rect=s=>document.querySelector(s).getBoundingClientRect().toJSON();return {map:rect('.map-frame'),viewport:rect('.map-scroll'),svg:rect('.rail-map'),market:rect('.train-market'),deck:rect('.train-deck'),card:rect('.train-market .train-card')}})()''')
        report['geometry'] = geometry
        assert geometry['market']['y'] >= geometry['map']['bottom'] - 2
        assert abs(geometry['deck']['width'] - geometry['card']['width']) < 2
        assert geometry['svg']['height'] >= geometry['viewport']['height'] - 2
        checkpoint('mobile-board')
        # Exercise a human's hidden draw, then observe opponents through the
        # normal bot scheduler; a market-only bot game cannot prove both paths.
        for _ in range(50):
            r = room()
            if not r['game']['rail']['setup'] and r['game']['turn'] == r['you']:
                break
            time.sleep(.3)
        assert r['game']['turn'] == r['you'] and r['game']['phase'] == 'turn'
        b('click', '.train-deck')
        time.sleep(.2)
        assert room()['game']['phase'] == 'draw'
        b('click', '.train-deck')
        time.sleep(.3)
        assert any(not f['public'] for f in js('window.drawFlights'))
        report['nativeHiddenDraw'] = True
    command('autoplay', enabled=True)
    start = time.monotonic()
    reloaded = False
    while time.monotonic() - start < 1200:
        r = room()
        g = r['game']
        entry = dict(version=r['version'], phase=g['phase'], round=g['round'], turn=g['turn'])
        if not report['trace'] or report['trace'][-1]['version'] != entry['version']:
            report['trace'].append(entry)
        assert not b('errors')['errors']
        if g['finished']:
            report['result'] = r['result']
            report['winners'] = g['winners']
            if args.kind == 'rail':
                report['drawFlights'] = js('window.drawFlights || []')
                names = {x['name'] for x in report['drawFlights']}
                assert len(names) >= 2, 'Missing opponent draw animations'
            b('reload')
            time.sleep(.6)
            after = room()
            assert after['game'] == g and after['result'] == r['result']
            assert js('!!document.querySelector(".results") && document.querySelector(".results").innerText.includes("获胜")')
            checkpoint('finished-reloaded')
            js('''(()=>{const e=[...document.querySelectorAll('button')].find(e=>e.textContent.trim()==='对局日志');e.dataset.acceptJournal='yes'})()''')
            b('click', '[data-accept-journal="yes"]')
            b('wait', '.journal-dialog li')
            report['journalEntries'] = js('document.querySelectorAll(".journal-dialog li").length')
            checkpoint('mobile-journal')
            if args.kind == 'rail':
                assert js('document.querySelectorAll(".hidden-train-flight").length') == 0
            report['passed'] = True
            print('PASS', args.kind, 'round', g['round'], 'winner', g['winners'], flush=True)
            break
        if not reloaded and time.monotonic() - start > 30:
            if args.kind == 'sanguosha':
                b('reload')
                time.sleep(.5)
            # Rail reload is checked at the end to retain the animation trace.
            checkpoint('midgame')
            reloaded = True
        if len(report['trace']) % 12 == 1:
            print(args.kind, entry, flush=True)
        time.sleep(4)
    assert report['passed'], 'Match did not finish in 20 minutes'
finally:
    (out / 'result.json').write_text(json.dumps(report, ensure_ascii=False, indent=2))
    # Preserve failed live tables for diagnosis; close only this browser/profile.
    b('close')
    if (out / 'profile').exists():
        shutil.rmtree(out / 'profile')
    print('Artifact:', out / 'result.json', flush=True)
