"""Actual public fullscreen DOM/default/optional-action contract, no game rules."""
from pathlib import Path
import argparse,http.server,json,os,subprocess,threading
ROOT=Path(__file__).resolve().parents[2];OUT=ROOT/'games/yuyuhakusho/verification/platform-web';TMP=ROOT/'.local/md-yuyuhakusho/fullscreen-contract'
parser=argparse.ArgumentParser();parser.add_argument('--before',action='store_true');args=parser.parse_args()
subprocess.run(['node',str(ROOT/'scripts/yuyuhakusho/build_fullscreen_contract.mjs')],cwd=ROOT,check=True)
css=(ROOT/'web/src/style.css').read_text()+'\n'+(ROOT/'web/src/ui/room-fullscreen.css').read_text()
page=('<!doctype html><meta charset=utf-8><style>'+css+'body{margin:0}#root{width:100%;padding:8px}</style><div id="root"></div><script src="/fixture.js"></script>').encode()
class Handler(http.server.BaseHTTPRequestHandler):
 def do_GET(self):
  data=(TMP/'fixture.js').read_bytes()if self.path=='/fixture.js'else page;self.send_response(200);self.send_header('Content-Type','text/javascript'if self.path=='/fixture.js'else'text/html; charset=utf-8');self.end_headers();self.wfile.write(data)
 def log_message(self,*args):pass
server=http.server.ThreadingHTTPServer(('127.0.0.1',0),Handler);threading.Thread(target=server.serve_forever,daemon=True).start();(TMP/'tmp').mkdir(exist_ok=True)
env=dict(os.environ,AGENT_BROWSER_SOCKET_DIR=str(ROOT/'.local/yu'),AGENT_BROWSER_SESSION='fscntr',TMPDIR=str(TMP/'tmp'))
def b(*args):
 p=subprocess.run(['agent-browser',*args,'--json'],cwd=ROOT,env=env,text=True,capture_output=True,timeout=55);value=json.loads(p.stdout);assert not p.returncode and value['success'],p.stdout+p.stderr;return value['data']
def js(code):return b('eval',code)['result']
def mount(before,actions=False,disabled=False):
 js('fullscreenContract.mount('+','.join(str(x).lower()for x in[before,actions,disabled])+');true');normal=js('document.querySelector("[data-game-fullscreen-surface]").outerHTML');js('fullscreenContract.enter();true');b('wait','--fn','!!document.querySelector("[data-room-fullscreen]")');return normal,js('document.querySelector("[data-game-fullscreen-surface]").outerHTML')
report={'pass':False,'scope':'Public fullscreen actual React fallback DOM/slot contract; no game rules','checks':[]}
try:
 b('--profile',str(TMP/'profile'),'open','http://127.0.0.1:'+str(server.server_port)+'/');b('wait','--fn','!!window.fullscreenContract');b('set','viewport','320','740')
 before=mount(True);after=mount(False);assert before==after;report['checks'].append('no_slot_default_normal_and_fullscreen_dom_exact_baseline');report['defaultMarkup']={'normal':before[0],'fullscreen':before[1]}
 if not args.before:
  mount(False,True);slot=js('!!document.querySelector("[data-room-fullscreen] [data-viewport-resume]")');assert slot,'Optional fullscreen toolbar actions missing'
  values=[]
  for width,height in[(1100,1000),(320,740),(740,360)]:
   b('set','viewport',str(width),str(height));js('(async()=>{await new Promise(requestAnimationFrame);await new Promise(requestAnimationFrame);return true})()');row=js('(()=>{const r=e=>{const b=e.getBoundingClientRect();return{x:b.x,y:b.y,right:b.right,bottom:b.bottom,width:b.width,height:b.height}},t=document.querySelector(".game-fullscreen-toolbar"),a=document.querySelector("[data-viewport-resume]"),e=document.querySelector(".room-fullscreen-toggle");return{toolbar:r(t),action:r(a),exit:r(e),hit:document.elementFromPoint(a.getBoundingClientRect().x+2,a.getBoundingClientRect().y+2)===a}})()');assert row['toolbar']['height']==36 and row['action']['right']<=row['exit']['x'] and row['action']['height']<=36 and row['hit'],row;values.append({'width':width,'height':height,**row})
  report['geometry']=values;report['checks'].append('optional_actions_share_original36px_toolbar_and_remain_hittable_at_three_viewports');mount(False,True,True);assert js('document.querySelector("[data-viewport-resume]").disabled');report['checks'].append('caller_disabled_action_retained');b('screenshot',str(OUT/'fullscreen-slot-contract.png'))
 report['pass']=True
except Exception as error:report['error']=repr(error);raise
finally:
 (OUT/('fullscreen-default-before.json'if args.before else'fullscreen-slot-contract-'+('red'if not report['pass']else'final')+'.json')).write_text(json.dumps(report,ensure_ascii=False,indent=2)+'\n');b('close');server.shutdown();server.server_close();print(json.dumps(report,ensure_ascii=False))
