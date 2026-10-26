#!/usr/bin/env python3
"""Failures specified before changes: skill lacks icon/progress and touch tooltip;
static terrain repaints every frame, including ordinary health snapshots; tooltip
escapes fullscreen/viewport; pause advances cooldown; map gestures leave stale art.
Runs a real HTTP room and production Canvas. CPU timing is local browser evidence,
not a measurement of the user's phone or cellular RTT.
"""
import json,sys,time,shutil
from pathlib import Path
import accept_combat_controls_ui as c
w=c.w
baseline='--baseline' in sys.argv
w.OUT=c.ROOT/'.local/skill-cd'/('before' if baseline else 'after')
w.OUT.mkdir(parents=True,exist_ok=True)
w.ENV.update(WIRE_BOARD_STARFORT_BROWSER_OUT=str(w.OUT),AGENT_BROWSER_PROFILE=str(w.OUT/'profile'))
for key in ['TMPDIR','GOTMPDIR']:
 (w.OUT/'tmp').mkdir(exist_ok=True);w.ENV[key]=str(w.OUT/'tmp')

def profile():
 return c.js('''new Promise(resolve=>{
  const times=[], strokes=[], proto=CanvasRenderingContext2D.prototype,orig=proto.stroke;let count=0,id;
  proto.stroke=function(...args){count++;return orig.apply(this,args)};
  const tick=at=>{times.push(at);strokes.push(count);count=0;id=requestAnimationFrame(tick)};id=requestAnimationFrame(tick);
  setTimeout(()=>{cancelAnimationFrame(id);proto.stroke=orig;
   const intervals=times.slice(1).map((t,i)=>t-times[i]).sort((a,b)=>a-b);
   resolve({frames:times.length,p95MS:intervals[Math.floor(intervals.length*.95)],meanStrokes:strokes.slice(3).reduce((a,b)=>a+b,0)/Math.max(1,strokes.length-3)});
  },1600);
 })''')
try:
 # No per-frame recording probe during performance measurement.
 c.BASELINE=True;c.setup();c.BASELINE=baseline
 before={**c.source_hashes(),**c.hashes([Path(__file__)])};results=[]
 for width,height in [(1366,900),(320,740)]:
  c.b('set','viewport',str(width),str(height));w.scenario('central-passives',2);c.enter()
  sample=profile()
  if sample['frames']<45:sample=profile()
  assert sample['frames']>=45,('insufficient visible frames',sample)
  sample['width']=width;results.append(sample);print('PERF',sample,flush=True)
  w.artifact('performance.json',results)
  if not baseline:
   assert sample['meanStrokes']<200,('unchanged terrain still repainted',sample)
   selector=c.choose_visible_tower('cannon');c.activate(selector,width==320)
   c.wait('!!document.querySelector("[data-sf-skill]")')
   skill=c.visibility('[data-sf-skill]');assert skill['visible'] and skill['hit']
   ring=c.js('document.querySelector("[data-sf-skill-progress]").getAttribute("stroke-dashoffset")');assert ring is not None
   if width==320:c.activate('[data-sf-skill]',True)
   else:
    r=c.rect('[data-sf-skill]');c.b('mouse','move',r['x']+r['width']/2,r['y']+r['height']/2)
   c.wait('!!document.querySelector("[data-sf-skill-tooltip]")')
   tip=c.visibility('[data-sf-skill-tooltip]');assert tip['visible'] and tip['inside'],tip
   assert '填' in c.js('document.querySelector("[data-sf-skill-tooltip]").textContent')
   c.screenshot('skill-'+str(width))
   c.b('press','Escape');c.stable()
   if c.js('!!document.querySelector("[data-sf-skill-tooltip]")'):
    assert not c.js('!!document.fullscreenElement');c.b('press','Escape');c.stable()
   assert not c.js('!!document.querySelector("[data-sf-skill-tooltip]")')
  else:c.screenshot('before-'+str(width))
  if c.js('!!document.querySelector("[data-room-fullscreen]")'):c.leave()
 assert not c.b('errors')['errors'],c.b('errors')
 assert before=={**c.source_hashes(),**c.hashes([Path(__file__)])},'source changed during capture'
 (w.OUT/'failure.png').unlink(missing_ok=True)
 w.artifact('result.json',{'passed':not baseline,'baselineComplete':baseline,'measurements':results,'sourceSHA256':before,'buildSHA256':c.hashes((c.ROOT/'web/dist').rglob('*'))})
except Exception:
 import traceback
 traceback.print_exc()
 if w.OPENED:
  try:c.screenshot('failure')
  except Exception:pass
 sys.exit(1)
finally:
 if w.OPENED:
  try:c.b('close')
  except Exception:pass
 if w.SERVER:
  (w.OUT/'stop').write_text('stop');w.SERVER.wait(timeout=20)
 for p in [w.OUT/'profile',w.OUT/'tmp']:
  if p.exists():shutil.rmtree(p)
