#!/usr/bin/env python3
"""compact-ui-failures.md: production UI and real HTTP room; native mouse/touch.
Chromium iPad/phone viewport emulation, not a physical device or Safari.
"""
import argparse,json,os,shutil,sys,traceback
from pathlib import Path
import accept_combat_controls_ui as c
p=argparse.ArgumentParser();p.add_argument('--baseline',action='store_true');p.add_argument('--web');a=p.parse_args()
w=c.w;w.OUT=c.ROOT/'.local/starfort-stall'/('toolbar-before'if a.baseline else'toolbar-after');w.OUT.mkdir(parents=True,exist_ok=True)
(w.OUT/'tmp').mkdir(exist_ok=True)
w.ENV.update(WIRE_BOARD_STARFORT_BROWSER_OUT=str(w.OUT),WIRE_BOARD_STARFORT_BROWSER_WEB=str(Path(a.web).resolve()if a.web else c.ROOT/'.local/starfort-stall/ui-dist'),AGENT_BROWSER_PROFILE=str(w.OUT/'profile'),GOTMPDIR=str(w.OUT/'tmp'),TMPDIR=str(w.OUT/'tmp'))
checks=[];passed=False

def check(name,ok,evidence=None):
 checks.append({'name':name,'passed':bool(ok),'evidence':evidence});print(('PASS 'if ok else'FAIL ')+name,flush=True)
 if not a.baseline:assert ok,(name,evidence)
def enter(touch):
 if not c.js('!!document.querySelector("[data-fullscreen-enter]")'):c.b('click','.compact-room-tools')
 c.wait('!!document.querySelector("[data-fullscreen-enter]")&&!document.querySelector("[data-fullscreen-enter]").disabled')
 c.js('document.querySelector("[data-fullscreen-enter]").scrollIntoView({block:"center",behavior:"instant"})')
 settled=c.js('new Promise(resolve=>{let prior="",n=0,frames=0;function frame(){const e=document.querySelector("[data-fullscreen-enter]"),r=e.getBoundingClientRect(),key=JSON.stringify([r.x,r.y,r.width]);if(r.y<0||r.bottom>innerHeight||!e.contains(document.elementFromPoint(r.x+r.width/2,r.y+r.height/2)))e.scrollIntoView({block:"center",behavior:"instant"});n=key===prior&&r.y>=0&&r.bottom<=innerHeight&&e.contains(document.elementFromPoint(r.x+r.width/2,r.y+r.height/2))?n+1:0;prior=key;if(n>=12||++frames>=180)resolve(n>=12);else requestAnimationFrame(frame)}frame()})')
 check('fullscreen entry is settled and hittable',settled)
 c.activate('[data-fullscreen-enter]',touch);c.wait('!!document.querySelector("[data-room-fullscreen]")')
def geometry():
 return c.js('''(()=>{const board=document.querySelector('.starfort-board').getBoundingClientRect(),hud=document.querySelector('.sf-hud').getBoundingClientRect(),selectors=['[data-sf-ready]','[data-sf-pause]','[data-sf-resume]','[data-sf-open-choice]','.sf-select-pad','[data-sf-use-medical]','[data-sf-open-shop]','[data-sf-open-tasks]','[data-sf-open-info]'];const buttons=selectors.flatMap(selector=>{const e=document.querySelector(selector);if(!e)return [];const r=e.getBoundingClientRect();return [{selector,rect:r.toJSON(),label:e.getAttribute('aria-label'),title:e.title,text:e.textContent.trim(),svg:!!e.querySelector('svg'),hit:e.contains(document.elementFromPoint(r.x+r.width/2,r.y+r.height/2)),disabled:e.disabled}]});return {board:board.toJSON(),hud:hud.toJSON(),buttons,overflow:document.documentElement.scrollWidth>innerWidth+1,legacyMap:!!document.querySelector('[data-sf-zoom-in],[data-sf-zoom-out],[data-sf-camera-base],[data-sf-fit],[data-sf-entry]'),calmText:document.body.innerText.includes('无特殊天气')}})()''')
