#!/usr/bin/env python3
"""Source8 core motion before shared tail, including exact partial child/FX writes."""
import argparse
from collections import Counter
import ctypes as C
import gzip
import itertools
import json
from pathlib import Path
from reference import ROOT, ReferenceSession, buttons_at, sha
from capture_ground_idle import actor as body_state
from capture_effect_pool import slot_index
from capture_animation_records import selected_data

ENTRIES = {0x1ca9c:0x0f,0x1cb08:0x11,0x1c09a:0x3e,0x1c196:0x3f,0x1c27c:0x40}
POSES = {0x0f:11,0x11:12,0x3e:0xa4,0x3f:0xa7,0x40:0xac}


def n(raw, off, size=2, signed=False):
    return int.from_bytes(raw[off:off+size], 'big', signed=signed)


def main():
    p=argparse.ArgumentParser();p.add_argument('--out',required=True,type=Path);args=p.parse_args()
    out=args.out.resolve();assert out.is_relative_to(ROOT);out.mkdir(parents=True,exist_ok=True)
    core=ROOT/'.local/md-yuyuhakusho/tools/Genesis-Plus-GX-60bee4c724859710ce05672239e5b601ad01382e/genesis_plus_gx_libretro.dylib'
    bridge_path=ROOT/'.local/md-yuyuhakusho/tools/hook_filter.dylib';bridge=C.CDLL(str(bridge_path))
    callback_type=C.CFUNCTYPE(None,C.c_int,C.c_int,C.c_uint,C.c_uint)
    bridge.yyh_set_observer.argtypes=[callback_type,C.POINTER(C.c_uint),C.c_size_t];bridge.yyh_set_observer.restype=C.c_int
    points=list(ENTRIES)+[0x15afc,0x1fae6,0x1faee,0x19cc2,0x3982,0x37c56,0x37cac]
    pcs=(C.c_uint*len(points))(*points)
    rom=(ROOT/'.local/md-yuyuhakusho/downloads/yuyuhakusho-jp.bin').read_bytes()
    sources=[('docs/research/md-yuyuhakusho/source8-next-inputs.json',
              'games/yuyuhakusho/engine/testdata/fighter8-attacks-reference-frames.json',
              'games/yuyuhakusho/engine/testdata/fighter8-attacks-reference-manifest.json')]
    special=('games/yuyuhakusho/engine/testdata/fighter8-special-inputs.json',
             'games/yuyuhakusho/engine/testdata/fighter8-attacks-special-reference-frames.json',
             'games/yuyuhakusho/engine/testdata/fighter8-attacks-special-reference-manifest.json')
    if all((ROOT/name).is_file()for name in special):sources.append(special)
    natural,isolated,sequences,errors,baselines=[],[],[],[],[]
    pending=None;private=False;clock_only=False;clock_done=False;label='';scenario='';library={}

    def snapshot(ram,address):
        off=address&65535;obj=ram[off:off+256];pointer=n(obj,0x90,4)&0xffffff
        memory,index=(rom,pointer)if pointer<len(rom)else(ram,pointer&65535)
        return {'object_id':off,'character_id':obj[0x58],'body':body_state(obj),'actor_bytes':obj.hex(),
                'controls':{'held':obj[0x71]|obj[0xc0]<<8,'pressed':obj[0x72]|obj[0xc1]<<8,'changed':obj[0x73]|obj[0xc2]<<8,'raw_held':obj[0x74],'retain_ticks':obj[0x75]},
                'energy':n(obj,0xb8,4,True),'power':n(obj,0x7c,2,True),'alternate_velocity':n(obj,0x48,4,True),'contact_flags':obj[0x89],
                'children':ram[off+256:off+1024].hex(),'pool':{'cursor':n(ram,0xa036),'bytes':ram[0x9400:0x9c00].hex()},
                'pending_sound':ram[0xa030],'sound_queue':list(ram[0xf80a:0xf80e]),'random_seed':n(ram,0xff18,4),
                'animation_set':n(obj,0x50,4),'shape_set':n(obj,0x8c,4),
                'parameters':{key:n(memory,index+delta,4,True)for key,delta in[('vertical_velocity',4),('gravity',8),('horizontal_velocity',12)]}}

    for input_index,(input_name,reference_name,reference_manifest_name)in enumerate(sources):
        scenario=Path(input_name).name;input_path=ROOT/input_name;reference_path=ROOT/reference_name
        raw_reference=reference_path.read_bytes()
        if reference_path.suffix=='.gz':raw_reference=gzip.decompress(raw_reference)
        reference_manifest=json.loads((ROOT/reference_manifest_name).read_text())
        assert reference_manifest['observer_attached']is False
        assert reference_manifest['identity']['core_sha256']=='1d08b724cf975de17af146f9ba4292946a609b26d03da3d0d9414c90983095da'
        assert sha(input_path.read_bytes())==reference_manifest['input']['sha256']
        reference_sha=reference_manifest['output']['sha256']
        assert sha(raw_reference)==reference_sha
        expected=json.loads(raw_reference);events=json.loads(input_path.read_text())
        with ReferenceSession(ROOT,out/('observed-'+str(input_index)),core=core)as ref:
            @callback_type
            def observe(kind,width,pc,value):
                nonlocal pending,clock_done
                try:
                    if clock_only:
                        if pc==0x19cc2:clock_done=True;ref.cpu.cycles=8000000
                        return
                    if pc==0x3982:
                        if pending is not None:pending['rng_calls']+=1
                        return
                    if pc in[0x37c56,0x37cac]:
                        if pending is not None:
                            address=ref.get_register(9)&65535
                            pending['external_queue_clears'].append({'pc':hex(pc),'address':hex(address),'before':ref.ram_cpu()[address]})
                        return
                    address=ref.get_register(13)&0xffffff;ram=ref.ram_cpu()
                    if pc in ENTRIES and ram[(address&65535)+0x58]==8:
                        assert pending is None,'overlapping source8 motion'
                        pending={'kind':ENTRIES[pc],'frame':ref.frame,'scenario':scenario,'source_register':hex(ref.get_register(13)),
                                 'entry_sp':ref.get_register(15),'before':snapshot(ram,address),'allocations':[],'rng_calls':0,'external_queue_clears':[]}
                        assert ref.get_register(13)==0xffff0000|(address&65535)
                    elif pending is not None:
                        if pc in[0x1fae6,0x1faee]:pending['allocations'].append(slot_index(ref.get_register(8)))
                        elif pc==0x15afc:
                            assert ref.get_register(15)==pending['entry_sp']
                            row=pending;pending=None;row.update(after=snapshot(ram,address),exit='0x15afc',exit_frame=ref.frame)
                            assert row['rng_calls']==0 and row['before']['random_seed']==row['after']['random_seed']
                            if row['before']['sound_queue']!=row['after']['sound_queue']:assert row['external_queue_clears']
                            if private:row['name']=label;isolated.append(row);ref.cpu.cycles=8000000
                            else:natural.append(row)
                except Exception as error:
                    errors.append(repr(error))
                    if private:ref.cpu.cycles=8000000
            assert bridge.yyh_set_observer(observe,pcs,len(points))
            hook=ref.api('set_cpu_hook',None,C.c_void_p);observer=C.cast(bridge.yyh_cpu_hook,C.c_void_p)
            hook(observer);frames=[]
            for frame in range(1,len(expected)+1):
                frames.append(ref.step(frame,buttons_at(events,frame))['reference'])
                if frame==2400:checkpoint,base_ram=ref.save(),ref.ram_cpu()
            hook(None);assert frames==expected and pending is None and not errors,(scenario,errors,pending is not None)
            baselines.append({'input':input_name,'input_sha256':sha(input_path.read_bytes()),'reference':reference_name,'reference_raw_sha256':reference_sha,
                              'reference_file_sha256':sha(reference_path.read_bytes()),'reference_manifest':reference_manifest_name,
                              'reference_manifest_sha256':sha((ROOT/reference_manifest_name).read_bytes()),'video_steps':len(expected),'all_ram_video_match':True})
            identity=ref.identity
            if input_index<len(sources)-1:continue
            assert base_ram[0x8458]==8
            binding={'source_tag':8,'animation_set':n(base_ram,0x8450,4),'shape_set':n(base_ram,0x848c,4),'parameter_pointer':n(base_ram,0x8490,4),
                     'source_actor_sha256':sha(base_ram[0x8400:0x8500])}
            private=True;setreg=ref.api('m68k_set_reg',None,C.c_uint,C.c_uint);execute=ref.api('m68k_run',None,C.c_uint)

            def write_bytes(address,raw):
                for i,byte in enumerate(raw):C.c_ubyte.from_address(ref.ram_pointer+(((address+i)&65535)^1)).value=byte

            def write(address,value,size=1):write_bytes(address,(value&((1<<(size*8))-1)).to_bytes(size,'big'))

            def add_record():
                ram=ref.ram_cpu();key,data,source=selected_data(rom,ram,ram[0x8400:0x8500]);identity=json.dumps(key,sort_keys=True)
                assert identity not in library or library[identity]['data']==data
                library[identity]={'key':key,'data':data,'source':source}
                write(0xff845a,data['duration']);write(0xff845c,data['flags'])
                return key

            def setup(kind,pool='empty',children=0,cursor=0,**changes):
                ref.restore(checkpoint,restore_refresh_cycles=True)
                write_bytes(0xff8400,bytes((i*73+kind*29+17)&255 for i in range(1024)))
                write_bytes(0xff9400,bytes((i*37+kind*23+13)&255 for i in range(2048)))
                free={'empty':set(range(16)),'full':set(),'free0':{0},'single':{1},'two':{1,3},'three':{0,5,14}}
                for i in range(16):write(0xff9404+i*128,0 if i in free[pool] else 128,2)
                for i in range(3):write(0xff8504+i*256,256 if children&(1<<i) else 0,2)
                write_bytes(0xff8450,base_ram[0x8450:0x8454]);write_bytes(0xff848c,base_ram[0x848c:0x8494])
                pose=changes.pop('pose',POSES[kind]);index=changes.pop('index',0)
                fields={0x24:(0x4321,2),0x6e:(0xf8,1),0x70:(255,1),0x58:(8,1),0x59:(index,1),0x5a:(8,1),0x5b:(0,1),0x5c:(0,1),0x5d:(kind,1),0x5f:(pose,1),0x60:(0,1),0x61:(0,1),
                        0x6b:(0,1),0x71:(0,1),0x72:(0,1),0x73:(0,1),0xc0:(0xa1,1),0xc1:(0xb2,1),0xc2:(0xc3,1),
                        0x40:(0x0064abcd,4),0x44:(0x01405678,4),0x9c:(0x01401234,4),0x10:(0x40001,4),0x14:(-0x40001,4),
                        0x48:(0x601234,4),0x4c:(0x00123456,4),0xb8:(223<<16|0xabcd,4),0x7c:(127,2),0x89:(0,1)}
                aliases={'attrs':0x24,'team':0x6e,'target_bit':0x70,'ticks':0x5b,'duration':0x5a,'flags':0x5c,'completed':0x60,'facing':0x61,'request':0x6b,'held':0x71,'pressed':0x72,'changed':0x73,
                         'x':0x40,'y':0x44,'ground':0x9c,'vx':0x10,'vy':0x14,'alt':0x48,'gravity':0x4c,'energy':0xb8,'power':0x7c,'contact':0x89}
                for key,value in changes.items():off=aliases[key];fields[off]=(value,fields[off][1])
                for off,(value,size)in fields.items():write(0xff8400+off,value,size)
                write(0xffa036,cursor,2);write(0xffa030,0x97);write(0xffff18,0x12345678,4)

            def prepare(entry):
                for reg in range(8):setreg(reg,0)
                setreg(17,0x2700);setreg(15,0xffffff00);write(0xffff00,0x3d8,4);setreg(13,0xffff8400);setreg(16,entry)
                ref.cpu.cycles,ref.cpu.refresh_cycles=0,896

            def call(kind,name):
                nonlocal label
                label=name;prepare(next(pc for pc,k in ENTRIES.items()if k==kind));before=ref.ram_cpu();count=len(isolated)
                hook(observer);execute(6000000);hook(None)
                assert len(isolated)==count+1 and pending is None and not errors,(name,errors,pending is not None)
                after=ref.ram_cpu();allowed=set(range(0x8400,0x8800))|set(range(0x9400,0x9c00))|{0xa030,0xa036,0xa037}|set(range(0xfec0,0xff04))
                unknown=[i for i,(a,b)in enumerate(zip(before,after))if a!=b and i not in allowed]
                assert not unknown,(name,[hex(i)for i in unknown])
                return isolated[-1]

            def case(kind,name,pool='empty',children=0,cursor=0,**changes):setup(kind,pool,children,cursor,**changes);return call(kind,name)

            def clock():
                nonlocal clock_only,clock_done
                clock_only,clock_done=True,False;prepare(0x19c96);hook(observer);execute(100000);hook(None);clock_only=False
                assert clock_done and not errors

            for held,vy,facing,y in itertools.product([0,1,2,3,8],[-0x44001,-0x40000,-0x3ffff,-1,0,1,0x3ffff,0x40000,0x40001],[0,1,2,255],[287,288,319,320,321]):
                case(0x3e,f'flight-input-{held}-{vy}-{facing}-{y}',index=7,held=held,vy=vy,facing=facing,y=y<<16|0xabcd)
            for held,facing,ticks,contact,pool in itertools.product([0,3],[0,1,2,255],[0,1,3,4],[0,4,8,12,16],['empty','full']):
                case(0x3e,f'flight-contact-{held}-{facing}-{ticks}-{contact}-{pool}',pool,index=7,held=held,facing=facing,ticks=ticks,contact=contact,y=0x013f5678,vy=0)
            for index,completed,y,facing in itertools.product([0,6,7,8,15,255],[0,1,2,255],[0x01405678,0x013fffff],[0,1,2,255]):
                case(0x3e,f'flight-phase-{index}-{completed}-{y}-{facing}',index=index,completed=completed,y=y,facing=facing)
            for facing,vx,x in itertools.product([0,1,2,255],[-0x80000000,-1,0x80000],[0x7fffffff,-0x80000000]):
                case(0x3e,f'flight-wrap-{facing}-{vx}-{x}',index=7,facing=facing,vx=vx,x=x,y=0x7fffffff,ground=-0x80000000,held=3,contact=12)
            for index,ticks,completed,facing,children in itertools.product([0,1,2,255],[0,1,255],[0,1,2],[0,1,2,255],[0,1]):
                case(0x3f,f'clone-gate-{index}-{ticks}-{completed}-{facing}-{children}',children=children,index=index,ticks=ticks,completed=completed,facing=facing,vx=0x00700000)
            for children,facing,vx,attrs in itertools.product([0,1],[0,1,2,255],[0,0x00300000,0x00700000,0x00b00000,-1,-0x80000000],[0,0x1fff,0x5fe0,0xffe0]):
                case(0x3f,f'clone-fields-{children}-{facing}-{vx}-{attrs}',children=children,index=0,ticks=0,facing=facing,vx=vx,attrs=attrs,team=0xf8,target_bit=255)
            for facing,x in itertools.product([0,1,2,255],[0x7fffffff,-0x80000000]):
                case(0x3f,f'clone-parent-wrap-{facing}-{x}',index=1,ticks=0,facing=facing,x=x,vx=0x00700001)
            for counter,low,completed,pressed,held in itertools.product([-32768,-2,-1,0,1,127,128,129,32767],[0,0xabcd],[0,1,2],[0,32],[0,4,8,12]):
                case(0x40,f'counter-{counter}-{low}-{completed}-{pressed}-{held}',pose=0xad,alt=counter<<16|low,completed=completed,pressed=pressed,held=held,vx=0x30000)
            for held,vx,facing in itertools.product([0,4,8,12],[-0x33000,-0x30000,-0x2ffff,-1,0,0x2ffff,0x30000,0x30001],[0,1,2,255]):
                case(0x40,f'acceleration-{held}-{vx}-{facing}',pose=0xad,alt=0x0001abcd,held=held,vx=vx,facing=facing)
            for completed,vx,facing,ticks in itertools.product([0,1,2,255],[-0x80000000,-0x30001,-3,-2,-1,0,1,2,3,0x7fffffff],[0,1,2,255],[0,1,255]):
                case(0x40,f'decelerate-{completed}-{vx}-{facing}-{ticks}',pose=0xae,completed=completed,vx=vx,facing=facing,ticks=ticks,alt=0x1234abcd)
            for pose,completed,counter in itertools.product([0xac,0xab,0xa7,255],[0,1,2],[-1,0,128,32767]):
                case(0x40,f'pose-{pose}-{completed}-{counter}',pose=pose,completed=completed,alt=counter<<16|0xabcd)
            for completed,index,ticks,facing,y in itertools.product([0,1,2],[0,1,2],[0,1],[0,1,2],[0x1405678,0x13fffff]):
                case(0x0f,f'shared-dash-{completed}-{index}-{ticks}-{facing}-{y}',completed=completed,index=index,ticks=ticks,facing=facing,y=y)
            for index,completed,facing,y in itertools.product(range(8),[0,1],[0,2],[0x1405678,0x13fffff]):
                case(0x11,f'shared-retreat-{index}-{completed}-{facing}-{y}',index=index,completed=completed,facing=facing,y=y)
            boundary_count=len(isolated)
            configs=[(0x0f,'dash',{},0),(0x11,'retreat',{},0),
                     (0x3e,'flight-neutral',{'vy':0,'vx':0x40000},2),
                     (0x3e,'flight-up-down',{'vy':0,'vx':0x80000,'facing':2},2),
                     (0x3e,'flight-contact',{'vy':0,'vx':0x40000,'contact':4},2),
                     (0x3f,'clone',{'vx':0x00700000},0),(0x3f,'clone-busy',{'vx':0x00700000,'facing':2},0),
                     (0x40,'move-timeout',{'vx':0,'alt':0xabcd},0),
                     (0x40,'move-accelerate',{'vx':0,'alt':0xabcd},0),
                     (0x40,'move-cancel',{'vx':0,'alt':0xabcd},0)]
            for(initial_kind,name,changes,terminal),pool in itertools.product(configs,['empty','full']):
                setup(initial_kind,pool,children=1 if name=='clone-busy'else 0,**changes);write(0xff845b,255);add_record();initial=snapshot(ref.ram_cpu(),0xff8400);steps=[]
                for step in range(512):
                    held=0;pressed=0
                    if name=='flight-up-down':held=1 if 20<=step<35 else 2 if 35<=step<50 else 0
                    elif name=='move-accelerate':held=8 if step<100 else 4
                    elif name=='move-cancel'and step==60:pressed=32
                    write(0xff8471,held);write(0xff8472,pressed)
                    clock();kind=ref.ram_cpu()[0x845d];assert kind in ENTRIES.values(),(name,kind)
                    row=call(kind,f'stream-{name}-{pool}-{step}');row['advance_clock']=True;row['external_input']={'held':held,'pressed':pressed}
                    row['record_after']=add_record();row['after_record']=snapshot(ref.ram_cpu(),0xff8400);steps.append(row)
                    if row['after_record']['body']['action_kind']not in ENTRIES.values():break
                else:raise AssertionError(('source8 motion did not reach recovery',name,pool))
                assert row['after_record']['body']['action_kind']==terminal,(name,row['after_record']['body']['action_kind'],terminal)
                sequences.append({'kind':initial_kind,'name':name,'pool':pool,'initial':initial,'steps':steps,'terminal':terminal})
    for name,rows,scope in [('fighter8-motion-oracle',natural,'Actual source8 motion entries from two real-menu input flows'),
                            ('fighter8-motion-boundaries',isolated[:boundary_count],'Explicit source8 complete pre-tail phase contexts'),
                            ('fighter8-motion-sequences',sequences,'Single initial state; original clock plus actual readonly animation timing records')]:
        raw=(json.dumps({'library':list(library.values()),'calls':rows},separators=(',',':'))+'\n').encode();z=gzip.compress(raw,mtime=0)
        (out/(name+'.json.gz')).write_bytes(z)
        calls=[r for s in rows for r in s['steps']]if name.endswith('sequences')else rows
        report={'pass':True,'scope':scope,'identity':identity,'capture_sha256':sha(Path(__file__).read_bytes()),'filter_sha256':sha(bridge_path.read_bytes()),
                'records':len(rows),'calls':len(calls),'kind_counts':dict(Counter(str(r['kind'])for r in calls)),
                'character_counts':dict(Counter(str(r['before']['character_id'])for r in calls)),
                'allocation_attempts':sum(len(r['allocations'])for r in calls),'external_queue_calls':sum(bool(r['external_queue_clears'])for r in calls),
                'fixture_sha256':sha(raw),'gzip_sha256':sha(z),'baselines':baselines,'frames_match_original_core':all(b['all_ram_video_match']for b in baselines),
                'binding':binding,'library_frames':len(library),'rng_calls':sum(r['rng_calls']for r in calls),'private_unknown_ram_writes':0,'errors':errors,'incomplete_calls':0,
                'limits':['Stops at15afc; no shared tail, actor/world/collision or child/FX lifetime.',
                          'Sequences refresh only Duration/Flags from actual immutable animation records, not other tail or shape outputs.']}
        (out/(name+'-manifest.json')).write_text(json.dumps(report,indent=2)+'\n');print(json.dumps({'name':name,'records':len(rows),'calls':len(calls),'kinds':report['kind_counts'],'pass':True}))


if __name__=='__main__':main()
