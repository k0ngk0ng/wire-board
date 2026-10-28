#!/usr/bin/env python3
"""Project retained original scene and boundary evidence into one replay fixture."""
import gzip
import hashlib
import json
from pathlib import Path

ROOT=Path(__file__).resolve().parents[2]
EVIDENCE=ROOT/'docs/research/nes-contra/evidence'


def main():
    sources=[];records=[]
    for name in ['static-player-modes.json.gz','static-player-mode-boundaries.json.gz','static-player-boss-lifecycle.json.gz']:
        raw=(EVIDENCE/name).read_bytes();records.append(json.loads(gzip.decompress(raw)))
        sources.append({'path':str((EVIDENCE/name).relative_to(ROOT)),'sha256':hashlib.sha256(raw).hexdigest()})
    modes,boundaries,boss_life=records
    assert modes['player_fields']==boundaries['player_fields'] and modes['world_fields']==boundaries['world_fields']
    assert modes['player_fields']==boss_life['player_fields'] and modes['world_fields']==boss_life['world_fields']
    index={f['name']:i for i,f in enumerate(modes['player_fields'])}
    blocked=0
    for row in boundaries['scenarios'][0]['steps']:
        name=boundaries['cases'][row['boundaryCase']-1]['name'];before=row['before']['player'];after=row['after']['player']
        if name in ['peer-entering','peer-dying','peer-waiting','peer-jumping']:
            assert after[index['sequence']]==0 and after[index['indoorAdvance']]==0
            assert row['before']['peer']==row['after']['peer'];blocked+=1
        if name=='forced-already-airborne':
            assert before[index['y']]==after[index['y']]
            assert before[index['vyLo']]==after[index['vyLo']] and before[index['vyHi']]==after[index['vyHi']]
        if name=='restore-anchors':
            assert after[index['y']]==before[index['indoorAnchorY']]
            assert after[index['yFraction']]==(before[index['yFraction']]+before[index['indoorVyLo']])&255
    assert blocked==4
    output=dict(modes)
    output['scenarios']=modes['scenarios']+boundaries['scenarios']+boss_life['scenarios']
    output['sources']=sources
    output['controlled_boundary_cases']=boundaries['cases']
    output['scope']='Original scene execution, 12 staged function boundaries and original boss deaths after a declared entry setup; no natural campaign completion claim.'
    raw=gzip.compress(json.dumps(output,separators=(',',':')).encode(),mtime=0)
    path=ROOT/'games/contra/engine/testdata/original-player-modes.json.gz';path.write_bytes(raw)
    summary={'source_is_original_execution':True,'source_evidence':sources,'fixture_sha256':hashlib.sha256(raw).hexdigest(),
             'calls':sum(len(s['steps']) for s in output['scenarios']),'blocked_peer_gate_cases':blocked,
             'scene_coverage':modes['coverage'],'boundary_cases':len(boundaries['cases']),
             'boss_lifecycle_counts':boss_life['boss_lifecycle_counts'],
             'repeat_labels':{'scene':['v2','v3'],'boundaries':['v2','v3'],'boss_lifecycle':['v1','v2']}}
    path.with_name('original-player-modes-source.json').write_text(json.dumps(summary,indent=2)+'\n')
    print(json.dumps(summary))


if __name__=='__main__':main()
