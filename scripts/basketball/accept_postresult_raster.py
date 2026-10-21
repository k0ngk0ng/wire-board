"""Test-first browser raster acceptance against original whole-console pixels.

Failures: wrong CHR page/palette/nametable/mirroring; OAM order/eight-sprite
limit/priority; alpha or color conversion; accepting missing indexed art.
Initial scope: original Scene7 final picture, not other scene bands or timing.
"""
import functools,http.server,json,os,subprocess,threading,uuid,csv,argparse,gzip
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
    parser=argparse.ArgumentParser();parser.add_argument('--rows',action='store_true');parser.add_argument('--wasm',action='store_true');parser.add_argument('--renderer',action='store_true');args=parser.parse_args()
    if args.renderer:args.wasm=True
    if args.wasm:args.rows=True
    source=ROOT/'.local/nes-basket/runs/postresult-ending-observed'
    frame=13500
    video=next(int(r['emulator_frame']) for r in csv.DictReader((source/'frames.csv').open()) if int(r['frame'])==frame)
    ppu=(source/f'ppu-{video:05d}-ppu.bin').read_bytes();oam=(source/f'ppu-{video:05d}-oam.bin').read_bytes()
    fixture=dict(expected=f'../../.local/nes-basket/runs/postresult-ending-observed/frame-{frame:05d}.png',frame=dict(nametables=list(ppu[0x2000:0x3000]),palette=list(ppu[0x3f00:0x3f20]),oam=list(oam),bands=[dict(y0=0,y1=240,nametable=0,scrollX=0,scrollY=0,backgroundCHR=[20,21,22,23],spriteCHR=[56,57,11,40],mask=30)]))
    if args.rows:
        from postresult_fetch import derive_frame
        samples=json.loads((ROOT/'.local/postresult-fetch/result.json').read_text())+json.loads((ROOT/'.local/postresult-fetch/holdout-result.json').read_text())
        fixture=dict(rows=True,cases=[dict(name=f'{r["kind"]}-{r["frame"]}',expected=f'../../.local/nes-basket/runs/postresult-{r["kind"]}-observed/frame-{r["frame"]:05d}.png',frame=derive_frame(r['kind'],r['frame']))for r in samples])
        assert len(fixture['cases'])==16
    if args.wasm:
        fixture['wasm']=True
        cases=json.loads(gzip.decompress((ROOT/'games/basketball/engine/testdata/postresult-raster-rows.json.gz').read_bytes()))
        source={(r['kind'],r['frame']):r for r in cases}
        for item,sample in zip(fixture['cases'],samples):
            original=source[sample['kind'],sample['frame']]
            item['projection']=dict(surface=original['surface'],plan=original['plan'])
            item['frame']=dict(oam=item['frame']['oam'])
        runtime=OUT/'runtime';runtime.mkdir(exist_ok=True)
        subprocess.run(['go','build','-o',str(runtime/'basketball.wasm'),'./games/basketball/cmd/browser-wasm'],cwd=ROOT,env=dict(env,GOOS='js',GOARCH='wasm'),check=True)
        goroot=Path(subprocess.check_output(['go','env','GOROOT'],cwd=ROOT,env=env,text=True).strip())
        (runtime/'wasm_exec.js').write_bytes((goroot/'lib/wasm/wasm_exec.js').read_bytes())
    if args.renderer:
        pin=json.loads((ROOT/'games/basketball/assets/release-manifest.json').read_text())
        fixture['renderer']=True
        fixture['assetsBase']=f'../../.local/basketball-asset-build/{pin["contentSHA256"]}/public/basketball/v1/graphics'
    (OUT/'fixture.json').write_text(json.dumps(fixture,separators=(',',':')))
    (OUT/'index.html').write_text('<!doctype html><meta charset="utf-8"><style>body{margin:0;background:#111;color:white}main{display:grid;grid-template-columns:repeat(4,256px)}figure{margin:0}canvas{image-rendering:pixelated}</style><main></main><script type="module" src="raster.js"></script>')
    subprocess.run([str(ROOT/'web/node_modules/.bin/esbuild'),str(ROOT/'scripts/basketball/postresult_raster_harness.ts'),'--bundle','--format=esm','--target=es2022','--outfile='+str(OUT/'raster.js')],cwd=ROOT,env=env,check=True)
    class Quiet(http.server.SimpleHTTPRequestHandler):
        def log_message(self,*args):pass
    server=http.server.ThreadingHTTPServer(('127.0.0.1',0),functools.partial(Quiet,directory=str(ROOT)))
    thread=threading.Thread(target=server.serve_forever,daemon=True);thread.start();opened=False
    try:
        browser('--profile',str(OUT/'profile'),'open',f'http://127.0.0.1:{server.server_port}/.local/postresult-raster/index.html');opened=True
        browser('wait','--fn','window.result !== undefined')
        result=browser('eval','window.result')['result']
        prefix='renderer-' if args.renderer else 'wasm-' if args.wasm else 'rows-' if args.rows else ''
        browser('screenshot',str(OUT/(prefix+'browser.png')),'--full')
        (OUT/(prefix+'result.json')).write_text(json.dumps(result,indent=2)+'\n');print(json.dumps(result))
        assert result.get('different')==0 and result.get('rejectsMissingPage'),result
    finally:
        if opened:browser('close')
        server.shutdown();server.server_close();thread.join()
if __name__=='__main__':main()
