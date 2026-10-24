#!/usr/bin/env python3
"""Shared sound browser acceptance, see SCENARIOS.md. Build web first."""
import argparse, hashlib, http.cookiejar, json, os, re, shutil, subprocess, time, traceback, urllib.request, uuid
from pathlib import Path
parser=argparse.ArgumentParser();parser.add_argument('--out',required=True);parser.add_argument('--tank-only',action='store_true');parser.add_argument('--games',default='');args=parser.parse_args()
root=Path(__file__).resolve().parents[2];out=(root/args.out).resolve();assert out.is_relative_to(root) and not out.exists();out.mkdir(parents=True)
run='sound'+uuid.uuid4().hex[:6];temp=out/'temporary';sockets=root/'.s'/run;env=dict(os.environ)
for k,v in {'TMPDIR':temp/'tmp','GOTMPDIR':temp/'tmp','GOCACHE':root/'.local/go-build','GOMODCACHE':root/'.local/go-mod','npm_config_cache':root/'.local/npm','AGENT_BROWSER_SOCKET_DIR':sockets,'AGENT_BROWSER_PROFILE':temp/'profile'}.items():v.mkdir(parents=True,exist_ok=True);env[k]=str(v)
env.update(AGENT_BROWSER_SESSION=run,AGENT_BROWSER_ARGS='--mute-audio,--disable-breakpad,--disable-crash-reporter',WIRE_BOARD_BUBBLEPARTY_BROWSER='1',WIRE_BOARD_BUBBLEPARTY_BROWSER_OUT=str(out),GOTOOLCHAIN='local')
report={'passed':False,'games':{},'command':'python3 -B scripts/sound/check-browser.py --out '+args.out+(' --tank-only' if args.tank_only else ''),'buildIndexSHA256':hashlib.sha256((root/'web/dist/index.html').read_bytes()).hexdigest()}
server=log=None;opened=False

def browser(*a):
 r=subprocess.run(['agent-browser',*a,'--json'],cwd=root,env=env,text=True,capture_output=True,timeout=45);assert r.returncode==0,r.stdout+r.stderr
 j=json.loads(r.stdout);assert j['success'],j;return j['data']
def js(code):return browser('eval',code)['result']
def api(path,body=None):
 opts='' if body is None else ',{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify('+json.dumps(body)+')}'
 r=js('(async()=>{const r=await fetch('+json.dumps('/api'+path)+opts+');return {status:r.status,data:await r.json()}})()');assert r['status'] in (200,201),r;return r['data']
def room():return api('/state').get('room')
def cmd(kind,**extra):
 r=room();return api('/rooms/'+r['id'],dict(type=kind,version=r['version'],nonce=uuid.uuid4().hex,**extra))
def wait(fn,seconds=20):
 end=time.monotonic()+seconds
 while time.monotonic()<end:
  if server is not None and server.poll() is not None:raise AssertionError('fixture exited; inspect server.log')
  if value:=fn():return value
  time.sleep(.1)
 raise AssertionError('Timed out')
def save(name,data):(out/(name+'.json')).write_text(json.dumps(data,ensure_ascii=False,indent=2)+'\n')

def shot(name):browser('screenshot',str(out/(name+'.png')))
def open_sound():
 if not js('!!document.querySelector("[data-sound-panel]")'):browser('click','[data-sound-trigger]')
 browser('wait','[data-volume]')

