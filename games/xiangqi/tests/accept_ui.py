#!/usr/bin/env python3
"""Xiangqi real-browser E2E, authored before the board implementation.

Failures: unpublished game visible; missing cover/options; wrong pointer or
keyboard coordinate; premature submission; clipped mobile board/tokens;
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
out=root/'.local/xiangqi/browser';out.mkdir(parents=True,exist_ok=True)
env=dict(os.environ)
for key,rel in {'GOCACHE':'.local/go-build','GOMODCACHE':'.local/go-mod','GOTMPDIR':'.local/tmp','TMPDIR':'.local/tmp','npm_config_cache':'.local/npm','AGENT_BROWSER_SOCKET_DIR':'.local/ab-xiangqi','AGENT_BROWSER_PROFILE':'.local/xiangqi/browser/profile'}.items():
    p=root/rel;p.mkdir(parents=True,exist_ok=True);env[key]=str(p)
env.update(GOTOOLCHAIN='local',WIRE_BOARD_XIANGQI_BROWSER='1',AGENT_BROWSER_SESSION='xiangqi-'+uuid.uuid4().hex[:8],AGENT_BROWSER_ARGS='--mute-audio,--disable-breakpad,--disable-crash-reporter')
report={'passed':False,'checks':[],'command':'. scripts/env.sh && python3 -B games/xiangqi/tests/accept_ui.py'}
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
      return [...document.querySelectorAll('.xiangqi-heading p,.xiangqi-heading strong,.xiangqi-actions p,.xiangqi-actions h3,.xiangqi-actions button,.xiangqi-player-summary small')].map(e=>{
        let chain=[],bg=[255,255,255];for(let n=e;n;n=n.parentElement)chain.unshift(n);for(const n of chain){const c=rgb(getComputedStyle(n).backgroundColor);bg=blend(c,bg,c[3]??1)}
        const a=lum(rgb(getComputedStyle(e).color)),b=lum(bg);return {text:e.textContent.trim(),ratio:(Math.max(a,b)+.05)/(Math.min(a,b)+.05)};
      });
    })()""")
    assert all(c['ratio']>=4.5 for c in contrast),contrast
    (out/(name+'.contrast.json')).write_text(json.dumps(contrast,ensure_ascii=False,indent=2))
    report['checks'].append(name);print('PASS',name,flush=True)
def click(name):b('find','role','button','click','--name',name,'--exact')
def ply(n):b('wait','--fn',f'document.querySelector(".xiangqi-board")?.dataset.ply==="{n}"&&!document.querySelector(".xiangqi-board").dataset.moving')
def select(square):
    sel=f'[data-xiangqi-square="{square}"]';b('scrollintoview',sel)
    r=js('document.querySelector('+json.dumps(sel)+').getBoundingClientRect().toJSON()')
    b('mouse','move',str(round(r['x']+r['width']/2)),str(round(r['y']+r['height']/2)));b('mouse','down')
    after=js('document.querySelector('+json.dumps(sel)+').getBoundingClientRect().toJSON()')
    assert abs(r['x']-after['x'])<.1 and abs(r['y']-after['y'])<.1,'active styling moved hit target'
    b('mouse','up')
def move(fr,to,promotion=None):
    before=room()['game']['data']['ply'];select(fr);select(to)
    assert room()['game']['data']['ply']==before,'move before confirmation'
    if promotion:click(promotion)
    b('click','[data-xiangqi-confirm]');ply(before+1)
