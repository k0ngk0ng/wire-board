#!/usr/bin/env python3
"""Chess real-browser E2E, authored before the board implementation.

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
out=root/'.local/chess/browser';out.mkdir(parents=True,exist_ok=True)
env=dict(os.environ)
for key,rel in {'GOCACHE':'.local/go-build','GOMODCACHE':'.local/go-mod','GOTMPDIR':'.local/tmp','TMPDIR':'.local/tmp','npm_config_cache':'.local/npm','AGENT_BROWSER_SOCKET_DIR':'.local/ab-chess','AGENT_BROWSER_PROFILE':'.local/chess/browser/profile'}.items():
    p=root/rel;p.mkdir(parents=True,exist_ok=True);env[key]=str(p)
env.update(GOTOOLCHAIN='local',WIRE_BOARD_CHESS_BROWSER='1',AGENT_BROWSER_SESSION='chess-'+uuid.uuid4().hex[:8],AGENT_BROWSER_ARGS='--mute-audio,--disable-breakpad,--disable-crash-reporter')
report={'passed':False,'checks':[],'command':'. scripts/env.sh && python3 -B games/chess/tests/accept_ui.py'}
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
      return [...document.querySelectorAll('.chess-heading p,.chess-heading strong,.chess-actions p,.chess-actions h3,.chess-actions button,.chess-player-summary small')].map(e=>{
        let chain=[],bg=[255,255,255];for(let n=e;n;n=n.parentElement)chain.unshift(n);for(const n of chain){const c=rgb(getComputedStyle(n).backgroundColor);bg=blend(c,bg,c[3]??1)}
        const a=lum(rgb(getComputedStyle(e).color)),b=lum(bg);return {text:e.textContent.trim(),ratio:(Math.max(a,b)+.05)/(Math.min(a,b)+.05)};
      });
    })()""")
    assert all(c['ratio']>=4.5 for c in contrast),contrast
    (out/(name+'.contrast.json')).write_text(json.dumps(contrast,ensure_ascii=False,indent=2))
    report['checks'].append(name);print('PASS',name,flush=True)
def click(name):b('find','role','button','click','--name',name,'--exact')
def ply(n):b('wait','--fn',f'document.querySelector(".chess-board")?.dataset.ply==="{n}"&&!document.querySelector(".chess-board").dataset.moving')
def select(square):
    sel=f'[data-chess-square="{square}"]';b('scrollintoview',sel)
    r=js('document.querySelector('+json.dumps(sel)+').getBoundingClientRect().toJSON()')
    b('mouse','move',str(round(r['x']+r['width']/2)),str(round(r['y']+r['height']/2)));b('mouse','down')
    after=js('document.querySelector('+json.dumps(sel)+').getBoundingClientRect().toJSON()')
    assert abs(r['x']-after['x'])<.1 and abs(r['y']-after['y'])<.1,'active styling moved hit target'
    b('mouse','up')
def move(fr,to,promotion=None):
    before=room()['game']['data']['ply'];select(fr);select(to)
    assert room()['game']['data']['ply']==before,'move before confirmation'
    if promotion:click(promotion)
    b('click','[data-chess-confirm]');ply(before+1)
