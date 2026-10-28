#!/usr/bin/env python3
"""Observe base-boss deaths and respawns after a declared boss-entry setup.

The original performs the room transition. Its temporary research barrier is
then removed once and enemy attacks proceed normally; player state, damage,
death timers, lives and respawn positions are never assigned by this script.
"""
import argparse
import gzip
import hashlib
import json
from pathlib import Path

from static_capture_player_modes import ROOT,BASE,FIELDS,GLOBALS,mode_hook
from capture_reference import run


def main():
    parser=argparse.ArgumentParser();parser.add_argument('--label',required=True);args=parser.parse_args()
    original=json.loads(gzip.decompress((ROOT/'docs/research/nes-contra/evidence/static-player-modes.json.gz').read_bytes()))
    source=next(s for s in original['scenarios'] if s['name']=='base2-boss')
    ready=f'base2-boss-ready-{args.label}'
    spec=dict(source['scenario'],name=f'player-boss-setup-{args.label}',frames=200,captures=[200],save_states={'200':ready})
    spec['segments']=[dict(s,end=min(200,s['end'])) for s in source['scenario']['segments'] if s['start']<=200]
    path=BASE/(spec['name']+'.json');path.write_text(json.dumps(spec,indent=2)+'\n');setup=run(path)
    ram=(setup/'ram.bin').read_bytes();assert ram[-2048+0x40]==128
    baseline=BASE/'runs'/source['scenario']['name']/'ram.bin'
    assert ram==baseline.read_bytes()[:200*2048], 'boss construction differs from retained original prefix'
    hook="local a=memory.readbyte(0xb0);local b=memory.readbyte(0xb1);memory.writebyte(0xb0,0);memory.writebyte(0xb1,0);writefile('barrier-release.json',string.format('{\"before\":[%d,%d],\"after\":[0,0]}',a,b))\n"+mode_hook()
    spec={'name':f'player-boss-lifecycle-{args.label}','load_state':ready,'frames':2400,'segments':[],
          'captures':[1,600,2400],'hooks_lua':hook,'timeout_seconds':90}
    path=BASE/(spec['name']+'.json');path.write_text(json.dumps(spec,indent=2)+'\n');out=run(path)
    steps=[json.loads(s) for s in (out/'player-steps.jsonl').read_text().splitlines()]
    maps=[json.loads(s) for s in (out/'player-maps.jsonl').read_text().splitlines()]
    wi={n:i for i,(n,_) in enumerate(GLOBALS)}
    counts={kind:sum(s['kind']==kind and s['before']['world'][wi['sceneType']]==128 for s in steps) for kind in ['normal','dying','spawn']}
    assert counts['dying']>0 and counts['spawn']>0,counts
    record={'name':'base-boss-lifecycle','source':json.loads((out/'source.json').read_text()),'scenario':spec,'maps':maps,'steps':steps,
            'initial_setup':{'source':source['source'],'additional_intervention':source['additional_intervention'],
                             'reproduced_prefix_sha256':hashlib.sha256(ram).hexdigest(),'snapshot_sha256':hashlib.sha256((BASE/'states'/f'{ready}.fcs').read_bytes()).hexdigest()},
            'barrier_release':json.loads((out/'barrier-release.json').read_text()),
            'trace_sha256':hashlib.sha256((out/'player-steps.jsonl').read_bytes()).hexdigest(),'ram_sha256':hashlib.sha256((out/'ram.bin').read_bytes()).hexdigest()}
    fixture={'schema':1,'source_is_original_execution':True,'scope':'controlled boss-room setup; one barrier release; subsequent unmodified enemy-driven deaths and respawns',
             'player_fields':[{'name':n,'address':hex(a)} for n,a in FIELDS],'world_fields':[{'name':n,'address':hex(a)} for n,a in GLOBALS],
             'scenarios':[record],'boss_lifecycle_counts':counts}
    packed=gzip.compress(json.dumps(fixture,separators=(',',':')).encode(),mtime=0)
    dest=ROOT/'docs/research/nes-contra/evidence/static-player-boss-lifecycle.json.gz'
    if dest.exists():
        old=json.loads(gzip.decompress(dest.read_bytes()))
        assert old['scenarios'][0]['steps']==steps and old['scenarios'][0]['maps']==maps
    else:dest.write_bytes(packed)
    print(json.dumps({'passed':True,'counts':counts,'fixture_sha256':hashlib.sha256(dest.read_bytes()).hexdigest()}))


if __name__=='__main__':main()
