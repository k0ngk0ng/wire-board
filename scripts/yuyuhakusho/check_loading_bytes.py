"""Real app/Wasm loading bytes; failure inventory precedes implementation.

Native downloads lose byte counts; decoded rather than compressed assets enter
the total; initialization/waiting clears counts; warm resources double count;
other arenas preload; stale callbacks reopen a gate; drag leaks game input.
Only transfer timing is controlled. Both seats use real authenticated browsers.
"""
from pathlib import Path
import argparse,functools,hashlib,http.cookiejar,json,os,subprocess,time,urllib.request
ROOT=Path(__file__).resolve().parents[2]
p=argparse.ArgumentParser();p.add_argument('--label',required=True);a=p.parse_args()
OUT=ROOT/'games/yuyuhakusho/verification/loading-bytes'/a.label;OUT.mkdir(parents=True,exist_ok=False)
TMP=ROOT/'.local/loading-chat'/a.label;TMP.mkdir(parents=True,exist_ok=False)
common=ROOT.parents[1]if ROOT.parent.name=='worktrees'else ROOT
env=dict(os.environ,AGENT_BROWSER_SOCKET_DIR=str(common/'.local/lc'),TMPDIR=str(TMP))
report={'passed':False,'checks':[],'scope':'Normal app, SQLite, actual Wasm, two human browsers; only response-stream timing controlled'}
clients=[];jars=[];host=None;opened=[]
def b(who,*args):
 task=dict(env,AGENT_BROWSER_SESSION='lb'+hashlib.sha256(a.label.encode()).hexdigest()[:5]+str(who))
 r=subprocess.run(['agent-browser',*args,'--json'],env=task,cwd=ROOT,text=True,capture_output=True,timeout=45);v=json.loads(r.stdout);assert r.returncode==0 and v['success'],r.stdout+r.stderr;return v['data']
def js(who,code):return b(who,'eval',code)['result']
def check(name,value):assert value,name;report['checks'].append(name)
def api(who,path,body=None):
 req=urllib.request.Request(url.rstrip('/')+'/api'+path,data=None if body is None else json.dumps(body).encode(),headers={'Content-Type':'application/json'})
 with clients[who].open(req,timeout=20)as response:return json.load(response)
def room(who=0):return api(who,'/state')['room']
def cmd(who,kind,**extra):
 r=room(who);return api(who,'/rooms/'+r['id'],{'type':kind,'version':r['version'],'nonce':os.urandom(12).hex(),**extra})
def wait(fn,seconds=30):
 deadline=time.monotonic()+seconds
 while time.monotonic()<deadline:
  v=fn()
  if v:return v
  time.sleep(.1)
 raise AssertionError('Loading condition timed out')
def open_browser(who):
 state={'cookies':[{'name':c.name,'value':c.value,'domain':c.domain,'path':c.path,'expires':c.expires or-1,'secure':False,'httpOnly':True,'sameSite':'Lax'}for c in jars[who]],'origins':[]};f=TMP/('state-'+str(who)+'.json');f.write_text(json.dumps(state));f.chmod(0o600)
 b(who,'--state',str(f),'open',url);opened.append(who);b(who,'set','viewport','320','740');b(who,'wait','.profile-nav')
