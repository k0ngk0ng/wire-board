#!/usr/bin/env python3
"""Real browser acceptance. Failure cases: BROWSER_REFACTOR_SCENARIOS.md.
Production HTTP, SQLite, timers and rules; native browser input; no fake state.
"""
import argparse
import hashlib
import http.cookiejar
import json
import math
import base64
import os
from pathlib import Path
import shutil
import socket
import subprocess
import time
import traceback
import urllib.error
import urllib.request
import uuid

parser = argparse.ArgumentParser()
parser.add_argument('--build', action='store_true')
parser.add_argument('--out', default='')
parser.add_argument('--only', choices=['all','interaction','timeline','effects','bots','reveal','terrain','cancel','facing','portraits','supplies','projectiles','slope'], default='all')
parser.add_argument('--tanks', nargs='+', type=int, choices=range(1,15), default=list(range(1,15)))
args = parser.parse_args()
root = Path(__file__).resolve().parents[2]
run = 'f' + uuid.uuid4().hex[:6]
out = (root / (args.out or '.local/fortressblue/refactor-browser-' + run)).resolve()
assert out.is_relative_to(root) and out != root and not out.exists(), 'Use a fresh repository output directory'
out.mkdir(parents=True)
temp = out / 'temporary'
sockets = root / '.local/fbe' / run
env = dict(os.environ)
for name, path in {'AGENT_BROWSER_SOCKET_DIR': sockets, 'AGENT_BROWSER_PROFILE': temp / 'profile', 'TMPDIR': temp / 'tmp', 'GOTMPDIR': temp / 'tmp', 'GOCACHE': root / '.local/go-build', 'GOMODCACHE': root / '.local/go-mod', 'npm_config_cache': root / '.local/npm'}.items():
    path.mkdir(parents=True, exist_ok=True)
    env[name] = str(path)
env.update(AGENT_BROWSER_SESSION=run, AGENT_BROWSER_ARGS='--mute-audio,--disable-breakpad,--disable-crash-reporter', WIRE_BOARD_FORTRESSBLUE_BROWSER='1', WIRE_BOARD_FORTRESSBLUE_BROWSER_OUT=str(out), GOTOOLCHAIN='local')
report = {'selection': args.only, 'passed': False, 'checks': [], 'screenshots': [], 'command': 'python3 scripts/fortressblue/check-refactor.py --only '+args.only+(' --build' if args.build else ''), 'scope': 'Fortress only; production HTTP/SQLite/clock; native CDP inputs; labelled opening fixtures only'}
if args.only=='projectiles':report['command']+=' --tanks '+' '.join(map(str,args.tanks))
report['sources']={str(p.relative_to(root)):hashlib.sha256(p.read_bytes()).hexdigest() for folder,pattern in [('games/fortressblue/server','*.go'),('games/fortressblue/web','*.ts*'),('games/fortressblue/data','*.json')] for p in (root/folder).glob(pattern)}
for rel in ['internal/server/server.go','internal/server/modules.go','internal/server/module_realtime.go','internal/gameapi/module.go','games/fortressblue/tests/browser_fixture_test.go','scripts/fortressblue/check-refactor.py','games/fortressblue/web/style.css','games/fortressblue/assets/picker-frames.json','scripts/fortressblue/prepare-picker-frames.py']:
    source=root/rel
    if source.exists():report['sources'][rel]=hashlib.sha256(source.read_bytes()).hexdigest()
report['distIndexSHA256']=hashlib.sha256((root/'web/dist/index.html').read_bytes()).hexdigest() if (root/'web/dist/index.html').exists() else None
server = log = None
opened = recording = False
base = ''


def browser(*args):
    p = subprocess.run(['agent-browser', *args, '--json'], cwd=root, env=env, text=True, capture_output=True, timeout=45)
    assert p.returncode == 0, p.stdout + p.stderr
    result = json.loads(p.stdout)
    assert result.get('success'), result
    return result['data']


def js(code):
    return browser('eval', code)['result']


def api(path, body=None, expected=(200, 201)):
    opts = '' if body is None else ',{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify(' + json.dumps(body) + ')}'
    r = js('(async()=>{const r=await fetch(' + json.dumps('/api' + path) + opts + ');return {status:r.status,body:await r.json()}})()')
    assert r['status'] in expected, r
    return r['body']


def state():
    return api('/state')['room']


def view():
    return state()['game']['data']


def command(kind, **extra):
    r = state()
    return api('/rooms/' + r['id'], {'type': kind, 'version': r['version'], 'nonce': uuid.uuid4().hex, **extra})


def action(a, expected=(200, 201)):
    r = state()
    return api('/rooms/' + r['id'], {'type': 'action', 'version': r['version'], 'nonce': uuid.uuid4().hex, 'action': a}, expected)


def wait(predicate, timeout=20, label='condition'):
    deadline = time.monotonic() + timeout
    while time.monotonic() < deadline:
        v = predicate()
        if v:
            return v
        time.sleep(.1)
    raise AssertionError('Timed out: ' + label)


def check(name, **evidence):
    report['checks'].append({'name': name, **evidence})
    print('PASS', name, flush=True)


def record(name, data):
    (out / (name + '.json')).write_text(json.dumps(data, ensure_ascii=False, indent=2) + '\n')


def shot(name, full=False):
    p = out / (name + '.png')
    browser('screenshot', str(p), *(['--full'] if full else []))
    assert p.exists() and p.stat().st_size > 1000
    report['screenshots'].append(str(p.relative_to(root)))


def rect(selector):
    return js('(()=>{const e=document.querySelector(' + json.dumps(selector) + ');if(!e)return null;const r=e.getBoundingClientRect();return {x:r.x,y:r.y,w:r.width,h:r.height}})()')


def center(selector):
    r = rect(selector)
    assert r and r['w'] > 0 and r['h'] > 0, (selector, r)
    return {'x': r['x'] + r['w']/2, 'y': r['y'] + r['h']/2}


# Read only measurement: exact public snapshot and actual draw-command output,
# pixel samples and DOM rectangles. No React state, timers or clock are mocked.
MEASURE = r'''(async()=>{
 const response=await fetch('/api/state'),fullState=await response.json(),publicState={room:fullState.room,serverNow:fullState.serverNow};
 const root=document.querySelector('.fortressblue-board'),c=document.querySelector('[data-fortressblue-canvas]'),t=document.querySelector('.fb-terrain');
 const box=e=>{if(!e)return null;const r=e.getBoundingClientRect();return {x:r.x,y:r.y,w:r.width,h:r.height}};
 const rows=[...document.querySelectorAll('[data-fortressblue-seat]')].map(e=>{const hp=e.querySelector(':scope > span'),range=document.createRange();if(hp){range.selectNodeContents(hp);const match=hp.textContent.match(/(?:生命|HP)\s+\d+\/\d+/);if(match){const end=match.index+match[0].length,walk=document.createTreeWalker(hp,NodeFilter.SHOW_TEXT);let node,total=0;while(node=walk.nextNode()){if(total+node.textContent.length>=end){range.setEnd(node,end-total);break}total+=node.textContent.length}}}const hr=hp?range.getBoundingClientRect():null;return {seat:Number(e.dataset.fortressblueSeat),hp:Number(e.dataset.hp),x:Number(e.dataset.x),y:Number(e.dataset.y),alive:e.dataset.alive,rect:box(e),hpRect:hr?{x:hr.x,y:hr.y,w:hr.width,h:hr.height}:null,hpBox:box(hp),text:e.innerText}});
 const pixels=[];if(c&&t&&c.width&&t.width){const camX=Number(c.dataset.cameraX),camY=Number(c.dataset.cameraY),vw=Number(c.dataset.worldViewWidth),scale=c.width/vw,tc=t.getContext('2d'),cc=c.getContext('2d');
  for(let y=Math.max(0,Math.ceil(camY+40));y<Math.min(t.height,camY+c.height/scale-25);y+=23)for(let x=Math.max(0,Math.ceil(camX+25));x<Math.min(t.width,camX+vw-25);x+=29){
   const sx=Math.floor((x-camX)*scale),sy=Math.floor((y-camY)*scale);if(sx<0||sy<0||sx>=c.width||sy>=c.height)continue;const tx=Math.floor(camX+(sx+.5)/scale),ty=Math.floor(camY+(sy+.5)/scale),a=Array.from(tc.getImageData(tx,ty,1,1).data);if(a[3]<250)continue;const b=Array.from(cc.getImageData(sx,sy,1,1).data);const delta=Math.max(...a.slice(0,3).map((v,i)=>Math.abs(v-b[i])));pixels.push({x,y,sx,sy,terrain:a,screen:b,delta});if(pixels.length>=100)break;
  }
 }
 return {at:Date.now(),publicState,root:root?{...root.dataset}:null,canvas:c?{...c.dataset}:null,terrainReady:t?.dataset.artReady,board:box(c),rows,pixels,timer:document.querySelector('[data-fortressblue-timer]')?.innerText,result:!!document.querySelector('.match-result-modal'),scroll:[scrollX,scrollY],viewport:[innerWidth,innerHeight],overflow:document.documentElement.scrollWidth>innerWidth+2};
})()'''

