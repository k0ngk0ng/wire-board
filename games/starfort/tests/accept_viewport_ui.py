#!/usr/bin/env python3
"""Real Starfort viewport E2E. Failure cases: viewport-ui-acceptance.md.
Build matching web/dist first; --baseline records old behavior without asserting
new geometry. Native touch/wheel are delivered to the agent-browser owned page.
"""
import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
import time

ROOT = Path(__file__).resolve().parents[3]
BASELINE = '--baseline' in sys.argv
OUT = Path(os.environ.get('STARFORT_VIEWPORT_OUT', str(ROOT / '.local/starfort-viewport/browser' / ('before' if BASELINE else 'after')))).resolve()
assert OUT.is_relative_to(ROOT), 'Artifacts must stay in the current repository'
os.environ['WIRE_BOARD_STARFORT_BROWSER_OUT'] = str(OUT)
import accept_world_ui as w

w.OUT = OUT
w.MAIN = '视区主守望者'
OUT.mkdir(parents=True, exist_ok=True)
CACHE = Path(os.environ.get('STARFORT_VIEWPORT_CACHE_ROOT', str(ROOT)))
for key, path in {'GOCACHE': CACHE/'.local/go-build', 'GOMODCACHE': CACHE/'.local/go-mod', 'GOTMPDIR': OUT/'tmp', 'TMPDIR': OUT/'tmp', 'AGENT_BROWSER_SOCKET_DIR': Path(os.environ.get('STARFORT_VIEWPORT_SOCKET_DIR',str(ROOT/'.local/starfort-viewport/s')))}.items():
    path.mkdir(parents=True, exist_ok=True)
    w.ENV[key] = str(path)
w.ENV['WIRE_BOARD_STARFORT_BROWSER_OUT'] = str(OUT)
w.ENV['AGENT_BROWSER_SESSION']='sfv-'+w.SESSION.rsplit('-',1)[-1]
w.REPORT = {'passed': False, 'baseline': BASELINE, 'visualOnly': '--visual-only' in sys.argv, 'checks': [], 'failures': [], 'command': ' '.join(sys.argv), 'baseCommit': subprocess.check_output(['git','rev-parse','HEAD'],cwd=ROOT,text=True).strip(), 'scope': 'Starfort production Canvas, real HTTP rooms, native Chromium mouse/wheel/touch and platform GameFullscreen only.', 'limitations': ['Milestone fixture seeds stage4/battle; not a natural complete match.', '320px Chromium viewport and native browser touch; no physical phone or Safari.']}
b, js, wait, click = w.b, w.js, w.wait, w.click

def verify(name, condition, detail=None):
    item = {'name': name, 'passed': bool(condition), 'detail': detail}
    w.REPORT['checks'].append(item)
    if not condition:
        w.REPORT['failures'].append(item)
        if not BASELINE: raise AssertionError((name, detail))

def source_hashes():
    paths = list((ROOT/'games/starfort/web').glob('*')) + [Path(__file__), ROOT/'games/starfort/tests/accept_world_ui.py', ROOT/'games/starfort/tests/browser_fixture_test.go', ROOT/'scripts/browser_touch.mjs', ROOT/'scripts/browser_wheel.mjs', ROOT/'web/src/ui/room-fullscreen.tsx', ROOT/'web/src/ui/room-fullscreen.css']
    return {str(p.relative_to(ROOT)): hashlib.sha256(p.read_bytes()).hexdigest() for p in paths if p.is_file()}

def build_hashes():
    return {str(p.relative_to(ROOT)): hashlib.sha256(p.read_bytes()).hexdigest() for p in (ROOT/'web/dist').rglob('*') if p.is_file()}

def rect(selector):
    return js('document.querySelector('+json.dumps(selector)+').getBoundingClientRect().toJSON()')

def stable():
    js('new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(resolve)))')

def toggle_enter():
    if not js('!!document.querySelector("[data-fullscreen-enter]")'):
        b('click', '.compact-room-tools')
    b('click', '[data-fullscreen-enter]')
    wait('!!document.querySelector("[data-room-fullscreen]")')
    stable()

def toggle_exit(key=False):
    if key: b('press', 'Escape')
    else: b('click', '[data-room-fullscreen] [data-fullscreen-toggle]')
    wait('!document.querySelector("[data-room-fullscreen]") && !document.fullscreenElement')
    stable()
    verify('fullscreen cleanup', js('!document.querySelector("[inert]") && document.documentElement.style.overflow!=="hidden"'))

