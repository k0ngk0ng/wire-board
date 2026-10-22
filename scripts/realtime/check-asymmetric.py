#!/usr/bin/env python3
"""Two real Bubbleparty humans: only one browser has 500ms API/WebSocket test-network RTT; assets load normally.
The world remains server-timed; record native input ACKs and actual draw clocks.
"""
import json,os,shutil,subprocess,time,uuid
from pathlib import Path
root=Path(__file__).resolve().parents[2];out=root/'.local/optimization-20261023/asymmetric';out.mkdir(parents=True,exist_ok=True);temp=out/'temporary';sockets=root/'.s'/('af'+uuid.uuid4().hex[:5]);env=dict(os.environ)
for key,path in {'TMPDIR':temp/'tmp','GOTMPDIR':temp/'tmp','GOCACHE':root/'.local/go-build','GOMODCACHE':root/'.local/go-mod','AGENT_BROWSER_SOCKET_DIR':sockets}.items():path.mkdir(parents=True,exist_ok=True);env[key]=str(path)
env.update(WIRE_BOARD_BUBBLEPARTY_BROWSER='1',WIRE_BOARD_BUBBLEPARTY_BROWSER_OUT=str(out),WIRE_BOARD_PER_CLIENT_RTT='1',GOTOOLCHAIN='local',AGENT_BROWSER_ARGS='--mute-audio,--disable-breakpad,--disable-crash-reporter')
report={'passed':False,'scope':__doc__};server=log=None;opened=[]
NATIVE=r'''const [endpoint,base]=process.argv.slice(2),ws=new WebSocket(endpoint);await new Promise((r,j)=>{ws.onopen=r;ws.onerror=j});let serial=0;const pending=new Map(),inputs=new Map(),acks=[];
ws.onmessage=({data})=>{const m=JSON.parse(data);if(m.method==='Network.webSocketFrameSent'||m.method==='Network.webSocketFrameReceived'){try{const v=JSON.parse(m.params.response.payloadData);if(v.type==='input')inputs.set(v.nonce,Date.now());if(v.type==='ack'&&inputs.has(v.nonce))acks.push({status:v.status,ms:Date.now()-inputs.get(v.nonce)})}catch{}}const p=pending.get(m.id);if(!p)return;pending.delete(m.id);m.error?p.j(Error(JSON.stringify(m.error))):p.r(m.result)};
const call=(method,params={},sessionId)=>new Promise((r,j)=>{const id=++serial;pending.set(id,{r,j});ws.send(JSON.stringify({id,method,params,sessionId}))});let sid;try{const {targetInfos}=await call('Target.getTargets');const t=targetInfos.find(t=>t.type==='page'&&t.url.startsWith(base));({sessionId:sid}=await call('Target.attachToTarget',{targetId:t.targetId,flatten:true}));await call('Network.enable',{},sid);const js=async expression=>{const v=await call('Runtime.evaluate',{expression,returnByValue:true,awaitPromise:true},sid);if(v.exceptionDetails)throw Error(JSON.stringify(v.exceptionDetails));return v.result.value};
const before=await js('(async()=>{const r=(await(await fetch("/api/state")).json()).room;return {you:r.you,...r.game.data}})()');const p=before.players[before.you];const [key]=[['ArrowRight',1,0],['ArrowDown',0,1],['ArrowLeft',-1,0],['ArrowUp',0,-1]].find(([k,x,y])=>p.x+x>=0&&p.x+x<15&&p.y+y>=0&&p.y+y<13&&before.cells[(p.y+y)*15+p.x+x]===0);
await js('window.__samples=[];window.__traceClock=setInterval(()=>{const c=document.querySelector("[data-bubbleparty-canvas]");window.__samples.push({local:Date.now(),render:+c.dataset.renderAt,x:+c.dataset.serverX,y:+c.dataset.serverY})},25)');await call('Input.dispatchKeyEvent',{type:'keyDown',key,code:key},sid);await new Promise(r=>setTimeout(r,180));await call('Input.dispatchKeyEvent',{type:'keyUp',key,code:key},sid);await new Promise(r=>setTimeout(r,2300));const samples=await js('clearInterval(window.__traceClock);window.__samples');const after=await js('(async()=>{const r=(await(await fetch("/api/state")).json()).room;return {you:r.you,...r.game.data}})()');console.log(JSON.stringify({before:before.serverAt,after:after.serverAt,matchEndsBefore:before.matchEndsAt,matchEndsAfter:after.matchEndsAt,acks,samples}));}finally{if(sid)await call('Target.detachFromTarget',{sessionId:sid});ws.close()}'''
def b(who,*args):
 e=dict(env,AGENT_BROWSER_SESSION=who,AGENT_BROWSER_PROFILE=str(temp/who));p=subprocess.run(['agent-browser',*args,'--json'],env=e,cwd=root,capture_output=True,text=True,timeout=45);assert p.returncode==0,p.stdout+p.stderr;v=json.loads(p.stdout);assert v['success'],v;return v['data']
