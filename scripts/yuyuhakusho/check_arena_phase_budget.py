#!/usr/bin/env python3
"""Apply the frozen closed-form arena budgets to original natural intervals."""
import argparse,gzip,json
from collections import Counter
from pathlib import Path
from arena_phase_cost_model import MODEL,budget
ROOT=Path(__file__).resolve().parents[2]

def main():
 p=argparse.ArgumentParser(description=__doc__);p.add_argument('--source',type=Path,required=True);p.add_argument('--out',type=Path,required=True);args=p.parse_args()
 r=json.loads(gzip.decompress(args.source.read_bytes()));events=r['events'];calls=[];pending=None;checks=[];outside=[];interrupts=[];irq_stack=[];fixed={phase['start']:phase for phase in MODEL}
 for i,e in enumerate(events):
  pc=e[1]&0xffffff
  if e[0]==5:assert pending is None;pending={'start':i,'entry':e,'blocks':[],'irq':[]}
  elif e[0]==6 and pending is not None:pending['blocks'].append((i,e))
  elif e[0]==7 and pending is not None:pending['end']=i;pending['exit']=e;calls.append(pending);pending=None
  if e[0]==3:
   irq_stack.append((i,e))
   if pending is not None:pending['irq'].append((i,e))
  elif e[0]==4 and irq_stack:irq_stack.pop()
  if e[0]==1 and pc==0x375ae and irq_stack:
   irq=irq_stack[-1]
   interrupts.append({'tick':i,'tick_event':e,'irq':irq[0],'irq_event':irq[1]})
 for call in calls:
  bs=call['blocks']
  for index,(at,start)in enumerate(bs):
   phase=fixed.get(start[1]&0xffffff)
   if phase is None:continue
   _,end=bs[index+1];assert end[1]&0xffffff==phase['end']
   elapsed=(end[5]-start[5])*896040+end[6]-start[6];pred=budget(phase)
   row={'callback_start_source':call['entry'][-1],'frame':start[5],'phase':phase['name'],'elapsed_master':elapsed,'observed_refreshes':end[19],'irq_entries':end[20],'bus_accesses':end[21],'prediction':pred}
   if end[20]or end[21]:outside.append(row)
   else:
    row['exact_base_plus_refresh']=elapsed==pred['master_base']+14*end[19]
    row['bounded']=pred['master_lower']<=elapsed<=pred['master_upper'];checks.append(row)
 # Conditional local prediction: an already-known current virtual-clock origin
 # at the first row block plus an external timer window. Here the timer window
 # is bounded from the original IRQ acknowledgement solely as an oracle input;
 # it does not solve independent VDP/IRQ assertion time or prior callback costs.
 predictions=[]
 for irq in interrupts:
  ctx=next((call for call in calls if call['start']<irq['irq']<call['end']),None)
  if ctx is None:continue
  pc_saved=irq['irq_event'][20]&0xffffff
  target=next((p['name']for p in MODEL if p['start']<=pc_saved<p['end']),None)
  if target is None:continue
  start=next((e for _,e in ctx['blocks']if e[1]&0xffffff==MODEL[0]['start']),None)
  if start is None:continue
  # Stop if a previous IRQ interrupted before the row origin; this prototype
  # cannot infer that earlier wait from a purely arithmetic arena budget.
  previous=[entry for index,entry in ctx['irq']if index<irq['irq']]
  if previous:continue
  accept_lo=(irq['irq_event'][5]-start[5])*896040+irq['irq_event'][6]-59*7
  accept_hi=(irq['irq_event'][5]-start[5])*896040+irq['irq_event'][6]-50*7
  cursor_lo=cursor_hi=start[6];candidates=[];guaranteed=[]
  for phase in MODEL:
   pred=budget(phase);end_lo=cursor_lo+pred['master_lower'];end_hi=cursor_hi+pred['master_upper']
   if accept_hi>=cursor_lo and accept_lo<=end_hi:candidates.append(phase['name'])
   if cursor_hi<=accept_lo and accept_hi<=end_lo:guaranteed.append(phase['name'])
   cursor_lo,cursor_hi=end_lo,end_hi
  predictions.append({'frame':irq['irq_event'][5],'interrupted_pc':pc_saved,'actual_semantic_phase':target,'candidate_phases':candidates,'guaranteed_phases':guaranteed,'actual_in_candidates':target in candidates,'known_timer_window_source':'OriginalIRQentry minus50..59CPU acknowledgement cycles; a conditional oracle input, not an independent deadline predictor'})
 mismatches=[x for x in checks if not x['exact_base_plus_refresh']or not x['bounded']]
 pred_bad=[x for x in predictions if not x['actual_in_candidates']]
 result={'pass':not mismatches and not pred_bad,'scope':'Frozen local row-loop model on natural intervals; IRQ-containing intervals retained separately, no global scheduling claim','arena_callbacks':len(calls),'ordinary_phase_checks':len(checks),'checks':checks,'irq_or_bus_intervals':outside,'mismatches':mismatches,'conditional_predictions':predictions,'conditional_prediction_mismatches':pred_bad,'limits':['The conditional timer window is an original-oracle input; it is not a way to infer a future IRQ in production','Frame origin, gates/camera/DIVU, DMA/FIFO, previous callbacks and sound-driver timing remain unmodeled','All audio callback cases remain in separate complete watched-write causality audit; none are discarded by this local timing classification']}
 out=args.out.resolve();assert out.is_relative_to(ROOT);out.write_text(json.dumps(result,indent=2)+'\n')
 print(json.dumps({'pass':result['pass'],'callbacks':len(calls),'ordinary_checks':len(checks),'external_intervals':len(outside),'mismatches':len(mismatches),'predictions':len(predictions),'prediction_bad':len(pred_bad),'examples':mismatches[:2],'pred_examples':predictions[:4]}))
 if not result['pass']:raise SystemExit(1)
if __name__=='__main__':main()
