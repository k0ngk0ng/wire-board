#!/usr/bin/env python3
"""Real HTTP/browser acceptance. Failure inventory: racing-remake/PLAN.md."""
import argparse,base64,hashlib,http.cookiejar,json,os,shutil,subprocess,time,traceback,urllib.request,urllib.error,uuid
from pathlib import Path
parser=argparse.ArgumentParser();parser.add_argument('--out',required=True);parser.add_argument('--maps-only',action='store_true');parser.add_argument('--skip-maps',action='store_true');parser.add_argument('--tracks',default='metro,harbor,woodland,coast,canyon');parser.add_argument('--models',action='store_true');parser.add_argument('--fallback',action='store_true');parser.add_argument('--players',type=int,default=2);parser.add_argument('--extended',action='store_true');args=parser.parse_args()
root=Path(__file__).resolve().parents[2];out=(root/args.out).resolve();assert out.is_relative_to(root) and not out.exists();out.mkdir(parents=True)
run='rc'+uuid.uuid4().hex[:5];temp=out/'temporary';sockets=root/'.s'/run;env=dict(os.environ)
for k,v in {'TMPDIR':temp/'tmp','GOTMPDIR':temp/'tmp','GOCACHE':root/'.local/go-build','GOMODCACHE':root/'.local/go-mod','npm_config_cache':root/'.local/npm','AGENT_BROWSER_SOCKET_DIR':sockets,'AGENT_BROWSER_PROFILE':temp/'profile'}.items():v.mkdir(parents=True,exist_ok=True);env[k]=str(v)
env.update(AGENT_BROWSER_SESSION=run,AGENT_BROWSER_ARGS='--mute-audio,--disable-breakpad,--disable-crash-reporter,--enable-unsafe-swiftshader',WIRE_BOARD_BUBBLEPARTY_BROWSER='1',WIRE_BOARD_BUBBLEPARTY_BROWSER_OUT=str(out),WIRE_BOARD_REALTIME_NETWORK_CONTROL='1',GOTOOLCHAIN='local')
if args.fallback:env['AGENT_BROWSER_ARGS']+=',--disable-webgl'
report={'passed':False,'checks':[],'screenshots':[],'scope':'production registry/HTTP/SQLite/clock, native CDP keyboard/touch; no game-state injection','distSHA256':hashlib.sha256((root/'web/dist/index.html').read_bytes()).hexdigest()};server=log=None;opened=False

def browser(*a):
 r=subprocess.run(['agent-browser',*a,'--json'],cwd=root,env=env,text=True,capture_output=True,timeout=45);assert r.returncode==0,r.stdout+r.stderr;j=json.loads(r.stdout);assert j['success'],j;return j['data']
def js(code):return browser('eval',code)['result']
def api(path,body=None):
 opts='' if body is None else ',{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify('+json.dumps(body)+')}'
 r=js('(async()=>{const r=await fetch('+json.dumps('/api'+path)+opts+');return {status:r.status,data:await r.json()}})()');assert r['status'] in (200,201),r;return r['data']
def room():return api('/state')['room']
def cmd(kind,**extra):
 r=room();return api('/rooms/'+r['id'],dict(type=kind,version=r['version'],nonce=uuid.uuid4().hex,**extra))
def wait(fn,seconds=20):
 end=time.monotonic()+seconds
 while time.monotonic()<end:
  if value:=fn():return value
  time.sleep(.1)
 raise AssertionError('condition timed out')
def check(name,**detail):report['checks'].append(dict(name=name,**detail));print('PASS',name,flush=True)
def shot(name):
 if js('!!document.querySelector("[role=dialog]")'):
  wait(lambda:js('(()=>{const e=document.querySelector("[role=dialog]");return e&&[e,e.parentElement].every(n=>Number(getComputedStyle(n).opacity)>.99&&n.getAnimations().every(a=>a.playState==="finished"))})()'))
 browser('screenshot',str(out/(name+'.png')),'--full');report['screenshots'].append(name+'.png')
def record(name,j):(out/(name+'.json')).write_text(json.dumps(j,ensure_ascii=False,indent=2)+'\n')
def native(name,actions,width=1280,touch=False):
 result=subprocess.run(['node','scripts/racing/chase-input.mjs',browser('get','cdp-url')['cdpUrl'],base,json.dumps({'actions':actions,'width':width,'height':844 if touch else 1000,'touch':touch})],cwd=root,env=env,text=True,capture_output=True,timeout=45);assert result.returncode==0,result.stdout+result.stderr;j=json.loads(result.stdout)
 for image in j.pop('screenshots'):(out/(image['name']+'.png')).write_bytes(base64.b64decode(image['data']));report['screenshots'].append(image['name']+'.png')
 record(name,j);return {s['name']:s['data'] for s in j['samples']},j