try:
    run(['npm','--prefix','web','run','build'],'build.log')
    for name in ['url','stop']:(out/name).unlink(missing_ok=True)
    with (out/'server.log').open('w') as f:server=subprocess.Popen(['go','test','-count=1','-timeout=13m','-run','^TestXiangqiBrowserFixture$','./games/xiangqi/tests'],cwd=root,env=env,stdout=f,stderr=subprocess.STDOUT)
    until=time.monotonic()+90
    while not (out/'url').exists():
        if server.poll() is not None or time.monotonic()>until:raise RuntimeError('See server.log')
        time.sleep(.1)
    base=(out/'url').read_text();b('open',base);opened=True;b('tab',next(t['tabId'] for t in b('tab')['tabs'] if t['active']))
    api('/login',{'name':'棋类验收','password':'xiangqi-browser-only'})
    assert 'xiangqi' not in api('/state')['availableGames']
    api('/admin/games/xiangqi',{'enabled':True});b('reload');b('wait','.lobby-create');b('click','.lobby-create')
    b('wait','[role=dialog]');b('fill','input[aria-label=搜索可选游戏]','中国象棋');b('click','.choose-games button')
    b('wait','select[aria-label=中国象棋电脑难度]')
    assert js('!!document.querySelector(".choose-games .selected svg")'),'missing illustrated cover'
    shot('create-cover')
    b('fill','input[name=name]','中国象棋原生输入验收');b('scrollintoview','[role=dialog] button[type=submit]');b('click','[role=dialog] button[type=submit]');b('wait','.waiting-actions')
    peer=urllib.request.build_opener(urllib.request.HTTPCookieProcessor(http.cookiejar.CookieJar()))
    def peer_api(path,body=None):
        raw=None if body is None else json.dumps(body).encode()
        with peer.open(urllib.request.Request(base+'/api'+path,data=raw,headers={'Content-Type':'application/json'}),timeout=10) as r:return json.load(r)
    def peer_command(kind,**extra):
        r=room();return peer_api('/rooms/'+r['id'],dict(type=kind,version=r['version'],nonce=uuid.uuid4().hex,**extra))
    def peer_move(uci):
        d=room()['game']['data'];peer_command('action',action={'type':'move','round':d['round'],'ply':d['ply'],'from':ord(uci[0])-97+(9-int(uci[1]))*9,'to':ord(uci[2])-97+(9-int(uci[3]))*9})
    peer_api('/register',{'name':'中国象棋对手','password':'xiangqi-browser-only','invite':'xiangqi-browser'});peer_command('join');peer_command('ready');command('ready');command('start');b('wait','.xiangqi-board')
    b('set','viewport','1440','1000');ply(0)
    b('wait','--fn','[...document.querySelectorAll(".xiangqi-piece img")].every(e=>e.complete&&e.naturalWidth>0)')
    js('window.__motion=[];window.__sounds=[];new MutationObserver(()=>{const e=document.querySelector(".xiangqi-board");if(e?.dataset.moving)window.__motion.push({id:e.dataset.moving,pieces:document.querySelectorAll(".xiangqi-moving").length,captured:document.querySelector(".xiangqi-captured")?.textContent,results:!!document.querySelector(".results")})}).observe(document.querySelector(".xiangqi-board"),{attributes:true,childList:true,subtree:true});const old=HTMLMediaElement.prototype.play;HTMLMediaElement.prototype.play=function(){window.__sounds.push(this.src);return old.call(this)}')
    mode=env.get('WIRE_BOARD_XIANGQI_CASE','natural')
    if mode=='loop':
        b('set','viewport','320','740')
        for cycle in range(3):
            move('d8','e8');peer_move('e9d9');ply(cycle*4+2)
            move('e8','d8');peer_move('d9e9');ply(cycle*4+4)
            if cycle==1:
                assert room()['game']['data']['phase']=='playing'
                b('wait','.xiangqi-warning');assert '长将' in b('get','text','.xiangqi-warning')['text']
                shot('long-check-warning-mobile')
        b('wait','.results');assert room()['game']['data']['reason']=='perpetual_check'
        assert room()['game']['winners']==[1]
    elif mode=='natural':
        select('h2');b('press','ArrowLeft');b('press','Enter');b('click','[data-xiangqi-confirm]');ply(1)
        assert room()['game']['data']['board'][69]==6
        peer_move('h9g7');ply(2)
        for fr,to,reply in [('b0','c2','a6a5'),('g2','e2','a5a4')]:move(fr,to);peer_move(reply);ply(room()['game']['data']['ply'])
        move('a3','a4');assert room()['game']['data']['captured'][0]==[-7]
        assert js('window.__motion.some(e=>e.pieces===1&&!e.results)'),'missing server-driven motion'
        shot('desktop-native-keyboard-capture')
    else:raise ValueError(mode)
    snapshot=room()['game']['data']
    if env.get('WIRE_BOARD_XIANGQI_ASSETS'):
        b('network','route',env['WIRE_BOARD_XIANGQI_ASSETS'].rstrip('/')+'/xiangqi/v1/**','--abort')
        subprocess.run(['node','scripts/browser_reload_uncached.mjs',b('get','cdp-url')['cdpUrl'],base],cwd=root,env=env,check=True,timeout=25)
    else:b('reload')
    b('wait','.xiangqi-board')
    if env.get('WIRE_BOARD_XIANGQI_ASSETS'):
        b('wait','--fn','[...document.querySelectorAll(".xiangqi-piece img")].every(e=>e.complete&&e.naturalWidth>0&&new URL(e.currentSrc).origin===location.origin)')
        shot('cdn-failure-local-pieces')
    js('window.__fallback=[];const old=HTMLMediaElement.prototype.play;HTMLMediaElement.prototype.play=function(){const src=this.src;return old.call(this).then(()=>{if(src.startsWith(location.origin+"/xiangqi/v1/audio/"))window.__fallback.push(src)})}')
    assert room()['game']['data']==snapshot and js('!document.querySelector(".xiangqi-board").dataset.moving'),'reload replayed move'
    for width in [390,320]:
        b('set','viewport',str(width),'844' if width==390 else '740')
        assert js('[...document.querySelectorAll(".xiangqi-piece")].every(e=>e.getBoundingClientRect().width>=24)'),'pieces too small'
        geometry=js('document.querySelector(".xiangqi-map").getBoundingClientRect().toJSON()');assert abs(geometry['width']/geometry['height']-.9)<.02
        shot(mode+'-mobile-'+str(width))
    if mode=='natural':
        # A real gesture after reload unlocks the browser media channel.
        select('a4')
        peer_move('b9c7');ply(8)
        b('wait','--fn','window.__fallback.length>0')
        report['localAudioFallback']=js('window.__fallback')
        api('/logout',{});api('/login',{'name':'中国象棋对手','password':'xiangqi-browser-only'});b('reload');b('wait','.xiangqi-board')
        assert room()['you']==1
        assert js('document.querySelector("[data-xiangqi-square=e9]").getBoundingClientRect().y>document.querySelector("[data-xiangqi-square=e0]").getBoundingClientRect().y'),'black orientation wrong'
        shot('black-own-side-bottom')
        api('/logout',{});api('/login',{'name':'棋类验收','password':'xiangqi-browser-only'});b('reload');b('wait','.xiangqi-board')
        b('set','media','light','reduced-motion');click('静音')
        endpoint=b('get','cdp-url')['cdpUrl']
        for sq in ['e3','e4']:
            selector=f'[data-xiangqi-square="{sq}"]';b('scrollintoview',selector);r=js('document.querySelector('+json.dumps(selector)+').getBoundingClientRect().toJSON()');x=r['x']+r['width']/2;y=r['y']+r['height']/2
            subprocess.run(['node','scripts/browser_touch.mjs',endpoint,base,'tap',str(x),str(y),str(x),str(y)],cwd=root,env=env,check=True,timeout=20)
        assert room()['game']['data']['ply']==8
        b('scrollintoview','[data-xiangqi-confirm]');b('click','[data-xiangqi-confirm]');ply(9)
        assert room()['game']['data']['board'][49]==7
        assert js('!document.querySelector(".xiangqi-board").dataset.moving&&document.querySelector("audio[data-platform-audio]").paused')
        shot('native-touch-reduced-muted')
    if mode=='natural':click('认输');click('确认认输')
    b('wait','.results');shot(mode+'-result')
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
    (out/('result-'+env.get('WIRE_BOARD_XIANGQI_CASE','natural')+'.json')).write_text(json.dumps(report,ensure_ascii=False,indent=2)+'\n')
    print('Artifact:',out,flush=True)
    if not report['passed']:raise SystemExit(1)
