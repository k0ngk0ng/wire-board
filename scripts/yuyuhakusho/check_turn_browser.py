"""Computed public compact/default timer contract, no game rules or fake match."""
from pathlib import Path
import json,os,subprocess,http.server,threading
ROOT=Path(__file__).resolve().parents[2];TMP=ROOT/'.local/md-yuyuhakusho/host-ui-contract';OUT=ROOT/'games/yuyuhakusho/verification/platform-web'
subprocess.run(['node',str(ROOT/'scripts/yuyuhakusho/check_host_turn_contract.mjs')],cwd=ROOT,check=True)
markup=json.loads((TMP/'turn-markup.json').read_text());css=(ROOT/'web/src/style.css').read_text()+'\n'+(ROOT/'web/src/game-api/host.css').read_text();body=''.join('<div class="game-sidebar" id="'+key+'">'+value+'</div>'for key,value in markup['after'].items());page=('<!doctype html><meta charset=utf-8><style>'+css+'body{margin:0}.game-sidebar{width:296px;position:static;margin:6px}</style><main class="room-page playing">'+body+'</main>').encode()
class Handler(http.server.BaseHTTPRequestHandler):
 def do_GET(self):self.send_response(200);self.send_header('Content-Type','text/html; charset=utf-8');self.end_headers();self.wfile.write(page)
 def log_message(self,*args):pass
server=http.server.ThreadingHTTPServer(('127.0.0.1',0),Handler);threading.Thread(target=server.serve_forever,daemon=True).start();(TMP/'tmp').mkdir(exist_ok=True);env=dict(os.environ,AGENT_BROWSER_SOCKET_DIR=str(ROOT/'.local/yu'),AGENT_BROWSER_SESSION='turnc',TMPDIR=str(TMP/'tmp'))
def b(*args):
 p=subprocess.run(['agent-browser',*args,'--json'],cwd=ROOT,env=env,text=True,capture_output=True,timeout=55);v=json.loads(p.stdout);assert not p.returncode and v['success'],p.stdout+p.stderr;return v['data']
try:
 b('--profile',str(TMP/'turn-profile'),'open','http://127.0.0.1:'+str(server.server_port)+'/');b('set','viewport','320','740')
 values=b('eval','[...document.querySelectorAll(".game-sidebar")].map(e=>{const c=e.querySelector(".turn-card"),t=c.querySelector("time"),r=c.getBoundingClientRect(),q=t.getBoundingClientRect();return {id:e.id,height:r.height,time:{text:t.textContent,display:getComputedStyle(t).display,visibility:getComputedStyle(t.parentElement.parentElement).visibility,width:q.width,height:q.height},placeholder:!!c.querySelector(".turn-clock-placeholder")}})')['result']
 by={v['id']:v for v in values};assert abs(by['contract-opted-clock']['height']-by['contract-opted-none']['height'])<=.5,values;assert by['contract-opted-clock']['time']['visibility']=='visible'and by['contract-opted-clock']['time']['display']!='none'and by['contract-opted-clock']['time']['width']>0,values;assert by['contract-opted-none']['placeholder'],values
 b('screenshot',str(OUT/'host-turn-browser.png'));(OUT/'host-turn-browser.json').write_text(json.dumps({'pass':True,'scope':'Public timer CSS/DOM contract only','computed':values},indent=2)+'\n');print('Public timer computed visibility/stable compact height pass')
finally:b('close');server.shutdown();server.server_close()
