#!/usr/bin/env python3
"""Source6 shared support phases and exact18682 constructor before integration."""
import argparse
from collections import Counter
import ctypes as C
import gzip
import itertools
import json
from pathlib import Path
from reference import ROOT, ReferenceSession, buttons_at, sha
from capture_animation_records import selected_data

ENTRIES = {0x1ca46: '0e', 0x1ca9c: '0f', 0x1cb08: '11', 0x1cbc8: '13', 0x1bc7e: '37'}
POSES = {'0e': 11, '0f': 11, '11': 12, '13': 0x83, '37': 0xa4}


def n(raw, off, size=2, signed=False):
    return int.from_bytes(raw[off:off+size], 'big', signed=signed)


def main():
    p=argparse.ArgumentParser();p.add_argument('--out',type=Path,required=True);p.add_argument('--natural-only',action='store_true');p.add_argument('--prefix',default='fighter6-support-motion')
    p.add_argument('--inputs',type=Path,default=ROOT/'docs/research/md-yuyuhakusho/source6-next-inputs.json')
    p.add_argument('--reference',type=Path,default=ROOT/'games/yuyuhakusho/engine/testdata/fighter6-attacks-reference-frames.json')
    p.add_argument('--reference-manifest',type=Path,default=ROOT/'games/yuyuhakusho/engine/testdata/fighter6-attacks-reference-manifest.json');args=p.parse_args()
    out=args.out.resolve();assert out.is_relative_to(ROOT)and args.prefix.replace('-','').isalnum();out.mkdir(parents=True,exist_ok=True)
    input_path,reference_path,manifest_path=args.inputs.resolve(),args.reference.resolve(),args.reference_manifest.resolve()
    reference_raw=reference_path.read_bytes();expected=json.loads(reference_raw);events=json.loads(input_path.read_text());original=json.loads(manifest_path.read_text())
    assert original['output']['sha256']==sha(reference_raw)and original['input']['sha256']==sha(input_path.read_bytes())and original['identity']['core_sha256']=='1d08b724cf975de17af146f9ba4292946a609b26d03da3d0d9414c90983095da'
    rom=(ROOT/'.local/md-yuyuhakusho/downloads/yuyuhakusho-jp.bin').read_bytes();bridge_path=ROOT/'.local/md-yuyuhakusho/tools/hook_filter.dylib';bridge=C.CDLL(str(bridge_path))
    cbtype=C.CFUNCTYPE(None,C.c_int,C.c_int,C.c_uint,C.c_uint);bridge.yyh_set_observer.argtypes=[cbtype,C.POINTER(C.c_uint),C.c_size_t];bridge.yyh_set_observer.restype=C.c_int
    points=[*ENTRIES,0x18682,0x1bd20,0x19d90,0x3d8,0x19c96,0x19cc2,0x15afc,0x3982,0x37c56,0x37cac,0xfff000,0xfff09a,0xfff0a8,0xfff13a,0xfffff000,0xfffff09a,0xfffff0a8,0xfffff13a]
    pcs=(C.c_uint*len(points))(*points);natural,isolated,streams,errors=[],[],[],[];library={};pending=None;private=False;requested='';label=''
    core=ROOT/'.local/md-yuyuhakusho/tools/Genesis-Plus-GX-60bee4c724859710ce05672239e5b601ad01382e/genesis_plus_gx_libretro.dylib'

    def snapshot(ram,parent=0x8400):
        obj=ram[parent:parent+256];ptr=n(obj,0x90,4)&0xffffff;memory,index=(rom,ptr)if ptr<len(rom)else(ram,ptr&65535)
        return {'object_id':parent,'family':ram[parent:parent+1024].hex(),'pool':{'cursor':n(ram,0xa036),'bytes':ram[0x9400:0x9c00].hex()},
                'buffers':ram[0xa200:0xa600].hex(),'offsets':[n(ram,0xa004+i*2)for i in range(4)],'pending_sound':ram[0xa030],'sound_queue':list(ram[0xf80a:0xf80e]),'random_seed':n(ram,0xff18,4),
                'parameters':{key:n(memory,index+delta,4,True)for key,delta in[('vertical_velocity',4),('gravity',8),('horizontal_velocity',12)]},
                'environment':{'pause_flags':ram[0xf550],'slow_flag':ram[0xf55a],'palette_attributes':n(ram,0xa03e),'arena_left':n(ram,0xa010,2,True),'arena_right':n(ram,0xa012,2,True),
                               'camera_x':n(ram,0xa014,2,True),'camera_y':n(ram,0xa016,2,True),'shake':n(ram,0xf566,2,True)}}

    with ReferenceSession(ROOT,out/'observed',core=core)as ref:
        @cbtype
        def observe(kind,width,pc,value):
            nonlocal pending
            try:
                raw_pc=pc;pc&=0xffffff
                if pc==0xfff000:
                    if pending is not None:pending['_irq'].append({'entry':hex(pc),'entry_register':hex(raw_pc),'_ram':ref.ram_cpu()})
                    return
                if pc in[0xfff09a,0xfff0a8,0xfff13a]:
                    if pending is not None and pending['_irq']:
                        irq=pending['_irq'].pop();ram=ref.ram_cpu();irq.update(exit=hex(pc),changes=[{'offset':i,'before':a,'after':b}for i,(a,b)in enumerate(zip(irq.pop('_ram'),ram))if a!=b]);pending['interrupts'].append(irq)
                    return
                if pc==0x3982:
                    if pending is not None:pending['rng_calls']+=1
                    return
                if pc in[0x37c56,0x37cac]:
                    if pending is not None:
                        a=ref.get_register(9)&65535;pending['external_queue_clears'].append({'pc':hex(pc),'address':hex(a),'before':ref.ram_cpu()[a]})
                    return
                address,sp=ref.get_register(13)&0xffffff,ref.get_register(15);parent=address&65535;ram=ref.ram_cpu();role=ENTRIES.get(pc)
                if pc==0x18682:role='exit'
                if private and requested=='clock'and pc==0x19c96:role='clock'
                if role and parent in[0x8400,0x8800,0x8c00,0x9000]and ram[parent+0x58]==6 and(private or ref.frame<=3600):
                    assert pending is None
                    pending={'kind':role,'frame':ref.frame,'object_id':parent,'entry_sp':sp,'source_register':hex(ref.get_register(13)),'before':snapshot(ram,parent),
                             'rng_calls':0,'external_queue_clears':[],'interrupts':[],'_irq':[],'_ram':ram}
                elif pending is not None:
                    finish=(pc==0x15afc and pending['kind']in ENTRIES.values()or pc==0x19cc2 and pending['kind']=='clock'or pc in[0x3d8,0x1bd20,0x19d90]and pending['kind']=='exit')
                    if finish:
                        row=pending;pending=None;assert sp==row['entry_sp']+(4 if row['kind']=='exit'else 0)and not row.pop('_irq')
                        row.update(after=snapshot(ram,parent),exit=hex(pc),exit_frame=ref.frame,return_sp=sp)
                        allowed=(set(range(parent,parent+1024))|set(range(0x9400,0x9c00))|set(range(0xa200,0xa600))|set(range(0xa004,0xa00c))|{0xa030,0xa036,0xa037}|set(range(0xf80a,0xf80e))|{(row['entry_sp']-i)&65535 for i in range(0x180)})
                        row['external_changes']=[{'offset':i,'before':a,'after':b}for i,(a,b)in enumerate(zip(row.pop('_ram'),ram))if a!=b and i not in allowed];row['unexplained_changes']=[]
                        for change in row['external_changes']:
                            current,steps=change['before'],0
                            for irq in row['interrupts']:
                                for delta in irq['changes']:
                                    if delta['offset']!=change['offset']:continue
                                    assert delta['before']==current;current,steps=delta['after'],steps+1
                            if steps==0 or current!=change['after']:row['unexplained_changes'].append(change)
                        assert not row['unexplained_changes']and row['rng_calls']==0 and row['before']['random_seed']==row['after']['random_seed']
                        if private:
                            assert not row['external_changes']and not row['external_queue_clears'];row['name']=label;isolated.append(row);ref.cpu.cycles=8000000
                        else:natural.append(row)
            except Exception as exc:
                errors.append(repr(exc))
                if private:ref.cpu.cycles=8000000

        assert bridge.yyh_set_observer(observe,pcs,len(points));hook=ref.api('set_cpu_hook',None,C.c_void_p);observer=C.cast(bridge.yyh_cpu_hook,C.c_void_p);hook(observer);frames=[]
        for frame in range(1,len(expected)+1):
            frames.append(ref.step(frame,buttons_at(events,frame))['reference'])
            if frame==2400:checkpoint,base_ram=ref.save(),ref.ram_cpu()
        hook(None);assert frames==expected and pending is None and not errors,errors[:3];assert base_ram[0x8458]==6
        identity=ref.identity;baselines=[{'input':str(input_path.relative_to(ROOT)),'input_sha256':sha(input_path.read_bytes()),'reference':str(reference_path.relative_to(ROOT)),'reference_raw_sha256':sha(reference_raw),
                                      'reference_manifest':str(manifest_path.relative_to(ROOT)),'reference_manifest_sha256':sha(manifest_path.read_bytes()),'video_steps':len(expected),'all_ram_video_match':True}]
        binding={'source_tag':6,'source_actor_sha256':sha(base_ram[0x8400:0x8500]),'animation_set':n(base_ram,0x8450,4),'shape_set':n(base_ram,0x848c,4),'movement_source':n(base_ram,0x8490,4)}
        if not args.natural_only:
            private=True;setreg=ref.api('m68k_set_reg',None,C.c_uint,C.c_uint);execute=ref.api('m68k_run',None,C.c_uint)

            def write_bytes(address,raw):
                for i,byte in enumerate(raw):C.c_ubyte.from_address(ref.ram_pointer+(((address+i)&65535)^1)).value=byte

            def write(address,value,size=1):write_bytes(address,(value&((1<<(size*8))-1)).to_bytes(size,'big'))

            def setup(role='0e',seed=0,**changes):
                ref.restore(checkpoint,restore_refresh_cycles=True);write_bytes(0xff8400,bytes((i*73+seed*29+17)&255 for i in range(1024)));write_bytes(0xff8450,base_ram[0x8450:0x8454]);write_bytes(0xff848c,base_ram[0x848c:0x8494])
                write_bytes(0xff9400,bytes((i*37+seed*23+13)&255 for i in range(2048)));write_bytes(0xffa200,bytes((i*19+seed*7+5)&255 for i in range(1024)))
                v={'index':0,'duration':8,'ticks':0,'flags':0,'completed':0,'facing':0,'pose':POSES.get(role,0x2b),'x':100<<16|0xabcd,'y':320<<16|0x5678,'ground':320<<16|0x1234,
                   'vx':0x40001,'vy':-0x40001,'gravity':0x123456,'extra_jump':255,'stun':0xabcd,'aux7e':-1,'aux80':128,'pool':'empty','cursor':0};v.update(changes)
                fields={0:(0x12668,4),4:(256,2),6:(2,2),0x58:(6,1),0x5d:(int(role,16)if role!='exit'else 0x38,1),0x5f:(v['pose'],1),0x59:(v['index'],1),0x5a:(v['duration'],1),0x5b:(v['ticks'],1),0x5c:(v['flags'],1),
                        0x60:(v['completed'],1),0x61:(v['facing'],1),0x40:(v['x'],4),0x44:(v['y'],4),0x9c:(v['ground'],4),0x10:(v['vx'],4),0x14:(v['vy'],4),0x4c:(v['gravity'],4),0x87:(v['extra_jump'],1),0x84:(v['stun'],2),0x7e:(v['aux7e'],2),0x80:(v['aux80'],2)}
                for off,(value,count)in fields.items():write(0xff8400+off,value,count)
                for i in range(16):write(0xff9404+i*128,128 if v['pool']=='full'else 0,2)
                write(0xffa036,v['cursor'],2);write(0xffa030,0x97);write(0xffff18,0x12345678,4);write_bytes(0xfff80a,bytes([0,0x80,0,0x81]))

            def run(role,name):
                nonlocal requested,label
                requested,label=role,name
                for reg in range(8):setreg(reg,0)
                setreg(17,0x2700);setreg(15,0xffffff00);write(0xffff00,0x3d8,4);setreg(13,0xffff8400)
                entry=0x18682 if role=='exit'else 0x19c96 if role=='clock'else next(pc for pc,k in ENTRIES.items()if k==role)
                setreg(16,entry);ref.cpu.cycles,ref.cpu.refresh_cycles=0,896;count=len(isolated);hook(observer);execute(8000000);hook(None)
                assert len(isolated)==count+1 and pending is None and not errors,(name,errors[:3]);return isolated[-1]

            def record(name):
                before=snapshot(ref.ram_cpu());ram=ref.ram_cpu();key,data,origin=selected_data(rom,ram,ram[0x8400:0x8500]);identity=json.dumps(key,sort_keys=True)
                assert identity not in library or library[identity]['data']==data;library[identity]={'key':key,'data':data,'source':origin};write(0xff845a,data['duration']);write(0xff845c,data['flags'])
                row={'kind':'record','name':name,'object_id':0x8400,'before':before,'after':snapshot(ref.ram_cpu()),'record_key':key,'rng_calls':0,'external_queue_clears':[],'interrupts':[],'external_changes':[],'unexplained_changes':[]};isolated.append(row);return row

            for seed,(role,completed,facing,y,ticks,index)in enumerate(itertools.product(['0e','0f'],[0,1,2],[0,1,2,255],[320<<16|0x5678,319<<16|0xffff,321<<16],[0,1,255],[0,1,2])):
                setup(role,seed,completed=completed,facing=facing,y=y,ticks=ticks,index=index);run(role,f'dash-{role}-{completed}-{facing}-{y}-{ticks}-{index}')
            for seed,(index,completed,facing,y)in enumerate(itertools.product(range(8),[0,1,2],[0,1,2,255],[320<<16|0x5678,319<<16|0xffff])):
                setup('11',seed,index=index,completed=completed,facing=facing,y=y);run('11',f'retreat-{index}-{completed}-{facing}-{y}')
            for seed,(completed,index,ticks,flags,facing,vx)in enumerate(itertools.product([0,1,2],[0,1,2,3,255],[0,1,255],[0,4],[0,1,2,255],[-0x80000000,-3,0,3,0x7fffffff])):
                setup('13',seed,completed=completed,index=index,ticks=ticks,flags=flags,facing=facing,vx=vx);run('13',f'slide-{seed}')
            for seed,(completed,y,extra,gravity)in enumerate(itertools.product([0,1,2,255],[320<<16|0x5678,319<<16|0xffff],[0,1,255],[-0x80000000,0,0x7fffffff])):
                setup('37',seed,completed=completed,y=y,extra_jump=extra,gravity=gravity);run('37',f'37-{completed}-{y}-{extra}-{gravity}')
            for seed,(aux80,aux7e,stun,extra,facing)in enumerate(itertools.product([-32768,-1,0,1,32767],[-32768,0,32767],[0,128,65535],[0,255],[0,1,255])):
                setup('exit',seed,aux80=aux80,aux7e=aux7e,stun=stun,extra_jump=extra,facing=facing);run('exit',f'18682-{aux80}-{aux7e}-{stun}-{extra}-{facing}')
            for seed,(role,pool,cursor)in enumerate(itertools.product(['0e','0f','13'],['empty','full'],[0,0xff80,0x780])):
                setup(role,seed,index=2,ticks=0,pool=pool,cursor=cursor);run(role,f'pool-{role}-{pool}-{cursor}')
            boundary_count=len(isolated)
            configs=[('0e',11,{}),('0f',11,{}),('11',12,{}),('13',0x83,{}),('13',0x87,{}),('37',0xa4,{}),('37',0xa5,{}),('0e',11,{'y':319<<16|0xffff}),('0f',11,{'y':319<<16|0xffff}),('11',12,{'y':319<<16|0xffff})]
            for stream,((role,pose,changes),pool)in enumerate(itertools.product(configs,['empty','full'])):
                setup(role,stream,pose=pose,pool=pool,**changes);write(0xff845b,255);record('initial-record');initial=snapshot(ref.ram_cpu());calls=[]
                for step in range(512):
                    calls.append(run('clock',f'{stream}/{step}-clock'));calls.append(run(role,f'{stream}/{step}-support'));calls.append(record(f'{stream}/{step}-record'))
                    if ref.ram_cpu()[0x845d]!=int(role,16):break
                else:raise AssertionError(('support flow did not finish',role,pose,pool))
                streams.append({'name':f'{role}-{pose}-{pool}-{stream}','initial':initial,'calls':calls})

    collections=[(args.prefix+'-oracle',natural,'Actual source6 support/18682 callbacks under declared real-menu input')]
    if not args.natural_only:collections += [(args.prefix+'-boundaries',isolated[:boundary_count],'Source6 complete private support phases and standalone18682 returns'),(args.prefix+'-sequences',streams,'One source6 initial state; original clock, support phase and readonly timing refresh')]
    for name,rows,scope in collections:
        raw=(json.dumps({'library':list(library.values()),'calls':rows},separators=(',',':'))+'\n').encode();z=gzip.compress(raw,mtime=0);(out/(name+'.json.gz')).write_bytes(z);calls=[r for s in rows for r in s['calls']]if name.endswith('sequences')else rows
        report={'pass':bool(rows)and not errors,'baseline_verified':frames==expected,'scope':scope,'identity':identity,'capture_sha256':sha(Path(__file__).read_bytes()),'filter_sha256':sha(bridge_path.read_bytes()),'records':len(rows),'calls':len(calls),'kind_counts':dict(Counter(r['kind']for r in calls)),
                'library_frames':len(library),'fixture_sha256':sha(raw),'gzip_sha256':sha(z),'baselines':baselines,'binding':binding,'frames_match_original_core':frames==expected,'rng_calls':sum(r['rng_calls']for r in calls),'external_queue_calls':sum(bool(r['external_queue_clears'])for r in calls),
                'external_change_calls':sum(bool(r['external_changes'])for r in calls),'unexplained_change_calls':sum(bool(r['unexplained_changes'])for r in calls),'interrupt_calls':sum(len(r['interrupts'])for r in calls),'errors':errors,'incomplete_calls':0,
                'limits':['Shared entrypoint reuse is verified in real source6 initialization contexts; no retagged other-source actor','Stop at15afc, original clock return, or complete18682 RTS; no full actor/tail/world/activeFX simulation','Timing refreshes modify only Duration/Flags from own current immutable keys; no expected-state or RNG reinjection']}
        (out/(name+'-manifest.json')).write_text(json.dumps(report,indent=2)+'\n');print(json.dumps({'name':name,'records':len(rows),'calls':len(calls),'kinds':report['kind_counts'],'pass':report['pass']}))


if __name__=='__main__':main()
