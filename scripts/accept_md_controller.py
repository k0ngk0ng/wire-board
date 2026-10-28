"""MD/FC component E2E, specified before the MD controller implementation.

Real React settings, native keyboard/mouse/touch; settings API is a fixture.
Actual HTTP and SQLite isolation are covered by TestController*E2E.
"""
from pathlib import Path
from http.server import BaseHTTPRequestHandler,ThreadingHTTPServer
import copy,json,os,subprocess,threading,uuid

ROOT=Path(__file__).resolve().parents[1]
OUT=ROOT/'.local/md-controller/browser';OUT.mkdir(parents=True,exist_ok=True)
env=dict(os.environ,AGENT_BROWSER_SESSION='md-'+uuid.uuid4().hex[:4],AGENT_BROWSER_SOCKET_DIR=str(ROOT/'.local/mds'),TMPDIR=str(ROOT/'.local/md-controller/tmp'))
for name in ['AGENT_BROWSER_SOCKET_DIR','TMPDIR']:Path(env[name]).mkdir(parents=True,exist_ok=True)
fc={'up':'KeyW','down':'KeyS','left':'KeyA','right':'KeyD','select':'KeyF','start':'Space','b':'KeyJ','a':'KeyK','turboB':'KeyU','turboA':'KeyI'}
md={'up':'KeyW','down':'KeyS','left':'KeyA','right':'KeyD','mode':'KeyF','start':'Space','b':'KeyJ','a':'KeyK','c':'KeyL','x':'KeyU','y':'KeyI','z':'KeyO'}
labels={'up':'上','down':'下','left':'左','right':'右','select':'选择（Select）','start':'开始（Start）','mode':'模式（Mode）','turboB':'Turbo B','turboA':'Turbo A',**{x:x.upper() for x in 'abcxyz'}}
models=[{'id':id,'name':name,'buttons':[{'id':k,'label':labels[k]}for k in keys],'defaults':{'revision':0,'turboHz':10,'keyboard':keys}}for id,name,keys in [('classic-2','经典双键（FC）',fc),('md-6','MD 六键手柄',md)]]
profiles={m['id']:copy.deepcopy(m['defaults'])for m in models}
class Handler(BaseHTTPRequestHandler):
 def log_message(self,*a):pass
 def send(self,data,mime='application/json'):
  b=data if isinstance(data,bytes)else json.dumps(data).encode();self.send_response(200);self.send_header('Content-Type',mime);self.send_header('Content-Length',str(len(b)));self.end_headers();self.wfile.write(b)
 def do_GET(self):
  if self.path=='/api/controllers':return self.send({'models':models,'profiles':profiles})
  if self.path=='/':return self.send(b'<!doctype html><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><link rel="stylesheet" href="/fixture.css"><div id="root"></div><script type="module" src="/fixture.js"></script>','text/html')
  relative=Path(self.path.lstrip('/'))
  if '..' in relative.parts or not (OUT/relative).is_file():self.send_error(404);return
  self.send((OUT/relative).read_bytes(),'image/svg+xml' if relative.suffix=='.svg' else 'text/css' if relative.suffix=='.css' else 'text/javascript')
 def do_POST(self):
  model=self.path.rsplit('/',1)[-1]
  if model not in profiles:self.send_error(404);return
  value=json.loads(self.rfile.read(int(self.headers['Content-Length'])));value['revision']=profiles[model]['revision']+1;profiles[model]=value;self.send(value)
def browser(*args):
 r=subprocess.run(['agent-browser',*args,'--json'],env=env,capture_output=True,text=True,timeout=50)
 if r.returncode:raise RuntimeError(r.stdout+r.stderr)
 result=json.loads(r.stdout)
 if not result.get('success'):raise RuntimeError(result)
 return result.get('data')
def js(code):return browser('eval',code)['result']
def held():return js('window.mdTest.sample()')
def native(actions):
 endpoint=browser('get','cdp-url');endpoint=endpoint.get('url')or endpoint.get('cdpUrl')or endpoint.get('value')
 r=subprocess.run(['node',str(ROOT/'scripts/controllers/md-touch.mjs'),endpoint,url,json.dumps(actions)],capture_output=True,text=True,timeout=25)
 if r.returncode:raise RuntimeError(r.stdout+r.stderr)
 return json.loads(r.stdout)
def key_event(key,down,expect):
 code='Space' if key=='Space' else 'Key'+key.upper()
 return {'method':'Input.dispatchKeyEvent','type':'keyDown' if down else 'keyUp','key':' ' if key=='Space' else key,'code':code,'windowsVirtualKeyCode':32 if key=='Space' else ord(key.upper()),'expect':expect}
