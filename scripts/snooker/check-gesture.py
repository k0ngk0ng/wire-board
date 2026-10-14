#!/usr/bin/env python3
"""Direct shooting acceptance, authored before replacing the old control panel.
Failure inventory: unlisted creation, wrong 2/4 seats, unreadable 320px scores,
misfire on pan/zoom, score or sound preceding the timeline, stale animation on
reload, partner controlling a stroke, foul restoration or turn order lost.
All accounts, profiles, SQLite and evidence remain inside this workspace.
"""
import json,os,subprocess,time,uuid,urllib.request,http.cookiejar
from pathlib import Path
root=Path(__file__).resolve().parents[2];out=root/'.local/snooker/gesture';out.mkdir(parents=True,exist_ok=True)
env=dict(os.environ);session='sn-'+uuid.uuid4().hex[:6]
for key,rel in {'AGENT_BROWSER_SOCKET_DIR':'.local/ab-sn','AGENT_BROWSER_PROFILE':'.local/snooker/browser/'+session,'TMPDIR':'.local/tmp','GOTMPDIR':'.local/tmp','GOCACHE':'.local/go-build','GOMODCACHE':'.local/go-mod','npm_config_cache':'.local/npm'}.items():
 p=root/rel;p.mkdir(parents=True,exist_ok=True);env[key]=str(p)
