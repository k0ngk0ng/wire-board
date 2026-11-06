"""Compare fixed semantic barriers against frozen complete sound-write traces.

This is a conditional causality investigation. Original IRQ frames and driver
internal groups remain oracle inputs. No sampled schedule enters production.
"""
from pathlib import Path
import hashlib, importlib.util

ROOT=Path(__file__).resolve().parents[2]

def _candidate(base,ticks,writes,events,priorities,initial,policy,start_frame):
    first={};anchor={}
    for i,e in enumerate(events):
        first.setdefault(e[5],i)
        if policy=='before-arena' and e[0]==10 and e[9]==0x20918:anchor.setdefault(e[5],i-.25)
        if policy=='after-manager' and e[0]==11 and e[8]==0x8040:anchor.setdefault(e[5],i+.25)
    operations=[]
    fallback=0
    for write in writes:
        if write['tick'] is None:operations.append((write['event'],write['event'],'world',write))
    for tick in ticks:
        frame=events[tick['start']][5]
        target=tick['start']
        if frame>=start_frame:
            target=first[frame]-.25
            if policy!='frame-head':
                if frame in anchor:target=anchor[frame]
                else:fallback+=1
        operations.append((target,tick['start'],'tick',tick))
    current=list(initial);same=0
    report={'policy':policy,'start_frame':start_frame,'fallback_to_frame_head':fallback,'independent_schedule_proved':False,
            'scope':'Given original driver groups and IRQ-frame labels; stop at the first queue/pause mismatch; no later PCM or arbitrary-input claim'}
    for _,_,kind,item in sorted(operations,key=lambda v:(v[0],v[1])):
        if kind=='world':
            signature=item['signature']
            if item['category']=='world-or-delayed-request':
                code=signature[3]
                slot=next((s for s,q in enumerate(current[3:])if q<128 or q==code),None)
                if slot is not None:current[3+slot]=code
            else:base.store(current,*signature[1:])
            continue
        tick=item
        if current[1]!=tick['entry_owned'][1]:
            return report|{'rejected':True,'reason':'pause order changed','frame':events[tick['start']][5],
                           'unchanged_dispatch_checks':same,'native_pause':tick['entry_owned'][1],'candidate_pause':current[1]}
        if not tick['selected']:
            for write in tick['writes']:base.store(current,*write['signature'][1:])
            continue
        split=tick.get('consumer_entry',tick['dispatch_point'])
        for write in tick['writes']:
            if write['event']<split and write['category']!='priority-consumer':base.store(current,*write['signature'][1:])
        queue=list(current[3:]);priority=current[0]
        if any(current[3:]):
            for signature in base.consume_writes(current,priorities):base.store(current,*signature[1:])
        actual=tick['selected'][0]
        if current[2]!=actual:
            return report|{'rejected':True,'reason':'selected command changed','frame':events[tick['start']][5],
                           'unchanged_dispatch_checks':same,'native_selected':actual,'candidate_selected':current[2],
                           'queue_before_consume':queue,'priority_before':priority}
        same+=1
        for write in tick['writes']:
            if write['event']>=split and write['category']!='priority-consumer':base.store(current,*write['signature'][1:])
    return report|{'rejected':False,'reason':'No counterexample in this finite original trace','unchanged_dispatch_checks':same}

def investigate():
    source=ROOT/'scripts/yuyuhakusho/analyze_audio_phase_budget.py'
    spec=importlib.util.spec_from_file_location('frozen_phase',source);base=importlib.util.module_from_spec(spec);spec.loader.exec_module(base)
    original=base.frame_tail_counterexample;captured=[]
    def retain(*args,**kwargs):
        if not captured:captured.append(args[:5])
        return original(*args,**kwargs)
    base.frame_tail_counterexample=retain
    scenarios=[]
    for kind in ['baseline','pause']:
        captured.clear()
        path=ROOT/'docs/research/md-yuyuhakusho/audio-phase-budget/compact'/f'audio-phase-{kind}.json.gz'
        control=base.analyze(path,kind)
        ticks,writes,events,priorities,initial=captured[0]
        scenarios.append({'name':kind,'source_sha256':hashlib.sha256(path.read_bytes()).hexdigest(),
                          'original_control':{key:control[key]for key in ['frame_tail_counterexample','battle_only_frame_tail_counterexample']},
                          'candidates':[_candidate(base,ticks,writes,events,priorities,initial,policy,start)
                                        for policy in ['frame-head','before-arena','after-manager']for start in [1,1800]]})
    return {'scope':'Read-only bounded fixed-barrier causality; independent world/IRQ schedule remains unresolved',
            'production_modified':False,'original_analyzer_sha256':hashlib.sha256(source.read_bytes()).hexdigest(),'scenarios':scenarios}
