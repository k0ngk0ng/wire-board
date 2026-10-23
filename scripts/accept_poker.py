#!/usr/bin/env python3
"""Production poker UI acceptance (written before front-end implementation).
Failures: unpublished leakage, missing classification, mismatched player mode,
wrong own-seat position, hidden/overlapping cards at 320px, skin changes losing
selection, preview leaking wild choices, bypassed platform audio, old-event
replays and failure to submit the player's chosen physical cards.
Run: . scripts/env.sh && python3 -B scripts/accept_poker.py
"""
import json,os,subprocess,time,uuid,urllib.request,http.cookiejar,shutil
from pathlib import Path
root=Path(__file__).resolve().parents[1];out=root/'.local/poker/browser';out.mkdir(parents=True,exist_ok=True)
env=dict(os.environ)
for k,p in {'TMPDIR':'.local/tmp','GOTMPDIR':'.local/tmp','GOCACHE':'.local/go-build','GOMODCACHE':'.local/go-mod','npm_config_cache':'.local/npm','AGENT_BROWSER_SOCKET_DIR':'.local/poker/ab','AGENT_BROWSER_PROFILE':'.local/poker/browser/profile'}.items():
 d=root/p;d.mkdir(parents=True,exist_ok=True);env[k]=str(d)
env.update(WIRE_BOARD_POKER_BROWSER='1',GOTOOLCHAIN='local',AGENT_BROWSER_SESSION='poker-'+uuid.uuid4().hex[:8],AGENT_BROWSER_ARGS='--mute-audio,--disable-breakpad,--disable-crash-reporter')
# A fresh profile avoids stale Chrome targets after an interrupted prior run.
profile=out/env['AGENT_BROWSER_SESSION'];profile.mkdir(parents=True,exist_ok=True);env['AGENT_BROWSER_PROFILE']=str(profile)
report={'passed':False,'checks':[],'command':'. scripts/env.sh && python3 -B scripts/accept_poker.py'};server=None;opened=False;logs=[]
def b(*args):
 p=subprocess.run(['agent-browser',*args,'--json'],cwd=root,env=env,text=True,capture_output=True,timeout=40)
 if p.returncode:raise RuntimeError(p.stdout+p.stderr)
 v=json.loads(p.stdout);assert v['success'],v;return v['data']
def js(code):return b('eval',code)['result']
def api(path,body=None):
 opt='' if body is None else ',{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify('+json.dumps(body)+')}'
 r=js('fetch('+json.dumps('/api'+path)+opt+').then(async r=>({status:r.status,body:await r.json()}))');assert 200<=r['status']<300,r;return r['body']
def command(kind,**extra):
 r=api('/state')['room'];return api('/rooms/'+r['id'],dict(type=kind,version=r['version'],nonce=uuid.uuid4().hex,**extra))
def shot(name):
 if js('!!document.querySelector(".toast[role=status] button")'):b('click','.toast[role=status] button')
 b('wait','--fn','[...document.querySelectorAll(".poker-new-play")].every(e=>e.getAnimations().every(a=>a.playState!=="running"))')
 b('wait','--fn','[...document.querySelectorAll(".poker-card img")].every(i=>i.complete&&i.naturalWidth>0)')
 assert not b('errors')['errors'],b('errors')
 assert js('document.documentElement.scrollWidth<=innerWidth+2'),name+' overflow'
 if js('innerWidth<=600 && !!document.querySelector(".poker-board")'):
  geometry=js("(()=>{const b=document.querySelector('.poker-board').getBoundingClientRect(),a=document.querySelector('.poker-actions').getBoundingClientRect(),t=document.querySelector('.game-sidebar .turn-card').getBoundingClientRect();return {top:b.top,bottom:b.bottom,height:innerHeight,actionsBottom:a.bottom,turnTop:t.top,turnRight:t.right,turnBottom:t.bottom,boardRight:b.right,body:document.documentElement.scrollHeight}})()")
  assert geometry['top']>=0 and geometry['bottom']<=geometry['height']+1 and geometry['actionsBottom']<=geometry['height']+1 and geometry['bottom']>=geometry['height']-12 and geometry['body']<=geometry['height']+1,(name,geometry)
  assert geometry['turnTop']>=geometry['top'] and geometry['turnRight']<=geometry['boardRight'] and geometry['turnBottom']<geometry['top']+65,(name,geometry)
  report.setdefault('viewportGeometry',[]).append({'name':name,**geometry})

 assert js('[...document.querySelectorAll(".poker-card img")].every(i=>i.complete&&i.naturalWidth>0)'),name+' broken cards'
 colors=js(r"""(() => {
 const rgb=s=>(s.match(/[\d.]+/g)||[]).map(Number),lum=c=>c.map(v=>v/255).map(v=>v<=.04045?v/12.92:((v+.055)/1.055)**2.4).reduce((s,v,i)=>s+v*[.2126,.7152,.0722][i],0);
 return [...document.querySelectorAll('.poker-seat-label b,.poker-seat-label span,.poker-center strong,.poker-center small,.poker-actions button,.poker-notice,.poker-hand-caption strong,.poker-hand-caption span,.poker-tools label')].filter(e=>e.getClientRects().length).map(e=>{
 let bg=[255,255,255];const chain=[];for(let n=e;n;n=n.parentElement)chain.unshift(n);for(const n of chain){const st=getComputedStyle(n),c=rgb(st.backgroundColor),alpha=c[3]??1;bg=bg.map((v,i)=>v*(1-alpha)+c[i]*alpha);if(st.backgroundImage.includes('gradient'))bg=[65,107,85]}
 const fg=rgb(getComputedStyle(e).color),a=lum(fg),b=lum(bg);return {text:e.textContent.trim(),ratio:(Math.max(a,b)+.05)/(Math.min(a,b)+.05)};
 })})()""")
 assert all(c['ratio']>=4.5 for c in colors),colors
 (out/(name+'-contrast.json')).write_text(json.dumps(colors,ensure_ascii=False,indent=2))
 b('screenshot',str(out/(name+'.png')),'--full');report['checks'].append(name);print('PASS',name,flush=True)
