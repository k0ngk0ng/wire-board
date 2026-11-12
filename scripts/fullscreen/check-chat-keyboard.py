#!/usr/bin/env python3
"""CHAT-KEYBOARD-SCENARIOS.md: real shared components and native input.
Soft-keyboard VisualViewport geometry is simulated; this is not physical iPad/Safari QA.
"""
import argparse,functools,hashlib,http.server,json,os,shutil,subprocess,threading,traceback,uuid
from pathlib import Path
ROOT=Path(__file__).resolve().parents[2]
p=argparse.ArgumentParser();p.add_argument('--out',required=True);p.add_argument('--baseline',action='store_true');a=p.parse_args()
OUT=(ROOT/a.out).resolve();assert OUT.is_relative_to(ROOT)and not OUT.exists();OUT.mkdir(parents=True)
TMP=OUT/'temporary';TMP.mkdir();session='ck'+uuid.uuid4().hex[:7]
env=dict(os.environ,AGENT_BROWSER_SESSION=session,AGENT_BROWSER_SOCKET_DIR=str(ROOT/'.local/ck'),AGENT_BROWSER_PROFILE=str(TMP/'profile'),TMPDIR=str(TMP),AGENT_BROWSER_ARGS='--mute-audio,--disable-breakpad,--disable-crash-reporter')
Path(env['AGENT_BROWSER_SOCKET_DIR']).mkdir(exist_ok=True)
report={'passed':False,'baseline':a.baseline,'cases':[],'checks':[],'failures':[],'scope':__doc__};server=None

def b(*args):
 r=subprocess.run(['agent-browser','--session',session,*map(str,args),'--json'],cwd=ROOT,env=env,text=True,capture_output=True,timeout=45)
 v=json.loads(r.stdout);assert r.returncode==0 and v['success'],r.stdout+r.stderr;return v['data']
def js(code):return b('eval',code)['result']
def wait(code):b('wait','--fn',code)
def check(name,ok,evidence=None):
 row={'name':name,'passed':bool(ok),'evidence':evidence};report['checks'].append(row)
 if not ok:report['failures'].append(row)
 print(('PASS 'if ok else'FAIL ')+name,flush=True)
 if not a.baseline:assert ok,(name,evidence)
def click(selector):
 r=subprocess.run(['node','scripts/fullscreen/tap.mjs',b('get','cdp-url')['cdpUrl'],url,selector,'touch'],cwd=ROOT,env=env,capture_output=True,text=True,timeout=20);assert r.returncode==0,r.stderr

def geometry():
 return js('''(()=>{const v=visualViewport,selectors=['.chat-panel','.chat-heading button','textarea','.chat-compose button'];return {viewport:{left:v.offsetLeft,top:v.offsetTop,right:v.offsetLeft+v.width,bottom:v.offsetTop+v.height},items:selectors.map(selector=>{const e=document.querySelector(selector),r=e.getBoundingClientRect(),points=[[r.left+r.width/2,r.top+r.height/2],[r.left+5,r.bottom-5],[r.right-5,r.bottom-5]];return {selector,rect:r.toJSON(),hit:points.every(([x,y])=>e.contains(document.elementFromPoint(x,y)))}})}})()''')
def fits(g):
 v=g['viewport'];return all(i['rect']['left']>=v['left']-1 and i['rect']['top']>=v['top']-1 and i['rect']['right']<=v['right']+1 and i['rect']['bottom']<=v['bottom']+1 for i in g['items'])
def keyboard(height,offset,shrink):
 js('window.__chatKeyboard('+json.dumps({'height':height,'offsetTop':offset,'shrink':shrink})+')')
 js('new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r)))')
