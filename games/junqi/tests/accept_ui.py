#!/usr/bin/env python3
"""Junqi real-browser E2E, authored before the board implementation.

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
out=root/'.local/junqi/browser';out.mkdir(parents=True,exist_ok=True)
env=dict(os.environ)
for key,rel in {'GOCACHE':'.local/go-build','GOMODCACHE':'.local/go-mod','GOTMPDIR':'.local/tmp','TMPDIR':'.local/tmp','npm_config_cache':'.local/npm','AGENT_BROWSER_SOCKET_DIR':'.local/ab-junqi','AGENT_BROWSER_PROFILE':'.local/junqi/browser/profile'}.items():
    p=root/rel;p.mkdir(parents=True,exist_ok=True);env[key]=str(p)
env.update(GOTOOLCHAIN='local',WIRE_BOARD_JUNQI_BROWSER='1',AGENT_BROWSER_SESSION='junqi-'+uuid.uuid4().hex[:8],AGENT_BROWSER_ARGS='--mute-audio,--disable-breakpad,--disable-crash-reporter')
report={'passed':False,'checks':[],'command':'. scripts/env.sh && python3 -B games/junqi/tests/accept_ui.py'}
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
      return [...document.querySelectorAll('.junqi-heading p,.junqi-heading strong,.junqi-actions p,.junqi-actions h3,.junqi-actions button,.junqi-map-tools button,.junqi-player-summary small')].map(e=>{
        let chain=[],bg=[255,255,255];for(let n=e;n;n=n.parentElement)chain.unshift(n);for(const n of chain){const c=rgb(getComputedStyle(n).backgroundColor);bg=blend(c,bg,c[3]??1)}
        const a=lum(rgb(getComputedStyle(e).color)),b=lum(bg);return {text:e.textContent.trim(),ratio:(Math.max(a,b)+.05)/(Math.min(a,b)+.05)};
      });
    })()""")
    assert all(c['ratio']>=4.5 for c in contrast),contrast
    (out/(name+'.contrast.json')).write_text(json.dumps(contrast,ensure_ascii=False,indent=2))
    report['checks'].append(name);print('PASS',name,flush=True)
def click(name):b('find','role','button','click','--name',name,'--exact')
def view():return room()['game']['data']
def settled(ply):b('wait','--fn',f'document.querySelector(".junqi-board")?.dataset.ply==="{ply}"&&!document.querySelector(".junqi-board").dataset.moving')
def node(pos):
    sel=f'[data-junqi-node="{pos}"]';b('scrollintoview',sel);r=js('document.querySelector('+json.dumps(sel)+').getBoundingClientRect().toJSON()')
    b('mouse','move',str(round(r['x']+r['width']/2)),str(round(r['y']+r['height']/2)));b('mouse','down')
    pressed=js('document.querySelector('+json.dumps(sel)+').getBoundingClientRect().toJSON()');assert abs(r['x']-pressed['x'])<.1 and abs(r['y']-pressed['y'])<.1,'pressed style moved hit target';b('mouse','up')
