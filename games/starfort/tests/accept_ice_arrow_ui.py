#!/usr/bin/env python3
"""Before implementation: walls not drawn; slow clears with the wall; branch
buttons send power/speed; chance shown as fake CD; skill tips clip on touch;
instant kill applies to elites; effects/HP bypass the combat timeline.
Uses real HTTP actions and native mouse/touch against production game code.
"""
import json,sys,shutil,time
from pathlib import Path
import accept_combat_controls_ui as c
w=c.w;w.OUT=c.ROOT/'.local/ice-arrow/browser';w.OUT.mkdir(parents=True,exist_ok=True)
w.ENV.update(WIRE_BOARD_STARFORT_BROWSER_OUT=str(w.OUT),AGENT_BROWSER_PROFILE=str(w.OUT/'profile'))
for key in ['TMPDIR','GOTMPDIR']:(w.OUT/'tmp').mkdir(exist_ok=True);w.ENV[key]=str(w.OUT/'tmp')
passed=False;checks=[]
def mark(name,value):
 assert value,name
 checks.append(name);print('PASS',name,flush=True)
try:
 c.setup();before={**c.source_hashes(),**c.hashes([Path(__file__)])}
 for width,height in [(1366,900),(320,740)]:
  touch=width==320;c.b('set','viewport',str(width),str(height))
  for branch,skill in [('volley','volley'),('heart','execute')]:
   w.scenario('ice-arrow');c.enter();selector=c.choose_visible_tower('arrow');c.activate(selector,touch)
   c.wait('!!document.querySelector("[data-sf-upgrade=volley]")')
   mark('two skill branches '+str(width),c.js('!!document.querySelector("[data-sf-upgrade=heart]")&&!document.querySelector("[data-sf-upgrade=power]")'))
   mark('branch consequences visible before choice '+str(width),c.js('document.querySelector("[data-sf-upgrade=heart]").textContent.includes("Lv.8")&&document.querySelector("[data-sf-upgrade=volley]").textContent.includes("25%")'))
   c.screenshot('branches-'+str(width))
   c.activate('[data-sf-upgrade='+branch+']',touch)
   c.wait('document.querySelector("[data-sf-skill-status]")?.textContent.includes("%")')
   mark('authoritative selected branch '+branch,any(t['kind']=='arrow' and t['branch']==branch for t in w.data()['towers']))
   c.activate('[data-sf-skill]',touch);c.wait('!!document.querySelector("[data-sf-skill-tooltip]")')
   mark('skill description visible '+branch,c.visibility('[data-sf-skill-tooltip]')['visible'] and c.visibility('[data-sf-skill-tooltip]')['hit'])
   c.screenshot(branch+'-'+str(width))
   seen=[];deadline=time.monotonic()+24
   while time.monotonic()<deadline:
    state=w.data();seen.extend(e for e in state['events'] if e.get('skill')==skill)
    if seen:break
    time.sleep(.25)
   mark('server skill event '+branch,bool(seen));w.artifact(branch+'-events-'+str(width)+'.json',seen)
   if branch=='heart':mark('elite and boss survive',all(e['hp']>0 for e in state['enemies'] if e['kind'] in ['elite','boss']))
   c.leave()
  w.scenario('ice-wall');c.enter()
  c.js("""(()=>{window.__icePaint=0;window.__iceRaster=0;const p=CanvasRenderingContext2D.prototype,fill=p.fill,draw=p.drawImage;p.fill=function(...a){if(this.fillStyle==='#b7edff'){window.__iceRaster++;if(this.canvas.matches('[data-starfort-canvas]'))window.__icePaint++;}return fill.apply(this,a)};p.drawImage=function(...a){if(this.canvas.matches('[data-starfort-canvas]')&&a[0]?.dataset?.sfIceArt==='true')window.__icePaint++;return draw.apply(this,a)};return true})()""")
  deadline=time.monotonic()+24;frozen=[]
  while time.monotonic()<deadline:
   state=w.data();frozen=[e for e in state['enemies'] if e.get('frostbound')]
   if frozen and state.get('iceWalls'):break
   time.sleep(.25)
  mark('enemy gains permanent frost '+str(width),bool(frozen));mark('ice wall painted '+str(width),c.js('window.__icePaint>0'))
  raster=c.js('window.__iceRaster');c.b('wait','500')
  rasterDelta=c.js('window.__iceRaster')-raster
  mark('steady walls reuse cached artwork '+str(width),rasterDelta<=14)
  w.artifact('ice-drawing-'+str(width)+'.json',{'crystalsRasterizedOver500MS':rasterDelta})
  wall=state['iceWalls'][0]
  point=c.js('''(()=>{const e=document.querySelector('[data-sf-minimap]'),p=e.createSVGPoint();p.x='''+str(wall['x'])+';p.y='+str(wall['y'])+''';const r=p.matrixTransform(e.getScreenCTM());return {x:r.x,y:r.y}})()''')
  w.mouse_click(point['x'],point['y']);c.b('wait','1200');c.screenshot('ice-wall-'+str(width))
  w.artifact('frost-'+str(width)+'.json',{'walls':state['iceWalls'],'enemies':frozen})
  c.b('reload');c.wait('document.querySelector(".sf-battlefield")?.dataset.assetsReady==="true"')
  mark('frost survives page reload '+str(width),any(e.get('frostbound') for e in w.data()['enemies']))
 mark('source unchanged',before=={**c.source_hashes(),**c.hashes([Path(__file__)])});mark('no browser errors',not c.b('errors')['errors']);passed=True
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
 w.artifact('result.json',{'passed':passed,'checks':checks,'sourceSHA256':{**c.source_hashes(),**c.hashes([Path(__file__)])},'buildSHA256':c.hashes((c.ROOT/'web/dist').rglob('*')),'scope':'Local Chromium 1366px mouse and 320px native touch emulation; real game module/HTTP, not physical phone'})
 for p in [w.OUT/'profile',w.OUT/'tmp']:
  if p.exists():shutil.rmtree(p)
 sys.exit(0 if passed else 1)
