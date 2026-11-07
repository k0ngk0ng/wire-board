#!/usr/bin/env python3
"""Real platform rooms, loading, native input, fullscreen and replay evidence.

Run browser_build.py, build web, and start TestContraPlatformBrowserFixture.
Only user actions and read-only snapshots are used; no injected game state.
"""
import argparse, hashlib, http.cookiejar, json, os, re, shutil, subprocess, sys, time, traceback, urllib.request, uuid
from pathlib import Path
sys.dont_write_bytecode=True
from browser_network import BrowserNetwork

root=Path(__file__).resolve().parents[2]
parser=argparse.ArgumentParser();parser.add_argument('--fixture',required=True);parser.add_argument('--label',required=True);parser.add_argument('--presentation',action='store_true');parser.add_argument('--network-boundaries',action='store_true');args=parser.parse_args()
assert re.fullmatch(r'[a-z0-9-]+',args.label)
fixture=json.loads((root/args.fixture).read_text());human_only=fixture.get('supportsBots') is False;base=fixture['url'];out=root/'.local/contra-browser'/('platform-'+args.label);out.mkdir();temp=out/'temporary';temp.mkdir();(temp/'tmp').mkdir()
script_bytes=Path(__file__).read_bytes();(out/'harness-at-run.py').write_bytes(script_bytes)
input_bytes=(root/'scripts/contra/browser_input.mjs').read_bytes();(out/'native-input-at-run.mjs').write_bytes(input_bytes)
network_bytes=(root/'scripts/contra/browser_network.py').read_bytes();(out/'network-at-run.py').write_bytes(network_bytes)
report={'passed':False,'scope':'Actual private platform, standard two-human new game and first-stage input; not a natural eight-stage campaign or WAN acceptance.','fixture':{key:fixture.get(key)for key in ['url','rulesBuildID','runtimeBuildID','visualsID','webRoot','publicRoot','webManifestSHA256']},'cases':[]};browsers=[]
network=BrowserNetwork(base)if args.network_boundaries else None
if network:base=network.url;report['browserOrigin']=base
class Browser:
 def __init__(self,name,width,height):
  self.name=name;self.session='cp'+uuid.uuid4().hex[:6];self.socket=root/'.s'/self.session;self.socket.mkdir(parents=True)
  self.env=dict(os.environ,TMPDIR=str(temp/'tmp'),AGENT_BROWSER_SESSION=self.session,AGENT_BROWSER_SOCKET_DIR=str(self.socket),AGENT_BROWSER_PROFILE=str(temp/name),AGENT_BROWSER_ARGS='--mute-audio,--disable-breakpad,--disable-crash-reporter')
  browsers.append(self);self.run('open',base);self.run('set','viewport',str(width),str(height))
 def run(self,*a):
  p=subprocess.run(['agent-browser',*a,'--json'],cwd=root,env=self.env,capture_output=True,text=True,timeout=45);assert p.returncode==0,p.stdout+p.stderr;j=json.loads(p.stdout);assert j['success'],j;return j['data']
 def js(self,code):return self.run('eval',code)['result']
 def api(self,path,body=None,expected_status=None):
  value=self.js('(async()=>{const r=await fetch('+json.dumps(path)+','+json.dumps({'method':'GET'}if body is None else {'method':'POST','headers':{'Content-Type':'application/json'},'body':json.dumps(body)})+');return {status:r.status,data:await r.json()}})()');assert (200<=value['status']<300 if expected_status is None else value['status']==expected_status),value;return value['data']
 def command(self,kind,extra=None,expected_status=None):
  state=self.api('/api/state');return self.api('/api/rooms/'+state['room']['id'],{'type':kind,'version':state['room']['version'],'nonce':uuid.uuid4().hex,'serverInstance':state['serverInstance'],**(extra or {})},expected_status)
 def login(self,index):self.api('/api/login',fixture['users'][index]);self.run('reload')
 def wait(self,expression):return self.run('wait','--fn',expression+'||!!document.querySelector(".contra-board-error")')
 def data(self):
  value=self.js('({surface:{...document.querySelector("[data-contra-runtime]")?.dataset},error:document.querySelector(".contra-board-error")?.textContent})');assert not value.get('error'),value;return value.get('surface')
 def native(self,options):
  assert (root/'scripts/contra/browser_input.mjs').read_bytes()==input_bytes,'native input helper changed during acceptance'
  p=subprocess.run(['node','scripts/contra/browser_input.mjs',self.run('get','cdp-url')['cdpUrl'],base,json.dumps({'platform':True,**options})],cwd=root,env=self.env,capture_output=True,text=True,timeout=50);assert p.returncode==0,p.stdout+p.stderr;return json.loads(p.stdout)
 def capture(self,name):self.run('screenshot',str(out/(name+'.png')))
 def sync(self):return self.api('/api/rooms/'+getattr(self,'room_id',room_id)+'/sync')['data']