def rectangle(selector):return js('(()=>{const r=document.querySelector('+json.dumps(selector)+').getBoundingClientRect();return {x:r.x,y:r.y,w:r.width,h:r.height}})()')
def point(r,x=.5,y=.5):return str(round(r['x']+r['w']*x)),str(round(r['y']+r['h']*y))
def geometry(selector):
 result=js('(()=>{const root=document.querySelector('+json.dumps(selector)+');return {overflow:document.documentElement.scrollWidth>innerWidth,buttons:[...root.querySelectorAll("[data-controller-button]")].map(e=>{const r=e.getBoundingClientRect(),hit=document.elementFromPoint(r.x+r.width/2,r.y+r.height/2);return {id:e.dataset.controllerButton,x:r.x,y:r.y,w:r.width,h:r.height,hit:e===hit||e.contains(hit)}})}})()')
 assert not result['overflow'] and all(b['hit'] for b in result['buttons']),result
 for i,a in enumerate(result['buttons']):
  for b in result['buttons'][i+1:]:assert min(a['x']+a['w'],b['x']+b['w'])<=max(a['x'],b['x'])+.2 or min(a['y']+a['h'],b['y']+b['h'])<=max(a['y'],b['y'])+.2,(a,b)
 return result
def contrast(selector):
 code=r'''(()=>{const rgb=s=>(s.match(/[\d.]+/g)||[]).map(Number),lum=c=>c.slice(0,3).map(x=>x/255).map(x=>x<=.04045?x/12.92:((x+.055)/1.055)**2.4).reduce((a,x,i)=>a+x*[.2126,.7152,.0722][i],0);return [...document.querySelectorAll(SELECTOR)].map(e=>{const s=getComputedStyle(e),fg=rgb(s.color),bg=rgb(s.backgroundColor);return {text:e.textContent,foreground:fg,background:bg,ratio:(Math.max(lum(fg),lum(bg))+.05)/(Math.min(lum(fg),lum(bg))+.05)}})})()'''.replace('SELECTOR',json.dumps(selector))
 result=js(code);assert result and all(r['ratio']>=4.5 for r in result),result;return result

