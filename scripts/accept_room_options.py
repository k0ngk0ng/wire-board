#!/usr/bin/env python3
"""Local creation-panel visual/interaction acceptance; writes repeatable evidence.

Failure cases: distorted/misaligned icons; visible native expansion checkboxes;
scenario dropdown retained; card click submits the form; selection state lost;
keyboard focus inaccessible; narrow-screen overflow or broken option disabling.
"""
import argparse
import json
import os
from pathlib import Path
import shutil
import subprocess
import time
import uuid
from urllib.parse import urlsplit

root = Path(__file__).resolve().parents[1]
p = argparse.ArgumentParser(description=__doc__)
p.add_argument('--url', default='http://127.0.0.1:5173')
p.add_argument('--width', type=int, default=390)
p.add_argument('--baseline', action='store_true')
a = p.parse_args()
assert urlsplit(a.url).hostname in ['127.0.0.1', 'localhost']
run = uuid.uuid4().hex[:8]
out = root / '.local' / ('room-options-' + run)
out.mkdir(parents=True)
env = dict(os.environ, AGENT_BROWSER_SOCKET_DIR=str(root / '.local/ab3'),
           AGENT_BROWSER_SESSION='options-' + run, AGENT_BROWSER_PROFILE=str(out / 'profile'),
           AGENT_BROWSER_ARGS='--mute-audio,--disable-breakpad', TMPDIR=str(root / '.local/tmp'))
report = {'passed': False, 'width': a.width, 'panels': []}

def b(*args):
    r = subprocess.run(['agent-browser', *args, '--json'], env=env, text=True, capture_output=True, timeout=35)
    assert r.returncode == 0, r.stdout + r.stderr
    data = json.loads(r.stdout)
    assert data['success'], data
    return data['data']

def js(code):
    return b('eval', code)['result']

def click_label(text):
    selector = js('''(text=>{const e=[...document.querySelectorAll('.modal label')].find(e=>e.textContent.trim().startsWith(text)&&e.querySelector('input[type=checkbox]'));if(!e)throw Error(text);e.dataset.acceptToggle='yes';return '[data-accept-toggle=yes]'} )('''+json.dumps(text)+')')
    b('click', selector)
    js('document.querySelectorAll("[data-accept-toggle]").forEach(e=>delete e.dataset.acceptToggle)')

try:
    b('open', a.url)
    assert js('fetch("/api/register",{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify('+json.dumps(dict(name='选项验收'+run,password=uuid.uuid4().hex,invite='style-accept-code'))+')}).then(r=>r.ok)')
    b('reload'); b('set', 'viewport', str(a.width), '900');b('wait','article')
    for game, panel in [('璀璨宝石','.gem-expansion-options'),('卡坦岛','.catan-helper-options'),('三国杀','.sg-options')]:
        js('''(text=>{[...document.querySelectorAll('article')].find(e=>e.querySelector('h2')?.textContent.startsWith(text)).querySelector('button').dataset.acceptCreate='yes'})('''+json.dumps(game)+')')
        b('click','[data-accept-create=yes]'); b('wait',panel);time.sleep(.3)
        js('document.querySelector('+json.dumps(panel)+').scrollIntoView({block:"start"})')
        info = js('''(()=>({overflow:document.documentElement.scrollWidth>innerWidth,checkboxes:[...document.querySelectorAll('.modal fieldset input[type=checkbox]')].map(e=>({opacity:getComputedStyle(e).opacity,w:e.getBoundingClientRect().width})),icons:[...document.querySelectorAll('.gem-expansion-options button > svg')].map(e=>({w:e.getBoundingClientRect().width,h:e.getBoundingClientRect().height,x:e.getBoundingClientRect().x-e.parentElement.getBoundingClientRect().x})),scenarioSelects:document.querySelectorAll('.catan-two-picker select').length}))()''')
        report['panels'].append(dict(game=game, **info))
        b('screenshot',str(out/(game+'-before-toggle.png')))
        assert not info['overflow']
        if not a.baseline:
            assert all(c['opacity']=='0' or c['w']<=1 for c in info['checkboxes']), (game, 'native checkbox visible')
            assert info['scenarioSelects']==0, 'Scenario is still a dropdown'
            assert all(abs(i['w']-22)<.1 and abs(i['h']-22)<.1 for i in info['icons']), info['icons']
            assert not info['icons'] or max(i['x'] for i in info['icons'])-min(i['x'] for i in info['icons'])<1, 'Icon columns are not aligned'
            if game=='卡坦岛':
                click_label('启用港口霸主');click_label('Helpers')
                assert js('document.querySelectorAll(".modal .catan-helper-options input:checked").length')==2
                b('click','button[data-scenario-value="cities-knights"]')
                assert js('document.querySelector("button[data-scenario-value=cities-knights]").getAttribute("aria-pressed")')=='true'
            elif game=='三国杀':
                click_label('风包');click_label('火包');click_label('风包')
                assert js('document.querySelectorAll(".sg-pack-option input:checked").length')==1
                b('press', 'Tab');b('press', 'Space')
                assert js('document.querySelectorAll(".sg-pack-option input:checked").length')==0
                b('press', 'Space')
                assert js('document.querySelectorAll(".sg-pack-option input:checked").length')==1
            else:
                b('click','.gem-expansion-options button:nth-child(3)')
                assert js('document.querySelector(".gem-expansion-options button:nth-child(3)").getAttribute("aria-pressed")')=='true'
            assert js('!!document.querySelector(".modal form")'), 'Option submitted the form'
            b('screenshot',str(out/(game+'-selected.png')))
        b('press','Escape')
        js('document.querySelectorAll("[data-accept-create]").forEach(e=>delete e.dataset.acceptCreate)')
    assert not b('errors')['errors']
    report['passed']=True
finally:
    (out/'result.json').write_text(json.dumps(report,ensure_ascii=False,indent=2))
    b('close')
    if (out/'profile').exists():shutil.rmtree(out/'profile')
    print('Artifact:',out/'result.json')
