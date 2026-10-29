#!/usr/bin/env python3
"""Physical gamepad browser acceptance, see SCENARIOS.md. Build web first."""
import argparse, math, hashlib, http.cookiejar, json, os, re, shutil, subprocess, time, traceback, urllib.request, uuid
from pathlib import Path
parser=argparse.ArgumentParser();parser.add_argument('--out',required=True);parser.add_argument('--settings-only',action='store_true');parser.add_argument('--games',default='');parser.add_argument('--contra',action='store_true');parser.add_argument('--junqi-teams',action='store_true');parser.add_argument('--catan-explorer',action='store_true');args=parser.parse_args()
root=Path(__file__).resolve().parents[3];out=(root/args.out).resolve();assert out.is_relative_to(root) and not out.exists();out.mkdir(parents=True)
run='pad'+uuid.uuid4().hex[:6];temp=out/'temporary';sockets=root/'.s'/run;env=dict(os.environ)
for k,v in {'TMPDIR':temp/'tmp','GOTMPDIR':temp/'tmp','GOCACHE':root/'.local/go-build','GOMODCACHE':root/'.local/go-mod','npm_config_cache':root/'.local/npm','AGENT_BROWSER_SOCKET_DIR':sockets,'AGENT_BROWSER_PROFILE':temp/'profile'}.items():v.mkdir(parents=True,exist_ok=True);env[k]=str(v)
env['AGENT_BROWSER_INIT_SCRIPTS']=str(root/'scripts/controllers/gamepad/virtual-device.js')
env.update(AGENT_BROWSER_SESSION=run,AGENT_BROWSER_ARGS='--mute-audio,--disable-breakpad,--disable-crash-reporter',WIRE_BOARD_BUBBLEPARTY_BROWSER='1',WIRE_BOARD_BUBBLEPARTY_BROWSER_OUT=str(out),GOTOOLCHAIN='local')
fixture_dir=out/'fixture'
web_dir=root/'.local/gamepad/web-dist'
env.update(WIRE_BOARD_GAMEPAD_BROWSER='1',WIRE_BOARD_GAMEPAD_BROWSER_OUT=str(fixture_dir),WIRE_BOARD_GAMEPAD_BROWSER_WEB=str(web_dir))
if args.contra:env['WIRE_BOARD_GAMEPAD_CONTRA_PUBLIC']=str(root/'.local/gamepad/contra-build/public')
report={'passed':False,'games':{},'command':'python3 -B scripts/controllers/gamepad/check-browser.py --out '+args.out+(' --settings-only' if args.settings_only else '')+(' --contra' if args.contra else '')+(' --games '+args.games if args.games else '')+(' --junqi-teams' if args.junqi_teams else '')+(' --catan-explorer' if args.catan_explorer else ''),'buildIndexSHA256':hashlib.sha256((web_dir/'index.html').read_bytes()).hexdigest()}
primary_model='game:contra' if args.contra else 'game:basketball'
other_model='game:splendor' if args.contra else 'game:tabletennis'
server=log=None;opened=False;watch_opened=False;sg_watchers=[]
watch_env={**env,"AGENT_BROWSER_SESSION":run+"w","AGENT_BROWSER_PROFILE":str(temp/"observer-profile")}

def browser(*a):
 r=subprocess.run(['agent-browser',*a,'--json'],cwd=root,env=env,text=True,capture_output=True,timeout=45);assert r.returncode==0,r.stdout+r.stderr
 j=json.loads(r.stdout);assert j['success'],j;return j['data']
def watch_browser(*a):
 r=subprocess.run(['agent-browser',*a,'--json'],cwd=root,env=watch_env,text=True,capture_output=True,timeout=45);assert r.returncode==0,r.stdout+r.stderr
 data=json.loads(r.stdout);assert data['success'],data;return data['data']
def watch_js(code):return watch_browser('eval',code)['result']
def watch_api(path,body):
 r=watch_js('(async()=>{const r=await fetch('+json.dumps('/api'+path)+',{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify('+json.dumps(body)+')});return {status:r.status,data:await r.json()}})()');assert r['status'] in (200,201),r;return r['data']
def js(code):return browser('eval',code)['result']
def api(path,body=None):
 opts='' if body is None else ',{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify('+json.dumps(body)+')}'
 r=js('(async()=>{const r=await fetch('+json.dumps('/api'+path)+opts+');return {status:r.status,data:await r.json()}})()');assert r['status'] in (200,201),r;return r['data']
def room():return api('/state').get('room')
def cmd(kind,**extra):
 for _ in range(8):
  r=room();body=dict(type=kind,version=r['version'],nonce=uuid.uuid4().hex,**extra)
  response=js('(async()=>{const r=await fetch('+json.dumps('/api/rooms/'+r['id'])+',{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify('+json.dumps(body)+')});return {status:r.status,data:await r.json()}})()')
  if response['status']==409:continue
  assert response['status'] in (200,201),response
  return response['data']
 raise AssertionError('room command kept conflicting')
def wait(fn,seconds=20):
 end=time.monotonic()+seconds
 while time.monotonic()<end:
  if server is not None and server.poll() is not None:raise AssertionError('fixture exited; inspect server.log')
  if value:=fn():return value
  time.sleep(.1)
 raise AssertionError('Timed out')
def save(name,data):(out/(name+'.json')).write_text(json.dumps(data,ensure_ascii=False,indent=2)+'\n')

def shot(name):browser('screenshot',str(out/(name+'.png')))
def check(name,**extra):report['games'][name]=extra;print('PASS',name,flush=True)
def pad(buttons=None,axes=None,index=0,id='Acceptance Standard Pad',mapping='standard'):
 js('window.__setGamepad('+','.join(map(json.dumps,[index,id,mapping,buttons or {},axes or {}]))+');true')
def native_key(down,code='KeyJ',key='j',vk=74,modifiers=0):
 p=subprocess.run(['node','scripts/controllers/gamepad/native-key.mjs',browser('get','cdp-url')['cdpUrl'],base,json.dumps({'key':key,'code':code,'vk':vk,'down':down,'modifiers':modifiers})],cwd=root,env=env,text=True,capture_output=True,timeout=20);assert p.returncode==0,p.stderr
 report.setdefault('nativeKeys',[]).append(json.loads(p.stdout))
def key_j(down):native_key(down)
def press_native(code,key,vk,modifiers=0):
 native_key(True,code,key,vk,modifiers);time.sleep(.08);native_key(False,code,key,vk);time.sleep(.08)
def native_tap(selector):
 point=js('(()=>{const target=document.querySelector('+json.dumps(selector)+');if(!target)throw Error("Touch target missing");const nodes=[target,...target.querySelectorAll("path,line,circle,rect")];for(const node of nodes){const r=node.getBoundingClientRect(),points=[{x:r.x+r.width/2,y:r.y+r.height/2}];if(node instanceof SVGGeometryElement&&node.getTotalLength()>0)points.push(node.getPointAtLength(node.getTotalLength()/2).matrixTransform(node.getScreenCTM()));for(let i=1;i<10;i++)for(let j=1;j<10;j++)points.push({x:Math.max(0,r.left)+(Math.min(innerWidth,r.right)-Math.max(0,r.left))*i/10,y:Math.max(0,r.top)+(Math.min(innerHeight,r.bottom)-Math.max(0,r.top))*j/10});for(const p of points)if(p.x>0&&p.x<innerWidth&&p.y>0&&p.y<innerHeight&&target.contains(document.elementFromPoint(p.x,p.y)))return {x:p.x,y:p.y,id:1}}throw Error("No visible touch hit for "+target.outerHTML.slice(0,120))})()')
 result=subprocess.run(['node','scripts/controllers/gamepad/native-touch.mjs',browser('get','cdp-url')['cdpUrl'],base,json.dumps([{'type':'touchStart','touchPoints':[point],'wait':80},{'type':'touchEnd','touchPoints':[],'wait':150}]),selector],cwd=root,env=env,text=True,capture_output=True,timeout=20);assert result.returncode==0,result.stderr
 report.setdefault('nativeTouches',[]).append({'selector':selector,'point':point,'result':json.loads(result.stdout)})
def settings():
 browser('scrollintoview','button[aria-label="控制器设置"]');js('new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r)))')
 browser('click','button[aria-label="控制器设置"]');browser('wait','[data-controller-model]');browser('select','[data-controller-model]',primary_model);browser('select','select[aria-label="输入来源"]','gamepad')
def binding(name):
 if js('!!document.querySelector(".gamepad-map-details:not([open])")'):browser('click','.gamepad-map-details summary')
 browser('click','[data-gamepad-binding="'+name.lower()+'"] .controller-key')
