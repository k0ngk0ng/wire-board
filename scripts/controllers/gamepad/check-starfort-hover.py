#!/usr/bin/env python3
"""Before implementation: hover description changes popover height/anchor,
repeatedly enters/leaves a stationary pointer; tooltip may also escape fullscreen,
cover its trigger, or lose touch pin/keyboard dismiss behavior. Real browser +
existing central-passives scenario, real server timers and Canvas; no state mock.
"""
import argparse,hashlib,json,os,shutil,subprocess,time,traceback,uuid
from pathlib import Path
parser=argparse.ArgumentParser();parser.add_argument('--out',required=True);args=parser.parse_args()
root=Path(__file__).resolve().parents[3];out=(root/args.out).resolve();assert out.is_relative_to(root) and not out.exists();out.mkdir(parents=True)
temp=out/'temporary';session='hover'+uuid.uuid4().hex[:6];sockets=root/'.s'/session;env=dict(os.environ)
for key,path in {'TMPDIR':temp/'tmp','GOTMPDIR':temp/'tmp','GOCACHE':root/'.local/go-build','GOMODCACHE':root/'.local/go-mod','AGENT_BROWSER_SOCKET_DIR':sockets,'AGENT_BROWSER_PROFILE':temp/'profile'}.items():path.mkdir(parents=True,exist_ok=True);env[key]=str(path)
env.update(GOTOOLCHAIN='local',AGENT_BROWSER_SESSION=session,AGENT_BROWSER_ARGS='--mute-audio,--disable-breakpad,--disable-crash-reporter',WIRE_BOARD_STARFORT_BROWSER='1',WIRE_BOARD_STARFORT_BROWSER_OUT=str(out/'fixture'),WIRE_BOARD_STARFORT_BROWSER_WEB=str(root/'.local/gamepad/web-dist'))
report={'passed':False,'command':'python3 -B scripts/controllers/gamepad/check-starfort-hover.py --out '+args.out,'scope':'Existing central-passives milestone; production UI/Canvas, native mouse/touch, real HTTP and timers.'};opened=False;server=log=None

def b(*args):
 p=subprocess.run(['agent-browser',*map(str,args),'--json'],cwd=root,env=env,text=True,capture_output=True,timeout=45);assert p.returncode==0,p.stdout+p.stderr
 r=json.loads(p.stdout);assert r['success'],r;return r['data']
def js(code):return b('eval',code)['result']
def api(path,body=None):
 opts='' if body is None else ',{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify('+json.dumps(body)+')}'
 r=js('(async()=>{const r=await fetch('+json.dumps('/api'+path)+opts+');return {status:r.status,body:await r.json()}})()');assert r['status'] in (200,201),r;return r['body']
def command(kind):
 for _ in range(12):
  room=api('/state')['room'];body={'type':kind,'version':room['version'],'nonce':uuid.uuid4().hex}
  result=js('(async()=>{const r=await fetch('+json.dumps('/api/rooms/'+room['id'])+',{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify('+json.dumps(body)+')});return {status:r.status,body:await r.json()}})()')
  if result['status']==409:continue
  assert result['status']==200,result;return
 raise AssertionError('stale room command')
def wait(fn,seconds=30):
 end=time.monotonic()+seconds
 while time.monotonic()<end:
  if server and server.poll() is not None:raise AssertionError('fixture exited')
  if value:=fn():return value
  time.sleep(.1)
 raise AssertionError('wait expired')
def rect(selector):return js('document.querySelector('+json.dumps(selector)+').getBoundingClientRect().toJSON()')
def save(name,value):(out/(name+'.json')).write_text(json.dumps(value,ensure_ascii=False,indent=2)+'\n')
def sample():
 return js('''new Promise(resolve=>{const frames=[],start=performance.now();const frame=at=>{const b=document.querySelector('[data-sf-skill]'),r=b?.getBoundingClientRect(),t=document.querySelector('[data-sf-skill-tooltip]'),p=document.querySelector('.sf-map-popover')?.getBoundingClientRect();frames.push({at,open:!!t,x:r?.x,y:r?.y,panelY:p?.y,panelH:p?.height});if(at-start<1200)requestAnimationFrame(frame);else resolve(frames)};requestAnimationFrame(frame)})''')
