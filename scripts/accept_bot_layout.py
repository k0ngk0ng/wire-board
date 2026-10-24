#!/usr/bin/env python3
"""Before implementation: bot labels force card/page overflow, long names grow
waiting rows/player strips, AI badge shrinks/disappears, removal remains offscreen,
and truncation hides public scores/cash or the accessible full identity.
"""
from pathlib import Path
import argparse,json,os,re,shutil,subprocess,time,uuid
ROOT=Path(__file__).resolve().parents[1]
p=argparse.ArgumentParser();p.add_argument('--baseline',action='store_true');a=p.parse_args()
OUT=ROOT/'.local/bot-layout'/('baseline' if a.baseline else 'verified');OUT.mkdir(parents=True,exist_ok=True)
env=dict(os.environ,AGENT_BROWSER_SESSION='bot-layout-'+uuid.uuid4().hex[:6],AGENT_BROWSER_SOCKET_DIR=str(ROOT/'.local/cs'),AGENT_BROWSER_PROFILE=str(OUT/'profile'),TMPDIR=str(ROOT/'.local/tmp'))
for path in [ROOT/'.local/cs',ROOT/'.local/tmp']:path.mkdir(parents=True,exist_ok=True)
config=OUT/'vite.config.mjs';config.write_text('export default '+json.dumps({'root':str(ROOT),'publicDir':False,'cacheDir':str(OUT/'vite-cache'),'esbuild':{'jsx':'automatic'},'resolve':{'alias':{n:str(ROOT/'web/node_modules'/n) for n in ['react','react-dom','lucide-react','three']}},'server':{'host':'127.0.0.1','port':0,'fs':{'allow':[str(ROOT)]}}})+';\n')
(OUT/'index.html').write_text('<!doctype html><html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>电脑玩家窄屏验收</title></head><body><div id="root"></div><script type="module" src="/scripts/bot_layout_harness.tsx"></script></body></html>')
report={'passed':False,'scope':'production ModuleWaiting/LegacyWaiting/ModulePlayers/LegacyPlayers, native add/remove controls; fixture state/command sink','checks':[],'screens':[]};server=None;opened=False
def b(*args):
 r=subprocess.run(['agent-browser',*args,'--json'],cwd=ROOT,env=env,capture_output=True,text=True,timeout=45)
 if r.returncode:raise RuntimeError(r.stdout+r.stderr)
 v=json.loads(r.stdout);assert v['success'],v;return v['data']
def js(code):return b('eval',code)['result']
def check(name,value):
 report['checks'].append({'name':name,'passed':bool(value)})
 if not a.baseline:assert value,name
def shot(label):
 v=js('''(()=>{const cards=[...document.querySelectorAll('.waiting-seat,.player-panel')];return {viewport:[innerWidth,innerHeight],pageWidth:document.documentElement.scrollWidth,cards:cards.map(c=>{const r=c.getBoundingClientRect(),h=c.querySelector('h3,.player-info>strong'),hr=h.getBoundingClientRect();return {left:r.left,right:r.right,width:r.width,height:r.height,nameHeight:hr.height,headingWithinCard:hr.left>=r.left-1&&hr.right<=r.right+1,scrollOverflow:c.scrollWidth>c.clientWidth+1,name:h.textContent.trim(),remove:c.querySelector('.bot-remove')?(()=>{const b=c.querySelector('.bot-remove').getBoundingClientRect();return b.left>=r.left&&b.right<=r.right})():null}})}})()''')
 v['label']=label;report['screens'].append(v);b('screenshot',str(OUT/(label+'.png')),'--full');return v
