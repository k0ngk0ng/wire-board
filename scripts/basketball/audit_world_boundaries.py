"""Audit retained phase boundaries before building a continuous world runner.

This does not run the Go world. It identifies mutations between independently
captured original phases, so the future driver cannot silently inject state.
"""
import collections
import csv
import gzip
import hashlib
import json
from pathlib import Path
from extract_world_actions import decode_world_action_state

ROOT=Path(__file__).resolve().parents[2]
RUNS=ROOT/'.local/nes-basket/runs'


def load(name,stem):
    base=RUNS/name
    path=base/(stem+'-ram.bin')
    raw=path.read_bytes() if path.exists() else gzip.decompress(path.with_suffix('.bin.gz').read_bytes())
    with (base/(stem+'.csv')).open() as f:rows=list(csv.DictReader(f))
    assert len(raw)==len(rows)*4096
    return rows,raw


def compare(left,right):
    return {k:[left[k],right[k]] for k in left if left[k]!=right[k]}


def main():
    actions,a=load('world-actions-cpu-long','world-actions')
    preparations,p=load('world-preparation-cpu-long','world-preparation')
    inputs,i=load('world-inputs-cpu-long','world-inputs')
    assert len(actions)==len(inputs)==len(preparations)+1
    reports=[]
    pairs=[('previous_actions_to_preparation',len(preparations),a,p,True,True),
           ('preparation_to_inputs',len(preparations),p,i,True,False),
           ('inputs_to_actions',len(actions),i,a,True,False)]
    for name,count,left,right,left_after,next_iteration in pairs:
        counts=collections.Counter();examples=[];rng_mismatches=[]
        for n in range(count):
            right_index=n+1 if name=='preparation_to_inputs' else n
            lhs=decode_world_action_state(left[n*4096+2048:(n+1)*4096])
            rhs=decode_world_action_state(right[right_index*4096:right_index*4096+2048])
            delta=compare(lhs,rhs);counts.update(delta.keys())
            if delta and len(examples)<3:examples.append(dict(index=n,fields=list(delta)))
            if next_iteration:
                old=lhs['random'];expected=(old>>1)|((((old>>25)^(old>>17))&1)<<31)
                if expected!=rhs['random']:rng_mismatches.append(n)
        reports.append(dict(boundary=name,records=count,changed_fields=dict(counts),examples=examples,extra_rng_step_mismatches=rng_mismatches if next_iteration else None))
    result=dict(scope='original phase-boundary analysis; not a Go continuous replay',phase_counts=dict(actions=len(actions),preparations=len(preparations),inputs=len(inputs)),source_sha256=dict(actions=hashlib.sha256(a).hexdigest(),preparations=hashlib.sha256(p).hexdigest(),inputs=hashlib.sha256(i).hexdigest()),boundaries=reports)
    out=ROOT/'.local/basketball/world-actions/boundary-audit.json'
    out.write_text(json.dumps(result,ensure_ascii=False,indent=2)+'\n')
    print(json.dumps(reports,ensure_ascii=False))


if __name__=='__main__':main()
