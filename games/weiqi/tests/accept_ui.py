#!/usr/bin/env python3
"""Before Board implementation: real native input, scoring, full mobile board,
zoom/hit sizes, muted sound, stale pending points, reload and asset fallback.
Results/screenshots persist under .local/weiqi/browser. No synthetic DOM clicks.
"""
import http.cookiejar
import json
import os
from pathlib import Path
import subprocess
import time
import urllib.request
import uuid
root=Path(__file__).resolve().parents[3];out=root/'.local/weiqi/browser';out.mkdir(parents=True,exist_ok=True)
env=dict(os.environ)
for key,rel in {'GOCACHE':'.local/go-build','GOMODCACHE':'.local/go-mod','GOTMPDIR':'.local/tmp','TMPDIR':'.local/tmp','npm_config_cache':'.local/npm','AGENT_BROWSER_SOCKET_DIR':'.local/ab-weiqi','AGENT_BROWSER_PROFILE':'.local/weiqi/browser/profile'}.items():
 p=root/rel;p.mkdir(parents=True,exist_ok=True);env[key]=str(p)
env.update(GOTOOLCHAIN='local',WIRE_BOARD_WEIQI_BROWSER='1',AGENT_BROWSER_SESSION='weiqi-'+uuid.uuid4().hex[:8],AGENT_BROWSER_ARGS='--mute-audio,--disable-breakpad,--disable-crash-reporter')
n=int(env.get('WIRE_BOARD_WEIQI_SIZE','19'));scenario=env.get('WIRE_BOARD_WEIQI_SCENARIO','natural');prefix=str(n) if scenario=='natural' else f'{n}-{scenario}';report={'passed':False,'size':n,'checks':[]};server=None;opened=False
peer=urllib.request.build_opener(urllib.request.HTTPCookieProcessor(http.cookiejar.CookieJar()))
def b(*args):
 r=subprocess.run(['agent-browser',*args,'--json'],cwd=root,env=env,capture_output=True,text=True,timeout=40)
 if r.returncode:raise RuntimeError(r.stdout+r.stderr)
 v=json.loads(r.stdout);assert v['success'],v;return v['data']
def js(code):return b('eval',code)['result']
def api(path,body=None):
 opts='' if body is None else ',{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify('+json.dumps(body)+')}'
 r=js('fetch('+json.dumps('/api'+path)+opts+').then(async r=>({status:r.status,body:await r.json()}))');assert 200<=r['status']<300,r;return r['body']
def peer_api(path,body=None):
 with peer.open(urllib.request.Request(base+'/api'+path,data=None if body is None else json.dumps(body).encode(),headers={'Content-Type':'application/json'}),timeout=15) as r:return json.load(r)
def room():return api('/state')['room']
def view():return room()['game']['data']
def command(kind,other=False,**extra):
 r=room();return (peer_api if other else api)('/rooms/'+r['id'],dict(type=kind,version=r['version'],nonce=uuid.uuid4().hex,**extra))
def act(kind,seat=None,**extra):
 d=view();seat=d['turn'] if seat is None else seat;return command('action',other=seat==1,action=dict(type=kind,round=d['round'],ply=d['ply'],revision=d['revision'],**extra))
def click(name):b('find','role','button','click','--name',name,'--exact')
def settled(ply):b('wait','--fn',f'document.querySelector(".weiqi-board")?.dataset.ply==="{ply}"&&!document.querySelector(".weiqi-board").dataset.moving')
def cell(pos):
 sel=f'[data-weiqi-cell="{pos}"]';b('scrollintoview',sel);r=js('document.querySelector('+json.dumps(sel)+').getBoundingClientRect().toJSON()');b('mouse','move',str(round(r['x']+r['width']/2)),str(round(r['y']+r['height']/2)));b('mouse','down');q=js('document.querySelector('+json.dumps(sel)+').getBoundingClientRect().toJSON()');assert abs(r['x']-q['x'])<.1 and abs(r['y']-q['y'])<.1;b('mouse','up')
