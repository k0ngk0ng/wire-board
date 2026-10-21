"""Test-first scope/failures: hidden eighth court or teams/characters; duplicate
teams; author-only edits exposed to guest/spectator; stale image on team switch;
mutating character choices while sending team action; overflow, tiny targets,
poor contrast at320px; CDN fallback; missing native touch. Real React/request
recording, not server permission or rule validation. Parent owns HTTP tests.
"""
from pathlib import Path
import functools,http.server,json,os,subprocess,threading,uuid
ROOT=Path(__file__).resolve().parents[2];OUT=ROOT/'.local/single-preparation';OUT.mkdir(parents=True,exist_ok=True)
env=dict(os.environ,AGENT_BROWSER_SESSION='bsp-'+uuid.uuid4().hex[:5],AGENT_BROWSER_SOCKET_DIR=str((ROOT/'.local/basketball').resolve()/'bkn-sockets'),NODE_PATH=str(ROOT/'web/node_modules'))
def browser(*args):
 p=subprocess.run(['agent-browser',*args,'--json'],env=env,capture_output=True,text=True,timeout=60)
 if p.returncode:raise RuntimeError(p.stdout+p.stderr)
 r=json.loads(p.stdout);assert r['success'],r;return r['data']
def js(code):return browser('eval',code)['result']
def wait(code):return browser('wait','--fn',code)
checks=[]
def expect(name,ok):
 assert ok,name
 checks.append(name)
def touch(selector,page):
 browser('scrollintoview',selector)
 point=js('(()=>{const r=document.querySelector('+json.dumps(selector)+').getBoundingClientRect();return{x:r.x+r.width/2,y:r.y+r.height/2}})()')
 endpoint=browser('get','cdp-url')['cdpUrl']
 with (OUT/'touch.stderr').open('w') as errors:
  inspector=subprocess.Popen(['node',str(ROOT/'scripts/basketball/browser_cdp_session.mjs'),endpoint,page],env=env,stdin=subprocess.PIPE,stdout=subprocess.PIPE,stderr=errors,text=True)
  try:
   assert json.loads(inspector.stdout.readline())['ready']
   for method,params in [('Emulation.setTouchEmulationEnabled',dict(enabled=True,maxTouchPoints=1)),('Input.dispatchTouchEvent',dict(type='touchStart',touchPoints=[dict(id=1,**point)])),('Input.dispatchTouchEvent',dict(type='touchEnd',touchPoints=[])),('Emulation.setTouchEmulationEnabled',dict(enabled=False))]:
    inspector.stdin.write(json.dumps(dict(method=method,params=params))+'\n');inspector.stdin.flush();r=json.loads(inspector.stdout.readline());assert 'error' not in r,r
  finally:
   inspector.stdin.close()
   try:inspector.wait(timeout=5)
   except subprocess.TimeoutExpired:inspector.terminate();inspector.wait(timeout=5)
class Handler(http.server.SimpleHTTPRequestHandler):
 def translate_path(self,path):
  if path.startswith('/basketball/v1/selection/'):
   relative=Path(path.split('/selection/',1)[1]);assert '..' not in relative.parts
   return str(ROOT/'.local/basketball/selection-assets'/relative)
  return super().translate_path(path)
 def log_message(self,*args):pass

