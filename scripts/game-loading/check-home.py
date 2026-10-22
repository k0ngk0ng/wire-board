import json,os,shutil,subprocess,uuid
from pathlib import Path
root=Path.cwd();out=root/'.local/game-loading/home';out.mkdir(parents=True,exist_ok=True);temp=out/'temporary';temp.mkdir(exist_ok=True);sockets=root/'.s'/('home'+uuid.uuid4().hex[:5]);sockets.mkdir(parents=True)
env=dict(os.environ,AGENT_BROWSER_SESSION=sockets.name,AGENT_BROWSER_SOCKET_DIR=str(sockets),AGENT_BROWSER_PROFILE=str(temp/'profile'),AGENT_BROWSER_ARGS='--disable-breakpad,--disable-crash-reporter',TMPDIR=str(temp))
def browser(*args):
 r=subprocess.run(['agent-browser',*args,'--json'],cwd=root,env=env,text=True,capture_output=True,timeout=90);assert r.returncode==0,r.stdout+r.stderr
 j=json.loads(r.stdout);assert j['success'],j;return j['data']
opened=False
try:
 browser('open','about:blank');opened=True
 endpoint=browser('get','cdp-url')['cdpUrl']
 node=r'''
const endpoint=process.argv[2],targetURL=process.argv[3],ws=new WebSocket(endpoint);await new Promise((r,j)=>{ws.onopen=r;ws.onerror=j});let id=0,sid;const pending=new Map();let requests=[],responses=[];
const call=(method,params={},sessionId=sid)=>new Promise((resolve,reject)=>{const n=++id;pending.set(n,{resolve,reject});ws.send(JSON.stringify({id:n,method,params,sessionId:method.startsWith('Target.')?undefined:sessionId}))});
ws.onmessage=({data})=>{const m=JSON.parse(data);if(m.method==='Network.responseReceived')responses.push({url:m.params.response.url,status:m.params.response.status,mime:m.params.response.mimeType,diskCache:m.params.response.fromDiskCache,serviceWorker:m.params.response.fromServiceWorker,headers:m.params.response.headers});if(m.method==='Network.requestWillBeSent')requests.push({url:m.params.request.url,type:m.params.type});if(!m.id)return;const p=pending.get(m.id);pending.delete(m.id);m.error?p.reject(m.error):p.resolve(m.result)};
const evaluate=async expression=>(await call('Runtime.evaluate',{expression,returnByValue:true,awaitPromise:true})).result.value;
try{
 const {targetInfos}=await call('Target.getTargets');const target=targetInfos.find(t=>t.type==='page'&&t.url==='about:blank');({sessionId:sid}=await call('Target.attachToTarget',{targetId:target.targetId,flatten:true}));
 await call('Network.enable');await call('Network.clearBrowserCache');await call('Page.enable');
 await call('Page.addScriptToEvaluateOnNewDocument',{source:`window.__longTasks=[];new PerformanceObserver(list=>window.__longTasks.push(...list.getEntries().map(e=>({start:e.startTime,duration:e.duration})))).observe({type:'longtask',buffered:true});`});
 const results=[];
 for(const kind of ['cold','reload']){
  requests=[];responses=[];
  if(kind==='cold')await call('Page.navigate',{url:targetURL});else await call('Page.reload',{ignoreCache:false});
  const deadline=Date.now()+60000;
  while(Date.now()<deadline){if(await evaluate(`location.host===${JSON.stringify(new URL(targetURL).host)}&&document.readyState==='complete'&&!!document.querySelector('input[type=password]')`))break;await new Promise(r=>setTimeout(r,100));}
  await new Promise(r=>setTimeout(r,1000));
  const result=await evaluate(`(()=>{const nav=performance.getEntriesByType('navigation')[0];return {ready:!!document.querySelector('input[type=password]'),navigation:nav?.toJSON(),paint:performance.getEntriesByType('paint').map(e=>e.toJSON()),longTasks:window.__longTasks,resources:performance.getEntriesByType('resource').map(e=>({url:e.name,type:e.initiatorType,start:e.startTime,duration:e.duration,transfer:e.transferSize,encoded:e.encodedBodySize,decoded:e.decodedBodySize}))}})()`);
  results.push({kind,...result,requests:[...requests],responses:[...responses]});
 }
 console.log(JSON.stringify(results));
}finally{if(sid)await call('Target.detachFromTarget',{sessionId:sid});ws.close()}
'''
 r=subprocess.run(['node','--input-type=module','-',endpoint,os.environ.get('LOADING_BASE','http://127.0.0.1:18189')],input=node,cwd=root,env=env,text=True,capture_output=True,timeout=150);assert r.returncode==0,r.stderr
 (out/'browser.json').write_text(json.dumps(json.loads(r.stdout),ensure_ascii=False,indent=2)+'\n')
 browser('screenshot',str(out/'homepage.png'))
finally:
 if opened:
  try:browser('close')
  except Exception:pass
 shutil.rmtree(temp,ignore_errors=True);shutil.rmtree(sockets,ignore_errors=True)
