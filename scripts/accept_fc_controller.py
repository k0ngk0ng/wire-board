"""Actual React FC/settings browser acceptance. Tests precede implementation.
Uses native mouse and CDP touch on its own headless browser, no game fixtures.
The API here is a settings fixture; real account persistence remains covered by
accept_controllers.py against the actual Go server.
"""
from pathlib import Path
from http.server import BaseHTTPRequestHandler,ThreadingHTTPServer
import argparse,json,os,re,subprocess,threading,uuid
parser=argparse.ArgumentParser();parser.add_argument('--label',default='browser');args=parser.parse_args();assert re.fullmatch(r'[a-z0-9-]+',args.label)
ROOT=Path(__file__).resolve().parents[1];OUT=ROOT/'.local/fc-controller'/args.label;OUT.mkdir(parents=True,exist_ok=True)
env=dict(os.environ,AGENT_BROWSER_SESSION='fc-'+uuid.uuid4().hex[:4],AGENT_BROWSER_SOCKET_DIR=str(ROOT/'.local/fcs'),TMPDIR=str(ROOT/'.local/tmp'));Path(env['AGENT_BROWSER_SOCKET_DIR']).mkdir(exist_ok=True)
def browser(*a):
 r=subprocess.run(['agent-browser',*a,'--json'],env=env,capture_output=True,text=True,timeout=90)
 if r.returncode:raise RuntimeError(r.stdout+r.stderr)
 d=json.loads(r.stdout)
 if not d.get('success'):raise RuntimeError(d)
 return d.get('data')
def js(s):return browser('eval',s)['result']
keys={'up':'KeyW','down':'KeyS','left':'KeyA','right':'KeyD','select':'KeyF','start':'Space','b':'KeyJ','a':'KeyK','turboB':'KeyU','turboA':'KeyI'};labels={'up':'上','down':'下','left':'左','right':'右','select':'选择（Select）','start':'开始（Start）','b':'B','a':'A','turboB':'Turbo B','turboA':'Turbo A','x':'X','y':'Y','l':'L','r':'R'}
models=[];profiles={}
for model,name,extra in [('classic-2','经典双键（FC）',{}),('classic-4','经典四键（SFC）',{'x':'KeyL','y':'KeyH','l':'KeyQ','r':'KeyE'})]:
 profile={'revision':0,'turboHz':10,'keyboard':{**keys,**extra}};profiles[model]=profile;models.append({'id':model,'name':name,'buttons':[{'id':k,'label':labels[k]} for k in profile['keyboard']],'defaults':profile})
class Handler(BaseHTTPRequestHandler):
 def log_message(self,*a):pass
 def send(self,data,mime='application/json'):
  raw=data if isinstance(data,bytes) else json.dumps(data).encode();self.send_response(200);self.send_header('Content-Type',mime);self.send_header('Content-Length',str(len(raw)));self.end_headers();self.wfile.write(raw)
 def do_GET(self):
  if self.path=='/api/controllers':return self.send({'models':models,'profiles':profiles})
  if self.path=='/':return self.send(b'<!doctype html><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><link rel="stylesheet" href="/fixture.css"><div id="root"></div><script type="module" src="/fixture.js"></script>','text/html')
  rel=Path(self.path.lstrip('/'))
  if '..' in rel.parts:self.send_error(404);return
  path=OUT/rel
  if not path.is_file():self.send_error(404);return
  self.send(path.read_bytes(),'image/svg+xml' if path.suffix=='.svg' else 'text/css' if path.suffix=='.css' else 'text/javascript')
 def do_POST(self):
  model=self.path.rsplit('/',1)[-1]
  if model not in profiles:self.send_error(404);return
  value=json.loads(self.rfile.read(int(self.headers['Content-Length'])));value['revision']=profiles[model]['revision']+1;profiles[model]=value;self.send(value)
