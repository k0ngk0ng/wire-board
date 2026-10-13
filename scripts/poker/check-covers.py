#!/usr/bin/env python3
"""Focused artwork acceptance: actual lobby, picker and waiting-room covers.
Checks both OSS images and SVG fallbacks at desktop/mobile widths. Does not
start games or replay rule matrices. All state and screenshots stay in .local.
Run: . scripts/env.sh && python3 -B scripts/poker/check-covers.py
"""
import hashlib,json,os,subprocess,time,urllib.request,uuid
from pathlib import Path
root=Path(__file__).resolve().parents[2];out=root/'.local/poker-cover-redesign';fixture=root/'.local/poker/browser'
out.mkdir(parents=True,exist_ok=True);fixture.mkdir(parents=True,exist_ok=True)
env=dict(os.environ)
for key,relative in {'AGENT_BROWSER_SOCKET_DIR':'.local/ab-pc','AGENT_BROWSER_PROFILE':'.local/poker-cover-redesign/profile','TMPDIR':'.local/tmp','GOTMPDIR':'.local/tmp','GOCACHE':'.local/go-build','GOMODCACHE':'.local/go-mod','npm_config_cache':'.local/npm'}.items():
 p=root/relative;p.mkdir(parents=True,exist_ok=True);env[key]=str(p)
env.update(WIRE_BOARD_POKER_BROWSER='1',WIRE_BOARD_POKER_ASSETS='https://cdn.ichenj.com/bga/v20261002-1',AGENT_BROWSER_SESSION='pc-'+uuid.uuid4().hex[:6],AGENT_BROWSER_ARGS='--mute-audio,--disable-breakpad,--disable-crash-reporter',GOTOOLCHAIN='local')
report={'passed':False,'checks':[],'assets':[]};server=None;opened=False;handle=None

def b(*args):
 p=subprocess.run(['agent-browser',*args,'--json'],cwd=root,env=env,text=True,capture_output=True,timeout=40)
 if p.returncode:raise RuntimeError(p.stdout+p.stderr)
 result=json.loads(p.stdout);assert result['success'],result;return result['data']
def js(code):return b('eval',code)['result']
def api(path,body=None):
 options='' if body is None else ',{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify('+json.dumps(body)+')}'
 r=js('fetch('+json.dumps('/api'+path)+options+').then(async r=>({status:r.status,body:await r.json()}))');assert 200<=r['status']<300,r;return r['body']
def command(kind):
 r=api('/state')['room'];return api('/rooms/'+r['id'],{'type':kind,'version':r['version'],'nonce':uuid.uuid4().hex})
def shot(name):
 b('wait','--fn','[...document.querySelectorAll(".cover img")].every(i=>i.complete&&i.naturalWidth>0)')
 assert js('document.documentElement.scrollWidth<=innerWidth+2'),name
 assert not b('errors')['errors'],b('errors')
 covers=js('''[...document.querySelectorAll('.cover')].filter(e=>e.querySelector('img,svg')).map(e=>{
 const img=e.querySelector('img'),svg=e.querySelector('svg'),r=e.getBoundingClientRect();return {width:r.width,height:r.height,src:img?.src,fit:img?getComputedStyle(img).objectFit:null,aspect:svg?.getAttribute('preserveAspectRatio')};})''')
 assert all(c['fit']=='contain' if c.get('src') else c['aspect']=='xMidYMid meet' for c in covers),covers
 b('screenshot',str(out/(name+'.png')),'--full');report['checks'].append({'name':name,'covers':covers});print('PASS',name,flush=True)
try:
 entries=json.loads((root/'assets/poker/manifest.json').read_text())['entries']
 for e in [e for e in entries if e.get('type')=='cover']:
  assert '/v2/covers/' in e['output']
  url='https://cdn.ichenj.com/bga/v20261002-1/'+e['output'].removeprefix('web/public/')
  with urllib.request.urlopen(url,timeout=30) as r:data=r.read();mime=r.headers.get_content_type()
  assert mime=='image/webp' and hashlib.sha256(data).hexdigest()==e['sha256'],url
  report['assets'].append({'url':url,'sha256':e['sha256']})
 for name in ['url','stop']:(fixture/name).unlink(missing_ok=True)
 with (out/'build.log').open('w') as f:subprocess.run(['npm','--prefix','web','run','build'],cwd=root,env=env,stdout=f,stderr=subprocess.STDOUT,check=True)
 handle=(out/'server.log').open('w');server=subprocess.Popen(['go','test','-count=1','-timeout=21m','-run','^TestPokerBrowserFixture$','./internal/gametest'],cwd=root,env=env,stdout=handle,stderr=subprocess.STDOUT)
 start=time.monotonic()
 while not (fixture/'url').exists():
  if server.poll() is not None or time.monotonic()-start>90:raise RuntimeError('fixture failed to start')
  time.sleep(.1)
 base=(fixture/'url').read_text();b('open',base);opened=True;api('/login',{'name':'扑克验收管理员','password':'poker-browser-test-only'})
 for kind in ['doudizhu','guandan','shengji']:api('/admin/games/'+kind,{'enabled':True})
 b('reload');b('wait','.game-selection');b('select','select[aria-label="游戏分类"]','poker')
 for width in [1280,320]:b('set','viewport',str(width),'900');shot('lobby-'+str(width))
 for kind in ['doudizhu','guandan','shengji']:
  b('click',f'.game-feature[data-game="{kind}"] button');b('wait','[role=dialog]');b('select','select[aria-label="可选游戏分类"]','poker');shot('picker-'+kind+'-320')
  b('scrollintoview','[role=dialog] button[type=submit]');b('click','[role=dialog] button[type=submit]');b('wait','.waiting-actions')
  for width in [1280,320]:b('set','viewport',str(width),'900');shot('waiting-'+kind+'-'+str(width))
  b('set','viewport','1280','900');js('document.querySelector(".waiting-cover img").dispatchEvent(new Event("error"))');b('wait','.waiting-cover svg');shot('fallback-'+kind+'-1280')
  command('leave');b('wait','.game-selection');b('set','viewport','320','900');b('select','select[aria-label="游戏分类"]','poker')
 report['passed']=True
except Exception as e:
 import traceback;traceback.print_exc();report['error']=repr(e)
 if opened:
  try:b('screenshot',str(out/'failure.png'),'--full');report['snapshot']=b('snapshot','-i')
  except Exception:pass
finally:
 if opened:
  try:b('close')
  except Exception:pass
 if server:
  (fixture/'stop').write_text('stop')
  try:server.wait(timeout=15)
  except subprocess.TimeoutExpired:server.terminate();server.wait(timeout=10)
 if handle:handle.close()
 (out/'result.json').write_text(json.dumps(report,ensure_ascii=False,indent=2)+'\n')
 if not report['passed']:raise SystemExit(1)
