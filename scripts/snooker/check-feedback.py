#!/usr/bin/env python3
"""Real 2D snooker UI/HTTP acceptance, authored before the frontend.
Failure inventory: unlisted creation, wrong 2/4 seats, unreadable 320px scores,
misfire on pan/zoom, score or sound preceding the timeline, stale animation on
reload, partner controlling a stroke, foul restoration or turn order lost.
All accounts, profiles, SQLite and evidence remain inside this workspace.
"""
import json,os,subprocess,time,uuid,urllib.request,http.cookiejar
from pathlib import Path
root=Path(__file__).resolve().parents[2];case=os.environ.get('WIRE_BOARD_SNOOKER_CASE','colour');assert case in ['colour','concede','black_foul'];hide_event=os.environ.get('WIRE_BOARD_SNOOKER_HIDDEN')=='1';reduced=os.environ.get('WIRE_BOARD_SNOOKER_REDUCED')=='1';fallback=os.environ.get('WIRE_BOARD_SNOOKER_FALLBACK')=='1';out=root/'.local/snooker/feedback'/(case+'-hidden' if hide_event else case+'-reduced' if reduced else case+'-fallback' if fallback else case);out.mkdir(parents=True,exist_ok=True);fixture=root/'.local/snooker/browser';fixture.mkdir(parents=True,exist_ok=True)
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
 room=api('/rooms',{'kind':'snooker','name':'斯诺克反馈验收','capacity':2},201)
 jar=http.cookiejar.CookieJar();client=urllib.request.build_opener(urllib.request.HTTPCookieProcessor(jar))
 def req(path,body=None):
  request=urllib.request.Request(base+'/api'+path,data=None if body is None else json.dumps(body).encode(),headers={'Content-Type':'application/json'})
  with client.open(request,timeout=25) as response:return json.load(response)
 req('/register',{'name':'斯诺克反馈对手','password':'snooker-browser-test-only','invite':'snooker-browser'})
 req('/rooms/'+room['id'],{'type':'join','version':api('/state')['room']['version'],'nonce':uuid.uuid4().hex})
 rr=req('/state')['room'];req('/rooms/'+rr['id'],{'type':'ready','version':rr['version'],'nonce':uuid.uuid4().hex});command('ready');command('start');browser('reload');browser('wait','.snooker-board');browser('set','viewport','390','900')
 if reduced:browser('set','media','light','reduced-motion');assert js('matchMedia("(prefers-reduced-motion: reduce)").matches')
 if case=='colour':
  browser('select','select[aria-label="本杆彩球"]','5')
  if js('document.querySelector(".sound-toggle").getAttribute("aria-pressed")==="true"'):browser('find','role','button','click','--name','取消静音','--exact')
  js("""window.__feedback={audio:[],score:[],spot:[]};const oldPlay=HTMLMediaElement.prototype.play;HTMLMediaElement.prototype.play=function(){const record={src:this.src,at:Date.now(),platform:this.hasAttribute('data-platform-audio')};window.__feedback.audio.push(record);return oldPlay.call(this).then(v=>{record.ok=true;return v},e=>{record.error=e.name;throw e})};new MutationObserver(()=>window.__feedback.score.push({at:Date.now(),value:document.querySelector('[data-snooker-score="0"]').textContent})).observe(document.querySelector('[data-snooker-score="0"]'),{subtree:true,characterData:true,childList:true});new MutationObserver(()=>window.__feedback.spot.push({at:Date.now(),ids:document.querySelector('.snooker-table').dataset.respot,noteOverlap:(()=>{const n=document.querySelector('.snooker-respot-note');if(!n)return false;const a=n.getBoundingClientRect(),b=document.querySelector('.snooker-table').getBoundingClientRect();return Math.min(a.right,b.right)>Math.max(a.left,b.left)&&Math.min(a.bottom,b.bottom)>Math.max(a.top,b.top)})()})).observe(document.querySelector('.snooker-table'),{attributes:true,attributeFilter:['data-respot']})""")
  shot('blue-before');strike(-90,.25);browser('wait','--fn','document.querySelector(".snooker-board").dataset.phase==="rolling"')
  d=api('/state')['room']['game']['data'];flight=d['flight'];assert d['scores']==[1,0];event=next(e for e in flight['motion']['events'] if e['kind']=='pot' and e['ball']==19);pot_at=flight['start']+event['time']*1000
  if hide_event:
   result=subprocess.run(['node','scripts/snooker/visibility-window.mjs',browser('get','cdp-url')['cdpUrl'],base,str(round(pot_at-220)),str(round(pot_at+110))],cwd=root,env=env,text=True,capture_output=True,check=True,timeout=20);report['visibility']=json.loads(result.stdout)
  browser('wait','--fn','document.querySelector(".snooker-board").dataset.phase==="aim"');browser('wait','--fn','window.__feedback.spot.some(e=>e.ids&&e.ids.includes("19"))')
  after=api('/state')['room']['game']['data'];assert after['scores']==[6,0];blue=next(b for b in after['balls'] if b['id']==19);assert not blue['down'] and blue['pos']=={'x':1.7845,'y':.889}
  observations=js('window.__feedback');assert not any(e.get('noteOverlap') for e in observations['spot']),'respot label covered a pocket';assert any(e['src'].endswith('/pocket.mp3') for e in observations['audio'])!=hide_event,'hidden pocket sound replayed or visible sound absent';assert all(e['platform'] for e in observations['audio']);assert hide_event or any(e.get('ok') and e['src'].endswith('/pocket.mp3') for e in observations['audio']),'pocket media failed to play';assert all(e['at']>=pot_at-35 for e in observations['audio'] if e['src'].endswith('/pocket.mp3'));assert all(e['at']>=pot_at-35 for e in observations['score'] if e['value']=='6');report['timeline']={'potAt':pot_at,'observations':observations};shot('blue-respotted')
  if fallback:assert any(e.get('error') and '/missing-snooker-assets/' in e['src'] for e in observations['audio']) and any(e.get('ok') and '/missing-snooker-assets/' not in e['src'] for e in observations['audio']),'local audio fallback not exercised'
  browser('reload');browser('wait','.snooker-board');assert js('document.querySelector(".snooker-table").dataset.respot')=='','reload repeated old respot animation';shot('blue-reload');command('close')
 elif case=='black_foul':
  strike(0,.06);browser('wait','--fn','document.querySelector(".snooker-board").dataset.phase==="finished"')
  r=api('/state')['room'];d=r['game']['data'];assert d['scores']==[1,134]
  assert not js('!!document.querySelector(".snooker-aim-preview")'),'finished match still offers aiming'
  assert any(not b['down'] and b['id']==21 for b in d['balls'])
  for width in [1280,390,320]:
   browser('set','viewport',str(width),'940');browser('scrollintoview','.snooker-end-explanation')
   explanation=js('document.querySelector(".snooker-end-explanation").textContent')
   assert all(t in explanation for t in ['未先击中目标球','7 分','首次得分或犯规']),explanation
   assert any('首次得分或犯规' in line for line in r['game']['log'])
   shot(f'black-foul-reason-{width}')
  browser('reload');browser('wait','.snooker-end-explanation');shot('black-foul-restored')
 else:
  shot('concession-offer');browser('find','role','button','click','--name','接受认输本局','--exact');browser('wait','--fn','document.querySelector(".snooker-board").dataset.phase==="finished"');assert api('/state')['room']['status']=='finished';shot('concession-accepted')
 command('leave');report['passed']=True
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
  (fixture/'stop').write_text('stop')
  try:server.wait(timeout=15)
  except subprocess.TimeoutExpired:server.terminate();server.wait(timeout=10)
 if log:log.close()
 (out/'result.json').write_text(json.dumps(report,ensure_ascii=False,indent=2)+'\n');print('Artifact:',out/'result.json',flush=True)
 if not report['passed']:raise SystemExit(1)
