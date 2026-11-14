#!/usr/bin/env python3
"""PUBLIC_EVENTS.md: native mouse/touch, real HTTP rooms and production Canvas.
Failures frozen before implementation: missing/road-obscuring notices, timer
restart, repeated boss modal, invisible debt, missing aura/stars/tower numbers.
"""
import json,os,shutil,sys,time
import accept_combat_controls_ui as c
w=c.w
w.OUT=c.ROOT/'.local/starfort-public/browser';w.OUT.mkdir(parents=True,exist_ok=True)
(w.OUT/'tmp').mkdir(exist_ok=True)
w.ENV.update(WIRE_BOARD_STARFORT_BROWSER_OUT=str(w.OUT),AGENT_BROWSER_PROFILE=str(w.OUT/'profile'),GOTMPDIR=str(w.OUT/'tmp'),TMPDIR=str(w.OUT/'tmp'))
checks=[];passed=False
def check(name,ok,evidence=None):
 checks.append({'name':name,'passed':bool(ok),'evidence':evidence});print(('PASS ' if ok else 'FAIL ')+name,flush=True);assert ok,(name,evidence)
def scene(name):
 w.scenario('public-'+name);c.wait('document.querySelector(".room-heading h1, .room-page h1")?.textContent.includes('+json.dumps('public-'+name)+')');c.wait('document.querySelector(".sf-battlefield")?.dataset.assetsReady==="true"');c.js('document.querySelector(".sf-battlefield").scrollIntoView({block:"center",behavior:"instant"})');c.stable()
def enter(touch):
 if not c.js('!!document.querySelector("[data-fullscreen-enter]")'):c.b('click','.compact-room-tools')
 c.b('scroll','up',2000);c.b('scrollintoview','[data-fullscreen-enter]');c.activate('[data-fullscreen-enter]',touch);c.wait('!!document.querySelector("[data-room-fullscreen]")');c.stable()
try:
 c.setup();sources=w.source_hashes()
 for width,height in [(1366,900),(320,740)]:
  touch=width==320;c.b('set','viewport',width,height)
  for kind in ['tax','grant']:
   scene(kind);c.wait('!!document.querySelector("[data-sf-public-event]")')
   v=c.js('(()=>{const e=document.querySelector("[data-sf-public-event]"),w=document.querySelector("[data-sf-weather-notice]"),a=e.getBoundingClientRect(),b=document.querySelector(".sf-battlefield").getBoundingClientRect();return {text:e.textContent,ratio:parseFloat(getComputedStyle(e).fontSize)/parseFloat(getComputedStyle(w).fontSize),outside:a.bottom<=b.top+1,width:a.width,viewport:innerWidth}})()')
   check('left event is large and outside road '+kind+str(width),v['ratio']>=3 and v['outside'] and v['width']<=width,v)
   check('event visible '+kind+str(width),c.visibility('[data-sf-public-event]')['hit'] and c.visibility('[data-sf-public-event]')['visible']);c.screenshot(kind+'-'+str(width));enter(touch);check('event in fullscreen '+kind+str(width),c.js('!!document.querySelector("[data-game-fullscreen-surface] [data-sf-public-event]")'));check('event visible in fullscreen '+kind+str(width),c.visibility('[data-sf-public-event]')['hit'] and c.visibility('[data-sf-public-event]')['visible']);c.screenshot(kind+'-fullscreen-'+str(width));c.leave()
   c.wait('!document.querySelector("[data-sf-public-event]")');check('notice expires after10 simulation seconds '+kind+str(width),w.data()['elapsedMS']>=11000)
  scene('boss');c.wait('!!document.querySelector("[data-sf-boss-dialog]")');check('boss reward announced '+str(width),'100' in c.js('document.querySelector("[data-sf-boss-dialog]").textContent'));c.screenshot('boss-'+str(width))
  c.activate('[data-sf-boss-dismiss]',touch);c.wait('!document.querySelector("[data-sf-boss-dialog]")');c.b('reload');c.wait('document.querySelector(".sf-battlefield")?.dataset.assetsReady==="true"');check('boss remains dismissed on reload '+str(width),not c.js('!!document.querySelector("[data-sf-boss-dialog]")'))
  c.activate('[data-sf-boss-open]',touch);c.wait('!!document.querySelector("[data-sf-boss-dialog]")');c.activate('[data-sf-boss-dismiss]',touch);enter(touch);c.activate('[data-sf-boss-open]',touch);check('boss overlay inside fullscreen '+str(width),c.js('!!document.querySelector("[data-game-fullscreen-surface] [data-sf-boss-dialog]")'));c.screenshot('boss-fullscreen-'+str(width));c.activate('[data-sf-boss-dismiss]',touch);c.leave()
  scene('debt');check('negative gold visible '+str(width),c.visibility('[data-sf-own-gold]')['visible'] and '-200' in c.js('document.querySelector("[data-sf-own-gold]").textContent'));c.screenshot('debt-'+str(width))
  scene('starlight');c.js('window.__aura=0;window.__labels=[];const s=CanvasRenderingContext2D.prototype.stroke;CanvasRenderingContext2D.prototype.stroke=function(...a){if(this.strokeStyle==="#72eced")window.__aura++;return s.apply(this,a)};const f=CanvasRenderingContext2D.prototype.fillText;CanvasRenderingContext2D.prototype.fillText=function(t,...a){if(this.canvas.matches("[data-starfort-canvas]"))window.__labels.push(t);return f.call(this,t,...a)}')
  c.choose_visible_tower('arrow');c.wait('window.__aura>0');check('aura and own tower number drawn '+str(width),c.js(r'window.__labels.some(t=>/^#\d+$/.test(t))'));c.screenshot('aura-'+str(width));c.choose_visible_tower('arcane');c.wait('window.__labels.includes("✦ ✦ ✦")');check('three mage stars '+str(width),True);c.screenshot('mage-stars-'+str(width))
  scene('earthquake');c.js('window.__sleep=[];const f=CanvasRenderingContext2D.prototype.fillText;CanvasRenderingContext2D.prototype.fillText=function(t,x,y,...a){if(t==="Zzz...")window.__sleep.push({x,y});return f.call(this,t,x,y,...a)}');c.choose_visible_tower('arrow');c.wait('window.__sleep.length>12 && Math.max(...window.__sleep.map(p=>p.y))-Math.min(...window.__sleep.map(p=>p.y))>.2');check('disabled tower animates '+str(width),True);c.screenshot('earthquake-'+str(width))
  check('no overflow '+str(width),c.js('document.documentElement.scrollWidth<=innerWidth+1'))
 check('sources unchanged',sources==w.source_hashes());check('no browser errors',not c.b('errors')['errors']);passed=True
except Exception as e:
 import traceback;traceback.print_exc();w.artifact('failure.json',{'error':str(e)})
 try:c.screenshot('failure')
 except Exception:pass
finally:
 if w.OPENED:
  try:c.b('close')
  except Exception:pass
 if w.SERVER:(w.OUT/'stop').write_text('stop');w.SERVER.wait(timeout=20)
 w.artifact('result.json',{'passed':passed,'checks':checks,'sources':w.source_hashes(),'scope':__doc__})
 for p in [w.OUT/'profile',w.OUT/'tmp',w.PROFILE]:
  if p.exists():shutil.rmtree(p)
 sys.exit(0 if passed else 1)
