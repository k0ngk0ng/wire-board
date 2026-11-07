#!/usr/bin/env python3
"""Real native Go authority + Wasm replay + Canvas/FCController browser E2E."""
import argparse,hashlib,json,os,re,shutil,subprocess,time,traceback,uuid
from pathlib import Path

root=Path(__file__).resolve().parents[2];parser=argparse.ArgumentParser();parser.add_argument('--label',required=True);args=parser.parse_args();assert re.fullmatch(r'[a-z0-9-]+',args.label)
out=root/'.local/contra-browser'/('accept-'+args.label);assert not out.exists();out.mkdir(parents=True);temp=out/'temporary';temp.mkdir();(temp/'tmp').mkdir()
build=root/'.local/contra-browser/build';assert (build/'development-server').is_file(),'run browser_build.py first'
session='cb'+uuid.uuid4().hex[:6];sockets=root/'.s'/session;sockets.mkdir(parents=True)
env=dict(os.environ,TMPDIR=str(temp/'tmp'),AGENT_BROWSER_SESSION=session,AGENT_BROWSER_SOCKET_DIR=str(sockets),AGENT_BROWSER_PROFILE=str(temp/'profile'),AGENT_BROWSER_ARGS='--mute-audio,--disable-breakpad,--disable-crash-reporter')
build_proof=json.loads((build/'build-source.json').read_text())
for source,digest in build_proof['inputs'].items():assert hashlib.sha256((root/source).read_bytes()).hexdigest()==digest,('stale development build',source)
report={'passed':False,'scope':'Unregistered local development host; standard new Session, Intro and stage-one native PC/touch controls. Go server authority and Go/Wasm replay; no ROM runtime or injected after-state.','cases':[]};server=None;opened=False
def browser(*args):
 p=subprocess.run(['agent-browser',*args,'--json'],cwd=root,env=env,capture_output=True,text=True,timeout=45);assert p.returncode==0,p.stdout+p.stderr;j=json.loads(p.stdout);assert j['success'],j;return j['data']
def js(code):return browser('eval',code)['result']
def api(path,body):return js('(async()=>{const r=await fetch('+json.dumps(path)+',{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify('+json.dumps(body)+')});return {status:r.status,data:await r.json()}})()')
def native(options):
 p=subprocess.run(['node','scripts/contra/browser_input.mjs',browser('get','cdp-url')['cdpUrl'],base,json.dumps(options)],cwd=root,env=env,capture_output=True,text=True,timeout=20);assert p.returncode==0,p.stdout+p.stderr;return json.loads(p.stdout)