env.update(AGENT_BROWSER_SESSION=session,AGENT_BROWSER_ARGS='--mute-audio,--disable-breakpad,--disable-crash-reporter',WIRE_BOARD_SNOOKER_BROWSER='1',GOTOOLCHAIN='local')
fixture=root/'.local/snooker/browser';fixture.mkdir(parents=True,exist_ok=True)
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
def shot(name):
 assert not browser('errors')['errors'],browser('errors')
 assert js('document.documentElement.scrollWidth<=innerWidth+2'),name
 colors=js(r"""(() => {
 const rgb=s=>(s.match(/[\d.]+/g)||[]).map(Number),lum=c=>c.slice(0,3).map(v=>v/255).map(v=>v<=.04045?v/12.92:((v+.055)/1.055)**2.4).reduce((s,v,i)=>s+v*[.2126,.7152,.0722][i],0);
 return [...document.querySelectorAll('.snooker-status span,.snooker-status strong,.snooker-scoreboard small,.snooker-scoreboard span,.snooker-scoreboard strong,.snooker-controls label,.snooker-controls output,.snooker-charge span,.snooker-aim-readout strong,.snooker-aim-readout > span,.snooker-board button,.snooker-turn,.snooker-order,.snooker-public-summary strong')].filter(e=>e.getClientRects().length).map(e=>{let bg=[255,255,255];const chain=[];for(let n=e;n;n=n.parentElement)chain.unshift(n);for(const n of chain){const c=rgb(getComputedStyle(n).backgroundColor),a=c[3]??1;bg=bg.map((v,i)=>v*(1-a)+(c[i]??0)*a)}const fg=rgb(getComputedStyle(e).color),a=lum(fg),b=lum(bg);return {text:e.textContent.trim().slice(0,80),ratio:(Math.max(a,b)+.05)/(Math.min(a,b)+.05)}});
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
def assert_clear_table(label):
 geometry=js(r"""(() => {
 const table=document.querySelector('.snooker-table canvas'),preview=document.querySelector('.snooker-aim-preview canvas');
 if(!preview)return {error:'missing off-table magnifier'};
 const rect=e=>{const r=e.getBoundingClientRect();return {x:r.x,y:r.y,width:r.width,height:r.height,right:r.right,bottom:r.bottom}},t=rect(table),p=rect(preview);
 const overlaps=r=>Math.min(t.right,r.right)>Math.max(t.x,r.x)&&Math.min(t.bottom,r.bottom)>Math.max(t.y,r.y);
 const overlays=[...document.querySelectorAll('.snooker-aim-preview,.snooker-charge')].filter(e=>e.getClientRects().length).map(e=>({class:e.className,rect:rect(e),overlap:overlaps(rect(e))}));
 return {table:t,preview:p,overlays,previewPixels:preview.getContext('2d').getImageData(0,0,preview.width,preview.height).data.some(v=>v>0)};
 })()""")
 assert 'error' not in geometry,(label,geometry)
 assert geometry['preview']['width']>=80 and geometry['preview']['height']>=80,(label,geometry)
 assert geometry['previewPixels'],(label,'empty magnifier')
 assert not any(o['overlap'] for o in geometry['overlays']),(label,geometry)
 report.setdefault('unobstructedTable',[]).append({'label':label,**geometry})

try:
 for name in ['url','stop']:(fixture/name).unlink(missing_ok=True)
 with (out/'build.log').open('w') as f:subprocess.run(['npm','--prefix','web','run','build'],cwd=root,env=env,stdout=f,stderr=subprocess.STDOUT,check=True)
 log=(out/'server.log').open('w');server=subprocess.Popen(['go','test','-count=1','-timeout=21m','-run','^TestSnookerBrowserFixture$','./internal/gametest'],cwd=root,env=env,stdout=log,stderr=subprocess.STDOUT)
 start=time.monotonic()
 while not (fixture/'url').exists():
  if server.poll() is not None or time.monotonic()-start>90:raise RuntimeError('fixture did not start')
  time.sleep(.1)
 base=(fixture/'url').read_text();browser('open',base);opened=True
 api('/login',{'name':'斯诺克验收管理员','password':'snooker-browser-test-only'})
 api('/admin/games/snooker',{'enabled':True})
 for width in [1280,390,320]:
  browser('set','viewport',str(width),'940')
  api('/rooms',{'kind':'snooker','name':'按住蓄力验收','capacity':2},201)
  jar=http.cookiejar.CookieJar();client=urllib.request.build_opener(urllib.request.HTTPCookieProcessor(jar))
  def peer(path,body=None):
   req=urllib.request.Request(base+'/api'+path,data=None if body is None else json.dumps(body).encode(),headers={'Content-Type':'application/json'})
   with client.open(req,timeout=25) as response:return json.load(response)
  peer('/register',{'name':f'蓄力对手{width}','password':'snooker-browser-test-only','invite':'snooker-browser'})
  r=api('/state')['room'];peer('/rooms/'+r['id'],{'type':'join','version':r['version'],'nonce':uuid.uuid4().hex})
  r=peer('/state')['room'];peer('/rooms/'+r['id'],{'type':'ready','version':r['version'],'nonce':uuid.uuid4().hex})
  command('ready');command('start');r=api('/state')['room'];d=r['game']['data']
  (api if d['turn']==0 else peer)('/rooms/'+r['id'],{'type':'action','version':r['version'],'nonce':uuid.uuid4().hex,'action':{'type':'break_choice','window':d['window'],'team':0}})
  browser('reload');browser('wait','.snooker-table canvas');browser('scrollintoview','.snooker-table canvas')
  assert not js('!!document.querySelector("input[type=number],.snooker-strike")'),"old menu controls remain"
  r=api('/state')['room'];initial=r['version']
  assert r['turnDeadline']>0 and js('!!document.querySelector(".turn-clock time")'), 'missing visible automatic takeover countdown'
  rect=js('document.querySelector(".snooker-table canvas").getBoundingClientRect().toJSON()');x=round(rect['x']+rect['width']*.52);y=round(rect['y']+rect['height']*.6)
  # Regression: the old top-right lens both covered a pocket and swallowed input.
  assert_clear_table(f'normal-{width}')
  lens_x=round(rect['right']-60);lens_y=round(rect['y']+60)
  browser('mouse','move',str(lens_x),str(lens_y));browser('mouse','down')
  browser('wait','--fn','!!document.querySelector(".snooker-charge")')
  assert_clear_table(f'former-lens-hit-{width}')
  browser('press','Escape');browser('mouse','up')
  assert api('/state')['room']['version']==initial,'cancelled former-lens hit shot'
  # Capture continues outside the table; escape, cancellation and zoom never fire.
  browser('mouse','move',str(x),str(y));browser('mouse','down');browser('press','Escape');browser('mouse','up')
  assert api('/state')['room']['version']==initial
  browser('mouse','down');browser('mouse','move',str(x),str(round(rect['y']-10)))
  assert js('!!document.querySelector(".snooker-charge")'),'leaving table cancelled the held shot'
  outside_angle=float(js('document.querySelector(".snooker-table canvas").dataset.angle'))
  browser('mouse','move',str(x-30),str(round(rect['y']-20)))
  continued_angle=float(js('document.querySelector(".snooker-table canvas").dataset.angle'))
  assert abs(continued_angle-outside_angle)>.001,'aim froze outside the table'
  browser('press','Escape');browser('mouse','up')
  assert api('/state')['room']['version']==initial
  browser('mouse','move',str(x),str(y));browser('mouse','down');browser('wait','--fn','Number(document.querySelector(".snooker-charge").getAttribute("aria-valuenow"))>10')
  before_angle=float(js('document.querySelector(".snooker-table canvas").dataset.angle'));browser('mouse','move',str(x+15),str(y+12));after_angle=float(js('document.querySelector(".snooker-table canvas").dataset.angle'));assert abs(after_angle-before_angle)>.001,'held drag did not change direction';assert_clear_table(f'charging-{width}');shot(f'charging-{width}')
  assert api('/state')['room']['version']==initial,'shot while still holding'
  endpoint=browser('get','cdp-url')['cdpUrl']
  subprocess.run(['node','scripts/browser_wheel.mjs',endpoint,base,str(x),str(y),'-220'],cwd=root,env=env,check=True,timeout=15)
  browser('mouse','up');assert_clear_table(f'zoom-{width}');assert api('/state')['room']['version']==initial,'zoom released a shot'
  browser('find','role','button','click','--name','移动视图','--exact')
  browser('mouse','move',str(x),str(y));browser('mouse','down');browser('mouse','move',str(x+30),str(y+30));browser('mouse','up')
  assert_clear_table(f'pan-{width}');assert api('/state')['room']['version']==initial,'pan released a shot'
  browser('find','role','button','click','--name','移动视图','--exact');browser('find','role','button','click','--name','复位视图','--exact')
  browser('scrollintoview','.snooker-table canvas');rect=js('document.querySelector(".snooker-table canvas").getBoundingClientRect().toJSON()');x=round(rect['x']+rect['width']*.52);y=round(rect['y']+rect['height']*.6)
  subprocess.run(['node','scripts/snooker/native-touch.mjs',endpoint,base,'pinch',str(x),str(y),str(x),str(y)],cwd=root,env=env,check=True,timeout=20)
  assert_clear_table(f'pinch-{width}');assert api('/state')['room']['version']==initial,'pinch released a shot'
  browser('find','role','button','click','--name','复位视图','--exact')
  subprocess.run(['node','scripts/snooker/native-touch.mjs',endpoint,base,'cancel',str(x),str(y),str(x+10),str(y-10)],cwd=root,env=env,check=True,timeout=20)
  assert api('/state')['room']['version']==initial,'cancelled touch released a shot'
  # One real held mouse / touch release creates exactly one authoritative shot.
  if width==1280:
   browser('mouse','move',str(x),str(y));browser('mouse','down');browser('wait','--fn','Number(document.querySelector(".snooker-charge").getAttribute("aria-valuenow"))>20');browser('mouse','move',str(round(rect['x']-8)),str(y-12));browser('mouse','up')
  else:
   subprocess.run(['node','scripts/snooker/native-touch.mjs',endpoint,base,'charge',str(x),str(y),str(round(rect['x']-8)),str(y-12)],cwd=root,env=env,check=True,timeout=20)
  browser('wait','--fn','document.querySelector(".snooker-board").dataset.phase==="rolling"');after=api('/state')['room']['game']['data'];assert after['flight']['id']==1
  assert .02<=after['flight']['strike']['power']<=1
  report.setdefault('shots',[]).append({'width':width,'strike':after['flight']['strike']})
  shot(f'released-{width}')
  browser('wait','--fn','document.querySelector(".snooker-board").dataset.phase!=="rolling"')
  command('close');command('leave')
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
  (fixture/'stop').write_text('stop')
  try:server.wait(timeout=15)
  except subprocess.TimeoutExpired:server.terminate();server.wait(timeout=10)
 if log:log.close()
 (out/'result.json').write_text(json.dumps(report,ensure_ascii=False,indent=2)+'\n');print('Artifact:',out/'result.json',flush=True)
 if not report['passed']:raise SystemExit(1)
