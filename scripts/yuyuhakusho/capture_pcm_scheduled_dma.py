"""Actual PCM plus explicitly driven original VDP DMA, including waiting across a frame."""
import argparse,ctypes as C,gzip,json
from pathlib import Path
from reference import ROOT,sha
from extract_audio_assets import AudioSession
from research_audio_loops import quiet_checkpoint
from inspect_dac_dsp_state import symbols,snapshot
from capture_pcm_transfer_gate import command,gate
from capture_world_dma_segments import Bitmap
F=896040;L=3420
def main():
 p=argparse.ArgumentParser(description=__doc__);p.add_argument('--out',type=Path,required=True);a=p.parse_args();out=a.out.resolve();assert out.is_relative_to(ROOT/'.local')and not out.exists();out.mkdir(parents=True)
 cases=[]
 for name,code,target,frames in [('single',0x73,'single',70),('dual-a',6,'a',48),('dual-b',0x7a,'b',26)]:
  cases.append({'name':name+'-scheduled-dma','frames':frames,'inputs':[command.pub(code,target=target)],'dma_begin':8*F+20000,'dma_words':560})
 for name,code,target,frames in [('single',0x73,'single',70),('dual-a',6,'a',48)]:
  cases.append({'name':name+'-cross-frame-dma','frames':frames,'inputs':[command.pub(code,target=target)],'dma_begin':9*F-1000,'dma_words':560})
 cases.append({'name':'single-gate-with-scheduled-dma','frames':70,'inputs':[command.pub(0x73),gate(8*F+10000,8*F+11000,1),gate(11*F+10000,11*F+11000,0)],'dma_begin':8*F+20000,'dma_words':560})
 for name,code,target,frames in [('single',0x73,'single',70),('dual-a',6,'a',48),('dual-b',0x7a,'b',26)]:
  cases.append({'name':name+'-active-line-dma','frames':frames,'inputs':[command.pub(code,target=target)],'dma_begin':8*F+200000,'dma_words':560})
 cases.append({'name':'single-gate-with-active-line-dma','frames':70,'inputs':[command.pub(0x73),gate(8*F+190000,8*F+191000,1),gate(11*F+10000,11*F+11000,0)],'dma_begin':8*F+200000,'dma_words':560})
 b=C.CDLL(str(command.BRIDGE));b.command_event_size.restype=C.c_uint;b.command_attach.argtypes=[C.c_void_p]*12+[C.c_size_t];b.command_attach.restype=C.c_int;b.command_count.restype=C.c_size_t;b.command_events.restype=C.c_void_p;b.command_overflow.restype=C.c_uint;b.command_detach.restype=C.c_int;assert b.command_event_size()==C.sizeof(command.Event)
 controls={};rows=[];identities={}
 for mode,core in [('original',command.ORIGINAL),('observed',command.OBSERVED)]:
  with AudioSession(ROOT,out/mode,core=core)as ref:
   checkpoint=quiet_checkpoint(ref);lookup=symbols(core);identities[mode]=ref.identity
   for case in cases:
    ref.restore(checkpoint,restore_refresh_cycles=True);z,zr=command.initial(ref);initial=snapshot(ref,lookup);resources={};transactions=[];frames=[];steps=[];rom=ref.rom_path.read_bytes();ram=ref.ram_cpu();video=ref.image
    for action in case['inputs']:
     if action['kind']=='publish':resources[str(action['code'])+action['protocol']]=command.resource(rom,action)
    de=C.c_uint.in_dll(ref.lib,'dma_endCycles');dt=C.c_uint8.in_dll(ref.lib,'dma_type');dl=C.c_uint.in_dll(ref.lib,'dma_length');zs=C.c_uint8.in_dll(ref.lib,'zstate');status=C.c_uint16.in_dll(ref.lib,'status');reg=(C.c_uint8*32).in_dll(ref.lib,'reg');bitmap=Bitmap.in_dll(ref.lib,'bitmap');bitmap.viewport.w=bitmap.viewport.ow=320;bitmap.viewport.h=bitmap.viewport.oh=224
    if mode=='observed':
     slots=[C.c_void_p.in_dll(ref.lib,k)for k in ['fm_write','fm_reset','z80_readmem','z80_writemem']];bank=C.c_uint.in_dll(ref.lib,'zbank');assert b.command_attach(*map(C.addressof,slots),C.addressof(z),C.addressof(ref.cpu)+type(ref.cpu).cycles.offset,C.addressof(bank),C.addressof(zr),*[C.addressof(x)for x in [de,dt,dl,zs]],400000)
    hold=ref.api('gen_zbusreq_w',None,C.c_uint,C.c_uint);run=ref.api('z80_run',None,C.c_uint);update=ref.api('audio_update',C.c_int,C.c_void_p);line=C.c_uint.in_dll(ref.lib,'mcycles_vdp');control=ref.api('vdp_68k_ctrl_w',None,C.c_uint);advance=ref.api('vdp_dma_update',None,C.c_uint);cursor=0;started=False;de.value=dl.value=0;dt.value=0;ref.cpu.cycles=0
    for frame in range(case['frames']):
     if mode=='observed':b.command_frame(frame)
     origin=frame*F;de.value=0
     def scheduled_run(target):
      before=ref.cpu.cycles
      if zs.value==1:run(target)
      steps.append({'Kind':'run','Frame':frame,'At':origin+target,'DMA':{'Begin':0,'End':de.value,'Mode':dt.value,'Remaining':dl.value},'Frontier':origin+z.cycles,'Delay':ref.cpu.cycles-before,'Waiting':bool(zs.value&4)})
     def progress(at,begin=False):
      line.value=(at//L)*L;status.value=(status.value&~8)|(8 if at<37*L else 0);before_time=z.cycles;before_wait=bool(zs.value&4);ref.cpu.cycles=at
      if begin:
       # Real public VDP control writes initialize its private source/code.
       control(0x8124);control(0x8c81);control(0x9300|(case['dma_words']&255));control(0x9400|(case['dma_words']>>8));control(0x9500);control(0x9600);control(0x977f);control(0x8f02);control(0x8134);control(0x4000);control(0x80)
      else:advance(at)
      assert dt.value==1
      steps.append({'Kind':'dma','Frame':frame,'At':origin+at,'End':origin+de.value,'Remaining':dl.value,'FrontierBefore':origin+before_time,'Frontier':origin+z.cycles,'WaitingBefore':before_wait,'Waiting':bool(zs.value&4)})
     # A pending DMA is continued by VDP before the new frame's first CPU
     # batch. Its first blank interval is then split at the real VINT edge.
     if dl.value:
      progress(0);scheduled_run(788)
      target=L
      while True:
       scheduled_run(target)
       if not dl.value or target==F:break
       progress(target);target+=L
     while cursor<len(case['inputs'])and case['inputs'][cursor]['begin']<origin+F:
      action=case['inputs'][cursor];cursor+=1;assert action['begin']>=origin and action['end']<origin+F
      before=ref.cpu.cycles;hold(1,action['begin']-origin);tr={'input':action,'grant':origin+z.cycles,'main_delay':ref.cpu.cycles-before,'pc':z.pc,'busy':int(zr[0x1f2a])}
      if action['kind']=='publish':command.stage(zr,action,resources[str(action['code'])+action['protocol']])
      else:zr[0x1fff]=action['value']
      hold(0,action['end']-origin);tr['resume']=origin+z.cycles;transactions.append(tr);steps.append({'Kind':'input','Frame':frame,'Index':len(transactions)-1})
     if not started and origin<=case['dma_begin']<origin+F:
      at=case['dma_begin']-origin;scheduled_run(at//L*L);progress(at,True);started=True;target=(at//L+1)*L
      while True:
       scheduled_run(target)
       if not dl.value or target==F:break
       progress(target);target+=L
     if not steps or steps[-1].get('At')!=origin+F:scheduled_run(F)
     line.value=F;buf=(C.c_int16*8820)();n=update(buf);pcm=C.string_at(buf,n*4);z.cycles-=F;ref.cpu.cycles-=F
     frames.append({'frame':frame,'pcm_hex':pcm.hex(),'stereo_frames':n,'native_dsp':snapshot(ref,lookup)['initial_state'],'player_state_hex':bytes(zr[0x1f80:]).hex(),'busy':int(zr[0x1f2a]),'wait':int(zr[0x1fff]),'dma_waiting':bool(zs.value&4),'frontier':origin+F+z.cycles});steps.append({'Kind':'frame','Frame':frame})
     assert ref.ram_cpu()==ram and ref.image==video
    body={'case':case,'resources':resources,'initial_dsp':initial,'transactions':transactions,'frames':frames,'steps':steps}
    if mode=='original':controls[case['name']]=body
    else:
     assert body==controls[case['name']],('scheduled DMA observer changed original execution',case['name']);assert b.command_detach()==1 and not b.command_overflow();count=b.command_count();raw=C.string_at(b.command_events(),count*C.sizeof(command.Event));events=(command.Event*count).from_buffer_copy(raw);b.command_free();body['events']=[{k:int(getattr(e,k))for k,_ in command.Event._fields_[:12]}|{'registers':list(e.registers),'state':bytes(e.state).hex(),'busy':int(e.busy),**{k:int(getattr(e,k))for k in ['dma_end','dma_type','dma_remaining','zstate_before','zstate_after']}}for e in events];rows.append(body)
    print(json.dumps({'core':mode,'case':case['name'],'frames':case['frames']}),flush=True)
 raw=(json.dumps({'version':1,'cases':rows},separators=(',',':'))+'\n').encode();fixture=out/'native-transfer-gate.json.gz';fixture.write_bytes(gzip.compress(raw,mtime=0));report={'pass':True,'cases':len(rows),'frames':sum(len(r['frames'])for r in rows),'cross_core_full_pcm_dsp_player_schedule_equal':True,'declared_viewport':[320,224],'wait_latches':sum(e['kind']==3 and e['zstate_after']&4 and not e['zstate_before']&4 for r in rows for e in r['events']),'waiting_progress_steps':sum(s['Kind']=='dma'and s['WaitingBefore']for r in rows for s in r['steps']),'identity':identities,'fixture_sha256':sha(fixture.read_bytes()),'sources':{str(p.relative_to(ROOT)):sha(p.read_bytes())for p in [Path(__file__),ROOT/'scripts/yuyuhakusho/pcm-command-observer.c',ROOT/'scripts/yuyuhakusho/pcm-command-capture.py']}};(out/'capture.json').write_text(json.dumps(report,indent=2)+'\n');print(json.dumps(report),flush=True)
if __name__=='__main__':main()
