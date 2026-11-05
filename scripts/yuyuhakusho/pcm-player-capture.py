"""Declared-private original PCM player inputs, no world or 68k execution.

Only the pinned original firmware executes in this research process. Production
will use typed semantic states and independently derived block costs instead.
"""
from pathlib import Path
import argparse,ctypes as C,gzip,json
from reference import ROOT,sha
from extract_audio_assets import AudioSession,sample_descriptor
from research_audio_loops import quiet_checkpoint
from capture_pcm_model import Event
F=896040
ORIGINAL=ROOT/'.local/md-yuyuhakusho/tools/genesis_plus_gx_libretro.dylib'
OBSERVED=ROOT/'.local/md-yuyuhakusho/tools/Genesis-Plus-GX-60bee4c724859710ce05672239e5b601ad01382e/genesis_plus_gx_libretro.dylib'
BRIDGE=ROOT/'.local/md-yuyuhakusho/audio-pcm-model-tools/pcm_model_observer.dylib'
class Z80(C.Structure):
 _fields_=[(k,C.c_uint32)for k in ['pc','sp','af','bc','de','hl','ix','iy','wz','af2','bc2','de2','hl2']]+[(k,C.c_uint8)for k in ['r','r2','iff1','iff2','halt','im','i','nmi_state','nmi_pending','irq_state','after_ei']]+[('cycles',C.c_uint32),('daisy',C.c_void_p),('irq_callback',C.c_void_p)]
assert C.sizeof(Z80)==88 and Z80.cycles.offset==64
CASES=[{'name':'dual-a','mode':'dual','codes':[6,None],'frames':42},
 {'name':'dual-b','mode':'dual','codes':[None,122],'frames':20},
 {'name':'dual-pair','mode':'dual','codes':[31,63],'frames':115},
 {'name':'dual-odd-tail','mode':'dual','codes':[6,122],'lengths':[259,133],'frames':6},
 {'name':'single-native','mode':'single','codes':[115],'frames':60},
 {'name':'single-tail129','mode':'single','codes':[115],'lengths':[129],'frames':3},
 {'name':'single-tail257','mode':'single','codes':[115],'lengths':[257],'frames':4},
 {'name':'dual-min69','mode':'dual','codes':[6,None],'lengths':[69,0],'frames':4},
 *[{'name':'single-rate%03d'%rate,'mode':'single','codes':[115],'lengths':[129],'rate':rate,'frames':20 if rate in [0,255] else 4}for rate in [0,1,20,255]]]
def dump(path,value):
 raw=(json.dumps(value,separators=(',',':'))+'\n').encode();path.write_bytes(gzip.compress(raw,mtime=0));return sha(raw)
def initial(ref,rom,case):
 z=Z80.in_dll(ref.lib,'Z80');zr=(C.c_ubyte*8192).in_dll(ref.lib,'zram');bank=C.c_uint.in_dll(ref.lib,'zbank')
 for key,_ in Z80._fields_:
  if key not in ['daisy','irq_callback']:setattr(z,key,0)
 for i in range(0xc00,0xe00):zr[i]=0
 for i in range(0x1f80,0x2000):zr[i]=0
 z.bc2=0x4001;z.de2=0x4000;z.hl2=0x1f2a
 def word(address,value):zr[address]=value&255;zr[address+1]=value>>8
 samples=[]
 for i,code in enumerate(case['codes']):
  if code is None:samples.append(None);continue
  d=sample_descriptor(rom,code);length=case.get('lengths',[d['encoded_bytes']]*len(case['codes']))[i];assert 0<length<=d['encoded_bytes'];payload=rom[d['sample_address']:d['sample_address']+length]
  samples.append({'id':sha(payload),'encoded_hex':payload.hex(),'length':length,'rate':case.get('rate',d['rate_byte']),'flags':d['flags'],'pan':d['routing_byte'],'source':d})
 if case['mode']=='dual':
  z.pc=0x04c9;z.sp=0x1fc0
  for i,base in enumerate([0x1f90,0x1fb0]):
   buf=0xd00 if i==0 else 0xc00;word(base+2,buf);word(0x1fc2+i*4,buf);zr[0x1fc1+i*4]=128
   if(samples[i]):
    sample=samples[i];d=sample['source'];zr[0x1f86+i*32]=0x40;zr[base]=1;zr[0x1ffc]=0xc1
    descriptor=bytes.fromhex(d['descriptor_bytes']);word(base+4,int.from_bytes(descriptor[:2],'little'));word(base+6,0x8000|(d['sample_address']&0x7fff));word(base+8,sample['length'])
 else:
  s=samples[0];d=s['source'];assert s['length']>=129
  payload=bytes.fromhex(s['encoded_hex']);zr[0xc00:0xc80]=payload[:128];word(0x1fe2,0xc00);word(0x1fe4,0x8000|((d['sample_address']+128)&0x7fff));word(0x1fe6,d['z80_header'])
  zr[0x1ffc]=128;z.pc=0x0120;z.sp=0x1fe4;z.bc=s['length']-128;z.de=0xc80;z.af2=0x8000;z.bc2=0x4000|s['rate'];z.iy=d['z80_header'];bank.value=d['sample_address']&~0x7fff
 return z,zr,samples

