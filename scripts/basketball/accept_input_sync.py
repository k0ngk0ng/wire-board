"""Real React Board, authenticated HTTP, browser Wasm and native keyboard/touch."""
from pathlib import Path
import json,os,subprocess,uuid
ROOT=Path(__file__).resolve().parents[2];OUT=Path(os.environ.get('BASKETBALL_BROWSER_OUTPUT',str(ROOT/'.local/basketball/network-board'))).resolve();assert OUT.is_relative_to(ROOT/'.local');OUT.mkdir(parents=True,exist_ok=True)
env=dict(os.environ,AGENT_BROWSER_SESSION='bkb-'+uuid.uuid4().hex[:6],AGENT_BROWSER_SOCKET_DIR=os.environ.get('AGENT_BROWSER_SOCKET_DIR',str(ROOT/'.local/bkn-sockets')),TMPDIR=str(ROOT/'.local/tmp'),NODE_PATH=str(ROOT/'web/node_modules'))
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
 r=cdp('Runtime.evaluate',dict(expression=source,returnByValue=True,awaitPromise=True,timeout=12000))
 if r.get('exceptionDetails'):raise RuntimeError(r['exceptionDetails'])
 return r['result'].get('value')
def wait(expression):
 return js('(async()=>{for(let n=0;n<500;n++){if(await ('+expression+'))return true;await new Promise(r=>setTimeout(r,20));}throw Error('+json.dumps('Timed out: '+expression)+')})()')
def key(kind,code,key,number):cdp('Input.dispatchKeyEvent',dict(type=kind,code=code,key=key,windowsVirtualKeyCode=number))
def expect(name,value):
 assert value,name
 checks.append(name)
try:
 (OUT/'result.json').write_text('{"status":"running"}\n')
 (OUT/'index.html').write_text('<!doctype html><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Basketball network Board</title><style>body{margin:0;background:#172126;color:#f4eedb;font-family:sans-serif}main{max-width:900px;margin:auto;padding:12px}h1{font-size:18px;margin:0 0 8px}button{font:inherit}.modal-backdrop{position:fixed;inset:0;background:#0008;display:grid;place-items:center;z-index:50}.modal{max-height:90svh;overflow:auto;background:#f5f0df;color:#222;padding:16px;max-width:600px;width:calc(100% - 48px)}</style><link rel="stylesheet" href="/board.css"><div id="root"></div><script type="module" src="/board.js"></script>')
 subprocess.run([str(ROOT/'web/node_modules/.bin/esbuild'),str(ROOT/'scripts/basketball/network_board_harness.tsx'),'--bundle','--format=esm','--jsx=automatic','--loader:.svg=dataurl','--target=es2022','--outfile='+str(OUT/'board.js')],cwd=ROOT,env=env,check=True)

 page=base+'/?stream=1'
 browser('--profile',str(OUT/'profile'),'open',page);opened=True
 endpoint=browser('get','cdp-url')['cdpUrl'];inspector=subprocess.Popen(['node',str(ROOT/'scripts/basketball/browser_cdp_session.mjs'),endpoint,page],env=env,stdin=subprocess.PIPE,stdout=subprocess.PIPE,stderr=(OUT/'cdp.stderr').open('w'),text=True);assert json.loads(inspector.stdout.readline())['ready']
 mirror=(ROOT/'scripts/basketball/browser_oss_mirror.js').read_text();cdp('Page.enable');cdp('Page.addScriptToEvaluateOnNewDocument',dict(source=mirror));browser('reload')
 wait('document.querySelector("[data-basketball-ready=true]") && networkBoard.packets.some(p=>p.type==="game")')
 cpu=float(env.get('BASKETBALL_BROWSER_CPU_RATE','1'));assert 1<=cpu<=8
 cdp('Emulation.setCPUThrottlingRate',dict(rate=cpu))
 js('document.querySelector(".basketball-board").focus()')
 key('keyDown','Space',' ',32);key('keyUp','Space',' ',32);wait('networkBoard.room.game.data.phase==="playing"')
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
 browser('screenshot',str(OUT/'playing-desktop.png'))
 browser('set','viewport','320','720');cdp('Emulation.setTouchEmulationEnabled',dict(enabled=True,maxTouchPoints=2))
 point=js('(()=>{const r=document.querySelector("[data-controller-button=right]").getBoundingClientRect();return{x:r.x+r.width/2,y:r.y+r.height/2}})()')
 cdp('Input.dispatchTouchEvent',dict(type='touchStart',touchPoints=[dict(id=1,**point)]));wait('networkBoard.room.game.data.inputs[0].buttons===1')
 cdp('Input.dispatchTouchEvent',dict(type='touchEnd',touchPoints=[]));wait('networkBoard.room.game.data.inputs[0].buttons===0')
 expect('native mobile touch sends and releases input',True)
 expect('320px board remains visible without horizontal overflow',js('document.documentElement.scrollWidth<=innerWidth'))
 browser('screenshot',str(OUT/'playing-mobile.png'))
 before=js('networkBoard.checkpoints.length');js('disconnectInputStream()')
 wait('networkBoard.checkpoints.length>='+str(before+1)+' && document.querySelector("[data-basketball-ready=true]")')
 js('new Promise(r=>setTimeout(r,1000))');after=js('networkBoard.checkpoints.length')
 expect('reconnection fetches one explicit checkpoint',after==before+1)
 expect('reconnection leaves the match running',js('networkBoard.room.game.data.phase==="playing" && !document.querySelector(".basketball-error")'))
 record=dict(status='passed',checks=checks,metrics=dict(cpuThrottle=cpu,rttMS=int(env.get('BASKETBALL_BROWSER_RTT_MS','0')),sameFrameCorrectionCount=len(corrections),maxSameFrameCorrectionPixels=max(corrections,default=0),gameBytesPerSecond=bytes_per_second,packetCount=len(traffic),frameP95MS=p95,localMoveMS=motion['elapsed'],initialCheckpoints=initial,steadyCheckpoints=current),motion=motion,packets=packets,frameTimes=times,paints=paints,scope='real Board and native input with production StreamInputs/stream merge and actual Go WS server; fixture outer page/audio sink; raw websocket message bytes before compression')
 (OUT/'result.json').write_text(json.dumps(record,ensure_ascii=False,indent=2)+'\n');print(json.dumps(dict(status='passed',checks=checks,metrics=record['metrics']),ensure_ascii=False))
except Exception as error:
 (OUT/'result.json').write_text(json.dumps(dict(status='failed',checks=checks,error=repr(error)),ensure_ascii=False,indent=2)+'\n')
 if opened and inspector:
  try:
   (OUT/'failure-state.json').write_text(json.dumps(js('({body:document.body.innerText,errors:networkBoard.errors,packets:networkBoard.packets,checkpoints:networkBoard.checkpoints,canvas:document.querySelector(".basketball-stage canvas")?.dataset,room:networkBoard.room})'),ensure_ascii=False,indent=2)+'\n')
   browser('screenshot',str(OUT/'failure.png'))
  except Exception:pass
 raise
finally:
 if inspector:
  inspector.stdin.close()
  try:inspector.wait(timeout=5)
  except subprocess.TimeoutExpired:inspector.terminate();inspector.wait(timeout=5)
 if opened:browser('close')
