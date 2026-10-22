#!/usr/bin/env python3
"""Browser geometry regression using the three production poker Boards.
Failure inventory before implementation: empty/played/pass seats resize rows;
long plays and details expand the arena; active borders move labels; waiting,
preview and phase controls shift the hand; shrinking/empty hands collapse the
page. Checks every animation frame, all seats, 320/390/1280px, and scroll access.
Snapshots below are labelled presentation cases, not simulated complete games.
"""
import json,os,shutil,socket,subprocess,time,uuid
from pathlib import Path
root=Path(__file__).resolve().parents[2];out=root/'.local/optimization-20261023/poker-layout';out.mkdir(parents=True,exist_ok=True)
temp=out/'tmp';temp.mkdir(exist_ok=True);env=dict(os.environ)
for key,name in {'TMPDIR':'tmp','npm_config_cache':'npm','AGENT_BROWSER_SOCKET_DIR':'s','AGENT_BROWSER_PROFILE':'profile'}.items():
 p=temp/name;p.mkdir(exist_ok=True);env[key]=str(p)
sockets=root/'.s'/('pl'+uuid.uuid4().hex[:5]);sockets.mkdir(parents=True);env['AGENT_BROWSER_SOCKET_DIR']=str(sockets)
env.update(AGENT_BROWSER_SESSION='p',AGENT_BROWSER_ARGS='--mute-audio,--disable-breakpad,--disable-crash-reporter')
html=root/'web/poker-layout-acceptance.html';entry=root/'web/poker-layout-acceptance.tsx';server=None;opened=False;created=[]
report={'passed':False,'scope':'production Boards with labelled projected-state transitions; no rule changes','checks':[],'failures':[],'interaction':[]}
def b(*args):
 p=subprocess.run(['agent-browser',*args,'--json'],cwd=root,env=env,capture_output=True,text=True,timeout=45)
 if p.returncode:raise RuntimeError(p.stdout+p.stderr)
 d=json.loads(p.stdout);assert d['success'],d;return d['data']
