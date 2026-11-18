#!/usr/bin/env python3
"""Test-first real two-human PAL transport/visibility E2E at the original inn.

Failures: queued old native keys move/open menus after a TCP outage; the fast
peer or server clock freezes; transport reconnect requires restarting a game;
backgrounding retains a held key, auto-takes over, remounts Canvas, or replays
old keys on return. Pixel convergence must agree with original authority.
"""
import json,os,shutil,signal,subprocess,time,uuid
from pathlib import Path
ROOT=Path(__file__).resolve().parents[3]
OUT=ROOT/'.local/pal/network-lifecycle';OUT.mkdir(parents=True,exist_ok=True)
BASE=(ROOT/'.local/pal/field-board-browser/url').read_text().strip()
ENV=dict(os.environ,AGENT_BROWSER_SOCKET_DIR=str(ROOT/'.local/pal-net-ab'),TMPDIR=str(ROOT/'.local/tmp'),AGENT_BROWSER_NO_VERSION_CHECK='1',AGENT_BROWSER_ARGS='--mute-audio,--disable-breakpad,--disable-crash-reporter')
Path(ENV['AGENT_BROWSER_SOCKET_DIR']).mkdir(parents=True,exist_ok=True)
SESSION=subprocess.check_output(['agent-browser','session','id','--scope','worktree','--prefix','pn'],cwd=ROOT,env=ENV,text=True).strip()
REPORT={'passed':False,'fullPerformanceAcceptance':False,'scope':'actual authenticated original inn and full React Board; TCP upstream stall, actual disconnect/reconnect, fast peer, native tab visibility and held/repeated keys; not full campaign or complete packet-loss/performance acceptance','cases':[]}
def wait(check,label,seconds=35):
 end=time.monotonic()+seconds
 while time.monotonic()<end:
  if check():return
  time.sleep(.08)
 raise AssertionError(label)
class Browser:
 def __init__(self,name,url):
  self.name=name;self.url=url;self.env=dict(ENV,AGENT_BROWSER_PROFILE=str(OUT/('profile-'+name)));Path(self.env['AGENT_BROWSER_PROFILE']).mkdir(exist_ok=True);self.driver=None;self.observer=None;self.device=None
 def b(self,*args):
  r=subprocess.run(['agent-browser','--session',SESSION+'-'+self.name,*map(str,args),'--json'],cwd=ROOT,env=self.env,text=True,capture_output=True,timeout=45)
  assert r.returncode==0,r.stdout+r.stderr
  value=json.loads(r.stdout);assert value['success'],value;return value['data']
 def js(self,source):return self.b('eval',source)['result']
 def api(self,path,body=None):
  options='' if body is None else ',{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify('+json.dumps(body)+')}'
  r=self.js('(async()=>{const r=await fetch('+json.dumps('/api'+path)+options+');return {status:r.status,body:await r.json()}})()');assert 200<=r['status']<300,r;return r['body']
 def room(self):return self.api('/state')['room']
 def data(self):return self.room()['game']['data']
 def command(self,type,**extra):
  room=self.room();return self.api('/rooms/'+room['id'],dict(type=type,version=room['version'],nonce=uuid.uuid4().hex,**extra))
 def attach(self):
  endpoint=self.b('get','cdp-url')['cdpUrl']
  self.driver=subprocess.Popen(['node','games/pal/tests/lifecycle-driver.mjs',endpoint,self.url],cwd=ROOT,env=self.env,text=True,stdin=subprocess.PIPE,stdout=subprocess.PIPE);assert self.driver.stdout.readline().strip()=='ready'
  self.observer=subprocess.Popen(['node','games/pal/tests/performance-observer.mjs',endpoint,self.url],cwd=ROOT,env=self.env,text=True,stdout=subprocess.PIPE);assert self.observer.stdout.readline().strip()=='ready'
 def drive(self,type,**extra):
  self.driver.stdin.write(json.dumps(dict(type=type,**extra))+'\n');self.driver.stdin.flush();return json.loads(self.driver.stdout.readline())
 def close(self):
  if self.device:self.device.terminate();self.device.wait(timeout=6)
  if self.driver:self.driver.stdin.close();self.driver.wait(timeout=6)
  if self.observer:self.observer.terminate();self.observer.communicate(timeout=6)
  try:self.b('close')
  except Exception:pass
  shutil.rmtree(self.env['AGENT_BROWSER_PROFILE'],ignore_errors=True)