def level_key(key):browser('focus','[data-volume]');browser('press',key)
def music():return js('(()=>{const m=document.querySelector("audio[data-platform-music]");return m?{volume:m.volume,paused:m.paused,time:m.currentTime,src:m.currentSrc||m.src}:null})()')
def check(name,**evidence):report['games'][name]=evidence;print('PASS',name,flush=True)
try:
 log=(out/'server.log').open('w');server=subprocess.Popen(['go','test','-count=1','-timeout=12m','-run','^TestBubblepartyBrowserFixture$','./games/bubbleparty/tests'],cwd=root,env=env,stdout=log,stderr=subprocess.STDOUT)
 wait(lambda:(out/'url').exists(),120);base=(out/'url').read_text();browser('open',base);opened=True
 api('/login',{'name':'泡泡验收管理员','password':'bubbleparty-browser-test-only'});browser('reload');browser('wait','.room-list');browser('set','viewport','1440','1000')
 assert js('document.querySelectorAll("[data-sound-trigger]").length===1&&!document.querySelector("[data-sound-panel]")')
 open_sound();assert js('Number(document.querySelector("[data-volume]").value)===50')
 level_key('End');assert js('localStorage.getItem("wb_volume")==="1"')
 center=js('(()=>{const r=document.querySelector("[data-volume]").getBoundingClientRect();return {x:r.x+r.width/2,y:r.y+r.height/2}})()')
 browser('mouse','move',str(round(center['x'])),str(round(center['y'])));browser('mouse','down');browser('mouse','up')
 mouse_volume=js('Number(localStorage.getItem("wb_volume"))');assert .4<mouse_volume<.6,mouse_volume
 level_key('End')
 browser('click','[data-sound-preview]');wait(lambda:js('document.querySelector("[data-sound-status]")?.textContent.includes("已播放")'))
 shot('desktop-sound');browser('press','Escape');assert not js('!!document.querySelector("[data-sound-panel]")')
 open_sound();browser('click','h1');assert not js('!!document.querySelector("[data-sound-panel]")')
 check('speaker-popover',previewMerged=True,keyboardVolume=True,mouseVolume=mouse_volume,escape=True,outsideDismiss=True)
 api('/rooms',{'kind':'bubbleparty','name':'泡泡堂音量验收','capacity':2,'options':{}});cmd('add_bot');cmd('ready');cmd('start')
 browser('wait','[data-bubbleparty-character-confirm]');browser('click','[data-bubbleparty-character-confirm]')
 wait(lambda:room()['game']['data']['phase']=='playing',45);wait(lambda:music() and not music()['paused'])
 open_sound();level_key('End');before=music();assert before['volume']==1
 level_key('ArrowLeft');after=music();assert abs(after['volume']-.99)<.0001 and after['src']==before['src'] and after['time']>=before['time'],(before,after)
 browser('click','[data-sound-preview]');assert not music()['paused'];shot('bubbleparty-sound')
 browser('click','[data-sound-mute]');wait(lambda:music()['paused'])
 assert js('document.querySelector("[data-sound-preview]").disabled')
 assert js('Array.from(document.querySelectorAll("audio[data-platform-audio]")).every(m=>m.paused)')
 browser('click','[data-sound-mute]');wait(lambda:music() and not music()['paused']);assert music()['volume']==.99
 level_key('Home');wait(lambda:music()['paused']);assert js('document.querySelector("[data-sound-preview]").disabled')
 level_key('End');wait(lambda:music() and not music()['paused']);assert music()['volume']==1
 level_key('ArrowLeft');browser('press','Escape');browser('reload');browser('wait','[data-sound-trigger]');open_sound();assert js('Number(document.querySelector("[data-volume]").value)===99')
 browser('press','Escape');check('bubbleparty-live-volume',before=before,after=after,mute=True,zero=True,reload=True)
 # A second normal same-origin tab exercises native storage events.
 browser('tab','new',base);browser('wait','[data-sound-trigger]');open_sound();level_key('Home');browser('press','ArrowRight');browser('click','[data-sound-mute]')
 browser('tab','t1');open_sound();wait(lambda:js('Number(document.querySelector("[data-volume]").value)===1&&document.querySelector("[data-sound-preview]").disabled'))
 browser('click','[data-sound-mute]');level_key('End');browser('press','Escape');browser('tab','close','t2');check('cross-tab-preferences',volume=True,mute=True)
 # Game-only fullscreen excludes all platform sound/menu controls; exit
 # restores the same speaker panel and saved volume.
 if js('!!document.querySelector(".toast.error")'):browser('click','button[aria-label="关闭错误"]')
 browser('scrollintoview','[data-fullscreen-toggle]');browser('click','[data-fullscreen-toggle]');wait(lambda:js('!!document.fullscreenElement'))
 assert js('document.fullscreenElement.matches("[data-game-fullscreen-surface]")&&!document.fullscreenElement.querySelector("[data-sound-trigger],.topbar,.players-strip,.room-heading")')
 browser('press','Escape');wait(lambda:js('!document.fullscreenElement'));js('scrollTo(0,0);true');open_sound();level_key('ArrowLeft');shot('sound-after-game-fullscreen');browser('press','Escape')
 check('fullscreen-sound',platformOutsideGame=True,restoredAfterExit=True)
 browser('set','viewport','320','740');js('scrollTo(0,0);true');open_sound()
 geometry=js('(()=>{const r=document.querySelector("[data-sound-panel]").getBoundingClientRect();return {x:r.x,y:r.y,w:r.width,h:r.height,viewport:[innerWidth,innerHeight],overflow:document.documentElement.scrollWidth>innerWidth+1}})()')
 assert geometry['x']>=0 and geometry['x']+geometry['w']<=320 and geometry['y']>=0 and geometry['y']+geometry['h']<=740 and not geometry['overflow'],geometry
 native=subprocess.run(['node','scripts/fullscreen/tap.mjs',browser('get','cdp-url')['cdpUrl'],base,'[data-sound-mute]'],cwd=root,env=env,text=True,capture_output=True,timeout=20);assert native.returncode==0,native.stderr
 assert js('document.querySelector("[data-sound-preview]").disabled');shot('mobile-sound');check('mobile-touch',geometry=geometry,nativeTouch=True)
 report['computedStyles']=js('(()=>{const panel=document.querySelector("[data-sound-panel]");return {panel:{color:getComputedStyle(panel).color,background:getComputedStyle(panel).backgroundColor},mutedPreview:{color:getComputedStyle(document.querySelector("[data-sound-preview]")).color,background:getComputedStyle(document.querySelector("[data-sound-preview]")).backgroundColor}}})()')
 def luminance(rgb):
  values=[int(v)/255 for v in re.findall(r'\d+',rgb)[:3]]
  values=[v/12.92 if v<=.04045 else ((v+.055)/1.055)**2.4 for v in values]
  return sum(v*k for v,k in zip(values,[.2126,.7152,.0722]))
 for pair in report['computedStyles'].values():
  a,b=sorted([luminance(pair['color']),luminance(pair['background'])]);pair['contrast']=(b+.05)/(a+.05);assert pair['contrast']>=4.5,pair
 report['passed']=True
