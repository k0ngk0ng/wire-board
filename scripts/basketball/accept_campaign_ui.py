"""Tests specified before campaign UI: entry/password intent, readable source
names/images, waiting and active permissions, same-index revision race, modal
close/reopen persistence, countdown and 320px native touch. Retained real server
views are presentation fixtures; this does not claim full browser gameplay.
"""
from pathlib import Path
import functools,http.server,json,os,subprocess,threading,uuid,sys
ROOT=Path(__file__).resolve().parents[2];OUT=ROOT/'.local/campaign-ui';OUT.mkdir(parents=True,exist_ok=True)
env=dict(os.environ,AGENT_BROWSER_SESSION='bcu-'+uuid.uuid4().hex[:5],AGENT_BROWSER_SOCKET_DIR=str((ROOT/'.local/basketball').resolve()/'bkn-sockets'),TMPDIR=str(ROOT/'.local/tmp'),NODE_PATH=str(ROOT/'web/node_modules'))
def browser(*args):
 p=subprocess.run(['agent-browser',*args,'--json'],env=env,capture_output=True,text=True,timeout=60)
 if p.returncode:raise RuntimeError(p.stdout+p.stderr)
 r=json.loads(p.stdout);assert r['success'],r;return r['data']
def js(code):return browser('eval',code)['result']
def wait(code):return browser('wait','--fn',code)
def touch(selector,page):
 browser('scrollintoview',selector)
 point=js('(()=>{const r=document.querySelector('+json.dumps(selector)+').getBoundingClientRect();return{x:r.x+r.width/2,y:r.y+r.height/2}})()')
 endpoint=browser('get','cdp-url')['cdpUrl']
 inspector=subprocess.Popen(['node',str(ROOT/'scripts/basketball/browser_cdp_session.mjs'),endpoint,page],env=env,stdin=subprocess.PIPE,stdout=subprocess.PIPE,stderr=(OUT/'touch.stderr').open('w'),text=True)
 try:
  assert json.loads(inspector.stdout.readline())['ready']
  for method,params in [('Emulation.setTouchEmulationEnabled',dict(enabled=True,maxTouchPoints=1)),('Input.dispatchTouchEvent',dict(type='touchStart',touchPoints=[dict(id=1,**point)])),('Input.dispatchTouchEvent',dict(type='touchEnd',touchPoints=[])),('Emulation.setTouchEmulationEnabled',dict(enabled=False))]:
   inspector.stdin.write(json.dumps(dict(method=method,params=params))+'\n');inspector.stdin.flush();r=json.loads(inspector.stdout.readline());assert 'error' not in r,r
 finally:
  inspector.stdin.close()
  try:inspector.wait(timeout=5)
  except subprocess.TimeoutExpired:inspector.terminate();inspector.wait(timeout=5)
checks=[]
def expect(name,ok):
 assert ok,name
 checks.append(name)
class Handler(http.server.SimpleHTTPRequestHandler):
 def translate_path(self,path):
  if path.startswith('/basketball/v1/selection/'):
   relative=Path(path.split('/selection/',1)[1]);assert '..' not in relative.parts
   return str(ROOT/'.local/basketball/selection-assets'/relative)
  return super().translate_path(path)
 def log_message(self,*args):pass

