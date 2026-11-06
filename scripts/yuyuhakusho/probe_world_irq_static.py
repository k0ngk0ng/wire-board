"""Bounded direct-Ghidra inspection of the game's VDP/IRQ setup, no execution."""
from pathlib import Path
import argparse, hashlib, json, os, re, subprocess

ROOT=Path(__file__).resolve().parents[2]
parser=argparse.ArgumentParser(description=__doc__)
parser.add_argument('--suffix',default='static')
parser.add_argument('--regions',nargs='+',default=['2ff4:2ff4:302f','3030,3054,307c:3030:30af','2514:2514:258f','219ac,219ea:219ac:21a07'])
args=parser.parse_args()
assert re.fullmatch(r'[A-Za-z0-9_-]+',args.suffix)
OUT=ROOT/'games/yuyuhakusho/verification/world-audio-barriers'/args.suffix
assert not (OUT/'validation.json').exists(),'Choose a new suffix; completed evidence is immutable'
OUT.mkdir(parents=True,exist_ok=True)
ROM=ROOT/'.local/md-yuyuhakusho/downloads/yuyuhakusho-jp.bin'
assert hashlib.sha256(ROM.read_bytes()).hexdigest()=='c39d52b2e2298b3784ada3de70d64b26b67e172114c7d18494faad70fc4d7c06'
TOOL=Path('/Users/jason/Sources/github.com/k0ngk0ng/wire-board/.local/nes-basket/tools/ghidra_12.1.4_PUBLIC/support/analyzeHeadless')
JAVA=Path('/opt/homebrew/Cellar/openjdk/26.0.1/libexec/openjdk.jdk/Contents/Home')
assert TOOL.is_file() and (JAVA/'bin/java').is_file()
project=ROOT/'docs/research/md-yuyuhakusho/static-work'/('world-audio-irq-'+args.suffix)
scratch=ROOT/'.local/md-yuyuhakusho'/('world-audio-irq-'+args.suffix)
for p in [project,scratch/'tmp',scratch/'home',scratch/'config',scratch/'cache']:p.mkdir(parents=True,exist_ok=True)
env=dict(os.environ,JAVA_HOME=str(JAVA),PATH=str(JAVA/'bin')+os.pathsep+os.environ.get('PATH',''),TMPDIR=str(scratch/'tmp'))
options={'user.home':scratch/'home','java.io.tmpdir':scratch/'tmp','application.settingsdir':scratch/'config','application.cachedir':scratch/'cache','application.tempdir':scratch/'tmp'}
env['JAVA_TOOL_OPTIONS']=' '.join('"-D'+key+'='+str(value)+'"'for key,value in options.items())
output=OUT/'vdp-and-irq-setup.txt'
command=[str(TOOL),str(project),'world_audio_irq_setup','-import',str(ROM),'-loader','BinaryLoader','-loader-baseAddr','0','-processor','68000:BE:32:default','-noanalysis','-scriptPath',str(ROOT/'scripts/yuyuhakusho/static'),'-postScript','MdControlProbe.java',str(output),str(ROOT),*args.regions,'-deleteProject','-log',str(OUT/'ghidra.log'),'-scriptlog',str(OUT/'ghidra-script.log')]
with (OUT/'console.log').open('w')as log:result=subprocess.run(command,cwd=ROOT,env=env,stdout=log,stderr=subprocess.STDOUT)
assert result.returncode==0 and output.is_file(),(result.returncode,OUT/'console.log')
raw=output.read_bytes();assert b'68000:BE:32:default'in raw and raw.count(b'printed_instructions=')==len(args.regions)
sources=[ROOT/'scripts/yuyuhakusho/static/MdControlProbe.java',Path(__file__),ROM]
report={'pass':True,'scope':'Static instructions only; direct Ghidra raw-68000 route, not REA-native decompilation or observed execution','command':command,'output_sha256':hashlib.sha256(raw).hexdigest(),'source_sha256':{str(p.relative_to(ROOT)):hashlib.sha256(p.read_bytes()).hexdigest()for p in sources},'settings':{key:str(value.relative_to(ROOT))for key,value in options.items()},'output':str(output.relative_to(ROOT))}
(OUT/'validation.json').write_text(json.dumps(report,indent=2)+'\n')
if not any(project.iterdir()):project.rmdir()
print(json.dumps({'pass':True,'output':report['output'],'sha256':report['output_sha256']}))
