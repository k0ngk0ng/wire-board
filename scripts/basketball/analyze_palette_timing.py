"""Validate read-only original palette traces and retain reusable commit cases."""
from pathlib import Path
from collections import Counter
import json,hashlib
ROOT=Path(__file__).resolve().parents[2];out=ROOT/'.local/basketball/palette-timing';capture=json.loads((out/'capture.json').read_text());samples=[];summaries=[];special={}
for record in capture['records']:
 run=ROOT/'.local/nes-basket/runs'/record['observed'];log=run/'palette.jsonl';events=[json.loads(line) for line in log.read_text().splitlines()];pending=None;before=None;committed=None;normal=[];all_commits=[]
 for event in events:
  if event['kind']=='queue':pending=event
  elif event['kind']=='commit-before':
   assert pending and pending['queuedID']==event['queuedID'];before=event
  elif event['kind']=='commit-after':
   assert pending and before
   assert event['cycles']>before['cycles']>=pending['cycles']
   assert event['queueCount'] in [16,32]
   assert [c&63 for c in event['ppu'][:event['queueCount']]]==[c&63 for c in before['buffer'][:event['queueCount']]],(record['observed'],event['frame'])
   item={'source_run':record['observed'],'queue_frame':pending['frame'],'commit_frame':event['frame'],'queue_cycle':pending['cycles'],'commit_cycle':event['cycles'],'queue_nmi':pending['nmi'],'commit_nmi':event['nmi'],'previous_committed_id':committed,'queued_id':pending['queuedID'],'phase':pending['phase'],'scene':pending['scene'],'flags':pending['flags'],'delay':pending['delay'],'ppu_before':before['ppu'],'ppu_after':event['ppu'],'queued_colors':before['buffer']}
   if pending['scene']==1 and pending['phase']&127==1 and pending['flags']==0 and pending['delay']==0:
    assert before['ppu']==pending['ppu'],'normal palette changed before NMI commit'
    assert event['nmi']==pending['nmi']+1,'normal palette did not commit at next NMI'
    assert item['queued_id'] in [5,9,10];normal.append(item)
   committed=event['queuedID'];all_commits.append(item);pending=None;before=None
 assert pending is None and before is None
 if 'pause' in record['observed']:
  phases=[e for e in events if e['kind']=='phase' and e['scene']==1];start=next(e['frame'] for e in phases if e['value']==2);end=next(e['frame'] for e in phases if e['frame']>start and e['value']==129)
  assert not any(e['kind'] in ['build','queue','commit-before','commit-after'] and start<e['frame']<end for e in events)
  last=next(e for e in reversed(events) if e['kind']=='commit-after' and e['frame']<=start);resume=next(e for e in events if e['kind']=='queue' and e['frame']>=end)
  assert last['queuedID']==9 and resume['queuedID']==10
  assert (run/'frame-01455-palette.bin').read_bytes()==(run/'frame-01475-palette.bin').read_bytes()
  special['pause']={'start_frame':start,'resume_frame':end,'held_id':9,'next_queued_id':10,'queue_commit_events_while_paused':0,'identical_paused_palette_screenshots':True}
 if 'complete' in record['observed']:
  selected=[e for e in events if e['kind'] in ['opening-base','period-reset','result-entry','result-restored']];special['boundaries']=selected
  result_entry=next(e for e in events if e['kind']=='result-entry');result_restored=next(e for e in events if e['kind']=='result-restored');assert result_entry['selection']==result_restored['selection']==9
  builds=[e for e in events if e['kind']=='build' and e['phase']==134 and e['frame']>=result_restored['frame']];assert builds[0]['selection']==9
  special['result']={'entry':result_entry,'restored':result_restored,'build':builds[0],'commits':[e for e in all_commits if e['scene']==1 and e['phase']==134 and e['queue_frame']>=result_restored['frame']]}
  special['intermission_selection_trace']=[{k:e[k] for k in ['kind','frame','pc','scene','phase','selection','value','flags','delay']} for e in events if 14362<=e['frame']<=14562 and e['kind'] in ['background','selection','build','opening-base']]
  special['fade_commits']=[e for e in all_commits if e['scene']==1 and e['phase']==4]
 # Keep one directly reusable sample for each normal ID transition per source.
 seen=set()
 for item in normal:
  key=item['previous_committed_id'],item['queued_id']
  if key not in seen:seen.add(key);samples.append(item)
 summaries.append({'run':record['observed'],'normal_commands':len(normal),'normal_transitions':{f'{a}->{b}':n for (a,b),n in Counter((e['previous_committed_id'],e['queued_id']) for e in normal).items()},'all_palette_commits':len(all_commits),'log_sha256':hashlib.sha256(log.read_bytes()).hexdigest(),'ram_and_png_equal':record['ram_and_png_equal']})
report={'status':'passed','scope':'original court4 configured normal-input palette selection/queue/NMI observations; not Go runtime tests','sources':summaries,'normal_queue_commit_samples':samples,**special};(out/'samples.json').write_text(json.dumps(report,indent=2)+'\n');print(json.dumps({'status':'passed','normal_commands':sum(e['normal_commands'] for e in summaries),'reusable_samples':len(samples),'sources':summaries,'pause':special['pause'],'result_selection':special['result']['restored']['selection']}))