try:
 subprocess.run(['node','scripts/fullscreen/build-chat-harness.mjs',str(TMP/'web')],cwd=ROOT,env=env,capture_output=True,text=True,check=True)
 class Handler(http.server.SimpleHTTPRequestHandler):
  def log_message(self,*args):pass
 server=http.server.ThreadingHTTPServer(('127.0.0.1',0),functools.partial(Handler,directory=str(TMP/'web')));threading.Thread(target=server.serve_forever,daemon=True).start();url='http://127.0.0.1:'+str(server.server_port)+'/'
 for width,height in [(1024,768),(768,1024),(320,740)]:
  for mode in ['native','page']:
   name=f'{width}-{mode}';case={'name':name};report['cases'].append(case)
   b('open',url);b('set','viewport',width,height);b('wait','.chat-toggle');normal_position=js('getComputedStyle(document.querySelector(".table-chat")).position');b('click','[data-start]');b('wait','[data-enter]:not([disabled])')
   if mode=='page':js('document.querySelector("[data-game-fullscreen-surface]").requestFullscreen=()=>Promise.reject(Error("page fallback fixture"))')
   click('[data-enter]');wait('document.querySelector("[data-room-fullscreen]")?.dataset.roomFullscreen==='+json.dumps(mode))
   click('.chat-toggle');wait('!!document.querySelector("textarea")');b('fill','textarea','键盘弹出后仍可输入的草稿')
   js('''(()=>{const v=visualViewport,initial={height:v.height,width:v.width,offsetTop:v.offsetTop,offsetLeft:v.offsetLeft},metric={...initial};window.__originalChatInput=document.querySelector('textarea');for(const key of Object.keys(metric))Object.defineProperty(v,key,{configurable:true,get:()=>key==='width'?innerWidth:metric[key]});window.__chatKeyboard=({height,offsetTop,shrink})=>{metric.height=height;metric.offsetTop=offsetTop;const root=document.querySelector('[data-room-fullscreen]');if(shrink)root.style.setProperty('height',height+'px','important');else root.style.removeProperty('height');v.dispatchEvent(new Event('resize'));v.dispatchEvent(new Event('scroll'))};window.__chatKeyboardReset=()=>{Object.assign(metric,initial);document.querySelector('[data-room-fullscreen]').style.removeProperty('height');v.dispatchEvent(new Event('resize'));v.dispatchEvent(new Event('scroll'))};return true})()''')
   for visible,offset in [(360,min(330,height-360)),(260,min(410,height-260))]:
    keyboard(visible,offset,mode=='page');g=geometry();case.setdefault('geometry',[]).append(g)
    check(name+' composer stays in keyboard viewport '+str(visible),fits(g),g)
    check(name+' panel does not collapse '+str(visible),g['items'][0]['rect']['height']>=min(180,visible-40),g)
    check(name+' input/send/close are hittable '+str(visible),all(i['hit']for i in g['items']if i['selector']!='.chat-panel'),g)
    if not a.baseline:
     b('focus','textarea');b('press','a');check(name+' native typing survives '+str(visible),js('document.querySelector("textarea").value.endsWith("a")&&window.__originalChatInput===document.querySelector("textarea")'))
    b('screenshot',OUT/(name+'-keyboard-'+str(visible)+'.png'))
   keyboard(0,0,mode=='page');check(name+' transient zero height does not collapse input',js('document.querySelector("textarea").getBoundingClientRect().height>=40&&window.__originalChatInput===document.querySelector("textarea")'))
   js('window.__chatKeyboardReset()');js('new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r)))');g=geometry()
   check(name+' keyboard dismissal restores panel',fits(g)and g['items'][0]['rect']['height']>=250,g)
   if not a.baseline:
    b('set','viewport',height,width);keyboard(min(360,width),0,False);g=geometry();check(name+' rotation keeps composer usable',fits(g)and all(i['hit']for i in g['items']if i['selector']!='.chat-panel'),g)
    b('set','viewport',width,height);js('window.__chatKeyboardReset()');js('new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r)))')
   check(name+' draft and input node survive',js('window.__originalChatInput===document.querySelector("textarea")&&document.querySelector("textarea").value.startsWith("键盘弹出后仍可输入的草稿")'))
   if not a.baseline:
    b('fill','textarea','键盘后的发送');b('press','Enter');wait('chatFixture.sent.length===1');keyboard(360,min(200,height-360),False);js('window.__chatKeyboardReset()');b('focus','textarea');b('press','Enter');check(name+' viewport change preserves send guard',js('chatFixture.sent.length===1'));js('chatFixture.finishSend()');wait('document.querySelector("textarea").value===""')
    click('.chat-heading button');wait('!document.querySelector(".chat-panel")');click('.chat-toggle');wait('!!document.querySelector("textarea")');check(name+' reopened chat remains usable',fits(geometry()))
    click('.chat-heading button');click('[data-fullscreen-toggle]');wait('!document.querySelector("[data-room-fullscreen]")');check(name+' normal page restored',js('!document.querySelector("[inert]")&&!document.querySelector(".table-chat").closest("[data-room-fullscreen]")&&getComputedStyle(document.querySelector(".table-chat")).position==='+json.dumps(normal_position)))
   b('screenshot',OUT/(name+'-restored.png'));b('close')
 report['passed']=not report['failures']
except Exception as e:
 report['error']=str(e);traceback.print_exc()
 try:b('screenshot',OUT/'failure.png')
 except Exception:pass
finally:
 try:b('close')
 except Exception:pass
 if server:server.shutdown();server.server_close()
 report['sources']={str(p.relative_to(ROOT)):hashlib.sha256(p.read_bytes()).hexdigest()for p in [Path(__file__),ROOT/'web/src/chat.tsx',ROOT/'web/src/ui/chat-position.ts',ROOT/'web/src/ui/room-fullscreen.css']}
 (OUT/'result.json').write_text(json.dumps(report,ensure_ascii=False,indent=2)+'\n');shutil.rmtree(TMP)
 print(json.dumps({'passed':report['passed'],'checks':len(report['checks']),'failures':len(report['failures'])}))
 sys_exit=0 if report['passed']or a.baseline else 1
 raise SystemExit(sys_exit)