def main():
 (OUT/'result.json').write_text('{"status":"running"}\n')
 # Reproduce the actual server-bound views instead of maintaining invented gear.
 if '--reuse-fixtures' not in sys.argv:subprocess.run(['go','test','./games/basketball/server','-run','^TestCampaignEntryPreparationHTTP$|^TestCampaignLifecycleHTTP$|^TestHostCampaignPreparationCommitRestore$|^TestCampaignAttributesOwnedPreviewAndProjection$','-count=1'],cwd=ROOT,env=env,check=True)
 entry=json.loads((ROOT/'.local/poker/basketball/http/TestCampaignEntryPreparationHTTP.json').read_text())
 lifecycle=json.loads((ROOT/'.local/poker/basketball/http/TestCampaignLifecycleHTTP.json').read_text())
 waiting=next(t['views'][0] for t in entry['trace'] if t['views'][0]['status']=='waiting')
 reward=next(t['views'][0]['game']['data'] for t in lifecycle['trace'] if t['views'][0]['game']['data']['phase']=='campaign-reward')
 prepared=next(t['views'][0]['game']['data'] for t in lifecycle['trace'] if t['views'][0]['game']['data']['phase']=='campaign-prepare')
 half=json.loads((ROOT/'.local/campaign-host-preparation/host-views.json').read_text())['views'][1]
 (OUT/'fixtures.json').write_text(json.dumps(dict(waiting=waiting,reward=reward,prepared=prepared,halftime=half,attributeOptions=json.loads((ROOT/'.local/campaign-attributes/preview-options.json').read_text()),mixedAttributeOptions=json.loads((ROOT/'.local/campaign-attributes/mixed-preview-options.json').read_text()),largeAttributeOptions=json.loads((ROOT/'.local/campaign-attributes/large-preview-options.json').read_text()))))
 subprocess.run([str(ROOT/'web/node_modules/.bin/esbuild'),str(ROOT/'scripts/basketball/campaign_ui_harness.tsx'),'--bundle','--format=esm','--jsx=automatic','--loader:.svg=dataurl','--target=es2022','--outfile='+str(OUT/'ui.js')],cwd=ROOT,env=env,check=True)
 (OUT/'index.html').write_text('<!doctype html><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>战役准备验收</title><link rel="stylesheet" href="/ui.css"><style>.campaign-fixture{max-width:920px;margin:auto;padding:12px}.campaign-fixture h1{font-size:20px}</style><div id="root"></div><script type="module" src="/ui.js"></script>')
 server=http.server.ThreadingHTTPServer(('127.0.0.1',0),functools.partial(Handler,directory=str(OUT)));threading.Thread(target=server.serve_forever,daemon=True).start();opened=False
 try:
  page=f'http://127.0.0.1:{server.server_port}/'
  browser('--profile',str(OUT/'profile'),'open',page);opened=True
  wait('!!window.campaignFixture && !!document.querySelector(".basketball-campaign-editor")')
  wait('[...document.querySelectorAll(".basketball-campaign-editor img")].every(i=>i.complete&&i.naturalWidth>0)')
  expect('waiting uses original court/portrait images with local fallback',js('[...document.querySelectorAll(".basketball-campaign-editor img")].every(i=>i.currentSrc.includes("/basketball/v1/selection/"))'))
  contrast=js('(()=>{const rgb=s=>s.match(/[\\d.]+/g).slice(0,3).map(Number),lum=c=>c.map(n=>{n/=255;return n<=.04045?n/12.92:((n+.055)/1.055)**2.4}).reduce((s,n,i)=>s+n*[.2126,.7152,.0722][i],0);return [...document.querySelectorAll(".basketball-campaign-editor button,.basketball-campaign-editor p,.basketball-campaign-editor h3,.basketball-campaign-editor h4,.basketball-campaign-editor label,.basketball-campaign-grades th,.basketball-campaign-grades td,.basketball-campaign-grades caption")].filter(e=>e.textContent.trim()).map(e=>{let b=e;while(b&&getComputedStyle(b).backgroundColor==="rgba(0, 0, 0, 0)")b=b.parentElement;const fg=lum(rgb(getComputedStyle(e).color)),bg=lum(rgb(getComputedStyle(b).backgroundColor));return{text:e.textContent,ratio:(Math.max(fg,bg)+.05)/(Math.min(fg,bg)+.05)}})})()')
  expect('computed preparation text contrast is at least 4.5:1',all(row['ratio']>=4.5 for row in contrast))
  (OUT/'contrast.json').write_text(json.dumps(contrast,ensure_ascii=False,indent=2)+'\n')
  expect('waiting shows all five authoritative equipped attributes',js('document.querySelectorAll("[data-grade]").length===5&&Number(document.querySelector("[data-grade=Run] [data-current]").textContent)===campaignFixture.options.campaign.attributes.current[0].Run'))
  expect('owned equipment uses a source-backed name and correct character slot',js('document.body.innerText.includes("火箭球衣") && document.querySelector(`[data-equipment-character="2"]`).innerText.includes("火箭球衣")'))
  browser('click','[data-campaign-position="0"] [data-character="3"]')
  expect('waiting emits scoped character request',js('JSON.stringify(campaignFixture.actions.at(-1))===JSON.stringify({type:"character",actor:0,character:3})'))
  browser('set','viewport','1100','850');browser('screenshot',str(OUT/'waiting-desktop.png'),'--full')
  js('campaignFixture.setOptions(campaignFixture.source.attributeOptions)')
  wait('document.querySelectorAll("[data-inventory-index]").length===2')
  before=js('campaignFixture.actions.length')
  browser('click','[data-inventory-index="0"]')
  expect('owned item selection previews without submitting a game action',js('campaignFixture.actions.length')==before)
  expect('post-confirmation stats come from original server program',js('Number(document.querySelector("[data-grade=Shot] [data-current]").textContent)===6&&Number(document.querySelector("[data-grade=Shot] [data-after]").dataset.after)===7'))
  browser('screenshot',str(OUT/'attribute-preview-desktop.png'),'--full')
  browser('click','[data-equip-confirm]')
  expect('preview confirmation sends the owned inventory index',js('campaignFixture.actions.at(-1).inventoryIndex===0&&campaignFixture.actions.at(-1).type==="equip"'))
  js('campaignFixture.setOptions(campaignFixture.source.mixedAttributeOptions)')
  wait('document.querySelectorAll("[data-inventory-index]").length===1')
  browser('click','[data-inventory-index="0"]')
  expect('replacement preview includes reductions as well as gains',js('document.querySelector("[data-grade=Recovery] [data-after]").dataset.after==="4"&&document.querySelector("[data-grade=Recovery] .grade-decrease").textContent==="-1"&&document.querySelector("[data-grade=Run] .grade-increase").textContent==="+1"'))
  changes_contrast=js('(()=>{const rgb=s=>s.match(/[\\d.]+/g).slice(0,3).map(Number),lum=c=>c.map(n=>{n/=255;return n<=.04045?n/12.92:((n+.055)/1.055)**2.4}).reduce((s,n,i)=>s+n*[.2126,.7152,.0722][i],0);return [...document.querySelectorAll(".grade-increase,.grade-decrease")].filter(e=>e.textContent.trim()).map(e=>{let b=e;while(b&&getComputedStyle(b).backgroundColor==="rgba(0, 0, 0, 0)")b=b.parentElement;const fg=lum(rgb(getComputedStyle(e).color)),bg=lum(rgb(getComputedStyle(b).backgroundColor));return{text:e.textContent,ratio:(Math.max(fg,bg)+.05)/(Math.min(fg,bg)+.05)}})})()')
  expect('attribute increases and decreases retain readable contrast',all(row['ratio']>=4.5 for row in changes_contrast) and len(changes_contrast)>=2)
  (OUT/'attribute-contrast.json').write_text(json.dumps(changes_contrast,ensure_ascii=False,indent=2)+'\n')
  browser('screenshot',str(OUT/'attribute-preview-mixed.png'),'--full')
  js('campaignFixture.setOptions(campaignFixture.source.largeAttributeOptions)')
  wait('document.querySelectorAll("[data-inventory-index]").length===13')
  browser('set','viewport','320','720')
  browser('click','[data-inventory-index="12"]')
  browser('wait','100')
  expect('last inventory item scrolls comparison and confirmation into the mobile viewport',js('(()=>{const a=document.querySelector(".basketball-campaign-grades").getBoundingClientRect(),b=document.querySelector("[data-equip-confirm]").getBoundingClientRect();return a.top>=0&&b.bottom<=innerHeight})()'))
  browser('screenshot',str(OUT/'attribute-preview-mobile-320.png'))
  js('campaignFixture.setSpectating(true)');wait('[...document.querySelectorAll(".basketball-campaign-editor button")].every(b=>b.disabled)')
  expect('spectator sees preparation without edit permission',True)
  js('campaignFixture.setSpectating(false);campaignFixture.setMode("active")');wait('!!document.querySelector(`[role="dialog"]`)')
  expect('halftime dialog suspends the playing controller',js('campaignFixture.open===true'))
  browser('select','[aria-label="装备给哪位球员"]','2')
  browser('click','[data-inventory-index="0"]')
  browser('click','[data-equip-confirm]')
  expect('halftime request binds visible epoch period revision and inventory index',js('(()=>{const a=campaignFixture.actions.at(-1),d=campaignFixture.data;return a.type==="campaign-preparation"&&a.epoch===d.epoch&&a.preparation.type==="equip"&&a.preparation.character===2&&a.preparation.inventoryIndex===0&&a.preparation.period===d.preparation.period&&a.preparation.revision===d.preparation.revision})()'))
  # Every revision has different DOM identity: pointer-up from an old view must
  # not equip whichever item now happens to occupy that same inventory index.
  browser('scrollintoview','[data-inventory-index="0"]')
  point=js('(()=>{const r=document.querySelector("[data-inventory-index=\\"0\\"]").getBoundingClientRect();return{x:Math.round(r.x+r.width/2),y:Math.round(r.y+r.height/2)}})()')
  before=js('campaignFixture.actions.length')
  browser('mouse','move',str(point['x']),str(point['y']));browser('mouse','down')
  js('campaignFixture.setData({...campaignFixture.data,preparation:{...campaignFixture.data.preparation,revision:campaignFixture.data.preparation.revision+1}})')
  wait('document.querySelector("[data-inventory-index=\\"0\\"]").dataset.revision===String(campaignFixture.data.preparation.revision)')
  browser('mouse','up')
  expect('new authoritative revision invalidates unconfirmed preview',js('!document.querySelector("[data-equip-confirm]")'))
  expect('snapshot revision change cannot retarget an in-progress inventory click',js('campaignFixture.actions.length')==before)
  browser('click','[role="dialog"] [aria-label="关闭"]');wait('!document.querySelector(`[role="dialog"]`)')
  js('campaignFixture.setData({...campaignFixture.data,serverAt:campaignFixture.data.serverAt+100})')
  expect('ordinary snapshot does not reopen a dismissed halftime dialog',js('!document.querySelector(`[role="dialog"]`)'))
  browser('click','[data-campaign-open]');wait('!!document.querySelector(`[role="dialog"]`)')
  expect('reopen retains accepted server inventory',js('document.body.innerText.includes("火箭球衣")'))
  browser('set','viewport','320','720')
  wait('document.querySelector("[role=dialog]").getAnimations().every(a=>a.playState!=="running")')
  expect('mobile modal fits one viewport with inner scrolling',js('(()=>{const e=document.querySelector(`[role="dialog"]`),r=e.getBoundingClientRect();return document.documentElement.scrollWidth<=innerWidth&&r.left>=0&&r.right<=innerWidth&&r.top>=0&&r.bottom<=innerHeight+1&&e.scrollHeight>=e.clientHeight})()'))
  expect('mobile preparation buttons and selects retain 44px targets',js('[...document.querySelectorAll(".basketball-campaign-editor button,.basketball-campaign-editor select")].every(e=>{const r=e.getBoundingClientRect();return r.width>=44&&r.height>=44})'))
  browser('screenshot',str(OUT/'halftime-mobile-320.png'))
  js('campaignFixture.setData(campaignFixture.source.reward)');wait('document.body.innerText.includes("比赛获胜")')
  browser('click','[data-campaign-continue]')
  expect('reward continue submits captured campaign revision',js('(()=>{const a=campaignFixture.actions.at(-1);return a.type==="campaign"&&a.campaign.type==="continue"&&a.campaign.revision===campaignFixture.data.campaign.revision})()'))
  js('campaignFixture.setData(campaignFixture.source.prepared)');wait('!!document.querySelector("[data-campaign-start]")')
  touch('[data-opponent="7"]',page)
  expect('native touch selects a visible opponent at 320px',js('campaignFixture.actions.at(-1).campaign.opponent===7'))
  browser('click','[data-campaign-start]')
  expect('next match requires explicit source preparation confirmation',js('campaignFixture.actions.at(-1).campaign.type==="start"'))
  js('campaignFixture.setData({...campaignFixture.data,campaign:{...campaignFixture.data.campaign,automaticAt:campaignFixture.data.serverAt+3000,canAct:false}})')
  wait('!!document.querySelector("[data-campaign-countdown]")')
  expect('automatic confirmation is visible and human controls disabled',js('document.querySelector("[data-campaign-countdown]").innerText.includes("秒")&&document.querySelector("[data-campaign-start]").disabled'))
  expect('automatic countdown stays in view while editing lower equipment rows',js('(()=>{const d=document.querySelector("[role=dialog]").getBoundingClientRect(),r=document.querySelector("[data-campaign-countdown]").getBoundingClientRect();return r.top>=d.top&&r.bottom<=d.bottom})()'))
  browser('screenshot',str(OUT/'next-round-mobile-320.png'))
  js('campaignFixture.setMode("creation")');wait('!!document.querySelector(`[aria-label="比赛形式"]`)')
  browser('select','[aria-label="比赛形式"]','password')
  browser('fill','[aria-label="原版密码"]','020120')
  expect('creation carries explicit password entry without selecting roster there',js('campaignFixture.options.campaign.password==="020120"&&!document.querySelector("[data-character]")'))
  browser('select','[aria-label="比赛形式"]','new')
  expect('new campaign discards imported preparation and password',js('!campaignFixture.options.campaign.password&&!campaignFixture.options.campaign.preparation'))
  browser('select','[aria-label="比赛形式"]','single')
  expect('single match remains available without campaign state',js('!campaignFixture.options.campaign'))
  js('campaignFixture.setOptions({...campaignFixture.options,campaign:{entry:"password",password:"020010",opponent:1,tactic:4}})')
  browser('wait','100')
  expect('externally restored entry options update the visible form',js('document.querySelector("[aria-label=比赛形式]").value==="password"&&document.querySelector("[aria-label=原版密码]").value==="020010"'))
  js('campaignFixture.setMode("board");campaignFixture.setData(campaignFixture.source.halftime)')
  wait('!!window.resolveCampaignClient && !!document.querySelector(`[role="dialog"]`)')
  js('resolveCampaignClient()');wait('document.querySelector("[data-basketball-ready]").dataset.basketballReady==="true"')
  expect('late controller loading cannot enable input behind an open modal',js('campaignClientBindings.at(-1)===false'))
  browser('click','[role="dialog"] [aria-label="关闭"]')
  wait('campaignClientBindings.at(-1)===true')
  expect('closing preparation restores controller input',True)
  result=dict(status='passed' ,checks=checks,scope='real React UI, native mouse/touch and original PNGs using retained real server views; request recording, not a full live campaign; deferred fake client isolates Board loading permission only')
  (OUT/'result.json').write_text(json.dumps(result,ensure_ascii=False,indent=2)+'\n');print(json.dumps(result,ensure_ascii=False))
 except Exception as error:
  failure=dict(status='failed',checks=checks,error=repr(error))
  if opened:
   try:
    failure['errors']=browser('errors');failure['console']=browser('console');failure['body']=js('document.body.innerText')
    browser('screenshot',str(OUT/'failure.png'),'--full')
   except Exception as inspection:failure['inspectionError']=repr(inspection)
  (OUT/'result.json').write_text(json.dumps(failure,ensure_ascii=False,indent=2)+'\n')
  raise
 finally:
  if opened:browser('close')
  server.shutdown();server.server_close()
if __name__=='__main__':main()
