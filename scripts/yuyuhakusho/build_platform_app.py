"""Build the normal platform UI with local Yuyu routes for authenticated E2E.

This is an isolated local acceptance build, not the release or CDN workflow.
Use its dist as server.New's os.DirFS argument to exercise the actual host.
"""
from pathlib import Path
import hashlib,json,os,shutil,subprocess
ROOT=Path(__file__).resolve().parents[2]
OUT=ROOT/'.local/md-yuyuhakusho/platform-app';PUBLIC=OUT/'public';DIST=OUT/'dist'
for path in [OUT,PUBLIC,OUT/'tmp']:path.mkdir(parents=True,exist_ok=True)
# Keep the shared platform brand route in this isolated local public tree.
shutil.copyfile(ROOT/'web/public/favicon.svg',PUBLIC/'favicon.svg')
env=dict(os.environ,TMPDIR=str(OUT/'tmp'),GOTMPDIR=str(OUT/'tmp'),GOCACHE=str(ROOT/'.local/go-cache'),GOMODCACHE=str(ROOT/'.local/go-mod'),TEST_TELEMETRY_DIR=str(ROOT/'.local/go-telemetry'))
subprocess.run(['python3',str(ROOT/'scripts/yuyuhakusho/build_platform_web.py')],cwd=ROOT,env=env,check=True)
descriptor=ROOT/'games/yuyuhakusho/data/runtime-resource-descriptor.json'
metadata=json.loads(descriptor.read_bytes())
for item in metadata['resources']:
 source=ROOT/'games/yuyuhakusho/data'/Path(item['logical_path']).name;raw=source.read_bytes()
 assert len(raw)==item['gzip_bytes'] and hashlib.sha256(raw).hexdigest()==item['gzip_sha256']
 target=PUBLIC/item['logical_path'].lstrip('/');target.parent.mkdir(parents=True,exist_ok=True);shutil.copyfile(source,target)
shutil.copyfile(descriptor,PUBLIC/'yuyuhakusho/data/runtime-resource-descriptor.json')
runtime=PUBLIC/'yuyuhakusho/v1/runtime';runtime.mkdir(parents=True,exist_ok=True)
for name in ['build.json','wasm_exec.js','client.wasm']:shutil.copyfile(ROOT/'.local/md-yuyuhakusho/platform-web'/name,runtime/name)
# A dedicated config keeps the production Vite/OSS pipeline and other game's
# generated asset directories untouched. Relative assets are local to this run.
config=OUT/'vite.config.mjs'
config.write_text('''import {defineConfig} from %s;
import react from %s;
import {gameLoadManifest} from %s;
export default defineConfig({root:%s,base:"/",publicDir:%s,plugins:[react(),gameLoadManifest()],resolve:{dedupe:["react","react-dom","lucide-react","three"]},build:{target:"es2022",outDir:%s,emptyOutDir:true}});
'''%tuple(json.dumps(str(path))for path in [ROOT/'web/node_modules/vite/dist/node/index.js',ROOT/'web/node_modules/@vitejs/plugin-react/dist/index.js',ROOT/'web/scripts/game-load-manifest.mjs',ROOT/'web',PUBLIC,DIST]))
subprocess.run([str(ROOT/'web/node_modules/.bin/vite'),'build','--config',str(config)],cwd=ROOT/'web',env=env,check=True)
subprocess.run(['node',str(ROOT/'scripts/yuyuhakusho/check_platform_app_build.mjs')],cwd=ROOT,env=env,check=True)
print(json.dumps({'local_only':True,'dist':str(DIST),'server_handler':'server.New(real config including Modules: games.Modules(), ClientLoading: true, os.DirFS(dist))','routes':{'resources':'/yuyuhakusho/data/{13 descriptor entries + runtime-resource-descriptor.json}','wasm':'/yuyuhakusho/v1/runtime/{build.json,wasm_exec.js,client.wasm}','loading':'/game-load-manifest.json'}}))
