"""Summarize observed IRQ/request/world ordering without inventing a scheduler."""
import collections
import hashlib
import json
import sys
from pathlib import Path
ROOT=Path(__file__).resolve().parents[2]


def main():
    path=Path(sys.argv[1]).resolve();assert path.is_relative_to(ROOT)
    rows=[json.loads(line) for line in path.open()]
    pending_request=None;pending_irq=None;pending_ball=None;phase='outside_world'
    interrupted=[];nested=[];ball=[];irqs=[];request_durations=[];phase_counts=collections.Counter()
    ready_returns=[];world_gaps=[];last_world_ticks=0;complete_ticks=0
    for index,row in enumerate(rows):
        kind=row['kind']
        if kind=='request_before':pending_request=row
        elif kind=='request_after':
            assert pending_request and pending_request['id']==row['id']
            request_durations.append(row['cycle']-pending_request['cycle']);pending_request=None
        elif kind=='irq_before':
            assert pending_irq is None;pending_irq=row
            phase_counts[phase]+=1
            if pending_request:interrupted.append({'request':pending_request,'irq':row})
        elif kind=='irq_after':
            assert pending_irq
            source=bytes.fromhex(row['driver']);ready=bool(source[0xe5]&128)
            entry={'before':pending_irq['order'],'after':row['order'],'frame_before':pending_irq['frame'],'frame_after':row['frame'],
                   'cycles':row['cycle']-pending_irq['cycle'],'work_before':pending_irq['work'],'work_after':row['work'],'ready':ready,'in_request':pending_request is not None}
            irqs.append(entry)
            if ready:complete_ticks+=1
            else:ready_returns.append(entry)
            pending_irq=None
        elif kind=='nmi' and pending_irq:nested.append({'irq_order':pending_irq['order'],'nmi_order':row['order'],'frame':row['frame'],'work_at_nmi':row['work']})
        elif kind in ['world_start','inputs_start','actions_start','actions_end','court_tail']:
            if kind=='world_start':
                world_gaps.append(complete_ticks-last_world_ticks);last_world_ticks=complete_ticks
            phase=kind
        elif kind=='ball14_enter':pending_ball=index
        elif kind=='ball14_return' and pending_ball is not None:
            sequence=rows[pending_ball:index+1]
            selected=[r for r in sequence if r['kind'] in ['request_before','request_after','irq_before','irq_after','ball14_enter','ball14_motion_before','ball14_motion_after','turn_compare','ball_action_write','ball14_return']]
            for value in selected:value.pop('driver',None)
            ball.append({'frame':row['frame'],'events':selected});pending_ball=None
    report={'scope':'observed event order and source CPU-cycle spans; not a universal timing model',
        'trace':str(path.relative_to(ROOT)),'sha256':hashlib.sha256(path.read_bytes()).hexdigest(),
        'counts':dict(collections.Counter(row['kind'] for row in rows)),
        'irq_by_recent_world_marker':dict(phase_counts),'complete_irq_count':complete_ticks,
        'not_ready_irq_returns':ready_returns,'irq_inside_request':interrupted,'nmi_inside_irq':nested,
        'irq_cycle_histogram':dict(collections.Counter(r['cycles'] for r in irqs)),
        'request_cycle_range':[min(request_durations),max(request_durations)],
        'complete_irqs_between_world_starts':dict(collections.Counter(world_gaps[1:])),
        'ball14_calls':ball,
        'limitations':['Recent marker is not a proof of function-stack ownership outside bounded call windows.',
            'Initial team/court selection is configured; later game state is not injected.',
            'A finite negative search does not prove Ball14-to-4 is unreachable.']}
    destination=ROOT/'.local/basketball/audio/world-order'/(path.parent.name+'-ordering.json')
    destination.write_text(json.dumps(report,indent=2)+'\n')
    print(json.dumps({'path':str(destination.relative_to(ROOT)),'complete_irqs':complete_ticks,'not_ready_irqs':len(ready_returns),'irq_inside_requests':len(interrupted),'nmi_inside_irqs':len(nested),'ball14_calls':len(ball),'world_gap_histogram':report['complete_irqs_between_world_starts']}))


if __name__=='__main__':main()
