#!/usr/bin/env python3
"""Native classic battlefield acceptance; see NATIVE-BROWSER-SCENARIOS.md.
Two untouched native maps, then three explicitly isolated INITIAL scenarios.
Production HTTP/auth/SQLite/Apply/OnTimer/View; native CDP input only after start.
Never builds dist or changes application sources.
"""
import argparse, hashlib, http.cookiejar, json, os
from pathlib import Path
import shutil, subprocess, time, traceback, urllib.request, uuid

ROOT = Path(__file__).resolve().parents[2]
ap = argparse.ArgumentParser(); ap.add_argument('--out', required=True); args = ap.parse_args()
OUT = (ROOT/args.out).resolve(); assert OUT.is_relative_to(ROOT) and OUT != ROOT and not OUT.exists()
OUT.mkdir(parents=True); TEMP=OUT/'temporary'; TEMP.mkdir(); RUN='nb'+uuid.uuid4().hex[:6]
SOCKETS=ROOT/'.s'/RUN; SOCKETS.mkdir(parents=True)
ENV=dict(os.environ, GOTOOLCHAIN='local', WIRE_BOARD_BUBBLEPARTY_BROWSER='1', WIRE_BOARD_BUBBLEPARTY_BROWSER_OUT=str(OUT), AGENT_BROWSER_SESSION=RUN, AGENT_BROWSER_PROFILE=str(TEMP/'profile'), AGENT_BROWSER_SOCKET_DIR=str(SOCKETS), AGENT_BROWSER_ARGS='--mute-audio,--disable-breakpad,--disable-crash-reporter')
for key,path in {'TMPDIR':TEMP/'tmp','GOTMPDIR':TEMP/'tmp','GOCACHE':ROOT/'.local/go-build','GOMODCACHE':ROOT/'.local/go-mod'}.items():path.mkdir(parents=True,exist_ok=True);ENV[key]=str(path)
REPORT={'passed':False,'checks':[],'scope':'two untouched production native maps; three initial-only isolated scenarios; production HTTP/SQLite/physics; native keyboard/touch input','distSHA256':hashlib.sha256((ROOT/'web/dist/index.html').read_bytes()).hexdigest(),'harnessSHA256':hashlib.sha256(Path(__file__).read_bytes()).hexdigest()}
SERVER=LOG=None;OPEN=False;CLIENTS={};BASE=''

def wait(fn,seconds=12):
 end=time.monotonic()+seconds
 while time.monotonic()<end:
  value=fn()
  if value:return value
  time.sleep(.04)
 raise AssertionError('condition timed out')

def browser(*args):
 r=subprocess.run(['agent-browser',*args,'--json'],cwd=ROOT,env=ENV,text=True,capture_output=True,timeout=45)
 assert r.returncode==0,r.stdout+r.stderr
 d=json.loads(r.stdout);assert d['success'],d
 return d['data']

def js(code):return browser('eval',code)['result']
def api(role,path,body=None):
 req=urllib.request.Request(BASE+'/api'+path,data=None if body is None else json.dumps(body).encode(),headers={'Content-Type':'application/json'})
 with CLIENTS[role].open(req,timeout=10) as r:return json.load(r)
def room(role=0):return api(role,'/state')['room']
def view():return room()['game']['data']
def cmd(role,kind,**kwargs):
 r=room(role);return api(role,'/rooms/'+r['id'],{'type':kind,'version':r['version'],'nonce':uuid.uuid4().hex,**kwargs})
def check(name,**extra):REPORT['checks'].append({'name':name,**extra});print('PASS',name,flush=True)
def save(name,value):(OUT/(name+'.json')).write_text(json.dumps(value,ensure_ascii=False,indent=2)+'\n')
def snapshot():return js('(()=>{const c=document.querySelector("[data-bubbleparty-canvas]");return {canvas:{...c.dataset},label:c.getAttribute("aria-label"),summary:[...document.querySelectorAll(".bubble-player-summary")].map(e=>e.textContent),assets:performance.getEntriesByType("resource").filter(r=>r.name.includes("/bubbleparty/v4/")).map(r=>({url:r.name,bytes:r.transferSize,duration:r.duration})),selection:document.getSelection()?.toString()||""}})()')
def screenshot(name):
 browser('screenshot',str(OUT/(name+'.png')),'--full')
 REPORT.setdefault('screenshots',[]).append(name+'.png')

