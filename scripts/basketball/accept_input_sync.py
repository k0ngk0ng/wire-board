"""Real React Board, authenticated HTTP, browser Wasm and native keyboard/touch."""
from pathlib import Path
import json,os,subprocess,uuid
ROOT=Path(__file__).resolve().parents[2];OUT=Path(os.environ.get('BASKETBALL_BROWSER_OUTPUT',str(ROOT/'.local/basketball/network-board'))).resolve();assert OUT.is_relative_to(ROOT/'.local');OUT.mkdir(parents=True,exist_ok=True)
env=dict(os.environ,AGENT_BROWSER_SESSION='bkb-'+uuid.uuid4().hex[:6],AGENT_BROWSER_SOCKET_DIR=os.environ.get('AGENT_BROWSER_SOCKET_DIR',str(ROOT/'.local/bkn-sockets')),TMPDIR=str(ROOT/'.local/tmp'),NODE_PATH=str(ROOT/'web/node_modules'))
Path(env['AGENT_BROWSER_SOCKET_DIR']).mkdir(exist_ok=True)
base=env['BASKETBALL_HTTP_URL'];checks=[];opened=False;inspector=None
media_mode=env.get('BASKETBALL_MEDIA_E2E')=='1';media_evidence={}
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
 r=cdp('Runtime.evaluate',dict(expression=source,returnByValue=True,awaitPromise=True,timeout=12000))
 if r.get('exceptionDetails'):raise RuntimeError(r['exceptionDetails'])
 return r['result'].get('value')
def wait(expression):
 return js('(async()=>{for(let n=0;n<500;n++){if(await ('+expression+'))return true;await new Promise(r=>setTimeout(r,20));}throw Error('+json.dumps('Timed out: '+expression)+')})()')
def key(kind,code,key,number):cdp('Input.dispatchKeyEvent',dict(type=kind,code=code,key=key,windowsVirtualKeyCode=number))
def expect(name,value):
 assert value,name
 checks.append(name)
def native_media():
 return js('(()=>{const m=document.querySelector("audio[data-platform-music]");return m?{src:m.getAttribute("src"),currentSrc:m.currentSrc,paused:m.paused,currentTime:m.currentTime,duration:m.duration,readyState:m.readyState,networkState:m.networkState,error:m.error?.code??null,events:networkBoard.mediaEvents.length,sameElement:m===window.musicElementBeforeFault}:null})()')
def media_resets_since(index):
 return js('networkBoard.mediaEvents.slice('+str(index)+').filter(e=>["loadstart","emptied","abort"].includes(e.type))')
