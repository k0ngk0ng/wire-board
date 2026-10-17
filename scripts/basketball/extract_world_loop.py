"""Build a single-initial-state core replay; expected snapshots are oracles only."""
import argparse
import csv
import gzip
import hashlib
import json
import re
from pathlib import Path
from extract_world_preparation import state as preparation_state, context as preparation_context
from extract_world_inputs import decode_world_input_state

ROOT=Path(__file__).resolve().parents[2]
RUNS=ROOT/'.local/nes-basket/runs'


def load(run,stem):
    base=RUNS/run;p=base/(stem+'-ram.bin')
    raw=p.read_bytes() if p.exists() else gzip.decompress(p.with_suffix('.bin.gz').read_bytes())
    rows=list(csv.DictReader((base/(stem+'.csv')).open()))
    assert len(raw)==len(rows)*4096
    return rows,raw


def presentation(r):
    word=lambda a:r[a]+256*r[a+1]
    signed=lambda v:v if v<32768 else v-65536
    camera=dict(displayedX=word(0x397),x=word(0x39b),y=word(0x39d),tileColumn=r[0x3a1],shake=r[0x6d5])
    projection=dict(projected=[dict(x=signed(r[0x3e7+i]+256*r[0x3f4+i]),y=signed(r[0x401+i]+256*r[0x40e+i])) for i in range(13)],drawOrder=list(r[0x41d:0x42a]),drawCount=r[0x41c])
    return camera,projection,word(0x399)


def loop_state(r,updates,contacts=None):
    prep=preparation_state(r);inp=decode_world_input_state(r)
    if contacts is not None:
        # The candidate array is reused as CPU scratch after its owning phase.
        # The semantic event result stays at its last preparation value.
        prep['contacts']['candidates']=contacts
    camera,projection,displayed_y=presentation(r)
    return dict(preparation=prep,input=dict(memory=inp['memory'],held=inp['held'],pressed=inp['pressed'],human=inp['human']),camera=camera,projection=projection,logicFrame=r[0x300],started=updates>0,updates=updates,displayedCameraY=displayed_y)


def main():
    p=argparse.ArgumentParser();p.add_argument('--source-run',help='one combined run containing all phase files');p.add_argument('--output',default='world-loop');p.add_argument('--complete-prefix',action='store_true',help='retain every completed iteration when capture ends inside the next iteration; preserve full traces');args=p.parse_args()
    if not re.fullmatch('[a-z][a-z0-9-]*',args.output):raise ValueError('output must be a fixture name')
    action_rows,actions=load(args.source_run or 'world-actions-cpu-long','world-actions')
    input_rows,inputs=load(args.source_run or 'world-inputs-cpu-long','world-inputs')
    prep_rows,preparations=load(args.source_run or 'world-preparation-cpu-long','world-preparation')
    projection_rows,projections=load(args.source_run or 'camera-cpu-long','projection')
    complete=min(len(action_rows),len(input_rows),len(projection_rows),len(prep_rows)+1)
    tails=dict(actions=len(action_rows)-complete,inputs=len(input_rows)-complete,projections=len(projection_rows)-complete,preparations=len(prep_rows)-(complete-1))
    if args.complete_prefix:
        assert complete>0 and all(0<=v<=1 for v in tails.values()),tails
    else:
        assert all(v==0 for v in tails.values()),tails
    first=inputs[:2048];initial=loop_state(first,0)
    pc=preparation_context(first)
    config=dict(actions=pc['actions'],scoringBonuses=pc['scoringBonuses'],objectKinds=list(first[0x3d6:0x3e3]),hoopLinks=list(first[0x649:0x64f]),attackGrades=list(first[0x625:0x629]),humanTactics=list(first[0x666:0x668]),difficulty=first[0x58c],backboardStrength=first[0x56d]>>6,layoutID=first[0x588],trackedObject=first[0x3a2],jumpEnhancements=[first[0x6e6+3*first[0x3e3+i]] for i in range(4)])
    config['pursuitReserved']=dict(action73=first[0x55b],depth73=first[0xe5],depth75=first[0xe7],depth88=first[0xf4],xHigh88=first[0xda])
    config['pursuitAlias74']=dict(actionByte=first[0x55c],depthByte=first[0xe6])
    records=[];candidates=initial['preparation']['contacts']['candidates']
    for i,row in enumerate(input_rows[:complete]):
        before=inputs[i*4096:i*4096+2048];after=projections[i*4096+2048:(i+1)*4096]
        # Configuration is initialized once. Reject captures whose immutable
        # character/arena configuration changes instead of injecting it later.
        assert dict(actionByte=before[0x55c],depthByte=before[0xe6])==config['pursuitAlias74']
        assert dict(actionByte=after[0x55c],depthByte=after[0xe6])==config['pursuitAlias74']
        assert dict(action73=before[0x55b],depth73=before[0xe5],depth75=before[0xe7],depth88=before[0xf4],xHigh88=before[0xda])==config['pursuitReserved']
        assert dict(action73=after[0x55b],depth73=after[0xe5],depth75=after[0xe7],depth88=after[0xf4],xHigh88=after[0xda])==config['pursuitReserved']
        c=preparation_context(before)
        assert c['actions']==config['actions'] and c['scoringBonuses']==config['scoringBonuses']
        assert list(before[0x3d6:0x3e3])==config['objectKinds'] and list(before[0x666:0x668])==config['humanTactics']
        if i>0:candidates=preparation_state(preparations[(i-1)*4096+2048:i*4096])['contacts']['candidates']
        frames=[]
        for actor in range(4):
            slot=before[0x4e8+actor];frames.append(dict(held=before[slot],pressed=before[slot+4]) if slot<4 else dict(held=0,pressed=0))
        records.append(dict(inputFrame=int(row['frame']),projectionFrame=int(projection_rows[i]['frame']),input=dict(frames=frames),expected=loop_state(after,i+1,candidates)))
    result=dict(initial=initial,config=config,records=records)
    payload=(json.dumps(result,separators=(',',':'))+'\n').encode();packed=gzip.compress(payload,mtime=0)
    out=ROOT/'games/basketball/engine/testdata';target=out/(args.output+'.json.gz');target.write_bytes(packed)
    target.with_name(args.output+'-source.json').write_text(json.dumps(dict(source_is_original_execution=True,single_initial_state=True,records=len(records),trailing_phase_records_not_in_a_complete_iteration=tails,full_traces_preserved=True,external_inputs='controller frames only; expected states are never replay inputs; video clock excluded',expected_candidate_semantics='last preparation event result; original zero-page reuse is not persisted',sha256=hashlib.sha256(packed).hexdigest(),uncompressed_sha256=hashlib.sha256(payload).hexdigest(),trace_sha256={n:hashlib.sha256(v).hexdigest() for n,v in [('actions',actions),('inputs',inputs),('preparations',preparations),('projection',projections)]}),indent=2)+'\n')
    print(args.output,len(records),len(packed))


if __name__=='__main__':main()