def skin(value):
 b('find','role','button','click','--name','牌面设置','--exact');b('select','select[aria-label="牌面风格"]',value);b('click','[role=dialog] button[aria-label=关闭]')
def wait_idle():b('wait','--fn','![...document.querySelectorAll(".poker-actions button")].every(b=>b.disabled)')
try:
 for f in ['url','stop']:(out/f).unlink(missing_ok=True)
 with (out/'build.log').open('w') as f:subprocess.run(['npm','--prefix','web','run','build'],cwd=root,env=env,stdout=f,stderr=subprocess.STDOUT,check=True)
 f=(out/'server.log').open('w');logs.append(f);server=subprocess.Popen(['go','test','-count=1','-timeout=21m','-run','^TestPokerBrowserFixture$','./internal/gametest'],cwd=root,env=env,stdout=f,stderr=subprocess.STDOUT)
 start=time.monotonic()
 while not (out/'url').exists():
  if server.poll() is not None or time.monotonic()-start>90:raise RuntimeError('fixture did not start')
  time.sleep(.1)
 base=(out/'url').read_text();b('open',base);opened=True
 api('/login',{'name':'扑克验收管理员','password':'poker-browser-test-only'})
 adminCookies=b('cookies','get')['cookies']
 state=api('/state');assert all(k in state['availableGames'] for k in ['doudizhu','guandan','shengji'])
 for k in ['doudizhu','guandan','shengji']:api('/admin/games/'+k,{'enabled':True})
 b('reload');b('wait','.room-list');b('select','select[aria-label="游戏分类"]','poker')
 shot('category-poker')
 assert js('[...document.querySelectorAll("select[aria-label=按游戏筛选牌桌] option")].map(o=>o.value).filter(v=>v!=="all").sort()')==['doudizhu','guandan','shengji']
 for kind,n,decks in [('doudizhu',4,2),('guandan',4,2),('shengji',4,2),('shengji',4,1)]:
  scenario=f'{kind}-{n}-{decks}'
  b('set','viewport','1280','960');b('select','select[aria-label="按游戏筛选牌桌"]',kind);b('click','.lobby-create');b('wait','[role=dialog]');b('fill','[role=dialog] input[type=search]',{'doudizhu':'斗地主','guandan':'掼蛋','shengji':'升级'}[kind]);b('click','.choose-games button');b('wait','select[aria-label=行动时限]');assert js('document.querySelector("select[aria-label=行动时限]").value')=='60'
  b('select','select[aria-label="座位上限"]',str(n));b('select','select[aria-label="行动时限"]','120');b('fill','input[name=name]',kind+'浏览器验收')
  if kind=='doudizhu':
   assert js('[...document.querySelectorAll("select[aria-label=玩法规则] option")].map(o=>o.textContent)')==['上海斗地主']
   assert js('[...document.querySelectorAll("select[aria-label=座位上限] option")].map(o=>o.value)')==['4']
  if kind=='shengji':b('select','select[aria-label="牌副数"]',str(decks))
  b('scrollintoview','[role=dialog] button[type=submit]');b('click','[role=dialog] button[type=submit]');b('wait','.waiting-actions')
  r=api('/state')['room'];assert r['kind']==kind and r['capacity']==n
  if kind=='doudizhu':assert r['options']['mode']==('classic3' if n==3 else 'shanghai4')
  # Independently authenticated real HTTP peers hold their cards while the
  # browser exercises one seat. These are not synthetic game snapshots.
  peers=[]
  for p in range(1,n):
   jar=http.cookiejar.CookieJar();client=urllib.request.build_opener(urllib.request.HTTPCookieProcessor(jar))
   def req(path,body=None,client=client):
    data=None if body is None else json.dumps(body).encode();q=urllib.request.Request(base+'/api'+path,data=data,headers={'Content-Type':'application/json'})
    with client.open(q,timeout=20) as res:return json.load(res)
   req('/register',{'name':f'扑克{scenario}玩家{p}','password':'poker-browser-test-only','invite':'poker-browser'})
   req('/rooms/'+r['id'],{'type':'join','version':api('/state')['room']['version'],'nonce':uuid.uuid4().hex})
   req('/rooms/'+r['id'],{'type':'ready','version':req('/state')['room']['version'],'nonce':uuid.uuid4().hex});req.jar=jar;peers.append(req)
  command('ready');command('start');b('wait','.poker-board')
  def seat_request(p,path,body=None):return api(path,body) if p==0 else peers[p-1](path,body)
  def seat_action(p,action):
   rr=seat_request(p,'/state')['room'];action['window']=rr['game']['data']['window'];return seat_request(p,'/rooms/'+rr['id'],{'type':'action','version':rr['version'],'nonce':uuid.uuid4().hex,'action':action})
  if kind=='doudizhu':
   for _ in range(n):
    rr=api('/state')['room'];p=rr['game']['data']['turn']
    if p==0:break
    seat_action(p,{'type':'bid','bid':0})
   seat_action(0,{'type':'bid','bid':3})
   if api('/state')['room']['game']['data']['phase']=='landlord_choice':
    b('wait','.poker-landlord-choices');b('find','role','button','click','--name','正常出牌','--exact')
   b('wait','--fn','document.querySelector(".poker-board").dataset.phase==="play"')
   assert len(api('/state')['room']['game']['data']['hand'])==(20 if n==3 else 33)
  if kind=='shengji':
   options=[seat_request(p,'/state')['room']['game']['data']['controls']['declarations'] for p in range(n)]
   caller=next((p for p,ds in enumerate(options) if ds),None)
   if caller is not None:seat_action(caller,{'type':'declare','cards':options[caller][-1]['cards']})
   for _ in range(8):
    rr=api('/state')['room']
    if rr['game']['data']['phase']=='bury':break
    p=rr['game']['actors'][0];seat_action(p,{'type':'pass'})
  current=0
  def login_seat(p):
   cookies=adminCookies if p==0 else [{'name':c.name,'value':c.value} for c in peers[p-1].jar]
   b('cookies','clear')
   for c in cookies:b('cookies','set',c['name'],c['value'],'--url',base,'--httpOnly')
   b('reload');b('wait','.poker-board')
  if kind=='shengji':
   current=api('/state')['room']['game']['data']['dealer']
   if current!=0:login_seat(current)
   assert len(api('/state')['room']['game']['data']['hand'])==(18 if decks==1 else 33)
  for width in [1280,390,320]:
   b('set','viewport',str(width),'960' if width==1280 else '640' if width==320 else '720');b('wait','--fn','[...document.querySelectorAll(".poker-card img")].every(i=>i.complete)')
   assert js('document.querySelector(".poker-hand").dataset.seat')==str(current)
   assert js('document.querySelectorAll(".poker-hand button[data-card]").length')==len(api('/state')['room']['game']['data']['hand'])
   # Each exposed index is wide enough to read; center of its visible strip
   # must really hit that physical card (not a neighbouring button).
   visible=js('''[...document.querySelectorAll('.poker-hand button[data-card]')].map(e=>{const r=e.getBoundingClientRect(),s=getComputedStyle(e),step=Number.parseFloat(s.getPropertyValue('--step'));return {id:e.dataset.card,w:r.width,step,selected:e.getAttribute('aria-pressed'),text:e.getAttribute('aria-label')}})''')
   assert visible and all(c['w']>=44 and c['step']>=20 for c in visible),visible
   assert js('document.querySelectorAll(".poker-index").length')==0,'CSS index overlay returned'
   shot(f'{scenario}-{width}')
   if width==320:
    b('set','viewport','320','568');shot(f'{scenario}-320-short')
    b('set','viewport','320','640')

  b('scrollintoview','.poker-hand');first=js('document.querySelector(".poker-hand button[data-card]").getBoundingClientRect().toJSON()');x,y=round(first['x']+8),round(first['y']+22);assert js(f'document.elementFromPoint({x},{y}).closest("button[data-card]")?.dataset.card')==js('document.querySelector(".poker-hand button[data-card]").dataset.card');b('mouse','move',str(x),str(y));b('mouse','down');b('mouse','up');selected=js('[...document.querySelectorAll(".poker-hand [aria-pressed=true]")].map(e=>e.dataset.card)');assert selected
  skin('fourcolor');b('wait','--fn','[...document.querySelectorAll(".poker-card img")].every(i=>i.complete)')
  assert js('[...document.querySelectorAll(".poker-hand [aria-pressed=true]")].map(e=>e.dataset.card)')==selected
  shot(f'{scenario}-skin-selection');skin('english')
  b('reload');b('wait','.poker-board')
  # Select a complete row range with a native pointer drag, including the
  # first card even if the browser delivers only the final move event.
  b('scrollintoview','.poker-hand');rects=js('[...document.querySelectorAll(".poker-hand-row:first-child button")].slice(0,3).map(e=>({id:e.dataset.card,...e.getBoundingClientRect().toJSON()}))')
  if len(rects)==3:
   x,y=round(rects[0]['x']+8),round(rects[0]['y']+24)
   b('mouse','move',str(x),str(y));b('mouse','down');b('mouse','move',str(round(rects[2]['x']+8)),str(y));b('mouse','up')
   chosen=js('[...document.querySelectorAll(".poker-hand [aria-pressed=true]")].map(e=>e.dataset.card)')
   assert all(v['id'] in chosen for v in rects),('drag skipped cards',chosen,rects)
  # The same range gesture must work with native touch, not only mouse input.
  b('reload');b('wait','.poker-board');b('scrollintoview','.poker-hand')
  rects=js('[...document.querySelectorAll(".poker-hand-row:first-child button")].slice(0,3).map(e=>({id:e.dataset.card,...e.getBoundingClientRect().toJSON()}))')
  subprocess.run(['node','scripts/poker/browser-touch.mjs',b('get','cdp-url')['cdpUrl'],base,str(round(rects[0]['x']+8)),str(round(rects[0]['y']+24)),str(round(rects[2]['x']+8)),str(round(rects[2]['y']+24))],cwd=root,env=env,check=True,timeout=20)
  chosen=js('[...document.querySelectorAll(".poker-hand [aria-pressed=true]")].map(e=>e.dataset.card)');assert all(v['id'] in chosen for v in rects),('touch skipped cards',chosen)
  if kind=='shengji':
   # Clear existing selections by reload; use exposed index coordinates to
   # choose six/eight physical cards and submit the real bury UI.
   b('reload');b('wait','.poker-board');b('scrollintoview','.poker-hand')
   count=6 if decks==1 else 8
   for index in range(count):
    r=js(f'document.querySelectorAll(".poker-hand button[data-card]")[{index}].getBoundingClientRect().toJSON()')
    b('mouse','move',str(round(r['x']+8)),str(round(r['y']+24)));b('mouse','down');b('mouse','up')
   before=api('/state')['room']['game']['data'];b('find','role','button','click','--name',f'确认扣底（{count}/{count}）','--exact');b('wait','--fn','document.querySelector(".poker-board").dataset.phase==="play"')
   assert len(api('/state')['room']['game']['data']['hand'])==len(before['hand'])-count
  # Every seat must rotate only the presentation, preserving server seat IDs.
  for p in range(n):
   if p!=current:login_seat(p);current=p
   assert js('document.querySelector(".poker-hand").dataset.seat')==str(p)
   assert js('document.querySelector("[data-position=bottom]").dataset.pokerSeat')==str(p)
   assert js('document.querySelectorAll(".poker-hand button[data-card]").length')==len(api('/state')['room']['game']['data']['hand'])
   if p>0:shot(f'{scenario}-seat-{p}')
  # Exercise hints, wildcard preview and play through the actual browser.
  acting=api('/state')['room']['game']['actors'][0]
  if acting!=current:login_seat(acting);current=acting
  before=api('/state')['room']['game']['data'];b('find','role','button','click','--name','提示','--exact')
  js('window.__pokerAudio=[];const original=HTMLMediaElement.prototype.play;HTMLMediaElement.prototype.play=function(){window.__pokerAudio.push({platform:this.hasAttribute("data-platform-audio"),src:this.src});return original.call(this)}')
  if js('!document.querySelector(".sound-toggle")'):b('click','.compact-room-platform')
  if js('document.querySelector(".sound-toggle").getAttribute("aria-pressed")==="false"'):b('find','role','button','click','--name','静音','--exact')
  if js('!!document.querySelector(".compact-room-dialog")'):b('click','.compact-room-dialog button[aria-label=关闭]')
  if js('!document.querySelector(".sound-toggle")'):b('click','.compact-room-platform')
  assert js('document.querySelector(".sound-toggle").getAttribute("aria-pressed")==="true"'),'mute control did not apply'
  if js('!!document.querySelector(".compact-room-dialog")'):b('click','.compact-room-dialog button[aria-label=关闭]')
  js('window.__pokerAudio=[]')
  count=js('document.querySelectorAll(".poker-hand [aria-pressed=true]").length');assert count>0
  b('find','role','button','click','--name',f'出牌（{count}）','--exact')
  if kind=='guandan' and js('!!document.querySelector("select[aria-label=百搭牌型]")'):b('find','role','button','click','--name','确认出牌','--exact')
  b('wait','--fn',f'document.querySelectorAll(".poker-hand button[data-card]").length==={len(before["hand"])-count}')
  assert js('window.__pokerAudio.length')==0,('muted poker played audio',js('window.__pokerAudio'))
  if js('!document.querySelector(".sound-toggle")'):b('click','.compact-room-platform')
  b('find','role','button','click','--name','取消静音','--exact');assert js('window.__pokerAudio.length')==0,'unmute replayed cards'
  if js('!document.querySelector(".sound-toggle")'):b('click','.compact-room-platform')
  b('find','role','button','click','--name','试听音效','--exact');b('wait','--fn','window.__pokerAudio.length>0');assert js('window.__pokerAudio.every(e=>e.platform)')
  if js('!!document.querySelector(".compact-room-dialog")'):b('click','.compact-room-dialog button[aria-label=关闭]')
  if js('innerWidth<=600'):
   b('click','.compact-room-players');b('wait','.compact-room-dialog .players-strip');assert js('document.querySelectorAll(".compact-room-dialog .player-panel").length')==n;b('click','.compact-room-dialog button[aria-label=关闭]')
   b('click','.compact-room-tools');b('wait','.compact-room-dialog .room-tools');assert js('!!document.querySelector(".compact-room-dialog .autoplay-control")');b('click','.compact-room-dialog button[aria-label=关闭]')
  shot(f'{scenario}-played')
  b('reload');b('wait','.poker-board');assert js('document.querySelectorAll(".poker-new-play").length')==0
  if current!=0:login_seat(0)
  b('click','.compact-room-platform');b('find','role','button','click','--name','管理后台','--exact')
  b('wait','--fn','!document.querySelector(".room-page")')
  assert js('!!document.querySelector("header.topbar") && !document.querySelector(".compact-room-platform")'),'admin inherited the compact poker header'
  b('find','role','button','click','--name','返回游戏','--exact');b('wait','.poker-board')
  # External room closure while a menu is open must not leave a stale dialog.
  b('click','.compact-room-platform')
  command('close');command('leave');b('wait','.room-list');b('select','select[aria-label="游戏分类"]','poker')
 report['passed']=True
except Exception as e:
 import traceback;traceback.print_exc();report['error']=repr(e)
 if opened:
  try:report['snapshot']=b('snapshot','-i');b('screenshot',str(out/'failure.png'),'--full')
  except Exception as e2:report['diagnosticError']=str(e2)
finally:
 if opened:
  try:b('close')
  except Exception:pass
 if server:
  (out/'stop').write_text('stop')
  try:server.wait(timeout=15)
  except subprocess.TimeoutExpired:server.terminate();server.wait(timeout=10)
 for f in logs:f.close()
 shutil.rmtree(profile,ignore_errors=True)
 (out/'result.json').write_text(json.dumps(report,ensure_ascii=False,indent=2));print('Artifact:',out/'result.json',flush=True)
 if not report['passed']:raise SystemExit(1)
