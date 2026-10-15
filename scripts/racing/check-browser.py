#!/usr/bin/env python3
"""Real racing E2E, declared before implementation.
Failure inventory: new game listed without admin approval; nonstandard creation
or waiting layout; missing 2..6 seats/1,3,5 laps/three tracks; text/canvas/critical
resources clipped at 320 px; contrast below WCAG limits; keyboard bursts lost to
public busy state or stale response; old sequence from a second client replacing
new input; touch direction lock or second finger canceling steering; release,
blur, cancel or offline retaining held controls; local prediction freezing or
camera losing the car; visual skins changing physics; spectator/manual takeover
changing another car; mute failing to stop loops; reconnect replaying start;
computer unable to finish naturally. No teleport, clock rewrite or result seed.

Run after the production frontend is built:
  . scripts/env.sh && python3 -B scripts/racing/check-browser.py
Optional --build explicitly builds web/dist; do not run while another build owns it.
"""
import argparse, select, http.cookiejar, json, math, os, subprocess, time, traceback, urllib.request, urllib.error, uuid
from pathlib import Path

parser=argparse.ArgumentParser();parser.add_argument('--build',action='store_true');parser.add_argument('--probe-mobile-recovery',action='store_true');parser.add_argument('--probe-touch',action='store_true');parser.add_argument('--probe-key-pedals',action='store_true');args=parser.parse_args()
root=Path(__file__).resolve().parents[2];out=root/'.local/racing/browser';out.mkdir(parents=True,exist_ok=True)
env=dict(os.environ);session='rc-'+uuid.uuid4().hex[:6]
for key,rel in {'AGENT_BROWSER_SOCKET_DIR':'.local/racing/browser/sockets','AGENT_BROWSER_PROFILE':'.local/racing/browser/'+session,'TMPDIR':'.local/tmp','GOTMPDIR':'.local/tmp','GOCACHE':'.local/go-build','GOMODCACHE':'.local/go-mod','npm_config_cache':'.local/npm'}.items():
 p=root/rel;p.mkdir(parents=True,exist_ok=True);env[key]=str(p)
env.update(AGENT_BROWSER_SESSION=session,AGENT_BROWSER_ARGS='--mute-audio,--disable-breakpad,--disable-crash-reporter',WIRE_BOARD_RACING_BROWSER='1',GOTOOLCHAIN='local')
report={'passed':False,'checks':[],'scope':'production registry/auth/SQLite and host clock; native input; natural race finish','command':'. scripts/env.sh && python3 -B scripts/racing/check-browser.py','screenshots':[]};server=None;log=None;opened=False;diagnostic_cookie='';driver=None;driver_log=None

def b(*args):
 p=subprocess.run(['agent-browser',*args,'--json'],cwd=root,env=env,text=True,capture_output=True,timeout=45)
 if p.returncode:raise RuntimeError(p.stdout+p.stderr)
 value=json.loads(p.stdout);assert value['success'],value;return value['data']
def js(code):return b('eval',code)['result']
def api(path,body=None,status=200):
 opt='' if body is None else ',{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify('+json.dumps(body)+')}'
 r=js('fetch('+json.dumps('/api'+path)+opt+').then(async r=>({status:r.status,body:await r.json()}))')
 assert r['status']==status,r;return r['body']
def command(kind,**extra):
 payload=json.dumps({'type':kind,'nonce':uuid.uuid4().hex,**extra})
 r=js('(async()=>{const room=(await (await fetch("/api/state")).json()).room;const body={...'+payload+',version:room.version};const response=await fetch("/api/rooms/"+room.id,{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify(body)});return {status:response.status,body:await response.json()}})()')
 assert r['status']==200,r;return r['body']
def check(name,**evidence):
 report['checks'].append({'name':name,**evidence});print('PASS',name,flush=True)
