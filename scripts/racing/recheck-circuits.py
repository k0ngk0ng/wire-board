#!/usr/bin/env python3
"""Recheck immutable complete browser artifacts without opening a live fixture."""
import argparse
import hashlib
import importlib.util
import json
from pathlib import Path
import traceback

ROOT=Path(__file__).resolve().parents[2]
def local(value):
    path=(ROOT/value).resolve()
    if not path.is_relative_to(ROOT):raise ValueError('evidence must remain in repository')
    return path
def sha(path):return hashlib.sha256(path.read_bytes()).hexdigest()

def main():
    parser=argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--run',required=True);parser.add_argument('--track',required=True,choices=('france','coastal','uk','park'));parser.add_argument('--out',required=True)
    parser.add_argument('--build-identity',default='.local/racing-followup/final-build-identity.json')
    args=parser.parse_args();run=local(args.run);out=local(args.out)
    if out.exists():raise ValueError('use a new output file; original reports are never overwritten')
    source=run/('offroad-circuit-'+args.track+'.json');original=run/'report.json'
    report=json.loads(original.read_text());native=json.loads(source.read_text());identity_path=local(args.build_identity);identity=json.loads(identity_path.read_text())
    spec=importlib.util.spec_from_file_location('nfs_acceptance',ROOT/'scripts/racing/check-nfs.py');module=importlib.util.module_from_spec(spec);spec.loader.exec_module(module)
    result={'passed':False,'method':'offline replay of the complete original production-browser native artifact against the same strict assertions; omitted false autoPlay is read using its protocol default; no live page, fixture, state mutation or new driving',
            'track':args.track,'source':str(source.relative_to(ROOT)),'sourceSHA256':sha(source),'sourceBytes':source.stat().st_size,
            'originalReport':str(original.relative_to(ROOT)),'originalReportSHA256':sha(original),'originalReportPassed':report['passed'],'originalError':report.get('error'),
            'distSHA256':report['distSHA256'],'buildIdentity':str(identity_path.relative_to(ROOT)),'buildIdentitySHA256':sha(identity_path),
            'originalHarnessSHA256':report['harnessSHA256'],'checkerSHA256':{'scripts/racing/recheck-circuits.py':sha(Path(__file__)), 'scripts/racing/check-nfs.py':sha(ROOT/'scripts/racing/check-nfs.py')},'checks':[]}
    try:
        module.require(report['distSHA256']==identity['indexSHA256'],'original browser artifact has a different final build')
        module.require(report['frozenBuildIdentity']['sourceSHA256']==identity['sourceSHA256'],'original browser artifact has a different source freeze')
        module.require(not native.get('exceptions'),'original browser artifact had uncaught runtime exceptions',native.get('exceptions'))
        runner=module.Acceptance.__new__(module.Acceptance);runner.report=result
        runner.validate_offroad_circuit(native,args.track,report['circuitPlan']['selected']['outsideM'])
        own=lambda name:next(c for c in native[name]['room']['game']['data']['cars']if c['seat']==native[name]['room']['you'])
        initial=native['before']['room']['game']['data'];final=own('after')
        result['summary']={'lap':final['lap'],'finishBeforeOriginalDeadlineMS':initial['maxRaceAt']-final['finishAt'],
                           'firstOutsideM':native['first']['maximumOutsideM'],'secondOutsideM':native['second']['maximumOutsideM'],
                           'firstRows':len(native['first']['rows']),'secondRows':len(native['second']['rows']),
                           'resetSpeedMS':own('preReset')['speed']/10,'repeatAfterMS':own('secondReset')['lastReset']-own('reset')['lastReset'],
                           'firstElapsedMS':native['first']['elapsedMS'],'remainingElapsedMS':native['second']['elapsedMS']}
        result['passed']=True
    except Exception as error:result['error']=str(error);result['traceback']=traceback.format_exc()
    out.write_text(json.dumps(result,ensure_ascii=False,indent=2)+'\n');print(json.dumps({k:result.get(k)for k in('passed','track','summary','error')},ensure_ascii=False,indent=2))
    return 0 if result['passed']else 1
if __name__=='__main__':raise SystemExit(main())