def screenshot(name):
    b('screenshot', str(OUT/(name+'.png')))

MEASURE = r'''(()=>{
 const box=s=>{const e=document.querySelector(s);return e?e.getBoundingClientRect().toJSON():null};
 const rgb=s=>(s.match(/[\d.]+/g)||[]).map(Number),lum=c=>c.map(v=>v/255).map(v=>v<=.04045?v/12.92:((v+.055)/1.055)**2.4).reduce((s,v,i)=>s+v*[.2126,.7152,.0722][i],0);
 const styles=e=>{let bg=[255,255,255],chain=[];for(let n=e;n;n=n.parentElement)chain.unshift(n);for(const n of chain){const a=rgb(getComputedStyle(n).backgroundColor),o=a[3]??1;bg=a.slice(0,3).map((v,i)=>v*o+bg[i]*(1-o))}const s=getComputedStyle(e),a=rgb(s.color),opacity=chain.reduce((v,n)=>v*Number(getComputedStyle(n).opacity),a[3]??1),fg=a.slice(0,3).map((v,i)=>v*opacity+bg[i]*(1-opacity));return {foreground:fg,background:bg,ratio:(Math.max(lum(fg),lum(bg))+.05)/(Math.min(lum(fg),lum(bg))+.05),fontSize:s.fontSize}};
 const required=['[data-sf-base]','[data-sf-wave]','[data-sf-own-gold]','[data-sf-countdown] strong','[data-sf-zoom-in]','[data-sf-zoom-out]','[data-sf-camera-base]','[data-sf-fit]','.sf-select-pad','[data-sf-open-info]',...(document.querySelector('.starfort-board').dataset.phase==='paused'?['[data-sf-resume]']:['[data-sf-ready]','[data-sf-pause]'])].map(selector=>{const e=document.querySelector(selector);if(!e)return {selector,visible:false};const r=e.getBoundingClientRect(),p=document.elementFromPoint(r.left+r.width/2,r.top+r.height/2);return {selector,visible:r.width>0&&r.height>0&&r.left>=0&&r.right<=innerWidth+1&&r.top>=0&&r.bottom<=innerHeight+1&&!!p&&(e.contains(p)||p.contains(e))}});
 const entries=[...document.querySelectorAll('.sf-overview span,.sf-overview strong,[data-sf-countdown] strong,[data-sf-countdown] > span,.sf-control-buttons button,.sf-camera-toolbar button,.sf-camera-toolbar output,.sf-map-navigation p')].filter(e=>e.getClientRects().length).map(e=>{const range=document.createRange();range.selectNodeContents(e);const r=range.getBoundingClientRect(),p=document.elementFromPoint(r.left+r.width/2,r.top+r.height/2);return {text:e.textContent.trim(),box:r.toJSON(),visible:r.left>=-1&&r.right<=innerWidth+1&&r.top>=-1&&r.bottom<=innerHeight+1,unoccluded:!!p&&(e.contains(p)||p.contains(e)),...styles(e)}});
 return {viewport:{width:innerWidth,height:innerHeight},canvas:box('[data-starfort-canvas]'),frame:box('.sf-world-viewport'),board:box('.starfort-board'),surface:box('[data-game-fullscreen-surface]'),navigation:box('.sf-map-navigation'),caption:box('.sf-map-caption'),controls:box('.sf-wave-controls'),mapTools:box('.sf-map-tools'),fieldActions:box('.sf-field-actions'),overview:box('.sf-overview'),fullscreen:document.querySelector('[data-game-fullscreen-surface]')?.dataset.roomFullscreen||'',native:document.fullscreenElement?.hasAttribute('data-game-fullscreen-surface')||false,entries,required,horizontalOverflow:document.documentElement.scrollWidth>innerWidth+2};
})()'''

