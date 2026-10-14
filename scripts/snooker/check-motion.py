#!/usr/bin/env python3
"""Real 2D snooker UI/HTTP acceptance, authored before the frontend.
Failure inventory: unlisted creation, wrong 2/4 seats, unreadable 320px scores,
misfire on pan/zoom, score or sound preceding the timeline, stale animation on
reload, partner controlling a stroke, foul restoration or turn order lost.
All accounts, profiles, SQLite and evidence remain inside this workspace.
"""
import json,os,subprocess,time,uuid,urllib.request,http.cookiejar
from pathlib import Path
resume=os.environ.get('WIRE_BOARD_SNOOKER_RESUME')=='1'
viewer=os.environ.get('WIRE_BOARD_SNOOKER_VIEWER')=='1';reject=os.environ.get('WIRE_BOARD_SNOOKER_REJECT')=='1'
root=Path(__file__).resolve().parents[2];case=os.environ.get('WIRE_BOARD_SNOOKER_CASE','colour');assert case in ['colour','colour_bot'];hide_event=os.environ.get('WIRE_BOARD_SNOOKER_HIDDEN')=='1';reduced=os.environ.get('WIRE_BOARD_SNOOKER_REDUCED')=='1';fallback=os.environ.get('WIRE_BOARD_SNOOKER_FALLBACK')=='1';out=root/'.local/snooker/motion'/(case+'-hidden' if hide_event else case+'-reduced' if reduced else case+'-fallback' if fallback else case+('-viewer' if viewer else '-reject' if reject else '-resume' if resume else ''));out.mkdir(parents=True,exist_ok=True);fixture=root/'.local/snooker/browser';fixture.mkdir(parents=True,exist_ok=True)
env=dict(os.environ);session='sn-'+uuid.uuid4().hex[:6]
for key,rel in {'AGENT_BROWSER_SOCKET_DIR':'.local/ab-sn','AGENT_BROWSER_PROFILE':'.local/snooker/browser/'+session,'TMPDIR':'.local/tmp','GOTMPDIR':'.local/tmp','GOCACHE':'.local/go-build','GOMODCACHE':'.local/go-mod','npm_config_cache':'.local/npm'}.items():
 p=root/rel;p.mkdir(parents=True,exist_ok=True);env[key]=str(p)
