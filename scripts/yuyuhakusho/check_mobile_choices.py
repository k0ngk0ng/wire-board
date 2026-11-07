"""Actual Options/Preparation components with native touch and mouse.

Declared failure cases: touch toggle/blur closes before selecting; image/label
taps miss; outside/ESC fail to close; a scroll commits an option; disabled
controls change state; duplicate submission; mobile popup clips or overflows.
This targeted fixture isolates UI transport. Real room persistence is checked
separately against the normal authenticated application.
"""
from pathlib import Path
import argparse, functools, hashlib, http.server, json, os, subprocess, threading

ROOT=Path(__file__).resolve().parents[2]
p=argparse.ArgumentParser();p.add_argument('--label',required=True);a=p.parse_args()
OUT=ROOT/'games/yuyuhakusho/verification/mobile-choices'/a.label
TMP=ROOT/'.local/yuyu-mobile'/a.label
OUT.mkdir(parents=True,exist_ok=False);TMP.mkdir(parents=True,exist_ok=False)
common=(ROOT.parents[1] if ROOT.parent.name=='worktrees' else ROOT)
env=dict(os.environ,AGENT_BROWSER_SESSION='ym'+hashlib.sha256(a.label.encode()).hexdigest()[:6],AGENT_BROWSER_SOCKET_DIR=str(common/'.local/ym'),TMPDIR=str(TMP))
report={'passed':False,'scope':'Actual production choice components; native CDP touch, real browser focus, local event sink','checks':[],'nativeTouches':[]}
def b(*args):
 r=subprocess.run(['agent-browser',*args,'--json'],cwd=ROOT,env=env,text=True,capture_output=True,timeout=40)
 v=json.loads(r.stdout);assert r.returncode==0 and v['success'],r.stdout+r.stderr;return v['data']
def js(code):return b('eval',code)['result']
def check(name,condition):
 assert condition,name;report['checks'].append(name)
def tap(selector):
 js('document.querySelector('+json.dumps(selector)+').scrollIntoView({block:"center"});true')
 box=js('(()=>{const e=document.querySelector('+json.dumps(selector)+'),r=e.getBoundingClientRect();return {x:r.x+r.width/2,y:r.y+r.height/2,hit:e.contains(document.elementFromPoint(r.x+r.width/2,r.y+r.height/2))}})()')
 assert box['hit'],(selector,box)
 endpoint=b('get','cdp-url')['cdpUrl']
 steps=[{'type':'touchStart','touchPoints':[{'x':box['x'],'y':box['y']}],'wait':70},{'type':'touchEnd','touchPoints':[],'wait':100}]
 r=subprocess.run(['node',str(ROOT/'scripts/controllers/gamepad/native-touch.mjs'),endpoint,url,json.dumps(steps),'main'],cwd=ROOT,env=env,text=True,capture_output=True,timeout=20)
 assert r.returncode==0,r.stdout+r.stderr
 report['nativeTouches'].append({'selector':selector,'point':box,'events':json.loads(r.stdout)})
def popup(kind,count):
 check(kind+' choices stay open after tap',js('document.querySelectorAll('+json.dumps('[data-'+kind+'-option]')+').length')==count)
def geometry():
 return js('(()=>{const r=document.querySelector(".yuyu-choice-grid").getBoundingClientRect();return {left:r.left,right:r.right,top:r.top,bottom:r.bottom,width:innerWidth,height:innerHeight,scroll:document.documentElement.scrollWidth}})()')

