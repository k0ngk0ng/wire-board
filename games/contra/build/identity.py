"""Local-only canonical build binding shared by native and Wasm preparation."""
import hashlib,json,os,re,subprocess
from pathlib import Path
PKG='github.com/k0ngk0ng/wire-board/games/contra/runtime.'
def sha(data):return hashlib.sha256(data).hexdigest()
def encode_manifest(manifest):
 raw=(json.dumps(manifest,sort_keys=True,ensure_ascii=False,separators=(',',':'))+'\n').encode()
 return sha(raw),raw
def native_flags(rules_id,runtime_id):
 if not all(re.fullmatch('[a-f0-9]{64}',value or '')for value in [rules_id,runtime_id]):raise ValueError('missing canonical native build identities')
 return '-s -w -X '+PKG+'CompiledRulesBuildID='+rules_id+' -X '+PKG+'CompiledRuntimeBuildID='+runtime_id
def go_env(root,target=None):
 root=Path(root).resolve();env=dict(os.environ,GOENV='off',GOWORK='off',GOFLAGS='',GOEXPERIMENT='',CGO_ENABLED='0')
 env.pop('GOOS',None);env.pop('GOARCH',None)
 version=re.search(r'^go (\S+)',(root/'go.mod').read_text(),re.M)[1];env['GOTOOLCHAIN']='go'+version
 for key,name in [('GOCACHE','go-build'),('GOMODCACHE','go-mod'),('GOTMPDIR','tmp'),('TMPDIR','tmp')]:
  env[key]=str(root/'.local'/name);Path(env[key]).mkdir(parents=True,exist_ok=True)
 if target:env.update(GOOS=target[0],GOARCH=target[1])
 return env
def go(root,args,env=None):return subprocess.check_output(['go',*args],cwd=root,env=env or go_env(root),text=True,stderr=subprocess.PIPE)
def runtime_inputs(root,env=None):
 root=Path(root).resolve();env=env or go_env(root,('js','wasm'))
 template='{{if not .Standard}}{{.Dir}}{{range .GoFiles}}|{{.}}{{end}}{{range .SFiles}}|{{.}}{{end}}{{range .EmbedFiles}}|{{.}}{{end}}{{end}}'
 names={'go.mod','go.sum','games/contra/assets/runtime-stages.json','games/contra/assets/audio-source.json','games/contra/assets/audio-loops.json','games/contra/server/data/visual-catalog-v1.json.gz'}
 names.update({'games/contra/build/identity.py','games/contra/build/media.py','scripts/contra/browser_build.py'})
 names.update({'games/contra/build/media_archive.py','games/contra/assets/audio-pack.json'})
 for row in go(root,['list','-deps','-f',template,'./games/contra/runtime/cmd/wasm'],env).splitlines():
  if not row:continue
  folder,*files=row.split('|')
  for file in files:
   path=(Path(folder)/file).resolve()
   if not path.is_relative_to(root):raise ValueError('Wasm dependency escaped checkout')
   name=path.relative_to(root).as_posix()
   if name.startswith('games/contra/server/'):raise ValueError('native visual assets must not enter Wasm dependencies')
   names.add(name)
 return [{'path':name,'sha256':sha((root/name).read_bytes())}for name in sorted(names)]
def native_inputs(root,package,env):
 root=Path(root).resolve();names={'go.mod','go.sum'}
 template='{{if not .Standard}}{{.Dir}}{{range .GoFiles}}|{{.}}{{end}}{{range .SFiles}}|{{.}}{{end}}{{range .EmbedFiles}}|{{.}}{{end}}{{end}}'
 for row in go(root,['list','-deps','-f',template,package],env).splitlines():
  if not row:continue
  folder,*files=row.split('|')
  for name in files:
   path=(Path(folder)/name).resolve()
   if not path.is_relative_to(root):raise ValueError('native dependency escaped checkout/cache scope')
   names.add(path.relative_to(root).as_posix())
 return [{'path':name,'sha256':sha((root/name).read_bytes())}for name in sorted(names)]
def inspect_build(root):
 root=Path(root).resolve();env=go_env(root)
 rules=json.loads(go(root,['run','./games/contra/build/cmd/identity','--root',str(root)],env))
 inputs=runtime_inputs(root);_,encoded=encode_manifest(inputs)
 loader=Path(go(root,['env','GOROOT'],env).strip())/'lib/wasm/wasm_exec.js'
 return {**rules,'runtime':{'goVersion':go(root,['env','GOVERSION'],env).strip(),'loaderSHA256':sha(loader.read_bytes()),'sourceSHA256':sha(encoded),'builderSHA256':sha(Path(__file__).read_bytes()),'inputs':inputs}}
def verify_runtime_file(root,manifest_file):
 root=Path(root).resolve();path=Path(manifest_file).resolve()
 if not path.is_relative_to(root):raise ValueError('manifest must be inside checkout')
 raw=path.read_bytes();manifest=json.loads(raw);runtime_id=sha(raw);current=inspect_build(root)
 if manifest.get('rulesBuildID')!=current['rules']['id'] or manifest.get('visualsID')!=current['visuals']['id'] or manifest.get('runtime')!=current['runtime']:raise ValueError('runtime artifact source/toolchain/visual identity differs from current build')
 catalog={'rgb':{key:manifest['rgb'][key]for key in ['bytes','sha256']},'pages':{key:{field:value[field]for field in ['bytes','sha256']}for key,value in manifest['pages'].items()}}
 if catalog!=current['visuals']['manifest']or manifest.get('bundle')!=current['rules']['bundle']:raise ValueError('browser resources differ from native embedded resources')
 if manifest['loader']['sha256']!=current['runtime']['loaderSHA256']:raise ValueError('loader differs from selected Go toolchain')
 if set(manifest.get('stages',{}))!=set(map(str,range(8))):raise ValueError('runtime must describe each individually loadable stage')
 for stage in manifest['bundle']['stages']:
  if manifest['stages'][str(stage['level'])]['sha256']!=stage['sha256']:raise ValueError('stage descriptor differs from verified bundle')
 verify_asset_files(path.parent,manifest)
 return manifest,runtime_id,current
def verify_asset_files(directory,manifest):
 # Bind nested audio payloads as well as the catalog which names them.
 directory=Path(directory).resolve()
 assets=[manifest[name]for name in ['wasm','loader','audioCatalog','rgb','resources']]+list(manifest['pages'].values())+list(manifest['stages'].values())
 def verify(asset):
  rel=asset['url'].lstrip('/');source=(directory/rel).resolve()
  if not source.is_relative_to(directory):raise ValueError('runtime asset escaped manifest directory')
  body=source.read_bytes()
  if len(body)!=asset['bytes']or sha(body)!=asset['sha256']:raise ValueError('runtime artifact checksum differs: '+rel)
  return body
 for asset in assets:verify(asset)
 audio=json.loads(verify(manifest['audioCatalog']))
 for asset in audio['paths'].values():verify(asset)