def main():
 p=argparse.ArgumentParser(description=__doc__);p.add_argument('--out',required=True,type=Path);p.add_argument('--smoke',action='store_true');p.add_argument('--audible',action='store_true');args=p.parse_args();out=args.out.resolve();assert out.is_relative_to(ROOT);out.mkdir(parents=True,exist_ok=True)
 rom=(ROOT/'.local/md-yuyuhakusho/downloads/yuyuhakusho-jp.bin').read_bytes();assert sha(rom)=='c39d52b2e2298b3784ada3de70d64b26b67e172114c7d18494faad70fc4d7c06'
 bridge=C.CDLL(str(BRIDGE));bridge.pcm_model_attach.argtypes=[C.c_void_p]*7+[C.c_size_t];bridge.pcm_model_attach.restype=C.c_int;bridge.pcm_model_events.restype=C.c_void_p;bridge.pcm_model_count.restype=C.c_size_t
 cases=CASES[:1]if args.smoke else CASES;controls={};reports=[];identities={}
 for mode,core in [('original',ORIGINAL),('observed',OBSERVED)]:
  with AudioSession(ROOT,out/mode,core=core)as ref:
   if args.audible:
    from inspect_dac_dsp_state import symbols,snapshot
    lookup=symbols(core)
   checkpoint=quiet_checkpoint(ref);identities[mode]=ref.identity
   for case in cases:
    ref.restore(checkpoint,restore_refresh_cycles=True);z,zr,samples=initial(ref,rom,case);frames=[];pcms=[];start=bytes(zr).hex();video=sha(ref.image);ram=sha(ref.ram_cpu())
    initial_dsp=None
    if args.audible:
     writer=C.CFUNCTYPE(None,C.c_uint,C.c_uint,C.c_uint)(C.c_void_p.in_dll(ref.lib,'fm_write').value)
     for port,reg,value in [(2,0xb6,0xc0),(0,0x2a,128),(0,0x2b,128)]:writer(0,port,reg);writer(0,port+1,value)
     initial_dsp=snapshot(ref,lookup)
    if mode=='observed':
     slots=[C.c_void_p.in_dll(ref.lib,k)for k in ['fm_write','z80_readmem','z80_writemem']];zb=C.c_uint.in_dll(ref.lib,'zbank')
     assert bridge.pcm_model_attach(*[C.addressof(s)for s in slots],C.addressof(z),C.addressof(ref.cpu)+type(ref.cpu).cycles.offset,C.addressof(zb),C.addressof(zr),400000)
    run=ref.api('z80_run',None,C.c_uint);update=ref.api('audio_update',C.c_int,C.c_void_p);end=C.c_uint.in_dll(ref.lib,'mcycles_vdp')
    for frame in range(case['frames']):
     if mode=='observed':bridge.pcm_model_frame(frame)
     run(F);end.value=F;buffer=(C.c_int16*8820)();n=update(buffer);pcm=C.string_at(buffer,n*4);pcms.append(pcm)
     z.cycles-=F
     # Only native player/Z80 and chip state advance; no 68k/world/video run.
     row={'frame':frame,'pc':z.pc,'cycles_ahead':z.cycles,'z80_ram_sha256':sha(bytes(zr)),'registers':{k:int(getattr(z,k))for k,_ in Z80._fields_ if k not in ['daisy','irq_callback']},'pcm_sha256':sha(pcm),'stereo_frames':n,'player_state_hex':bytes(zr[0x1f80:]).hex()}
     if args.audible:
      ref.frame=checkpoint['frame']+frame+1;row['pcm_hex']=pcm.hex();row['native_dsp']=snapshot(ref,lookup)['initial_state']
     assert sha(ref.image)==video and sha(ref.ram_cpu())==ram
     if mode=='observed':assert row==controls[case['name']]['frames'][frame],('observer changed private native state',case['name'],frame)
     frames.append(row)
    body={'case':case,'samples':samples,'initial_z80_ram_hex':start,'frames':frames,'pcm_sha256':sha(b''.join(pcms))}
    if args.audible:body['initial_dsp']=initial_dsp
    if mode=='original':controls[case['name']]=body
    else:
     assert bridge.pcm_model_detach()==1 and not bridge.pcm_model_overflow();count=bridge.pcm_model_count();raw=C.string_at(bridge.pcm_model_events(),count*C.sizeof(Event));events=(Event*count).from_buffer_copy(raw);bridge.pcm_model_free()
     body['events']=[{k:int(getattr(e,k))for k,_ in Event._fields_[:12]}|{'registers':list(e.registers),'state':bytes(e.state).hex()}for e in events]
     path=out/(case['name']+'.json.gz');dump(path,body)
     reports.append({'name':case['name'],'frames':case['frames'],'events':count,'dac_events':sum(e.kind==1 for e in events),'bank_reads':sum(e.kind==3 for e in events),'final_busy':bool(zr[0x1ffc]),'pass':True,'path':str(path.relative_to(ROOT)),'sha256':sha(path.read_bytes())})
    print(json.dumps({'mode':mode,'case':case['name'],'frames':case['frames'],'final_busy':bool(zr[0x1ffc])}),flush=True)
 (out/'capture.json').write_text(json.dumps({'pass':True,'scope':'Declared private initialized native player; no world/68k run and no external bus holds; not normal gameplay reachability','identities':identities,'audible_dac_initialized':args.audible,'producer_sha256':sha(Path(__file__).read_bytes()),'bridge_sha256':sha(BRIDGE.read_bytes()),'rom_sha256':sha(rom),'cases':reports},indent=2)+'\n')
if __name__=='__main__':main()