subprocess.run([str(ROOT/'web/node_modules/.bin/esbuild'),str(ROOT/'scripts/controllers/fc-fixture.tsx'),'--bundle','--format=esm','--jsx=automatic','--alias:react=./web/node_modules/react','--alias:react-dom=./web/node_modules/react-dom','--loader:.svg=file','--outfile='+str(OUT/'fixture.js')],cwd=ROOT,env=env,check=True,capture_output=True)
server=ThreadingHTTPServer(('127.0.0.1',0),Handler);thread=threading.Thread(target=server.serve_forever,daemon=True);thread.start();url='http://127.0.0.1:'+str(server.server_port)+'/';opened=False
report={}
def rect(selector):return js('(()=>{const r=document.querySelector('+json.dumps(selector)+').getBoundingClientRect();return {x:r.x,y:r.y,w:r.width,h:r.height}})()')
def point(r,x=.5,y=.5):return str(round(r['x']+r['w']*x)),str(round(r['y']+r['h']*y))
def held():return sorted(js('[...new Set(Object.values(window.fcTest.held).flat())]'))
def contrast(scope):
 source=r"""(()=>{const root=document.querySelector(SCOPE),rgb=s=>(s.match(/[\d.]+/g)||[]).map(Number),lum=c=>c.slice(0,3).map(v=>v/255).map(v=>v<=.04045?v/12.92:((v+.055)/1.055)**2.4).reduce((s,v,i)=>s+v*[.2126,.7152,.0722][i],0);return [...root.querySelectorAll('kbd,.fc-play-direction')].map(e=>{let bg=[255,255,255],chain=[];for(let n=e;n;n=n.parentElement)chain.unshift(n);for(const n of chain){const c=rgb(getComputedStyle(n).backgroundColor),a=c[3]??1;bg=bg.map((v,i)=>c[i]*a+v*(1-a))}const fg=rgb(getComputedStyle(e).color),a=lum(fg),b=lum(bg);return {text:e.textContent,foreground:fg,background:bg,ratio:(Math.max(a,b)+.05)/(Math.min(a,b)+.05)}})})()""".replace('SCOPE',json.dumps(scope))
 result=js(source);assert result and all(v['ratio']>=4.5 for v in result),result;return result

def touch(actions):
 value=browser('get','cdp-url');cdp=value.get('url') or value.get('cdpUrl') or value.get('value')
 if not cdp:raise RuntimeError(value)
 result=subprocess.run(['node',str(ROOT/'scripts/controllers/fc-touch.mjs'),cdp,url,json.dumps(actions)],capture_output=True,text=True,timeout=20)
 if result.returncode:raise RuntimeError(result.stdout+result.stderr)
 (OUT/'native-touch.json').write_text(result.stdout)
