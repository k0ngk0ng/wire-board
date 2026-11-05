"""Actual browser Board with native Go module/Wasm; not authenticated host QA."""
from pathlib import Path
import argparse,hashlib,json,os,runpy,subprocess,time
ROOT=Path(__file__).resolve().parents[2]
parser=argparse.ArgumentParser(description=__doc__);parser.add_argument('--label',default='final');parser.add_argument('--red',action='store_true');parser.add_argument('--controls',action='store_true');parser.add_argument('--pause-review',action='store_true');parser.add_argument('--loading-review',action='store_true');parser.add_argument('--performance-review',action='store_true');parser.add_argument('--performance-only',action='store_true');args=parser.parse_args()
OUT=ROOT/'games/yuyuhakusho/verification/platform-web'/args.label;TMP=ROOT/'.local/md-yuyuhakusho/platform-web'
for p in [OUT,TMP,TMP/'tmp',ROOT/'.local/yw']:p.mkdir(parents=True,exist_ok=True)
env=dict(os.environ,TMPDIR=str(TMP/'tmp'),GOTMPDIR=str(TMP/'tmp'),GOCACHE=str(ROOT/'.local/go-cache'),GOMODCACHE=str(ROOT/'.local/go-mod'),TEST_TELEMETRY_DIR=str(ROOT/'.local/go-telemetry'),AGENT_BROWSER_SOCKET_DIR=str(ROOT/'.local/yw'))
build=[str(ROOT/'web/node_modules/.bin/esbuild'),'scripts/yuyuhakusho/platform_board_harness.tsx','--bundle','--format=esm','--jsx=automatic','--alias:react=./web/node_modules/react','--alias:react-dom=./web/node_modules/react-dom','--loader:.svg=file','--outfile='+str(TMP/'harness.js')]
r=subprocess.run(build,cwd=ROOT,env=env,text=True,capture_output=True)
if args.red:
 report={'test_first':True,'scope':'Missing real Board/module UI compile contract before implementation; browser/Go execution remains required','command':build,'exit_code':r.returncode,'stdout':r.stdout,'stderr':r.stderr,'missing_board':r.returncode!=0 and 'Could not resolve' in r.stderr,'harness_sha256':hashlib.sha256((ROOT/'scripts/yuyuhakusho/platform_board_harness.tsx').read_bytes()).hexdigest()};(OUT/'missing-board-red.json').write_text(json.dumps(report,indent=2)+'\n');assert report['missing_board'];print('actual missing Board compile red retained');raise SystemExit(0)
assert r.returncode==0,r.stderr
subprocess.run(['python3',str(ROOT/'scripts/yuyuhakusho/build_platform_web.py')],cwd=ROOT,env=env,check=True)
process=subprocess.Popen([str(TMP/'host'),'-build',str(TMP),'-root',str(ROOT)],cwd=ROOT,env=env,stdout=subprocess.PIPE,stderr=subprocess.PIPE,text=True)
url=process.stdout.readline().strip();assert url.startswith('http://127.0.0.1:'),url
seed=subprocess.check_output(['agent-browser','session','id','--scope','worktree','--prefix','yuyu-platform-web'],cwd=ROOT,env=env,text=True).strip();env['AGENT_BROWSER_SESSION']='pw'+seed.rsplit('-',1)[-1][:6]
def browser(*items):
 result=subprocess.run(['agent-browser',*items,'--json'],cwd=ROOT,env=env,text=True,capture_output=True,timeout=55)
 if result.returncode:raise RuntimeError(result.stdout+result.stderr)
 value=json.loads(result.stdout);assert value['success'],value;return value['data']
