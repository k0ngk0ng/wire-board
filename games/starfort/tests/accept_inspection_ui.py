#!/usr/bin/env python3
"""Pre-implementation failures: tower panel stays in corner or clips at 320px;
clicking enemy cannot inspect; inspection leaks HP before visual impact;
pan selects accidentally; panel blocks map gestures; selection outlives enemy.
Real HTTP fixture, production Canvas, native mouse / touch. Build first.
"""
import json,sys,shutil,subprocess
from pathlib import Path
import accept_combat_controls_ui as c
w=c.w;w.OUT=c.ROOT/'.local/inspection/browser';w.OUT.mkdir(parents=True,exist_ok=True)
w.ENV.update(WIRE_BOARD_STARFORT_BROWSER_OUT=str(w.OUT),AGENT_BROWSER_PROFILE=str(w.OUT/'profile'))
passed=False
try:
 c.setup()
 for width,height in [(1366,900),(320,740)]:
  touch=width==320;c.b('set','viewport',str(width),str(height))
  w.scenario('ice-arrow');c.enter();selector=c.choose_visible_tower('arrow')
  point=c.painted_tower_point(selector);tower_point=dict(point)
  def tap_tower():
   if touch:c.native('tap',tower_point['x'],tower_point['y'],0,0)
   else:w.mouse_click(tower_point['x'],tower_point['y'])
  tap_tower()
  c.wait('!!document.querySelector("[data-sf-tower-tools]")')
  c.verify('selected tower remains hittable '+str(width),c.js('document.querySelector('+json.dumps(selector)+').contains(document.elementFromPoint('+str(point['x'])+','+str(point['y'])+'))'))
  if touch:
   c.b('set','viewport','320','400');c.stable()
   c.wait('document.querySelector(".sf-map-popover").scrollHeight>document.querySelector(".sf-map-popover").clientHeight+2')
   box=c.rect('.sf-map-popover');before_camera=w.camera()
   w.artifact('phone-menu-scroll-before.json',c.js('(()=>{const e=document.querySelector(".sf-map-popover");return {scroll:e.scrollTop,height:e.clientHeight,content:e.scrollHeight}})()'))
   subprocess.run(['node','games/starfort/tests/popup_touch_scroll.mjs',c.b('get','cdp-url')['cdpUrl'],w.BASE,str(box['x']+box['width']/2),str(box['bottom']-40)],cwd=c.ROOT,env=w.ENV,check=True,timeout=20)
   c.wait('document.querySelector(".sf-map-popover").scrollTop>0')
   w.artifact('phone-menu-scroll-after.json',c.js('(()=>{const e=document.querySelector(".sf-map-popover");return {scroll:e.scrollTop,height:e.clientHeight,content:e.scrollHeight}})()'))
   c.verify('phone menu scroll does not pan map',w.camera()==before_camera)
   c.b('set','viewport','320','740');c.stable();tower_point=c.painted_tower_point(selector);point=dict(tower_point)
  tap_tower();c.wait('!document.querySelector("[data-sf-tower-tools]")')
  c.verify('second tower tap clears selection '+str(width),c.js('document.querySelector('+json.dumps(selector)+').getAttribute("aria-pressed")==="false"'))
  if not touch:
   w.mouse_click(point['x'],point['y']);w.mouse_click(point['x'],point['y']);c.wait('!document.querySelector("[data-sf-tower-tools]")')
   c.verify('rapid consecutive clicks leave menu closed',c.js('document.querySelector('+json.dumps(selector)+').getAttribute("aria-pressed")==="false"'))
  tap_tower()
  c.wait('!!document.querySelector("[data-sf-tower-tools]")')
  c.verify('anchored tower '+str(width),c.js('!!document.querySelector(".sf-world-camera [data-sf-tower-tools]")'))
  c.verify('panel visible '+str(width),c.visibility('[data-sf-tower-tools]')['visible'])
  c.verify('branch usable '+str(width),c.visibility('[data-sf-upgrade=heart]')['visible'])
  if not touch:
   icon=c.rect('[data-sf-skill]');c.b('mouse','move',icon['x']+icon['width']/2,icon['y']+icon['height']/2);c.wait('!!document.querySelector("[data-sf-skill-tooltip]")')
   frames=c.js('''new Promise(resolve=>{const rows=[];function sample(){const e=document.querySelector('[data-sf-skill]'),r=e.getBoundingClientRect();rows.push({open:!!document.querySelector('[data-sf-skill-tooltip]'),x:r.x,y:r.y});if(rows.length>=45)resolve(rows);else requestAnimationFrame(sample)}requestAnimationFrame(sample)})''')
   w.artifact('stationary-skill-hover.json',frames)
   c.verify('stationary skill hover does not move trigger or flicker',all(f['open'] for f in frames) and max(f['x'] for f in frames)-min(f['x'] for f in frames)<1 and max(f['y'] for f in frames)-min(f['y'] for f in frames)<1)
   c.b('mouse','move',1,1);c.wait('!document.querySelector("[data-sf-skill-tooltip]")')
  c.b('focus','[data-sf-open-tasks]');c.b('press','Enter');c.wait('!!document.querySelector(".sf-tasks-dialog")')
  c.verify('task modal stays above tower menu '+str(width),c.js('''(()=>{const m=document.querySelector(".sf-tasks-dialog"),a=m.getBoundingClientRect(),b=document.querySelector(".sf-map-popover").getBoundingClientRect();const left=Math.max(a.left,b.left),right=Math.min(a.right,b.right),top=Math.max(a.top,b.top),bottom=Math.min(a.bottom,b.bottom);return left>=right||top>=bottom||m.contains(document.elementFromPoint((left+right)/2,(top+bottom)/2))})()'''))
  c.b('click','.sf-tasks-dialog [aria-label="关闭"]')
  c.screenshot('tower-'+str(width));c.activate('[data-sf-tower-close]',touch)
  # Probe records actual drawn enemy positions, not the newer server snapshot.
  point=c.js("window.__sfCombatProbe.export().frames.at(-1)?.draws.filter(p=>p.name.startsWith('enemy-')).map(p=>({x:p.screenX,y:p.screenY})).find(p=>p.x>30&&p.x<innerWidth-30&&p.y>70&&p.y<innerHeight-70)")
  assert point,'visible drawn enemy required'
  if touch:c.native('tap',point['x'],point['y'],0,0)
  else:w.mouse_click(point['x'],point['y'])
  c.wait('!!document.querySelector("[data-sf-enemy-info]")')
  c.verify('enemy visible '+str(width),c.visibility('[data-sf-enemy-info]')['visible'])
  c.verify('enemy details '+str(width),c.js('document.querySelector("[data-sf-enemy-info]").textContent.includes("生命")'))
  c.screenshot('enemy-'+str(width))
  if touch:c.native('tap',point['x'],point['y'],0,0)
  else:w.mouse_click(point['x'],point['y'])
  c.wait('!document.querySelector("[data-sf-enemy-info]")')
  c.b('wait','1100');c.verify('second enemy tap stays closed across snapshots '+str(width),c.js('!document.querySelector("[data-sf-enemy-info]")'))
  if touch:c.native('tap',point['x'],point['y'],0,0)
  else:w.mouse_click(point['x'],point['y'])
  c.wait('!!document.querySelector("[data-sf-enemy-info]")');c.activate('[data-sf-enemy-close]',touch)
  c.leave()
 for width,height in [(1366,900),(320,740)]:
  touch=width==320;c.b('set','viewport',str(width),str(height))
  w.scenario('coop-growth');c.wait('!!document.querySelector("[data-sf-route-choice]")')
  original=w.data();choice=original['world']['choice']['options'][0]['id']
  selector='[data-sf-route-choice="'+choice+'"]'
  c.js('document.querySelector('+json.dumps(selector)+').scrollIntoView({block:"center"})');c.activate(selector,touch)
  c.wait('!!document.querySelector("[data-sf-boon-choice=power]")')
  c.js('document.querySelector("[data-sf-boon-choice=power]").scrollIntoView({block:"center"})');c.activate('[data-sf-boon-choice=power]',touch)
  c.wait('!document.querySelector("[data-sf-route-choice]")')
  current=w.data();c.verify('cooperative expansion '+str(width),current['world']['stage']==2 and w.tower_identity(current)==w.tower_identity(original))
  c.screenshot('expanded-'+str(width))
  w.scenario('coop-objectives');c.enter();c.activate('[data-sf-open-tasks]',touch)
  c.wait('!!document.querySelector(".sf-tasks-dialog")')
  objectives=w.data()['world']['objectives'];c.verify('three restored tasks '+str(width),{o['kind'] for o in objectives}=={'mine','beacon','convoy'})
  selector='[data-sf-objective="'+objectives[0]['id']+'"]'
  c.js('document.querySelector('+json.dumps(selector)+').scrollIntoView({block:"center"})');c.activate(selector,touch)
  c.verify('accept task '+str(width),w.data()['world']['objectives'][0]['status']=='active');c.screenshot('task-'+str(width))
  c.b('press','Escape')
 # Use a level-one menu for the cross-selection regression so the enemy is
 # genuinely exposed to native pointer input, rather than hidden by branch choices.
 c.b('set','viewport','1366','900');w.scenario('cadence');c.enter()
 state=w.data();pads={p['id']:p for p in state['map']['pads']}
 tower=max(state['towers'],key=lambda t:pads[t['pad']]['x'])
 selector=c.choose_visible_tower(tower['kind']);tower_point=c.painted_tower_point(selector)
 w.mouse_click(tower_point['x'],tower_point['y']);c.wait('!!document.querySelector("[data-sf-tower-tools]")');c.stable()
 enemy_point=c.js("window.__sfCombatProbe.export().frames.at(-1).draws.filter(p=>p.name.startsWith('enemy-')).map(p=>({x:p.screenX,y:p.screenY})).find(p=>document.elementFromPoint(p.x,p.y)?.matches('[data-starfort-canvas]'))")
 assert enemy_point,'enemy must be exposed for native switching'
 w.mouse_click(enemy_point['x'],enemy_point['y']);c.wait('!!document.querySelector("[data-sf-enemy-info]")')
 c.b('focus',selector);c.b('press','Enter');c.wait('!!document.querySelector("[data-sf-tower-tools]")')
 c.verify('switching enemy back to previous tower keeps its menu open',c.js('!document.querySelector("[data-sf-enemy-info]")&&document.querySelector('+json.dumps(selector)+').getAttribute("aria-pressed")==="true"'))
 c.leave()
 passed=True
except Exception:
 import traceback;traceback.print_exc()
 if w.OPENED:
  try:c.screenshot('failure')
  except Exception:pass
finally:
 if w.OPENED:c.b('close')
 if w.SERVER:(w.OUT/'stop').write_text('stop');w.SERVER.wait(timeout=20)
 w.artifact('result.json',{'passed':passed,'checks':w.REPORT['checks'],'sourceSHA256':c.source_hashes(),'scope':'real local HTTP and Chromium mouse / 320px touch emulation'})
 for p in [w.OUT/'profile',w.OUT/'tmp']:
  if p.exists():shutil.rmtree(p)
 sys.exit(0 if passed else 1)
