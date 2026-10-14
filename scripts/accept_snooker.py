#!/usr/bin/env python3
"""Real 2D snooker UI/HTTP acceptance, authored before the frontend.
Failure inventory: unlisted creation, wrong 2/4 seats, unreadable 320px scores,
misfire on pan/zoom, score or sound preceding the timeline, stale animation on
reload, partner controlling a stroke, foul restoration or turn order lost.
All accounts, profiles, SQLite and evidence remain inside this workspace.
"""
import json,os,subprocess,time,uuid,urllib.request,http.cookiejar
from pathlib import Path
root=Path(__file__).resolve().parents[1];out=root/'.local/snooker/browser';out.mkdir(parents=True,exist_ok=True)
env=dict(os.environ);session='sn-'+uuid.uuid4().hex[:6]
for key,rel in {'AGENT_BROWSER_SOCKET_DIR':'.local/ab-sn','AGENT_BROWSER_PROFILE':'.local/snooker/browser/'+session,'TMPDIR':'.local/tmp','GOTMPDIR':'.local/tmp','GOCACHE':'.local/go-build','GOMODCACHE':'.local/go-mod','npm_config_cache':'.local/npm'}.items():
 p=root/rel;p.mkdir(parents=True,exist_ok=True);env[key]=str(p)
env.update(AGENT_BROWSER_SESSION=session,AGENT_BROWSER_ARGS='--mute-audio,--disable-breakpad,--disable-crash-reporter',WIRE_BOARD_SNOOKER_BROWSER='1',GOTOOLCHAIN='local')
report={'passed':False,'checks':[],'scope':'production registry, natural snooker rack, real HTTP identities and native UI actions'};server=None;log=None;opened=False

def browser(*args):
 p=subprocess.run(['agent-browser',*args,'--json'],cwd=root,env=env,text=True,capture_output=True,timeout=45)
 if p.returncode:raise RuntimeError(p.stdout+p.stderr)
 result=json.loads(p.stdout);assert result['success'],result;return result['data']
def js(code):return browser('eval',code)['result']
def api(path,body=None,status=200):
 opt='' if body is None else ',{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify('+json.dumps(body)+')}'
 r=js('fetch('+json.dumps('/api'+path)+opt+').then(async r=>({status:r.status,body:await r.json()}))');assert r['status']==status,r;return r['body']
def command(kind):
 r=api('/state')['room'];return api('/rooms/'+r['id'],{'type':kind,'version':r['version'],'nonce':uuid.uuid4().hex})
def strike(angle,power):
 browser('scrollintoview','.snooker-table canvas');browser('focus','.snooker-table canvas')
 subprocess.run(['node','scripts/snooker/strike.mjs',browser('get','cdp-url')['cdpUrl'],base,str(angle),str(power)],cwd=root,env=env,check=True,timeout=20)
