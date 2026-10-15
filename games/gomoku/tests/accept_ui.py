#!/usr/bin/env python3
"""Gomoku real-browser E2E, authored before the board implementation.

Failures: unpublished game visible; missing cover/options; wrong pointer or
keyboard coordinate; drag/wheel commits a stone; clipped mobile board/tokens;
stale selection after an opponent move; lost state/replayed animation on reload;
missing/double sound or mute bypass; wrong win/draw labels; inaccessible actions.
"""
import http.cookiejar
import json
import os
from pathlib import Path
import subprocess
import time
import urllib.request
import uuid

root=Path(__file__).resolve().parents[3]
out=root/'.local/gomoku/browser';out.mkdir(parents=True,exist_ok=True)
env=dict(os.environ)
for key,rel in {'GOCACHE':'.local/go-build','GOMODCACHE':'.local/go-mod','GOTMPDIR':'.local/tmp','TMPDIR':'.local/tmp','npm_config_cache':'.local/npm','AGENT_BROWSER_SOCKET_DIR':'.local/ab-gomoku','AGENT_BROWSER_PROFILE':'.local/gomoku/browser/profile'}.items():
    p=root/rel;p.mkdir(parents=True,exist_ok=True);env[key]=str(p)
env.update(GOTOOLCHAIN='local',WIRE_BOARD_GOMOKU_BROWSER='1',AGENT_BROWSER_SESSION='gomoku-'+uuid.uuid4().hex[:8],AGENT_BROWSER_ARGS='--mute-audio,--disable-breakpad,--disable-crash-reporter')
report={'passed':False,'checks':[],'command':'. scripts/env.sh && python3 -B games/gomoku/tests/accept_ui.py'}
server=None;opened=False
def run(args,log):
    with (out/log).open('w') as f:subprocess.run(args,cwd=root,env=env,stdout=f,stderr=subprocess.STDOUT,check=True)
def b(*args):
    r=subprocess.run(['agent-browser',*args,'--json'],cwd=root,env=env,capture_output=True,text=True,timeout=40)
    if r.returncode:raise RuntimeError(r.stdout+r.stderr)
    v=json.loads(r.stdout);assert v['success'],v;return v['data']
def js(code):return b('eval',code)['result']
def api(path,body=None):
    opts='' if body is None else ',{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify('+json.dumps(body)+')}'
    r=js('fetch('+json.dumps('/api'+path)+opts+').then(async r=>({status:r.status,body:await r.json()}))');assert 200<=r['status']<300,r;return r['body']
def room():return api('/state')['room']
def command(kind,**extra):
    r=room();return api('/rooms/'+r['id'],dict(type=kind,version=r['version'],nonce=uuid.uuid4().hex,**extra))
def shot(name):
    assert not b('errors')['errors'],b('errors')
    assert js('document.documentElement.scrollWidth<=innerWidth+2'),'horizontal overflow'
    b('screenshot',str(out/(name+'.png')),'--full')
    (out/(name+'.state.json')).write_text(json.dumps(api('/state'),ensure_ascii=False,indent=2))
    contrast=js(r"""(() => {
      const rgb=s=>(s.match(/[\d.]+/g)||[]).map(Number),blend=(a,b,t)=>a.slice(0,3).map((v,i)=>v*t+b[i]*(1-t));
      const lum=c=>c.map(v=>v/255).map(v=>v<=.04045?v/12.92:((v+.055)/1.055)**2.4).reduce((s,v,i)=>s+v*[.2126,.7152,.0722][i],0);
      return [...document.querySelectorAll('.gomoku-heading p,.gomoku-heading strong,.gomoku-actions p,.gomoku-actions h3,.gomoku-actions button,.gomoku-map-tools button,.gomoku-player-summary small')].map(e=>{
        let chain=[],bg=[255,255,255];for(let n=e;n;n=n.parentElement)chain.unshift(n);for(const n of chain){const c=rgb(getComputedStyle(n).backgroundColor);bg=blend(c,bg,c[3]??1)}
        const a=lum(rgb(getComputedStyle(e).color)),b=lum(bg);return {text:e.textContent.trim(),ratio:(Math.max(a,b)+.05)/(Math.min(a,b)+.05)};
      });
    })()""")
    assert all(c['ratio']>=4.5 for c in contrast),contrast
    (out/(name+'.contrast.json')).write_text(json.dumps(contrast,ensure_ascii=False,indent=2))
    report['checks'].append(name);print('PASS',name,flush=True)