except BaseException:
 if opened:
  try:shot('failure');save('failure-state',{'snapshot':browser('snapshot','-i'),'errors':browser('errors')})
  except Exception:pass
 report['error']=traceback.format_exc();print(report['error'],flush=True)
finally:
 if opened:
  try:browser('close')
  except BaseException:report['passed']=False
 if server:
  (out/'stop').write_text('stop')
  try:server.wait(timeout=20)
  except subprocess.TimeoutExpired:server.kill();server.wait();report['passed']=False
  if server.returncode!=0:report['passed']=False
 if log:log.close()
 for path in [temp,sockets,out/'data']:
  if path.exists():shutil.rmtree(path)
 for name in ['url','stop']:(out/name).unlink(missing_ok=True)
 report['sources']={str(p.relative_to(root)):hashlib.sha256(p.read_bytes()).hexdigest() for p in [root/'web/src/audio.ts',root/'web/src/audio-preferences.ts',root/'web/src/ui/sound-controls.tsx',root/'web/src/ui/sound-controls.css',root/'web/src/main.tsx',root/'scripts/sound/check-browser.py'] if p.exists()}
 report['scope']='Actual Bubbleparty/native media; Chromium desktop, native touch at 320px, same-origin tabs and native fullscreen; not physical iOS.'
 save('report',report)
raise SystemExit(0 if report['passed'] else 1)
