#!/usr/bin/env python3
"""Responsive creation-dialog check, specified before implementation.

Failure cases: desktop remains narrow; viewport growth exposes no additional
cards; cards become excessively wide; mobile overflows; short windows lose
the close/filter controls; scrolling cannot reach the last game; selecting or
returning changes the room draft; second-step form stretches across the screen.
"""
from pathlib import Path
import argparse,json,os,re,shutil,subprocess,time,uuid

ROOT=Path(__file__).resolve().parents[1]
parser=argparse.ArgumentParser();parser.add_argument('--baseline',action='store_true');args=parser.parse_args()
OUT=ROOT/'.local/create-size'/('baseline' if args.baseline else 'verified');OUT.mkdir(parents=True,exist_ok=True)
env=dict(os.environ,AGENT_BROWSER_SESSION='create-size-'+uuid.uuid4().hex[:6],AGENT_BROWSER_SOCKET_DIR=str(ROOT/'.local/cs'),AGENT_BROWSER_PROFILE=str(OUT/'profile'),TMPDIR=str(ROOT/'.local/tmp'))
for p in [ROOT/'.local/cs',ROOT/'.local/tmp']:p.mkdir(parents=True,exist_ok=True)
config=OUT/'vite.config.mjs'
aliases={name:str(ROOT/'web/node_modules'/name) for name in ['react','react-dom','lucide-react','three']}
if args.baseline:
 (OUT/'baseline-style.css').write_bytes(subprocess.check_output(['git','show','bae92cd4:web/src/style.css'],cwd=ROOT))
 aliases['../web/src/style.css']=str(OUT/'baseline-style.css')
config.write_text('export default '+json.dumps({'root':str(ROOT),'publicDir':False,'cacheDir':str(OUT/'vite-cache'),'esbuild':{'jsx':'automatic'},'resolve':{'alias':aliases},'server':{'host':'127.0.0.1','port':0,'fs':{'allow':[str(ROOT)]}}})+';\n')
(OUT/'index.html').write_text('<!doctype html><html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>创建房间尺寸验收</title></head><body><div id="root"></div><script type="module" src="/scripts/create_size_harness.tsx"></script></body></html>')
report={'passed':False,'scope':'actual shared CreateRoom, catalogue covers and ModuleCreate; fixture outer lobby/submit sink','checks':[],'sizes':[]};server=None;opened=False
def b(*args):
 p=subprocess.run(['agent-browser',*args,'--json'],cwd=ROOT,env=env,capture_output=True,text=True,timeout=45)
 if p.returncode:raise RuntimeError(p.stdout+p.stderr)
 value=json.loads(p.stdout);assert value['success'],value;return value['data']
def js(code):return b('eval',code)['result']
def expect(name,value):
 if not args.baseline:assert value,name
 report['checks'].append({'name':name,'passed':bool(value)})
def measure(label):
 result=js('''(()=>{const m=document.querySelector('.create-room-modal'),g=document.querySelector('.choose-games'),r=m.getBoundingClientRect(),v=g.getBoundingClientRect();const cards=[...g.children].map(e=>e.getBoundingClientRect());const visible=cards.filter(c=>c.top>=v.top-1&&c.bottom<=Math.min(v.bottom,innerHeight)+1);return {viewport:[innerWidth,innerHeight],modal:{x:r.x,y:r.y,width:r.width,height:r.height,bottom:r.bottom},grid:{width:v.width,height:v.height,scrollHeight:g.scrollHeight,clientHeight:g.clientHeight,columns:getComputedStyle(g).gridTemplateColumns.split(' ').length},cardWidth:cards[0]?.width,visibleCards:visible.length,totalCards:cards.length,horizontalOverflow:m.scrollWidth>m.clientWidth+1||document.documentElement.scrollWidth>innerWidth+1,closeVisible:(()=>{const b=m.querySelector('[aria-label="关闭"]').getBoundingClientRect();return b.top>=0&&b.bottom<=innerHeight})()}})()''')
 result['clippedCaptions']=js('(()=>{const g=document.querySelector(".choose-games");return [...g.children].filter(b=>{const t=b.querySelector(":scope > span").getBoundingClientRect(),r=b.getBoundingClientRect();return t.bottom>r.bottom+1||t.top<r.top-1}).length})()')
 result['label']=label;report['sizes'].append(result);b('screenshot',str(OUT/(label+'.png')));return result
