#!/usr/bin/env python3
"""OVERLAY_PROMPTS.md: real HTTP, native input and frame-by-frame map geometry.
Uses existing production-module milestone scenes; no mocked responses or new
server fixtures. Natural starts exercise actual task generation and acceptance.
"""
import json,shutil,sys,time
import accept_combat_controls_ui as c
w=c.w
w.OUT=c.ROOT/'.local/starfort-overlay/browser';w.OUT.mkdir(parents=True,exist_ok=True)
(w.OUT/'tmp').mkdir(exist_ok=True)
w.ENV.update(WIRE_BOARD_STARFORT_BROWSER_OUT=str(w.OUT),AGENT_BROWSER_PROFILE=str(w.OUT/'profile'),TMPDIR=str(w.OUT/'tmp'),GOTMPDIR=str(w.OUT/'tmp'))
checks=[];passed=False
def check(name,ok,evidence=None):
 checks.append({'name':name,'passed':bool(ok),'evidence':evidence});print(('PASS ' if ok else 'FAIL ')+name,flush=True);assert ok,(name,evidence)
def scene(name):
 w.scenario(name);c.wait('document.querySelector(".room-heading h1, .room-page h1")?.textContent.includes('+json.dumps(name)+')');c.wait('document.querySelector(".sf-battlefield")?.dataset.assetsReady==="true"');c.wait('!document.querySelector("[data-game-loading]")');c.js('document.querySelector(".starfort-board").scrollIntoView({block:"center",behavior:"instant"})')
 settled=c.js('new Promise(resolve=>{let last="",stable=0,n=0;function f(){const r=document.querySelector(".sf-battlefield").getBoundingClientRect(),v=JSON.stringify([r.x,r.y,r.width,r.height]);stable=last===v?stable+1:0;last=v;if(stable>=20||++n>=180)resolve(stable>=20);else requestAnimationFrame(f)}f()})');check('entry scrolling settles '+name,settled)
def enter(touch):
 if not c.js('!!document.querySelector("[data-fullscreen-enter]")'):c.b('click','.compact-room-tools')
 c.b('scroll','up',2000);c.b('scrollintoview','[data-fullscreen-enter]')
 settled=c.js('new Promise(resolve=>{let last="",stable=0,n=0;function f(){const e=document.querySelector("[data-fullscreen-enter]"),r=e.getBoundingClientRect(),v=JSON.stringify([r.x,r.y,r.width,r.height]);if(r.y<0||r.bottom>innerHeight)e.scrollIntoView({block:"center",behavior:"instant"});stable=r.y>=0&&r.bottom<=innerHeight&&last===v?stable+1:0;last=v;if(stable>=20||++n>=180)resolve(stable>=20);else requestAnimationFrame(f)}f()})')
 check('fullscreen entry settles',settled);c.activate('[data-fullscreen-enter]',touch);c.wait('!!document.querySelector("[data-room-fullscreen]")');c.stable()
def activate(selector,touch=False):
 if c.js('!!document.querySelector('+json.dumps(selector)+')?.hasAttribute("data-sf-overflow") && document.querySelector(".sf-topbar")?.dataset.actionsOpen!=="true"'):
  c.activate('[data-sf-toolbar-more]',touch)
 c.activate(selector,touch)
def geometry():
 return c.js('(()=>{const b=document.querySelector(".sf-battlefield").getBoundingClientRect(),r=document.querySelector(".starfort-board").getBoundingClientRect(),h=document.querySelector(".sf-topbar").getBoundingClientRect();return {battle:b.toJSON(),root:r.toJSON(),hud:h.toJSON(),floating:Math.abs(b.top-r.top)<2&&Math.abs(b.height-r.height)<2&&h.top>=b.top&&h.bottom<=b.bottom&&h.left>=b.left&&h.right<=b.right+2}})()')
def notice_visible(selector):
 return c.js('(()=>{const e=document.querySelector('+json.dumps(selector)+'),b=document.querySelector(".sf-battlefield"),r=e?.getBoundingClientRect(),a=b.getBoundingClientRect(),h=r&&document.elementFromPoint(r.x+r.width/2,r.y+r.height/2);return {exists:!!e,inside:!!e&&b.contains(e)&&r.left>=a.left&&r.right<=a.right+1&&r.top>=a.top&&r.bottom<=a.bottom+1,left:!!e&&r.left-a.left<=16,clickThrough:!!e&&!e.contains(h),rect:r?.toJSON()}})()')
def start_frames():
 c.js('(()=>{window.__sfFrames=[];window.__sfCapture=true;function f(){if(!window.__sfCapture)return;const b=document.querySelector(".sf-battlefield"),r=b.getBoundingClientRect(),c=b.querySelector("canvas");window.__sfFrames.push({at:performance.now(),x:r.x,y:r.y,w:r.width,h:r.height,zoom:+c.dataset.zoom,cx:+c.dataset.cameraX,cy:+c.dataset.cameraY,event:!!b.querySelector("[data-sf-public-event]"),geo:!!b.querySelector("[data-sf-geology]")});requestAnimationFrame(f)}f()})()')
def stop_frames(name):
 frames=c.js('(()=>{window.__sfCapture=false;return window.__sfFrames})()');w.artifact(name+'.frames.json',frames)
 spans={key:max(f[key]for f in frames)-min(f[key]for f in frames)for key in ['x','y','w','h','zoom','cx','cy']}
 check('map geometry remains stable '+name,len(frames)>30 and all(v<.05 for v in spans.values()),spans)
 return frames
