#!/usr/bin/env python3
"""Targeted production fallback E2E; declared before exercising this branch.
Failure inventory: CDN error leaves blank car/scenery; fallback requests CDN
again; decode never succeeds; media loop keeps retrying the remote source,
creates a game-owned player, or keeps playing after the platform is muted.
Only the CDN racing path is blocked. No game state/asset response is replaced.
The full browser/result.json and its screenshots are deliberately preserved.
"""
import http.cookiejar,json,os,select,subprocess,time,traceback,uuid
from pathlib import Path
root=Path(__file__).resolve().parents[2];fixture=root/'.local/racing/browser';out=fixture/'fallback';out.mkdir(parents=True,exist_ok=True)
manifest=json.loads((root/'games/racing/assets/manifest.json').read_text());cdn=manifest['assetsBaseURL'].rstrip('/')
env=dict(os.environ);session='rc-fb-'+uuid.uuid4().hex[:4]
for key,path in {'AGENT_BROWSER_SOCKET_DIR':fixture/'sockets','AGENT_BROWSER_PROFILE':out/session,'TMPDIR':root/'.local/tmp','GOTMPDIR':root/'.local/tmp','GOCACHE':root/'.local/go-build','GOMODCACHE':root/'.local/go-mod','npm_config_cache':root/'.local/npm'}.items():path.mkdir(parents=True,exist_ok=True);env[key]=str(path)
env.update(AGENT_BROWSER_SESSION=session,AGENT_BROWSER_ARGS='--mute-audio,--disable-breakpad,--disable-crash-reporter',WIRE_BOARD_RACING_BROWSER='1',WIRE_BOARD_RACING_ASSETS=cdn,GOTOOLCHAIN='local')
report={'passed':False,'scope':'real CDN network failures, production image/media fallback, shared mute','checks':[],'command':'. scripts/env.sh && python3 -B scripts/racing/check-fallback.py','cdn':cdn};server=None;driver=None;opened=False;logs=[]
def b(*args):
 p=subprocess.run(['agent-browser',*args,'--json'],cwd=root,env=env,text=True,capture_output=True,timeout=40)
 if p.returncode:raise RuntimeError(p.stdout+p.stderr)
 r=json.loads(p.stdout);assert r['success'],r;return r['data']
def js(code):return b('eval',code)['result']
def api(path,body=None):
 options='' if body is None else ',{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify('+json.dumps(body)+')}'
 r=js('fetch('+json.dumps('/api'+path)+options+').then(async r=>({status:r.status,body:await r.json()}))');assert 200<=r['status']<300,r;return r['body']
def command(kind):
 r=api('/state')['room'];return api('/rooms/'+r['id'],{'type':kind,'version':r['version'],'nonce':uuid.uuid4().hex})