def js(code):return browser('eval',code)['result']
opened=False;report={}
try:
 browser('--profile',str(TMP/'profile'),'open',url);opened=True;browser('set','viewport','1100','1000');browser('snapshot','-i')
 assert js('document.querySelectorAll("[aria-label=比赛准备] [aria-label=场地] option").length')==8
 assert js('document.querySelectorAll("[data-character-seat] option").length')>=22
 browser('select','[aria-label=比赛准备] [aria-label=场地]','10');assert js('window.yuyuBoardProbe.prepare.at(-1)?.arena')==10
 browser('select','[data-character-seat="1"]','8');assert js('window.yuyuBoardProbe.prepare.at(-1)?.character')==8
 if args.loading_review:js('yuyuBoardProbe.testGate=true')
 if args.performance_review:js('yuyuBoardProbe.pollMS=50')
 browser('click','#start-board');browser('wait','--fn','yuyuBoardProbe.ready || !!yuyuBoardProbe.error');assert not js('yuyuBoardProbe.error'),js('yuyuBoardProbe.error')
 if args.loading_review:
  browser('wait','2200')
  held=js('({actions:yuyuBoardProbe.actions,syncCount:yuyuBoardProbe.syncCount,ready:yuyuBoardProbe.ready,error:yuyuBoardProbe.error,phase:yuyuBoardProbe.authority.phase})');report['public_loading_hold']=held
  assert held['ready'] and held['phase']=='loading' and held['actions']==[] and held['syncCount']<=2 and not held['error'],held
  browser('click','#release-public-gate');browser('wait','--fn','yuyuBoardProbe.actions.some(action=>action.type==="loaded")')
  assert js('yuyuBoardProbe.actions.filter(action=>action.type==="loaded").length')==1
 browser('wait','--fn','yuyuBoardProbe.authority?.phase !== "loading" && BigInt(yuyuBoardProbe.authority?.frame??"0") > 250n')
 if args.performance_review:
  js('yuyuBoardProbe.rpc=[];yuyuBoardProbe.paints=[];yuyuBoardProbe.fetches=[];performance.clearResourceTimings()')
  endpoint=browser('get','cdp-url')['cdpUrl']
  subprocess.run(['node',str(ROOT/'scripts/yuyuhakusho/profile_platform_runtime.mjs'),endpoint,url,str(OUT/'cpu.cpuprofile'),'5'],cwd=ROOT,env=env,check=True)
  measured=js('({rpc:yuyuBoardProbe.rpc,paints:yuyuBoardProbe.paints,fetches:yuyuBoardProbe.fetches,resources:performance.getEntriesByType("resource").map(e=>({url:e.name,start:e.startTime,requestStart:e.requestStart,responseStart:e.responseStart,responseEnd:e.responseEnd,duration:e.duration,transferSize:e.transferSize}))})')
  intervals=[b['at']-a['at'] for a,b in zip(measured['paints'],measured['paints'][1:]) if b['at']>a['at']]
  import statistics
  metric={'fps':1000/ statistics.mean(intervals),'median_ms':statistics.median(intervals),'p95_ms':sorted(intervals)[int(.95*(len(intervals)-1))],'max_ms':max(intervals),'paint_count':len(measured['paints'])}
  by_op={}
  for op in {item['op']for item in measured['rpc']}:
   values=[item['duration']for item in measured['rpc']if item['op']==op];by_op[op]={'count':len(values),'mean_ms':statistics.mean(values),'total_ms':sum(values),'max_ms':max(values)}
  report['performance']={'paint':metric,'rpc':by_op,'cpu_profile':'cpu.cpuprofile','authority_poll_ms':50}
  (OUT/'performance-raw.json').write_text(json.dumps(measured,indent=2)+'\n');(OUT/'performance-result.json').write_text(json.dumps(report['performance'],indent=2)+'\n')
  assert metric['fps']>=50 and metric['median_ms']<=25 and metric['p95_ms']<=40,report['performance']
  if args.performance_only:
   report.update(pass_=True,scope='Steady-state actual Module/Go-Wasm Board at20Hz authority; independent fresh-match functional acceptance is separate.',source_sha256={str(p.relative_to(ROOT)):hashlib.sha256(p.read_bytes()).hexdigest()for p in [*(ROOT/'games/yuyuhakusho/web').glob('*'),ROOT/'games/yuyuhakusho/cmd/platform-client-wasm/main_js.go',ROOT/'games/yuyuhakusho/runtimebridge/performance.go']if p.is_file()})
   report['pass']=report.pop('pass_');(OUT/'result.json').write_text(json.dumps(report,ensure_ascii=False,indent=2)+'\n');print('Actual Board steady-state performance pass');raise SystemExit(0)
 assert js('document.querySelector("[data-controller-layout=md]") !== null')
 before=js('window.yuyuBoardProbe.authority.players[0].x')
 js('window.yuyuBoardProbe.delay=500')
 browser('focus','[data-yuyu-board]')
 endpoint=browser('get','cdp-url')['cdpUrl']
 def native(actions):return json.loads(subprocess.check_output(['node',str(ROOT/'scripts/yuyuhakusho/native_platform_input.mjs'),endpoint,url,json.dumps(actions)],cwd=ROOT,env=env,text=True))
 observed=native([{'type':'keyDown','key':'d','code':'KeyD','windowsVirtualKeyCode':68,'observeMove':before}])[0]
 assert observed['local']>before+5 and observed['authority']<=before+3,observed
 assert observed['pixelHash']!=observed['before']['pixelHash'],observed
 native([{'type':'keyUp','key':'d','code':'KeyD','windowsVirtualKeyCode':68}]);js('window.yuyuBoardProbe.delay=0')
 report['prediction_before_delayed_authority']=observed
 if args.pause_review:runpy.run_path(str(ROOT/'scripts/yuyuhakusho/platform_pause_acceptance.py'))['check'](browser,js,native,report)
 if args.controls:runpy.run_path(str(ROOT/'scripts/yuyuhakusho/platform_controls_acceptance.py'))['check'](browser,js,native,report)
 browser('screenshot',str(OUT/'desktop.png'))
 browser('click','#fullscreen');browser('wait','--fn','document.querySelector("[data-room-fullscreen]") !== null')
 assert js('!document.querySelector("[data-game-fullscreen-surface]").contains(document.querySelector("#platform-header"))')
 browser('press','Escape');browser('wait','--fn','document.querySelector("[data-room-fullscreen]") === null')
 browser('set','viewport','320','740');assert not js('document.documentElement.scrollWidth > innerWidth')
 pad=js('(()=>{const a=document.querySelector("canvas").getBoundingClientRect(),b=document.querySelector("[data-controller-layout=md]").getBoundingClientRect();return {canvas:a.bottom,pad:b.top}})()');assert pad['pad']>=pad['canvas']
 if args.controls:runpy.run_path(str(ROOT/'scripts/yuyuhakusho/platform_touch_acceptance.py'))['check'](browser,js,native,report)
 browser('screenshot',str(OUT/'mobile-320.png'))
 assert js('yuyuBoardProbe.actions.some(action=>action.type==="loaded")')
 report['timings']=js('yuyuBoardProbe.timings')
 report['paint_intervals_ms']=js('yuyuBoardProbe.paints.slice(1).map((paint,index)=>paint.at-yuyuBoardProbe.paints[index].at)')
 report.update(pass_=True,scope='Actual Go module and Go/Wasm Board integration with a local test host; not platform authentication/socket/slow-peer acceptance',native_keyboard=True,source_sha256={str(p.relative_to(ROOT)):hashlib.sha256(p.read_bytes()).hexdigest()for p in (ROOT/'games/yuyuhakusho/web').glob('*')if p.is_file()},pad_geometry=pad)
 report['pass']=report.pop('pass_');(OUT/'result.json').write_text(json.dumps(report,ensure_ascii=False,indent=2)+'\n');print('Board actual browser E2E pass')
except Exception as error:
 diagnostic={}
 if opened:
  try:diagnostic=js('({authority:yuyuBoardProbe.authority,actions:yuyuBoardProbe.actions.slice(-20),timings:yuyuBoardProbe.timings.slice(-20),lastHeld:yuyuBoardProbe.lastHeld,lastStep:yuyuBoardProbe.lastStep,active:document.activeElement?.outerHTML.slice(0,250),hidden:document.hidden})')
  except Exception:pass
 (OUT/'failure.json').write_text(json.dumps({'error':repr(error),'completed':report,'diagnostic':diagnostic},ensure_ascii=False,indent=2)+'\n')
 if opened:
  try:browser('screenshot',str(OUT/'failure.png'))
  except Exception:pass
 raise
finally:
 if opened:browser('close')
 process.terminate();process.wait(timeout=10)