def shot(name):
 assert not browser('errors')['errors'],browser('errors')
 assert js('document.documentElement.scrollWidth<=innerWidth+2'),name
 colors=js(r"""(() => {
 const rgb=s=>(s.match(/[\d.]+/g)||[]).map(Number),lum=c=>c.slice(0,3).map(v=>v/255).map(v=>v<=.04045?v/12.92:((v+.055)/1.055)**2.4).reduce((s,v,i)=>s+v*[.2126,.7152,.0722][i],0);
 return [...document.querySelectorAll('.snooker-status span,.snooker-status strong,.snooker-scoreboard small,.snooker-scoreboard span,.snooker-scoreboard strong,.snooker-controls label,.snooker-controls output,.snooker-board button,.snooker-turn,.snooker-order,.snooker-public-summary strong')].filter(e=>e.getClientRects().length).map(e=>{let bg=[255,255,255];const chain=[];for(let n=e;n;n=n.parentElement)chain.unshift(n);for(const n of chain){const c=rgb(getComputedStyle(n).backgroundColor),a=c[3]??1;bg=bg.map((v,i)=>v*(1-a)+(c[i]??0)*a)}const fg=rgb(getComputedStyle(e).color),a=lum(fg),b=lum(bg);return {text:e.textContent.trim().slice(0,80),ratio:(Math.max(a,b)+.05)/(Math.min(a,b)+.05)}});
 })()""")
 assert all(c['ratio']>=4.5 for c in colors),(name,colors)
 borders=js(r"""(() => {
 const rgb=s=>(s.match(/[\d.]+/g)||[]).map(Number),lum=c=>c.slice(0,3).map(v=>v/255).map(v=>v<=.04045?v/12.92:((v+.055)/1.055)**2.4).reduce((s,v,i)=>s+v*[.2126,.7152,.0722][i],0),ratio=(a,b)=>(Math.max(lum(a),lum(b))+.05)/(Math.min(lum(a),lum(b))+.05);
 const bg=e=>{let b=[255,255,255],chain=[];for(let n=e;n;n=n.parentElement)chain.unshift(n);for(const n of chain){const c=rgb(getComputedStyle(n).backgroundColor),a=c[3]??1;b=b.map((v,i)=>v*(1-a)+(c[i]??0)*a)}return b};
 return [...document.querySelectorAll('.snooker-board button:not(:disabled),.snooker-controls input[type=number]:not(:disabled),.snooker-controls select:not(:disabled),.snooker-options select:not(:disabled)')].filter(e=>e.getClientRects().length).map(e=>({control:e.getAttribute('aria-label')||e.textContent.trim().slice(0,30),ratio:Math.max(ratio(bg(e),bg(e.parentElement)),ratio(rgb(getComputedStyle(e).borderTopColor),bg(e.parentElement)))}));
 })()""")
 assert all(b['ratio']>=3 for b in borders),(name,borders)
 (out/(name+'-controls-contrast.json')).write_text(json.dumps(borders,ensure_ascii=False,indent=2))

 (out/(name+'-contrast.json')).write_text(json.dumps(colors,ensure_ascii=False,indent=2))
 browser('screenshot',str(out/(name+'.png')),'--full');report['checks'].append(name);print('PASS',name,flush=True)