def shot(name):
 assert not b('errors')['errors'];assert js('document.documentElement.scrollWidth<=innerWidth+2'),'horizontal overflow'
 b('screenshot',str(out/f'{prefix}-{name}.png'),'--full');report['checks'].append(name);print('PASS',n,name,flush=True)
 (out/f'{prefix}-{name}.state.json').write_text(json.dumps(api('/state'),ensure_ascii=False,indent=2))
 contrast=js(r"""(() => {
   const rgb=s=>(s.match(/[\d.]+/g)||[]).map(Number),blend=(a,b,t)=>a.slice(0,3).map((v,i)=>v*t+b[i]*(1-t));
   const lum=c=>c.map(v=>v/255).map(v=>v<=.04045?v/12.92:((v+.055)/1.055)**2.4).reduce((s,v,i)=>s+v*[.2126,.7152,.0722][i],0);
   return [...document.querySelectorAll('.weiqi-heading p,.weiqi-heading strong,.weiqi-actions p,.weiqi-actions h3,.weiqi-actions button,.weiqi-map-tools button,.weiqi-player-summary small')].map(e=>{
     let chain=[],bg=[255,255,255];for(let n=e;n;n=n.parentElement)chain.unshift(n);for(const n of chain){const c=rgb(getComputedStyle(n).backgroundColor);bg=blend(c,bg,c[3]??1)}
     const a=lum(rgb(getComputedStyle(e).color)),b=lum(bg);return {text:e.textContent.trim(),ratio:(Math.max(a,b)+.05)/(Math.min(a,b)+.05)};
   });
 })()""")
 assert all(c['ratio']>=4.5 for c in contrast),contrast
 (out/f'{prefix}-{name}.contrast.json').write_text(json.dumps(contrast,ensure_ascii=False,indent=2))

