#!/usr/bin/env python3
"""Bounded retained-listing check for direct PSG stores in the capture points."""
import argparse
import ast
import hashlib
import json
import re
from pathlib import Path


def sha(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def main():
    parser=argparse.ArgumentParser()
    parser.add_argument('--physical-filter-check')
    parser.add_argument('--out',default='games/yuyuhakusho/verification/root-control-orchestration/observer-coverage.json')
    args=parser.parse_args()
    root=Path.cwd()
    listing=root/'docs/research/md-yuyuhakusho/static/audio-loop-sequencer.txt'
    capture=root/'scripts/yuyuhakusho/root-control-capture.py'
    tree=ast.parse(capture.read_text())
    points=None
    for node in tree.body:
        if isinstance(node,ast.Assign) and any(isinstance(target,ast.Name) and target.id=='POINTS' for target in node.targets):
            points=set(ast.literal_eval(node.value))
    assert points is not None
    sites=[]
    for line in listing.read_text().splitlines():
        match=re.fullmatch(r'([0-9a-f]{8}) (move\.b .*,\(0x00c00011\)\.l)',line)
        if match:
            address=int(match[1],16)
            sites.append({'address':address,'hex':f'{address:x}','instruction':match[2],'point_observed':address in points})
    assert len(sites)==13
    physical=None
    physical_route=False
    if args.physical_filter_check:
        physical_path=root/args.physical_filter_check
        physical=json.loads(physical_path.read_bytes())
        assert physical['pass'] and physical['type4_byte_C00011_forward'] and physical['all_13_unselected_sites_forward']
        for binding in [physical['source'],*physical['include_sources']]:
            assert sha(root/binding['path'])==binding['sha256'],binding['path']
        branches=[node for node in ast.walk(tree) if isinstance(node,ast.If) and isinstance(node.test,ast.Compare) and isinstance(node.test.left,ast.Name) and node.test.left.id=='kind' and len(node.test.comparators)==1 and isinstance(node.test.comparators[0],ast.Constant) and node.test.comparators[0].value==4]
        psg_updates=[node for node in ast.walk(tree) if isinstance(node,ast.Call) and any(keyword.arg=='kind' and isinstance(keyword.value,ast.Constant) and keyword.value.value=='psg-request' for keyword in node.keywords)]
        physical_updates=[]
        for branch in branches:
            for statement in branch.body:
                physical_updates += [node for node in ast.walk(statement) if node in psg_updates]
        correct_entry=any(isinstance(node,ast.Attribute) and isinstance(node.value,ast.Name) and node.value.id=='b' and node.attr==physical['entrypoint'] for node in ast.walk(tree))
        physical_route=correct_entry and len(psg_updates)==len(physical_updates)==1
    result={'version':1,'scope':'Direct literal PSG byte stores in retained audio-loop-sequencer listing only; not all ROM/MMIO or event decoder completeness',
            'pass':all(site['point_observed'] for site in sites) or physical_route,'sites':sites,
            'physical_route_verified':physical_route,
            'coverage_route':'actual-byte-store' if physical_route else 'selected-PC-markers',
            'physical_filter_check':None if physical is None else {'path':args.physical_filter_check,'sha256':sha(root/args.physical_filter_check)},
            'missing_points':[site['hex'] for site in sites if not site['point_observed']],
            'sources':[{'path':str(path.relative_to(root)),'sha256':sha(path)} for path in [listing,capture]],
            'whole_goal_complete':False,'full_port_ledger_proven':False}
    (root/args.out).write_text(json.dumps(result,sort_keys=True,indent=2)+'\n')
    print(json.dumps({'pass':result['pass'],'sites':len(sites),'route':result['coverage_route'],'missing':result['missing_points']}))


if __name__=='__main__':
    main()
