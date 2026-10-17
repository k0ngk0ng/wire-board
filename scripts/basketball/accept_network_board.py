"""Real React Board, authenticated HTTP, browser Wasm and native keyboard/touch."""
from pathlib import Path
import json,os,subprocess,uuid
ROOT=Path(__file__).resolve().parents[2];OUT=ROOT/'.local/basketball/network-board';OUT.mkdir(parents=True,exist_ok=True)
env=dict(os.environ,AGENT_BROWSER_SESSION='bkb-'+uuid.uuid4().hex[:6],AGENT_BROWSER_SOCKET_DIR=str(ROOT/'.local/bkb-sockets'),TMPDIR=str(ROOT/'.local/tmp'),NODE_PATH=str(ROOT/'web/node_modules'))
Path(env['AGENT_BROWSER_SOCKET_DIR']).mkdir(exist_ok=True)
base=env['BASKETBALL_HTTP_URL'];checks=[];opened=False;inspector=None
def browser(*args):
 r=subprocess.run(['agent-browser',*args,'--json'],env=env,capture_output=True,text=True,timeout=60)
 if r.returncode:raise RuntimeError(r.stdout+r.stderr)
 value=json.loads(r.stdout)
 if not value['success']:raise RuntimeError(value)
 return value['data']
def cdp(method,params={}):
 inspector.stdin.write(json.dumps(dict(method=method,params=params))+'\n');inspector.stdin.flush();r=json.loads(inspector.stdout.readline())
 if 'error'in r:raise RuntimeError(r['error'])
 return r['result']
def js(source):
 r=cdp('Runtime.evaluate',dict(expression=source,returnByValue=True,awaitPromise=True,timeout=4000))
 if r.get('exceptionDetails'):raise RuntimeError(r['exceptionDetails'])
 return r['result'].get('value')
def wait(expression):
 return js('(async()=>{for(let n=0;n<100;n++){if('+expression+')return true;await new Promise(r=>setTimeout(r,20));}throw Error('+json.dumps('Timed out: '+expression)+')})()')
def key(kind,code,key,number):cdp('Input.dispatchKeyEvent',dict(type=kind,code=code,key=key,windowsVirtualKeyCode=number))
def expect(name,value):
 assert value,name
 checks.append(name)
