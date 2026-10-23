#!/usr/bin/env python3
"""Production poker Boards driven by labelled snapshot changes, with native input.
Before implementation: another action clears selection; receipt of a tribute or
kitty is invisible; reused card IDs carry selection into a new deal; clock text
escapes its frame; phase transitions leave buttons in a scrolled-away action bar.
These UI projections do not claim to simulate game rules or complete matches.
"""
import argparse,json,os,shutil,socket,subprocess,time,uuid,urllib.request
from pathlib import Path
root=Path(__file__).resolve().parents[2]
p=argparse.ArgumentParser();p.add_argument('--case',choices=['all','selection','layout'],default='all');p.add_argument('--webkit',action='store_true',default=True,help='WebKit supplement for Safari; Chrome is covered by check-hints.py and accept_poker.py');args=p.parse_args()
out=root/'.local/poker-selection-oct23'/(args.case+('-webkit' if args.webkit else ''));out.mkdir(parents=True,exist_ok=True)
temp=out/'temporary';temp.mkdir(exist_ok=True);sockets=root/'.s'/('ps'+uuid.uuid4().hex[:5]);sockets.mkdir(parents=True)
env=dict(os.environ,AGENT_BROWSER_SESSION=sockets.name,AGENT_BROWSER_PROFILE=str(temp/'profile'),AGENT_BROWSER_SOCKET_DIR=str(sockets),TMPDIR=str(temp),AGENT_BROWSER_ARGS='--mute-audio,--disable-breakpad,--disable-crash-reporter')
slug='poker-selection-'+uuid.uuid4().hex[:6];html=root/'web'/f'{slug}.html';entry=root/'web'/f'{slug}.tsx'
server=log=bridge=None;opened=False;report={'passed':False,'scope':__doc__,'checks':[]}
fixture='''import React,{useState} from "react";
import {createRoot} from "react-dom/client";
import {Board as DDZ} from "../games/doudizhu/web/Board";
import {Board as GD} from "../games/guandan/web/Board";
import {Board as SJ} from "../games/shengji/web/Board";
import {ModuleTurn} from "./src/game-api/host";
import "./src/style.css";
const original=[2,15,28,29,42,6,19,32];
const initial=(kind="doudizhu")=>({kind,window:1,round:1,dealSerial:1,matchId:"match1",roomId:"selection",seat:0,turn:1,phase:"play",hand:original,busy:false,remaining:60});
const audio={play:async()=>true,playWithFallback:async()=>true,stop:()=>{}};
function App(){const [s,set]=useState(initial());(window as any).projection=s;(window as any).patchPoker=(v:any)=>set(old=>({...old,...v}));(window as any).resetPoker=(kind:string)=>{(window as any).actions=[];set(initial(kind))};
const seats=[0,1,2,3].map(i=>({id:"p"+i,name:"玩家"+i}));
const c:any={canAct:s.turn===s.seat,canPass:s.turn===s.seat,canReady:false,canBid:false,canDouble:false,canChoose:false,canOpen:false,reports:[],bombRemaining:-1,moves:[],choices:[2,15],previewCards:[],interpretations:[],declarations:s.phase==="declare"?[0,1,2,3].map(i=>({cards:[i*13],strength:1,trump:i})):[],canRebel:s.phase==="declare",bids:[0,1,2,3],buryCount:8};
const data:any={...s,options:{decks:2,mode:"shanghai4",rounds:3},ruleset:s.kind==="doudizhu"?"doudizhu-shanghai-huanle-v2":"guandan-jiangsu-2017-v1",controls:c,players:seats.map((_,i)=>({count:s.hand.length,team:i%2,role:i===0?"landlord":"farmer",score:0,place:0})),events:[],table:[],trick:[],roundResults:[],level:2,levels:[2,2],trump:0,dealer:0,trickNumber:1,attackPoints:0,challengeSeat:-1,landlord:0,bid:1,multiplier:1,marker:0,markerSeat:0,bottom:[],reports:[],landlordHand:[],declaration:null,revealed:[]};
const room:any={id:s.roomId,matchId:s.matchId,kind:s.kind,you:s.seat,seats,game:{actors:[s.turn],attention:[s.turn],finished:false,winners:[],data},status:"playing",turnDeadline:100000+s.remaining*1000};
const Board:any=s.kind==="doudizhu"?DDZ:s.kind==="guandan"?GD:SJ;
return <div data-table-layout="poker"><main data-table-layout="poker" data-projection={JSON.stringify(s)} className={"room-page playing game-"+s.kind}><div className="game-ready-surface"><div className="game-layout"><div className="game-main"><Board key={s.kind} room={room} data={data} assets="" audio={audio} busy={s.busy} clock={{serverNow:100000,receivedAt:performance.now()}} act={async(a:any)=>{(window as any).actions.push(a);if(a.type==="pass")set(old=>({...old,turn:1,window:old.window+1}));else throw Error("验收：服务器拒绝本次出牌")}}/></div><aside className="game-sidebar"><ModuleTurn room={room} serverNow={100000} receivedAt={performance.now()}/></aside></div></div></main></div>}
createRoot(document.getElementById("root")!).render(<App/>);
'''
def b(*cmd):
 bridge.stdin.write(json.dumps(cmd)+'\n');bridge.stdin.flush();v=json.loads(bridge.stdout.readline());assert v['success'],v;return v['data']