def js(who,s):return b(who,'eval',s)['result']
def api(who,path,body=None):
 options='' if body is None else ',{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify('+json.dumps(body)+')}'
 r=js(who,'(async()=>{const r=await fetch('+json.dumps('/api'+path)+options+');return {status:r.status,data:await r.json()}})()');assert r['status'] in (200,201),r;return r['data']
def cmd(who,kind,**extra):
 r=api(who,'/state')['room'];return api(who,'/rooms/'+r['id'],dict(type=kind,version=r['version'],nonce=uuid.uuid4().hex,**extra))
def wait(fn,seconds=60):
 end=time.monotonic()+seconds
 while time.monotonic()<end:
  if fn():return
  time.sleep(.1)
 raise AssertionError('condition timed out')
try:
 for name in ['url','stop']:(out/name).unlink(missing_ok=True)
 log=(out/'server.log').open('w');server=subprocess.Popen(['go','test','-count=1','-timeout=12m','-run','^TestBubblepartyBrowserFixture$','./games/bubbleparty/tests'],cwd=root,env=env,stdout=log,stderr=subprocess.STDOUT);wait(lambda:(out/'url').exists(),100);base=(out/'url').read_text()
 for who in ['fast','slow']:b(who,'open',base);opened.append(who)
 api('fast','/login',{'name':'泡泡验收管理员','password':'bubbleparty-browser-test-only'})
 r=api('fast','/rooms',{'kind':'bubbleparty','name':'非对称网络时钟验收','capacity':2,'options':{'map':'community10','roundSeconds':300}})
 api('slow','/register',{'name':'慢连接玩家','password':'bubbleparty-browser-test-only','invite':'bubbleparty-browser'})
 api('slow','/rooms/'+r['id'],{'type':'join','version':api('fast','/state')['room']['version'],'nonce':uuid.uuid4().hex});cmd('slow','ready');cmd('fast','ready');cmd('fast','start')
 for who in ['fast','slow']:
  r=api(who,'/state')['room'];cmd(who,'action',action={'type':'character','epoch':r['game']['data']['epoch'],'character':0,'confirm':True})
 b('slow','set','headers','{"X-Acceptance-RTT":"500"}')
 for who in ['fast','slow']:
  print('LOAD',who,flush=True);b(who,'reload');b(who,'wait','[data-bubbleparty-canvas]');b(who,'wait','--fn','!document.querySelector("[data-game-loading]") && document.querySelector(".bubble-joystick").getAttribute("aria-disabled")==="false"')
 processes={}
 for who in ['fast','slow']:
  processes[who]=subprocess.Popen(['node','--input-type=module','-',b(who,'get','cdp-url')['cdpUrl'],base],cwd=root,env=env,stdin=subprocess.PIPE,stdout=subprocess.PIPE,stderr=subprocess.PIPE,text=True);processes[who].stdin.write(NATIVE);processes[who].stdin.close()
 for who,process in processes.items():
  process.wait(timeout=30);stdout=process.stdout.read();stderr=process.stderr.read();assert process.returncode==0,stderr;d=json.loads(stdout);(out/(who+'.json')).write_text(json.dumps(d,indent=2));assert len(d['samples'])>30 and all(b['render']>=a['render'] for a,b in zip(d['samples'],d['samples'][1:]));assert d['after']-d['before']>=2300 and d['matchEndsBefore']==d['matchEndsAfter'];assert len(d['acks'])>=2 and all(a['status']==200 for a in d['acks']),d['acks'];motion=max(abs(v['x']-d['samples'][0]['x'])+abs(v['y']-d['samples'][0]['y']) for v in d['samples']);assert motion>.1,'authoritative player did not move';report[who]={'motion':motion,'ackMS':[a['ms'] for a in d['acks']],'serverAdvancedMS':d['after']-d['before'],'drawClockAdvanceMS':d['samples'][-1]['render']-d['samples'][0]['render']};b(who,'screenshot',str(out/(who+'.png')))
 assert max(report['fast']['ackMS'])<250,report
 assert min(report['slow']['ackMS'])>=400,report
 report['passed']=True;print(json.dumps(report,ensure_ascii=False))
finally:
 if not report['passed']:
  report['failurePages']={}
  for who in opened:
   try:report['failurePages'][who]=js(who,'document.body.innerText');b(who,'screenshot',str(out/(who+'-failure.png')),'--full')
   except Exception as e:report['failurePages'][who]=str(e)
 for who in opened:
  try:b(who,'close')
  except:pass
 if server:
  (out/'stop').write_text('stop')
  try:server.wait(timeout=15)
  except subprocess.TimeoutExpired:server.terminate();server.wait(timeout=10)
 if log:log.close()
 for d in [temp,sockets,out/'data']:shutil.rmtree(d,ignore_errors=True)
 (out/'result.json').write_text(json.dumps(report,ensure_ascii=False,indent=2)+'\n')