def state():return api('/state')['room']
def native(mode):
 global driver,driver_log
 path=out/(mode+'-'+str(len(report['checks']))+'.json')
 if driver is None:
  driver_log=(out/'native-driver.log').open('w')
  driver=subprocess.Popen(['node','scripts/racing/native-input.mjs',b('get','cdp-url')['cdpUrl'],base,'serve'],cwd=root,env=env,text=True,stdin=subprocess.PIPE,stdout=subprocess.PIPE,stderr=driver_log,bufsize=1)
 driver.stdin.write(json.dumps({'mode':mode,'output':str(path)})+'\n');driver.stdin.flush()
 if not select.select([driver.stdout],[],[],65 if mode=='diagnose' else 40)[0]:raise TimeoutError('persistent native driver observation timed out: '+mode)
 line=driver.stdout.readline();print(line.strip(),flush=True);result=json.loads(line)
 if result.get('error'):raise RuntimeError(result['error'])
 return json.loads(path.read_text())

def instrument():
 js(r'''(()=>{window.__racingRequests=[];window.__racingAudio=[];window.__racingMedia=[];window.__racingImages=[];window.__racingNativeImage??=window.Image;window.Image=class extends window.__racingNativeImage{constructor(...args){super(...args);window.__racingImages.push(this)}};window.__racingNativeFetch??=window.fetch;const f=window.__racingNativeFetch;window.fetch=async(...args)=>{let record;try{const p=JSON.parse(args[1]?.body||'{}');if(p.type==='action'){record={at:performance.now(),action:p.action};window.__racingRequests.push(record)}}catch{}try{const r=await f(...args);if(record){record.status=r.status;record.done=performance.now()}return r}catch(e){if(record)record.error=e.name;throw e}};window.__racingNativePlay??=HTMLMediaElement.prototype.play;window.__racingNativePause??=HTMLMediaElement.prototype.pause;const play=window.__racingNativePlay,pause=window.__racingNativePause;HTMLMediaElement.prototype.play=function(){const record={type:'play',src:this.currentSrc||this.src,platform:this.hasAttribute('data-platform-audio'),at:performance.now()};window.__racingAudio.push(record);if(!window.__racingMedia.includes(this))window.__racingMedia.push(this);return play.call(this).then(v=>{record.ok=true;return v},e=>{record.error=e.name;throw e})};HTMLMediaElement.prototype.pause=function(){window.__racingAudio.push({type:'pause',src:this.currentSrc||this.src,at:performance.now()});return pause.call(this)}})()''')
def shot(name):
 js('Promise.all(document.getAnimations().filter(a=>a instanceof CSSTransition).map(a=>a.finished.catch(()=>{}))).then(()=>true)')
 assert js('document.documentElement.scrollWidth<=innerWidth+2'),('horizontal overflow',name)
 errors=b('errors')['errors'];assert not errors,(name,errors)
 colors=js(r'''(()=>{const rgb=s=>(s.match(/[\d.]+/g)||[]).map(Number),lum=c=>c.slice(0,3).map(v=>v/255).map(v=>v<=.04045?v/12.92:((v+.055)/1.055)**2.4).reduce((a,v,i)=>a+v*[.2126,.7152,.0722][i],0);const bg=e=>{let b=[255,255,255],chain=[];for(let n=e;n;n=n.parentElement)chain.unshift(n);for(const n of chain){const c=rgb(getComputedStyle(n).backgroundColor),a=c[3]??1;b=b.map((v,i)=>v*(1-a)+(c[i]??0)*a)}return b};return [...document.querySelectorAll('.racing-board *, .racing-options *, .player-summary *, .module-player-summary *, .racing-player-summary *')].filter(e=>e.getClientRects().length&&[...e.childNodes].some(n=>n.nodeType===3&&n.textContent.trim())).map(e=>{const s=getComputedStyle(e),a=lum(rgb(s.color)),z=lum(bg(e));return {text:e.textContent.trim().slice(0,75),class:e.className,foreground:s.color,background:bg(e),ratio:(Math.max(a,z)+.05)/(Math.min(a,z)+.05),minimum:parseFloat(s.fontSize)>=24||(parseFloat(s.fontSize)>=18.66&&+s.fontWeight>=700)?3:4.5}})})()''')
 assert all(c['ratio']>=c['minimum']-.01 for c in colors),(name,colors)
 (out/(name+'-contrast.json')).write_text(json.dumps(colors,ensure_ascii=False,indent=2))
 borders=js(r'''(()=>{const rgb=s=>(s.match(/[\d.]+/g)||[]).map(Number),lum=c=>c.slice(0,3).map(v=>v/255).map(v=>v<=.04045?v/12.92:((v+.055)/1.055)**2.4).reduce((a,v,i)=>a+v*[.2126,.7152,.0722][i],0),ratio=(a,b)=>(Math.max(lum(a),lum(b))+.05)/(Math.min(lum(a),lum(b))+.05),bg=e=>{let b=[255,255,255],chain=[];for(let n=e;n;n=n.parentElement)chain.unshift(n);for(const n of chain){const c=rgb(getComputedStyle(n).backgroundColor),a=c[3]??1;b=b.map((v,i)=>v*(1-a)+(c[i]??0)*a)}return b};return [...document.querySelectorAll('.racing-board button:not(:disabled),.racing-board select:not(:disabled)')].filter(e=>e.getClientRects().length).map(e=>({text:e.getAttribute('aria-label')||e.textContent.trim(),ratio:Math.max(ratio(bg(e),bg(e.parentElement)),ratio(rgb(getComputedStyle(e).borderTopColor),bg(e.parentElement)))}))})()''')
 assert all(c['ratio']>=2.99 for c in borders),(name,'important control boundary contrast',borders)
 (out/(name+'-controls-contrast.json')).write_text(json.dumps(borders,ensure_ascii=False,indent=2))
 path=out/(name+'.png');b('screenshot',str(path),'--full');report['screenshots'].append(str(path.relative_to(root)));check(name,contrast=colors)
