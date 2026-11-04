#!/usr/bin/env python3
"""Complete active Chu rule callback12682 up to126ba, before sprite emission."""
import argparse
from collections import Counter
import ctypes as C
import gzip
import itertools
import json
from pathlib import Path
from reference import ROOT, ReferenceSession, buttons_at, sha
from capture_chu_action import snapshot as action_snapshot, ACTION_POSES
from capture_animation_records import selected_data

FIGHTERS = [0x8400, 0x8800, 0x8c00, 0x9000]


def n(raw, off, size, signed=False): return int.from_bytes(raw[off:off+size], 'big', signed=signed)


def snapshot(ram, address, rom):
    return {'action': action_snapshot(ram, address, rom), 'family_world': ram[0x8400:0x9400].hex(),
            'stream': ram[0x6000:0x6400].hex(), 'stopped': ram[0xa09a], 'markers': list(ram[0xfffc:]),
            'registration': ram[0xa880:0xa900].hex(), 'registration_used': n(ram, 0xa034, 2),
            'samples': ram[0xa800:0xa880].hex(), 'samples_used': n(ram, 0xa00e, 2),
            'hud_cells': ram[0xe602:0xea02].hex(), 'hud_queue': ram[0xd400:0xe000].hex(), 'hud_cursor': n(ram, 0xf518, 2)}


def view(ram):
    return {'pause': ram[0xf550], 'slow': ram[0xf55a], 'disabled': ram[0xa032], 'difficulty': n(ram, 0xff12, 2),
            'camera_x': n(ram, 0xa014, 2, True), 'camera_y': n(ram, 0xa016, 2, True), 'camera_shake': n(ram, 0xf566, 2, True),
            'blink': ram[0xf585], 'stream_base': 0xff6000,
            'controllers': [{key: ram[0xf780+channel+i*8] | ram[0xf7a8+channel+i*8]<<8
                             for i, key in enumerate(['current', 'previous', 'changed', 'pressed', 'released'])} for channel in range(8)]}


