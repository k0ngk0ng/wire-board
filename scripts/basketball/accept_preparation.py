"""Repeatable isolated UI E2E; not a substitute for authenticated preparation tests.
Failure cases: missing images/fallback, creation leaking preparation, incorrect seat
mapping (solo/cooperative/versus), spectator or busy editing, host unable to adjust roster, guest taking another
human character, changing the opposing team, overflow/too-small targets at 320px.
"""
from pathlib import Path
import functools, http.server, json, os, subprocess, threading, uuid
ROOT=Path(__file__).resolve().parents[2]
OUT=ROOT/'.local/basketball/preparation-ui';OUT.mkdir(parents=True,exist_ok=True)
env=dict(os.environ,AGENT_BROWSER_SESSION='bprep-'+uuid.uuid4().hex[:7],AGENT_BROWSER_SOCKET_DIR=str(ROOT/'.local/bprep-sockets'),TMPDIR=str(ROOT/'.local/tmp'),NODE_PATH=str(ROOT/'web/node_modules'))
Path(env['AGENT_BROWSER_SOCKET_DIR']).mkdir(exist_ok=True)
checks=[];opened=False;server=None;inspector=None

def browser(*args):
 r=subprocess.run(['agent-browser',*args,'--json'],env=env,capture_output=True,text=True,timeout=60)
 if r.returncode:raise RuntimeError(r.stdout+r.stderr)
 v=json.loads(r.stdout)
 if not v['success']:raise RuntimeError(v)
 return v['data']
def cdp(method,params={}):
 inspector.stdin.write(json.dumps(dict(method=method,params=params))+'\n');inspector.stdin.flush();result=json.loads(inspector.stdout.readline())
 if 'error' in result:raise RuntimeError(result['error'])
 return result['result']
def js(source):
 result=cdp('Runtime.evaluate',dict(expression=source,returnByValue=True,awaitPromise=True,timeout=4000))
 if result.get('exceptionDetails'):raise RuntimeError(result['exceptionDetails'])
 return result['result'].get('value')
def wait(expression):return js('(async()=>{for(let n=0;n<100;n++){if('+expression+')return true;await new Promise(r=>setTimeout(r,20));}throw Error("timeout")})()')
def click(selector,touch=False):
 point=js('(()=>{const e=document.querySelector('+json.dumps(selector)+');e.scrollIntoView({block:"center"});const r=e.getBoundingClientRect();return{x:r.x+r.width/2,y:r.y+r.height/2}})()')
 if touch:
  cdp('Emulation.setTouchEmulationEnabled',dict(enabled=True,maxTouchPoints=1))
  cdp('Input.dispatchTouchEvent',dict(type='touchStart',touchPoints=[dict(id=1,**point)]));cdp('Input.dispatchTouchEvent',dict(type='touchEnd',touchPoints=[]))
 else:
  cdp('Input.dispatchMouseEvent',dict(type='mousePressed',button='left',clickCount=1,**point));cdp('Input.dispatchMouseEvent',dict(type='mouseReleased',button='left',clickCount=1,**point))
def expect(name,ok):
 assert ok,name
 checks.append(name)
class Handler(http.server.SimpleHTTPRequestHandler):
 def translate_path(self,path):
  if path.startswith('/basketball/v1/selection/'):
   return str(ROOT/'.local/basketball/selection-assets'/path.split('/selection/',1)[1])
  return super().translate_path(path)
 def log_message(self,*args):pass