def contrast(name):
 ratios=js(r"""(()=>{
 const rgb=s=>(s.match(/[\d.]+/g)||[]).map(Number);
 const lum=c=>c.slice(0,3).map(v=>v/255).map(v=>v<=.04045?v/12.92:((v+.055)/1.055)**2.4).reduce((a,v,i)=>a+v*[.2126,.7152,.0722][i],0);
 const bg=e=>{let b=[255,255,255],chain=[];for(let n=e;n;n=n.parentElement)chain.unshift(n);for(const n of chain){const c=rgb(getComputedStyle(n).backgroundColor),a=c[3]??1;b=b.map((v,i)=>v*(1-a)+(c[i]??0)*a)}return b};
 return [...document.querySelectorAll('.racing-hud span,.racing-hud strong,.racing-status,.racing-view-controls button,.racing-view-controls span,.racing-controls button,.racing-controls small,.racing-wheel span,.racing-settings label,.racing-settings select')].filter(e=>e.getClientRects().length).map(e=>{const s=getComputedStyle(e),a=lum(rgb(s.color)),b=lum(bg(e));return {text:e.textContent.trim(),foreground:s.color,background:bg(e),opacity:s.opacity,ratio:(Math.max(a,b)+.05)/(Math.min(a,b)+.05)}})
 })()""")
 record(name,ratios);assert ratios and all(r['ratio']>=4.5 and r['opacity']=='1' for r in ratios),ratios;check(name)

def map_gestures():
 for width,mode in [(1280,'camera'),(320,'pinch')]:
  browser('set','viewport',str(width),'1000' if width==1280 else '844');browser('click','button[aria-label="查看赛道地图"]');browser('wait','canvas[aria-label="赛车赛道"]')
  target=out/('map-'+mode+'.json');r=subprocess.run(['node','scripts/racing/native-input.mjs',browser('get','cdp-url')['cdpUrl'],base,mode,str(target)],cwd=root,env=env,text=True,capture_output=True,timeout=45);assert r.returncode==0,r.stderr
  samples=[s['canvas'] for s in json.loads(target.read_text())['samples']];assert float(samples[1]['cameraScale'])>float(samples[0]['cameraScale'])
  if mode=='camera':
   before=float(samples[0]['cameraX'])+float(samples[0]['width'])*.1/float(samples[0]['cameraScale']);after=float(samples[1]['cameraX'])+float(samples[1]['width'])*.1/float(samples[1]['cameraScale'])
   assert abs(after-before)<1 and abs(float(samples[1]['cameraY'])-float(samples[0]['cameraY']))<1
   assert samples[2]['cameraX']!=samples[1]['cameraX'] and float(samples[4]['cameraX'])>float(samples[3]['cameraX'])+50
  shot('map-'+mode);browser('press','Escape');wait(lambda:not js('!!document.querySelector("[role=dialog]")'));check('native map '+mode+' gestures')

def close():
 if room()['status']!='finished':cmd('close')
 cmd('leave');wait(lambda:js('!!document.querySelector(".room-list")'))
