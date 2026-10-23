#!/usr/bin/env python3
"""Actual-hook/native-input camera acceptance. No production server required."""
import functools,http.server,json,os,pathlib,subprocess,threading,time,uuid,shutil
ROOT=pathlib.Path(__file__).resolve().parents[3]
OUT=ROOT/'.local/conquest-camera';OUT.mkdir(parents=True,exist_ok=True)
run=uuid.uuid4().hex[:6]; env=dict(os.environ)
for key,p in {'TMPDIR':ROOT/'.local/tmp','AGENT_BROWSER_SOCKET_DIR':ROOT/'.c','AGENT_BROWSER_PROFILE':OUT/('profile-'+run),'npm_config_cache':ROOT/'.local/npm'}.items():p.mkdir(parents=True,exist_ok=True);env[key]=str(p)
env.update(AGENT_BROWSER_SESSION='cm'+run[:3],AGENT_BROWSER_ARGS='--disable-breakpad,--disable-crash-reporter')
env.setdefault('NODE_PATH',str(ROOT/'web/node_modules'))
def ab(*args):
 p=subprocess.run(['agent-browser',*args,'--json'],cwd=ROOT,env=env,text=True,capture_output=True,timeout=40)
 if p.returncode:raise RuntimeError(p.stdout+p.stderr)
 d=json.loads(p.stdout);assert d['success'],d;return d['data']
def js(code):return ab('eval',code)['result']
def reset_fit():
 ab('find','role','button','click','--name','Reset','--exact')
 ab('wait','--fn','''(()=>{const v=document.querySelector('#camera-viewport'),s=Math.min(v.clientWidth/1440,v.clientHeight/720),m=document.querySelector('#scene').transform.baseVal.consolidate().matrix;return Math.abs(m.a-s)<.0001&&Math.abs(m.e-(v.clientWidth-1440*s)/2)<.01&&Math.abs(m.f-(v.clientHeight-720*s)/2)<.01})()''')
def native(mode):
 selector='#minimap' if mode=='mini' else '[data-label=fra-east]'
 if mode in ['mouse-fit','touch-fit']:
  point=js('''(()=>{for(const p of document.querySelectorAll('[data-region]')){const b=p.getBBox(),m=p.getScreenCTM();for(let y=1;y<10;y++)for(let x=1;x<10;x++){const q=new DOMPoint(b.x+b.width*x/10,b.y+b.height*y/10);if(!p.isPointInFill(q))continue;const s=q.matrixTransform(m);if([[0,0],[-2,0],[2,0],[0,-2],[0,2]].every(([dx,dy])=>document.elementFromPoint(s.x+dx,s.y+dy)===p))return {x:s.x,y:s.y}}}return null})()''')
 elif mode in ['mouse-path','mouse-outside']:
  point=js('''(()=>{const p=document.querySelector('[data-region=fra-east]'),b=p.getBBox(),m=p.getScreenCTM();for(let y=1;y<10;y++)for(let x=1;x<10;x++){const q=new DOMPoint(b.x+b.width*x/10,b.y+b.height*y/10);if(!p.isPointInFill(q))continue;const s=q.matrixTransform(m);if(document.elementFromPoint(s.x,s.y)===p)return {x:s.x,y:s.y}}return null})()''')
 else:point=js('(()=>{const r=document.querySelector('+json.dumps(selector)+').getBoundingClientRect();return {x:r.x+r.width/2,y:r.y+r.height/2}})()')
 assert point is not None,('no real target hit point',mode)
 endpoint=ab('get','cdp-url')['cdpUrl']
 p=subprocess.run(['node','games/conquest/tests/camera-native.mjs',endpoint,base,mode,str(point['x']),str(point['y'])],cwd=ROOT,env=env,text=True,capture_output=True,check=True,timeout=25)
 return json.loads(p.stdout)
bundle=OUT/'app.js'
subprocess.run([env.get('CONQUEST_CAMERA_ESBUILD',str(ROOT/'web/node_modules/.bin/esbuild')),'games/conquest/tests/camera-browser.tsx','--bundle','--jsx=automatic','--platform=browser','--define:process.env.NODE_ENV="development"','--outfile='+str(bundle)],cwd=ROOT,env=env,check=True)
(OUT/'index.html').write_text('''<!doctype html><meta name="viewport" content="width=device-width,initial-scale=1"><div id="root"></div><style>*{box-sizing:border-box}body{margin:0;font:14px sans-serif}header{height:40px}.cq-viewport{width:100%;height:440px;position:relative;overflow:hidden;touch-action:none;user-select:none}.cq-map{display:block}button{height:36px}</style><script src="app.js"></script>''')
class Quiet(http.server.SimpleHTTPRequestHandler):
 def log_message(self,*args):pass