def measure(name, fullscreen=False):
    stable()
    value = js(MEASURE)
    c, v = value['canvas'], value['viewport']
    value['canvasAreaFraction'] = c['width']*c['height']/(v['width']*v['height'])
    value['canvasWidthFraction'] = c['width']/value['board']['width']
    value['aspectRatio'] = c['width']/c['height']
    w.artifact(name+'.geometry.json', value)
    screenshot(name)
    verify(name+' no horizontal overflow', not value['horizontalOverflow'], value)
    verify(name+' text contrast', all(e['ratio']>=4.5 for e in value['entries']), [e for e in value['entries'] if e['ratio']<4.5])
    if fullscreen:
        verify(name+' required resource and time text visible',all(e['visible'] for e in value['required']),value['required'])
        verify(name+' primary text visible and unobscured', all(e['visible'] and e['unoccluded'] for e in value['entries']), [e for e in value['entries'] if not(e['visible'] and e['unoccluded'])])
        a=value['mapTools'];other=value['controls']
        overlap_width=max(0,min(a['right'],other['right'])-max(a['left'],other['left'])) if a and other else 0
        overlap_height=max(0,min(a['bottom'],other['bottom'])-max(a['top'],other['top'])) if a and other else 0
        verify(name+' map and wave control panels do not overlap',overlap_width<.5 or overlap_height<.5,{'overlapWidth':overlap_width,'overlapHeight':overlap_height})
        verify(name+' canvas uses available width', c['width']>=value['board']['width']*.95, value['canvasWidthFraction'])
        verify(name+' canvas fits viewport', c['bottom']<=v['height']+1 and c['left']>=0 and c['right']<=v['width'], c)
        last_control_bottom=max(value[key]['bottom'] for key in ['navigation','controls','mapTools','fieldActions'] if value[key])
        gap = v['height'] - last_control_bottom
        verify(name+' navigation fits viewport',0<=value['navigation']['top'] and value['navigation']['bottom']<=v['height']+1,value['navigation'])
        verify(name+' no large empty area below map controls', -1<=gap<=80, gap)
        if v['width']>850:
            verify(name+' full desktop map area', value['canvasAreaFraction']>=.60, value['canvasAreaFraction'])
        elif v['height']>v['width']:
            verify(name+' full mobile map height', c['height']>=v['height']*.40, c['height'])
    elif v['width']>850:
        verify(name+' expanded ordinary desktop map', c['width']>=min(value['board']['width']*.90, 1440) and c['height']>=400, c)
        verify(name+' ordinary desktop proportion', 1.45<=value['aspectRatio']<=2.6, value['aspectRatio'])
    return value

def setup():
    for name in ['url','stop']: (OUT/name).unlink(missing_ok=True)
    with (OUT/'server.log').open('w') as log:
        w.SERVER=subprocess.Popen(['go','test','-count=1','-timeout=20m','-run','^TestStarfortBrowserFixture$','./games/starfort/tests'],cwd=ROOT,env=w.ENV,stdout=log,stderr=subprocess.STDOUT)
    deadline=time.monotonic()+120
    while not (OUT/'url').exists():
        if w.SERVER.poll() is not None or time.monotonic()>deadline: raise RuntimeError('Fixture startup failed; see server.log')
        time.sleep(.1)
    w.BASE=(OUT/'url').read_text()
    b('open',w.BASE);w.OPENED=True
    w.REPORT['browserUserAgent']=js('navigator.userAgent')
    w.api('/register',{'name':w.MAIN,'password':'starfort-browser-test','invite':'starfort-browser'})
    w.login('上架管理员');w.label('管理后台');b('wait','input[aria-label="搜索管理游戏"]')
    b('fill','input[aria-label="搜索管理游戏"]','星垒')
    if '已下架' in js('document.querySelector(".admin-game-card").textContent'):
        click('.admin-game-card button');w.label('确认操作')
    wait('document.querySelector(".admin-game-card")?.textContent.includes("已上架")')
    w.login(w.MAIN)

def waiting_room():
    w.api('/rooms',{'kind':'starfort','name':'全屏入口检查','capacity':1,'options':{'scenario':'natural','mode':'endless','waves':20,'difficulty':'normal','prepSeconds':45}})
    b('wait','.waiting-layout')
    verify('waiting room hides fullscreen', not js('!!document.querySelector("[data-fullscreen-enter],[data-fullscreen-toggle]")'))
    w.command('leave')

