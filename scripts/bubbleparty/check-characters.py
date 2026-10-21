#!/usr/bin/env python3
"""Character selection through the real registry/HTTP/SQLite game and browser.
Failure inventory (recorded before UI): missing pre-match chooser or attributes;
confirmed choice cannot be changed; desktop/mobile controls obscured; refresh
loses confirmation; match-time changes allowed; timeout enables autoplay;
hidden characters exposed; blocked images lose distinct identities.
The original detailed plan is retained in .local/bubble-characters/E2E-SCENARIOS.md.
No game-state injection. Native mouse/touch; only public HTTP actions for peers.
"""
import argparse, hashlib, http.cookiejar, json, os, shutil, subprocess, time, traceback, urllib.request, uuid
from pathlib import Path
root = Path(__file__).resolve().parents[2]
p = argparse.ArgumentParser(); p.add_argument('--out', required=True); p.add_argument('--fallback-only',action='store_true'); args = p.parse_args()
out = (root / args.out).resolve(); assert out.is_relative_to(root) and out != root and not out.exists(); out.mkdir(parents=True)
temp = out/'temporary'; run='chars'+uuid.uuid4().hex[:6]; sockets=root/'.s'/run
env=dict(os.environ)
for key,path in {'TMPDIR':temp/'tmp','GOTMPDIR':temp/'tmp','GOCACHE':root/'.local/go-build','GOMODCACHE':root/'.local/go-mod','AGENT_BROWSER_SOCKET_DIR':sockets,'AGENT_BROWSER_PROFILE':temp/'profile'}.items():
    path.mkdir(parents=True,exist_ok=True);env[key]=str(path)
env.update(AGENT_BROWSER_SESSION=run,AGENT_BROWSER_ARGS='--mute-audio,--disable-breakpad,--disable-crash-reporter',WIRE_BOARD_BUBBLEPARTY_BROWSER='1',WIRE_BOARD_BUBBLEPARTY_BROWSER_OUT=str(out),GOTOOLCHAIN='local')
report={'passed':False,'checks':[],'screenshots':[],'scope':'real HTTP/SQLite; original character profiles; native browser mouse/touch','distSHA256':hashlib.sha256((root/'web/dist/index.html').read_bytes()).hexdigest()}
server=log=None; opened=False; clients={}

def browser(*args):
    r=subprocess.run(['agent-browser',*args,'--json'],cwd=root,env=env,text=True,capture_output=True,timeout=45)
    assert r.returncode==0,r.stdout+r.stderr
    obj=json.loads(r.stdout);assert obj['success'],obj
    return obj['data']
def js(code):return browser('eval',code)['result']
def api(path,body=None):
    opts='' if body is None else ',{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify('+json.dumps(body)+')}'
    r=js('(async()=>{const r=await fetch('+json.dumps('/api'+path)+opts+');return {status:r.status,data:await r.json()}})()')
    assert r['status'] in [200,201],r
    return r['data']
def request(client,path,body=None):
    req=urllib.request.Request(base+'/api'+path,data=None if body is None else json.dumps(body).encode(),headers={'Content-Type':'application/json'})
    with client.open(req,timeout=10) as response:return json.load(response)
def room():return api('/state')['room']
def view():return room()['game']['data']
def command(kind,**extra):
    r=room();return api('/rooms/'+r['id'],{'type':kind,'version':r['version'],'nonce':uuid.uuid4().hex,**extra})
def other_command(client,kind,**extra):
    r=request(client,'/state')['room'];return request(client,'/rooms/'+r['id'],{'type':kind,'version':r['version'],'nonce':uuid.uuid4().hex,**extra})
def wait(fn,timeout=10):
    end=time.monotonic()+timeout
    while time.monotonic()<end:
        result=fn()
        if result:return result
        time.sleep(.04)
    raise AssertionError('condition timed out')
def check(name,**values):report['checks'].append(dict(name=name,**values));print('PASS',name,flush=True)
def shot(name):browser('screenshot',str(out/(name+'.png')),'--full');report['screenshots'].append(name+'.png')
def layout():
    return js('''(()=>{const modal=document.querySelector('.bubble-character-modal'),r=modal.getBoundingClientRect();return {width:innerWidth,scrollWidth:document.documentElement.scrollWidth,modal:{left:r.left,right:r.right,top:r.top,bottom:r.bottom},cards:[...document.querySelectorAll('[data-bubbleparty-character]')].map(b=>({id:Number(b.dataset.bubblepartyCharacter),text:b.innerText,selected:b.getAttribute('aria-pressed')})),confirm:document.querySelector('[data-bubbleparty-character-confirm]')?.innerText}})()''')