CDP = r'''
import {execFileSync} from 'node:child_process';
import {readFileSync} from 'node:fs';
const [endpoint,prefix,raw,measure,outdir,captureMode]=process.argv.slice(2),steps=JSON.parse(raw);
const ws=new WebSocket(endpoint);await new Promise((r,j)=>{ws.onopen=r;ws.onerror=j});let n=0,sessionId;const pending=new Map(),frames=[],network=[],captures=[];let sampling=false,sampler=null,busy=false,delay=0,block='';
const call=(method,params={},sid=sessionId)=>new Promise((r,j)=>{const id=++n;pending.set(id,{r,j});ws.send(JSON.stringify({id,method,params,sessionId:method.startsWith('Target.')?undefined:sid}))});
ws.onmessage=({data})=>{const m=JSON.parse(data);if(m.method==='Fetch.requestPaused'){network.push({url:m.params.request.url,at:Date.now(),delay,block});if(block&&m.params.request.url.includes(block))void call('Fetch.failRequest',{requestId:m.params.requestId,errorReason:'Failed'});else setTimeout(()=>void call('Fetch.continueRequest',{requestId:m.params.requestId}),delay);return;}if(!m.id)return;const p=pending.get(m.id);pending.delete(m.id);m.error?p.j(Error(JSON.stringify(m.error))):p.r(m.result)};
const read=async()=>{const r=await call('Runtime.evaluate',{expression:measure,awaitPromise:true,returnByValue:true});if(r.exceptionDetails)throw Error(JSON.stringify(r.exceptionDetails));return r.result.value;};
try{
 const {targetInfos}=await call('Target.getTargets',{},undefined);const target=targetInfos.find(t=>t.type==='page'&&t.url.startsWith(prefix));if(!target)throw Error('owned browser page missing');({sessionId}=await call('Target.attachToTarget',{targetId:target.targetId,flatten:true},undefined));
 await call('Network.enable');
 if(steps.some(s=>s.touch))await call('Emulation.setTouchEmulationEnabled',{enabled:true,maxTouchPoints:5});
 for(const s of steps){
  if(s.sample){sampling=true;sampler=setInterval(async()=>{if(!sampling||busy)return;busy=true;try{frames.push(await read())}catch(e){frames.push({error:String(e),at:Date.now()})}finally{busy=false}},40);}
  if(s.network){delay=s.network.delay||0;block=s.network.block||'';await call('Network.setCacheDisabled',{cacheDisabled:true});await call('Fetch.enable',{patterns:[{urlPattern:s.network.pattern||'*fortressblue*',resourceType:'Image'}]});}
  if(s.reload)await call('Page.reload',{ignoreCache:true});
  if(s.key){const code=s.key===' '?'Space':s.key;const vk={' ':32,ArrowLeft:37,ArrowUp:38,ArrowRight:39,ArrowDown:40,Escape:27}[s.key];await call('Input.dispatchKeyEvent',{type:s.up?'keyUp':'keyDown',key:s.key,code,windowsVirtualKeyCode:vk,nativeVirtualKeyCode:vk,autoRepeat:!!s.repeat});}
  if(s.mouse)await call('Input.dispatchMouseEvent',{type:s.mouse,x:s.x,y:s.y,button:s.mouse==='mouseWheel'?'none':'left',buttons:s.mouse==='mouseReleased'||s.mouse==='mouseWheel'?0:1,clickCount:1,...(s.mouse==='mouseWheel'?{deltaX:s.dx||0,deltaY:s.dy||0}:{})});
  if(s.touch)await call('Input.dispatchTouchEvent',{type:s.touch,touchPoints:s.touch==='touchEnd'||s.touch==='touchCancel'?[]:(s.points||[{x:s.x,y:s.y,id:1}])});
  if(s.blur){const {targetId}=await call('Target.createTarget',{url:'about:blank'},undefined);await call('Target.activateTarget',{targetId},undefined);await new Promise(r=>setTimeout(r,150));await call('Target.activateTarget',{targetId:target.targetId},undefined);await call('Target.closeTarget',{targetId},undefined);}
  if(s.waitUntil)await new Promise(r=>setTimeout(r,Math.max(0,s.waitUntil-Date.now())));
  if(s.wait)await new Promise(r=>setTimeout(r,s.wait));
  if(s.read)frames.push({...await read(),label:s.read});
  if(s.capture){if(captureMode==='browser'){const path=outdir+'/'+s.capture+'.png';const result=JSON.parse(execFileSync('agent-browser',['screenshot',path,'--json'],{encoding:'utf8',timeout:45000}));if(!result.success)throw Error('Browser screenshot failed');captures.push({name:s.capture,data:readFileSync(path).toString('base64')});}else captures.push({name:s.capture,...await call('Page.captureScreenshot',{format:'png'})});}
 }
 sampling=false;if(sampler)clearInterval(sampler);while(busy)await new Promise(r=>setTimeout(r,20));
 if(steps.some(s=>s.touch))await call('Emulation.setTouchEmulationEnabled',{enabled:false});
 if(steps.some(s=>s.network))await call('Fetch.disable');
 console.log(JSON.stringify({frames,network,captures}));
}finally{sampling=false;if(sampler)clearInterval(sampler);if(sessionId)await call('Target.detachFromTarget',{sessionId},undefined);ws.close()}
'''


def native(steps, name=None):
    import base64
    endpoint = browser('get', 'cdp-url')['cdpUrl']
    p = subprocess.run(['node', '--input-type=module', '-', endpoint, base, json.dumps(steps), MEASURE, str(out), 'browser' if args.only in ('supplies','projectiles') else 'cdp'], input=CDP, cwd=root, env=env, text=True, capture_output=True, timeout=60)
    if p.returncode:
        (out/((name or 'native')+'-failure.txt')).write_text(p.stderr)
        if p.stdout: (out/((name or 'native')+'.json')).write_text(p.stdout)
        raise AssertionError('Native CDP failed: '+p.stderr[-1800:])
    data = json.loads(p.stdout)
    for capture in data.pop('captures', []):
        dest = out / (capture['name'] + '.png')
        dest.write_bytes(base64.b64decode(capture['data']))
        report['screenshots'].append(str(dest.relative_to(root)))
    if name:
        record(name, data)
    return data


def measure():
    return js(MEASURE)


def foreground(m):
    matching = [p for p in m['pixels'] if p['delta'] <= 12]
    assert len(matching) >= 8, {'sampleCount': len(m['pixels']), 'matching': len(matching)}
    return {'sampleCount': len(m['pixels']), 'matching': len(matching), 'examples': matching[:12]}


def camera():
    c = measure()['canvas']
    return {k:float(c[k]) for k in ['cameraX','cameraY','worldViewWidth']}


def layout(name):
    # ResizeObserver changes CSS size before the drawing camera consumes it.
    # Require three consistent observations instead of measuring that transient.
    previous=None;stable=0;deadline=time.monotonic()+3
    while time.monotonic()<deadline:
        m=measure();key=(m['board']['w'],m['board']['h'],m['canvas'].get('worldViewWidth'))
        stable=stable+1 if key==previous else 0;previous=key
        if stable>=2:break
        time.sleep(.05)
    assert stable>=2,'camera/viewport did not stabilize'
    assert not m['overflow'], m['viewport']
    b=m['board']; assert .8 < b['w']/b['h'] < 2.3, b
    if m['viewport'][0]>=1000: assert b['w']/b['h']>1.1,b
    if m['viewport'][0] > 1900: assert b['w'] < 1300, b
    for row in m['rows']:
        r=row['rect']; assert r['x']>=-1 and r['x']+r['w']<=m['viewport'][0]+1, row
        assert str(row['hp']) in row['text'], row
        h=row['hpRect'];hb=row['hpBox'];assert h and hb and h['x']>=hb['x']-1 and h['x']+h['w']<=hb['x']+hb['w']+1,{'clippedHP':row}
    record(name,m);shot(name)
    drawn=json.loads(m['canvas'].get('renderedTanks','[]'))
    if m['viewport'][0]==320: assert all(t.get('screenWidth',0)>=24 for t in drawn if t.get('alive') and t.get('artReady')),drawn
    check(name,canvas=b,viewport=m['viewport'],players=m['rows'],foreground=foreground(m))


def contrast():
    return js(r'''(()=>{
 const rgb=s=>(s.match(/[\d.]+/g)||[]).map(Number),over=(f,b,o=1)=>b.map((v,i)=>v*(1-(f[3]??1)*o)+(f[i]??0)*(f[3]??1)*o),lum=c=>c.slice(0,3).map(v=>v/255).map(v=>v<=.04045?v/12.92:((v+.055)/1.055)**2.4).reduce((a,v,i)=>a+v*[.2126,.7152,.0722][i],0);
 return ['.fb-turn-strip strong','.fb-timer','.fb-gauge b','.fb-help','.fb-fire','.fortressblue-player > span','.fortressblue-player strong','.fortressblue-player strong em',...(document.querySelector('.fortressblue-player.dead')?['.fortressblue-player.dead > span','.fortressblue-player.dead strong']:[])].map(sel=>{const e=document.querySelector(sel),style=getComputedStyle(e),chain=[];for(let p=e;p;p=p.parentElement)chain.unshift(p);let bg=[255,255,255];for(const p of chain){const s=getComputedStyle(p);bg=over(rgb(s.backgroundColor),bg,Number(s.opacity))}const fg=over(rgb(style.color),bg,Number(style.opacity)),a=lum(fg),b=lum(bg);return {selector:sel,ratio:(Math.max(a,b)+.05)/(Math.min(a,b)+.05),fg,bg,disabled:e.disabled??null}});
 })()''')


opener = urllib.request.build_opener(urllib.request.HTTPCookieProcessor(http.cookiejar.CookieJar()))
def other(path, body=None):
    req=urllib.request.Request(base+'/api'+path,data=None if body is None else json.dumps(body).encode(),headers={'Content-Type':'application/json'})
    with opener.open(req,timeout=15) as response: return json.load(response)


def other_command(kind, **extra):
    r=other('/state')['room']
    return other('/rooms/'+r['id'],{'type':kind,'version':r['version'],'nonce':uuid.uuid4().hex,**extra})


def leave():
    current=api('/state').get('room')
    if current and current['status']=='playing': command('close')
    if current: command('leave')
    current=other('/state').get('room')
    if current: other_command('leave')


