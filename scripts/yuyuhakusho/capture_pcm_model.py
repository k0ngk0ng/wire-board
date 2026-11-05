"""Original unfiltered DAC, DPCM operand and bank timing observations only."""
from pathlib import Path
import argparse,ctypes as C,gzip,json
from extract_audio_assets import AudioSession,queue_command,write_wave,sample_descriptor
from research_audio_loops import quiet_checkpoint
from reference import ROOT,sha
ORIGINAL=ROOT/'.local/md-yuyuhakusho/tools/genesis_plus_gx_libretro.dylib'
OBSERVED=ROOT/'.local/md-yuyuhakusho/tools/Genesis-Plus-GX-60bee4c724859710ce05672239e5b601ad01382e/genesis_plus_gx_libretro.dylib'
BRIDGE=ROOT/'.local/md-yuyuhakusho/audio-pcm-model-tools/pcm_model_observer.dylib'
CASES=[('single-short',[(0,6)],80),('pair-short',[(0,6),(8,0x7a)],90),('single-mode',[(0,0x73)],90),('pair-long',[(0,0x1f),(8,0x3f)],160)]
class Event(C.Structure):
 _fields_=[(k,C.c_uint32)for k in ['kind','frame','cycles_before','cycles_after','m68k_before','m68k_after','z80_before','z80_after','address','data','bank','pc']]+[('registers',C.c_uint32*13),('state',C.c_ubyte*128)]
def write_json(path,data):path.write_text(json.dumps(data,indent=2)+'\n')
def dump(path,data):
 raw=(json.dumps(data,separators=(',',':'))+'\n').encode();path.write_bytes(gzip.compress(raw,mtime=0));return sha(raw)
def pairs(events):
 encoded_pc={0x012b:'single',0x01a4:'single',0x02a2:'a',0x04dd:'a',0x02bb:'b',0x0502:'b'}
 low_pc={0x013f:'single',0x01b8:'single',0x02aa:'a',0x04e5:'a',0x02c2:'b',0x0509:'b'}
 pending={};result=[]
 for index,e in enumerate(events):
  role=encoded_pc.get(e['pc'])if e['kind']==4 else None
  if role:
   assert role not in pending,('overlapping decoder pairs',role,index,pending.get(role))
   pending[role]={'role':role,'frame':e['frame'],'old_accumulator':e['registers'][2]>>8&255,'encoded':e['data'],'encoded_pointer':e['address'],'encoded_event':index}
  role=low_pc.get(e['pc'])if e['kind']==5 and 0xa00<=e['address']<0xb00 else None
  if role:
   assert role in pending,('low without encoded',role,index)
   pending[role]['native_high']=e['registers'][2]>>8&255;pending[role]['low_table_event']=index
  role=('a'if e['address']==0x1fc1 else'b')if e['kind']==7 and e['address']in(0x1fc1,0x1fc5)and e['pc']in(0x02b2,0x02ca,0x04ed,0x0511)else None
  if e['kind']==1 and e['pc']in(0x0176,0x01d8):role='single'
  if role:
   assert role in pending and'native_high'in pending[role],('native low without complete input',role,index,e)
   row=pending.pop(role);row.update(native_low=e['data'],native_low_event=index);result.append(row)
 assert not pending,('unclosed decoder pair',pending)
 return result