# The generated overlay changes the browser FIXTURE ONLY, never production files.
# The wrapper changes precisely three initial snapshots; all subsequent callbacks
# remain original module callbacks. No HTTP state mutation endpoint is added.
PATCH = r'''
func nativeBattlefieldModules() []gameapi.Module {
 modules := games.Modules()
 for i := range modules {
  if modules[i].ID != "bubbleparty" { continue }
  original := modules[i].Start
  starts := 0
  modules[i].Start = func(ctx gameapi.Context, options json.RawMessage) (gameapi.Match,error) {
   m,err := original(ctx,options); if err != nil { return m,err }; starts++
   if starts <= 2 { return m,nil }
   var s bubbleparty.State
   if err=json.Unmarshal(m.Data,&s);err!=nil{return m,err}
   for y:=3;y<=7;y++ {for x:=1;x<=11;x++ {s.Cells[y*15+x]=0;s.NativeObjects[y*15+x]=-1}}
   s.Players[0].X=2;s.Players[0].Y=5;s.Players[0].OffsetX=0;s.Players[0].OffsetY=0
   s.Cells[5*15+8]=1;s.NativeObjects[5*15+8]=91
   if starts==3 {
    s.Players[0].X=3;s.Cells[5*15+4]=4;s.NativeObjects[5*15+4]=66
   } else {
    s.Cells[5*15+6]=4;s.NativeObjects[5*15+6]=66
    kind:="turtle";if starts==5 {kind="airship"}
    s.ItemID=1;s.Items=[]bubbleparty.Item{{ID:1,X:3,Y:5,Type:kind,At:ctx.Now.UnixMilli()}}
   }
   m.Data,err=json.Marshal(&s);return m,err
  }
 }
 return modules
}
'''
CDP = r'''
const [endpoint,prefix,raw,outdir]=process.argv.slice(2), spec=JSON.parse(raw);
const {writeFileSync}=await import('node:fs');const ws=new WebSocket(endpoint);await new Promise((r,j)=>{ws.onopen=r;ws.onerror=j});let id=0;
function call(method,params={},sessionId){const n=++id;return new Promise((r,j)=>{const timer=setTimeout(()=>j(Error('CDP timeout '+method)),12000);const handler=e=>{const m=JSON.parse(e.data);if(m.id!==n)return;clearTimeout(timer);ws.removeEventListener('message',handler);m.error?j(Error(JSON.stringify(m.error))):r(m.result)};ws.addEventListener('message',handler);ws.send(JSON.stringify({id:n,method,params,sessionId}))})}
let sid;const samples=[];let up=false,midshot=false;
try{
 const {targetInfos}=await call('Target.getTargets');const target=targetInfos.find(t=>t.type==='page'&&t.url.startsWith(prefix));if(!target)throw Error('owned target missing');
 ({sessionId:sid}=await call('Target.attachToTarget',{targetId:target.targetId,flatten:true}));
 await call('Runtime.evaluate',{expression:'document.querySelector("[data-bubbleparty-canvas]").focus({preventScroll:true})'},sid);
 const key=spec.key||'ArrowRight',vk={ArrowLeft:37,ArrowUp:38,ArrowRight:39,ArrowDown:40}[key];
 const dispatch=async release=>spec.touch?call('Input.dispatchTouchEvent',{type:release?'touchEnd':'touchStart',touchPoints:release?[]:[{x:spec.touch.x,y:spec.touch.y,id:1}]},sid):call('Input.dispatchKeyEvent',{type:release?'keyUp':'keyDown',key,code:key,windowsVirtualKeyCode:vk,modifiers:0},sid);
 if(spec.touch){await call('Emulation.setTouchEmulationEnabled',{enabled:true,maxTouchPoints:2},sid);await dispatch(false);await call('Input.dispatchTouchEvent',{type:'touchMove',touchPoints:[{x:spec.touch.x+spec.touch.dx,y:spec.touch.y+spec.touch.dy,id:1}]},sid)}else await dispatch(false);
 const started=Date.now();
 while(Date.now()-started<spec.hold+spec.settle){
  if(!up&&Date.now()-started>=spec.hold){await dispatch(true);up=true}
  const r=await call('Runtime.evaluate',{expression:'(async()=>{const r=(await(await fetch("/api/state")).json()).room,c=document.querySelector("[data-bubbleparty-canvas]"),v=r.game.data;return {serverAt:v.serverAt,player:v.players[0],cells:v.cells,nativeObjects:v.nativeObjects,boxMotions:v.boxMotions,events:v.events,canvas:{...c.dataset},selection:document.getSelection()?.toString()||""}})()',awaitPromise:true,returnByValue:true},sid);
  if(r.exceptionDetails)throw Error(JSON.stringify(r.exceptionDetails));const value=r.result.value;samples.push({elapsed:Date.now()-started,...value});
  if(spec.midshot&&!midshot){const boxes=JSON.parse(value.canvas.boxMotionsDrawn||'[]');if(boxes.some(m=>Math.abs(m.x-Math.round(m.x))>.15)){const s=await call('Page.captureScreenshot',{format:'png'},sid);writeFileSync(outdir+'/'+spec.midshot+'.png',Buffer.from(s.data,'base64'));midshot=true}}
  await new Promise(r=>setTimeout(r,16));
 }
 if(!up)await dispatch(true);
 if(spec.touch)await call('Emulation.setTouchEmulationEnabled',{enabled:false},sid);
 console.log(JSON.stringify({samples,midshot}));
}finally{if(sid)await call('Target.detachFromTarget',{sessionId:sid});ws.close()}
'''
def native(name,hold,settle=350,key='ArrowRight',touch=None,midshot=None):
 endpoint=browser('get','cdp-url')['cdpUrl']
 spec={'hold':hold,'settle':settle,'key':key,'touch':touch,'midshot':midshot}
 p=subprocess.run(['node','--input-type=module','-',endpoint,BASE,json.dumps(spec),str(OUT)],input=CDP,cwd=ROOT,env=ENV,text=True,capture_output=True,timeout=30)
 assert p.returncode==0,p.stdout+p.stderr
 d=json.loads(p.stdout);save(name,{'input':spec,**d})
 if midshot and d['midshot']:REPORT.setdefault('screenshots',[]).append(midshot+'.png')
 return d

