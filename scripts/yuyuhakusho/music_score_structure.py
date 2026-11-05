"""Research-only music control-flow/rhythm model, not a CPU or runtime player.

Uses static original command semantics to derive bounded sampling periods.
Full raw driver/synthesis recurrence remains an independent native obligation.
"""
import math
from survey_music_sequences import header_for

# E4 is variable; F6..F9 are branches. Unknown FF extensions stay explicit.
ARG_BYTES={0xe0:1,0xe1:1,0xe2:1,0xe3:0,0xe5:2,0xe6:1,0xe7:0,0xe8:1,
           0xe9:2,0xea:1,0xeb:1,0xec:1,0xed:2,0xee:2,0xef:1,
           0xf0:4,0xf1:2,0xf2:0,0xf3:1,0xf4:1,0xf5:1,
           0xfa:1,0xfb:1,0xfc:0,0xfd:0,0xfe:4}


def walk_track(rom, record, max_commands=200000, max_quanta=1000000):
    pc=record['sequence'];mult=record['duration_multiplier'];last_duration=0
    # +28..2f has one owner: two big-endian words. F7 accesses their bytes;
    # F8/F9 accesses their word-sized return slots. No duplicate loop/stack owner.
    words=[0,0];stack_position=0x30;quanta=0;events=[];seen={};notes=0
    def byte_at(index):
        return words[index//4]>>((3-index%4)*8)&255
    def set_byte(index,value):
        shift=(3-index%4)*8;owner=index//4
        words[owner]=(words[owner]&~(255<<shift))|((value&255)<<shift)
    def args(size):
        if pc+size>len(rom):raise ValueError('Truncated music command or note')
        return rom[pc:pc+size]
    for _ in range(max_commands):
        if not 0<=pc<len(rom):raise ValueError('Music sequence outside the supplied ROM')
        if quanta>max_quanta:raise ValueError('Music rhythm exceeds the declared structural search bound')
        key=(pc,mult,last_duration,tuple(words),stack_position)
        if key in seen:
            first,index=seen[key];period=quanta-first
            if period<=0:raise ValueError('Zero-duration music control-flow cycle')
            return {'row':record['row'],'sequence':record['sequence'],'cycle_start_quanta':first,
                    'period_quanta':period,'stopped':False,'prefix_commands':events[:index],
                    'cycle_commands':[{**event,'quanta':event['quanta']-first}for event in events[index:]],
                    'notes_scanned':notes,'commands_scanned':len(events),'shared_stack_counter_words':True}
        seen[key]=(quanta,len(events));source=pc;token=rom[pc];pc+=1
        if token<0xe0:
            if token>=0x80:
                if pc<len(rom)and rom[pc]<0x80:
                    raw=rom[pc];pc+=1;last_duration=(raw*mult)&255
            else:last_duration=(token*mult)&255
            quanta+=last_duration or 256;notes+=1
            continue
        event={'source':source,'command':token,'quanta':quanta}
        if token==0xea:
            if pc>=len(rom):raise ValueError('Truncated tempo command')
            event['tempo_value']=rom[pc]
        events.append(event)
        if token in (0xf6,0xf8):
            raw=args(2);target=source+2+int.from_bytes(raw,'big',signed=True)
            if token==0xf8:
                stack_position-=4
                if stack_position<0x28:raise ValueError('Music return stack exceeds the recovered shared words')
                words[(stack_position-0x28)//4]=source+1
            pc=target
        elif token==0xf7:
            raw=args(4);index,count=raw[:2]
            if index>=8:raise ValueError('Music loop counter outside the recovered shared words')
            left=byte_at(index)
            if left==0:left=count
            left=(left-1)&255;set_byte(index,left)
            pc=source+4+int.from_bytes(raw[2:4],'big',signed=True)if left else pc+4
        elif token==0xf9:
            if stack_position>=0x30:raise ValueError('Music return stack underflow')
            pc=words[(stack_position-0x28)//4]+2;stack_position+=4
        elif token==0xe4:
            first=args(1)[0];size=5 if first else 1;args(size);pc+=size
        elif token==0xff:
            raise ValueError('Extended FF music command is not recovered by this rhythm model')
        elif token in ARG_BYTES:
            size=ARG_BYTES[token];data=args(size);pc+=size
            if token==0xfa:mult=data[0]
            elif token in (0xe3,0xf2):
                return {'row':record['row'],'sequence':record['sequence'],'cycle_start_quanta':None,
                        'period_quanta':None,'stopped':True,'prefix_commands':events,'cycle_commands':[],
                        'notes_scanned':notes,'commands_scanned':len(events),'shared_stack_counter_words':True}
        else:raise ValueError('Unrecovered music command')
    raise ValueError('Music command search bound exhausted')


def frame_for_quantum(quanta,tempo):
    if tempo==1:raise ValueError('Tempo1 does not advance the recovered rhythmic clock')
    return 1+quanta+(quanta//(tempo-1)if tempo>1 else 0)


def project_commands(track,tempo,steps):
    result=[]
    if track.get('constant_tempo_before_quanta') is not None and frame_for_quantum(track['constant_tempo_before_quanta'],tempo)<steps:
        raise ValueError('Projection crosses a recovered global tempo change; use a shared clock')
    def append(events,offset):
        for event in events:
            at=frame_for_quantum(event['quanta']+offset,tempo)
            if at>=steps:continue
            result.append({'step':at,'row':track['row'],'source':event['source'],'command':event['command']})
    append(track['prefix_commands'],0)
    if not track['stopped']:
        start,period=track['cycle_start_quanta'],track['period_quanta'];repeat=0
        while frame_for_quantum(start+repeat*period,tempo)<steps:
            append(track['cycle_commands'],start+repeat*period);repeat+=1
    return result


def derive_song(rom,code):
    header=header_for(rom,code);tempo=header['tempo_byte'];tracks=[]
    for voice in header['voices']:
        record={**voice,'duration_multiplier':header['duration_multiplier']}
        track=walk_track(rom,record)
        if any(event['command']==0xeb for group in ('prefix_commands','cycle_commands')for event in track[group]):
            raise ValueError('Queue-producing commands need explicit cross-track recovery')
        tracks.append(track)
    tempo_changes=[]
    for track in tracks:
        for group in ('prefix_commands','cycle_commands'):
            for event in track[group]:
                if event['command']==0xea:
                    tempo_changes.append({'row':track['row'],'source':event['source'],'quanta':event['quanta']+(track['cycle_start_quanta']if group=='cycle_commands'else 0),'tempo':rom[event['source']+1]})
    if tempo_changes:
        first_change=min(event['quanta']for event in tempo_changes)
        for track in tracks:track['constant_tempo_before_quanta']=first_change
    periods=[track['period_quanta']for track in tracks if not track['stopped']]
    period=math.lcm(*periods)if periods else None
    frame_period=None if tempo_changes else period if tempo==0 else period*tempo//math.gcd(period,tempo-1)if tempo>1 else None
    initialization=max((track['cycle_start_quanta']or 0 for track in tracks),default=0)
    initialization_frames=frame_for_quantum(initialization,tempo)
    # Two complete rhythm/tempo cycles plus the observed typed prefix and an
    # explicit128-frame margin, not an arbitrary long-recording extension.
    bound=initialization_frames+2*frame_period+128 if frame_period else 2*(initialization+2*max(periods))+128
    track_quanta_bound=max((track['cycle_start_quanta']or 0)+2*(track['period_quanta']or 0)for track in tracks)
    individual_bound=frame_for_quantum(track_quanta_bound,tempo)+128 if not tempo_changes else 2*track_quanta_bound+128
    return {'code':code,'tempo_byte':tempo,'tracks':tracks,'common_rhythm_quanta':period,
            'common_rhythm_tempo_frames':frame_period,'initialization_frames':initialization_frames,
            'proposed_native_bound':bound,'per_track_two_cycle_native_bound':individual_bound,'tempo_changes':tempo_changes,
            'limits':['Rhythmic control-flow and tempo phase only; every raw driver byte and full PCM must still be validated',
                      'No exact chip/DSP recurrence is inferred from this score model']}


def project_song(score,steps):
    """Shared video/tempo clock; row command order matches the native DAC/FM/PSG scan."""
    def commands(track):
        for event in track['prefix_commands']:yield {**event,'row':track['row']}
        if not track['stopped']:
            repeat=0
            while True:
                start=track['cycle_start_quanta']+repeat*track['period_quanta']
                for event in track['cycle_commands']:
                    yield {**event,'row':track['row'],'quanta':event['quanta']+start}
                repeat+=1
    streams=[iter(commands(track))for track in score['tracks']]
    pending=[next(stream,None)for stream in streams]
    tempo=score['tempo_byte'];remaining=tempo;quanta=0;result=[]
    # Native dispatch initializes the rows on step0, and its stack return skips
    # their scan. The first actual scan is step1. A tempo expiry increments all
    # row waits before their decrement; this defers rhythmic advancement.
    for step in range(1,steps):
        if tempo:
            remaining=(remaining-1)&255
            if remaining==0:
                remaining=tempo
                continue
        for index,stream in enumerate(streams):
            while pending[index]is not None and pending[index]['quanta']==quanta:
                event=pending[index]
                result.append({'step':step,'row':event['row'],'source':event['source'],'command':event['command']})
                if event['command']==0xea:
                    tempo=event['tempo_value'];remaining=tempo
                pending[index]=next(stream,None)
            if pending[index]is not None and pending[index]['quanta']<quanta:
                raise ValueError('Unrecovered zero-duration or cross-row rhythmic dependency')
        quanta+=1
    return result
