"""Export original normal and attack geometry source tables, with observed auxiliary data."""
import argparse
import hashlib
import json
from pathlib import Path
ROOT=Path(__file__).resolve().parents[2]
BASE=ROOT/'.local/nes-basket'


def capture_rider():
    from capture_reference import run
    from capture_contacts import HOOK
    reference=BASE/'runs/rider-normal-reference'
    spec=json.loads((reference/'scenario.json').read_text())
    spec.update(name='contact-rider-natural',hooks_lua=HOOK)
    path=BASE/'contact-rider-natural.json';path.write_text(json.dumps(spec,indent=2)+'\n')
    run(path)
    observed=BASE/'runs'/spec['name']
    assert (observed/'ram.bin').read_bytes()==(reference/'ram.bin').read_bytes()
    for image in reference.glob('frame-*.png'):assert image.read_bytes()==(observed/image.name).read_bytes()
    profiles={}
    for line in (observed/'contacts.jsonl').read_text().splitlines():
        row=json.loads(line)
        for o in row['before']['objects']:
            action=o['action']&127
            key=(o['kind'],action,o['variant'] if action>=96 else 0,o['cursor'])
            value=dict(kind=key[0],action=key[1],variant=key[2],cursor=key[3],shape=o['shape'],attackFlags=o['attackFlags'],receiveFlags=o['receiveFlags'])
            if key in profiles:assert profiles[key]==value
            profiles[key]=value
    target=ROOT/'games/basketball/engine/testdata/contact-rider-profiles.json'
    target.write_text(json.dumps([profiles[k] for k in sorted(profiles)],indent=2)+'\n')
    target.with_name('contact-rider-profiles-source.json').write_text(json.dumps(dict(source_is_original_geometry_data=True,ram_and_screenshots_match=True,records=len(profiles),sha256=hashlib.sha256(target.read_bytes()).hexdigest(),source=json.loads((observed/'source.json').read_text())),indent=2)+'\n')