try:
 with (OUT/'server.log').open('w') as log:server=subprocess.Popen([str(ROOT/'web/node_modules/.bin/vite'),'--config',str(config)],cwd=ROOT,env=env,stdout=log,stderr=subprocess.STDOUT)
 base=None
 for _ in range(200):
  match=re.search(r'http://127\.0\.0\.1:\d+',(OUT/'server.log').read_text())
  if match:base=match[0];break
  if server.poll() is not None:raise RuntimeError((OUT/'server.log').read_text())
  time.sleep(.05)
 assert base
 b('open',base+'/'+str((OUT/'index.html').relative_to(ROOT)));opened=True;b('wait','.waiting-seat')
 for width in [320,375,1280]:
  b('set','viewport',str(width),'900')
  for mode in ['module-waiting','legacy-waiting']:
   b('select','[aria-label="验收界面"]',mode);b('wait','.waiting-seat')
   for _ in range(3-js('document.querySelectorAll(".bot-remove").length')):b('find','role','button','click','--name','添加电脑玩家','--exact')
   normal=shot(f'{width}-{mode}-normal')
   b('find','role','button','click','--name','长名称','--exact');long=shot(f'{width}-{mode}-long')
   check(f'{width} {mode} page does not overflow',long['pageWidth']<=width and all(c['left']>=0 and c['right']<=width for c in long['cards']))
   check(f'{width} {mode} names fit their cards',all(c['headingWithinCard'] and not c['scrollOverflow'] and c['nameHeight']<=22 for c in long['cards']))
   check(f'{width} {mode} long names preserve row geometry',all(abs(c['width']-n['width'])<=1 and abs(c['height']-n['height'])<=1 for c,n in zip(long['cards'],normal['cards'])))
   check(f'{width} {mode} full identity remains accessible',js('[...document.querySelectorAll(".waiting-seat h3")].some(e=>e.textContent.includes("ComputerOpponentWithAnExtremelyLongUnbrokenName12345"))'))
   b('find','role','button','click','--name','移除用于验证窄屏布局的超长电脑玩家名称二号','--exact');removed=shot(f'{width}-{mode}-removed')
   check(f'{width} {mode} remove keeps page bounded',removed['pageWidth']<=width and js('document.querySelectorAll(".bot-remove").length===2'))
   b('find','role','button','click','--name','普通名称','--exact')
  for mode in ['module-playing','legacy-playing']:
   b('select','[aria-label="验收界面"]',mode);b('wait','.player-panel');normal=shot(f'{width}-{mode}-normal')
   b('find','role','button','click','--name','长名称','--exact');long=shot(f'{width}-{mode}-long')
   check(f'{width} {mode} page remains bounded',long['pageWidth']<=width)
   check(f'{width} {mode} name length does not grow panels',all(abs(c['height']-n['height'])<=1 and not c['scrollOverflow'] and c['nameHeight']<=22 for c,n in zip(long['cards'],normal['cards'])))
   if mode.startswith('module'):
    check(f'{width} {mode} all cash including zero is actually visible',js('document.querySelectorAll("[data-mono-cash]").length===3 && document.querySelectorAll("[data-mono-cash]")[1].textContent.trim()==="M 0" && [...document.querySelectorAll("[data-mono-cash]")].every(e=>{const r=e.getBoundingClientRect();return r.width>0&&r.left>=0&&r.right<=innerWidth&&r.top>=0&&r.bottom<=innerHeight&&getComputedStyle(e).visibility!=="hidden"})'))
   else:
    b('scroll','left','10000','--selector','.players-strip')
    for seat in range(3):
     if seat:b('scroll','right','175','--selector','.players-strip')
     check(f'{width} {mode} score for seat {seat} is reachable without clipping',js('(()=>{const e=document.querySelectorAll("[data-player-seat]")['+str(seat)+'].querySelector(".player-score"),r=e.getBoundingClientRect(),p=e.closest(".player-panel").getBoundingClientRect(),s=e.closest(".players-strip").getBoundingClientRect();return r.width>0&&r.left>=Math.max(p.left,s.left)-1&&r.right<=Math.min(p.right,s.right)+1&&r.top>=p.top&&r.bottom<=p.bottom})()'))
    b('screenshot',str(OUT/f'{width}-{mode}-last-seat.png'))
   b('find','role','button','click','--name','普通名称','--exact')
 report['passed']=all(c['passed'] for c in report['checks'])
 report['limitation']='Legacy player strip retains its pre-existing horizontal scrolling; verified each score can be scrolled into view. This is not a claim that every legacy seat fits simultaneously.'
except Exception as error:
 report['error']=str(error)
 if opened:report['snapshot']=b('snapshot','-i')
 raise
finally:
 (OUT/'result.json').write_text(json.dumps(report,ensure_ascii=False,indent=2)+'\n')
 if opened:b('close')
 if server:server.terminate();server.wait(timeout=15)
 shutil.rmtree(OUT/'profile',ignore_errors=True);shutil.rmtree(OUT/'vite-cache',ignore_errors=True)
 config.unlink(missing_ok=True);(OUT/'index.html').unlink(missing_ok=True)
 print(OUT/'result.json')
