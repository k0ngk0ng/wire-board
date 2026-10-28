"""Repeat the base-only original execution oracle with Go's race detector.

Select all production engine files and base tests, so unrelated in-progress
test-first enemy groups do not block this scoped evidence. This does not replace
the parent world's integration, complete campaign or browser acceptance.
"""
from pathlib import Path
import subprocess,json,hashlib,sys
ROOT=Path(__file__).resolve().parents[2]

def main():
 directory=ROOT/'.local/contra-enemies-base';directory.mkdir(parents=True,exist_ok=True)
 engine=ROOT/'games/contra/engine'
 files=sorted(p for p in engine.glob('*.go')if not p.name.endswith('_test.go')or p.name.startswith('enemies_base_')and p.name.endswith('_test.go')or p.name in['enemies_spawn_test.go','enemies_drawing_test.go','enemies_step_test.go'])
 def digest(p):return hashlib.sha256(p.read_bytes()).hexdigest()
 input_hashes={p:digest(p)for p in files}
 command=['go','test','-race',*[str(p.relative_to(ROOT))for p in files],'-run','^TestOriginal(Base|WeaponPickups$)','-count=1','-json']
 result=subprocess.run(command,cwd=ROOT,capture_output=True,text=True)
 rows=[json.loads(line)for line in result.stdout.splitlines()if line.startswith('{')]
 output=directory/'scoped-race-results.jsonl';output.write_text(result.stdout)
 tests=[dict(name=r['Test'],result=r['Action'],seconds=r.get('Elapsed'))for r in rows if r.get('Test')and r.get('Action')in['pass','fail','skip']]
 fixtures=sorted((ROOT/'docs/research/nes-contra/evidence').glob('enemies-base-*.json.gz'))+[ROOT/'docs/research/nes-contra/evidence/enemies-base-compatibility.json']
 report=dict(success=result.returncode==0,scope='base rules only; all production engine files; unrelated in-progress tests excluded',command=command,tests=tests,
  output=dict(path=str(output.relative_to(ROOT)),sha256=digest(output)),fixtures=[dict(path=str(p.relative_to(ROOT)),sha256=digest(p))for p in fixtures],
  source=[dict(path=str(p.relative_to(ROOT)),sha256=input_hashes[p])for p in sorted(engine.glob('enemies_base*.go'))+[engine/'enemies_pickup.go',engine/'enemies_bullets.go']],
  compilation_inputs=[dict(path=str(p.relative_to(ROOT)),sha256=value)for p,value in input_hashes.items()],
  source_changed_during_test=[str(p.relative_to(ROOT))for p,value in input_hashes.items()if digest(p)!=value],
  limitations=['Not an integrated complete campaign or browser/network acceptance.','Controlled common-wall capture retains the source regular-room attribute page while SceneType selects Boss update tiles.','Roller stale-attribute exception is bounded by a pinned source/id/slot/value manifest and separate complete original paired-runtime proof.'])
 (ROOT/'docs/research/nes-contra/evidence/enemies-base-verification.json').write_text(json.dumps(report,indent=2)+'\n')
 print(json.dumps(dict(success=report['success'],tests=tests)))
 if result.stderr:print(result.stderr,file=sys.stderr)
 raise SystemExit(result.returncode)
if __name__=='__main__':main()
