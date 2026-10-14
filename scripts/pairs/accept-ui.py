#!/usr/bin/env python3
"""Before UI implementation: validate real clicks, original assets/effects,
source-engine legal pairs, complete race, PC/mobile and read-only spectators.
See games/pairs/README.md for the full failure inventory.
"""
import json,os,subprocess,time,uuid,urllib.request,http.cookiejar,shutil
from pathlib import Path
root=Path(__file__).resolve().parents[2];out=root/'.local/pairs/browser';out.mkdir(parents=True,exist_ok=True)
env=dict(os.environ)
for key,rel in {'AGENT_BROWSER_SOCKET_DIR':'.local/ab-pairs','AGENT_BROWSER_PROFILE':'.local/pairs/browser/profile','TMPDIR':'.local/tmp'}.items():
 p=root/rel;p.mkdir(parents=True,exist_ok=True);env[key]=str(p)
env.update(AGENT_BROWSER_SESSION='pairs-'+uuid.uuid4().hex[:7],AGENT_BROWSER_ARGS='--mute-audio,--disable-breakpad,--disable-crash-reporter',WIRE_BOARD_PAIRS_BROWSER='1')
report={'passed':False,'checks':[]};server=None;opened=False

def b(*args):
 r=subprocess.run(['agent-browser',*args,'--json'],cwd=root,env=env,capture_output=True,text=True,timeout=40)
 if r.returncode:raise RuntimeError(r.stdout+r.stderr)
 v=json.loads(r.stdout);assert v['success'],v;return v['data']
def js(s):return b('eval',s)['result']
def api(path,body=None):
 opt='' if body is None else ',{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify('+json.dumps(body)+')}'
 r=js('fetch('+json.dumps('/api'+path)+opt+').then(async r=>({status:r.status,body:await r.json()}))');assert 200<=r['status']<300,r;return r['body']
def command(kind,**extra):
 r=api('/state')['room'];return api('/rooms/'+r['id'],{'type':kind,'version':r['version'],'nonce':uuid.uuid4().hex,**extra})
def shot(name):
 assert js('document.documentElement.scrollWidth<=innerWidth+2'),name
 assert not b('errors')['errors'],b('errors')
 b('screenshot',str(out/(name+'.png')),'--full');report['checks'].append(name);print('PASS',name,flush=True)
