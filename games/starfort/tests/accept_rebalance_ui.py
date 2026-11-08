#!/usr/bin/env python3
"""Failure cases specified before UI changes in REBALANCE.md: per-wave prompt
missing/reopens; accept/skip inaccessible at 320px or fullscreen; pad claim lacks
native mouse/touch selection; new branch text disagrees with saved effects.
Real local HTTP room and production Canvas; labelled reward boundary fixture.
"""
import json,sys,time,shutil
from pathlib import Path
import accept_combat_controls_ui as c
w=c.w
w.OUT=c.ROOT/'.local/starfort-rebalance/browser'
w.OUT.mkdir(parents=True,exist_ok=True)
w.ENV.update(WIRE_BOARD_STARFORT_BROWSER_OUT=str(w.OUT),AGENT_BROWSER_PROFILE=str(w.OUT/'profile'))
checks=[];passed=False
def mark(name,value):
 assert value,name
 checks.append(name);print('PASS',name,flush=True)
try:
 c.setup();source_start={**c.source_hashes(),**c.hashes([Path(__file__)])}
 for width,height in [(1366,900),(320,740)]:
  touch=width==320;c.b('set','viewport',str(width),str(height))
  w.scenario('rebalance-tasks')
  w.artifact('initial-'+str(width)+'.json',{'state':w.data(),'text':c.js('document.body.innerText')})
  c.wait('!!document.querySelector("[data-sf-task-offer]")')
  mark('automatic task prompt '+str(width),True)
  c.activate('.sf-tasks-dialog [aria-label="关闭"]',touch)
  c.enter()
  c.activate('[data-sf-open-tasks]',touch)
  c.wait('!!document.querySelector("[data-sf-task-offer]")')
  mark('task offer visible '+str(width),c.visibility('[data-sf-task-offer]')['visible'])
  mark('task risk shown '+str(width),'×1.2' in c.js('document.querySelector("[data-sf-task-offer]").textContent'))
  c.screenshot('offer-'+str(width))
  c.activate('[data-sf-skip-task]',touch)
  c.wait('!document.querySelector("[data-sf-task-offer]")')
  c.b('reload');c.wait('document.querySelector(".sf-battlefield")?.dataset.assetsReady==="true"')
  mark('skipped offer stays closed '+str(width),not c.js('!!document.querySelector("[data-sf-task-offer]")'))
  w.scenario('rebalance-tasks')
  c.wait('!!document.querySelector("[data-sf-task-offer]")')
  initial=w.data();offer=initial['world']['objectives'][0]
  c.activate('[data-sf-objective]',touch)
  c.wait('!document.querySelector(".sf-tasks-dialog")')
  paid=w.data()
  mark('native task acceptance '+str(width),paid['world']['objectives'][0]['status']=='active' and paid['players'][0]['gold']==initial['players'][0]['gold']-offer['cost'])
  w.scenario('rebalance-reward')
  c.wait('!!document.querySelector("[data-sf-pad-reward]")')
  c.activate('[data-sf-choose-reward]',touch)
  c.enter()
  c.wait('document.querySelector("[data-sf-placement]")?.dataset.sfPlacement==="add"')
  before=len(w.data()['map']['pads'])
  # Select a legal visible position from public map data and rendered camera.
  state=w.data();m=state['map']
  probe=c.js("(()=>{const e=document.querySelector('[data-starfort-canvas]');return {rect:e.getBoundingClientRect().toJSON(),x:+e.dataset.cameraX,y:+e.dataset.cameraY,zoom:+e.dataset.zoom}})()")
  r=probe['rect'];candidates=[]
  for sx in range(32,int(r['width'])-24,24):
   for sy in range(70,int(r['height'])-70,24):
    x=(sx-r['width']/2)/probe['zoom']+probe['x'];y=(sy-r['height']/2)/probe['zoom']+probe['y']
    if x<28 or y<28 or x>m['width']-28 or y>m['height']-28:continue
    if ((x-m['base']['x'])**2+(y-m['base']['y'])**2)**.5<65:continue
    if any(((x-p['x'])**2+(y-p['y'])**2)**.5<55 for p in m['pads']):continue
    candidates.append({'x':r['x']+sx,'y':r['y']+sy,'worldX':x,'worldY':y})
  point=c.js("("+json.dumps(candidates)+").find(p=>document.elementFromPoint(p.x,p.y)?.matches('[data-starfort-canvas]'))")
  mark('legal visible placement '+str(width),point is not None)
  w.artifact('placement-'+str(width)+'.json',point)
  c.native('tap',point['x'],point['y'],0,0) if touch else w.mouse_click(point['x'],point['y'])
  deadline=time.monotonic()+8
  while time.monotonic()<deadline and len(w.data()['map']['pads'])==before:time.sleep(.1)
  mark('native map reward selection '+str(width),len(w.data()['map']['pads'])==before+1)
  c.screenshot('reward-'+str(width))
  c.leave()
  w.scenario('rebalance-highground')
  c.wait('!!document.querySelector("[data-sf-pad-reward]")')
  c.activate('[data-sf-choose-reward]',touch);c.enter()
  state=w.data();normal=[p['id'] for p in state['map']['pads'] if p['terrain']=='plain']
  chosen=c.js('''(()=>{const ids='''+json.dumps(normal)+''';return [...document.querySelectorAll('[data-sf-pad]')].find(e=>{const r=e.getBoundingClientRect();return ids.includes(+e.dataset.sfPad)&&r.width>0&&r.x>=0&&r.y>=0&&r.right<=innerWidth&&r.bottom<=innerHeight&&e.contains(document.elementFromPoint(r.x+r.width/2,r.y+r.height/2))})?.dataset.sfPad})()''')
  mark('normal pad reachable '+str(width),chosen is not None)
  c.activate('[data-sf-pad="'+str(chosen)+'"]',touch)
  c.wait('!document.querySelector("[data-sf-placement]")')
  mark('native highground selection '+str(width),next(p for p in w.data()['map']['pads'] if p['id']==int(chosen))['terrain']=='highground')
  c.screenshot('highground-'+str(width));c.leave()
  for kind,branch,skill in [('frost','freeze','freeze'),('arcane','magic','meteor')]:
   w.scenario('rebalance-combat-'+kind);c.enter()
   mark('new-rule combat fixture '+kind+' '+str(width),w.data()['balanceRules']==1)
   c.js("(()=>{window.__rebalanceFreeze={first:0,last:0};const p=CanvasRenderingContext2D.prototype,stroke=p.stroke;p.stroke=function(...args){if(this.canvas.matches('[data-starfort-canvas]')&&this.strokeStyle==='#e2faff'){const n=performance.now();if(!window.__rebalanceFreeze.first)window.__rebalanceFreeze.first=n;window.__rebalanceFreeze.last=n;}return stroke.apply(this,args)};return true})()")
   selector=c.choose_visible_tower(kind);c.activate(selector,touch)
   c.activate('[data-sf-upgrade='+branch+']',touch)
   c.wait('document.querySelector("[data-sf-skill-status]")?.textContent.includes("%")')
   c.activate('[data-sf-skill]',touch)
   c.wait('!!document.querySelector("[data-sf-skill-tooltip]")')
   tooltip=c.js('document.querySelector("[data-sf-skill-tooltip]").textContent')
   mark('new skill description '+kind+' '+str(width),('10 秒' in tooltip and '恢复' in tooltip) if kind=='frost' else '沉默' in tooltip)
   c.screenshot('skill-'+kind+'-'+str(width))
   deadline=time.monotonic()+35;events=[]
   while time.monotonic()<deadline:
    state=w.data();events=[e for e in state['events'] if e.get('skill')==skill]
    if events:break
    time.sleep(.2)
   mark('new authoritative skill '+skill+' '+str(width),bool(events))
   if kind=='frost':
    mark('saved ten-second recovery '+str(width),any(e.get('recoveryUntil',0)-e.get('frozenUntil',0)==50 for e in state['enemies']))
    c.wait('window.__rebalanceFreeze.first>0&&performance.now()-window.__rebalanceFreeze.last>300')
    timing=c.js('({...window.__rebalanceFreeze,duration:window.__rebalanceFreeze.last-window.__rebalanceFreeze.first})')
    mark('two-second visible freeze '+str(width),1800<=timing['duration']<=2400)
    w.artifact('freeze-duration-'+str(width)+'.json',timing)
   w.artifact('effects-'+kind+'-'+str(width)+'.json',{'events':events,'enemies':state['enemies']})
   c.b('reload');c.wait('document.querySelector(".sf-battlefield")?.dataset.assetsReady==="true"')
   mark('new branch survives reload '+kind+' '+str(width),any(t['kind']==kind and t.get('branch')==branch for t in w.data()['towers']))


 mark('sources unchanged during browser acceptance',source_start=={**c.source_hashes(),**c.hashes([Path(__file__)])})
 mark('no browser errors',not c.b('errors')['errors']);passed=True
except Exception:
 import traceback;traceback.print_exc()
 if w.OPENED:
  try:c.screenshot('failure');w.artifact('failure-state.json',{'state':w.data(),'text':c.js('document.body.innerText')})
  except Exception:pass
finally:
 if w.OPENED:
  try:c.b('close')
  except Exception:pass
 if w.SERVER:(w.OUT/'stop').write_text('stop');w.SERVER.wait(timeout=20)
 w.artifact('result.json',{'passed':passed,'checks':checks,'sourceSHA256':{**c.source_hashes(),**c.hashes([Path(__file__)])},'buildSHA256':c.hashes([c.ROOT/'web/dist/index.html',c.ROOT/'web/dist/game-load-manifest.json']),'scope':'Production UI, real HTTP; mouse and 320px touch emulation; reward setup is a labelled boundary fixture'})
 for p in [w.OUT/'profile',w.OUT/'tmp',w.PROFILE]:
  if p.exists():shutil.rmtree(p)
 sys.exit(0 if passed else 1)
