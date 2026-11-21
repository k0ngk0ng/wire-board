#!/usr/bin/env python3
"""Render all art directions and verify prototype controls, not production gameplay."""
from pathlib import Path
import functools,http.server,threading,subprocess,os,json,hashlib,shutil
ROOT=Path(__file__).resolve().parents[3];HERE=Path(__file__).resolve().parent;OUT=HERE/'screenshots';AREA=ROOT/'.local/table-tennis-design';PROFILE=AREA/'profile';SOCKET=ROOT/'.local/ttb'
for p in [OUT,AREA,PROFILE,SOCKET,AREA/'tmp']:p.mkdir(parents=True,exist_ok=True)
ENV=dict(os.environ,AGENT_BROWSER_SESSION='tt-art',AGENT_BROWSER_SOCKET_DIR=str(SOCKET),AGENT_BROWSER_PROFILE=str(PROFILE),TMPDIR=str(AREA/'tmp'),AGENT_BROWSER_ARGS='--disable-breakpad,--disable-crash-reporter')
class Quiet(http.server.SimpleHTTPRequestHandler):
 def log_message(self,*args):pass
server=http.server.ThreadingHTTPServer(('127.0.0.1',0),functools.partial(Quiet,directory=str(ROOT)));threading.Thread(target=server.serve_forever,daemon=True).start();BASE='http://127.0.0.1:'+str(server.server_port)+'/docs/prototypes/table-tennis/'
report={'passed':False,'scope':'3D art/gesture prototype only; native Chromium mouse/keyboard/CDP touch; mobile viewport emulation','checks':[],'screenshots':[]}
def browser(*args):
 r=subprocess.run(['agent-browser',*[str(round(x)) if isinstance(x,float) else str(x) for x in args],'--json'],cwd=ROOT,env=ENV,capture_output=True,text=True,timeout=45)
 if r.returncode:raise RuntimeError(r.stdout+r.stderr)
 d=json.loads(r.stdout);assert d['success'],d;return d['data']
def js(code):return browser('eval',code)['result']
try:
 for theme in ['club','arena','heritage']:
  for label,w,h in [('desktop',1440,1000),('mobile',390,844),('narrow',320,760)]:
   browser('set','viewport',w,h);url=BASE+'index.html?style='+theme+'&view=preview&still=1';browser('open',url);browser('wait','--fn','window.__prototype?.frame > 2')
   state=js('({prototype:window.__prototype,overflow:document.documentElement.scrollWidth>innerWidth,frame:document.querySelector(".frame").getBoundingClientRect().toJSON(),gesture:document.querySelector("#court").getBoundingClientRect().toJSON(),extraControls:document.querySelectorAll(".control,#replay,#gesture").length,canvas:document.querySelector("canvas").getBoundingClientRect().toJSON()})')
   assert state['prototype']['perspective']=='first-person' and state['prototype']['selfBodyVisible']==False and state['extraControls']==0
   assert state['prototype']['rackets']==4 and state['prototype']['bodyMovementRequired']==False
   assert state['prototype']['players']==4 and state['prototype']['theme']==theme and not state['overflow'],state
   assert state['gesture']['bottom']<=h and state['canvas']['height']>300,state
   destination=OUT/('rackets-'+theme+'-'+label+'.png');browser('screenshot',destination)
   report['screenshots'].append({'file':str(destination.relative_to(ROOT)),'width':w,'height':h,'geometry':state})
   report['checks'].append(theme+'-'+label+'-first-person-no-extra-controls-no-overflow')
   g=state['gesture'];x=g['x']+g['width']*.6;y=g['y']+g['height']*.7
   if label=='desktop':
    browser('mouse','move',x,y);browser('mouse','down');browser('mouse','move',x,y-30);browser('mouse','up');assert js('window.__prototype.shots.at(-1)')=='抽球';report['checks'].append(theme+'-native-mouse-drive')
    browser('click','#court');browser('press','ArrowDown');assert js('window.__prototype.shots.at(-1)')=='削球';report['checks'].append(theme+'-keyboard-chop')
   if label=='mobile':
    ws=browser('get','cdp-url');ws=ws if isinstance(ws,str) else ws.get('url',ws.get('cdpUrl',ws.get('value')))
    assert ws, 'Missing CDP URL'
    r=subprocess.run(['node',str(HERE/'native-touch.mjs'),ws,url,json.dumps([[x,y-30],[x,y-20],[x,y-10],[x,y]])],capture_output=True,text=True,check=True,timeout=20)
    assert js('window.__prototype.shots.at(-1)')=='削球';report['checks'].append(theme+'-native-touch-chop')
    before=js('window.__prototype.shots.length');browser('click','#court');assert js('window.__prototype.shots.length')==before+1;report['checks'].append(theme+'-direct-court-tap')
 browser('set','viewport',1440,1000);browser('open',BASE+'index.html?still=1');browser('wait','--fn','window.__prototype?.ready')
 for theme in ['arena','heritage','club']:
  browser('click','[data-theme="'+theme+'"]');assert js('window.__prototype.theme')==theme;report['checks'].append('native-theme-switch-'+theme)
 browser('screenshot',OUT/'rackets-interactive-overview.png');report['passed']=True
finally:
 try:browser('close')
 except Exception:pass
 server.shutdown();server.server_close();shutil.rmtree(PROFILE,ignore_errors=True)
 report['sourceSHA256']={p.name:hashlib.sha256(p.read_bytes()).hexdigest() for p in [HERE/'shell.html',HERE/'scene.js',HERE/'index.html',Path(__file__),HERE/'native-touch.mjs']}
 (AREA/'rackets-verification.json').write_text(json.dumps(report,ensure_ascii=False,indent=2)+'\n')
 print(json.dumps({'passed':report['passed'],'checks':len(report['checks']),'screenshots':len(report['screenshots'])}))