def geometry_matrix():
    w.scenario('stage4')
    # Stage4 is a live battle: production rules allow pause only in preparation.
    # Preserve identity while normal server movement and damage continue.
    for width,height in [(1366,768),(1920,1080),(2560,1080),(320,740),(740,320)]:
        b('set','viewport',str(width),str(height));click('[data-sf-fit]')
        b('scrollintoview','.starfort-board');stable()
        measure(f'normal-{width}x{height}')
        js('window.__viewportCanvas=document.querySelector("[data-starfort-canvas]");true')
        b('scrollintoview','[data-fullscreen-enter]')
        before=w.data(); before_camera=w.camera(); scroll=js('scrollY')
        toggle_enter()
        verify('native fullscreen target', js('document.fullscreenElement===document.querySelector("[data-game-fullscreen-surface]")'))
        measure(f'native-{width}x{height}',True)
        verify('canvas not remounted entering',js('window.__viewportCanvas===document.querySelector("[data-starfort-canvas]")'))
        verify('public room UI outside target',js('!document.querySelector("[data-room-fullscreen]").querySelector(".room-heading,.room-tools,.players-strip,.topbar")'))
        toggle_exit(key=True)
        verify('canvas not remounted exiting',js('window.__viewportCanvas===document.querySelector("[data-starfort-canvas]")'))
        after=w.data();after_camera=w.camera()
        verify('fullscreen restores normal camera',all(abs(before_camera[k]-after_camera[k])<.05 for k in ['x','y','zoom']),{'before':before_camera,'after':after_camera})
        verify('fullscreen preserves live match',before['wave']==after['wave'] and [(t['id'],t['pad'],t['kind'],t['level']) for t in before['towers']]==[(t['id'],t['pad'],t['kind'],t['level']) for t in after['towers']],{'before':before['wave'],'after':after['wave']})
        verify('fullscreen restores page scroll',abs(js('scrollY')-scroll)<3,{'before':scroll,'after':js('scrollY')})
        # Explicit capability failure probe; no synthetic game state or input.
        js('window.__nativeRequest=Element.prototype.requestFullscreen;window.__webkitRequest=Element.prototype.webkitRequestFullscreen;Element.prototype.requestFullscreen=()=>Promise.reject(new Error("viewport acceptance denied"));Element.prototype.webkitRequestFullscreen=undefined;true')
        toggle_enter()
        verify('page fallback mode',js('document.querySelector("[data-room-fullscreen]").dataset.roomFullscreen==="page"&&!document.fullscreenElement'))
        measure(f'fallback-{width}x{height}',True)
        toggle_exit()
        js('Element.prototype.requestFullscreen=window.__nativeRequest;Element.prototype.webkitRequestFullscreen=window.__webkitRequest;true')

def tap(selector):
    b('scrollintoview',selector)
    r=rect(selector)
    native('scripts/browser_touch.mjs','tap',r['x']+r['width']/2,r['y']+r['height']/2,0,0)

def observe_tower_ink():
    # Read actual sprite draw calls and the alpha at the chosen pixel. Keep the
    # instrument on this Canvas instance only; do not modify game state/input.
    js(r"""(()=>{
      const c=document.querySelector('[data-starfort-canvas]').getContext('2d'),draw=c.drawImage,clear=c.clearRect,cache=new WeakMap();
      window.__viewportTowerDraws=[];
      c.clearRect=function(...a){window.__viewportTowerDraws=[];return clear.apply(this,a)};
      c.drawImage=function(...a){
        const image=a[0];
        if(a.length===5&&image instanceof HTMLCanvasElement&&image.width===204&&image.height===236){
          let pixel=cache.get(image);
          if(!pixel){
            const data=image.getContext('2d').getImageData(0,0,image.width,image.height).data;
            for(let d=0;d<45&&!pixel;d++)for(let dy=-d;dy<=d&&!pixel;dy++)for(let dx=-d;dx<=d&&!pixel;dx++){
              const x=102+dx,y=82+dy,alpha=data[(y*204+x)*4+3];if(alpha>=200)pixel={x,y,alpha};
            }
            cache.set(image,pixel);
          }
          const m=this.getTransform(),r=this.canvas.getBoundingClientRect(),s=r.width/this.canvas.width,
            screen=(x,y)=>{const p=m.transformPoint({x,y});return {x:r.left+p.x*s,y:r.top+p.y*s}},
            p=screen(a[1]+a[3]*6/204,a[2]+a[4]*6/236),q=screen(a[1]+a[3]*198/204,a[2]+a[4]*230/236);
          if(pixel)window.__viewportTowerDraws.push({box:{x:p.x,y:p.y,width:q.x-p.x,height:q.y-p.y},point:screen(a[1]+a[3]*pixel.x/204,a[2]+a[4]*pixel.y/236),alpha:pixel.alpha});
        }
        return draw.apply(this,a);
      };
    })()""")