def start(scenario='normal', map_id='stage-16', bots=0, ui=False, tank_id=1):
    leave();(out/'scenario').write_text(scenario)
    if ui:
        browser('wait','.lobby-create');browser('click','.lobby-create');browser('fill','input[aria-label=搜索可选游戏]','疯狂坦克');browser('wait','.choose-games button');browser('click','.choose-games button');browser('wait','[data-create-step=setup]:not([hidden])')
        choices=js("[...document.querySelector('select[aria-label=\"座位上限\"]').options].map(o=>o.value)");assert choices==['2','4','6','8'],choices
        browser('select','select[aria-label="座位上限"]','2');browser('select','select[aria-label="疯狂坦克地图"]',map_id);browser('fill','input[name=name]','疯狂坦克原生操作验收');browser('click','[role=dialog] button[type=submit]');browser('wait','.waiting-actions')
        assert state()['options']['map']==map_id
        assert state()['options']['turnSeconds']==60,'new room must default to a full minute'
        check('shared creation modal selects game/map/two seats and 60-second default',capacityChoices=choices)
    else:
        api('/rooms',{'kind':'fortressblue','name':'疯狂坦克 '+scenario,'capacity':max(2,bots+1),'options':{'map':map_id,'turnSeconds':60}})
    if bots:
        for _ in range(bots): command('add_bot')
    else:
        r=state();other('/rooms/'+r['id'],{'type':'join','version':r['version'],'nonce':uuid.uuid4().hex});other_command('ready')
    command('ready');command('start')
    # The scripted opponent is an HTTP client. Acknowledge its real platform
    # load gate; the browser player must complete actual code/assets loading.
    if not bots:
        gate=other('/state')['room'].get('loading')
        if gate:
            client='fortress-http-opponent-'+uuid.uuid4().hex
            for operation in ('load_begin','load_ready'):
                other_command(operation,loadToken=gate['token'],loadBuild=gate['build'],loadClient=client)
    wait(lambda: not state().get('loading'),timeout=60)
    wait(lambda: js('!document.querySelector(".game-loading")'),timeout=60)
    wait(lambda: view()['phase'] in ('select','playing'))
    if scenario=='normal':
        wait(lambda: js('!!document.querySelector("[data-fortressblue-tank]")'))
        assert js('document.querySelectorAll("[data-fortressblue-tank]").length')==14
        browser('click','[data-fortressblue-tank="'+str(tank_id)+'"]')
        if not bots:
            v=other('/state')['room']['game']['data'];other_command('action',action={'type':'select','epoch':v['epoch'],'tank':1})
    wait(lambda: view()['phase']=='playing',timeout=30)
    wait(lambda: js('!!document.querySelector("[data-fortressblue-canvas]")'))
    epoch=view()['epoch']
    wait(lambda:js('document.querySelector(".fortressblue-board")?.dataset.fortressblueEpoch')==epoch)
    return view()


def facing_pixels(tank_id,direction,name):
    # Independently verified native primary block faces left; alternate faces right.
    # Compare actual world-canvas pixels, not just labels or selected indices.
    tank=json.loads((root/'games/fortressblue/data/animations.json').read_text())['tanks'][str(tank_id)]
    expression=r"""(async()=>{
      const frames=FRAMES,paths=PATHS;
      const images=await Promise.all(paths.map(src=>new Promise((resolve,reject)=>{const image=new Image();image.onload=()=>resolve(image);image.onerror=reject;image.src=src})));
      const c=document.querySelector('[data-fortressblue-canvas]'),t=JSON.parse(c.dataset.renderedTanks).find(t=>t.seat===0);
      const frame=frames[t.sourceIndex*2+BLOCK],ref=document.createElement('canvas');ref.width=frame.w;ref.height=frame.h;const rc=ref.getContext('2d');rc.drawImage(images[frame.atlas],frame.x,frame.y,frame.w,frame.h,0,0,frame.w,frame.h);
      const expected=rc.getImageData(0,0,frame.w,frame.h).data,cc=c.getContext('2d'),scale=c.width/Number(c.dataset.worldViewWidth),cx=Number(c.dataset.cameraX),cy=Number(c.dataset.cameraY);let count=0,matched=0;
      const left=(t.x-frame.ax-cx)*scale,top=(t.y-frame.ay-cy)*scale;
      for(let sy=Math.max(0,Math.floor(top));sy<Math.min(c.height,Math.ceil(top+frame.h*scale));sy++)for(let sx=Math.max(0,Math.floor(left));sx<Math.min(c.width,Math.ceil(left+frame.w*scale));sx++){
       const x=Math.floor((sx+.5-left)/scale),y=Math.floor((sy+.5-top)/scale);if(x<0||y<0||x>=frame.w||y>=frame.h)continue;
       const i=(y*frame.w+x)*4;if(expected[i+3]!==255)continue;
       const pixel=cc.getImageData(sx,sy,1,1).data;count++;if([0,1,2].every(k=>Math.abs(pixel[k]-expected[i+k])<=12))matched++;
      }
      return {tankId:TANK,direction:DIRECTION,sourceIndex:t.sourceIndex,artReady:t.artReady,count,matched,ratio:matched/Math.max(1,count),slope:t.slope};
    })()""".replace('FRAMES',json.dumps(tank['frames'])).replace('PATHS',json.dumps(tank['atlases'])).replace('BLOCK','0' if direction<0 else '1').replace('TANK',str(tank_id)).replace('DIRECTION',str(direction))
    result=js(expression);record(name,result);assert result['artReady'] and result['count']>=50 and result['ratio']>.85,result
    return result


def current_shot(): return view().get('lastShot',{}).get('id',0)


def ensure_own_turn():
    v=view()
    if v.get('resolution'): wait(lambda:not view().get('resolution'),timeout=15)
    v=view()
    if v['current']!=0:
        other_command('action',action={'type':'fire','epoch':v['epoch'],'turn':v['turn'],'angle':75,'power':100,'facing':1,'weapon':'primary'})
        wait(lambda:view()['current']==0 and not view().get('resolution'),timeout=18)
    wait(lambda:js('document.querySelector("[data-fortressblue-fire]")?.disabled===false'))


def hold(kind,ms,name):
    ensure_own_turn();before=current_shot();p=center('[data-fortressblue-fire]')
    down={'key':' '} if kind=='key' else {'mouse':'mousePressed',**p} if kind=='mouse' else {'touch':'touchStart',**p}
    up={'key':' ','up':True} if kind=='key' else {'mouse':'mouseReleased',**p} if kind=='mouse' else {'touch':'touchEnd'}
    if kind=='key': browser('focus','[data-fortressblue-canvas]')
    data=native([{'sample':True},down,{'wait':ms,'read':'held'},up,{'wait':750,'read':'released'}],name)
    held=next(f for f in data['frames'] if f.get('label')=='held'); released=data['frames'][-1]
    assert held['root']['fortressblueCharging']=='1' and held['publicState']['room']['game']['data'].get('lastShot',{}).get('id',0)==before, held
    after=released['publicState']['room']['game']['data']; assert after.get('lastShot',{}).get('id',0)>before
    assert sum(e['type']=='fire_ready' and e['at']>=held['at']-ms-500 for e in after['events'])==1
    power=int(released['root']['fortressbluePower']);check(name,power=power,shot=after['lastShot']['id'])
    return power


def full_charge(kind):
    ensure_own_turn();before=current_shot();p=center('[data-fortressblue-fire]')
    ticks=float(js('document.querySelector(".fortressblue-board").dataset.fortressblueChargeTicks'));hz=float(js('document.querySelector(".fortressblue-board").dataset.fortressblueChargeHz'));duration=int(ticks/hz*1000)+180
    down={'key':' '} if kind=='key' else {'touch':'touchStart',**p}
    up={'key':' ','up':True} if kind=='key' else {'touch':'touchEnd'}
    if kind=='key':browser('focus','[data-fortressblue-canvas]')
    data=native([down,{'wait':duration,'read':'at-full'},up,{'wait':500,'read':'released'}],'full-charge-'+kind)
    held,released=data['frames'];h=held['publicState']['room']['game']['data'];r=released['publicState']['room']['game']['data']
    assert h.get('lastShot',{}).get('id',0)>before and h['lastShot']['id']==r['lastShot']['id'],data
    assert int(held['root']['fortressbluePower'])==100 and held['root']['fortressblueCharging']=='0',held['root']
    check('full charge automatically fires once; release does not fire again '+kind,holdMS=duration,shot=h['lastShot']['id'])