def check(name):report['checks'].append(name);print('PASS',name,flush=True)
try:
 for name in ['url','stop']:(fixture/name).unlink(missing_ok=True)
 log=(out/'server.log').open('w');logs.append(log);server=subprocess.Popen(['go','test','-count=1','-timeout=5m','-run','^TestRacingBrowserFixture$','./games/racing/tests'],cwd=root,env=env,stdout=log,stderr=subprocess.STDOUT)
 until=time.monotonic()+120
 while not (fixture/'url').exists():
  if server.poll() is not None or time.monotonic()>until:raise RuntimeError('production fixture did not start')
  time.sleep(.1)
 base=(fixture/'url').read_text();b('open',base);opened=True;api('/login',{'name':'赛车验收管理员','password':'racing-browser-test-only'});api('/admin/games/racing',{'enabled':True});b('reload');b('wait','.room-list');b('set','viewport','390','844')
 log=(out/'native.log').open('w');logs.append(log);driver=subprocess.Popen(['node','scripts/racing/native-input.mjs',b('get','cdp-url')['cdpUrl'],base,'serve'],cwd=root,env=env,text=True,stdin=subprocess.PIPE,stdout=subprocess.PIPE,stderr=log,bufsize=1)
 driver.stdin.write(json.dumps({'mode':'block-cdn','output':str(out/'blocked.json')})+'\n');driver.stdin.flush();assert select.select([driver.stdout],[],[],20)[0],'CDP block timed out';block=json.loads(driver.stdout.readline());assert 'error' not in block,block
 js(r'''(()=>{window.__fallbackImages=[];window.__fallbackImageErrors=[];window.__fallbackAudio=[];window.__fallbackMedia=[];const ImageCtor=window.Image;window.Image=class extends ImageCtor{constructor(...args){super(...args);window.__fallbackImages.push(this);this.addEventListener('error',()=>window.__fallbackImageErrors.push({src:this.currentSrc||this.src,at:performance.now()}))}};const play=HTMLMediaElement.prototype.play,pause=HTMLMediaElement.prototype.pause;HTMLMediaElement.prototype.play=function(){const r={type:'play',src:this.currentSrc||this.src,platform:this.hasAttribute('data-platform-audio'),at:performance.now()};window.__fallbackAudio.push(r);if(!window.__fallbackMedia.includes(this))window.__fallbackMedia.push(this);return play.call(this).then(v=>{r.ok=true;r.resolvedSrc=this.currentSrc||this.src;return v},e=>{r.error=e.name;throw e})};HTMLMediaElement.prototype.pause=function(){window.__fallbackAudio.push({type:'pause',src:this.currentSrc||this.src,at:performance.now()});return pause.call(this)}})()''')
 api('/rooms',{'kind':'racing','name':'资源回退验收','capacity':2,'options':{'track':'woodland','laps':1}});command('add_bot');command('ready');command('start');b('wait','.racing-board')
 b('wait','--fn','window.__fallbackImages.filter(i=>/cars[.]webp|scenery[.]webp/.test(i.currentSrc)).length===2&&window.__fallbackImages.every(i=>i.complete&&i.naturalWidth>0)')
 images=js('window.__fallbackImages.map(i=>({src:i.currentSrc,width:i.naturalWidth,height:i.naturalHeight,complete:i.complete}))');errors=js('window.__fallbackImageErrors')
 assert len(images)==2 and all(i['src'].startswith(base+'/racing/v1/') for i in images),images
 assert len(errors)==2 and all(i['src'].startswith(cdn+'/racing/v1/') for i in errors),errors
 report['images']=images;report['imageErrors']=errors;check('blocked-cdn-errors-to-decoded-same-origin-images')
 # Real trusted pointer activation unlocks the platform media channel.
 if js('document.querySelector(".sound-toggle")?.getAttribute("aria-pressed")==="true"'):b('find','role','button','click','--name','取消静音','--exact')
 js('document.querySelector("[aria-label=方向盘]").scrollIntoView({block:"center",behavior:"instant"})');b('click','[aria-label="方向盘"]')
 b('wait','--fn','window.__fallbackAudio.some(a=>a.type==="play"&&a.resolvedSrc===location.origin+"/racing/v1/engine.mp3"&&a.ok)')
 audio=js('window.__fallbackAudio');assert any(a['type']=='play' and a['src']==cdn+'/racing/v1/engine.mp3' for a in audio),audio
 assert all(a['platform'] for a in audio if a['type']=='play'),audio
 assert js('window.__fallbackMedia.some(m=>m.currentSrc===location.origin+"/racing/v1/engine.mp3"&&m.loop&&!m.paused&&m.readyState>=2)'),'engine was not actually looping from fallback'
 report['audioBeforeMute']=audio;check('native-platform-engine-plays-same-origin-fallback')
 b('find','role','button','click','--name','静音','--exact');assert js('window.__fallbackMedia.every(m=>m.paused)'),'platform mute failed to stop fallback loop'
 report['audioAfterMute']=js('window.__fallbackAudio');check('platform-mute-stops-fallback-loop')
 b('screenshot',str(out/'fallback-muted-390.png'),'--full');report['screenshot']='.local/racing/browser/fallback/fallback-muted-390.png';assert not b('errors')['errors'],b('errors')
 report['passed']=True
except Exception as error:
 traceback.print_exc();report['error']=repr(error)
 if opened:
  try:report['audioAtFailure']=js('window.__fallbackAudio');report['mediaAtFailure']=js('window.__fallbackMedia?.map(m=>({src:m.src,currentSrc:m.currentSrc,readyState:m.readyState,paused:m.paused,loop:m.loop,error:m.error?.code}))');report['snapshot']=b('snapshot','-i');b('screenshot',str(out/'failure.png'),'--full')
  except Exception:pass
finally:
 if driver:
  driver.stdin.close()
  try:driver.wait(timeout=15)
  except subprocess.TimeoutExpired:driver.terminate();driver.wait(timeout=10)
 if opened:
  try:b('close')
  except Exception:pass
 if server:
  (fixture/'stop').write_text('stop')
  try:server.wait(timeout=15)
  except subprocess.TimeoutExpired:server.terminate();server.wait(timeout=10)
 for f in logs:f.close()
 (out/'result.json').write_text(json.dumps(report,ensure_ascii=False,indent=2)+'\n');print('Artifact:',out/'result.json',flush=True)
 if not report['passed']:raise SystemExit(1)
