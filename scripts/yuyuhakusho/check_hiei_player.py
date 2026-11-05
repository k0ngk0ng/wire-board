"""Hiei browser E2E specified before Hiei selection UI implementation.

Uses the actual independent Go/Wasm rules, decoded assets, Canvas and shared
MD pad. Native CDP keyboard/touch events are marked isTrusted by Chromium.
"""
from pathlib import Path
import hashlib, json, os, subprocess, threading, time
from http.server import ThreadingHTTPServer
from serve_local_player import build, PlayerHandler, ROOT

OUT=ROOT/'games/yuyuhakusho/verification/hiei-local-player'
OUT.mkdir(parents=True,exist_ok=True)
TMP=ROOT/'.local/md-yuyuhakusho/hiei-browser'
TMP.mkdir(parents=True,exist_ok=True)
env=dict(os.environ,TMPDIR=str(TMP),AGENT_BROWSER_SOCKET_DIR=str(ROOT/'.local/yys'))
Path(env['AGENT_BROWSER_SOCKET_DIR']).mkdir(parents=True,exist_ok=True)
session=subprocess.run(['agent-browser','session','id','--scope','worktree','--prefix','yy-player'],cwd=ROOT,env=env,capture_output=True,text=True,check=True).stdout.strip()
# Keep the Unix socket inside this worktree and below macOS's path limit.
session='hi'+session.rsplit('-',1)[-1][:6]
env['AGENT_BROWSER_SESSION']=session
build()
server=ThreadingHTTPServer(('127.0.0.1',0),PlayerHandler)
thread=threading.Thread(target=server.serve_forever,daemon=True);thread.start()
url='http://127.0.0.1:'+str(server.server_port)+'/'
def browser(*args):
    result=subprocess.run(['agent-browser',*args,'--json'],cwd=ROOT,env=env,capture_output=True,text=True,timeout=55)
    if result.returncode: raise RuntimeError(result.stdout+result.stderr)
    data=json.loads(result.stdout)
    if not data.get('success'): raise RuntimeError(data)
    return data.get('data')
def js(expression): return browser('eval',expression)['result']
def native(actions):
    endpoint=browser('get','cdp-url');endpoint=endpoint.get('url')or endpoint.get('cdpUrl')or endpoint.get('value')
    result=subprocess.run(['node',str(ROOT/'scripts/yuyuhakusho/native_player_input.mjs'),endpoint,url,json.dumps(actions)],cwd=ROOT,env=env,capture_output=True,text=True,timeout=25)
    if result.returncode: raise RuntimeError(result.stdout+result.stderr)
    return json.loads(result.stdout)
def key(letter,down,expected=None):
    return {'method':'Input.dispatchKeyEvent','type':'keyDown' if down else 'keyUp','key':letter,'code':'Key'+letter.upper(),'windowsVirtualKeyCode':ord(letter.upper()),**({'expect':expected}if expected is not None else{})}
def rect(selector):
    return js('(()=>{const r=document.querySelector('+json.dumps(selector)+').getBoundingClientRect();return {x:r.x,y:r.y,w:r.width,h:r.height}})()')