def timeline(scenario,reload=False):
    start(scenario);cal=json.loads((out/('calibration-'+scenario+'.json')).read_text());before=measure();a=cal['action'];v=view();a.update(epoch=v['epoch'],turn=v['turn'])
    # Exact calibrated API shot isolates rendering/host timeline from charge QA.
    browser('record','start',str(out/('timeline-'+scenario+('-reload' if reload else '')+'.webm')))
    global recording;recording=True
    action(a)
    rejected=action({'type':'move','epoch':v['epoch'],'turn':v['turn'],'dir':1},expected=(400,))
    record('resolution-rejected-'+scenario,rejected)
    steps=[{'sample':True},{'wait':250,'read':'flying'}]
    if reload: steps += [{'reload':True},{'wait':250,'read':'reconnected'}]
    steps += [{'wait':min(12000,cal['flightMS']+2600),'read':'settled'}]
    data=native(steps,'timeline-'+scenario+('-reload' if reload else ''))
    browser('record','stop');recording=False
    frames=[f for f in data['frames'] if f.get('publicState',{}).get('room',{}).get('game')]
    flying=[f for f in frames if f['publicState']['room']['game']['data'].get('resolution',{}).get('stage')=='flying']
    settling=[f for f in frames if f['publicState']['room']['game']['data'].get('resolution',{}).get('stage')=='settling']
    assert len(flying)>=3 and settling, {'flight':len(flying),'settling':len(settling)}
    initial=before['publicState']['room']['game']['data']; shot_id=flying[0]['publicState']['room']['game']['data']['lastShot']['id']
    for f in flying:
        r=f['publicState']['room'];s=r['game']['data']
        assert not r['game'].get('actors') and s['deadline']==0 and not s['canFire'] and not s['canMove']
        assert s['tanks']==initial['tanks'] or all((t['hp'],t['x'],t['y'],t['alive'])==(initial['tanks'][i]['hp'],initial['tanks'][i]['x'],initial['tanks'][i]['y'],initial['tanks'][i]['alive']) for i,t in enumerate(s['tanks']))
        assert s.get('craters',[])==initial.get('craters',[]) and not s['phase']=='finished'
        if f['rows']: assert [x['hp'] for x in f['rows']]==[t['hp'] for t in initial['tanks']], f['rows']
    windup_frames=0
    for f in flying:
        s=f['publicState']['room']['game']['data'];shot_data=s['lastShot'];launch=shot_data.get('launchAt',shot_data['at'])
        if f['at']<launch:
            windup_frames+=1;assert int((f.get('canvas') or {}).get('projectiles','0'))==0,'projectile visible during launch preparation'
        elif f['at']>launch+250:
            assert any(e['type']=='shot' and e['at']==launch for e in s['events']),{'lateLaunchEventAt':f['at'],'launchAt':launch,'events':s['events']}
    if not reload: assert windup_frames>=1,{'windupFrames':windup_frames}
    projectile_frames=0
    for f in flying:
        s=f['publicState']['room']['game']['data'];at=float((f.get('canvas') or {}).get('renderedAt',0));drawn=json.loads((f.get('canvas') or {}).get('renderedProjectiles','[]'))
        if not drawn:continue
        arc=s['lastShot']['arcs'][0];times=arc.get('pathTimes',[]);elapsed=at-s['lastShot']['at']
        if not times or elapsed<times[0] or elapsed>times[-1]:continue
        idx=next((i for i in range(1,len(times)) if times[i]>=elapsed),len(times)-1);p0,p1=arc['path'][idx-1:idx+1];part=(elapsed-times[idx-1])/max(1,times[idx]-times[idx-1]);expected={k:p0[k]+(p1[k]-p0[k])*part for k in ['x','y']}
        assert max(abs(drawn[0][k]-expected[k]) for k in ['x','y'])<3,{'draw':drawn[0],'expected':expected,'at':at}
        projectile_frames+=1
    assert projectile_frames>=3,{'projectileFrames':projectile_frames}
    assert any(f['publicState']['room']['game']['data']['tanks'][1]['hp']<initial['tanks'][1]['hp'] for f in settling)
    assert any(f['publicState']['room']['game']['data'].get('craters') for f in settling)
    observed_fall_frames=0
    for f in settling:
        s=f['publicState']['room']['game']['data'];assert s['phase']!='finished' and not f['publicState']['room']['game'].get('actors')
        drawn=json.loads((f.get('canvas') or {}).get('renderedTanks','[]'));at=float((f.get('canvas') or {}).get('renderedAt',0))
        for fall in s['resolution'].get('falls',[]):
            tank=next((t for t in drawn if t['seat']==fall['seat']),None)
            if tank and fall['at']+60<at<fall['endAt']-50:
                observed_fall_frames+=1
                frac=(at-fall['at'])/(fall['endAt']-fall['at']);expected=fall['fromY']+(fall['toY']-fall['fromY'])*frac*frac
                assert abs(tank['y']-expected)<3, {'draw':tank,'expected':expected,'fall':fall,'at':at}
    if scenario=='timeline': assert observed_fall_frames>=2, {'observedFallFrames':observed_fall_frames}
    board_frames=[f['board'] for f in frames if f.get('board') and f['board']['w']>0]
    for dimension in ['y','w','h']:
        assert max(b[dimension] for b in board_frames)-min(b[dimension] for b in board_frames)<1.5,{'unstableBoardDimension':dimension,'range':[min(b[dimension] for b in board_frames),max(b[dimension] for b in board_frames)]}
    final=view();assert not final.get('resolution')
    if scenario=='lethal': assert final['phase']=='finished'
    else:
        assert final['turn']==initial['turn']+1 and final['deadline']>final['lastShot']['settleAt']+38000
    check('server flight/impact/fall timeline '+scenario+(' with reload' if reload else ''),flightFrames=len(flying),settlingFrames=len(settling),shot=shot_id,finalPhase=final['phase'])
    shot('after-'+scenario)
    return data

def effects(kind):
    start(kind);cal=json.loads((out/('calibration-'+kind+'.json')).read_text());initial=view();before_hp=[t['hp'] for t in initial['tanks']]
    if kind=='heal':
        browser('click','[data-fortressblue-item="2"]')
        wait(lambda:js('document.querySelector(".fortressblue-board")?.dataset.fortressbluePendingShot')=='heal')
        pending=view();assert [t['hp'] for t in pending['tanks']]==before_hp and pending['tanks'][0]['pendingShotEffect']=='heal'
        check('healing item arms a real projectile without immediate HP gain',hp=before_hp)
    v=view();a={**cal['action'],'epoch':v['epoch'],'turn':v['turn']}
    browser('record','start',str(out/('effect-'+kind+'.webm')))
    global recording;recording=True
    action(a)
    duration=cal['flightMS'];lifetime=12000 if kind=='poison' else 8000
    if kind=='heal':
        steps=[{'sample':True},{'wait':duration+140,'read':'healed','capture':'healing-impact'},{'wait':900,'read':'settled'}]
    else:
        steps=[{'sample':True},{'wait':duration+1200,'read':'first-tick','capture':kind+'-ground-effect'},{'wait':lifetime+300,'read':'expired','capture':kind+'-expired'}]
    data=native(steps,'effect-'+kind);browser('record','stop');recording=False
    frames=[f for f in data['frames'] if f.get('publicState',{}).get('room',{}).get('game') and f.get('canvas')]
    impact=next(f['publicState']['room']['game']['data']['lastShot']['impactAt'] for f in frames if f['publicState']['room']['game']['data'].get('lastShot'))
    flight=[f for f in frames if f['at']<impact-80];assert len(flight)>=3
    for f in flight:
        sv=f['publicState']['room']['game']['data'];draw=json.loads(f['canvas'].get('renderedFeedback','[]'))
        assert [t['hp'] for t in sv['tanks']]==before_hp and not sv.get('hazards')
        assert not any(e['type']=='heal' for e in draw)
    if kind=='heal':
        final=view();assert final['tanks'][1]['hp']>before_hp[1] and final.get('craters',[])==initial.get('craters',[])
        healing=[e for e in final['events'] if e['type']=='heal'];assert healing and all(e['at']==impact for e in healing)
        shown=[e for f in frames for e in json.loads(f['canvas'].get('renderedFeedback','[]')) if e['type']=='heal'];assert shown and all(e['at']==impact for e in shown)
        record('heal-events',{'events':healing,'visibleFeedback':shown});check('healing happens at impact with actual green feedback and no crater',before=before_hp,after=[t['hp'] for t in final['tanks']],drawnFrames=len(shown))
    else:
        observed=0;hazards={};status_drawn=[]
        for f in frames:
            sv=f['publicState']['room']['game']['data'];at=float(f['canvas']['renderedAt']);draw=json.loads(f['canvas'].get('renderedHazards','[]'))
            for h in sv.get('hazards',[]):hazards[h['id']]=h
            for d in draw:
                h=hazards.get(d['id']);assert h and h['at']<=at<h['endAt'],{'draw':d,'known':h,'renderedAt':at}
                assert abs(d['x']-h['x'])<.1 and abs(d['y']-h['y'])<.1 and d['kind']==kind,{'draw':d,'server':h}
                observed+=1
            for h in sv.get('hazards',[]):
                if h['at']+300<at<h['endAt']-100:assert any(d['id']==h['id'] for d in draw),{'missingHazard':h,'draw':draw,'at':at}
            status_drawn += [e for e in json.loads(f['canvas'].get('renderedFeedback','[]')) if e['type']=='status']
        assert observed>=5 and status_drawn,{'drawnHazardFrames':observed,'statusFeedbackFrames':len(status_drawn)}
        final=view();assert not final.get('hazards'),final.get('hazards');assert all(h['at']==impact for h in hazards.values())
        statuses=[e for e in final['events'] if e['type']=='status' and e.get('text')==kind];assert len(statuses)>=2 and all(e['at']>=impact+1000 for e in statuses),statuses
        assert final['tanks'][1]['hp']<before_hp[1]
        record(kind+'-events',{'hazards':hazards,'events':statuses,'feedback':status_drawn});check('ground '+kind+' appears at impact, ticks on server events, stays fixed and expires',drawnHazardFrames=observed,statusTicks=len(statuses),feedbackFrames=len(status_drawn))


def extinguish():
    start('fire-crater');cal=json.loads((out/'calibration-fire-crater.json').read_text());v=view();action({**cal['action'],'epoch':v['epoch'],'turn':v['turn']})
    before=native([{'sample':True},{'wait':cal['flightMS']+1300,'read':'ground-fire','capture':'fire-before-support-removal'}],'fire-before-support-removal')
    sv=view();assert not sv.get('resolution') and sv.get('hazards');hazard=sv['hazards'][0];old_craters=len(sv.get('craters',[]));follow={**cal['followupAction'],'epoch':sv['epoch'],'turn':sv['turn']}
    assert sv['current']==1
    other_command('action',action=follow)
    shot_state=view();impact=shot_state['lastShot']['impactAt'];assert impact<hazard['endAt']-500
    after=native([{'sample':True},{'wait':max(500,impact-int(time.time()*1000)+500),'read':'support-removed','capture':'fire-support-removed'}],'fire-support-removed')
    final=view();assert len(final.get('craters',[]))>old_craters and all(h['id']!=hazard['id'] for h in final.get('hazards',[]))
    f=after['frames'][-1];draw=json.loads(f['canvas'].get('renderedHazards','[]'));assert all(h['id']!=hazard['id'] for h in draw) and f['at']<hazard['endAt']
    check('later real crater removes ground hazard before lifetime expiration',hazard=hazard,secondImpact=impact,observedAt=f['at'])


