#!/usr/bin/env python3
"""Check real Splendor flights against their DOM anchors on a LOCAL test server.

Requires agent-browser and a server using a disposable DATA_DIR. No fixture
injection: four accounts join, take tokens, reserve cards and return excess gems
via authenticated HTTP. Artifacts include per-frame checks and a screenshot.
"""
import argparse
import http.cookiejar
import json
import os
from pathlib import Path
import subprocess
import sys
import time
import urllib.error
import urllib.parse
import urllib.request
import uuid

# Failure cases: wrong player after scrolling; visible flights to clipped seats;
# missing return events; card origin/target moves to another slot/player.
ROOT = Path(__file__).resolve().parents[1]


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--base-url', default='http://127.0.0.1:18099')
    parser.add_argument('--width', type=int, default=390)
    parser.add_argument('--expanded', action='store_true')
    parser.add_argument('--acquisition', action='store_true', help='Buy a reserved card, obtain a noble, and finish the base game')
    args = parser.parse_args()
    url = urllib.parse.urlsplit(args.base_url)
    if url.hostname not in ('127.0.0.1', 'localhost', '::1'):
        parser.error('Use a disposable local server; remote tables are not supported.')
    if args.width < 320:
        parser.error('width must be at least 320')
    if args.acquisition and args.expanded:
        parser.error('acquisition currently verifies the base game')
    base = args.base_url.rstrip('/')
    run = uuid.uuid4().hex[:8]
    out = ROOT / '.local' / f'splendor-animation-{args.width}-{run}'
    out.mkdir(parents=True)
    runtime = ROOT / '.local' / 'ab3'
    runtime.mkdir(exist_ok=True)
    temporary = ROOT / '.local' / 'tmp'
    temporary.mkdir(exist_ok=True)
    env = dict(os.environ, AGENT_BROWSER_SOCKET_DIR=str(runtime),
               AGENT_BROWSER_SESSION='sa-' + run,
               AGENT_BROWSER_PROFILE=str(out / 'profile'),
               AGENT_BROWSER_ARGS='--mute-audio,--disable-breakpad',
               TMPDIR=str(temporary))
    clients = [urllib.request.build_opener(urllib.request.HTTPCookieProcessor(
        http.cookiejar.CookieJar())) for _ in range(4)]
    names = ['动画验收' + run + str(i) for i in range(4)]
    password = uuid.uuid4().hex
    report = {'passed': False, 'width': args.width, 'expanded': args.expanded, 'actions': []}
    started = False

    def browser(*command):
        p = subprocess.run(['agent-browser', *command, '--json'], env=env,
                           capture_output=True, text=True, timeout=30)
        if p.returncode:
            raise RuntimeError(p.stderr or p.stdout)
        data = json.loads(p.stdout)
        if not data['success']:
            raise RuntimeError(data)
        return data['data']

    def js(source):
        return browser('eval', source)['result']

    def api(player, path, body=None):
        req = urllib.request.Request(base + path,
            data=json.dumps(body).encode() if body is not None else None,
            headers={'Content-Type': 'application/json'})
        try:
            with clients[player].open(req, timeout=15) as response:
                return json.load(response)
        except urllib.error.HTTPError as error:
            raise RuntimeError(f'HTTP {error.code}: {error.read().decode()}') from None

    def room(player=0):
        return api(player, '/api/state')['room']

    def command(player, kind, **extra):
        r = room()
        return api(player, '/api/rooms/' + r['id'], dict(
            type=kind, version=r['version'], nonce=uuid.uuid4().hex, **extra))

    def observe(label, actor, action, panel=False):
        before = room()['game']['splendor']
        js('window.flightAfter = ' + json.dumps(dict(
            card=before.get('cardEventId', 0), token=before.get('tokenEventId', 0))))
        if args.acquisition:
            time.sleep(1.5)  # Let earlier rapid HTTP actions finish animating.
        if panel:
            browser('click', f'.reserved [data-card-id="{action["card"]}"]')
        delta = 35 if actor == 0 else -35
        js('document.querySelector(".players-strip").scrollLeft=' +
           str(430 if actor >= 2 else 0) +
           ';scrollTo(0,0);window.flightScrollDelta=' + str(delta))
        time.sleep(.15)
        js('window.flightFixedSource = null; window.flightCardSource = null')
        if panel:
            js('window.flightFixedSource = document.querySelector(".gem-purchase-panel [data-card-id]").getBoundingClientRect().toJSON()')
        elif action.get('card'):
            source = js('(() => {\n              const e=document.querySelector(\'.market [data-card-id="CARD"]\');\n              if(!e)return null;\n              const row=e.closest(\'[data-splendor-tier]\'), slot=e.closest(\'[data-market-slot]\');\n              return \'[data-splendor-tier="\'+row.dataset.splendorTier+\'"][data-splendor-orient="\'+row.dataset.splendorOrient+\'"] [data-market-slot="\'+slot.dataset.marketSlot+\'"]\';\n            })()'.replace('CARD', str(action['card'])))
            js('window.flightCardSource = ' + json.dumps(source))
        js((ROOT / 'scripts/splendor_animation_probe.js').read_text())
        if panel:
            browser('click', f'button[data-card-acquire="{action["card"]}"]')
        else:
            command(actor, 'action', action=action)
        time.sleep(1.5)
        result = js('window.flightResult')
        result['label'] = label
        report['actions'].append(result)
        assert result.get('done'), result
        assert not result['errors'], result
        assert result['samples'] or result['card'], result
        assert result['visible'] > 0, result
        assert actor in result['players'], result
        print(label, 'visible=', result['visible'], 'hidden=', result['hidden'], flush=True)
        return result

    try:
        for i in range(4):
            api(i, '/api/register', dict(name=names[i], password=password,
                invite=os.environ.get('INVITE_CODE', 'page-accept-code')))
        options = dict(orient=True, cities=True, tradingPosts=True, strongholds=True) if args.expanded else {}
        api(0, '/api/rooms', dict(kind='splendor', name='宝石动画验收',
                                capacity=4, splendorOptions=options))
        for i in range(1, 4):
            command(i, 'join')
        for i in range(4):
            command(i, 'ready')
        command(0, 'start')
        browser('open', base)
        started = True
        js('fetch("/api/login",{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify(' +
           json.dumps(dict(name=names[0], password=password)) + ')}).then(r=>r.ok)')
        browser('set', 'viewport', str(args.width), '844')
        browser('open', base)
        time.sleep(.5)
        assert js('document.documentElement.scrollWidth <= innerWidth'), 'Horizontal overflow'
        browser('screenshot', str(out / 'opening.png'))
        if args.acquisition:
            sys.dont_write_bytecode = True
            from splendor_acquisition_flow import run_acquisition
            run_acquisition(room, command, observe, browser, js, report, out, base)
        else:
            for step in range(20):
                g = room()['game']
                sg, actor = g['splendor'], g['turn']
                if g['phase'] == 'discard':
                    assert actor == 3, 'Unexpected responder'
                    tokens = sg['players'][actor]['tokens']
                    due, give = sum(tokens) - 10, [0] * 6
                    for i, n in enumerate(tokens):
                        give[i] = min(due, n)
                        due -= give[i]
                    observe('return', actor, dict(type='discard', tokens=give))
                    break
                own = room(actor)['game']['splendor']['players'][actor]
                if actor == 3 or len(own.get('reserved') or []) >= 3:
                    take = [0] * 6
                    colors = range(3) if actor == 3 else sorted(range(5), key=lambda i: sg['bank'][i], reverse=True)[:3]
                    for i in colors:
                        if sg['bank'][i] > 0:
                            take[i] = 1
                    observe('take', actor, dict(type='take', tokens=take))
                else:
                    # The top tier remains visible even on the expanded 320px board.
                    card = next(c for c in sg['market'][2] if c['id'] > 0)
                    observe('reserve', actor, dict(type='reserve', card=card['id']))
            else:
                raise AssertionError('Return phase not reached')
        assert not browser('errors')['errors'], 'Browser errors'
        report['passed'] = True
    finally:
        (out / 'result.json').write_text(json.dumps(report, ensure_ascii=False, indent=2))
        if started:
            browser('close')
        # This is this run's dedicated browser profile; preserve only the evidence.
        import shutil
        if (out / 'profile').exists():
            shutil.rmtree(out / 'profile')
        print('Artifact:', out / 'result.json', flush=True)


if __name__ == '__main__':
    main()
