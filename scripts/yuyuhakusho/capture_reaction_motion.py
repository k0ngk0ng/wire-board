#!/usr/bin/env python3
"""Original shared reaction phases, explicit CPU boundaries and trajectories."""
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

ENTRIES = {0x19f90:3, 0x19fb0:4, 0x1a02c:5, 0x1a13c:6, 0x1a1ae:7,
           0x1a016:8, 0x1a22c:9, 0x1a282:12, 0x1a2c2:13}
ACTOR_WRITES = {0x59,0x5a,0x5b,0x5d,0x5f,0x60,0x61,0x64,0x6b,0x6c,0x6d,0x6e,0x77,0x78,0x79,0x7e,0x7f,0x84,0x85,0x87,0x26,0x3e,0x3f} | set(range(0x10,0x18)) | set(range(0x40,0x48)) | set(range(0x4c,0x50)) | set(range(0x9c,0xa0)) | set(range(0xa8,0xbc))

def num(raw, off, size, signed=False):
    return int.from_bytes(raw[off:off+size], 'big', signed=signed)


def main():
    parser=argparse.ArgumentParser();parser.add_argument('--out',required=True,type=Path);args=parser.parse_args()
    out=args.out.resolve();assert out.is_relative_to(ROOT);out.mkdir(parents=True,exist_ok=True)
    core=ROOT/'.local/md-yuyuhakusho/tools/Genesis-Plus-GX-60bee4c724859710ce05672239e5b601ad01382e/genesis_plus_gx_libretro.dylib'
    bridge_path=ROOT/'.local/md-yuyuhakusho/tools/hook_filter.dylib';bridge=C.CDLL(str(bridge_path))
    cbtype=C.CFUNCTYPE(None,C.c_int,C.c_int,C.c_uint,C.c_uint)
    bridge.yyh_set_observer.argtypes=[cbtype,C.POINTER(C.c_uint),C.c_size_t];bridge.yyh_set_observer.restype=C.c_int
    points=list(ENTRIES)+[0x15afc,0x1fae6,0x1faee];pcs=(C.c_uint*len(points))(*points)
    events=json.loads((ROOT/'docs/research/md-yuyuhakusho/inputs.json').read_text())
    expected=json.loads((ROOT/'docs/research/md-yuyuhakusho/frame-hashes.json').read_text())
    rom=(ROOT/'.local/md-yuyuhakusho/downloads/yuyuhakusho-jp.bin').read_bytes()
    natural=[];isolated=[];sequences=[];pending={};errors=[];private=False;case_name=''

    def snapshot(ram,address):
        off=address&0xffff;obj=ram[off:off+256];p=num(obj,0x90,4)&0xffffff
        source,index=(rom,p) if p<len(rom) else (ram,p&0xffff)
        return {'actor_id':off,'body':body_state(obj),'actor_bytes':obj.hex(),
                'controls':{'held':obj[0x71]|obj[0xc0]<<8,'pressed':obj[0x72]|obj[0xc1]<<8,'changed':obj[0x73]|obj[0xc2]<<8,'raw_held':obj[0x74],'retain_ticks':obj[0x75]},
                'guard_mask':obj[0x63],'stun_ticks':num(obj,0x84,2),'charge':num(obj,0xb8,4,True),
                'pool':{'cursor':num(ram,0xa036,2),'bytes':ram[0x9400:0x9c00].hex()},'occupied_lanes':ram[0xa042],
                'pending_sound':ram[0xa030],'global_f568':num(ram,0xf568,2),'global_f56a':num(ram,0xf56a,2),
                'left':num(ram,0xa010,2,True),'right':num(ram,0xa012,2,True),
                'parameters':{name:num(source,index+offset,4,True) for name,offset in [('vertical_velocity',4),('gravity',8),('horizontal_velocity',12)]}}

    def finish(row,ram,address):
        row['after']=snapshot(ram,address);before=bytes.fromhex(row['before']['actor_bytes']);after=bytes.fromhex(row['after']['actor_bytes'])
        unknown=[i for i,(a,b) in enumerate(zip(before,after)) if a!=b and i not in ACTOR_WRITES]
        assert not unknown,unknown
        return row

    with ReferenceSession(ROOT,out,core=core) as ref:
        @cbtype
        def observe(kind,width,pc,value):
            try:
                address=ref.get_register(13)&0xffffff;ram=ref.ram_cpu()
                if pc in ENTRIES:
                    if address in pending:
                        assert pc==0x1a02c and pending[address]['kind']==8,'Unexpected nested reaction phase'
                        return
                    pending[address]={'kind':ENTRIES[pc],'frame':ref.frame,'before':snapshot(ram,address),'allocations':[]}
                elif address in pending:
                    if pc in (0x1fae6,0x1faee):pending[address]['allocations'].append(slot_index(ref.get_register(8)))
                    elif pc==0x15afc:
                        row=pending.pop(address);row.update(exit_frame=ref.frame,exit=hex(pc))
                        finish(row,ram,address)
                        if private:
                            row['name']=case_name;isolated.append(row);ref.cpu.cycles=4000000
                        else:natural.append(row)
            except Exception as exc:
                errors.append(str(exc))
                if private:ref.cpu.cycles=4000000
        assert bridge.yyh_set_observer(observe,pcs,len(points))
        hook=ref.api('set_cpu_hook',None,C.c_void_p);observer=C.cast(bridge.yyh_cpu_hook,C.c_void_p)
        hook(observer);frames=[]
        for frame in range(1,3601):
            frames.append(ref.step(frame,buttons_at(events,frame))['reference'])
            if frame==2400:checkpoint=ref.save()
        hook(None);assert frames==expected and not errors and not pending,(len(natural),errors)
        private=True;setreg=ref.api('m68k_set_reg',None,C.c_uint,C.c_uint);execute=ref.api('m68k_run',None,C.c_uint)
        def write(address,value,size=1):
            for i,byte in enumerate((value&((1<<(size*8))-1)).to_bytes(size,'big')):
                C.c_ubyte.from_address(ref.ram_pointer+(((address+i)&0xffff)^1)).value=byte
        def fill(address,size,seed):
            for i in range(size):write(address+i,(i*73+seed*29+17)&255)
        def setup(kind,pool='empty',**changes):
            ref.restore(checkpoint,restore_refresh_cycles=True)
            fill(0xff8400,256,kind);fill(0xff9400,0x800,kind+1)
            for i in range(16):write(0xff9404+i*128,0 if pool=='empty' or pool=='single' and i==1 else 0x80,2)
            defaults={0x5d:(kind,1),0x5f:(0x23,1),0x59:(1,1),0x5a:(8,1),0x5b:(1,1),0x5c:(0,1),0x60:(0,1),0x61:(0,1),0x63:(0,1),
                      0x6e:(2,1),0x6f:(0,1),0x71:(0,1),0x72:(0,1),0x73:(0,1),0xc0:(0,1),0xc1:(0,1),0xc2:(0,1),
                      0x78:(100,2),0x84:(0,2),0x90:(0xff7000,4),0x40:(100<<16,4),0x44:(310<<16,4),0x9c:(320<<16,4),
                      0x10:(0x18000,4),0x14:(-0x20000,4),0x4c:(0x3800,4),0xb8:(0,4)}
            aliases={'pose':0x5f,'index':0x59,'ticks':0x5b,'flags':0x5c,'completed':0x60,'facing':0x61,'guard':0x63,'held':0x71,'pressed':0x72,'changed':0x73,'ext_pressed':0xc1,'health':0x78,'stun':0x84,'x':0x40,'y':0x44,'ground':0x9c,'vx':0x10,'vy':0x14,'gravity':0x4c,'charge':0xb8}
            for key,value in changes.items():
                off=aliases[key];defaults[off]=(value,defaults[off][1])
            for off,(value,size) in defaults.items():write(0xff8400+off,value,size)
            for off,value in [(4,-0x58001),(8,0x3600),(12,0x1c000)]:write(0xff7000+off,value,4)
            write(0xffa036,0,2);write(0xffa042,3);write(0xffa030,0x91);write(0xfff568,0x1234,2);write(0xfff56a,0xabcd,2)
            write(0xffa010,32,2);write(0xffa012,600,2)
        def call(kind,name):
            nonlocal case_name
            case_name=name;setreg(17,0x2700);setreg(15,0xffffe0);write(0xffffe0,0x3d8,4);setreg(13,0xff8400)
            for reg in range(8):setreg(reg,0)
            before=ref.ram_cpu();n=len(isolated);setreg(16,next(pc for pc,k in ENTRIES.items() if k==kind));ref.cpu.cycles=0;ref.cpu.refresh_cycles=896
            hook(observer);execute(2000000);hook(None)
            assert len(isolated)==n+1 and not errors and not pending,(name,errors,pending)
            after=ref.ram_cpu();allowed=set(range(0x8400,0x8500))|set(range(0x9400,0x9c00))|{0xa030,0xa036,0xa037,0xa042,0xf568,0xf569,0xf56a,0xf56b}|set(range(0xff80,0xffe4))
            unknown=[i for i,(a,b)in enumerate(zip(before,after))if a!=b and i not in allowed];assert not unknown,(name,unknown)
            return isolated[-1]
        def case(kind,name,pool='empty',**changes):setup(kind,pool,**changes);return call(kind,name)
        for completed,flags,facing in itertools.product([0,1,2,255],[0,4],[0,1,255]):
            case(3,f'complete-{completed}-flags-{flags}-facing-{facing}',completed=completed,flags=flags,facing=facing)
        for vx,facing,guard,held in itertools.product([-0x80000000,-1,0,1,0x4800,0x4801,0x9000,0x7fffffff],[0,1,2],[0,4,8],[0,2]):
            case(4,f'ground-slide-{vx}-{facing}-{guard}-{held}',vx=vx,facing=facing,guard=guard,held=held,x=0x7fffffff)
        for kind,stun,health,pool in itertools.product([5,8],[-32768,-1,0,1,128,129,320,321,32767],[-1,0,1],['empty','full','single']):
            case(kind,f'air-effect-{kind}-{stun}-{health}-{pool}',pool,index=0,ticks=1,stun=stun,health=health,facing=(stun%3),y=300<<16,vy=-0x10000)
        for kind,pose,vy,y,pool in itertools.product([5,6],[0x23,0x24,0x25,0x27,0x28,0x2f,0x30,0],[0,-1,1,0x10000],[(320<<16)-1,320<<16,(320<<16)+1],['empty','full','single']):
            case(kind,f'landing-{kind}-{pose}-{vy}-{y}-{pool}',pool,pose=pose,index=1,vy=vy,y=y,health=-1 if pose==0x28 else 0)
        for held,pressed,ext in itertools.product([0,1,2,4,8,0x20],[0,1],[0,2]):
            case(5,f'landing-control-{held}-{pressed}-{ext}',pose=0x24,index=1,vy=1,y=320<<16,held=held,pressed=pressed,ext_pressed=ext)
        for health,vy in itertools.product([-32768,-1,0,1,32767],[-1,0,1]):
            case(6,f'index-zero-{health}-{vy}',index=0,health=health,vy=vy,y=0x12345678)
        for x,vx,facing in itertools.product([31,32,33,599,600,601,32767,-32768],[-0x18000,0,0x18000],[0,1,255]):
            case(7,f'wall-{x}-{vx}-{facing}',x=(x<<16)|0xabcd,vx=vx,facing=facing)
        for completed,facing,extra in itertools.product([0,1,2,255],[0,1,255],[0,255]):
            setup(9,completed=completed,facing=facing);write(0xff8487,extra);call(9,f'recover-jump-{completed}-{facing}-{extra}')
        for extra in [0,255]:
            setup(9,completed=1);write(0xff8487,extra);call(9,f'extra-jump-{extra}')
        for stun,pressed,ticks in itertools.product([-32768,-1,0,1,2,3,128,200,32767],[0,1],[0,199,200,201,255]):
            case(12,f'stun-{stun}-{pressed}-{ticks}',stun=stun,pressed=pressed,ticks=ticks)
        for charge,vx,held in itertools.product([-0x80000000,-1,0,(223<<16)-1,223<<16,0x7fffffff],[-1,0,1,0x7fffffff],[0,0x10,0x40,0x50]):
            case(13,f'charge-{charge}-{vx}-{held}',charge=charge,vx=vx,held=held)
        boundary_count=len(isolated)
        configs=[(4,{'vx':0x90000}),(5,{'pose':0x23}),(5,{'pose':0x25}),(5,{'pose':0x28}),(6,{'pose':0x28}),(7,{'x':599<<16,'vx':0x18000}),(12,{'stun':37}),(13,{'charge':200<<16,'vx':0x10000,'held':0x10}),(9,{'completed':1}),(3,{'completed':1,'flags':4})]
        for kind,options in configs:
            for pool in ['empty','full']:
                setup(kind,pool,**options);steps=[]
                for index in range(512):
                    row=call(kind,f'sequence-{kind}-{pool}-{index}');steps.append(row)
                    if row['after']['body']['action_kind']!=kind:break
                assert steps[-1]['after']['body']['action_kind']!=kind,('Sequence did not transition',kind)
                sequences.append({'kind':kind,'pool':pool,'initial':steps[0]['before'],'steps':steps})
        identity=ref.identity
    for name,records,scope in [('reaction-motion-oracle',natural,'Natural full shared-reaction phases before shared tail'),('reaction-motion-boundaries',isolated[:boundary_count],'Explicit original CPU boundary inputs'),('reaction-motion-sequences',sequences,'Single-initial-state original phase trajectories')]:
        payload=(json.dumps(records,separators=(',',':'))+'\n').encode();compressed=gzip.compress(payload,mtime=0)
        (out/(name+'.json.gz')).write_bytes(compressed)
        calls=[step for seq in records for step in seq['steps']] if name.endswith('sequences') else records
        report={'scope':scope,'identity':identity,'capture_sha256':sha(Path(__file__).read_bytes()),'filter_sha256':sha(bridge_path.read_bytes()),'records':len(records),'calls':len(calls),'kind_counts':dict(Counter(str(r['kind'])for r in calls)),'allocation_attempts':sum(len(r['allocations'])for r in calls),'fixture_sha256':sha(payload),'gzip_sha256':sha(compressed),'frames_match_original_core':frames==expected,'incomplete_calls':0,'errors':errors,'pass':bool(records)and not errors}
        (out/(name+'-manifest.json')).write_text(json.dumps(report,indent=2)+'\n');print(json.dumps(report,indent=2))

if __name__=='__main__':main()