def dialog_interactions():
    b('set','viewport','320','740');w.scenario('battle');toggle_enter();click('[data-sf-fit]')
    # Build on an actual visible empty pad, using native touch and real actions.
    pad=js('(()=>{const c=document.querySelector("[data-starfort-canvas]").getBoundingClientRect();return [...document.querySelectorAll(".sf-pad:not(.sf-pad-occupied):not([hidden])")].find(e=>{const r=e.getBoundingClientRect();const hit=document.elementFromPoint(r.left+r.width/2,r.top+r.height/2);return r.left>=c.left&&r.right<=c.right&&r.top>=c.top&&r.bottom<=c.bottom&&!!hit&&e.contains(hit)})?.dataset.sfPad})()')
    verify('visible touch build point',pad is not None)
    edges=js(r"""(()=>{const rgb=s=>(s.match(/[\d.]+/g)||[]).map(Number),lum=c=>c.slice(0,3).map(v=>v/255).map(v=>v<=.04045?v/12.92:((v+.055)/1.055)**2.4).reduce((s,v,i)=>s+v*[.2126,.7152,.0722][i],0),ratio=(a,b)=>(Math.max(lum(rgb(a)),lum(rgb(b)))+.05)/(Math.min(lum(rgb(a)),lum(rgb(b)))+.05);return [...document.querySelectorAll('.sf-pad:not([hidden]) > span')].filter(e=>e.getClientRects().length).map(e=>{const s=getComputedStyle(e);return {border:s.borderColor,background:s.backgroundColor,ratio:ratio(s.borderColor,s.backgroundColor)}})})()""")
    verify('actual empty pad boundary contrast',bool(edges) and all(e['ratio']>=3 for e in edges),edges)
    w.artifact('nontext-boundary-contrast.json',edges)
    initial=w.data();tap('[data-sf-pad="'+pad+'"]')
    wait('!!document.querySelector("[role=dialog] [data-sf-build=arrow]")')
    verify('build dialog inside fullscreen',js('document.querySelector("[data-room-fullscreen]").contains(document.querySelector("[role=dialog]"))'))
    screenshot('mobile-fullscreen-build');tap('[data-sf-build=arrow]')
    wait('!!document.querySelector("[role=dialog] [data-sf-upgrade=level]")')
    built=w.data();tower=next(t for t in built['towers'] if t['pad']==int(pad))
    wait('document.querySelector("[data-sf-own-gold]").textContent==='+json.dumps(str(built['players'][0]['gold'])))
    verify('touch build updates server gold and towers',len(built['towers'])==len(initial['towers'])+1 and built['players'][0]['gold']<initial['players'][0]['gold'])
    observe_tower_ink()
    click('[role=dialog] [aria-label="关闭"]')
    verify('built tower hides pad marker',js('!document.querySelector('+json.dumps('[data-sf-pad="'+pad+'"] > span')+')'))
    # The occupied hitbox is computed from the drawn tower dimensions. Tap its
    # upper body, not the ground pad, to exercise camera-aware tower selection.
    # Empty-pad to tower CSS transforms can animate briefly. Require actual
    # draw/hit agreement before sampling, without relaxing the 2px contract.
    wait('(()=>{const r=document.querySelector('+json.dumps('[data-sf-pad="'+pad+'"]')+').getBoundingClientRect();return window.__viewportTowerDraws?.some(d=>["x","y","width","height"].every(k=>Math.abs(d.box[k]-r[k])<2))})()')
    stable()
    r=rect('[data-sf-pad="'+pad+'"]')
    wait('window.__viewportTowerDraws?.length>0')
    drawn=js('window.__viewportTowerDraws')
    ink=min(drawn,key=lambda d:abs(d['box']['x']-r['x'])+abs(d['box']['y']-r['y']))
    verify('drawn tower matches touch target',all(abs(ink['box'][k]-r[k])<2 for k in ['x','y','width','height']),{'drawn':ink,'hitbox':r})
    verify('actual tower body pixel is opaque',ink['alpha']>=200,ink)
    verify('actual painted body point hits tower',js('document.querySelector('+json.dumps('[data-sf-pad="'+pad+'"]')+').contains(document.elementFromPoint('+str(ink['point']['x'])+','+str(ink['point']['y'])+'))'))
    native('scripts/browser_touch.mjs','tap',ink['point']['x'],ink['point']['y'],0,0)
    wait('!!document.querySelector("[role=dialog] [data-sf-upgrade=level]")')
    tap('[data-sf-upgrade=level]');wait('document.querySelector("[data-sf-upgrade=level]")?.textContent.includes("3")')
    upgraded=w.data();wait('document.querySelector("[data-sf-own-gold]").textContent==='+json.dumps(str(upgraded['players'][0]['gold'])))
    verify('native tower body upgrade',next(t for t in upgraded['towers'] if t['id']==tower['id'])['level']==2)
    js('window.__selectedCanvas=document.querySelector("[data-starfort-canvas]");true')
    selected=js('document.querySelector(".sf-pad-selected")?.dataset.sfPad')
    screenshot('mobile-fullscreen-upgraded')
    # Browser API exit models the browser chrome's native fullscreen exit while
    # a game dialog is open; this changes capability state, never game state.
    js('document.exitFullscreen()');wait('!document.querySelector("[data-room-fullscreen]")')
    verify('native exit preserves open tower selection',js('document.querySelector(".sf-pad-selected")?.dataset.sfPad')==selected and js('!!document.querySelector("[role=dialog]")'))
    verify('native exit preserves canvas node',js('window.__selectedCanvas===document.querySelector("[data-starfort-canvas]")'))
    click('[role=dialog] [aria-label="关闭"]')
    w.artifact('touch-build-upgrade.json',{'pad':pad,'beforeGold':initial['players'][0]['gold'],'built':tower,'towerBodyHit':{'hitbox':r,'actualDraw':ink},'upgraded':next(t for t in upgraded['towers'] if t['id']==tower['id'])})
    # Choice countdown and portal remain visible in both viewport orientations.
    w.scenario('growth-vote');wait('!!document.querySelector("[role=dialog]")');click('[role=dialog] [aria-label="关闭"]')
    toggle_enter();click('[data-sf-open-choice]');wait('!!document.querySelector("[role=dialog] [data-sf-route-choice]")')
    verify('world choice stays in fullscreen target',js('document.querySelector("[data-room-fullscreen]").contains(document.querySelector("[role=dialog]"))'))
    screenshot('mobile-fullscreen-choice');click('[role=dialog] [aria-label="关闭"]')
    measure('mobile-choice-countdown',True)
    b('set','viewport','740','320');measure('landscape-choice-countdown',True)
    b('set','viewport','320','740');click('[data-sf-open-choice]');screenshot('mobile-choice-reopened');click('[role=dialog] [aria-label="关闭"]');toggle_exit(key=True)
    # Plain preparation exposes the actual countdown digits, not only battle text.
    w.scenario('natural');toggle_enter();measure('mobile-prepare-countdown',True);toggle_exit()

