#!/usr/bin/env python3
"""Original base/boss/vertical player observations before scene support.

Failure cases: treating bases as horizontal outdoor; missing shock/clear gate;
resetting stored transition anchors; changing only one cooperative actor;
premature forced jump; losing subpixel carry or vertical camera consumption;
wrong actor's jump flag in boss aiming; sprite-stage transition sounds omitted.
Initial stage selection and optional boss-room setup are explicitly controlled;
subsequent player inputs and captured calls execute the unmodified original.
"""
import argparse
import gzip
import hashlib
import json
import os
from pathlib import Path
import shutil

ROOT=Path(__file__).resolve().parents[2]
BASE=ROOT/'.local/contra-static/modes-oracle'
os.environ['CONTRA_REFERENCE_ROOT']=str(BASE)
from static_capture_playerstep import PLAYER_FIELDS, WORLD_FIELDS, ENTRIES, HOOK
from capture_reference import run

FIELDS=PLAYER_FIELDS+[('indoorAnchorY',0xa8),('indoorAnchorX',0xbe)]
GLOBALS=WORLD_FIELDS+[('indoorScroll',0x73)]
MODE_ENTRIES=dict(ENTRIES)
MODE_ENTRIES.update({0xd762:'shock',0xd771:'indoorMotion',0xd78a:'endIndoorMotion',0xd7d1:'beginIndoorMotion',0xd63c:'upInput'})


def mode_hook():
    hook=HOOK.replace('PLAYER_FIELDS','{'+','.join(hex(a) for _,a in FIELDS)+'}')
    hook=hook.replace('WORLD_FIELDS','{'+','.join(hex(a) for _,a in GLOBALS)+'}')
    hook=hook.replace('"world":%s,"map":%d}', '"world":%s,"screenCleared":%d,"map":%d}')
    hook=hook.replace('array_fields(worldFields,0),map_id()', 'array_fields(worldFields,0),read(0x6a),map_id()')
    hook=hook.replace('"aimContext":%d,"before":%s', '"kind":"%s","aimContext":%d,"before":%s')
    hook=hook.replace('active.actor,active.aimContext,active.before', 'active.actor,active.kind,active.aimContext,active.before')
    hook=hook.replace('"calls":[%s]}', '"sounds":[%s],"calls":[%s]}')
    hook=hook.replace('active.beforePool,pool(),table.concat(parts,', 'active.beforePool,pool(),table.concat(active.sounds,\',\'),table.concat(parts,')
    hook=hook.replace('entryHooks[0xd45e]=function()', 'local function rootHook(address,kind)\nentryHooks[address]=function()')
    hook=hook.replace('active={id=serial,frame=', 'active={kind=kind,sounds={},id=serial,frame=')
    hook=hook.replace('arm(0xd45e)', "arm(address)\nend\nrootHook(0xd45e,'normal')\nrootHook(0xd3ed,'spawn')\nrootHook(0xd4bd,'dying')\nentryHooks[0xf9bc]=function() if active then active.sounds[#active.sounds+1]=tostring(memory.getregister('a')) end end\narm(0xf9bc)")
    hook=hook.replace('ENTRY_HOOKS','\n'.join(f'hook({hex(a)},"{n}")' for a,n in MODE_ENTRIES.items()))
    return hook


