#!/usr/bin/env python3
"""Real public loading overlay, delayed response chunks and unavailable music.
Specified before the loading fix. Production Board/runtime and real rooms.
"""
import json,os,subprocess,time,uuid,shutil
from pathlib import Path
ROOT=Path(__file__).resolve().parents[3];OUT=ROOT/'.local/pvz/followup-loading';OUT.mkdir(parents=True,exist_ok=True)
BASE=os.environ.get('PVZ_BROWSER_URL','').rstrip('/')
ENV=dict(os.environ,AGENT_BROWSER_SESSION='pl'+uuid.uuid4().hex[:4],AGENT_BROWSER_SOCKET_DIR=str(ROOT/'.local/ab'),AGENT_BROWSER_PROFILE=str(OUT/'profile'),TMPDIR=str(OUT/'tmp'),AGENT_BROWSER_ARGS='--mute-audio,--disable-breakpad,--disable-crash-reporter')
for k in ['AGENT_BROWSER_SOCKET_DIR','AGENT_BROWSER_PROFILE','TMPDIR']:Path(ENV[k]).mkdir(parents=True,exist_ok=True)
REPORT={'passed':False,'scope':'real loading overlay and production room; only fixture response delivery is delayed; music endpoint intentionally unavailable','command':'PVZ_BROWSER_URL=<fixture> python3 -B games/pvz/tests/followup-loading-browser.py','samples':[]}
opened=False

def b(*args):
 r=subprocess.run(['agent-browser',*map(str,args),'--json'],cwd=ROOT,env=ENV,text=True,capture_output=True,timeout=60)
 if r.returncode:raise RuntimeError(r.stdout+r.stderr)
 v=json.loads(r.stdout);assert v['success'],v;return v['data']
def js(code):return b('eval',code)['result']
def api(path,body=None):
 options='' if body is None else ',{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify('+json.dumps(body)+')}'
 r=js('(async()=>{const r=await fetch('+json.dumps('/api'+path)+options+');return {status:r.status,body:await r.json()}})()');assert 200<=r['status']<300,r;return r['body']
def command(kind):
 r=api('/state')['room'];return api('/rooms/'+r['id'],dict(type=kind,version=r['version'],nonce=uuid.uuid4().hex))

try:
 assert BASE,'PVZ_BROWSER_URL required'
 init=OUT/'fixture-init.js';init.write_text(r'''(()=>{const original=window.fetch;window.__followupRequests=[];window.fetch=async(...args)=>{const u=String(args[0]);window.__followupRequests.push({url:u,at:performance.now(),ready:document.querySelector('[data-pvz-board]')?.dataset.ready});if(/\/music\/.*\.flac/.test(u))throw Error('test fixture: full music download unavailable');const r=await original(...args);if(!/\.(pvzr|wasm)(\?|$)/.test(u)||!r.body)return r;const reader=r.body.getReader();return new Response(new ReadableStream({async pull(c){const next=await reader.read();await new Promise(resolve=>setTimeout(resolve,30));if(next.done)c.close();else c.enqueue(next.value)},cancel(reason){return reader.cancel(reason)}}),{status:r.status,statusText:r.statusText,headers:r.headers})}})()''')
 b('open','--init-script',init,BASE);opened=True
 api('/register',{'name':'加载验收'+uuid.uuid4().hex[:6],'password':'pvz-loading-test','invite':'pvz-browser'});b('reload')
 api('/rooms',{'kind':'pvz','name':'加载字节与流式音乐验收','capacity':2,'options':{'mode':'duel','teams':[0,1],'scene':'day','roundSeconds':180,'prepSeconds':10}})
 command('add_bot');command('ready');command('start')
 until=time.monotonic()+75
 while time.monotonic()<until:
  sample=js('(()=>{const p=document.querySelector("[data-game-loading=pvz]"),b=document.querySelector("[data-pvz-board]");return {text:p?.innerText??"",size:p?.querySelector("small")?.textContent??"",ready:b?.dataset.ready==="true",error:document.querySelector("[data-game-loading] [role=alert]")?.textContent,at:performance.now()}})()')
  REPORT['samples'].append(sample)
  if 'MB' in sample['size'] and '加载本局' in sample['text'] and not REPORT.get('byteScreenshot'):
   b('screenshot',OUT/'loading-bytes.png','--full');REPORT['byteScreenshot']='loading-bytes.png'
  if sample['ready']:break
  if sample.get('error'):break
  time.sleep(.15)
 REPORT['requests']=js('window.__followupRequests')
 b('screenshot',OUT/'loading-result.png','--full')
 assert any('MB' in s['size'] and '/' in s['size'] for s in REPORT['samples']),'public loading overlay omitted byte sizes'
 assert REPORT['samples'][-1]['ready'],'background music blocked battle readiness'
 assert not any('/music/' in r['url'] and '.flac' in r['url'] for r in REPORT['requests']),'music was fetched as a complete bootstrap dependency'
 REPORT['passed']=True
finally:
 if opened:
  try:command('close');command('leave')
  except Exception:pass
  try:b('close')
  except Exception:pass
 shutil.rmtree(OUT/'profile',ignore_errors=True)
 (OUT/'result.json').write_text(json.dumps(REPORT,ensure_ascii=False,indent=2)+'\n')
 print(json.dumps({'passed':REPORT['passed'],'samples':len(REPORT['samples'])}))
