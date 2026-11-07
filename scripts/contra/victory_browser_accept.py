#!/usr/bin/env python3
"""Assisted recorded victory boundary -> real platform settlement and UI.

The source contains declared life grants. Private rating eligibility is a test
projection; this does not qualify the diagnostic as a standard scored campaign.
"""
import argparse,hashlib,json,os,re,shutil,subprocess,time,traceback,uuid
from pathlib import Path
root=Path(__file__).resolve().parents[2]
p=argparse.ArgumentParser();p.add_argument('--fixture',required=True);p.add_argument('--label',required=True);args=p.parse_args();assert re.fullmatch('[a-z0-9-]+',args.label)
fixture=json.loads((root/args.fixture).read_text());base=fixture['url'];victory=fixture['victory'];assert victory['ratingProjectionOnly']and victory['boundary']['assisted']
release=Path(victory['release']).resolve();assert release.is_relative_to(root)and not release.exists(),'use a fresh gated fixture'
out=root/'.local/contra-victory'/('browser-'+args.label);out.mkdir();temp=out/'temporary';temp.mkdir();(temp/'tmp').mkdir()
session='cv'+uuid.uuid4().hex[:7];socket=root/'.s'/session;socket.mkdir(parents=True)
env=dict(os.environ,TMPDIR=str(temp/'tmp'),AGENT_BROWSER_SESSION=session,AGENT_BROWSER_SOCKET_DIR=str(socket),AGENT_BROWSER_PROFILE=str(temp/'profile'),AGENT_BROWSER_ARGS='--mute-audio,--disable-breakpad,--disable-crash-reporter')
source=Path(__file__).read_bytes();(out/'harness-at-run.py').write_bytes(source)
report={'passed':False,'scope':'Verified assisted single-player boundary; actual module terminal transition and private rating projection only. Not standard lives, a fresh campaign, full bot acceptance, or a two-player clear.','fixture':{k:fixture.get(k)for k in ['url','rulesBuildID','runtimeBuildID','visualsID','webManifestSHA256','publicRoot']},'victory':victory}
opened=False
def browser(*args):
 r=subprocess.run(['agent-browser',*args,'--json'],cwd=root,env=env,capture_output=True,text=True,timeout=45);assert r.returncode==0,r.stdout+r.stderr;v=json.loads(r.stdout);assert v['success'],v;return v['data']
def js(code):return browser('eval',code)['result']
def api(path,body=None):
 options={'method':'GET'}if body is None else{'method':'POST','headers':{'Content-Type':'application/json'},'body':json.dumps(body)}
 value=js('(async()=>{const r=await fetch('+json.dumps(path)+','+json.dumps(options)+');return {status:r.status,data:await r.json()}})()');assert 200<=value['status']<300,value;return value['data']
def command(kind):
 state=api('/api/state');return api('/api/rooms/'+state['room']['id'],{'type':kind,'version':state['room']['version'],'nonce':uuid.uuid4().hex,'serverInstance':state['serverInstance']})
def wait(code):browser('wait','--fn',code+'||!!document.querySelector(".contra-board-error")');assert not js('document.querySelector(".contra-board-error")?.textContent')
def screenshot(name):browser('screenshot',str(out/(name+'.png')))
def surface():return js('({...document.querySelector("[data-contra-runtime]")?.dataset})')
def pixels():
 return js('(async()=>{const c=document.querySelector(".contra-runtime canvas"),p=c.getContext("2d").getImageData(0,0,c.width,c.height).data,colors=new Set();let nonBlack=0;for(let i=0;i<p.length;i+=4){colors.add(p[i]+","+p[i+1]+","+p[i+2]);if(p[i]+p[i+1]+p[i+2]>30)nonBlack++}return {colors:colors.size,nonBlack,sha256:[...new Uint8Array(await crypto.subtle.digest("SHA-256",p))].map(v=>v.toString(16).padStart(2,"0")).join("")}})()')
rect='(()=>{const r=document.querySelector(".contra-runtime canvas").getBoundingClientRect();return {x:r.x,y:r.y,width:r.width,height:r.height}})()'
def measure():
 return js('(()=>{const modal=document.querySelector(".match-result-modal"),close=modal.querySelector("button[aria-label=关闭]"),m=modal.getBoundingClientRect(),c=close.getBoundingClientRect();const text=[...modal.querySelectorAll("h2,.result-ratings b")].map(n=>{const r=n.getBoundingClientRect(),s=getComputedStyle(n);let p=n,bg;while(p){bg=getComputedStyle(p).backgroundColor;if(bg!=="rgba(0, 0, 0, 0)"&&bg!=="transparent")break;p=p.parentElement}return {text:n.textContent,x:r.x,y:r.y,width:r.width,height:r.height,color:s.color,background:bg,font:s.fontSize}});return {modal:{x:m.x,y:m.y,width:m.width,height:m.height},viewport:[innerWidth,innerHeight],overflow:document.documentElement.scrollWidth>innerWidth,closeHit:close.contains(document.elementFromPoint(c.x+c.width/2,c.y+c.height/2)),insideGame:!!modal.closest("[data-game-fullscreen-surface]"),text}})()')
