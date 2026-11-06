"""Original empty-command PCM plus canonical main-CPU YM service oracle."""
import argparse,ctypes as C,gzip,json,os,importlib.util
from pathlib import Path
from extract_audio_assets import AudioSession,write_cpu
from reference import ROOT,CPUHead,sha
from research_audio_loops import ORIGINAL,OBSERVED,quiet_checkpoint
from inspect_dac_dsp_state import symbols,snapshot

def module(name,path):
 s=importlib.util.spec_from_file_location(name,path);m=importlib.util.module_from_spec(s);s.loader.exec_module(m);return m
shared=module('score_bus_shared',ROOT/'scripts/yuyuhakusho/score-clock-capture.py')
command=module('score_bus_command',ROOT/'scripts/yuyuhakusho/pcm-command-capture.py')
PINNED_COMMAND='4349a80c12019688324fbea3cb988ddcab16da8a76d170be9842b935c5d8356f'
CASES=[{'id':'ready-bank0','origin':50000,'bank':0,'register':0x28,'value':0},{'id':'ready-bank1','origin':50000,'bank':1,'register':0xb4,'value':192},{'id':'critical64-bank0','origin':70933,'bank':0,'register':0x28,'value':0},{'id':'startup-critical128-bank1','origin':63273,'bank':1,'register':0xb4,'value':192}]

def bind_pcm(b,ref,z,zr):
 slots=[C.c_void_p.in_dll(ref.lib,k)for k in ['fm_write','fm_reset','z80_readmem','z80_writemem']]
 fields=[C.c_uint.in_dll(ref.lib,'dma_endCycles'),C.c_uint8.in_dll(ref.lib,'dma_type'),C.c_uint.in_dll(ref.lib,'dma_length'),C.c_uint8.in_dll(ref.lib,'zstate')]
 bank=C.c_uint.in_dll(ref.lib,'zbank')
 assert b.command_attach(*[C.addressof(x)for x in slots],C.addressof(z),C.addressof(ref.cpu)+CPUHead.cycles.offset,C.addressof(bank),C.addressof(zr),*[C.addressof(x)for x in fields],100000)
 b.command_frame(0)
def pcm_events(b):
 assert not b.command_overflow();n=b.command_count();raw=C.string_at(b.command_events(),n*C.sizeof(command.Event));rows=(command.Event*n).from_buffer_copy(raw)
 return[{k:int(getattr(e,k))for k,_ in command.Event._fields_[:12]}|{'registers':list(e.registers),'state':bytes(e.state).hex(),'busy':int(e.busy),**{k:int(getattr(e,k))for k in ['dma_end','dma_type','dma_remaining','zstate_before','zstate_after']}}for e in rows]
def initialize(ref,quiet,case,lookup,pcm=None,ports=None):
 ref.restore(quiet,restore_refresh_cycles=True)
 # Reconstruct the declared quiescent fixture omitted by native restore.
 image=C.cast(ref.lib.audio_update,C.c_void_p).value-lookup['_audio_update'][0]
 C.c_int.from_address(image+lookup['_fm_cycles_busy'][0]).value=0
 C.c_void_p.from_address(image+lookup['_fm_ptr'][0]).value=image+lookup['_fm_buffer'][0]
 z,zr=command.initial(ref)
 for key,typ in [('dma_endCycles',C.c_uint),('dma_type',C.c_uint8),('dma_length',C.c_uint)]:typ.in_dll(ref.lib,key).value=0
 ref.cpu.cycles=0
 if pcm:bind_pcm(pcm,ref,z,zr)
 if ports:shared.attach(ports,ref)
 action=command.pub(0x73);rom=(ROOT/'.local/md-yuyuhakusho/downloads/yuyuhakusho-jp.bin').read_bytes();resource=command.resource(rom,action)
 hold=ref.api('gen_zbusreq_w',None,C.c_uint,C.c_uint);hold(1,0);command.stage(zr,action,resource);hold(0,0)
 ref.api('z80_run',None,C.c_uint)(case['origin'])
 before={'aux_frontier':int(z.cycles),'busy':int(zr[0x1f2a]),'startup_main_steal_before_declared_origin':ref.cpu.cycles,'player_state_hex':bytes(zr[0x1f80:]).hex()}
 # Origin is a declared conditional main state, not a reconstructed world path.
 ref.cpu.cycles=case['origin'];ref.cpu.refresh_cycles=case['origin']
 setreg=ref.api('m68k_set_reg',None,C.c_uint,C.c_uint)
 for index,value in [(17,0x2700),(15,0xffff00),(0,case['register']),(1,case['value']),(16,0x38b2e if case['bank']==0 else 0x38b9e)]:setreg(index,value)
 write_cpu(ref,0xffff00,0x3d8,4)
 return z,zr,before,resource