def contrast():
    return js('''(()=>{const rgb=s=>s.match(/[0-9.]+/g).slice(0,3).map(Number),l=c=>{const a=c.map(n=>{const v=n/255;return v<=.04045?v/12.92:((v+.055)/1.055)**2.4});return .2126*a[0]+.7152*a[1]+.0722*a[2]},bg=e=>{for(let n=e;n;n=n.parentElement){const s=getComputedStyle(n).backgroundColor;if(s!=='rgba(0, 0, 0, 0)'&&s!=='transparent')return s}return 'rgb(255,255,255)'};return ['.bubble-character-note','.bubble-character-deadline','.bubble-character-grid strong','.bubble-character-stats dt','.bubble-character-stats dd','.bubble-character-stats dd span','.bubble-character-selected','.bubble-character-confirm'].map(sel=>{const e=document.querySelector(sel),fg=getComputedStyle(e).color,b=bg(e),a=l(rgb(fg)),c=l(rgb(b));return {selector:sel,foreground:fg,background:b,ratio:(Math.max(a,c)+.05)/(Math.min(a,c)+.05)}})})()''')
TOUCH=r'''const [endpoint,base,selector]=process.argv.slice(2),ws=new WebSocket(endpoint);await new Promise((r,j)=>{ws.onopen=r;ws.onerror=j});let id=0;const pending=new Map();ws.onmessage=({data})=>{const m=JSON.parse(data),p=pending.get(m.id);if(!p)return;pending.delete(m.id);m.error?p.j(Error(JSON.stringify(m.error))):p.r(m.result)};const call=(method,params={},sessionId)=>new Promise((r,j)=>{const n=++id;pending.set(n,{r,j});ws.send(JSON.stringify({id:n,method,params,sessionId}))});let sessionId;try{const {targetInfos}=await call('Target.getTargets');const t=targetInfos.find(t=>t.type==='page'&&t.url.startsWith(base));({sessionId}=await call('Target.attachToTarget',{targetId:t.targetId,flatten:true}));await call('Emulation.setTouchEmulationEnabled',{enabled:true,maxTouchPoints:5},sessionId);const v=await call('Runtime.evaluate',{expression:`(()=>{const e=document.querySelector(${JSON.stringify(selector)}),r=e.getBoundingClientRect();const x=r.x+r.width/2,y=r.y+r.height/2;if(document.elementFromPoint(x,y)!==e&&!e.contains(document.elementFromPoint(x,y)))throw Error('touch target obscured');return {x,y}})()`,returnByValue:true},sessionId);if(v.exceptionDetails)throw Error(JSON.stringify(v.exceptionDetails));await call('Input.dispatchTouchEvent',{type:'touchStart',touchPoints:[{...v.result.value,id:1,radiusX:4,radiusY:4,force:1}]},sessionId);await call('Input.dispatchTouchEvent',{type:'touchEnd',touchPoints:[]},sessionId)}finally{if(sessionId)await call('Target.detachFromTarget',{sessionId});ws.close()}'''
BLOCK_CHARACTERS=r'''const [endpoint,base]=process.argv.slice(2),ws=new WebSocket(endpoint);await new Promise((r,j)=>{ws.onopen=r;ws.onerror=j});let id=0;const pending=new Map(),requests=new Map(),failed=[];ws.onmessage=({data})=>{const m=JSON.parse(data);if(m.method==='Network.requestWillBeSent')requests.set(m.params.requestId,m.params.request.url);if(m.method==='Network.loadingFailed'&&m.params.blockedReason==='inspector'){const url=requests.get(m.params.requestId);if(url?.includes('/bubbleparty/v4/characters/'))failed.push(url)}const p=pending.get(m.id);if(!p)return;pending.delete(m.id);m.error?p.j(Error(JSON.stringify(m.error))):p.r(m.result)};const call=(method,params={},sessionId)=>new Promise((r,j)=>{const n=++id;pending.set(n,{r,j});ws.send(JSON.stringify({id:n,method,params,sessionId}))});let sessionId;try{const {targetInfos}=await call('Target.getTargets');const t=targetInfos.find(t=>t.type==='page'&&t.url.startsWith(base));({sessionId}=await call('Target.attachToTarget',{targetId:t.targetId,flatten:true}));await call('Network.enable',{},sessionId);await call('Network.setCacheDisabled',{cacheDisabled:true},sessionId);await call('Network.setBlockedURLs',{urls:['*bubbleparty/v4/characters/*']},sessionId);await call('Page.reload',{ignoreCache:true},sessionId);await new Promise(r=>setTimeout(r,3000));console.log(JSON.stringify({failed}));}finally{if(sessionId)await call('Target.detachFromTarget',{sessionId});ws.close()}'''

