"""Build the local-only actual-module browser fixture, keeping all caches in repo."""
from pathlib import Path
import hashlib,json,os,shutil,subprocess
ROOT=Path(__file__).resolve().parents[2]
OUT=ROOT/'.local/md-yuyuhakusho/platform-web'
for path in [OUT,OUT/'tmp',ROOT/'.local/go-cache',ROOT/'.local/go-mod',ROOT/'.local/go-telemetry']:path.mkdir(parents=True,exist_ok=True)
env=dict(os.environ,TMPDIR=str(OUT/'tmp'),GOTMPDIR=str(OUT/'tmp'),GOCACHE=str(ROOT/'.local/go-cache'),GOMODCACHE=str(ROOT/'.local/go-mod'),TEST_TELEMETRY_DIR=str(ROOT/'.local/go-telemetry'))
subprocess.run(['go','build','-o',str(OUT/'client.wasm'),'./games/yuyuhakusho/cmd/platform-client-wasm'],cwd=ROOT,env=dict(env,GOOS='js',GOARCH='wasm'),check=True)
subprocess.run(['go','build','-o',str(OUT/'host'),'./games/yuyuhakusho/cmd/platform-web-harness'],cwd=ROOT,env=env,check=True)
go_root=Path(subprocess.check_output(['go','env','GOROOT'],cwd=ROOT,env=env,text=True).strip())
support=go_root/'lib/wasm/wasm_exec.js'
if not support.exists():support=go_root/'misc/wasm/wasm_exec.js'
shutil.copyfile(support,OUT/'wasm_exec.js')
wasm=(OUT/'client.wasm').read_bytes()
(OUT/'build.json').write_text(json.dumps({'wasm':{'url':'/yuyuhakusho/v1/runtime/client.wasm','sha256':hashlib.sha256(wasm).hexdigest(),'bytes':len(wasm)}},indent=2)+'\n')
print(json.dumps({'local_only':True,'wasm_bytes':len(wasm),'wasm_sha256':hashlib.sha256(wasm).hexdigest()}))
