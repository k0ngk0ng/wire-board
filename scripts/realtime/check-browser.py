#!/usr/bin/env python3
"""Real browser transport E2E; failure inventory: docs/research/realtime-efficiency/PLAN.md.
Uses the existing production-registry browser fixture; native input and passive CDP network reads.
"""
import argparse, base64, hashlib, http.cookiejar, json, os, shutil, subprocess, time, traceback, urllib.request, uuid
from pathlib import Path
p=argparse.ArgumentParser();p.add_argument('--out',default='');p.add_argument('--games',default='bubbleparty,racing,battlecity,tetris,spf2t');p.add_argument('--idle-seconds',type=int,default=65);p.add_argument('--rtt-ms',type=int,default=0);p.add_argument('--prediction',action='store_true');p.add_argument('--reconnect',action='store_true');args=p.parse_args()
root=Path(__file__).resolve().parents[2];run='rt'+uuid.uuid4().hex[:6]
out=(root/(args.out or '.local/realtime-efficiency/browser-'+run)).resolve()
assert out.is_relative_to(root) and not out.exists();out.mkdir(parents=True)
temp=out/'temporary';sockets=root/'.local/rt-sockets'/run
env=dict(os.environ)
for k,v in {'TMPDIR':temp/'tmp','GOTMPDIR':temp/'tmp','GOCACHE':root/'.local/go-build','GOMODCACHE':root/'.local/go-mod','npm_config_cache':root/'.local/npm','AGENT_BROWSER_SOCKET_DIR':sockets,'AGENT_BROWSER_PROFILE':temp/'profile'}.items():v.mkdir(parents=True,exist_ok=True);env[k]=str(v)
env.update(AGENT_BROWSER_SESSION=run,AGENT_BROWSER_ARGS='--mute-audio,--disable-breakpad,--disable-crash-reporter',WIRE_BOARD_BUBBLEPARTY_BROWSER='1',WIRE_BOARD_BUBBLEPARTY_BROWSER_OUT=str(out),GOTOOLCHAIN='local',WIRE_BOARD_REALTIME_RTT_MS=str(args.rtt_ms),WIRE_BOARD_REALTIME_NETWORK_CONTROL='1' if args.reconnect else '0')
report={'passed':False,'checks':[],'screenshots':[],'rttMS':args.rtt_ms,'scope':'production registry/HTTP/SQLite/clock; native browser input and passive network capture; optional ordered TCP delay'}
report['sources']={str(f.relative_to(root)):hashlib.sha256(f.read_bytes()).hexdigest() for folder,pattern in [('internal/server','*.go'),('internal/gameapi','*.go'),('internal/nettest','*.go'),('web/src','**/*.ts*')]+[(f'games/{g}/web','**/*') for g in args.games.split(',')]+[(f'games/{g}/server','*.go') for g in args.games.split(',')] for f in (root/folder).glob(pattern) if f.is_file() and not f.name.endswith('_test.go')}
report['fixtureSHA256']=hashlib.sha256((root/'games/bubbleparty/tests/browser_fixture_test.go').read_bytes()).hexdigest()
report['distIndexSHA256']=hashlib.sha256((root/'web/dist/index.html').read_bytes()).hexdigest()
server=log=None;opened=False

def browser(*a):
 r=subprocess.run(['agent-browser',*a,'--json'],cwd=root,env=env,text=True,capture_output=True,timeout=45)
 assert r.returncode==0,r.stdout+r.stderr
 d=json.loads(r.stdout);assert d.get('success'),d;return d['data']
def js(code):return browser('eval',code)['result']
def api(path,body=None):
 opts='' if body is None else ',{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify('+json.dumps(body)+')}'
 r=js('(async()=>{const r=await fetch('+json.dumps('/api'+path)+opts+');return {status:r.status,data:await r.json()}})()')
 assert r['status'] in (200,201),r;return r['data']
def room():return api('/state').get('room')
def cmd(kind,**extra):
 r=room();return api('/rooms/'+r['id'],dict(type=kind,version=r['version'],nonce=uuid.uuid4().hex,**extra))
def wait(fn,seconds=20):
 end=time.monotonic()+seconds
 while time.monotonic()<end:
  value=fn()
  if value:return value
  time.sleep(.1)
 raise AssertionError('Timed out')
def record(name,data):(out/(name+'.json')).write_text(json.dumps(data,ensure_ascii=False,indent=2)+'\n')
def check(name,**data):report['checks'].append(dict(name=name,**data));print('PASS',name,flush=True)
def shot(name):
 browser('screenshot',str(out/(name+'.png')),'--full');report['screenshots'].append(str((out/(name+'.png')).relative_to(root)))