try:
 (OUT/'result.json').write_text('{"status":"running"}\n')
 (OUT/'index.html').write_text('<!doctype html><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Basketball network Board</title><style>body{margin:0;background:#172126;color:#f4eedb;font-family:sans-serif}main{max-width:900px;margin:auto;padding:12px}h1{font-size:18px;margin:0 0 8px}button{font:inherit}.modal-backdrop{position:fixed;inset:0;background:#0008;display:grid;place-items:center;z-index:50}.modal{max-height:90svh;overflow:auto;background:#f5f0df;color:#222;padding:16px;max-width:600px;width:calc(100% - 48px)}</style><link rel="stylesheet" href="/board.css"><div id="root"></div><script type="module" src="/board.js"></script>')
 subprocess.run([str(ROOT/'web/node_modules/.bin/esbuild'),str(ROOT/'scripts/basketball/network_board_harness.tsx'),'--bundle','--format=esm','--jsx=automatic','--loader:.svg=dataurl','--target=es2022','--outfile='+str(OUT/'board.js')],cwd=ROOT,env=env,check=True)
 subprocess.run(['go','build','-o',str(OUT/'basketball.wasm'),'./games/basketball/cmd/browser-wasm'],cwd=ROOT,env=dict(env,GOOS='js',GOARCH='wasm'),check=True)
 goroot=subprocess.check_output(['go','env','GOROOT'],cwd=ROOT,env=env,text=True).strip();(OUT/'wasm_exec.js').write_bytes((Path(goroot)/'lib/wasm/wasm_exec.js').read_bytes())
 page=base+'/'
 browser('--profile',str(OUT/'profile'),'open',page);opened=True
 endpoint=browser('get','cdp-url')['cdpUrl'];inspector=subprocess.Popen(['node',str(ROOT/'scripts/basketball/browser_cdp_session.mjs'),endpoint,page],env=env,stdin=subprocess.PIPE,stdout=subprocess.PIPE,stderr=(OUT/'cdp.stderr').open('w'),text=True);assert json.loads(inspector.stdout.readline())['ready']
 wait('document.querySelector("[data-basketball-ready=true]")')
 browser('click','#room-options-preview summary')
 expect('solo creation defaults to original computer teammates',js('JSON.parse(document.querySelector("#mode-config").textContent).mode==="solo" && document.querySelector("#room-options-preview").textContent.includes("电脑")'))
 browser('select','[aria-label="座位"]','2');wait('document.querySelector("[aria-label=比赛模式]")?.value==="cooperative"')
 browser('select','[aria-label="比赛模式"]','versus');wait('JSON.parse(document.querySelector("#mode-config").textContent).mode==="versus"')
 expect('two-person creation offers cooperation and versus',js('document.querySelectorAll("[aria-label=比赛模式] option").length===2'))
 browser('set','viewport','320','720');browser('screenshot',str(OUT/'creation-mobile-320.png'))
 expect('creation options fit 320px',js('document.documentElement.scrollWidth<=innerWidth'))
 browser('select','[aria-label="座位"]','1');wait('JSON.parse(document.querySelector("#mode-config").textContent).mode==="solo"')
 expect('changing to solo clears incompatible versus mode',js('!document.querySelector("[aria-label=比赛模式]")'))
 browser('click','#room-options-preview summary')
 browser('set','viewport','1100','820')
 expect('live Board restores current music through the platform port',js('networkBoard.music.some(m=>m.options.startOffset>0 && m.options.playbackID.includes(networkBoard.room.game.data.music.assetSHA256))'))
 js('document.querySelector(".basketball-board").focus()')
 key('keyDown','KeyD','d',68);wait('(networkBoard.room.game.data.inputs[0].buttons&1)!==0')
 expect('autoplay refusal retries only the current music on a native gesture',js('networkBoard.denyFirstMusic && networkBoard.music.length>=2 && networkBoard.music[1].options.startOffset>=networkBoard.music[0].options.startOffset'))
 expect('native D reaches the authenticated Go seat',js('networkBoard.room.you===0 && networkBoard.room.game.data.inputs[0].buttons===1'))
 key('keyUp','KeyD','d',68);wait('networkBoard.room.game.data.inputs[0].buttons===0')
 key('keyDown','Space',' ',32);key('keyUp','Space',' ',32);wait('networkBoard.room.game.data.phase==="playing"')
 before=js('networkBoard.room.game.data.runtime.playing.world.Preparation.Actions.actors[0].player.body.position.x')
 key('keyDown','KeyD','d',68);wait('networkBoard.room.game.data.runtime.playing.world.Preparation.Actions.actors[0].player.body.position.x>'+str(before));key('keyUp','KeyD','d',68)
 expect('native input moves the server-authoritative original player',True)
 wait('networkBoard.room.game.data.inputs[0].buttons===0')
 key('keyDown','Space',' ',32);key('keyUp','Space',' ',32);wait('networkBoard.room.game.data.phase==="paused"')
 expect('original START pauses without a separate game menu',True)
 expect('play controls show original names without keyboard mappings',js('document.querySelectorAll(".fc-play kbd").length===0 && !!document.querySelector(".fc-play")'))
 browser('set','viewport','1100','820')
 placement=js('(()=>{const screen=document.querySelector(".basketball-screen"),pad=document.querySelector(".fc-play");return {outside:!screen?.contains(pad),screen:screen?.getBoundingClientRect().toJSON(),pad:pad.getBoundingClientRect().toJSON()}})()')
 expect('desktop controller sits below and outside the game screen',placement['outside'] and placement['screen'] and placement['screen']['bottom']<=placement['pad']['top'])
 browser('screenshot',str(OUT/'desktop.png'))
 js('networkBoard.ackDelay=250');key('keyDown','KeyJ','j',74);key('keyUp','KeyJ','j',74)
 wait('networkBoard.actions.some(a=>a.events.some(e=>e.buttons===64))');wait('networkBoard.room.game.data.inputs[0].buttons===0')
 expect('short native tap survives delayed HTTP acknowledgement',True);js('networkBoard.ackDelay=0')
 browser('click','#settings');wait('!!document.querySelector("[role=dialog]")');wait('networkBoard.room.game.data.inputs[0].buttons===0')
 key('keyDown','KeyD','d',68);key('keyUp','KeyD','d',68)
 expect('settings dialog blocks game input',js('new Promise(r=>setTimeout(()=>r(networkBoard.room.game.data.inputs[0].buttons===0),150))'))
 browser('click','[aria-label="修改 右 按键"]');key('keyDown','ArrowRight','ArrowRight',39);key('keyUp','ArrowRight','ArrowRight',39);browser('click','[data-controller-save]')
 wait('document.querySelector(".controller-notice")?.textContent.includes("已保存")')
 browser('press','Escape');wait('!document.querySelector("[role=dialog]")')
 js('document.querySelector(".basketball-board").focus()');wait('networkBoard.actions.at(-1)?.sequence===networkBoard.room.game.data.inputs[0].sequence')
 count=js('networkBoard.actions.length');key('keyDown','KeyD','d',68);key('keyUp','KeyD','d',68)
 expect('old mapping stops immediately after saving personal settings',js('new Promise(r=>setTimeout(()=>r(networkBoard.actions.length==='+str(count)+'),120))'))
 key('keyDown','ArrowRight','ArrowRight',39);wait('networkBoard.room.game.data.inputs[0].buttons===1');key('keyUp','ArrowRight','ArrowRight',39);wait('networkBoard.room.game.data.inputs[0].buttons===0')
 expect('saved personal mapping controls the live room without reload',True)
 browser('set','viewport','320','720');cdp('Emulation.setTouchEmulationEnabled',dict(enabled=True,maxTouchPoints=2));js('document.querySelector(".basketball-board").focus()')
 point=js('(()=>{const r=document.querySelector("[data-controller-button=a]").getBoundingClientRect();return{x:r.x+r.width/2,y:r.y+r.height/2}})()')
 key('keyDown','ArrowRight','ArrowRight',39);cdp('Input.dispatchTouchEvent',dict(type='touchStart',touchPoints=[dict(id=1,**point)]));wait('networkBoard.room.game.data.inputs[0].buttons===129')
 cdp('Input.dispatchTouchEvent',dict(type='touchCancel',touchPoints=[]));wait('networkBoard.room.game.data.inputs[0].buttons===1');key('keyUp','ArrowRight','ArrowRight',39);wait('networkBoard.room.game.data.inputs[0].buttons===0')
 expect('native touch cancellation preserves an independently held keyboard direction',True)
 layout=js('(()=>{const b=document.querySelector(".basketball-board").getBoundingClientRect();return{right:b.right,bottom:b.bottom,overflow:document.documentElement.scrollWidth>innerWidth}})()')
 expect('320px board and FC controls fit one viewport',not layout['overflow'] and layout['right']<=320 and layout['bottom']<=720)
 placement=js('(()=>{const screen=document.querySelector(".basketball-screen"),pad=document.querySelector(".fc-play");return {outside:!screen.contains(pad),screen:screen.getBoundingClientRect().toJSON(),pad:pad.getBoundingClientRect().toJSON()}})()')
 expect('mobile controller sits below and outside the game screen',placement['outside'] and placement['screen']['bottom']<=placement['pad']['top'])
 expect('game controller uses Turbo labels without keyboard mappings',js('document.querySelector("[data-controller-button=turboB]").getAttribute("aria-label")==="Turbo B" && document.querySelector("[data-controller-button=turboA]").getAttribute("aria-label")==="Turbo A" && document.querySelectorAll(".fc-play kbd").length===0'))
 browser('screenshot',str(OUT/'mobile-320.png'))
 js('switchNetworkUser("牌局玩家1")');wait('networkBoard.room.you===1 && !!document.querySelector("[data-basketball-ready=true]")')
 js('document.querySelector(".basketball-board").focus()');key('keyDown','KeyD','d',68);wait('networkBoard.room.game.data.inputs[2].buttons===1');key('keyUp','KeyD','d',68);wait('networkBoard.room.game.data.inputs[2].buttons===0')
 expect('versus second seat controls actor2 with independent profile and authenticated seat1',js('networkBoard.room.game.data.inputs[0].buttons===0 && networkBoard.room.game.data.inputs[1].automated && networkBoard.room.game.data.localActor===2 && networkBoard.actions.at(-1).seat===1'))
 js('switchNetworkUser("牌局玩家2")');wait('networkBoard.room.you===-1 && !!document.querySelector("[data-basketball-ready=true]")')
 expect('spectator has no playable controller',js('!document.querySelector(".fc-play")'))
 count=js('networkBoard.actions.length');key('keyDown','KeyD','d',68);key('keyUp','KeyD','d',68)
 expect('switching identity to spectator stops keyboard submissions',js('new Promise(r=>setTimeout(()=>r(networkBoard.actions.length==='+str(count)+'),180))'))
 expect('no board or host API errors',js('networkBoard.errors.length===0 && !document.querySelector(".basketball-error")'))
 browser('screenshot',str(OUT/'spectator-mobile.png'))
 record=dict(status='passed',checks=checks,layout=layout,nativeInput=True,scope='actual React Board, authenticated HTTP, Go/Wasm prediction, native keyboard/touch; not complete multiplayer match')
 (OUT/'result.json').write_text(json.dumps(record,ensure_ascii=False,indent=2)+'\n');print(json.dumps(record,ensure_ascii=False))
except Exception as error:
 (OUT/'result.json').write_text(json.dumps(dict(status='failed',checks=checks,error=repr(error)),ensure_ascii=False,indent=2)+'\n')
 if opened:
  if inspector:
   try:(OUT/'failure-state.json').write_text(json.dumps(js('({errors:networkBoard.errors,body:document.body.innerText,settings:document.querySelector("#settings")?.getBoundingClientRect().toJSON(),dialogs:document.querySelectorAll("[role=dialog]").length})'),ensure_ascii=False,indent=2)+'\n')
   except Exception:pass
  try:browser('screenshot',str(OUT/'failure.png'))
  except Exception:pass
 raise
finally:
 if inspector:
  inspector.stdin.close()
  try:inspector.wait(timeout=5)
  except subprocess.TimeoutExpired:inspector.terminate();inspector.wait(timeout=5)
 if opened:browser('close')
