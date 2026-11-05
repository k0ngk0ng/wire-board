"""Read only scalar DAC/DSP runtime state from the pinned reference process."""
from pathlib import Path
import argparse,ctypes as C,json,subprocess
from extract_audio_assets import AudioSession
from research_audio_loops import quiet_checkpoint
from reference import ROOT,sha
class Config(C.Structure):
 _fields_=[('version',C.c_char*16)]+[(name,C.c_uint8)for name in ['hq_fm','filter','hq_psg','ym2612','ym2413','ym3438','opll','mono']]+[(name,C.c_int16)for name in ['psg_preamp','fm_preamp','cdda_volume','pcm_volume']]+[('lp_range',C.c_uint16)]+[(name,C.c_int16)for name in ['low_freq','high_freq','lg','mg','hg']]
class Snd(C.Structure):
 _fields_=[('sample_rate',C.c_int),('frame_rate',C.c_double),('enabled',C.c_int),('blips',C.c_void_p*4),('states',C.c_void_p*4),('fm_last_save',C.c_int*2),('cd_last_save',C.c_int16*2)]
class Blip(C.Structure):
 _fields_=[('factor',C.c_uint64),('offset',C.c_uint64),('size',C.c_int),('integrator',C.c_int32*2),('buffer',C.c_void_p*2)]
class Slot(C.Structure):
 _fields_=[('DT',C.c_void_p),('KSR',C.c_uint8)]+[(k,C.c_uint32)for k in ['ar','d1r','d2r','rr']]+[('ksr',C.c_uint8)]+[(k,C.c_uint32)for k in ['mul','phase']]+[('Incr',C.c_int32),('state',C.c_uint8),('tl',C.c_uint32),('volume',C.c_int32),('sl',C.c_uint32),('vol_out',C.c_uint32)]+[(k,C.c_uint8)for k in ['eg_sh_ar','eg_sel_ar','eg_sh_d1r','eg_sel_d1r','eg_sh_d2r','eg_sel_d2r','eg_sh_rr','eg_sel_rr','ssg','ssgn','key']]+[('AMmask',C.c_uint32)]
class Channel(C.Structure):
 _fields_=[('SLOT',Slot*4),('ALGO',C.c_uint8),('FB',C.c_uint8),('op1_out',C.c_int32*2)]+[(k,C.c_void_p)for k in ['connect1','connect3','connect2','connect4','mem_connect']]+[('mem_value',C.c_int32),('pms',C.c_int32),('ams',C.c_uint8),('fc',C.c_uint32),('kcode',C.c_uint8),('block_fnum',C.c_uint32)]
class ST(C.Structure):
 _fields_=[('address',C.c_uint16),('status',C.c_uint8),('mode',C.c_uint32),('fn_h',C.c_uint8)]+[(k,C.c_int32)for k in ['TA','TAL','TAC','TB','TBL','TBC']]+[('dt_tab',(C.c_int32*32)*8)]
class SL3(C.Structure):
 _fields_=[('fc',C.c_uint32*3),('fn_h',C.c_uint8),('kcode',C.c_uint8*3),('block_fnum',C.c_uint32*3),('key_csm',C.c_uint8)]
class OPN(C.Structure):
 _fields_=[('ST',ST),('SL3',SL3),('pan',C.c_uint32*12),('eg_cnt',C.c_uint32),('eg_timer',C.c_uint32),('lfo_cnt',C.c_uint8)]+[(k,C.c_uint32)for k in ['lfo_timer','lfo_timer_overflow','LFO_AM','LFO_PM']]
class YM(C.Structure):
 _fields_=[('CH',Channel*6),('dacen',C.c_uint8),('dacout',C.c_int32),('OPN',OPN)]

def symbols(path):
 output=subprocess.check_output(['nm',str(path)],text=True);result={}
 for row in output.splitlines():
  fields=row.split()
  if len(fields)==3:
   try:address=int(fields[0],16)
   except ValueError:continue
   result.setdefault(fields[2],[]).append(address)
 return result