def touch(selector):
    browser('scrollintoview',selector)
    r=subprocess.run(['node','--input-type=module','-',browser('get','cdp-url')['cdpUrl'],base,selector],input=TOUCH,cwd=root,env=env,text=True,capture_output=True,timeout=20)
    assert r.returncode==0,r.stdout+r.stderr

def make_match(label,width):
    browser('set','viewport',str(width),'844' if width==320 else '1000')
    api('/register',{'name':'选角验收'+label,'password':'character-test-only','invite':'bubbleparty-browser'})
    made=api('/rooms',{'kind':'bubbleparty','name':'原版角色'+label,'capacity':2,'options':{'map':'community10','roundSeconds':300}})
    other=urllib.request.build_opener(urllib.request.HTTPCookieProcessor(http.cookiejar.CookieJar()))
    request(other,'/register',{'name':'选角对手'+label,'password':'character-test-only','invite':'bubbleparty-browser'})
    request(other,'/rooms/'+made['id'],{'type':'join','version':room()['version'],'nonce':uuid.uuid4().hex})
    other_command(other,'ready');command('ready');command('start')
    browser('reload');browser('wait','.bubble-character-modal')
    assert view()['phase']=='select'
    return other
try:
    log=(out/'server.log').open('w');server=subprocess.Popen(['go','test','-count=1','-timeout=12m','-run','^TestBubblepartyBrowserFixture$','./games/bubbleparty/tests'],cwd=root,env=env,stdout=log,stderr=subprocess.STDOUT)
    wait(lambda:(out/'url').exists(),120);base=(out/'url').read_text()
    browser('open',base);opened=True
    if not args.fallback_only:
        other=make_match('桌面',1280)
        initial=view();assert (initial.get('rulesVersion') or 0)>=3
        assert [p['character'] for p in initial['players']]==[0,1]
        cards=layout();assert [c['id'] for c in cards['cards']]==list(range(8)),cards
        assert '皮蛋' in cards['cards'][0]['text'] and '宝宝' in cards['cards'][6]['text']
        ratios=contrast();assert all(r['ratio']>=4.5 for r in ratios),ratios
        shot('desktop-select-eight-characters')
        browser('click','[data-bubbleparty-character="2"]')
        wait(lambda:view()['players'][0]['character']==2)
        assert not view()['players'][0].get('selected',False)
        browser('click','[data-bubbleparty-character-confirm]')
        wait(lambda:view()['players'][0].get('selected',False))
        browser('click','[data-bubbleparty-character-open]');browser('wait','.bubble-character-modal')
        browser('click','[data-bubbleparty-character="0"]')
        wait(lambda:view()['players'][0]['character']==0 and not view()['players'][0].get('selected',False))
        browser('click','[data-bubbleparty-character-confirm]')
        wait(lambda:view()['players'][0].get('selected',False))
        other_command(other,'action',action={'type':'character','epoch':view()['epoch'],'character':6,'confirm':True})
        ready=wait(lambda:(lambda v:v if v['phase']=='countdown' else None)(view()))
        assert ready['startAt']-ready['serverAt']<=3000
        wait(lambda:view()['phase']=='playing',5)
        started=view();a,b=started['players'];assert a['character']==0 and b['character']==6
        assert a['nativeSpeed']==5 and b['nativeSpeed']==5 and a['maxBombs']==b['maxBombs']==1 and a['range']==b['range']==1
        check('desktop selection confirms, changes clear confirmation, all-confirm starts separate countdown',layout=cards,contrast=ratios,initial=initial['players'],started=started['players'])
        browser('click','[data-bubbleparty-character-open]');browser('wait','.bubble-character-modal')
        assert js('document.querySelectorAll("[data-bubbleparty-character]").length')==0
        shot('desktop-character-readonly')
        browser('click','.bubble-character-modal .modal-head button[aria-label=关闭]')
        command('close')
        other=make_match('手机',320)
        mobile_ratios=contrast();assert all(r['ratio']>=4.5 for r in mobile_ratios),mobile_ratios
        mobile=layout();assert mobile['scrollWidth']<=320 and mobile['modal']['left']>=0 and mobile['modal']['right']<=320,mobile
        shot('mobile-select-eight-characters')
        touch('[data-bubbleparty-character="1"]')
        wait(lambda:view()['players'][0]['character']==1)
        browser('click','.bubble-character-modal .modal-head button[aria-label=关闭]')
        time.sleep(.5);assert not js('!!document.querySelector(".bubble-character-modal")')
        touch('[data-bubbleparty-character-open]');browser('wait','.bubble-character-modal')
        touch('[data-bubbleparty-character-confirm]')
        wait(lambda:view()['players'][0].get('selected',False))
        browser('reload');browser('wait','.bubble-board')
        assert not js('!!document.querySelector(".bubble-character-modal")')
        chosen=view()['players'][0];assert chosen['character']==1 and chosen['selected']
        other_command(other,'action',action={'type':'character','epoch':view()['epoch'],'character':2,'confirm':True})
        wait(lambda:view()['phase']=='playing',6)
        players=view()['players'];assert players[0]['nativeSpeed']==4 and players[0]['maxBombs']==2 and players[0]['range']==1
        assert players[1]['nativeSpeed']==4 and players[1]['maxBombs']==1 and players[1]['range']==2
        shot('mobile-selected-character-in-game')
        check('320px native touch and reload preserve character/confirmation; source bubble and range differences',layout=mobile,contrast=mobile_ratios,players=players)
        command('close')
        other=make_match('到期',320)
        before=view();assert before['selectUntil']>before['serverAt']
        browser('click','.bubble-character-modal .modal-head button[aria-label=关闭]')
        wait(lambda:view()['phase']=='playing',26)
        r=room();assert all(not seat.get('autoPlay',False) for seat in r['seats'])
        assert [p['character'] for p in r['game']['data']['players']]==[0,1]
        check('unconfirmed selection deadline uses current defaults and never enables autoplay',selectUntil=before['selectUntil'],started=r['game']['data']['startAt'],seats=r['seats'])
    if not args.fallback_only: command('close')
    other=make_match('素材回退',320)
    endpoint=browser('get','cdp-url')['cdpUrl']
    blocked=subprocess.run(['node','--input-type=module','-',endpoint,base],input=BLOCK_CHARACTERS,cwd=root,env=env,text=True,capture_output=True,timeout=20)
    assert blocked.returncode==0,blocked.stdout+blocked.stderr
    blocked=json.loads(blocked.stdout);assert len(blocked['failed'])>=8,blocked
    browser('wait','.bubble-character-modal')
    pixels=js('[...document.querySelectorAll("[data-bubbleparty-character] canvas")].map(c=>c.toDataURL())')
    hashes=[hashlib.sha256(image.encode()).hexdigest() for image in pixels]
    assert len(hashes)==8 and len(set(hashes))==8,hashes
    shot('mobile-eight-distinct-fallback-portraits')
    check('blocked character assets keep eight distinct team-colored fallback portraits',failedRequests=len(blocked['failed']),pixelHashes=hashes)
    assert not browser('errors')['errors']
    report['passed']=True
except BaseException:
    report['error']=traceback.format_exc();print(report['error'],flush=True)
    if opened:
        try:shot('failure');report['lastRoom']=room()
        except BaseException:pass
finally:
    if opened:
        try:browser('close')
        except BaseException:report['passed']=False
    if server:
        (out/'stop').write_text('stop')
        try:server.wait(timeout=20)
        except subprocess.TimeoutExpired:server.kill();server.wait();report['passed']=False
        if server.returncode:report['passed']=False
    if log:log.close()
    report['cleaned']=[]
    for path in [temp,sockets,out/'data']:
        if path.exists():shutil.rmtree(path);report['cleaned'].append(str(path.relative_to(root)))
    for name in ['url','stop']:(out/name).unlink(missing_ok=True)
    (out/'report.json').write_text(json.dumps(report,ensure_ascii=False,indent=2)+'\n');print(out/'report.json',flush=True)
    if not report['passed']:raise SystemExit(1)
