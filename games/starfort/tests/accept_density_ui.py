#!/usr/bin/env python3
"""Before changes: full legal late-wave maps stall PC as splash work multiplies;
network snapshots repeatedly carry complete enemies and per-victim coordinates;
UI clicks/RTT timers starve; optimization must retain impact-time HP and resume.
Profiles production Canvas and real HTTP/WebSocket, with lightweight counters.
"""
import json,os,sys,time,shutil,subprocess
from pathlib import Path
import accept_combat_controls_ui as c
w=c.w;w.OUT=c.ROOT/'.local/starfort-density/browser';w.OUT.mkdir(parents=True,exist_ok=True)
w.ENV.update(WIRE_BOARD_STARFORT_BROWSER_OUT=str(w.OUT),AGENT_BROWSER_PROFILE=str(w.OUT/'profile'),AGENT_BROWSER_INIT_SCRIPTS=str(c.ROOT/'games/starfort/tests/density_probe.js'))
# Disable the detailed sprite recorder: copying every drawing would bias a profile.
c.BASELINE=True;c.BASELINE_COMBAT=False
passed=False;results=[];controller=None
try:
 w.run(['go','test','-count=1','-run','^TestLateWaveDensityProfile$','./games/starfort/server'],'fixture-build.log')
 c.setup();c.BASELINE=False
 w.PEERS.extend([w.Peer('密集防线同伴二'),w.Peer('密集防线同伴三')])
 for width,height,rate in [(1366,900,1),(1366,900,4),(320,740,4)]:
  c.b('set','viewport',str(width),str(height))
  controller=subprocess.Popen(['node','games/starfort/tests/density_control.mjs',c.b('get','cdp-url')['cdpUrl'],w.BASE,str(rate)],cwd=c.ROOT,env=w.ENV,stdin=subprocess.PIPE,stdout=subprocess.PIPE,stderr=subprocess.PIPE,text=True)
  assert controller.stdout.readline().strip()=='READY'
  calibration=c.js('(()=>{const at=performance.now();let value=0;for(let i=0;i<1000000;i++)value+=Math.sqrt(i);return {ms:performance.now()-at,value}})()')
  for kind in (['observed'] if '--observed-only' in sys.argv else ['central','coop','observed','ceiling']):
   w.scenario('density-'+kind,2 if kind=='observed' else 4);c.enter();c.b('wait','4000');c.js('window.__resetDensity()');c.b('wait','16000')
   c.activate('[data-sf-open-info]',width==320);c.b('wait','500')
   metrics=c.js('JSON.parse(JSON.stringify(window.__density))')
   sample=w.data();network=w.api('/state').get('network',{});latency=c.js('''(async()=>{let rows=[];for(let i=0;i<5;i++){const url='/healthz?density='+i+'&at='+Date.now();const start=performance.now();await (await fetch(url)).text();const entries=performance.getEntriesByName(new URL(url,location.href).href);rows.push({elapsed:performance.now()-start,network:entries.at(-1)?.duration});}return rows})()''')
   def pct(values,p):return sorted(values)[min(len(values)-1,int(len(values)*p))] if values else None
   result={'layout':kind,'width':width,'cpuRate':rate,'cpuCalibration':calibration,'wave':sample['wave'],'towers':len(sample['towers']),'enemies':len(sample['enemies']),'frameP50':pct(metrics['frames'],.5),'frameP95':pct(metrics['frames'],.95),'frameMax':max(metrics['frames'],default=0),'longTasks':metrics['longTasks'],'drawCallsPerPaint':metrics['draws']/max(1,metrics['painted']),'messageMaxBytes':max((p['bytes'] for p in metrics['messages']),default=0),'messageCount':len(metrics['messages']),'connection':network,'paintFPS':metrics['painted']/(max(1,sum(metrics['frames']))/1000),'clickMS':metrics['clicks'],'healthRequests':latency}
   results.append(result);print(json.dumps(result),flush=True);w.artifact(kind+'-'+str(width)+'-cpu'+str(rate)+'.json',result);c.screenshot(kind+'-'+str(width)+'-cpu'+str(rate))
   c.b('press','Escape')
   if c.js('!!document.querySelector("[data-room-fullscreen]")'):c.leave()
  controller.communicate('stop',timeout=10);controller=None
 if '--enforce' in sys.argv:
  for row in results:
   assert row['messageMaxBytes']<=192*1024, row
   assert row['drawCallsPerPaint']<=2000, row
   assert row['frameP95']<=34, row
   assert row['clickMS'] and max(row['clickMS'])<=200, row
 passed=True
except Exception:
 import traceback;traceback.print_exc()
 if w.OPENED:
  try:print(c.b("errors"));c.screenshot("failure")
  except Exception:pass
finally:
 if controller and controller.poll() is None:controller.communicate('stop',timeout=10)
 if w.OPENED:c.b('close')
 if w.SERVER:(w.OUT/'stop').write_text('stop');w.SERVER.wait(timeout=20)
 w.artifact('result.json',{'passed':passed,'profiles':results,'sourceSHA256':{**c.source_hashes(),**c.hashes([Path(__file__),c.ROOT/'games/starfort/tests/density_probe.js',c.ROOT/'games/starfort/tests/density_control.mjs'])},'scope':'Native PC mouse, 320px touch emulation; 45-wave artificial durable-enemy full-map stress. No physical mobile device. No detailed draw recorder.'})
 for p in [w.OUT/'profile',w.OUT/'tmp']:
  if p.exists():shutil.rmtree(p)
 sys.exit(0 if passed else 1)
