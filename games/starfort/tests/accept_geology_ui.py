#!/usr/bin/env python3
"""GEOLOGY.md: real HTTP room, production Canvas, native PC/320px touch/fullscreen.
Milestone fixtures are paused to inspect geometry; rule and natural runs are separate.
"""
import json
import shutil
import sys
import accept_combat_controls_ui as c
w=c.w
w.OUT=c.ROOT/'.local/starfort-geology/browser'
w.OUT.mkdir(parents=True,exist_ok=True)
(w.OUT/'tmp').mkdir(exist_ok=True)
w.ENV.update(WIRE_BOARD_STARFORT_BROWSER_OUT=str(w.OUT),AGENT_BROWSER_PROFILE=str(w.OUT/'profile'),GOTMPDIR=str(w.OUT/'tmp'),TMPDIR=str(w.OUT/'tmp'))
checks=[]
def check(name,ok,evidence=None):
 checks.append({'name':name,'passed':bool(ok),'evidence':evidence});print(('PASS ' if ok else 'FAIL ')+name,flush=True);assert ok,(name,evidence)
def scene(kind):
 name='weather-geology-'+kind;w.scenario(name)
 c.wait('document.querySelector(".room-heading h1, .room-page h1")?.textContent.includes('+json.dumps(name)+')')
 c.wait('document.querySelector(".sf-battlefield")?.dataset.assetsReady==="true"');c.stable()
def enter(touch):
 if not c.js('!!document.querySelector("[data-fullscreen-enter]")'):c.b('click','.compact-room-tools')
 c.js('document.querySelector("[data-fullscreen-enter]").scrollIntoView({block:"center",behavior:"instant"})');c.stable();c.activate('[data-fullscreen-enter]',touch);c.wait('!!document.querySelector("[data-room-fullscreen]")');c.stable()
