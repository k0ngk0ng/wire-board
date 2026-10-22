#!/usr/bin/env python3
"""Targeted real UI feedback checks using explicitly labelled starting hands.
Exercises manual unordered selections, trio/straight/bomb/rocket speech, mute,
Joker artwork, trump-first ordering and atomic refusal of an invalid throw.
No production debug routes; only the opt-in Go test fixture changes starting hands.
"""
import json,os,subprocess,time,uuid,urllib.request,http.cookiejar
from pathlib import Path
root=Path(__file__).resolve().parents[2];out=root/'.local/poker-fixes/feedback';fixture=root/'.local/poker/browser';out.mkdir(parents=True,exist_ok=True);fixture.mkdir(parents=True,exist_ok=True)
env=dict(os.environ)
for key,rel in {'AGENT_BROWSER_SOCKET_DIR':'.local/ab-pfb','AGENT_BROWSER_PROFILE':'.local/poker-fixes/feedback-profile','TMPDIR':'.local/tmp','GOTMPDIR':'.local/tmp','GOCACHE':'.local/go-build','GOMODCACHE':'.local/go-mod','npm_config_cache':'.local/npm'}.items():
 p=root/rel;p.mkdir(parents=True,exist_ok=True);env[key]=str(p)
env.update(WIRE_BOARD_POKER_BROWSER='1',WIRE_BOARD_POKER_FEEDBACK='1',AGENT_BROWSER_SESSION='pf-'+uuid.uuid4().hex[:6],AGENT_BROWSER_ARGS='--mute-audio,--disable-breakpad,--disable-crash-reporter',GOTOOLCHAIN='local')
report={'passed':False,'scope':'labelled deterministic opening hands; real UI/HTTP and production actions','checks':[],'audio':[]};opened=False;server=None;handle=None

def b(*args):
 p=subprocess.run(['agent-browser',*args,'--json'],cwd=root,env=env,text=True,capture_output=True,timeout=40)
 if p.returncode:raise RuntimeError(p.stdout+p.stderr)
 r=json.loads(p.stdout);assert r['success'],r;return r['data']
def js(code):return b('eval',code)['result']
def api(path,body=None):
 opt='' if body is None else ',{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify('+json.dumps(body)+')}'
 r=js('fetch('+json.dumps('/api'+path)+opt+').then(async r=>({status:r.status,body:await r.json()}))');assert 200<=r['status']<300,r;return r['body']
def command(kind):
 r=api('/state')['room'];return api('/rooms/'+r['id'],{'type':kind,'version':r['version'],'nonce':uuid.uuid4().hex})
def choose_skin(value):
 b('find','role','button','click','--name','牌面设置','--exact');b('select','select[aria-label="牌面风格"]',value);b('click','[role=dialog] button[aria-label=关闭]')
def select(ids):
 if js('document.querySelectorAll(".poker-hand [aria-pressed=true]").length'):b('focus','.poker-hand button');b('press','Escape')
 for id in ids:
  selector=f'.poker-hand button[data-card="{id}"]';b('scrollintoview',selector);r=js('document.querySelector('+json.dumps(selector)+').getBoundingClientRect().toJSON()');x,y=round(r['x']+8),round(r['y']+24)
  assert js(f'document.elementFromPoint({x},{y}).closest("button[data-card]")?.dataset.card')==str(id)
  b('mouse','move',str(x),str(y));b('mouse','down');b('mouse','up')
 assert sorted(js('[...document.querySelectorAll(".poker-hand [aria-pressed=true]")].map(e=>+e.dataset.card)'))==sorted(ids)
def probe():
 js('window.__feedbackAudio=[];if(!window.__originalMediaPlay){window.__originalMediaPlay=HTMLMediaElement.prototype.play;HTMLMediaElement.prototype.play=function(){window.__feedbackAudio.push({src:this.src,platform:this.hasAttribute("data-platform-audio")});return window.__originalMediaPlay.call(this)}}')
def shot(name):
 b('wait','--fn','[...document.querySelectorAll(".poker-new-play")].every(e=>e.getAnimations().every(a=>a.playState!=="running"))')
 b('wait','--fn','[...document.querySelectorAll(".poker-card img")].every(e=>e.complete&&e.naturalWidth>0)')
 assert not b('errors')['errors'],b('errors');assert js('document.documentElement.scrollWidth<=innerWidth+2'),name
 assert js('document.querySelectorAll(".poker-index").length')==0
 assert js('document.querySelectorAll(".poker-card-mark").length')==0,'card corners must not carry Chinese badges'
 assert js('[...document.querySelectorAll(".poker-card[data-card-face] img")].filter(e=>+e.parentElement.dataset.cardFace>=0).every(e=>e.src.includes("/poker/v4/"))'),'latest integrated faces required'
 # Every played row must show each rank's exposed corner, including wrapped rows.
 b('scrollintoview','.poker-arena')
 hits=js('''[...document.querySelectorAll('.poker-play .poker-strip-slot')].map(e=>{const area=e.closest('.poker-play');area.scrollLeft=e.offsetLeft;const r=e.getBoundingClientRect(),x=r.left+6,y=r.top+10;const result={id:e.querySelector('[data-card-face]').dataset.cardFace,hit:e.contains(document.elementFromPoint(x,y)),x,y};area.scrollLeft=0;return result})''')
 assert all(h['hit'] for h in hits),hits
 b('screenshot',str(out/(name+'.png')),'--full');report['checks'].append(name);print('PASS',name,flush=True)
