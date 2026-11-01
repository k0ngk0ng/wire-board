#!/usr/bin/env python3
"""Browser acceptance written before the layout change.
Failures: fullscreen clips scores/table; charging/rolling/respot shifts table;
zoom/pan crops the table; cue tip is collapsed; resized input misfires;
320px controls overflow or become inaccessible. Uses labelled colour opening,
real production HTTP, SQLite, canvas and native mouse/keyboard/touch inputs.
"""
import argparse, json, os, subprocess, time, uuid, urllib.request, http.cookiejar, shutil
from pathlib import Path
parser=argparse.ArgumentParser();parser.add_argument('--baseline',action='store_true');parser.add_argument('--skip-build',action='store_true');args=parser.parse_args()
root=Path(__file__).resolve().parents[2];out=root/'.local/snooker/layout'/('before' if args.baseline else 'after');out.mkdir(parents=True,exist_ok=True)
session='sn-layout-'+uuid.uuid4().hex[:6];env=dict(os.environ)
profile=root/'.local/snooker/layout'/session
for key,rel in {'AGENT_BROWSER_SOCKET_DIR':'.local/snooker/layout/sockets','AGENT_BROWSER_PROFILE':str(profile.relative_to(root)),'TMPDIR':'.local/tmp','GOTMPDIR':'.local/tmp','GOCACHE':'.local/go-build','GOMODCACHE':'.local/go-mod','npm_config_cache':'.local/npm'}.items():
 p=root/rel;p.mkdir(parents=True,exist_ok=True);env[key]=str(p)
env.update(AGENT_BROWSER_SESSION=session,AGENT_BROWSER_ARGS='--mute-audio,--disable-breakpad,--disable-crash-reporter',WIRE_BOARD_SNOOKER_BROWSER='1',WIRE_BOARD_SNOOKER_CASE='colour',GOTOOLCHAIN='local')
fixture=root/'.local/snooker/browser';fixture.mkdir(parents=True,exist_ok=True)
report={'passed':False,'checks':[],'failures':[],'scope':'labelled colour opening, real HTTP/SQLite and native browser input'};server=None;opened=False

def browser(*a):
 p=subprocess.run(['agent-browser',*map(str,a),'--json'],cwd=root,env=env,text=True,capture_output=True,timeout=45)
 if p.returncode:raise RuntimeError(p.stdout+p.stderr)
 r=json.loads(p.stdout);assert r['success'],r;return r['data']
def js(code):return browser('eval',code)['result']
def api(path,body=None):
 opts='' if body is None else ',{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify('+json.dumps(body)+')}'
 r=js('fetch('+json.dumps('/api'+path)+opts+').then(async r=>({status:r.status,body:await r.json()}))');assert r['status'] in (200,201),r;return r['body']
def command(kind):
 r=api('/state')['room'];return api('/rooms/'+r['id'],{'type':kind,'version':r['version'],'nonce':uuid.uuid4().hex})
def check(name,ok,detail=None):
 report['checks'].append({'name':name,'passed':bool(ok),'detail':detail})
 if not ok:report['failures'].append(name)
 print(('PASS ' if ok else 'FAIL ')+name,flush=True)
