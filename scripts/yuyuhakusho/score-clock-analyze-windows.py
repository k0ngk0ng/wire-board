"""Report actual pause/priority write boundaries, without inferring a clock."""
import argparse,gzip,json
from pathlib import Path
from reference import ROOT,sha
def main():
 p=argparse.ArgumentParser(description=__doc__);p.add_argument('--source',type=Path,required=True);p.add_argument('--out',type=Path,required=True);a=p.parse_args();out=a.out.resolve();assert out.is_relative_to(ROOT)
 raw=a.source.read_bytes();source=json.loads(gzip.decompress(raw));frames={r['frame']:r for r in source['frames']}
 paused=[]
 for n in range(3002,3041):
  es=frames[n]['events'];entry=[e for e in es if e['kind']==8];assert len(entry)==1 and entry[0]['value']==255
  assert not any(e['kind']in[1,2]and e['owner']==1 for e in es),'Paused held tick emitted main chip IO'
  assert entry[0]['zstate']==3,'World/IRQ did not preserve external bus hold'
  paused.append({'frame':n,'driver_entry':entry[0]})
 transitions=[]
 for frame,request,ack,pc in [(3001,1,255,0x37ba2),(3041,128,0,0x37c08)]:
  es=frames[frame]['events'];entry=next(e for e in es if e['kind']==8);assert entry['value']==request
  change=next(e for e in es if e['kind']==10 and e['pc']==pc);assert change['value']==ack
  writes=[e for e in es if e['kind']==2 and e['owner']==1 and e['at']>=change['at']]
  assert writes and writes[0]['at']>change['at']
  transitions.append({'frame':frame,'driver_entry':entry,'actual_pause_field_store':change,'first_main_YM_write_after_store':writes[0],'main_YM_writes_after_store':len(writes),'last_control_end':next(e for e in reversed(es)if e['kind']==5)})
 resumes=[e for e in frames[3041]['events']if e['kind']==5 and e['pc']==0x37c12];assert len(resumes)==1;resume=resumes[0];assert resume['z80']==((resume['at']+14)//15)*15
 priority=[]
 for row in source['frames']:
  es=row['events']
  for i,e in enumerate(es):
   if e['kind']!=9 or e['value']!=0:continue
   before=next((v for v in reversed(es[:i])if v['kind']==2 and v['owner']==1),None)
   after=next((v for v in es[i+1:]if v['kind']==2 and v['owner']==1),None)
   priority.append({'frame':row['frame'],'actual_priority_store':e,'last_main_YM_write_before':before,'next_main_YM_write_after':after})
 assert any(x['actual_priority_store']['pc']==0x390f6 and x['last_main_YM_write_before']and x['next_main_YM_write_after']for x in priority)
 result={'pass':True,'scope':'Actual selected natural IO/owned-field boundaries only; not a complete driver cost predictor','source':{'path':str(a.source.resolve().relative_to(ROOT)),'sha256':sha(raw)},'pause_transitions':transitions,'held_pause_ticks':paused,'resume_bus_release':resume,'priority_store_windows':priority,'priority_summary_bool_has_no_unique_time':True,'global_independent_schedule_solved':False,'producer_sha256':sha(Path(__file__).read_bytes()),'limits':['Late world/IRQ bus assertions are distinct from the leaf YM service release; do not infer final frame hold from the leaf alone','The stale held Z80 uint32 cursor can wrap as frames subtract; release overwrites it with ceil15 rather than max','Only existing pinned-source selected windows are retained; this is no all-possible-input execution claim']}
 out.parent.mkdir(parents=True,exist_ok=True);out.write_text(json.dumps(result,indent=2)+'\n');print(json.dumps({'pass':True,'held_ticks':len(paused),'priority_windows':len(priority)}))
if __name__=='__main__':main()
