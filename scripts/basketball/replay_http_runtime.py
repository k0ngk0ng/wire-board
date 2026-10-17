"""Reproduce a real HTTP runtime before its next platform control deadline.

Uses a retained public runtime snapshot and zero subsequent controller input.
This is a diagnostic for an idle HTTP test, not a substitute original-ROM oracle.
"""
import argparse,hashlib,json,re,subprocess
from pathlib import Path

ROOT=Path(__file__).resolve().parents[2]
def main():
    p=argparse.ArgumentParser(description=__doc__)
    p.add_argument('evidence');p.add_argument('--output',default='.local/basketball/http-runtime-replay')
    args=p.parse_args();source=(ROOT/args.evidence).resolve();out=(ROOT/args.output).resolve()
    if not source.is_relative_to(ROOT) or not out.is_relative_to(ROOT):p.error('paths must stay inside workspace')
    out.mkdir(parents=True,exist_ok=True)
    source_bytes=source.read_bytes();source_hash=hashlib.sha256(source_bytes).hexdigest()
    archive=out/('evidence-'+source_hash+'.json');archive.write_bytes(source_bytes)
    data=json.loads(source_bytes);view=data['trace'][-1]['view']
    if any(i['buttons'] or i['pending'] or i.get('appliedButtons') or i.get('changes') for i in view['inputs']):p.error('only an idle source snapshot can use zero-input replay')
    deadline=min(i['idleAt'] for i in view['inputs'] if not i['automated'])
    limit=max(0,(deadline-view['serverAt'])*60//1000)
    binary=out/'runtime-stdio'
    subprocess.run(['go','build','-o',str(binary),'./games/basketball/cmd/runtime-stdio'],cwd=ROOT,check=True)
    process=subprocess.Popen([str(binary)],cwd=ROOT,stdin=subprocess.PIPE,stdout=subprocess.PIPE,text=True)
    def call(request):
        process.stdin.write(json.dumps(request)+'\n');process.stdin.flush()
        return json.loads(process.stdout.readline())
    report=dict(source=str(source.relative_to(ROOT)),sourceSHA256=source_hash,sourceArchive=str(archive.relative_to(ROOT)),scope='idle runtime replay before first platform deadline; no original-ROM claim',maximumAdditionalVideoTicks=limit)
    try:
        restored=call(dict(op='restore',checkpoint=dict(version=1,state=view['runtime'])));assert restored['ok'],restored
        sid=restored['data']['session'];state=restored['data']['state'];completed=0
        while completed<limit:
            ticks=min(600,limit-completed)
            result=call(dict(op='advance',session=sid,ticks=ticks,held=[0,0,0,0]))
            if not result['ok']:
                report['failure']=result['error'];match=re.match(r'video tick (\d+):',result['error']['detail'])
                if match and int(match[1])>0:
                    advance=int(match[1]);prefix=call(dict(op='advance',session=sid,ticks=advance,held=[0,0,0,0]));assert prefix['ok'],prefix
                    state=prefix['data']['state'];completed+=advance
                again=call(dict(op='advance',session=sid,ticks=1,held=[0,0,0,0]));assert not again['ok'],again
                report.update(reproduced=True,completedAdditionalVideoTicks=completed,before=state)
                break
            state=result['data']['state'];completed+=ticks
            if state.get('outcome',{}).get('confirmed'):break
        else:report.update(reproduced=False,completedAdditionalVideoTicks=completed)
        report.setdefault('reproduced',False)
        (out/'result.json').write_text(json.dumps(report,indent=2)+'\n')
        print(json.dumps({k:v for k,v in report.items() if k!='before'},indent=2))
    finally:
        process.stdin.close();process.wait(timeout=10)

if __name__=='__main__':main()