try:
 browser('--profile',str(OUT/'profile'),'open',url);opened=True;browser('wait','[data-controller-layout="fc"]');browser('set','viewport','1100','820')
 expected_play={k:labels[k] for k in keys}
 play_labels=js('Object.fromEntries([...document.querySelectorAll("[data-controller-button]")].map(e=>[e.dataset.controllerButton,e.getAttribute("aria-label")]))');assert play_labels==expected_play,play_labels
 assert js('document.querySelectorAll("[data-controller-layout=fc] kbd").length')==0
 image=js('(()=>{const i=document.querySelector("[data-controller-layout=fc] img");return {complete:i?.complete,width:i?.naturalWidth,source:i?.src}})()');assert image['complete'] and image['width']>0,image
 faceplate=js('fetch(document.querySelector(".fc-background").src).then(r=>r.text())');assert 'Turbo B</text>' in faceplate and 'Turbo A</text>' in faceplate and '连发' not in faceplate;report['turbo_faceplate_labels']=True
 d=rect('[data-controller-dpad]');browser('mouse','move',*point(d,.15,.5));browser('mouse','down','left');assert held()==['left'],held()
 browser('mouse','move',*point(d,.8,.2));assert held()==['right','up'],held()
 browser('mouse','move',*point(d,.5,.5));assert held()==[],held()
 browser('mouse','move',*point(d,.5,.85));assert held()==['down'],held();browser('mouse','up','left');assert held()==[],held()
 report['native_mouse_drag_and_diagonal']=True
 a=rect('[data-controller-button="a"]');browser('mouse','move',*point(a));browser('mouse','down','left');assert held()==['a'];js('document.querySelector("#disable").click()');assert held()==[];browser('mouse','up','left');js('document.querySelector("#disable").click()')
 browser('mouse','down','left');js('document.querySelector("#profile").click()');assert held()==[];browser('mouse','up','left');assert js('document.querySelector("[data-controller-button=right]").textContent.trim()')=='→'
 browser('mouse','move',*point(a));browser('mouse','down','left');js('document.querySelector("#unmount").click()');assert held()==[];browser('mouse','up','left');js('document.querySelector("#unmount").click()');browser('wait','[data-controller-layout="fc"]')
 report['disabled_profile_unmount_release']=True
 (OUT/'contrast-play.json').write_text(json.dumps(contrast('[data-controller-layout=fc]'),indent=2)+'\n')
 browser('screenshot',str(OUT/'play-desktop.png'));browser('set','viewport','320','720')
 d=rect('[data-controller-dpad]');a=rect('[data-controller-button="a"]');dx,dy=map(float,point(d,.15,.5));ax,ay=map(float,point(a))
 ux,uy=map(float,point(d,.8,.2))
 touch([{'type':'touchStart','touchPoints':[{'id':1,'x':dx,'y':dy},{'id':2,'x':ax,'y':ay}],'expect':['a','left']},{'type':'touchMove','touchPoints':[{'id':1,'x':ux,'y':uy},{'id':2,'x':ax,'y':ay}],'expect':['a','right','up']},{'type':'touchCancel','touchPoints':[],'expect':[]}]);assert held()==[],held();report['native_multitouch_and_cancel']=True
 b=rect('[data-controller-button="b"]');bx,by=map(float,point(b))
 touch([{'type':'touchStart','touchPoints':[{'id':11,'x':bx,'y':by}],'expect':['b']},
        {'type':'touchStart','touchPoints':[{'id':11,'x':bx,'y':by},{'id':12,'x':ax,'y':ay}],'expect':['a','b']},
        {'type':'touchStart','touchPoints':[{'id':11,'x':bx,'y':by},{'id':12,'x':ax,'y':ay},{'id':13,'x':dx,'y':dy}],'expect':['a','b','left']},
        {'type':'touchEnd','touchPoints':[{'id':12,'x':ax,'y':ay}],'expect':['b','left']},
        {'type':'touchEnd','touchPoints':[{'id':13,'x':dx,'y':dy}],'expect':['b']},
        {'type':'touchCancel','touchPoints':[],'expect':[]}])
 report['native_a_b_and_direction_chords']=True
 layout=js('(()=>{const root=document.querySelector("[data-controller-layout=fc]"),r=root.getBoundingClientRect();return {overflow:document.documentElement.scrollWidth>innerWidth,width:r.width,right:r.right,buttons:[...root.querySelectorAll("[data-controller-button]")].map(e=>{const b=e.getBoundingClientRect(),hit=document.elementFromPoint(b.x+b.width/2,b.y+b.height/2);return {id:e.dataset.controllerButton,x:b.x,y:b.y,w:b.width,h:b.height,hit:e===hit||e.contains(hit)}})}})()');assert not layout['overflow'] and layout['right']<=320 and all(b['hit'] for b in layout['buttons']),layout
 physical=layout['buttons']
 by_id={b['id']:b for b in physical}
 for turbo,normal in [('turboB','b'),('turboA','a')]:
  a,b=by_id[turbo],by_id[normal];assert abs((a['x']+a['w']/2)-(b['x']+b['w']/2))<1 and a['y']+a['h']<=b['y']+.5,(a,b)
 assert js('document.querySelectorAll(".fc-extra-controls").length')==0
 for i,b in enumerate(physical):
  for c in physical[i+1:]:assert min(b['x']+b['w'],c['x']+c['w'])<=max(b['x'],c['x']) or min(b['y']+b['h'],c['y']+c['h'])<=max(b['y'],c['y']),(b,c)
 (OUT/'hitboxes-320.json').write_text(json.dumps(layout,indent=2)+'\n');browser('screenshot',str(OUT/'play-mobile-320.png'))
 browser('click','#settings');browser('wait','.controller-settings [data-controller-layout="fc"]');assert js('document.querySelectorAll(".controller-settings [data-controller-layout=fc] kbd").length')==10;assert js('document.querySelectorAll("[aria-label=\\"修改 右 按键\\"]").length')==1
 browser('click','[aria-label="修改 右 按键"]');browser('press','w');browser('wait','--text','每个按键只能对应一个操作');browser('press','ArrowRight');browser('click','[data-controller-save]');browser('wait','--text','控制器设置已保存')
 (OUT/'contrast-settings.json').write_text(json.dumps(contrast('.controller-settings [data-controller-layout=fc]'),indent=2)+'\n')
 assert js('["修改 Turbo B 按键","修改 Turbo A 按键"].every(label=>[...document.querySelectorAll(".controller-settings button")].some(e=>e.getAttribute("aria-label")===label))');assert profiles['classic-2']['keyboard']['right']=='ArrowRight';browser('screenshot',str(OUT/'settings-mobile-320.png'))
 browser('select','[aria-label="控制器型号"]','classic-4');assert js('document.querySelectorAll(".controller-settings [data-controller-layout=fc]").length')==0
 browser('select','[aria-label="控制器型号"]','classic-2');browser('set','viewport','1100','900');browser('screenshot',str(OUT/'settings-desktop.png'));report['settings_remap_save_model_isolation']=True
 assert all(js('window.fcTest.trusted')), 'Synthetic pointer events were used'
 report.update(status='passed',background=image,logical_keys=list(keys),native_pointer_events=True,viewport_320=True);(OUT/'result.json').write_text(json.dumps(report,indent=2,ensure_ascii=False)+'\n');print(json.dumps(report,ensure_ascii=False))
except Exception as e:
 (OUT/'result.json').write_text(json.dumps({'status':'failed','error':repr(e),'completed':report},ensure_ascii=False,indent=2)+'\n')
 if opened:
  try:browser('screenshot',str(OUT/'failure.png'))
  except Exception:pass
 raise
finally:
 if opened:browser('close')
 server.shutdown();server.server_close();thread.join(timeout=5)