CDP=r'''
const [endpoint,prefix,raw]=process.argv.slice(2),options=JSON.parse(raw),ws=new WebSocket(endpoint);
await new Promise((r,j)=>{ws.onopen=r;ws.onerror=j});
let id=0,sessionId;const waiting=new Map(),events=[];
ws.onmessage=({data})=>{const m=JSON.parse(data);if(m.method?.startsWith('Network.')){if(['Network.requestWillBeSent','Network.webSocketFrameSent','Network.webSocketFrameReceived','Network.webSocketClosed'].includes(m.method))events.push({method:m.method,params:m.params,observedAt:Date.now()})}if(!m.id)return;const p=waiting.get(m.id);waiting.delete(m.id);m.error?p.j(Error(JSON.stringify(m.error))):p.r(m.result)};
const call=(method,params={},sessionId)=>new Promise((r,j)=>{const n=++id;waiting.set(n,{r,j});ws.send(JSON.stringify({id:n,method,params,sessionId}))});
try {
 const {targetInfos}=await call('Target.getTargets');const page=targetInfos.find(t=>t.type==='page'&&t.url.startsWith(prefix));if(!page)throw Error('owned target missing');
 ({sessionId}=await call('Target.attachToTarget',{targetId:page.targetId,flatten:true}));await call('Network.enable',{},sessionId);
 if(options.prediction)await call('Emulation.setDeviceMetricsOverride',{width:options.width,height:options.height,deviceScaleFactor:1,mobile:false},sessionId);
 const key=options.key;
 const readPose=async()=>{const r=await call('Runtime.evaluate',{expression:`(()=>{const e=document.querySelector('[data-bubbleparty-canvas],[data-battlecity-canvas]');return e?{viewport:innerWidth,x:Number(e.dataset.renderX),y:Number(e.dataset.renderY),serverX:Number(e.dataset.serverX),serverY:Number(e.dataset.serverY)}:null})()`,returnByValue:true},sessionId);return r.result.value};
 const poses=[],previews=[];let reconnect;
 const capture=async()=>{const {cssContentSize:c}=await call('Page.getLayoutMetrics',{},sessionId);return (await call('Page.captureScreenshot',{format:'png',captureBeyondViewport:true,clip:{x:0,y:0,width:c.width,height:c.height,scale:1}},sessionId)).data};
 if(options.prediction&&key){previews.push(await capture());poses.push({at:Date.now(),pose:await readPose()})}
 if(options.reconnect&&key){
  const code={ArrowRight:39,ArrowLeft:37,ArrowUp:38,ArrowDown:40}[key];
  const keyEvent=type=>call('Input.dispatchKeyEvent',{type,key,code:key,windowsVirtualKeyCode:code,nativeVirtualKeyCode:code},sessionId);
  await keyEvent('keyDown');await new Promise(r=>setTimeout(r,650));
  const cutAt=Date.now();
  const fault=await call('Runtime.evaluate',{expression:`fetch('/_test/disconnect',{method:'POST'}).then(r=>r.json())`,awaitPromise:true,returnByValue:true},sessionId);
  await new Promise(r=>setTimeout(r,2000));
  const bootstrap=events.find(e=>e.method==='Network.webSocketFrameReceived'&&e.observedAt>cutAt&&(()=>{try{return JSON.parse(e.params.response.payloadData).type==='state'}catch{return false}})());
  const quietUntil=Date.now();await keyEvent('keyUp');await new Promise(r=>setTimeout(r,60));
  // Recovery is tested on a controllable piece, not across a legitimate
  // landing/resolution boundary that can reject a delayed movement packet.
  if(['tetris','spf2t'].includes(options.game)){
   const until=Date.now()+12000;let ready=false;
   while(Date.now()<until){
    const latest=[...events].reverse().find(e=>e.method==='Network.webSocketFrameReceived'&&(()=>{try{return JSON.parse(e.params.response.payloadData).type==='game'}catch{return false}})());
    if(latest){const d=JSON.parse(latest.params.response.payloadData).game.data,p=d.players[0];ready=d.phase==='playing'&&p.active&&p.active.y<=4&&!p.resolving&&!p.lost&&p.alive!==false;if(ready)break}
    await new Promise(r=>setTimeout(r,50));
   }
   if(!ready)throw Error('puzzle did not produce a controllable early piece');
  }
  const freshAt=Date.now();await keyEvent('keyDown');await new Promise(r=>setTimeout(r,180));await keyEvent('keyUp');
  reconnect={cutAt,closed:fault.result.value?.closed,bootstrapAt:bootstrap?.observedAt,quietUntil,freshAt};
 }else if(key){
  const code={ArrowRight:39,ArrowLeft:37,ArrowUp:38,ArrowDown:40}[key];
  let touch;
  if(options.touch){
   const r=await call('Runtime.evaluate',{expression:`(()=>{const e=document.querySelector('[data-bubbleparty-dpad],[data-battlecity-dpad]');e.scrollIntoView({block:'center'});const r=e.getBoundingClientRect();const d=${JSON.stringify(key)};return {x:r.x+r.width*(d==='ArrowLeft'?.2:d==='ArrowRight'?.8:.5),y:r.y+r.height*(d==='ArrowUp'?.2:d==='ArrowDown'?.8:.5)}})()`,returnByValue:true},sessionId);
   touch=r.result.value;
   await call('Input.dispatchTouchEvent',{type:'touchStart',touchPoints:[{...touch,id:1,radiusX:3,radiusY:3,force:1}]},sessionId);
  }else await call('Input.dispatchKeyEvent',{type:'keyDown',key,code:key,windowsVirtualKeyCode:code,nativeVirtualKeyCode:code},sessionId);
  await new Promise(r=>setTimeout(r,80));
  if(options.prediction){poses.push({at:Date.now(),pose:await readPose()});previews.push(await capture())}
  await new Promise(r=>setTimeout(r,options.prediction?40:160));
  if(options.touch)await call('Input.dispatchTouchEvent',{type:'touchEnd',touchPoints:[]},sessionId);
  else await call('Input.dispatchKeyEvent',{type:'keyUp',key,code:key,windowsVirtualKeyCode:code,nativeVirtualKeyCode:code},sessionId);
 }

 await new Promise(r=>setTimeout(r,options.milliseconds));
 const frames=events.filter(e=>e.method==='Network.webSocketFrameReceived').map(e=>{try{return {...JSON.parse(e.params.response.payloadData),observedAt:e.observedAt}}catch{return null}}).filter(Boolean);
 const inputs=events.filter(e=>e.method==='Network.webSocketFrameSent').map(e=>{try{return {...JSON.parse(e.params.response.payloadData),observedAt:e.observedAt}}catch{return null}}).filter(Boolean);
 const requests=events.filter(e=>e.method==='Network.requestWillBeSent').map(e=>({url:e.params.request.url,method:e.params.request.method}));
 console.log(JSON.stringify({frames,inputs,requests,poses,previews,reconnect}));
}finally{if(sessionId)await call('Target.detachFromTarget',{sessionId});ws.close()}
'''
def observe(milliseconds,key=None,touch=False,reconnect=False):
 viewport=js('({width:innerWidth,height:innerHeight})')
 r=subprocess.run(['node','--input-type=module','-',browser('get','cdp-url')['cdpUrl'],base,json.dumps(dict(milliseconds=milliseconds,key=key,prediction=args.prediction,touch=touch,reconnect=reconnect,game=kind,**viewport))],input=CDP,cwd=root,env=env,text=True,capture_output=True,timeout=milliseconds/1000+30)
 assert r.returncode==0,r.stdout+r.stderr
 if args.prediction:browser('set','viewport',str(viewport['width']),str(viewport['height']))
 return json.loads(r.stdout)

