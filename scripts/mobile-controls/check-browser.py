#!/usr/bin/env python3
"""Native touch acceptance; inventory: docs/research/mobile-controls/PLAN.md."""
import argparse, base64, hashlib, http.cookiejar, json, os, shutil, subprocess, time, traceback, urllib.request, uuid
from pathlib import Path
parser=argparse.ArgumentParser();parser.add_argument('--out',required=True);args=parser.parse_args()
root=Path(__file__).resolve().parents[2];out=(root/args.out).resolve();assert out.is_relative_to(root) and not out.exists();out.mkdir(parents=True)
run='touch'+uuid.uuid4().hex[:6];temp=out/'temporary';sockets=root/'.s'/run;env=dict(os.environ)
for k,v in {'TMPDIR':temp/'tmp','GOTMPDIR':temp/'tmp','GOCACHE':root/'.local/go-build','GOMODCACHE':root/'.local/go-mod','npm_config_cache':root/'.local/npm','AGENT_BROWSER_SOCKET_DIR':sockets,'AGENT_BROWSER_PROFILE':temp/'profile'}.items():v.mkdir(parents=True,exist_ok=True);env[k]=str(v)
env.update(AGENT_BROWSER_SESSION=run,AGENT_BROWSER_ARGS='--mute-audio,--disable-breakpad,--disable-crash-reporter',WIRE_BOARD_BUBBLEPARTY_BROWSER='1',WIRE_BOARD_BUBBLEPARTY_BROWSER_OUT=str(out),GOTOOLCHAIN='local')
report={'passed':False,'games':{},'failures':[],'scope':'Native Chromium touch; production HTTP/SQLite/clock. WebKit declarations inspected separately; not a physical iPhone test.','sources':{str(f):hashlib.sha256(f.read_bytes()).hexdigest() for f in [root/'web/src/style.css',root/'web/dist/index.html']}}
server=log=None;opened=False

def browser(*a):
 r=subprocess.run(['agent-browser',*a,'--json'],cwd=root,env=env,text=True,capture_output=True,timeout=45);assert r.returncode==0,r.stdout+r.stderr
 j=json.loads(r.stdout);assert j['success'],j;return j['data']
def js(code):return browser('eval',code)['result']
def api(path,body=None):
 opts='' if body is None else ',{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify('+json.dumps(body)+')}'
 r=js('(async()=>{const r=await fetch('+json.dumps('/api'+path)+opts+');return {status:r.status,data:await r.json()}})()');assert r['status'] in (200,201),r;return r['data']
def room():return api('/state').get('room')
def cmd(kind,**extra):
 r=room();return api('/rooms/'+r['id'],dict(type=kind,version=r['version'],nonce=uuid.uuid4().hex,**extra))
def wait(fn,seconds=20):
 end=time.monotonic()+seconds
 while time.monotonic()<end:
  if value:=fn():return value
  time.sleep(.1)
 raise AssertionError('Timed out')
def save(name,data):(out/(name+'.json')).write_text(json.dumps(data,ensure_ascii=False,indent=2)+'\n')

def verify_release(trace,kind):
 if kind=='fortressblue':return
 action=trace['inputs'][-1]['action']
 if kind=='bubbleparty':assert action['move']=='none' and not action['bomb'],action
 elif kind=='battlecity':assert action['move']=='none' and not action['fire'],action
 elif kind=='racing':assert action['steer']==0 and not action['brake'] and not action['drift'],action
 else:assert action['events'][-1]['kind']=='rotateCW',action

