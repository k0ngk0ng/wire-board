"""Final actual app screenshots, illustrated choices and measured alignment."""
from pathlib import Path
import argparse, hashlib, json, os, subprocess
ROOT=Path(__file__).resolve().parents[2]
p=argparse.ArgumentParser(description=__doc__);p.add_argument('--label',required=True);args=p.parse_args()
OUT=ROOT/'games/yuyuhakusho/verification/review-polish'/args.label;OUT.mkdir(parents=True,exist_ok=False)
TMP=ROOT/'.local/md-yuyuhakusho/interface-qa'/args.label;TMP.mkdir(parents=True,exist_ok=False)
env=dict(os.environ,AGENT_BROWSER_SOCKET_DIR=str((ROOT.parents[1]if ROOT.parent.name=='worktrees'else ROOT)/'.local/au'),AGENT_BROWSER_SESSION='if'+hashlib.sha256(args.label.encode()).hexdigest()[:4])
def b(*a):
 r=subprocess.run(['agent-browser',*a,'--json'],cwd=ROOT,env=env,text=True,capture_output=True,timeout=50);assert r.returncode==0,r.stdout+r.stderr;return json.loads(r.stdout)['data']
def js(s):return b('eval',s)['result']
def wait(s):return b('wait','--fn',s)
def click(name):return b('find','role','button','click','--name',name,'--exact')
report={'pass':False};host=None
try:
 host=subprocess.Popen([str(ROOT/'.local/md-yuyuhakusho/review-polish-final/host'),'-build',str(ROOT/'.local/md-yuyuhakusho/review-polish-final/dist'),'-data',str(TMP/'database')],cwd=ROOT,env=env,stdin=subprocess.PIPE,stdout=subprocess.PIPE,stderr=(OUT/'host.log').open('w'),text=True)
 urls=json.loads(host.stdout.readline());b('--profile',str(TMP/'browser'),'open',urls['direct']);b('set','viewport','1100','1000');click('创建账号')
 for name,value in [('name','图示验收'+args.label[-6:]),('password','LocalVisualOnly01'),('invite','yuyu-local-e2e')]:b('fill','input[name='+name+']',value)
 click('创建账号，入座');wait('!document.querySelector("input[name=password]")');click('创建房间');b('find','role','button','click','--name','魔强统一战')
 wait('!!document.querySelector("[aria-label=房间设置] input[name=name]")');b('click','[data-arena-picker] summary');wait('document.querySelectorAll("[data-arena-option]").length===8');b('click','[data-arena-option="2"]')
 b('fill','input[name=name]','图示与对齐验收');b('click','[aria-label=房间设置] form button.primary');wait('!!document.querySelector("[aria-label=比赛准备]")')
 for width,height,label in [(1100,1000,'desktop'),(320,740,'mobile')]:
  b('set','viewport',str(width),str(height));b('eval','scrollTo(0,0);true')
  geometry=js('(()=>{const a=document.querySelector("[data-arena-field]>span").getBoundingClientRect(),c=document.querySelector("[data-arena-picker] summary").getBoundingClientRect(),d=document.querySelector("[data-character-seat] summary").getBoundingClientRect(),n=document.querySelector(".yuyu-role>span").getBoundingClientRect();return {captionDelta:Math.abs(a.left-n.left),controlDelta:Math.abs(c.left-d.left),overflow:document.documentElement.scrollWidth>innerWidth}})()')
  assert geometry['captionDelta']<=1 and geometry['controlDelta']<=1 and not geometry['overflow'],geometry
  report[label]=geometry;b('screenshot',str(OUT/(label+'-waiting.png')))
  b('click','[data-arena-picker] summary');wait('document.querySelectorAll("[data-arena-option]").length===8');wait('[...document.querySelectorAll("[data-arena-option] img")].every(i=>i.complete&&i.naturalWidth===1280)')
  assert js('(()=>{const e=document.querySelector("[data-arena-option]"),r=e.getBoundingClientRect();return e.contains(document.elementFromPoint(r.left+r.width/2,r.top+r.height/2))})()')
  b('screenshot',str(OUT/(label+'-arenas.png')));b('press','Escape')
  b('click','[data-character-seat="0"] summary');wait('document.querySelectorAll("[data-character-option]").length===11');b('screenshot',str(OUT/(label+'-characters.png')));b('click','[data-character-option="3"]')
  wait('document.querySelector("[data-character-seat]").dataset.character==="3"')
 click('控制器设置');wait('!!document.querySelector("[data-controller-layout=md]")');wait('[...document.querySelector("[role=dialog]").getAnimations()].every(a=>a.playState==="finished")')
 keys=js('Object.fromEntries([...document.querySelectorAll("[data-controller-button]")].map(e=>[e.dataset.controllerButton,e.querySelector("kbd")?.textContent]))');assert keys['a']=='J'and keys['b']=='K',keys
 b('screenshot',str(OUT/'mobile-controller.png'));click('关闭')
 backend=js('fetch("/api/state").then(r=>r.json()).then(s=>({arena:s.room.options.arena,characters:s.room.options.characters}))');assert backend['arena']==2 and backend['characters'][0]==3,backend
 report.update(pass_=True,keyboard=keys,actual_room=backend);report['pass']=report.pop('pass_')
except Exception as e:
 report['error']=repr(e)
 try:b('screenshot',str(OUT/'failure.png'))
 except Exception:pass
 raise
finally:
 report['checker_sha256']=hashlib.sha256(Path(__file__).read_bytes()).hexdigest();(OUT/'result.json').write_text(json.dumps(report,ensure_ascii=False,indent=2)+'\n')
 try:b('close')
 except Exception:pass
 if host:host.terminate();host.wait(timeout=10)
print(json.dumps({'pass':True,'alignment':report.get('mobile')}))