def main():
    parser=argparse.ArgumentParser();parser.add_argument('--label',default='v1');args=parser.parse_args()
    entries=json.loads((ROOT/'docs/research/nes-contra/evidence/campaign-controlled-entries.json').read_text())['entries']
    provenance={e['stage']:e for e in entries}
    for stage in [2,3,4]:
        src=ROOT/f'.local/contra-campaign/states/stage-{stage}-v2.fcs';dest=BASE/f'states/stage-{stage}.fcs';dest.parent.mkdir(parents=True,exist_ok=True)
        assert hashlib.sha256(src.read_bytes()).hexdigest()==provenance[stage]['snapshotSHA256']
        if dest.exists():assert src.read_bytes()==dest.read_bytes()
        else:shutil.copyfile(src,dest)
    controls=[(1,20,1,['up']),(70,150,1,['down']),(165,185,1,['left']),
              (190,190,1,['A','right']),(191,220,1,['right']),(1,240,2,['down'])]
    controls += [(n,n,1,['B']) for n in range(70,151,4)]
    progress=[(1,320,p,['down']) for p in [1,2]]+[(n,n,p,['B']) for p in [1,2] for n in range(1,1201,4)]
    progress += [(340,500,p,['up']) for p in [1,2]]+[(800,1000,p,['up']) for p in [1,2]]
    climb=[(1,140,p,['right','up']) for p in [1,2]]+[(141,900,p,['up']) for p in [1,2]]
    climb += [(n,n,p,['A']) for p in [1,2] for n in range(150,900,70)]
    climb += [(n,n,p,['B']) for p in [1,2] for n in range(1,900,4)]
    climb += [(350,390,1,['right']),(350,390,2,['left'])]
    boss=[(1,650,p,['up']) for p in [1,2]]+[(n,n,p,['B']) for p in [1,2] for n in range(700,901,4)]
    boss += [(710,710,1,['A']),(740,760,2,['down']),(780,800,1,['left','up']),(820,840,2,['right','up'])]
    specs=[('base2-controls',2,240,controls,False),('base2-progress',2,1200,progress,False),
           ('base4-controls',4,240,controls,False),('waterfall-climb',3,900,climb,False),
           ('waterfall-drop',3,240,[(1,4,1,['down','A']),(1,100,2,['right']),(110,110,2,['A'])],False),
           ('base2-boss',2,900,boss,True)]
    scenarios=[];coverage={'sceneTypes':set(),'scrollTypes':set(),'shock':0,'advancing':0,'forceJump':0,'camera':0};counts={}
    pi={n:i for i,(n,_) in enumerate(FIELDS)};wi={n:i for i,(n,_) in enumerate(GLOBALS)}
    for name,stage,frames,segments,boss_setup in specs:
        hook=mode_hook()
        if boss_setup:
            hook="local before=memory.readbyte(0x64);local cleared=memory.readbyte(0x6a);local inv0=memory.readbyte(0xb0);local inv1=memory.readbyte(0xb1);local target=memory.readbyte(0x58)-1;memory.writebyte(0x64,target);memory.writebyte(0x6a,128);memory.writebyte(0xb0,255);memory.writebyte(0xb1,255);writefile('mode-intervention.json',string.format('{\"screenBefore\":%d,\"screenAfter\":%d,\"clearedBefore\":%d,\"clearedAfter\":128,\"invincibilityBefore\":[%d,%d],\"invincibilityAfter\":[255,255]}',before,target,cleared,inv0,inv1))\n"+hook
        spec={'name':f'player-modes-{args.label}-{name}','load_state':f'stage-{stage}','frames':frames,
              'segments':[{'start':a,'end':b,'player':p,'keys':k} for a,b,p,k in segments],
              'captures':[1,frames],'hooks_lua':hook,'timeout_seconds':90}
        path=BASE/(spec['name']+'.json');path.write_text(json.dumps(spec,indent=2)+'\n');out=run(path)
        steps=[json.loads(s) for s in (out/'player-steps.jsonl').read_text().splitlines()]
        maps=[json.loads(s) for s in (out/'player-maps.jsonl').read_text().splitlines()]
        assert steps
        for st in steps:
            before=st['before'];coverage['sceneTypes'].add(before['world'][wi['sceneType']]);coverage['scrollTypes'].add(before['world'][wi['scrollType']])
            coverage['shock']+=before['player'][pi['shock']]!=0;coverage['advancing']+=before['player'][pi['indoorAdvance']]!=0;coverage['forceJump']+=before['player'][pi['forceJump']]!=0
            coverage['camera']+=st['after']['player'][pi['scrolling']]!=0 and before['world'][wi['scrollType']]==1
            counts[st['kind']]=counts.get(st['kind'],0)+1
        scenarios.append({'name':name,'stage':stage,'initial_stage_source':provenance[stage],
                          'additional_intervention':json.loads((out/'mode-intervention.json').read_text()) if boss_setup else None,
                          'source':json.loads((out/'source.json').read_text()),'scenario':spec,'steps':steps,'maps':maps,
                          'trace_sha256':hashlib.sha256((out/'player-steps.jsonl').read_bytes()).hexdigest(),'ram_sha256':hashlib.sha256((out/'ram.bin').read_bytes()).hexdigest()})
    coverage['sceneTypes']=sorted(coverage['sceneTypes']);coverage['scrollTypes']=sorted(coverage['scrollTypes'])
    assert coverage['sceneTypes']==[0,1,128] and coverage['scrollTypes']==[0,1]
    assert coverage['shock']>0 and coverage['advancing']>0 and coverage['forceJump']>0 and coverage['camera']>0
    fixture={'schema':1,'source_is_original_execution':True,'scope':'controlled stage entries followed by original execution; one declared boss-room shortcut',
             'player_fields':[{'name':n,'address':hex(a)} for n,a in FIELDS],'world_fields':[{'name':n,'address':hex(a)} for n,a in GLOBALS],
             'register_order':['a','x','y','p'],'scenarios':scenarios,'coverage':coverage,'counts':counts}
    packed=gzip.compress(json.dumps(fixture,separators=(',',':')).encode(),mtime=0)
    path=ROOT/'docs/research/nes-contra/evidence/static-player-modes.json.gz'
    if path.exists():
        old=json.loads(gzip.decompress(path.read_bytes()))
        for a,b in zip(old['scenarios'],scenarios,strict=True):assert a['steps']==b['steps'] and a['maps']==b['maps'],f"mode replay differs: {b['name']}"
    else:path.write_bytes(packed)
    summary={'passed':True,'label':args.label,'counts':counts,'coverage':coverage,'fixture_sha256':hashlib.sha256(path.read_bytes()).hexdigest()}
    (BASE/f'modes-{args.label}-summary.json').write_text(json.dumps(summary,indent=2)+'\n');print(json.dumps(summary))


if __name__=='__main__':main()