CDP=r'''
const [endpoint,prefix,raw]=process.argv.slice(2),o=JSON.parse(raw),ws=new WebSocket(endpoint);
await new Promise((r,j)=>{ws.onopen=r;ws.onerror=j});let next=0,sessionId;const pending=new Map(),inputs=[],acks=[];
ws.onmessage=({data})=>{const m=JSON.parse(data);if(m.method?.startsWith('Network.webSocketFrame')){try{const v=JSON.parse(m.params.response.payloadData);if(v.type==='input')inputs.push(v);if(v.type==='ack')acks.push(v)}catch{}}if(!m.id)return;const p=pending.get(m.id);pending.delete(m.id);m.error?p.j(Error(JSON.stringify(m.error))):p.r(m.result)};
const call=(method,params={},sessionId)=>new Promise((r,j)=>{const id=++next;pending.set(id,{r,j});ws.send(JSON.stringify({id,method,params,sessionId}))});
try{
 const {targetInfos}=await call('Target.getTargets');const target=targetInfos.find(t=>t.type==='page'&&t.url.startsWith(prefix));if(!target)throw Error('owned target missing');
 ({sessionId}=await call('Target.attachToTarget',{targetId:target.targetId,flatten:true}));
 const send=(m,p)=>call(m,p,sessionId),evaluate=async expression=>(await send('Runtime.evaluate',{expression,returnByValue:true,awaitPromise:true})).result.value;
 await send('Network.enable',{});await send('Emulation.setDeviceMetricsOverride',{width:o.width,height:844,deviceScaleFactor:1,mobile:true});await send('Emulation.setTouchEmulationEnabled',{enabled:true,maxTouchPoints:5});
 const points=await evaluate(`(()=>{const group=document.querySelector(${JSON.stringify(o.group)});group.scrollIntoView({block:'center'});getSelection().removeAllRanges();window.__touchAudit={events:[],selections:[]};for(const name of ['pointerup','pointercancel','contextmenu','selectstart'])group.addEventListener(name,e=>window.__touchAudit.events.push({type:e.type,text:e.target.textContent,prevented:e.defaultPrevented}));document.addEventListener('selectionchange',()=>{const s=getSelection().toString();if(s)window.__touchAudit.selections.push(s)});return ${JSON.stringify(o.keys)}.map(selector=>{const e=document.querySelector(selector);if(!e||e.disabled)throw Error('missing/disabled '+selector);const r=e.getBoundingClientRect();return {x:r.x+r.width*(e.hasAttribute('data-bubbleparty-dpad')?.8:.5),y:r.y+r.height*.5,radiusX:3,radiusY:3,force:1}})})()`);
 if(!points)throw Error('touch target missing');const point=(i,id)=>({...points[i],id});const pause=ms=>new Promise(r=>setTimeout(r,ms));const touch=(type,touchPoints)=>send('Input.dispatchTouchEvent',{type,touchPoints});
 await touch('touchStart',[point(0,1)]);await pause(1200);const held=await evaluate('({selected:getSelection().toString(),scrollY,viewport:innerWidth})');
 await touch('touchMove',[{...point(0,1),x:points[0].x+18}]);await pause(100);await touch('touchEnd',[]);await pause(250);
 // Browser interruption must release held actions, including charge without firing.
 await touch('touchStart',[point(0,1)]);await pause(100);await touch('touchCancel',[]);await pause(250);
 if(o.multi){await touch('touchStart',[point(0,1)]);await pause(100);await touch('touchStart',[point(0,1),point(1,2)]);await pause(120);await touch('touchEnd',[point(0,1)]);await pause(100);await touch('touchEnd',[])}
 else{await touch('touchStart',[point(1,2)]);await pause(150);await touch(o.cancelSecondary?'touchCancel':'touchEnd',[])}
 await pause(650);
 const audit=await evaluate('({...window.__touchAudit,selected:getSelection().toString(),viewport:innerWidth,scrollY})');
 const {data}=await send('Page.captureScreenshot',{format:'png',captureBeyondViewport:false});console.log(JSON.stringify({held,audit,inputs,acks,screenshot:data}));
}finally{if(sessionId)await call('Target.detachFromTarget',{sessionId});ws.close()}
'''
CONFIG={
 'bubbleparty':('.bubble-controls',['[data-bubbleparty-dpad]','[data-bubbleparty-bomb]'],True),
 'battlecity':('.bc-touch-controls',['.bc-dpad','[data-battlecity-fire]'],True),
 'racing':('.racing-controls',['.racing-wheel','button[aria-label="刹车"]'],True),
 'tetris':('.tetris-controls',['button[aria-label="向左移动"]','button[aria-label="顺时针旋转"]'],False),
 'spf2t':('.spf2t-controls',['button[aria-label="左移"]','button[aria-label="顺时针"]'],False),
 'fortressblue':('.fb-control-row',['[data-fortressblue-move="-1"]','[data-fortressblue-fire]'],False),
}
try:
 log=(out/'server.log').open('w');server=subprocess.Popen(['go','test','-count=1','-timeout=12m','-run','^TestBubblepartyBrowserFixture$','./games/bubbleparty/tests'],cwd=root,env=env,stdout=log,stderr=subprocess.STDOUT)
 wait(lambda:(out/'url').exists(),120);base=(out/'url').read_text();browser('open',base);opened=True
 api('/login',{'name':'泡泡验收管理员','password':'bubbleparty-browser-test-only'});browser('reload');browser('wait','.room-list')
 opponent=urllib.request.build_opener(urllib.request.HTTPCookieProcessor(http.cookiejar.CookieJar()))
 def other(path,body=None):
  request=urllib.request.Request(base+'/api'+path,data=None if body is None else json.dumps(body).encode(),headers={'Content-Type':'application/json'})
  with opponent.open(request,timeout=10) as response:return json.load(response)
 other('/register',{'name':'触屏按键验收对手','password':'bubbleparty-browser-test-only','invite':'bubbleparty-browser'})
 for kind,(group,keys,multi) in CONFIG.items():
  browser('set','viewport','390','844');api('/rooms',{'kind':kind,'name':kind+' 手机长按验收','capacity':2,'options':{}})
  r=room();other('/rooms/'+r['id'],{'type':'join','version':r['version'],'nonce':uuid.uuid4().hex});r=other('/state')['room'];other('/rooms/'+r['id'],{'type':'ready','version':r['version'],'nonce':uuid.uuid4().hex});cmd('ready');cmd('start');browser('wait','.game-layout');time.sleep(3.5)
  if kind in ('battlecity','spf2t','fortressblue'):
   attr={'battlecity':'data-battlecity-tank','spf2t':'data-character','fortressblue':'data-fortressblue-tank'}[kind];value=js(f'document.querySelector("[{attr}]").getAttribute("{attr}")');browser('click',f'[{attr}="{value}"]');r=other('/state')['room'];action={'type':'select','epoch':r['game']['data']['epoch'],('character' if kind=='spf2t' else 'tank'):int(value) if kind=='fortressblue' else value};other('/rooms/'+r['id'],{'type':'action','version':r['version'],'nonce':uuid.uuid4().hex,'action':action})
  wait(lambda:room()['game']['data']['phase'] in ('playing','racing','finishing'));wait(lambda:js('!!document.querySelector('+json.dumps(keys[0])+')&&!document.querySelector('+json.dumps(keys[0])+').disabled'))
  entry={'widths':{}};report['games'][kind]=entry
  for width in [320,390]:
   browser('set','viewport',str(width),'844')
   options={'group':group,'keys':keys,'multi':multi,'width':width,'cancelSecondary':kind=='fortressblue'}
   result=subprocess.run(['node','--input-type=module','-',browser('get','cdp-url')['cdpUrl'],base,json.dumps(options)],input=CDP,cwd=root,env=env,text=True,capture_output=True,timeout=30);assert result.returncode==0,result.stdout+result.stderr
   trace=json.loads(result.stdout);(out/f'{kind}-{width}.png').write_bytes(base64.b64decode(trace.pop('screenshot')));save(f'{kind}-{width}-touch',trace)
   if width==320:verify_release(trace,kind)  # Later samples may legitimately become trapped/dead after the earlier bomb.
   style=js('(()=>{const group=document.querySelector('+json.dumps(group)+');return [group,...document.querySelectorAll("button,[role=button]"),...group.querySelectorAll("small,span")].filter(e=>e.getClientRects().length).map(e=>({tag:e.tagName,text:e.textContent?.trim().slice(0,50),userSelect:getComputedStyle(e).userSelect,webkitUserSelect:getComputedStyle(e).webkitUserSelect,touchAction:getComputedStyle(e).touchAction}))})()')
   unprotected=[s for s in style if s['userSelect']!='none' or s['webkitUserSelect']!='none'];entry['widths'][width]={'styles':style,'selected':trace['audit']['selected'],'selectionEvents':trace['audit']['selections'],'unprotected':unprotected,'inputs':len(trace['inputs']),'acks':len(trace['acks'])}
   if unprotected:report['failures'].append(f'{kind}/{width}: unprotected text {len(unprotected)}')
   if trace['held']['selected'] or trace['audit']['selected'] or trace['audit']['selections']:report['failures'].append(f'{kind}/{width}: native touch selected text')
   assert trace['audit']['viewport']==width
   if kind!='fortressblue':assert trace['inputs'] and any(a['status']==200 for a in trace['acks']),trace
   assert any(e['type']=='pointercancel' for e in trace['audit']['events']),trace['audit']
   print('CHECK',kind,width,'unprotected',len(unprotected),'selected',repr(trace['audit']['selected']),flush=True)
  browser('set','viewport','1280','1000');browser('screenshot',str(out/f'{kind}-desktop.png'),'--full')
  assert js('getComputedStyle(document.querySelector("h1")).userSelect!=="none"'), 'ordinary heading cannot be selected'
  # The existing chat textarea stays editable/selectable under shared button styles.
  browser('click','.chat-toggle');browser('fill','textarea','可复制的聊天输入');browser('focus','textarea');browser('press','Shift+ArrowLeft');selection=js('(()=>{const e=document.querySelector("textarea");return {start:e.selectionStart,end:e.selectionEnd,text:e.value,select:getComputedStyle(e).userSelect}})()');assert selection['start']<selection['end']==len(selection['text']) and selection['select']!='none',selection;entry['chatSelection']=selection
  browser('click','.chat-toggle');cmd('close');cmd('leave');r=other('/state').get('room')
  if r:other('/rooms/'+r['id'],{'type':'leave','version':r['version'],'nonce':uuid.uuid4().hex})
 report['passed']=not report['failures']
except BaseException:
 report['error']=traceback.format_exc();print(report['error'],flush=True)
finally:
 if opened:
  try:browser('close')
  except BaseException:report['passed']=False
 if server:
  (out/'stop').write_text('stop')
  try:server.wait(timeout=20)
  except subprocess.TimeoutExpired:server.kill();server.wait();report['passed']=False
  if server.returncode!=0:report['passed']=False
 if log:log.close()
 for path in [temp,sockets,out/'data']:
  if path.exists():shutil.rmtree(path)
 for name in ['url','stop']:(out/name).unlink(missing_ok=True)
 save('report',report);print(out/'report.json',flush=True)
 if not report['passed']:raise SystemExit(1)
