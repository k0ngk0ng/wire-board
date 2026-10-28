#!/usr/bin/env python3
"""Pre-implementation failures: tower panel stays in corner or clips at 320px;
clicking enemy cannot inspect; inspection leaks HP before visual impact;
pan selects accidentally; panel blocks map gestures; selection outlives enemy.
Real HTTP fixture, production Canvas, native mouse / touch. Build first.
"""
import json,sys,shutil
from pathlib import Path
import accept_combat_controls_ui as c
w=c.w;w.OUT=c.ROOT/'.local/inspection/browser';w.OUT.mkdir(parents=True,exist_ok=True)
w.ENV.update(WIRE_BOARD_STARFORT_BROWSER_OUT=str(w.OUT),AGENT_BROWSER_PROFILE=str(w.OUT/'profile'))
passed=False
try:
 c.setup()
 for width,height in [(1366,900),(320,740)]:
  touch=width==320;c.b('set','viewport',str(width),str(height))
  w.scenario('ice-arrow');c.enter();selector=c.choose_visible_tower('arrow');c.activate(selector,touch)
  c.wait('!!document.querySelector("[data-sf-tower-tools]")')
  c.verify('anchored tower '+str(width),c.js('!!document.querySelector(".sf-world-viewport [data-sf-tower-tools]")'))
  c.verify('panel visible '+str(width),c.visibility('[data-sf-tower-tools]')['visible'])
  c.verify('branch usable '+str(width),c.visibility('[data-sf-upgrade=heart]')['visible'])
  c.screenshot('tower-'+str(width));c.activate('[data-sf-tower-close]',touch)
  # Probe records actual drawn enemy positions, not the newer server snapshot.
  point=c.js("window.__sfCombatProbe.export().frames.at(-1)?.draws.filter(p=>p.name.startsWith('enemy-')).map(p=>({x:p.screenX,y:p.screenY})).find(p=>p.x>30&&p.x<innerWidth-30&&p.y>70&&p.y<innerHeight-70)")
  assert point,'visible drawn enemy required'
  if touch:c.native('tap',point['x'],point['y'],0,0)
  else:w.mouse_click(point['x'],point['y'])
  c.wait('!!document.querySelector("[data-sf-enemy-info]")')
  c.verify('enemy visible '+str(width),c.visibility('[data-sf-enemy-info]')['visible'])
  c.verify('enemy details '+str(width),c.js('document.querySelector("[data-sf-enemy-info]").textContent.includes("生命")'))
  c.screenshot('enemy-'+str(width));c.activate('[data-sf-enemy-close]',touch)
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
  w.scenario('coop-objectives');c.enter();c.activate('[data-sf-open-info]',touch)
  c.js('document.querySelector(".sf-objectives").closest("details").open=true')
  objectives=w.data()['world']['objectives'];c.verify('three restored tasks '+str(width),{o['kind'] for o in objectives}=={'mine','beacon','convoy'})
  selector='[data-sf-objective="'+objectives[0]['id']+'"]'
  c.js('document.querySelector('+json.dumps(selector)+').scrollIntoView({block:"center"})');c.activate(selector,touch)
  c.verify('accept task '+str(width),w.data()['world']['objectives'][0]['status']=='active');c.screenshot('task-'+str(width))
  c.b('press','Escape')
 passed=True
finally:
 if w.OPENED:c.b('close')
 if w.SERVER:(w.OUT/'stop').write_text('stop');w.SERVER.wait(timeout=20)
 w.artifact('result.json',{'passed':passed,'checks':w.REPORT['checks'],'sourceSHA256':c.source_hashes(),'scope':'real local HTTP and Chromium mouse / 320px touch emulation'})
 for p in [w.OUT/'profile',w.OUT/'tmp']:
  if p.exists():shutil.rmtree(p)
 sys.exit(0 if passed else 1)