def js(code):return b('eval',code)['result']
def patch(v):
 js('patchPoker('+json.dumps(v)+')')
 b('wait','--fn','Object.entries('+json.dumps(v)+').every(([k,v])=>JSON.stringify(JSON.parse(document.querySelector("main").dataset.projection)[k])===JSON.stringify(v))')

def selected():return js('[...document.querySelectorAll(".poker-hand [aria-pressed=true]")].map(e=>+e.dataset.card).sort((a,b)=>a-b)')
def pick(card):
 b('wait','--fn','(()=>{const h=document.querySelector(".poker-hand"),b=h.querySelector("button");const w=h.clientWidth;return Math.abs(b.getBoundingClientRect().width-(w<340?60:w<420?62:68))<.5})()')
 r=js('document.querySelector(".poker-hand [data-card=\\"'+str(card)+'\\"]").getBoundingClientRect().toJSON()')
 if js('innerWidth<=600'):b('tap',str(round(r['x']+8)),str(round(r['y']+24)))
 else:b('mouse','move',str(round(r['x']+8)),str(round(r['y']+24)));b('mouse','down');b('mouse','up')
def check(name,want=None):
 if want is not None:assert selected()==sorted(want),(name,selected(),want,js('projection'))
 report['checks'].append(name)
try:
 html.write_text('<!doctype html><html><head><meta name="viewport" content="width=device-width,initial-scale=1"></head><body><div id="root"></div><script type="module" src="/'+slug+'.tsx"></script></body></html>');entry.write_text(fixture)
 with socket.socket() as sock:sock.bind(('127.0.0.1',0));port=sock.getsockname()[1]
 log=(out/'vite.log').open('w');server=subprocess.Popen(['node','web/node_modules/vite/bin/vite.js','web','--host','127.0.0.1','--port',str(port),'--strictPort'],cwd=root,env=env,stdout=log,stderr=subprocess.STDOUT)
 for _ in range(100):
  try:urllib.request.urlopen(f'http://127.0.0.1:{port}/{slug}.html',timeout=1);break
  except Exception:time.sleep(.1)
 if args.webkit:
  env['PLAYWRIGHT_BROWSERS_PATH']=str(root/'.local/poker-selection-oct23/webkit-browsers');env['POKER_PLAYWRIGHT_MODULE']=str(root/'.local/poker-selection-oct23/webkit/node_modules/playwright/index.mjs')
  bridge=subprocess.Popen(['node','scripts/poker/webkit-bridge.mjs'],cwd=root,env=env,stdin=subprocess.PIPE,stdout=subprocess.PIPE,stderr=log,text=True)
 b('open',f'http://127.0.0.1:{port}/{slug}.html');opened=True;b('wait','.poker-hand')
 for width in [390,320,1280]:
  b('set','viewport',str(width),'720' if width==390 else '640' if width==320 else '960')
  for game in ['doudizhu','guandan','shengji']:
   js('resetPoker('+json.dumps(game)+')');b('wait','.game-'+game+' .poker-hand');patch({})
   label=f'{game}-{width}'
   b('wait','--fn','(()=>{const h=document.querySelector(".poker-hand"),b=h.querySelector("button"),w=h.clientWidth;return Math.abs(b.getBoundingClientRect().width-(w<340?60:w<420?62:68))<.5})()')
   js("(()=>{window.boxRanges={};window.tracking=true;function sample(){if(!window.tracking)return;for(const selector of ['.poker-board','.poker-arena','.poker-hand','.poker-actions']){const r=document.querySelector(selector)?.getBoundingClientRect();if(!r)continue;for(const k of ['x','y','width','height']){const key=selector+k,a=boxRanges[key]??[r[k],r[k]];boxRanges[key]=[Math.min(a[0],r[k]),Math.max(a[1],r[k])];}}requestAnimationFrame(sample)};requestAnimationFrame(sample)})()")

   if args.case in ('all','selection'):
    pick(2);pick(29);check(label+'-choose-while-waiting',[2,29])
    patch({'window':2,'turn':2});check(label+'-opponent-play',[2,29])
    patch({'window':3,'turn':0});check(label+'-own-turn',[2,29]);assert js('actions.length')==0
    patch({'busy':True});check(label+'-busy',[2,29]);patch({'busy':False})
    b('find','role','button','click','--name','出牌（2）','--exact');b('wait','[role=alert]');check(label+'-rejected',[2,29]);b('click','[role=dialog] button[aria-label=关闭]')
    if game!='shengji':
     b('find','role','button','click','--name','不出','--exact');patch({});check(label+'-own-pass',[2,29])
    patch({'hand':[15,28,29,42,6,19,32]});check(label+'-remove-only-missing',[29])
    patch({'hand':[32,19,6,42,29,28,15]});check(label+'-reorder',[29])
    incoming=[53] if game=='guandan' else [0,1,3,4,5,7,8,9]
    patch({'hand':[32,19,6,42,29,28,15]+incoming,'window':10});check(label+'-received-card',[29]+incoming)
    patch({'window':11,'turn':2});check(label+'-receipt-stays-selected',[29]+incoming)
    pick(incoming[0]);check(label+'-manual-deselect-receipt',[29]+incoming[1:]);patch({'window':12});check(label+'-not-selected-again',[29]+incoming[1:])
    exchange=js('projection.hand.filter(id=>id!==15).concat(52)')
    patch({'hand':exchange,'window':13});check(label+'-same-count-return-receipt',[29]+incoming[1:]+[52])
    b('focus','.poker-hand button');b('press','Escape');check(label+'-explicit-clear',[])
    pick(29);patch({'dealSerial':2});check(label+'-redeal-same-round',[])
    pick(29);patch({'matchId':'match2'});check(label+'-new-match',[])
    pick(29);patch({'seat':1});check(label+'-different-seat',[])
    pick(29);patch({'roomId':'another'});check(label+'-new-room',[])
   if args.case in ('all','layout'):
    for phase in (['tribute','return','play'] if game=='guandan' else ['declare','bury','play'] if game=='shengji' else ['play']):
     patch({'phase':phase,'seat':0,'turn':0,'hand':list(range(27))})
     for turn,remaining in [(turn,left) for turn in [0,1] for left in [60,9,0]]:
      patch({'turn':turn,'remaining':remaining})
      if width>600:continue
      boxes=js('''(()=>{const frame=document.querySelector('.turn-card').getBoundingClientRect(),time=document.querySelector('.turn-card time').getBoundingClientRect(),area=document.querySelector('.poker-actions'),a=area.getBoundingClientRect();return {frame:frame.toJSON(),time:time.toJSON(),scrollTop:area.scrollTop,scrollHeight:area.scrollHeight,clientHeight:area.clientHeight,buttons:[...area.querySelectorAll(':scope>button')].map(e=>{const r=e.getBoundingClientRect();return {rect:r.toJSON(),hit:e.contains(document.elementFromPoint(r.x+r.width/2,r.y+r.height/2))}})}})()''')
      f,t=boxes['frame'],boxes['time'];assert t['top']>=f['top'] and t['bottom']<=f['bottom'] and t['left']>=f['left'] and t['right']<=f['right'],(label,boxes)
      if phase=='play':assert len(boxes['buttons'])==3,(label,boxes)
      if turn==0:assert boxes['buttons'] and all(x['hit'] and x['rect']['bottom']<=int(720 if width==390 else 640) for x in boxes['buttons']),(label,phase,boxes)
      contrast=js('''(()=>{const rgb=s=>(s.match(/[0-9.]+/g)||[]).map(Number),lum=c=>c.slice(0,3).map(v=>v/255).map(v=>v<=.04045?v/12.92:((v+.055)/1.055)**2.4).reduce((s,v,i)=>s+v*[.2126,.7152,.0722][i],0);const a=lum(rgb(getComputedStyle(document.querySelector('.turn-card time')).color)),b=lum(rgb(getComputedStyle(document.querySelector('.turn-card')).backgroundColor));return (Math.max(a,b)+.05)/(Math.min(a,b)+.05)})()''')
      assert contrast>=4.5,(label,turn,remaining,contrast)
      check(label+'-'+phase+'-'+str(turn)+'-'+str(remaining)+'-clock-buttons')
      if game=='shengji' and phase=='declare' and turn==0:
       b('find','role','button','click','--name','亮主／反主','--exact');b('wait','[role=dialog]')
       assert js('document.querySelectorAll("[role=dialog] .poker-choice-buttons button").length')==5
       b('click','[role=dialog] button[aria-label=关闭]')

    b('screenshot',str(out/(label+'.png')))
   ranges=js('window.tracking=false;window.boxRanges');assert ranges and all(x[1]-x[0]<.75 for x in ranges.values()),(label,ranges)
   report.setdefault('frameGeometry',[]).append({'case':label,'ranges':ranges})

 report['passed']=True
finally:
 if opened:
  if not report['passed']:
   try:b('screenshot',str(out/'failure.png'))
   except Exception:pass
  try:b('close')
  except Exception:pass
 if bridge:
  bridge.stdin.close()
  try:bridge.wait(timeout=10)
  except subprocess.TimeoutExpired:bridge.terminate();bridge.wait(timeout=10)
 if server:server.terminate();server.wait(timeout=15)
 if log:log.close()
 html.unlink(missing_ok=True);entry.unlink(missing_ok=True)
 shutil.rmtree(temp,ignore_errors=True);shutil.rmtree(sockets,ignore_errors=True)
 (out/'result.json').write_text(json.dumps(report,ensure_ascii=False,indent=2)+'\n');print(out/'result.json')