def js(code):return b('eval',code)['result']
try:
 assert not html.exists() and not entry.exists(),'acceptance entry already exists'
 html.write_text('<!doctype html><html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"></head><body><div id="root"></div><script type="module" src="/poker-layout-acceptance.tsx"></script></body></html>');created.append(html)
 entry.write_text('''import React,{useState} from "react";
import {createRoot} from "react-dom/client";
import {Board as DDZ} from "../games/doudizhu/web/Board";
import {Board as GD} from "../games/guandan/web/Board";
import {Board as SJ} from "../games/shengji/web/Board";
import "./src/style.css";
const ids=(n:number)=>Array.from({length:n},(_,i)=>i), audio={play:async()=>true,playWithFallback:async()=>true,stop:()=>{}};
function App(){const [state,setState]=useState({kind:"doudizhu",mode:"idle",seat:0,tick:0});
(window as any).showPoker=(kind:string,mode:string,seat=0)=>setState(s=>({kind,mode,seat,tick:s.tick+1}));
const {kind,mode,seat,tick}=state,cap=kind==="guandan"?28:33;
const seats=ids(4).map(i=>({id:"p"+i,name:i===2?"牌桌另一侧的长名字玩家":"玩家"+(i+1)}));
const controls:any={canAct:mode!=="waiting",canPass:true,canBid:false,canDouble:false,canReady:false,canChoose:false,canOpen:false,reports:[],bombRemaining:-1,bids:[0,1,2,3],moves:[{cards:[0],key:"single:2:1",kind:"single",high:2,wild:[]}],choices:[0],previewCards:[],interpretations:[],declarations:[],canRebel:false,buryCount:8};
let phase=["bid","landlord_choice","declare","bury","tribute","return","round_end","finished"].includes(mode)?mode:"play";
if(mode==="bid")controls.canBid=true;
if(mode==="landlord_choice"){controls.canChoose=true;controls.canOpen=true;controls.reports=[ids(8),ids(7)];}
if(mode==="declare")controls.declarations=[{cards:[0],strength:1,trump:0},{cards:[0,54],strength:2,trump:0}];
if(mode==="round_end")controls.canReady=true;
if(mode==="preview"){controls.previewCards=[0];controls.interpretations=[{cards:[0],key:"single:2:1",kind:"single",high:2,wild:[]}];}
const hand=mode==="empty"?[]:mode==="few"?ids(1):mode==="played"?ids(cap).slice(1):ids(cap);
const long=kind==="guandan"?10:kind==="shengji"?25:30;
const plays=["played","long","pass"].includes(mode)?ids(4).map(i=>({seat:i,cards:mode==="pass"?[]:mode==="long"?ids(long):[i],pass:mode==="pass"})):[];
const room:any={id:"layout-"+kind,kind,you:seat,spectating:mode==="spectator",status:mode==="finished"?"finished":"playing",seats,game:{actors:[mode==="waiting"?1:seat],attention:[],data:{}}};
const data:any={ruleset:"doudizhu-shanghai-huanle-v2",options:{rounds:3,mode:"shanghai4",decks:2},phase,turn:seat,window:tick,round:1,hand,players:ids(4).map(i=>({count:i===seat?hand.length:25,role:i===0?"landlord":"farmer",team:i%2,place:0,score:0})),table:plays,trick:plays,events:mode==="played"||mode==="long"?[{id:tick,type:"play",seat:0,at:Date.now(),kind:"single",cards:[0]}]:[],roundResults:[],controls,level:2,levels:[2,2],landlord:0,bid:3,multiplier:1,marker:0,markerSeat:0,huangFan:false,touLiao:false,openHand:mode==="long",landlordHand:ids(33),reports:mode==="long"?[ids(8)]:[],bottom:ids(8),dealer:0,trump:3,trickNumber:1,attackPoints:0,challengeSeat:-1,declaration:null,revealed:[],lastTrick:mode==="long"?{winner:2,points:20,plays}:null};
const Board:any=kind==="doudizhu"?DDZ:kind==="guandan"?GD:SJ;
return <main className={"room-page playing game-"+kind}><div className="game-layout"><div className="game-main"><Board key={kind} room={room} data={data} assets="" audio={audio} busy={false} clock={{serverNow:Date.now(),receivedAt:performance.now()}} act={async(a:any)=>{if(a.type==="preview")setState(s=>({...s,mode:"preview"}));else setState(s=>({...s,mode:a.type==="pass"?"pass":"played",tick:s.tick+1}));}}/></div></div><div id="after" aria-hidden="true"/></main>}
createRoot(document.getElementById("root")!).render(<App/>);
''');created.append(entry)
 with socket.socket() as sock:sock.bind(('127.0.0.1',0));port=sock.getsockname()[1]
 log=(out/'vite.log').open('w');server=subprocess.Popen(['node','web/node_modules/vite/bin/vite.js','web','--host','127.0.0.1','--port',str(port),'--strictPort'],cwd=root,env=env,stdout=log,stderr=subprocess.STDOUT)
 import urllib.request
 for attempt in range(100):
  try:urllib.request.urlopen(f'http://127.0.0.1:{port}/poker-layout-acceptance.html',timeout=1);break
  except Exception:time.sleep(.1)
 b('open',f'http://127.0.0.1:{port}/poker-layout-acceptance.html');opened=True;b('wait','.poker-board')
 js('''window.layoutBoxes=()=>Object.fromEntries(['.poker-board','.poker-arena','.poker-actions','.poker-hand','#after',...['top','left','right','bottom'].map(p=>'.poker-seat-'+p+' .poker-seat-label')].map(s=>{const e=document.querySelector(s);if(!e)return [s,null];const r=e.getBoundingClientRect();return [s,{x:r.x+scrollX,y:r.y+scrollY,width:r.width,height:r.height}]})); window.transitionPoker=(kind,mode,seat=0)=>new Promise(resolve=>{const frames=[];const start=performance.now();function frame(){frames.push(layoutBoxes());if(performance.now()-start<200)requestAnimationFrame(frame);else resolve(frames)};requestAnimationFrame(frame);showPoker(kind,mode,seat)});''')
 for width in [1280,390,320]:
  b('set','viewport',str(width),'1000')
  for game in ['doudizhu','guandan','shengji']:
   js(f'showPoker({json.dumps(game)},"idle")');b('wait','--fn','document.querySelectorAll(".poker-hand button").length>=28');js('new Promise(r=>setTimeout(r,100))')
   assert js('document.querySelector(".poker-arena").contains(document.querySelector(".poker-hand"))'),'own cards must be on the felt'
   buttons=js('[...document.querySelectorAll(".poker-actions>button")].map(e=>({text:e.textContent,rect:e.getBoundingClientRect().toJSON()}))')
   assert len(buttons)==3 and len({round(b['rect']['y']) for b in buttons})==1,buttons
   assert js('[...document.querySelectorAll(".poker-hand-row")].every(e=>{const r=e.getBoundingClientRect(),p=e.parentElement.getBoundingClientRect();return Math.abs(r.x+r.width/2-p.x-p.width/2)<1})'),'hand rows not centered'
   counts=js('[...document.querySelectorAll(".poker-seat-count")].map(e=>{const a=e.getBoundingClientRect(),b=e.closest(".poker-seat-label").getBoundingClientRect();return {text:e.textContent,visible:a.x>=b.x&&a.right<=b.right&&e.scrollWidth<=e.clientWidth}})');assert len(counts)==4 and all(c['visible'] for c in counts),counts
   baseline=js('layoutBoxes()');handBottom=js('document.querySelector(".poker-hand-row:last-child").getBoundingClientRect().bottom+scrollY');cases=['played','long','pass','waiting','few','empty','round_end','finished','idle']
   cases+=['bid','landlord_choice'] if game=='doudizhu' else ['tribute','return','preview'] if game=='guandan' else ['declare','bury']
   for mode in cases:
    frames=js(f'transitionPoker({json.dumps(game)},{json.dumps(mode)})');deltas=[]
    for f in frames:
     for selector,want in baseline.items():
      if f.get(selector) is None:continue
      for axis in ['x','y','width','height']:
       delta=abs(f[selector][axis]-want[axis])
       if delta>.75:deltas.append({'selector':selector,'axis':axis,'delta':round(delta,2)})
    if mode in ['few','played','long','idle']:
     bottom=js('document.querySelector(".poker-hand-row:last-child").getBoundingClientRect().bottom+scrollY');assert abs(bottom-handBottom)<.75,(game,width,mode,bottom,handBottom)
    check={'game':game,'width':width,'mode':mode,'frames':len(frames),'boxes':frames[-1],'maxShift':max((d['delta'] for d in deltas),default=0)};report['checks'].append(check)
    if deltas:report['failures'].append({**check,'deltas':deltas[:6]})
    assert js('document.documentElement.scrollWidth<=innerWidth+1'),'page overflow'
    if mode in ['idle','long','empty']:b('screenshot',str(out/f'{game}-{width}-{mode}.png'),'--full')
   # Compact hand rows overlap artwork, keeping the exposed index clickable
   # even after a card in the following row is lifted by selection.
   js(f'transitionPoker({json.dumps(game)},"idle")')
   overlap=js('''(()=>{const rs=[...document.querySelectorAll('.poker-hand-row')];if(rs.length<2)return null;return {step:rs[1].getBoundingClientRect().top-rs[0].getBoundingClientRect().top,height:rs[0].querySelector('button').getBoundingClientRect().height};})()''')
   if overlap:assert overlap['step']<overlap['height']*.75,overlap
   rows=js('document.querySelectorAll(".poker-hand-row").length')
   if rows>1:
    b('focus','.poker-hand-row:last-child button:first-child');b('press','Enter')
   for row in range(rows):
    selector=f'.poker-hand-row:nth-child({row+1})';b('scrollintoview',selector)
    hits=js('''[...document.querySelectorAll('''+json.dumps(selector)+'''+ ' button')].map(e=>{const r=e.getBoundingClientRect();return e.contains(document.elementFromPoint(r.x+8,r.y+22));})''')
    assert all(hits),(game,width,row,hits)
   report['interaction'].append({'game':game,'width':width,'overlap':overlap,'allHandCornersClickable':True})
   # Native input and actual Board action callbacks must preserve the regions.
   js(f'transitionPoker({json.dumps(game)},"idle")')
   b('scrollintoview','.poker-hand');r=js('document.querySelector(".poker-hand button").getBoundingClientRect().toJSON()')
   b('mouse','move',str(round(r['x']+8)),str(round(r['y']+22)));b('mouse','down');b('mouse','up')
   assert js('document.querySelectorAll(".poker-hand [aria-pressed=true]").length')==1
   b('find','role','button','click','--name','出牌（1）','--exact')
   b('wait','--fn','document.querySelectorAll(".poker-hand button").length==='+str(27 if game=='guandan' else 32))
   current=js('layoutBoxes()')
   assert all(abs(current[s][k]-v[k])<.75 for s,v in baseline.items() for k in v),'real play callback moved layout'
   js(f'transitionPoker({json.dumps(game)},"idle")');b('scrollintoview','.poker-hand')
   rects=js('[...document.querySelectorAll(".poker-hand-row:first-child button")].slice(0,3).map(e=>e.getBoundingClientRect().toJSON())')
   subprocess.run(['node','scripts/poker/browser-touch.mjs',b('get','cdp-url')['cdpUrl'],f'http://127.0.0.1:{port}',str(round(rects[0]['x']+8)),str(round(rects[0]['y']+24)),str(round(rects[2]['x']+8)),str(round(rects[2]['y']+24))],cwd=root,env=env,check=True,timeout=20)
   assert js('document.querySelectorAll(".poker-hand [aria-pressed=true]").length')==3,'touch multi-select'
   js(f'transitionPoker({json.dumps(game)},"long")');b('scrollintoview','.poker-seat-left .poker-play')
   b('wait','--fn','[...document.querySelectorAll(".poker-new-play")].every(e=>e.getAnimations().every(a=>a.playState!=="running"))')
   scrollable=js('document.querySelector(".poker-seat-left .poker-play").scrollWidth>document.querySelector(".poker-seat-left .poker-play").clientWidth')
   if scrollable:
    b('focus','.poker-seat-left .poker-play');b('press','ArrowRight');b('wait','--fn','document.querySelector(".poker-seat-left .poker-play").scrollLeft>0')
    js('document.querySelector(".poker-seat-left .poker-play").scrollLeft=0')
    b('hover','.poker-seat-left .poker-play');b('scroll','right','100')
    assert js('document.querySelector(".poker-seat-left .poker-play").scrollLeft>0'),'mouse horizontal scroll'
    js('document.querySelector(".poker-seat-left .poker-play").scrollLeft=0')
    r=js('document.querySelector(".poker-seat-left .poker-play").getBoundingClientRect().toJSON()')
    subprocess.run(['node','scripts/browser_touch.mjs',b('get','cdp-url')['cdpUrl'],f'http://127.0.0.1:{port}','drag',str(round(r['right']-8)),str(round(r['y']+30)),str(round(r['left']+8)),str(round(r['y']+30))],cwd=root,env=env,check=True,timeout=20)
    assert js('document.querySelector(".poker-seat-left .poker-play").scrollLeft>0'),'touch scroll'
   # Inspect every exposed corner by scrolling it into the fixed play region.
   visible=js('''(()=>{const all=[];for(const area of document.querySelectorAll('.poker-play')){for(const card of area.querySelectorAll('.poker-strip-slot')){area.scrollLeft=card.offsetLeft;const r=card.getBoundingClientRect(),a=area.getBoundingClientRect();all.push({x:r.x,y:r.y,visible:r.x>=a.x-.5&&r.x+8<=a.right+.5&&r.y>=a.y-.5&&r.bottom<=a.bottom+.5});}area.scrollLeft=0;}return all;})()''')
   assert all(v['visible'] for v in visible),visible
   report['interaction'].append({'game':game,'width':width,'mousePlay':True,'touchSelection':True,'scrollable':scrollable,'cardCorners':len(visible)})
   # Every orientation has the same fixed seat areas.
   for seat in [1,2,3]:
    js(f'transitionPoker({json.dumps(game)},"played",{seat})')
    assert js('document.querySelector(".poker-seat-bottom").dataset.pokerSeat')==str(seat)
   print('CHECKED',game,width,flush=True)
 assert not report['failures'],str(report['failures'][:2])
 assert not b('errors')['errors'],b('errors')
 report['passed']=True
finally:
 if opened:
  try:b('close')
  except Exception:pass
 if server:server.terminate();server.wait(timeout=15);log.close()
 for p in created:p.unlink(missing_ok=True)
 # This task owns these temporary profiles/socket/cache directories only.
 shutil.rmtree(temp)
 shutil.rmtree(sockets)
 (out/'result.json').write_text(json.dumps(report,ensure_ascii=False,indent=2)+'\n')
 print('Artifact:',out/'result.json',flush=True)