def geometry():return js("""(() => {
 const rect=s=>{const e=document.querySelector(s);return e?e.getBoundingClientRect().toJSON():null};
 const scores=[...document.querySelectorAll('[data-snooker-score]')].map(e=>{const r=e.getBoundingClientRect();return {text:e.textContent,rect:r.toJSON(),visible:r.width>0&&r.height>0&&r.top>=0&&r.bottom<=innerHeight&&r.left>=0&&r.right<=innerWidth&&e.contains(document.elementFromPoint(r.x+r.width/2,r.y+r.height/2))}});
 const tip=document.querySelector('.snooker-tip-ball'),r=tip?.getBoundingClientRect();const tipVisible=!!r&&[[r.left+12,r.top+12],[r.right-12,r.top+12],[r.left+12,r.bottom-12],[r.right-12,r.bottom-12]].every(([x,y])=>tip.contains(document.elementFromPoint(x,y)));
 return {table:rect('.snooker-table'),tip:rect('.snooker-tip-ball'),tipVisible,scores,width:innerWidth,height:innerHeight,scroll:document.documentElement.scrollWidth,zoom:document.querySelector('.snooker-table').dataset.zoom,phase:document.querySelector('.snooker-board').dataset.phase};
})()""")
try:
 if not args.skip_build:
  with (out/'build.log').open('w') as log:subprocess.run(['npm','--prefix','web','run','build'],cwd=root,env=env,stdout=log,stderr=subprocess.STDOUT,check=True)
 for name in ['url','stop']:(fixture/name).unlink(missing_ok=True)
 log=(out/'server.log').open('w');server=subprocess.Popen(['go','test','-count=1','-timeout=21m','-run','^TestSnookerBrowserFixture$','./internal/gametest'],cwd=root,env=env,stdout=log,stderr=subprocess.STDOUT)
 deadline=time.monotonic()+90
 while not (fixture/'url').exists():
  if server.poll() is not None or time.monotonic()>deadline:raise RuntimeError('fixture did not start')
  time.sleep(.1)
 base=(fixture/'url').read_text();browser('open',base);opened=True
 api('/login',{'name':'斯诺克验收管理员','password':'snooker-browser-test-only'});api('/admin/games/snooker',{'enabled':True})
 room=api('/rooms',{'kind':'snooker','name':'布局验收','capacity':2})
 jar=http.cookiejar.CookieJar();peer=urllib.request.build_opener(urllib.request.HTTPCookieProcessor(jar))
 def req(path,body):
  with peer.open(urllib.request.Request(base+'/api'+path,data=json.dumps(body).encode(),headers={'Content-Type':'application/json'})) as r:return json.load(r)
 req('/register',{'name':'布局验收对手','password':'snooker-browser-test-only','invite':'snooker-browser'})
 req('/rooms/'+room['id'],{'type':'join','version':api('/state')['room']['version'],'nonce':uuid.uuid4().hex})
 req('/rooms/'+room['id'],{'type':'ready','version':api('/state')['room']['version'],'nonce':uuid.uuid4().hex});command('ready');command('start')
 browser('reload');browser('wait','.snooker-table canvas');browser('set','viewport',1280,800)
 browser('wait','--fn','!document.querySelector("[aria-label=全屏]").disabled')
 check('no zoom/pan controls',not js('!!document.querySelector("[aria-label=放大球桌],[aria-label=缩小球桌],.snooker-camera button")'))
 check('cue tip expanded',js('!!document.querySelector(".snooker-tip-ball") && !document.querySelector("details.snooker-spin")'))
 for width,height in [(1280,800),(1280,600),(320,740),(844,390)]:
  browser('set','viewport',width,height);browser('find','role','button','click','--name','全屏','--exact');browser('wait','[data-room-fullscreen]')
  browser('focus','.snooker-table canvas');time.sleep(.15)
  g=geometry();label=f'fullscreen-{width}-{height}'
  check(label+' scores visible',len(g['scores'])==2 and all(s['visible'] for s in g['scores']),g)
  t=g['table'];check(label+' table fits',t['y']>=0 and t['bottom']<=height+1 and t['x']>=0 and t['right']<=width+1,g)
  check(label+' tip visible',g['tipVisible'] and g['tip'] and g['tip']['width']>0 and g['tip']['y']>=0 and g['tip']['bottom']<=height+1,g)
  browser('screenshot',out/(label+'.png'))
  browser('find','role','button','click','--name','退出全屏','--exact');browser('wait','--fn','!document.querySelector("[data-room-fullscreen]")')
  check(label+' exits cleanly',not js('document.body.inert'))
 browser('set','viewport',1280,800);browser('find','role','button','click','--name','全屏','--exact');browser('wait','[data-room-fullscreen]');browser('focus','.snooker-table canvas')
 js("window.__layout=[];window.__sample=true;function sample(){if(!window.__sample)return;const r=document.querySelector('.snooker-table').getBoundingClientRect();window.__layout.push({x:r.x,y:r.y,width:r.width,height:r.height,phase:document.querySelector('.snooker-board').dataset.phase,tip:!!document.querySelector('.snooker-tip-ball')});requestAnimationFrame(sample)};requestAnimationFrame(sample)")
 g=geometry();t=g['table'];x=round(t['x']+t['width']*.45);y=round(t['y']+t['height']*.6)
 browser('mouse','move',x,y);browser('mouse','down');time.sleep(.15);charged=geometry();check('charging does not move table',all(abs(charged['table'][k]-t[k])<=1 for k in ['x','y','width','height']));browser('press','Escape');browser('mouse','up')
 # Escape may leave native fullscreen; measure again after re-entering.
 if not js('!!document.querySelector("[data-room-fullscreen]")'):browser('find','role','button','click','--name','全屏','--exact');browser('wait','[data-room-fullscreen]')
 browser('focus','.snooker-table canvas');js('window.__layout=[]')
 g=geometry();t=g['table'];k=min(t['width']/(3.569+.38),t['height']/(1.778+.38))
 x=round(t['x']+t['width']/2);y=round(t['y']+t['height']/2+(.6-.889)*k-.12*k)
 browser('mouse','move',x,y);browser('mouse','down')
 limit=max(80,min(140,min(t['width'],t['height'])*.35));pull=6+(.25-.02)/.98*(limit-6)
 browser('mouse','move',x,round(y+pull));browser('mouse','up')
 browser('wait','--fn','document.querySelector(".snooker-board").dataset.phase==="rolling"');browser('wait','--fn','document.querySelector(".snooker-board").dataset.phase==="aim"');time.sleep(.5)
 samples=js('window.__sample=false;window.__layout');(out/'frames.json').write_text(json.dumps(samples))
 spread={k:max(s[k] for s in samples)-min(s[k] for s in samples) for k in ['x','y','width','height']};check('table stable through shot and respot',max(spread.values())<=1,spread)
 data=api('/state')['room']['game']['data'];check('native shot scored',data['scores']==[6,0],data['scores'])
 check('tip remains mounted during rolling',any(s['phase']=='rolling' for s in samples) and all(s['tip'] for s in samples))
 g=geometry();version=api('/state')['room']['version'];t=g['table'];browser('mouse','move',round(t['x']+t['width']/2),round(t['y']+t['height']/2));browser('scroll','down',150);check('wheel does not zoom',geometry()['zoom']=='1.000')
 browser('find','role','button','click','--name','退出全屏','--exact');browser('set','viewport',320,740);browser('scrollintoview','.snooker-table canvas');g=geometry();check('normal mobile table fits viewport',g['table']['height']<=740,g)
 t=g['table'];x=round(t['x']+t['width']/2);y=round(t['y']+t['height']/2)
 subprocess.run(['node','scripts/snooker/native-touch.mjs',browser('get','cdp-url')['cdpUrl'],base,'pinch',str(x),str(y),str(x),str(y)],cwd=root,env=env,check=True,timeout=20)
 check('pinch does not zoom or shoot',geometry()['zoom']=='1.000' and api('/state')['room']['version']==version)
 browser('screenshot',out/'normal-mobile.png','--full');check('no browser errors',not browser('errors')['errors'])
 report['passed']=not report['failures']
except Exception as e:
 import traceback;traceback.print_exc();report['error']=repr(e)
finally:
 if opened:
  try:browser('close')
  except Exception:pass
 if server:
  (fixture/'stop').write_text('stop')
  try:server.wait(timeout=15)
  except subprocess.TimeoutExpired:server.terminate();server.wait(timeout=10)
 shutil.rmtree(profile,ignore_errors=True)
 (out/'report.json').write_text(json.dumps(report,ensure_ascii=False,indent=2)+'\n');print(out/'report.json',flush=True)
 if not report['passed']:raise SystemExit(1)