def main():
    p=argparse.ArgumentParser();p.add_argument('--out',required=True,type=Path);args=p.parse_args()
    out=args.out.resolve();assert out.is_relative_to(ROOT);out.mkdir(parents=True,exist_ok=True)
    core=ROOT/'.local/md-yuyuhakusho/tools/Genesis-Plus-GX-60bee4c724859710ce05672239e5b601ad01382e/genesis_plus_gx_libretro.dylib'
    bridge=C.CDLL(str(ROOT/'.local/md-yuyuhakusho/tools/hook_filter.dylib'));cbtype=C.CFUNCTYPE(None,C.c_int,C.c_int,C.c_uint,C.c_uint)
    bridge.yyh_set_observer.argtypes=[cbtype,C.POINTER(C.c_uint),C.c_size_t];bridge.yyh_set_observer.restype=C.c_int
    points=[0x12682,0x126ba,0x14922,0x15540,0x15a18,0x156f0,0x208d8,0x15772,0x15b78,0x3982,0x37c56,0x37cac]
    pcs=(C.c_uint*len(points))(*points);rom=(ROOT/'.local/md-yuyuhakusho/downloads/yuyuhakusho-jp.bin').read_bytes()
    paths=[ROOT/'docs/research/md-yuyuhakusho/inputs.json',ROOT/'games/yuyuhakusho/engine/testdata/chu-special-motion-inputs.json']
    natural=[];isolated=[];streams=[];errors=[];pending={};library={};baselines=[];private=False;label='';scenario=''
    for input_index,input_path in enumerate(paths):
        scenario=input_path.name;events=json.loads(input_path.read_text())
        if input_index==0:expected=json.loads((ROOT/'docs/research/md-yuyuhakusho/frame-hashes.json').read_text())
        else:
            with ReferenceSession(ROOT,out/'unobserved-special')as original:expected=[original.step(f,buttons_at(events,f))['reference']for f in range(1,3601)]
        with ReferenceSession(ROOT,out/('observed-'+str(input_index)),core=core)as ref:
            @cbtype
            def observe(kind,width,pc,value):
                try:
                    address,sp=ref.get_register(13)&0xffffff,ref.get_register(15);key=(address,sp)
                    if pc==0x12682:
                        assert not pending,'Nested12682 callback'
                        ram=ref.ram_cpu();assert ram[(address&0xffff)+0x58]==9
                        pending[key]={'object_id':address&0xffff,'frame':ref.frame,'scenario':scenario,'view':view(ram),
                                      'before':snapshot(ram,address,rom),'stages':{},'rng_calls':0,'external_queue_clears':[]}
                    elif pc in(0x37c56,0x37cac):
                        for row in pending.values():
                            a=ref.get_register(9)&0xffff;row['external_queue_clears'].append({'pc':hex(pc),'address':hex(a),'before':ref.ram_cpu()[a],'sr':ref.get_register(17),'sp':sp})
                    elif pc==0x3982:
                        for row in pending.values():row['rng_calls']+=1
                    elif pc==0x15b78:
                        for row in pending.values():
                            ram=ref.ram_cpu();frame,data,source=selected_data(rom,ram,ram[address&0xffff:(address&0xffff)+256]);identity=json.dumps(frame,sort_keys=True)
                            assert identity not in library or library[identity]['data']==data
                            library[identity]={'key':frame,'data':data,'source':source};row['animation_key']=frame
                    elif pc==0x126ba and key in pending:
                        row=pending.pop(key);ram=ref.ram_cpu();row.update(after=snapshot(ram,address,rom),exit_frame=ref.frame)
                        active=row['view']['pause']&128==0 and row['view']['slow']==0
                        for stage in['0x15540','0x15a18','0x156f0']:assert row['stages'].get(stage,0)==int(active),(stage,row['stages'])
                        assert row['stages'].get('0x14922')==row['stages'].get('0x15772')==1
                        health=n(ram,(address&0xffff)+0x78,2,True);assert row['stages'].get('0x208d8',0)==int(health!=-1024)
                        assert (row['rng_calls']>=1)==active
                        if private:row['name']=label;isolated.append(row);ref.cpu.cycles=4000000
                        else:natural.append(row)
                    elif pending:
                        for row in pending.values():
                            row['stages'][hex(pc)]=row['stages'].get(hex(pc),0)+1
                except Exception as exc:
                    errors.append(str(exc))
                    if private:ref.cpu.cycles=4000000
            assert bridge.yyh_set_observer(observe,pcs,len(points));hook=ref.api('set_cpu_hook',None,C.c_void_p);observer=C.cast(bridge.yyh_cpu_hook,C.c_void_p)
            hook(observer);frames=[]
            for f in range(1,3601):
                frames.append(ref.step(f,buttons_at(events,f))['reference'])
                if f==2400:checkpoint,base_ram=ref.save(),ref.ram_cpu()
            hook(None);assert frames==expected and not pending and not errors,(scenario,errors[:4],list(pending))
            baselines.append({'inputs':str(input_path.relative_to(ROOT)),'sha256':sha(input_path.read_bytes()),'frames':3600,'match':True,'reference_sha256':sha(json.dumps(expected,separators=(',',':')).encode())})
            identity=ref.identity
            if input_index==0:continue
            private=True;setreg=ref.api('m68k_set_reg',None,C.c_uint,C.c_uint);execute=ref.api('m68k_run',None,C.c_uint)
            def write(address,value,size=1):
                for i,b in enumerate((value&((1<<(size*8))-1)).to_bytes(size,'big')):C.c_ubyte.from_address(ref.ram_pointer+(((address+i)&0xffff)^1)).value=b
            def write_bytes(address,data):
                for i,b in enumerate(data):C.c_ubyte.from_address(ref.ram_pointer+(((address+i)&0xffff)^1)).value=b
            input_routes=[(0,0,0),(0,255,0),(1,0,0),(-1,0,0),(-1,255,0),(0,255,1)]
            def setup(seed,source,action=0,route=0,edge=False,pause=0,slow=0,health=100):
                ref.restore(checkpoint,restore_refresh_cycles=True)
                for reg in range(15):setreg(reg,0)
                for j,owner in enumerate(FIGHTERS):
                    write_bytes(0xff0000+owner,bytes((i*73+seed*11+j*29+17)&255 for i in range(1024)))
                    for off,val,size in[(4,0x400,2),(0x58,9,1),(0x6e,0 if owner==source else 1,1),(0x78,100,2),(0x40,(100+j*45)<<16|0x1234,4),(0x44,0x1405678,4),(0x9c,0x1405678,4),(0x5c,0,1),(0x64,0,1),(0xb8,100<<16,4)]:write(0xff0000+owner+off,val,size)
                    for child in range(3):write(0xff0000+owner+0x104+child*256,0,2);write(0xff0000+owner+0x144+child*256,0,4);write(0xff0000+owner+0x164+child*256,0)
                base=0xff0000+source
                write_bytes(base+0x50,base_ram[0x8450:0x8454]);write_bytes(base+0x8c,base_ram[0x848c:0x8490])
                pose=ACTION_POSES[action];obj=bytearray(ref.ram_cpu()[source:source+256]);obj[0x5f],obj[0x59]=pose,0
                _,frame,_=selected_data(rom,ref.ram_cpu(),obj)
                mode,channel,disabled=input_routes[route]
                for off,val,size in[(0x58,9,1),(0x59,0,1),(0x5a,frame['duration'],1),(0x5b,frame['duration']-1 if edge else 0,1),(0x5c,frame['flags'],1),(0x5d,action,1),(0x5f,pose,1),(0x60,0,1),(0x61,seed%3,1),
                                    (0x69,seed%3,1),(0x6a,seed%4,1),(0x6b,seed%9,1),(0x6c,0x1f if seed%2 else 0,1),(0x6d,0,1),(0x6e,0,1),(0x6f,0,1),(0x70,(source-0x8400)//256,1),
                                    (0x71,0,1),(0x72,0,1),(0x73,0,1),(0x74,0,1),(0x75,seed%3,1),(0x76,channel,1),(0xc0,0,1),(0xc1,0,1),(0xc2,0,1),(0xc7,seed%66,1),(0xc9,mode,1),(0xca,0xff6000,4),
                                    (0x78,health,2),(0x7a,[0,100,223][seed%3],2),(0x7c,seed%256,2),(0x7e,-1,2),(0x80,100,2),(0x84,129,2),(0x86,seed%3,1),(0x87,seed%2,1),(0x88,0,1),(0x89,seed%17,1),(0xc8,seed%3,1),
                                    (0x40,([31,100,601][seed%3]<<16)|0x1234,4),(0x44,0x13fffff if seed%2 else 0x1405678,4),(0x9c,0x1405678,4),(0x10,0x18000,4),(0x14,0x18000,4),(0x4c,(seed%3)<<24 if action==66 else 0x3600,4),(0x48,0x14000,4),
                                    (8,0x111,2),(12,0x222,2),(0x54,-3,2),(0x56,5,2),(0x8a,[0,0x40,0x100,0x200][(source-0x8400)//1024],2),(0x90,0xff7000,4),(0xb8,(223<<16)|0x5678,4),(0xbc,[0,100,223][seed%3],2),(0x98,seed%3*0x4800,4),(0xc4,seed%65536,2),
                                    (0xce,2+seed%15,1),(0xd0,(seed%4)*2,2),(0xd2,seed%3,2),(0xd4,seed%9,2),(0xd6,FIGHTERS[(FIGHTERS.index(source)+1)%4],2)]:write(base+off,val,size)
                for i in range(16):write(base+0xa8+i,seed%8 if i<8 else 0)
                for off,val in[(0,0x18000),(4,-0x58003),(8,0x3601),(12,0x1c000),(24,0xc000)]:write(0xff7000+off,val,4)
                write(0xff701c,10,2)
                write_bytes(0xff6000,bytes([8,16,16,8,0,64,64,0])*128)
                write_bytes(0xffa200,bytes((i*19+seed*31+91)&255 for i in range(0x700)))
                for i in range(4):write(0xffa004+i*2,[0,240,256][(seed+i)%3],2)
                write(0xffa00c,0,2);write(0xffa00e,[0,60,64,65535][seed%4],2);write(0xffa034,0,2)
                pool=bytearray((i*37+seed*23+13)&255 for i in range(0x800))
                for i in range(16):pool[i*128+4:i*128+6]=(0x80 if seed%3==2 else 0).to_bytes(2,'big')
                write_bytes(0xff9400,pool);write(0xffa036,0xff80 if seed%2 else 0,2);write(0xffa03a,0,2);write(0xffa042,0xa0);write(0xffa030,0x97 if seed%2 else 0)
                write(0xffa010,32,2);write(0xffa012,600,2);write(0xffa014,[0,500,-32768,32767][seed%4],2);write(0xffa016,320,2)
                write(0xffa032,disabled);write(0xffa09a,0);write_bytes(0xfffffc,bytes([1,0x22,0x44,0x7f]))
                write(0xfff550,pause);write(0xfff55a,slow);write(0xfff566,seed%4-2,2);write(0xfff568,0,2);write(0xfff56a,0,2);write(0xfff585,seed%2)
                write(0xffff12,(seed%4)*2,2);write(0xffff18,[0,1,0x12345678,0xffffffff][seed%4],4)
                write_bytes(0xfff80a,bytes([0x80,0x81,0x82,0x83]if seed%2 else[0,0x80,0,0x81]))
                write_bytes(0xffd400,bytes((i*13+seed*3+7)&255 for i in range(3072)));write(0xfff518,0xd400,2)
                write_bytes(0xffe602,bytes((i*17+seed*5+9)&255 for i in range(1024)))
                for c in range(8):
                    current=[0,8,16,64,80,1][seed%6]
                    for i,value in enumerate([current,0,current,current,0]):write(0xfff780+c+i*8,value);write(0xfff7a8+c+i*8,0)
            def invoke(source,name):
                nonlocal label
                label=name;setreg(17,0x2700);setreg(15,0xffffe0);write(0xffffe0,0x3d8,4);setreg(13,0xffff0000+source);setreg(16,0x12682);ref.cpu.cycles=0;ref.cpu.refresh_cycles=896
                before=ref.ram_cpu();count=len(isolated);hook(observer);execute(3000000);hook(None)
                assert len(isolated)==count+1 and not pending and not errors,(name,errors[:4],list(pending))
                after=ref.ram_cpu();allowed=set(range(source,source+1024))|set(range(0x6000,0x6400))|set(range(0x9400,0x9c00))|set(range(0xa200,0xa900))|set(range(0xa004,0xa010))|{0xa030,0xa034,0xa035,0xa036,0xa037,0xa042,0xa09a}|set(range(0xd400,0xe000))|set(range(0xe602,0xea02))|{0xf518,0xf519}|set(range(0xf568,0xf56c))|set(range(0xf80a,0xf80e))|set(range(0xff18,0xff1c))|set(range(0xff00,0xffe4))|set(range(0xfffc,0x10000))
                for owner in FIGHTERS:
                    if owner!=source:allowed|={owner+0xb8,owner+0xb9}
                unexpected=[i for i,(a,b)in enumerate(zip(before,after))if a!=b and i not in allowed];assert not unexpected,(name,unexpected)
                return isolated[-1]
            for seed,(source,action,route,edge)in enumerate(itertools.product(FIGHTERS,ACTION_POSES,range(6),[False,True])):
                setup(seed,source,action,route,edge);invoke(source,f'active-{source:x}-{action:x}-{route}-{edge}')
            for seed,(pause,slow,health,route,source)in enumerate(itertools.product([0,1,0x7f,0x80,255],[0,1,255],[-1024,-1,0,223],[0,1,2,4],[0x8400,0x9000])):
                setup(seed,source,0,route,False,pause,slow,health);invoke(source,f'gate-{pause}-{slow}-{health}-{route}-{source:x}')
            for seed,(source,channel,stopped)in enumerate(itertools.product(FIGHTERS,range(8),[0,1])):
                setup(seed,source,0,3);write(0xff0000+source+0x76,channel);write(0xffa09a,stopped)
                for c in range(8):
                    for i,value in enumerate([128,0,128,128,0]):write(0xfff780+c+i*8,value);write(0xfff7a8+c+i*8,0)
                invoke(source,f'record-terminal-{source:x}-{channel}-{stopped}')
            for seed,source in enumerate(FIGHTERS):
                setup(seed,source,0,2);write_bytes(0xff6000,bytes([128,16,64,8]));invoke(source,f'playback-terminal-{source:x}')
            for seed,source in enumerate([0x8400,0x9000]):
                setup(seed,source,0,4)
                for c in range(8):write(0xfff780+c,128)
                invoke(source,f'ai-record-start-is-not-human-{source:x}')
            boundary_count=len(isolated)
            for stream_index,(source,action,route)in enumerate([(0x8400,0,3),(0x9000,0,1),(0x8800,0,4),(0x8c00,0,2),(0x8400,66,0),(0x9000,0,0)]):
                setup(stream_index,source,action,route);write(0xffa00e,0,2);write(0xffa034,0,2);calls=[];previous=0
                for step in range(24):
                    held=128 if stream_index==0 and step==15 else 8 if step<6 else 16 if 8<=step<12 else 0;changed=held^previous
                    for c in range(8):
                        for i,value in enumerate([held,previous,changed,held&changed,previous&changed]):write(0xfff780+c+i*8,value);write(0xfff7a8+c+i*8,0)
                    pause=128 if stream_index==5 and 6<=step<10 else 0;slow=1 if stream_index==5 and 14<=step<18 else 0
                    write(0xfff550,pause);write(0xfff55a,slow)
                    row=invoke(source,f'stream-{stream_index}/{step}');row['external_view']={'controllers':row['view']['controllers'],'pause':pause,'slow':slow};calls.append(row);previous=held
                streams.append({'initial':calls[0]['before'],'view':calls[0]['view'],'calls':calls})
    for name,calls,scope in[('chu-actor-oracle',natural,'Natural complete12682 rules to126ba across two original baselines'),('chu-actor-boundaries',isolated[:boundary_count],'Declared isolated complete active rules'),('chu-actor-sequences',streams,'Singleinitial actor/world subset; no otherobject callbacks')]:
        raw=(json.dumps({'library':list(library.values()),'calls':calls},separators=(',',':'))+'\n').encode();z=gzip.compress(raw,mtime=0);(out/(name+'.json.gz')).write_bytes(z)
        rows=[r for s in calls for r in s['calls']]if name.endswith('sequences')else calls
        report={'scope':scope,'identity':identity,'capture_sha256':sha(Path(__file__).read_bytes()),'records':len(calls),'calls':len(rows),'baselines':baselines,'frames_match_original_core':all(b['match']for b in baselines),'external_queue_calls':sum(bool(r['external_queue_clears'])for r in rows),'rng_counts':dict(Counter(r['rng_calls']for r in rows)),'fixture_sha256':sha(raw),'gzip_sha256':sha(z),'incomplete_calls':0,'errors':errors,'pass':bool(calls)and not errors,'limits':['Stops before actualsprite1580; initialization126c8 and child/world callbacks excluded','Natural externalaudio events preserved; expectedqueue never reinjected','Singleinitial streams omit collision manager and otheractors/children']}
        (out/(name+'-manifest.json')).write_text(json.dumps(report,indent=2)+'\n');print(json.dumps(report,indent=2))


if __name__=='__main__':main()
