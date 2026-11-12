"""Production Chat/GameFullscreen; all drag failures declared before implementation.

Mouse/touch do not move; drag opens chat or reaches game controls; capture/cancel
sticks; next click is lost; panel/draft/pending sends remount; native/page differ;
rotation/viewport shrink loses the entry or panel; normal layout is changed.
Transport is a controlled local sink: no messages are sent to real people.
"""
from pathlib import Path
import argparse,functools,hashlib,http.server,json,os,shutil,subprocess,threading
ROOT=Path(__file__).resolve().parents[2]
p=argparse.ArgumentParser();p.add_argument('--out',required=True);a=p.parse_args();OUT=(ROOT/a.out).resolve();assert OUT.is_relative_to(ROOT)and not OUT.exists();OUT.mkdir(parents=True)
TMP=OUT/'temporary';TMP.mkdir();common=ROOT.parents[1]if ROOT.parent.name=='worktrees'else ROOT
env=dict(os.environ,AGENT_BROWSER_SESSION='cd'+hashlib.sha256(a.out.encode()).hexdigest()[:6],AGENT_BROWSER_SOCKET_DIR=str(common/'.local/cd'),TMPDIR=str(TMP))
report={'passed':False,'cases':[],'scope':'Real shared components; native mouse/touch; simulated incoming/pending transport only'}
def b(*args):
 r=subprocess.run(['agent-browser',*args,'--json'],cwd=ROOT,env=env,text=True,capture_output=True,timeout=40);v=json.loads(r.stdout);assert r.returncode==0 and v['success'],r.stdout+r.stderr;return v['data']
def js(code):return b('eval',code)['result']
def wait(code):return b('wait','--fn',code)
def click(selector,mobile):
 r=subprocess.run(['node','scripts/fullscreen/tap.mjs',b('get','cdp-url')['cdpUrl'],url,selector,'touch'if mobile else'mouse'],cwd=ROOT,env=env,capture_output=True,text=True,timeout=20);assert r.returncode==0,r.stdout+r.stderr
def drag(dx,dy,mobile,end='up'):
 r=subprocess.run(['node','scripts/fullscreen/drag.mjs',b('get','cdp-url')['cdpUrl'],url,'.chat-toggle','touch'if mobile else'mouse',str(dx),str(dy),end],cwd=ROOT,env=env,capture_output=True,text=True,timeout=20);assert r.returncode==0,r.stdout+r.stderr;return json.loads(r.stdout)
