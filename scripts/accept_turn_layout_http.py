#!/usr/bin/env python3
"""Live Weiqi HTTP/browser regression for the shared turn bar.
Failure cases: landing animation hides timer and shifts the board; repeated
own/opponent turns change height; countdown visibility leaks a false 00:00.
Uses existing opt-in fixture; no game-rule or production configuration changes.
"""
import http.cookiejar,json,os,subprocess,time,urllib.request,uuid
from pathlib import Path
root=Path(__file__).resolve().parents[1];out=root/'.local/turn-layout/http';out.mkdir(parents=True,exist_ok=True);fixture=root/'.local/weiqi/browser';fixture.mkdir(parents=True,exist_ok=True)
env=dict(os.environ)
for k,v in {'GOCACHE':'.local/go-build','GOMODCACHE':'.local/go-mod','GOTMPDIR':'.local/tmp','TMPDIR':'.local/tmp','npm_config_cache':'.local/npm','AGENT_BROWSER_SOCKET_DIR':'.local/ab-turn-live','AGENT_BROWSER_PROFILE':'.local/turn-layout/http/profile'}.items():
 p=root/v;p.mkdir(parents=True,exist_ok=True);env[k]=str(p)
env.update(GOTOOLCHAIN='local',WIRE_BOARD_WEIQI_BROWSER='1',AGENT_BROWSER_SESSION='turn-live-'+uuid.uuid4().hex[:8],AGENT_BROWSER_ARGS='--mute-audio,--disable-breakpad,--disable-crash-reporter')
env.pop('WIRE_BOARD_WEIQI_SCENARIO',None)
report={'passed':False,'widths':[],'command':'. scripts/env.sh && python3 -B scripts/accept_turn_layout_http.py'};server=None;opened=False
peer=urllib.request.build_opener(urllib.request.HTTPCookieProcessor(http.cookiejar.CookieJar()))
def b(*args):
 r=subprocess.run(['agent-browser',*args,'--json'],cwd=root,env=env,capture_output=True,text=True,timeout=40)
 if r.returncode:raise RuntimeError(r.stdout+r.stderr)
 d=json.loads(r.stdout);assert d['success'],d;return d['data']
def js(code):return b('eval',code)['result']
def api(path,body=None):
 opts='' if body is None else ',{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify('+json.dumps(body)+')}'
 d=js('fetch('+json.dumps('/api'+path)+opts+').then(async r=>({status:r.status,body:await r.json()}))');assert d['status']<300,d;return d['body']
def peer_api(path,body=None):
 with peer.open(urllib.request.Request(base+'/api'+path,data=None if body is None else json.dumps(body).encode(),headers={'Content-Type':'application/json'}),timeout=10) as r:return json.load(r)
def contrast():
 rows=js(r"""(()=>{const rgb=s=>(s.match(/[\d.]+/g)||[]).map(Number),lum=c=>c.slice(0,3).map(v=>v/255).map(v=>v<=.04045?v/12.92:((v+.055)/1.055)**2.4).reduce((s,v,i)=>s+v*[.2126,.7152,.0722][i],0);return [...document.querySelectorAll('.match-result-modal .results p,.match-result-modal .results h2,.match-result-modal .result-ratings b,.match-result-modal .eyebrow,.match-result-modal button')].map(e=>{let bg=[255,255,255],chain=[];for(let n=e;n;n=n.parentElement)chain.unshift(n);for(const n of chain){const c=rgb(getComputedStyle(n).backgroundColor),a=c[3]??1;bg=bg.map((v,i)=>c[i]*a+v*(1-a))};const a=lum(rgb(getComputedStyle(e).color)),b=lum(bg);return {text:e.textContent.trim(),ratio:(Math.max(a,b)+.05)/(Math.min(a,b)+.05)}})})()""")
 assert all(r['ratio']>=4.5 for r in rows),rows
 return rows
def room():return api('/state')['room']
def command(kind,other=False,**extra):
 r=room();return (peer_api if other else api)('/rooms/'+r['id'],dict(type=kind,version=r['version'],nonce=uuid.uuid4().hex,**extra))
