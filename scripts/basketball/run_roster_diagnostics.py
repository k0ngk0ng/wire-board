"""Explicit fixed-seed roster gates; retain every failure and its exact input.

Run after sourcing scripts/env.sh. This does not alter production selections or
replace original AI. The existing cold gate runs two full matches with normal
clocks, periodic restoration and a 48,000-video-frame limit.
"""
import argparse, concurrent.futures, hashlib, json, os, re, subprocess
from pathlib import Path

ROOT=Path(__file__).resolve().parents[2]

def workspace_path(value):
    p=Path(value).resolve()
    if not p.is_relative_to(ROOT):raise ValueError('output/input must remain inside the workspace')
    return p

def source_files():
    paths=list((ROOT/'games/basketball/engine').rglob('*.go'))
    paths += list((ROOT/'games/basketball/engine/testdata').rglob('*'))
    paths += [ROOT/'go.mod',ROOT/'go.sum']
    return {str(p.relative_to(ROOT)):hashlib.sha256(p.read_bytes()).hexdigest() for p in sorted(paths) if p.is_file()}

def main():
    ap=argparse.ArgumentParser();ap.add_argument('--cases',required=True);ap.add_argument('--output',required=True);ap.add_argument('--jobs',type=int,default=2)
    args=ap.parse_args();out=workspace_path(args.output);cases=json.loads(workspace_path(args.cases).read_text())
    if not cases or args.jobs<1 or args.jobs>4:raise ValueError('nonempty explicit cases and 1..4 workers required')
    out.mkdir(parents=True,exist_ok=False)
    (out/'cases.json').write_text(json.dumps(cases,indent=2)+'\n')
    sources=source_files();(out/'sources.json').write_text(json.dumps(sources,indent=2)+'\n')
    binary=out/'engine.test'
    with (out/'build.log').open('w') as log:subprocess.run(['go','test','-c','-o',str(binary),'./games/basketball/engine'],cwd=ROOT,stdout=log,stderr=subprocess.STDOUT,check=True)
    def run(item):
        index,selection=item;directory=out/f'case-{index:02d}';directory.mkdir()
        env={**os.environ,'BASKETBALL_RUNTIME_DIAGNOSTIC':'1','BASKETBALL_RUNTIME_SELECTION':json.dumps(selection,separators=(',',':'))}
        with (directory/'native.log').open('w') as log:
            process=subprocess.run([str(binary),'-test.run=^TestRuntimeTeamRosterDiagnostic$','-test.count=1','-test.v','-test.timeout=10m'],cwd=ROOT/'games/basketball/engine',env=env,stdout=log,stderr=subprocess.STDOUT)
        log=(directory/'native.log').read_text();failures=[line.strip() for line in log.splitlines() if 'runtime cold seed' in line or 'diagnostic requires' in line]
        if 'before=' in log:
            before=log.split('before=',1)[1].splitlines()[0];state=json.loads(before);(directory/'before.json').write_text(json.dumps(state,indent=2)+'\n')
        repeats=[line.strip() for line in log.splitlines() if 'runtime cold repeat=' in line]
        result=dict(index=index,selection=selection,exitCode=process.returncode,passed=process.returncode==0 and len(repeats)==2,failures=failures,repeats=repeats,log=str((directory/'native.log').relative_to(ROOT)))
        (directory/'result.json').write_text(json.dumps(result,indent=2)+'\n');print(json.dumps(result),flush=True);return result
    with concurrent.futures.ThreadPoolExecutor(max_workers=args.jobs) as pool:results=list(pool.map(run,enumerate(cases)))
    summary=dict(scope='explicit cold Go runtime configurations, unchanged seed and normal clocks; not an original-ROM or menu oracle',binarySHA256=hashlib.sha256(binary.read_bytes()).hexdigest(),sourceStable=sources==source_files(),passed=sum(r['passed'] for r in results),failed=sum(not r['passed'] for r in results),results=results)
    (out/'summary.json').write_text(json.dumps(summary,indent=2)+'\n')
    if summary['failed'] or not summary['sourceStable']:raise SystemExit(1)
if __name__=='__main__':main()
