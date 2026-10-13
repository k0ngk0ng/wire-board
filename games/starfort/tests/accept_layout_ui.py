#!/usr/bin/env python3
"""Desktop aspect ratio and one health bar regressions, specified before changes. Reuses real HTTP fixture.
Run: . scripts/env.sh && python3 -B games/starfort/tests/accept_interaction_ui.py
Artifacts: .local/starfort-layout/browser/ (screenshots, source hashes, commands).
"""
import json
import os
from pathlib import Path
import subprocess
import sys
import time
os.environ['WIRE_BOARD_STARFORT_BROWSER_OUT'] = str(Path(__file__).resolve().parents[3] / '.local/starfort-layout/browser')
import accept_world_ui as w

w.OUT = w.ROOT / '.local/starfort-layout/browser'
w.OUT.mkdir(parents=True, exist_ok=True)
w.ENV['WIRE_BOARD_STARFORT_BROWSER_OUT'] = str(w.OUT)
w.REPORT['checks'] = []
b, js, wait, click = w.b, w.js, w.wait, w.click
try:
    if '--skip-build' not in sys.argv: w.run(['npm','--prefix','web','run','build'], 'build.log')
    w.SOURCE_START = w.source_hashes()
    for name in ['url', 'stop']: (w.OUT/name).unlink(missing_ok=True)
    with (w.OUT/'server.log').open('w') as log:
        w.SERVER = subprocess.Popen(['go','test','-count=1','-timeout=20m','-run','^TestStarfortBrowserFixture$','./games/starfort/tests'],cwd=w.ROOT,env=w.ENV,stdout=log,stderr=subprocess.STDOUT)
    deadline=time.monotonic()+120
    while not (w.OUT/'url').exists():
        if w.SERVER.poll() is not None or time.monotonic()>deadline: raise RuntimeError('fixture startup failed')
        time.sleep(.1)
    w.BASE=(w.OUT/'url').read_text()
    w.PEERS=[w.Peer('交互同伴'+str(i)) for i in range(1,4)]
    b('open',w.BASE);w.OPENED=True
    wait('document.body.textContent.includes("进入游戏大厅")')
    assert not js('document.body.textContent.includes("桌游")')
    w.checkpoint('game-lobby-login-copy',False)
    w.api('/register',{'name':w.MAIN,'password':'starfort-browser-test','invite':'starfort-browser'})
    w.login('上架管理员');w.label('管理后台');b('wait','input[aria-label="搜索管理游戏"]')
    b('fill','input[aria-label="搜索管理游戏"]','星垒');click('.admin-game-card button');w.label('确认操作')
    wait('document.querySelector(".admin-game-card")?.textContent.includes("已上架")')
    w.login(w.MAIN)
    wait('document.querySelector(".nav-active")?.textContent==="游戏大厅"')
    assert '游戏大厅' in js('document.querySelector("meta[name=description]").content')
    assert js('document.querySelector(".room-filters option[value=all]")?.textContent==="全部游戏"')
    w.checkpoint('game-lobby-navigation-copy',False)

    w.scenario('stage4',4)
    for width,height in [(1366,768),(1440,900),(1920,1080),(2560,1080),(1024,768),(390,844),(320,740)]:
        b('set','viewport',str(width),str(height))
        click('[data-sf-fit]')
        box=js('document.querySelector(".sf-world-viewport").getBoundingClientRect().toJSON()')
        if width>850:
            assert abs(box['width']/box['height']-16/9)<.02, box
            assert box['height']>=359 and box['width']<=1140, box
        else:
            assert abs(box['height']-max(240,min(380,height-580)))<1, box
            assert box['width']<=width, box
        w.artifact('geometry-'+str(width)+'.json',box)
        b('scrollintoview','[data-starfort-canvas]')
        w.checkpoint('map-proportion-'+str(width))
        b('screenshot',str(w.OUT/('viewport-'+str(width)+'.png')))
    b('set','viewport','1440','900');w.camera_checks('desktop-ratio-navigation')
    w.scenario('vitality')
    wait('document.querySelector(".sf-battlefield").dataset.assetsReady==="true"')
    before=w.data()['enemies'][0]
    assert before['shield']>0
    js(r"""(()=>{const c=document.querySelector('[data-starfort-canvas]').getContext('2d'),clear=c.clearRect,fill=c.fillRect;window.__bars=[];let width=0;c.clearRect=function(...a){window.__barFrame={at:performance.now(),health:[],shield:0};window.__bars.push(window.__barFrame);return clear.apply(this,a)};c.fillRect=function(...a){if(this.fillStyle==='#283b34')width=a[2]-2;if(this.fillStyle==='#add584'&&width)window.__barFrame.health.push(a[2]/width);if(this.fillStyle==='#cef0ff')window.__barFrame.shield++;return fill.apply(this,a)}})()""")
    b('wait','4000')
    after=w.data()['enemies'][0];frames=js('window.__bars')
    assert after['hp']==before['hp'] and after['shield']<before['shield'],(before,after)
    assert frames and all(len(f['health'])==1 and f['shield']==0 for f in frames), 'enemy painted two bars'
    values=[f['health'][0] for f in frames]
    assert max(values)-min(values)>.02, 'shield damage did not lower the single health bar'
    assert after['health']==after['hp']+after['shield'] and after['maxHealth']==after['maxHP']*1.8
    w.artifact('single-bar-shield-damage.json',{'before':before,'after':after,'frames':frames})
    w.checkpoint('single-health-bar-desktop')
    b('set','viewport','320','740');click('[data-sf-fit]');w.checkpoint('single-health-bar-mobile')
    b('reload');b('wait','.starfort-board')
    restored=w.data()['enemies'][0]
    assert restored['maxHealth']==after['maxHealth'], 'reconnect changed the health bar maximum'
    w.checkpoint('single-health-bar-reconnect')
    w.REPORT['passed']=True
except Exception as error:
    import traceback
    traceback.print_exc();w.REPORT['error']=repr(error)
    if w.OPENED:
        try:
            w.REPORT['snapshot']=b('snapshot','-i');b('screenshot',str(w.OUT/'failure.png'),'--full')
        except Exception: pass
finally:
    if w.OPENED:
        try:b('close')
        except Exception:pass
    if w.SERVER:
        (w.OUT/'stop').write_text('stop')
        try:w.SERVER.wait(timeout=15)
        except subprocess.TimeoutExpired:w.SERVER.terminate();w.SERVER.wait(timeout=10)
    w.REPORT['sourceSHA256']=w.source_hashes()
    w.REPORT['sourceUnchangedDuringRun']=w.SOURCE_START==w.REPORT['sourceSHA256']
    w.artifact('result.json',w.REPORT)
    print('Artifact:',w.OUT/'result.json')
    if not w.REPORT['passed']:raise SystemExit(1)
