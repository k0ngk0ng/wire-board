#!/usr/bin/env python3
"""Two-step creation E2E, authored before implementation.
Failures: settings shown before choosing; choice submits a room; missing/hidden
selection or cover; legacy/module forms diverge; back loses drafts or sends
another game's options; hidden steps retain keyboard focus; unpublished games
appear; narrow-screen overflow; selected capacity/options are not submitted.
Builds the real application with all registered games and a local test database.
"""
import json,os,re,subprocess,tempfile,time,uuid
from pathlib import Path
root=Path(__file__).resolve().parents[1];out=root/'.local/create-flow';out.mkdir(parents=True,exist_ok=True)
env=dict(os.environ)
for key,rel in {'GOCACHE':'.local/go-build','GOMODCACHE':'.local/go-mod','GOTMPDIR':'.local/tmp','TMPDIR':'.local/tmp','npm_config_cache':'.local/npm','AGENT_BROWSER_SOCKET_DIR':'.local/ab-create-flow','AGENT_BROWSER_PROFILE':'.local/create-flow/profile'}.items():
 p=root/rel;p.mkdir(parents=True,exist_ok=True);env[key]=str(p)
env.update(GOTOOLCHAIN='local',AGENT_BROWSER_SESSION='create-flow-'+uuid.uuid4().hex[:8],AGENT_BROWSER_ARGS='--mute-audio,--disable-breakpad,--disable-crash-reporter',ADDR='127.0.0.1:0',ADMIN_USERNAME='创建流程验收',ADMIN_PASSWORD='create-flow-only-password',ADMIN_EXISTING_USERNAME='',INVITE_CODE='create-flow',PUBLIC_ORIGIN='',COOKIE_SECURE='false',ASSETS_BASE_URL='')
report={'passed':False,'checks':[],'rooms':[],'command':'. scripts/env.sh && python3 -B scripts/accept_create_flow.py'};server=None;opened=False;data_dir=None

def run(args,name):
 with (out/name).open('w') as f:subprocess.run(args,cwd=root,env=env,stdout=f,stderr=subprocess.STDOUT,check=True)
def b(*args):
 r=subprocess.run(['agent-browser',*args,'--json'],cwd=root,env=env,capture_output=True,text=True,timeout=40)
 if r.returncode:raise RuntimeError(r.stdout+r.stderr)
 d=json.loads(r.stdout);assert d['success'],d;return d['data']
def js(code):return b('eval',code)['result']
def api(path,body=None):
 opts='' if body is None else ',{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify('+json.dumps(body)+')}'
 r=js('fetch('+json.dumps('/api'+path)+opts+').then(async r=>({status:r.status,body:await r.json()}))');assert r['status']<300,r;return r['body']
def click(name):b('find','role','button','click','--name',name,'--exact')
def choose(kind,name,keyboard=False):
 b('fill','input[aria-label=搜索可选游戏]',kind)
 if keyboard:b('focus','.choose-games button');b('press','Enter')
 else:click(name)
 b('wait','[data-create-step=setup]:not([hidden])')
 assert api('/state').get('room') is None,'choosing a game accidentally submitted the room'
 assert js('document.querySelector("[data-create-step=choose]").hidden')
 assert js('document.querySelector(".create-selected-game").textContent.includes('+json.dumps(name)+')')
 assert js('document.querySelector(".create-selected-game svg,.create-selected-game img").getBoundingClientRect().width>0')
 b('wait','--fn','[...document.querySelectorAll(".create-selected-game img")].every(e=>e.complete&&e.naturalWidth>0)')
def first():
 b('click','.lobby-create');b('wait','[role=dialog]')
 assert js('document.querySelectorAll("input[name=name]").length===0'),'room fields appeared before selection'
 assert js('document.querySelectorAll("form button[type=submit]").length===0'),'submit appeared before game selection'
 assert js('document.querySelector(".choose-games").getBoundingClientRect().height>0')
def shot(name):
 assert not b('errors')['errors'],b('errors');assert js('document.documentElement.scrollWidth<=innerWidth+1'),'page overflow'
 assert js('(()=>{const m=document.querySelector(".modal");return !m||m.scrollWidth<=m.clientWidth+1})()'),'modal overflow'
 colors=js(r"""(()=>{const rgb=s=>(s.match(/[\d.]+/g)||[]).map(Number),lum=c=>c.slice(0,3).map(v=>v/255).map(v=>v<=.04045?v/12.92:((v+.055)/1.055)**2.4).reduce((s,v,i)=>s+v*[.2126,.7152,.0722][i],0);return [...document.querySelectorAll('.create-selected-game strong,.create-selected-game span,.create-selected-game p,.create-back')].filter(e=>e.getClientRects().length).map(e=>{let bg=[255,255,255],chain=[];for(let n=e;n;n=n.parentElement)chain.unshift(n);for(const n of chain){const c=rgb(getComputedStyle(n).backgroundColor),a=c[3]??1;bg=bg.map((v,i)=>c[i]*a+v*(1-a))};const a=lum(rgb(getComputedStyle(e).color)),b=lum(bg);return {text:e.textContent.trim(),ratio:(Math.max(a,b)+.05)/(Math.min(a,b)+.05)}})})()""")
 assert all(c['ratio']>=4.5 for c in colors),colors
 (out/(name+'-contrast.json')).write_text(json.dumps(colors,ensure_ascii=False,indent=2))
 b('screenshot',str(out/(name+'.png')),'--full');report['checks'].append(name);print('PASS',name,flush=True)