def main():
    parser=argparse.ArgumentParser();parser.add_argument('--capture-rider',action='store_true');args=parser.parse_args()
    if args.capture_rider:capture_rider()
    rom=(BASE/'basketball.nes').read_bytes()
    assert hashlib.sha256(rom).hexdigest()=='0fa28f60abb8d2ccbbeee90e79e3fa160a5cf32feb90f47ebf1a89168cff569b'
    bank=rom[16+5*16384:16+6*16384]
    word=lambda a:int.from_bytes(bank[a-0x8000:a-0x8000+2],'little')
    action_maps=[word(0x8cf2+2*i) for i in range(8)]
    receive_maps=[word(0x9301+2*i) for i in range(8)]
    assert action_maps==[0x8ed8]*8 and receive_maps==[0x9321]*8
    (ROOT/'games/basketball/engine/testdata/contact-player-shared-source.json').write_text(json.dumps(dict(rom_sha256=hashlib.sha256(rom).hexdigest(),action_maps=action_maps,receive_maps=receive_maps,attack_variant_table='bank5 8D10; indexed by variant independent of character kind'),indent=2)+'\n')
    profiles={}
    for name in ('contact-profiles.json','contact-rider-profiles.json'):
        for row in json.loads((ROOT/'games/basketball/engine/testdata'/name).read_text()):
            if row['kind']<8:row=dict(row,kind=0)
            key=tuple(row[k] for k in ('kind','action','variant','cursor'))
            if key in profiles:assert profiles[key]==row,'conflicting original geometry'
            profiles[key]=row
    # 8FA5..8FE0 is the complete 30-entry normal geometry pointer table.
    # Adjacent unique pointed arrays delimit source frames; 912F is the
    # independent attack-shape base pointer, not another normal frame entry.
    frame_pointers=[word(0x8fa5+2*i) for i in range(30)]
    assert min(frame_pointers)==0x8fe1 and max(frame_pointers)==0x911b
    boundaries=sorted(set(frame_pointers+[0x912f]))
    shapes=[]
    shape_addresses=[]
    for address in frame_pointers:
        end=next(v for v in boundaries if v>address)
        assert (end-address)%2==0
        pointers=[word(a) for a in range(address,end,2)]
        assert all(0x9131<=v<=0x92f9 and (v-0x9131)%8==0 for v in pointers)
        shape_addresses.append(pointers)
        shapes.append([list(bank[v-0x8000:v-0x8000+8]) for v in pointers])
    player_groups=list(bank[0x8ed8-0x8000:0x8f38-0x8000])
    ball_groups=list(bank[0x8f38-0x8000:0x8f51-0x8000])
    player_flags=list(bank[0x9321-0x8000:0x9321-0x8000+96])
    ball_flags=list(bank[0x938b-0x8000:0x938b-0x8000+25])
    assert word(0x8cf2+8*2)==0x8f38 and word(0x9301+8*2)==0x938b
    assert all(v<30 for v in player_groups+ball_groups)
    verified=0
    for key,row in profiles.items():
        if key[0] in (0,8) and key[1]<96:
            groups,flags=(player_groups,player_flags) if key[0]==0 else (ball_groups,ball_flags)
            assert row['shape']==shapes[groups[key[1]]][key[3]]
            assert row['attackFlags']==row['receiveFlags']==flags[key[1]]
            verified+=1
    # Attack frame domains come from the same original animations already
    # exported into attack_profiles.go, not from the current observation keys.
    attack_bank=rom[16+6*16384:16+7*16384]
    attack_word=lambda a:int.from_bytes(attack_bank[a-0x8000:a-0x8000+2],'little')
    variants=set()
    for action in range(9):
        address=attack_word(0xa022+action*2)
        variants.update(attack_bank[address-0x8000:address-0x8000+64:2])
    attack_shapes={};attack_sources=[]
    for variant in sorted(variants):
        animation=attack_word(0xac8f+variant*2);frames=0
        while attack_bank[animation+frames*2-0x8000]<253:
            assert attack_bank[animation+frames*2-0x8000+1]>0
            frames+=1;assert frames<128
        address=word(0x8d10+variant*2)
        ids=list(bank[address-0x8000:address-0x8000+frames])
        pointers=[word(0x912f)+index*8 for index in ids]
        assert all(0x9131<=pointer<=0x92f9 for pointer in pointers)
        attack_shapes[variant]=[list(bank[pointer-0x8000:pointer-0x8000+8]) for pointer in pointers]
        attack_sources.append(dict(variant=variant,animation=animation,frames=frames,geometry_indices_address=address,shape_indices=ids,shape_addresses=pointers))
    attack_flags=list(bank[0x9321-0x8000+96:0x9321-0x8000+106])
    verified_attack=0
    for key,row in profiles.items():
        if key[0]==0 and key[1]>=96:
            assert row['shape']==attack_shapes[key[2]][key[3]]
            assert row['attackFlags']==row['receiveFlags']==attack_flags[key[1]-96]
            verified_attack+=1
    # Restored auxiliary normal action domains have 16 entries. Kind12 has
    # further source entries before kind13's map, so use explicit pointers.
    # The complete source frame arrays also cover idle's retained cursor;
    # geometry does not establish action/capture-response support.
    observed={key:row for key,row in profiles.items() if key[0]>8}
    auxiliary_sources=[]
    for kind in (10,11,12,13,14):
        action_map=word(0x8cf2+kind*2);flag_map=word(0x9301+kind*2)
        assert action_map=={10:0x8f51,11:0x8f61,12:0x8f71,13:0x8f85,14:0x8f95}[kind] and flag_map==0x93a3
        for action in range(16):
            group=bank[action_map+action-0x8000];flags=bank[flag_map+action-0x8000]
            if group>=len(shapes):
                assert (kind,action,group)==(14,3,30)
                auxiliary_sources.append(dict(kind=kind,action=action,action_map=action_map,flag_map=flag_map,group=group,flags=flags,static_supported=False,requires_context=True,restored_context_frames=[0,1,2,3],reason='Original8318 selects current physical controller inputs or canonical world X aliases via pointers 0000/0080; no static substitute',oracle='auxiliary-geometry-dynamic-isolated.json'))
                continue
            for cursor in range(len(shapes[group])):
                observed[(kind,action,0,cursor)]=dict(shape=shapes[group][cursor],attackFlags=flags,receiveFlags=flags)
            auxiliary_sources.append(dict(kind=kind,action=action,action_map=action_map,flag_map=flag_map,group=group,flags=flags,frames=len(shapes[group]),shape_addresses=shape_addresses[group]))
    (ROOT/'games/basketball/engine/testdata/contact-auxiliary-source.json').write_text(json.dumps(dict(rom_sha256=hashlib.sha256(rom).hexdigest(),source_is_original_geometry_data=True,natural_gameplay=False,profiles=auxiliary_sources),indent=2)+'\n')
    signed=lambda v:v if v<128 else v-256
    def shape_literal(s):
        box=lambda offset,minimum:f'ContactBoxSpec{{Enabled: {str(s[offset]!=128).lower()}, MinX: {signed(s[offset+3])}, MaxX: {signed(s[offset+2])}, MinHeight: {minimum}, MaxHeight: {s[offset+1]}}}'
        return f'ContactShape{{Attack: {box(0,signed(s[0]))}, Body: {box(4,s[4])}}}'
    lines=['package engine','','// Original frame geometry. Generated by scripts/basketball/export_contact_profiles.py.',
           '// Normal actions and restored attack animations use original source frame arrays.',
           '// Positions and contact outcomes are computed by the engine, never replayed.']
    for name,values in [('originalPlayerContactGroups',player_groups),('originalBallContactGroups',ball_groups),('originalPlayerContactFlags',player_flags),('originalBallContactFlags',ball_flags),('originalAttackContactFlags',attack_flags)]:
        lines.append(f'var {name} = [{len(values)}]uint8{{'+','.join(str(v) for v in values)+'}')
    lines.append('var originalNormalContactShapes = [30][]ContactShape{')
    for group in shapes:
        lines.append(' {'+','.join(shape_literal(s) for s in group)+'},')
    lines.append('}')
    lines.append('var originalAttackContactShapes = map[uint8][]ContactShape{')
    for variant,group in attack_shapes.items():
        lines.append(str(variant)+':{'+','.join(shape_literal(shape) for shape in group)+'},')
    lines.append('}')
    lines.append('var originalContactProfiles = map[ContactProfileKey]ContactProfile{')
    for key,row in sorted(observed.items()):
        lines.append(f'\t{{Kind: {key[0]}, Action: 0x{key[1]:02x}, Variant: {key[2]}, Frame: {key[3]}}}: {{AttackFlags: 0x{row["attackFlags"]:02x}, ReceiveFlags: 0x{row["receiveFlags"]:02x}, Shape: {shape_literal(row["shape"])}}},')
    lines.append('}')
    (ROOT/'games/basketball/engine/contact_profiles.go').write_text('\n'.join(lines)+'\n')
    source=dict(source_is_original_geometry_data=True,source_is_execution_observation=False,rea_evidence_id='ev_e5ee9618afe7af1df31ab2462a11aa2c6af8d954fd246d832b5a312ab587c8c0',rom_sha256=hashlib.sha256(rom).hexdigest(),normal_geometry_function='bank5 8318',frame_pointer_table=dict(address=0x8fa5,count=30,pointers=frame_pointers,end=0x8fe1),frame_array_end=0x912f,shape_addresses=shape_addresses,player_action_map=dict(address=0x8ed8,values=player_groups),ball_action_map=dict(address=0x8f38,values=ball_groups),player_flags=dict(address=0x9321,values=player_flags),ball_flags=dict(address=0x938b,values=ball_flags),observed_profiles_verified=verified,retained_auxiliary_profiles=len(observed),geometry_availability_does_not_imply_action_implementation=True)
    (ROOT/'games/basketball/engine/testdata/contact-normal-source.json').write_text(json.dumps(source,indent=2)+'\n')
    attack_source=dict(source_is_original_geometry_data=True,source_is_natural_execution=False,rea_evidence_id='ev_8f6ec94a9284a3c93bcce71dc0be376ee57c73f229f13970beaee02150e5a6c7',rom_sha256=hashlib.sha256(rom).hexdigest(),variant_table=0x8d10,shape_base_pointer=0x912f,selection_table_bank6=0xa022,animation_table_bank6=0xac8f,variants=attack_sources,flags=attack_flags,observed_profiles_verified=verified_attack,geometry_availability_does_not_imply_action_implementation=True)
    (ROOT/'games/basketball/engine/testdata/contact-attack-source.json').write_text(json.dumps(attack_source,indent=2)+'\n')
    print(json.dumps(dict(attack_variants=len(attack_shapes),attack_frames=sum(map(len,attack_shapes.values())),observed_attack_profiles_verified=verified_attack,source_groups=len(shapes),source_frame_entries=sum(map(len,shapes)),normal_action_profiles=sum(len(shapes[g]) for g in player_groups+ball_groups),observed_profiles_verified=verified,retained_profiles=len(observed))))


if __name__=='__main__':main()