try:
 with (OUT/'server.log').open('w') as log:server=subprocess.Popen([str(ROOT/'web/node_modules/.bin/vite'),'--config',str(config)],cwd=ROOT,env=env,stdout=log,stderr=subprocess.STDOUT)
 base=None
 for _ in range(200):
  match=re.search(r'http://127\.0\.0\.1:\d+',(OUT/'server.log').read_text())
  if match:base=match[0];break
  if server.poll() is not None:raise RuntimeError((OUT/'server.log').read_text())
  time.sleep(.05)
 assert base,'Vite did not start'
 page=base+'/'+str((OUT/'index.html').relative_to(ROOT))
 b('open',page);opened=True;b('wait','button.primary');b('set','viewport','1280','900');b('click','button.primary');b('wait','.choose-games button')
 desktop=measure('1280-choose')
 expect('desktop cards retain fully visible name captions',desktop['clippedCaptions']==0)
 expect('1280px selection uses at least 1000px width and five columns',desktop['modal']['width']>=1000 and desktop['grid']['columns']>=5)
 expect('1280px shows at least fifteen whole cards',desktop['visibleCards']>=15)
 b('set','viewport','1920','1080');large=measure('1920-choose')
 expect('larger window increases dialog and visible game capacity',large['modal']['width']>desktop['modal']['width'] and large['grid']['columns']>desktop['grid']['columns'] and large['visibleCards']>=24)
 expect('desktop card widths remain readable and bounded',all(160<=r['cardWidth']<=240 for r in [desktop,large]))
 b('set','viewport','1280','560');short=measure('1280-short')
 expect('short desktop keeps dialog and close control inside viewport',short['modal']['y']>=0 and short['modal']['bottom']<=560 and short['closeVisible'])
 b('set','viewport','320','740');mobile=measure('320-choose')
 expect('mobile keeps two columns and fits the screen',mobile['grid']['columns']==2 and not mobile['horizontalOverflow'] and mobile['modal']['width']<=320)
 js('document.querySelector(".choose-games").scrollTop=document.querySelector(".choose-games").scrollHeight')
 expect('last game stays reachable in the scroll area',js('(()=>{const g=document.querySelector(".choose-games"),c=g.lastElementChild.getBoundingClientRect(),r=g.getBoundingClientRect();return c.top>=r.top-1&&c.bottom<=r.bottom+1})()'))
 b('fill','input[aria-label="搜索可选游戏"]','chess');b('click','.choose-games button');b('wait','input[name="name"]');b('fill','input[name="name"]','窗口变化保留的房间')
 b('set','viewport','1920','1080')
 expect('settings retain readable form width and draft',js('document.querySelector(".create-room-modal").getBoundingClientRect().width<=620&&document.querySelector("input[name=name]").value==="窗口变化保留的房间"'))
 b('screenshot',str(OUT/'1920-settings.png'));b('click','.create-back');b('fill','input[aria-label="搜索可选游戏"]',' ')
 report['returnState']=js('({query:document.querySelector("input[aria-label=搜索可选游戏]").value,category:document.querySelector("select[aria-label=可选游戏分类]").value,cards:document.querySelectorAll(".choose-games button").length,focus:document.activeElement.outerHTML})')
 b('wait','--fn','document.querySelectorAll(".choose-games button").length==='+str(large['totalCards']));back=measure('1920-return')
 expect('return restores wide catalogue',back['modal']['width']==large['modal']['width'])
 b('fill','input[aria-label="搜索可选游戏"]','not-a-game-match');expect('empty search is visible without overflow',js('!!document.querySelector(".game-search-empty")&&document.querySelector(".create-room-modal").scrollWidth<=document.querySelector(".create-room-modal").clientWidth+1'))
 b('press','Escape');expect('Escape closes the shared dialog',js('!document.querySelector("[role=dialog]")'))
 expect('no horizontal overflow at any measured size',all(not r['horizontalOverflow'] for r in report['sizes']))
 report['passed']=all(c['passed'] for c in report['checks'])
except Exception as error:
 report['error']=str(error)
 if opened:
  report['failureState']=b('snapshot','-i');b('screenshot',str(OUT/'failure.png'))
 raise
finally:
 (OUT/'result.json').write_text(json.dumps(report,ensure_ascii=False,indent=2)+'\n')
 if opened:b('close')
 if server:server.terminate();server.wait(timeout=15)
 shutil.rmtree(OUT/'profile',ignore_errors=True);shutil.rmtree(OUT/'vite-cache',ignore_errors=True)
 config.unlink(missing_ok=True);(OUT/'index.html').unlink(missing_ok=True)
 (OUT/'baseline-style.css').unlink(missing_ok=True)
 print(OUT/'result.json')