def cancelled_windup(nonfatal=False):
    mode='windup-hit' if nonfatal else 'cancel';name='nonfatal-windup' if nonfatal else 'cancelled-windup'
    start(mode);v=view();h=v['hazards'][0];tick=h['nextTickAt'];initial=measure()
    ticks=float(initial['root']['fortressblueChargeTicks']);hz=float(initial['root']['fortressblueChargeHz']);hold_ms=int(ticks/hz*1000*.75)
    assert tick-int(time.time()*1000)>hold_ms+500,'fixture setup consumed cancellation scheduling window'
    browser('focus','[data-fortressblue-canvas]');browser('record','start',str(out/(name+'.webm')))
    global recording;recording=True
    data=native([{'sample':True},{'waitUntil':tick-hold_ms-180},{'key':' '},{'wait':hold_ms},{'key':' ','up':True},{'wait':50,'read':'armed'},{'wait':1500,'read':'cancelled','capture':name}],name)
    browser('record','stop');recording=False
    frames=[f for f in data['frames'] if f.get('publicState',{}).get('room',{}).get('game') and f.get('canvas')];final=view();shot_data=final.get('lastShot')
    if nonfatal:
        assert shot_data and not shot_data.get('cancelledAt') and shot_data['at']<tick<shot_data['launchAt'],shot_data
        actual=[];launch=shot_data['launchAt'];catalog=json.loads((root/'games/fortressblue/data/animations.json').read_text())['tanks']['1'];clip=catalog['actions']['fire'];base_source=catalog['frames'][clip['right'][5]]['sourceIndex']
        for f in frames:
            at=float(f['canvas']['renderedAt'])
            if launch<=at<launch+67:
                drawn=json.loads(f['canvas'].get('renderedTanks','[]'));tank=next(t for t in drawn if t['seat']==0)
                assert tank['action']=='fire' and tank['sourceIndex']==base_source-tank['slope'],{'tank':tank,'baseSourceIndex':base_source,'at':at,'launchAt':launch}
                assert tank['hp']==82 and tank['alive'],tank
                assert any(e['type']=='status' and e['target']==0 and e['at']==tick for e in json.loads(f['canvas'].get('renderedFeedback','[]'))),f['canvas']
                actual.append({'at':at,'tank':tank})
        assert actual,'no actual drawing samples in native projectile frame 5 window'
        assert any(e['type']=='shot' and e['at']==launch for e in final['events'])
        check('nonfatal ground hit preserves actual native Fire frame 5 and simultaneous HP feedback',hitAt=tick,launchAt=launch,samples=actual)
        return
    assert shot_data and shot_data.get('cancelledAt')==tick and shot_data['at']<tick<shot_data['launchAt'],shot_data
    assert final['tanks'][0]['hp']==0 and not final['tanks'][0]['alive']
    assert not any(e['type'] in ['shot','explode'] for e in final['events']),final['events']
    ready=[f for f in frames if f['publicState']['room']['game']['data'].get('resolution') and f['at']<tick];assert ready,'no actually observed windup before real hazard death'
    late=[f for f in frames if f['at']>shot_data['launchAt']+100];assert len(late)>=5
    baseline=ready[-1]['canvas']
    for f in late:
        c=f['canvas'];assert int(c['projectiles'])==0 and not json.loads(c.get('renderedProjectiles','[]')),c
        assert max(abs(float(c[k])-float(baseline[k])) for k in ['cameraX','cameraY'])<2,{'before':baseline,'after':c}
    check('real hazard death cancels windup without ghost projectile, tracking, launch or explosion',shotAt=shot_data['at'],cancelledAt=tick,plannedLaunch=shot_data['launchAt'],lateFrames=len(late))



