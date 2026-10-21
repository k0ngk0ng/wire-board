"""Test-first real-browser bundle fallback: valid CDN, unavailable CDN,
corrupted PNG hash, and both sources unavailable. No mixed-version bundle.
"""
import functools,http.server,json,os,subprocess,threading,uuid,argparse
from pathlib import Path
ROOT=Path(__file__).resolve().parents[2];OUT=ROOT/'.local/basketball-asset-fallback';OUT.mkdir(exist_ok=True)
env=dict(os.environ,AGENT_BROWSER_SESSION='baf-'+uuid.uuid4().hex[:6],AGENT_BROWSER_SOCKET_DIR=str((ROOT/'.local/basketball').resolve()/'bkn-sockets'),TMPDIR=str(ROOT/'.local/tmp'))
requests=[]
assetRoot=ROOT/".local/basketball"
class Handler(http.server.SimpleHTTPRequestHandler):
    def log_message(self,*args):pass
    def do_GET(self):
        requests.append(self.path)
        if self.path.startswith('/missing'):
            self.send_error(503);return
        if self.path.startswith('/corrupt/raster/') and self.path.endswith('.png'):
            self.send_response(200);self.send_header('Content-Type','image/png');self.end_headers();self.wfile.write(b'corrupt source bytes');return
        super().do_GET()
    def translate_path(self,path):
        parts=path.split('/')
        if len(parts)>2 and parts[1] in ('valid','local','corrupt'):
            relative=Path('/'.join(parts[2:]));assert '..' not in relative.parts
            return str(assetRoot/relative)
        return super().translate_path(path)
def browser(*args):
    r=subprocess.run(['agent-browser',*args,'--json'],env=env,capture_output=True,text=True,timeout=60)
    if r.returncode:raise RuntimeError(r.stderr or r.stdout)
    value=json.loads(r.stdout)
    if not value.get('success'):raise RuntimeError(value)
    return value['data']
def main():
    global assetRoot
    parser=argparse.ArgumentParser();parser.add_argument("--dist",action="store_true");args=parser.parse_args()
    if args.dist:assetRoot=ROOT/"web/dist/basketball/v1/graphics"
    subprocess.run([str(ROOT/'web/node_modules/.bin/esbuild'),str(ROOT/'scripts/basketball/asset_fallback_harness.ts'),'--bundle','--format=esm','--target=es2022','--outfile='+str(OUT/'check.js')],env=env,cwd=ROOT,check=True)
    (OUT/'index.html').write_text('<!doctype html><meta charset="utf-8"><style>body{white-space:pre-wrap}</style><script type="module" src="check.js"></script>')
    server=http.server.ThreadingHTTPServer(('127.0.0.1',0),functools.partial(Handler,directory=str(ROOT)));t=threading.Thread(target=server.serve_forever,daemon=True);t.start();opened=False
    try:
        browser('--profile',str(OUT/'profile'),'open',f'http://127.0.0.1:{server.server_port}/.local/basketball-asset-fallback/index.html');opened=True
        browser('wait','--fn','window.result !== undefined')
        result=browser('eval','window.result')['result'];result['requests']=requests
        (OUT/('dist-result.json' if args.dist else 'result.json')).write_text(json.dumps(result,indent=2)+'\n')
        assert result.get('rejectsUnavailable') and len(result.get('cases',[]))==3,result
        assert all(c['courts']==8 and c['resultCourts']==8 and c['poses']==120 for c in result['cases'])
        assert any(p.startswith('/corrupt/raster/') and p.endswith('.png') for p in requests)
        assert any(p.startswith('/local/courts/') and p.endswith('.png') for p in requests)
        browser('screenshot',str(OUT/('dist-browser.png' if args.dist else 'browser.png')),'--full')
        print(json.dumps(dict(passed=True,cases=result['cases'],requests=len(requests),rejectsUnavailable=True)))
    finally:
        if opened:browser('close')
        server.shutdown();server.server_close();t.join()
if __name__=='__main__':main()
