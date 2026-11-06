"""Full native chip oracle for the23 actual PCM physical-port command streams.

This replay is the given-native-port layer, not the autonomous Go Device proof.
No address is synthesized. A captured quiescent native scalar sidecar restores
the FM fields omitted by the native save format; no Go output supplies expected
samples or state. Complete native initial YM/PSG/FIR is read directly.
"""
from pathlib import Path
import argparse,ctypes as C,gzip,json,runpy
from reference import ROOT,sha
from extract_audio_assets import AudioSession
from research_audio_loops import quiet_checkpoint
from inspect_dac_dsp_state import symbols
from fmpsg_capture_ym import ORIGINAL,PINNED

F=896040
PORTS=ROOT/'games/yuyuhakusho/audio/pcmcommands/testdata/original-command-ports-v3.json.gz'
PORTS_SHA='e435f0f778088e0bb133082f82720c8ca7ed10c7d13be388faea837d76fff995'
OWNER_CAPTURE=ROOT/'scripts/yuyuhakusho/pcm-command-capture.py'
OWNER_INITIAL=runpy.run_path(str(OWNER_CAPTURE))['initial']
RESET_ORACLE=ROOT/'scripts/yuyuhakusho/fmpsg-reset-capture.py'
Oracle=runpy.run_path(str(RESET_ORACLE))['Oracle']
KINDS={'ym-address','dac','pan','enable','ym-write'}

def partial(state):
 return dict(Time=state['Time'],DACValue=state['YM']['DACValue'],Enabled=state['YM']['DACEnabled'],Pan=state['YM']['Channels'][5]['Pan'],LastChipOutput=state['LastFMOutput'],Integrator=state['Integrator'],Tail=state['Tail'])
def captured_fields(source):return {key:source[key]for key in ['Time','DACValue','Enabled','Pan','LastChipOutput','Integrator','Tail']}

def scalar_sidecar(ref,lookup):
 image=C.cast(ref.lib.psg_reset,C.c_void_p).value-lookup['_psg_reset'][0]
 base=image+lookup['_fm_buffer'][0]
 values={name:C.c_int.from_address(image+lookup['_'+name][0]).value for name in ['fm_cycles_start','fm_cycles_count','fm_cycles_busy']}
 values['fm_ptr_offset_bytes']=C.c_void_p.from_address(image+lookup['_fm_ptr'][0]).value-base
 return values
def restore_sidecar(ref,lookup,values):
 image=C.cast(ref.lib.psg_reset,C.c_void_p).value-lookup['_psg_reset'][0]
 # The native save format owns start. The omitted values are restored only
 # from observed original checkpoint values, never fitted to a result.
 assert C.c_int.from_address(image+lookup['_fm_cycles_start'][0]).value==values['fm_cycles_start']
 changes=[]
 for name in ['fm_cycles_count','fm_cycles_busy']:
  field=C.c_int.from_address(image+lookup['_'+name][0])
  if field.value!=values[name]:changes.append(dict(field=name,before=field.value,restored=values[name]));field.value=values[name]
 pointer=C.c_void_p.from_address(image+lookup['_fm_ptr'][0]);desired=image+lookup['_fm_buffer'][0]+values['fm_ptr_offset_bytes']
 if pointer.value!=desired:changes.append(dict(field='fm_ptr_offset_bytes',restored=values['fm_ptr_offset_bytes']));pointer.value=desired
 assert scalar_sidecar(ref,lookup)==values
 return changes