try:
 log=(out/'server.log').open('w');server=subprocess.Popen([str(build/'development-server'),'--public',str(build/'public'),'--resources',str(root/'games/contra/engine/leveldata/stage-1.json'),'--data',str(temp/'data'),'--url-file',str(temp/'url')],cwd=root,env=env,stdout=log,stderr=subprocess.STDOUT)
 for _ in range(200):
  if (temp/'url').exists():break
  assert server.poll()is None,'development host exited';time.sleep(.02)
 base=(temp/'url').read_text();browser('open',base);opened=True
 registered=api('/api/register',{'name':'Contra'+uuid.uuid4().hex[:7],'password':'contra-browser-accept-only','invite':'contra-browser'});assert registered['status']==200,registered
 browser('reload');browser('wait','--fn','window.contraBrowser?.ready===true||!!window.contraBrowser?.error');assert not js('window.contraBrowser.error'),js('window.contraBrowser.error')
 for name,width,height in [('desktop',1100,1000),('mobile',320,740)]:
  browser('set','viewport',str(width),str(height));browser('click','#begin');browser('wait','--fn',"window.contraBrowser?.snapshot?.phase==='intro'&&Number(window.contraBrowser.snapshot.frame)>=12||!!window.contraBrowser?.error")
  assert not js('window.contraBrowser.error'),js('window.contraBrowser.error');browser('screenshot',str(out/(name+'-intro.png')))
  intro=js('window.contraBrowser.snapshot');assert intro['players'][0]['lives']==2
  if name=='desktop':native({'keys':['Space'],'frames':2})
  else:native({'touch':[{'selector':'[data-controller-button="start"]'}],'frames':2})
  browser('wait','--fn',"window.contraBrowser.snapshot.phase==='playing'&&window.contraBrowser.snapshot.players[0].state===1||!!window.contraBrowser.error")
  assert not js('window.contraBrowser.error'),js('window.contraBrowser.error')
  before=js('window.contraBrowser.snapshot')
  if name=='desktop':action=native({'keys':['KeyD','KeyJ'],'frames':14});mouse=native({'mouse':'[data-controller-button="b"]','frames':3})
  else:action=native({'touch':[{'selector':'[data-controller-dpad]','x':.83,'y':.5},{'selector':'[data-controller-button="b"]'}],'frames':14});mouse=None
  jump=native({'keys':['KeyK'],'frames':4})if name=='desktop'else native({'touch':[{'selector':'[data-controller-button="a"]'}],'frames':4})
  after=js('window.contraBrowser.snapshot');assert after['players'][0]['x']!=before['players'][0]['x'],(before['players'][0],after['players'][0])
  proof=js('({checked:window.contraBrowser.client.evidence.checked,frames:window.contraBrowser.client.evidence.frames,inputs:window.contraBrowser.client.evidence.inputs,checkpoints:window.contraBrowser.client.evidence.checkpoints,missingAudio:window.contraBrowser.client.missingAudio,error:window.contraBrowser.error})');assert not proof.get('error')and proof['checked']>=25,proof
  assert any(f['projectiles']>0 for f in proof['frames']), 'no authoritative projectiles'
  assert any(i['mask']&65==65 for i in proof['inputs']),'combined physical input was not sampled'
  pause_input={'keys':['Space'],'frames':3,'cursor':'revision'}if name=='desktop'else {'touch':[{'selector':'[data-controller-button="start"]'}],'frames':3,'cursor':'revision'}
  pause=native(pause_input);browser('wait','--fn','window.contraBrowser.snapshot.paused||!!window.contraBrowser.error');paused=js('window.contraBrowser.snapshot')
  browser('wait','--fn','BigInt(window.contraBrowser.snapshot.audioTick)>=BigInt('+json.dumps(paused['audioTick'])+')+30n||!!window.contraBrowser.error')
  held_pause=js('window.contraBrowser.snapshot');assert held_pause['frame']==paused['frame']and held_pause['players']==paused['players'],(paused,held_pause)
  resumed=native(pause_input);browser('wait','--fn','!window.contraBrowser.snapshot.paused||!!window.contraBrowser.error');assert not js('window.contraBrowser.error'),js('window.contraBrowser.error')
  browser('focus','input[aria-label="输入隔离验收"]');index=js('window.contraBrowser.client.evidence.inputs.length');native({'keys':['KeyD','KeyJ'],'frames':5});blocked=js('window.contraBrowser.client.evidence.inputs')[index:];assert all(i['mask']==0 for i in blocked),blocked
  js('document.querySelector("[data-contra-runtime]").focus();true')
  resync=None
  if name=='desktop':
   native({'keys':['KeyD'],'frames':2,'hold':True});binding=js('window.contraBrowser.snapshot');browser('tab','new',base+'/idle.html')
   deadline=time.monotonic()+6
   while time.monotonic()<deadline:
    observed=api('/runtime/poll',{'session':binding['session'],'epoch':binding['epoch'],'frame':binding['frame']});assert observed['status']==200,observed
    if int(observed['data']['snapshot']['frame'])>=int(binding['frame'])+130:break
    time.sleep(.02)
   assert observed['data'].get('checkpoint'),'expected real history truncation checkpoint'
   held=observed['data']['frames'][-1]['pads']if observed['data']['frames']else observed['data']['checkpoint']['state']['Controls']['held']
   assert held==[0,0],held
   browser('tab','t1');native({'keys':['KeyD'],'releaseOnly':True});browser('wait','--fn','window.contraBrowser.client.evidence.resyncs>=1||!!window.contraBrowser.error')
   assert not js('window.contraBrowser.error'),js('window.contraBrowser.error');resync=js('({epoch:window.contraBrowser.snapshot.epoch,resyncs:window.contraBrowser.client.evidence.resyncs,frame:window.contraBrowser.snapshot.frame,checkpointFallback:true})');assert resync['epoch']==binding['epoch'];browser('tab','close','t2')
  geometry=js('(()=>{const c=document.querySelector("canvas").getBoundingClientRect(),p=document.querySelector("[data-controller-layout]").getBoundingClientRect();const d=document.querySelector("canvas").getContext("2d").getImageData(0,0,256,240).data;const colors=new Set();for(let i=0;i<d.length;i+=4)colors.add(d[i]+","+d[i+1]+","+d[i+2]);return {canvas:{x:c.x,y:c.y,w:c.width,h:c.height},controller:{x:p.x,y:p.y,w:p.width,h:p.height},overflow:document.documentElement.scrollWidth>innerWidth+1,colors:colors.size}})()')
  assert not geometry['overflow']and geometry['controller']['y']>=geometry['canvas']['y']+geometry['canvas']['h']-1 and geometry['colors']>=8 and geometry['canvas']['y']>=0 and geometry['controller']['y']+geometry['controller']['h']<=height,geometry
  js('scrollTo(0,0);true');browser('screenshot',str(out/(name+'-playing.png')))
  proof=js('({checked:window.contraBrowser.client.evidence.checked,frames:window.contraBrowser.client.evidence.frames,inputs:window.contraBrowser.client.evidence.inputs,checkpoints:window.contraBrowser.client.evidence.checkpoints,missingAudio:window.contraBrowser.client.missingAudio,error:window.contraBrowser.error})')
  requests=js('performance.getEntriesByType("resource").map(x=>x.name)');assert not any('.nes' in u or '.fcs' in u for u in requests);assert not any('stage-2.json' in u for u in requests)
  report['cases'].append({'name':name,'intro':intro,'before':before,'after':after,'nativeInput':action,'mouse':mouse,'jump':jump,'pause':pause,'pausedSnapshot':held_pause,'resume':resumed,'resync':resync,'geometry':geometry,'proof':proof,'requests':requests})
  closed=js('(async()=>{const t=window.contraBrowser;t.client.dispose();const r=await fetch("/runtime/close",{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify({session:t.snapshot.session,epoch:t.snapshot.epoch})});return r.status})()');assert closed==200
  if name=='desktop':browser('reload');browser('wait','--fn','window.contraBrowser?.ready===true')
 report['passed']=True
except BaseException:
 report['error']=traceback.format_exc()
 if opened:
  try:report['browser']=js('({error:window.contraBrowser?.error,snapshot:window.contraBrowser?.snapshot,checked:window.contraBrowser?.client?.evidence.checked})');browser('screenshot',str(out/'failure.png'));report['browserErrors']=browser('errors')
  except BaseException:pass
finally:
 if opened:
  try:browser('close')
  except BaseException:report['passed']=False
 if server:
  server.terminate()
  try:server.wait(timeout=10)
  except subprocess.TimeoutExpired:server.kill();server.wait()
  log.close()
 for p in [temp,sockets]:
  if p.exists():shutil.rmtree(p)
 report['build']=json.loads((build/'build-source.json').read_text());report['scriptSHA256']=hashlib.sha256(Path(__file__).read_bytes()).hexdigest();(out/'result.json').write_text(json.dumps(report,ensure_ascii=False,indent=2)+'\n');print(json.dumps({'passed':report['passed'],'error':report.get('error'),'output':str(out)},ensure_ascii=False))
raise SystemExit(0 if report['passed']else 1)
