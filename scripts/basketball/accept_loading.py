"""Real React Board, authenticated HTTP, browser Wasm and native keyboard/touch."""
from pathlib import Path
import json,os,subprocess,uuid
ROOT=Path(__file__).resolve().parents[2];OUT=Path(os.environ.get('BASKETBALL_BROWSER_OUTPUT',str(ROOT/'.local/basketball/platform-rooms'))).resolve();assert OUT.is_relative_to((ROOT/'.local/basketball').resolve());OUT.mkdir(parents=True,exist_ok=True)
env=dict(os.environ,AGENT_BROWSER_SESSION='bkl-'+uuid.uuid4().hex[:6],AGENT_BROWSER_SOCKET_DIR=str((ROOT/'.local/basketball').resolve()/'bl-sockets'),TMPDIR=str(ROOT/'.local/tmp'),NODE_PATH=str(ROOT/'web/node_modules'))
Path(env['AGENT_BROWSER_SOCKET_DIR']).mkdir(exist_ok=True)
base=env['BASKETBALL_HTTP_URL'];checks=[];opened=False;inspector=None
def browser(*args):
 if inspector and args[0] in ('click','select','fill') and not args[1].startswith('@'):
  wait('(()=>{const e=document.querySelector('+json.dumps(args[1])+');return !!e&&!e.disabled})()')
 if inspector and args[:4]==('find','role','button','click') and '--name' in args:
  label=args[args.index('--name')+1]
  wait('(()=>{const e=[...document.querySelectorAll("button")].find(b=>b.textContent.trim()==='+json.dumps(label)+'||b.getAttribute("aria-label")==='+json.dumps(label)+');return !!e&&!e.disabled})()')
 r=subprocess.run(['agent-browser',*args,'--json'],env=env,capture_output=True,text=True,timeout=60)
 if r.returncode:raise RuntimeError(r.stdout+r.stderr)
 value=json.loads(r.stdout)
 if not value['success']:raise RuntimeError(value)
 return value['data']
def cdp(method,params={}):
 inspector.stdin.write(json.dumps(dict(method=method,params=params))+'\n');inspector.stdin.flush();r=json.loads(inspector.stdout.readline())
 if 'error'in r:raise RuntimeError(r['error'])
 return r['result']
def js(source):
 r=cdp('Runtime.evaluate',dict(expression=source,returnByValue=True,awaitPromise=True,timeout=20000))
 if r.get('exceptionDetails'):raise RuntimeError(r['exceptionDetails'])
 return r['result'].get('value')
def wait(expression):
 return js('(async()=>{for(let n=0;n<750;n++){if(await ('+expression+'))return true;await new Promise(r=>setTimeout(r,20));}throw Error('+json.dumps('Timed out: '+expression)+')})()')
def key(kind,code,key,number):cdp('Input.dispatchKeyEvent',dict(type=kind,code=code,key=key,windowsVirtualKeyCode=number))
def expect(name,value):
 assert value,name
 checks.append(name)
