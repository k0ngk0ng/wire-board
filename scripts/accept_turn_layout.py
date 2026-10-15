#!/usr/bin/env python3
"""Real-browser geometry check of the production shared turn component.

Failure cases (before implementation): removing the clock shrinks the card;
state text moves the clock onto another row; narrow widths clip the countdown;
urgent/zero time changes height; hidden clocks remain visible/accessibility
announcements; game-specific CSS overrides the fix. No game rules are run.
Temporary Vite entry points are removed even on failure; artifacts stay local.
"""
import json,os,socket,subprocess,time,uuid
from pathlib import Path
root=Path(__file__).resolve().parents[1];out=root/'.local/turn-layout';out.mkdir(parents=True,exist_ok=True)
env=dict(os.environ)
for key,rel in {'npm_config_cache':'.local/npm','TMPDIR':'.local/tmp','AGENT_BROWSER_SOCKET_DIR':'.local/ab-turn-layout','AGENT_BROWSER_PROFILE':'.local/turn-layout/profile'}.items():
 p=root/rel;p.mkdir(parents=True,exist_ok=True);env[key]=str(p)
env.update(AGENT_BROWSER_SESSION='turn-layout-'+uuid.uuid4().hex[:8],AGENT_BROWSER_ARGS='--mute-audio,--disable-breakpad,--disable-crash-reporter')
html=root/'web/turn-layout-acceptance.html';entry=root/'web/turn-layout-acceptance.tsx';server=None;opened=False;created=[]
report={'passed':False,'scope':'production ModuleTurn, all independently registered games and actual stylesheet cascade','widths':[],'failures':[],'command':'. scripts/env.sh && python3 -B scripts/accept_turn_layout.py'}
def b(*args):
 r=subprocess.run(['agent-browser',*args,'--json'],cwd=root,env=env,capture_output=True,text=True,timeout=50)
 if r.returncode:raise RuntimeError(r.stdout+r.stderr)
 v=json.loads(r.stdout);assert v['success'],v;return v['data']
def js(code):return b('eval',code)['result']
try:
 assert not html.exists() and not entry.exists(),'test entry already exists'
 html.write_text('<!doctype html><html><head><meta charset="UTF-8"><meta name="viewport" content="width=device-width,initial-scale=1"></head><body><div id="root"></div><script type="module" src="/turn-layout-acceptance.tsx"></script></body></html>');created.append(html)
 entry.write_text('''import React,{useState} from "react";
import {createRoot} from "react-dom/client";
import {gameModule} from "./src/game-api/registry";
import {ModuleTurn} from "./src/game-api/host";
import "./src/style.css";
const ids=__TURN_IDS__.filter((id:string)=>!gameModule(id)?.legacy);
(window as any).turnKinds=ids;
function App(){const [display,setDisplay]=useState({kind:"weiqi",mode:"mine"});
(window as any).showTurn=async(kind:string,mode:string)=>{setDisplay({kind,mode});await new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r)));};
const {kind,mode}=display,serverNow=Date.now(),receivedAt=performance.now();
const room:any={id:"layout",kind,name:"公共提示栏布局验收",status:"playing",you:0,seats:[{id:"a",name:"自己"},{id:"b",name:"对手"}],turnDeadline:mode==="neutral"||mode==="finished"?0:serverNow+(mode==="urgent"?19000:mode==="zero"?-1000:94000),game:{finished:mode==="finished",attention:mode==="neutral"||mode==="finished"?[]:mode==="waiting"?[1]:[0],actors:[0,1],winners:[],log:[]}};
return <main className={`room-page playing game-${kind}`}><h1>{gameModule(kind)?.name} · 公共提示栏验收</h1><div className="game-layout"><div className="game-main"><div style={{height:300,border:"2px solid #8a9b7b",background:"#eee9d7",padding:20}}>棋盘位置参照</div></div><aside className="game-sidebar"><ModuleTurn room={room} serverNow={serverNow} receivedAt={receivedAt}/></aside></div></main>}
createRoot(document.getElementById("root")!).render(<App/>);
'''.replace('__TURN_IDS__',json.dumps(sorted(p.parent.parent.name for p in (root/'games').glob('*/web/module.tsx')))));created.append(entry)
 with socket.socket() as s:s.bind(('127.0.0.1',0));port=s.getsockname()[1]
 with (out/'vite.log').open('w') as log:server=subprocess.Popen(['npm','--prefix','web','run','dev','--','--port',str(port),'--strictPort'],cwd=root,env=env,stdout=log,stderr=subprocess.STDOUT)
 for _ in range(100):
  with socket.socket() as s:
   if s.connect_ex(('127.0.0.1',port))==0:break
  if server.poll() is not None:raise RuntimeError('Vite exited')
  time.sleep(.1)
 b('open',f'http://127.0.0.1:{port}/turn-layout-acceptance.html');opened=True;b('wait','.turn-card')
 for width in [320,360,390,480,600,850,851,1024,1440]:
  b('set','viewport',str(width),'900')
  rows=js('''(async()=>{const rows=[];for(const kind of window.turnKinds)for(const mode of ['mine','neutral','waiting','urgent','zero','finished']){await window.showTurn(kind,mode);const card=document.querySelector('.turn-card'),r=card.getBoundingClientRect(),board=document.querySelector('.game-main').getBoundingClientRect(),clock=card.querySelector('.turn-clock'),reading=card.querySelector('.clock-reading'),time=card.querySelector('time');rows.push({kind,mode,height:r.height,boardY:board.y+scrollY,overflow:document.documentElement.scrollWidth>innerWidth+1,clockVisible:!!clock&&getComputedStyle(clock).visibility!=='hidden',clockHidden:clock?.getAttribute('aria-hidden'),clockFits:!reading||reading.getBoundingClientRect().right<=r.right-1,time:time?.textContent,headline:card.querySelector('h3').textContent})}return rows})()''')
  report['widths'].append({'width':width,'rows':rows})
  for kind in {r['kind'] for r in rows}:
   group=[r for r in rows if r['kind']==kind];base=group[0]
   for r in group:
    if abs(r['height']-base['height'])>.5 or abs(r['boardY']-base['boardY'])>.5 or r['overflow'] or not r['clockFits']:
     report['failures'].append({'width':width,**r,'expectedHeight':base['height'],'expectedBoardY':base['boardY']})
  for mode in ['mine','neutral']:
   js(f'window.showTurn("weiqi",{json.dumps(mode)})');b('screenshot',str(out/f'{width}-{mode}.png'),'--full')
  print('MEASURED',width,'games',len({r['kind'] for r in rows}),flush=True)
 assert not b('errors')['errors'],b('errors')
 assert not report['failures'],report['failures'][:12]
 for item in report['widths']:
  for r in item['rows']:
   if r['mode'] in ['neutral','finished']:assert not r['clockVisible'] and r['clockHidden']=='true',r
   else:assert r['clockVisible'] and r['clockFits'],r
 report['passed']=True
except Exception as e:
 report['error']=str(e)
 if opened:
  report['browserErrors']=b('errors');b('screenshot',str(out/'failure.png'),'--full')
 raise
finally:
 if opened:
  try:b('close')
  except Exception:pass
 if server:
  server.terminate();server.wait(timeout=15)
 for p in created:p.unlink()
 (out/'result.json').write_text(json.dumps(report,ensure_ascii=False,indent=2)+'\n')
 print('Artifacts:',out,flush=True)
