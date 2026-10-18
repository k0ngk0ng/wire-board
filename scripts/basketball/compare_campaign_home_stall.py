"""Compare independent Go/original worlds after the declared campaign origin."""
import argparse, collections, hashlib, json
from capture_reference import ROOT, BASE
from extract_world_loop import loop_state
from compare_rest_stall_worlds import normalized, differences


def main():
    parser=argparse.ArgumentParser();parser.add_argument('--mode',choices=['solo','coop'],default='solo');parser.add_argument('--cold',action='store_true');args=parser.parse_args()
    out=ROOT/'.local/basketball/rea/campaign-home-stall'/args.mode
    if args.cold:out=out/'cold'
    meta=json.loads((out/'original-world-result.json').read_text())
    path=BASE/'runs'/('campaign-home-stall-'+args.mode+('-cold' if args.cold else '')+'-observed')/'world-after.ram';raw=path.read_bytes()
    go=out/'go-worlds.jsonl';counts=collections.Counter();first={};strict=0;index=-1
    matches={k:0 for k in ['actors','ball','teamai','input']}
    for index,line in enumerate(go.open()):
        g=normalized(json.loads(line));s=normalized(loop_state(raw[index*2048:(index+1)*2048],index+meta['initialUpdates']))
        ds=[d for d in differences(s,g) if not d[0].startswith('.preparation.contacts.candidates') and not d[0].startswith('.projection')]
        if not ds:strict+=1
        for field,source,actual in ds:
            counts[field]+=1
            if field not in first:first[field]=dict(index=index,source=source,go=actual)
        for key in matches:
            if key in ['actors','ball']:a=s['preparation']['actions'][key];b=g['preparation']['actions'][key]
            elif key=='teamai':a=s['preparation']['teamai'];b=g['preparation']['teamai']
            else:a=s['input'];b=g['input']
            if not list(differences(a,b)):matches[key]+=1
    assert len(raw)==(index+1)*2048 and index+1==meta['worlds']
    result=dict(scope='independent Go and original complete worlds after one controlled origin; reused contact scratch and rendering projection explicitly excluded',mode=args.mode,worlds=index+1,strictMatches=strict,componentMatches=matches,differences=counts,firstDifferences=first,sourceSHA256=hashlib.sha256(raw).hexdigest(),goSHA256=hashlib.sha256(go.read_bytes()).hexdigest())
    (out/'world-comparison.json').write_text(json.dumps(result,indent=2)+'\n');print(json.dumps(result))
    assert not counts,'source differences retained; do not claim equivalent worlds'
    segments=[]
    for mode in ['solo','coop']:
        for cold in [False,True]:
            directory=ROOT/'.local/basketball/rea/campaign-home-stall'/mode
            if cold:directory=directory/'cold'
            if not (directory/'world-comparison.json').exists():return
            capture=json.loads((directory/'original-world-result.json').read_text())
            comparison=json.loads((directory/'world-comparison.json').read_text())
            assert capture['traceSHA256']==comparison['sourceSHA256']
            assert comparison['worlds']==comparison['strictMatches'] and not comparison['differences']
            segments.append(dict(mode=mode,origin='first-playing-world' if cold else 'pre-rolloff-world',capture=capture,comparison=comparison))
    manifest=dict(romSHA256=hashlib.sha256((BASE/'basketball.nes').read_bytes()).hexdigest(),scope='four independently compared original whole-machine segments from declared Go origins; full video RAM and screenshots match unobserved references; not natural original-menu or campaign completion',productionChanged=False,originalFailuresRemain=True,segments=segments)
    (ROOT/'games/basketball/engine/testdata/campaign-home-stall-source.json').write_text(json.dumps(manifest,indent=2)+'\n')

if __name__=='__main__':main()