def click(name):b('find','role','button','click','--name',name,'--exact')
def choose(x,y):
    selector=f'[data-gomoku-cell="{y*15+x}"]'
    b('wait','--fn','document.querySelector('+json.dumps(selector)+').getAttribute("aria-disabled")==="false"')
    b('scrollintoview',selector)
    before=js('document.querySelector('+json.dumps(selector)+').getBoundingClientRect().toJSON()')
    b('mouse','move',str(round(before['x']+before['width']/2)),str(round(before['y']+before['height']/2)));b('mouse','down')
    pressed=js('document.querySelector('+json.dumps(selector)+').getBoundingClientRect().toJSON()')
    assert abs(before['x']-pressed['x'])<.1 and abs(before['y']-pressed['y'])<.1,'pressed-button styling moved the hit target'
    b('mouse','up')
    b('wait','--fn','!document.querySelector("[data-gomoku-confirm]").disabled')
    b('scrollintoview','[data-gomoku-confirm]');b('click','[data-gomoku-confirm]')
def ply(n):b('wait','--fn',f'document.querySelector(".gomoku-board")?.dataset.ply==="{n}"')
try:
    run(['npm','--prefix','web','run','build'],'build.log')
    for name in ['url','stop']:(out/name).unlink(missing_ok=True)
    with (out/'server.log').open('w') as f:server=subprocess.Popen(['go','test','-count=1','-timeout=13m','-run','^TestGomokuBrowserFixture$','./games/gomoku/tests'],cwd=root,env=env,stdout=f,stderr=subprocess.STDOUT)
    until=time.monotonic()+90
    while not (out/'url').exists():
        if server.poll() is not None or time.monotonic()>until:raise RuntimeError('See server.log')
        time.sleep(.1)
    base=(out/'url').read_text();b('open',base);opened=True;b('tab',next(t['tabId'] for t in b('tab')['tabs'] if t['active']))
    api('/login',{'name':'棋类验收','password':'gomoku-browser-only'})
    assert 'gomoku' not in api('/state')['availableGames']
    api('/admin/games/gomoku',{'enabled':True});b('reload');b('wait','.lobby-create');b('click','.lobby-create')
    b('wait','[role=dialog]');b('fill','input[aria-label=搜索可选游戏]','五子棋');b('click','.choose-games button')
    b('wait','select[aria-label=五子棋规则]')
    assert js('!!document.querySelector(".choose-games .selected svg")'),'missing illustrated cover'
    b('fill','input[name=name]','五子棋原生输入验收');b('scrollintoview','[role=dialog] button[type=submit]');b('click','[role=dialog] button[type=submit]');b('wait','.waiting-actions')
    peer=urllib.request.build_opener(urllib.request.HTTPCookieProcessor(http.cookiejar.CookieJar()))
    def peer_api(path,body=None):
        raw=None if body is None else json.dumps(body).encode()
        with peer.open(urllib.request.Request(base+'/api'+path,data=raw,headers={'Content-Type':'application/json'}),timeout=10) as r:return json.load(r)
    def peer_command(kind,**extra):
        r=room();return peer_api('/rooms/'+r['id'],dict(type=kind,version=r['version'],nonce=uuid.uuid4().hex,**extra))
    def peer_place(x,y):
        d=room()['game']['data'];peer_command('action',action={'type':'place','round':d['round'],'ply':d['ply'],'x':x,'y':y})
    peer_api('/register',{'name':'五子棋对手','password':'gomoku-browser-only','invite':'gomoku-browser'});peer_command('join');peer_command('ready');command('ready');command('start');b('wait','.gomoku-board')
    b('set','viewport','1440','1000');ply(0)
    js('window.__movesPlayed=[];const old=HTMLMediaElement.prototype.play;HTMLMediaElement.prototype.play=function(){const src=this.src;return old.call(this).then(()=>{if(src.includes("/gomoku/v1/audio/"))window.__movesPlayed.push(src)})}')
    # Native wheel zoom and left-button pan without placing a stone.
    b('scrollintoview','.gomoku-viewport')
    rect=js('document.querySelector(".gomoku-viewport").getBoundingClientRect().toJSON()');x=round(rect['x']+rect['width']/2);y=round(rect['y']+rect['height']/2)
    before=room()['version'];endpoint=b('get','cdp-url')['cdpUrl']
    subprocess.run(['node','scripts/browser_wheel.mjs',endpoint,base,str(x),str(y),'-250'],cwd=root,env=env,check=True,timeout=20)
    b('wait','--fn','Number(document.querySelector(".gomoku-board").dataset.zoom)>1')
    b('mouse','move',str(x),str(y));b('mouse','down');b('mouse','move',str(x-60),str(y-40));b('mouse','up')
    assert room()['version']==before,'pan submitted a move'
    assert js('document.querySelector(".gomoku-viewport").scrollLeft>0')
    mini=js('document.querySelector(".gomoku-minimap").getBoundingClientRect().toJSON()');mx=round(mini['x']+mini['width']*.25);my=round(mini['y']+mini['height']*.25)
    b('mouse','move',str(mx),str(my));b('mouse','down');b('mouse','move',str(round(mini['x']+mini['width']*.8)),str(round(mini['y']+mini['height']*.8)));b('mouse','up')
    assert js('document.querySelector(".gomoku-viewport").scrollTop>0'),'minimap drag did not locate board'
    assert room()['version']==before
    click('复位视图');b('wait','--fn','document.querySelector(".gomoku-board").dataset.zoom==="1"')
    # Keyboard coordinates: right from H8 selects I8, then explicit confirm.
    b('scrollintoview','[data-gomoku-cell="112"]');b('click','[data-gomoku-cell="112"]');b('press','ArrowRight');b('press','Enter')
    assert room()['game']['data']['ply']==0
    b('click','[data-gomoku-confirm]');ply(1)
    assert room()['game']['data']['board'][113]==1
    b('wait','--fn','window.__movesPlayed.length===1');peer_place(0,14);ply(2)
    shot('desktop-native-wheel-pan-keyboard')
    # Clear/reload suppresses animation and historic media playback.
    snapshot=room()['game']['data']
    if env.get('WIRE_BOARD_GOMOKU_ASSETS'):
        b('network','route',env['WIRE_BOARD_GOMOKU_ASSETS'].rstrip('/')+'/gomoku/v1/**','--abort')
    if env.get('WIRE_BOARD_GOMOKU_ASSETS'):
        subprocess.run(['node','scripts/browser_reload_uncached.mjs',endpoint,base],cwd=root,env=env,check=True,timeout=25)
    else:b('reload')
    b('wait','.gomoku-board');assert room()['game']['data']==snapshot
    assert js('document.querySelectorAll(".gomoku-stone.arriving").length===0')
    if env.get('WIRE_BOARD_GOMOKU_ASSETS'):
        b('wait','--fn','document.querySelectorAll(".gomoku-stone img").length===2&&[...document.querySelectorAll(".gomoku-stone img")].every(e=>new URL(e.currentSrc).origin===location.origin&&e.complete&&e.naturalWidth>0)')
        b('wait','--fn','document.querySelector(".gomoku-grid image").getAttribute("href").startsWith("/gomoku/v1/")')
        shot('cdn-failure-local-image-fallback')
    js('window.__fallbackPlayed=[];const old=HTMLMediaElement.prototype.play;HTMLMediaElement.prototype.play=function(){const src=this.src;return old.call(this).then(()=>{if(src.startsWith(location.origin+"/gomoku/v1/audio/"))window.__fallbackPlayed.push(src)})}')
    for width in [390,320]:
        b('set','viewport',str(width),'844' if width==390 else '740')
        b('wait','--fn','document.querySelector(".gomoku-map").getBoundingClientRect().width>=480')
        assert js('[...document.querySelectorAll(".gomoku-stone")].every(e=>e.getBoundingClientRect().width>=24)'),'unreadable stones'
        shot('mobile-'+str(width))
    b('set','media','light','reduced-motion')
    # Touch on a visible native target selects only; confirmation commits once.
    b('scrollintoview','[data-gomoku-cell="112"]');r=js('document.querySelector("[data-gomoku-cell=\\"112\\"]").getBoundingClientRect().toJSON()');x=r['x']+r['width']/2;y=r['y']+r['height']/2
    subprocess.run(['node','scripts/browser_touch.mjs',endpoint,base,'drag',str(x),str(y),str(x+45),str(y+35)],cwd=root,env=env,check=True,timeout=20)
    assert room()['game']['data']['ply']==2 and js('document.querySelector(".gomoku-board").dataset.selected===""'),'touch pan selected or placed a stone'
    b('scrollintoview','[data-gomoku-cell="112"]');r=js('document.querySelector("[data-gomoku-cell=\\"112\\"]").getBoundingClientRect().toJSON()');x=r['x']+r['width']/2;y=r['y']+r['height']/2
    js('window.__touchEvents=[];for(const type of ["pointerdown","pointermove","pointerup","pointercancel","gotpointercapture","lostpointercapture","click","touchstart","touchend"])document.addEventListener(type,e=>window.__touchEvents.push({type,cell:e.target.closest?.("[data-gomoku-cell]")?.dataset.gomokuCell,x:e.clientX,y:e.clientY,pointer:e.pointerType,target:e.target.tagName,cls:e.target.className,point:document.querySelectorAll(".gomoku-cell")[111]?.getBoundingClientRect().toJSON(),buttons:e.buttons,capture:document.querySelector(".gomoku-viewport").hasPointerCapture(e.pointerId||0),scroll:[document.querySelector(".gomoku-viewport").scrollLeft,document.querySelector(".gomoku-viewport").scrollTop],disabled:e.target.closest?.("[data-gomoku-cell]")?.getAttribute("aria-disabled"),selected:document.querySelector(".gomoku-board").dataset.selected}),true)')
    report['touchTarget']={'rect':r,'x':x,'y':y,'hit':js(f'document.elementFromPoint({x},{y})?.closest("[data-gomoku-cell]")?.dataset.gomokuCell')}
    subprocess.run(['node','scripts/browser_touch.mjs',endpoint,base,'tap',str(x),str(y),str(x),str(y)],cwd=root,env=env,check=True,timeout=20)
    b('wait','--fn','document.querySelector(".gomoku-board").dataset.selected==="112"')
    assert room()['game']['data']['ply']==2
    click('静音');b('scrollintoview','[data-gomoku-confirm]');b('click','[data-gomoku-confirm]');ply(3)
    assert room()['game']['data']['board'][112]==1
    assert js('document.querySelectorAll(".gomoku-stone.arriving").length===0'),'reduced motion still animated'
    assert js('document.querySelector("audio[data-platform-audio]").paused'),'muted move played audio'
    shot('mobile-native-touch-confirm-muted');peer_place(2,14);ply(4)
    click('取消静音')
    # Finish a genuine line through the normal UI and opponent HTTP actions.
    for own,other,n in [(6,4,5),(5,6,7),(4,None,9)]:
        choose(own,7);ply(n)
        if other is not None:peer_place(other,14);ply(n+1)
    b('wait','.results');assert room()['game']['data']['reason']=='five'
    assert room()['game']['winners']==[0]
    assert js('document.querySelector(".gomoku-win-line")!==null')
    b('wait','--fn','window.__fallbackPlayed.length>0')
    report['localAudioFallback']=js('window.__fallbackPlayed')
    shot('natural-win-mobile');report['passed']=True
except Exception as e:
    import traceback
    traceback.print_exc();report['error']=repr(e)
    if opened:
        try:report['images']=js('[...document.querySelectorAll(".gomoku-stone img")].map(e=>({src:e.src,current:e.currentSrc,width:e.naturalWidth,complete:e.complete}))');report['touchEvents']=js('window.__touchEvents||[]');report['state']=api('/state');report['snapshot']=b('snapshot','-i');b('screenshot',str(out/'failure.png'),'--full')
        except Exception:pass
finally:
    if opened:
        try:b('close')
        except Exception:pass
    if server:
        (out/'stop').write_text('stop')
        try:server.wait(timeout=15)
        except subprocess.TimeoutExpired:server.terminate();server.wait(timeout=10)
    (out/'result.json').write_text(json.dumps(report,ensure_ascii=False,indent=2)+'\n');print('Artifact:',out/'result.json',flush=True)
    if not report['passed']:raise SystemExit(1)
