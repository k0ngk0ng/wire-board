"""Per-track extended original music E2E, frozen before clock/capture extension."""
from pathlib import Path
import copy, gzip, hashlib, json
from music_score_structure import derive_song, project_song

ROOT=Path(__file__).resolve().parents[2]
STUDY=ROOT/'.local/md-yuyuhakusho/audio-music-survey'
CODES=[0x83,0x84,0x85,0x86,0x88,0x8a,0x8d,0x95]

def read_states(path):
    with gzip.open(path,'rt')as stream:return [json.loads(line)for line in stream]

def main():
    rom=(ROOT/'.local/md-yuyuhakusho/downloads/yuyuhakusho-jp.bin').read_bytes()
    structures={code:derive_song(rom,code)for code in CODES}
    plan=json.loads((STUDY/'extended-plan.json').read_text())
    results=[];total=0;events_total=0
    for code in CODES:
        path=STUDY/f'extended-{code:02x}'/'survey.json'
        report=json.loads(path.read_text());assert report['pass']and report['codes']==[code]and not report['release_ready']
        song=report['songs'][0];score=structures[code]
        bound=plan['per_code_bounds'][str(code)]
        assert bound==score['per_track_two_cycle_native_bound']and song['sampling']['steps']==song['sampling']['limit']==bound
        assert song['sampling']['original_ram_video_driver_pcm_equal']and song['sampling']['world_clear_every_step']and song['sampling']['effect_rows_clear_every_step']
        states=read_states(ROOT/song['files']['states']['path']);assert len(states)==bound
        prior=read_states(STUDY/'probe'/f'{code:02x}-states.jsonl.gz')
        for old,new in zip(prior,states):
            for key in ('step','reference','driver_hex','stereo_frames','pcm_sha256'):
                assert old[key]==new[key],(code,key,old['step'])
        observed=json.loads(gzip.decompress((ROOT/song['files']['events']['path']).read_bytes()))
        assert observed['observer_equal']and not observed['errors']
        expected=[{key:event[key]for key in ('step','row','source','command')}
                  for event in observed['events']if event['kind']=='sequence']
        actual=project_song(score,bound)
        assert actual==expected,(hex(code),next(((a,b)for a,b in zip(actual,expected)if a!=b),None),len(actual),len(expected))
        repeated={branch['row']for branch in song['track_f6_observations']if len(branch['steps'])>=2}
        assert {track['row']for track in score['tracks']}<=repeated,(hex(code),'some tracks lack two actualF6 returns')
        assert not song['exact_pcm_loop_verified']
        total+=bound;events_total+=len(expected)
        results.append({'code':code,'pass':True,'steps':bound,'sequence_events_compared':len(expected),
                        'tracks_with_repeated_f6':len(repeated),'common_rhythm_quanta':score['common_rhythm_quanta'],
                        'common_rhythm_tempo_frames':score['common_rhythm_tempo_frames'],'has_mid_song_tempo_changes':bool(score['tempo_changes']),
                        'raw_driver_recurrence':song['raw_driver_recurrence'],'selected_event_recurrence':song['selected_event_recurrence'],
                        'pcm_recurrence':song['pcm_recurrence'],'capture':str(path.relative_to(ROOT)),
                        'capture_sha256':hashlib.sha256(path.read_bytes()).hexdigest()})
        if code==0x95:
            changed=copy.deepcopy(score)
            for track in changed['tracks']:
                for group in ('prefix_commands','cycle_commands'):
                    for event in track[group]:
                        if event['command']==0xea:event['tempo_value']+=1
            assert project_song(changed,bound)!=expected,'global tempo mutation was not detected'
        changed=copy.deepcopy(score);changed['tempo_byte']+=1
        assert project_song(changed,bound)!=expected,('header tempo mutation not detected',code)
        for binding in song['files'].values():
            assert hashlib.sha256((ROOT/binding['path']).read_bytes()).hexdigest()==binding['sha256']
    result={'pass':True,'scope':'Eight songs, all72 track returns and shared tempo clock against extended original events',
            'songs':results,'original_steps_compared':total,'sequence_events_compared':events_total,
            'source_sha256':{str(path.relative_to(ROOT)):hashlib.sha256(path.read_bytes()).hexdigest()for path in [Path(__file__).resolve(),ROOT/'scripts/yuyuhakusho/music_score_structure.py']},
            'limits':['Per-track two-cycle bounds; long all-track/common-tempo recurrence is not falsely claimed where not captured',
                      'No synth, exactPCMloop or browser playback acceptance is inferred from rhythmic control validation']}
    (STUDY/'extended-validation.json').write_text(json.dumps(result,indent=2)+'\n')
    print(json.dumps({key:value for key,value in result.items()if key!='songs'},indent=2))

if __name__=='__main__':main()
