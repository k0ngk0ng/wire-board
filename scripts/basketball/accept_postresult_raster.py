"""Test-first browser raster acceptance against original whole-console pixels.

Failures: wrong CHR page/palette/nametable/mirroring; OAM order/eight-sprite
limit/priority; alpha or color conversion; accepting missing indexed art.
Initial scope: original Scene7 final picture, not other scene bands or timing.
"""
import functools,http.server,json,os,subprocess,threading,uuid,csv
from pathlib import Path
ROOT=Path(__file__).resolve().parents[2]
OUT=ROOT/'.local/postresult-raster';OUT.mkdir(exist_ok=True)
env=dict(os.environ,AGENT_BROWSER_SESSION='bpr-'+uuid.uuid4().hex[:6],AGENT_BROWSER_SOCKET_DIR=str((ROOT/'.local/basketball').resolve()/'bkn-sockets'),TMPDIR=str(ROOT/'.local/tmp'))
def browser(*args):
    r=subprocess.run(['agent-browser',*args,'--json'],env=env,capture_output=True,text=True,timeout=60)
    if r.returncode:raise RuntimeError(r.stderr or r.stdout)
    result=json.loads(r.stdout)
    if not result.get('success'):raise RuntimeError(result)
    return result['data']

def main():
    source=ROOT/'.local/nes-basket/runs/postresult-ending-observed'
    frame=13500
    video=next(int(r['emulator_frame']) for r in csv.DictReader((source/'frames.csv').open()) if int(r['frame'])==frame)
    ppu=(source/f'ppu-{video:05d}-ppu.bin').read_bytes();oam=(source/f'ppu-{video:05d}-oam.bin').read_bytes()
    fixture=dict(expected=f'../../.local/nes-basket/runs/postresult-ending-observed/frame-{frame:05d}.png',frame=dict(nametables=list(ppu[0x2000:0x3000]),palette=list(ppu[0x3f00:0x3f20]),oam=list(oam),bands=[dict(y0=0,y1=240,nametable=0,scrollX=0,scrollY=0,backgroundCHR=[20,21,22,23],spriteCHR=[56,57,11,40],mask=30)]))
    (OUT/'fixture.json').write_text(json.dumps(fixture,separators=(',',':')))
    (OUT/'index.html').write_text('<!doctype html><meta charset="utf-8"><style>body{margin:0;background:#111}canvas{image-rendering:pixelated;width:768px;height:720px}</style><canvas width="256" height="240"></canvas><script type="module" src="raster.js"></script>')
    subprocess.run([str(ROOT/'web/node_modules/.bin/esbuild'),str(ROOT/'scripts/basketball/postresult_raster_harness.ts'),'--bundle','--format=esm','--target=es2022','--outfile='+str(OUT/'raster.js')],cwd=ROOT,env=env,check=True)
    class Quiet(http.server.SimpleHTTPRequestHandler):
        def log_message(self,*args):pass
    server=http.server.ThreadingHTTPServer(('127.0.0.1',0),functools.partial(Quiet,directory=str(ROOT)))
    thread=threading.Thread(target=server.serve_forever,daemon=True);thread.start();opened=False
    try:
        browser('--profile',str(OUT/'profile'),'open',f'http://127.0.0.1:{server.server_port}/.local/postresult-raster/index.html');opened=True
        browser('wait','--fn','window.result !== undefined')
        result=browser('eval','window.result')['result']
        browser('screenshot',str(OUT/'browser.png'),'--full')
        (OUT/'result.json').write_text(json.dumps(result,indent=2)+'\n');print(json.dumps(result))
        assert result.get('different')==0 and result.get('rejectsMissingPage'),result
    finally:
        if opened:browser('close')
        server.shutdown();server.server_close();thread.join()
if __name__=='__main__':main()