def ensure_manual():
 r=state()
 if r['seats'][r['you']].get('autoPlay'):command('autoplay',enabled=False)
 b('wait','--fn','document.querySelector(".racing-controls")?.getAttribute("aria-disabled")!=="true"')
def controls(sample):
 c=sample['controls'];assert c is not None,sample;return {k:float(v) if k in ['steer','throttle'] else v=='true' for k,v in c.items() if k in ['steer','throttle','brake','drift']}
def close_room():
 if state()['status'] in ['waiting','playing']:command('close')
 command('leave');b('wait','.room-list')

try:
 for name in ['url','stop']:(out/name).unlink(missing_ok=True)
 if args.build:
  with (out/'build.log').open('w') as f:subprocess.run(['npm','--prefix','web','run','build'],cwd=root,env=env,stdout=f,stderr=subprocess.STDOUT,check=True)
 assert (root/'web/dist/index.html').exists(),'build frontend first'
 log=(out/'server.log').open('w');server=subprocess.Popen(['go','test','-count=1','-timeout=25m','-run','^TestRacingBrowserFixture$','./games/racing/tests'],cwd=root,env=env,stdout=log,stderr=subprocess.STDOUT)
 deadline=time.monotonic()+120
 while not (out/'url').exists():
  if server.poll() is not None or time.monotonic()>deadline:raise RuntimeError('fixture did not start; inspect server.log')
  time.sleep(.1)
 base=(out/'url').read_text();b('open',base);opened=True
 api('/login',{'name':'赛车验收管理员','password':'racing-browser-test-only'});diagnostic_cookie='; '.join(c['name']+'='+c['value'] for c in b('cookies','get')['cookies'])
 assert 'racing' not in api('/state')['availableGames'],'new game was automatically listed'
 api('/rooms',{'kind':'racing','name':'未上架赛车','capacity':2},403)
 api('/admin/games/racing',{'enabled':True});b('reload');b('wait','.room-list');check('default-unlisted-admin-only')
 if args.probe_mobile_recovery or args.probe_touch or args.probe_key_pedals:
  report['scope']='targeted real mobile auto-throttle recovery regression';report['command']+=(' --probe-key-pedals' if args.probe_key_pedals else ' --probe-touch' if args.probe_touch else ' --probe-mobile-recovery')
  b('set','viewport','1280' if args.probe_key_pedals else '320','844');api('/rooms',{'kind':'racing','name':'手机恢复回归','capacity':2,'options':{'track':'woodland','laps':1}},201)
  command('add_bot');command('ready');command('start');b('wait','.racing-board');b('wait','--fn','document.querySelector(".racing-board").dataset.phase==="racing"')
  js('document.querySelector(".racing-controls").scrollIntoView({block:"center",behavior:"instant"})');instrument()
  if args.probe_key_pedals:
   b('focus','canvas[aria-label="赛车赛道"]');native('slow-keyboard');native('blur');pedals=native('key-pedals');report['probe']=pedals;report['postHelperURL']=b('get','url');report['schedulerProbe']=native('diagnose');report['postHelperState']=state()['game']['data'];report['passed']=True;raise SystemExit(0)
  recovery=native('touch' if args.probe_touch else 'touch-recovery');report['probe']=recovery;rc={s['label']:controls(s) for s in recovery['samples']}
  if args.probe_touch:
   assert rc['two-finger-drift']['drift'] and rc['two-finger-drift']['steer']>.1,recovery
   assert not rc['drift-released-wheel-held']['drift'] and rc['drift-released-wheel-held']['steer']>.1,recovery
   report['passed']=True;raise SystemExit(0)
  assert rc['auto-active']['throttle']==1 and rc['wake-after-hidden']['throttle']==1 and rc['wake-after-reconnect']['throttle']==1,recovery
  assert rc['hidden']['throttle']==0 and rc['visible-without-gesture']['throttle']==0 and rc['reconnected-without-gesture']['throttle']==0,recovery
  check('mobile-recovery-needs-new-gesture',samples=recovery);report['passed']=True;raise SystemExit(0)
 # Create via the public catalog and retain the actual selected defaults.
 b('select','select[aria-label="按游戏筛选牌桌"]','racing');b('click','.lobby-create');b('wait','[role=dialog]')
 assert js('document.querySelector(".choose-games .selected").textContent.includes("逐风赛车")')
 assert js('[...document.querySelectorAll("select[aria-label=座位上限] option")].map(e=>e.value)')==['2','3','4','5','6']
 assert js('document.querySelector("select[aria-label=比赛圈数]").value')=='3'
 assert js('[...document.querySelectorAll("select[aria-label=比赛圈数] option")].map(e=>e.value)')==['1','3','5']
 assert js('[...document.querySelectorAll("select[aria-label=赛道] option")].map(e=>e.value)')==['woodland','coast','canyon']
 for width in [1280,390,320]:b('set','viewport',str(width),'900');shot('create-'+str(width))
 b('set','viewport','1280','940');b('select','select[aria-label="座位上限"]','6');b('select','select[aria-label="比赛圈数"]','1');b('fill','input[name=name]','逐风赛车真实验收');b('click','[role=dialog] button[type=submit]');b('wait','.waiting-actions')
 for _ in range(5):command('add_bot')
 assert len(state()['seats'])==6
 for width in [1280,390,320]:b('set','viewport',str(width),'900');shot('waiting-six-'+str(width))
 b('set','viewport','320','844');command('ready');command('start');command('autoplay',enabled=True);b('wait','.racing-board')
 viewport=js('(()=>{const rect=s=>{const e=document.querySelector(s),r=e.getBoundingClientRect();return {selector:s,top:r.top,bottom:r.bottom,left:r.left,right:r.right,height:r.height}};return {scrollY,height:innerHeight,parts:[".racing-hud","canvas[aria-label=赛车赛道]","[aria-label=方向盘]","button[aria-label=漂移]","button[aria-label=刹车]"].map(rect)}})()')
 report['firstScreen']=viewport;b('screenshot',str(out/'six-player-first-screen-320.png'));report['screenshots'].append(str((out/'six-player-first-screen-320.png').relative_to(root)))
 assert all(p['top']>=0 and p['bottom']<=viewport['height'] and p['left']>=0 and p['right']<=320 for p in viewport['parts']),('main racing controls not visible on first screen',viewport)
 check('six-player-first-screen-320',viewport=viewport)
 b('find','role','button','click','--name','车手与房间','--exact');b('wait','--fn','scrollY<=1')
 tick=js('Number(document.querySelector(".racing-board").dataset.tick)');b('wait','--fn',f'Number(document.querySelector(".racing-board").dataset.tick)>{tick+200}')
 assert js('scrollY<=1'),'snapshot forced view back down after user returned to driver resources'
 check('return-to-public-resources-not-forced')
 for width in [1280,390,320]:
  b('set','viewport',str(width),'900');b('wait','.racing-player-summary')
  summaries=js('[...document.querySelectorAll(".racing-player-summary")].map(e=>{const r=e.getBoundingClientRect();return {text:e.textContent,left:r.left,right:r.right,width:r.width,height:r.height}})')
  assert len(summaries)==6 and all(s['left']>=0 and s['right']<=width+1 and s['width']>20 and s['height']>15 for s in summaries),summaries
  shot('six-player-race-'+str(width))
 close_room()
 # Three real races exercise distinct tracks and actual native inputs at all widths.
 for width,track in [(390,'coast'),(320,'canyon'),(1280,'woodland')]:
  b('set','viewport',str(width),'940' if width==1280 else '844')
  api('/rooms',{'kind':'racing','name':'原生操作 '+track,'capacity':2,'options':{'track':track,'laps':1}},201)
  command('add_bot');command('ready');b('wait','.waiting-actions');instrument();command('start');b('wait','.racing-board');b('wait','canvas[aria-label="赛车赛道"]')
  b('wait','--fn','document.querySelector(".racing-board").dataset.phase==="racing"')
  # Inspect actual Canvas image objects decoded from the configured CDN.
  b('wait','--fn','window.__racingImages.filter(i=>/cars[.]webp|scenery[.]webp/.test(i.currentSrc)).length>=2&&window.__racingImages.filter(i=>/cars[.]webp|scenery[.]webp/.test(i.currentSrc)).every(i=>i.complete&&i.naturalWidth>0)')
  decoded=js('window.__racingImages.filter(i=>/cars[.]webp|scenery[.]webp/.test(i.currentSrc)).map(i=>({src:i.currentSrc,width:i.naturalWidth,height:i.naturalHeight,complete:i.complete}))')
  if env.get('WIRE_BOARD_RACING_ASSETS'):assert all(i['src'].startswith(env['WIRE_BOARD_RACING_ASSETS'].rstrip('/')+'/racing/v1/') for i in decoded),decoded
  check('decoded-canvas-assets-'+track,images=decoded)
  # One actual desktop/cellphone input burst, not synthetic dispatchEvent.
  js('document.querySelector(".racing-controls").scrollIntoView({block:"center",behavior:"instant"})');ensure_manual()
  if width==1280:b('focus','canvas[aria-label="赛车赛道"]')
  initial=state()['game']['data'];burst=native('slow-keyboard' if width==1280 else 'touch')
  samples={s['label']:controls(s) for s in burst['samples']}
  assert samples['left']['steer']<-.1 and samples['right']['steer']>.1 and samples['left-again']['steer']<-.1,burst
  assert abs(samples['released']['steer'])<.01,burst
  if width==1280:assert samples['drift']['drift'] and samples['released']['throttle']==0,burst
  else:
   assert samples['two-finger-drift']['drift'] and samples['two-finger-drift']['steer']>.1,burst
   assert not samples['drift-released-wheel-held']['drift'] and samples['drift-released-wheel-held']['steer']>.1,burst
   assert samples['brake']['brake'],burst
  requests=js('window.__racingRequests');drives=[r for r in requests if r['action']['type']=='drive']
  assert len(drives)>=3,drives
  if width!=1280:assert any(r['action']['steer']<-.1 for r in drives) and any(r['action']['steer']>.1 for r in drives),drives
  else:
   assert drives[-1]['action']['throttle']==0 and abs(drives[-1]['action']['steer'])<.01,('final release lost behind in-flight input',drives)
  assert all(a['action']['seq']<z['action']['seq'] for a,z in zip(drives,drives[1:])),drives
  assert all(r.get('status',200)<400 for r in drives),drives
  if width==1280:
   snapshots=[s['board'].get('tick') for s in burst['samples'] if s['label'].startswith('slow-snapshot')]
   rendered=[(s['canvas'].get('renderX'),s['canvas'].get('renderY')) for s in burst['samples'] if s['label'].startswith('slow-snapshot')]
   assert len(set(snapshots))>=2 and len(set(rendered))>=3,('slow-network snapshots/render froze',snapshots,rendered)
  check('native-input-'+str(width),burst=burst,requests=requests)
  # Stale sequence from another HTTP client with the same identity is rejected.
  r=state();car=r['game']['data']['cars'][r['you']];stale={'type':'drive','epoch':r['game']['data']['epoch'],'seq':max(1,car['seq']-1),'steer':1,'throttle':1,'brake':False,'drift':False}
  cookies=b('cookies','get')['cookies'];cookie='; '.join(c['name']+'='+c['value'] for c in cookies)
  req=urllib.request.Request(base+'/api/rooms/'+r['id'],data=json.dumps({'type':'action','version':r['version'],'nonce':uuid.uuid4().hex,'action':stale}).encode(),headers={'Content-Type':'application/json','Cookie':cookie})
  try:
   with urllib.request.urlopen(req,timeout=15) as response:stale_status=response.status
  except urllib.error.HTTPError as e:stale_status=e.code
  assert stale_status in [400,403,409],('old client sequence accepted',stale_status)
  check('stale-second-client-'+str(width),status=stale_status)
  if width==1280:
   b('focus','canvas[aria-label="赛车赛道"]');blur=native('blur');released=next(controls(s) for s in blur['samples'] if s['label']=='blurred');assert released['throttle']==0 and abs(released['steer'])<.01,blur;check('blur-releases-controls',samples=blur)
   pedals=native('key-pedals');pc={s['label']:controls(s) for s in pedals['samples']}
   assert pc['漂移-held']['drift'] and not pc['漂移-released']['drift'] and pc['刹车-held']['brake'] and not pc['刹车-released']['brake'],pedals
   assert not pc['pedal-blurred']['drift'] and not pc['pedal-blurred']['brake'] and not pc['follow-button-space']['drift'],pedals
   check('keyboard-pedals',samples=pedals)
  else:
   canceled=native('touch-cancel');released=next(controls(s) for s in canceled['samples'] if s['label']=='released');assert not released['drift'] and abs(released['steer'])<.01,canceled;check('touch-cancel-'+str(width),samples=canceled)
   recovery=native('touch-recovery');rc={s['label']:controls(s) for s in recovery['samples']}
   assert rc['auto-active']['throttle']==1 and rc['wake-after-hidden']['throttle']==1 and rc['wake-after-reconnect']['throttle']==1,recovery
   assert rc['hidden']['throttle']==0 and rc['visible-without-gesture']['throttle']==0 and rc['reconnected-without-gesture']['throttle']==0,recovery
   check('mobile-recovery-needs-new-gesture-'+str(width),samples=recovery)
  # Appearance travels through a legal skin action, never replaces drive seq.
  skin_before=state()['game']['data'];skin_car=skin_before['cars'][0];skin=(skin_car['skin']+1)%5
  b('select','select[aria-label="车辆外观"]',str(skin));b('wait','--fn',f'window.__racingRequests.some(r=>r.action.type==="skin"&&r.action.skin==={skin}&&r.status===200)')
  skin_after=state()['game']['data'];assert skin_after['cars'][0]['skin']==skin
  assert skin_after['epoch']==skin_before['epoch'] and skin_after['cars'][0]['lap']>=skin_car['lap']
  assert skin_after['cars'][0]['seq']>=skin_car['seq']
  check('cosmetic-skin-'+str(width),before=skin_car,after=skin_after['cars'][0])
  command('autoplay',enabled=True)
  # Visual evidence + continuous render observations while the real AI drives.
  motion=native('motion');positions=[(s['canvas'].get('renderX'),s['canvas'].get('renderY')) for s in motion['samples']];assert len(set(positions))>3,('render motion froze',positions)
  report.setdefault('motion',[]).append({'width':width,**motion})
  geometry=js('(()=>{const c=document.querySelector("canvas[aria-label=赛车赛道]"),r=c.getBoundingClientRect();return {width:r.width,height:r.height,data:{...c.dataset},viewport:{width:innerWidth,height:innerHeight}}})()')
  assert geometry['height']>=240 and geometry['width']/geometry['height']<=2.5,geometry
  shot('race-'+track+'-'+str(width))
  js('document.querySelector("canvas[aria-label=赛车赛道]").scrollIntoView({block:"center",behavior:"instant"})')
  camera=native('camera' if width==1280 else 'pinch');camera_samples=[s['canvas'] for s in camera['samples']]
  assert float(camera_samples[1]['cameraScale'])>float(camera_samples[0]['cameraScale']),camera
  if width==1280:
   assert camera_samples[-1]['follow']=='false',camera
   anchor_before=float(camera_samples[0]['cameraX'])+float(camera_samples[0]['width'])*.1/float(camera_samples[0]['cameraScale'])
   anchor_after=float(camera_samples[1]['cameraX'])+float(camera_samples[1]['width'])*.1/float(camera_samples[1]['cameraScale'])
   assert abs(anchor_after-anchor_before)<1 and abs(float(camera_samples[1]['cameraY'])-float(camera_samples[0]['cameraY']))<1,('wheel anchor moved',camera)
   assert camera_samples[2]['cameraX']!=camera_samples[1]['cameraX'] or camera_samples[2]['cameraY']!=camera_samples[1]['cameraY'],camera
   assert float(camera_samples[4]['cameraX'])>float(camera_samples[3]['cameraX'])+50,('minimap drag did not continuously reposition',camera)
  b('find','role','button','click','--name','跟车','--exact')
  b('wait','--fn','document.querySelector("canvas[aria-label=赛车赛道]").dataset.follow==="true"');check('camera-input-'+str(width),samples=camera)
  # Platform takeover must retain this race epoch and existing lap progress.
  before=state();command('autoplay',enabled=True);b('wait','--fn','document.querySelector(".autoplay-control")?.textContent.includes("取消托管")');after=state()
  assert after['game']['data']['epoch']==before['game']['data']['epoch']
  assert all(a['lap']>=z['lap'] for a,z in zip(after['game']['data']['cars'],before['game']['data']['cars']))
  if width==1280:
   command('autoplay',enabled=False);b('wait','--fn','document.querySelector(".autoplay-control")?.textContent.includes("开启托管")');assert state()['game']['data']['epoch']==before['game']['data']['epoch'];command('autoplay',enabled=True)
   # Native loop media stop immediately when the shared platform mute is used.
   if js('document.querySelector(".sound-toggle")?.getAttribute("aria-pressed")==="true"'):b('find','role','button','click','--name','取消静音','--exact')
   b('wait','--fn','window.__racingAudio.some(a=>a.type==="play"&&a.src.includes("/racing/")&&a.ok)')
   b('find','role','button','click','--name','静音','--exact');assert js('window.__racingMedia.every(m=>m.paused)'),js('window.__racingAudio')
   assert js('window.__racingAudio.filter(a=>a.type==="play"&&a.src.includes("/racing/")).every(a=>a.platform)')
   check('platform-audio-mute',events=js('window.__racingAudio'))
  if width==1280:
   tick_before=state()['game']['data']['serverAt'];starts=js('window.__racingAudio.filter(a=>a.type==="play"&&a.src.includes("start.")).length')
   recovery=native('offline');b('wait','--fn',f'Number(document.querySelector(".racing-board").dataset.tick)>{tick_before}')
   assert js('window.__racingAudio.filter(a=>a.type==="play"&&a.src.includes("start.")).length')==starts,'reconnect replayed start audio'
   check('network-recovery',samples=recovery)
  # Real-time AI completes a full race; record checkpoints/laps throughout.
  history=[];deadline=time.monotonic()+150
  while time.monotonic()<deadline:
   r=state();d=r['game']['data'];history.append({'at':d['serverAt'],'phase':d['phase'],'cars':[{k:c[k] for k in ['seat','lap','checkpoint','progress','rank','finishAt']} for c in d['cars']]})
   if r['status']=='finished' or d['phase']=='finished':break
   time.sleep(2)
  assert d['phase']=='finished',('natural race failed to finish',d)
  assert any(c['finishAt']>0 and c['lap']==1 for c in d['cars']),d
  assert any(h['cars'][0]['checkpoint']>0 for h in history),history
  (out/('natural-'+track+'.json')).write_text(json.dumps(history,ensure_ascii=False,indent=2))
  check('natural-finish-'+track,final=d,steps=len(history));shot('finish-'+track+'-'+str(width));close_room()
 # A separate real spectator sees public progress and cannot submit controls.
 b('set','viewport','390','844');api('/rooms',{'kind':'racing','name':'观战只读验收','capacity':2,'options':{'track':'woodland','laps':1}},201)
 command('add_bot');command('ready');command('start');command('autoplay',enabled=True)
 watched=state();owner_cookies=b('cookies','get')['cookies'];owner_cookie='; '.join(c['name']+'='+c['value'] for c in owner_cookies)
 jar=http.cookiejar.CookieJar();observer=urllib.request.build_opener(urllib.request.HTTPCookieProcessor(jar))
 def observe(path,body=None):
  request=urllib.request.Request(base+'/api'+path,data=None if body is None else json.dumps(body).encode(),headers={'Content-Type':'application/json'})
  with observer.open(request,timeout=15) as response:return json.load(response)
 observe('/register',{'name':'赛车真实观众','password':'racing-browser-test-only','invite':'racing-browser'})
 observe('/rooms/'+watched['id']+'/watch',{})
 b('cookies','clear')
 for c in jar:b('cookies','set',c.name,c.value,'--url',base,'--httpOnly')
 b('reload');b('wait','.racing-board');instrument();view=state();assert view['spectating'] and not view['game']['data']['controls']['canDrive']
 before_tick=view['game']['data']['serverAt'];native('keyboard');assert not js('window.__racingRequests.some(r=>r.action.type==="drive")'),'spectator keyboard submitted drive'
 b('wait','--fn',f'Number(document.querySelector(".racing-board").dataset.tick)>{before_tick}');shot('spectator-mobile-readonly')
 b('cookies','clear')
 for c in owner_cookies:b('cookies','set',c['name'],c['value'],'--url',base,'--httpOnly')
 b('reload');b('wait','.racing-board');close_room()
 report['passed']=True
