#!/usr/bin/env python3
"""Native input into the actual React long-press hook. No production game state."""
import functools,hashlib,http.server,json,os,pathlib,shutil,subprocess,threading,uuid
ROOT=pathlib.Path(__file__).resolve().parents[3]
OUT=ROOT/'.local/conquest-hold';OUT.mkdir(parents=True,exist_ok=True)
sources=['games/conquest/web/hold-action-controller.ts','games/conquest/web/hold-action.ts','games/conquest/tests/hold-action.test.mjs','games/conquest/tests/hold-browser.tsx','games/conquest/tests/accept_hold.py','games/conquest/tests/map_ux_input.mjs']
binding={p:hashlib.sha256((ROOT/p).read_bytes()).hexdigest() for p in sources}
run=uuid.uuid4().hex[:5];env=dict(os.environ)
for key,p in {'TMPDIR':ROOT/'.local/tmp','AGENT_BROWSER_SOCKET_DIR':ROOT/'.h','AGENT_BROWSER_PROFILE':OUT/('profile-'+run),'npm_config_cache':ROOT/'.local/npm'}.items():p.mkdir(parents=True,exist_ok=True);env[key]=str(p)
env.update(AGENT_BROWSER_SESSION='h'+run[:3],AGENT_BROWSER_ARGS='--disable-breakpad,--disable-crash-reporter')
def ab(*args):
 r=subprocess.run(['agent-browser',*args,'--json'],cwd=ROOT,env=env,text=True,capture_output=True,timeout=40)
 if r.returncode:raise RuntimeError(r.stdout+r.stderr)
 d=json.loads(r.stdout);assert d['success'],d;return d['data']
def js(code):return ab('eval',code)['result']
subprocess.run([str(ROOT/'web/node_modules/.bin/esbuild'),'games/conquest/tests/hold-browser.tsx','--bundle','--jsx=automatic','--platform=browser','--define:process.env.NODE_ENV="development"','--outfile='+str(OUT/'app.js')],cwd=ROOT,env={**env,'NODE_PATH':str(ROOT/'web/node_modules')},check=True)
(OUT/'index.html').write_text('''<!doctype html><meta name="viewport" content="width=device-width,initial-scale=1"><div id="root"></div><style>body{margin:20px;font:16px sans-serif}#actions{display:flex;gap:20px;margin-top:40px}#actions button{width:120px;height:80px;touch-action:none;user-select:none;-webkit-user-select:none}span{display:block;font-size:12px}output{display:block;margin-top:20px}</style><script src="app.js"></script>''')
class Quiet(http.server.SimpleHTTPRequestHandler):
 def log_message(self,*args):pass
server=http.server.ThreadingHTTPServer(('127.0.0.1',0),functools.partial(Quiet,directory=str(OUT)));threading.Thread(target=server.serve_forever,daemon=True).start();base='http://127.0.0.1:'+str(server.server_port)
report={'passed':False,'scope':'actual React hook in StrictMode, trusted Chromium mouse/touch and keyboard; isolated callback counter, not game rules','cases':[]}
def reset(mode='normal'):
 ab('select','#mode',mode);ab('click','#reset');ab('wait','--fn','document.querySelector("#action-A")&&!document.querySelector("#action-A").disabled&&document.querySelector("#count").textContent==="0"')
def native(mode,**more):
 p=js('(()=>{const r=document.querySelector("#action-A").getBoundingClientRect();return {x:r.x+r.width/2,y:r.y+r.height/2}})()')
 task={'mode':mode,'x':p['x'],'y':p['y'],'steps':1,'pace':0,**more}
 endpoint=ab('get','cdp-url')['cdpUrl']
 subprocess.run(['node','games/conquest/tests/map_ux_input.mjs',endpoint,base,json.dumps(task)],cwd=ROOT,env=env,check=True,stdout=subprocess.PIPE,text=True,timeout=20)
def inspect(name,expected):
 d=js('JSON.parse(JSON.stringify(window.holdEvidence))');assert [x['kind'] for x in d['calls']]==expected,(name,d)
 assert not d['errors'],d
 if d['presses']:assert all(x['trusted'] for x in d['presses']),d
 report['cases'].append({'name':name,**d});return d
try:
 ab('open',base);ab('set','viewport','390','700');ab('wait','--fn','!!document.querySelector("#action-A")')
 for mode,expected in [('tap',['tap']),('mouse-tap',['tap']),('hold',['hold'])]:reset();native(mode,holdMS=800);inspect(mode,expected)
 reset();native('drag',dx=50,steps=12,pace=16);ab('wait','--fn','performance.now()>window.holdEvidence.presses[0].at+900');inspect('drag-cancel',[])
 reset();native('cancel',steps=12,pace=16);ab('wait','--fn','performance.now()>window.holdEvidence.presses[0].at+900');inspect('touchcancel',[])
 for mode,expected in [('identity',[]),('disable',[]),('unmount',[]),('fast',['hold']),('latest',['hold']),('reject',['hold'])]:
  reset(mode);native('hold',holdMS=900);d=inspect(mode,expected)
  if mode=='latest':assert d['calls'][0]['payload']=='new',d
 reset();p=js('(()=>{const a=document.querySelector("#action-A").getBoundingClientRect(),b=document.querySelector("#action-B").getBoundingClientRect();return {x:(a.x+a.width/2+b.x+b.width/2)/2,y:a.y+a.height/2,d:(b.x+b.width/2-a.x-a.width/2)/2}})()')
 native('pinch',x=p['x'],y=p['y'],start=p['d'],end=p['d'],steps=50,pace=16);inspect('two-buttons-multitouch',[])
 reset();ab('focus','#action-A');ab('press','Shift+Enter');inspect('shift-enter',['hold']);ab('press','Enter');inspect('ordinary-enter-after-hold',['hold','tap']);ab('press','Space');inspect('space',['hold','tap','tap'])
 ab('screenshot',str(OUT/'native-hold.png'));report['passed']=True
except Exception as e:
 report['error']=str(e)
 try:ab('screenshot',str(OUT/'failure.png'))
 except Exception:pass
 raise
finally:
 report['sourceSHA256']=binding
 report['sourceMismatches']=[p for p,s in binding.items() if hashlib.sha256((ROOT/p).read_bytes()).hexdigest()!=s]
 if report['sourceMismatches']:report['passed']=False;report['error']='sources changed during acceptance'
 report['bundleSHA256']=hashlib.sha256((OUT/'app.js').read_bytes()).hexdigest()
 (OUT/'result.json').write_text(json.dumps(report,indent=2)+'\n')
 try:ab('close')
 except Exception:pass
 server.shutdown();shutil.rmtree(OUT/('profile-'+run),ignore_errors=True)
if not report['passed']:raise SystemExit('hold acceptance failed')
print(json.dumps({'passed':True,'cases':len(report['cases'])}))