def capture(core,out,cases):
 lookup=symbols(core);rows=[];reports=[]
 with AudioSession(ROOT,out,core=core)as ref:
  checkpoint=quiet_checkpoint(ref);origin=checkpoint['frame']*F;sidecar=scalar_sidecar(ref,lookup)
  quiescent=Oracle(ref,lookup,origin);full=quiescent.committed(quiescent.snapshot())
  initial_identity=sha((json.dumps(full,separators=(',',':'))+'\n').encode())
  for source in cases:
   assert source['MasterOrigin']==origin
   ref.restore(checkpoint,restore_refresh_cycles=True);changes=restore_sidecar(ref,lookup,sidecar)
   initial_oracle=Oracle(ref,lookup,origin);before=initial_oracle.committed(initial_oracle.snapshot());assert before==full,('native restore does not recreate full quiescent chip state',source['Name'])
   OWNER_INITIAL(ref)
   observer=Oracle(ref,lookup,origin);initial=observer.committed(observer.snapshot());assert initial==before,('PCM initial changed chip state',source['Name'])
   assert partial(initial)==captured_fields(source['InitialDSP'])
   frames=[];last_chip=None;physical_count=0;raw_backsteps=[]
   for frame_index,frame in enumerate(source['Frames']):
    for event in frame['Events']:
     kind=event['Kind']
     if kind in KINDS:
      assert event['Port']in ([0,2]if kind=='ym-address'else[1,3]),('wrong physical port',source['Name'],event)
      observer.apply(dict(Kind='write',Cycle=event['Cycle'],Port=event['Port'],Value=event['Value'],Observe=False))
     elif kind=='reset':
      if event['Cycle']<observer.cursor:
       assert last_chip is not None and last_chip['Kind']=='dac'and last_chip['Cycle']==observer.cursor,('reset lacks actual DAC cause',source['Name'],event)
       raw_backsteps.append(dict(complete=last_chip['Cycle'],request=event['Cycle']))
       observer.apply(dict(Kind='after-dac',Cycle=event['Cycle'],CompletedDAC=last_chip['Cycle'],Observe=False))
      else:observer.apply(dict(Kind='reset',Cycle=event['Cycle'],Observe=False))
     elif kind in ['bank','metadata','dummy']:continue
     else:raise AssertionError(('unknown original physical event',kind))
     last_chip=event;physical_count+=1
    row=observer.apply(dict(Kind='frame',Cycle=0));state=row['Committed']
    expected_pcm=bytes.fromhex(frame['PCMHex']);actual_pcm=__import__('base64').b64decode(row['PCM'])
    assert actual_pcm==expected_pcm,('native full port replay does not match original CPU PCM',source['Name'],frame_index)
    assert partial(state)==captured_fields(frame['DSP']),('original captured DSP fields differ',source['Name'],frame_index)
    assert state['YM']['Address']==frame['YMAddress'],('actual physical latch differs',source['Name'],frame_index)
    frames.append(dict(PCM=row['PCM'],State=state))
   rows.append(dict(Name=source['Name'],SourceSHA256=source['SourceSHA256'],Config=dict(Profile=1,MasterOrigin=origin),Initial=initial,Frames=frames))
   reports.append(dict(name=source['Name'],frames=len(frames),chip_operations=physical_count,full_initial_sha256=initial_identity,sidecar_restore=changes,raw_reset_backsteps=raw_backsteps,native_projection_checks=observer.checks,original_cpu_pcm_and_captured_DSP_equal=True,actual_original_address_latches_equal=True))
   print(json.dumps(dict(core=core.name,case=source['Name'],frames=len(frames),chip_operations=physical_count)),flush=True)
 return rows,dict(identity=ref.identity,observed_quiescent_fm_sidecar=sidecar,full_initial_sha256=initial_identity,cases=reports)

def main():
 p=argparse.ArgumentParser(description=__doc__);p.add_argument('--out',type=Path,required=True);a=p.parse_args();out=a.out.resolve();assert out.is_relative_to(ROOT);out.mkdir(parents=True,exist_ok=True)
 assert sha(PORTS.read_bytes())==PORTS_SHA
 source=json.loads(gzip.decompress(PORTS.read_bytes()));assert source['Version']==3 and len(source['Cases'])==23
 original,original_report=capture(ORIGINAL,ROOT/'.local/md-yuyuhakusho/fmpsg-pcm-integration-capture/original',source['Cases'])
 observed,observed_report=capture(PINNED,ROOT/'.local/md-yuyuhakusho/fmpsg-pcm-integration-capture/observed',source['Cases'])
 assert original==observed,'original/hooked full initial, chip state or PCM differ'
 raw=(json.dumps(dict(Version=1,Cases=original),separators=(',',':'))+'\n').encode();compressed=gzip.compress(raw,mtime=0);(out/'fmpsg-pcm-integration-native.json.gz').write_bytes(compressed)
 report=dict(pass_=True,scope='Given original actual physical-port streams into full native chips from captured complete quiescent state; this is not autonomous Device output or global world scheduling proof',cases=len(original),frames=sum(len(c['Frames'])for c in original),stereo_frames=sum(len(__import__('base64').b64decode(f['PCM']))//4 for c in original for f in c['Frames']),physical_ledger_events=sum(len(f['Events'])for c in source['Cases']for f in c['Frames']),raw_cycles_modified=False,address_times_synthesized=False,source_ports_sha256=PORTS_SHA,source_owner_capture_sha256=sha(OWNER_CAPTURE.read_bytes()),projector_sha256=sha(RESET_ORACLE.read_bytes()),producer_sha256=sha(Path(__file__).read_bytes()),json_sha256=sha(raw),gzip_sha256=sha(compressed),gzip_bytes=len(compressed),original=original_report,observed=observed_report,limits=['Source InitialDSP is only a limited original scalar projection; complete initial YM/PSG/FIR above is directly observed and checked unchanged by PCM initialization','Source DSP.InputAhead is an exporter-derived DAC-only cursor, not a complete physical-port frontier; the listed actual original DSP scalar fields and all original PCM/address latches are independently checked','Native omitted FM checkpoint fields are restored solely from the observed original quiescent sidecar; no production context or Go expected output enters the oracle'])
 report['pass']=report.pop('pass_');(out/'fmpsg-pcm-integration-native-capture.json').write_text(json.dumps(report,indent=2)+'\n');print(json.dumps({k:report[k]for k in ['pass','cases','frames','stereo_frames','physical_ledger_events','gzip_sha256','gzip_bytes']}))
if __name__=='__main__':main()