try:
 log=(out/'server.log').open('w');server=subprocess.Popen(['go','test','-count=1','-timeout=12m','-run','^TestBubblepartyBrowserFixture$','./games/bubbleparty/tests'],cwd=root,env=env,stdout=log,stderr=subprocess.STDOUT)
 deadline=time.monotonic()+120
 while not (out/'url').exists():
  assert server.poll() is None,'fixture exited';assert time.monotonic()<deadline,'fixture start timeout';time.sleep(.1)
 base=(out/'url').read_text();browser('open',base);opened=True
 api('/login',{'name':'泡泡验收管理员','password':'bubbleparty-browser-test-only'});browser('reload');browser('wait','.room-list')
 opponent=urllib.request.build_opener(urllib.request.HTTPCookieProcessor(http.cookiejar.CookieJar()))
 def other(path,body=None):
  req=urllib.request.Request(base+'/api'+path,data=None if body is None else json.dumps(body).encode(),headers={'Content-Type':'application/json'})
  with opponent.open(req,timeout=10) as res:return json.load(res)
 other('/register',{'name':'实时网络验收对手','password':'bubbleparty-browser-test-only','invite':'bubbleparty-browser'})
 for kind in args.games.split(','):
  browser('set','viewport','1280','1000')
  api('/rooms',{'kind':kind,'name':kind+' 实时通道验收','capacity':2,'options':{}})
  r=room();other('/rooms/'+r['id'],{'type':'join','version':r['version'],'nonce':uuid.uuid4().hex})
  r=other('/state')['room'];other('/rooms/'+r['id'],{'type':'ready','version':r['version'],'nonce':uuid.uuid4().hex})
  cmd('ready');cmd('start');browser('wait','.game-layout');time.sleep(3.5)
  if kind in ('battlecity','spf2t'):
   attr='data-battlecity-tank' if kind=='battlecity' else 'data-character'
   value=js('document.querySelector("['+attr+']").getAttribute("'+attr+'")')
   browser('click','['+attr+'="'+value+'"]')
   opponent_room=other('/state')['room']
   action={'type':'select','epoch':opponent_room['game']['data']['epoch'],('tank' if kind=='battlecity' else 'character'):value}
   other('/rooms/'+opponent_room['id'],{'type':'action','version':opponent_room['version'],'nonce':uuid.uuid4().hex,'action':action})
   wait(lambda:room()['game']['data']['phase']=='playing')
   time.sleep(.5)
  r=room();assert r.get('turnDeadline',0)==0,r
  clocks=js("[...document.querySelectorAll('.turn-card .turn-clock')].map(e=>({visibility:getComputedStyle(e).visibility,text:e.innerText}))")
  assert not any(c['visibility']=='visible' for c in clocks),clocks
  key="ArrowUp" if kind=="racing" else "ArrowRight"
  v=r["game"]["data"]
  entity_key={"racing":"cars","battlecity":"tanks"}.get(kind,"players")
  if kind=='bubbleparty':
   v=r['game']['data'];player=v['players'][0]
   for name,dx,dy in [('ArrowRight',1,0),('ArrowDown',0,1),('ArrowLeft',-1,0),('ArrowUp',0,-1)]:
    x,y=player['x']+dx,player['y']+dy
    if 0<=x<15 and 0<=y<13 and v['cells'][y*15+x]==0:key=name;break
   assert key,'no usable initial input direction'
  trace=observe(2500,key)
  for i,png in enumerate(trace.pop('previews',[])):
   path=out/f'{kind}-prediction-desktop-{i}.png';path.write_bytes(base64.b64decode(png));report['screenshots'].append(str(path.relative_to(root)))
  record(kind+'-network',trace)
  fetches=[r for r in trace['requests'] if '/api/state' in r['url']]
  posts=[r for r in trace['requests'] if r['method']=='POST' and '/api/rooms/' in r['url']]
  frames=[f for f in trace['frames'] if f.get('type')=='game']
  assert len(frames)>=10,(kind,len(frames));assert not fetches,(kind,fetches);assert not posts,(kind,posts)
  assert all('rooms' not in f and 'availableGames' not in f and f['roomId']==r['id'] for f in frames)
  if args.prediction:
   assert args.rtt_ms>=300 and kind in ('bubbleparty','battlecity')
   start,end=trace['poses'];a,b=start['pose'],end['pose']
   assert a is not None and b is not None and all(v is not None for v in [a['x'],a['y'],b['x'],b['y']]),trace['poses']
   delta=((b['x']-a['x'])**2+(b['y']-a['y'])**2)**.5
   authoritative=((b['serverX']-a['serverX'])**2+(b['serverY']-a['serverY'])**2)**.5
   assert delta>.08 and authoritative<.0001,trace['poses']
   check(kind+' locally drawn movement before authoritative movement',samples=trace['poses'])
  if key:
   inputs=[f for f in trace['inputs'] if f.get('type')=='input'];acks=[f for f in trace['frames'] if f.get('type')=='ack']
   assert len(inputs)>=2 and all(any(a['nonce']==i['nonce'] and a['status']==200 for a in acks) for i in inputs),(inputs,acks)
   assert frames[-1]['game']['data'][entity_key][0].get('seq',0)>v[entity_key][0].get('seq',0)
   delays=[next(a['observedAt'] for a in acks if a['nonce']==i['nonce'])-i['observedAt'] for i in inputs]
   assert all(isinstance(a.get('durable'),bool) for a in acks),acks
   check(kind+' authoritative input acknowledgements',rttMS=args.rtt_ms,ackLatencyMS=delays,durable=[a['durable'] for a in acks])
   if args.rtt_ms>=300 and kind in ('tetris','spf2t'):
    first_ack=next(a for a in acks if a['nonce']==inputs[0]['nonce'])
    assert len([i for i in inputs if i['observedAt']<first_ack['observedAt']])>=2,(inputs,acks)
    check(kind+' two ordered input batches before first ACK',rttMS=args.rtt_ms)
   if args.rtt_ms>=300 and kind in ('bubbleparty','battlecity','racing'):
    released=next(i for i in inputs if i['action'].get('move')=='none' or kind=='racing' and i['action'].get('throttle')==0)
    first_ack=next(a for a in acks if a['nonce']==inputs[0]['nonce'])
    assert released['observedAt']<first_ack['observedAt'],(released,first_ack)
    check(kind+' release is pipelined before first ACK',rttMS=args.rtt_ms,releaseAt=released['observedAt'],firstAckAt=first_ack['observedAt'])
  check(kind+' direct stream without full-state pulls',frames=len(frames),stateGETs=len(fetches),actionPOSTs=len(posts),inputPackets=len(trace['inputs']),clocks=clocks)
  shot(kind+'-desktop')
  browser('set','viewport','320','844');time.sleep(.3)
  assert js('document.documentElement.scrollWidth<=innerWidth+2')
  shot(kind+'-320')
  if args.prediction:
   reverse={'ArrowDown':'ArrowUp','ArrowUp':'ArrowDown','ArrowLeft':'ArrowRight','ArrowRight':'ArrowLeft'}[key]
   touch=observe(1800,reverse,True)
   for i,png in enumerate(touch.pop('previews',[])):
    path=out/f'{kind}-prediction-touch-{i}.png';path.write_bytes(base64.b64decode(png));report['screenshots'].append(str(path.relative_to(root)))
   record(kind+'-touch-network',touch)
   a,b=[v['pose'] for v in touch['poses']]
   assert a['viewport']==320 and b['viewport']==320,touch['poses']
   assert ((b['x']-a['x'])**2+(b['y']-a['y'])**2)**.5>.08,touch['poses']
   assert a['serverX']==b['serverX'] and a['serverY']==b['serverY'],touch['poses']
   check(kind+' 320px native touch predicts before authoritative movement',samples=touch['poses'])
  if kind=='bubbleparty' and args.idle_seconds:
   print('Measuring idle ownership for',args.idle_seconds,'seconds',flush=True)
   idle=observe(args.idle_seconds*1000);record('bubbleparty-idle-network',idle)
   r=room();assert not r['seats'][0].get('autoPlay') and r.get('turnDeadline',0)==0
   check('idle does not enable autoplay',seconds=args.idle_seconds,frames=len(idle['frames']),stateGETs=sum('/api/state' in q['url'] for q in idle['requests']))
  if args.reconnect:
   healed=observe(1800,key,reconnect=True);record(kind+'-reconnect',healed)
   info=healed['reconnect'];assert info['closed']>=1 and info.get('bootstrapAt'),info
   def active(i):
    a=i.get('action',{});return a.get('move','none')!='none' or a.get('fire',False) or a.get('bomb',False) or a.get('throttle',0)>0 or bool(a.get('events'))
   old=[i for i in healed['inputs'] if info['bootstrapAt']+300<i['observedAt']<info['quietUntil'] and active(i)]
   fresh=[i for i in healed['inputs'] if i['observedAt']>=info['freshAt'] and active(i)]
   assert not old,{'oldHeldInputs':old,'reconnect':info}
   assert fresh,{'missingFreshInput':info}
   assert all(any(a.get('type')=='ack' and a.get('nonce')==i['nonce'] and a.get('status')==200 for a in healed['frames']) for i in fresh),{'fresh':fresh,'acks':[a for a in healed['frames'] if a.get('type')=='ack'],'reconnect':info}
   check(kind+' actual socket drop clears held input; fresh key works after reconnect',**info)
  cmd('autoplay',enabled=True);wait(lambda:room()['seats'][0].get('autoPlay'))
  cmd('autoplay',enabled=False);wait(lambda:not room()['seats'][0].get('autoPlay'))
  check(kind+' manual autoplay and reclaim')
  cmd('close')
  if room():cmd('leave')
  r=other('/state').get('room')
  if r:other('/rooms/'+r['id'],{'type':'leave','version':r['version'],'nonce':uuid.uuid4().hex})
  wait(lambda:js('!!document.querySelector(".room-list")'))
 assert not browser('errors')['errors'],browser('errors')
 report['passed']=True
except BaseException:
 report['error']=traceback.format_exc();print(report['error'],flush=True)
 if opened:
  try:shot('failure')
  except BaseException:pass
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
 record('report',report);print(out/'report.json',flush=True)
 if not report['passed']:raise SystemExit(1)