probe=r'''(()=>{if(window.loadByteProbe)return;const p=window.loadByteProbe={samples:[],requests:[],hold:false};const f=window.fetch;window.fetch=async(...args)=>{const r=await f(...args),u=String(args[0]);if(!(u.includes('client.wasm')||u.includes('.json.gz'))||!r.body)return r;const reader=r.body.getReader(),entry={url:u,bytes:0};p.requests.push(entry);return new Response(new ReadableStream({async pull(c){while(p.hold)await new Promise(r=>setTimeout(r,20));await new Promise(r=>setTimeout(r,80));const next=await reader.read();if(next.done)c.close();else{entry.bytes+=next.value.length;c.enqueue(next.value)}},cancel(){return reader.cancel()}}),{status:r.status,headers:r.headers})};setInterval(()=>{const e=document.querySelector('[data-game-loading]');if(e){const s=e.querySelector('small');p.samples.push({at:performance.now(),label:e.querySelector('[role=status]')?.textContent,percent:e.querySelector('progress')?.value,text:s?.textContent,loaded:s?.dataset.loadedBytes,total:s?.dataset.totalBytes});if(p.samples.length>4000)p.samples.shift()}},20)})()'''
try:
 host=subprocess.Popen([str(ROOT/'.local/loading-chat/app-host'),'-build',str(ROOT/'web/dist'),'-data',str(TMP/'data')],cwd=ROOT,env=env,stdin=subprocess.PIPE,stdout=subprocess.PIPE,stderr=(OUT/'host.log').open('w'),text=True)
 url=json.loads(host.stdout.readline())['direct'];assert url.startswith('http://127.0.0.1:')
 for who in[0,1]:
  jar=http.cookiejar.CookieJar();jars.append(jar);clients.append(urllib.request.build_opener(urllib.request.HTTPCookieProcessor(jar)))
  api(who,'/register',{'name':'大小验收'+str(who)+hashlib.sha256(a.label.encode()).hexdigest()[:5],'password':'LocalByteAcceptance01','invite':'yuyu-local-e2e'})
 open_browser(0)
 descriptor=json.loads((ROOT/'games/yuyuhakusho/data/runtime-resource-descriptor.json').read_text());runtime=json.loads((ROOT/'web/dist/yuyuhakusho/v1/runtime/build.json').read_text())
 ids={'rules','sprites','hud','render_meta','arena-default'};expected=runtime['wasm']['bytes']+runtime.get('files',{}).get('wasm_exec.js',{}).get('bytes',0)+sum(r['gzip_bytes']for r in descriptor['resources']if r['id']in ids)
 for phase in['cold','warm']:
  api(0,'/rooms',{'kind':'yuyuhakusho','capacity':2,'name':'加载大小验收'});r=room(0);api(1,'/rooms/'+r['id'],{'type':'join','version':r['version'],'nonce':os.urandom(12).hex()})
  if phase=='cold':b(0,'reload');b(0,'wait','[aria-label=比赛准备]');js(0,probe)
  cmd(1,'ready');cmd(0,'ready');cmd(0,'start')
  wait(lambda:room()['loading']['ready'][0]if room().get('loading')else False,45)
  samples=js(0,'loadByteProbe.samples');report[phase]={'samples':samples,'requests':js(0,'loadByteProbe.requests'),'expectedNativeBytes':expected}
  native=[s for s in samples if s.get('total')and int(s['total'])==expected]
  check(phase+' reports real native total',bool(native))
  check(phase+' retains full sizes while waiting for peer',any(s.get('percent')==100 and s.get('loaded')==str(expected) and s.get('total')==str(expected) and s.get('text')for s in samples))
  if phase=='cold':check('cold download shows intermediate measured bytes',any(0<int(s.get('loaded')or 0)<expected for s in native))
  check('peer gate holds simulation',room()['game']['data']['frame']=='0')
  check('mobile loading fits',js(0,'document.documentElement.scrollWidth<=innerWidth'))
  b(0,'screenshot',str(OUT/(phase+'-mobile.png')))
  if phase=='cold':cmd(0,'close');cmd(1,'leave');cmd(0,'leave');wait(lambda:not api(0,'/state').get('room'));js(0,'loadByteProbe.samples=[];true')
 report['warm']['requestsAfterWarm']=js(0,'loadByteProbe.requests');check('warm load does not redownload Wasm or graphics',len(report['cold']['requests'])==len(report['warm']['requestsAfterWarm']))
 open_browser(1);wait(lambda:not room().get('loading'),45);wait(lambda:room()['game']['data']['phase']=='playing',30)
 b(0,'find','role','button','click','--name','全屏','--exact');b(0,'wait','[data-room-fullscreen]')
 endpoint=b(0,'get','cdp-url')['cdpUrl'];drag=subprocess.run(['node','scripts/fullscreen/drag.mjs',endpoint,url,'.chat-toggle','touch','-90','-120'],cwd=ROOT,env=env,text=True,capture_output=True,check=True);moved=json.loads(drag.stdout)
 check('actual game shared chat moves',moved['after']['x']<moved['before']['x']-50 and moved['after']['y']<moved['before']['y']-80)
 check('drag does not send game buttons',all(i['appliedButtons']==0 for i in room()['game']['data']['inputs']))
 b(0,'screenshot',str(OUT/'actual-fullscreen-chat.png'));check('no runtime errors',not b(0,'errors')['errors']);report['passed']=True
except BaseException as error:
 report['error']=repr(error)
 try:b(0,'screenshot',str(OUT/'failure.png'))
 except Exception:pass
 raise
finally:
 for who in opened:
  try:b(who,'close')
  except Exception:pass
 if host:host.stdin.close();host.wait(timeout=15)
 report['sources']={str(p.relative_to(ROOT)):hashlib.sha256(p.read_bytes()).hexdigest()for p in[Path(__file__),ROOT/'games/yuyuhakusho/web/Board.tsx',ROOT/'games/yuyuhakusho/web/assets.ts',ROOT/'games/yuyuhakusho/web/wasm.ts',ROOT/'web/src/game-api/loading.tsx',ROOT/'web/src/chat.tsx']}
 (OUT/'result.json').write_text(json.dumps(report,ensure_ascii=False,indent=2)+'\n');print(json.dumps({'passed':report['passed'],'checks':len(report['checks']),'result':str(OUT/'result.json')},ensure_ascii=False))