def center(r):return {'x':r['x']+r['w']/2,'y':r['y']+r['h']/2}
opened=False
report={}
try:
    browser('--profile',str(TMP/'profile'),'open',url);opened=True
    browser('set','viewport','1120','900')
    browser('wait','--fn','window.yyResearch?.assetsReady === true')
    assert js('window.yyResearch.view?.tick ?? 0')==0
    assert js('document.querySelector("#start-match").disabled') is False
    assert js('Array.from(document.querySelector("#player-character").options).find(option => option.value === "8").textContent') == '户愚吕兄'
    assert js('Array.from(document.querySelector("#player-character").options).find(option => option.value === "10").textContent') == '户愚吕弟'
    assert js('Array.from(document.querySelector("#player-character").options).length') == 9
    browser('select','#player-character','4')
    browser('select','#opponent-character','9')
    browser('click','#start-match')
    browser('wait','--fn','window.yyResearch.view?.tick > 260')
    assert js('window.yyResearch.view.ready && !window.yyResearch.view.players[0].computer')
    assert js('window.yyResearch.view.window_complete')
    assert js('window.yyResearch.view.players[0].character_id === 4 && window.yyResearch.view.players[1].character_id === 9')
    before=js('({x:window.yyResearch.view.players[0].X,pixel:window.yyResearch.pixelSignature})')
    native([key('d',True,['right'])])
    browser('wait','--fn','window.yyResearch.view.players[0].X > '+str(before['x']+8))
    native([key('d',False,[])])
    after=js('({x:window.yyResearch.view.players[0].X,pixel:window.yyResearch.pixelSignature})')
    assert after['x']>before['x'] and after['pixel']!=before['pixel'],(before,after)
    report['native_keyboard_visible_movement']={'before':before,'after':after}
    native([key('d',True,['right']),key('k',True,['a','right'])])
    browser('wait','--fn','window.yyResearch.view.players[0].character_id === 4 && window.yyResearch.view.players[0].Action === 3 && window.yyResearch.view.players[0].Pose === 131')
    report['source4_forward_attack']=js('window.yyResearch.view.players[0]')
    native([key('k',False,['right']),key('d',False,[])])
    native([key('u',True,['x']),key('o',True,['x','z'])])
    sample=js('window.yyResearch.sample()');assert sample==['x','z']
    native([key('u',False,['z']),key('o',False,[])])
    native([key('j',True,['b']),{'method':'acceptance:hide','expect':[]},key('j',False,[])])
    report['native_visibility_release']=True
    assert js('!window.yyResearch.view.players[0].computer && window.yyResearch.view.players[0].input_channel === 0')
    browser('screenshot',str(OUT/'desktop.png'))
    browser('set','viewport','320','740')
    browser('scrollintoview','[data-controller-layout="md"]')
    assert not js('document.documentElement.scrollWidth > innerWidth')
    canvas=rect('canvas');pad=rect('[data-controller-layout="md"]')
    assert pad['y']>=canvas['y']+canvas['h'],(canvas,pad)
    left=center(rect('[data-controller-button="left"]'));z=center(rect('[data-controller-button="z"]'));c=center(rect('[data-controller-button="c"]'))
    actions=[{'type':'touchStart','touchPoints':[{'id':1,**left},{'id':2,**z},{'id':3,**c}],'expect':['c','left','z']},{'type':'touchCancel','touchPoints':[],'expect':[]}]
    report['native_mobile_touch']=native(actions)
    assert js('window.yyResearch.trusted.length>0 && window.yyResearch.trusted.every(Boolean)')
    assert js('window.yyResearch.errors.length')==0,js('window.yyResearch.errors')
    browser('screenshot',str(OUT/'mobile-320.png'))
    report.update(pass_=True,scope='Nine-character independent local Wasm research player; no platform room, network or full-roster acceptance',viewport=320,input_trusted=True,human_idle_no_takeover=True,final_view=js('window.yyResearch.view'),asset_requests=PlayerHandler.requested_paths)
    report['pass']=report.pop('pass_')
    assert not any(any(s in path for s in ('.bin','.rom','libretro'))for path in PlayerHandler.requested_paths)
    report['source_sha256']={str(p.relative_to(ROOT)):hashlib.sha256(p.read_bytes()).hexdigest()for p in [ROOT/'scripts/yuyuhakusho/local_player.tsx',ROOT/'games/yuyuhakusho/cmd/local-player-wasm/main_js.go',ROOT/'games/yuyuhakusho/runtimebridge/session.go']}
    (OUT/'result.json').write_text(json.dumps(report,ensure_ascii=False,indent=2)+'\n')
    print(json.dumps(report,ensure_ascii=False))
except Exception as error:
    (TMP/'failure.json').write_text(json.dumps({'error':repr(error),'completed':report},ensure_ascii=False,indent=2)+'\n')
    if opened:
        try: browser('screenshot',str(TMP/'failure.png'))
        except Exception: pass
    raise
finally:
    if opened: browser('close')
    server.shutdown();server.server_close();thread.join(timeout=5)