try:
 log=(out/'server.log').open('w');server=subprocess.Popen(['go','test','-count=1','-timeout=15m','-run','^TestBubblepartyBrowserFixture$','./games/bubbleparty/tests'],cwd=root,env=env,stdout=log,stderr=subprocess.STDOUT)
 wait(lambda:(out/'url').exists(),120);base=(out/'url').read_text();browser('open',base);opened=True;browser('set','viewport','1280','1000')
 api('/login',{'name':'泡泡验收管理员','password':'bubbleparty-browser-test-only'});browser('reload');browser('wait','.room-list')
 # Reproduce the reported waiting-room map switch with an already-ready bot.
 for name in ([] if args.skip_maps else ['waterpark','forest','desert']):
  api('/rooms',{'kind':'bubbleparty','name':'换地图与电脑开局验收','capacity':2,'options':{'map':'community10','roundSeconds':180}});cmd('add_bot');cmd('ready');browser('wait','.waiting-layout')
  browser('select','select[aria-label="水泡战场地图"]',name);browser('find','role','button','click','--name','保存并准备','--exact')
  wait(lambda:room()['options']['map']==name);r=room();assert r['seats'][1]['bot'] and r['seats'][1]['ready'] and r['seats'][0]['ready'],r['seats']
  wait(lambda:js('[...document.querySelectorAll("button")].some(b=>b.textContent.trim()==="开始游戏"&&!b.disabled)'));browser('find','role','button','click','--name','开始游戏','--exact')
  wait(lambda:room().get('game',{}).get('data',{}).get('phase')=='playing');assert room()['game']['data']['mapId']==name;shot('bubble-'+name);check('bubble '+name+' map switch, bot ready, actual UI start')
  if name=='waterpark':
   browser('set','viewport','320','844');js('document.querySelector(".bubble-controls").scrollIntoView({block:"center"})')
   samples,trace=native('bubble-joystick',[{'touch':'touchStart','points':[{'selector':'[data-bubbleparty-dpad]','id':1}]},{'wait':150},{'read':'center'},{'touch':'touchMove','points':[{'selector':'[data-bubbleparty-dpad]','id':1,'x':.85}]},{'wait':200},{'read':'moving'},{'touch':'touchStart','points':[{'selector':'[data-bubbleparty-dpad]','id':1,'x':.85},{'selector':'[data-bubbleparty-bomb]','id':2}]},{'wait':100},{'read':'bomb'},{'touch':'touchCancel','points':[]},{'wait':250},{'read':'released'},{'shot':'bubble-joystick-320'}],320,True)
   center=samples['center']['room']['game']['data']['players'][0];moving=samples['moving']['room']['game']['data']['players'][0];released=samples['released']['room']['game']['data']['players'][0]
   assert center['applied']['move']=='none' and moving['px']>center['px'] and released['applied']['move']=='none',(center,moving,released)
   assert any(p.get('type')=='input' and p['action'].get('bomb') and p['action'].get('move')=='right' for p in trace['packets']);assert not samples['released']['selected'];check('320px joystick dead zone, drag, second-finger bomb and cancellation')
  close();browser('set','viewport','1280','1000')
 if not args.maps_only:
  for track in args.tracks.split(','):
   if args.extended and track=='metro':
    browser('click','.lobby-create');browser('wait','[role=dialog]');browser('fill','input[aria-label="搜索可选游戏"]','极品飞车');browser('click','.choose-games button');browser('wait','select[aria-label="赛道"]')
    assert js('document.querySelector(".create-selected-game strong").textContent')=='极品飞车'
    assert set(js('[...document.querySelectorAll("select[aria-label=赛道] option")].map(e=>e.value)'))=={'metro','harbor','woodland','coast','canyon'}
    browser('select','select[aria-label="座位上限"]',str(args.players));browser('select','select[aria-label="比赛圈数"]','1');browser('fill','input[name=name]','极品飞车完整验收');shot('create-classic-racer');browser('click','[role=dialog] button[type=submit]');browser('wait','.waiting-layout');check('shared create flow, renamed game and all five track choices')
    browser('set','viewport','320','844')
   else:
    api('/rooms',{'kind':'racing','name':'极品飞车追逐视角验收','capacity':args.players,'options':{'track':track,'laps':1}})
   for _ in range(args.players-1):cmd('add_bot')
   cmd('ready');cmd('start');browser('wait','.racing-board')
   wait(lambda:room().get('game',{}).get('data',{}).get('phase')=='racing');wait(lambda:js('document.querySelector(".racing-chase")?.dataset.renderer==='+json.dumps('canvas' if args.fallback else 'webgl')),60)
   if args.extended and track=='metro':
    layout=js('(()=>{const selectors=[".racing-hud",".racing-chase",".racing-view-controls",".racing-wheel","button[aria-label=油门]","button[aria-label=手刹]","button[aria-label=刹车]"];return {width:innerWidth,height:innerHeight,scrollY,parts:selectors.map(selector=>{const r=document.querySelector(selector).getBoundingClientRect();return {selector,left:r.left,top:r.top,right:r.right,bottom:r.bottom}})}})()')
    assert layout['width']==320 and all(p['left']>=0 and p['right']<=320 and p['top']>=0 and p['bottom']<=layout['height'] for p in layout['parts']),layout
    record('initial-mobile-layout',layout);browser('screenshot',str(out/'initial-mobile-320.png'));report['screenshots'].append('initial-mobile-320.png');check('six-seat mobile first view contains scene and controls without test scrolling',layout=layout)
    browser('click','button[aria-label="车手与房间"]');wait(lambda:js('scrollY<=1'));time.sleep(.4);assert js('scrollY<=1');check('return to public players is not undone by snapshots')
   browser('set','viewport','1280','1000');js('document.querySelector(".racing-chase").scrollIntoView({block:"center"})');time.sleep(.7);shot(track+'-desktop')
   pose=js('JSON.parse(document.querySelector(".racing-chase").dataset.camera)');check(track+' '+('canvas' if args.fallback else 'WebGL')+' chase',camera=pose,fps=js('document.querySelector(".racing-chase").dataset.fps'))
   if track=='metro':
    contrast('desktop-contrast')
    if args.models:
     for skin in range(6):
      browser('select','.racing-settings select',str(skin));wait(lambda:room()['game']['data']['cars'][0]['skin']==skin);js('document.querySelector(".racing-chase").scrollIntoView({block:"center"})');time.sleep(.2);shot('model-'+str(skin)+'-rear')
      browser('focus','.racing-chase');native('model-'+str(skin)+'-front',[{'key':'b','down':True},{'wait':150},{'shot':'model-'+str(skin)+'-front'},{'key':'b','down':False}])
     check('six classic models selected through actual UI, front and rear screenshots')
    samples,trace=native('keyboard-rear',[{'key':'w','down':True},{'wait':450},{'motion':True},{'read':'moving'},{'key':'b','down':True},{'wait':150},{'read':'rear'},{'shot':'metro-rear'},{'key':'b','down':False},{'wait':150},{'read':'front'},{'key':'w','down':False},{'key':' ','down':True},{'wait':150},{'read':'handbrake'},{'key':' ','down':False},{'wait':150}])
    assert samples['rear']['canvas']['rear']=='true' and samples['front']['canvas']['rear']=='false';assert samples['handbrake']['controls']['drift']=='true';assert samples['after']['controls']['drift']=='false';assert trace['packets'];check('native W + B rear view and Space handbrake')
    assert any(a['snapshot']==b['snapshot'] and ((a['x']-b['x'])**2+(a['y']-b['y'])**2)>.01 for a,b in zip(trace['motion'],trace['motion'][1:])),trace['motion'];check('opponents move smoothly between authoritative snapshots')
    samples,_=native('reset',[{'key':'s','down':True},{'wait':1200},{'key':'s','down':False},{'wait':150},{'key':'r','down':True},{'key':'r','down':False},{'wait':200},{'read':'reset'}]);c=samples['reset']['room']['game']['data']['cars'][0];assert c['resetUntil']>samples['reset']['room']['game']['data']['serverAt'];check('native R authoritative reset',car=c)
    samples,_=native('reconnect-held-view',[{'key':'w','down':True},{'key':'b','down':True},{'wait':200},{'disconnect':True},{'wait':2000},{'read':'reconnected'},{'key':'w','down':True,'repeat':True},{'key':'b','down':True,'repeat':True},{'wait':250},{'read':'repeated'},{'key':'w','down':False},{'key':'b','down':False}])
    assert samples['reconnected']['canvas']['rear']=='false' and samples['repeated']['canvas']['rear']=='false',{k:{'rear':v['canvas']['rear'],'controls':v['controls']} for k,v in samples.items()}
    assert samples['repeated']['controls']['throttle']=='0',samples['repeated']['controls'];check('actual socket reconnect clears rear view and ignores stale key repeats')
    samples,_=native('reduced-motion',[{'reduced':True},{'key':'w','down':True},{'wait':500},{'read':'moving'},{'wait':250},{'read':'faster'},{'key':'w','down':False},{'reduced':False}]);assert float(samples['moving']['canvas']['speed'])>10;assert json.loads(samples['moving']['canvas']['camera'])['fov']==json.loads(samples['faster']['canvas']['camera'])['fov']==64;check('reduced motion keeps speed-based camera effects off')
    browser('set','viewport','320','844');js('document.querySelector(".racing-controls").scrollIntoView({block:"center"})');shot('metro-controls-320')
    samples,_=native('touch-rear',[{'scrollTo':'.racing-view-controls'},{'touch':'touchStart','points':[{'selector':'button[aria-label="向后看"]','id':1}]},{'wait':700},{'read':'rear'},{'touch':'touchEnd','points':[]},{'wait':150},{'read':'released'}],320,True)
    assert samples['rear']['canvas']['rear']=='true' and samples['released']['canvas']['rear']=='false';assert not samples['released']['selected'];check('320px long touch rear view releases without text selection')
    browser('click','button[aria-label="查看赛道地图"]');browser('wait','[role=dialog]');shot('metro-map-320');browser('press','Escape');wait(lambda:not js('!!document.querySelector("[role=dialog]")'));check('shared map dialog opens and closes')
    assert js('document.activeElement?.classList.contains("racing-chase")'), 'map close left keyboard focus on the map button'
    if args.extended:
     contrast('mobile-contrast');map_gestures()
     epoch=room()['game']['data']['epoch'];cmd('autoplay',enabled=True);wait(lambda:room()['seats'][0].get('autoPlay'));assert js('document.querySelector("button[aria-label=油门]").disabled');time.sleep(.3);cmd('autoplay',enabled=False);wait(lambda:not room()['seats'][0].get('autoPlay'));assert room()['game']['data']['epoch']==epoch;check('manual takeover and reclaim keep the same race')
     cmd('autoplay',enabled=True)
     owner=browser('cookies','get')['cookies'];watcher=urllib.request.build_opener(urllib.request.HTTPCookieProcessor(http.cookiejar.CookieJar()))
     def watch(path,body=None):
      request=urllib.request.Request(base+'/api'+path,data=None if body is None else json.dumps(body).encode(),headers={'Content-Type':'application/json'})
      with watcher.open(request,timeout=15) as response:return json.load(response)
     watch('/register',{'name':'追车视角观众','password':'racing-watch-test','invite':'bubbleparty-browser'});watch('/rooms/'+room()['id']+'/watch',{})
     browser('cookies','clear')
     for cookie in watcher.handlers:
      if isinstance(cookie,urllib.request.HTTPCookieProcessor):
       for c in cookie.cookiejar:browser('cookies','set',c.name,c.value,'--url',base,'--httpOnly')
     browser('reload');browser('wait','.racing-board');wait(lambda:room().get('spectating'));assert not room()['game']['data']['controls']['canDrive']
     _,spectator=native('spectator-readonly',[{'key':'w','down':True},{'key':'b','down':True},{'wait':300},{'read':'rear'},{'key':'w','down':False},{'key':'b','down':False}],320,True)
     assert not any(p['type']=='input' for p in spectator['packets']);shot('spectator-320');check('real separate spectator can look back but cannot drive')
     browser('cookies','clear')
     for c in owner:browser('cookies','set',c['name'],c['value'],'--url',base,'--httpOnly')
     browser('reload');browser('wait','.racing-board');wait(lambda:not room().get('spectating'))
     browser('click','button[aria-label="查看赛道地图"]');browser('wait','[role=dialog][aria-label="赛道地图"]')
     print('Waiting for real natural race finish with all seats controlled',flush=True)
     wait(lambda:js('document.querySelector(".racing-board")?.dataset.phase==="finished"'),150)
     wait(lambda:js('!!document.querySelector(".match-result-modal")&&!document.querySelector("[aria-label=赛道地图]")'))
     assert js('!!document.activeElement?.closest(".match-result-modal")');shot('real-result-320');check('real finish closes map and preserves result modal focus',result=room()['game'].get('winners'))
     browser('click','.match-result-modal button[aria-label="关闭"]');wait(lambda:not js('!!document.querySelector(".match-result-modal")'));time.sleep(.5);assert not js('!!document.querySelector(".match-result-modal")');shot('final-scene-320');check('closed result stays closed with final scene visible')

   browser('set','viewport','320','844');js('document.querySelector(".racing-chase").scrollIntoView({block:"center"})');time.sleep(.3);shot(track+'-320');assert js('document.documentElement.scrollWidth<=innerWidth+2');check(track+' 320px visible, no horizontal overflow');close()
 report['passed']=True
except BaseException:
 report['error']=traceback.format_exc();print(report['error'],flush=True)
 if opened:
  try:shot('failure')
  except BaseException:pass
finally:
 if opened:
  try:browser('close')
  except BaseException:report['passed']=False
 if server:
  (out/'stop').write_text('stop')
  try:server.wait(timeout=20)
  except subprocess.TimeoutExpired:server.kill();server.wait();report['passed']=False
  if server.returncode!=0:report['passed']=False
 if log:log.close()
 for path in [temp,sockets,out/'data']:
  if path.exists():shutil.rmtree(path)
 for name in ['url','stop']:(out/name).unlink(missing_ok=True)
 record('report',report);print(out/'report.json',flush=True)
 if not report['passed']:raise SystemExit(1)