try:
 c.setup();sources=w.source_hashes();c.js('(()=>{const probe=document.createElement("canvas").getContext("2d");probe.fillStyle="#617d9477";const color=probe.fillStyle,fill=CanvasRenderingContext2D.prototype.fill;window.__sfGeoPaint=[];CanvasRenderingContext2D.prototype.fill=function(...a){if(this.canvas.matches("[data-starfort-canvas]")&&this.fillStyle===color)window.__sfGeoPaint.push(performance.now());return fill.apply(this,a)}})()')
 for width,height in [(1366,900),(320,740)]:
  touch=width==320;c.b('set','viewport',width,height)
  for full,kind in [(False,'tax'),(True,'grant')]:
   scene('public-'+kind)
   if full:enter(touch)
   g=geometry();check('HUD floats inside battlefield '+str(width)+str(full),g['floating'],g);check('compact toolbar is one row '+str(width)+str(full),width>600 or g['hud']['height']<=40,g['hud'])
   v=notice_visible('[data-sf-public-event]');check('public notice inside map-left and input transparent '+str(width)+str(full),v['inside'] and v['left'] and v['clickThrough'],v)
   start_frames();c.screenshot('notice-'+str(width)+'-'+str(full));c.wait('!document.querySelector("[data-sf-public-event]")');c.wait('window.__sfFrames.length>60')
   frames=stop_frames('notice-'+str(width)+'-'+str(full));check('expiration observed '+str(width)+str(full),any(f['event']for f in frames) and not frames[-1]['event'])
   c.screenshot('expired-'+str(width)+'-'+str(full))
   if full:c.leave()
  scene('public-boss');c.wait('!!document.querySelector("[data-sf-boss-dialog]")');d=w.data();routes=[r for r in d['map']['routes']if r['active']];route=routes[(d['waveInfo']['total']-1)%len(routes)]
  check('boss dialog names actual spawn route '+str(width),route['label'] in c.js('document.querySelector("[data-sf-boss-route]").textContent'))
  c.screenshot('boss-route-'+str(width));c.b('press','Escape');c.wait('!document.querySelector("[data-sf-boss-dialog]")');c.b('reload');c.wait('document.querySelector(".sf-battlefield")?.dataset.assetsReady==="true"');check('dismissal survives refresh '+str(width),not c.js('!!document.querySelector("[data-sf-boss-dialog]")'))
  enter(touch);activate('[data-sf-boss-open]',touch);c.wait('!!document.querySelector("[data-sf-boss-dialog]")');check('boss route remains in fullscreen '+str(width),c.js('!!document.querySelector("[data-game-fullscreen-surface] [data-sf-boss-route]")'));c.activate('[data-sf-boss-dismiss]',touch);c.leave()
  for attempt in range(10):
   scene('natural');d=w.data();offered=next((o for o in d.get('world',{}).get('objectives',[])if o['status']=='available'),None)
   if offered:break
  check('real natural task generated '+str(width),bool(offered));c.wait('!!document.querySelector(".sf-tasks-dialog [data-sf-objective]")')
  check('task is asked before the wave '+str(width),w.data()['phase']=='prepare');c.screenshot('task-'+str(width))
  c.js('document.querySelector(".sf-tasks-dialog [data-sf-objective]").scrollIntoView({block:"center",behavior:"instant"})');c.stable();c.activate('.sf-tasks-dialog [data-sf-objective]',touch);c.wait('!document.querySelector(".sf-tasks-dialog")');check('native task acceptance '+str(width),any(o['id']==offered['id'] and o['status']=='active' for o in w.data()['world']['objectives']))
  scene('weather-geology-typhoon-live');c.wait('!!document.querySelector("[data-sf-geology]")');check('disaster text uses normal notification size '+str(width),c.js('parseFloat(getComputedStyle(document.querySelector("[data-sf-geology]")).fontSize)')<=18);c.js('(()=>{const probe=document.createElement("canvas").getContext("2d");probe.fillStyle="#617d9477";const color=probe.fillStyle;window.__sfGeoOriginalFill??=CanvasRenderingContext2D.prototype.fill;const fill=window.__sfGeoOriginalFill;window.__sfGeoPaint=[];CanvasRenderingContext2D.prototype.fill=function(...a){if(this.canvas.matches("[data-starfort-canvas]")&&this.fillStyle===color)window.__sfGeoPaint.push(performance.now());return fill.apply(this,a)}})()');v=notice_visible('[data-sf-geology]');check('disaster is map-left '+str(width),v['inside'] and v['left'],v);start_frames();c.screenshot('disaster-'+str(width));c.wait('!document.querySelector("[data-sf-geology]")');c.wait('window.__sfFrames.length>60');frames=stop_frames('disaster-'+str(width));check('disaster text ends while wave continues '+str(width),w.data()['phase']=='battle' and any(f['geo']for f in frames) and not frames[-1]['geo']);last=c.js('window.__sfGeoPaint.at(-1)');ended=next(f['at']for f in frames if not f['geo']);check('text and actual effect end together '+str(width),last is not None and last<=ended+34,{'lastPaint':last,'textEnded':ended});c.screenshot('disaster-ended-'+str(width))
  scene('weather-toolbar');g=geometry();check('all-operation toolbar stays one row '+str(width),width>600 or g['hud']['height']<=40,g);
  if touch:
   c.activate('[data-sf-toolbar-more]',True);c.wait('document.querySelector(".sf-topbar")?.dataset.actionsOpen==="true"');c.screenshot('more-'+str(width));activate('[data-sf-open-tasks]',True);c.wait('!!document.querySelector(".sf-tasks-dialog")');c.activate('[role=dialog] [aria-label="关闭"]',True);check('overflow operation works '+str(width),True)
  check('no horizontal overflow '+str(width),c.js('document.documentElement.scrollWidth<=innerWidth+1'))
 check('source-bound browser result',sources==w.source_hashes());check('no browser errors',not c.b('errors')['errors']);passed=True
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