try:
 (OUT/'result.json').write_text('{"status":"running"}\n')
 subprocess.run([str(ROOT/'web/node_modules/.bin/esbuild'),str(ROOT/'scripts/basketball/preparation_harness.tsx'),'--bundle','--format=esm','--jsx=automatic','--loader:.svg=dataurl','--target=es2022','--outfile='+str(OUT/'ui.js')],cwd=ROOT,env=env,check=True)
 (OUT/'index.html').write_text('<!doctype html><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>准备阶段验收</title><style>*{box-sizing:border-box}body{margin:0;background:#f5f1e8;color:#302b26;font-family:sans-serif}main{max-width:920px;margin:auto;padding:12px}h1{font-size:20px}button{font:inherit}</style><link rel="stylesheet" href="/ui.css"><div id="root"></div><script type="module" src="/ui.js"></script>')
 server=http.server.ThreadingHTTPServer(('127.0.0.1',0),functools.partial(Handler,directory=str(OUT)));threading.Thread(target=server.serve_forever,daemon=True).start()
 page='http://127.0.0.1:'+str(server.server_port)+'/'
 browser('--profile',str(OUT/'profile'),'open',page);opened=True
 endpoint=browser('get','cdp-url')['cdpUrl'];inspector=subprocess.Popen(['node',str(ROOT/'scripts/basketball/browser_cdp_session.mjs'),endpoint,page],env=env,stdin=subprocess.PIPE,stdout=subprocess.PIPE,stderr=(OUT/'cdp.stderr').open('w'),text=True);assert json.loads(inspector.stdout.readline())['ready']
 wait('!!document.querySelector(".basketball-preparation")')
 wait('[...document.querySelectorAll(".basketball-preparation img")].every(i=>i.complete&&i.naturalWidth>0)')
 expect('actual original court and character assets load with local fallback',js('[...document.querySelectorAll(".basketball-preparation img")].every(i=>i.currentSrc.includes("/basketball/v1/selection/"))'))
 contrast=js('(()=>{const rgb=s=>s.match(/[\\d.]+/g).slice(0,3).map(Number),lum=c=>c.map(n=>{n/=255;return n<=.04045?n/12.92:((n+.055)/1.055)**2.4}).reduce((s,n,i)=>s+n*[.2126,.7152,.0722][i],0);return [...document.querySelectorAll(".basketball-preparation span,.basketball-preparation strong,.basketball-preparation h3,.basketball-preparation h4")].filter(e=>e.textContent.trim()).map(e=>{let b=e;while(b&&getComputedStyle(b).backgroundColor==="rgba(0, 0, 0, 0)")b=b.parentElement;const fg=lum(rgb(getComputedStyle(e).color)),bg=lum(rgb(getComputedStyle(b).backgroundColor));return {text:e.textContent,ratio:(Math.max(fg,bg)+.05)/(Math.min(fg,bg)+.05)}})})()')
 expect('computed text contrast is at least 4.5:1 on actual backgrounds',all(item['ratio']>=4.5 for item in contrast))
 (OUT/'contrast.json').write_text(json.dumps(contrast,ensure_ascii=False,indent=2)+'\n')
 expect('only verified courts and team-local characters are selectable',js('document.querySelectorAll("[data-court]").length===7 && document.querySelectorAll("[data-character]").length===8'))
 expect('verified court names follow IDs rather than menu order or team names',js('JSON.stringify([...document.querySelectorAll("[data-court]")].map(b=>b.innerText.replace(/已选|\\s/g,"")))===JSON.stringify(["尼亚加拉","纽约","旧金山","得克萨斯","美国海军","夏威夷","拉斯维加斯"])'))
 expect('player names match original team-local portrait identities',js('[[0,0,"国夫"],[0,1,"阿力"],[2,0,"迈克尔"],[2,1,"埃迪"]].every(([a,c,name])=>{const b=document.querySelector(`[data-actor="${a}"] [data-character="${c}"]`);return b.getAttribute("aria-label")===name && b.textContent.includes(name) && b.querySelector("img").src.includes(`team-${a/2}-character-${c}-portrait.png`)})'))
 expect('host can adjust the complete roster',js('[...document.querySelectorAll("[data-actor] button")].every(b=>!b.disabled)'))
 click('[data-court="court-0"]');wait('preparationFixture.options.court==="court-0"')
 click('[data-actor="1"] [data-character="0"]');wait('preparationFixture.options.characters[1]===0')
 expect('choice emits scoped command and swaps only the same team',js('JSON.stringify(preparationFixture.actions)===JSON.stringify([{type:"court",court:"court-0"},{type:"character",actor:1,character:0}]) && preparationFixture.options.characters[0]===1 && preparationFixture.options.characters[2]===0 && preparationFixture.options.characters[3]===1'))
 browser('set','viewport','1100','850');browser('screenshot',str(OUT/'desktop.png'),'--full')
 js('preparationFixture.setYou(1)');wait('document.querySelector("[data-court]").disabled')
 expect('versus guest controls actor2 only',js('[...document.querySelectorAll("[data-actor]")].every(e=>[...e.querySelectorAll("button")].every(b=>b.disabled===(e.dataset.actor!=="2")))'))
 click('[data-actor="2"] [data-character="1"]');wait('preparationFixture.options.characters[2]===1')
 js('preparationFixture.setOptions({...preparationFixture.options,mode:"cooperative"})');wait('!document.querySelector(`[data-actor="1"] button`).disabled')
 expect('cooperative guest cannot take the other human character',js('document.querySelector(`[data-actor="1"] [data-character="1"]`).disabled && document.querySelector(`[data-actor="2"] button`).disabled'))
 js('preparationFixture.setBusy(true)');wait('[...document.querySelectorAll(".basketball-preparation button")].every(b=>b.disabled)')
 expect('pending command blocks repeated edits',True)
 js('preparationFixture.setBusy(false);preparationFixture.setYou(-1)');wait('[...document.querySelectorAll(".basketball-preparation button")].every(b=>b.disabled)')
 expect('spectator sees images but cannot edit',True);browser('screenshot',str(OUT/'spectator.png'),'--full')
 js('preparationFixture.setYou(0);preparationFixture.setOptions({...preparationFixture.options,mode:"solo"})');wait('!document.querySelector(`[data-actor="3"] button`).disabled')
 expect('solo host owns own character and all computer choices',js('[...document.querySelectorAll("[data-actor] button")].every(b=>!b.disabled)'))
 browser('set','viewport','320','720')
 expect('320px has no horizontal overflow and touch targets remain usable',js('document.documentElement.scrollWidth<=innerWidth && [...document.querySelectorAll(".basketball-preparation button")].every(b=>b.getBoundingClientRect().width>=44 && b.getBoundingClientRect().height>=44)'))
 click('[data-court="court-2"]',touch=True);wait('preparationFixture.options.court==="court-2"')
 expect('native touch selects the visible court image at 320px',True)
 browser('screenshot',str(OUT/'mobile-320.png'),'--full')
 js('preparationFixture.setCreation(true)');wait('!document.querySelector(".basketball-preparation")')
 expect('creation options contain no preparation selection',js('!document.querySelector("[data-court]") && !document.querySelector("[data-character]")'))
 result=dict(status='passed',checks=checks,scope='isolated real React preparation UI, original PNG assets, native clicks; server permissions covered separately')
 (OUT/'result.json').write_text(json.dumps(result,ensure_ascii=False,indent=2)+'\n');print(json.dumps(result,ensure_ascii=False))
except Exception as error:
 if inspector:
  try:(OUT/'failure-state.json').write_text(json.dumps(js('({body:document.body.innerText,actions:preparationFixture.actions,options:preparationFixture.options})'),ensure_ascii=False,indent=2))
  except Exception:pass
 if opened:
  try:browser('screenshot',str(OUT/'failure.png'),'--full')
  except Exception:pass
 (OUT/'result.json').write_text(json.dumps(dict(status='failed',checks=checks,error=repr(error)),ensure_ascii=False,indent=2)+'\n')
 raise
finally:
 if inspector:
  inspector.stdin.close()
  try:inspector.wait(timeout=5)
  except subprocess.TimeoutExpired:inspector.terminate();inspector.wait(timeout=5)
 if opened:browser('close')
 if server:server.shutdown()