try:
 for name in ['url','stop']:(fixture/name).unlink(missing_ok=True)
 with (out/'build.log').open('w') as f:subprocess.run(['npm','--prefix','web','run','build'],cwd=root,env=env,stdout=f,stderr=subprocess.STDOUT,check=True)
 handle=(out/'server.log').open('w');server=subprocess.Popen(['go','test','-count=1','-timeout=21m','-run','^TestPokerBrowserFixture$','./internal/gametest'],cwd=root,env=env,stdout=handle,stderr=subprocess.STDOUT)
 start=time.monotonic()
 while not (fixture/'url').exists():
  if server.poll() is not None or time.monotonic()-start>90:raise RuntimeError('fixture failed')
  time.sleep(.1)
 base=(fixture/'url').read_text();b('open',base);opened=True;api('/login',{'name':'扑克验收管理员','password':'poker-browser-test-only'})
 for game in ['doudizhu','shengji']:
  api('/admin/games/'+game,{'enabled':True});r=api('/rooms',{'kind':game,'name':'牌面与报牌验收','capacity':4,'options':{'turnSeconds':120}});peers=[]
  for p in range(1,4):
   jar=http.cookiejar.CookieJar();client=urllib.request.build_opener(urllib.request.HTTPCookieProcessor(jar))
   def req(path,body=None,client=client):
    request=urllib.request.Request(base+'/api'+path,data=None if body is None else json.dumps(body).encode(),headers={'Content-Type':'application/json'})
    with client.open(request,timeout=20) as response:return json.load(response)
   req('/register',{'name':game+'反馈玩家'+str(p),'password':'poker-browser-test-only','invite':'poker-browser'})
   req('/rooms/'+r['id'],{'type':'join','version':api('/state')['room']['version'],'nonce':uuid.uuid4().hex})
   rr=req('/state')['room'];req('/rooms/'+r['id'],{'type':'ready','version':rr['version'],'nonce':uuid.uuid4().hex});peers.append(req)
  command('ready');command('start');b('reload');b('wait','.poker-board');b('set','viewport','320','900')
  if js('document.querySelector(".sound-toggle").getAttribute("aria-pressed")==="true"'):b('find','role','button','click','--name','取消静音','--exact')
  if game=='doudizhu':
   # Check all three skins without changing the selected physical cards.
   for skin in ['english','illustrated','fourcolor']:
    choose_skin(skin);shot('joker-'+skin)
   choose_skin('english')
   for ids,expected,cue in [([19,1,6,27,14],[1,14,27,6,19],'trio-pair'),([12,3,13,2,40],[3,2,40,13,12],'straight'),([44,5,31,18],[5,18,31,44],'bomb'),([52,107,53,106],[53,107,52,106],'rocket')]:
    select(ids);probe();b('find','role','button','click','--name',f'出牌（{len(ids)}）','--exact');b('wait','--fn','window.__feedbackAudio.some(v=>v.src.includes("/poker/"))')
    played=api('/state')['room']['game']['data']['lastPlay']['cards'];assert played==expected,(played,expected)
    audio=js('window.__feedbackAudio.filter(v=>v.src.includes("/poker/"))');assert len(audio)==1 and audio[0]['platform'] and audio[0]['src'].endswith('/'+cue+'.mp3'),audio;report['audio'].extend(audio)
    for width in [1280,320]:b('set','viewport',str(width),'900');shot(cue+'-'+str(width))
    for req in peers:
     rr=req('/state')['room'];req('/rooms/'+rr['id'],{'type':'action','version':rr['version'],'nonce':uuid.uuid4().hex,'action':{'type':'pass','window':rr['game']['data']['window']}})
   # Muting covers speech and bomb clips, and unmuting does not resume a cue.
   select([api('/state')['room']['game']['data']['hand'][0]]);b('find','role','button','click','--name','静音','--exact');probe();b('find','role','button','click','--name','出牌（1）','--exact');b('wait','--fn','document.querySelectorAll(".poker-hand [aria-pressed=true]").length===0');assert js('window.__feedbackAudio.length')==0
   b('find','role','button','click','--name','取消静音','--exact');assert js('window.__feedbackAudio.length')==0
  else:
   d=api('/state')['room']['game']['data'];assert [id%54 for id in d['hand'][:4]]==[53,53,52,52],d['hand']
   dom=js('[...document.querySelectorAll(".poker-hand button[data-card]")].map(e=>+e.dataset.card)');assert dom==d['hand'];shot('shengji-trumps-first')
   ids=[9,12,64,10];select(ids);probe();before=api('/state')['room'];b('find','role','button','click','--name','出牌（4）','--exact');b('wait','--text','这些牌不能一起甩出');after=api('/state')['room']
   assert before['version']==after['version'] and before['game']['data']==after['game']['data'],'rejected throw changed state'
   assert sorted(js('[...document.querySelectorAll(".poker-hand [aria-pressed=true]")].map(e=>+e.dataset.card)'))==sorted(ids),'rejected selection was cleared'
   assert js('window.__feedbackAudio.length')==0,'invalid throw announced as played';shot('shengji-rejected-throw')
  command('close');command('leave');b('wait','.room-list')
 report['passed']=True
except Exception as e:
 import traceback;traceback.print_exc();report['error']=repr(e)
 if opened:
  try:b('screenshot',str(out/'failure.png'),'--full');report['snapshot']=b('snapshot','-i');report['state']=api('/state')
  except Exception:pass
finally:
 if opened:
  try:b('close')
  except Exception:pass
 if server:
  (fixture/'stop').write_text('stop')
  try:server.wait(timeout=15)
  except subprocess.TimeoutExpired:server.terminate();server.wait(timeout=10)
 if handle:handle.close()
 (out/'result.json').write_text(json.dumps(report,ensure_ascii=False,indent=2)+'\n');print('Artifact:',out/'result.json',flush=True)
 if not report['passed']:raise SystemExit(1)