def visible():return js('''(()=>{const selectors=['.chat-toggle',...(document.querySelector('.chat-panel')?['.chat-panel','.chat-heading button','textarea','.chat-compose button']:[])];return selectors.map(s=>{const e=document.querySelector(s),r=e.getBoundingClientRect();return {selector:s,left:r.left,top:r.top,right:r.right,bottom:r.bottom,width:innerWidth,height:innerHeight,hit:e.contains(document.elementFromPoint(r.x+r.width/2,r.y+r.height/2))};});})()''')
def fits(items):return all(i['left']>=0 and i['top']>=0 and i['right']<=i['width']+.5 and i['bottom']<=i['height']+.5 for i in items)
server=None
try:
 subprocess.run(['node','scripts/fullscreen/build-chat-harness.mjs',str(TMP/'web')],cwd=ROOT,env=env,capture_output=True,text=True,check=True)
 class Handler(http.server.SimpleHTTPRequestHandler):
  def log_message(self,*args):pass
 server=http.server.ThreadingHTTPServer(('127.0.0.1',0),functools.partial(Handler,directory=str(TMP/'web')));threading.Thread(target=server.serve_forever,daemon=True).start();url='http://127.0.0.1:'+str(server.server_port)+'/'
 for mobile in[False,True]:
  for mode in['native','page']:
   case={'mobile':mobile,'mode':mode,'checks':[]};report['cases'].append(case)
   b('open',url);b('set','viewport','320'if mobile else'1280','740'if mobile else'900');b('wait','.chat-toggle')
   click('.chat-toggle',mobile);b('fill','textarea','保留的草稿');click('.chat-toggle',mobile);js('chatFixture.incoming("未读");window.originalChat=document.querySelector(".table-chat");true')
   b('click','[data-start]');b('wait','[data-enter]:not([disabled])')
   if mode=='page':js('document.querySelector("[data-game-fullscreen-surface]").requestFullscreen=()=>Promise.reject(Error("test denied"));true')
   click('[data-enter]',mobile);wait('document.querySelector("[data-room-fullscreen]")?.dataset.roomFullscreen==='+json.dumps(mode))
   moved=drag(-90 if mobile else -240,-150,mobile);case['drag']=moved
   assert moved['after']['x']<moved['before']['x']-50 and moved['after']['y']<moved['before']['y']-100,moved
   assert not moved['after']['panel'];assert js('document.querySelector(".chat-toggle b")?.textContent==="1"&&window.originalChat===document.querySelector(".table-chat")')
   case['checks'].append('native drag moves without opening or losing unread state')
   click('.chat-toggle',mobile);assert js('document.querySelector("textarea").value==="保留的草稿"');js('window.originalInput=document.querySelector("textarea");true')
   before=js('document.querySelector("textarea").value');movedOpen=drag(35,40,mobile);assert movedOpen['after']['panel']
   assert js('window.originalInput===document.querySelector("textarea")&&document.querySelector("textarea").value==='+json.dumps(before))
   geom=visible();assert fits(geom),geom;assert all(i['hit']for i in geom if i['selector']!='.chat-panel'),geom
   case['checks'].append('open panel stays visible and preserves input node/draft');case['geometry']=geom
   b('screenshot',str(OUT/(mode+('-mobile'if mobile else'-desktop')+'.png')))
   b('fill','textarea','延迟发送');b('press','Enter');wait('chatFixture.sent.length===1');drag(-20,20,mobile);b('focus','textarea');b('press','Enter');assert js('chatFixture.sent.length')==1
   js('chatFixture.finishSend();true');wait('document.querySelector("textarea").value===""');case['checks'].append('drag retains pending-send guard')
   click('.chat-toggle',mobile);boundary=drag(-200,-250,mobile);assert fits(visible());case['boundary']=boundary
   b('set','viewport','740'if mobile else'640','360');wait('(()=>{const r=document.querySelector(".chat-toggle").getBoundingClientRect();return r.left>=0&&r.top>=0&&r.right<=innerWidth+.5&&r.bottom<=innerHeight+.5})()');assert fits(visible());click('.chat-toggle',mobile);assert fits(visible());click('.chat-toggle',mobile)
   case['checks'].append('viewport resize clamps entry and composer')
   cancelled=drag(35,20,mobile,'cancel'if mobile else'up');assert not cancelled['after']['panel'];click('.chat-toggle',mobile);assert js('!!document.querySelector(".chat-panel")');click('.chat-toggle',mobile)
   case['checks'].append('cancel/drag does not swallow the next click')
   click('[data-fullscreen-toggle]',mobile);wait('!document.querySelector("[data-room-fullscreen]")');assert js('getComputedStyle(document.querySelector(".table-chat")).position')=='relative'
   case['checks'].append('exit restores normal viewport layout')
   assert not b('errors')['errors'];case['passed']=True;b('close')
 report['passed']=True
except BaseException as error:
 report['error']=repr(error)
 try:b('screenshot',str(OUT/'failure.png'))
 except Exception:pass
 raise
finally:
 try:b('close')
 except Exception:pass
 if server:server.shutdown();server.server_close()
 report['sources']={str(p.relative_to(ROOT)):hashlib.sha256(p.read_bytes()).hexdigest()for p in[Path(__file__),ROOT/'scripts/fullscreen/drag.mjs',ROOT/'web/src/chat.tsx',ROOT/'web/src/ui/room-fullscreen.css']}
 (OUT/'result.json').write_text(json.dumps(report,ensure_ascii=False,indent=2)+'\n');shutil.rmtree(TMP)
 print(json.dumps({'passed':report['passed'],'cases':len(report['cases']),'result':str(OUT/'result.json')},ensure_ascii=False))