def main():
 p=argparse.ArgumentParser(description=__doc__);p.add_argument('--out',type=Path,required=True);a=p.parse_args();out=a.out.resolve();assert out.is_relative_to(ROOT);out.mkdir(parents=True,exist_ok=True)
 rom=(ROOT/'.local/md-yuyuhakusho/downloads/yuyuhakusho-jp.bin').read_bytes();assert sha(rom)=='c39d52b2e2298b3784ada3de70d64b26b67e172114c7d18494faad70fc4d7c06'
 fw=rom[0x393fe:0x39ffe];d=[0,1,2,4,8,16,32,64,128,255,254,252,248,240,224,192]
 assert all(fw[0xa00+i]==d[i&15]and fw[0xb00+i]==d[i>>4]for i in range(256));assert all(fw[0x800+i]==max(0,min(255,i-128))for i in range(512))
 tables={'source_rom_sha256':sha(rom),'source_firmware_sha256':sha(fw),'source_table_sha256':sha(fw[0x800:0xc00]),'delta_low':list(fw[0xa00:0xb00]),'delta_high':list(fw[0xb00:0xc00]),'mix':list(fw[0x800:0xa00]),'all_delta_entries_equal':True,'all_mix_entries_equal':True};write_json(out/'tables.json',tables)
 bridge=C.CDLL(str(BRIDGE));bridge.pcm_model_event_size.restype=C.c_uint;assert bridge.pcm_model_event_size()==C.sizeof(Event)
 bridge.pcm_model_z80_size.restype=C.c_uint;assert bridge.pcm_model_z80_size()==88
 bridge.pcm_model_attach.argtypes=[C.c_void_p]*7+[C.c_size_t];bridge.pcm_model_attach.restype=C.c_int;bridge.pcm_model_detach.restype=C.c_int
 bridge.pcm_model_frame.argtypes=[C.c_uint];bridge.pcm_model_count.restype=C.c_size_t;bridge.pcm_model_overflow.restype=C.c_uint;bridge.pcm_model_events.restype=C.c_void_p
 expected={};identities={};summaries=[]
 for mode,core in [('original',ORIGINAL),('observed',OBSERVED)]:
  with AudioSession(ROOT,out/mode,core=core)as ref:
   checkpoint=quiet_checkpoint(ref);identities[mode]=ref.identity
   for name,commands,frames in CASES:
    ref.restore(checkpoint,restore_refresh_cycles=True);rows=[];chunks=[];schedule=dict(commands)
    if mode=='observed':
     slots=[C.c_void_p.in_dll(ref.lib,k)for k in ['fm_write','z80_readmem','z80_writemem']]
     z=C.c_byte.in_dll(ref.lib,'Z80');zb=C.c_uint.in_dll(ref.lib,'zbank');zr=(C.c_ubyte*8192).in_dll(ref.lib,'zram')
     assert bridge.pcm_model_attach(*[C.addressof(x)for x in slots],C.addressof(z),C.addressof(ref.cpu)+type(ref.cpu).cycles.offset,C.addressof(zb),C.addressof(zr),250000)
    for step in range(frames):
     if step in schedule:queue_command(ref,schedule[step])
     if mode=='observed':bridge.pcm_model_frame(step)
     r,pcm=ref.step_audio(ref.frame+1,[0]*8);row={'step':step,'reference':r['reference'],'stereo_frames':len(pcm)//4,'pcm_sha256':sha(pcm)};rows.append(row);chunks.append(pcm)
     if mode=='observed':assert row==expected[name]['rows'][step],('observer changed original RAM/video/PCM',name,step)
    pcm=b''.join(chunks);path=out/f'{name}-{mode}.wav';write_wave(path,pcm,44100)
    if mode=='original':expected[name]={'rows':rows,'pcm':pcm};dump(out/f'{name}-control.json.gz',{'name':name,'rows':rows,'commands':commands})
    else:
     assert bridge.pcm_model_detach()==1,'wrapper slot changed during observation';assert bridge.pcm_model_overflow()==0,'observer overflow'
     total=bridge.pcm_model_count();raw=C.string_at(bridge.pcm_model_events(),total*C.sizeof(Event));native=(Event*total).from_buffer_copy(raw);bridge.pcm_model_free()
     events=[{k:int(getattr(e,k))for k,_ in Event._fields_[:12]}|{'registers':list(e.registers),'state':bytes(e.state).hex()}for e in native]
     decoded=pairs(events);assert decoded and pcm==expected[name]['pcm']
     event_path=out/f'{name}-events.json.gz';dump(event_path,{'schema':1,'name':name,'commands':commands,'rows':rows,'events':events,'decoder_pairs':decoded})
     artifacts=[{'path':str(f.relative_to(ROOT)),'sha256':sha(f.read_bytes())}for f in [path,out/f'{name}-original.wav',event_path,out/f'{name}-control.json.gz']]
     counts={kind:sum(e['kind']==kind for e in events)for kind in range(1,8)}
     summary={'name':name,'frames':frames,'frames_match':True,'commands':commands,'events':len(events),'dac_events':counts[1],'bank_reads':counts[3],'decoder_pairs':len(decoded),'event_kinds':counts,'artifacts':artifacts}
     summaries.append(summary)
    print(json.dumps({'mode':mode,'case':name,'frames':frames,'observed_events':total if mode=='observed'else 0}),flush=True)
 sources=[{'code':c,'descriptor':sample_descriptor(rom,c),'encoded_hex':rom[(v:=sample_descriptor(rom,c))['sample_address']:v['sample_address']+v['encoded_bytes']].hex()}for c in sorted({c for _,schedule,_ in CASES for _,c in schedule})]
 write_json(out/'samples.json',{'schema':1,'source_rom_sha256':sha(rom),'samples':sources})
 record={'pass':True,'scope':'Original unfiltered DAC/operand/bank events; no independent cadence or YM2612/filter/resampler claim','identity':identities,'all_original_observed_frames_equal':True,'observer_overflows':0,'tables':tables,'cases':summaries,'producer_sha256':sha(Path(__file__).read_bytes()),'observer_source_sha256':sha((ROOT/'scripts/yuyuhakusho/pcm_model_observer.c').read_bytes()),'observer_binary_sha256':sha(BRIDGE.read_bytes()),'event_bytes':C.sizeof(Event),'z80_context_bytes':88,'limits':['Function pointer delegates preserve the original handlers; cycle timestamps are reference-core handler observation points','Empirical core bus delays are not physical hardware timing','DAC values precede YM2612 DAC behavior/filter/resampling; they are not speaker PCM']}
 write_json(out/'capture.json',record);print(json.dumps({'pass':True,'cases':len(summaries),'events':sum(x['events']for x in summaries),'decoder_pairs':sum(x['decoder_pairs']for x in summaries)}),flush=True)
if __name__=='__main__':main()