env.update(AGENT_BROWSER_SESSION=session,AGENT_BROWSER_ARGS='--mute-audio,--disable-breakpad,--disable-crash-reporter',WIRE_BOARD_SNOOKER_BROWSER='1',WIRE_BOARD_SNOOKER_CASE=case,GOTOOLCHAIN='local')
if fallback:env['WIRE_BOARD_SNOOKER_ASSETS']='https://cdn.ichenj.com/bga/v20261002-1/missing-snooker-assets'
report={'passed':False,'checks':[],'scope':'labelled milestone layouts; real production HTTP, physics, referee and frontend'};server=None;log=None;opened=False

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
 return [...document.querySelectorAll('.snooker-status span,.snooker-status strong,.snooker-scoreboard small,.snooker-scoreboard span,.snooker-scoreboard strong,.snooker-controls label,.snooker-controls output,.snooker-end-explanation p,.snooker-end-explanation strong,.snooker-board button,.snooker-turn,.snooker-order,.snooker-public-summary strong')].filter(e=>e.getClientRects().length).map(e=>{let bg=[255,255,255];const chain=[];for(let n=e;n;n=n.parentElement)chain.unshift(n);for(const n of chain){const c=rgb(getComputedStyle(n).backgroundColor),a=c[3]??1;bg=bg.map((v,i)=>v*(1-a)+(c[i]??0)*a)}const fg=rgb(getComputedStyle(e).color),a=lum(fg),b=lum(bg);return {text:e.textContent.trim().slice(0,80),ratio:(Math.max(a,b)+.05)/(Math.min(a,b)+.05)}});
 })()""")
 assert all(c['ratio']>=4.5 for c in colors),(name,colors)
 (out/(name+'-contrast.json')).write_text(json.dumps(colors,ensure_ascii=False,indent=2))
 browser('screenshot',str(out/(name+'.png')),'--full');report['checks'].append(name);print('PASS',name,flush=True)
try:
 for name in ['url','stop']:(fixture/name).unlink(missing_ok=True)
 with (out/'build.log').open('w') as f:subprocess.run(['npm','--prefix','web','run','build'],cwd=root,env=env,stdout=f,stderr=subprocess.STDOUT,check=True)
 log=(out/'server.log').open('w');server=subprocess.Popen(['go','test','-count=1','-timeout=21m','-run','^TestSnookerBrowserFixture$','./internal/gametest'],cwd=root,env=env,stdout=log,stderr=subprocess.STDOUT)
 start=time.monotonic()
 while not (fixture/'url').exists():
  if server.poll() is not None or time.monotonic()-start>90:raise RuntimeError('snooker fixture did not start')
  time.sleep(.1)
 base=(fixture/'url').read_text();browser('open',base);opened=True;api('/login',{'name':'斯诺克验收管理员','password':'snooker-browser-test-only'})
 assert 'snooker' not in api('/state')['availableGames']
 api('/rooms',{'kind':'snooker','name':'下架不能创建','capacity':2},403)
 api('/admin/games/snooker',{'enabled':True});browser('reload');browser('wait','.room-list');browser('select','select[aria-label="游戏分类"]','sports')
 browser('set','viewport','390','940')
 if reduced:browser('set','media','light','reduced-motion')
 room=api('/rooms',{'kind':'snooker','name':'本地动画慢网验收','capacity':2},201)
 if case=='colour_bot':command('add_bot')
 else:
  jar=http.cookiejar.CookieJar();client=urllib.request.build_opener(urllib.request.HTTPCookieProcessor(jar))
  def peer(path,body=None):
   req=urllib.request.Request(base+'/api'+path,data=None if body is None else json.dumps(body).encode(),headers={'Content-Type':'application/json'})
   with client.open(req,timeout=25) as response:return json.load(response)
  peer('/register',{'name':'动画对手','password':'snooker-browser-test-only','invite':'snooker-browser'})
  r=api('/state')['room'];peer('/rooms/'+r['id'],{'type':'join','version':r['version'],'nonce':uuid.uuid4().hex})
  r=peer('/state')['room'];peer('/rooms/'+r['id'],{'type':'ready','version':r['version'],'nonce':uuid.uuid4().hex})
 command('ready')
 if viewer:
  cookies=browser('cookies','get')['cookies'];cookie='; '.join(c['name']+'='+c['value'] for c in cookies)
  def owner(path,body=None):
   req=urllib.request.Request(base+'/api'+path,data=None if body is None else json.dumps(body).encode(),headers={'Content-Type':'application/json','Cookie':cookie})
   with urllib.request.urlopen(req,timeout=20) as response:return json.load(response)
  r=owner('/state')['room'];owner('/rooms/'+r['id'],{'type':'start','version':r['version'],'nonce':uuid.uuid4().hex})
  browser('cookies','clear');api('/register',{'name':'动画观战者','password':'snooker-browser-test-only','invite':'snooker-browser'})
  api('/rooms/'+r['id']+'/watch',{})
 browser('reload');browser('wait','.snooker-board' if viewer else '.waiting-actions')
 if js('document.querySelector(".sound-toggle").getAttribute("aria-pressed")==="true"'):browser('find','role','button','click','--name','取消静音','--exact')
 browser('find','role','button','click','--name','试听音效','--exact')
 browser('wait','--fn','document.querySelector("[data-platform-audio]")?.paused===true')
 js(r"""(() => {
 window.__motion={cue:[],pendingCue:[],balls:[],audio:[],scores:[],resizes:0,flights:{},received:{},done:false};
 const r=window.__motion,originalFetch=window.fetch;
 window.fetch=async(...args)=>{const response=await originalFetch(...args);if(String(args[0]).includes('/api/state'))response.clone().json().then(s=>{const d=s.room?.game?.data;if(d?.flight){r.flights[d.flight.id]??=d.flight;r.received[d.flight.id]??=Date.now();if((d.flight.id===1&&d.phase!=='rolling')||d.flight.id>1)r.done=true}});return response};
 const main=c=>c.canvas.matches('.snooker-table canvas')&&document.querySelector('.snooker-board')?.dataset.phase==='rolling';
 const stroke=CanvasRenderingContext2D.prototype.stroke,draw=CanvasRenderingContext2D.prototype.drawImage;
 CanvasRenderingContext2D.prototype.drawImage=function(...a){if(main(this)&&a[0]?.src?.endsWith('/ball-0.webp')){const p=this.getTransform().transformPoint(new DOMPoint(a[1]+.02625,a[2]+.02625));r.white=p;r.balls.push({at:Date.now(),x:a[1]+.02625,y:a[2]+.02625})}return draw.apply(this,a)};
 CanvasRenderingContext2D.prototype.stroke=function(...a){if(this.canvas.matches('.snooker-table canvas')&&document.querySelector('.snooker-board')?.dataset.phase==='aim'&&document.querySelector('.snooker-board')?.dataset.shotPending==='true'&&this.strokeStyle==='#dfc591')r.pendingCue.push(Date.now());if(main(this)&&this.strokeStyle==='#dfc591'&&r.white){const m=this.getTransform(),p=m.transformPoint(new DOMPoint(-.02625-.028,0));r.cue.push({at:Date.now(),gap:Math.hypot(p.x-r.white.x,p.y-r.white.y)/(.02625*Math.hypot(m.a,m.b))})}return stroke.apply(this,a)};
 for(const key of ['width','height']){const d=Object.getOwnPropertyDescriptor(HTMLCanvasElement.prototype,key);Object.defineProperty(HTMLCanvasElement.prototype,key,{...d,set(v){if(this.matches('.snooker-table canvas')&&document.querySelector('.snooker-board')?.dataset.phase==='rolling')r.resizes++;return d.set.call(this,v)}})}
 const play=HTMLMediaElement.prototype.play;HTMLMediaElement.prototype.play=function(){r.audio.push({at:Date.now(),src:this.src});return play.call(this)};
 new MutationObserver(()=>{const values=[...document.querySelectorAll('[data-snooker-score]')].map(e=>e.textContent);if(values.length&&JSON.stringify(values)!==JSON.stringify(r.scores.at(-1)?.values))r.scores.push({at:Date.now(),values})}).observe(document.body,{subtree:true,childList:true,characterData:true});
 })()""")
 endpoint=browser('get','cdp-url')['cdpUrl']
 subprocess.run(['node','scripts/snooker/throttle.mjs',endpoint,base,'250','4'],cwd=root,env=env,check=True,timeout=20)
 if not viewer:command('start')
 browser('wait','.snooker-table canvas')
 if reject:
  js(r'''const previous=window.fetch;window.fetch=(url,options)=>{if(options?.body){const a=JSON.parse(options.body);if(a.type==='action'&&a.action?.type==='shoot'){a.action.window=-1;options={...options,body:JSON.stringify(a)}}}return previous(url,options)}''')
 if viewer:
  r=owner('/state')['room'];owner('/rooms/'+r['id'],{'type':'action','version':r['version'],'nonce':uuid.uuid4().hex,'action':{'type':'shoot','angle':-1.5707963267948966,'power':.25,'nominate':5,'window':r['game']['data']['window']}})
 elif case=='colour':
  browser('select','select[aria-label="本杆彩球"]','5');strike(0,.5) if resume else strike(-90,.25)
 if reject:
  browser('wait','--fn','document.querySelector(".snooker-board").dataset.shotPending==="false"&&!!document.querySelector(".toast.error")')
  observed=js('window.__motion');r=api('/state')['room']
  assert observed['pendingCue'],'no immediate local cue before rejected confirmation'
  assert not r['game']['data'].get('flight') and r['game']['data']['scores']==[1,0]
  assert not observed['balls'] and not any(a['src'].endswith('/cue.mp3') for a in observed['audio'])
  shot('rejected-shot-restored');report['motion']=observed;report['passed']=True
 else:
  browser('wait','--fn','window.__motion.cue.length>0');shot('cue-visible')
  if resume:
   browser('wait','--fn','Number(document.querySelector(".snooker-board").dataset.flightTime)>.25')
   before=float(js('document.querySelector(".snooker-board").dataset.flightTime'));report['motion']=js('window.__motion')
   browser('reload');browser('wait','.snooker-board')
   after=js('({phase:document.querySelector(".snooker-board").dataset.phase,age:Number(document.querySelector(".snooker-board").dataset.flightTime),cue:document.querySelector(".snooker-table canvas").dataset.cueOffset??null})')
   assert after['age']>=before and after['cue'] is None,('old cue replayed after reload',before,after)
   report['resume']={'before':before,'after':after};shot('resume-at-current-progress');report['passed']=True
  else:
   browser('wait','--fn','window.__motion.done')
   browser('wait','--fn','document.querySelector(".snooker-board").dataset.phase!=="rolling"')
   observed=js('window.__motion');report['motion']=observed;flight=observed['flights']['1'];initial=next(b['pos'] for b in flight['motion']['frames'][0]['balls'] if b['id']==0)
   moving=next(b for b in observed['balls'] if abs(b['x']-initial['x'])+abs(b['y']-initial['y'])>.0005)
   assert observed['cue'][0]['gap']>1.4,'first rendered cue already touched the white: missing wind-up'
   assert moving['at']-observed['cue'][0]['at']>=150,'missing locally played shot prefix'
   cues=[a for a in observed['audio'] if a['src'].endswith('/cue.mp3')]
   assert cues and abs(cues[0]['at']-moving['at'])<120,'cue sound and ball motion diverged'
   assert observed['resizes']<=4,'canvas was reallocated on every animation frame'
   if len(observed['scores'])>1:
    pots=[e['time'] for e in flight['motion']['events'] if e['kind']=='pot']
    if pots:assert observed['scores'][1]['at']>=moving['at']+min(pots)*1000-100,'visible score preceded local pocket event'
   if case=='colour' and not viewer:assert observed['pendingCue'] and observed['pendingCue'][0]<observed['received']['1'],'local response waited for server confirmation'
   report['motion']=observed;shot('after-motion')
   subprocess.run(['node','scripts/snooker/throttle.mjs',endpoint,base,'0','1'],cwd=root,env=env,check=True,timeout=20)
   report['passed']=True
except Exception as error:
 import traceback;traceback.print_exc();report['error']=repr(error)
 if opened:
  try:report['motion']=js('window.__motion');browser('screenshot',str(out/'failure.png'),'--full');report['snapshot']=browser('snapshot','-i')
  except Exception:pass
finally:
 if opened:
  try:browser('close')
  except Exception:pass
 if server:
  (fixture/'stop').write_text('stop')
  try:server.wait(timeout=15)
  except subprocess.TimeoutExpired:server.terminate();server.wait(timeout=10)
 if log:log.close()
 (out/'result.json').write_text(json.dumps(report,ensure_ascii=False,indent=2)+'\n');print('Artifact:',out/'result.json',flush=True)
 if not report['passed']:raise SystemExit(1)