try:
    if args.build: subprocess.run(['npm','--prefix','web','run','build'],cwd=root,env=env,check=True)
    assert (root/'web/dist/index.html').exists(),'Build frontend first'
    (out/'scenario').write_text('normal')
    log=(out/'server.log').open('w')
    if args.only in ('facing','portraits','supplies','projectiles','slope'):
        # Use the production binary with its real CDN/local-fallback settings;
        # no opening fixture or response rewriting is needed for direction tests.
        executable=temp/'server';subprocess.run(['go','build','-o',str(executable),'.'],cwd=root,env=env,stdout=log,stderr=subprocess.STDOUT,check=True)
        with socket.socket() as listener:listener.bind(('127.0.0.1',0));port=listener.getsockname()[1]
        base='http://127.0.0.1:'+str(port)
        env.update(ADDR='127.0.0.1:'+str(port),DATA_DIR=str(temp/'data'),INVITE_CODE='fortress-browser',ADMIN_USERNAME='坦克验收管理员',ADMIN_PASSWORD='fortress-browser-test-only',ADMIN_EXISTING_USERNAME='',COOKIE_SECURE='false',PUBLIC_ORIGIN=base,ASSETS_BASE_URL='https://cdn.ichenj.com/bga/v20261002-1')
        server=subprocess.Popen([str(executable)],cwd=root,env=env,stdout=log,stderr=subprocess.STDOUT)
        deadline=time.monotonic()+30
        while True:
            assert server.poll() is None,'server exited; inspect server.log'
            try:
                with urllib.request.urlopen(base+'/healthz',timeout=1) as response:
                    if response.status==200:break
            except (OSError,urllib.error.URLError):pass
            assert time.monotonic()<deadline,'server startup timeout';time.sleep(.1)
        (out/'url').write_text(base)
        report['scope']='Production binary/HTTP/SQLite/clock and CDN-local fallback; native browser input; no game-state fixtures'
    else:
        server=subprocess.Popen(['go','test','-count=1','-timeout=27m','-run','^TestFortressBlueBrowserFixture$','./games/fortressblue/tests'],cwd=root,env=env,stdout=log,stderr=subprocess.STDOUT)
        deadline=time.monotonic()+180
        while not (out/'url').exists():
            assert server.poll() is None,'fixture exited; inspect server.log'
            assert time.monotonic()<deadline,'fixture start timeout'
            time.sleep(.1)
    base=(out/'url').read_text();browser('open',base);opened=True
    api('/login',{'name':'坦克验收管理员','password':'fortress-browser-test-only'})
    other('/register',{'name':'坦克静止对手','password':'fortress-browser-test-only','invite':'fortress-browser'})
    browser('reload');browser('set','viewport','1280','1000')
    if args.only=='slope':
        catalog=json.loads((root/'games/fortressblue/maps/catalog.json').read_text())
        terrain=next(m for m in catalog['maps'] if m['id']=='stage-16')
        bits=base64.b64decode(terrain['mask']);cell=catalog['cell']
        def solid(x,y):
            if not 0<=x<terrain['width'] or not 0<=y<terrain['height']:return False
            i=math.floor(y/cell)*terrain['maskCols']+math.floor(x/cell)
            return bool(bits[i>>3]&(1<<(7-(i&7))))
        def verify_dial(name):
            data=view();tank=data['tanks'][data['you']]
            def foot(x):
                ys=[y for y in range(math.floor(tank['y'])-24,math.ceil(tank['y'])+25) if solid(x,y) and not solid(x,y-1)]
                return min(ys,key=lambda y:abs(y-tank['y'])) if ys else None
            left,right=foot(tank['x']-12),foot(tank['x']+12)
            slope=math.degrees(math.atan2(right-left,24)) if left is not None and right is not None else 0
            dial=js(r'(()=>{const b=document.querySelector(".fortressblue-board"),s=document.querySelector(".fb-angle-dial"),p=s.querySelector("line[stroke=\"#ffdf79\"]");return {local:Number(b.dataset.fortressblueAngle),facing:Number(b.dataset.fortressblueFacing),text:s.querySelector("text").textContent,aria:s.getAttribute("aria-label"),x:Number(p.getAttribute("x2")),y:Number(p.getAttribute("y2"))}})()')
            world=dial['local']-dial['facing']*slope
            assert dial['text'].strip()==str(math.floor(world+.5))+'°',(name,dial,world)
            assert abs(dial['x']-(65+dial['facing']*math.cos(math.radians(world))*40))<.01
            assert abs(dial['y']-(57-math.sin(math.radians(world))*40))<.01
            evidence={'tank':tank,'support':[left,right],'slope':slope,'worldAngle':world,'dial':dial}
            record(name,evidence);shot(name);return evidence
        for width in (1280,320):
            browser('set','viewport',str(width),'1000' if width==1280 else '844')
            start(ui=width==1280)
            initial=verify_dial('slope-initial-'+str(width))
            assert abs(initial['slope'])>1,'opening must actually exercise a slope'
            browser('click','button[aria-label="切换朝向"]');verify_dial('slope-reversed-'+str(width))
            browser('focus','[data-fortressblue-canvas]')
            native([{'key':'ArrowRight'},{'wait':170},{'key':'ArrowRight','up':True},{'wait':150}],'slope-movement-'+str(width))
            after=verify_dial('slope-after-move-'+str(width))
            assert after['tank']['x']>initial['tank']['x']
            assert abs(after['slope']-initial['slope'])>1,(initial,after)
            assert after['dial']['local']==initial['dial']['local'],'movement rewrote relative aim'
            check('body slope changes world angle and dial while relative aim stays fixed '+str(width),before=initial,after=after)
    if args.only=='projectiles':
        browser('network','route','https://cdn.ichenj.com/bga/v20261002-1/fortressblue/**','--abort')
        for tank_id in args.tanks:
            for mode in ['primary','special']:
                width=320 if mode=='special' and tank_id in (7,10,13,14) else 1280
                browser('set','viewport',str(width),'844' if width==320 else '1000')
                start(tank_id=tank_id,ui=tank_id==1 and mode=='primary')
                wait(lambda:js('[...document.querySelectorAll(".fb-player-head img")].every(i=>i.complete&&i.naturalWidth>0)'))
                browser('click','[data-fortressblue-weapon="'+mode+'"]')
                wait(lambda:js('document.querySelector('+json.dumps('[data-fortressblue-weapon="'+mode+'"]')+').getAttribute("aria-pressed")')=='true')
                browser('focus','[data-fortressblue-canvas]')
                name='projectile-'+str(tank_id)+'-'+mode
                steps=[{'sample':True},{'key':' '},{'wait':50 if tank_id==6 and mode=='special' and len(args.tanks)<14 else 700},{'key':' ','up':True},{'wait':450,'read':'flying'}]
                if mode=='special' and tank_id in (1,7,8,9,10,13,14):steps[-1]['capture']=name
                data=native(steps)
                current=view();shot_data=current['lastShot'];assert shot_data.get('mode')==mode,shot_data
                count=({4:3,7:9,8:3,9:2,12:2,13:9}.get(tank_id,1) if mode=='special' else (3 if tank_id in (7,13) else 1))
                if tank_id==10 and mode=='special' and not shot_data['arcs'][0].get('outOfBounds'):count=2
                assert len(shot_data['arcs'])==count,(tank_id,mode,len(shot_data['arcs']))
                tail_steps=[{'sample':True}]
                if mode=='special' and tank_id==10 and len(shot_data['arcs'])>1:
                    tail_steps.append({'waitUntil':shot_data['arcs'][1]['launchAt']+80,'read':'ion-beam','capture':'ion-beam-native'})
                if mode=='special' and tank_id==6 and len(args.tanks)<14:
                    lock_at=shot_data['arcs'][0].get('lockAt');assert lock_at,'low-power descending shot should lock own visible tank'
                    tail_steps.append({'waitUntil':lock_at+30,'read':'lock','capture':'homing-native'})
                tail_steps.extend([{'waitUntil':max(a['impactAt'] for a in shot_data['arcs'])+200},{'read':'settled'}])
                if mode=='special' and tank_id==9:tail_steps[-1]['capture']='mines-native'
                tail=native(tail_steps)
                frames=data['frames']+tail['frames'];painted=[];brief=[]
                for f in frames:
                    canvas=f.get('canvas') or {};now=float(canvas.get('renderedAt',0));drawn=json.loads(canvas.get('renderedProjectiles','[]'));blasts=json.loads(canvas.get('renderedBlasts','[]'))
                    for q in drawn:
                        assert q['launchAt']<=now<q['impactAt'],(q,now)
                        if q.get('effect')!='ion_beam':
                            assert q['mode']==mode and q['path']=='/fortressblue/v5/projectile-'+str(tank_id)+'.webp',q
                            assert (32<=q['frame']<64) if mode=='special' else (0<=q['frame']<32),q
                        if q.get('artReady'):painted.append(q)
                    assert all(b['at']<=now for b in blasts)
                    brief.append({'at':f['at'],'renderedAt':now,'projectiles':drawn,'blasts':blasts,'hp':[r['hp'] for r in f.get('rows',[])],'cameraY':canvas.get('cameraY'),'overflow':f.get('overflow'),'mines':json.loads(canvas.get('renderedMines','[]')),'locks':json.loads(canvas.get('renderedLocks','[]'))})
                assert len(painted)>=2,(tank_id,mode,'native projectile not actually drawn')
                if mode=='special' and tank_id==9:
                    assert any(m['artReady'] for f in brief for m in f['mines']),'mine image was never drawn'
                if mode=='special' and tank_id==10 and len(shot_data['arcs'])>1:
                    assert any(q.get('effect')=='ion_beam' and q['artReady'] for q in painted),'delayed native ion beam was never drawn'
                if mode=='special' and tank_id==6 and len(args.tanks)<14:
                    assert any(m['artReady'] for f in brief for m in f['locks']),'native target lock reticle was never drawn'
                assert not any(f['overflow'] for f in brief)
                if count>2:assert max(len(f['projectiles']) for f in brief)>=count,(tank_id,mode,'volley truncated')
                record(name,{'tank':tank_id,'mode':mode,'width':width,'shot':shot_data,'frames':brief})
                check('native projectile '+str(tank_id)+' '+mode,arcs=count,nativeFrames=len(painted),width=width)
    if args.only=='supplies':
        browser('network','route','https://cdn.ichenj.com/bga/v20261002-1/fortressblue/**','--abort')
        start(ui=True)
        assert state()['options']['botDifficulty']=='normal'
        def harmless_fire():
            data=view();seat=data['current'];payload={'type':'fire','epoch':data['epoch'],'turn':data['turn'],'angle':0,'power':100,'facing':-1 if seat==0 else 1,'weapon':'primary'}
            if seat==0:action(payload)
            else:other_command('action',action=payload)
            wait(lambda:not view().get('resolution'),timeout=15,label='actual shell completes')
        for _ in range(7):
            assert not view().get('supplies'), 'supply appeared before turn eight'
            harmless_fire()
        assert view()['turn']==8
        delivery=wait(lambda:(view().get('supplies') or [None])[0],label='scheduled original-style supply')
        record('supply-original-schedule',delivery)
        data=native([{'sample':True},{'wait':500,'read':'transport','capture':'supply-transport'},{'wait':1150,'read':'falling','capture':'supply-falling'},{'reload':True},{'wait':2400,'read':'landed','capture':'supply-landed-after-reload'}],'supply-flight')
        airborne=[d for f in data['frames'] for d in json.loads((f.get('canvas') or {}).get('renderedSupplies','[]')) if d.get('id')==delivery['id'] and d.get('airborne')]
        assert len(airborne)>=3 and sum(d['artReady'] for d in airborne)>=3,{'airborneFrames':len(airborne),'originalArtFrames':sum(d['artReady'] for d in airborne)}
        reloaded_at=next(f['at'] for f in data['frames'] if f.get('label')=='falling')
        fallback_frames=[f for f in data['frames'] if any(d.get('id')==delivery['id'] and not d.get('artReady') for d in json.loads((f.get('canvas') or {}).get('renderedSupplies','[]')))]
        assert all(reloaded_at<=f['at']<reloaded_at+1000 for f in fallback_frames),'Original art failed to recover after reload'
        landed_frame=next(f for f in data['frames'] if f.get('label')=='landed')
        assert any(d.get('id')==delivery['id'] and d.get('artReady') for d in json.loads(landed_frame['canvas']['renderedSupplies']))
        assert max(d['y'] for d in airborne)-min(d['y'] for d in airborne)>20
        live=wait(lambda:next((d for d in view().get('supplies',[]) if d['id']==delivery['id'] and d.get('landed')),None),timeout=6)
        assert live['at']==delivery['at'] and live['dropAt']==delivery['dropAt'] and live['landAt']==delivery['landAt']
        check('original supply falls continuously and reload preserves server schedule',airborneFrames=len(airborne),delivery=delivery)
        current=view();target=min(current['tanks'],key=lambda t:abs(t['x']-live['x']))['seat']
        assert abs(current['tanks'][target]['y']-live['y'])<=8,{'tank':current['tanks'][target],'supply':live}
        if current['current']!=target:harmless_fire()
        before=view()['tanks'][target];inventory=len(before['items']);hp=before['hp']
        for _ in range(12):
            current=view();tank=current['tanks'][target]
            if abs(tank['x']-live['x'])<=26:break
            payload={'type':'move','epoch':current['epoch'],'turn':current['turn'],'dir':1 if live['x']>tank['x'] else -1}
            if target==0:action(payload)
            else:other_command('action',action=payload)
        # Touch and immediately reverse, rather than parking for the 180ms
        # presentation delay. This reproduces a quick pass through the edge.
        touched=view();collector=touched['tanks'][target]
        pending=next(d for d in touched['supplies'] if d['id']==live['id'])
        assert pending.get('pickupAt',0)>0 and len(collector['items'])==inventory
        away=-1 if collector['x']<live['x'] else 1
        payload={'type':'move','epoch':touched['epoch'],'turn':touched['turn'],'dir':away}
        if target==0:action(payload)
        else:other_command('action',action=payload)
        escaped=view();assert abs(escaped['tanks'][target]['x']-live['x'])>26
        record('supply-quick-contact',{'touched':collector,'away':escaped['tanks'][target],'scheduledPickup':pending})
        wait(lambda:not any(d['id']==live['id'] for d in view().get('supplies',[])),timeout=5,label='quick contact pickup after moving away')
        after=view();tank=after['tanks'][target];assert len(tank['items'])==inventory+1 and tank['hp']==hp
        pickup=next(e for e in after['events'] if e['type']=='supply_pickup' and e['seat']==target)
        assert pickup['at']>=delivery['landAt']+180
        time.sleep(.4);assert len(view()['tanks'][target]['items'])==inventory+1
        shot('supply-picked-up');record('supply-pickup',{'before':before,'after':tank,'event':pickup});check('quick contact stays latched after moving away and adds one item without healing',seat=target)
        browser('set','viewport','320','844');start()
        for _ in range(7):
            assert not view().get('supplies'), 'mobile supply appeared before turn eight'
            harmless_fire()
        assert view()['turn']==8
        mobile_delivery=wait(lambda:(view().get('supplies') or [None])[0])
        mobile=native([{'sample':True},{'wait':550,'read':'transport','capture':'supply-mobile-transport'},{'wait':1000,'read':'falling','capture':'supply-mobile-falling'},{'wait':2500,'read':'landed','capture':'supply-mobile-landed'}],'supply-mobile-flight')
        in_flight=[f for f in mobile['frames'] if f.get('root',{}).get('fortressblueTransporting')=='1']
        assert in_flight and all(not any('行动中' in row.get('text','') for row in f.get('rows',[])) and not f['publicState']['room']['game'].get('actors') and f['publicState']['room']['game']['data']['deadline']==0 and not f['overflow'] for f in in_flight)
        assert all(float(f.get('canvas',{}).get('cameraY',0))>=-.01 for f in mobile['frames'])
        layout('supply-mobile-playable');check('320px supply transport stops action timer and stays within original map',frames=len(in_flight),delivery=mobile_delivery)
        for level,width in [('easy',320),('normal',1280),('hard',320)]:
            leave();browser('set','viewport',str(width),'844');browser('wait','.lobby-create');browser('click','.lobby-create');browser('fill','input[aria-label=搜索可选游戏]','疯狂坦克');browser('click','.choose-games button');browser('wait','select[aria-label="疯狂坦克电脑难度"]')
            assert js("document.querySelector('select[aria-label=\"疯狂坦克电脑难度\"]').value")=='normal'
            browser('select','select[aria-label="疯狂坦克电脑难度"]',level);shot('difficulty-'+level)
            browser('fill','input[name=name]','难度 '+level);browser('click','[role=dialog] button[type=submit]');browser('wait','.waiting-actions')
            assert state()['options']['botDifficulty']==level
            assert not js('document.documentElement.scrollWidth>innerWidth+2')
            check('difficulty selection persists '+level,viewport=width)
    if args.only=='portraits':
        # Failure cases: source lobby portraits are baked blue; selected/unselected
        # cards diverge; atlas outage regresses to blue; all media missing hides
        # identity; 320px crops cards or leaves later rows unreachable.
        browser('network','route','https://cdn.ichenj.com/bga/v20261002-1/fortressblue/**','--abort')
        for mode,width in [('desktop',1280),('mobile',320),('idle-fallback',320),('no-media',320)]:
            leave();browser('set','viewport',str(width),'1000' if width>320 else '844')
            if mode=='idle-fallback':browser('network','route','**/fortressblue/v3/**','--abort')
            if mode=='no-media':browser('network','route','**/fortressblue/v2/**','--abort')
            api('/rooms',{'kind':'fortressblue','name':'彩色选车 '+mode,'capacity':2,'options':{'map':'stage-16','turnSeconds':40}})
            r=state();other('/rooms/'+r['id'],{'type':'join','version':r['version'],'nonce':uuid.uuid4().hex});other_command('ready')
            command('ready');command('start');browser('wait','[data-fortressblue-tank]')
            # New page intentionally clears image cache between network scenarios.
            browser('reload');browser('wait','[data-fortressblue-tank]')
            expected='fallback' if mode=='no-media' else 'idle' if mode=='idle-fallback' else 'atlas'
            wait(lambda:js("[...document.querySelectorAll('[data-fortressblue-tank] .fb-tank-portrait')].filter(e=>e.dataset.portraitState===" +json.dumps(expected)+').length')==14,timeout=8,label='all fourteen original-color portraits '+mode)
            palette=js(r'''(async()=>{
              const catalog=CATALOG, results=[];
              for(const card of document.querySelectorAll('[data-fortressblue-tank]')){
                const id=card.dataset.fortressblueTank,c=card.querySelector('canvas'),pixels=c.getContext('2d').getImageData(0,0,c.width,c.height).data;
                let opaque=0,bright=0,minX=c.width,maxX=0,minY=c.height,maxY=0;const colors=new Set();
                for(let i=0;i<pixels.length;i+=4)if(pixels[i+3]>250){opaque++;const x=(i/4)%c.width,y=Math.floor(i/4/c.width);minX=Math.min(minX,x);maxX=Math.max(maxX,x);minY=Math.min(minY,y);maxY=Math.max(maxY,y);if(Math.max(...pixels.slice(i,i+3))>100)bright++;colors.add([...pixels.slice(i,i+3)].join(','))}
                let originalPalette=true;
                if(c.dataset.portraitState!=='fallback'){
                  const t=catalog[id],f=t.frame,im=new Image();im.src=c.dataset.portraitState==='atlas'?t.atlases[f.atlas]:'/fortressblue/v2/tanks/'+id+'-idle.webp';await im.decode();
                  const ref=document.createElement('canvas');ref.width=im.naturalWidth;ref.height=im.naturalHeight;const ctx=ref.getContext('2d');ctx.drawImage(im,0,0);
                  const src=c.dataset.portraitState==='atlas'?ctx.getImageData(f.x,f.y,f.w,f.h).data:ctx.getImageData(0,0,ref.width,ref.height).data;
                  const original=new Set();for(let i=0;i<src.length;i+=4)if(src[i+3]>250)original.add([...src.slice(i,i+3)].join(','));
                  originalPalette=[...colors].every(color=>original.has(color));
                }
                const box=c.getBoundingClientRect(),scale=Math.min(box.width/c.width,box.height/c.height);results.push({id,opaque,bright,colors:colors.size,originalPalette,visibleWidth:(maxX-minX+1)*scale,visibleHeight:(maxY-minY+1)*scale});
              }return results;
            })()'''.replace('CATALOG',json.dumps({k:{'atlases':t['atlases'],'frame':t['frames'][t['actions']['idle']['right'][0]]} for k,t in json.loads((root/'games/fortressblue/data/animations.json').read_text())['tanks'].items()})))
            assert all(p['opaque']>100 and p['bright']>50 and p['colors']>=3 and p['originalPalette'] and p['visibleWidth']>=30 and p['visibleHeight']>=30 for p in palette),palette
            record('picker-pixels-'+mode,palette)
            rows=js("[...document.querySelectorAll('[data-fortressblue-tank]')].map(e=>{const c=e.querySelector('.fb-tank-portrait'),r=e.getBoundingClientRect(),s=getComputedStyle(c);return {id:e.dataset.fortressblueTank,disabled:e.disabled,state:c.dataset.portraitState,opacity:s.opacity,filter:s.filter,x:r.x,width:r.width,name:e.querySelector('strong').textContent}})")
            assert len(rows)==14 and all(not r['disabled'] and r['opacity']=='1' and r['filter']=='none' and r['x']>=0 and r['x']+r['width']<=width+1 for r in rows),rows
            shot('picker-'+mode+'-top')
            browser('click','[data-fortressblue-tank="14"]');wait(lambda:js("document.querySelector('[data-fortressblue-tank=\"14\"]')?.classList.contains('selected')"))
            assert js("document.querySelector('[data-fortressblue-tank=\"14\"] .fb-tank-portrait')?.dataset.portraitState")==expected
            shot('picker-'+mode+'-selected-bottom')
            record('picker-'+mode,rows);check('all 14 selectable portraits retain color/identity '+mode,cards=rows)
    if args.only=='facing':
        browser('network','route','https://cdn.ichenj.com/bga/v20261002-1/fortressblue/**','--abort')
        assert api('/state')['assetsBaseURL']=='https://cdn.ichenj.com/bga/v20261002-1'
        for tank_id in [4,*[i for i in range(1,15) if i!=4]]:
            browser('set','viewport','1280','1000');start(tank_id=tank_id);ensure_own_turn()
            wait(lambda:all(t['artReady'] for t in json.loads(measure()['canvas']['renderedTanks'])))
            for direction in [-1,1]:
                if int(js('document.querySelector(".fortressblue-board").dataset.fortressblueFacing'))!=direction:browser('click','button[aria-label="切换朝向"]')
                time.sleep(.12);name='tank-'+str(tank_id)+('-left' if direction<0 else '-right');evidence=facing_pixels(tank_id,direction,name);shot(name);check(name+' actual native sprite direction',pixels=evidence)
        start(tank_id=4);ensure_own_turn()
        requests=js("performance.getEntriesByType('resource').map(r=>r.name)");record('direction-local-fallback-requests',requests)
        assert any('cdn.ichenj.com' in url and 'fortressblue/v3/' in url for url in requests)
        assert any(url.startswith(base+'/fortressblue/v3/') for url in requests)
        wait(lambda:all(t['artReady'] for t in json.loads(measure()['canvas']['renderedTanks'])))
        if int(js('document.querySelector(".fortressblue-board").dataset.fortressblueFacing'))!=-1:browser('click','button[aria-label="切换朝向"]')
        time.sleep(.1);facing_pixels(4,-1,'local-fallback-pixels');shot('local-fallback');check('CDN failure uses local atlas with the same visible direction')
        for width,direction,weapon in [(1280,-1,'primary'),(320,1,'special')]:
            browser('set','viewport',str(width),'844' if width==320 else '1000');start(tank_id=4);ensure_own_turn();browser('focus','[data-fortressblue-canvas]');before=view()['tanks'][0]['x'];key='ArrowLeft' if direction<0 else 'ArrowRight'
            data=native([{'sample':True},{'key':key},{'wait':200,'read':'moving'},{'key':key,'up':True},{'wait':200,'read':'stopped'}],'move-'+str(width));after=view()['tanks'][0]
            assert (after['x']-before)*direction>0 and after['facing']==direction,(before,after)
            facing_pixels(4,direction,'moved-pixels-'+str(width));check('native movement, visible sprite and server facing agree '+str(width))
            browser('click','[data-fortressblue-weapon="'+weapon+'"]');browser('scrollintoview','[data-fortressblue-fire]');point=center('[data-fortressblue-fire]')
            down={'touch':'touchStart',**point} if width==320 else {'mouse':'mousePressed',**point};up={'touch':'touchEnd'} if width==320 else {'mouse':'mouseReleased',**point}
            data=native([{'sample':True},down,{'wait':500},up,{'wait':220,'read':'windup','capture':'fire-'+str(width)},{'wait':650,'read':'flight'}],'fire-direction-'+str(width));shot_data=view()['lastShot'];path=shot_data['arcs'][0]['path']
            assert (path[1]['x']-path[0]['x'])*direction>0 and shot_data['weapon']=={'primary':'主炮','special':'特殊弹'}[weapon],shot_data
            assert any(any(t['seat']==0 and t['action'] in ['fire','special'] for t in json.loads(f['canvas'].get('renderedTanks','[]'))) for f in data['frames'] if f.get('canvas'))
            check('native '+weapon+' shot and Fire animation agree '+str(width),direction=direction,pathStart=path[:2]);shot('flight-'+str(width))
    if args.only in ('all','interaction'):
        start(ui=True)
        wait(lambda:len(measure()['pixels'])>=8)
        for width,height in [(1280,1000),(2560,1080),(320,844)]:
            browser('set','viewport',str(width),str(height));time.sleep(.3);layout('playing-'+str(width))
        ratios=contrast();assert all(x['ratio']>=4.5 for x in ratios),ratios;check('computed text contrast',values=ratios)
        browser('set','viewport','1280','1000')
        # Native mouse / keyboard / touch charging, with strict no-fire while held.
        powers={}
        for kind in ['key','mouse','touch']:
            if kind=='touch':browser('set','viewport','320','844');browser('scrollintoview','[data-fortressblue-fire]')
            for ms in [150,1300]: powers[kind+str(ms)]=hold(kind,ms,'charge-'+kind+'-'+str(ms))
            assert powers[kind+'1300']>powers[kind+'150']+30,powers
        check('short and long native charge differ for keyboard/mouse/touch',powers=powers)
        full_charge('touch')
        browser('set','viewport','1280','1000');full_charge('key')
        ensure_own_turn();before=current_shot();p=center('[data-fortressblue-fire]')
        cancelled=native([{'touch':'touchStart',**p},{'wait':400},{'touch':'touchCancel'},{'wait':250,'read':'cancelled'}],'touch-cancel')
        assert current_shot()==before and cancelled['frames'][-1]['root']['fortressblueCharging']=='0';check('touch cancellation does not fire')
        browser('set','viewport','1280','1000');browser('focus','[data-fortressblue-canvas]')
        before=current_shot();native([{'key':' '},{'wait':200},{'key':'Escape'},{'key':'Escape','up':True},{'key':' ','up':True},{'wait':200}],'escape-cancel')
        assert current_shot()==before;check('escape cancels charge')
        native([{'key':' '},{'wait':200},{'blur':True},{'key':' ','up':True},{'wait':200}],'blur-cancel')
        assert current_shot()==before;check('native window focus loss cancels charge')
        # Camera interactions. Zoom first to avoid boundary clamping during pan.
        browser('click','button[aria-label="放大地图"]');browser('click','button[aria-label="放大地图"]');r=rect('[data-fortressblue-canvas]');p={'x':r['x']+r['w']*.6,'y':r['y']+r['h']*.6}
        old=camera();native([{'mouse':'mousePressed',**p},{'mouse':'mouseMoved','x':p['x']-100,'y':p['y']-30},{'mouse':'mouseReleased','x':p['x']-100,'y':p['y']-30},{'wait':100}],'mouse-pan');new=camera()
        assert abs(new['cameraX']-old['cameraX'])>10 and current_shot()==before;check('native mouse drag pans without firing',before=old,after=new)
        old=camera();native([{'mouse':'mouseWheel',**p,'dy':-120},{'wait':150}],'wheel-zoom');new=camera();assert new['worldViewWidth']<old['worldViewWidth']
        anchor_old=old['cameraX']+.6*old['worldViewWidth'];anchor_new=new['cameraX']+.6*new['worldViewWidth'];assert abs(anchor_old-anchor_new)<3,(old,new);check('wheel zoom preserves cursor world anchor',error=abs(anchor_old-anchor_new))
        m=measure();mini=json.loads(m['canvas']['minimap']);r=m['board'];a={'x':r['x']+mini['x']+mini['w']*.2,'y':r['y']+mini['y']+mini['h']*.65};b={'x':r['x']+mini['x']+mini['w']*.8,'y':a['y']}
        data=native([{'mouse':'mousePressed',**a},{'read':'mini-start'},{'mouse':'mouseMoved',**b},{'read':'mini-drag'},{'mouse':'mouseReleased',**b}],'minimap-drag');assert abs(float(data['frames'][1]['canvas']['cameraX'])-float(data['frames'][0]['canvas']['cameraX']))>100;check('minimap continuously tracks drag before release')
        browser('set','viewport','320','844');r=rect('[data-fortressblue-canvas]');p={'x':r['x']+r['w']*.5,'y':r['y']+r['h']*.6};old=camera()
        native([{'touch':'touchStart',**p},{'touch':'touchMove','x':p['x']-50,'y':p['y']},{'touch':'touchEnd'},{'wait':100}],'touch-pan');new=camera();assert abs(old['cameraX']-new['cameraX'])>10;check('native touch pan',before=old,after=new)
        points=[{'id':1,'x':p['x']-25,'y':p['y']},{'id':2,'x':p['x']+25,'y':p['y']}];spread=[{'id':1,'x':p['x']-55,'y':p['y']},{'id':2,'x':p['x']+55,'y':p['y']}];old=camera()
        native([{'touch':'touchStart','points':points},{'touch':'touchMove','points':spread},{'touch':'touchEnd'},{'wait':100}],'touch-pinch');new=camera();assert new['worldViewWidth']<old['worldViewWidth'];assert current_shot()==before;check('native two-finger pinch zoom without fire',before=old,after=new)
        browser('set','viewport','1280','1000');browser('click','button[aria-label="跟随当前坦克"]')
        # Actual delayed/failed image requests; pixel checks identify terrain itself.
        delayed=native([{'network':{'pattern':'*fortressblue*fg*','delay':1800}},{'reload':True},{'wait':700,'read':'pending','capture':'delayed-foreground-pending'},{'wait':2300,'read':'loaded','capture':'delayed-foreground-loaded'}],'delayed-foreground')
        assert delayed['network'],'foreground interception did not match any request'
        for f in delayed['frames']:foreground(f)
        assert delayed['frames'][0]['terrainReady']=='0' and delayed['frames'][-1]['terrainReady']=='1',[(f.get('label'),f.get('terrainReady')) for f in delayed['frames']]
        assert len({tuple(p['terrain'][:3]) for p in delayed['frames'][-1]['pixels']})>=4,'foreground artwork did not replace flat fallback'
        check('delayed foreground remains visible and repaints',requests=len(delayed['network']))
        failed=native([{'network':{'pattern':'*fortressblue*','block':'fortressblue'}},{'reload':True},{'wait':1500,'read':'fallback','capture':'all-images-failed-fallback'}],'failed-images')
        assert failed['network'];assert failed['frames'][-1]['terrainReady']=='0';foreground(failed['frames'][-1]);check('failed image terrain fallback is actually visible',requests=len(failed['network']))
        browser('reload');wait(lambda:js('!!document.querySelector("[data-fortressblue-canvas]")'))
        # Real representative original terrains, not an unrelated full game matrix.
        for map_id in ['stage-2','stage-4']:
            start(map_id=map_id);layout('map-'+map_id)
    if args.only in ('all','timeline'):
        timeline('timeline',reload=True)
        timeline('lethal')
        dead_ratios=contrast();assert all(x['ratio']>=4.5 for x in dead_ratios),dead_ratios;check('zero HP and disabled controls remain readable',values=dead_ratios)
        # Platform result overlay remains closed across normal snapshots.
        buttons=js('[...document.querySelectorAll("button")].map(e=>({text:e.textContent,aria:e.getAttribute("aria-label")}))');record('result-buttons',buttons)
        close=js('(()=>{const b=[...document.querySelectorAll("button")].find(e=>/关闭/.test(e.getAttribute("aria-label")||"")&&e.closest("[role=dialog]"));if(!b)return false;b.click();return true})()')
        assert close,'result close control missing';before_box=rect('[data-fortressblue-canvas]');time.sleep(1.2);assert not js('!!document.querySelector("[role=dialog][aria-modal=true]")');after_box=rect('[data-fortressblue-canvas]');assert abs(before_box['y']-after_box['y'])<1;check('closed settlement stays closed and board stable',before=before_box,after=after_box)
    if args.only in ('all','effects'):
        for kind in ['heal','fire','poison']:effects(kind)
    if args.only in ('all','effects','terrain'):extinguish()
    if args.only in ('all','cancel'):
        cancelled_windup()
        cancelled_windup(nonfatal=True)
    if args.only in ('all','reveal'):
        for width,height in [(1280,900),(320,844)]:
            browser('set','viewport',str(width),str(height));start();time.sleep(.4)
            c=rect('[data-fortressblue-canvas]');f=rect('[data-fortressblue-fire]');m=measure()
            assert c['y']>=-1 and c['y']+c['h']<=height+1 and f['y']>=-1 and f['y']+f['h']<=height+1,{'canvas':c,'fire':f,'viewport':[width,height]}
            record('initial-reveal-'+str(width),m);shot('initial-reveal-'+str(width));check('initial board and fire simultaneously visible '+str(width),canvas=c,fire=f,scroll=m['scroll'])
    if args.only in ('all','bots'):
        start(bots=1);command('autoplay',enabled=True);wait(lambda:state()['seats'][0].get('autoPlay') is True)
        auto=native([{'sample':True},{'wait':20000,'read':'auto-play'}],'all-computer-timeline')
        auto_views=[f['publicState']['room']['game']['data'] for f in auto['frames'] if f.get('publicState',{}).get('room',{}).get('game')]
        ids={v['lastShot']['id'] for v in auto_views if v.get('lastShot')};assert len(ids)>=2,ids
        for f in auto['frames']:
            if f.get('publicState',{}).get('room',{}).get('game'):
                game=f['publicState']['room']['game']
                if game['data'].get('resolution'): assert not game.get('actors')
        before_take=view();command('autoplay',enabled=False);after_take=view();assert not state()['seats'][0].get('autoPlay');assert before_take['epoch']==after_take['epoch'];check('mixed bot match, all computer shots and manual takeover',shotIds=sorted(ids),room=state()['id'])
        start(bots=7);browser('set','viewport','320','844');layout('eight-seats-320')
    errors=browser('errors').get('errors',[]);assert not errors,errors;report['javascriptErrors']=errors
    report['passed']=True
except Exception:
    report['error']=traceback.format_exc();print(report['error'],flush=True)
    if opened:
        try:shot('failure',True)
        except Exception:pass
finally:
    if recording:
        try:browser('record','stop')
        except Exception:pass
    if opened:
        try:browser('close')
        except Exception:pass
    (out/'stop').write_text('stop')
    if server:
        if args.only in ('facing','portraits','supplies','projectiles','slope'):server.terminate()
        try:server.wait(timeout=15)
        except subprocess.TimeoutExpired:server.terminate();server.wait(timeout=10)
    if log:log.close()
    shutil.rmtree(temp,ignore_errors=True);shutil.rmtree(sockets,ignore_errors=True)
    report['cleanup']=['temporary profile, SQLite DB and compiler temporary files','owned short browser socket directory']
    record('report',report)
    print(str(out/'report.json'),flush=True)
raise SystemExit(0 if report['passed'] else 1)