def visual_detail():
    for scenario in ['stage4','battle']:
        w.scenario(scenario);b('set','viewport','1920','1080');toggle_enter();click('[data-sf-camera-base]')
        stable();screenshot(scenario+'-detail-1920')
        w.artifact(scenario+'-detail.state.json',w.data())
        b('set','viewport','320','740');click('[data-sf-camera-base]');stable();screenshot(scenario+'-detail-320');toggle_exit()
    w.scenario('growth-vote');wait('!!document.querySelector("[role=dialog]")')
    choices=w.data()['world']['choice']['options'];w.artifact('river-candidates.json',choices)
    river=next((o for o in choices if any(r.get('biome')=='river' for r in o['map'].get('regions',[]))),None)
    if river:
        click('[data-sf-route-choice="'+river['id']+'"]');click('[data-sf-boon-choice="reach"]')
        wait('!document.querySelector("[role=dialog]")');w.pause_all(1)
        b('set','viewport','1920','1080');toggle_enter();click('[data-sf-fit]');screenshot('river-selected-1920');w.artifact('river-selected.state.json',w.data());toggle_exit()

def native(helper, *args):
    endpoint=b('get','cdp-url')['cdpUrl']
    result=subprocess.run(['node',helper,endpoint,w.BASE,*map(str,args)],cwd=ROOT,env=w.ENV,capture_output=True,text=True,timeout=20)
    with (OUT/'commands.jsonl').open('a') as log:
        log.write(json.dumps({'nativeInputHelper':helper,'arguments':args,'returncode':result.returncode,'stdout':result.stdout,'stderr':result.stderr},ensure_ascii=False)+'\n')
    result.check_returncode()

