#!/usr/bin/env python3
"""LATEST.md: real HTTP room, Canvas purple flood ring and native mouse/touch.
Failure cases: wrong price/debit, rule-gated tools disappear, ring absent/expired,
full-screen geometry or zoomed tower hit-testing breaks at 320px.
"""
import hashlib,json,shutil,sys
import accept_combat_controls_ui as c
w=c.w
w.OUT=c.ROOT/'.local/starfort-latest/browser'
w.OUT.mkdir(parents=True,exist_ok=True)
(w.OUT/'tmp').mkdir(exist_ok=True)
w.ENV.update(WIRE_BOARD_STARFORT_BROWSER_OUT=str(w.OUT),AGENT_BROWSER_PROFILE=str(w.OUT/'profile'),GOTMPDIR=str(w.OUT/'tmp'),TMPDIR=str(w.OUT/'tmp'))
checks=[];passed=False
sources=lambda:{str(p.relative_to(c.ROOT)):hashlib.sha256(p.read_bytes()).hexdigest() for p in list((c.ROOT/'games/starfort/web').glob('*'))+list((c.ROOT/'games/starfort/server').glob('*.go'))+[c.ROOT/'games/starfort/tests/browser_fixture_test.go',__import__('pathlib').Path(__file__)] if p.is_file()}
def check(name,ok,evidence=None):
 checks.append({'name':name,'passed':bool(ok),'evidence':evidence});print(('PASS 'if ok else'FAIL ')+name,flush=True);assert ok,(name,evidence)
def scene(name):
 w.scenario('latest-'+name);c.wait('document.querySelector(".room-heading h1, .room-page h1")?.textContent.includes('+json.dumps('latest-'+name)+')');c.wait('document.querySelector(".sf-battlefield")?.dataset.assetsReady==="true"');c.js('document.querySelector(".sf-battlefield").scrollIntoView({block:"center",behavior:"instant"})');c.stable()
def enter(touch):
 if not c.js('!!document.querySelector("[data-fullscreen-enter]")'):c.b('click','.compact-room-tools')
 c.b('scroll','up',2000);c.stable();c.b('scrollintoview','[data-fullscreen-enter]');c.stable();c.activate('[data-fullscreen-enter]',touch);c.wait('!!document.querySelector("[data-room-fullscreen]")');c.stable()
try:
 c.setup();initial=sources()
 c.js('window.__purple=[];const stroke=CanvasRenderingContext2D.prototype.stroke;CanvasRenderingContext2D.prototype.stroke=function(...a){if(this.canvas.matches("[data-starfort-canvas]")&&this.strokeStyle==="#b45de8")window.__purple.push({at:performance.now(),alpha:this.globalAlpha});return stroke.apply(this,a)}')
 for width,height in [(1366,900),(320,740)]:
  touch=width==320;c.b('set','viewport',width,height);scene('shop');d=w.data()
  check('schema13 visible '+str(width),d.get('revisionRules')==2 and d['baseMaxHP']==32)
  for full in [False,True]:
   if full:enter(touch)
   check('700 visible '+str(width)+str(full),'700' in c.js('document.querySelector("[data-sf-shop-buy=shield]").textContent'))
   check('42px weather survives new rules '+str(width)+str(full),c.js('document.querySelector("[data-sf-weather-notice] svg").getBoundingClientRect().width')>=42)
   check('no horizontal overflow '+str(width)+str(full),c.js('document.documentElement.scrollWidth<=innerWidth+1'))
   c.screenshot('shop-'+str(width)+'-'+str(full))
   if full:c.leave()
  gold=d['players'][0]['gold'];c.js('document.querySelector("[data-sf-shop-buy=shield]").scrollIntoView({block:"center",behavior:"instant"})');c.stable();c.activate('[data-sf-shop-buy=shield]',touch);c.wait('!!document.querySelector("[data-sf-invulnerability]")');d=w.data();check('purchase debits700 '+str(width),d['players'][0]['gold']==gold-700 and d.get('invulnerabilityPacks')==1)
  c.activate('[data-sf-invulnerability]',touch);c.wait('!!document.querySelector("[data-sf-invulnerable]")');check('pack use '+str(width),w.data().get('invulnerabilityPacks',0)==0)
  for full in [False,True]:
   scene('flood')
   if full:enter(touch)
   c.js('window.__purple=[]');tower=c.choose_visible_tower('arrow');c.wait('window.__purple.length>1');c.screenshot('purple-'+str(width)+'-'+str(full))
   point=c.painted_tower_point(tower)
   c.native('tap',point['x'],point['y'],0,0) if touch else w.mouse_click(point['x'],point['y'])
   c.wait('!!document.querySelector("[data-sf-tower-close]")');check('purple tower remains clickable '+str(width)+str(full),True)
   c.activate('[data-sf-tower-close]',touch)
   c.wait('performance.now()-window.__purple[0].at>12000')
   v=c.js('({last:window.__purple.at(-1),now:performance.now(),first:window.__purple[0],count:window.__purple.length})')
   check('ring fades and expires '+str(width)+str(full),v['count']>2 and v['last']['alpha']<v['first']['alpha'] and v['now']-v['last']['at']>300,v)
   c.screenshot('recovered-'+str(width)+'-'+str(full))
   if full:c.leave()
 check('source-bound result',initial==sources());check('no browser errors',not c.b('errors')['errors']);passed=True
except Exception as e:
 import traceback;traceback.print_exc();w.artifact('failure.json',{'error':str(e)})
 try:c.screenshot('failure')
 except Exception:pass
finally:
 if w.OPENED:
  try:c.b('close')
  except Exception:pass
 if w.SERVER:(w.OUT/'stop').write_text('stop');w.SERVER.wait(timeout=20)
 w.artifact('result.json',{'passed':passed,'checks':checks,'sources':sources(),'scope':__doc__})
 for p in [w.OUT/'profile',w.OUT/'tmp',w.PROFILE]:
  if p.exists():shutil.rmtree(p)
 sys.exit(0 if passed else 1)
