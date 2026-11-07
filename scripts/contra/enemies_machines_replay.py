"""Replay the exact retained machine oracles; never regenerate expected data.

Uses source-bound saved entries and scenario JSON embedded in each fixture.
The native outputs must match all recorded fields, including the full pool,
graphics bytes, globals, audio and palettes where captured. Controlled setup
and controller-only continuation retain their distinct fixture descriptions.
"""
import argparse,gzip,hashlib,json,os,shutil
from pathlib import Path
ROOT=Path(__file__).resolve().parents[2]
BASE=ROOT/'.local/contra-enemies-machines/replay-oracle'
os.environ['CONTRA_REFERENCE_ROOT']=str(BASE)
from capture_reference import run

def main():
    parser=argparse.ArgumentParser();parser.add_argument('--label',default='v1');parser.add_argument('--fixture');args=parser.parse_args()
    report=[]
    fixtures=sorted((ROOT/'games/contra/engine/testdata').glob('original-enemy-machines*.json.gz'))
    if args.fixture:
        fixtures=[p for p in fixtures if p.name==args.fixture];assert len(fixtures)==1,'unknown machine fixture'
    for fixture in fixtures:
        d=json.loads(gzip.decompress(fixture.read_bytes()));assert d['source_is_original_execution']
        entries=d['sources']if'sources'in d else[{'source':d['source'],'scenario':d['scenario']}]
        actual=[]
        for entry in entries:
            spec=dict(entry['scenario']);originalName=spec['name'];spec['name']='machines-replay-'+args.label+'-'+str(len(report))+'-'+str(len(actual))
            source=entry['source'];stateName=source['initial_state']['name'];state=BASE/'states'/stateName
            stage=int(stateName.split('-')[1].split('.')[0])
            origin=ROOT/f'.local/contra-campaign/states/stage-{stage}-v2.fcs'
            assert hashlib.sha256(origin.read_bytes()).hexdigest()==source['initial_state']['sha256']
            state.parent.mkdir(parents=True,exist_ok=True)
            if state.exists():assert state.read_bytes()==origin.read_bytes()
            else:shutil.copyfile(origin,state)
            # Natural stage 6 has zero actor records. A unique source suffix
            # avoids confusing its completed zero-count run with stage 7.
            spec['name']+='-s'+str(stage)
            path=BASE/(spec['name']+'.json');path.write_text(json.dumps(spec,indent=2)+'\n');out=BASE/'runs'/spec['name']
            if out.exists():
                assert json.loads((out/'capture-complete.json').read_text())=={'status':'complete'}
                assert json.loads((out/'scenario.json').read_text())==spec
            else:out=run(path)
            name='graphics-requests.jsonl'if'graphics-'in fixture.name else'machines.jsonl'
            rows=[json.loads(line)for line in(out/name).read_text().splitlines()]
            actual.extend(rows)
            report.append({'fixture':str(fixture.relative_to(ROOT)),'fixtureSHA256':hashlib.sha256(fixture.read_bytes()).hexdigest(),'originalScenario':originalName,'replayScenario':spec['name'],'records':len(rows),'ramSHA256':hashlib.sha256((out/'ram.bin').read_bytes()).hexdigest()})
        assert actual==d['records'],'original replay differs: '+fixture.name
    dest=ROOT/'docs/research/nes-contra/evidence/enemies-machines-replay.json'
    if args.fixture and dest.exists():
        old=json.loads(dest.read_text());assert old['passed']
        changed={r['fixture']for r in report}
        report=[r for r in old['runs']if r['fixture']not in changed]+report
    result={'passed':True,'scope':'Exact original replay only; independent-engine tests are a separate check. Saved stage entry overrides and controlled function-boundary interventions remain declared.','runs':report}
    dest.write_text(json.dumps(result,indent=2)+'\n')
    print(json.dumps({'passed':True,'runs':len(report),'records':sum(r['records']for r in report)}))

if __name__=='__main__':main()
