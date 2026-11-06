"""Original transfer-wait flag coexistence with full PCM streams and DSP."""
import argparse,ctypes as C,gzip,json,importlib.util
from pathlib import Path
from reference import ROOT,sha
from extract_audio_assets import AudioSession
from research_audio_loops import quiet_checkpoint
from inspect_dac_dsp_state import symbols,snapshot
spec=importlib.util.spec_from_file_location('gate_command',ROOT/'scripts/yuyuhakusho/pcm-command-capture.py');command=importlib.util.module_from_spec(spec);spec.loader.exec_module(command)
F=command.F

def gate(begin,end,value):return {'kind':'gate','begin':begin,'end':end,'value':value}
def main():
 p=argparse.ArgumentParser(description=__doc__);p.add_argument('--out',type=Path,required=True);a=p.parse_args();out=a.out.resolve();assert out.is_relative_to(ROOT/'.local')and not out.exists();out.mkdir(parents=True)
 cases=[]
 for kind,code,target,frames in [('single',0x73,'single',70),('dual-a',6,'a',48),('dual-b',0x7a,'b',26)]:
  for early in [True,False]:
   begin=5000 if early else 8*F+20000;end=220000 if early else 11*F+35000
   cases.append({'name':kind+('-preload'if early else'-refill'),'frames':frames,'inputs':[command.pub(code),gate(begin,begin+1000,1),gate(end,end+1000,0)]})
   cases[-1]['inputs'][0]['target']=target
 # Repeated short gates test reads which are latched before the next branch.
 for code,target in [(0x73,'single'),(6,'a')]:
  ins=[command.pub(code,target=target)]
  for i in range(8):ins+=[gate(50000+i*17000,50001+i*17000,i&1)]
  cases.append({'name':'repeated-'+target,'frames':12,'inputs':ins})
 controls={};rows=[];identities={};bridge=C.CDLL(str(command.BRIDGE));bridge.command_event_size.restype=C.c_uint;assert bridge.command_event_size()==C.sizeof(command.Event);bridge.command_attach.argtypes=[C.c_void_p]*12+[C.c_size_t];bridge.command_attach.restype=C.c_int;bridge.command_count.restype=C.c_size_t;bridge.command_events.restype=C.c_void_p;bridge.command_overflow.restype=C.c_uint;bridge.command_detach.restype=C.c_int
 for mode,core in [('original',command.ORIGINAL),('observed',command.OBSERVED)]:
  with AudioSession(ROOT,out/mode,core=core)as ref:
   checkpoint=quiet_checkpoint(ref);lookup=symbols(core);identities[mode]=ref.identity
   for case in cases:
    ref.restore(checkpoint,restore_refresh_cycles=True);z,zr=command.initial(ref);initial=snapshot(ref,lookup);resources={};transactions=[];frames=[];rom=ref.rom_path.read_bytes();ram=ref.ram_cpu();video=ref.image
    for action in case['inputs']:
     if action['kind']=='publish':resources[str(action['code'])+action['protocol']]=command.resource(rom,action)
    if mode=='observed':
     slots=[C.c_void_p.in_dll(ref.lib,k)for k in ['fm_write','fm_reset','z80_readmem','z80_writemem']];bank=C.c_uint.in_dll(ref.lib,'zbank');fields=[C.c_uint.in_dll(ref.lib,'dma_endCycles'),C.c_uint8.in_dll(ref.lib,'dma_type'),C.c_uint.in_dll(ref.lib,'dma_length'),C.c_uint8.in_dll(ref.lib,'zstate')];assert bridge.command_attach(*[C.addressof(x)for x in slots],C.addressof(z),C.addressof(ref.cpu)+type(ref.cpu).cycles.offset,C.addressof(bank),C.addressof(zr),*[C.addressof(x)for x in fields],400000)
    hold=ref.api('gen_zbusreq_w',None,C.c_uint,C.c_uint);run=ref.api('z80_run',None,C.c_uint);update=ref.api('audio_update',C.c_int,C.c_void_p);frame_end=C.c_uint.in_dll(ref.lib,'mcycles_vdp');cursor=0
    for frame in range(case['frames']):
     if mode=='observed':bridge.command_frame(frame)
     origin=frame*F
     while cursor<len(case['inputs'])and case['inputs'][cursor]['begin']<origin+F:
      action=case['inputs'][cursor];cursor+=1;assert origin<=action['begin']<=action['end']<origin+F
      hold(1,action['begin']-origin);transaction={'input':action,'grant':z.cycles+origin,'pc':z.pc,'busy':int(zr[0x1f2a])}
      if action['kind']=='publish':command.stage(zr,action,resources[str(action['code'])+action['protocol']])
      else:zr[0x1fff]=action['value']
      hold(0,action['end']-origin);transaction['resume']=z.cycles+origin;transactions.append(transaction)
     run(F);frame_end.value=F;buf=(C.c_int16*8820)();n=update(buf);pcm=C.string_at(buf,n*4);z.cycles-=F
     row={'frame':frame,'pcm_hex':pcm.hex(),'stereo_frames':n,'native_dsp':snapshot(ref,lookup)['initial_state'],'player_state_hex':bytes(zr[0x1f80:]).hex(),'busy':int(zr[0x1f2a]),'wait':int(zr[0x1fff])};frames.append(row)
     assert ref.ram_cpu()==ram and ref.image==video
    body={'case':case,'resources':resources,'initial_dsp':initial,'transactions':transactions,'frames':frames}
    if mode=='original':controls[case['name']]=body
    else:
     assert body==controls[case['name']],('observer changed transfer gate',case['name']);assert bridge.command_detach()==1 and not bridge.command_overflow();count=bridge.command_count();raw=C.string_at(bridge.command_events(),count*C.sizeof(command.Event));events=(command.Event*count).from_buffer_copy(raw);bridge.command_free();body['events']=[{k:int(getattr(e,k))for k,_ in command.Event._fields_[:12]}|{'registers':list(e.registers),'state':bytes(e.state).hex(),'busy':int(e.busy),**{k:int(getattr(e,k))for k in ['dma_end','dma_type','dma_remaining','zstate_before','zstate_after']}}for e in events];rows.append(body)
    print(json.dumps({'core':mode,'case':case['name'],'frames':case['frames']}),flush=True)
 raw=(json.dumps({'version':1,'cases':rows},separators=(',',':'))+'\n').encode();p=out/'native-transfer-gate.json.gz';p.write_bytes(gzip.compress(raw,mtime=0));report={'pass':True,'cases':len(rows),'frames':sum(len(r['frames'])for r in rows),'cross_core_full_frame_pcm_dsp_state_and_transactions_equal':True,'identity':identities,'fixture_sha256':sha(p.read_bytes()),'sources':{str(p.relative_to(ROOT)):sha(p.read_bytes())for p in [Path(__file__),ROOT/'scripts/yuyuhakusho/pcm-command-capture.py',ROOT/'scripts/yuyuhakusho/pcm-command-observer.c']}};(out/'capture.json').write_text(json.dumps(report,indent=2)+'\n');print(json.dumps(report),flush=True)
if __name__=='__main__':main()
