#!/usr/bin/env python3
"""Closed-form semantic row-loop budgets; never executes or decodes a CPU.

Only fixed structural counts from the recovered20918 loops enter the model.
Original PCs label verification intervals, never drive production playback.
"""
import argparse,gzip,json,math
from pathlib import Path

ROOT=Path(__file__).resolve().parents[2]
MODEL=[
 {'name':'rows-a-main','start':0x209da,'end':0x209f8,'setup_cpu':40,'iterations':16,'body_cpu':48,'dbf_taken':10,'dbf_last':14,'meaning':'128 row words; four long stores per iteration'},
 {'name':'rows-a-tail','start':0x209f8,'end':0x20a12,'setup_cpu':32,'iterations':4,'body_cpu':48,'dbf_taken':10,'dbf_last':14,'meaning':'32 row words; four long stores per iteration'},
 {'name':'rows-b-prefix','start':0x20a12,'end':0x20a30,'setup_cpu':40,'iterations':8,'body_cpu':48,'dbf_taken':10,'dbf_last':14,'meaning':'64 row words; four long stores per iteration'},
 {'name':'wave-first','start':0x20a30,'end':0x20a62,'setup_cpu':72,'iterations':16,'body_cpu':52,'dbf_taken':10,'dbf_last':14,'meaning':'16 rows; indexed byte wave and arithmetic shift4'},
 {'name':'wave-second','start':0x20a62,'end':0x20a94,'setup_cpu':72,'iterations':16,'body_cpu':50,'dbf_taken':10,'dbf_last':14,'meaning':'16 rows; indexed byte wave and arithmetic shift3'},
 {'name':'rows-b-flat','start':0x20a94,'end':0x20ab4,'setup_cpu':48,'iterations':9,'body_cpu':48,'dbf_taken':10,'dbf_last':14,'meaning':'72 row words; four long stores per iteration'},
 {'name':'parallax-tail','start':0x20ab4,'end':0x20ade,'setup_cpu':92,'iterations':7,'body_cpu':80,'dbf_taken':10,'dbf_last':14,'meaning':'56 ROM row words; four long copies per iteration'},
]

def budget(phase):
 n=phase['iterations'];cpu=phase['setup_cpu']+n*phase['body_cpu']+(n-1)*phase['dbf_taken']+phase['dbf_last']
 base=cpu*7
 # Instruction-boundary bus refresh adds14 master cycles, with a new deadline
 # 896 after that boundary. A conservative interval accounts for refresh cost
 # extending the block itself, without reading an observed refresh count.
 upper_refresh=1+math.ceil(base/(896-14))
 return {'cpu_base':cpu,'master_base':base,'master_lower':base,'master_upper':base+14*upper_refresh}

def main():
 p=argparse.ArgumentParser(description=__doc__);p.add_argument('--capture',type=Path,required=True);p.add_argument('--out',type=Path,required=True);args=p.parse_args()
 source=json.loads(gzip.decompress(args.capture.read_bytes()));checks=[];mismatches=[]
 for case in source['cases']:
  blocks=[event for event in case['events']if event[0]==6]
  for phase in MODEL:
   starts=[(i,event)for i,event in enumerate(blocks)if event[1]&0xffffff==phase['start']]
   if not starts:continue
   assert len(starts)==1
   index,start=starts[0];end=blocks[index+1];assert end[1]&0xffffff==phase['end']
   predicted=budget(phase);elapsed=end[6]-start[6];refreshes=end[19]
   row={'case':case['seed'],'phase':phase['name'],'observed_master':elapsed,'observed_refreshes':refreshes,'predicted':predicted,'exact_base_plus_refresh':elapsed==predicted['master_base']+14*refreshes,'bounded_without_observed_refresh_input':predicted['master_lower']<=elapsed<=predicted['master_upper']}
   checks.append(row)
   if not row['exact_base_plus_refresh']or not row['bounded_without_observed_refresh_input']:mismatches.append(row)
 out=args.out.resolve();assert out.is_relative_to(ROOT)
 report={'pass':not mismatches,'scope':'Semantic fixed-row loop cost prototype only; no globalIRQ/driver/DMA/synthesis timing model','cases':len(source['cases']),'phase_checks':len(checks),'model':MODEL,'checks':checks,'mismatches':mismatches,'limits':['Current virtual clock/IRQ insertion and data-dependent camera/divide paths remain separate obligations','Observed refresh count is used only for oracle attribution; the interval predictor never consumes it','No CPU/register/RAM machine is executed by this closed-form model']}
 out.write_text(json.dumps(report,indent=2)+'\n')
 print(json.dumps({'pass':report['pass'],'phase_checks':len(checks),'mismatches':len(mismatches),'examples':mismatches[:3]}))
 if not report['pass']:raise SystemExit(1)
if __name__=='__main__':main()
