"""Collect actual final-frame versus restored-preview evidence for frozen49."""
from pathlib import Path
import concurrent.futures,hashlib,json,os,subprocess,shutil
from datetime import datetime,timezone
ROOT=Path(__file__).resolve().parents[2]
DATA=ROOT/'games/yuyuhakusho/runtimebridge/testdata'
OUT=ROOT/'games/yuyuhakusho/verification/client-terminal-preview'
TEMP=ROOT/'.local/md-yuyuhakusho/client-terminal-probe'
for p in(OUT,TEMP):p.mkdir(parents=True,exist_ok=True)
def sha(p):return hashlib.sha256(p.read_bytes()).hexdigest()
source=DATA/'terminal-probe.go.txt';program=TEMP/'main.go';program.write_bytes(source.read_bytes());binary=TEMP/'probe'
env=dict(os.environ,TEST_TELEMETRY_DIR=str(ROOT/'.local/go-telemetry'),GOCACHE=str(ROOT/'.local/go-cache'),GOMODCACHE=str(ROOT/'.local/go-mod'),GOTMPDIR=str(ROOT/'.local/md-yuyuhakusho/tmp'),TMPDIR=str(ROOT/'.local/md-yuyuhakusho/tmp'))
binding=json.loads((DATA/'schema2-verification.json').read_text())['source_sha256']
for name,digest in binding.items():
 if sha(ROOT/name)!=digest:raise RuntimeError('source binding changed '+name)
meta={'started_at_utc':datetime.now(timezone.utc).isoformat(),'source_sha256':binding,'probe_sha256':sha(source),'failure_list_sha256':sha(DATA/'terminal-probe-failures.md'),'commands':[],'complete':False}
(OUT/'command.json').write_text(json.dumps(meta,indent=2)+'\n')
command=['go','build','-o',str(binary),str(program)]
with(OUT/'build.txt').open('w')as f:p=subprocess.run(command,cwd=ROOT,env=env,stdout=f,stderr=subprocess.STDOUT)
meta['build_command']=command;meta['build_exit_code']=p.returncode
if p.returncode:raise RuntimeError('terminal probe build failed')
def run(i):
 command=[str(binary),str(OUT),str(i),'4']
 with(OUT/f'shard-{i}.txt').open('w')as f:p=subprocess.run(command,cwd=ROOT,env=env,stdout=f,stderr=subprocess.STDOUT)
 result={'command':command,'exit_code':p.returncode};print(json.dumps(result),flush=True);return result
with concurrent.futures.ThreadPoolExecutor(max_workers=4)as pool:meta['commands']=list(pool.map(run,range(4)))
if any(r['exit_code']for r in meta['commands']):raise RuntimeError('terminal probe failed')
rows=[]
for i in range(4):rows+=json.loads((OUT/f'rows-{i}.json').read_text())
rows.sort(key=lambda r:r['name'])
terminal=[r for r in rows if r['complete']];different=[r for r in terminal if not r['pixels_equal']]
meta.update(complete=True,ended_at_utc=datetime.now(timezone.utc).isoformat(),source_unchanged=all(sha(ROOT/name)==digest for name,digest in binding.items()))
report={'schema':1,'pass':len(rows)==49 and meta['source_unchanged'],'trajectories':len(rows),'completed_trajectories':len(terminal),'nonterminal_pan_endpoints':len(rows)-len(terminal),'completed_pixel_differences':len(different),'completed_empty_draw_frames':sum(r['last_frame_draw_requests']==0 for r in terminal),'completed_clear_world_events':sum(r['event_kinds'].get('clear-world',0)for r in terminal),'completed_same_authority_and_preview_frame':all(r['authority_frame']==r['preview_frame']for r in terminal),'rows':rows}
(OUT/'result.json').write_text(json.dumps(report,indent=2)+'\n');(OUT/'command.json').write_text(json.dumps(meta,indent=2)+'\n')
print(json.dumps({k:v for k,v in report.items()if k!='rows'},indent=2))
if not report['pass']:raise RuntimeError('terminal probe incomplete')
# Only this verifier's compiled program and temporary source are removed.
shutil.rmtree(TEMP)