def external(index):
 client=urllib.request.build_opener(urllib.request.HTTPCookieProcessor(http.cookiejar.CookieJar()))
 def request(path,body=None):
  req=urllib.request.Request(base+path,data=None if body is None else json.dumps(body).encode(),headers={}if body is None else {'Content-Type':'application/json'})
  with client.open(req,timeout=15)as response:return json.load(response)
 request('/api/login',fixture['users'][index]);return request

try:
 for index,user in enumerate(fixture['users']):
  user['name']='魂斗罗'+args.label[:6]+'-'+str(index)+'-'+uuid.uuid4().hex[:5]
  request=urllib.request.Request(base+'/api/register',data=json.dumps({**user,'invite':'contra-browser'}).encode(),headers={'Content-Type':'application/json'})
  with urllib.request.urlopen(request,timeout=15)as response:assert response.status==200
 desktop=Browser('desktop',1100,1000);desktop.login(0)
 created=desktop.api('/api/rooms',{'kind':'contra','capacity':2,'name':'魂斗罗双人平台验收','options':{}});room_id=created['id']
 if human_only:
  desktop.run('reload');desktop.run('wait','--text','魂斗罗双人平台验收')
  assert not desktop.js('[...document.querySelectorAll("button")].some(b=>b.textContent.includes("添加电脑"))')
  denied=desktop.command('add_bot',expected_status=400);report['computerAdmissionDenied']=denied
 teammate=external(1)
 def teammate_command(kind):
  state=teammate('/api/state');room=state.get('room')or created
  return teammate('/api/rooms/'+room_id,{'type':kind,'version':room['version'],'nonce':uuid.uuid4().hex})
 teammate_command('join');teammate_command('ready');desktop.run('reload');desktop.run('wait','--text','魂斗罗双人平台验收')
 assert desktop.js('document.querySelectorAll("[data-fullscreen-toggle]").length')==0
 desktop.capture('desktop-waiting');desktop.command('ready');desktop.command('start')
 desktop.wait('!!document.querySelector("[data-contra-runtime]")?.dataset.digest');desktop.data()
 first=desktop.sync();assert first['view']['snapshot']['frame']=='0',first['view']['snapshot']['frame']
 assert desktop.api('/api/state')['room']['loading'];desktop.capture('desktop-loading-barrier')
 mobile=Browser('mobile',320,740);mobile.login(1)
 mobile.wait('Number(document.querySelector("[data-contra-runtime]")?.dataset.checked)>5');mobile.data()
 desktop.wait('Number(document.querySelector("[data-contra-runtime]")?.dataset.checked)>5');desktop.data()
 assert not desktop.api('/api/state')['room'].get('loading')
 # Use the platform's actual fullscreen entry. This leaves the Canvas mounted.
 for browser in [desktop,mobile]:
  before=browser.data();browser.js('window.__contraCanvas=document.querySelector(".contra-runtime canvas");true');browser.run('click','[data-fullscreen-toggle]');browser.wait('!!document.querySelector("[data-room-fullscreen]")')
  assert browser.js('window.__contraCanvas===document.querySelector(".contra-runtime canvas")')
  assert browser.js('(()=>{const r=document.querySelector("[data-room-fullscreen]");return !r.querySelector(".room-head,.player-strip,.topbar")&&r!==document.body&&r!==document.documentElement})()')
  browser.capture(browser.name+'-intro-fullscreen')
 desktop.native({'keys':['Space'],'frames':3});desktop.wait('document.querySelector("[data-contra-runtime]")?.dataset.phase==="playing"')
 # Standard physical movement and firing, independently authenticated seats.
 for browser,actor in [(desktop,0),(mobile,1)]:
  if args.presentation:browser.js('(()=>{window.__contraVisible=[];window.__contraObserve=true;let previous="";const sample=at=>{const r=document.querySelector("[data-contra-runtime]");if(r&&r.dataset.revision!==previous){previous=r.dataset.revision;window.__contraVisible.push({at,revision:previous,frame:r.dataset.frame,audioTick:r.dataset.audioTick,lives:r.dataset.lives,hud:[...document.querySelectorAll(".contra-game-resources>span")].map(n=>n.textContent),publicHUD:[...document.querySelectorAll(".module-players .player-extras")].map(n=>n.textContent)});}if(window.__contraObserve)requestAnimationFrame(sample)};requestAnimationFrame(sample);return true})()')
  before=browser.sync()['view']['snapshot']
  damage=None
  if args.presentation and actor==0 and browser.data().get('lives'):
   desktop.native({'keys':['KeyD'],'frames':2,'hold':True});mobile.native({'keys':['KeyD'],'frames':2,'hold':True})
   try:damage=browser.native({'frames':1800,'captureNetwork':True,'stopOnLifeDrop':0,'timeoutMS':40000})
   finally:
    desktop.native({'keys':['KeyD'],'releaseOnly':True});mobile.native({'keys':['KeyD'],'releaseOnly':True})
   browser.capture('desktop-life-loss')
   deadline=time.monotonic()+5
   while time.monotonic()<deadline:
    recovered=browser.sync()['view']['snapshot']
    if recovered['players'][0]['state']==1:break
    time.sleep(.05)
   assert recovered['players'][0]['state']==1,recovered['players'][0]
   before=browser.sync()['view']['snapshot']
  frame_count=72 if args.presentation else 18
  options={'keys':['KeyD','KeyJ'],'frames':frame_count,'captureNetwork':True}if actor==0 else {'touch':[{'selector':'[data-controller-dpad]','x':.83,'y':.5},{'selector':'[data-controller-button="b"]'}],'frames':frame_count,'captureNetwork':True}
  movement=browser.native(options);fire=browser.native({'mouse':'[data-controller-button="b"]','frames':4})if actor==0 else None
  jump=browser.native({'keys':['KeyK'],'frames':6}if actor==0 else {'touch':[{'selector':'[data-controller-button="a"]'}],'frames':6})
  after=browser.sync();assert after['view']['snapshot']['players'][actor]['x']!=before['players'][actor]['x']
  assert any(row['pads'][actor]&65==65 for row in after['view']['frames']if row['kind']=='pads'),'combined native input not accepted'
  browser.capture(browser.name+'-playing-fullscreen')
  geometry=browser.js('(()=>{const c=document.querySelector(".contra-runtime canvas").getBoundingClientRect(),p=document.querySelector("[data-controller-layout]").getBoundingClientRect();const hud=[...document.querySelectorAll(".contra-game-resources>span")].map(n=>{const r=n.getBoundingClientRect(),s=getComputedStyle(n);return {text:n.textContent,x:r.x,y:r.y,w:r.width,h:r.height,color:s.color,background:getComputedStyle(n.parentElement).backgroundColor}});return {canvas:{x:c.x,y:c.y,w:c.width,h:c.height},controller:{x:p.x,y:p.y,w:p.width,h:p.height},hud,overflow:document.documentElement.scrollWidth>innerWidth+1,width:innerWidth,height:innerHeight}})()')
  assert not geometry['overflow']and geometry['controller']['y']>=geometry['canvas']['y']+geometry['canvas']['h']-1 and geometry['controller']['y']+geometry['controller']['h']<=geometry['height']+1,geometry
  assert all(0<=h['x']and h['x']+h['w']<=geometry['width']+1 for h in geometry['hud'])
  report['cases'].append({'name':browser.name,'before':before,'after':after,'nativeMove':movement,'nativeMouse':fire,'nativeJump':jump,'nativeDamage':damage,'geometry':geometry,'client':browser.data()})
  if args.presentation:
   visible=browser.js('window.__contraObserve=false;window.__contraVisible');report['cases'][-1]['visible']=visible
   steps=[int(b['revision'])-int(a['revision'])for a,b in zip(visible,visible[1:])];intervals=[b['at']-a['at']for a,b in zip(visible,visible[1:])]
   assert len(steps)>=30 and sum(step==1 for step in steps)/len(steps)>=.9,('authoritative frames skipped between visible RAFs',steps)
   assert 13<=sorted(intervals)[len(intervals)//2]<=21,intervals
   for row in visible:
    lives=json.loads(row['lives']);assert [int(re.search(r'生命\s+(\d+)',value)[1])for value in row['hud']]==lives[:len(row['hud'])],row
    assert [int(re.search(r'生命\s+(\d+)',value)[1])for value in row['publicHUD']]==lives[:len(row['publicHUD'])],row
 desktop.native({'keys':['Space'],'frames':3,'cursor':'revision'});desktop.wait('document.querySelector("[data-contra-runtime]")?.dataset.paused==="true"');paused=desktop.sync()
 cursor=desktop.data()['audioTick'];desktop.wait('BigInt(document.querySelector("[data-contra-runtime]").dataset.audioTick)>=BigInt('+json.dumps(cursor)+')+30n');held=desktop.sync()
 assert held['view']['snapshot']['frame']==paused['view']['snapshot']['frame']and held['view']['snapshot']['players']==paused['view']['snapshot']['players']
 report['pause']={'before':paused,'after':held}
 # Real ESC exit keeps game instance and input state; settings modal blocks keys.
 desktop.run('press','Escape');desktop.wait('!document.querySelector("[data-room-fullscreen]")');assert desktop.js('window.__contraCanvas===document.querySelector(".contra-runtime canvas")')
 mobile.run('click','[data-room-fullscreen] [data-fullscreen-toggle]');mobile.wait('!document.querySelector("[data-room-fullscreen]")');mobile.capture('mobile-exit')
 report['exit']={b.name:b.js('({scroll:getComputedStyle(document.body).overflow,inert:document.querySelectorAll("[inert]").length})')for b in [desktop,mobile]}
 assert all(v['inert']==0 for v in report['exit'].values())
 # The real settings modal owns keyboard capture. Account profiles stay separate.
 mobile.run('click','button[aria-label="控制器设置"]');mobile.wait('!!document.querySelector(".controller-settings [data-controller-save]")')
 isolated=mobile.native({'keys':['KeyD','KeyJ'],'frames':4,'cursor':'revision'});assert mobile.sync()['view']['inputs'][1]['held']==0
 mobile.run('click','.controller-settings [data-controller-button="b"]');mobile.native({'keys':['KeyH'],'frames':2,'cursor':'revision'});mobile.run('click','[data-controller-save]');mobile.run('wait','--text','控制器设置已保存');mobile.capture('mobile-controller-settings')
 profiles={'mobile':mobile.api('/api/controllers'),'desktop':desktop.api('/api/controllers')};assert profiles['mobile']['profiles']['classic-2']['keyboard']['b']=='KeyH';assert profiles['desktop']['profiles']['classic-2']['keyboard']['b']=='KeyJ'
 mobile.run('click','.controller-settings button[aria-label="关闭"]');mobile.wait('!document.querySelector(".controller-settings")');assert mobile.js('window.__contraCanvas===document.querySelector(".contra-runtime canvas")')
 # Closing the real modal and draining authoritative input are asynchronous.
 # Keep the native key held and require a server acknowledgement, not merely
 # three unrelated timer revisions; capture every sample on failure.
 mobile.js('document.querySelector("[data-contra-runtime]").focus({preventScroll:true});true');remapped=mobile.native({'keys':['KeyH'],'frames':3,'cursor':'revision','hold':True})
 samples=[];deadline=time.monotonic()+3
 while True:
  held_profile=mobile.sync();samples.append({'at':time.monotonic(),'revision':held_profile['view']['revision'],'inputs':held_profile['view']['inputs']})
  if held_profile['view']['inputs'][1]['held']&64 or time.monotonic()>=deadline:break
  time.sleep(.03)
 report['remappedInputAcknowledgement']=samples
 assert held_profile['view']['inputs'][1]['held']&64
 mobile.native({'keys':['KeyH'],'cursor':'revision','releaseOnly':True});report['profiles']={'settings':profiles,'isolated':isolated,'remapped':remapped,'held':held_profile}
 # Keep the rule clock paused while creating a real clipped revision gap.
 blank=desktop.run('tab','new','about:blank')['tabId'];gap_start=int(paused['view']['revision']);deadline=time.monotonic()+8
 while time.monotonic()<deadline:
  state=teammate('/api/rooms/'+room_id+'/sync')['data']
  if int(state['view']['revision'])>gap_start+135:break
  time.sleep(.05)
 assert int(state['view']['revision'])>gap_start+135
 desktop.run('tab','t1');desktop.wait('Number(document.querySelector("[data-contra-runtime]")?.dataset.resyncs)>=1');desktop.data();desktop.run('tab','close',blank)
 report['recovery']={'client':desktop.data(),'server':desktop.sync()}
 # Observer uses the same authority and graphics without sending any controls.
 observer=Browser('observer',1100,1000);observer.login(2);observer.api('/api/rooms/'+room_id+'/watch',{});observer.run('reload');observer.wait('Number(document.querySelector("[data-contra-runtime]")?.dataset.checked)>5');observer.data()
 assert observer.js('document.querySelectorAll("[data-controller-layout]").length')==0
 report['observer']={'client':observer.data(),'actor':observer.sync()['view']['actor']};assert report['observer']['actor']==-1
 # Public human-only delivery omits both UI capabilities and server admission.
 if human_only:
  assert desktop.js('document.querySelectorAll("button.autoplay-control").length')==0
  denied=desktop.command('autoplay',{'enabled':True},expected_status=400)
  manual=desktop.sync();assert not any(row['automated'] for row in manual['view']['inputs'])
  report['humanOnly']={'autoplayAdmissionDenied':denied,'inputs':manual['view']['inputs']}
 else:
  # Explicit player takeover is platform-owned and never creates another game.
  desktop.run('scrollintoview','button.autoplay-control');desktop.js('document.querySelector("[data-contra-runtime]").focus({preventScroll:true});true')
  handoff=desktop.native({'keys':['KeyD'],'frames':3,'cursor':'revision','clickAfterHold':'button.autoplay-control'})
  report['handoff']={'native':handoff}
  desktop.run('wait','--text','取消托管');desktop.data()
  auto=desktop.sync();assert desktop.api('/api/state')['room']['seats'][0]['autoPlay']
  desktop.command('autoplay',{'enabled':False});desktop.run('wait','--text','开启托管');manual=desktop.sync();assert not manual['view']['inputs'][0]['automated'];assert manual['view']['epoch']==first['view']['epoch']
  report['handoff'].update({'automatic':auto,'manual':manual})
  sent=[json.loads(event['params']['response']['payloadData'])for event in handoff['network']if event['method']=='Network.webSocketFrameSent']
  release=[packet for packet in sent if packet.get('action',{}).get('type')=='release']
  report['handoff']['releasePackets']=release
  assert release,'held direction -> native autoplay click did not send a release'
  acks=[json.loads(event['params']['response']['payloadData'])for event in handoff['network']if event['method']=='Network.webSocketFrameReceived']
  assert all(any(ack.get('nonce')==packet['nonce']and ack.get('status')==200 for ack in acks)for packet in release),'release lacked successful authoritative ACK'
  assert manual['view']['inputs'][0]['held']==0
 # Reload restores the same epoch and server sequence with persisted mapping.
 before_reload=mobile.sync();mobile.run('reload');mobile.wait('Number(document.querySelector("[data-contra-runtime]")?.dataset.checked)>3');restored=mobile.sync();assert restored['view']['epoch']==before_reload['view']['epoch'];assert int(restored['view']['revision'])>=int(before_reload['view']['revision'])
 assert mobile.api('/api/controllers')['profiles']['classic-2']['keyboard']['b']=='KeyH'
 report['reload']={'before':before_reload,'after':restored,'client':mobile.data()}
 # One human can start and play without filling the second optional seat.
 if human_only:
  solo=Browser('solo',1100,1000)
  user={'name':'魂斗罗单人-'+uuid.uuid4().hex[:6],'password':'contra-solo-browser-123','invite':'contra-browser'}
  solo.api('/api/register',user);solo.run('reload')
  room=solo.api('/api/rooms',{'kind':'contra','capacity':1,'name':'魂斗罗单人开局验收','options':{}})
  solo.room_id=room['id']
  solo.run('reload');solo.run('wait','--text','魂斗罗单人开局验收')
  assert not solo.js('[...document.querySelectorAll("button")].some(b=>b.textContent.includes("添加电脑"))')
  solo.command('ready');solo.command('start');solo.wait('!!document.querySelector("[data-contra-runtime]")?.dataset.digest')
  solo.js('document.querySelector("[data-contra-runtime]").focus({preventScroll:true});true')
  if solo.data()['phase']=='intro':solo.native({'keys':['Space'],'frames':3})
  solo.wait('document.querySelector("[data-contra-runtime]")?.dataset.phase==="playing"')
  before_solo=solo.sync();moved=solo.native({'keys':['KeyD','KeyJ'],'frames':12});after_solo=solo.sync()
  assert after_solo['view']['snapshot']['players'][0]['x']!=before_solo['view']['snapshot']['players'][0]['x']
  assert len(solo.api('/api/state')['room']['seats'])==1
  assert solo.js('document.querySelectorAll("button.autoplay-control").length')==0
  solo.capture('solo-playing')
  report['solo']={'roomID':room['id'],'before':before_solo,'after':after_solo,'nativeInput':moved}
 # Actual network resources remain below the immutable prefix; all media is event-lazy.
 for browser in [desktop,mobile,observer]+([solo]if human_only else[]):
  requests=browser.js('performance.getEntriesByType("resource").map(x=>x.name)');game=[u for u in requests if '/contra/'in u]
  assert game and all('/contra/runtime-'+fixture['runtimeBuildID']+'/'in u for u in game),game
  assert not any('.nes'in u or '.fcs'in u or 'stage-2.json'in u for u in requests),requests
  report.setdefault('requests',{})[browser.name]=requests
  browser.data()
 if network:
  desktop.js('document.querySelector("[data-contra-runtime]").focus({preventScroll:true});true');desktop.native({'keys':['Space'],'frames':4,'cursor':'revision'});desktop.wait('document.querySelector("[data-contra-runtime]").dataset.paused==="false"')
  desktop.js('window.__contraSeeks=[];document.querySelectorAll("audio[data-platform-track]").forEach(m=>m.addEventListener("seeking",()=>window.__contraSeeks.push({at:performance.now(),key:m.dataset.platformTrack,position:m.currentTime})));true')
  start=desktop.data();network.stream.clear();time.sleep(.65);held=desktop.data();network.stream.set();desktop.wait('BigInt(document.querySelector("[data-contra-runtime]").dataset.revision)>BigInt('+json.dumps(start['revision'])+')+30n')
  desktop.run('wait','--fn','window.__contraSeeks.length>0');report['streamDelay']={'before':start,'held':held,'after':desktop.data(),'nativeSeeks':desktop.js('window.__contraSeeks')};assert int(held['revision'])-int(start['revision'])<12
  network.checkpoint.clear();network.waiting.clear();blank=desktop.run('tab','new','about:blank')['tabId'];desktop.run('tab','t1');assert network.waiting.wait(5),'explicit checkpoint request did not reach real host'
  desktop.run('tab',blank);network.checkpoint.set();time.sleep(.15)
  hidden=desktop.native({'observeOnly':True});assert hidden['hidden'],hidden
  desktop.run('tab','t1');desktop.wait('Number(document.querySelector("[data-contra-runtime]").dataset.resyncs)>='+str(int(start['resyncs'])+2))
  desktop.run('wait','--fn','[...document.querySelectorAll("audio[data-platform-track]")].some(m=>!m.paused&&m.currentTime>0)')
  report['hiddenCheckpoint']={'hidden':hidden,'visible':desktop.native({'observeOnly':True}),'server':desktop.sync()};desktop.run('tab','close',blank)
 report['passed']=True
except BaseException:
 report['error']=traceback.format_exc()
 for browser in browsers:
  try:browser.capture(browser.name+'-failure');report.setdefault('failure',{})[browser.name]={'snapshot':browser.run('snapshot','-i'),'errors':browser.run('errors'),'surface':browser.js('({...document.querySelector("[data-contra-runtime]")?.dataset})')}
  except BaseException:pass
finally:
 if network:network.stream.set();network.checkpoint.set()
 for browser in browsers:
  try:browser.run('close')
  except BaseException:report['passed']=False
  if browser.socket.exists():shutil.rmtree(browser.socket)
 if temp.exists():shutil.rmtree(temp)
 if network:network.close()
 report['scriptSHA256']=hashlib.sha256(script_bytes).hexdigest();report['inputSHA256']=hashlib.sha256(input_bytes).hexdigest();report['networkSHA256']=hashlib.sha256(network_bytes).hexdigest();report['build']=json.loads((Path(fixture.get('publicRoot',root/'.local/contra-browser/build/public')).parent/'build-source.json').read_text());(out/'result.json').write_text(json.dumps(report,ensure_ascii=False,indent=2)+'\n');print(json.dumps({'passed':report['passed'],'error':report.get('error'),'output':str(out)},ensure_ascii=False))
raise SystemExit(0 if report['passed']else 1)
