"""Own loopback observer reload setup contract, without application/game data."""
from pathlib import Path
import http.server,json,os,subprocess,threading
ROOT=Path(__file__).resolve().parents[2];TMP=ROOT/'.local/md-yuyuhakusho/reload-observer';TMP.mkdir(exist_ok=True);(TMP/'tmp').mkdir(exist_ok=True);OUT=ROOT/'games/yuyuhakusho/verification/platform-web'
class Handler(http.server.BaseHTTPRequestHandler):
 def do_GET(self):self.send_response(200);self.send_header('Content-Type','text/html');self.end_headers();self.wfile.write(b'<!doctype html><title>Observer setup contract</title>Own loopback setup only')
 def log_message(self,*args):pass
server=http.server.ThreadingHTTPServer(('127.0.0.1',0),Handler);threading.Thread(target=server.serve_forever,daemon=True).start();env=dict(os.environ,AGENT_BROWSER_SESSION='obsreload',AGENT_BROWSER_SOCKET_DIR=str(ROOT/'.local/yu'),TMPDIR=str(TMP/'tmp'))
def b(*args):
 p=subprocess.run(['agent-browser',*args,'--json'],cwd=ROOT,env=env,text=True,capture_output=True,timeout=55);v=json.loads(p.stdout);assert not p.returncode and v['success'],p.stdout+p.stderr;return v['data']
report={'pass':False,'scope':'Observer setup only; blank owned page, no game or authority'}
try:
 url='http://127.0.0.1:'+str(server.server_port)+'/';b('--profile',str(TMP/'profile'),'open',url);before=b('eval','performance.timeOrigin')['result'];endpoint=b('get','cdp-url')['cdpUrl'];subprocess.run(['node',str(ROOT/'scripts/yuyuhakusho/install_app_observer.mjs'),endpoint,url],cwd=ROOT,env=env,check=True);after=b('eval','({timeOrigin:performance.timeOrigin,observer:!!window.yuyuAppObservation})')['result'];assert after['observer']and after['timeOrigin']!=before,after;report.update({'pass':True,'realReload':True,'passiveObserverPresentAfterHelperDetached':True})
except Exception as error:report['error']=repr(error);raise
finally:(OUT/('reload-observer-contract-'+('final'if report['pass']else'red')+'.json')).write_text(json.dumps(report,indent=2)+'\n');b('close');server.shutdown();server.server_close();print(json.dumps(report))
