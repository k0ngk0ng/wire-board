#!/usr/bin/env python3
"""Chu special/dash continuations, full parent/child/pool CPU comparisons."""
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

ENTRIES={0x1ca9c:0x0f,0x1cc28:0x14,0x1a2f2:0x15,0x1c33a:0x41,0x1c3ba:0x42,0x1c50a:0x43,0x1c6a0:0x44,0x1c6e2:0x45,0x1c7bc:0x46}

def num(raw,off,size,signed=False):return int.from_bytes(raw[off:off+size],'big',signed=signed)

def main():
 p=argparse.ArgumentParser();p.add_argument('--out',required=True,type=Path);args=p.parse_args();out=args.out.resolve();assert out.is_relative_to(ROOT);out.mkdir(parents=True,exist_ok=True)
 core=ROOT/'.local/md-yuyuhakusho/tools/Genesis-Plus-GX-60bee4c724859710ce05672239e5b601ad01382e/genesis_plus_gx_libretro.dylib'
 bridge_path=ROOT/'.local/md-yuyuhakusho/tools/hook_filter.dylib';bridge=C.CDLL(str(bridge_path));cbtype=C.CFUNCTYPE(None,C.c_int,C.c_int,C.c_uint,C.c_uint)
 bridge.yyh_set_observer.argtypes=[cbtype,C.POINTER(C.c_uint),C.c_size_t];bridge.yyh_set_observer.restype=C.c_int
 points=list(ENTRIES)+[0x15afc,0x1fae6,0x1faee,0x19cc2];pcs=(C.c_uint*len(points))(*points)
 original_events=json.loads((ROOT/'docs/research/md-yuyuhakusho/inputs.json').read_text())
 events=[event for event in original_events if event['end']<=1800]
 for start in [1850,2150,2450,2750,3050,3350]:
  events += [{'start':start,'end':start+2,'buttons':[7]},{'start':start+6,'end':start+35,'buttons':[7]},
             {'start':start+48,'end':start+50,'buttons':[6]},{'start':start+54,'end':start+83,'buttons':[6]}]
  for delta in [100,104,108]:events.append({'start':start+delta,'end':start+delta+1,'buttons':[0]})
 events.sort(key=lambda e:e['start'])
 input_raw=(json.dumps(events,indent=2)+'\n').encode();(out/'chu-special-motion-inputs.json').write_bytes(input_raw)
 with ReferenceSession(ROOT,out/'unobserved') as ref:
  expected=[ref.step(frame,buttons_at(events,frame))['reference']for frame in range(1,3601)];unobserved_identity=ref.identity
 rom=(ROOT/'.local/md-yuyuhakusho/downloads/yuyuhakusho-jp.bin').read_bytes()
 natural=[];isolated=[];sequences=[];pending={};errors=[];private=False;clock_only=False;clock_done=False;case_name=''
 def snapshot(ram,address):
  off=address&0xffff;obj=ram[off:off+256];pointer=num(obj,0x90,4)&0xffffff
  memory,index=(rom,pointer)if pointer<len(rom)else(ram,pointer&0xffff)
  return {'object_id':off,'character_id':obj[0x58],'body':body_state(obj),'actor_bytes':obj.hex(),
          'controls':{'held':obj[0x71]|obj[0xc0]<<8,'pressed':obj[0x72]|obj[0xc1]<<8,'changed':obj[0x73]|obj[0xc2]<<8,'raw_held':obj[0x74],'retain_ticks':obj[0x75]},
          'energy':num(obj,0xb8,4,True),'power':num(obj,0x7c,2,True),'alternate_velocity':num(obj,0x48,4,True),'contact_flags':obj[0x89],
          'children':ram[off+256:off+1024].hex(),'pool':{'cursor':num(ram,0xa036,2),'bytes':ram[0x9400:0x9c00].hex()},
          'parameters':{name:num(memory,index+delta,4,True)for name,delta in [('vertical_velocity',4),('gravity',8),('horizontal_velocity',12)]}}
 with ReferenceSession(ROOT,out,core=core) as ref:
  @cbtype
  def observe(kind,width,pc,value):
   nonlocal clock_done
   try:
    if clock_only:
     if pc==0x19cc2:clock_done=True;ref.cpu.cycles=4000000
     return
    address=ref.get_register(13)&0xffffff;ram=ref.ram_cpu()
    if pc in ENTRIES:
     assert address not in pending,'Unexpected nested special phase'
     pending[address]={'kind':ENTRIES[pc],'frame':ref.frame,'before':snapshot(ram,address),'allocations':[]}
    elif address in pending:
     if pc in(0x1fae6,0x1faee):pending[address]['allocations'].append(slot_index(ref.get_register(8)))
     elif pc==0x15afc:
      row=pending.pop(address);row.update(after=snapshot(ram,address),exit=hex(pc),exit_frame=ref.frame)
      if private:row['name']=case_name;isolated.append(row);ref.cpu.cycles=4000000
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
   for i,byte in enumerate((value&((1<<(size*8))-1)).to_bytes(size,'big')):C.c_ubyte.from_address(ref.ram_pointer+(((address+i)&0xffff)^1)).value=byte
  def fill(address,size,seed):
   for i in range(size):write(address+i,(i*73+seed*29+17)&255)
  def setup(kind,pool='empty',children=0,**changes):
   ref.restore(checkpoint,restore_refresh_cycles=True);fill(0xff8400,1024,kind);fill(0xff9400,0x800,kind+1)
   for i in range(16):write(0xff9404+i*128,0 if pool=='empty' or pool=='single'and i==1 else 0x80,2)
   for i in range(3):write(0xff8504+i*256,0x100 if children&(1<<i)else 0,2)
   fields={0x58:(9,1),0x5d:(kind,1),0x5f:(0xa0,1),0x59:(1,1),0x5a:(8,1),0x5b:(0,1),0x5c:(0,1),0x60:(0,1),0x61:(0,1),0x6b:(0,1),
           0x6e:(2,1),0x6f:(0,1),0x70:(31,1),0x71:(0,1),0x72:(0,1),0x73:(0,1),0xc0:(0,1),0xc1:(0,1),0xc2:(0,1),0x78:(100,2),
           0x90:(0xff7000,4),0x40:((100<<16)|0xabcd,4),0x44:((300<<16)|0x5678,4),0x9c:((320<<16)|0x1234,4),
           0x10:(0x40000,4),0x14:(0x3c000,4),0x48:(0x60000,4),0x4c:(0x3800,4),0xb8:((220<<16)|0xabcd,4),0x7c:(0,2),0x89:(0,1)}
   aliases={'pose':0x5f,'index':0x59,'ticks':0x5b,'duration':0x5a,'flags':0x5c,'completed':0x60,'facing':0x61,'request':0x6b,'held':0x71,'pressed':0x72,'changed':0x73,'x':0x40,'y':0x44,'ground':0x9c,'vx':0x10,'vy':0x14,'gravity':0x4c,'energy':0xb8,'power':0x7c,'contact':0x89,'alt':0x48}
   for key,value in changes.items():
    off=aliases[key];fields[off]=(value,fields[off][1])
   for off,(value,size)in fields.items():write(0xff8400+off,value,size)
   for off,value in[(4,-0x58001),(8,0x3600),(12,0x1c000)]:write(0xff7000+off,value,4)
   write(0xffa036,0,2)
  def prepare_cpu(entry):
   setreg(17,0x2700);setreg(15,0xffffe0);write(0xffffe0,0x3d8,4);setreg(13,0xff8400)
   for reg in range(8):setreg(reg,0)
   setreg(16,entry);ref.cpu.cycles=0;ref.cpu.refresh_cycles=896
  def call(kind,name):
   nonlocal case_name
   case_name=name;prepare_cpu(next(pc for pc,k in ENTRIES.items()if k==kind));before=ref.ram_cpu();n=len(isolated)
   hook(observer);execute(2000000);hook(None)
   assert len(isolated)==n+1 and not pending and not errors,(name,errors)
   after=ref.ram_cpu();allowed=set(range(0x8400,0x8800))|set(range(0x9400,0x9c00))|{0xa036,0xa037}|set(range(0xff80,0xffe4))
   unknown=[i for i,(a,b)in enumerate(zip(before,after))if a!=b and i not in allowed];assert not unknown,(name,unknown)
   return isolated[-1]
  def tick_clock():
   nonlocal clock_only,clock_done
   clock_only=True;clock_done=False;prepare_cpu(0x19c96);hook(observer);execute(100000);hook(None);clock_only=False
   assert clock_done and not errors,errors
  def case(kind,name,pool='empty',children=0,**changes):setup(kind,pool,children,**changes);return call(kind,name)
  for kind,completed,index,ticks,facing,pool in itertools.product([0xf,0x14,0x41,0x44],[0,1,2],[0,1,3,4,255],[0,1,4,255],[0,1,2,255],['empty','full']):
   case(kind,f'dash-{kind:x}-{completed}-{index}-{ticks}-{facing}-{pool}',pool,completed=completed,index=index,ticks=ticks,facing=facing,flags=4 if index&1 else 0,contact=12 if ticks==255 else 0,vx=-0x40001 if facing==2 else 0x40001, y=(320<<16)|0x5678 if index&1 else 300<<16)
  for request,energy,completed in itertools.product([0,1,2,3,255],[-32768,-1,0,7,8,11,12,32767],[0,1,255]):
   case(0x15,f'repeat-{request}-{energy}-{completed}',request=request,energy=(energy<<16)|0xabcd,completed=completed,vx=-1)
  for variant,held,pressed,facing,energy,power in itertools.product([0,1,2,255],[0,0x10,0x40],[0,0x10,0x20,0x40,0x50,4,8],[0,1],[0,1,-1],[0,254,255]):
   case(0x42,f'charge-{variant}-{held}-{pressed}-{facing}-{energy}-{power}',children=(2 if held else 0),gravity=(variant<<24)|0x123456,held=held,pressed=pressed,facing=facing,energy=(energy<<16)|0xabcd,power=power)
  for power in[-32768,-1,0,1,127,128,254,255,256,32767]:
   case(0x42,f'charge-word-{power}',gravity=0x03000000,power=power,energy=0x80000001)
  for index,power,facing,children in itertools.product([0,1,3,4],range(256),[0,1],[0,1,4,7]):
   case(0x43,f'projectile-{index}-{power}-{facing}-{children}',index=index,power=power,facing=facing,children=children,vx=0x0040abcd,vy=-0x0037edcc)
  for kind,index,ticks,facing,contact,y in itertools.product([0x45,0x46],[0,1,18,19,255],[0,1,4],[0,1,2],[0,4,8],[255,256,319,320,321]):
   case(kind,f'flight-{kind:x}-{index}-{ticks}-{facing}-{contact}-{y}',index=index,ticks=ticks,facing=facing,contact=contact,y=(y<<16)|0xabcd)
  boundary_count=len(isolated)
  configs=[(0xf,{'flags':0x80,'duration':4,'index':1,'ticks':0,'y':320<<16}),
           (0xf,{'flags':0x80,'duration':4,'index':1,'ticks':0,'y':300<<16}),
           (0x14,{'flags':0x84,'duration':4,'index':1,'ticks':0}),
           (0x15,{'flags':0x80,'duration':3,'index':0,'ticks':0,'request':1}),
           (0x41,{'flags':0x80,'duration':5,'index':1,'ticks':0}),
           (0x44,{'flags':0x80,'duration':5,'index':1,'ticks':0}),
           (0x42,{'gravity':0x01000000,'held':0x10,'energy':100<<16,'power':0}),
           (0x43,{'duration':1,'index':0,'ticks':255,'power':255,'vx':0x0040abcd,'vy':-0x0037edcc}),
           (0x45,{'duration':2,'index':1,'ticks':0,'y':300<<16,'contact':0}),
           (0x45,{'duration':2,'index':1,'ticks':0,'y':300<<16,'contact':4}),
           (0x46,{'duration':2,'index':17,'ticks':0,'y':300<<16})]
  for initial_kind,options in configs:
   for pool in ['empty','full']:
    setup(initial_kind,pool,**options);initial=snapshot(ref.ram_cpu(),0xff8400);steps=[]
    for index in range(512):
     external={}
     if initial_kind==0x42 and index==6:write(0xff8471,0);external={'held':0}
     tick_clock();kind=ref.ram_cpu()[0x845d]
     assert kind in ENTRIES.values()
     row=call(kind,f'sequence-{initial_kind:x}-{pool}-{index}');row['advance_clock']=True;row['external_input']=external;steps.append(row)
     if row['after']['body']['action_kind']!=initial_kind:break
     if initial_kind==0x43 and num(bytes.fromhex(row['after']['children']),4,2)!=0:break
    assert index<511,('No transition/milestone',initial_kind)
    sequences.append({'kind':initial_kind,'pool':pool,'initial':initial,'steps':steps,'milestone':'projectile-created'if initial_kind==0x43 else'action-transition'})
  identity=ref.identity
 for name,records,scope in [('chu-special-motion-oracle',natural,'Natural original special/dash phases with declared input pulses'),('chu-special-motion-boundaries',isolated[:boundary_count],'Explicit original CPU phase and child-initializer boundaries'),('chu-special-motion-sequences',sequences,'Single-initial-state isolated phase plus original-clock trajectories')]:
  raw=(json.dumps(records,separators=(',',':'))+'\n').encode();z=gzip.compress(raw,mtime=0);(out/(name+'.json.gz')).write_bytes(z)
  calls=[step for seq in records for step in seq['steps']]if name.endswith('sequences')else records
  report={'scope':scope,'identity':identity,'unobserved_identity':unobserved_identity,'capture_sha256':sha(Path(__file__).read_bytes()),'filter_sha256':sha(bridge_path.read_bytes()),'inputs_sha256':sha(input_raw),'records':len(records),'calls':len(calls),'kind_counts':dict(Counter(str(r['kind'])for r in calls)),'character_counts':dict(Counter(str(r['before']['character_id'])for r in calls)),'allocation_attempts':sum(len(r['allocations'])for r in calls),'fixture_sha256':sha(raw),'gzip_sha256':sha(z),'frames_match_original_core':frames==expected,'incomplete_calls':0,'errors':errors,'pose_tables':[{'address':hex(addr),'bytes':rom[addr:addr+8].hex(),'sha256':sha(rom[addr:addr+8])}for addr in[0x1c5f4,0x1c68c]],'pass':bool(records)and not errors,'limits':['Child initialization only, not 1313e/132de/1361e subsequent object update','Trajectory clocks use declared private record timing; no animation record refresh or sharedtail','Projectile power lookup is validated only for original charge-derived values0..255']}
  (out/(name+'-manifest.json')).write_text(json.dumps(report,indent=2)+'\n');print(json.dumps(report,indent=2))

if __name__=='__main__':main()
