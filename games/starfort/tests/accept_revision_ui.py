#!/usr/bin/env python3
"""REVISION.md: public HTTP/WebSocket, production Canvas, native desktop/touch.
Before UI implementation: weather lacks its name/42px icon; modal shop masks
the road and blocks towers; deferred rewards auto-open; merchant uses a cart.
Paused fixtures inspect layout; natural campaign acceptance is separate.
"""
import hashlib
import json
import shutil
import sys
from pathlib import Path
import accept_combat_controls_ui as c
w=c.w
w.OUT=c.ROOT/'.local/starfort-next/browser'
w.OUT.mkdir(parents=True,exist_ok=True)
(w.OUT/'tmp').mkdir(exist_ok=True)
w.ENV.update(WIRE_BOARD_STARFORT_BROWSER_OUT=str(w.OUT),AGENT_BROWSER_PROFILE=str(w.OUT/'profile'),GOTMPDIR=str(w.OUT/'tmp'),TMPDIR=str(w.OUT/'tmp'))
checks=[];passed=False
def sources():
 paths=list((c.ROOT/'games/starfort/web').glob('*'))+list((c.ROOT/'games/starfort/server').glob('*.go'))+[Path(__file__),c.ROOT/'games/starfort/tests/browser_fixture_test.go']
 return {str(p.relative_to(c.ROOT)):hashlib.sha256(p.read_bytes()).hexdigest() for p in paths if p.is_file()}
def check(name,ok,evidence=None):
 checks.append({'name':name,'passed':bool(ok),'evidence':evidence});print(('PASS 'if ok else'FAIL ')+name,flush=True);assert ok,(name,evidence)
def scene(name):
 w.scenario('revision-'+name);c.wait('document.querySelector(".room-heading h1, .room-page h1")?.textContent.includes('+json.dumps('revision-'+name)+')');c.wait('document.querySelector(".sf-battlefield")?.dataset.assetsReady==="true"');c.js('document.querySelector(".sf-battlefield").scrollIntoView({block:"start",behavior:"instant"})');c.stable()
def enter(touch):
 if not c.js('!!document.querySelector("[data-fullscreen-enter]")'):c.b('click','.compact-room-tools')
 c.js('document.querySelector("[data-fullscreen-enter]").scrollIntoView({block:"center",behavior:"instant"})');c.stable();c.activate('[data-fullscreen-enter]',touch);c.wait('!!document.querySelector("[data-room-fullscreen]")');c.stable()