try:
 with (out/'build.log').open('w') as f:subprocess.run(['npm','--prefix','web','run','build'],cwd=root,env=env,stdout=f,stderr=subprocess.STDOUT,check=True)
 for name in ['url','stop']:(out/name).unlink(missing_ok=True)
 with (out/'server.log').open('w') as f:server=subprocess.Popen(['go','test','-count=1','-timeout=13m','-run','^TestWeiqiBrowserFixture$','./games/weiqi/tests'],cwd=root,env=env,stdout=f,stderr=subprocess.STDOUT)
 until=time.monotonic()+90
 while not (out/'url').exists():
  if server.poll() is not None or time.monotonic()>until:raise RuntimeError('See server.log')
  time.sleep(.1)
 base=(out/'url').read_text();b('open',base);opened=True;b('tab',next(t['tabId'] for t in b('tab')['tabs'] if t['active']))
 api('/login',{'name':'棋类验收','password':'weiqi-browser-only'});assert 'weiqi' not in api('/state')['availableGames'];api('/admin/games/weiqi',{'enabled':True});b('reload');b('wait','.lobby-create');b('click','.lobby-create');b('wait','[role=dialog]');b('fill','input[aria-label=搜索可选游戏]','围棋');b('click','.choose-games button');b('fill','input[name=name]','围棋原生输入验收');shot('create-cover');b('scrollintoview','[role=dialog] button[type=submit]');b('click','[role=dialog] button[type=submit]');b('wait','.waiting-actions')
 b('select','select[aria-label=围棋路数]',str(n))
 if room()['options']['size']!=n:
  click('保存设置');b('wait','--fn','fetch("/api/state").then(r=>r.json()).then(s=>s.room.options.size==='+str(n)+')')
 assert room()['options']['size']==n,'size option was not saved'
 peer_api('/register',{'name':'围棋对手','password':'weiqi-browser-only','invite':'weiqi-browser'});command('join',True);command('ready',True);command('ready');command('start');b('wait','.weiqi-board');b('set','viewport','1440','1000');endpoint=b('get','cdp-url')['cdpUrl']
 assert view()['options']['size']==n and len(view()['board'])==n*n,'wrong actual board size'
 if scenario=='capture':
  assert n==9 and view()['black']==0
  js('window.__frames=[];window.__sample=true;const sample=()=>{if(!window.__sample)return;const b=document.querySelector(".weiqi-board"),e=document.querySelector(".weiqi-stone.arriving"),r=e?.getBoundingClientRect();window.__frames.push({moving:b?.dataset.moving,removed:document.querySelectorAll(".weiqi-stone.removing").length,width:r?.width,opacity:e?getComputedStyle(e).opacity:null,summary:document.querySelector(".weiqi-player-summary")?.textContent});requestAnimationFrame(sample)};sample()')
  report['reducedMotion']=js('matchMedia("(prefers-reduced-motion: reduce)").matches')
  cell(10);b('click','[data-weiqi-confirm]');b('wait','--fn','document.querySelector(".weiqi-board").dataset.moving==="1"')
  r=peer_api('/state')['room'];d=r['game']['data'];peer_api('/rooms/'+r['id'],dict(type='action',version=r['version'],nonce=uuid.uuid4().hex,action=dict(type='place',round=d['round'],ply=d['ply'],revision=d['revision'],pos=d['legal'][0])))
  settled(2);frames=js('window.__sample=false;window.__frames');report['captureFrames']=frames
  first=[f for f in frames if f['moving']=='1'];second=[f for f in frames if f['moving']=='2']
  assert len(first)>2 and all(f['removed']==4 and '已提 0 子' in f['summary'] for f in first),first
  assert len(second)>1 and max(f['width'] for f in first)-min(f['width'] for f in first)>2,'missing actual consecutive landing animation'
  assert view()['captured'][0]==4 and js('document.querySelectorAll(".weiqi-stone.removing").length===0');shot('capture-after-landing');report['passed']=True;raise SystemExit(0)
 if view()['turn']==1:act('place',pos=0);settled(1)
 ply=view()['ply'];center=n*n//2;cell(center);b('press','ArrowRight');assert js('document.activeElement?.dataset.weiqiCell==='+json.dumps(str(center+1)));b('press','Enter');assert view()['ply']==ply;b('click','[data-weiqi-confirm]');settled(ply+1);assert view()['board'][center+1] !=0
 act('place',pos=n-1);settled(ply+2);shot('desktop-keyboard')
 saved=view();b('reload');b('wait','.weiqi-board');assert view()==saved and js('!document.querySelector(".weiqi-board").dataset.moving');shot('restore')
 b('scrollintoview','.weiqi-viewport');r=js('document.querySelector(".weiqi-viewport").getBoundingClientRect().toJSON()');x=r['x']+r['width']/2;y=r['y']+r['height']/2;before=view()['ply']
 subprocess.run(['node','scripts/browser_wheel.mjs',endpoint,base,str(x),str(y),'-230'],cwd=root,env=env,check=True,timeout=20);b('wait','--fn','Number(document.querySelector(".weiqi-board").dataset.zoom)>1');b('mouse','move',str(round(x)),str(round(y)));b('mouse','down');b('mouse','move',str(round(x-60)),str(round(y-40)));b('mouse','up');assert view()['ply']==before and js('document.querySelector(".weiqi-board").dataset.selected===""');shot('mouse-wheel-pan');click('全图')
 if env.get('WIRE_BOARD_WEIQI_ASSETS'):
  b('network','route',env['WIRE_BOARD_WEIQI_ASSETS'].rstrip('/')+'/weiqi/v1/**','--abort');subprocess.run(['node','scripts/browser_reload_uncached.mjs',endpoint,base],cwd=root,env=env,check=True,timeout=25);b('wait','.weiqi-board');b('wait','--fn','[...document.querySelectorAll(".weiqi-stone img")].every(e=>e.complete&&e.naturalWidth>0&&new URL(e.currentSrc).origin===location.origin)');shot('cdn-fallback')
 js('window.__weiqiLocal=[];const original=HTMLMediaElement.prototype.play;HTMLMediaElement.prototype.play=function(){const src=this.src;return original.call(this).then(()=>{if(src.startsWith(location.origin+"/weiqi/v1/audio/"))window.__weiqiLocal.push(src)})}')
 before=view()['ply'];cell(center+n);b('click','[data-weiqi-confirm]');settled(before+1);act('place',pos=n*n-1);settled(before+2);b('wait','--fn','window.__weiqiLocal.length>0');report['localAudio']=js('window.__weiqiLocal')

 for width in [390,320]:
  b('set','viewport',str(width),'844' if width==390 else '740');click('全图');b('wait','--fn','document.querySelector(".weiqi-map").getBoundingClientRect().width>0');r=js('(()=>{const a=document.querySelector(".weiqi-map").getBoundingClientRect(),b=document.querySelector(".weiqi-viewport").getBoundingClientRect();return {fit:a.left>=b.left-1&&a.right<=b.right+1&&a.top>=b.top-1&&a.bottom<=b.bottom+1}})()');assert r['fit'];shot(f'mobile-full-{width}')
  if js('document.querySelector(".weiqi-board").dataset.overview==="true"'):cell(center);b('wait','--fn','document.querySelector(".weiqi-board").dataset.overview==="false"')
  assert js('[...document.querySelectorAll(".weiqi-stone")].every(e=>e.getBoundingClientRect().width>=24)');shot(f'mobile-detail-{width}')
 b('scrollintoview','.weiqi-viewport');r=js('document.querySelector(".weiqi-viewport").getBoundingClientRect().toJSON()');x=r['x']+r['width']/2;y=r['y']+r['height']/2;before=view()['ply']
 pan_before=js('({x:document.querySelector(".weiqi-viewport").scrollLeft,y:document.querySelector(".weiqi-viewport").scrollTop})')
 subprocess.run(['node','scripts/browser_touch.mjs',endpoint,base,'drag',str(x),str(y),str(x-60),str(y-50)],cwd=root,env=env,check=True,timeout=20);assert view()['ply']==before and js('document.querySelector(".weiqi-board").dataset.selected===""');pan_after=js('({x:document.querySelector(".weiqi-viewport").scrollLeft,y:document.querySelector(".weiqi-viewport").scrollTop})');assert abs(pan_after['x']-pan_before['x'])+abs(pan_after['y']-pan_before['y'])>5,'native touch did not pan';shot('native-touch-drag')
 click('全图');b('scrollintoview','.weiqi-viewport');r=js('document.querySelector(".weiqi-viewport").getBoundingClientRect().toJSON()');x=r['x']+r['width']/2;y=r['y']+r['height']/2
 subprocess.run(['node','scripts/browser_touch.mjs',endpoint,base,'pinch',str(x),str(y),str(x),str(y)],cwd=root,env=env,check=True,timeout=20);b('wait','--fn','Number(document.querySelector(".weiqi-board").dataset.zoom)>1');assert view()['ply']==before
 cell(center);b('wait','--fn','document.querySelector(".weiqi-board").dataset.overview==="false"')
 # Native touch selects a point but only the confirm button commits it.
 p=center-1;sel=f'[data-weiqi-cell="{p}"]';b('scrollintoview',sel);r=js('document.querySelector('+json.dumps(sel)+').getBoundingClientRect().toJSON()');x=r['x']+r['width']/2;y=r['y']+r['height']/2
 subprocess.run(['node','scripts/browser_touch.mjs',endpoint,base,'tap',str(x),str(y),str(x),str(y)],cwd=root,env=env,check=True,timeout=20);assert view()['ply']==before and js('document.querySelector(".weiqi-board").dataset.selected==='+json.dumps(str(p)))
 b('set','media','light','reduced-motion');click('静音');b('click','[data-weiqi-confirm]');settled(before+1);act('place',pos=n*n-2);settled(before+2);before=view()['ply'];shot('native-touch-muted')
 click('停一手');b('wait','--fn','document.querySelector(".weiqi-board").dataset.ply==='+json.dumps(str(before+1)));act('pass');b('wait','.weiqi-scoring');assert view()['phase']=='scoring';shot('score-proposal')
 # Clicking a group proposes dead stones only; it does not remove them yet.
 cell(center+1);b('wait','--fn','document.querySelector(".weiqi-board").dataset.selected!==""');click('切换整块死子标记');b('wait','.weiqi-stone.dead');assert view()['board'][center+1]!=0;shot('marked-group')
 click('继续对弈');b('wait','--fn','document.querySelector(".weiqi-board").dataset.phase==="playing"');assert view()['board'][center+1]!=0 and not view()['dead'];act('pass');act('pass');b('wait','.weiqi-scoring');click('确认计分');act('confirm_score',seat=1);b('wait','.results');assert view()['phase']=='finished' and js('document.querySelector("audio[data-platform-audio]").paused');shot('confirmed-score');report['passed']=True
except Exception as e:
 import traceback;traceback.print_exc();report['error']=repr(e)
 if opened:
  try:report['state']=api('/state');report['snapshot']=b('snapshot','-i');b('screenshot',str(out/f'{prefix}-failure.png'),'--full')
  except Exception:pass
finally:
 if opened:
  try:b('close')
  except Exception:pass
 if server:
  (out/'stop').write_text('stop')
  try:server.wait(timeout=15)
  except subprocess.TimeoutExpired:server.terminate();server.wait(timeout=10)
 (out/f'result-{prefix}.json').write_text(json.dumps(report,ensure_ascii=False,indent=2)+'\n')
 if not report['passed']:raise SystemExit(1)