try:
 (OUT/'result.json').write_text('{"status":"running"}\n')
 page=base+'/'
 browser('--profile',str(OUT/'profile'),'open',page);opened=True
 endpoint=browser('get','cdp-url')['cdpUrl'];inspector=subprocess.Popen(['node',str(ROOT/'scripts/basketball/browser_cdp_session.mjs'),endpoint,page],env=env,stdin=subprocess.PIPE,stdout=subprocess.PIPE,stderr=(OUT/'cdp.stderr').open('w'),text=True);assert json.loads(inspector.stdout.readline())['ready']
 # Mirror verified OSS bytes for loopback CORS; requests retain their real logical
 # URLs in evidence. Only transfer timing and one error are controlled here.
 bootstrap=r"""window.assetRequests=[];window.progressSamples=[];window.failCourt=!sessionStorage.getItem('basketball-load-failure-tested');
 const originalFetch=window.fetch;
 window.fetch=async(input,options)=>{
  const url=String(input);if(!url.startsWith('https://cdn.ichenj.com/'))return originalFetch(input,options);
  assetRequests.push({url,at:performance.now()});
  if(window.failCourt && url.includes('/courts/6/court-')){window.failCourt=false;sessionStorage.setItem('basketball-load-failure-tested','1');return new Response('test outage',{status:503})}
  const response=await originalFetch('/__oss/'+new URL(url).pathname.slice(1),options);
  if(!url.endsWith('.wasm'))return response;
  const reader=response.body.getReader();
  return new Response(new ReadableStream({async pull(c){await new Promise(r=>setTimeout(r,25));const next=await reader.read();if(next.done)c.close();else c.enqueue(next.value)},cancel(){return reader.cancel()}}),{status:response.status,headers:response.headers});
 };
 setInterval(()=>{const p=document.querySelector('.basketball-loading progress');if(p)progressSamples.push({value:p.value,at:performance.now()})},30);"""
 cdp('Page.enable');cdp('Page.addScriptToEvaluateOnNewDocument',dict(source=bootstrap));js(bootstrap)
 wait('!!document.querySelector("input[name=name]")')
 browser('fill','input[name=name]','牌局管理员');browser('fill','input[name=password]','poker-test-password');browser('click','.auth-form button[type=submit], .auth-form button.primary')
 wait('!!document.querySelector(".basketball-error")')
 state=js('fetch("/api/state").then(r=>r.json())');room=state['room'];epoch=room['game']['data']['epoch']
 expect('failed graphics keep original match at video zero',room['game']['data']['phase']=='loading' and room['game']['data']['runtime']['videoFrames']==0)
 browser('set','viewport','320','720');browser('screenshot',str(OUT/'load-error-mobile.png'))
 browser('find','role','button','click','--name','重新连接')
 wait('document.querySelector("[data-basketball-ready=true]") && document.querySelector("progress")?.value===100')
 wait('document.querySelector(".basketball-loading").textContent.includes("等待：牌局玩家1")')
 expect('loaded first player waits for second human',js('fetch("/api/state").then(r=>r.json()).then(s=>s.room.game.data.phase==="loading" && s.room.game.data.runtime.videoFrames===0 && s.room.game.data.loading.ready[0] && !s.room.game.data.loading.ready[2])'))
 expect('actual download produces intermediate progress',js('progressSamples.some(p=>p.value>0&&p.value<100)'))
 expect('320px loading UI fits horizontally',js('document.documentElement.scrollWidth<=innerWidth'))
 browser('screenshot',str(OUT/'waiting-player-mobile.png'))
 browser('set','viewport','1280','900');browser('screenshot',str(OUT/'waiting-player-desktop.png'))
 requests=js('assetRequests');samples=js('progressSamples')
 expect('no other court textures are fetched before start',all('/courts/' not in r['url'] or '/courts/6/' in r['url'] or r['url'].endswith('/courts/manifest.json') for r in requests))
 expect('no audio is fetched by the loading barrier',all('/audio/' not in r['url'] for r in requests) and js('performance.getEntriesByType("resource").every(e=>!e.name.includes("/audio/"))'))
 expect('Wasm downloads from immutable OSS address',any('/runtime-' in r['url'] and r['url'].endswith('/basketball.wasm') for r in requests))
 import urllib.request,http.cookiejar
 peer=urllib.request.build_opener(urllib.request.HTTPCookieProcessor(http.cookiejar.CookieJar()))
 def request(path,body=None):
  req=urllib.request.Request(base+'/api'+path,data=json.dumps(body).encode() if body is not None else None,headers={'Content-Type':'application/json'})
  with peer.open(req) as response:return json.load(response)
 request('/login',{'name':'牌局玩家1','password':'poker-test-password'})
 other=request('/state')['room'];request('/rooms/'+room['id'],{'type':'action','version':other['version'],'nonce':'loading-peer-ack-001','action':{'type':'loaded','epoch':epoch}})
 wait('Number(document.querySelector(".basketball-stage canvas")?.dataset.videoFrame)>=120')
 expect('second acknowledgement starts original game',js('!document.querySelector(".basketball-loading") && !document.querySelector(".basketball-error")'))
 before=request('/state')['room']['game']['data']['runtime']['videoFrames']
 browser('reload');wait('!!document.querySelector("[data-basketball-ready=true]")')
 after=request('/state')['room']['game']['data']
 expect('reloading active player does not restart barrier or match',after['phase']!='loading' and after['epoch']==epoch and after['runtime']['videoFrames']>before)
 browser('screenshot',str(OUT/'playing-after-reload.png'))
 (OUT/'result.json').write_text(json.dumps(dict(status='passed',checks=checks,requests=requests,progress=samples,scope='production UI and actual host; verified OSS bytes mirrored for loopback CORS, controlled slow transfer/image failure; no production network timing claim'),ensure_ascii=False,indent=2)+'\n')
 print(json.dumps(checks,ensure_ascii=False))
except Exception as error:
 (OUT/'result.json').write_text(json.dumps(dict(status='failed',checks=checks,error=repr(error)),ensure_ascii=False,indent=2)+'\n')
 if opened:
  try:
   (OUT/'failure-state.json').write_text(json.dumps(js('fetch("/api/state").then(r=>r.json()).then(state=>({body:document.body.innerText,fetch:String(fetch),requests:window.assetRequests,resources:performance.getEntriesByType("resource").map(e=>e.name),state}))'),ensure_ascii=False,indent=2)+'\n')
   browser('screenshot',str(OUT/'failure.png'))
  except Exception:pass
 raise
finally:
 if inspector:
  inspector.stdin.close()
  try:inspector.wait(timeout=5)
  except subprocess.TimeoutExpired:inspector.terminate();inspector.wait(timeout=5)
 if opened:browser('close')