def main():
 subprocess.run([str(ROOT/'web/node_modules/.bin/esbuild'),str(ROOT/'scripts/basketball/single_preparation_harness.tsx'),'--bundle','--format=esm','--jsx=automatic','--loader:.svg=dataurl','--target=es2022','--outfile='+str(OUT/'ui.js')],cwd=ROOT,env=env,check=True)
 (OUT/'index.html').write_text('<!doctype html><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>单场准备验收</title><link rel="stylesheet" href="/ui.css"><style>.single-preparation-fixture{max-width:960px;margin:auto;padding:12px}.single-preparation-fixture h1{font-size:20px}</style><div id="root"></div><script type="module" src="/ui.js"></script>')
 server=http.server.ThreadingHTTPServer(('127.0.0.1',0),functools.partial(Handler,directory=str(OUT)));threading.Thread(target=server.serve_forever,daemon=True).start();opened=False
 try:
  page=f'http://127.0.0.1:{server.server_port}/';browser('--profile',str(OUT/'profile'),'open',page);opened=True
  wait('!!window.singlePreparation && !!document.querySelector(".basketball-preparation")')
  expect('all eight source courts are offered',js('document.querySelectorAll("[data-court]").length===8'))
  expect('two labelled team selectors expose all eight teams',js('document.querySelectorAll("[data-team-side]").length===2&&[...document.querySelectorAll("[data-team-side]")].every(e=>e.options.length===8)'))
  expect('each roster position offers four source characters',js('[...document.querySelectorAll("[data-actor]")].every(e=>e.querySelectorAll("[data-character]").length===4)'))
  wait('[...document.querySelectorAll(".basketball-preparation img")].every(i=>i.complete&&i.naturalWidth>0)')
  expect('every court and character image uses successful local fallback',js('[...document.querySelectorAll(".basketball-preparation img")].every(i=>new URL(i.currentSrc).pathname.startsWith("/basketball/v1/selection/"))'))
  browser('select','[data-team-side="0"]','7')
  expect('host team request has only side and source team ID',js('JSON.stringify(singlePreparation.actions.at(-1))===JSON.stringify({type:"team",side:0,team:7})'))
  expect('team request waits for authoritative options without overwriting characters',js('JSON.stringify(singlePreparation.options.characters)==="[0,1,2,3]"&&document.querySelector("[data-team-side=\\"0\\"]").value==="0"'))
  pairChecks=js('''(async()=>{let n=0;for(let a=0;a<8;a++)for(let b=0;b<8;b++){if(a===b)continue;singlePreparation.setOptions({...singlePreparation.options,teams:[a,b]});await new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r)));for(let side=0;side<2;side++){const select=document.querySelector(`[data-team-side="${side}"]`);if(select.value!==String(side===0?a:b)||!select.options[side===0?b:a].disabled||select.options[side===0?a:b].disabled)throw Error('invalid pair '+a+','+b);for(let actor=side*2;actor<side*2+2;actor++){const row=document.querySelector(`[data-actor="${actor}"]`);if(row.querySelectorAll('[data-character]').length!==4||![...row.querySelectorAll('img')].every(i=>i.getAttribute('src').includes(`team-${side===0?a:b}-character-`)))throw Error('stale roster images')}}if(JSON.stringify(singlePreparation.options.characters)!=='[0,1,2,3]')throw Error('character reset');n++}return n})()''')
  expect('all56 distinct team pairs keep legal roster indices and refresh imagery',pairChecks==56)
  browser('click','[data-actor="0"] [data-character="3"]')
  expect('fourth character has a native mouse action',js('JSON.stringify(singlePreparation.actions.at(-1))===JSON.stringify({type:"character",actor:0,character:3})'))
  browser('click','[data-court="court-7"]')
  expect('eighth court has a native mouse action',js('JSON.stringify(singlePreparation.actions.at(-1))===JSON.stringify({type:"court",court:"court-7"})'))
  js('singlePreparation.setHost(false);singlePreparation.setYou(1)');wait('document.querySelector("[data-team-side]").disabled')
  expect('versus guest edits only their own actor',js('[...document.querySelectorAll("[data-actor]")].every(e=>[...e.querySelectorAll("button")].every(b=>b.disabled===(e.dataset.actor!=="2")))&&[...document.querySelectorAll("[data-court],[data-team-side]")].every(b=>b.disabled)'))
  js('singlePreparation.setOptions({...singlePreparation.options,mode:"cooperative"})');wait('!document.querySelector("[data-actor=\\"1\\"] [data-character=\\"3\\"]").disabled')
  expect('cooperative guest cannot take a human teammate character',js('document.querySelector("[data-actor=\\"1\\"] [data-character=\\"0\\"]").disabled&&!document.querySelector("[data-actor=\\"1\\"] [data-character=\\"3\\"]").disabled'))
  js('singlePreparation.setSpectating(true)');wait('[...document.querySelectorAll(".basketball-preparation button,.basketball-preparation select")].every(e=>e.disabled)')
  expect('spectator has read-only full preparation',True)
  js('singlePreparation.setSpectating(false);singlePreparation.setHost(true);singlePreparation.setBusy(true)');wait('[...document.querySelectorAll(".basketball-preparation button,.basketball-preparation select")].every(e=>e.disabled)')
  expect('pending action locks all preparation controls',True)
  js('singlePreparation.setBusy(false);singlePreparation.setOptions({...singlePreparation.options,mode:"versus"})');wait('!document.querySelector("[data-team-side]").disabled')
  wait('[...document.querySelectorAll(".basketball-preparation img")].every(i=>i.complete&&i.naturalWidth>0)')
  browser('set','viewport','1100','850');browser('screenshot',str(OUT/'desktop.png'),'--full')
  browser('set','viewport','320','720')
  expect('320px preparation fits horizontally',js('document.documentElement.scrollWidth<=innerWidth'))
  expect('all visible preparation targets are at least44px',js('[...document.querySelectorAll(".basketball-preparation button,.basketball-preparation select")].every(e=>{const r=e.getBoundingClientRect();return r.width>=44&&r.height>=44})'))
  touch('[data-actor="2"] [data-character="1"]',page)
  expect('native mobile touch selects a source character',js('JSON.stringify(singlePreparation.actions.at(-1))===JSON.stringify({type:"character",actor:2,character:1})'))
  contrast=js('''(()=>{const rgb=s=>s.match(/[\\d.]+/g).slice(0,3).map(Number),lum=c=>c.map(n=>{n/=255;return n<=.04045?n/12.92:((n+.055)/1.055)**2.4}).reduce((s,n,i)=>s+n*[.2126,.7152,.0722][i],0);return [...document.querySelectorAll('.basketball-preparation button,.basketball-preparation select,.basketball-preparation h3,.basketball-preparation h4,.basketball-preparation label,.basketball-selection-heading>span,.basketball-player-owner,.basketball-selection-state')].filter(e=>e.textContent.trim()).map(e=>{let b=e;while(b&&getComputedStyle(b).backgroundColor==='rgba(0, 0, 0, 0)')b=b.parentElement;const fg=lum(rgb(getComputedStyle(e).color)),bg=lum(rgb(getComputedStyle(b).backgroundColor));return{text:e.textContent,ratio:(Math.max(fg,bg)+.05)/(Math.min(fg,bg)+.05)}})})()''')
  expect('actual foreground/background contrast reaches4.5:1',all(r['ratio']>=4.5 for r in contrast));(OUT/'contrast.json').write_text(json.dumps(contrast,ensure_ascii=False,indent=2)+'\n')
  browser('screenshot',str(OUT/'mobile-320.png'),'--full')
  result=dict(status='passed',checks=checks,pairs=pairChecks,scope='isolated real React preparation, native mouse/touch and exported original assets; recorded requests, server permission/rules validated separately')
  (OUT/'result.json').write_text(json.dumps(result,ensure_ascii=False,indent=2)+'\n');print(json.dumps(result,ensure_ascii=False))
 except Exception as e:
  (OUT/'result.json').write_text(json.dumps(dict(status='failed',checks=checks,error=str(e)),ensure_ascii=False,indent=2)+'\n');raise
 finally:
  if opened:browser('close')
  server.shutdown();server.server_close()
if __name__=='__main__':main()