host=None
try:
 subprocess.run([str(ROOT/'web/node_modules/.bin/esbuild'),'scripts/yuyuhakusho/mobile_choices_fixture.tsx','--bundle','--format=esm','--jsx=automatic','--alias:react=./web/node_modules/react','--alias:react-dom=./web/node_modules/react-dom','--loader:.png=file','--outfile='+str(TMP/'fixture.js')],cwd=ROOT,env=env,stdout=(OUT/'build.log').open('w'),stderr=subprocess.STDOUT,check=True)
 (TMP/'index.html').write_text('<!doctype html><html lang="zh-CN"><meta name="viewport" content="width=device-width,initial-scale=1"><link rel="stylesheet" href="fixture.css"><style>body{margin:0;font:16px system-ui}button{font:inherit}*{box-sizing:border-box}</style><div id="root"></div><script type="module" src="fixture.js"></script></html>')
 class Handler(http.server.SimpleHTTPRequestHandler):
  def log_message(self,*args):pass
 host=http.server.ThreadingHTTPServer(('127.0.0.1',0),functools.partial(Handler,directory=str(TMP)))
 threading.Thread(target=host.serve_forever,daemon=True).start();url='http://127.0.0.1:'+str(host.server_port)+'/'
 b('open',url);b('set','viewport','320','740');b('wait','[data-arena-picker] summary')
 tap('[data-arena-picker] summary');popup('arena',8)
 g=geometry();check('create popup fits mobile viewport',g['left']>=0 and g['right']<=320 and g['top']>=0 and g['bottom']<=740 and g['scroll']<=320)
 b('screenshot',str(OUT/'mobile-create-open.png'))
 tap('[data-arena-option="10"] .yuyu-arena-image');check('touching arena image selects once',js('document.querySelector("[data-arena-picker]").dataset.arena')=='10' and js('mobileChoiceEvents.length')==1)
 b('find','role','button','click','--name','准备','--exact')
 tap('[data-character-seat="0"] summary');popup('character',11)
 tap('[data-character-option="8"] .yuyu-portrait');check('touching own character image selects once',js('document.querySelector("[data-character-seat]").dataset.character')=='8' and js('mobileChoiceEvents.length')==2)
 tap('[data-arena-picker] summary');popup('arena',8)
 tap('[data-arena-option="4"] span:last-child');check('waiting arena label selects',js('document.querySelector("[data-arena-picker]").dataset.arena')=='4')
 tap('[data-character-seat="1"] summary');popup('character',11)
 tap('[data-character-option="10"]');check('host selects bot character',js('document.querySelectorAll("[data-character-seat]")[1].dataset.character')=='10')
 tap('[data-character-seat="0"] summary');popup('character',11)
 # WebKit can report a touch-related focusout with no next focus target.
 # Exercise a real browser blur, never a fabricated pointer/click event.
 js('document.activeElement.blur();true');popup('character',11)
 js('document.querySelector(".yuyu-choice-grid [aria-pressed=true]").focus();document.body.tabIndex=-1;document.body.focus();true');popup('character',11)
 tap('[data-character-option="1"]');check('null-target touch blur preserves selection',js('document.querySelector("[data-character-seat]").dataset.character')=='1')
 tap('[data-arena-picker] summary');tap('#outside');check('outside touch dismisses menu',js('document.querySelectorAll("[data-arena-option]").length')==0)
 tap('[data-arena-picker] summary');b('press','Escape');check('Escape dismisses menu',js('document.querySelectorAll("[data-arena-option]").length')==0)
 b('set','viewport','320','360');tap('[data-arena-picker] summary');popup('arena',8)
 count=js('mobileChoiceEvents.length');box=geometry();x=(box['left']+box['right'])/2;y=box['top']+min(80,(box['bottom']-box['top'])-10)
 steps=[{'type':'touchStart','touchPoints':[{'x':x,'y':y}],'wait':70}]+[{'type':'touchMove','touchPoints':[{'x':x,'y':y-step}],'wait':30}for step in [10,25,45]]+[{'type':'touchEnd','touchPoints':[],'wait':100}]
 endpoint=b('get','cdp-url')['cdpUrl'];subprocess.run(['node',str(ROOT/'scripts/controllers/gamepad/native-touch.mjs'),endpoint,url,json.dumps(steps),'main'],cwd=ROOT,env=env,capture_output=True,text=True,check=True)
 check('native menu scroll does not select',js('mobileChoiceEvents.length')==count);b('press','Escape');b('set','viewport','320','740')
 b('find','role','button','click','--name','切换禁用','--exact');tap('[data-arena-picker] summary');check('disabled field does not open',js('document.querySelectorAll("[data-arena-option]").length')==0)
 b('screenshot',str(OUT/'mobile-selected.png'))
 b('find','role','button','click','--name','切换禁用','--exact');b('set','viewport','1100','900')
 b('click','[data-arena-picker] summary');popup('arena',8);b('click','[data-arena-option="14"]');check('desktop mouse still selects',js('document.querySelector("[data-arena-picker]").dataset.arena')=='14')
 b('focus','[data-arena-picker] summary');b('press','Space');popup('arena',8);b('press','Escape');check('keyboard toggle and Escape still work',js('document.querySelectorAll("[data-arena-option]").length')==0)
 b('screenshot',str(OUT/'desktop-selected.png'))
 check('no browser errors',not b('errors')['errors']);report['events']=js('mobileChoiceEvents');report['passed']=True
except BaseException as error:
 report['error']=repr(error)
 try:
  report['diagnostic']=js('({active:document.activeElement?.outerHTML,events:mobileChoiceEvents,details:[...document.querySelectorAll("details")].map(e=>({open:e.open,html:e.outerHTML}))})')
  b('screenshot',str(OUT/'failure.png'))
 except Exception:pass
 raise
finally:
 report['sources']={str(f.relative_to(ROOT)):hashlib.sha256(f.read_bytes()).hexdigest() for f in [Path(__file__),ROOT/'scripts/yuyuhakusho/mobile_choices_fixture.tsx',ROOT/'games/yuyuhakusho/web/CharacterPicker.tsx']}
 (OUT/'result.json').write_text(json.dumps(report,ensure_ascii=False,indent=2)+'\n')
 try:b('close')
 except Exception:pass
 if host:host.shutdown();host.server_close()
 print(json.dumps({'passed':report['passed'],'checks':len(report['checks']),'result':str(OUT/'result.json')},ensure_ascii=False))