try:
 browser('open',base);opened=True;browser('set','viewport','1100','1000')
 api('/api/register',{'name':'辅助胜利验收-'+uuid.uuid4().hex[:6],'password':'contra-victory-test-only','invite':'contra-browser'})
 state=api('/api/state');user_id=state['user']['id'];report['profileBefore']=api('/api/players/'+user_id)
 created=api('/api/rooms',{'kind':'contra','capacity':victory['boundary']['players'],'name':'辅助通关胜利验收（仅测试计分投影）','options':{}});room_id=created['id'];browser('reload');browser('wait','--text','辅助通关胜利验收')
 assert js('document.querySelectorAll("[data-fullscreen-toggle]").length')==0;screenshot('waiting')
 command('ready');command('start');browser('reload')
 wait('!!document.querySelector("[data-contra-runtime]")?.dataset.digest&&!!document.querySelector("[data-fullscreen-toggle]:not(:disabled)")')
 state=api('/api/state');assert not state['room'].get('loading');assert state['room']['ratingStarted']and state['room']['ratingEligible']
 before=surface();assert int(before['frame'])==victory['boundary']['prefixFrame']and before['phase']!='finished';assert not js('!!document.querySelector(".match-result-modal")')
 browser('click','[data-fullscreen-toggle]');wait('!!document.querySelector("[data-room-fullscreen]")');screenshot('preterminal-fullscreen')
 time.sleep(.2);assert surface()['digest']==before['digest'],'test timing gate advanced rules'
 js('window.__victoryTrace=[];window.__victoryObserve=true;window.__victoryCanvas=document.querySelector(".contra-runtime canvas");requestAnimationFrame(function frame(at){const r=document.querySelector("[data-contra-runtime]");window.__victoryTrace.push({at,...r?.dataset,result:!!document.querySelector(".match-result-modal")});if(window.__victoryObserve)requestAnimationFrame(frame)});true')
 release.write_text('test-only timing gate; no input/result injection\n')
 wait('!!document.querySelector(".match-result-modal")')
 browser('wait','--fn','document.querySelector(".match-result-modal").getAnimations({subtree:true}).every(a=>a.playState!=="running")')
 report['trace']=js('window.__victoryObserve=false;window.__victoryTrace');assert any(r['result']for r in report['trace']);assert all(r.get('phase')=='finished'for r in report['trace']if r['result'])
 report['finalSurface']=surface();assert int(report['finalSurface']['frame'])==victory['boundary']['finalFrame']and report['finalSurface']['phase']=='finished'
 assert js('window.__victoryCanvas===document.querySelector(".contra-runtime canvas")')
 report['finalRoom']=api('/api/state')['room'];result=report['finalRoom']['result'];assert result['cooperative']and result['rated']and result['outcome']=='cleared'and result['ratingPolicy']=='coop_campaign_v1';assert len(result['players'])==1 and result['players'][0]['won']and result['players'][0]['ratingDelta']==20
 report['resultText']=js('document.querySelector(".match-result-modal").textContent');assert '全队通关'in report['resultText']and '+20'in report['resultText']
 report['desktop']=measure();assert report['desktop']['insideGame']and report['desktop']['closeHit'];screenshot('desktop-victory-result')
 report['beforeClose']=js(rect);browser('click','.match-result-modal button[aria-label="关闭"]');time.sleep(.4)
 for _ in range(3):api('/api/state')
 assert not js('!!document.querySelector(".match-result-modal")');report['afterClose']=js(rect);assert report['beforeClose']==report['afterClose'];screenshot('desktop-final-board')
 report['finalPixels']=pixels();assert report['finalPixels']['nonBlack']>6000 and report['finalPixels']['colors']>=4,'final battlefield is blank or unreadable'
 browser('click','.result-reopen');browser('set','viewport','320','740');browser('wait','--fn','document.querySelector(".match-result-modal").getAnimations({subtree:true}).every(a=>a.playState!=="running")')
 report['mobile']=measure();assert not report['mobile']['overflow']and report['mobile']['insideGame']and report['mobile']['closeHit'];m=report['mobile']['modal'];assert 0<=m['x']and 0<=m['y']and m['x']+m['width']<=320 and m['y']+m['height']<=740;screenshot('mobile-victory-result')
 browser('click','.match-result-modal button[aria-label="关闭"]');screenshot('mobile-final-board');browser('click','[data-room-fullscreen] [data-fullscreen-toggle]');wait('!document.querySelector("[data-room-fullscreen]")');assert js('document.querySelectorAll("[inert]").length')==0
 report['profileAfter']=api('/api/players/'+user_id);assert report['profileAfter']['rating']['points']==report['profileBefore']['rating']['points']+20;assert report['profileAfter']['total']==report['profileBefore']['total']+1
 assert report['profileAfter']['rating']['wins']==report['profileBefore']['rating']['wins']+1 and report['profileAfter']['rating']['played']==report['profileBefore']['rating']['played']+1
 assert sum(row['id']==result['id']for row in report['profileAfter']['history'])==1
 browser('reload');wait('!!document.querySelector("[data-contra-runtime]")?.dataset.digest');again=api('/api/players/'+user_id);assert again['rating']==report['profileAfter']['rating']and again['total']==report['profileAfter']['total'];report['profileAfterReload']=again
 report['pixelsAfterReload']=pixels();assert report['pixelsAfterReload']==report['finalPixels'],'refresh lost the retained battlefield'
 report['requests']=js('performance.getEntriesByType("resource").map(x=>x.name)');assert not any('.nes'in u or '.fcs'in u for u in report['requests'])
 report['passed']=True
except BaseException:
 report['error']=traceback.format_exc()
 if opened:
  try:screenshot('failure');report['snapshot']=browser('snapshot','-i');report['errors']=browser('errors')
  except BaseException:pass
finally:
 if opened:
  try:browser('close')
  except BaseException:report['passed']=False
 for path in [temp,socket]:
  if path.exists():shutil.rmtree(path)
 report['scriptSHA256']=hashlib.sha256(source).hexdigest();(out/'result.json').write_text(json.dumps(report,ensure_ascii=False,indent=2)+'\n');print(json.dumps({'passed':report['passed'],'error':report.get('error'),'output':str(out)},ensure_ascii=False))
raise SystemExit(0 if report['passed']else 1)