server=http.server.ThreadingHTTPServer(('127.0.0.1',0),functools.partial(Quiet,directory=str(OUT)));threading.Thread(target=server.serve_forever,daemon=True).start();base='http://127.0.0.1:'+str(server.server_port)
report={'passed':False,'scope':'actual React camera hook and 206-region world map, native Chromium CDP inputs','cases':[]}
try:
 ab('open',base);ab('set','viewport','320','740');ab('wait','--fn','document.querySelectorAll("[data-region]").length===206')
 fit_errors=[]
 for width,mode in [(1440,'mouse-fit'),(1440,'mouse-fit-text'),(320,'touch-fit')]:
  ab('set','viewport',str(width),'900' if width==1440 else '740');reset_fit()
  record=native(mode);report['cases'].append(record)
  assert record['startTarget']['tag']==('text' if mode=='mouse-fit-text' else 'path'),(mode,'not on a real territory/text')
  assert record['clickDelta']==0 and record['renderDelta']<=3,(mode,'gesture selected/rerendered')
  if not (abs(record['after']['camera']['x']-record['before']['camera']['x'])>20 or abs(record['after']['camera']['y']-record['before']['camera']['y'])>20):fit_errors.append(mode)
 assert not fit_errors,('fit state refuses to pan',fit_errors)
 for mode in ['pinch','drag','mouse','cancel','mini']:
  reset_fit()
  ab('find','role','button','click','--name','Zoom','--exact')
  ab('wait','--fn','document.querySelector("#scene").transform.baseVal.consolidate().matrix.a > .3')
  record=native(mode);report['cases'].append(record)
  assert record['clickDelta']==0,(mode,'gesture clicked a region')
  assert record['renderDelta']<=3,(mode,'gesture rerendered whole world',record['renderDelta'])
  if mode=='pinch':assert record['after']['camera']['scale']>record['before']['camera']['scale']*1.5,(mode,'pinch did not magnify')
  else:assert record['after']['camera']!=record['before']['camera'],(mode,'map did not move')
  if mode=='mini':assert all(abs(record['after']['camera'][k]-record['after']['miniExpected'][k])<.01 for k in ['x','y','scale']),('minimap touch overwritten by main map',record['after']['camera'],record['after']['miniExpected'])
 reset_fit()
 ab('find','role','button','click','--name','Zoom','--exact');ab('wait','--fn','document.querySelector("#scene").transform.baseVal.consolidate().matrix.a > .6')
 record=native('tap');report['cases'].append(record);assert record['clickDelta']==1,'tap after gesture must select once'
 ab('screenshot',str(OUT/'camera-mobile.png'),'--full')
 ab('set','viewport','1440','900')
 for mode in ['mouse-path','mouse-text','mouse-outside','mouse-tap']:
  reset_fit()
  ab('find','role','button','click','--name','Focus France','--exact')
  ab('wait','--fn','document.querySelector("#scene").transform.baseVal.consolidate().matrix.a > 1.4')
  record=native(mode);report['cases'].append(record)
  assert record['startTarget']['tag']==('path' if mode in ['mouse-path','mouse-outside'] else 'text'),(mode,'wrong native hit target',record['startTarget'])
  pressed=record['after']['presses'][len(record['before']['presses']):]
  assert pressed and pressed[0]['trusted'] and pressed[0]['pointerType']=='mouse',(mode,'non-native input',pressed)
  assert record['renderDelta']<=3,(mode,'too many renders',record['renderDelta'])
  assert record['clickDelta']==(1 if mode=='mouse-tap' else 0),(mode,'wrong selection count')
  if mode!='mouse-tap':assert record['after']['camera']!=record['before']['camera'],(mode,'drag did not move map')
  assert record['released']['camera']==record['postRelease']['camera'],(mode,'map continued after mouse release')
 ab('screenshot',str(OUT/'camera-desktop.png'),'--full');report['passed']=True
except Exception as error:
 report['error']=str(error)
 try:ab('screenshot',str(OUT/'failure.png'),'--full')
 except Exception:pass
 raise
finally:
 (OUT/'result.json').write_text(json.dumps(report,indent=2)+'\n')
 try:ab('close')
 except Exception:pass
 server.shutdown();shutil.rmtree(OUT/('profile-'+run),ignore_errors=True)
print(json.dumps({'passed':True,'cases':[(r['mode'],r['renderDelta'],r['clickDelta']) for r in report['cases']]}))
