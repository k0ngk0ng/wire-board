"""Report actual original successful mailbox retry boundaries."""
import argparse,gzip,json
from pathlib import Path
from reference import ROOT,sha
def main():
 p=argparse.ArgumentParser(description=__doc__);p.add_argument('--source',type=Path,required=True);p.add_argument('--out',type=Path,required=True);a=p.parse_args();out=a.out.resolve();assert out.is_relative_to(ROOT)
 raw=a.source.read_bytes();s=json.loads(gzip.decompress(raw));rows=[]
 for row in s['cases']:
  io=row['port_bus_events'];requests=[];pending=None
  for e in io:
   if e['kind']==4:assert pending is None;pending=e
   elif e['kind']==5:
    assert pending is not None;v=pending['value']>>8 if pending['width']==2 else pending['value'];requests.append({'at':pending['at'],'after':e['after'],'asserted':bool(v&1),'aux_before':pending['z80'],'aux_after':e['z80'],'new_main_steal':e['after']-pending['at']});pending=None
  mailbox=[{'at':e['after'],'value':e['value'],'held_aux_frontier':e['z80']}for e in io if e['kind']==3]
  rows.append({'case':row['input']['id'],'origin':row['input']['origin'],'bank':row['input']['bank'],'startup_main_steal_before_declared_origin':row['before_service']['startup_main_steal_before_declared_origin'],'requests':requests,'mailbox':mailbox,'busy_mailbox_reads':sum(e['value']!=0 for e in mailbox),'main_end':row['end']['cycle'],'refresh_end':row['end']['refresh'],'aux_end':row['end']['aux_frontier']})
 critical=next(r for r in rows if r['case']=='critical64-bank0');assert critical['requests'][0]['at']==71535 and critical['mailbox'][0]['value']==64 and critical['mailbox'][0]['at']>71640 and critical['mailbox'][-1]['value']==0
 report={'pass':True,'scope':'Original actual shared bus/mailbox/main port boundaries, not fake wall-time clears or independent global scheduling','source':{'path':str(a.source.resolve().relative_to(ROOT)),'sha256':sha(raw)},'cases':rows,'fake_time_mailbox_counterexample':{'conditional_origin':70933,'original_unheld_clear_would_be':71640,'actual_held_read':critical['mailbox'][0],'actual_clear_requires_real_release_then_aux_execution':True},'producer_sha256':sha(Path(__file__).read_bytes()),'limits':['Only main origin is declared, not a reconstructed preceding main CPU path','Auxiliary raw address/latch intermediate states and full mixed PCM are not asserted by the semantic-data integration view']}
 out.parent.mkdir(parents=True,exist_ok=True);out.write_text(json.dumps(report,indent=2)+'\n');print(json.dumps({'pass':True,'cases':len(rows),'busy_mailbox_reads':sum(r['busy_mailbox_reads']for r in rows)}))
if __name__=='__main__':main()