try:
    run(['npm','--prefix','web','run','build'],'build.log')
    for name in ['url','stop']:(out/name).unlink(missing_ok=True)
    with (out/'server.log').open('w') as f:server=subprocess.Popen(['go','test','-count=1','-timeout=13m','-run','^TestChessBrowserFixture$','./games/chess/tests'],cwd=root,env=env,stdout=f,stderr=subprocess.STDOUT)
    until=time.monotonic()+90
    while not (out/'url').exists():
        if server.poll() is not None or time.monotonic()>until:raise RuntimeError('See server.log')
        time.sleep(.1)
    base=(out/'url').read_text();b('open',base);opened=True;b('tab',next(t['tabId'] for t in b('tab')['tabs'] if t['active']))
    api('/login',{'name':'棋类验收','password':'chess-browser-only'})
    assert 'chess' not in api('/state')['availableGames']
    api('/admin/games/chess',{'enabled':True});b('reload');b('wait','.lobby-create');b('click','.lobby-create')
    b('wait','[role=dialog]');b('fill','input[aria-label=搜索可选游戏]','国际象棋');b('click','.choose-games button')
    b('wait','select[aria-label=国际象棋电脑难度]')
    assert js('!!document.querySelector(".choose-games .selected svg")'),'missing illustrated cover'
    shot('create-cover')
    b('fill','input[name=name]','国际象棋原生输入验收');b('scrollintoview','[role=dialog] button[type=submit]');b('click','[role=dialog] button[type=submit]');b('wait','.waiting-actions')
    peer=urllib.request.build_opener(urllib.request.HTTPCookieProcessor(http.cookiejar.CookieJar()))
    def peer_api(path,body=None):
        raw=None if body is None else json.dumps(body).encode()
        with peer.open(urllib.request.Request(base+'/api'+path,data=raw,headers={'Content-Type':'application/json'}),timeout=10) as r:return json.load(r)
    def peer_command(kind,**extra):
        r=room();return peer_api('/rooms/'+r['id'],dict(type=kind,version=r['version'],nonce=uuid.uuid4().hex,**extra))
    def peer_move(uci):
        d=room()['game']['data'];peer_command('action',action={'type':'move','round':d['round'],'ply':d['ply'],'uci':uci})
    peer_api('/register',{'name':'国际象棋对手','password':'chess-browser-only','invite':'chess-browser'});peer_command('join');peer_command('ready');command('ready');command('start');b('wait','.chess-board')
    b('set','viewport','1440','1000');ply(0)
    b('wait','--fn','[...document.querySelectorAll(".chess-piece img")].every(e=>e.complete&&e.naturalWidth>0)')
    js('window.__motion=[];window.__sounds=[];new MutationObserver(()=>{const e=document.querySelector(".chess-board");if(e?.dataset.moving)window.__motion.push({id:e.dataset.moving,pieces:document.querySelectorAll(".chess-moving").length,captured:document.querySelector(".chess-captured")?.textContent,results:!!document.querySelector(".results")})}).observe(document.querySelector(".chess-board"),{attributes:true,childList:true,subtree:true});const old=HTMLMediaElement.prototype.play;HTMLMediaElement.prototype.play=function(){window.__sounds.push(this.src);return old.call(this)}')
    mode=env.get('WIRE_BOARD_CHESS_CASE','natural')
    if mode=='natural':
        # Keyboard chooses source; native mouse chooses destination.
        select('e2');b('press','ArrowUp');b('press','Enter');b('click','[data-chess-confirm]');ply(1)
        assert room()['game']['data']['board'][20]=='wP'
        peer_move('e7e5');ply(2)
        for fr,to,reply in [('g1','f3','b8c6'),('f1','e2','g8f6')]:move(fr,to);peer_move(reply);ply(room()['game']['data']['ply'])
        move('e1','g1');assert room()['game']['data']['board'][5]=='wR'
        assert js('window.__motion.some(e=>e.pieces===2)'),'castle did not move rook and king together'
        shot('desktop-native-keyboard-castle')
    elif mode=='ep':
        move('e5','d6');assert room()['game']['data']['captured'][0]==['bP'];assert js('window.__motion.length>0');shot('en-passant')
    elif mode=='promotion':
        move('a7','a8','升变为后');assert room()['game']['data']['board'][56]=='wQ';shot('promotion')
    else:raise ValueError(mode)
    snapshot=room()['game']['data']
    if env.get('WIRE_BOARD_CHESS_ASSETS'):
        b('network','route',env['WIRE_BOARD_CHESS_ASSETS'].rstrip('/')+'/chess/v1/**','--abort')
        subprocess.run(['node','scripts/browser_reload_uncached.mjs',b('get','cdp-url')['cdpUrl'],base],cwd=root,env=env,check=True,timeout=25)
    else:b('reload')
    b('wait','.chess-board')
    if env.get('WIRE_BOARD_CHESS_ASSETS'):
        b('wait','--fn','[...document.querySelectorAll(".chess-piece img")].every(e=>e.complete&&e.naturalWidth>0&&new URL(e.currentSrc).origin===location.origin)')
        shot('cdn-failure-local-pieces')
    js('window.__fallback=[];const old=HTMLMediaElement.prototype.play;HTMLMediaElement.prototype.play=function(){const src=this.src;return old.call(this).then(()=>{if(src.startsWith(location.origin+"/chess/v1/audio/"))window.__fallback.push(src)})}')
    assert room()['game']['data']==snapshot and js('!document.querySelector(".chess-board").dataset.moving'),'reload replayed move'
    for width in [390,320]:
        b('set','viewport',str(width),'844' if width==390 else '740')
        assert js('[...document.querySelectorAll(".chess-piece")].every(e=>e.getBoundingClientRect().width>=24)'),'pieces too small'
        geometry=js('document.querySelector(".chess-grid").getBoundingClientRect().toJSON()');assert abs(geometry['width']-geometry['height'])<2
        shot(mode+'-mobile-'+str(width))
    if mode=='natural':
        # A real gesture after reload unlocks the browser media channel.
        select('g1')
        peer_move('f8e7');ply(8)
        b('wait','--fn','window.__fallback.length>0')
        report['localAudioFallback']=js('window.__fallback')
        api('/logout',{});api('/login',{'name':'国际象棋对手','password':'chess-browser-only'});b('reload');b('wait','.chess-board')
        assert room()['you']==1
        assert js('document.querySelector("[data-chess-square=e8]").getBoundingClientRect().y>document.querySelector("[data-chess-square=e1]").getBoundingClientRect().y'),'black orientation wrong'
        shot('black-own-side-bottom')
        api('/logout',{});api('/login',{'name':'棋类验收','password':'chess-browser-only'});b('reload');b('wait','.chess-board')
        b('set','media','light','reduced-motion');click('静音')
        endpoint=b('get','cdp-url')['cdpUrl']
        for sq in ['d2','d4']:
            selector=f'[data-chess-square="{sq}"]';b('scrollintoview',selector);r=js('document.querySelector('+json.dumps(selector)+').getBoundingClientRect().toJSON()');x=r['x']+r['width']/2;y=r['y']+r['height']/2
            subprocess.run(['node','scripts/browser_touch.mjs',endpoint,base,'tap',str(x),str(y),str(x),str(y)],cwd=root,env=env,check=True,timeout=20)
        assert room()['game']['data']['ply']==8
        b('scrollintoview','[data-chess-confirm]');b('click','[data-chess-confirm]');ply(9)
        assert room()['game']['data']['board'][27]=='wP'
        assert js('!document.querySelector(".chess-board").dataset.moving&&document.querySelector("audio[data-platform-audio]").paused')
        shot('native-touch-reduced-muted')
    click('认输');click('确认认输');b('wait','.results');shot(mode+'-result')
    report['passed']=True
except Exception as e:
    import traceback
    traceback.print_exc();report['error']=repr(e)
    if opened:
        try:report['state']=api('/state');report['snapshot']=b('snapshot','-i');b('screenshot',str(out/'failure.png'),'--full')
        except Exception:pass
finally:
    if opened:
        try:b('close')
        except Exception:pass
    if server:
        (out/'stop').write_text('stop')
        try:server.wait(timeout=15)
        except subprocess.TimeoutExpired:server.terminate();server.wait(timeout=10)
    (out/('result-'+env.get('WIRE_BOARD_CHESS_CASE','natural')+'.json')).write_text(json.dumps(report,ensure_ascii=False,indent=2)+'\n')
    print('Artifact:',out,flush=True)
    if not report['passed']:raise SystemExit(1)