try:
 run(['npm','--prefix','web','run','build'],'build.log');run(['go','build','-o',str(out/'wire-board'),'.'],'go-build.log')
 data_dir=tempfile.TemporaryDirectory(dir=out,prefix='data-');data=data_dir.name
 env['DATA_DIR']=data
 with (out/'server.log').open('w') as f:server=subprocess.Popen([str(out/'wire-board')],cwd=root,env=env,stdout=f,stderr=subprocess.STDOUT)
 base=None
 for _ in range(200):
  match=re.search(r'listening on (127\.0\.0\.1:\d+)',(out/'server.log').read_text())
  if match:base='http://'+match[1];break
  if server.poll() is not None:raise RuntimeError('server exited')
  time.sleep(.05)
 assert base;b('open',base);opened=True;b('tab',next(t['tabId'] for t in b('tab')['tabs'] if t['active']));api('/login',{'name':env['ADMIN_USERNAME'],'password':env['ADMIN_PASSWORD']})
 games=api('/admin')['games'];assert len(games)>=19
 for g in games:api('/admin/games/'+g['kind'],{'enabled':g['kind']!='racing'})
 b('reload');b('wait','.lobby-create');first();b('fill','input[aria-label=搜索可选游戏]','racing');assert js('document.querySelectorAll(".choose-games button").length===0');b('press','Enter');assert api('/state').get('room') is None;shot('unpublished-excluded');b('click','.modal-head button[aria-label=关闭]')
 api('/admin/games/racing',{'enabled':True});b('reload');b('wait','.lobby-create');first();choose('weiqi','围棋',keyboard=True);b('fill','input[name=name]','返回时保留的房间');b('fill','input[name=password]','test-draft-password');b('select','select[aria-label=围棋路数]','9');click('返回选择游戏')
 assert js('document.querySelector("input[aria-label=搜索可选游戏]").value==="weiqi"');b('press','Tab');assert js('document.activeElement.closest("[hidden]")===null'),'hidden step stole keyboard focus';click('围棋');b('wait','[data-create-step=setup]:not([hidden])');assert js('document.querySelector("select[aria-label=围棋路数]").value==="9"');click('返回选择游戏');choose('chess','国际象棋');assert js('document.querySelector("input[name=name]").value==="返回时保留的房间"&&document.querySelector("input[name=password]").value==="test-draft-password"');assert js('document.querySelector("select[aria-label=围棋路数]")===null');shot('back-preserves-common-draft');b('click','.modal-head button[aria-label=关闭]')
 # The selected game can be withdrawn while its settings are open.
 first();choose('weiqi','围棋');api('/admin/games/weiqi',{'enabled':False});b('wait','[data-create-step=choose]:not([hidden])');b('fill','input[aria-label=搜索可选游戏]','weiqi');assert js('document.querySelectorAll(".choose-games button").length===0');shot('withdraw-selected-game');b('click','.modal-head button[aria-label=关闭]');api('/admin/games/weiqi',{'enabled':True});b('reload');b('wait','.lobby-create')
 for width in [1280,320]:
  b('set','viewport',str(width),'900' if width==1280 else '740')
  for g in games:
   first();choose(g['kind'],g['name']);assert js('document.querySelector("input[name=password]").value===""'),'cancelled password leaked into new dialog'
   capacities=js('[...document.querySelector("select[aria-label=座位上限]").options].filter(o=>!o.disabled).map(o=>Number(o.value))');capacity=max(capacities);b('select','select[aria-label=座位上限]',str(capacity));name=str(width)+'-'+g['kind'];b('fill','input[name=name]',name);b('fill','input[name=password]','create-test-only')
   if g['kind']=='weiqi':b('select','select[aria-label=围棋路数]','9');b('select','select[aria-label=围棋贴目]','0')
   shot(f'{width}-{g["kind"]}-settings');b('scrollintoview','form button[type=submit]');b('click','form button[type=submit]');b('wait','.waiting-layout');r=api('/state')['room'];assert r['kind']==g['kind'] and r['capacity']==capacity and r['name']==name and r['locked'],r
   if g['kind']=='weiqi':assert r['options']['size']==9 and r['options']['komi']==0,r['options']
   report['rooms'].append({'width':width,'kind':r['kind'],'capacity':r['capacity'],'name':r['name'],'locked':r['locked'],'options':r.get('options')});api('/rooms/'+r['id'],{'type':'leave','version':r['version'],'nonce':uuid.uuid4().hex});b('wait','.lobby-create')
 report['passed']=True
except Exception as e:
 report['error']=repr(e)
 if opened:
  try:report['snapshot']=b('snapshot','-i');b('screenshot',str(out/'failure.png'),'--full')
  except Exception:pass
 raise
finally:
 if opened:
  try:b('close')
  except Exception:pass
 if server:server.terminate();server.wait(timeout=15)
 if data_dir:data_dir.cleanup()
 (out/'result.json').write_text(json.dumps(report,ensure_ascii=False,indent=2)+'\n')
