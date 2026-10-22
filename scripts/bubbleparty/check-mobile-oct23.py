#!/usr/bin/env python3
"""Production Bubbleparty: floating touch origin, two independent thumbs,
cancellation, real movement/bombs, and unchanged native PC keyboard input.
No fabricated game state or synthetic DOM pointer events.
"""
import argparse,hashlib,http.cookiejar,json,os,shutil,subprocess,time,urllib.request,uuid
from pathlib import Path
root=Path(__file__).resolve().parents[2]
p=argparse.ArgumentParser();p.add_argument('--out',default='.local/optimization-20261023/mobile');args=p.parse_args()
out=(root/args.out).resolve();assert out.is_relative_to(root) and out!=root;out.mkdir(parents=True,exist_ok=True)
temp=out/'temporary';sockets=root/'.s'/('bm'+uuid.uuid4().hex[:5]);env=dict(os.environ)
for k,v in {'TMPDIR':temp/'tmp','GOTMPDIR':temp/'tmp','AGENT_BROWSER_PROFILE':temp/'profile','AGENT_BROWSER_SOCKET_DIR':sockets,'GOCACHE':root/'.local/go-build','GOMODCACHE':root/'.local/go-mod'}.items():v.mkdir(parents=True,exist_ok=True);env[k]=str(v)
env.update(AGENT_BROWSER_SESSION='bm',AGENT_BROWSER_ARGS='--mute-audio,--disable-breakpad,--disable-crash-reporter',WIRE_BOARD_BUBBLEPARTY_BROWSER='1',WIRE_BOARD_BUBBLEPARTY_BROWSER_OUT=str(out),GOTOOLCHAIN='local')
report={'passed':False,'checks':[],'scope':__doc__,'distSHA256':hashlib.sha256((root/'web/dist/index.html').read_bytes()).hexdigest()};opened=False;server=log=None
CDP=r'''const [endpoint,base,raw]=process.argv.slice(2),spec=JSON.parse(raw),ws=new WebSocket(endpoint);await new Promise((r,j)=>{ws.onopen=r;ws.onerror=j});let seq=0;const pending=new Map();ws.onmessage=({data})=>{const m=JSON.parse(data),p=pending.get(m.id);if(!p)return;pending.delete(m.id);m.error?p.j(Error(JSON.stringify(m.error))):p.r(m.result)};const call=(method,params={},sessionId)=>new Promise((r,j)=>{const id=++seq;pending.set(id,{r,j});ws.send(JSON.stringify({id,method,params,sessionId}))});let sid;
try{const {targetInfos}=await call('Target.getTargets');const t=targetInfos.find(t=>t.type==='page'&&t.url.startsWith(base));({sessionId:sid}=await call('Target.attachToTarget',{targetId:t.targetId,flatten:true}));
const evaluate=async expression=>{const r=await call('Runtime.evaluate',{expression,returnByValue:true,awaitPromise:true},sid);if(r.exceptionDetails)throw Error(JSON.stringify(r.exceptionDetails));return r.result.value};
const delay=ms=>new Promise(r=>setTimeout(r,ms));const state=()=>evaluate('(async()=>{const r=(await(await fetch("/api/state")).json()).room,d=r.game.data,c=document.querySelector("[data-bubbleparty-canvas]");return {player:d.players[r.you],bombs:d.bombs,events:d.events,renderAt:+c.dataset.renderAt,active:document.querySelector(".bubble-joystick").dataset.active}})()');
const samples=[];await evaluate(`window.__pointers=[];for(const type of ['pointerdown','pointerup','pointercancel','lostpointercapture'])document.addEventListener(type,e=>window.__pointers.push({type,id:e.pointerId,target:e.target.className}),true)`);const before=await state();
await evaluate('window.__clockFrames=[];window.__record=true;function sampleClock(){if(!window.__record)return;window.__clockFrames.push(+document.querySelector("[data-bubbleparty-canvas]").dataset.renderAt);requestAnimationFrame(sampleClock)};requestAnimationFrame(sampleClock)');
if(spec.keyboard){const key=spec.direction==='right'?'ArrowRight':spec.direction==='left'?'ArrowLeft':spec.direction==='up'?'ArrowUp':'ArrowDown';await call('Input.dispatchKeyEvent',{type:'keyDown',key,code:key},sid);await delay(180);samples.push(await state());await call('Input.dispatchKeyEvent',{type:'keyUp',key,code:key},sid);await delay(220);samples.push(await state());}
else{await call('Emulation.setTouchEmulationEnabled',{enabled:true,maxTouchPoints:5},sid);const rects=await evaluate('(()=>{const selectors=[".bubble-joystick","[data-bubbleparty-bomb]"];return selectors.map(s=>document.querySelector(s).getBoundingClientRect().toJSON())})()'),[pad,bomb]=rects;
const vector={right:[1,0],left:[-1,0],up:[0,-1],down:[0,1]}[spec.direction];const first={id:1,x:pad.x+pad.width*.75,y:pad.y+pad.height*.35,radiusX:5,radiusY:5,force:1};const moved={...first,x:first.x+vector[0]*35,y:first.y+vector[1]*35};const second={id:2,x:bomb.x+bomb.width/2,y:bomb.y+bomb.height/2,radiusX:5,radiusY:5,force:1};
await call('Input.dispatchTouchEvent',{type:'touchStart',touchPoints:[first]},sid);await delay(120);samples.push(await state());
await call('Input.dispatchTouchEvent',{type:'touchMove',touchPoints:[moved]},sid);await delay(180);samples.push(await state());
await call('Input.dispatchTouchEvent',{type:'touchCancel',touchPoints:[]},sid);await delay(160);samples.push(await state());
await call('Input.dispatchTouchEvent',{type:'touchStart',touchPoints:[first]},sid);await call('Input.dispatchTouchEvent',{type:'touchMove',touchPoints:[moved]},sid);await delay(80);
await call('Input.dispatchTouchEvent',{type:'touchStart',touchPoints:[moved,second]},sid);await delay(50);await call('Input.dispatchTouchEvent',{type:'touchEnd',touchPoints:[second]},sid);await delay(100);samples.push(await state());
await call('Input.dispatchTouchEvent',{type:'touchEnd',touchPoints:[]},sid);await delay(180);samples.push(await state());await call('Emulation.setTouchEmulationEnabled',{enabled:false},sid);}
const frames=await evaluate('window.__record=false;window.__clockFrames');const pointers=await evaluate('window.__pointers');console.log(JSON.stringify({before,samples,frames,pointers}));
}finally{if(sid)await call('Target.detachFromTarget',{sessionId:sid});ws.close()}'''
def b(*a):
 p=subprocess.run(['agent-browser',*a,'--json'],cwd=root,env=env,capture_output=True,text=True,timeout=45);assert p.returncode==0,p.stdout+p.stderr;r=json.loads(p.stdout);assert r['success'],r;return r['data']
