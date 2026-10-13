#!/usr/bin/env python3
"""Regression scenarios authored before implementation. Reuses real HTTP fixture.
Run: . scripts/env.sh && python3 -B games/starfort/tests/accept_interaction_ui.py
Artifacts: .local/starfort-interaction/browser/ (screenshots, source hashes, commands).
"""
import json
import os
from pathlib import Path
import subprocess
import sys
import time
os.environ['WIRE_BOARD_STARFORT_BROWSER_OUT'] = str(Path(__file__).resolve().parents[3] / '.local/starfort-interaction/browser')
import accept_world_ui as w

w.OUT = w.ROOT / '.local/starfort-interaction/browser'
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
    w.api('/register',{'name':w.MAIN,'password':'starfort-browser-test','invite':'starfort-browser'})
    w.login('上架管理员');w.label('管理后台');b('wait','input[aria-label="搜索管理游戏"]')
    b('fill','input[aria-label="搜索管理游戏"]','星垒');click('.admin-game-card button');w.label('确认操作')
    wait('document.querySelector(".admin-game-card")?.textContent.includes("已上架")')
    w.login(w.MAIN)

    w.scenario('stage4', 4)
    b('set','viewport','1366','900')
    w.camera_checks('desktop-camera')
    click('[data-sf-fit]');b('scrollintoview','[data-starfort-canvas]')
    before=w.camera(); page=js('scrollY')
    b('mouse','move',before['left']+before['width']/2,before['top']+before['height']/2)
    endpoint=b('get','cdp-url')['cdpUrl']
    subprocess.run(['node','scripts/browser_wheel.mjs',endpoint,w.BASE,str(round(before['left']+before['width']/2)),str(round(before['top']+before['height']/2)),'-240'],cwd=w.ROOT,env=w.ENV,check=True,timeout=15)
    wait('Number(document.querySelector("[data-starfort-canvas]").dataset.zoom)>'+str(before['zoom']))
    after=w.camera()
    assert after['zoom']>before['zoom'] and abs(js('scrollY')-page)<2, (before,after)
    w.artifact('wheel.json',{'before':before,'after':after,'pageScroll':page})
    b('scrollintoview','[data-sf-minimap]')
    r=js('document.querySelector("[data-sf-minimap]").getBoundingClientRect().toJSON()')
    b('mouse','move',r['x']+r['width']*.35,r['y']+r['height']*.4);b('mouse','down')
    start=w.camera()
    b('mouse','move',r['x']+r['width']*.75,r['y']+r['height']*.6)
    moved=w.camera()  # Must move before mouseup, not just click at destination.
    b('mouse','up')
    assert abs(start['x']-moved['x'])+abs(start['y']-moved['y'])>30, (start,moved)
    w.checkpoint('native-wheel-and-minimap-drag')

    w.scenario('growth-vote',4)
    wait('!!document.querySelector("[role=dialog] [data-sf-route-choice]")')
    w.checkpoint('expansion-dialog-desktop')
    click('[role=dialog] [aria-label="关闭"]')
    wait('!document.querySelector("[role=dialog]")')
    text=js('document.querySelector("[data-sf-countdown]").textContent')
    assert '选择' in text and '自动开始' not in text, text
    b('wait','1100');assert not js('!!document.querySelector("[role=dialog]")')
    click('[data-sf-open-choice]');wait('!!document.querySelector("[role=dialog]")')
    b('set','viewport','320','844');w.checkpoint('expansion-dialog-mobile')
    click('[role=dialog] [aria-label="关闭"]')
    w.pause_all(4)
    assert '暂停' in js('document.querySelector("[data-sf-countdown]").textContent')
    b('reload');b('wait','.starfort-board');wait('!!document.querySelector("[role=dialog]")')
    assert js('[...document.querySelectorAll("[data-sf-route-choice],[data-sf-boon-choice]")].every(e=>e.disabled)')
    click('[role=dialog] [aria-label="关闭"]');w.resume_all(4)
    click('[data-sf-open-choice]')
    original=w.data();choice=original['world']['choice']['options'][0]['id']
    click('[data-sf-route-choice="'+choice+'"]');click('[data-sf-boon-choice="reach"]')
    for peer in w.PEERS:
        peer.action('route_vote',choice=choice);peer.action('boon_vote',choice='reach')
    wait('!document.querySelector("[role=dialog]")')
    assert '自动开始' in js('document.querySelector("[data-sf-countdown]").textContent')
    w.pause_all(4)
    for width,height in [(1366,900),(390,844),(320,740)]:
        b('set','viewport',str(width),str(height))
        b('scrollintoview','.starfort-board')
        rects=js('(()=>{const board=document.querySelector(".starfort-board").getBoundingClientRect();return Object.fromEntries([".sf-overview","[data-sf-countdown]",".sf-world-viewport","[data-sf-ready], [data-sf-resume]"].map(s=>{const r=document.querySelector(s).getBoundingClientRect();return [s,{top:r.top-board.top,bottom:r.bottom-board.top,width:r.width,height:r.height}]}))})()')
        assert all(r['width']>0 and r['bottom']<=height for r in rects.values()), rects
        w.artifact('layout-'+str(width)+'.json',rects)
        w.resources('layout-'+str(width));w.checkpoint('compact-layout-'+str(width))
        js('document.querySelector(".starfort-board").scrollIntoView({block:"start"})')
        wait('document.querySelector("[data-starfort-canvas]").getBoundingClientRect().bottom<=innerHeight')
        b('screenshot',str(w.OUT/('primary-viewport-'+str(width)+'.png')))
    w.resume_all(4)
    # Real server timer, not a simulated click, must drive the next wave.
    deadline=time.monotonic()+55
    while time.monotonic()<deadline and w.data()['phase']=='prepare':
        value=js('document.querySelector("[data-sf-countdown]").textContent')
        assert '自动开始' in value,value
        time.sleep(1)
    assert w.data()['phase']=='battle'
    w.checkpoint('automatic-wave-start-explained')
    b('set','viewport','1366','900')
    w.scenario('impact')
    atlas=json.loads((w.ROOT/'games/starfort/assets/public/starfort/v1/atlas.json').read_text())['frames']
    keys={str(f['x'])+':'+str(f['y']):name for name,f in atlas.items()}
    js('window.__impactKeys='+json.dumps(keys))
    js(r"""(()=>{
      const c=document.querySelector('[data-starfort-canvas]').getContext('2d');
      const clear=c.clearRect,fill=c.fillRect,draw=c.drawImage;
      window.__impactFrames=[];
      c.clearRect=function(...a){window.__impactFrame={at:performance.now(),bars:[],shots:0,hits:0};window.__impactFrames.push(window.__impactFrame);return clear.apply(this,a)};
      let barWidth=0;
      c.fillRect=function(...a){if(this.fillStyle==='#283b34')barWidth=a[2]-2;if(this.fillStyle==='#add584'&&barWidth)window.__impactFrame.bars.push(a[2]/barWidth);return fill.apply(this,a)};
      c.drawImage=function(...a){const key=window.__impactKeys[a[1]+':'+a[2]]||'';if(key.startsWith('projectile-'))window.__impactFrame.shots++;if(key.startsWith('effect-'))window.__impactFrame.hits++;return draw.apply(this,a)};
    })()""")
    b('wait','8000')
    frames=js('window.__impactFrames')
    hits=[]
    for i in range(1,len(frames)):
        prev,current=frames[i-1],frames[i]
        if prev['bars'] and (not current['bars'] or current['bars'][0]<prev['bars'][0]-.001):
            recent=[f for f in frames[max(0,i-20):i] if f['shots']]
            assert recent, ('damage without preceding projectile',prev,current)
            flight=current['at']-recent[0]['at']
            assert 120<=flight<=300, ('health committed outside arrival window',flight)
            assert not current['shots'], ('health fell with projectile still in flight',current)
            hits.append({'at':current['at'],'flightMS':flight,'before':prev['bars'],'after':current['bars']})
    assert any(not h['after'] for h in hits), ('death did not follow visual impact',hits)
    assert len(hits)>=3, ('not enough real timed impacts',hits)
    w.artifact('native-impact-timing.json',{'impacts':hits,'frames':frames})
    w.checkpoint('native-projectile-before-health')
    # Inspect actual Canvas tower ink, then click its body rather than its pad.
    b('set','viewport','320','740');click('[data-sf-fit]')
    js(r"""(()=>{const c=document.querySelector('[data-starfort-canvas]').getContext('2d'),draw=c.drawImage;window.__towerInk=[];c.drawImage=function(...a){const k=window.__impactKeys[a[1]+':'+a[2]]||'';if(k.startsWith('tower-')&&a[0].src.includes('/v1/')){const m=this.getTransform(),p=m.transformPoint({x:a[5],y:a[6]}),q=m.transformPoint({x:a[5]+a[7],y:a[6]+a[8]}),r=this.canvas.getBoundingClientRect(),s=r.width/this.canvas.width;window.__towerInk=[{x:r.left+p.x*s,y:r.top+p.y*s,w:(q.x-p.x)*s,h:(q.y-p.y)*s}]}return draw.apply(this,a)}})()""")
    b('scrollintoview','[data-starfort-canvas]');wait('window.__towerInk.length>0')
    rings=js('[...document.querySelectorAll(".sf-pad:not(.sf-pad-occupied):not([hidden])")].map(e=>({hit:e.getBoundingClientRect().width,ink:e.querySelector("span").getBoundingClientRect().width}))')
    assert rings and all(r['ink']<=20 and r['hit']>=30 for r in rings), rings
    assert not js('!!document.querySelector(".sf-pad-occupied > span")'), 'built tower retained the build marker'
    ink=js('window.__towerInk[0]');hit=js('document.querySelector(".sf-pad-occupied").getBoundingClientRect().toJSON()')
    assert abs(ink['x']-hit['x'])<2 and abs(ink['y']-hit['y'])<2 and abs(ink['w']-hit['width'])<2 and abs(ink['h']-hit['height'])<2, (ink,hit)
    w.mouse_click(ink['x']+ink['w']/2,ink['y']+ink['h']*.35)
    wait('!!document.querySelector("[role=dialog] [data-sf-upgrade]")')
    w.checkpoint('mobile-small-pad-tower-body-upgrade')
    w.artifact('tower-ink-and-hitbox.json',{'towerInk':ink,'hitbox':hit,'emptyPads':rings})
    click('[role=dialog] [aria-label="关闭"]')
    # Drag from the tower body: camera motion is allowed, upgrade must not open.
    for _ in range(4):click('[data-sf-zoom-in]')
    # Locate it through the real minimap before sampling the latest draw.
    value=w.data();pad=next(p for p in value['map']['pads'] if p['id']==0)
    b('scrollintoview','[data-sf-minimap]')
    point=js('(()=>{const e=document.querySelector("[data-sf-minimap]"),p=e.createSVGPoint();p.x='+str(pad['x'])+';p.y='+str(pad['y'])+';const q=p.matrixTransform(e.getScreenCTM());return {x:q.x,y:q.y}})()')
    w.mouse_click(point['x'],point['y']);b('scrollintoview','[data-starfort-canvas]')
    b('wait','--fn','document.querySelector(".sf-pad-occupied").getBoundingClientRect().width>42')
    ink=js('window.__towerInk[0]');x,y=ink['x']+ink['w']/2,ink['y']+ink['h']*.35
    b('mouse','move',x,y);b('mouse','down');b('mouse','move',x+35,y+20);b('mouse','up')
    assert not js('!!document.querySelector("[role=dialog]")'), 'dragging tower opened upgrade'
    w.checkpoint('tower-body-drag-without-upgrade')
    b('wait','350');click('[data-sf-pad="0"]')
    click('[data-sf-sell]')
    wait('!!document.querySelector(\'[data-sf-pad="0"]:not(.sf-pad-occupied) > span\')')
    assert len(w.data()['towers'])==0
    click('[role=dialog] [aria-label="关闭"]')
    w.checkpoint('sold-tower-restores-build-point')
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