def reachable_pad():
    # Place a known empty point beneath the HUD using real panning, then reach
    # that same point through minimap navigation and another native drag.
    for _ in range(5):click('[data-sf-zoom-in]')
    value=w.data();base=value['map']['base'];occupied={t['pad'] for t in value['towers']}
    pad=min((p for p in value['map']['pads'] if p['id'] not in occupied),key=lambda p:(p['x']-base['x'])**2+(p['y']-base['y'])**2)
    selector='[data-sf-pad="'+str(pad['id'])+'"]'
    point=js('(()=>{const e=document.querySelector("[data-sf-minimap]"),p=e.createSVGPoint();p.x='+str(pad['x'])+';p.y='+str(pad['y'])+';const q=p.matrixTransform(e.getScreenCTM());return {x:q.x,y:q.y}})()')
    native('scripts/browser_touch.mjs','tap',point['x'],point['y'],0,0)
    c=rect('[data-starfort-canvas]');r=rect(selector);hud=rect('.sf-hud')
    x=c['x']+c['width']*.5;y=c['y']+c['height']*.65
    dx=(hud['left']+hud['right'])/2-(r['left']+r['right'])/2;dy=hud['bottom']-20-(r['top']+r['bottom'])/2
    native('scripts/browser_touch.mjs','drag',x,y,x+dx,y+dy)
    blocked=js('(()=>{const e=document.querySelector('+json.dumps(selector)+'),r=e.getBoundingClientRect(),p=document.elementFromPoint(r.left+r.width/2,r.top+r.height/2);return {padBox:r.toJSON(),coveredByHUD:!!p?.closest(".sf-hud")}})()')
    verify('HUD coverage scenario established',blocked['coveredByHUD'],blocked)
    screenshot('mobile-hud-covered-point')
    native('scripts/browser_touch.mjs','tap',point['x'],point['y'],0,0)
    native('scripts/browser_touch.mjs','drag',x,y,x+20,y+25)
    verify('native drag does not open build dialog',not js('!!document.querySelector("[role=dialog]")'))
    wait('(()=>{const e=document.querySelector('+json.dumps(selector)+'),r=e.getBoundingClientRect(),p=document.elementFromPoint(r.left+r.width/2,r.top+r.height/2);return !e.hidden&&!!p&&e.contains(p)})()')
    # The production 300ms post-drag guard must expire before an intentional
    # second tap; the preceding assertion separately verifies ghost-click safety.
    b('wait','350')
    r=rect(selector);native('scripts/browser_touch.mjs','tap',r['x']+r['width']/2,r['y']+r['height']/2,0,0)
    wait('!!document.querySelector("[role=dialog].sf-command-dialog")')
    verify('panning reaches the HUD-covered map point',js('document.querySelector(".sf-pad-selected")?.dataset.sfPad')==str(pad['id']))
    w.artifact('pan-reachable-point.json',{'initialHUDCover':blocked,'pad':pad,'minimapTouch':point,'reachedHitbox':r})
    screenshot('mobile-pan-reached-point');click('[role=dialog] [aria-label="关闭"]')

def information_dialog(name):
    js('window.__infoCanvas=document.querySelector("[data-starfort-canvas]");true')
    click('[data-sf-open-info]');wait('!!document.querySelector(".sf-info-dialog .sf-information")')
    verify(name+' info dialog inside game target',js('document.querySelector("[data-room-fullscreen]").contains(document.querySelector(".sf-info-dialog"))'))
    verify(name+' information panels retained',js('document.querySelector(".sf-info-dialog").textContent.includes("战场情报")&&document.querySelector(".sf-info-dialog").textContent.includes("防御塔")'))
    screenshot(name+'-information');click('[role=dialog] [aria-label="关闭"]')
    b('wait','1100')
    verify(name+' ordinary snapshots keep info closed',not js('!!document.querySelector(".sf-info-dialog")'))
    click('[data-sf-open-info]');wait('!!document.querySelector(".sf-info-dialog")');click('[role=dialog] [aria-label="关闭"]')
    verify(name+' info preserves canvas',js('window.__infoCanvas===document.querySelector("[data-starfort-canvas]")'))