try:
 with (out/'build.log').open('w') as f:subprocess.run(['npm','--prefix','web','run','build'],cwd=root,env=env,stdout=f,stderr=subprocess.STDOUT,check=True)
 for name in ['url','stop']:(out/name).unlink(missing_ok=True)
 with (out/'server.log').open('w') as f:server=subprocess.Popen(['go','test','-count=1','-timeout=13m','-run','^TestPairsBrowserFixture$','./games/pairs/tests'],cwd=root,env=env,stdout=f,stderr=subprocess.STDOUT)
 until=time.monotonic()+90
 while not (out/'url').exists():
  if server.poll() is not None or time.monotonic()>until:raise RuntimeError('browser fixture did not start')
  time.sleep(.1)
 base=(out/'url').read_text();b('open',base);opened=True
 if env.get('WIRE_BOARD_PAIRS_REDUCED')=='1':b('set','media','light','reduced-motion')
 api('/login',{'name':'连连看验收','password':'pairs-browser-only'})
 assert 'pairs' not in api('/state')['availableGames']
 api('/admin/games/pairs',{'enabled':True});b('reload');b('wait','.room-list')
 b('select','select[aria-label="按游戏筛选牌桌"]','pairs');b('click','.lobby-create');b('wait','[role=dialog]')
 assert js('document.querySelector(".choose-games .selected").textContent.includes("连连看")')
 b('select','select[aria-label="座位上限"]','2');shot('create-desktop')
 b('scrollintoview','[role=dialog] button[type=submit]');b('click','[role=dialog] button[type=submit]');b('wait','.waiting-actions')
 peer_client=urllib.request.build_opener(urllib.request.HTTPCookieProcessor(http.cookiejar.CookieJar()))
 def peer(path,body=None):
  payload=None if body is None else json.dumps(body).encode()
  with peer_client.open(urllib.request.Request(base+'/api'+path,data=payload,headers={'Content-Type':'application/json'})) as r:return json.load(r)
 peer('/register',{'name':'竞速好友','password':'pairs-browser-only','invite':'pairs-browser'})
 r=api('/state')['room'];peer('/rooms/'+r['id'],{'type':'join','version':r['version'],'nonce':uuid.uuid4().hex})
 r=peer('/state')['room'];peer('/rooms/'+r['id'],{'type':'ready','version':r['version'],'nonce':uuid.uuid4().hex})
 command('ready');shot('waiting');command('start');b('wait','.pairs-board')
 b('wait','.pairs-cell');b('wait','--fn','[...document.querySelectorAll(".pairs-cell img")].every(e=>e.complete&&e.naturalWidth>0)')
 # Text contrast measured from rendered styles, not source CSS declarations.
 colors=js(r'''(()=>{const rgb=s=>(s.match(/[\d.]+/g)||[]).map(Number),lum=c=>c.slice(0,3).map(x=>x/255).map(x=>x<=.04045?x/12.92:((x+.055)/1.055)**2.4).reduce((s,x,i)=>s+x*[.2126,.7152,.0722][i],0);return [...document.querySelectorAll('.pairs-heading h2,.pairs-heading p,.pairs-clock,.pairs-toolbar button,.pairs-notice,.pairs-help')].map(e=>{let n=e,b;do{b=rgb(getComputedStyle(n).backgroundColor);n=n.parentElement}while(n&&b[3]===0);const f=lum(rgb(getComputedStyle(e).color)),a=lum(b);return {text:e.textContent,ratio:(Math.max(a,f)+.05)/(Math.min(a,f)+.05)}})})()''')
 assert all(c['ratio']>=4.5 for c in colors),colors
 report['contrast']=colors
 # Independent original TS solver, used to choose real browser inputs only.
 solver=subprocess.check_output(['node','--input-type=module','-e',"import ts from './web/node_modules/typescript/lib/typescript.js';import fs from 'node:fs';process.stdout.write(ts.transpile(fs.readFileSync('games/pairs/tests/source-engine.ts','utf8'),{target:ts.ScriptTarget.ES2022,module:ts.ModuleKind.ES2022}).replace(/export /g,''));"],cwd=root,env=env,text=True)
 js('(()=>{'+solver+';window.pairsFindMove=findMove;})()')
 for width in [1280,320]:
  b('set','viewport',str(width),'900');b('wait','.pairs-cell')
  dimensions=js('[...document.querySelectorAll(".pairs-cell")].filter(e=>e.querySelector("img")).map(e=>({w:e.getBoundingClientRect().width,h:e.getBoundingClientRect().height}))')
  assert all(d['w']>=24 and d['h']>=24 for d in dimensions),dimensions
  shot('board-'+str(width))
 if env.get('WIRE_BOARD_PAIRS_CASE')=='fallback':assert js('[...document.querySelectorAll(".pairs-cell img")].every(e=>e.dataset.fallback==="true" && new URL(e.currentSrc).origin===location.origin)')
 # All media must use the migrated namespace and native platform channels.
 assert js('[...document.querySelectorAll(".pairs-cell img")].every(e=>e.currentSrc.includes("/pairs/v1/icons/"))')
 r=api('/state')['room'];v=r['game']['data'];pair=js('window.pairsFindMove('+json.dumps(v['players'][0]['board'])+')')
 b('scrollintoview',f'.pairs-cell[data-index="{pair[0]}"]')
 endpoint=b('get','cdp-url')['cdpUrl']
 for index in pair:
  selector=f'.pairs-cell[data-index="{index}"]';b('scrollintoview',selector)
  rect=js('document.querySelector('+json.dumps(selector)+').getBoundingClientRect().toJSON()');x=rect['x']+rect['width']/2;y=rect['y']+rect['height']/2
  subprocess.run(['node','scripts/snooker/native-touch.mjs',endpoint,base,'tap',str(x),str(y),str(x),str(y)],cwd=root,env=env,check=True,timeout=20)
 b('wait','--fn','document.querySelector(".pairs-board").dataset.revision!=="0"');shot('native-touch-mobile')
 b('wait','--fn','[...document.querySelectorAll("audio[data-platform-music]")].some(e=>!e.paused && e.loop)')
 b('find','role','button','click','--name','静音','--exact')
 assert js('[...document.querySelectorAll("audio[data-platform-audio]")].every(e=>e.paused && e.muted)')
 b('find','role','button','click','--name','取消静音','--exact')
 assert js('[...document.querySelectorAll("audio[data-platform-audio]")].every(e=>e.paused)')
 r=api('/state')['room'];v=r['game']['data'];pair=js('window.pairsFindMove('+json.dumps(v['players'][0]['board'])+')')
 b('focus',f'.pairs-cell[data-index="{pair[0]}"]');b('press','Enter')
 # Another seat changing the shared version must not clear our selection.
 pr=peer('/state')['room'];pv=pr['game']['data'];pp=js('window.pairsFindMove('+json.dumps(pv['players'][1]['board'])+')')
 peer('/rooms/'+pr['id'],{'type':'action','version':pr['version'],'nonce':uuid.uuid4().hex,'action':{'type':'match','round':pv['round'],'revision':pv['players'][1]['revision'],'a':pp[0],'b':pp[1]}})
 b('wait','--fn',f'document.querySelectorAll(".pairs-progress strong")[1].textContent.includes("{pv["players"][1]["remaining"]-2} /")')
 assert js('document.querySelector('+json.dumps(f'.pairs-cell[data-index="{pair[0]}"]')+').getAttribute("aria-pressed")==="true"')
 b('focus',f'.pairs-cell[data-index="{pair[1]}"]');b('press','Enter')
 b('wait','--fn',f'document.querySelector(".pairs-board").dataset.revision!=="{v["players"][0]["revision"]}"');shot('keyboard-concurrent-mobile')
 b('find','role','button','click','--name','提示 3','--exact');b('wait','.pairs-bolt');shot('hint-mobile')
 b('find','role','button','click','--name','重排 2','--exact')
 b('wait','--fn','document.querySelector(".pairs-board").textContent.includes("重排 1")')
 steps=0
 while api('/state')['room']['status']=='playing' and steps<100:
  r=api('/state')['room'];v=r['game']['data'];p=v['players'][r['you']];pair=js('window.pairsFindMove('+json.dumps(p['board'])+')');assert pair
  for i in pair:b('click',f'.pairs-cell[data-index="{i}"]')
  b('wait','--fn',f'document.querySelector(".pairs-board").dataset.revision!=="{p["revision"]}"')
  if steps==0:shot('match-mobile')
  steps+=1
 v=api('/state')['room']['game']['data'];assert v['phase']=='finished' and v['winner']==0 and v['players'][0]['remaining']==0,v
 shot('complete-race');report['matches']=steps
 b('reload');b('wait','.pairs-board');assert not js('!!document.querySelector(".pairs-bolt")'),'replayed old event'
 report['passed']=True