try:
 c.setup();initial=sources()
 c.js('window.__revisionVisuals={};window.__revisionStroke=CanvasRenderingContext2D.prototype.stroke;CanvasRenderingContext2D.prototype.stroke=function(...a){if(this.canvas.matches("[data-starfort-canvas]")&&["#e5e8bc","#e4b5ff"].includes(this.strokeStyle)){const v=window.__revisionVisuals[this.strokeStyle]??={first:performance.now(),last:0,count:0};v.last=performance.now();v.count++}return window.__revisionStroke.apply(this,a)};window.__merchantColors=new Set();window.__revisionFill=CanvasRenderingContext2D.prototype.fill;CanvasRenderingContext2D.prototype.fill=function(...a){if(this.canvas.matches("[data-starfort-canvas]"))window.__merchantColors.add(this.fillStyle);return window.__revisionFill.apply(this,a)}')
 for width,height in [(1366,900),(320,740)]:
  touch=width==320;c.b('set','viewport',width,height);scene('shop')
  c.wait('!!document.querySelector("[data-sf-shop]")')
  check('current rule visible '+str(width),w.data().get('revisionRules')==1)
  for fullscreen in [False,True]:
   if fullscreen:enter(touch)
   check('shop stays outside modal '+str(width)+str(fullscreen),not c.js('!!document.querySelector("[data-sf-shop]")?.closest("[aria-modal=true]")'))
   notice=c.js('(()=>{const e=document.querySelector("[data-sf-weather-notice]"),s=e?.querySelector("svg");return {label:e?.textContent,icon:s?.getBoundingClientRect().toJSON()}})()')
   check('42px weather icon and Chinese name '+str(width)+str(fullscreen),notice['label'] and '薄雾' in notice['label'] and notice['icon']['width']>=42 and notice['icon']['height']>=42,notice)
   geometry=c.js('(()=>{const a=document.querySelector("[data-sf-shop]").getBoundingClientRect(),b=document.querySelector(".sf-battlefield").getBoundingClientRect();return {shop:a.toJSON(),map:b.toJSON(),overlap:a.left<b.right&&a.right>b.left&&a.top<b.bottom&&a.bottom>b.top}})()')
   check('shop preserves separate battlefield '+str(width)+str(fullscreen),not geometry['overlap'] and geometry['map']['height']>=200,geometry)
   check('shop shows wave price and invulnerability '+str(width)+str(fullscreen),'200' in c.js('document.querySelector("[data-sf-shop-buy=damage]").textContent') and '10 秒' in c.js('document.querySelector("[data-sf-shop]").textContent'))
   check('no horizontal overflow '+str(width)+str(fullscreen),c.js('document.documentElement.scrollWidth<=innerWidth+1'))
   c.screenshot('shop-'+str(width)+'-'+str(fullscreen))
   c.js('document.querySelector(".sf-battlefield").scrollIntoView({block:"center",behavior:"instant"})');c.stable();selected=c.choose_visible_tower('arrow');c.stable();p=c.painted_tower_point(selected)
   c.native('tap',p['x'],p['y'],0,0) if touch else w.mouse_click(p['x'],p['y'])
   c.wait('!!document.querySelector("[data-sf-tower-close]")')
   check('painted tower works with shop open '+str(width)+str(fullscreen),c.js('!!document.querySelector("[data-sf-shop]")'))
   c.activate('[data-sf-tower-close]',touch)
   if fullscreen:c.leave()
  c.js('document.querySelector("[data-sf-shop-buy=shield]").scrollIntoView({block:"center",behavior:"instant"})');c.stable();c.activate('[data-sf-shop-buy=shield]',touch)
  c.wait('!!document.querySelector("[data-sf-invulnerability]")')
  check('pack purchased through UI '+str(width),w.data().get('invulnerabilityPacks')==1)
  c.activate('[data-sf-invulnerability]',touch);c.wait('document.querySelector("[data-sf-invulnerable]")!==null')
  check('pack used once through UI '+str(width),w.data().get('invulnerabilityPacks',0)==0 and w.data().get('invulnerableUntil',0)>0)
  scene('rewards');check('deferred rewards do not force modal '+str(width),not c.js('!!document.querySelector("[data-sf-pad-reward]")'))
  c.js('window.__rewardInput=[];for(const type of ["touchstart","touchend","pointerdown","pointerup","click"]){document.addEventListener(type,e=>window.__rewardInput.push({type,target:e.target.outerHTML?.slice(0,300),at:performance.now(),y:scrollY}),{capture:true,once:true})}')
  c.activate('[data-sf-rewards-open]',touch);w.artifact('reward-input-'+str(width)+'.json',c.js('({events:window.__rewardInput,dialog:document.querySelector("[role=dialog]")?.outerHTML,button:document.querySelector("[data-sf-rewards-open]")?.getBoundingClientRect().toJSON(),scrollY})'));c.wait('!!document.querySelector("[data-sf-pad-reward]")')
  c.screenshot('rewards-'+str(width))
  scene('merchant');c.wait('window.__merchantColors.has("#272735")&&window.__merchantColors.has("#2c2731")&&window.__merchantColors.has("#91849f")');check('merchant phone suit and hair drawn '+str(width),True);c.js('document.querySelector(".sf-battlefield").scrollIntoView({block:"center",behavior:"instant"})');c.stable()
  point=c.js('(()=>{const o='+json.dumps(w.data()['world']['objectives'][0])+',m=document.querySelector("[data-sf-minimap]"),p=m.createSVGPoint();p.x=o.x;p.y=o.y;const q=p.matrixTransform(m.getScreenCTM());return {x:q.x,y:q.y}})()')
  c.native('tap',point['x'],point['y'],0,0) if touch else w.mouse_click(point['x'],point['y'])
  c.stable();c.screenshot('merchant-'+str(width))
  for kind,color,lifetime in [('rain','#e5e8bc',2000),('meteor','#e4b5ff',3000)]:
   c.js('window.__revisionVisuals={}');scene(kind);c.choose_visible_tower('arrow' if kind=='rain' else 'arcane');c.stable()
   c.wait('!!window.__revisionVisuals['+json.dumps(color)+']')
   c.screenshot(kind+'-'+str(width))
   c.wait('performance.now()>window.__revisionVisuals['+json.dumps(color)+'].first+'+str(lifetime+300))
   w.artifact(kind+'-events-'+str(width)+'.json',w.data());measured=c.js('window.__revisionVisuals['+json.dumps(color)+']');span=measured['last']-measured['first']
   check('actual Canvas '+kind+' duration '+str(width),lifetime-220<=span<=lifetime+100,measured)

  for weather,label in [('calm','无特殊天气'),('frost','寒潮'),('starlight','星辉')]:
   scene(weather);notice=c.js('(()=>{const e=document.querySelector("[data-sf-weather-notice]"),s=e?.querySelector("svg");return {label:e?.textContent,icon:s?.getBoundingClientRect().toJSON()}})()')
   check('weather '+weather+' name and icon '+str(width),label in notice['label'] and notice['icon']['width']>=42,notice)
   c.screenshot('weather-'+weather+'-'+str(width))
 check('sources unchanged during acceptance',initial==sources());check('no browser errors',not c.b('errors')['errors']);passed=True
except Exception as error:
 import traceback;traceback.print_exc();w.artifact('failure.json',{'error':str(error)})
 try:c.screenshot('failure')
 except Exception:pass
finally:
 if w.OPENED:
  try:c.b('close')
  except Exception:pass
 if w.SERVER:(w.OUT/'stop').write_text('stop');w.SERVER.wait(timeout=20)
 w.artifact('result.json',{'passed':passed,'checks':checks,'sources':sources(),'scope':__doc__})
 for path in [w.OUT/'profile',w.OUT/'tmp',w.PROFILE]:
  if path.exists():shutil.rmtree(path)
 sys.exit(0 if passed else 1)