subprocess.run([str(ROOT/'web/node_modules/.bin/esbuild'),'scripts/controllers/md-fixture.tsx','--bundle','--format=esm','--jsx=automatic','--alias:react=./web/node_modules/react','--alias:react-dom=./web/node_modules/react-dom','--loader:.svg=file','--outfile='+str(OUT/'fixture.js')],cwd=ROOT,env=env,check=True,capture_output=True)
server=ThreadingHTTPServer(('127.0.0.1',0),Handler);thread=threading.Thread(target=server.serve_forever,daemon=True);thread.start()
url='http://127.0.0.1:'+str(server.server_port)+'/'
opened=False;report={}
try:
 browser('--profile',str(OUT/'profile'),'open',url);opened=True;browser('wait','[data-controller-layout="md"]');browser('set','viewport','1100','820')
 assert js('document.querySelectorAll("[data-controller-layout=md] kbd").length')==0
 assert sorted(js('[...document.querySelectorAll("[data-controller-layout=md] [data-controller-button]")].map(e=>e.dataset.controllerButton)'))==sorted(md)
 keys=[]
 for key,expected in [('w','up'),('s','down'),('a','left'),('d','right'),('j','b'),('k','a'),('l','c'),('u','x'),('i','y'),('o','z'),('f','mode'),('Space','start')]:
  keys.extend([key_event(key,True,[expected]),key_event(key,False,[])])
 keys.extend([key_event('u',True,['x']),key_event('o',True,['x','z']),key_event('u',False,['z']),key_event('o',False,[])])
 (OUT/'native-keyboard.json').write_text(json.dumps(native(keys),indent=2)+'\n')
 report['native_keyboard_defaults_and_chords']=True
 browser('focus','#text-input');native([key_event('l',True,[]),key_event('l',False,[])]);browser('focus','#play-surface')
 d=rectangle('[data-controller-layout=md] [data-controller-dpad]')
 browser('mouse','move',*point(d,.15,.5));browser('mouse','down','left');assert held()==['left']
 browser('mouse','move',*point(d,.8,.2));assert held()==['right','up'];browser('mouse','move',*point(d,.5,.5));assert held()==[];browser('mouse','up','left')
 for control in ['disable','profile','unmount']:
  a=rectangle('[data-controller-layout=md] [data-controller-button=a]');browser('mouse','move',*point(a));browser('mouse','down','left');assert held()==['a']
  js('document.querySelector("#'+control+'").click()');assert held()==[];browser('mouse','up','left')
  if control!='profile':js('document.querySelector("#'+control+'").click()')
 report['pointer_slide_diagonal_and_cleanup']=True
 native([key_event('j',True,['b'])])
 c=rectangle('[data-controller-layout=md] [data-controller-button=c]');browser('mouse','move',*point(c));browser('mouse','down','left');assert held()==['b','c']
 (OUT/'native-visibility.json').write_text(json.dumps(native([{'method':'acceptance:hide','expect':[]},key_event('j',False,[])]),indent=2)+'\n')
 browser('mouse','up','left');report['mixed_input_and_native_visibility_release']=True
 (OUT/'contrast-play.json').write_text(json.dumps(contrast('[data-controller-layout=md] .md-control'),indent=2)+'\n')
 browser('screenshot',str(OUT/'play-desktop.png'));browser('set','viewport','320','720')
 layout=geometry('[data-controller-layout=md]');(OUT/'hitboxes-320.json').write_text(json.dumps(layout,indent=2)+'\n')
 d=rectangle('[data-controller-layout=md] [data-controller-dpad]');z=rectangle('[data-controller-layout=md] [data-controller-button=z]');c=rectangle('[data-controller-layout=md] [data-controller-button=c]')
 dx,dy=map(float,point(d,.15,.5));zx,zy=map(float,point(z));cx,cy=map(float,point(c));ux,uy=map(float,point(d,.8,.2))
 actions=[{'type':'touchStart','touchPoints':[{'id':1,'x':dx,'y':dy},{'id':2,'x':zx,'y':zy},{'id':3,'x':cx,'y':cy}],'expect':['c','left','z']},{'type':'touchMove','touchPoints':[{'id':1,'x':ux,'y':uy},{'id':2,'x':zx,'y':zy},{'id':3,'x':cx,'y':cy}],'expect':['c','right','up','z']},{'type':'touchCancel','touchPoints':[],'expect':[]}]
 cdp=browser('get','cdp-url');cdp=cdp.get('url')or cdp.get('cdpUrl')or cdp.get('value')
 touch=subprocess.run(['node',str(ROOT/'scripts/controllers/md-touch.mjs'),cdp,url,json.dumps(actions)],capture_output=True,text=True,timeout=25)
 assert touch.returncode==0,touch.stdout+touch.stderr
 (OUT/'native-touch.json').write_text(touch.stdout);report['native_three_finger_touch_and_cancel']=True
 browser('screenshot',str(OUT/'play-mobile-320.png'))
 browser('click','#settings');browser('wait','[aria-label="控制器型号"]');browser('select','[aria-label="控制器型号"]','md-6');browser('wait','.controller-settings [data-controller-layout=md]')
 assert js('[...document.querySelectorAll("[aria-label=控制器型号] option")].map(e=>e.value)')==['classic-2','md-6']
 assert js('document.querySelectorAll(".controller-turbo").length')==0
 assert js('document.querySelectorAll(".controller-settings [data-controller-layout=md] kbd").length')==12
 browser('click','[aria-label="修改 C 按键"]');native([key_event('o',True,[]),key_event('o',False,[])]);browser('wait','--text','每个按键只能对应一个操作');native([key_event('p',True,[]),key_event('p',False,[])]);browser('click','[data-controller-save]');browser('wait','--text','控制器设置已保存')
 assert profiles['md-6']['keyboard']['c']=='KeyP' and profiles['classic-2']['keyboard']==fc
 browser('select','[aria-label="控制器型号"]','classic-2');browser('wait','.controller-settings [data-controller-layout=fc]');assert js('document.querySelectorAll(".controller-turbo").length')==1
 browser('select','[aria-label="控制器型号"]','md-6');browser('find','role','button','click','--name','恢复此型号默认键位','--exact');browser('click','[data-controller-save]');browser('wait','--text','控制器设置已保存')
 assert profiles['md-6']['keyboard']==md
 browser('scrollintoview','.controller-settings [data-controller-layout=md]');geometry('.controller-settings [data-controller-layout=md]')
 (OUT/'contrast-settings.json').write_text(json.dumps(contrast('.controller-settings [data-controller-layout=md] kbd'),indent=2)+'\n')
 browser('screenshot',str(OUT/'settings-mobile-320.png'));browser('set','viewport','1100','900');browser('screenshot',str(OUT/'settings-desktop.png'))
 assert held()==[],held();assert js('window.mdTest.trusted.length>0 && window.mdTest.trusted.every(Boolean)')
 report.update(status='passed',settings_model_isolation=True,sfc_removed=True,default_keys=md,viewport_320=True,scope='Shared controller and settings components; not a playable Yu Yu Hakusho game')
 (OUT/'result.json').write_text(json.dumps(report,ensure_ascii=False,indent=2)+'\n');print(json.dumps(report,ensure_ascii=False))
except Exception as error:
 (OUT/'result.json').write_text(json.dumps({'status':'failed','error':repr(error),'completed':report},ensure_ascii=False,indent=2)+'\n')
 if opened:
  try:browser('screenshot',str(OUT/'failure.png'))
  except Exception:pass
 raise
finally:
 if opened:browser('close')
 server.shutdown();server.server_close();thread.join(timeout=5)