try:
 (OUT/'result.json').write_text('{"status":"running"}\n')
 (OUT/'index.html').write_text('<!doctype html><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Basketball network Board</title><style>body{margin:0;background:#172126;color:#f4eedb;font-family:sans-serif}main{max-width:900px;margin:auto;padding:12px}h1{font-size:18px;margin:0 0 8px}button{font:inherit}.modal-backdrop{position:fixed;inset:0;background:#0008;display:grid;place-items:center;z-index:50}.modal{max-height:90svh;overflow:auto;background:#f5f0df;color:#222;padding:16px;max-width:600px;width:calc(100% - 48px)}</style><link rel="stylesheet" href="/board.css"><div id="root"></div><script type="module" src="/board.js"></script>')
 subprocess.run([str(ROOT/'web/node_modules/.bin/esbuild'),str(ROOT/'scripts/basketball/network_board_harness.tsx'),'--bundle','--format=esm','--jsx=automatic','--loader:.svg=dataurl','--target=es2022','--outfile='+str(OUT/'board.js')],cwd=ROOT,env=env,check=True)

 page=base+'/?stream=1'+('&audio=media' if media_mode else '')
 browser('--profile',str(OUT/'profile'),'open',page);opened=True
 endpoint=browser('get','cdp-url')['cdpUrl'];inspector=subprocess.Popen(['node',str(ROOT/'scripts/basketball/browser_cdp_session.mjs'),endpoint,page],env=env,stdin=subprocess.PIPE,stdout=subprocess.PIPE,stderr=(OUT/'cdp.stderr').open('w'),text=True);assert json.loads(inspector.stdout.readline())['ready']
 mirror=(ROOT/'scripts/basketball/browser_oss_mirror.js').read_text();cdp('Page.enable');cdp('Page.addScriptToEvaluateOnNewDocument',dict(source=mirror))
 # Pre-implementation regression: fault only the test page's native bridge
 # response, preserving all normal Go/Wasm behavior until explicitly enabled.
 cdp('Page.addScriptToEvaluateOnNewDocument',dict(source="""
 (() => {
   let invoke;
   Object.defineProperty(window, 'basketballWasm', {
     configurable: true,
     get() { return invoke; },
     set(native) {
       invoke = request => {
         const command = JSON.parse(request);
         if (command.op === 'advance') window.basketballAdvanceCalls = (window.basketballAdvanceCalls || 0) + 1;
         if (window.injectBasketballFault && command.op === 'advance') {
           window.basketballInjectedFaults = (window.basketballInjectedFaults || 0) + 1;
           return JSON.stringify({ok:false,error:{code:typeof window.injectBasketballFault === 'string' ? window.injectBasketballFault : 'rule_unavailable',detail:'acceptance native action failure'}});
         }
         return native(request);
       };
     }
   });
 })();
 """));browser('reload')
 wait('document.querySelector("[data-basketball-ready=true]") && networkBoard.packets.some(p=>p.type==="game")')
 cpu=float(env.get('BASKETBALL_BROWSER_CPU_RATE','1'));assert 1<=cpu<=8
 cdp('Emulation.setCPUThrottlingRate',dict(rate=cpu))
 js('document.querySelector(".basketball-board").focus()')
 key('keyDown','Space',' ',32);key('keyUp','Space',' ',32);wait('networkBoard.room.game.data.phase==="playing"')
 # Real-media acceptance is declared before adding the fixture's optional
 # production GameAudio port. No replacement Audio or successful play stub is
 # permitted: decode a verified MP3, observe native media events and identity.
 if media_mode:
  wait('(()=>{const m=document.querySelector("audio[data-platform-music]");return m && !m.paused && m.readyState>=2 && m.currentSrc.includes("/__oss/") && m.currentSrc.includes("/audio/music-")})()')
  js('window.musicElementBeforeFault=document.querySelector("audio[data-platform-music]")')
  media_evidence['initial']=native_media()
  expect('production HTMLAudioElement decodes and plays the mirrored immutable MP3',media_evidence['initial']['duration']>0 and media_evidence['initial']['error'] is None)
  wait('document.querySelector("audio[data-platform-music]").currentTime>'+str(media_evidence['initial']['currentTime']+.05))
 if env.get('BASKETBALL_INCIDENT_E2E')=='1':
  # This saved game starts mid-jump/aim, where the original rules lock
  # horizontal motion. Measure walking only after that action ends.
  wait('Number(document.querySelector(".basketball-stage canvas")?.dataset.playerAction)===0')
 expect('actual ordered WebSocket sends controller inputs',js('networkBoard.sent.length>0 && networkBoard.packets.some(p=>p.type==="ack")'))
 js("""window.paintSamples=[];const originalPaint=CanvasRenderingContext2D.prototype.putImageData;
 CanvasRenderingContext2D.prototype.putImageData=function(...args){const result=originalPaint.apply(this,args);if(window.measure&&this.canvas.closest('.basketball-stage'))paintSamples.push({at:performance.now(),frame:Number(this.canvas.dataset.videoFrame),action:Number(this.canvas.dataset.playerAction),ballAction:Number(this.canvas.dataset.ballAction),ballX:Number(this.canvas.dataset.ballWorldX),x:Number(this.canvas.dataset.playerWorldX),screenX:Number(this.canvas.dataset.playerX),cameraX:Number(this.canvas.dataset.cameraX)});return result};
 window.frameTimes=[];window.motionProbe=null;window.measure=false;window.motionResult=null;
 window.addEventListener('keydown',e=>{if(e.code==='KeyD'&&!e.repeat){const c=document.querySelector('.basketball-stage canvas');motionProbe={at:performance.now(),x:Number(c.dataset.playerWorldX)}}});
 const sample=now=>{if(window.measure)frameTimes.push(now);const c=document.querySelector('.basketball-stage canvas');if(motionProbe&&!motionResult&&c?.dataset.playerVisible==='true'&&Number(c.dataset.playerWorldX)!==motionProbe.x)motionResult={elapsed:now-motionProbe.at,authorityButtons:networkBoard.room.game.data.inputs[0].appliedButtons,x:Number(c.dataset.playerWorldX)};requestAnimationFrame(sample)};requestAnimationFrame(sample);""")
 start=js('performance.now()');initial=js('networkBoard.checkpoints.length')
 js('measure=true');key('keyDown','KeyD','d',68);wait('motionResult!==null');motion=js('motionResult')
 js('new Promise(r=>setTimeout(r,1500))');key('keyUp','KeyD','d',68)
 key('keyDown','KeyA','a',65);js('new Promise(r=>setTimeout(r,1000))');key('keyUp','KeyA','a',65)
 js('new Promise(r=>setTimeout(r,2500))');end=js('performance.now()');js('measure=false')
 packets=js('networkBoard.packets');times=js('frameTimes');paints=js('paintSamples');current=js('networkBoard.checkpoints.length')
 traffic=[p for p in packets if p['type']=='game' and start<=p['at']<=end]
 intervals=sorted(b-a for a,b in zip(times,times[1:]));p95=intervals[int(len(intervals)*.95)]
 bytes_per_second=sum(p['bytes'] for p in traffic)/((end-start)/1000)
 corrections=[abs(b['x']-a['x'])/256 for a,b in zip(paints,paints[1:]) if a['frame']==b['frame'] and a['x']!=b['x']]
 expect('normal play never transfers full runtime',all(not p['full'] for p in packets))
 expect('normal play does not request repeated full checkpoints',current==initial)
 expect('local movement reacts within 150ms',motion['elapsed']<150)
 if int(env.get('BASKETBALL_BROWSER_RTT_MS','0'))>=300:expect('rendered local movement precedes authoritative input confirmation',motion['authorityButtons']==0)
 expect('actual game stream remains below 40KB/s',bytes_per_second<40000)
 expect('desktop animation frames have p95 below 35ms',p95<35)
 expect('network reconciliation has no instant walking jump over two source pixels',max(corrections,default=0)<=2)
 # Exercise held directions, normal actions, simultaneous A+B and turbo over
 # several replay windows; input release must not cause checksum recovery.
 combat_start=js('performance.now()');combat_frame=js('networkBoard.room.game.data.frame')
 combat_checkpoints=js('networkBoard.checkpoints.length');combat_sent=js('networkBoard.sent.length')
 for direction,letter,number in [('KeyD','d',68),('KeyA','a',65)]*2:
  key('keyDown',direction,letter,number)
  for buttons in [[('KeyJ','j',74)],[('KeyK','k',75)],[('KeyJ','j',74),('KeyK','k',75)],[('KeyU','u',85),('KeyI','i',73)]]:
   for code,label,virtual in buttons:key('keyDown',code,label,virtual)
   js('new Promise(r=>setTimeout(r,400))')
   for code,label,virtual in reversed(buttons):key('keyUp',code,label,virtual)
   js('new Promise(r=>setTimeout(r,150))')
  key('keyUp',direction,letter,number)
 js('new Promise(r=>setTimeout(r,1000))')
 combat_end=js('performance.now()');combat_packets=js('networkBoard.packets').copy()
 combat_traffic=[p for p in combat_packets if p['type']=='game' and combat_start<=p['at']<=combat_end]
 combat_bps=sum(p['bytes'] for p in combat_traffic)/((combat_end-combat_start)/1000)
 expect('continuous action and turbo inputs are acknowledged',js('networkBoard.sent.slice('+str(combat_sent)+').length>20 && networkBoard.sent.slice('+str(combat_sent)+').every(p=>p.ackAt!==undefined)'))
 expect('continuous combat advances authority without full recovery',js('networkBoard.room.game.data.frame>'+str(combat_frame+200)+' && networkBoard.checkpoints.length==='+str(combat_checkpoints)))
 expect('combat stream remains compact',combat_bps<40000 and all(not p['full'] for p in combat_traffic))
 expect('combat leaves no client error or stuck input',js('networkBoard.errors.length===0 && networkBoard.room.game.data.inputs[0].buttons===0 && !document.querySelector(".basketball-error")'))
 browser('screenshot',str(OUT/'playing-desktop.png'))
 browser('set','viewport','320','720');cdp('Emulation.setTouchEmulationEnabled',dict(enabled=True,maxTouchPoints=2))
 point=js('(()=>{const r=document.querySelector("[data-controller-button=right]").getBoundingClientRect();return{x:r.x+r.width/2,y:r.y+r.height/2}})()')
 cdp('Input.dispatchTouchEvent',dict(type='touchStart',touchPoints=[dict(id=1,**point)]));wait('networkBoard.room.game.data.inputs[0].buttons===1')
 cdp('Input.dispatchTouchEvent',dict(type='touchEnd',touchPoints=[]));wait('networkBoard.room.game.data.inputs[0].buttons===0')
 expect('native mobile touch sends and releases input',True)
 expect('320px board remains visible without horizontal overflow',js('document.documentElement.scrollWidth<=innerWidth'))
 browser('screenshot',str(OUT/'playing-mobile.png'))
 # A short prediction-only fault can be overtaken by valid authoritative
 # frames still inside the replay window. It must recover without a checkpoint
 # or restarting the current music. The longer frozen case below separately
 # verifies that a permanently stalled authority cannot cause request storms.
 auto_failure_before=js('({checkpoints:networkBoard.checkpoints.length,music:networkBoard.music.length,frame:Number(document.querySelector(".basketball-stage canvas").dataset.videoFrame)})')
 if media_mode:
  wait('!document.querySelector("audio[data-platform-music]").paused')
  media_evidence['beforeAutomaticRecovery']=native_media()
 js('freezeBasketballAuthority=true; injectBasketballFault=true')
 wait('document.querySelector(".basketball-error")?.textContent.includes("游戏运行")')
 js('injectBasketballFault=false; freezeBasketballAuthority=false')
 wait('!document.querySelector(".basketball-error") && Number(document.querySelector(".basketball-stage canvas").dataset.videoFrame)>'+str(auto_failure_before['frame']))
 expect('advancing authority clears a transient fault without a checkpoint or music reload',js('networkBoard.checkpoints.length==='+str(auto_failure_before['checkpoints'])+' && networkBoard.music.length==='+str(auto_failure_before['music'])))
 if media_mode:
  wait('!document.querySelector("audio[data-platform-music]").paused')
  media_evidence['afterAutomaticRecovery']=native_media()
  before=media_evidence['beforeAutomaticRecovery'];after=media_evidence['afterAutomaticRecovery']
  expect('authority recovery resumes the same native MP3 without loadstart emptied or abort',after['sameElement'] and after['src']==before['src'] and after['currentSrc']==before['currentSrc'] and not media_resets_since(before['events']))
 js('document.querySelector(".basketball-board").focus()')
 key('keyDown','KeyD','d',68);wait('networkBoard.room.game.data.inputs[0].buttons===1');key('keyUp','KeyD','d',68);wait('networkBoard.room.game.data.inputs[0].buttons===0')
 expect('automatic fault recovery restores real controller input',True)
 # Hold the exact authority while the original runtime reports an unavailable
 # action. The production state stream still exists; this recreates the stalled
 # server boundary without replacing any game physics or modifying the server.
 if media_mode:media_evidence['beforeFrozenFailure']=native_media()
 js('freezeBasketballAuthority=true; injectBasketballFault=true')
 wait('window.basketballInjectedFaults>0');wait('document.querySelector(".basketball-error")?.textContent.includes("游戏运行")')
 failure_counts=js('({checkpoints:networkBoard.checkpoints.length,music:networkBoard.music.length,frame:document.querySelector(".basketball-stage canvas").dataset.videoFrame})')
 js('new Promise(r=>setTimeout(r,1800))')
 expect('deterministic engine failure never floods checkpoints or music',js('networkBoard.checkpoints.length=== '+str(failure_counts['checkpoints'])+' && networkBoard.music.length=== '+str(failure_counts['music'])))
 expect('engine failure preserves visible picture and is not mislabeled disconnected',js('document.querySelector(".basketball-stage canvas").dataset.videoFrame==='+json.dumps(failure_counts['frame'])+' && document.querySelector(".basketball-status").textContent.includes("运行异常")'))
 expect('engine failure disables the visible controller',js('[...document.querySelectorAll(".fc-play button")].every(b=>b.disabled)'))
 if media_mode:
  media_evidence['afterFrozenFailure']=native_media()
  before=media_evidence['beforeFrozenFailure'];after=media_evidence['afterFrozenFailure']
  expect('1.8-second engine fault pauses one stable native MP3 without repeated media resets',after['sameElement'] and after['src']==before['src'] and after['currentSrc']==before['currentSrc'] and after['paused'] and not media_resets_since(before['events']))
  media_evidence['eventsBeforeManualRetry']=js('networkBoard.mediaEvents')
 browser('screenshot',str(OUT/'engine-failure-mobile.png'))
 # Keep the test's dropped-frame gate closed until explicit retry completes;
 # otherwise an arriving history-gap update can legitimately start automatic
 # recovery concurrently with the click and make its checkpoint count race.
 js('injectBasketballFault=false');browser('click','.basketball-error button')
 wait('document.querySelector("[data-basketball-ready=true]") && !document.querySelector(".basketball-error")')
 js('freezeBasketballAuthority=false')
 expect('explicit retry takes one checkpoint and restores control',js('networkBoard.checkpoints.length==='+str(failure_counts['checkpoints']+1)))
 before=js('networkBoard.checkpoints.length');js('disconnectInputStream()')
 wait('networkBoard.checkpoints.length>='+str(before+1)+' && document.querySelector("[data-basketball-ready=true]")')
 js('new Promise(r=>setTimeout(r,1000))');after=js('networkBoard.checkpoints.length')
 expect('reconnection fetches one explicit checkpoint',after==before+1)
 expect('reconnection leaves the match running',js('networkBoard.room.game.data.phase==="playing" && !document.querySelector(".basketball-error")'))
 js('freezeBasketballAuthority=true; injectBasketballFault="runtime_stopped"')
 wait('document.querySelector(".basketball-error button")?.textContent==="刷新页面"')
 stopped_room=js('networkBoard.room.id')
 browser('click','.basketball-error button')
 wait('document.querySelector("[data-basketball-ready=true]") && networkBoard.packets.some(p=>p.type==="game")')
 expect('stopped runtime action reloads the page and rejoins the same match',js('networkBoard.room.id==='+json.dumps(stopped_room)+' && !window.injectBasketballFault && !document.querySelector(".basketball-error")'))
 record=dict(status='passed',checks=checks,metrics=dict(cpuThrottle=cpu,rttMS=int(env.get('BASKETBALL_BROWSER_RTT_MS','0')),sameFrameCorrectionCount=len(corrections),maxSameFrameCorrectionPixels=max(corrections,default=0),gameBytesPerSecond=bytes_per_second,combatBytesPerSecond=combat_bps,combatDurationMS=combat_end-combat_start,packetCount=len(traffic),frameP95MS=p95,localMoveMS=motion['elapsed'],initialCheckpoints=initial,steadyCheckpoints=current),motion=motion,packets=combat_packets,frameTimes=times,paints=paints,media=media_evidence,scope='real Board and native input with production StreamInputs/stream merge and actual Go WS server; '+('production GameAudio and real HTMLAudioElement using verified OSS bytes over loopback' if media_mode else 'fixture outer page/audio sink')+'; raw websocket message bytes before compression')
 (OUT/'result.json').write_text(json.dumps(record,ensure_ascii=False,indent=2)+'\n');print(json.dumps(dict(status='passed',checks=checks,metrics=record['metrics']),ensure_ascii=False))
except Exception as error:
 (OUT/'result.json').write_text(json.dumps(dict(status='failed',checks=checks,media=media_evidence,error=repr(error)),ensure_ascii=False,indent=2)+'\n')
 if opened and inspector:
  try:
   (OUT/'failure-state.json').write_text(json.dumps(js('({advanceCalls:window.basketballAdvanceCalls,injectedFaults:window.basketballInjectedFaults,body:document.body.innerText,errors:networkBoard.errors,packets:networkBoard.packets,checkpoints:networkBoard.checkpoints,canvas:document.querySelector(".basketball-stage canvas")?.dataset,room:networkBoard.room})'),ensure_ascii=False,indent=2)+'\n')
   browser('screenshot',str(OUT/'failure.png'))
  except Exception:pass
 raise
finally:
 if inspector:
  inspector.stdin.close()
  try:inspector.wait(timeout=5)
  except subprocess.TimeoutExpired:inspector.terminate();inspector.wait(timeout=5)
 if opened:browser('close')