def save_settings():browser('click','[data-controller-save]');browser('wait','--text','控制器设置已保存')
def close_settings():browser('click','.controller-settings button[aria-label="关闭"]')
try:
 log=(out/'server.log').open('w');server=subprocess.Popen(['go','test','-count=1','-timeout=12m','-run','^TestControllerGamepadBrowserFixture$','./internal/gametest'],cwd=root,env=env,stdout=log,stderr=subprocess.STDOUT)
 wait(lambda:(fixture_dir/'url').exists(),120);base=(fixture_dir/'url').read_text();browser('open',base);opened=True
 credentials={'name':'手柄验收管理员','password':'gamepad-browser-test-only'}
 api('/login',credentials);browser('reload');browser('wait','.room-list');browser('set','viewport','1280','1000')
 assert js('document.querySelectorAll(".header-right .profile-nav").length===1&&document.querySelectorAll(".header-right .username button").length===0')
 browser('click','[aria-label="我的战绩与好友"]');browser('wait','[role="dialog"][aria-label="玩家页面"]');browser('wait','--text','好友与申请');browser('click','[role="dialog"] button[aria-label="关闭"]')
 wait(lambda:not js('!!document.querySelector("[role=dialog]")'));check('profile-menu',singleEntry=True,opensProfile=True)
 settings();browser('wait','--text','连接 USB');pad();browser('wait','--text','Acceptance Standard Pad')
 binding('A');pad({'4':1});browser('wait','--fn','!document.querySelector("[data-controller-save]").disabled');pad({'4':0});save_settings()
 p=api('/controllers')['profiles'][primary_model];assert p['gamepads']['standard:Acceptance Standard Pad']['bindings']['a']==[{'type':'button','index':4}],p
 assert p['keyboard']['a']=='KeyK';shot('settings-desktop')
 binding('A');pad({'0':1});browser('wait','--text','此输入已用于');assert js('document.querySelector("[data-controller-save]").disabled');pad({'0':0});browser('press','Escape')
 browser('select','[data-controller-model]',other_model);browser('wait','.gamepad-bindings')
 binding('confirm' if args.contra else 'A');pad({'17':1});browser('wait','--fn','!document.querySelector("[data-controller-save]").disabled');pad({'17':0});save_settings()
 browser('select','[data-controller-model]',primary_model);assert api('/controllers')['profiles'][primary_model]['gamepads']['standard:Acceptance Standard Pad']['bindings']['a'][0]['index']==4
 js('window.__disconnectGamepad(0);true');pad(index=1,id='Generic USB Adapter',mapping='');browser('wait','--text','此手柄没有标准映射')
 binding('B');pad({'5':1},index=1,id='Generic USB Adapter',mapping='');browser('wait','--fn','!document.querySelector("[data-controller-save]").disabled');pad({'5':0},index=1,id='Generic USB Adapter',mapping='');save_settings()
 assert api('/controllers')['profiles'][primary_model]['gamepads']['raw:Generic USB Adapter']['bindings']['b'][0]['index']==5
 browser('set','viewport','320','740');browser('scrollintoview','select[aria-label="使用手柄"]');shot('settings-320-top');browser('scrollintoview','[data-controller-save]');shot('settings-320-save')
 assert js('document.documentElement.scrollWidth<=innerWidth+1')
 close_settings();browser('reload');settings();assert api('/controllers')['profiles'][primary_model]['gamepads']['standard:Acceptance Standard Pad']['bindings']['a'][0]['index']==4;close_settings()
 check('settings',standardAndRaw=True,buttonCapture=True,conflict=True,modelIsolation=True,deviceIsolation=True,reload=True,mobile320=True)
 if not args.settings_only:
  browser('set','viewport','1280','1000');pad();
  for kind in (args.games.split(',') if args.games else ['contra'] if args.contra else ['basketball','tabletennis']):
   capacity=4 if kind in ('doudizhu','guandan','shengji','sanguosha') else 2 if kind in ('tabletennis','gomoku','splendor','bubbleparty','battlecity','spf2t','fortressblue','chess','xiangqi','weiqi','racing','monopoly','junqi','carcassonne','rail','catan','conquest','snooker') else 1
   if kind=='junqi' and args.junqi_teams:capacity=4
   api('/rooms',{'kind':kind,'name':'实体手柄验收 '+kind,'capacity':capacity,'options':({'mapId':'china','turnSeconds':180,'difficulty':'easy','fog':False} if kind=='conquest' else {'mode':'campaign','waves':10,'difficulty':'easy','prepSeconds':45} if kind=='starfort' else {}),**({'catanScenario':'pirate-lairs'} if kind=='catan' and args.catan_explorer else {})})
   browser('wait','.waiting-layout');waiting_menu=js('[...document.querySelectorAll(".room-tools > button")].map(b=>b.textContent.trim())');assert waiting_menu[-1]=='离开房间' and '全屏' not in waiting_menu,waiting_menu
   if kind=='sanguosha':
    for index in range(1,4):
     watch_env={**env,'AGENT_BROWSER_SESSION':run+'s'+str(index),'AGENT_BROWSER_PROFILE':str(temp/('sg-opponent-'+str(index)))}
     watch_browser('open',base);sg_watchers.append(watch_env)
     watch_api('/register',{'name':'三国对手'+str(index)+run,'password':'gamepad-viewer-only','invite':'gamepad-browser'})
     joining=room();rid=joining['id'];watch_api('/rooms/'+rid,{'type':'join','version':joining['version'],'nonce':uuid.uuid4().hex});watch_browser('reload');watch_browser('wait','.waiting-layout')
     wr=watch_js('(async()=>await(await fetch("/api/state")).json())()')['room'];watch_api('/rooms/'+rid,{'type':'ready','version':wr['version'],'nonce':uuid.uuid4().hex})
   elif kind in ('bubbleparty','battlecity','spf2t','fortressblue','chess','xiangqi','weiqi','racing','monopoly','junqi','carcassonne','rail','catan','conquest','snooker'):
    watch_env.update(AGENT_BROWSER_SESSION=run+kind[:3],AGENT_BROWSER_PROFILE=str(temp/('opponent-'+kind)))
    watch_browser('open',base);watch_opened=True
    watch_api('/register',{'name':'对手'+kind[:3]+run,'password':'gamepad-viewer-only','invite':'gamepad-browser'})
    joining=room();rid=joining['id'];watch_api('/rooms/'+rid,{'type':'join','version':joining['version'],'nonce':uuid.uuid4().hex});watch_browser('reload');watch_browser('wait','.waiting-layout')
    wr=watch_js('(async()=>await(await fetch("/api/state")).json())()')['room'];watch_api('/rooms/'+rid,{'type':'ready','version':wr['version'],'nonce':uuid.uuid4().hex})
    for _ in range(capacity-2):cmd('add_bot')
   else:
    for _ in range(capacity-1):cmd('add_bot')
   cmd('ready');cmd('start');wait(lambda:js('!!document.querySelector(".game-main")&&!document.querySelector(".game-loading")'),90)
   menu=js('[...document.querySelectorAll(".room-tools > button")].map(b=>b.textContent.trim())');assert menu[0]=='全屏',menu
   if not args.contra and kind=='basketball':
    watch_browser('open',base);watch_opened=True
    watch_api('/register',{'name':'手柄观战'+run,'password':'gamepad-viewer-only','invite':'gamepad-browser'})
    watch_api('/rooms/'+room()['id']+'/watch',{});watch_browser('reload');watch_browser('wait','.room-tools')
    watch_menu=watch_js('[...document.querySelectorAll(".room-tools > button")].map(b=>b.textContent.trim())');assert watch_menu[0]=='全屏' and watch_menu[-1]=='离开观战',watch_menu
    watch_browser('screenshot',str(out/'spectator-menu.png'));watch_browser('click','.room-tools > button:last-child');watch_browser('wait','.room-list');watch_browser('close');watch_opened=False
    check('room-menu-order',waiting=waiting_menu,playing=menu,spectating=watch_menu)
   browser('focus','[data-fullscreen-board]');time.sleep(.3)
   if kind=='snooker':
    # Use the randomly drawn opening player, preserving real rules and auth.
    if room()['game']['data']['turn']!=room()['you']:
     api('/login',{'name':'对手sno'+run,'password':'gamepad-viewer-only'});browser('reload');watch_api('/login',credentials)
    browser('wait' ,'[data-controller-input="snooker"]')
    def press(button):
     js('new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r)))');js('window.__pulseGamepad(0,'+json.dumps({str(button):1})+',{},3)');js('new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r)))')
    def neutral():pad({str(i):0 for i in range(24)},{str(i):0 for i in range(4)})
    def data():return room()['game']['data']
    def ui():return js('({...document.querySelector(".snooker-table").dataset})')
    def charge_up():
     browser('focus','.snooker-table > canvas');neutral();time.sleep(.1);pad({'0':1,'12':1});wait(lambda:float(ui()['chargePower'])>.04);pad({'0':1,'12':0})
    before=data()
    def opponent_action(action):
     wr=watch_js('(async()=>await(await fetch("/api/state")).json())()')['room'];return watch_api('/rooms/'+wr['id'],{'type':'action','version':wr['version'],'nonce':uuid.uuid4().hex,'action':{**action,'window':wr['game']['data']['window']}})
    def shot_id():return (data().get('flight') or {}).get('id',0)
    baseline_shot=shot_id()
    wait(lambda:data()['phase']=='aim' and data()['controls']['canShoot']);browser('focus','.snooker-table > canvas');neutral();time.sleep(.15)
    if ui()['placing']!='true':press(2)
    cursor_before=ui();pad({'12':1});time.sleep(.15);pad({'12':0});cursor=ui();assert float(cursor['cursorY'])<float(cursor_before['cursorY']),[cursor_before,cursor]
    press(0);wait(lambda:abs(data()['balls'][0]['pos']['x']-float(cursor['cursorX']))<.002 and abs(data()['balls'][0]['pos']['y']-float(cursor['cursorY']))<.002);wait(lambda:ui()['placing']=='false')
    neutral();time.sleep(.15);angle=float(ui()['angle']);pad({'15':1});time.sleep(.16);pad({'15':0});normal=float(ui()['angle'])-angle;angle=float(ui()['angle']);pad({'15':1,'4':1});time.sleep(.16);pad({'15':0,'4':0});fine=float(ui()['angle'])-angle;assert normal>.04 and 0<fine<normal*.5,(normal,fine)
    angle=float(ui()['angle']);pad(axes={'0':.4});time.sleep(.18);pad(axes={'0':0});weak=float(ui()['angle'])-angle;angle=float(ui()['angle']);pad(axes={'0':1});time.sleep(.18);pad(axes={'0':0});strong=float(ui()['angle'])-angle;assert weak>.01 and strong>weak*1.4,(weak,strong)
    pad({'6':1,'12':1});time.sleep(.18);pad({'6':0,'12':0});assert float(ui()['spinTop'])>.02
    browser('focus','.snooker-tip-ball');press(15);assert float(ui()['spinSide'])>0;press(3);assert float(ui()['spinTop'])==0 and float(ui()['spinSide'])==0;press(1);wait(lambda:js('document.activeElement.matches(".snooker-table > canvas")'))
    charge_up();charged=ui();js('window.__disconnectGamepad(0);true');wait(lambda:ui()['charging']=='false');assert shot_id()==baseline_shot
    pad({'0':1,'12':1});time.sleep(.2);assert ui()['charging']=='false';neutral();time.sleep(.15)
    # Keyboard fire remains active when only the physical source disconnects.
    native_key(True,'Space',' ',32);pad({'0':1,'12':1});wait(lambda:float(ui()['chargePower'])>.04);js('window.__disconnectGamepad(0);true');time.sleep(.1);assert ui()['charging']=='true';press_native('Escape','Escape',27);native_key(False,'Space',' ',32);assert ui()['charging']=='false' and shot_id()==baseline_shot;neutral();time.sleep(.15)
    charge_up();settings();wait(lambda:ui()['charging']=='false');neutral();assert shot_id()==baseline_shot;browser('select','[data-controller-model]','game:snooker');browser('select','select[aria-label="输入来源"]','keyboard');browser('click','button[aria-label="修改 向右瞄准／移动 按键"]');press_native('KeyH','h',72);save_settings();browser('set','viewport','320','740');browser('scrollintoview','[data-controller-save]');shot('snooker-settings-320');assert js('document.documentElement.scrollWidth<=innerWidth+1');browser('set','viewport','1280','1000');close_settings()
    browser('focus','.snooker-table > canvas');angle=float(ui()['angle']);press_native('ArrowRight','ArrowRight',39);assert float(ui()['angle'])==angle;js('(()=>{const canvas=document.querySelector(".snooker-table > canvas"),ctx=canvas.getContext("2d"),before=ctx.getImageData(0,0,canvas.width,canvas.height).data,began=performance.now();window.__snookerResponse={began};const tick=()=>{const event=window.__nativeKeyEvents.find(e=>e.code==="KeyH"&&e.at>=began);if(event){const now=ctx.getImageData(0,0,canvas.width,canvas.height).data;if(now.some((v,i)=>v!==before[i])){window.__snookerResponse={began,event,elapsed:performance.now()-event.at,pixelsChanged:true};return}}if(performance.now()-began<2000)requestAnimationFrame(tick)};requestAnimationFrame(tick);return true})()');press_native('KeyH','h',72);assert float(ui()['angle'])>angle;latency=wait(lambda:js('window.__snookerResponse.pixelsChanged&&window.__snookerResponse'));assert latency['event']['trusted'] and latency['elapsed']<180,latency
    browser('click','.room-tools > button:first-child');browser('wait','[data-room-fullscreen]');shot('snooker-fullscreen');browser('set','viewport','320','740');browser('focus','.snooker-table > canvas');neutral();time.sleep(.15);press(2);wait(lambda:ui()['placing']=='true');portrait_before=ui();pad({'15':1});time.sleep(.15);pad({'15':0});portrait_after=ui();assert float(portrait_after['cursorY'])<float(portrait_before['cursorY']) and abs(float(portrait_after['cursorX'])-float(portrait_before['cursorX']))<.001;shot('snooker-320-placement');press(1);wait(lambda:ui()['placing']=='false');browser('scrollintoview','.snooker-power-pull');shot('snooker-320-controller')
    point=js('(()=>{const r=document.querySelector(".snooker-power-pull").getBoundingClientRect();return {x:r.x+20,y:r.y+r.height/2,id:1}})()');steps=[{'type':'touchStart','touchPoints':[point],'wait':50},{'type':'touchMove','touchPoints':[{**point,'x':point['x']+40}],'wait':100},{'type':'touchCancel','touchPoints':[],'wait':100}]
    native=subprocess.run(['node','scripts/controllers/gamepad/native-touch.mjs',browser('get','cdp-url')['cdpUrl'],base,json.dumps(steps),'.snooker-table'],cwd=root,env=env,text=True,capture_output=True,timeout=20);assert native.returncode==0,native.stderr;touch=json.loads(native.stdout);assert float(touch[1]['render']['chargePower'])>.02 and touch[-1]['render']['charging']=='false',touch;assert shot_id()==baseline_shot
    browser('press','Escape');browser('set','viewport','1280','1000');browser('scrollintoview','.snooker-table > canvas');rect=js('document.querySelector(".snooker-table > canvas").getBoundingClientRect().toJSON()');mx=round(rect['x']+rect['width']*.6);my=round(rect['y']+rect['height']*.5);browser('mouse','move',str(mx),str(my));mouse_angle=float(ui()['angle']);browser('mouse','move',str(mx),str(my-35));assert abs(float(ui()['angle'])-mouse_angle)>.01;browser('mouse','down');wait(lambda:ui()['charging']=='true');browser('mouse','move',str(mx-45),str(my-35));browser('press','Escape');browser('mouse','up');assert ui()['charging']=='false' and shot_id()==baseline_shot;browser('focus','.snooker-table > canvas');neutral();time.sleep(.15);press(3);charge_up();last_power=float(ui()['chargePower']);pad({'0':0});flight=wait(lambda:data().get('flight') if shot_id()>baseline_shot else None);assert abs(flight['strike']['power']-last_power)<.02 and flight['seat']==room()['you'],flight
    wait(lambda:data()['phase']=='foul',40);opponent_action({'type':'foul_choice','choice':'accept'});opponent_action({'type':'shoot','angle':math.pi,'power':.02});wait(lambda:data()['phase']=='foul' and data()['controls']['canAct'],40);browser('wait','.snooker-decision button:not(:disabled)');browser('focus','.snooker-decision button:not(:disabled)');press(0);wait(lambda:data()['phase']=='aim' and data()['controls']['canShoot'])
    save('snooker-controller-actions',{'before':before,'placement':cursor,'aimNormal':normal,'aimFine':fine,'analogWeak':weak,'analogStrong':strong,'nativeAimLatency':latency,'portraitCursor':{'before':portrait_before,'after':portrait_after},'cancelledCharge':charged,'touch':touch,'flight':flight,'afterFoulDecision':data()});check(kind,authoritativePlacement=True,authoritativeShot=True,foulDecision=True,aiming=True,fineAim=True,analogAim=True,spin=True,portraitPlacement=True,nativeMouseCancel=True,nativeAimResponse=latency['elapsed'],disconnectCancel=True,neutralRearm=True,mixedSources=True,settingsCancel=True,keyboardRemapping=True,fullscreen=True,nativeTouch320=True)
   elif kind in ('conquest','starfort'):
    browser('wait','[data-controller-navigation="'+kind+'"]')
    def press(button):
     js('new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r)))');js('window.__pulseGamepad(0,'+json.dumps({str(button):1})+',{},3)');js('new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r)))')
    def data():return room()['game']['data']
    def click_control(selector):
     browser('wait',selector+':not(:disabled)');browser('focus',selector);press(0)
    before=data()
    if kind=='conquest':
     browser('wait','[data-cq-province]');press(15);assert js('document.activeElement.hasAttribute("data-cq-province")');press(15);assert not js('document.querySelector("[data-conquest-board]").dataset.cqSelected')
     own=next(p['id'] for p in data()['provinces'] if p['owner']==room()['you']);focus_selector='[data-cq-province="'+own+'"]';browser('focus',focus_selector);press(0);assert js('document.querySelector("[data-conquest-board]").dataset.cqSelected')==own
     press(1);assert not js('document.querySelector("[data-conquest-board]").dataset.cqSelected');press(0)
     # Explicit town shortcut uses the same resource validation as long press.
     press(3);browser('wait','--text','这块领地已经有城镇');assert not data()['orders']
     click_control('[data-cq-action="recruit"]');browser('wait','input[data-cq-amount="number"]');browser('focus','input[data-cq-amount="number"]');amount=int(browser('get','value','input[data-cq-amount="number"]')['value']);press(12);assert int(browser('get','value','input[data-cq-amount="number"]')['value'])==amount+1
     assert not data()['orders'];click_control('[data-cq-action="confirm-recruit"]');wait(lambda:any(o['type']=='recruit' and o['amount']==amount+1 for o in data()['orders']))
     click_control('[data-cq-action="attack"]');browser('wait','input[data-cq-amount="number"]');browser('fill','input[data-cq-amount="number"]','20');browser('focus','input[data-cq-amount="number"]');press(12)
     browser('wait','.cq-province.reachable');browser('focus','.cq-province.reachable');press(0);wait(lambda:any(o['type']=='move' and o['amount']==21 for o in data()['orders']));planned=data()['orders']
     click_control('[data-cq-action="plans"]');browser('wait','[role=dialog]');before_orders=data()['orders'];notice=js('document.querySelector(".cq-map-message").textContent');press(3);assert data()['orders']==before_orders and js('document.querySelector(".cq-map-message").textContent')==notice
     for order in list(reversed(before_orders)):
      click_control('[aria-label="撤销计划 '+str(order['id'])+'"]');wait(lambda:not any(o['id']==order['id'] for o in data()['orders']))
     press(1);wait(lambda:not js('!!document.querySelector("[role=dialog]")'));browser('focus',focus_selector);press(1);press(0);click_control('[data-cq-action="fort"]');wait(lambda:any(o['type']=='build' and o['building']=='fort' for o in data()['orders']));save('conquest-recruit-march-undo',{'planned':planned,'afterUndoAndBuild':data()['orders']})
     click_control('.cq-submit');wait(lambda:data()['players'][room()['you']]['submitted']);click_control('.cq-submit');wait(lambda:not data()['players'][room()['you']]['submitted'])
     viewport_selector='.cq-viewport';zoom_selector='button[aria-label="放大地图"]';focus_attr='data-cq-province'
    else:
     browser('wait','[data-starfort-canvas]');click_control('[data-sf-pause]');wait(lambda:data()['phase']=='paused')
     press(15);assert not data()['towers'];browser('focus','[data-sf-pad]:not([hidden])');press(15);assert js('document.activeElement.hasAttribute("data-sf-pad")');press(0);browser('wait','.sf-command-dialog');press(1);wait(lambda:not js('!!document.querySelector(".sf-command-dialog")'))
     # Resume to build; one human seat controls pause without fabricated timers.
     click_control('[data-sf-resume]');wait(lambda:data()['phase']!='paused');browser('focus','[data-sf-pad]:not([hidden])');press(0);click_control('[data-sf-build="arrow"]');wait(lambda:len(data()['towers'])==1);tower=data()['towers'][0];focus_selector='[data-sf-pad="'+str(tower['pad'])+'"]'
     if js('!!document.querySelector(".sf-command-dialog")'):press(1)
     browser('focus',focus_selector);press(0);click_control('.sf-map-popover [data-sf-upgrade="level"]');wait(lambda:data()['towers'][0]['level']>=2)
     click_control('[data-sf-tower-more]');browser('wait','.sf-command-dialog');browser('focus','[data-sf-target]');press(13);wait(lambda:data()['towers'][0].get('priority')=='strong');press(0);press(1);wait(lambda:not js('!!document.querySelector(".sf-command-dialog")'))
     upgraded=data()['towers'][0];click_control('.sf-map-popover [data-sf-sell]');wait(lambda:not data()['towers']);browser('focus',focus_selector);press(0);click_control('[data-sf-build="arrow"]');wait(lambda:len(data()['towers'])==1)
     if js('!!document.querySelector(".sf-command-dialog")'):press(1)
     save('starfort-upgrade-target-sell',{'upgraded':upgraded,'rebuilt':data()['towers'][0]})
     click_control('[data-sf-pause]');wait(lambda:data()['phase']=='paused');browser('focus',focus_selector);press(0);press(1);assert not js('!!document.querySelector(".sf-map-popover")')
     viewport_selector='.sf-world-viewport';zoom_selector='[data-sf-zoom-in]';focus_attr='data-sf-pad'
    save(kind+'-controller-actions',{'before':before,'after':data()})
    settings();browser('select','[data-controller-model]','game:'+kind);browser('select','select[aria-label="输入来源"]','keyboard');browser('click','button[aria-label="修改 下一组操作 按键"]');press_native('KeyH','h',72);save_settings();close_settings()
    browser('focus',focus_selector);old=js('document.activeElement.getAttribute('+json.dumps(focus_attr)+')');press_native('PageDown','PageDown',34);assert js('document.activeElement.getAttribute('+json.dumps(focus_attr)+')')==old;press_native('KeyH','h',72);assert js('document.activeElement.getAttribute('+json.dumps(focus_attr)+')')!=old
    browser('click','.room-tools > button:first-child');browser('wait','[data-room-fullscreen]');shot(kind+'-fullscreen');browser('set','viewport','320','740')
    for _ in range(2):browser('click',zoom_selector)
    browser('focus',focus_selector);press(15)
    box=js('(()=>{const n=document.activeElement,r=n.getBoundingClientRect(),v=document.querySelector('+json.dumps(viewport_selector)+').getBoundingClientRect();return {x:r.x,y:r.y,w:r.width,h:r.height,left:Math.max(v.left,0),right:Math.min(v.right,innerWidth),top:Math.max(v.top,0),bottom:Math.min(v.bottom,innerHeight),focus:n.getAttribute('+json.dumps(focus_attr)+')}})()');assert box['focus'] is not None and box['x']+box['w']>box['left'] and box['x']<box['right'] and box['y']+box['h']>box['top'] and box['y']<box['bottom'],box
    shot(kind+'-320-controller');native_tap('[data-controller-focus]')
    if kind=='conquest':wait(lambda:js('document.querySelector("[data-conquest-board]").dataset.cqSelected')==box['focus'])
    else:wait(lambda:js('document.querySelector('+json.dumps('[data-sf-pad="'+box["focus"]+'"]')+').getAttribute("aria-pressed")')=='true')
    browser('press','Escape');browser('set','viewport','1280','1000');check(kind,serverActions=True,directionalFocus=True,cancel=True,keyboardRemapping=True,fullscreen=True,mobile320=box,nativeTouch=True)
   elif kind=='carcassonne':
    browser('wait','[data-controller-navigation="carcassonne"]')
    def press(button):
     js('new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r)))');js('window.__pulseGamepad(0,'+json.dumps({str(button):1})+',{},3)');js('new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r)))')
    def car():return room()['game']['carcassonne']
    def focus_box():
     return js('(()=>{const r=document.activeElement.getBoundingClientRect(),v=document.querySelector(".car-viewport").getBoundingClientRect();return {x:r.x,y:r.y,w:r.width,h:r.height,left:Math.max(v.left,0),right:Math.min(v.right,innerWidth),top:Math.max(v.top,0),bottom:Math.min(v.bottom,innerHeight)}})()')
    before=car();press(15);first=js('document.activeElement.getAttribute("data-car-cell")');assert first is not None
    press(15);assert len(car()['tiles'])==len(before['tiles']);assert not js('!!document.querySelector(".car-preview")')
    press(0);browser('wait','.car-preview');assert len(car()['tiles'])==len(before['tiles'])
    press(1);wait(lambda:not js('!!document.querySelector(".car-preview")'))
    # Real mouse pan moves legal positions outside the SVG viewBox. Directional
    # navigation must reveal them and later confirm must survive drag suppression.
    box=js('(()=>{const r=document.querySelector(".car-viewport").getBoundingClientRect();return {x:r.x+r.width/2,y:r.y+r.height/2}})()')
    browser('mouse','move',str(round(box['x'])),str(round(box['y'])));browser('mouse','down');browser('mouse','move',str(round(box['x']+350)),str(round(box['y']+170)));browser('mouse','up')
    press(14);box=focus_box();assert box['x']>=box['left']-1 and box['y']>=box['top']-1 and box['x']+box['w']<=box['right']+1 and box['y']+box['h']<=box['bottom']+1,box
    press(0);browser('wait','.car-preview')
    press(5);assert js('document.activeElement.closest(".car-place-actions")!==null')
    # Rotate through all four orientations and return to the legal preview.
    for _ in range(4):press(0)
    press(13);assert js('document.activeElement.matches(".car-place-actions .primary:not(:disabled)")')
    press(0);wait(lambda:len(car()['tiles'])==len(before['tiles'])+1);wait(lambda:room()['game']['phase']=='car_meeple')
    browser('wait','.car-feature-target');browser('focus','.car-feature-target');press(0);assert js('!document.querySelector(".car-deploy-buttons .primary").disabled')
    press(1);assert js('document.querySelector(".car-deploy-buttons .primary").disabled');press(0)
    browser('focus','.car-deploy-buttons .primary');press(0);wait(lambda:room()['game']['turn']!=room()['you'])
    after=car();assert after['tiles'][-1]['owner']==room()['you'],after['tiles'][-1]
    save('carcassonne-actions',{'before':before,'after':after,'panFocus':box})
    # Opponent completes one legal turn through the real authenticated API.
    wr=watch_js('(async()=>await(await fetch("/api/state")).json())()')['room'];cg=wr['game']['carcassonne'];p=cg['legal'][0]
    watch_api('/rooms/'+wr['id'],{'type':'action','version':wr['version'],'nonce':uuid.uuid4().hex,'action':{'type':'car_place',**p}})
    wr=watch_js('(async()=>await(await fetch("/api/state")).json())()')['room'];watch_api('/rooms/'+wr['id'],{'type':'action','version':wr['version'],'nonce':uuid.uuid4().hex,'action':{'type':'car_meeple','feature':-1}})
    wait(lambda:bool(car()['legal']));browser('wait','.car-empty-cell')
    settings();browser('select','[data-controller-model]','game:carcassonne');browser('select','select[aria-label="输入来源"]','keyboard');browser('click','button[aria-label="修改 下一组操作 按键"]');press_native('KeyH','h',72);save_settings();close_settings()
    browser('focus','.car-empty-cell');old=js('document.activeElement.getAttribute("data-car-cell")');press_native('PageDown','PageDown',34);assert js('document.activeElement.getAttribute("data-car-cell")')==old;press_native('KeyH','h',72);assert js('document.activeElement.closest(".car-place-actions")!==null')
    browser('click','.room-tools > button:first-child');browser('wait','[data-room-fullscreen]');shot('carcassonne-fullscreen');browser('set','viewport','320','740');browser('focus','.car-empty-cell');press(15);box=focus_box();assert box['x']>=box['left']-1 and box['y']>=box['top']-1 and box['x']+box['w']<=box['right']+1 and box['y']+box['h']<=box['bottom']+1,box
    shot('carcassonne-320-controller');native_tap('[data-controller-focus]');browser('wait','.car-preview');browser('press','Escape');browser('set','viewport','1280','1000')
    check(kind,serverPlacement=True,serverMeeple=True,directionalFocus=True,cancel=True,dragThenConfirm=True,cameraReveal=box,keyboardRemapping=True,fullscreen=True,mobile320=True)
   elif kind in ('catan','rail'):
    browser('wait','[data-controller-navigation="'+kind+'"]')
    def press(button):
     js('new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r)))');js('window.__pulseGamepad(0,'+json.dumps({str(button):1})+',{},3)');js('new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r)))')
    def game():return room()['game'][kind]
    def click_control(selector):
     browser('wait',selector+':not(:disabled)');browser('focus',selector);press(0)
    def opponent_action(action):
     wr=watch_js('(async()=>await(await fetch("/api/state")).json())()')['room'];return watch_api('/rooms/'+wr['id'],{'type':'action','version':wr['version'],'nonce':uuid.uuid4().hex,'action':action})
    before=game()
    if kind=='catan' and args.catan_explorer:
     for _ in range(12):
      if game()['explorer']['choices']:break
      wr=watch_js('(async()=>await(await fetch("/api/state")).json())()')['room'];choices=wr['game']['catan']['explorer']['choices'];assert choices;opponent_action(choices[0])
     before=game();browser('wait','.explorer-map [role=button]');press(15);assert js('document.activeElement.closest(".explorer-map")!==null');press(15)
     press(0);browser('wait','.explorer-confirm');assert game()['vertices']==before['vertices']
     browser('focus','.explorer-confirm-actions .primary');press(1);wait(lambda:not js('!!document.querySelector(".explorer-confirm")'))
     browser('focus','.explorer-map [role=button]');press(0);browser('wait','.explorer-confirm');click_control('.explorer-confirm-actions .primary');wait(lambda:game()['vertices']!=before['vertices'] or game()['edges']!=before['edges'])
     save('catan-explorer-controller-actions',{'before':before,'after':game()})
     focus_selector='.explorer-map-tools button:not(:disabled)';map_selector='.explorer-map-scroll'
    elif kind=='catan':
     def advance_catan_opponent():
      for _ in range(8):
       wr=watch_js('(async()=>await(await fetch("/api/state")).json())()')['room']
       if wr['game']['turn']==room()['you']:return
       cg=wr['game']['catan'];phase=wr['game']['phase'];action={'type':'catan_settlement','vertex':cg['legal']['settlements'][0]} if phase=='catan_setup_settlement' else {'type':'catan_road','edge':cg['legal']['roads'][0]}
       opponent_action(action)
      raise AssertionError('Opponent setup failed to reach local turn')
     advance_catan_opponent();before=game()
     browser('wait' ,'.catan-vertex.selectable');press(15);assert js('document.activeElement.matches(".catan-vertex.selectable")');press(15)
     assert game()['vertices']==before['vertices'];press(0);browser('wait','.catan-confirm[aria-label="确认行动"]');assert game()['vertices']==before['vertices']
     press(1);wait(lambda:not js('!!document.querySelector(".catan-confirm[aria-label=确认行动]")'));press(0)
     vertex=int(js('document.activeElement.dataset.catanVertex'));click_control('.catan-confirm[aria-label="确认行动"] .primary');wait(lambda:game()['vertices'][vertex]['owner']==room()['you'])
     browser('wait','.catan-edge.selectable');browser('focus','.catan-edge.selectable');press(0);edge=int(js('document.activeElement.dataset.catanEdge'));click_control('.catan-confirm[aria-label="确认行动"] .primary');wait(lambda:game()['edges'][edge]['owner']==room()['you'])
     # The next player's setup keeps authoritative changes observable and lets
     # this browser navigate UI without advancing any hidden game state.
     save('catan-controller-actions',{'before':before,'after':game(),'vertex':vertex,'edge':edge})
     for _ in range(6):
      wr=watch_js('(async()=>await(await fetch("/api/state")).json())()')['room']
      if wr['game']['turn']==room()['you']:break
      cg=wr['game']['catan'];phase=wr['game']['phase'];action={'type':'catan_settlement','vertex':cg['legal']['settlements'][0]} if phase=='catan_setup_settlement' else {'type':'catan_road','edge':cg['legal']['roads'][0]}
      opponent_action(action)
     wait(lambda:room()['game']['turn']==room()['you']);browser('wait','.catan-vertex.selectable')
     focus_selector='.catan-vertex.selectable';map_selector='.catan-map-scroll'
    else:
     browser('wait','.pending-tickets button');browser('focus','.pending-tickets button');press(0);press(15);press(0);assert js('document.querySelectorAll(".pending-tickets .selected").length')==2
     assert not game()['setupReady'][room()['you']];press(1);assert js('document.querySelectorAll(".pending-tickets .selected").length')==0
     press(0);press(14);press(0);assert js('document.querySelectorAll(".pending-tickets .selected").length')==2
     click_control('#ticket-choice-controls > .primary');wait(lambda:game()['setupReady'][room()['you']])
     wr=watch_js('(async()=>await(await fetch("/api/state")).json())()')['room'];opponent_action({'type':'keep','keep':[t['id'] for t in wr['game']['rail']['pending'][:2]]});wait(lambda:not game()['setup'])
     # Route details remain usable even for routes that cannot yet be paid for.
     browser('focus','.map-route');press(15);route=js('document.activeElement.dataset.railRoute');press(0);browser('wait','[role=dialog] .route-preview');press(1);wait(lambda:not js('!!document.querySelector("[role=dialog]")'))
     claimed=None
     for _ in range(15):
      payments=game().get('payments',{});available=[rid for rid,plans in payments.items() if plans]
      if available:
       claimed=available[0];browser('focus','[data-rail-route="'+claimed+'"]');press(0);browser('wait','[role=dialog] .route-preview');assert str(claimed) not in game()['owners'];click_control('[role=dialog] .primary.wide');wait(lambda:game()['owners'].get(str(claimed))==room()['you']);break
      click_control('.train-deck');wait(lambda:room()['game']['phase']=='draw');click_control('.train-deck');wait(lambda:room()['game']['turn']!=room()['you']);opponent_action({'type':'draw','slot':-1});opponent_action({'type':'draw','slot':-1});wait(lambda:room()['game']['turn']==room()['you'])
     assert claimed is not None,'No route affordable after drawing'
     save('rail-controller-actions',{'before':before,'after':game(),'route':claimed})
     focus_selector='.map-route';map_selector='.map-scroll'
    settings();browser('select','[data-controller-model]','game:'+kind);browser('select','select[aria-label="输入来源"]','keyboard');browser('click','button[aria-label="修改 '+('向右选择' if args.catan_explorer else '下一组操作')+' 按键"]');press_native('KeyH','h',72);save_settings();close_settings()
    browser('focus',focus_selector);old=js('document.activeElement.outerHTML');press_native('ArrowRight' if args.catan_explorer else 'PageDown','ArrowRight' if args.catan_explorer else 'PageDown',39 if args.catan_explorer else 34);assert js('document.activeElement.outerHTML')==old;press_native('KeyH','h',72);assert js('document.activeElement.outerHTML')!=old
    browser('click','.room-tools > button:first-child');browser('wait','[data-room-fullscreen]');shot(kind+'-fullscreen');browser('set','viewport','320','740');browser('focus',focus_selector);press(15)
    box=js('(()=>{const r=document.activeElement.getBoundingClientRect();return {x:r.x,y:r.y,w:r.width,h:r.height,vw:innerWidth,vh:innerHeight}})()');assert box['x']+box['w']>0 and box['y']+box['h']>0 and box['x']<box['vw'] and box['y']<box['vh'],box
    shot(kind+'-320-controller')
    if not args.catan_explorer:
     native_tap('[data-controller-focus]');browser('wait','.catan-confirm[aria-label="确认行动"]' if kind=='catan' else '[role=dialog] .route-preview');press(1)
    browser('press','Escape');browser('set','viewport','1280','1000');check(kind,serverActions=True,directionalFocus=True,cancel=True,keyboardRemapping=True,fullscreen=True,mobile320=box,nativeTouch=not args.catan_explorer)
   elif kind in ('monopoly','junqi','sanguosha'):
    browser('wait','[data-controller-navigation="'+kind+'"]')
    def press(button):
     js('new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r)))');js('window.__pulseGamepad(0,'+json.dumps({str(button):1})+',{},3)');js('new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r)))')
    def data():return room()['game']['data']
    def click_control(selector):
     browser('wait',selector+':not(:disabled)');browser('focus',selector);press(0)
    def opponent_action(action):
     wr=watch_js('(async()=>await(await fetch("/api/state")).json())()')['room'];return watch_api('/rooms/'+wr['id'],{'type':'action','version':wr['version'],'nonce':uuid.uuid4().hex,'action':action})
    if kind=='monopoly':
     for _ in range(24):
      d=data();c=d['controls']
      if c['auction']:break
      if c['roll']:
       old=d['rollSerial'];click_control('[data-controller-action="roll"]');wait(lambda:data()['rollSerial']>old)
      elif c['end']:click_control('[data-controller-action="end"]')
      else:
       wr=watch_js('(async()=>await(await fetch("/api/state")).json())()')['room'];wd=wr['game']['data'];wc=wd['controls']
       action_type='buy' if wc['buy'] else 'roll' if wc['roll'] else 'end' if wc['end'] else None
       if action_type:opponent_action({'type':action_type,'window':wd['window']})
       else:time.sleep(.2)
     assert data()['controls']['auction'];click_control('[data-controller-action="auction"]');wait(lambda:data().get('auction') is not None)
     prop=data()['auction']['property'];browser('wait','input[aria-label="出价金额"]');browser('focus','input[aria-label="出价金额"]');value=float(js('document.activeElement.value'));bid_before=data()['auction']['bid'];maximum=data()['players'][room()['you']]['cash'];minimum=float(js('document.activeElement.min'))
     browser('fill','input[aria-label="出价金额"]',str(maximum));press(12);assert float(js('document.activeElement.value'))==maximum
     browser('fill','input[aria-label="出价金额"]',str(minimum));press(13);assert float(js('document.activeElement.value'))==minimum
     pad({'12':1});time.sleep(1.4);pad({'12':0});assert float(js('document.activeElement.value'))>=minimum+15
     browser('fill','input[aria-label="出价金额"]',str(value));press(12)
     assert float(js('document.activeElement.value'))==value+1 and data()['auction']['bid']==bid_before
     press(0);assert js('document.activeElement.matches("[data-controller-action=bid]")');press(0);wait(lambda:data()['auction']['bid']==value+1)
     opponent_action({'type':'auction_pass','window':data()['window']});wait(lambda:data()['properties'][prop]['owner']==room()['you']);before=data()
     click_control('[data-mono-tab="trade"]');browser('focus','[data-trade-side=give] input[type=number]');press(12);press(12);press(13);assert js('document.activeElement.value')=='1'
     # Typing in the numeric field cannot move focus or trigger a board shortcut.
     focus_before=js('document.activeElement.outerHTML');press_native('ArrowRight','ArrowRight',39);assert js('document.activeElement.outerHTML')==focus_before
     press(0);assert js('document.activeElement.matches('+json.dumps('[data-trade-side=give] input[data-property="'+str(prop)+'"]')+')');press(0);press(5);assert js('document.activeElement.matches("[data-trade-side=want] input[type=number]")')
     for _ in range(4):press(12)
     assert js('document.activeElement.value')=='4';assert data()['players'][room()['you']]['cash']==before['players'][room()['you']]['cash']
     press(5);assert js('document.activeElement.matches("[data-controller-action=trade-offer]")');press(0);wait(lambda:len(data()['offers'])>0);offer=data()['offers'][0];assert offer['give']['cash']==1 and offer['want']['cash']==4,offer
     opponent_action({'type':'trade_accept','window':data()['window'],'tradeId':offer['id']});wait(lambda:data()['properties'][prop]['owner']!=room()['you']);after=data();assert after['players'][room()['you']]['cash']==before['players'][room()['you']]['cash']+3
     browser('focus','[data-mono-square="1"]');press(14);assert js('document.activeElement.getAttribute("data-mono-square")')!='1';press(0);browser('wait','[data-mono-property-dialog]');press(1);wait(lambda:not js('!!document.querySelector("[data-mono-property-dialog]")'))
     save('monopoly-controller-actions',{'property':prop,'offer':offer,'before':before,'after':after});focus_selector='[data-mono-square="1"]';focus_attr='data-mono-square'
    elif kind=='junqi':
     before=data();own=[p for p in before['pieces'] if p['seat']==room()['you'] and p['kind'] not in ('flag','mine','bomb')];a,b=own[:2]
     def choose(pos):
      browser('focus','[data-junqi-node="'+str(pos)+'"]');press(0)
      if js('document.activeElement.getAttribute("aria-pressed")')!='true':press(0)
     choose(a['pos']);press(1);assert js('document.querySelectorAll(".junqi-node.selected").length')==0;choose(a['pos']);choose(b['pos']);assert data()['revision']==before['revision']
     click_control('[data-controller-action="swap"]');wait(lambda:data()['revision']>before['revision']);assert next(p for p in data()['pieces'] if p['id']==a['id'])['pos']==b['pos']
     click_control('[data-controller-action="deploy-ready"]');watch_browser('wait','[data-controller-action="deploy-ready"]');watch_browser('click','[data-controller-action="deploy-ready"]');wait(lambda:data()['phase']=='playing')
     for _ in range(80):
      if data()['canMove']:break
      wr=watch_js('(async()=>await(await fetch("/api/state")).json())()')['room'];wd=wr['game']['data']
      if wd['canMove']:
       m=wd['legal'][0];opponent_action({'type':'move','round':wd['round'],'ply':wd['ply'],'revision':wd['revision'],'unit':m['unit'],'to':m['to']})
      time.sleep(.15)
     wait(lambda:data()['canMove']);browser('wait','[data-controller-action="resign"]:not(:disabled)');before_move=data();move=min(before_move['legal'],key=lambda m:len(m['path']));choose(move['from']);choose(move['to']);assert data()['ply']==before_move['ply'];click_control('[data-junqi-confirm]');wait(lambda:data()['ply']>before_move['ply'])
     save('junqi-controller-actions',{'deployment':before,'move':move,'after':data()});focus_selector='[data-junqi-node="'+str(move['to'])+'"]';focus_attr='data-junqi-node'
    else:
     def sg():return room()['game']['sanguosha']
     responses=[];attack_state={'enabled':False,'sent':False}
     def advance_sg_opponents():
      global watch_env
      for opponent in sg_watchers:
       watch_env=opponent;wr=watch_js('(async()=>await(await fetch("/api/state")).json())()')['room'];wg=wr['game']['sanguosha'];wp=wg.get('pending');who=wr['you'];action=None
       if wp and wp.get('canRespond'):
        if wg['selecting']:
         choices=wg['players'][who]['choices'];simple=[id for id in choices if not set(next(g for g in wg['generals'] if g['id']==id)['skills'])&{'guanxing','luoshen','yingzi','tuxi'}];action={'type':'sg_respond','prompt':wp['id'],'choice':(simple or choices)[0]}
        elif wp['kind']=='draw_phase':action={'type':'sg_respond','prompt':wp['id'],'choice':'normal'}
        elif wp['kind']=='discard':action={'type':'sg_respond','prompt':wp['id'],'cards':wg['players'][who]['hand'][:wp['amount']]}
        elif wp['kind']=='guanxing':action={'type':'sg_respond','prompt':wp['id'],'cards':[],'take':wp.get('cards') or []}
        else:action={'type':'sg_respond','prompt':wp['id'],'choice':'pass','cards':[],'targets':[]}
       elif not wg['selecting'] and wr['game']['turn']==who and not wp:
        hand=wg['players'][who]['hand'];red_slash=next((c['id'] for c in wg['cards'] if c['id'] in hand and c['kind']=='slash' and c['suit'] in (1,3)),None)
        long_weapon=next((c['id'] for c in wg['cards'] if c['id'] in hand and wg['cardTypes'][c['kind']].get('slot')=='weapon' and wg['cardTypes'][c['kind']].get('range',1)>=wg['distances'][0]),None)
        if attack_state['enabled'] and not attack_state['sent'] and red_slash is not None and wg['distances'][0]>wg['range'] and long_weapon is not None:
         action={'type':'sg_play','cards':[long_weapon],'targets':[],'skill':''}
        elif attack_state['enabled'] and not attack_state['sent'] and red_slash is not None and wg['distances'][0]<=wg['range']:
         action={'type':'sg_play','cards':[red_slash],'targets':[0],'skill':''};attack_state['sent']=True
        else:action={'type':'sg_end','cards':[],'targets':[],'skill':''}
       if action:watch_api('/rooms/'+wr['id'],{'type':'action','version':wr['version'],'nonce':uuid.uuid4().hex,'action':action})
     for _ in range(140):
      advance_sg_opponents()
      g=sg();r=room();prompt=g.get('pending')
      if not g['selecting'] and not prompt and r['game']['turn']==r['you']:break
      if g['selecting'] and prompt and prompt.get('canRespond'):
       click_control('.sg-general-choice');responses.append('general')
      elif prompt and prompt.get('canRespond'):
       if js('!!document.querySelector("[data-sg-pass]:not(:disabled)")'):click_control('[data-sg-pass]');responses.append(prompt['kind'])
       elif js('!!document.querySelector("[data-sg-confirm]:not(:disabled)")'):click_control('[data-sg-confirm]');responses.append(prompt['kind'])
       elif js('!!document.querySelector(".sg-actions button:not(:disabled)")'):click_control('.sg-actions button:not(:disabled)');responses.append(prompt['kind'])
       elif prompt.get('amount',0)>0 or prompt.get('count',0)>0:
        own_hand=g['players'][r['you']]['hand']
        for card in own_hand[:prompt.get('amount') or prompt.get('count')]:click_control('.sg-hand [data-sg-card="'+str(card)+'"]')
        click_control('[data-sg-confirm]');responses.append(prompt['kind'])
      time.sleep(.12)
     g=sg();r=room();assert not g.get('pending') and r['game']['turn']==r['you'];before=g;hand=g['players'][r['you']]['hand'];assert len(hand)>=2
     browser('focus','.sg-hand [data-sg-card="'+str(hand[0])+'"]');press(0);press(15);second=js('document.activeElement.getAttribute("data-sg-card")');press(12);assert js('document.activeElement.getAttribute("data-sg-card")')==second;press(0);assert js('document.querySelectorAll(".sg-hand .selected").length')==2;press(1);assert js('document.querySelectorAll(".sg-hand .selected").length')==0
     cards={c['id']:c for c in g['cards']};equipment=[i for i in hand if g['cardTypes'][cards[i]['kind']].get('slot')];equip=min(equipment,key=lambda i:{'weapon':0,'armor':1,'attack':2,'defense':3}.get(g['cardTypes'][cards[i]['kind']]['slot'],4)) if equipment else None
     if equip is not None:
      browser('focus','.sg-hand [data-sg-card="'+str(equip)+'"]');press(0)
     else:
      slash=next(i for i in hand if cards[i]['kind']=='slash');browser('focus','.sg-hand [data-sg-card="'+str(slash)+'"]');press(0)
      target=next(i for i,p in enumerate(g['players']) if i!=r['you'] and not p['dead'] and g['distances'][i]<=g['range']);click_control('[data-player-seat="'+str(target)+'"] .sg-seat-target')
     click_control('[data-sg-confirm]');wait(lambda:len(sg()['players'][room()['you']]['hand'])<len(hand));after_play=sg();attack_state['enabled']=True;defended=None
     for _ in range(160):
      advance_sg_opponents();g=sg();r=room();prompt=g.get('pending')
      if prompt and prompt.get('canRespond'):
       if prompt['kind']=='card':
        defended={'before':g};wanted=prompt.get('wanted','jink');own_hand=g['players'][r['you']]['hand'];response=next((c['id'] for c in g['cards'] if c['id'] in own_hand and c['kind']==wanted),None)
        if response is not None:click_control('.sg-hand [data-sg-card="'+str(response)+'"]');click_control('[data-sg-confirm]')
        else:click_control('[data-sg-pass]')
        wait(lambda:sg()['sequence']>g['sequence'] or (sg().get('pending') or {}).get('id')!=prompt['id']);defended['after']=sg();responses.append('card-response');break
       elif prompt['kind']=='discard':
        browser('focus','.sg-hand .sg-card:first-child');press(1)
        for card in g['players'][r['you']]['hand'][:prompt['amount']]:click_control('.sg-hand [data-sg-card="'+str(card)+'"]')
        click_control('[data-sg-confirm]')
       elif js('!!document.querySelector("[data-sg-pass]:not(:disabled)")'):click_control('[data-sg-pass]')
       elif js('!!document.querySelector(".sg-actions button:not(:disabled)")'):click_control('.sg-actions button:not(:disabled)')
      elif r['game']['turn']==r['you']:click_control('[data-sg-end]')
      time.sleep(.12)
     assert defended is not None,'controlled opponent did not create a card response'
     save('sanguosha-controller-actions',{'responses':responses,'before':before,'after':after_play,'defense':defended});focus_selector='.sg-hand .sg-card:first-child';focus_attr='data-sg-card'
    if kind=='junqi':
     focus_id=js('(()=>{const nodes=[...document.querySelectorAll("[data-junqi-node]")];return nodes.find(e=>{const r=e.getBoundingClientRect();return nodes.some(n=>{const b=n.getBoundingClientRect();return b.x>r.x+2&&Math.abs(b.y-r.y)<2})}).dataset.junqiNode})()');focus_selector='[data-junqi-node="'+focus_id+'"]'
    remap='下一组操作' if kind=='sanguosha' else '向右选择'
    settings();browser('select','[data-controller-model]','game:'+kind);browser('select','select[aria-label="输入来源"]','keyboard');browser('click','button[aria-label="修改 '+remap+' 按键"]');press_native('KeyH','h',72);save_settings();close_settings()
    browser('focus',focus_selector);before_focus=js('document.activeElement.getAttribute('+json.dumps(focus_attr)+')');old_key='PageDown' if kind=='sanguosha' else 'ArrowRight';press_native(old_key,old_key,34 if kind=='sanguosha' else 39);assert js('document.activeElement.getAttribute('+json.dumps(focus_attr)+')')==before_focus;press_native('KeyH','h',72);assert js('document.activeElement.getAttribute('+json.dumps(focus_attr)+')')!=before_focus
    browser('click','.room-tools > button:first-child');browser('wait','[data-room-fullscreen]');shot(kind+'-fullscreen');browser('set','viewport','320','740');browser('focus',focus_selector);js('new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r)))')
    if kind=='junqi':press(0)
    press(15)
    box=js('(()=>{const r=document.activeElement.getBoundingClientRect();return {x:r.x,y:r.y,w:r.width,h:r.height,vw:innerWidth,vh:innerHeight}})()');assert box['x']>=-1 and box['y']>=-1 and box['x']+box['w']<=box['vw']+1 and box['y']+box['h']<=box['vh']+1,box
    shot(kind+'-320-controller');browser('press','Escape');browser('set','viewport','1280','1000');check(kind,serverActions=True,directionalFocus=True,cancel=True,keyboardRemapping=True,fullscreen=True,mobile320=True)
   elif kind=='racing':
    browser('wait','[data-controller-input="racing"]');wait(lambda:room()['game']['data']['phase']=='racing',20);browser('focus','[data-fullscreen-board]');time.sleep(.2)
    def racer():
     r=room();return next(c for c in r['game']['data']['cars'] if c['seat']==r['you'])
    def racing_ui():return js('({...document.querySelector(".racing-controls").dataset})')
    before=racer();pad({'7':.3});wait(lambda:abs(racer()['throttle']-.3)<.01);time.sleep(.3);light=racer();assert light['speed']>before['speed']
    pad({'7':.8},axes={'0':.625});wait(lambda:abs(racer()['steer']-.5)<.02 and abs(racer()['throttle']-.8)<.01);medium=racer();assert abs(float(racing_ui()['steer'])-.5)<.02
    pad(axes={'0':0});wait(lambda:racer()['steer']==0);time.sleep(.2);slow_before=racer();angle_before=float(js('document.querySelector(".racing-chase").dataset.angle'))
    js('window.__controllerTransportDelay=650;true');pad(axes={'0':-.625});time.sleep(.12);slow_visual=js('({...document.querySelector(".racing-chase").dataset})');slow_authority=racer()
    difference=math.atan2(math.sin(float(slow_visual['angle'])-angle_before),math.cos(float(slow_visual['angle'])-angle_before))
    assert difference<-.005 and slow_authority['steer']==0 and abs(slow_authority['angle']-slow_before['angle'])<.000001,(difference,slow_before,slow_authority)
    delayed=js('window.__controllerDelayedMessages');assert any(m.get('action',{}).get('steer',0)<0 and m['delay']>=640 for m in delayed),delayed
    js('window.__controllerTransportDelay=0;true');pad(axes={'0':0});wait(lambda:racer()['seq']>slow_before['seq'] and racer()['steer']==0);time.sleep(.2)
    pad({'6':.25},axes={'0':0});wait(lambda:abs(racer().get('brakeAmount',0)-.25)<.01);partial=racer();assert partial['throttle']==0
    pad({'6':.85});wait(lambda:abs(racer().get('brakeAmount',0)-.85)<.01);strong=racer();pad({'6':0,'7':0});time.sleep(.15)
    native_key(True,'KeyW','w',87);wait(lambda:racer()['throttle']==1);pad({'7':.4});time.sleep(.1);js('window.__disconnectGamepad(0);true');time.sleep(.2);assert racer()['throttle']==1;native_key(False,'KeyW','w',87);wait(lambda:racer()['throttle']==0)
    pad({'7':.8});time.sleep(.2);assert racer()['throttle']==0;pad({'7':0});time.sleep(.1);pad({'7':.5});wait(lambda:abs(racer()['throttle']-.5)<.01)
    pad({'0':1});wait(lambda:racer()['drift']);pad({'0':0,'7':0});pad({'3':1});wait(lambda:js('document.querySelector(".racing-view-controls button:first-child").getAttribute("aria-pressed")==="true"'));pad({'3':0});time.sleep(.15)
    pad({'9':1});browser('wait','.racing-map-modal');pad({'9':0});time.sleep(.1);assert racer()['throttle']==0;pad({'1':1});wait(lambda:not js('!!document.querySelector(".racing-map-modal")'));pad({'1':0});time.sleep(.1)
    settings();browser('select','[data-controller-model]','game:racing');pad({'7':.3});browser('wait','--text','30%');assert racer()['throttle']==0;pad({'7':0});browser('select','select[aria-label="输入来源"]','keyboard');browser('click','button[aria-label="修改 油门 按键"]');press_native('KeyH','h',72);save_settings();close_settings();browser('focus','[data-fullscreen-board]')
    press_native('KeyW','w',87);assert racer()['throttle']==0;native_key(True,'KeyH','h',72);wait(lambda:racer()['throttle']==1);native_key(False,'KeyH','h',72);wait(lambda:racer()['throttle']==0)
    pad({'6':1});wait(lambda:racer()['speed']<55,15);pad({'6':0});time.sleep(.15);pad({'2':1});wait(lambda:racer()['resetUntil']>0);pad({'2':0})
    browser('click','.room-tools > button:first-child');browser('wait','[data-room-fullscreen]');shot('racing-gamepad-fullscreen');browser('set','viewport','320','740');browser('scrollintoview','button[aria-label="油门"]');shot('racing-gamepad-320')
    point=js('(()=>{const r=document.querySelector(".racing-pedals button:last-child").getBoundingClientRect();return {x:r.x+r.width/2,y:r.y+r.height/2,id:1}})()');native=subprocess.run(['node','scripts/controllers/gamepad/native-touch.mjs',browser('get','cdp-url')['cdpUrl'],base,json.dumps([{'type':'touchStart','touchPoints':[point],'wait':300},{'type':'touchCancel','touchPoints':[],'wait':200}]),'.racing-controls'],cwd=root,env=env,text=True,capture_output=True,timeout=20);assert native.returncode==0,native.stderr
    touched=json.loads(native.stdout);assert float(touched[0]['render']['throttle'])==1 and float(touched[-1]['render']['throttle'])==0,touched
    browser('press','Escape');browser('set','viewport','1280','1000');browser('focus','[data-fullscreen-board]');wait(lambda:room()['game']['data']['serverAt']>=racer()['resetUntil'],10);pad({'6':1});wait(lambda:racer()['speed']<.05);pad({'6':0});wait(lambda:not racer()['brake']);time.sleep(.15)
    js('(()=>{const canvas=document.querySelector(".racing-chase");window.__raceLatency={began:performance.now()};window.__controllerTransportDelay=650;const observer=new MutationObserver(()=>{const key=window.__nativeKeyEvents.find(e=>e.code==="KeyH"&&e.at>=window.__raceLatency.began);if(key&&+canvas.dataset.speed>.1){window.__raceLatency={...window.__raceLatency,event:key,elapsed:performance.now()-key.at,speed:+canvas.dataset.speed};observer.disconnect();}});observer.observe(canvas,{attributes:true,attributeFilter:["data-speed"]});return true})()')
    native_key(True,'KeyH','h',72);latency=wait(lambda:js('window.__raceLatency.elapsed!==undefined&&window.__raceLatency'));assert latency['event']['trusted'] and latency['elapsed']<180 and racer()['throttle']==0,latency
    native_key(False,'KeyH','h',72);js('window.__controllerTransportDelay=0;true');time.sleep(.15);sent=js('Math.max(...window.__controllerSentInputs.filter(s=>s.action?.type==="drive").map(s=>s.action.seq))');wait(lambda:racer()['seq']>=sent and racer()['throttle']==0)
    save('racing-native-latency',latency);save('racing-analog-evidence' ,{'before':before,'light':light,'medium':medium,'partialBrake':partial,'strongBrake':strong,'slowInput':{'before':slow_before,'authority':slow_authority,'visual':slow_visual,'angleDelta':difference,'messages':js('window.__controllerDelayedMessages')},'touch':touched,'after':racer()});check(kind,analogThrottle=True,analogSteering=True,analogBrake=True,predictionBeforeAcknowledgement=True,mixedSources=True,disconnectRearm=True,drift=True,rear=True,map=True,reset=True,remapping=True,mobileTouch=True,fullscreen=True)
   elif kind in ('guandan','shengji'):
    browser('wait','[data-controller-navigation="'+kind+'"]')
    def card_press(button):
     js('window.__pulseGamepad(0,'+json.dumps({str(button):1})+',{},3)');js('new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r)))')
    def card_data():return room()['game']['data']
    def action(name):
     browser('wait','[data-controller-action="'+name+'"]:not(:disabled)');browser('focus','[data-controller-action="'+name+'"]');card_press(0)
    if kind=='guandan':
     browser('click','.room-tools > button:first-child');browser('wait','[data-room-fullscreen]');js('new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r)))');card_press(15);assert js('!!document.activeElement.closest(".poker-hand")');browser('press','Escape')
    stages=[]
    for _ in range(80):
     d=card_data();c=d['controls']
     if d['phase']=='play':break
     if c['canAct']:
      if d['phase']=='declare':
       if c.get('declarations'):
        action('declare');browser('wait','.poker-choice-buttons button');choices=card_data()['controls']['declarations'];best=max(range(len(choices)),key=lambda i:choices[i]['strength']);browser('focus','.poker-choice-buttons button:nth-child('+str(best+1)+')');card_press(0);stages.append('declare-choice')
       else:action('pass');stages.append('declare-pass')
      elif d['phase']=='bury':
       browser('focus','.poker-hand button[data-card]:first-child');card_press(1)
       for i in range(c['buryCount']):
        if i:card_press(15)
        card_press(0)
       action('bury');stages.append('bury')
     time.sleep(.2)
    assert card_data()['phase']=='play',card_data()['phase']
    browser('focus','.poker-hand button[data-card]:first-child');card_press(1);card_press(0);card_press(15);card_press(0)
    assert js('document.querySelectorAll(".poker-hand .chosen").length')==2;time.sleep(.2);assert js('document.querySelectorAll(".poker-hand .chosen").length')==2
    card_press(1);assert js('document.querySelectorAll(".poker-hand .chosen").length')==0
    # A real game-owned select exercises options and dialog exit without sending a play.
    browser('focus','.poker-table-toolbar button');card_press(0);browser('wait','select[aria-label="牌面风格"]');browser('focus','select[aria-label="牌面风格"]');skin=js('document.querySelector('+json.dumps('select[aria-label="牌面风格"]')+').value')
    index=js('document.activeElement.selectedIndex');card_press(13 if index==0 else 12);assert js('document.querySelector('+json.dumps('select[aria-label="牌面风格"]')+').value')!=skin
    card_press(0);assert js('document.activeElement.tagName')!='SELECT';card_press(1);wait(lambda:not js('!!document.querySelector("[role=dialog]")'))
    for _ in range(80):
     d=card_data();c=d['controls']
     if c['canAct'] and c['moves']:break
     if c.get('canPass'):action('pass')
     time.sleep(.2)
    before=card_data();assert before['controls']['canAct'] and before['controls']['moves']
    browser('focus','.poker-hand button[data-card]:first-child');card_press(5);assert js('!!document.activeElement.closest(".poker-actions")')
    action('hint');assert js('document.querySelectorAll(".poker-hand .chosen").length')>0;action('play')
    if js('!!document.querySelector('+json.dumps('[role=dialog] select[aria-label="百搭牌型"]')+')'):
     browser('focus','[role="dialog"] button.primary');card_press(0)
    wait(lambda:len(card_data()['hand'])<len(before['hand']));save(kind+'-play-evidence',{'stages':stages,'before':before,'after':card_data()})
    settings();browser('select','[data-controller-model]','game:'+kind);browser('select','select[aria-label="输入来源"]','keyboard');browser('click','button[aria-label="修改 向右选择 按键"]');press_native('KeyH','h',72);save_settings();close_settings()
    browser('focus','.poker-hand button[data-card]:first-child');first=js('document.activeElement.dataset.card');press_native('ArrowRight','ArrowRight',39);assert js('document.activeElement.dataset.card')==first;press_native('KeyH','h',72);assert js('document.activeElement.dataset.card')!=first
    if kind=='guandan':
     settings();browser('select','[data-controller-model]','game:guandan');browser('select','select[aria-label="输入来源"]','keyboard');browser('click','button[aria-label="修改 向右选择 按键"]');press_native('ControlLeft','Control',17,2);save_settings();close_settings()
     browser('focus','.poker-hand button[data-card]:first-child');first=js('document.activeElement.dataset.card');press_native('ControlLeft','Control',17,2);assert js('document.activeElement.dataset.card')!=first
    browser('click','.room-tools > button:first-child');browser('wait','[data-room-fullscreen]');browser('set','viewport','320','740');browser('focus','.poker-hand-row:first-child button:first-child');js('new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r)))');card_press(13)
    assert js('document.activeElement.closest(".poker-hand-row")!==document.querySelector(".poker-hand-row")');shot(kind+'-320-navigation');browser('press','Escape');browser('set','viewport','1280','1000');check(kind,selectionAndCancel=True,gameSelect=True,legalPlay=True,perGameRemapping=True,mobileRows=True,fullscreen=True)
   elif kind in ('chess','xiangqi','weiqi','pairs'):
    browser('wait','[data-controller-navigation="'+kind+'"]')
    def nav_press(button):
     js('window.__pulseGamepad(0,'+json.dumps({str(button):1})+',{},3)');js('new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r)))')
    def board_data():return room()['game']['data']
    def go_to(selector):
     for _ in range(70):
      point=js('(()=>{const a=document.activeElement,b=document.querySelector('+json.dumps(selector)+');if(a===b)return {done:true};const x=a.getBoundingClientRect(),y=b.getBoundingClientRect();return {dx:y.x+y.width/2-x.x-x.width/2,dy:y.y+y.height/2-x.y-x.height/2}})()')
      if point.get('done'):return
      nav_press((13 if point['dy']>0 else 12) if abs(point['dy'])>abs(point['dx']) else (15 if point['dx']>0 else 14))
     raise AssertionError('directional navigation did not reach '+selector)
    if kind=='pairs':wait(lambda:board_data()['canAct'])
    browser('click','.room-tools > button:first-child');browser('wait','[data-room-fullscreen]');js('new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r)))');nav_press(15);assert js('!!document.activeElement.closest("[data-controller-group=board]")');browser('press','Escape')
    if kind in ('chess','xiangqi','weiqi'):
     flag='canPlay' if kind=='weiqi' else 'canMove'
     if not board_data()[flag]:
      wr=watch_js('(async()=>await(await fetch("/api/state")).json())()')['room'];d=wr['game']['data']
      action={'type':'place','pos':d['legal'][0]} if kind=='weiqi' else {'type':'move',**({'uci':d['legal'][0]['uci']} if kind=='chess' else d['legal'][0])}
      action.update(round=d['round'],ply=d['ply'])
      if kind=='weiqi':action.update(revision=d['revision'])
      watch_api('/rooms/'+wr['id'],{'type':'action','version':wr['version'],'nonce':uuid.uuid4().hex,'action':action})
     wait(lambda:board_data()[flag]);before=board_data()
     if kind=='weiqi':
      n=before['options']['size'];target=next(p for p in before['legal'] if p%n>0);start=target-1
      square=lambda pos:'[data-weiqi-cell="'+str(pos)+'"]'
     else:
      width=8 if kind=='chess' else 9
      move=next(m for m in before['legal'] if m['from']%width==m['to']%width and abs(m['from']-m['to'])==width)
      start=move['from'];target=move['to']
      square=lambda pos:'[data-'+kind+'-square="'+chr(97+pos%width)+str(pos//width+1 if kind=='chess' else 9-pos//width)+'"]'
     browser('wait','[data-'+kind+'-resign]:not(:disabled)');browser('focus',square(start));js('new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r)))')
     horizontal=js('(()=>{const r=document.activeElement.getBoundingClientRect();return [...document.querySelectorAll('+json.dumps('[data-'+kind+('-cell]' if kind=='weiqi' else '-square]'))+')].some(e=>{const b=e.getBoundingClientRect();return b.x>r.x+2&&Math.abs(b.y-r.y)<2})?15:14})()')
     nav_press(horizontal);assert js('document.activeElement.matches('+json.dumps(square(start))+')') is False
     assert board_data()['ply']==before['ply'];go_to(square(start))
     if kind!='weiqi':
      nav_press(0);assert js('document.querySelector('+json.dumps(square(start))+').getAttribute("aria-pressed")')=='true'
      nav_press(1);assert js('document.querySelector('+json.dumps(square(start))+').getAttribute("aria-pressed")')=='false';nav_press(0)
      nav_press(5);go_to('[data-'+kind+'-resign]');nav_press(0);browser('wait','[role="dialog"]');nav_press(1);wait(lambda:not js('!!document.querySelector("[role=dialog]")'))
      assert js('document.querySelector('+json.dumps(square(start))+').getAttribute("aria-pressed")')=='true'
      nav_press(4)
     go_to(square(target));nav_press(0)
     if kind=='weiqi':
      if js('document.querySelector(".weiqi-board").dataset.selected')=='':nav_press(0)
      nav_press(1);assert js('document.querySelector(".weiqi-board").dataset.selected')=='';nav_press(0)
     assert board_data()['ply']==before['ply']
     nav_press(5)
     if kind=='weiqi':go_to('[data-weiqi-confirm]')
     else:
      # The draw checkbox must be reachable without becoming an editor trap.
      go_to('.'+kind+'-offer input');nav_press(0);assert js('document.querySelector(".'+kind+'-offer input").checked');nav_press(0)
      go_to('[data-'+kind+'-confirm]')
     nav_press(0);wait(lambda:board_data()['ply']>before['ply']);save(kind+'-navigation-action',{'before':before,'after':board_data()})
    else:
     wait(lambda:board_data()['canAct']);before=board_data();browser('focus','.pairs-toolbar button:first-child');nav_press(0);wait(lambda:board_data()['players'][room()['you']]['event']['type']=='hint')
     pair=board_data()['players'][room()['you']]['event']['pair'];square=lambda pos:'.pairs-cell[data-index="'+str(pos)+'"]'
     browser('focus',square(pair[0]));nav_press(0);nav_press(1);assert js('document.querySelectorAll(".pairs-cell.selected").length')==0;nav_press(0);go_to(square(pair[1]))
     removed=js('(()=>{const r=document.activeElement.getBoundingClientRect();return {x:r.x+r.width/2,y:r.y+r.height/2}})()');remaining=board_data()['players'][room()['you']]['remaining'];nav_press(0);wait(lambda:board_data()['players'][room()['you']]['remaining']<remaining)
     nav_press(15);assert js('!!document.activeElement.closest(".pairs-field")&&!document.activeElement.disabled')
     distance=js('(()=>{const p='+json.dumps(removed)+',nodes=[...document.querySelectorAll(".pairs-cell:not(:disabled)")],d=e=>{const r=e.getBoundingClientRect();return Math.hypot(r.x+r.width/2-p.x,r.y+r.height/2-p.y)};return {actual:d(document.activeElement),nearest:Math.min(...nodes.map(d))}})()');assert distance['actual']<=distance['nearest']+2,distance
     save(kind+'-navigation-action',{'before':before,'after':board_data(),'recoveredFocus':distance})
    settings();browser('select','[data-controller-model]','game:'+kind);browser('select','select[aria-label="输入来源"]','keyboard');browser('click','button[aria-label="修改 向右选择 按键"]');press_native('KeyH','h',72);save_settings();close_settings()
    # Check remapping on two actual horizontally adjacent displayed cells.
    selector='.pairs-cell:not(:disabled)' if kind=='pairs' else '[data-'+kind+('-cell]' if kind=='weiqi' else '-square]')
    candidate=js('(()=>{const a=[...document.querySelectorAll('+json.dumps(selector)+')];for(const x of a){const p=x.getBoundingClientRect();const y=a.find(z=>{const r=z.getBoundingClientRect();return r.x>p.x+2&&Math.abs(r.y-p.y)<2});if(y)return {first:x.getAttribute("data-index")??x.getAttribute("data-'+kind+'-cell")??x.getAttribute("data-'+kind+'-square"),second:y.getAttribute("data-index")??y.getAttribute("data-'+kind+'-cell")??y.getAttribute("data-'+kind+'-square")};}return null})()');assert candidate
    attr='data-index' if kind=='pairs' else 'data-'+kind+('-cell' if kind=='weiqi' else '-square')
    first_selector='['+attr+'="'+candidate['first']+'"]';browser('focus',first_selector);press_native('ArrowRight','ArrowRight',39);assert js('document.activeElement.getAttribute('+json.dumps(attr)+')')==candidate['first'];press_native('KeyH','h',72);assert js('document.activeElement.getAttribute('+json.dumps(attr)+')')==candidate['second']
    browser('click','.room-tools > button:first-child');browser('wait','[data-room-fullscreen]');shot(kind+'-fullscreen');browser('set','viewport','320','740');browser('focus',first_selector);js('new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r)))');nav_press(15)
    assert js('document.activeElement.getAttribute('+json.dumps(attr)+')')!=candidate['first']
    focus_style=js('(()=>{const s=getComputedStyle(document.activeElement);return {width:parseFloat(s.outlineWidth),style:s.outlineStyle,marked:document.activeElement.hasAttribute("data-controller-focus")}})()');assert focus_style['marked'] and focus_style['width']>=2 and focus_style['style']=='solid',focus_style
    visible=js('(()=>{const r=document.activeElement.getBoundingClientRect();return {x:r.x,y:r.y,w:r.width,h:r.height,vw:innerWidth,vh:innerHeight}})()');assert visible['x']>=-1 and visible['y']>=-1 and visible['x']+visible['w']<=visible['vw']+1 and visible['y']+visible['h']<=visible['vh']+1,visible
    shot(kind+'-320-navigation');browser('press','Escape');browser('set','viewport','1280','1000')
    if kind=='weiqi':
     wr=watch_js('(async()=>await(await fetch("/api/state")).json())()')['room'];d=wr['game']['data'];watch_api('/rooms/'+wr['id'],{'type':'action','version':wr['version'],'nonce':uuid.uuid4().hex,'action':{'type':'pass','round':d['round'],'ply':d['ply'],'revision':d['revision']}})
     wait(lambda:board_data()['canPlay']);browser('focus','[data-weiqi-pass]');nav_press(0);wait(lambda:board_data()['phase']=='scoring')
     stone=next(i for i,c in enumerate(board_data()['board']) if c);browser('focus','[data-weiqi-cell="'+str(stone)+'"]');nav_press(0)
     if js('document.querySelector(".weiqi-board").dataset.selected')=='':nav_press(0)
     nav_press(5);go_to('[data-weiqi-mark-dead]');nav_press(0);wait(lambda:len(board_data()['dead'])>0)
     go_to('[data-weiqi-score]');nav_press(0);wait(lambda:board_data()['confirmed'][room()['you']]);save('weiqi-scoring',board_data())
    check(kind,directionsOnlyFocus=True,confirmedServerAction=True,cancel=True,keyboardRemapping=True,fullscreen=True,mobileFocusVisible=True)
   elif kind=='fortressblue':
    browser('wait','[data-controller-input="fortressblue"]');watch_browser('wait','[data-fortressblue-tank="1"]');watch_browser('click','[data-fortressblue-tank="1"]')
    browser('focus','[data-fortressblue-tank="1"]');js('window.__pulseGamepad(0,{0:1},{},3)')
    def fb():return room()['game']['data']
    def fb_own():
     d=fb();return next(t for t in d['tanks'] if t['seat']==d['you'])
    def fb_ui():return js('({...document.querySelector(".fortressblue-board").dataset})')
    wait(lambda:fb()['phase']=='playing',40)
    if not fb()['canFire']:
     wr=watch_js('(async()=>await(await fetch("/api/state")).json())()')['room'];wd=wr['game']['data'];watch_api('/rooms/'+wr['id'],{'type':'action','version':wr['version'],'nonce':uuid.uuid4().hex,'action':{'type':'fire','epoch':wd['epoch'],'turn':wd['turn'],'angle':45,'power':10,'facing':1,'weapon':'primary'}})
    wait(lambda:fb()['canFire'],30);browser('focus','[data-fullscreen-board]');pad({'0':0});time.sleep(.2)
    before=fb();original_shot=(before.get('lastShot') or {}).get('id',0);initial_angle=fb_ui()['fortressblueAngle']
    pad(axes={'1':-1});wait(lambda:fb_ui()['fortressblueAngle']!=initial_angle);pad(axes={'1':0});time.sleep(.1)
    fuel=fb_own()['fuel'];pad(axes={'0':1});time.sleep(.10);pad(axes={'0':0});wait(lambda:fb_own()['fuel']<fuel)
    pad({'2':1});time.sleep(.1);pad({'2':0});assert js('document.querySelector("[data-fortressblue-weapon=special]").getAttribute("aria-pressed")')=='true'
    browser('wait','[data-fortressblue-fire]:not(:disabled)');count=len(fb_own().get('items',[]));pad({'4':1});time.sleep(.15);pad({'4':0});wait(lambda:len(fb_own()['items'])<count)
    browser('wait','[data-fortressblue-fire]:not(:disabled)');pad({'0':1});wait(lambda:fb_ui()['fortressblueCharging']=='1');time.sleep(.15);js('window.__disconnectGamepad(0);true');wait(lambda:fb_ui()['fortressblueCharging']=='0');assert (fb().get('lastShot') or {}).get('id',0)==original_shot
    pad({'0':1});time.sleep(.15);assert fb_ui()['fortressblueCharging']=='0';pad({'0':0});time.sleep(.1)
    pad({'0':1});wait(lambda:fb_ui()['fortressblueCharging']=='1');pad({'1':1});wait(lambda:fb_ui()['fortressblueCharging']=='0');pad({'0':0,'1':0});js('new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r)))');assert (fb().get('lastShot') or {}).get('id',0)==original_shot
    pad({'0':1});wait(lambda:fb_ui()['fortressblueCharging']=='1');settings();wait(lambda:fb_ui()['fortressblueCharging']=='0');pad({'0':0})
    browser('select','[data-controller-model]','game:fortressblue');browser('select','select[aria-label="输入来源"]','keyboard');browser('click','button[aria-label="修改 蓄力／松开发射 按键"]');press_native('KeyV','v',86);save_settings();close_settings();browser('focus','[data-fullscreen-board]')
    press_native('Space',' ',32);assert fb_ui()['fortressblueCharging']=='0'
    native_key(True,'KeyV','v',86);wait(lambda:fb_ui()['fortressblueCharging']=='1');pad({'0':1});time.sleep(.1);js('window.__disconnectGamepad(0);true');assert fb_ui()['fortressblueCharging']=='1';browser('press','Escape');native_key(False,'KeyV','v',86);pad();time.sleep(.1);assert (fb().get('lastShot') or {}).get('id',0)==original_shot
    camera_before=js('document.querySelector("[data-fortressblue-canvas]").dataset.worldViewWidth');pad({'9':1});time.sleep(.2);pad({'9':0});assert js('document.querySelector("[data-fortressblue-canvas]").dataset.worldViewWidth')!=camera_before
    browser('click','.room-tools > button:first-child');browser('wait','[data-room-fullscreen]');shot('fortressblue-fullscreen');browser('set','viewport','320','740');shot('fortressblue-320');assert js('document.documentElement.scrollWidth<=innerWidth+1')
    browser('wait','[data-fortressblue-move="-1"]:not(:disabled)');browser('scrollintoview','[data-fortressblue-move="-1"]');move_point=js('(()=>{const r=document.querySelector("[data-fortressblue-move=\\"-1\\"]").getBoundingClientRect();return {x:r.x+r.width/2,y:r.y+r.height/2,id:1}})()')
    touch_fuel=fb_own()['fuel'];save('fortress-touch-point',{'point':move_point,'hit':js('document.elementFromPoint('+str(move_point['x'])+','+str(move_point['y'])+')?.outerHTML'),'ui':fb_ui()});moving=subprocess.run(['node','scripts/controllers/gamepad/native-touch.mjs',browser('get','cdp-url')['cdpUrl'],base,json.dumps([{'type':'touchStart','touchPoints':[move_point],'wait':0},{'type':'touchCancel','touchPoints':[],'wait':120}]),'.fortressblue-board'],cwd=root,env=env,text=True,capture_output=True,timeout=20);assert moving.returncode==0,moving.stderr;save('fortress-native-movement',json.loads(moving.stdout));wait(lambda:fb_own()['fuel']<touch_fuel)
    browser('wait','[data-fortressblue-fire]:not(:disabled)');browser('scrollintoview','[data-fortressblue-fire]');point=js('(()=>{const r=document.querySelector("[data-fortressblue-fire]").getBoundingClientRect();return {x:r.x+r.width/2,y:r.y+r.height/2,id:1}})()')
    native=subprocess.run(['node','scripts/controllers/gamepad/native-touch.mjs',browser('get','cdp-url')['cdpUrl'],base,json.dumps([{'type':'touchStart','touchPoints':[point],'wait':250},{'type':'touchCancel','touchPoints':[],'wait':100}]),'.fortressblue-board'],cwd=root,env=env,text=True,capture_output=True,timeout=20);assert native.returncode==0,native.stderr
    assert (fb().get('lastShot') or {}).get('id',0)==original_shot;save('fortressblue-native-touch',json.loads(native.stdout));shot('fortressblue-320-controls')
    browser('press','Escape');browser('set','viewport','1280','1000');browser('focus','[data-fullscreen-board]');native_key(True,'KeyV','v',86);time.sleep(.25);native_key(False,'KeyV','v',86)
    wait(lambda:(fb().get('lastShot') or {}).get('id',0)>original_shot);after=fb()
    wait(lambda:fb().get('resolution') is None,30)
    if not fb()['canFire']:
     wr=watch_js('(async()=>await(await fetch("/api/state")).json())()')['room'];wd=wr['game']['data'];watch_api('/rooms/'+wr['id'],{'type':'action','version':wr['version'],'nonce':uuid.uuid4().hex,'action':{'type':'fire','epoch':wd['epoch'],'turn':wd['turn'],'angle':45,'power':10,'facing':1,'weapon':'primary'}})
    wait(lambda:fb()['canFire'],30);browser('wait','[data-fortressblue-fire]:not(:disabled)');pad();time.sleep(.1);full_before=(fb().get('lastShot') or {}).get('id',0);pad({'0':1})
    wait(lambda:(fb().get('lastShot') or {}).get('id',0)>full_before,8);full=fb();assert int(fb_ui()['fortressbluePower'])==100;time.sleep(.3);assert fb()['lastShot']['id']==full['lastShot']['id'];pad({'0':0})
    save('fortressblue-evidence',{'before':before,'afterRelease':after,'afterFullCharge':full});check(kind,selection=True,aimAndMove=True,weapon=True,item=True,disconnectCancels=True,keyboardSurvivesDisconnect=True,rearm=True,explicitCancel=True,settingsCancel=True,keyboardRemap=True,escCancel=True,cameraZoom=True,nativeTouchCancel=True,nativeTouchMovement=True,releaseFires=True,fullChargeOnce=True,fullscreen=True,mobile320=True)
   elif kind in ('tetris','spf2t'):
    browser('wait','[data-controller-input="'+kind+'"]')
    if kind=='spf2t':
     watch_browser('wait','.spf2t-card');watch_browser('click','.spf2t-card:first-child')
     browser('focus','.spf2t-card:first-child');js('window.__pulseGamepad(0,{0:1},{},3)')
    wait(lambda:room()['game']['data']['phase']=='playing',40)
    browser('focus','[data-fullscreen-board]');time.sleep(.2)
    def block_self():
     r=room();return r['game']['data']['players'][r['you']]
    block_selector='.tetris-board' if kind=='tetris' else '.spf2t-panel.is-self'
    def block_visual():return js('({...document.querySelector('+json.dumps(block_selector)+').dataset})')
    before=block_self();x=before['active']['x'];pad(axes={'0':-1});wait(lambda:block_self()['active']['x']<x);time.sleep(.25);pad(axes={'0':0});time.sleep(.15)
    moved=block_self();assert moved['active']['x']<x
    # Move back toward the middle before testing rotation and delayed input.
    pad(axes={'0':1});time.sleep(.15);pad(axes={'0':0});time.sleep(.15)
    if kind=='tetris' and block_self()['active']['kind']==4:
     square_seq=block_self()['seq'];js('window.__pulseGamepad(0,{0:1},{},3)');wait(lambda:block_self()['seq']>square_seq);assert block_self()['active']['rot']==0
     js('window.__pulseGamepad(0,{4:1},{},3)');wait(lambda:block_self()['active']['kind']!=4)
    rot=block_self()['active']['rot'];pad({'0':1});wait(lambda:block_self()['active']['rot']!=rot);rotated=block_self()['active']['rot'];time.sleep(.3);assert block_self()['active']['rot']==rotated;pad({'0':0})
    press_native('ControlLeft','Control',17,2);wait(lambda:block_self()['active']['rot']!=rotated)
    # Capture a visual change while the production input packet waits in transit.
    seq=block_self().get('seq',0);server_x=block_self()['active']['x'];original=block_visual();axis=1 if server_x<3 else -1
    field='tetrisActiveX' if kind=='tetris' else 'spf2tActiveX'
    js('(()=>{window.__controllerTransportDelay=650;window.__blockFrames=[];window.__blockRecord=true;const node=document.querySelector('+json.dumps(block_selector)+'),field='+json.dumps(field)+';const sample=()=>{window.__blockFrames.push({at:performance.now(),x:+node.dataset[field]});if(window.__blockRecord)requestAnimationFrame(sample)};requestAnimationFrame(sample);return true})()')
    js('window.__pulseGamepad(0,{},'+json.dumps({'0':axis})+',2)');time.sleep(.05);predicted=block_visual();during=block_self()
    field='tetrisActiveX' if kind=='tetris' else 'spf2tActiveX'
    assert predicted[field]!=original[field] and during.get('seq',0)==seq,(original,predicted,during)
    js('window.__controllerTransportDelay=0;true');wait(lambda:block_self().get('seq',0)>seq);time.sleep(.2)
    assert float(block_visual()[field])==block_self()['active']['x']
    frames=js('window.__blockRecord=false;window.__blockFrames');assert all(f['x'] in (server_x,server_x+axis) for f in frames),frames
    # A held hard-drop button must not drop the next piece as well.
    first_id=block_self()['active']['id'];pad({'3':1});wait(lambda:block_self().get('active',{}).get('id',first_id)!=first_id);new_id=block_self()['active']['id'];time.sleep(.5);assert block_self()['active']['id']==new_id;pad({'3':0});time.sleep(.15)
    if kind=='tetris':
     hold_id=block_self()['active']['id'];press_native('ShiftLeft','Shift',16,8);wait(lambda:block_self()['active']['id']!=hold_id)
    settings();browser('select','[data-controller-model]','game:'+kind);browser('select','select[aria-label="输入来源"]','keyboard');browser('click','button[aria-label="修改 向右移动 按键"]');press_native('KeyH','h',72);save_settings();close_settings();browser('focus','[data-fullscreen-board]')
    old_x=block_self()['active']['x'];press_native('ArrowRight','ArrowRight',39);assert block_self()['active']['x']==old_x
    press_native('KeyH','h',72);wait(lambda:block_self()['active']['x']>old_x)
    settings();blocked=block_self().get('seq',0);pad({'0':1},axes={'0':1});time.sleep(.25);assert block_self().get('seq',0)<=blocked+1;pad({'0':0},axes={'0':0});close_settings()
    browser('click','.room-tools > button:first-child');browser('wait','[data-room-fullscreen]');shot(kind+'-fullscreen');browser('set','viewport','320','740');shot(kind+'-320');assert js('document.documentElement.scrollWidth<=innerWidth+1')
    left_button='.tetris-controls button[aria-label="向左移动"]' if kind=='tetris' else '.spf2t-controls button[aria-label="左移"]'
    browser('scrollintoview',left_button);point=js('(()=>{const r=document.querySelector('+json.dumps(left_button)+').getBoundingClientRect();return {x:r.x+r.width/2,y:r.y+r.height/2,id:1}})()')
    touch_before=block_self()['active']['x'];native=subprocess.run(['node','scripts/controllers/gamepad/native-touch.mjs',browser('get','cdp-url')['cdpUrl'],base,json.dumps([{'type':'touchStart','touchPoints':[point],'wait':280},{'type':'touchCancel','touchPoints':[],'wait':200}]),block_selector],cwd=root,env=env,text=True,capture_output=True,timeout=20);assert native.returncode==0,native.stderr
    assert block_self()['active']['x']<touch_before
    released_x=block_self()['active']['x'];time.sleep(.2);assert block_self()['active']['x']==released_x
    save(kind+'-native-touch',json.loads(native.stdout));shot(kind+'-320-controls');browser('press','Escape');browser('set','viewport','1280','1000')
    save(kind+'-block-evidence',{'before':before,'moved':moved,'originalVisual':original,'predictedVisual':predicted,'authorityDuringDelay':during,'visibleFramesThroughAcknowledgement':frames,'delayedMessages':js('window.__controllerDelayedMessages'),'final':block_self()})
    check(kind,directionalRepeat=True,rotationOnce=True,hardDropOnce=True,predictionBeforeSend=True,noAcknowledgementReplay=True,modifierAliases=True,nativeTouch=True,remapping=True,settingsBlocked=True,fullscreen=True,mobile320=True)
   elif kind in ('bubbleparty','battlecity'):
    canvas_selector='[data-bubbleparty-canvas]' if kind=='bubbleparty' else '[data-battlecity-canvas]'
    browser('wait','[data-controller-input="'+kind+'"]')
    if kind=='battlecity':
     watch_browser('wait','[data-battlecity-tank="balanced"]');watch_browser('click','[data-battlecity-tank="balanced"]')
     browser('focus','[data-battlecity-tank="balanced"]');js('window.__pulseGamepad(0,{0:1},{},3)');wait(lambda:room()['game']['data']['tanks'][room()['you']]['selected'])
    else:
     watch_browser('wait','[data-bubbleparty-character-confirm]');watch_browser('click','[data-bubbleparty-character-confirm]')
     browser('wait','[data-bubbleparty-character-confirm]');browser('focus','[data-bubbleparty-character-confirm]');js('window.__pulseGamepad(0,{0:1},{},3)')
     wait(lambda:room()['game']['data']['players'][room()['you']]['selected'])
    wait(lambda:room()['game']['data']['phase']=='playing',50)
    browser('focus','[data-fullscreen-board]');pad(axes={'0':0,'1':0});time.sleep(.2)
    def own():
     r=room();return r['game']['data']['players' if kind=='bubbleparty' else 'tanks'][r['you']]
    def rendered():return js('({...document.querySelector('+json.dumps(canvas_selector)+').dataset})')
    before=own();first=rendered();moved=None;chosen=None
    for axis,direction in [('0',1),('0',-1),('1',1),('1',-1)]:
     pad(axes={axis:direction});time.sleep(.4);during=own();visual=rendered();pad(axes={axis:0});time.sleep(.1)
     if abs(float(visual['renderX'])-float(first['renderX']))+abs(float(visual['renderY'])-float(first['renderY']))>.08:
      moved=visual;chosen=(axis,direction);break
    assert moved and during['seq']>before['seq'],(first,rendered(),before,own())
    hardware_direction=chosen;chosen=(chosen[0],-chosen[1])
    direction_name={('0',1):'right',('0',-1):'left',('1',1):'down',('1',-1):'up'}[chosen]
    action='bomb' if kind=='bubbleparty' else 'fire'
    # Remap the movement actually available at this spawn through native UI.
    settings();browser('select','[data-controller-model]','game:'+kind);browser('select','select[aria-label="输入来源"]','keyboard')
    label={'right':'向右移动','left':'向左移动','up':'向上移动','down':'向下移动'}[direction_name]
    browser('click','button[aria-label="修改 '+label+' 按键"]');press_native('KeyH','h',72);save_settings();close_settings();wait(lambda:not js('!!document.querySelector(".controller-settings")'))
    browser('focus','[data-fullscreen-board]');time.sleep(.2);idle=own()['seq'];old_code={'right':'KeyD','left':'KeyA','up':'KeyW','down':'KeyS'}[direction_name]
    print('CHECK',kind,'old key',old_code,flush=True);press_native(old_code,old_code[-1].lower(),ord(old_code[-1]));time.sleep(.15);assert own()['seq']<=idle+1
    js('(()=>{const canvas=document.querySelector('+json.dumps(canvas_selector)+'),x=+canvas.dataset.renderX,y=+canvas.dataset.renderY;window.__inputLatency={began:performance.now()};const observer=new MutationObserver(()=>{const key=window.__nativeKeyEvents.find(e=>e.code==="KeyH"&&e.at>=window.__inputLatency.began);if(key&&Math.abs(+canvas.dataset.renderX-x)+Math.abs(+canvas.dataset.renderY-y)>.001){window.__inputLatency={...window.__inputLatency,event:key,elapsed:performance.now()-key.at};observer.disconnect();}});observer.observe(canvas,{attributes:true,attributeFilter:["data-render-x","data-render-y"]});return true})()')
    native_key(True,'KeyH','h',72);wait(lambda:own()['seq']>idle+1);held=own()['seq']
    latency=wait(lambda:js('window.__inputLatency.elapsed!==undefined&&window.__inputLatency'));assert latency['event']['trusted'] and latency['elapsed']<180,latency
    pad(axes={chosen[0]:chosen[1]});time.sleep(.15);js('window.__disconnectGamepad(0);true');disconnected=own()['seq'];time.sleep(.4);assert own()['seq']>disconnected
    native_key(False,'KeyH','h',72);pad();time.sleep(.2)
    events_before=room()['game']['data']['events'];last_id=max([e['id'] for e in events_before]+[0])
    pad({'0':1});time.sleep(.3);firing=room();pad({'0':0});time.sleep(.1)
    evidence=[e for e in firing['game']['data']['events'] if e['id']>last_id and e.get('seat')==firing['you']]
    assert evidence,(kind,firing['game']['data']['events'])
    settings();blocked=own()['seq'];pad({'0':1},axes={chosen[0]:chosen[1]});time.sleep(.3);assert own()['seq']<=blocked+1;pad({'0':0},axes={chosen[0]:0});close_settings()
    browser('click','.room-tools > button:first-child');browser('wait','[data-room-fullscreen]');shot(kind+'-fullscreen')
    browser('set','viewport','320','740');shot(kind+'-320');assert js('document.documentElement.scrollWidth<=innerWidth+1')
    stick_selector='[data-bubbleparty-dpad]' if kind=='bubbleparty' else '[data-battlecity-dpad]'
    browser('scrollintoview',stick_selector);time.sleep(.2)
    center=js('(()=>{const r=document.querySelector('+json.dumps(stick_selector)+').getBoundingClientRect();return {x:r.x+r.width/2,y:r.y+r.height/2,id:1}})()');moved_touch={**center,'x':center['x']+36}
    touch_before=own()['seq'];touch_start=rendered()
    native=subprocess.run(['node','scripts/controllers/gamepad/native-touch.mjs',browser('get','cdp-url')['cdpUrl'],base,json.dumps([{'type':'touchStart','touchPoints':[center],'wait':40},{'type':'touchMove','touchPoints':[moved_touch],'wait':300},{'type':'touchCancel','touchPoints':[],'wait':200}])],cwd=root,env=env,text=True,capture_output=True,timeout=20)
    assert native.returncode==0,native.stderr
    touch_samples=json.loads(native.stdout);assert own()['seq']>touch_before
    assert any(abs(float(s['render']['renderX'])-float(touch_start['renderX']))+abs(float(s['render']['renderY'])-float(touch_start['renderY']))>.01 for s in touch_samples),touch_samples
    save(kind+'-native-touch',touch_samples);shot(kind+'-320-controls');browser('press','Escape');browser('set','viewport','1280','1000')
    save(kind+'-input-evidence',{'before':before,'during':during,'firstVisual':first,'movedVisual':moved,'events':evidence,'hardwareDirection':hardware_direction,'keyboardDirection':direction_name,'nativeInputLatency':latency})
    check(kind,selectionWithPad=True,visibleMovement=True,acceptedInput=True,remapping=True,oldPrimaryDisabled=True,keyboardSurvivesDisconnect=True,actionEvents=True,settingsBlocked=True,fullscreen=True,mobile320=True,nativeTouch=True)
   elif kind in ('gomoku','splendor','doudizhu'):
    browser('wait','[data-controller-navigation="'+kind+'"]')
    def pulse(button):
     js('new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r)))')
     js('window.__pulseGamepad(0,'+json.dumps({str(button):1})+',{},3)');time.sleep(.1)
    if kind=='gomoku':
     wait(lambda:room()['game']['data']['canPlace'],30);before=room();browser('focus','[data-gomoku-cell="112"]');pulse(15)
     assert js('document.activeElement.dataset.gomokuCell')=='113'
     assert js('document.querySelector(".gomoku-board").dataset.selected')==''
     pulse(0);assert js('document.querySelector(".gomoku-board").dataset.selected')=='113'
     assert room()['game']['data']['ply']==before['game']['data']['ply']
     browser('focus','[data-gomoku-confirm]');pulse(0)
     wait(lambda:room()['game']['data']['ply']>before['game']['data']['ply'])
     settings();browser('select','[data-controller-model]','game:gomoku');browser('select','select[aria-label="输入来源"]','keyboard');browser('click','button[aria-label="修改 向右选择 按键"]');press_native('KeyH','h',72);save_settings();close_settings();wait(lambda:not js('!!document.querySelector("[role=dialog]")'))
     browser('focus','[data-gomoku-cell="112"]');press_native('ArrowRight','ArrowRight',39);assert js('document.activeElement.dataset.gomokuCell')=='112'
     press_native('KeyH','h',72);assert js('document.activeElement.dataset.gomokuCell')=='113'
     wait(lambda:room()['game']['data']['canPlace'],30);browser('wait','.gomoku-secondary button:last-child:not(:disabled)');browser('focus','.gomoku-secondary button:last-child');js('new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r)))');pulse(0);browser('wait','[role="dialog"]');pulse(1);wait(lambda:not js('!!document.querySelector("[role=dialog]")'))
     browser('click','.room-tools > button:first-child');browser('wait','[data-room-fullscreen]')
     browser('set','viewport','320','740');browser('focus','[data-gomoku-cell="112"]');pulse(13)
     assert js('document.activeElement.dataset.gomokuCell')=='127'
     box=js('(()=>{const r=document.activeElement.getBoundingClientRect();return {x:r.x,y:r.y,w:r.width,h:r.height,innerWidth,innerHeight}})()')
     assert box['x']>=0 and box['y']>=0 and box['x']+box['w']<=box['innerWidth']+1 and box['y']+box['h']<=box['innerHeight']+1,box
     shot('gomoku-navigation-320-fullscreen');browser('press','Escape');browser('set','viewport','1280','1000')
     check(kind,directionsOnlyFocus=True,separateConfirmation=True,visible320=True,fullscreen=True,remappedKeyboard=True,legacyKeyDisabled=True,dialogCancel=True)
    elif kind=='doudizhu':
     browser('wait','.poker-hand button[data-card]');cards=js('[...document.querySelectorAll(".poker-hand button[data-card]")].map(e=>e.dataset.card)')
     browser('focus','.poker-hand button[data-card]:first-child');pulse(15)
     assert js('document.activeElement.dataset.card')==cards[1]
     assert js('document.querySelectorAll(".poker-hand .chosen").length')==0
     pulse(0);assert js('document.querySelectorAll(".poker-hand .chosen").length')==1
     pulse(15);pulse(0);assert js('document.querySelectorAll(".poker-hand .chosen").length')==2
     time.sleep(.3);assert js('document.querySelectorAll(".poker-hand .chosen").length')==2
     pulse(1);assert js('document.querySelectorAll(".poker-hand .chosen").length')==0
     browser('set','viewport','320','740');browser('focus','.poker-hand-row:first-child button:first-child');pulse(13)
     assert js('document.activeElement===document.querySelectorAll(".poker-hand-row")[1].querySelector("button")')
     pulse(0);pulse(12);assert js('document.activeElement===document.querySelector(".poker-hand-row button")')
     shot('doudizhu-navigation-320');browser('set','viewport','1280','1000')
     check(kind,adjacentCards=True,multiSelection=True,noAutomaticPlay=True,cancelSelection=True,mobileRows=True)
    else:
     browser('wait','.dev-card[data-card-id]');browser('focus','.dev-card[data-card-id]');before=js('document.activeElement.dataset.cardId');pulse(15)
     assert js('document.activeElement.dataset.cardId')!=before
     pulse(0);browser('wait','.gem-purchase-panel');pulse(1);wait(lambda:not js('!!document.querySelector(".gem-purchase-panel")'))
     check(kind,cardFocus=True,confirmationOpensCardDetails=True,cancelClosesDetails=True)
   elif kind in ('basketball','contra'):
    if kind=='basketball':wait(lambda:room()['game']['data']['phase']=='playing',45)
    else:wait(lambda:room()['game']['data']['snapshot']['phase']=='playing',45)
    def mask():
     d=room()['game']['data'];return d['inputs'][d.get('localActor',d.get('actor',0))].get('held',d['inputs'][d.get('localActor',d.get('actor',0))].get('buttons',0))
    def visual():
     return js('({...document.querySelector(".basketball-board canvas")?.dataset})') if kind=='basketball' else js('({...document.querySelector("[data-contra-runtime]")?.dataset})')
    initial=room();first_visual=visual();pad(axes={'0':1});wait(lambda:mask()==1);time.sleep(.3);during=room();moved_visual=visual()
    key_j(True);wait(lambda:mask()==65)
    js('window.__disconnectGamepad(0);true');wait(lambda:mask()==64)
    key_j(False);wait(lambda:mask()==0)
    pad(axes={'0':1});time.sleep(.2);assert mask()==0,'reconnected held stick resumed input'
    pad(axes={'0':0});time.sleep(.15);pad(axes={'0':1});wait(lambda:mask()==1)
    # Open real settings while the stick remains held; closing cannot resume it.
    settings();wait(lambda:mask()==0);close_settings();time.sleep(.2);assert mask()==0
    pad(axes={'0':0});time.sleep(.15);pad(axes={'0':1});wait(lambda:mask()==1);pad(axes={'0':0});wait(lambda:mask()==0)
    # Native tab focus/visibility boundary. HTTP observer does not send input.
    observer=urllib.request.build_opener(urllib.request.HTTPCookieProcessor(http.cookiejar.CookieJar()))
    with observer.open(urllib.request.Request(base+'/api/login',data=json.dumps(credentials).encode(),headers={'Content-Type':'application/json'}),timeout=10) as response:assert response.status==200
    def other_mask():
     with observer.open(base+'/api/state',timeout=10) as response:d=json.load(response)['room']['game']['data']
     own=d['inputs'][d.get('localActor',d.get('actor',0))];return own.get('held',own.get('buttons',0))
    pad(axes={'0':1});wait(lambda:mask()==1);browser('tab','new','about:blank');wait(lambda:other_mask()==0)
    browser('tab','t1');browser('focus','[data-fullscreen-board]');time.sleep(.2);assert mask()==0
    browser('tab','close','t2');pad(axes={'0':0});time.sleep(.2)
    pad({'3':1});observed=[]
    for _ in range(12):observed.append(mask());time.sleep(.025)
    pad({'3':0});wait(lambda:mask()==0);assert 0 in observed and 128 in observed,observed
    # Real game position, not only an acknowledged mask, must respond.
    if kind=='basketball':assert float(moved_visual['playerWorldX'])!=float(first_visual['playerWorldX']),(first_visual,moved_visual)
    else:
     assert json.loads(moved_visual['visualPlayers'])[0]['x']!=json.loads(first_visual['visualPlayers'])[0]['x'],(first_visual,moved_visual)
    after=room();assert after['game']['data']['epoch']==initial['game']['data']['epoch']
    save(kind+'-snapshots',{'before':initial,'during':during,'after':after,'firstVisual':first_visual,'movedVisual':moved_visual,'turboMasks':observed})
    check(kind,authoritativeRight=True,visibleMovement=True,keyboardCoexistence=True,disconnect=True,neutralRearm=True,settingsBlocked=True,visibilityRelease=True,turbo=True,epochPreserved=True)
   else:
    wait(lambda:room()['game']['data']['phase']=='serve' and room()['game']['data']['controls']['canSwing'],30)
    before=room();seq=before['game']['data']['paddles'][before['you']]['seq']
    size=js('(()=>{const r=document.querySelector("[data-tabletennis-board] canvas").getBoundingClientRect();return {w:r.width,h:r.height}})()')
    # Aim at the visible serve ball using only directional gamepad input. This
    # projects the public serve contact through the production camera geometry;
    # no game state or rendered paddle position is injected.
    cy,cz,fov,targetY=(4.8,12,72,2.1) if size['w']<650 else (4.3,9.2,54,2.1)
    pitch=math.atan2(cy-targetY,cz+1.1);t=math.tan(math.radians(fov)/2);dy=1.1*2.75-cy;dz=1.28*2.75-cz;depth=-dy*math.sin(pitch)-dz*math.cos(pitch)
    target={'x':(.34*2.75/(depth*t*size['w']/size['h'])+1)/2,'y':(1-(dy*math.cos(pitch)-dz*math.sin(pitch))/(depth*t))/2}
    pulses=[]
    for axis,delta in [('0',target['x']-.5),('1',target['y']-.7)]:
     frames=round(abs(delta)/.0225)
     if frames:pulses.append(js('window.__pulseGamepad(0,{},'+json.dumps({axis:1 if delta>0 else -1})+','+str(frames)+')'))
    js('window.__pulseGamepad(0,{17:1},{},3)')
    wait(lambda:room()['game']['data']['paddles'][room()['you']]['seq']>seq)
    wait(lambda:room()['game']['data']['shot']>before['game']['data']['shot'])
    after=room();result=after['game']['data']['paddles'][after['you']]['result'];assert result!='miss',after['game']['data']['paddles']
    save(kind+'-snapshots',{'before':before,'after':after,'target':target,'axisPulses':pulses});check(kind,authoritativeStroke=True,physicalA=True,axisAiming=True,result=result)
   shot(kind+'-gamepad')
   if kind=='snooker':api('/login',credentials);browser('reload');browser('wait','.room-tools')
   cmd('close');cmd('leave')
   for opponent in sg_watchers:
    watch_env=opponent;watch_browser('close')
   sg_watchers=[]
   if watch_opened:watch_browser('close');watch_opened=False
   browser('reload');browser('wait','.room-list');pad()
 report['passed']=True
except BaseException:
 report['error']=traceback.format_exc();print(report['error'],flush=True)
 if opened:
  try:shot('failure');save('failure-state',{'snapshot':browser('snapshot','-i'),'errors':browser('errors'),'room':room(),'active':js('document.activeElement?.outerHTML.slice(0,500)'),'transport':js('({acks:window.__controllerAcks,sent:window.__controllerSentInputs})'),'hardware':js('({pads:window.__gamepadDevices,pulse:window.__gamepadPulse,samples:window.__gamepadSamples?.slice(-5),nav:document.querySelector("[data-controller-navigation]")?.dataset.controllerNavigation,focus:document.hasFocus(),hidden:document.hidden})')})
  except Exception:pass
finally:
 for opponent in sg_watchers:
  watch_env=opponent
  try:watch_browser('close')
  except Exception:report['passed']=False
 if watch_opened:
  try:watch_browser("close")
  except Exception:report["passed"]=False
 if opened:
  try:browser('close')
  except BaseException:report['passed']=False
 if server:
  (fixture_dir/'stop').write_text('stop')
  try:server.wait(timeout=20)
  except subprocess.TimeoutExpired:server.kill();server.wait();report['passed']=False
  if server.returncode!=0:report['passed']=False
 if log:log.close()
 for path in [temp,sockets,out/'data',out/'fixture']:
  if path.exists():shutil.rmtree(path)
 for name in ['url','stop']:(out/name).unlink(missing_ok=True)
 report['scope']='Production settings/HTTP/SQLite and real local rooms; virtual Gamepad API hardware source, native mouse/keyboard UI; physical hardware not certified.'
 report['sources']={str(p.relative_to(root)):hashlib.sha256(p.read_bytes()).hexdigest() for p in [root/'web/src/controllers/gamepad.ts',root/'web/src/controllers/game-controller.ts',root/'web/src/controllers/use-game-controller.ts',root/'scripts/controllers/gamepad/native-key.mjs',root/'scripts/controllers/gamepad/native-touch.mjs',root/'scripts/controllers/gamepad/virtual-device.js',root/'web/src/controllers/navigation.ts',root/'web/src/controllers/NavigationController.tsx',root/'internal/server/controller_models.go',root/'web/src/controllers/GamepadSettings.tsx',root/'web/src/controllers/Settings.tsx',root/'internal/server/controllers.go',root/'web/src/main.tsx',root/'web/src/style.css',root/'scripts/controllers/gamepad/check-browser.py']}
 save('report',report)
raise SystemExit(0 if report['passed'] else 1)