def camera_interactions():
    b('set','viewport','1366','768');toggle_enter();information_dialog('desktop');click('[data-sf-fit]')
    # Both world axes must exceed the viewport before testing a free zoom
    # anchor: at fit zoom, legal camera clamping intentionally centers an axis.
    for _ in range(3):click('[data-sf-zoom-in]')
    before=w.camera();x=before['left']+before['width']*.63;y=before['top']+before['height']*.52
    native('scripts/browser_wheel.mjs',x,y,-240)
    wait('Number(document.querySelector("[data-starfort-canvas]").dataset.zoom)>'+str(before['zoom']))
    after=w.camera()
    for axis,screen in [('x',x-before['left']),('y',y-before['top'])]:
        old=before[axis]+(screen-(before['width'] if axis=='x' else before['height'])/2)/before['zoom']
        new=after[axis]+(screen-(after['width'] if axis=='x' else after['height'])/2)/after['zoom']
        verify('native wheel anchor '+axis,abs(new-old)<3,{'old':old,'new':new})
    b('mouse','move',x,y);b('mouse','down');b('mouse','move',x-80,y-30);b('mouse','up')
    moved=w.camera();verify('native mouse pan',abs(moved['x']-after['x'])+abs(moved['y']-after['y'])>20)
    r=rect('[data-sf-minimap]');b('mouse','move',r['x']+r['width']*.35,r['y']+r['height']*.4);b('mouse','down');start=w.camera();b('mouse','move',r['x']+r['width']*.7,r['y']+r['height']*.6);end=w.camera();b('mouse','up')
    verify('continuous minimap drag before mouseup',abs(start['x']-end['x'])+abs(start['y']-end['y'])>30)
    w.artifact('native-mouse.json',{'before':before,'zoomed':after,'panned':moved,'minimapBefore':start,'minimapDuring':end})
    toggle_exit()
    b('set','viewport','320','740');toggle_enter();information_dialog('mobile');click('[data-sf-fit]');r=rect('[data-starfort-canvas]');x=r['x']+r['width']/2;y=r['y']+r['height']/2
    before=w.camera();native('scripts/browser_touch.mjs','pinch',x,y,x,y);after=w.camera();verify('native touch pinch',after['zoom']>before['zoom'])
    native('scripts/browser_touch.mjs','drag',x,y,x-60,y-25);moved=w.camera();verify('native touch pan',abs(moved['x']-after['x'])+abs(moved['y']-after['y'])>20)
    native('scripts/browser_touch.mjs','cancel',x,y,x+20,y+20)
    verify('touch drag has no accidental dialog',not js('!!document.querySelector("[role=dialog]")'))
    w.artifact('native-touch.json',{'before':before,'zoomed':after,'panned':moved})
    screenshot('mobile-native-touch');reachable_pad();toggle_exit()

try:
    w.SOURCE_START=source_hashes();w.REPORT['buildSHA256']=build_hashes()
    setup()
    if '--visual-only' in sys.argv: visual_detail()
    else: waiting_room();geometry_matrix()
    if not BASELINE and '--visual-only' not in sys.argv:
        camera_interactions();dialog_interactions()
    verify('no uncaught browser errors',not b('errors')['errors'],b('errors'))
    w.REPORT['passed']=not w.REPORT['failures']
    w.REPORT['baselineComplete']=BASELINE
except Exception as error:
    import traceback
    traceback.print_exc();w.REPORT['error']=repr(error)
    if w.OPENED:
        try:w.REPORT['snapshot']=b('snapshot','-i');screenshot('failure')
        except Exception:pass
finally:
    if w.OPENED:
        try:b('close')
        except Exception:pass
    if w.SERVER:
        (OUT/'stop').write_text('stop')
        try:w.SERVER.wait(timeout=15)
        except subprocess.TimeoutExpired:w.SERVER.terminate();w.SERVER.wait(timeout=10)
    w.REPORT['sourceSHA256']=source_hashes();w.REPORT['sourceUnchangedDuringRun']=w.SOURCE_START==w.REPORT['sourceSHA256']
    w.artifact('result.json',w.REPORT)
    if w.REPORT['passed'] or w.REPORT.get('baselineComplete'):(OUT/'failure.png').unlink(missing_ok=True)
    for name in ['profiles','tmp','socket']:
        path=OUT/name
        if path.is_dir():shutil.rmtree(path)
    for name in ['url','stop','server.log']:(OUT/name).unlink(missing_ok=True)
    socket_dir=Path(w.ENV['AGENT_BROWSER_SOCKET_DIR'])
    if socket_dir.is_dir() and not any(socket_dir.iterdir()):socket_dir.rmdir()
    print('Artifact:',OUT/'result.json',flush=True)
    if not w.REPORT['passed'] and not w.REPORT.get('baselineComplete'):raise SystemExit(1)