try:
    n=int(env.get('WIRE_BOARD_JUNQI_PLAYERS','2'));assert n in [2,4]
    run(['npm','--prefix','web','run','build'],'build.log')
    for name in ['url','stop']:(out/name).unlink(missing_ok=True)
    with (out/'server.log').open('w') as f:server=subprocess.Popen(['go','test','-count=1','-timeout=13m','-run','^TestJunqiBrowserFixture$','./games/junqi/tests'],cwd=root,env=env,stdout=f,stderr=subprocess.STDOUT)
    until=time.monotonic()+90
    while not (out/'url').exists():
        if server.poll() is not None or time.monotonic()>until:raise RuntimeError('See server.log')
        time.sleep(.1)
    base=(out/'url').read_text();b('open',base);opened=True;b('tab',next(t['tabId'] for t in b('tab')['tabs'] if t['active']))
    api('/login',{'name':'棋类验收','password':'junqi-browser-only'});assert 'junqi' not in api('/state')['availableGames'];api('/admin/games/junqi',{'enabled':True});b('reload');b('wait','.lobby-create');b('click','.lobby-create')
    b('wait','[role=dialog]');b('fill','input[aria-label=搜索可选游戏]','军棋');b('click','.choose-games button');b('select','select[aria-label=座位上限]',str(n))
    assert js('!!document.querySelector(".choose-games .selected svg")');b('fill','input[name=name]','军棋原生输入验收');shot(f'create-cover-{n}')
    b('scrollintoview','[role=dialog] button[type=submit]');b('click','[role=dialog] button[type=submit]');b('wait','.waiting-actions')
    peers=[]
    def peer_api(seat,path,body=None):
        raw=None if body is None else json.dumps(body).encode()
        with peers[seat-1].open(urllib.request.Request(base+'/api'+path,data=raw,headers={'Content-Type':'application/json'}),timeout=10) as r:return json.load(r)
    def peer_command(seat,kind,**extra):
        r=room();return peer_api(seat,'/rooms/'+r['id'],dict(type=kind,version=r['version'],nonce=uuid.uuid4().hex,**extra))
    def peer_act(seat,kind,**extra):
        d=peer_api(seat,'/state')['room']['game']['data'];peer_command(seat,'action',action=dict(type=kind,round=d['round'],ply=d['ply'],revision=d['revision'],**extra))
    def peer_move(seat):
        r=peer_api(seat,'/state')['room'];d=r['game']['data'];m=d['legal'][0]
        peer_api(seat,'/rooms/'+r['id'],dict(type='action',version=r['version'],nonce=uuid.uuid4().hex,action=dict(type='move',round=d['round'],ply=d['ply'],revision=d['revision'],unit=m['unit'],to=m['to'])))
    for seat in range(1,n):
        peers.append(urllib.request.build_opener(urllib.request.HTTPCookieProcessor(http.cookiejar.CookieJar())));peer_api(seat,'/register',{'name':f'军棋对手{seat}','password':'junqi-browser-only','invite':'junqi-browser'});peer_command(seat,'join')
    for seat in range(1,n):peer_command(seat,'ready')
    command('ready');command('start');b('wait','.junqi-board');b('set','viewport','1440','1000')
    b('wait','--fn','document.querySelectorAll(".junqi-piece img").length>0&&[...document.querySelectorAll(".junqi-piece img")].every(e=>e.complete&&e.naturalWidth>0)')
    assert len(view()['pieces'])==n*25 and view()['phase']=='deploy'
    assert js('document.querySelectorAll(".junqi-seat-tag").length===0'),'numeric piece badges returned'
    assert all(p['kind']=='unknown' for p in view()['pieces'] if p['seat']!=0)
    assert js('[...document.querySelectorAll("[data-piece-seat]")].filter(e=>e.dataset.pieceSeat!=="0").every(e=>e.getAttribute("aria-label").includes("暗子")&&e.querySelector("img").src.includes("-back.webp"))'),'hidden faces leaked'
    own=[p for p in view()['pieces'] if p['seat']==0 and p['kind'] in ['company','platoon']];first,second=own[:2];node(first['pos']);node(second['pos']);assert view()['revision']==0
    click('交换位置');b('wait','--fn','document.querySelector(".junqi-board").dataset.revision==="1"&&!document.querySelector(".junqi-board").dataset.moving')
    assert next(p for p in view()['pieces'] if p['id']==first['id'])['pos']==second['pos'];shot(f'deployment-swap-{n}')
    # Adjacent front-row pieces always allow a legal keyboard-driven swap.
    first=next(p for p in view()['pieces'] if p['pos']==1)
    second=next(p for p in view()['pieces'] if p['pos']==2)
    node(1);b('press','ArrowRight')
    assert js('document.activeElement?.dataset.junqiNode==="2"'),'keyboard focus missed adjacent piece'
    b('press','Enter');click('交换位置')
    b('wait','--fn','document.querySelector(".junqi-board").dataset.revision==="2"&&!document.querySelector(".junqi-board").dataset.moving')
    assert next(p for p in view()['pieces'] if p['id']==first['id'])['pos']==2
    assert next(p for p in view()['pieces'] if p['id']==second['id'])['pos']==1
    shot(f'keyboard-swap-{n}')
    click('完成布阵')
    for seat in range(1,n):peer_act(seat,'deploy_ready',ready=True)
    b('wait','--fn','document.querySelector(".junqi-board").dataset.phase==="playing"');settled(0)
    b('scrollintoview','.junqi-viewport');r=js('document.querySelector(".junqi-viewport").getBoundingClientRect().toJSON()');x=round(r['x']+r['width']/2);y=round(r['y']+r['height']/2);endpoint=b('get','cdp-url')['cdpUrl'];version=room()['version']
    subprocess.run(['node','scripts/browser_wheel.mjs',endpoint,base,str(x),str(y),'-250'],cwd=root,env=env,check=True,timeout=20)
    b('wait','--fn','Number(document.querySelector(".junqi-board").dataset.zoom)>1');b('mouse','move',str(x),str(y));b('mouse','down');b('mouse','move',str(x-70),str(y-40));b('mouse','up');assert room()['version']==version,'map drag submitted action'
    click('全图');b('wait','--fn','document.querySelector(".junqi-board").dataset.zoom==="1"')
    js('window.__motionFrames={};let tracked="";new MutationObserver(()=>{const board=document.querySelector(".junqi-board"),id=board?.dataset.moving;if(!id||id===tracked)return;tracked=id;const frames=window.__motionFrames[id]=[];const sample=()=>{if(document.querySelector(".junqi-board")?.dataset.moving!==id)return;const el=document.querySelector(".junqi-moving");if(el){const r=el.getBoundingClientRect();frames.push({x:r.x,y:r.y,t:performance.now()})}requestAnimationFrame(sample)};sample()}).observe(document.querySelector(".junqi-board"),{attributes:true,childList:true,subtree:true})')
    d=view();m=d['legal'][0];node(m['from']);node(m['to']);assert view()['ply']==0;b('click','[data-junqi-confirm]')
    b('wait','--fn','!!document.querySelector(".junqi-board").dataset.moving')
    peer_move(1);settled(2)
    frames=js('window.__motionFrames');report['rapidMoveFrames']=frames
    assert len(frames)>=2 and all(len(a)>1 and max(((p['x']-a[0]['x'])**2+(p['y']-a[0]['y'])**2)**.5 for p in a)>3 for a in frames.values()),'consecutive route animation did not move: '+str(frames)
    for seat in range(2,n):peer_move(seat);settled(seat+1)
    shot(f'desktop-native-play-{n}')
    api('/login',{'name':'军棋对手1','password':'junqi-browser-only'});b('reload');b('wait','.junqi-board');assert room()['you']==1
    opposite=0 if n==2 else 3
    own_back=30+27;opposite_back=opposite*30+27
    own_selector=json.dumps(f'[data-junqi-node="{own_back}"]'); opposite_selector=json.dumps(f'[data-junqi-node="{opposite_back}"]')
    orientation=f'document.querySelector({own_selector}).getBoundingClientRect().y>document.querySelector({opposite_selector}).getBoundingClientRect().y'
    b('wait','--fn',orientation);assert js(orientation),'own army did not rotate to bottom'
    assert all(p['kind']=='unknown' for p in view()['pieces'] if p['seat']!=1 and not p['revealed'])
    shot(f'blue-side-bottom-{n}')
    api('/login',{'name':'棋类验收','password':'junqi-browser-only'});b('reload');b('wait','.junqi-board');assert room()['you']==0
    saved=view()
    if env.get('WIRE_BOARD_JUNQI_ASSETS'):
        b('network','route',env['WIRE_BOARD_JUNQI_ASSETS'].rstrip('/')+'/junqi/v1/**','--abort');subprocess.run(['node','scripts/browser_reload_uncached.mjs',endpoint,base],cwd=root,env=env,check=True,timeout=25)
    else:b('reload')
    b('wait','.junqi-board');assert view()==saved and js('!document.querySelector(".junqi-board").dataset.moving')
    if env.get('WIRE_BOARD_JUNQI_ASSETS'):
        b('wait','--fn','[...document.querySelectorAll(".junqi-piece img")].every(e=>e.complete&&e.naturalWidth>0&&new URL(e.currentSrc).origin===location.origin)');shot(f'cdn-fallback-{n}')
    js('window.__junqiLocal=[];const old=HTMLMediaElement.prototype.play;HTMLMediaElement.prototype.play=function(){const src=this.src;return old.call(this).then(()=>{if(src.startsWith(location.origin+"/junqi/v1/audio/"))window.__junqiLocal.push(src)})}')
    m=view()['legal'][0];node(m['from']);node(m['to']);b('click','[data-junqi-confirm]');settled(n+1)
    for seat in range(1,n):peer_move(seat);settled(n+1+seat)
    b('wait','--fn','window.__junqiLocal.length>0');report['localAudioFallback']=js('window.__junqiLocal');base_ply=2*n
    for width in [390,320]:
        b('set','viewport',str(width),'844' if width==390 else '740');b('wait','--fn','document.querySelector(".junqi-map").getBoundingClientRect().width>0')
        fit='(() => {const board=document.querySelector(".junqi-board"),a=document.querySelector(".junqi-map").getBoundingClientRect(),b=document.querySelector(".junqi-viewport").getBoundingClientRect();return board.dataset.zoom==="1"&&board.dataset.overview==="true"&&a.width>0&&a.left>=b.left-1&&a.top>=b.top-1&&a.right<=b.right+1&&a.bottom<=b.bottom+1})()'
        b('wait','--fn',fit);assert js(fit),'default phone view clips the board'
        shot(f'mobile-full-board-{n}-{width}')
        first=next(p for p in view()['pieces'] if p['seat']==0)
        node(first['pos']);b('wait','--fn','document.querySelector(".junqi-board").dataset.overview==="false"')
        assert view()['ply']==base_ply,'overview tap submitted a move'
        assert js('[...document.querySelectorAll(".junqi-piece")].every(e=>e.getBoundingClientRect().width>=28)'),'detail pieces too small'
        shot(f'mobile-detail-{n}-{width}');click('全图')

    b('scrollintoview','.junqi-viewport');r=js('document.querySelector(".junqi-viewport").getBoundingClientRect().toJSON()');x=round(r['x']+r['width']/2);y=round(r['y']+r['height']/2)
    subprocess.run(['node','scripts/browser_touch.mjs',endpoint,base,'pinch',str(x),str(y),str(x),str(y)],cwd=root,env=env,check=True,timeout=20)
    b('wait','--fn','Number(document.querySelector(".junqi-board").dataset.zoom)>1');assert view()['ply']==base_ply,'pinch submitted a move';click('全图')
    first=next(p for p in view()['pieces'] if p['seat']==0)
    node(first['pos']);b('wait','--fn','document.querySelector(".junqi-board").dataset.overview==="false"')
    # Native touch drag beginning on a piece must pan without selecting/moving it.
    sel=json.dumps(f'[data-junqi-node="{first["pos"]}"]')
    b('scrollintoview',f'[data-junqi-node="{first["pos"]}"]')
    r=js('document.querySelector('+sel+').getBoundingClientRect().toJSON()')
    x=r['x']+r['width']/2;y=r['y']+r['height']/2
    subprocess.run(['node','scripts/browser_touch.mjs',endpoint,base,'drag',str(x),str(y),str(x-45),str(y-50)],cwd=root,env=env,check=True,timeout=20)
    assert view()['ply']==base_ply and js('!document.querySelector(".junqi-node.selected")'),'touch drag selected or moved a piece'
    shot(f'touch-drag-navigation-{n}')
    # Continuous minimap drag is navigation only.
    b('scrollintoview','.junqi-minimap');r=js('document.querySelector(".junqi-minimap").getBoundingClientRect().toJSON()');x=round(r['x']+r['width']*.2);y=round(r['y']+r['height']*.2)
    b('mouse','move',str(x),str(y));b('mouse','down');b('mouse','move',str(round(r['x']+r['width']*.8)),str(round(r['y']+r['height']*.8)));b('mouse','up');assert view()['ply']==base_ply
    b('set','media','light','reduced-motion');click('静音');m=view()['legal'][0]
    node(m['from']);b('wait','--fn','document.querySelector(".junqi-board").dataset.overview==="false"')
    for pos in [m['from'],m['to']]:
        sel=f'[data-junqi-node="{pos}"]';b('scrollintoview',sel);r=js('document.querySelector('+json.dumps(sel)+').getBoundingClientRect().toJSON()');x=r['x']+r['width']/2;y=r['y']+r['height']/2
        subprocess.run(['node','scripts/browser_touch.mjs',endpoint,base,'tap',str(x),str(y),str(x),str(y)],cwd=root,env=env,check=True,timeout=20)
    assert view()['ply']==base_ply;b('scrollintoview','[data-junqi-confirm]');b('click','[data-junqi-confirm]');settled(base_ply+1);assert js('document.querySelector("audio[data-platform-audio]").paused');shot(f'touch-muted-{n}')
    for seat in range(1,n):peer_move(seat);settled(base_ply+1+seat)
    click('提议和棋');b('wait','--fn','document.querySelector(".junqi-draw-offer")!==null');offer=view()['offer']['id']
    for seat in range(1,n):peer_act(seat,'vote_draw',offerID=offer,accept=True)
    b('wait','.results');assert view()['reason']=='agreement' and room()['result']['draw'] is True;shot(f'agreed-draw-{n}');report['passed']=True
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
    (out/('result-'+env.get('WIRE_BOARD_JUNQI_PLAYERS','2')+'.json')).write_text(json.dumps(report,ensure_ascii=False,indent=2)+'\n');print('Artifact:',out,flush=True)
    if not report['passed']:raise SystemExit(1)
