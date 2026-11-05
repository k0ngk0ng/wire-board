"""Address-free typed score note clock; research proof, no ROM or CPU access."""


def event_stream(voice):
    timeline=voice['timeline']
    for event in timeline['prefix']:yield event
    if timeline['cycle_quanta']:
        count=0
        while True:
            offset=timeline['prefix_quanta']+count*timeline['cycle_quanta']
            for event in timeline['cycle']:yield {**event,'at':event['at']+offset}
            count+=1


def replay_notes(library,score,steps):
    voices=score['voices'];streams=[iter(event_stream(voice))for voice in voices]
    pending=[next(stream,None)for stream in streams]
    states=[{'transpose':voice['initial']['transpose']&255,'level':voice['initial']['level'],
             'base_pitch':None if voice['channel']=='dac'else 0,'last_duration':0,'active':True,'quanta':0}for voice in voices]
    psg_dividers=dict(enumerate(library['pitch_tables']['psg_dividers']))
    for row in library['pitch_tables']['psg_extended_dividers']:
        index=row['index']
        if type(index)is not int or not 70<=index<=127 or index in psg_dividers:raise ValueError('Invalid or duplicate extended PSG pitch index')
        if type(row['divider'])is not int or not 0<=row['divider']<=65535:raise ValueError('Invalid extended PSG divider')
        psg_dividers[index]=row['divider']
    tempo=score['initial_tempo'];countdown=tempo;out=[]
    for step in range(steps):
        if score['start_phase']=='next-driver-tick'and step==0:continue
        if score['kind']=='music'and tempo:
            countdown=(countdown-1)&255
            if countdown==0:
                countdown=tempo
                continue
        for index,(voice,stream,state)in enumerate(zip(voices,streams,states)):
            if not state['active']:continue
            ends_driver_tick=False
            while pending[index]is not None and pending[index]['at']==state['quanta']:
                event=pending[index];kind=event['kind']
                if kind=='transpose-add':state['transpose']=(state['transpose']+event['semitones'])&255
                elif kind=='level-add':state['level']=(state['level']+event['delta'])&255
                elif kind=='tempo':tempo=event['period'];countdown=tempo
                elif kind=='track-end':state['active']=False;ends_driver_tick=event['ends_driver_tick']
                elif kind=='note':
                    pitch=event['pitch'];channel=voice['channel'];pitch_kind='dac'if channel=='dac'else'psg'if channel.startswith('psg')else'fm'
                    if pitch['kind']=='rest':state['base_pitch']=None if pitch_kind=='dac'else 65535 if pitch_kind=='psg'else 0
                    elif pitch['kind']=='dac':state['base_pitch']=pitch['descriptor_id']
                    elif pitch['kind']=='chromatic':
                        if pitch_kind=='fm':
                            value=(pitch['semitone']+1+state['transpose'])&127
                            state['base_pitch']=library['pitch_tables']['fm_fnums'][value%12]|((value//12)&7)<<11
                        else:
                            value=(pitch['semitone']+state['transpose'])&127
                            if value not in psg_dividers:raise ValueError('PSG note reads beyond the typed pitch table')
                            state['base_pitch']=psg_dividers[value]
                    # Native FM-rest and every explicit PSG pitch enter the
                    # common note cleanup before their duration lookahead.
                    # Preserve that observable old-duration boundary, then the
                    # final cleanup; no expected state is injected here.
                    early=(pitch_kind=='fm'and pitch['kind']=='rest')or(pitch_kind=='psg'and pitch['kind']!='previous')
                    if early:
                        out.append({'step':step,'voice':voice['id'],'duration':state['last_duration']or 256,'pitch_kind':pitch_kind,'base_pitch':state['base_pitch'],'transpose':state['transpose'],'level':state['level']})
                    state['last_duration']=event['duration']&255
                    out.append({'step':step,'voice':voice['id'],'duration':state['last_duration']or 256,'pitch_kind':pitch_kind,'base_pitch':state['base_pitch'],'transpose':state['transpose'],'level':state['level']})
                pending[index]=next(stream,None)
            if pending[index]is not None and pending[index]['at']<state['quanta']:raise ValueError('Typed event stream moved backwards')
            if ends_driver_tick:break
            state['quanta']+=1
    return out