except Exception as error:
 traceback.print_exc();report['error']=repr(error)
 if diagnostic_cookie:
  try:
   began=time.monotonic();request=urllib.request.Request(base+'/api/state',headers={'Cookie':diagnostic_cookie})
   with urllib.request.urlopen(request,timeout=6) as response:
    value=json.load(response);room=value.get('room') or {};report['independentHTTP']={'status':response.status,'ms':(time.monotonic()-began)*1000,'room':room.get('id'),'phase':room.get('game',{}).get('data',{}).get('phase'),'serverAt':room.get('game',{}).get('data',{}).get('serverAt')}
  except Exception as diagnostic_error:report['independentHTTP']={'error':repr(diagnostic_error)}
  try:report['cdpDiagnostic']=native('diagnose')
  except Exception as diagnostic_error:report['cdpDiagnostic']={'error':repr(diagnostic_error)}
 if opened:
  try:report['snapshot']=b('snapshot','-i');b('screenshot',str(out/'failure.png'),'--full')
  except Exception:pass
finally:
 if driver:
  driver.stdin.close()
  try:driver.wait(timeout=15)
  except subprocess.TimeoutExpired:driver.terminate();driver.wait(timeout=10)
 if driver_log:driver_log.close()
 if opened:
  try:b('close')
  except Exception:pass
 if server:
  (out/'stop').write_text('stop')
  try:server.wait(timeout=15)
  except subprocess.TimeoutExpired:server.terminate();server.wait(timeout=10)
 if log:log.close()
 report['assetsBase']=env.get('WIRE_BOARD_RACING_ASSETS','local')
 if env.get('WIRE_BOARD_RACING_ASSETS'):report['command']=report['command'].replace('&& python3','&& WIRE_BOARD_RACING_ASSETS='+env['WIRE_BOARD_RACING_ASSETS']+' python3')
 (out/'result.json').write_text(json.dumps(report,ensure_ascii=False,indent=2)+'\n');print('Artifact:',out/'result.json',flush=True)
 if not report['passed']:raise SystemExit(1)