try:
 with (out/'build.log').open('w') as f:subprocess.run(['npm','--prefix','web','run','build'],cwd=root,env=env,stdout=f,stderr=subprocess.STDOUT,check=True)
 for name in ['url','stop']:(fixture/name).unlink(missing_ok=True)
 with (out/'server.log').open('w') as f:server=subprocess.Popen(['go','test','./games/weiqi/tests','-run','^TestWeiqiBrowserFixture$','-count=1','-timeout=13m'],cwd=root,env=env,stdout=f,stderr=subprocess.STDOUT)
 for _ in range(900):
  if (fixture/'url').exists():break
  if server.poll() is not None:raise RuntimeError('fixture exited')
  time.sleep(.1)
 base=(fixture/'url').read_text();b('open',base);opened=True;b('tab',next(t['tabId'] for t in b('tab')['tabs'] if t['active']))
 api('/login',{'name':'棋类验收','password':'weiqi-browser-only'});api('/admin/games/weiqi',{'enabled':True});api('/rooms',{'kind':'weiqi','name':'回合栏布局验收','capacity':2,'options':{'size':19}})
 peer_api('/register',{'name':'回合布局对手','password':'layout-browser-only','invite':'weiqi-browser'});command('join',True);command('ready',True);command('ready');command('start');b('reload');b('wait','.weiqi-board')
 for width in [1440,851,850,390,320]:
  b('set','viewport',str(width),'900');b('scroll','up','10000');js('new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r)))')
  js('(()=>{window.__turnFrames=[];window.__sampleTurn=true;const sample=()=>{if(!window.__sampleTurn)return;const e=document.querySelector(".turn-card"),g=document.querySelector(".game-main");window.__turnFrames.push({at:performance.now(),height:e.getBoundingClientRect().height,boardY:g.getBoundingClientRect().y+scrollY,label:e.querySelector("h3").textContent,moving:document.querySelector(".weiqi-board").dataset.moving,clockVisible:!!e.querySelector(".turn-clock")&&getComputedStyle(e.querySelector(".turn-clock")).visibility!=="hidden"});window.__turnRAF=requestAnimationFrame(sample)};sample()})()')
  for _ in range(6):
   r=room();actor=r['game']['data']['turn'];c=peer_api if actor==1 else api;r=c('/state')['room'];d=r['game']['data'];assert d['legal'];c('/rooms/'+r['id'],dict(type='action',version=r['version'],nonce=uuid.uuid4().hex,action=dict(type='place',round=d['round'],ply=d['ply'],revision=d['revision'],pos=d['legal'][0])))
   b('wait','--fn','document.querySelector(".weiqi-board").dataset.ply==='+json.dumps(str(d['ply']+1))+'&&!document.querySelector(".weiqi-board").dataset.moving');js('new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r)))')
  frames=js('window.__sampleTurn=false;cancelAnimationFrame(window.__turnRAF);window.__turnFrames');row={'width':width,'frames':frames,'heightRange':[min(f['height'] for f in frames),max(f['height'] for f in frames)],'boardYRange':[min(f['boardY'] for f in frames),max(f['boardY'] for f in frames)]};report['widths'].append(row)
  b('screenshot',str(out/f'{width}.png'),'--full')
  assert {'请行动','对局进行中','等待其他玩家'}<=set(f['label'] for f in frames), 'animation/turn states not sampled'
  assert row['heightRange'][1]-row['heightRange'][0]<.5 and row['boardYRange'][1]-row['boardYRange'][0]<.5,{k:v for k,v in row.items() if k!='frames'}
  assert all(not f['clockVisible'] for f in frames if f['moving']),'animation displayed a false countdown'
  durations=[]
  for event in {f['moving'] for f in frames if f['moving']}:
   motion=[f for f in frames if f['moving']==event];elapsed=motion[-1]['at']-motion[0]['at'];durations.append(elapsed);assert elapsed<1200,{'width':width,'event':event,'animationMilliseconds':elapsed}
  row['animationDurations']=durations
  print('PASS',width,'height',row['heightRange'],'board Y',row['boardYRange'],flush=True)
 # Terminal result must float over the board, close/reopen, and stay closed
 # when ordinary server snapshots arrive. Rematch must clear its local state.
 before_finish=js('document.querySelector(".game-layout").getBoundingClientRect().top+scrollY')
 r=peer_api('/state')['room'];d=r['game']['data'];peer_api('/rooms/'+r['id'],dict(type='action',version=r['version'],nonce=uuid.uuid4().hex,action=dict(type='resign',round=d['round'],ply=d['ply'],revision=d['revision'])))
 b('wait','.results');assert js('!!document.querySelector(".results").closest("[role=dialog]")'),'result was inserted above the board'
 assert js('getComputedStyle(document.querySelector(".results").closest(".modal-backdrop")).position==="fixed"')
 after_finish=js('document.querySelector(".game-layout").getBoundingClientRect().top+scrollY');report['finishBoardY']={'before':before_finish,'after':after_finish};assert abs(after_finish-before_finish)<.5,report['finishBoardY']
 assert js('document.querySelector(".results h2").textContent.includes("获胜")')
 for width in [1440,390,320]:
  b('set','viewport',str(width),'900');js('new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r)))');position=js('document.querySelector(".game-layout").getBoundingClientRect().top+scrollY')
  b('screenshot',str(out/f'{width}-result-overlay.png'),'--full');(out/f'{width}-result-contrast.json').write_text(json.dumps(contrast(),ensure_ascii=False,indent=2));b('click','.match-result-modal button[aria-label=关闭]');print('CLOSED RESULT',width,flush=True);(out/f'{width}-closed.snapshot.json').write_text(json.dumps(b('snapshot','-i'),ensure_ascii=False));assert js('document.querySelector(".results")===null')
  assert abs(js('document.querySelector(".game-layout").getBoundingClientRect().top+scrollY')-position)<.5,'closing result moved the board'
  hit=js('(()=>{const e=document.querySelector(".result-reopen"),r=e.getBoundingClientRect();return {rect:r.toJSON(),top:document.elementFromPoint(r.x+r.width/2,r.y+r.height/2)?.outerHTML}})()');(out/f'{width}-reopen-hit.json').write_text(json.dumps(hit,ensure_ascii=False,indent=2));assert 'result-reopen' in hit['top'] or '<svg' in hit['top'] or '<path' in hit['top'],hit
  b('click','.result-reopen');b('wait','.results');assert abs(js('document.querySelector(".game-layout").getBoundingClientRect().top+scrollY')-position)<.5,'opening result moved the board'
  assert js('document.documentElement.scrollWidth<=innerWidth+1')
 b('click','.match-result-modal button[aria-label=关闭]');b('wait','15500');assert js('document.querySelector(".results")===null'),'state poll reopened dismissed result'
 b('click','.result-reopen');b('find','role','button','click','--name','再来一局','--exact');b('wait','.waiting-layout');assert js('document.querySelector(".results")===null&&document.querySelector(".result-reopen")===null')
 command('ready',True);command('ready');command('start');b('wait','.weiqi-board')
 for _ in range(2):
  r=room();c=peer_api if r['game']['data']['turn']==1 else api;r=c('/state')['room'];d=r['game']['data'];c('/rooms/'+r['id'],dict(type='action',version=r['version'],nonce=uuid.uuid4().hex,action=dict(type='place',round=d['round'],ply=d['ply'],revision=d['revision'],pos=d['legal'][0])));b('wait','--fn','document.querySelector(".weiqi-board").dataset.ply==='+json.dumps(str(d['ply']+1))+'&&!document.querySelector(".weiqi-board").dataset.moving')
 for _ in range(2):
  r=room();d=r['game']['data'];command('action',other=d['turn']==1,action=dict(type='pass',round=d['round'],ply=d['ply'],revision=d['revision']))
 b('wait','.weiqi-scoring');d=room()['game']['data'];dead=next(i for i,c in enumerate(d['board']) if c);command('action',action=dict(type='mark_dead',round=d['round'],ply=d['ply'],revision=d['revision'],pos=dead));d=room()['game']['data'];command('action',other=True,action=dict(type='confirm_score',round=d['round'],ply=d['ply'],revision=d['revision']))
 js('(()=>{window.__finalFrames=[];window.__sampleFinal=true;const sample=()=>{if(!window.__sampleFinal)return;window.__finalFrames.push({y:document.querySelector(".game-layout").getBoundingClientRect().top+scrollY,moving:document.querySelector(".weiqi-board").dataset.moving});window.__finalRAF=requestAnimationFrame(sample)};sample()})()')
 d=room()['game']['data'];command('action',action=dict(type='confirm_score',round=d['round'],ply=d['ply'],revision=d['revision']));b('wait','.results');js('new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r)))');frames=js('window.__sampleFinal=false;cancelAnimationFrame(window.__finalRAF);window.__finalFrames');report['animatedResult']=frames;assert any(f['moving'] for f in frames),'final removal animation not sampled';assert max(f['y'] for f in frames)-min(f['y'] for f in frames)<.5,{'animatedResultY':[min(f['y'] for f in frames),max(f['y'] for f in frames)]}

 b('wait','.results');assert js('!!document.querySelector(".results").closest("[role=dialog]")');report['resultOverlay']={'win':True,'widths':[1440,390,320],'closeReopenStable':True,'staysClosedAfterPoll':True,'rematchResets':True}
 assert not b('errors')['errors'];report['passed']=True
except Exception as e:
 report['error']=str(e);raise
finally:
 if opened:
  try:b('close')
  except Exception:pass
 if server:
  (fixture/'stop').write_text('stop')
  try:server.wait(timeout=15)
  except subprocess.TimeoutExpired:server.terminate();server.wait(timeout=15)
 (out/'result.json').write_text(json.dumps(report,ensure_ascii=False,indent=2)+'\n')