def snapshot(ref,lookup):
 image=C.cast(ref.lib.audio_update,C.c_void_p).value-lookup['_audio_update'][0]
 cfg=Config.in_dll(ref.lib,'config');snd=Snd.in_dll(ref.lib,'snd');blip=Blip.from_address(snd.blips[0]);ym=YM.from_address(image+lookup['_ym2612'][0])
 context=C.create_string_buffer(65536);size=ref.api('YM2612SaveContext',C.c_int,C.c_void_p)(context);assert size==C.sizeof(YM)+48,(size,C.sizeof(YM))
 raw=context.raw[:size];assert raw[:C.sizeof(YM)]==C.string_at(C.addressof(ym),C.sizeof(YM))
 scalar={name:getattr(cfg,name)for name,_ in Config._fields_ if name!='version'}
 scalar.update(sample_rate=snd.sample_rate,frame_rate=snd.frame_rate,sound_enabled=snd.enabled)
 fm={name:C.c_int.from_address(image+lookup['_'+name][0]).value for name in ['fm_cycles_ratio','fm_cycles_start','fm_cycles_count']}
 previous=[C.c_int.from_address(image+lookup['_fm_last.'+str(i)][0]).value for i in range(2)]
 lowpass=[C.c_int16.from_address(image+lookup[k][0]).value for k in ['_llp','_rrp']]
 tail=[list((C.c_int32*18).from_address(blip.buffer[i]))for i in range(2)]
 state={'master_origin':ref.frame*896040,'video_frame':ref.frame,'dac_value':(ym.dacout>>6)+128,'dac_enabled':bool(ym.dacen),'dac_raw14':ym.dacout,'dac_pan_masks':list(ym.OPN.pan[10:12]),'previous_chip_output':previous,'fm_timing':fm,'resampler':{'factor':blip.factor,'offset':blip.offset,'available_samples':blip.offset>>52,'integrator':list(blip.integrator),'tail':tail,'capacity':blip.size},'previous_lowpass_output':lowpass,'fm_operator_states':[[slot.state for slot in ch.SLOT]for ch in ym.CH],'fm_operator_key':[[slot.key for slot in ch.SLOT]for ch in ym.CH]}
 return {'configuration':scalar,'initial_state':state,'abi':{'config_prefix_bytes':C.sizeof(Config),'snd_bytes':C.sizeof(Snd),'blip_prefix_bytes':C.sizeof(Blip),'ym_context_bytes':size,'ym_struct_bytes':C.sizeof(YM),'slot_bytes':C.sizeof(Slot),'channel_bytes':C.sizeof(Channel),'dac_enable_offset':YM.dacen.offset,'dac_output_offset':YM.dacout.offset,'pan_offset':YM.OPN.offset+OPN.pan.offset,'host_pointers_discarded':True}}

def main():
 p=argparse.ArgumentParser(description=__doc__);p.add_argument('--out',type=Path,required=True);a=p.parse_args();out=a.out.resolve();assert out.is_relative_to(ROOT);out.mkdir(parents=True,exist_ok=True)
 core=ROOT/'.local/md-yuyuhakusho/tools/Genesis-Plus-GX-60bee4c724859710ce05672239e5b601ad01382e/genesis_plus_gx_libretro.dylib';lookup=symbols(core)
 with AudioSession(ROOT,out/'reference',core=core)as ref:
  checkpoint=quiet_checkpoint(ref);ref.restore(checkpoint,restore_refresh_cycles=True);record=snapshot(ref,lookup);record.update(identity=ref.identity,pass_=True,scope='Read-only runtime configuration and explicit normalized quiescentDAC DSP initial state, no production host pointers');record['pass']=record.pop('pass_')
 record['inspector_sha256']=sha(Path(__file__).read_bytes());(out/'runtime-config.json').write_text(json.dumps(record,indent=2)+'\n');print(json.dumps(record))
if __name__=='__main__':main()
