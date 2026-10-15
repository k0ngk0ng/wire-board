#!/usr/bin/env python3
"""Rapid input failures declared before changes: adjacent taps lost; busy disables
next pair; acknowledgement clears a new selection; stale queued revisions;
compatibility clicks double-select; scrolling/cancellation selects a tile.
See games/pairs/README.md for the full failure inventory.
"""
import json,os,subprocess,time,uuid,urllib.request,http.cookiejar,shutil
from pathlib import Path
root=Path(__file__).resolve().parents[2];out=root/'.local/pairs/rapid';out.mkdir(parents=True,exist_ok=True)
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
 fixture=root/'.local/pairs/browser';fixture.mkdir(parents=True,exist_ok=True)
 for name in ['url','stop']:(fixture/name).unlink(missing_ok=True)
 with (out/'server.log').open('w') as f:server=subprocess.Popen(['go','test','-count=1','-timeout=13m','-run','^TestPairsBrowserFixture$','./games/pairs/tests'],cwd=root,env=env,stdout=f,stderr=subprocess.STDOUT)
 until=time.monotonic()+90
 while not (fixture/'url').exists():
  if server.poll() is not None or time.monotonic()>until:raise RuntimeError('browser fixture did not start')
  time.sleep(.1)
 base=(fixture/'url').read_text();b('open',base);opened=True
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
 # Latency changes transport only. Inputs remain native and boards authoritative.
 endpoint=b('get','cdp-url')['cdpUrl']
 subprocess.run(['node','scripts/snooker/throttle.mjs',endpoint,base,'180','1'],cwd=root,env=env,check=True,timeout=20)
 js('window.__inputs=[];window.__requests=[];for(const kind of ["pointerup","click"])document.addEventListener(kind,e=>{const t=e.target.closest?.(".pairs-cell");window.__inputs.push({kind,index:t?.dataset.index,disabled:t?.disabled,at:performance.now()})},true);const f=window.fetch;window.fetch=(url,options)=>{if(options?.body){const b=JSON.parse(options.body);if(b.type==="action")window.__requests.push({action:b.action,at:performance.now()})}return f(url,options)}')
 for width,mode in [(1280,'mouse'),(320,'touch')]:
  b('set','viewport',str(width),'1100')
  js('document.querySelector(".pairs-field").scrollIntoView({block:"center",behavior:"instant"})')
  def plan(count):
   board=api('/state')['room']['game']['data']['players'][0]['board']
   code='(()=>{'+solver+';const b='+json.dumps(board)+r''';const rects=[...document.querySelectorAll('.pairs-cell')].map(e=>({i:+e.dataset.index,r:e.getBoundingClientRect()})).filter(e=>e.r.top>=0&&e.r.bottom<=innerHeight);const candidates=[];for(const a of rects)for(const z of rects){if(a.i<z.i&&findPath(b,a.i,z.i))candidates.push([a.i,z.i])}candidates.sort((a,z)=>Math.abs(a[0]-a[1])-Math.abs(z[0]-z[1]));const used=new Set(),pairs=[];for(const p of candidates){if(p.some(i=>used.has(i)))continue;pairs.push(p);p.forEach(i=>used.add(i))}return pairs.slice(0,COUNT)})()'''
   return js(code.replace('COUNT',str(count)))
  pairs=plan(3);assert len(pairs)>=2,pairs
  r=api('/state')['room'];revision=r['game']['data']['players'][0]['revision'];remaining=r['game']['data']['players'][0]['remaining']
  indices=[i for p in pairs[:2] for i in p]+([pairs[2][0]] if len(pairs)>2 else [])
  points=js(json.dumps(indices)+'.map(i=>{const e=[...document.querySelectorAll(".pairs-cell")].find(e=>+e.dataset.index===i),r=e.getBoundingClientRect();return {x:r.x+r.width/2,y:r.y+r.height/2}})')
  path=out/(mode+'-points.json');path.write_text(json.dumps(points))
  subprocess.run(['node','scripts/pairs/rapid-input.mjs',endpoint,base,mode,str(path),'35'],cwd=root,env=env,check=True,timeout=20)
  try:b('wait','--fn',f'document.querySelector(".pairs-board").dataset.revision==="{revision+2}"')
  finally:
   report.setdefault('bursts',[]).append({'width':width,'mode':mode,'pairs':pairs,'beforeRevision':revision,'state':api('/state')['room']['game']['data']['players'][0],'events':js('window.__inputs'),'requests':js('window.__requests')})
  assert api('/state')['room']['game']['data']['players'][0]['remaining']==remaining-4
  if len(pairs)>2:
   assert js('document.querySelector('+json.dumps(f'.pairs-cell[data-index="{pairs[2][0]}"]')+').getAttribute("aria-pressed")==="true"'),'acknowledgement lost the next selection'
   b('click',f'.pairs-cell[data-index="{pairs[2][1]}"]');b('wait','--fn',f'document.querySelector(".pairs-board").dataset.revision==="{revision+3}"')
  shot('rapid-'+mode)
  b('wait','--fn','!document.querySelector(".pairs-toolbar button").disabled')
  before=api('/state')['room']['game']['data']['players'][0]['revision']
  index=plan(1)[0][0]
  b('focus',f'.pairs-cell[data-index="{index}"]');b('press','Enter')
  assert js('document.querySelector('+json.dumps(f'.pairs-cell[data-index="{index}"]')+').getAttribute("aria-pressed")==="true"')
  b('press','Escape')
  assert not js('!!document.querySelector(".pairs-cell.selected")')
  for gesture_mode in ['cancel','touch']:
   js('document.querySelector(".pairs-field").scrollIntoView({block:"center",behavior:"instant"})')
   point=js('(()=>{const e=[...document.querySelectorAll(".pairs-cell")].find(e=>+e.dataset.index==='+str(index)+'),r=e.getBoundingClientRect();return {x:r.x+r.width/2,y:r.y+r.height/2}})()')
   if gesture_mode=='touch':point['to']={'x':point['x']+35,'y':point['y']+25}
   path.write_text(json.dumps([point]));subprocess.run(['node','scripts/pairs/rapid-input.mjs',endpoint,base,gesture_mode,str(path),'35'],cwd=root,env=env,check=True,timeout=20)
   assert not js('!!document.querySelector(".pairs-cell.selected")'),'cancel/drag selected a tile'
   assert api('/state')['room']['game']['data']['players'][0]['revision']==before
  shot('cancel-and-keyboard-'+mode)
 # Reject one queued pair through the real server; retain the subsequent pair.
 js('document.querySelector(".pairs-field").scrollIntoView({block:"center",behavior:"instant"})')
 pairs=plan(2);assert len(pairs)==2
 before=api('/state')['room']['game']['data']['players'][0]['revision']
 js('(()=>{const previous=window.fetch;let rejectOnce=true;window.fetch=(url,options)=>{if(rejectOnce&&options?.body){const p=JSON.parse(options.body);if(p.type==="action"&&p.action?.type==="match"){p.action.revision=-1;options={...options,body:JSON.stringify(p)};rejectOnce=false}}return previous(url,options)}})()')
 points=js(json.dumps([i for p in pairs for i in p])+'.map(i=>{const e=[...document.querySelectorAll(".pairs-cell")].find(e=>+e.dataset.index===i),r=e.getBoundingClientRect();return {x:r.x+r.width/2,y:r.y+r.height/2}})')
 path=out/'rejected-points.json';path.write_text(json.dumps(points));subprocess.run(['node','scripts/pairs/rapid-input.mjs',endpoint,base,'touch',str(path),'35'],cwd=root,env=env,check=True,timeout=20)
 b('wait','--fn',f'document.querySelector(".pairs-board").dataset.revision==="{before+1}"')
 player=api('/state')['room']['game']['data']['players'][0]
 assert all(player['board']['cells'][i] is not None for i in pairs[0])
 assert all(player['board']['cells'][i] is None for i in pairs[1])
 report['rejection']={'rejected':pairs[0],'accepted':pairs[1],'revision':player['revision']}
 shot('rejected-pair-preserves-next-input')
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
  (fixture/'stop').write_text('stop')
  try:server.wait(timeout=15)
  except subprocess.TimeoutExpired:server.terminate();server.wait(timeout=10)
 report['assetsBase']=env.get('WIRE_BOARD_PAIRS_ASSETS','local')
 report['command']='. scripts/env.sh && python3 -B scripts/pairs/check-rapid.py'
 (out/'result.json').write_text(json.dumps(report,ensure_ascii=False,indent=2)+'\n')
 archive=out;archive.mkdir(parents=True,exist_ok=True)
 if archive!=out:
  shutil.copyfile(out/'result.json',archive/'result.json')
  for check in report['checks']:shutil.copyfile(out/(check+'.png'),archive/(check+'.png'))
 if not report['passed']:raise SystemExit(1)