except Exception as e:
 import traceback;traceback.print_exc();report['error']=repr(e)
 if opened:
  try:report['snapshot']=b('snapshot','-i');b('screenshot',str(out/'failure.png'),'--full')
  except Exception:pass
finally:
 if opened:
  try:b('close')
  except Exception:pass
 if server:
  (out/'stop').write_text('stop')
  try:server.wait(timeout=15)
  except subprocess.TimeoutExpired:server.terminate();server.wait(timeout=10)
 report['assetsBase']=env.get('WIRE_BOARD_PAIRS_ASSETS','local')
 report['command']='. scripts/env.sh && WIRE_BOARD_PAIRS_ASSETS='+env.get('WIRE_BOARD_PAIRS_ASSETS','')+' python3 -B scripts/pairs/accept-ui.py'
 (out/'result.json').write_text(json.dumps(report,ensure_ascii=False,indent=2)+'\n')
 archive=root/'.local/pairs'/('browser-'+env.get('WIRE_BOARD_PAIRS_CASE','local'));archive.mkdir(parents=True,exist_ok=True)
 if archive!=out:
  shutil.copyfile(out/'result.json',archive/'result.json')
  for check in report['checks']:shutil.copyfile(out/(check+'.png'),archive/(check+'.png'))
 if not report['passed']:raise SystemExit(1)
