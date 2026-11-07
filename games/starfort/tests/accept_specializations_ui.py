#!/usr/bin/env python3
"""Before implementation: wrong/hidden branch choices; touch choice upgrades a
different tower; tooltips omit chance, immunity or duration; freeze/silence not
rendered; status applied before impact; refresh loses chosen skill. Native
Chromium mouse/320px touch, real HTTP/actions/timers, labelled combat fixtures.
"""
import json, sys, time, shutil
from pathlib import Path
import accept_combat_controls_ui as c
w=c.w
w.OUT=c.ROOT/'.local/starfort-specializations/browser'
w.OUT.mkdir(parents=True,exist_ok=True)
w.ENV.update(WIRE_BOARD_STARFORT_BROWSER_OUT=str(w.OUT),AGENT_BROWSER_PROFILE=str(w.OUT/'profile'))
for key in ['TMPDIR','GOTMPDIR']:
 (w.OUT/'tmp').mkdir(exist_ok=True);w.ENV[key]=str(w.OUT/'tmp')
checks=[];passed=False
def mark(name,value):
 assert value,name
 checks.append(name);print('PASS',name,flush=True)
try:
 c.setup();before={**c.source_hashes(),**c.hashes([Path(__file__)])}
 for width,height in [(1366,900),(320,740)]:
  touch=width==320;c.b('set','viewport',str(width),str(height))
  cases=[('frost',['freeze','speed'],'freeze','freeze'),('cannon',['power','speed'],'power','blast'),('arcane',['power','magic'],'magic','meteor'),('arrow',['volley','heart'],'volley','knockback')]
  if '--frost-only' in sys.argv:cases=cases[:1]
  for kind,branches,selected,skill in cases:
   w.scenario('specialization-'+kind);c.enter()
   c.js('''(()=>{window.__specialPaint={freeze:0,silence:0};window.__freezePaint={first:0,last:0};const p=CanvasRenderingContext2D.prototype,stroke=p.stroke;p.stroke=function(...args){if(this.canvas.matches('[data-starfort-canvas]')){if(this.strokeStyle==='#e2faff'){window.__specialPaint.freeze++;const n=performance.now();if(!window.__freezePaint.first)window.__freezePaint.first=n;window.__freezePaint.last=n;}if(this.strokeStyle==='#e9d9ff')window.__specialPaint.silence++;}return stroke.apply(this,args)};return true})()''')
   selector=c.choose_visible_tower(kind);c.activate(selector,touch)
   c.wait('!!document.querySelector("[data-sf-upgrade='+selected+']")')
   mark('branches '+kind+' '+str(width),all(c.js('!!document.querySelector('+json.dumps('[data-sf-upgrade='+branch+']')+')') for branch in branches))
   mark('choice descriptions '+kind+' '+str(width),c.js('document.querySelector('+json.dumps('[data-sf-upgrade='+selected+']')+').textContent.includes("%")'))
   c.screenshot(kind+'-choices-'+str(width))
   c.activate('[data-sf-upgrade='+selected+']',touch)
   c.wait('document.querySelector("[data-sf-skill-status]")?.textContent.includes("%")')
   mark('saved branch '+kind,any(t['kind']==kind and t['branch']==selected for t in w.data()['towers']))
   c.activate('[data-sf-skill]',touch);c.wait('!!document.querySelector("[data-sf-skill-tooltip]")')
   mark('readable skill tooltip '+kind+' '+str(width),c.visibility('[data-sf-skill-tooltip]')['visible'])
   c.screenshot(kind+'-skill-'+str(width))
   seen=[];deadline=time.monotonic()+45
   while time.monotonic()<deadline:
    state=w.data();seen.extend(e for e in state['events'] if e.get('skill')==skill)
    if seen:break
    time.sleep(.25)
   mark('authoritative skill '+skill+' '+str(width),bool(seen))
   if kind in ['frost','arcane']:
    status='freeze' if kind=='frost' else 'silence'
    c.wait('window.__specialPaint.'+status+'>0')
    mark('Canvas status painted '+status+' '+str(width),c.js('window.__specialPaint.'+status)>0)
    c.screenshot(kind+'-status-'+str(width))
    if kind=='frost':
     c.wait('window.__freezePaint.first>0&&performance.now()-window.__freezePaint.last>300')
     timing=c.js('({...window.__freezePaint,duration:window.__freezePaint.last-window.__freezePaint.first})')
     w.artifact('freeze-duration-'+str(width)+'.json',timing)
     mark('freeze lasts two seconds on display clock '+str(width),1800<=timing['duration']<=2400)
   w.artifact(kind+'-effects-'+str(width)+'.json',{'events':seen,'enemies':state['enemies']})
   c.b('reload');c.wait('document.querySelector(".sf-battlefield")?.dataset.assetsReady==="true"')
   mark('selected branch survives reload '+kind,any(t['kind']==kind and t['branch']==selected for t in w.data()['towers']))
 mark('sources unchanged during acceptance',before=={**c.source_hashes(),**c.hashes([Path(__file__)])})
 mark('no browser errors',not c.b('errors')['errors']);passed=True
except Exception:
 import traceback;traceback.print_exc()
 if w.OPENED:
  try:c.screenshot('failure')
  except Exception:pass
finally:
 if w.OPENED:
  try:c.b('close')
  except Exception:pass
 if w.SERVER:(w.OUT/'stop').write_text('stop');w.SERVER.wait(timeout=20)
 w.artifact('result.json',{'passed':passed,'checks':checks,'sourceSHA256':{**c.source_hashes(),**c.hashes([Path(__file__)])},'buildSHA256':c.hashes((c.ROOT/'web/dist').rglob('*')),'scope':'Production UI and real HTTP in local Chromium; deterministic combat fixtures; desktop mouse and 320px touch emulation, not a physical phone'})
 for p in [w.OUT/'profile',w.OUT/'tmp',w.PROFILE]:
  if p.exists():shutil.rmtree(p)
 sys.exit(0 if passed else 1)
