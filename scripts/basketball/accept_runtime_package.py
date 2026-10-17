"""Build the real web/app package, serve embedded files, execute its Wasm in Chrome."""
from pathlib import Path
from urllib.request import urlopen
from urllib.error import HTTPError,URLError
import hashlib,json,os,socket,subprocess,time,uuid
ROOT=Path(__file__).resolve().parents[2];OUT=ROOT/'.local/basketball/runtime-package';OUT.mkdir(parents=True,exist_ok=True)
env=dict(os.environ,AGENT_BROWSER_SESSION='bkp-'+uuid.uuid4().hex[:6],AGENT_BROWSER_SOCKET_DIR=str(ROOT/'.local/bkp-sockets'),TMPDIR=str(ROOT/'.local/tmp'))
Path(env['AGENT_BROWSER_SOCKET_DIR']).mkdir(exist_ok=True)
checks=[];opened=False;app=None
def browser(*args):
 r=subprocess.run(['agent-browser',*args,'--json'],env=env,capture_output=True,text=True,timeout=60)
 if r.returncode:raise RuntimeError(r.stdout+r.stderr)
 v=json.loads(r.stdout)
 if not v['success']:raise RuntimeError(v)
 return v['data']
def check(name,value):
 assert value,name
 checks.append(name)
try:
 (OUT/'result.json').write_text('{"status":"running"}\n')
 with (OUT/'web-build.log').open('w') as log:subprocess.run(['npm','--prefix','web','run','build'],cwd=ROOT,env=env,stdout=log,stderr=subprocess.STDOUT,check=True)
 with (OUT/'verify.log').open('w') as log:subprocess.run(['node','games/basketball/build/verify-runtime.mjs','web/dist'],cwd=ROOT,env=env,stdout=log,stderr=subprocess.STDOUT,check=True)
 subprocess.run(['go','build','-trimpath','-buildvcs=false','-o',str(OUT/'wire-board'),'.'],cwd=ROOT,env=env,check=True)
 with socket.socket() as sock:sock.bind(('127.0.0.1',0));port=sock.getsockname()[1]
 base=f'http://127.0.0.1:{port}'
 data=OUT/('data-'+uuid.uuid4().hex[:6]);data.mkdir()
 app=subprocess.Popen([str(OUT/'wire-board')],cwd=ROOT,env=dict(env,ADDR=f'127.0.0.1:{port}',DATA_DIR=str(data),INVITE_CODE='runtime-package-test',ADMIN_USERNAME='构建验收',ADMIN_PASSWORD='runtime-package-test-only'),stdout=(OUT/'server.stdout').open('w'),stderr=(OUT/'server.stderr').open('w'))
 for _ in range(100):
  if app.poll() is not None:raise RuntimeError('Packaged server stopped: '+(OUT/'server.stderr').read_text())
  try:
   with urlopen(base+'/healthz',timeout=1) as response:
    if response.status==200:break
  except (URLError,TimeoutError):time.sleep(.05)
 else:raise RuntimeError('Packaged server did not become healthy')
 manifest=json.loads((ROOT/'web/dist/basketball/v1/runtime/build.json').read_text())
 for file in ['basketball.wasm','wasm_exec.js','build.json']:
  with urlopen(base+'/basketball/v1/runtime/'+file,timeout=10) as response:
   body=response.read();csp=response.headers.get('Content-Security-Policy','')
   check('embedded '+file+' equals verified frontend bytes',body==(ROOT/'web/dist/basketball/v1/runtime'/file).read_bytes())
   if file=='basketball.wasm':check('Wasm has the correct response MIME type',response.headers.get_content_type()=='application/wasm')
   check('embedded runtime is served with actual production Wasm CSP',"'wasm-unsafe-eval'" in csp and "'unsafe-eval'" not in csp)
 for route in ['/basketball/v1/runtime/rom.nes','/games/basketball/engine/runtime.go','/.local/basketball/rom.nes']:
  try:urlopen(base+route,timeout=3);status=200
  except HTTPError as error:status=error.code
  check('private source/ROM path is absent: '+route,status==404)
 browser('--profile',str(OUT/'profile'),'open',base+'/');opened=True
 result=browser('eval','''(async()=>{
   await import('/basketball/v1/runtime/wasm_exec.js');
   const go=new Go(),response=await fetch('/basketball/v1/runtime/basketball.wasm');
   const binary=await WebAssembly.instantiate(await response.arrayBuffer(),go.importObject);void go.run(binary.instance);
   const call=value=>{const r=JSON.parse(basketballWasm(JSON.stringify(value)));if(!r.ok)throw Error(JSON.stringify(r.error));return r.data;};
   const created=call({op:'create'}),view=call({op:'advance',session:created.session,ticks:210,held:[0,0,0,0]});
   const result={videoFrames:view.state.videoFrames,playing:!!view.state.playing,visible:view.state.display.HasVisible,soundIDs:view.soundIDs};
   call({op:'dispose',session:created.session});return result;
 })()''')['result']
 check('packaged Wasm actually reaches original play in a browser',result['videoFrames']==210 and result['playing'] and result['visible'])
 record=dict(status='passed',checks=checks,browser=result,runtime=manifest,scope='real web build and native app embedding; actual production CSP and Wasm execution, not graphics/OSS release or full game')
 (OUT/'result.json').write_text(json.dumps(record,ensure_ascii=False,indent=2)+'\n');print(json.dumps(dict(status='passed',checks=checks,browser=result),ensure_ascii=False))
except Exception as error:
 (OUT/'result.json').write_text(json.dumps(dict(status='failed',checks=checks,error=repr(error)),ensure_ascii=False,indent=2)+'\n');raise
finally:
 if opened:browser('close')
 if app:
  app.terminate()
  try:app.wait(timeout=10)
  except subprocess.TimeoutExpired:app.kill();app.wait(timeout=5)