def start_case(index,mapid):
 global CLIENTS
 CLIENTS={}
 names=[]
 for role in [0,1]:
  CLIENTS[role]=urllib.request.build_opener(urllib.request.HTTPCookieProcessor(http.cookiejar.CookieJar()))
  name=f'原生验收{RUN}{index}{role}';names.append(name)
  api(role,'/register',{'name':name,'password':'bubbleparty-browser-test-only','invite':'bubbleparty-browser'})
 made=api(0,'/rooms',{'kind':'bubbleparty','name':f'原生战场验收{index}','capacity':2,'options':{'map':mapid,'roundSeconds':300}})
 api(1,'/rooms/'+made['id'],{'type':'join','version':room()['version'],'nonce':uuid.uuid4().hex})
 cmd(0,'ready');cmd(1,'ready');cmd(0,'start');v=view();assert v['phase']=='select'
 for role in [0,1]:cmd(role,'action',action={'type':'character','epoch':v['epoch'],'character':role,'confirm':True})
 js('(async()=>{const r=await fetch("/api/login",{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify('+json.dumps({'name':names[0],'password':'bubbleparty-browser-test-only'})+')});if(!r.ok)throw Error(await r.text());return true})()')
 browser('reload');browser('wait','.bubble-board')
 wait(lambda:view()['phase']=='playing')
 wait(lambda:int(snapshot()['canvas'].get('nativeSpritesLoaded',0))>0 and int(snapshot()['canvas'].get('playersNative',0))==2)
 return view()