def js(s):return b('eval',s)['result']
def api(path,body=None):
 opts='' if body is None else ',{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify('+json.dumps(body)+')}'
 r=js('(async()=>{const r=await fetch('+json.dumps('/api'+path)+opts+');return {status:r.status,data:await r.json()}})()');assert r['status'] in (200,201),r;return r['data']
def request(c,path,body=None):
 with c.open(urllib.request.Request(base+'/api'+path,data=None if body is None else json.dumps(body).encode(),headers={'Content-Type':'application/json'}),timeout=20) as r:return json.load(r)
def cmd(kind):
 r=api('/state')['room'];return api('/rooms/'+r['id'],dict(type=kind,version=r['version'],nonce=uuid.uuid4().hex))
def action(req,a):
 r=req('/state')['room'];return req('/rooms/'+r['id'],dict(type='action',version=r['version'],nonce=uuid.uuid4().hex,action=a))
def wait(fn,seconds=20):
 end=time.monotonic()+seconds
 while time.monotonic()<end:
  if fn():return
  time.sleep(.08)
 raise AssertionError('condition timed out')
try:
 for name in ['url','stop']:(out/name).unlink(missing_ok=True)
 log=(out/'server.log').open('w');server=subprocess.Popen(['go','test','-count=1','-timeout=12m','-run','^TestBubblepartyBrowserFixture$','./games/bubbleparty/tests'],cwd=root,env=env,stdout=log,stderr=subprocess.STDOUT);wait(lambda:(out/'url').exists(),100);base=(out/'url').read_text();b('open',base);opened=True
 api('/login',{'name':'泡泡验收管理员','password':'bubbleparty-browser-test-only'})
 for width in [320,390,1280]:
  b('set','viewport',str(width),'640' if width<600 else '1000')
  room=api('/rooms',{'kind':'bubbleparty','name':'双拇指操作验收','capacity':2,'options':{'map':'community10','roundSeconds':300}})
  c=urllib.request.build_opener(urllib.request.HTTPCookieProcessor(http.cookiejar.CookieJar()));peer=lambda path,body=None:request(c,path,body)
  peer('/register',{'name':'触控对手'+str(width),'password':'bubbleparty-browser-test-only','invite':'bubbleparty-browser'});peer('/rooms/'+room['id'],dict(type='join',version=api('/state')['room']['version'],nonce=uuid.uuid4().hex));r=peer('/state')['room'];peer('/rooms/'+room['id'],dict(type='ready',version=r['version'],nonce=uuid.uuid4().hex));cmd('ready');cmd('start')
  for req in [api,peer]:
   r=req('/state')['room'];action(req,{'type':'character','epoch':r['game']['data']['epoch'],'character':0,'confirm':True})
  b('reload');b('wait','[data-bubbleparty-canvas]');wait(lambda:api('/state')['room']['game']['data']['phase']=='playing')
  js('window.scrollTo({top:document.querySelector(".bubble-board").getBoundingClientRect().top+scrollY-8,behavior:"instant"})');js('new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r)))')
  layout=js('(()=>{const sels=["[data-bubbleparty-canvas]",".bubble-joystick","[data-bubbleparty-bomb]"];return sels.map(s=>{const e=document.querySelector(s),r=e.getBoundingClientRect(),h=document.elementFromPoint(r.x+r.width/2,r.y+r.height/2);return {selector:s,rect:r.toJSON(),visible:r.y>=0&&r.bottom<=innerHeight,hit:h===e||e.contains(h)}})})()')
  if width<600:assert all(x['visible'] and x['hit'] for x in layout),layout
  view=api('/state')['room']['game']['data'];p=view['players'][0]
  direction=next(name for name,dx,dy in [('right',1,0),('left',-1,0),('down',0,1),('up',0,-1)] if 0<=p['x']+dx<15 and 0<=p['y']+dy<13 and view['cells'][(p['y']+dy)*15+p['x']+dx]==0)
  native=subprocess.run(['node','--input-type=module','-',b('get','cdp-url')['cdpUrl'],base,json.dumps({'direction':direction,'keyboard':width>=600})],input=CDP,cwd=root,env=env,capture_output=True,text=True,timeout=25);assert native.returncode==0,native.stdout+native.stderr;trace=json.loads(native.stdout);(out/f'{width}-input.json').write_text(json.dumps(trace,indent=2));samples=trace['samples'];start=trace['before']['player']
  def dist(p):return abs(p['px']-start['px'])+abs(p['py']-start['py'])
  if width<600:
   assert dist(samples[0]['player'])<.03,'off-center touch moved before dragging'
   assert dist(samples[1]['player'])>.1,'touch drag did not move'
   assert samples[2]['player']['applied']['move']=='none' and samples[2]['active']=='false','cancel did not release'
   assert samples[3]['player']['applied']['move']==direction,'bomb finger interrupted movement'
   previous=max((e['id'] for e in trace['before']['events']),default=0);places=[e for e in samples[-1]['events'] if e['id']>previous and e['type']=='place' and e['seat']==0];assert len(places)==1,places
  else:assert dist(samples[0]['player'])>.1,'PC keyboard stopped working'
  assert samples[-1]['player']['applied']['move']=='none','release kept moving'
  assert len(trace['frames'])>10 and all(b>=a for a,b in zip(trace['frames'],trace['frames'][1:])),'render clock moved backwards'
  (out/f'{width}-input.json').write_text(json.dumps(trace,indent=2));b('screenshot',str(out/f'{width}.png'),'--full');report['checks'].append({'width':width,'layout':layout,'inputTrace':f'{width}-input.json','passed':True});print('PASS',width,flush=True)
  cmd('close');cmd('leave')
 report['passed']=True
finally:
 if opened and not report['passed']:
  try:b('screenshot',str(out/'failure.png'),'--full');report['state']=api('/state')
  except:pass
 if opened:
  try:b('close')
  except:pass
 if server:
  (out/'stop').write_text('stop')
  try:server.wait(timeout=15)
  except subprocess.TimeoutExpired:server.terminate();server.wait(timeout=10)
 if log:log.close()
 for folder in [temp,sockets,out/'data']:shutil.rmtree(folder,ignore_errors=True)
 (out/'result.json').write_text(json.dumps(report,ensure_ascii=False,indent=2)+'\n');print('Artifact',out/'result.json')