def overlap(x,y):return min(x['right'],y['right'])-max(x['left'],y['left'])>1 and min(x['bottom'],y['bottom'])-max(x['top'],y['top'])>1
try:
 c.BASELINE=True;c.setup();c.BASELINE=a.baseline
 sizes=[(1366,900),(320,740)]if a.baseline else[(1366,900),(1024,768),(900,800),(768,1024),(320,740)]
 for width,height in sizes:
  touch=width<=1024;c.b('set','viewport',width,height);w.scenario('weather-toolbar',2)
  c.wait('document.querySelector(".room-page h1")?.textContent.includes("weather-toolbar")&&document.querySelector(".sf-battlefield")?.dataset.assetsReady==="true"')
  c.stable()
  if c.js('!!document.querySelector(".sf-shop-dialog")'):c.b('click','.sf-shop-dialog [aria-label="关闭"]');c.wait('!document.querySelector(".sf-shop-dialog")')
  for fullscreen in [False,True]:
   name=f'{width}-'+('full'if fullscreen else'page')
   if fullscreen:enter(touch)
   else:c.b('scrollintoview','.starfort-board');c.stable()
   g=geometry();w.artifact(name+'-geometry.json',g)
   pairs=[]
   for i,button in enumerate(g['buttons']):
    for other in g['buttons'][i+1:]:
     if overlap(button['rect'],other['rect']):pairs.append([button['selector'],other['selector']])
    if overlap(button['rect'],g['hud']):pairs.append([button['selector'],'hud'])
   check(name+' no overlap or horizontal overflow',not pairs and not g['overflow'],{'overlap':pairs,'geometry':g})
   check(name+' icon buttons have accessible names',all(b['svg']and b['label']and b['title']and (not b['text']or b['text'].isdigit())for b in g['buttons']),g['buttons'])
   check(name+' controls inside field',all(b['rect']['left']>=g['board']['left']-1 and b['rect']['right']<=g['board']['right']+1 and b['rect']['top']>=g['board']['top'] and b['rect']['bottom']<=g['board']['top']+150 for b in g['buttons']),g)
   check(name+' calm label and old map buttons removed',not g['calmText']and not g['legacyMap'])
   c.screenshot(name)
   if not a.baseline and fullscreen:
    c.activate('[data-sf-ready]',touch);c.wait('document.querySelector("[data-sf-ready]")?.getAttribute("aria-pressed")==="true"');c.activate('[data-sf-ready]',touch)
    c.activate('[data-sf-use-medical]',touch);c.wait('document.querySelector("[data-sf-base]")?.textContent==="23"');check(name+' medical action/count preserved',w.data()['controls']['medical']['count']==1)
    for selector,dialog in [('.sf-select-pad','.sf-command-dialog'),('[data-sf-open-shop]','.sf-shop-dialog'),('[data-sf-open-tasks]','.sf-tasks-dialog'),('[data-sf-open-info]','.sf-info-dialog')]:
     c.activate(selector,touch);c.wait('!!document.querySelector('+json.dumps(dialog)+')');check(name+' '+dialog+' visible',c.visibility(dialog)['visible']);check(name+' no redundant calm text in dialog',not c.js('document.body.innerText.includes("无特殊天气")'));c.b('click',dialog+' [aria-label="关闭"]')
    center=c.js('(()=>{const r=document.querySelector("[data-starfort-canvas]").getBoundingClientRect();return {x:r.x+r.width*.65,y:r.y+r.height*.6,zoom:+document.querySelector("[data-starfort-canvas]").dataset.zoom}})()')
    c.b('mouse','move',center['x'],center['y']);c.b('mouse','wheel',-250);c.wait('+document.querySelector("[data-starfort-canvas]").dataset.zoom>'+str(center['zoom'])+'+.001');check(name+' native wheel zoom preserved',True)
    if touch:
     before=c.js('+document.querySelector("[data-starfort-canvas]").dataset.zoom');c.native('pinch',center['x'],center['y'],center['x'],center['y']);c.wait('+document.querySelector("[data-starfort-canvas]").dataset.zoom>'+str(before)+'+.001');check(name+' native pinch zoom preserved',True)
    check(name+' gestures do not build towers',not w.data()['towers']and not c.js('!!document.querySelector(".sf-command-dialog")'))
   if fullscreen and not a.baseline:
    c.activate('[data-sf-pause]',touch);w.PEERS[0].action('pause');c.wait('!!document.querySelector("[data-sf-resume]")')
    g=geometry();check(name+' paused icon remains usable',all(b['svg']and b['label']and b['title']and b['hit']for b in g['buttons']))
    c.activate('[data-sf-resume]',touch);c.wait('document.querySelector("[data-sf-resume]")?.getAttribute("aria-pressed")==="true"');w.PEERS[0].action('resume');c.wait('!!document.querySelector("[data-sf-ready]")');check(name+' native pause/resume preserved',True)
   if fullscreen:c.leave()
 if not a.baseline:
  for width,height in [(1366,900),(320,740)]:
   touch=width==320;c.b('set','viewport',width,height);w.scenario('growth-vote',2)
   c.wait('document.querySelector(".room-page h1")?.textContent.includes("growth-vote")&&!!document.querySelector("[data-sf-open-choice]")')
   c.wait('!!document.querySelector(".sf-choice-dialog")');c.b('click','.sf-choice-dialog [aria-label="关闭"]');enter(touch)
   g=geometry();check(str(width)+' choosing icon controls fit',all(b['svg']and b['label']and b['title']and b['hit']for b in g['buttons']))
   c.activate('[data-sf-open-choice]',touch);c.wait('!!document.querySelector(".sf-choice-dialog")');check(str(width)+' choosing icon opens voting',c.visibility('.sf-choice-dialog')['visible']);c.screenshot('choosing-'+str(width));c.b('click','.sf-choice-dialog [aria-label="关闭"]');c.leave()
 check('no browser errors',not c.b('errors')['errors']);passed=not any(not x['passed']for x in checks)
except Exception as error:
 traceback.print_exc()
 try:c.screenshot('failure')
 except Exception:pass
 w.artifact('failure.json',{'error':str(error)})
finally:
 if w.OPENED:
  try:c.b('close')
  except Exception:pass
 if w.SERVER:(w.OUT/'stop').write_text('stop');w.SERVER.wait(timeout=20)
 w.artifact('result.json',{'passed':passed,'baseline':a.baseline,'checks':checks,'sources':c.source_hashes(),'scope':__doc__})
 for path in [w.OUT/'profile',w.OUT/'tmp',w.PROFILE]:
  if path.exists():shutil.rmtree(path)
 raise SystemExit(0 if passed or a.baseline else 1)