def speed_samples(result,kind,expected):
 upper=7.0 if kind=='airship' else 5.0
 samples=[x for x in result['samples'] if x['player'].get('mount')==kind and 3.05<x['player']['px']<upper]
 bytime={x['serverAt']:x for x in samples};samples=list(bytime.values());assert len(samples)>=3,('insufficient speed samples',[(x['serverAt'],x['player']['px']) for x in samples])
 a,b=samples[0],samples[-1];actual=(b['player']['px']-a['player']['px'])/((b['serverAt']-a['serverAt'])/1000)
 assert abs(actual-1000/expected)<.4,(kind,actual,expected)
 assert all(abs(s['player']['speedMs']-expected)<1e-6 for s in samples)
 return {'serverMSPerCell':expected,'measuredCellsPerSecond':actual,'from':{'at':a['serverAt'],'x':a['player']['px']},'to':{'at':b['serverAt'],'x':b['player']['px']}}

try:
 fixture=ROOT/'games/bubbleparty/tests/browser_fixture_test.go';src=fixture.read_text();assert 'Modules: games.Modules(),' in src
 src=src.replace('"github.com/k0ngk0ng/wire-board/games"','"github.com/k0ngk0ng/wire-board/games"\n "github.com/k0ngk0ng/wire-board/internal/gameapi"\n bubbleparty "github.com/k0ngk0ng/wire-board/games/bubbleparty/server"').replace('Modules: games.Modules(),','Modules: nativeBattlefieldModules(),')+PATCH
 replacement=TEMP/'browser_fixture_test.go';replacement.write_text(src);overlay=TEMP/'overlay.json';overlay.write_text(json.dumps({'Replace':{str(fixture):str(replacement)}}));REPORT['fixtureOverlaySHA256']=hashlib.sha256(src.encode()).hexdigest()
 LOG=(OUT/'server.log').open('w');SERVER=subprocess.Popen(['go','test','-overlay',str(overlay),'-count=1','-timeout=12m','-run','^TestBubblepartyBrowserFixture$','./games/bubbleparty/tests'],cwd=ROOT,env=ENV,stdout=LOG,stderr=subprocess.STDOUT)
 wait(lambda:(OUT/'url').exists() or (SERVER.poll() is not None and (_ for _ in ()).throw(AssertionError('fixture exited'))),120)
 BASE=(OUT/'url').read_text();browser('open',BASE);OPEN=True;browser('set','viewport','1280','1000')
 for i,mapid in enumerate(['classic2002-village00','classic2002-factory07'],1):
  v=start_case(i,mapid);d=snapshot();assert d['canvas']['terrain']=='classic';assert int(d['canvas']['nativeSpritesFailed'])==0;assert int(d['canvas']['nativeObjectsDrawn'])>20;assert int(d['canvas']['nativeObjectsFallback'])==0;assert 'classic2002' not in d['label'];assert any('/v4/classic/sprites/' in a['url'] for a in d['assets'])
  save('original-map-'+str(i),{'snapshot':v,'render':d});screenshot('original-map-'+str(i));check('untouched native map renders original terrain and Chinese HUD',map=mapid,diagnostics=d['canvas'])
 browser('set','viewport','1280','1000');start_case(3,'classic2002-village07');screenshot('push-before')
 result=native('push-intermediate',180,500,midshot='push-midpoint');assert result['midshot'],'no actual fractional box frame captured'
 boxes=[b for x in result['samples'] for b in json.loads(x['canvas'].get('boxMotionsDrawn','[]'))];assert any(4.1<b['x']<4.9 for b in boxes);assert all(len(json.loads(x['canvas'].get('boxMotionsDrawn','[]')))<=1 for x in result['samples'])
 assert all(m['endAt']-m['at']==300 for x in result['samples'] for m in (x.get('boxMotions') or []))
 assert all(int(x['canvas']['nativeObjectsDrawn'])==sum(i>=0 for i in x['nativeObjects']) and int(x['canvas']['nativeObjectsFallback'])==0 for x in result['samples']), 'native box was duplicated or source picture failed'
 v=view();pushes=[e for e in v['events'] if e['type']=='push'];assert pushes and pushes[0]['x']==5;assert v['cells'][5*15+4]==0 and v['nativeObjects'][5*15+4]==-1 and v['nativeObjects'][5*15+5]==66
 check('native keyboard creates authoritative 300ms box animation without duplicate destination',event=pushes[0],fractionalFrames=len(boxes));screenshot('push-after')
 result=native('push-until-wall',2300,400);v=view();assert v['cells'][5*15+8]==1 and v['nativeObjects'][5*15+8]==91;assert v['players'][0]['px']<=6.22001;assert all(e['x']<=7 for e in v['events'] if e['type']=='push')
 check('continued push cannot move crate or player through solid tree',player=v['players'][0],objects=v['nativeObjects']);screenshot('push-wall')
 browser('reload');browser('wait','.bubble-board');wait(lambda:int(snapshot()['canvas'].get('nativeObjectsDrawn',0))>0);assert view()['nativeObjects'][5*15+7]==66;assert json.loads(snapshot()['canvas']['boxMotionsDrawn'])==[]
 check('reload preserves pushed destination without replay')
 browser('set','viewport','320','900');js('document.querySelector(".bubble-controls").scrollIntoView({block:"end"})');r=js('(()=>{const r=document.querySelector(".bubble-joystick").getBoundingClientRect();return {x:r.x+r.width/2,y:r.y+r.height/2}})()');before=view()['players'][0]['px'];result=native('mobile-joystick',400,300,touch={**r,'dx':-45,'dy':0});after=view()['players'][0];assert after['px']<before-.5 and after['applied']['move']=='none';assert all(not x['selection'] for x in result['samples']);screenshot('push-mobile-320');check('320px native touch joystick moves left and releases without selected text',before=before,after=after)
 for index,kind,expected in [(4,'turtle',600),(5,'airship',120)]:
  browser('set','viewport','1280','1000');start_case(index,'classic2002-village07');wait(lambda:int(snapshot()['canvas'].get('mountItemsNative',0))==1);screenshot(kind+'-item')
  result=native(kind+'-ride',1800 if kind=='turtle' else 1100,400);v=view();assert v['players'][0].get('mount')==kind;wait(lambda:int(snapshot()['canvas'].get('mountedNative',0))==1);d=snapshot();assert any('/v4/mounts/' in a['url'] for a in d['assets']);assert any(('乌龟' if kind=='turtle' else '飞碟') in s for s in d['summary']);speed=speed_samples(result,kind,expected);screenshot(kind+'-mounted');check('legal pickup draws '+kind+' rider and preserves original speed',speed=speed,diagnostics=d['canvas'])
  if kind=='airship':
   assert v['cells'][5*15+6]==4 and v['nativeObjects'][5*15+6]==66;assert 6.8<v['players'][0]['px']<=7.22001;assert not any(e['type']=='push' for e in v['events']);check('airship crosses crate without pushing and stops at tree',player=v['players'][0])
   browser('set','viewport','320','900');browser('reload');browser('wait','.bubble-board');wait(lambda:int(snapshot()['canvas'].get('mountedNative',0))==1);assert view()['players'][0].get('mount')=='airship';screenshot('airship-mobile-320');check('mount persists and draws after 320px reload')
 assert not browser('errors')['errors'];REPORT['passed']=True
