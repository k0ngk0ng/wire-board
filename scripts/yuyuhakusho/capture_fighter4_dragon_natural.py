#!/usr/bin/env python3
"""Append natural dragon evidence using a separately frozen original control.

The initial capture script, Go/tests, boundaries, sequences and semantic rules
remain untouched. This observer only replays declared physical inputs.
"""
import argparse
from collections import Counter
import ctypes as C
import gzip
import json
from pathlib import Path
from reference import ROOT, ReferenceSession, buttons_at, sha
from capture_animation_records import selected_data
from capture_fighter4_projectile import ENTRIES, n


def main():
    parser=argparse.ArgumentParser()
    for name in ['input','reference','manifest','out']: parser.add_argument('--'+name,type=Path,required=True)
    args=parser.parse_args(); paths=[p.resolve() for p in [args.input,args.reference,args.manifest,args.out]]
    assert all(p.is_relative_to(ROOT) for p in paths)
    input_path,reference_path,manifest_path,out=paths;out.mkdir(parents=True,exist_ok=True)
    target=ROOT/'games/yuyuhakusho/engine/testdata'
    old_packed=(target/'fighter4-projectile-oracle.json.gz').read_bytes();old_raw=gzip.decompress(old_packed)
    old_manifest_raw=(target/'fighter4-projectile-oracle-manifest.json').read_bytes();old_manifest=json.loads(old_manifest_raw)
    prior_ledger_path=target/'fighter4-projectile-natural-append.json'
    prior_ledger_raw=prior_ledger_path.read_bytes() if prior_ledger_path.is_file() else None
    old=json.loads(old_raw);assert sha(old_raw)==old_manifest['fixture_sha256'] and sha(old_packed)==old_manifest['gzip_sha256']
    source=str(input_path.relative_to(ROOT));assert not any(b['input']==source for b in old_manifest['baselines']),'input already appended'
    library={json.dumps(row['key'],sort_keys=True):row for row in old['library']}
    source_manifest=json.loads(manifest_path.read_text());expected=json.loads(reference_path.read_text());events=json.loads(input_path.read_text())
    assert source_manifest['identity']['core_sha256']=='1d08b724cf975de17af146f9ba4292946a609b26d03da3d0d9414c90983095da'
    assert source_manifest['observer_attached'] is False
    assert source_manifest['input']['sha256']==sha(input_path.read_bytes()) and source_manifest['output']['sha256']==sha(reference_path.read_bytes())
    assert source_manifest['frames']==len(expected) and source_manifest.get('drain_frames',2)==2
    entry_limit=source_manifest.get('entry_frame_limit',len(expected)-2)
    core=ROOT/'.local/md-yuyuhakusho/tools/Genesis-Plus-GX-60bee4c724859710ce05672239e5b601ad01382e/genesis_plus_gx_libretro.dylib'
    bridge_path=ROOT/'.local/md-yuyuhakusho/tools/hook_filter.dylib';bridge=C.CDLL(str(bridge_path))
    cbtype=C.CFUNCTYPE(None,C.c_int,C.c_int,C.c_uint,C.c_uint)
    bridge.yyh_set_observer.argtypes=[cbtype,C.POINTER(C.c_uint),C.c_size_t];bridge.yyh_set_observer.restype=C.c_int
    points=[*ENTRIES,0x1580,0x3556,0x3574,0x1eb40,0x13c64,0x13d0c,0x13d1e,0x1fae6,0x1faee,0x3982,0x37c56,0x37cac,
            0xfff000,0xfff09a,0xfff0a8,0xfff13a,0xfffff000,0xfffff09a,0xfffff0a8,0xfffff13a]
    pcs=(C.c_uint*len(points))(*points);rom=(ROOT/'.local/md-yuyuhakusho/downloads/yuyuhakusho-jp.bin').read_bytes()
    rows,errors,curve_reads=[],[],[];pending=None;sequence=max([r['sequence'] for r in old['calls']],default=0)

    def snapshot(ram,parent):
        return {'object_id':parent,'family':ram[parent:parent+1024].hex(),'pool':{'cursor':n(ram,0xa036),'bytes':ram[0x9400:0x9c00].hex()},
                'buffers':ram[0xa200:0xa600].hex(),'offsets':[n(ram,0xa004+i*2) for i in range(4)],'pending_sound':ram[0xa030],
                'sound_queue':list(ram[0xf80a:0xf80e]),'random_seed':n(ram,0xff18,4),'history':ram[0x7000:0x7800].hex(),'bank_cursor':n(ram,0xa038),
                # JumpParameters belongs to the separate MAIN creator API.
                # None of these three callbacks accepts that input context.
                # Preserve actual table observations separately; +90 and all
                # parent bytes remain part of the exact state comparison.
                'parameters':{},
                'movement_parameters_observed':{'source':n(ram,parent+0x90,4),'data':{key:n(rom,n(ram,parent+0x90,4)+delta,4,True) for key,delta in [('vertical_velocity',4),('gravity',8),('horizontal_velocity',12)]}},
                'environment':{'pause_flags':ram[0xf550],'slow_flag':ram[0xf55a],'palette_attributes':n(ram,0xa03e),'arena_left':n(ram,0xa010,2,True),
                               'arena_right':n(ram,0xa012,2,True),'camera_x':n(ram,0xa014,2,True),'camera_y':n(ram,0xa016,2,True),'shake':n(ram,0xf566,2,True)}}

    def draw(ram,off):
        return {'ObjectID':off,'GraphicsSet':n(ram,off+0x1c,4),'TileBank':n(ram,off+0x20,4),'Parameter':n(ram,off+0x3c),
                'ScreenX':n(ram,off+8),'ScreenY':n(ram,off+12),'OffsetX':n(ram,off+0x28,2,True),'OffsetY':n(ram,off+0x2a,2,True),
                'Attributes':n(ram,off+0x24),'Depth':n(ram,off+0x3e),'Layer':ram[off+0x26],'SkipTiles':ram[off+0x27],
                'Cache':[n(ram,off+0x2c+i*4,4) for i in range(4)]}

    with ReferenceSession(ROOT,out/'observed',core=core) as ref:
        @cbtype
        def observe(kind,width,pc,value):
            nonlocal pending,sequence
            try:
                raw_pc=pc;pc&=0xffffff
                if pending is None and pc not in ENTRIES:return
                if pc==0xfff000:
                    pending['_irq'].append({'entry':hex(pc),'entry_register':hex(raw_pc),'_ram':ref.ram_cpu()});return
                if pc in [0xfff09a,0xfff0a8,0xfff13a]:
                    if pending['_irq']:
                        irq=pending['_irq'].pop();ram=ref.ram_cpu();irq.update(exit=hex(pc),changes=[{'offset':i,'before':a,'after':b}for i,(a,b)in enumerate(zip(irq.pop('_ram'),ram))if a!=b]);pending['interrupts'].append(irq)
                    return
                if pc in [0x37c56,0x37cac]:
                    a=ref.get_register(9)&65535;pending['external_queue_clears'].append({'pc':hex(pc),'address':hex(a),'before':ref.ram_cpu()[a]});return
                if pc==0x3982:pending['rng_calls']+=1;return
                address,sp=ref.get_register(13)&65535,ref.get_register(15);ram=ref.ram_cpu()
                if pc in ENTRIES:
                    if ref.frame>entry_limit:return
                    parent=(address&0xfc00)if address<0x9400 else(n(ram,address+0x48)&0xfc00)
                    if parent not in range(0x8400,0x9400,1024)or ram[parent+0x58]!=4:return
                    assert pending is None,'overlapping callbacks'
                    pending={'kind':ENTRIES[pc],'frame':ref.frame,'scenario':source,'object_id':address,'family_base':parent,'entry_sp':sp,
                             'source_register':hex(ref.get_register(13)),'before':snapshot(ram,parent),'draw':False,'cleared_bytes':0,'rng_calls':0,
                             'external_queue_clears':[],'interrupts':[],'allocations':[],'curve_reads':[],'_irq':[],'_ram':ram}
                    return
                if pc in [0x13c64,0x13d0c,0x13d1e]:
                    idx=ref.get_register(0)&65535;idx=idx if idx<32768 else idx-65536;addr=(ref.get_register(8)+idx)&0xffffff
                    reading={'pc':hex(pc),'address':addr,'index':(addr-0x34000)//2,'value':n(rom,addr,2,True)}
                    assert -32<=reading['index']<=1311
                    pending['curve_reads'].append(reading);curve_reads.append(reading)
                if pc in [0x1fae6,0x1faee]:pending['allocations'].append(ref.get_register(8)&65535)
                if pc==0x1eb40:
                    key,data,origin=selected_data(rom,ram,ram[address:address+256]);identity=json.dumps(key,sort_keys=True)
                    assert identity in library and library[identity]['data']==data,'natural AN differs from frozen semantic library'
                if pc==0x3556:pending['cleared_bytes']=(n(ram,address+4)or 64)//16*16
                if pc==0x1580 or pc==0x3574 and pending['cleared_bytes']:
                    row=pending;pending=None;assert sp==row['entry_sp']and not row.pop('_irq')
                    sequence+=1;row.update(after=snapshot(ram,row['family_base']),draw=pc==0x1580,exit=hex(pc),exit_frame=ref.frame,sequence=sequence)
                    if row['draw']:row['draw_request']=draw(ram,address)
                    parent=row['family_base'];history_allowed={0x7000+b*512+i for b in range(4)for i in [*range(0x140),*range(0x140,0x180,2)]}
                    allowed=set(range(parent,parent+1024))|set(range(0x9400,0x9c00))|history_allowed|set(range(0xa200,0xa600))|set(range(0xa004,0xa00c))|{0xa030,0xa036,0xa037,0xa038,0xa039}|set(range(0xf80a,0xf80e))|{(row['entry_sp']-i)&65535 for i in range(0x180)}
                    row['ram_changes']=[{'offset':i,'before':a,'after':b}for i,(a,b)in enumerate(zip(row.pop('_ram'),ram))if a!=b]
                    row['external_changes']=[c for c in row['ram_changes']if c['offset']not in allowed];row['unexplained_changes']=[]
                    for change in row['external_changes']:
                        current,steps=change['before'],0
                        for irq in row['interrupts']:
                            for delta in irq['changes']:
                                if delta['offset']==change['offset']:
                                    assert delta['before']==current;current,steps=delta['after'],steps+1
                        if steps==0 or current!=change['after']:row['unexplained_changes'].append(change)
                    assert not row['unexplained_changes']and row['rng_calls']==0 and row['before']['random_seed']==row['after']['random_seed']
                    rows.append(row)
            except Exception as exc:errors.append(repr(exc))
        assert bridge.yyh_set_observer(observe,pcs,len(points))
        hook=ref.api('set_cpu_hook',None,C.c_void_p);hook(C.cast(bridge.yyh_cpu_hook,C.c_void_p));frames=[]
        for frame in range(1,len(expected)+1):frames.append(ref.step(frame,buttons_at(events,frame))['reference'])
        hook(None);assert frames==expected and pending is None and not errors,(errors,pending is not None)
        identity=ref.identity
    assert rows,'no natural dragon callbacks under declared physical input'
    combined=old['calls']+rows;raw=(json.dumps({'library':old['library'],'curve':old['curve'],'calls':combined},separators=(',',':'))+'\n').encode();packed=gzip.compress(raw,mtime=0)
    baseline={'input':source,'input_sha256':sha(input_path.read_bytes()),'reference':str(reference_path.relative_to(ROOT)),'reference_raw_sha256':sha(reference_path.read_bytes()),
              'reference_manifest':str(manifest_path.relative_to(ROOT)),'reference_manifest_sha256':sha(manifest_path.read_bytes()),'video_steps':len(expected),'all_ram_video_match':True}
    report=dict(old_manifest);report.update(scope='Actual full source4 dragon callbacks under ordinary and appended declared real-menu physical inputs',identity=identity,
        capture_sha256=sha(Path(__file__).read_bytes()),records=len(combined),calls=len(combined),kind_counts=dict(Counter(r['kind']for r in combined)),
        fixture_sha256=sha(raw),gzip_sha256=sha(packed),baselines=old_manifest['baselines']+[baseline],curve_read_count=sum(len(r['curve_reads'])for r in combined),
        curve_min_index=min((r['index']for c in combined for r in c['curve_reads']),default=None),curve_max_index=max((r['index']for c in combined for r in c['curve_reads']),default=None),
        rng_calls=sum(r['rng_calls']for r in combined),unexplained_change_calls=sum(bool(r['unexplained_changes'])for r in combined),
        interrupt_calls=sum(len(r['interrupts'])for r in combined),external_change_calls=sum(bool(r['external_changes'])for r in combined),external_queue_calls=sum(bool(r['external_queue_clears'])for r in combined),
        errors=[],limits=['Each callback stops at1580 or completed3556; hardware sprites are outside this observer scope.',
                          'Natural activity is bounded by the declared legal input/control and is not asserted to cover swallowing or boosted child unless observed.'])
    report['jump_parameter_context']='Empty for appended natural head/trail/boost: these APIs do not consume MAIN JumpParameters. Actual current +90/table is preserved in movement_parameters_observed; complete parent/family bytes remain exact.'
    (out/'fighter4-projectile-oracle.json.gz').write_bytes(packed);(out/'fighter4-projectile-oracle-manifest.json').write_text(json.dumps(report,indent=2)+'\n')
    append={'scope':'Append natural CPU evidence only; frozen Go/test/boundaries/sequences/rules are unchanged','previous_oracle_raw_sha256':sha(old_raw),'previous_oracle_gzip_sha256':sha(old_packed),
            'previous_manifest_sha256':sha(old_manifest_raw),'previous_manifest_snapshot':old_manifest,'appended_baseline':baseline,'appended_records':len(rows),
            'appended_kind_counts':dict(Counter(r['kind']for r in rows)),'appended_frame_range':[min(r['frame']for r in rows),max(r['frame']for r in rows)],
            'combined_records':len(combined),'new_fixture_raw_sha256':sha(raw),'new_fixture_gzip_sha256':sha(packed),'capture_sha256':sha(Path(__file__).read_bytes()),
            'frozen_bindings':{str(p.relative_to(ROOT)):sha(p.read_bytes())for p in [ROOT/'games/yuyuhakusho/engine/fighter4_projectile.go',ROOT/'games/yuyuhakusho/engine/fighter4_projectile_test.go',
                               target/'fighter4-projectile-boundaries.json.gz',target/'fighter4-projectile-sequences.json.gz',target/'fighter4-projectile-rules.json',ROOT/'scripts/yuyuhakusho/capture_fighter4_projectile.py']}}
    if prior_ledger_raw is not None:
        append['previous_append_report_sha256']=sha(prior_ledger_raw)
        append['previous_append_report_snapshot']=json.loads(prior_ledger_raw)
    (out/'fighter4-projectile-natural-append.json').write_text(json.dumps(append,indent=2)+'\n');print(json.dumps({'records':len(rows),'kinds':append['appended_kind_counts'],'frame_range':append['appended_frame_range'],'baseline_match':True,'unexplained':report['unexplained_change_calls']}),flush=True)


if __name__=='__main__':main()