try:
 log=(out/'server.log').open('w');server=subprocess.Popen(['go','test','-count=1','-timeout=8m','-run','^TestStarfortBrowserFixture$','./games/starfort/tests'],cwd=root,env=env,stdout=log,stderr=subprocess.STDOUT)
 wait(lambda:(out/'fixture/url').exists(),120);base=(out/'fixture/url').read_text();b('open',base);opened=True
 api('/login',{'name':'上架管理员','password':'starfort-browser-test'});b('reload');b('wait','.room-list');b('set','viewport',1280,900)
 api('/rooms',{'kind':'starfort','name':'技能悬停验收','capacity':1,'options':{'scenario':'central-passives','mode':'endless','waves':20,'difficulty':'normal','prepSeconds':45}});command('ready');command('start')
 b('wait','.starfort-board');b('wait','--fn','document.querySelector(".sf-battlefield")?.dataset.assetsReady==="true"');b('click','.room-tools > button:first-child');b('wait','[data-room-fullscreen]');b('click','[data-sf-camera-base]')
 tower=wait(lambda:js('(()=>{const towers=[...document.querySelectorAll(".sf-pad-occupied:not([hidden])")];return towers.find(e=>{const r=e.getBoundingClientRect();return r.width>0&&r.x>=0&&r.right<=innerWidth&&r.y>80&&r.bottom<innerHeight-80&&e.contains(document.elementFromPoint(r.x+r.width/2,r.y+r.height*.35))})?.dataset.sfPad})()'))
 r=rect('[data-sf-pad="'+tower+'"]');b('mouse','move',round(r['x']+r['width']/2),round(r['y']+r['height']*.35));b('mouse','down','left');b('mouse','up','left');b('wait','[data-sf-skill]')
 for index,fraction in enumerate([.25,.5,.75]):
  b('mouse','move',5,5);time.sleep(.2);r=rect('[data-sf-skill]');b('mouse','move',round(r['x']+r['width']/2),round(r['y']+r['height']*fraction));frames=sample();save('hover-'+str(index),frames)
  stable=frames[3:];assert len(stable)>20 and all(f['open'] for f in stable),('hover toggled',sum(f['open'] for f in stable),len(stable))
  assert max(f['y'] for f in frames)-min(f['y'] for f in frames)<1,('trigger moved',frames[:5])
 b('screenshot',str(out/'desktop-hover.png'));tip=rect('[data-sf-skill-tooltip]');b('mouse','move',round(tip['x']+tip['width']/2),round(tip['y']+tip['height']/2));time.sleep(.25);assert js('!!document.querySelector("[data-sf-skill-tooltip]")');b('mouse','move',5,5);wait(lambda:not js('!!document.querySelector("[data-sf-skill-tooltip]")'))
 b('focus','[data-sf-skill]');b('wait','[data-sf-skill-tooltip]');b('press','Escape');wait(lambda:not js('!!document.querySelector("[data-sf-skill-tooltip]")'))
 if js('!!document.querySelector("[data-room-fullscreen]")'):b('press','Escape');wait(lambda:not js('!!document.querySelector("[data-room-fullscreen]")'))
 b('focus','[data-fullscreen-board]');b('scrollintoview','[data-sf-skill]');time.sleep(.25);b('mouse','move',5,5);r=rect('[data-sf-skill]');b('mouse','move',round(r['x']+r['width']/2),round(r['y']+r['height']/2));normal=sample();save('normal-hover',normal)
 assert all(f['open'] for f in normal[3:]) and max(f['y'] for f in normal)-min(f['y'] for f in normal)<1
 b('screenshot',str(out/'normal-hover.png'));b('mouse','move',5,5);wait(lambda:not js('!!document.querySelector("[data-sf-skill-tooltip]")'))
 b('click','.room-tools > button:first-child');b('wait','[data-room-fullscreen]')
 b('set','viewport',320,740);b('scrollintoview','[data-sf-skill]');r=rect('[data-sf-skill]');point={'x':r['x']+r['width']/2,'y':r['y']+r['height']/2,'id':1}
 native=subprocess.run(['node','scripts/controllers/gamepad/native-touch.mjs',b('get','cdp-url')['cdpUrl'],base,json.dumps([{'type':'touchStart','touchPoints':[point],'wait':60},{'type':'touchEnd','touchPoints':[],'wait':120}]),'[data-sf-skill]'],cwd=root,env=env,text=True,capture_output=True,timeout=20);assert native.returncode==0,native.stderr
 b('wait','[data-sf-skill-tooltip]');tip=rect('[data-sf-skill-tooltip]');assert tip['x']>=0 and tip['y']>=0 and tip['right']<=320 and tip['bottom']<=740,tip
 assert js('document.querySelector("[data-room-fullscreen]").contains(document.querySelector("[data-sf-skill-tooltip]"))')
 b('screenshot',str(out/'mobile-pinned.png'));b('press','Escape');wait(lambda:not js('!!document.querySelector("[data-sf-skill-tooltip]")'))
 report.update(passed=True,stationaryHoverStable=True,normalAndFullscreen=True,triggerDoesNotMove=True,tooltipHover=True,keyboardDismiss=True,mobilePinned=True,fullscreenContained=True)
except BaseException:
 report['error']=traceback.format_exc();print(report['error'],flush=True)
 if opened:
  try:b('screenshot',str(out/'failure.png'));save('failure-state',{'snapshot':b('snapshot','-i'),'errors':b('errors')})
  except Exception:pass
finally:
 if opened:
  try:b('close')
  except Exception:report['passed']=False
 if server:
  if (out/'fixture').exists():(out/'fixture/stop').write_text('stop')
  try:server.wait(timeout=20)
  except subprocess.TimeoutExpired:server.kill();server.wait();report['passed']=False
  report['serverExitCode']=server.returncode
  if server.returncode!=0:report['passed']=False
 if log:log.close()
 for path in [temp,sockets,out/'fixture']:
  if path.exists():shutil.rmtree(path)
 report['sources']={str(p.relative_to(root)):hashlib.sha256(p.read_bytes()).hexdigest() for p in [root/'games/starfort/web/SkillBadge.tsx',root/'games/starfort/web/viewport.css',root/'games/starfort/web/Canvas.tsx',Path(__file__)]}
 save('report',report)
raise SystemExit(0 if report['passed'] else 1)