def status(ref,zr):
 return {'ModeFlags':int(zr[0x1ffc]),'StartMarker':int(zr[0x1ffd]),'RateRequest':int(zr[0x1ffe]),'SlotBusy':[int(zr[0x1f86]),int(zr[0x1fa6])],'SlotFlags':[int(zr[0x1f87]),int(zr[0x1fa7])],'Busy':int(zr[0x1f2a]),'Held':C.c_uint8.in_dll(ref.lib,'zstate').value==3}
def main():
 p=argparse.ArgumentParser(description=__doc__);p.add_argument('--out',type=Path,required=True);p.add_argument('--smoke',action='store_true');a=p.parse_args();out=a.out.resolve();assert out.is_relative_to(ROOT);out.mkdir(parents=True,exist_ok=True)
 command_path=ROOT/'scripts/yuyuhakusho/pcm-command-capture.py';assert sha(command_path.read_bytes())==PINNED_COMMAND,'Command native producer changed during declared freeze'
 ports,portlib=shared.build(out);pcm=C.CDLL(str(command.BRIDGE));pcm.command_event_size.restype=C.c_uint;assert pcm.command_event_size()==C.sizeof(command.Event)==252;pcm.command_attach.argtypes=[C.c_void_p]*12+[C.c_size_t];pcm.command_attach.restype=C.c_int;pcm.command_count.restype=C.c_size_t;pcm.command_events.restype=C.c_void_p;pcm.command_overflow.restype=C.c_uint;pcm.command_detach.restype=C.c_int
 controls={};results=[];identities={};cases=CASES[2:3]if a.smoke else CASES
 for mode,core in [('original',ORIGINAL),('hook',OBSERVED)]:
  with AudioSession(ROOT,out/('core-'+mode),core=core)as ref:
   quiet=quiet_checkpoint(ref);lookup=symbols(core);identities[mode]=ref.identity
   for case in cases:
    z,zr,before,resource=initialize(ref,quiet,case,lookup);control=shared.execute(ref);plain=ref.save();plain_zram=bytes(zr);plain_status=status(ref,zr)
    z,zr,again,resource=initialize(ref,quiet,case,lookup,pcm,ports);assert before==again,'Conditional service start differed'
    if mode=='hook':ref.api('set_cpu_hook',None,C.c_void_p)(C.cast(ports.score_clock_hook,C.c_void_p))
    try:
     steps=shared.execute(ref,ports);io=shared.events(ports);aux=pcm_events(pcm)
    finally:
     if mode=='hook':ref.api('set_cpu_hook',None,C.c_void_p)(None)
     assert ports.score_clock_detach()==1 and pcm.command_detach()==1;pcm.command_free()
    saved=ref.save();final=status(ref,zr)
    assert steps==control and saved['data']==plain['data']and saved['refresh_cycles']==plain['refresh_cycles']and bytes(zr)==plain_zram and final==plain_status,('Combined recorder changed native',mode,case['id'])
    end={'cycle':ref.cpu.cycles,'refresh':ref.cpu.refresh_cycles,'aux_frontier':int(z.cycles),'status':final,'player_state_hex':bytes(zr[0x1f80:]).hex()}
    row={'input':case,'before_service':before,'resource':resource,'steps':steps,'port_bus_events':io,'PCM_events':aux,'end':end,'all_native_state_equal':True}
    if mode=='original':controls[case['id']]=row
    else:
     assert row==controls[case['id']],('Native builds disagree',case['id']);results.append(row)
    print(json.dumps({'mode':mode,'case':case['id'],'main_end':end['cycle'],'mailbox_reads':sum(e['kind']==3 for e in io),'busy_mailbox_reads':sum(e['kind']==3 and e['value']!=0 for e in io),'PCM_events':len(aux)}),flush=True)
 target=out/'native-bus-services-full.json.gz';shared.saved(target,{'schema':1,'scope':'From empty pub073@0 to given conditional main service origin, actual PCM/bus/mailbox/YM coexistence; not a world or IRQ schedule','identity':identities,'cases':results})
 shared.saved(out/'capture.json',{'pass':True,'cases':len(results),'recorder_on_off_calls':len(results)*4,'identity':identities,'source_sha256':{str(p.relative_to(ROOT)):sha(p.read_bytes())for p in[Path(__file__),command_path,ROOT/'scripts/yuyuhakusho/score-clock-bridge.c']},'artifact':{'path':str(target.relative_to(ROOT)),'sha256':sha(target.read_bytes())},'native_PCM_observer_sha256':sha(command.BRIDGE.read_bytes()),'score_observer_sha256':sha(portlib.read_bytes()),'global_main_schedule_solved':False})
 assert sha(command_path.read_bytes())==PINNED_COMMAND
if __name__=='__main__':main()
