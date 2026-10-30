#!/usr/bin/env python3
"""Failure cases before this layout edit: controls remain at the bottom;
prepare/build/info overlap the HUD or each other; choice actions overflow on
320px; fullscreen exit is covered; actual mouse/touch activation stops working.
Build first, then run with scripts/env.sh sourced. Evidence: .local/sf-top-actions/.
"""
import json,sys,shutil,traceback
import accept_combat_controls_ui as c
w=c.w;w.OUT=c.ROOT/'.local/sf-top-actions';w.OUT.mkdir(parents=True,exist_ok=True)
w.ENV.update(WIRE_BOARD_STARFORT_BROWSER_OUT=str(w.OUT),AGENT_BROWSER_PROFILE=str(w.OUT/'profile'))
passed=False
try:
 c.BASELINE=True;c.setup();c.BASELINE=False
 for width,height in [(1366,900),(900,900),(768,1024),(320,740)]:
  touch=width==320;c.b('set','viewport',str(width),str(height))
  for scenario in ['natural','coop-growth','coop-objectives','battle']:
   state=w.scenario(scenario,2)
   c.wait('document.querySelector(".starfort-board")?.dataset.phase==='+json.dumps(state['phase'])+'&&[...document.querySelectorAll("h1")].some(e=>e.textContent.includes('+json.dumps(scenario)+'))')
   if scenario=='coop-growth':c.wait('!!document.querySelector(".sf-choice-dialog")')
   if c.js('!!document.querySelector(".sf-choice-dialog")'):c.b('click','.sf-choice-dialog [aria-label="关闭"]');c.wait('!document.querySelector(".modal-backdrop")')
   for fullscreen in [False,True]:
    if fullscreen:c.enter()
    else:c.b('scrollintoview','.starfort-board');c.stable()
    geometry=c.js('''(()=>{const board=document.querySelector('.starfort-board').getBoundingClientRect(),r=s=>{const e=document.querySelector(s);if(!e)return null;const b=e.getBoundingClientRect();return {x:b.x-board.x,y:b.y-board.y,width:b.width,height:b.height,right:b.right-board.x,bottom:b.bottom-board.y}};return {board:board.toJSON(),hud:r('.sf-hud'),waves:r('.sf-wave-controls'),actions:r('.sf-field-actions'),exit:r('[data-room-fullscreen] [data-fullscreen-toggle]')}})()''')
    def overlaps(a,b):return a and b and min(a['right'],b['right'])-max(a['x'],b['x'])>1 and min(a['bottom'],b['bottom'])-max(a['y'],b['y'])>1
    name=f'{width}-{scenario}-'+('fullscreen' if fullscreen else 'page')
    for key in ['waves','actions']:
     box=geometry[key];assert box and box['y']>=0 and box['bottom']<=min(180,geometry['board']['height']/3),(name,key,geometry)
     assert box['x']>=0 and box['right']<=geometry['board']['width'],(name,key,geometry)
     assert not overlaps(box,geometry['hud']) and not overlaps(box,geometry['exit']),(name,key,geometry)
    assert not overlaps(geometry['waves'],geometry['actions']),(name,geometry)
    assert geometry['board']['width']-geometry['actions']['right']<=16,(name,geometry)
    w.artifact(name+'-geometry.json',geometry);c.screenshot(name)
    if scenario=='natural':
     c.activate('[data-sf-ready]',touch);c.wait('document.querySelector("[data-sf-ready]")?.getAttribute("aria-pressed")==="true"');c.activate('[data-sf-ready]',touch)
     c.activate('[data-sf-pause]',touch);c.wait('document.querySelector("[data-sf-pause]")?.getAttribute("aria-pressed")=="true"');c.activate('[data-sf-pause]',touch)
     c.activate('.sf-select-pad',touch);c.wait('!!document.querySelector(".sf-command-dialog")');c.b('click','.sf-command-dialog [aria-label="关闭"]')
    c.activate('[data-sf-open-info]',touch);c.wait('!!document.querySelector(".sf-info-dialog")')
    assert c.js('!document.querySelector(".sf-info-dialog .sf-objectives")'),'tasks must not remain in intelligence'
    c.b('click','.sf-info-dialog [aria-label="关闭"]')
    c.activate('[data-sf-open-tasks]',touch);c.wait('!!document.querySelector(".sf-tasks-dialog")')
    assert c.js('document.querySelector(".sf-tasks-dialog").getAttribute("aria-label")==="任务"&&!document.querySelector(".sf-tasks-dialog").textContent.includes("战区支线")')
    assert c.js('document.querySelector("[data-game-fullscreen-surface]").contains(document.querySelector(".sf-tasks-dialog"))')
    if scenario=='coop-objectives':
     assert c.js('document.querySelectorAll(".sf-tasks-dialog .sf-objective-card").length===3')
     if not fullscreen:
      before=w.data();objective=next(o for o in before['world']['objectives'] if o['kind']=='mine');selector='[data-sf-objective="'+objective['id']+'"]'
      c.b('scrollintoview',selector);c.activate(selector,touch)
      c.wait('document.querySelector(".sf-objective-active")!==null')
      assert w.data()['players'][0]['gold']==before['players'][0]['gold']-objective['cost']
    else:assert c.js('!!document.querySelector(".sf-tasks-dialog")')
    c.screenshot(name+'-tasks');c.b('click','.sf-tasks-dialog [aria-label="关闭"]')
    if scenario=='battle':c.b('wait','1100');assert c.js('!document.querySelector(".sf-tasks-dialog")')
    if scenario=='coop-growth':
     c.activate('[data-sf-open-choice]',touch);c.wait('!!document.querySelector(".sf-choice-dialog")');c.b('click','.sf-choice-dialog [aria-label="关闭"]')
    if fullscreen:c.leave()
    c.verify(name+' controls at upper right; visible and usable',True)
 passed=True
except Exception:
 traceback.print_exc()
 if w.OPENED:
  try:c.screenshot('failure')
  except Exception:pass
finally:
 if w.OPENED:c.b('close')
 if w.SERVER:(w.OUT/'stop').write_text('stop');w.SERVER.wait(timeout=20)
 w.artifact('result.json',{'passed':passed,'checks':w.REPORT['checks'],'sourceSHA256':{**c.source_hashes(),**c.hashes([c.ROOT/'games/starfort/tests/accept_top_actions_ui.py'])},'scope':'Real HTTP, production layout; native PC mouse and 320px touch emulation; normal and fullscreen; prepare, expansion choice, battle.'})
 for p in [w.OUT/'profile',w.OUT/'tmp']:
  if p.exists():shutil.rmtree(p)
 sys.exit(0 if passed else 1)
