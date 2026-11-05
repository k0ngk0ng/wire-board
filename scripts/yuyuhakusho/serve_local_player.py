"""Build and serve the independent local research player on loopback only."""
from pathlib import Path
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import argparse, os, shutil, subprocess
from urllib.parse import urlsplit

ROOT=Path(__file__).resolve().parents[2]
BUILD=ROOT/'.local/md-yuyuhakusho/local-player'
def build():
    BUILD.mkdir(parents=True,exist_ok=True)
    temp=ROOT/'.local/md-yuyuhakusho/tmp';temp.mkdir(parents=True,exist_ok=True)
    env=dict(os.environ,GOCACHE=str(ROOT/'.local/go-cache'),GOMODCACHE=str(ROOT/'.local/go-mod'),GOTMPDIR=str(temp),TMPDIR=str(temp),TEST_TELEMETRY_DIR=str(ROOT/'.local/go-telemetry'))
    goroot=subprocess.run(['go','env','GOROOT'],cwd=ROOT,env=env,text=True,capture_output=True,check=True).stdout.strip()
    subprocess.run(['go','build','-o',str(BUILD/'player.wasm'),'./games/yuyuhakusho/cmd/local-player-wasm'],cwd=ROOT,env=dict(env,GOOS='js',GOARCH='wasm'),check=True)
    shutil.copyfile(Path(goroot)/'lib/wasm/wasm_exec.js',BUILD/'wasm_exec.js')
    subprocess.run([str(ROOT/'web/node_modules/.bin/esbuild'),'scripts/yuyuhakusho/local_player.tsx','--bundle','--format=esm','--jsx=automatic','--alias:react=./web/node_modules/react','--alias:react-dom=./web/node_modules/react-dom','--loader:.svg=file','--outfile='+str(BUILD/'player.js')],cwd=ROOT,env=env,check=True)

class PlayerHandler(BaseHTTPRequestHandler):
    requested_paths=[]
    def log_message(self,*args): pass
    def do_GET(self):
        path=urlsplit(self.path).path
        self.requested_paths.append(path)
        if path=='/':
            data=b'<!doctype html><html lang="zh-CN"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>\xe9\xad\x94\xe5\xbc\xba\xe7\xbb\x9f\xe4\xb8\x80\xe6\x88\x98</title><link rel="stylesheet" href="/player.css"></head><body><div id="root"></div><script src="/wasm_exec.js"></script><script type="module" src="/player.js"></script></body></html>'
            mime='text/html; charset=utf-8'
        else:
            assets={'/rules.json.gz':'chu-world-rules.json.gz','/sprites.json.gz':'world-sprites.json.gz','/arena.json.gz':'arena-assets-default.json.gz','/hud.json.gz':'hud-assets.json.gz'}
            assets.update({f'/arena-{arena:x}.json.gz':f'arena-assets-{arena:x}.json.gz' for arena in (2,4,6,8,10,12,14)})
            assets['/arena-a-aux.json.gz']='arena-a-aux-assets.json.gz'
            if path in assets:
                source=ROOT/'games/yuyuhakusho/data'/assets[path]
            else:
                relative=Path(path.lstrip('/'))
                if '..' in relative.parts or len(relative.parts)!=1:
                    self.send_error(404);return
                source=BUILD/relative
            if not source.is_file():self.send_error(404);return
            data=source.read_bytes()
            mime={'.wasm':'application/wasm','.js':'text/javascript','.css':'text/css','.svg':'image/svg+xml','.gz':'application/octet-stream'}.get(source.suffix,'application/octet-stream')
        self.send_response(200)
        self.send_header('Content-Type',mime)
        self.send_header('Content-Length',str(len(data)))
        self.send_header('Cache-Control','no-store')
        self.end_headers();self.wfile.write(data)

if __name__=='__main__':
    parser=argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--port',type=int,default=8799)
    parser.add_argument('--build-only',action='store_true')
    args=parser.parse_args()
    build()
    if not args.build_only:
        print(f'Independent research player: http://127.0.0.1:{args.port}/',flush=True)
        server=ThreadingHTTPServer(('127.0.0.1',args.port),PlayerHandler)
        try:server.serve_forever()
        finally:server.server_close()