passed=False
try:
 c.setup();sources=c.source_hashes()
 c.js('window.__geoArc=CanvasRenderingContext2D.prototype.arc;CanvasRenderingContext2D.prototype.arc=function(x,y,r,...a){this.__geoPoint={x,y,r};return window.__geoArc.call(this,x,y,r,...a)};window.__geoDraws=[];window.__geoFill=CanvasRenderingContext2D.prototype.fill;CanvasRenderingContext2D.prototype.fill=function(...a){if(this.canvas.hasAttribute("data-starfort-canvas")){window.__geoDraws.push({color:this.fillStyle,point:this.__geoPoint,at:performance.now()});if(window.__geoDraws.length>200)window.__geoDraws.shift()}return window.__geoFill.apply(this,a)};window.__geoZZZ=[];window.__geoText=CanvasRenderingContext2D.prototype.fillText;CanvasRenderingContext2D.prototype.fillText=function(t,x,y,...a){if(t==="ZZZ"&&this.canvas.hasAttribute("data-starfort-canvas"))window.__geoZZZ.push({x,y});return window.__geoText.call(this,t,x,y,...a)}')
 for width,height in [(1366,900),(320,740)]:
  touch=width==320;c.b('set','viewport',width,height)
  for kind,label,color in [('earthquake','地震','#79554888'),('flood','洪水','#2584b877'),('typhoon','台风','#617d9477')]:
   scene(kind);c.js('document.querySelector(".sf-battlefield").scrollIntoView({block:"start",behavior:"instant"})');c.choose_visible_tower('arrow');c.stable();check('public geology '+kind+str(width),w.data().get('geologyRules')==1)
   for fs in [False,True]:
    if fs:enter(touch)
    c.js('window.__geoDraws=[];window.__geoZZZ=[]')
    c.wait('(()=>{const ctx=document.createElement("canvas").getContext("2d");ctx.fillStyle='+json.dumps(color)+';return window.__geoDraws.some(d=>d.color===ctx.fillStyle)})()')
    check('actual Canvas draws '+kind+str(width)+str(fs),bool(c.js('window.__geoDraws.length')))
    notice=c.js('document.querySelector("[data-sf-geology]")?.textContent')
    check('visible geology status '+kind+str(width)+str(fs),notice and label in notice,notice)
    check('status fits '+kind+str(width)+str(fs),c.visibility('[data-sf-geology]')['visible'] and c.visibility('[data-sf-geology]')['inside'])
    contrast=c.js('(()=>{const s=getComputedStyle(document.querySelector("[data-sf-geology]")),lum=v=>{const c=v.match(/[0-9.]+/g).slice(0,3).map(Number).map(v=>{v/=255;return v<=.04045?v/12.92:((v+.055)/1.055)**2.4});return c[0]*.2126+c[1]*.7152+c[2]*.0722},a=lum(s.color),b=lum(s.backgroundColor);return (Math.max(a,b)+.05)/(Math.min(a,b)+.05)})()')
    check('status contrast '+str(width)+str(fs),contrast>=4.5,contrast)
    check('no horizontal overflow '+str(width),c.js('document.documentElement.scrollWidth<=innerWidth+1'))
    if kind=='earthquake':c.wait('window.__geoZZZ.length>0');check('tower ZZZ drawn '+str(width)+str(fs),bool(c.js('window.__geoZZZ.length')))
    c.screenshot(kind+'-'+str(width)+'-'+str(fs))
    c.b('set','media','light','reduced-motion');c.js('window.__geoDraws=[]');c.wait('window.__geoDraws.length>0');check('reduced motion retains '+kind+str(width),True);c.b('set','media','light')
    if fs:c.leave()
   # Actual painted tower remains selectable through the effect.
   c.js('document.querySelector(".sf-battlefield").scrollIntoView({block:"start",behavior:"instant"})');c.stable();sel=c.choose_visible_tower('arrow');c.wait('document.querySelector(".sf-battlefield").dataset.overview==="false"');c.stable();p=c.painted_tower_point(sel)
   if touch:c.native('tap',p['x'],p['y'],0,0)
   else:w.mouse_click(p['x'],p['y'])
   c.wait('!!document.querySelector("[data-sf-tower-close]")');check('effects preserve tower input '+kind+str(width),True)
 c.b('set','viewport',1366,900);scene('typhoon-live')
 c.js('window.__geoDraws=[]')
 c.wait('(()=>{const color=document.createElement("canvas").getContext("2d");color.fillStyle="#617d9477";const p=window.__geoDraws.filter(d=>d.color===color.fillStyle&&d.point).map(d=>d.point.x.toFixed(2)+","+d.point.y.toFixed(2));return new Set(p).size>12})()')
 check('live typhoon moves smoothly between server snapshots',True)
 w.artifact('live-wind-draws.json',c.js('window.__geoDraws'))
 c.source_hashes();check('sources unchanged' ,sources==c.source_hashes());check('no browser errors',not c.b('errors')['errors']);passed=True
except Exception as error:
 import traceback
 traceback.print_exc()
 try:w.artifact("debug.json",c.js('({draws:window.__geoDraws,zzz:window.__geoZZZ,fill:CanvasRenderingContext2D.prototype.fill.toString(),status:document.querySelector("[data-sf-geology]")?.outerHTML})'))
 except Exception:pass
 w.artifact("failure.json",{"error":str(error)})
 try:c.screenshot("failure")
 except Exception:pass
finally:
 if w.OPENED:
  try:c.b('close')
  except Exception:pass
 if w.SERVER:(w.OUT/'stop').write_text('stop');w.SERVER.wait(timeout=20)
 w.artifact('result.json',{'passed':passed,'checks':checks,'sources':c.source_hashes(),'scope':__doc__})
 for p in [w.OUT/'profile',w.OUT/'tmp',w.PROFILE]:
  if p.exists():shutil.rmtree(p)
 sys.exit(0 if passed else 1)
