"""Read-only arithmetic and reference-bus/cadence evidence analysis."""
from pathlib import Path
import argparse,collections,gzip,json
from reference import ROOT,sha

def main():
 parser=argparse.ArgumentParser(description=__doc__);parser.add_argument('--capture',type=Path,required=True);parser.add_argument('--out',type=Path,required=True);parser.add_argument('--expect-constant-cadence',action='store_true');a=parser.parse_args()
 capture=a.capture.resolve();out=a.out.resolve();assert capture.is_relative_to(ROOT)and out.is_relative_to(ROOT)
 r=json.loads((capture/'capture.json').read_text());assert r['pass']and r['all_original_observed_frames_equal'];tables=r['tables'];sample_rows=json.loads((capture/'samples.json').read_text())['samples'];samples={x['code']:bytes.fromhex(x['encoded_hex'])for x in sample_rows}
 encoded_pcs={0x12b:'single',0x1a4:'single',0x2a2:'a',0x4dd:'a',0x2bb:'b',0x502:'b'};first_mix={0x2dd,0x518};second_mix={0x349,0x584,0x376,0x5b1}
 cases=[]
 for case in r['cases']:
  path=capture/(case['name']+'-events.json.gz');body=json.loads(gzip.decompress(path.read_bytes()));events=body['events'];roles={'single':0x73}if case['name']=='single-mode'else{'a':case['commands'][0][1]}|({'b':case['commands'][1][1]}if len(case['commands'])>1 else{})
  state={role:128 for role in roles};positions={role:0 for role in roles};value={'single':[128,128],'a':[128,128],'b':[128,128]};pending=None;pairs=dac_count=clamped=0
  native_pairs={p['encoded_event']:p for p in body['decoder_pairs']};bank=[];dac=[];mode_changes=[]
  for index,e in enumerate(events):
   role=encoded_pcs.get(e['pc'])if e['kind']==4 else None
   if role:
    b=samples[roles[role]][positions[role]];assert b==e['data']and state[role]==(e['registers'][2]>>8&255)
    hi=(state[role]+tables['delta_high'][b])&255;lo=(hi+tables['delta_low'][b])&255;n=native_pairs[index];assert(hi,lo)==(n['native_high'],n['native_low'])
    value[role]=[hi,lo];state[role]=lo;positions[role]+=1;pairs+=1
   if e['kind']==6:
    assert pending is None;assert e['pc']in first_mix|second_mix;half=0 if e['pc']in first_mix else 1
    raw=value['a'][half]+value['b'][half]-128;pending=max(0,min(255,raw));clamped+=raw<0 or raw>255;assert pending==e['data']
    if half:value['a']=value['b']=[128,128]
   if e['kind']==1:
    expected=value['single'][0 if e['pc']in(0x137,0x1b0)else 1]if case['name']=='single-mode'else pending;assert expected==e['data'];pending=None;dac_count+=1;dac.append(e)
   if e['kind']==3:
    assert e['z80_after']-e['z80_before']==45
    assert e['m68k_after']-e['m68k_before']==((e['z80_before']%7+72)//7)*7;bank.append(e)
   if e['kind']==7 and e['address']in(0x1f86,0x1fa6,0x1ffc):mode_changes.append({'video_step':e['frame'],'reference_cycle':e['cycles_before'],'pc':hex(e['pc']),'address':hex(e['address']),'value':e['data']})
  assert pending is None and pairs==case['decoder_pairs']and dac_count==case['dac_events']
  for role,code in roles.items():assert positions[role]==len(samples[code])
  times=[e['frame']*896040+e['cycles_before']for e in dac];intervals=collections.Counter(b-a for a,b in zip(times,times[1:]));assert all(k>0 for k in intervals)
  cases.append({'name':case['name'],'source_path':str(path.relative_to(ROOT)),'source_sha256':sha(path.read_bytes()),'encoded_bytes_by_role':positions,'all_payloads_consumed_from_initial128':True,'decoded_pairs':pairs,'ordered_dac_values':dac_count,'value_mismatches':0,'dual_8bit_clamps':clamped,'bank_reads':len(bank),'reference_bank_z80_wait_cycles':dict(collections.Counter(e['z80_after']-e['z80_before']for e in bank)),'reference_bank_m68k_stolen_cycles':dict(collections.Counter(e['m68k_after']-e['m68k_before']for e in bank)),'reference_bus_formula_mismatches':0,'reference_dac_interval_master_cycles':dict(sorted(intervals.items())),'constant_dac_cadence':len(intervals)<=1,'mode_ack_end_writes':mode_changes})
 result={'pass':not a.expect_constant_cadence or all(c['constant_dac_cadence']for c in cases),'scope':'Whole encoded payloads and ordered nativeDAC value matches; reference timing observation only','table_sha256':tables['source_table_sha256'],'cases':cases,'totals':{'encoded_bytes':sum(c['decoded_pairs']for c in cases),'decoded_samples':sum(c['decoded_pairs']*2 for c in cases),'ordered_dac_values':sum(c['ordered_dac_values']for c in cases),'dual_8bit_clamps':sum(c['dual_8bit_clamps']for c in cases),'bank_reads':sum(c['bank_reads']for c in cases),'value_mismatches':0},'clock_basis':{'reference_master_cycles_per_ntsc_video_frame':896040,'event_time_point':'Reference handler invocation, not a claimed real chip pin edge','bus_formula':'M68k += floor((Z80.cycles % 7 + 72)/7)*7; Z80 +=45. This is the pinned core empirical model.'},'independent_cadence_implemented':False,'ym2612_dac_filter_resampler_implemented':False,'analyzer_sha256':sha(Path(__file__).read_bytes())}
 out.parent.mkdir(parents=True,exist_ok=True);out.write_text(json.dumps(result,indent=2)+'\n');print(json.dumps({'pass':result['pass'],'totals':result['totals']}))
 if not result['pass']:raise SystemExit(1)
if __name__=='__main__':main()