except BaseException:
 REPORT['error']=traceback.format_exc();print(REPORT['error'],flush=True)
 if OPEN:
  try:save('failure-state',{'view':view(),'render':snapshot()});screenshot('failure')
  except BaseException:pass
finally:
 if OPEN:
  try:browser('close')
  except BaseException:REPORT['passed']=False
 if SERVER:
  (OUT/'stop').write_text('stop')
  try:SERVER.wait(timeout=20)
  except subprocess.TimeoutExpired:SERVER.kill();SERVER.wait();REPORT['passed']=False
  if SERVER.returncode:REPORT['passed']=False
 if LOG:LOG.close()
 REPORT['cleaned']=[]
 for path in [TEMP,SOCKETS,OUT/'data']:
  if path.exists():shutil.rmtree(path);REPORT['cleaned'].append(str(path.relative_to(ROOT)))
 for name in ['url','stop']:(OUT/name).unlink(missing_ok=True)
 REPORT['artifacts']=[{'path':p.name,'bytes':p.stat().st_size,'sha256':hashlib.sha256(p.read_bytes()).hexdigest()} for p in sorted(OUT.iterdir()) if p.is_file() and p.suffix in ['.png','.json']]
 (OUT/'report.json').write_text(json.dumps(REPORT,ensure_ascii=False,indent=2)+'\n');print(OUT/'report.json',flush=True)
 if not REPORT['passed']:raise SystemExit(1)