try:
 for name in ['url','stop']:(out/name).unlink(missing_ok=True)
 with (out/'build.log').open('w') as f:subprocess.run(['npm','--prefix','web','run','build'],cwd=root,env=env,stdout=f,stderr=subprocess.STDOUT,check=True)
 log=(out/'server.log').open('w');server=subprocess.Popen(['go','test','-count=1','-timeout=21m','-run','^TestSnookerBrowserFixture$','./internal/gametest'],cwd=root,env=env,stdout=log,stderr=subprocess.STDOUT)
 start=time.monotonic()
 while not (out/'url').exists():
  if server.poll() is not None or time.monotonic()-start>90:raise RuntimeError('snooker fixture did not start')
  time.sleep(.1)
 base=(out/'url').read_text();browser('open',base);opened=True;api('/login',{'name':'斯诺克验收管理员','password':'snooker-browser-test-only'});admin_cookies=browser('cookies','get')['cookies']
 assert 'snooker' not in api('/state')['availableGames']
 api('/rooms',{'kind':'snooker','name':'下架不能创建','capacity':2},403)
 api('/admin/games/snooker',{'enabled':True});browser('reload');browser('wait','.game-selection');browser('select','select[aria-label="游戏分类"]','sports')
 for n in [2,4]:
  browser('set','viewport','1280','960');browser('select','select[aria-label="游戏分类"]','sports');browser('click','.game-feature[data-game="snooker"] button');browser('wait','[role=dialog]')
  assert js('[...document.querySelectorAll("select[aria-label=座位上限] option")].map(o=>o.value)')==['2','4']
  browser('select','select[aria-label="座位上限"]',str(n));browser('fill','input[name=name]',f'斯诺克 {n} 人验收');browser('click','[role=dialog] button[type=submit]');browser('wait','.waiting-actions')
  room=api('/state')['room'];peers=[]
  for p in range(1,n):
   jar=http.cookiejar.CookieJar();client=urllib.request.build_opener(urllib.request.HTTPCookieProcessor(jar))
   def req(path,body=None,client=client):
    request=urllib.request.Request(base+'/api'+path,data=None if body is None else json.dumps(body).encode(),headers={'Content-Type':'application/json'})
    with client.open(request,timeout=25) as response:return json.load(response)
   req('/register',{'name':f'斯诺克{n}席位{p}','password':'snooker-browser-test-only','invite':'snooker-browser'})
   req('/rooms/'+room['id'],{'type':'join','version':api('/state')['room']['version'],'nonce':uuid.uuid4().hex})
   rr=req('/state')['room'];req('/rooms/'+rr['id'],{'type':'ready','version':rr['version'],'nonce':uuid.uuid4().hex});req.jar=jar;peers.append(req)
  command('ready');shot(f'waiting-{n}');command('start');browser('wait','.snooker-board')
  d=api('/state')['room']['game']['data'];chooser=d['turn']
  if chooser==0:browser('find','role','button','click','--name','A 队先开球','--exact')
  else:
   req=peers[chooser-1];rr=req('/state')['room'];req('/rooms/'+rr['id'],{'type':'action','version':rr['version'],'nonce':uuid.uuid4().hex,'action':{'type':'break_choice','window':d['window'],'team':0}})
  browser('wait','--fn','document.querySelector(".snooker-board").dataset.phase==="aim"')
  for width in [1280,390,320]:browser('set','viewport',str(width),'900');shot(f'aim-{n}-{width}')
  for seat in range(1,n):
   browser('cookies','clear')
   for c in peers[seat-1].jar:browser('cookies','set',c.name,c.value,'--url',base,'--httpOnly')
   browser('reload');browser('wait','.snooker-board');r=api('/state')['room'];assert r['you']==seat and not r['game']['data']['controls']['canShoot']
   assert js('document.querySelector("canvas[aria-disabled]").getAttribute("aria-disabled")=="true"'),'partner can submit active player stroke';shot(f'seat-{n}-{seat}')
  browser('cookies','clear')
  for c in admin_cookies:browser('cookies','set',c['name'],c['value'],'--url',base,'--httpOnly')
  browser('reload');browser('wait','.snooker-board')

  # Native zoom / pan must change the camera, never submit a shot.
  initial=api('/state')['room'];browser('scrollintoview','.snooker-table canvas')
  rect=js('document.querySelector(".snooker-table canvas").getBoundingClientRect().toJSON()');x,y=round(rect['x']+rect['width']/2),round(rect['y']+rect['height']/2)
  subprocess.run(['node','scripts/browser_wheel.mjs',browser('get','cdp-url')['cdpUrl'],base,str(x),str(y),'-200'],cwd=root,env=env,check=True,timeout=15)
  browser('wait','--fn','Number(document.querySelector(".snooker-table").dataset.zoom)>1')
  browser('find','role','button','click','--name','移动视图','--exact');browser('mouse','move',str(x),str(y));browser('mouse','down');browser('mouse','move',str(x+35),str(y+25));browser('mouse','up');browser('find','role','button','click','--name','移动视图','--exact')
  assert api('/state')['room']['version']==initial['version'],'pan/zoom submitted an action'
  browser('find','role','button','click','--name','复位视图','--exact');shot(f'camera-{n}')
  browser('scrollintoview','.snooker-table canvas');rect=js('document.querySelector(".snooker-table canvas").getBoundingClientRect().toJSON()');x,y=round(rect['x']+rect['width']/2),round(rect['y']+rect['height']/2)
  endpoint=browser('get','cdp-url')['cdpUrl']
  subprocess.run(['node','scripts/snooker/native-touch.mjs',endpoint,base,'pinch',str(x),str(y),str(x),str(y)],cwd=root,env=env,check=True,timeout=20)
  browser('wait','--fn','Number(document.querySelector(".snooker-table").dataset.zoom)>1.2')
  assert api('/state')['room']['version']==initial['version'],'pinch submitted a shot'
  browser('find','role','button','click','--name','复位视图','--exact');browser('scrollintoview','.snooker-table canvas');rect=js('document.querySelector(".snooker-table canvas").getBoundingClientRect().toJSON()');x,y=round(rect['x']+rect['width']*.35),round(rect['y']+rect['height']*.65)
  subprocess.run(['node','scripts/snooker/native-touch.mjs',endpoint,base,'tap',str(x),str(y),str(x),str(y)],cwd=root,env=env,check=True,timeout=20)
  assert abs(float(js("document.querySelector('.snooker-table canvas').dataset.angle")))>.01,'native touch did not aim'
  assert api('/state')['room']['version']==initial['version'],'aim tap submitted a shot';shot(f'touch-{n}')

  # A deliberately soft natural opening stroke misses the distant red pack.
  if js('document.querySelector(".sound-toggle").getAttribute("aria-pressed")==="false"'):browser('find','role','button','click','--name','静音','--exact')
  js('window.__snookerAudio=[];const oldPlay=HTMLMediaElement.prototype.play;HTMLMediaElement.prototype.play=function(){window.__snookerAudio.push(this.src);return oldPlay.call(this)}')
  strike(0,.06);browser('wait','--fn','document.querySelector(".snooker-board").dataset.phase==="rolling"')
  assert api('/state')['room']['game']['data']['scores']==[0,0]
  assert js('window.__snookerAudio.length')==0
  browser('wait','--fn','document.querySelector(".snooker-board").dataset.phase==="foul"');after=api('/state')['room'];d=after['game']['data'];assert d['scores']==[0,4]
  shot(f'foul-{n}')
  req=peers[d['turn']-1];req('/rooms/'+after['id'],{'type':'action','version':after['version'],'nonce':uuid.uuid4().hex,'action':{'type':'foul_choice','choice':'restore','window':d['window']}})
  browser('wait','--fn','document.querySelector(".snooker-board").dataset.phase==="aim"');browser('reload');browser('wait','.snooker-board');assert api('/state')['room']['game']['data']['turn']==0

  shot(f'restored-{n}')
  if js('document.querySelector(".sound-toggle").getAttribute("aria-pressed")==="true"'):browser('find','role','button','click','--name','取消静音','--exact')
  js('window.__snookerAudio=[];const original=HTMLMediaElement.prototype.play;HTMLMediaElement.prototype.play=function(){const record={src:this.src,at:Date.now(),platform:this.hasAttribute("data-platform-audio")};window.__snookerAudio.push(record);return original.call(this).then(v=>{record.ok=true;return v},e=>{record.error=e.name;throw e})}')
  strike(0,.35);browser('wait','--fn','window.__snookerAudio.some(a=>a.src.includes("/snooker/")&&a.src.includes("/ball-"))')
  flight=api('/state')['room']['game']['data']['flight'];first=min(e['time'] for e in flight['motion']['events'] if e['kind']=='ball')
  attempts=js('window.__snookerAudio.filter(a=>a.src.includes("/snooker/"))');assert all(a['platform'] for a in attempts)
  assert all(a['at']>=flight['start']+first*1000-35 for a in attempts if '/ball-' in a['src']),'ball sound preceded contact'
  report.setdefault('audio',[]).append({'players':n,'flight':flight['id'],'firstImpact':first,'attempts':attempts})
  browser('wait','--fn','document.querySelector(".snooker-board").dataset.phase!=="rolling"');assert js('window.__snookerAudio.some(a=>a.src.includes("/ball-")&&a.ok)'), 'no collision media actually played';shot(f'impact-audio-{n}')
  browser('reload');browser('wait','.snooker-board')
  command('close');command('leave');browser('wait','.game-selection')
 report['passed']=True
except Exception as error:
 import traceback;traceback.print_exc();report['error']=repr(error)
 if opened:
  try:browser('screenshot',str(out/'failure.png'),'--full');report['snapshot']=browser('snapshot','-i')
  except Exception:pass
finally:
 if opened:
  try:browser('close')
  except Exception:pass
 if server:
  (out/'stop').write_text('stop')
  try:server.wait(timeout=15)
  except subprocess.TimeoutExpired:server.terminate();server.wait(timeout=10)
 if log:log.close()
 (out/'result.json').write_text(json.dumps(report,ensure_ascii=False,indent=2)+'\n');print('Artifact:',out/'result.json',flush=True)
 if not report['passed']:raise SystemExit(1)
