"""Bounded static tail missing from the prior PCM dispatch listing."""
from pathlib import Path
import argparse,hashlib,json,os,subprocess
ROOT=Path(__file__).resolve().parents[2]
p=argparse.ArgumentParser();p.add_argument('--out',type=Path,required=True);a=p.parse_args();out=a.out.resolve();assert out.is_relative_to(ROOT);out.mkdir(parents=True,exist_ok=True)
ghidra=ROOT.parents[1]/'.local/nes-basket/tools/ghidra_12.1.4_PUBLIC'
# Worktree is repo/worktrees/yuyuhakusho: installed existing tool is in repo.
assert (ghidra/'support/analyzeHeadless').is_file(),ghidra
java=Path('/opt/homebrew/Cellar/openjdk/26.0.1/libexec/openjdk.jdk/Contents/Home');assert(java/'bin/java').is_file()
tmp=out/'tmp';project=ROOT/'docs/research/md-yuyuhakusho/static-work/pcm-command-private';project.mkdir(parents=True,exist_ok=True)
for path in [tmp,tmp/'java-home',tmp/'config',tmp/'cache']:path.mkdir(parents=True,exist_ok=True)
env=dict(os.environ,JAVA_HOME=str(java),TMPDIR=str(tmp));env['PATH']=str(java/'bin')+':'+env['PATH'];env['JAVA_TOOL_OPTIONS']=' '.join('"-D'+k+'='+str(v)+'"'for k,v in {'user.home':tmp/'java-home','java.io.tmpdir':tmp,'application.settingsdir':tmp/'config','application.cachedir':tmp/'cache','application.tempdir':tmp}.items())
command=['nice','-n','15',str(ghidra/'support/analyzeHeadless'),str(project),'pcm-command-tail','-import',str(ROOT/'.local/md-yuyuhakusho/downloads/yuyuhakusho-jp.bin'),'-loader','BinaryLoader','-loader-baseAddr','0','-processor','68000:BE:32:default','-noanalysis','-scriptPath',str(ROOT/'scripts/yuyuhakusho/static'),'-postScript','MdControlProbe.java',str(out/'pcm-command-static.txt'),str(ROOT),'37f58,37fce,3807e:37f58:380a7','-deleteProject','-log',str(out/'ghidra.log'),'-scriptlog',str(out/'ghidra-script.log')]
with(out/'console.log').open('w')as f:r=subprocess.run(command,env=env,stdout=f,stderr=subprocess.STDOUT,cwd=ROOT)
if r.returncode:raise SystemExit(r.returncode)
if not any(project.iterdir()):project.rmdir()
print(json.dumps({'route':'direct-ghidra-headless; REA raw68000 unsupported','command':command,'listing_sha256':hashlib.sha256((out/'pcm-command-static.txt').read_bytes()).hexdigest()}))