proxy=subprocess.Popen(['node','games/pal/tests/performance-proxy.mjs',BASE],cwd=ROOT,env=ENV,text=True,stdin=subprocess.PIPE,stdout=subprocess.PIPE)
URL=json.loads(proxy.stdout.readline())['url']
def network(**change):
 proxy.stdin.write(json.dumps(change)+'\n');proxy.stdin.flush();return json.loads(proxy.stdout.readline())
a,b=Browser('a',URL),Browser('b',BASE);players=[a,b]
def position(player):
 d=player.data();return [d['x'],d['y']]
def converged(player):
 return player.js('(async()=>{const {authorityPixelsMatch}=await import("/pal-renderer-probe.js"),d=(await(await fetch("/api/state")).json()).room.game.data;return authorityPixelsMatch(d)})()')
probe=ROOT/'web/dist/pal-renderer-probe.js'
subprocess.run(['node','--input-type=module','-e','import {build} from "esbuild";await build({entryPoints:[process.argv[1]],outfile:process.argv[2],bundle:true,format:"esm",target:"es2022"});',str(ROOT/'games/pal/tests/field-render-probe.ts'),str(probe)],cwd=ROOT/'web',env=ENV,check=True)
try:
 names=[]
 for p in players:
  p.b('open',p.url);p.b('set','viewport',1280,900);names.append('仙剑恢复'+uuid.uuid4().hex[:6]);p.api('/register',{'name':names[-1],'password':'pal-net-test-123','invite':'test-invite'});p.attach()
 (OUT/'accounts.json').write_text(json.dumps(names,ensure_ascii=False)+'\n')
 room=a.api('/rooms',{'kind':'pal','name':'原版输入网络恢复','capacity':2})
 b.api('/rooms/'+room['id'],{'type':'join','version':a.room()['version'],'nonce':uuid.uuid4().hex})
 for p in players:p.command('ready')
 a.command('start')
 for p in players:p.b('reload')
 wait(lambda:not b.room().get('loading'),'both actual canvases did not finish loading')
 for p in players:p.b('wait','[data-pal-ready=true]');p.drive('remember-canvas')
 initial=b.data();assert initial['phase']=='field'
 a.b('click','[data-pal-board] canvas');network(pauseUp=True)
 a.b('press','ArrowDown');time.sleep(1.2)
 during=b.data();assert [during['x'],during['y']]==[initial['x'],initial['y']]
 assert during['field']['frame']>initial['field']['frame'],'one stalled client stopped the original server clock'
 network(pauseUp=False)
 wait(lambda:a.data().get('discardedSeq',0)>0,'old queued native input was not discarded')
 time.sleep(.35)
 assert position(b)==[initial['x'],initial['y']],'old tap replayed after restoring the upstream'
 wait(lambda:converged(a),'stale local prediction did not converge to original authority')
 REPORT['cases'].append({'check':'queued old native tap discarded; fast peer sees original clock; pixels converge','passed':True,'beforeFrame':initial['field']['frame'],'duringFrame':during['field']['frame'],'discardedSeq':a.data().get('discardedSeq'),'trace':a.drive('trace')})
 d=a.data();a.command('action',action={'type':'leader','epoch':d['epoch'],'target':1});wait(lambda:b.data()['leader']==1,'fast peer did not receive leader control')
 network(pauseUp=True,pauseDown=True)
 samples=[]
 b.b('click','[data-pal-board] canvas');b.js('window.palPerformance.mode="fast-peer-outage"')
 for _ in range(3):
  for key,dx,dy in [('ArrowDown',-16,8),('ArrowUp',16,-8)]:
   before=position(b);count=b.js('window.palPerformance.trials.length');b.b('press',key)
   wait(lambda:b.js('window.palPerformance.trials.length')>count,'trusted fast input was not observed')
   wait(lambda:b.js('window.palPerformance.trials.at(-1).responseMS!==undefined||!!window.palPerformance.trials.at(-1).error'),'fast peer canvas did not respond')
   sample=b.js('(()=>{const {expected,canvas,...x}=window.palPerformance.trials.at(-1);return x})()');assert not sample.get('error') and sample['trusted'] and sample['responseMS']<=100,sample;samples.append(sample)
   wait(lambda:position(b)==[before[0]+dx,before[1]+dy],'fast native rule step was blocked by the stalled peer')
 assert max(s['responseMS'] for s in samples)<=50,samples
 before_reconnect=a.drive('trace')['events'];old_closed=sum(e['kind']=='closed' for e in before_reconnect);old_connected=sum(e['kind']=='connected' for e in before_reconnect)
 network(pauseUp=False,pauseDown=False,disconnect=True)
 wait(lambda:sum(e['kind']=='closed' for e in a.drive('trace')['events'])>old_closed,'actual WebSocket disconnect was not observed')
 wait(lambda:sum(e['kind']=='connected' for e in a.drive('trace')['events'])>old_connected,'actual WebSocket did not reconnect')
 wait(lambda:a.drive('status').get('ready')=='true' and not a.drive('status').get('error'),'reconnect left an error/loading overlay')
 assert not b.room().get('loading') and all(not s.get('autoPlay') for s in b.room()['seats'])
 assert a.drive('status')['sameCanvas'];wait(lambda:converged(a),'reconnected original view did not converge in pixels')
 REPORT['cases'].append({'check':'fast native input during peer outage; actual socket reconnect without reload, takeover or global loading gate','passed':True,'fastSamples':samples,'reconnectTrace':a.drive('trace')})
 d=b.data();b.command('action',action={'type':'leader','epoch':d['epoch'],'target':0})
 wait(lambda:a.data()['leader']==0 and a.js('!!document.querySelector("[data-pal-direction=down]:not(:disabled)")'),'held reconnect owner was not ready')
 a.b('click','[data-pal-board] canvas');before=position(b)
 trace=a.drive('trace')['events'];sent_before=sum(e['kind']=='sent' for e in trace);closed_before=sum(e['kind']=='closed' for e in trace);connected_before=sum(e['kind']=='connected' for e in trace)
 network(pauseUp=True);a.drive('key',code='ArrowDown',down=True)
 wait(lambda:sum(e['kind']=='sent' for e in a.drive('trace')['events'])>sent_before,'native held input was not in flight')
 network(pauseUp=False,pauseDown=False,disconnect=True)
 wait(lambda:sum(e['kind']=='closed' for e in a.drive('trace')['events'])>closed_before,'held-input connection did not close')
 wait(lambda:sum(e['kind']=='connected' for e in a.drive('trace')['events'])>connected_before,'held-input connection did not reconnect')
 wait(lambda:a.drive('status').get('ready')=='true' and not a.drive('status').get('error'),'in-flight rejection left a permanent Board error')
 a.drive('key',code='ArrowDown',down=True,repeat=True);time.sleep(.35)
 assert position(b)==before,'unacknowledged held input replayed or resumed after reconnect'
 a.drive('key',code='ArrowDown',down=False);a.b('click','[data-pal-board] canvas');a.b('press','ArrowDown')
 wait(lambda:position(b)==[before[0]-16,before[1]+8],'fresh native key failed after in-flight rejection')
 a.b('press','ArrowUp');wait(lambda:position(b)==before,'fresh native key did not return after in-flight rejection')
 wait(lambda:converged(a),'in-flight rejection did not converge to original authority pixels')
 assert a.drive('status')['sameCanvas']
 REPORT['cases'].append({'check':'disconnect with unacknowledged native hold; no replay/repeat or permanent error; fresh key and pixels recover','passed':True,'trace':a.drive('trace')})
 d=a.data();a.command('action',action={'type':'leader','epoch':d['epoch'],'target':1});wait(lambda:b.data()['leader']==1 and b.js('!!document.querySelector("[data-pal-direction=down]:not(:disabled)")'),'background owner was not ready')
 b.b('click','[data-pal-board] canvas');b.drive('key',code='ArrowDown',down=True);time.sleep(.18)
 visibility=b.drive('background');assert visibility['hidden'],visibility
 time.sleep(.45);stopped=position(a)
 for _ in range(4):time.sleep(.12);assert position(a)==stopped,'original hero kept walking while the owner was hidden'
 restored=b.drive('foreground');assert not restored['hidden'],restored
 b.drive('key',code='ArrowDown',down=True,repeat=True);time.sleep(.3);assert position(a)==stopped,'a held key resumed on foreground without neutral input'
 b.drive('key',code='ArrowDown',down=False)
 wait(lambda:b.drive('status').get('ready')=='true' and not b.drive('status').get('error'),'foreground did not recover the original canvas')
 b.b('click','[data-pal-board] canvas');b.b('press','ArrowUp')
 wait(lambda:position(a)==[stopped[0]+16,stopped[1]-8],'a new native key did not work after foreground recovery')
 wait(lambda:converged(b),'foreground pixels did not converge to original authority')
 assert b.drive('status')['sameCanvas'] and all(not s.get('autoPlay') for s in b.room()['seats'])
 REPORT['cases'].append({'check':'real hidden tab releases held direction; foreground ignores repeats until neutral/new key; same Canvas and authority pixels','passed':True,'hidden':visibility,'foreground':restored,'trace':b.drive('trace')})
 b.b('set','viewport',320,820)
 endpoint=b.b('get','cdp-url')['cdpUrl']
 b.device=subprocess.Popen(['node','scripts/browser_device.mjs',endpoint,BASE,'320','820','--keep'],cwd=ROOT,env=b.env,text=True,stdout=subprocess.PIPE);assert b.device.stdout.readline().strip()=='ready'
 selector='[data-pal-direction=down]'
 b.js('document.querySelector('+json.dumps(selector)+').scrollIntoView({block:"center",behavior:"instant"})')
 x,y=b.js('(()=>{const r=document.querySelector('+json.dumps(selector)+').getBoundingClientRect();return [r.x+r.width/2,r.y+r.height/2]})()')
 before=position(a)
 subprocess.run(['node','scripts/browser_touch.mjs',endpoint,BASE,'tap',*map(str,[x,y,x,y])],cwd=ROOT,env=b.env,check=True)
 wait(lambda:position(a)==[before[0]-16,before[1]+8],'native touch did not recover after the background cycle')
 wait(lambda:converged(b),'recovered touch did not settle to authority pixels')
 geometry=b.js('(()=>{const a=document.querySelector("[data-pal-board] canvas").getBoundingClientRect(),s=document.querySelector("[data-pal-stage]").getBoundingClientRect(),buttons=[...document.querySelectorAll("[data-pal-controls] button")].filter(e=>e.getClientRects().length);return {width:a.width,height:a.height,stageWidth:s.width,touch:matchMedia("(pointer:coarse)").matches,inside:buttons.every(e=>{const r=e.getBoundingClientRect();return r.left>=a.left&&r.right<=a.right&&r.top>=a.top&&r.bottom<=a.bottom})}})()')
 assert geometry['touch'] and geometry['inside'] and abs(geometry['width']-geometry['stageWidth'])<2 and abs(geometry['width']/geometry['height']-1.6)<.01,geometry
 assert b.drive('status')['sameCanvas']
 REPORT['cases'].append({'check':'320px native touch after recovery; same proportional full-width Canvas and overlay controls','passed':True,'geometry':geometry})
 for p in players:p.b('screenshot',OUT/(p.name+'-recovered.png'),'--full')
 REPORT['passed']=True
finally:
 for p in players:
  if not REPORT['passed']:
   try:
    p.b('screenshot',OUT/('failure-'+p.name+'.png'),'--full');REPORT['debug-'+p.name]=p.drive('trace') if p.driver else None
   except Exception:pass
  try:p.close()
  except Exception:pass
 proxy.terminate();proxy.communicate(timeout=6);probe.unlink(missing_ok=True)
 (OUT/'result.json').write_text(json.dumps(REPORT,ensure_ascii=False,indent=2)+'\n');print(json.dumps({'passed':REPORT['passed'],'result':str(OUT/'result.json')},ensure_ascii=False),flush=True)
assert REPORT['passed']
