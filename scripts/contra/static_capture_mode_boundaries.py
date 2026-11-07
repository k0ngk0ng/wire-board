#!/usr/bin/env python3
"""Observe explicitly staged original base-player boundary calls.

The cases distinguish peer-state/jump gates, inactive roles, already-advancing
peers, shock cancellation, forced-jump guards and retained anchor fractions.
Every original call is recorded before full RAM restoration. This is not a
natural room clear or campaign run, and no implementation result is generated.
"""
import argparse
import gzip
import hashlib
import json
from pathlib import Path

from static_capture_player_modes import ROOT,BASE,FIELDS,GLOBALS,mode_hook
from capture_reference import run


def cases():
    base={0x6a:128,0x73:0,0x90:1,0xa0:0,0xa4:0,0xb2:0,0xc8:0,0xca:0,0xd0:0,
          0xc0:2,0xf1:8,0xf5:0,0x334:112,0x31a:168,0x94:73,0x92:23,0x98:0,0x96:0,
          0x9a:0,0x9c:0,0xa8:168,0xbe:112,0x9e:0,0xa6:0,0xd2:0,0xda:0,0xb6:0,
          0x91:1,0xa1:0,0x39:0,0xd1:0,0xcb:0,0xf2:0,0xf6:0,0x335:144,0x31b:168,
          0x95:137,0x93:137,0xa9:168,0xbf:144,0x9b:0,0x9d:0,0x99:0,0x97:0,0x9f:0,0xa7:0}
    variations=[('both-ready',{}),('peer-entering',{0x91:0}),('peer-dying',{0x91:2}),
                ('peer-waiting',{0x91:3}),('peer-jumping',{0xa1:17}),
                ('inactive-peer',{0x39:1,0x91:2,0xa1:17}),
                ('peer-already-advancing',{0x73:1,0xd1:1,0x9b:49,0x9d:255,0xa9:160,0xbf:136}),
                ('shock-cleared',{0xc8:1}),('fence-active',{0x6a:0}),
                ('forced-already-airborne',{0xca:1,0xa0:17,0xd0:1,0x31a:130,0x334:100,0xc4:240,0xc6:251,0x98:1,0x96:64}),
                ('forced-from-advance',{0xca:1,0xd0:1,0x31a:130,0x334:100,0x96:32}),
                ('restore-anchors',{0x73:2,0xd0:1,0x31a:135,0x334:100,0x96:32,0x9a:80,0x9c:255})]
    return [{'name':name,'writes':base|changes} for name,changes in variations]


def main():
    parser=argparse.ArgumentParser();parser.add_argument('--label',required=True);args=parser.parse_args()
    variants=cases()
    lua='local boundaryCases={'+','.join('{'+','.join(f'[{a}]={v}' for a,v in sorted(c['writes'].items()))+'}' for c in variants)+'};local boundaryIndex=0\n'
    hook=mode_hook()
    needle="local actor=memory.getregister('x');assert(actor==0 or actor==1)"
    assert needle in hook
    patch="""
 local restore=nil;local boundaryCase=0
 if address==0xd45e and actor==0 and boundaryIndex<#boundaryCases then
  boundaryIndex=boundaryIndex+1;boundaryCase=boundaryIndex
  restore={};for a=0,2047 do restore[a]=read(a) end
  for a,v in pairs(boundaryCases[boundaryIndex]) do memory.writebyte(a,v) end
 end
"""
    hook=hook.replace(needle,needle+patch)
    hook=hook.replace('active={kind=kind,sounds={}', 'active={boundaryCase=boundaryCase,restore=restore,kind=kind,sounds={}')
    hook=hook.replace('"kind":"%s","aimContext":%d','"kind":"%s","boundaryCase":%d,"aimContext":%d')
    hook=hook.replace('active.actor,active.kind,active.aimContext','active.actor,active.kind,active.boundaryCase,active.aimContext')
    hook=hook.replace('    active=nil','    if active.restore then for a=0,2047 do memory.writebyte(a,active.restore[a]) end end\n    active=nil')
    hook=lua+hook
    spec={'name':f'player-mode-boundaries-{args.label}','load_state':'stage-2','frames':len(variants)+180,
          'segments':[],'captures':[],'hooks_lua':hook,'timeout_seconds':90}
    path=BASE/(spec['name']+'.json');path.write_text(json.dumps(spec,indent=2)+'\n');out=run(path)
    all_steps=[json.loads(s) for s in (out/'player-steps.jsonl').read_text().splitlines()]
    steps=[r for r in all_steps if r.get('boundaryCase',0)>0]
    assert len(steps)==len(variants)
    maps=[json.loads(s) for s in (out/'player-maps.jsonl').read_text().splitlines()]
    result={'schema':1,'source_is_original_execution':True,'scope':'controlled-function-boundaries',
            'interventions':'Exact fields staged at actor-zero normal-player entry; full RAM restored after each original call. Not natural game progression.',
            'cases':[{'name':c['name'],'writes':{hex(a):v for a,v in c['writes'].items()}} for c in variants],
            'player_fields':[{'name':n,'address':hex(a)} for n,a in FIELDS],'world_fields':[{'name':n,'address':hex(a)} for n,a in GLOBALS],
            'scenarios':[{'name':'base-function-boundaries','source':json.loads((out/'source.json').read_text()),'scenario':spec,'steps':steps,'maps':maps}],
            'trace_sha256':hashlib.sha256((out/'player-steps.jsonl').read_bytes()).hexdigest(),
            'ram_sha256':hashlib.sha256((out/'ram.bin').read_bytes()).hexdigest()}
    packed=gzip.compress(json.dumps(result,separators=(',',':')).encode(),mtime=0)
    dest=ROOT/'docs/research/nes-contra/evidence/static-player-mode-boundaries.json.gz'
    if dest.exists():
        old=json.loads(gzip.decompress(dest.read_bytes()))
        assert old['scenarios'][0]['steps']==steps and old['scenarios'][0]['maps']==maps
    else:dest.write_bytes(packed)
    print(json.dumps({'passed':True,'cases':len(steps),'fixture_sha256':hashlib.sha256(dest.read_bytes()).hexdigest()}))


if __name__=='__main__':main()
