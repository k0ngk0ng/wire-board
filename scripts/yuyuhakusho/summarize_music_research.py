"""Bind completed music-structure and native-carrier research; no new core run."""
from pathlib import Path
from fractions import Fraction
import datetime,gzip,hashlib,json,subprocess
from reference import ROOT,sha

STUDY=ROOT/'.local/md-yuyuhakusho/audio-music-survey'
CARRIER=ROOT/'.local/md-yuyuhakusho/audio-music-carrier'
DATA=ROOT/'games/yuyuhakusho/data'
EVIDENCE=ROOT/'games/yuyuhakusho/verification/music-study'
CODES=[0x83,0x84,0x85,0x86,0x88,0x8a,0x8d,0x95]


def bind(path):return {'path':str(path.relative_to(ROOT)),'sha256':sha(path.read_bytes()),'bytes':path.stat().st_size}

def read(path):return json.loads(path.read_text())

def write(path,value):path.write_text(json.dumps(value,ensure_ascii=False,indent=2)+'\n')

def copy_evidence(path,name):
    target=EVIDENCE/name;target.write_bytes(path.read_bytes());return bind(target)


def main():
    EVIDENCE.mkdir(parents=True,exist_ok=True)
    first=read(STUDY/'test-first.json')
    for path,expected in first['files'].items():
        if path.endswith('AUDIO_MUSIC_PLAN.md'):
            assert sha((STUDY/'AUDIO_MUSIC_PLAN-initial.md').read_bytes())==expected
        else:assert sha((ROOT/path).read_bytes())==expected
    frozen=[]
    for name in ['structure-test-first.json','clock-test-first.json']:
        record=read(STUDY/name);assert sha((ROOT/record['test']).read_bytes())==record['sha256'];frozen.append(copy_evidence(STUDY/name,name))
    carrier_first=read(CARRIER/'test-first.json')
    assert all(sha((ROOT/path).read_bytes())==expected for path,expected in carrier_first['files'].items())
    probe=read(STUDY/'probe/survey.json');probe_check=read(STUDY/'probe-validation.json')
    structure=read(STUDY/'structure-validation.json');extended=read(STUDY/'extended-validation.json')
    raw8d=read(STUDY/'raw-proof-8d/survey.json')
    assert probe['pass']and probe['codes']==CODES and probe_check['pass']and probe_check['original_steps_compared']==32768
    assert structure['pass']and structure['tracks']==72 and structure['original_sequence_events_compared']==7498
    assert extended['pass']and extended['original_steps_compared']==88377 and extended['sequence_events_compared']==21147
    for record in [structure,extended]:
        for path,expected in record['source_sha256'].items():assert sha((ROOT/path).read_bytes())==expected
    phases=[('first-probe',STUDY/'probe/survey.json')]
    songs=[];captured_steps=32768
    score_map={score['code']:score for score in structure['songs']}
    for item in extended['songs']:
        code=item['code'];capture=read(ROOT/item['capture']);song=capture['songs'][0]
        assert capture['pass']and capture['codes']==[code]
        assert song['sampling']['original_ram_video_driver_pcm_equal']and not song['exact_pcm_loop_verified']
        for binding in song['files'].values():assert sha((ROOT/binding['path']).read_bytes())==binding['sha256']
        captured_steps+=song['sampling']['steps'];phases.append((f'per-track-{code:02x}',ROOT/item['capture']))
        selected=raw8d['songs'][0]if code==0x8d else song
        if code==0x8d:
            assert selected['sampling']['steps']==9335 and selected['raw_driver_recurrence']['verified']
            captured_steps+=9335
        native=selected['raw_driver_recurrence']
        if native['verified']:
            assert not selected['pcm_recurrence']['comparison']['pcm_bytes_equal']
        score=score_map[code]
        songs.append({'code':code,'header':song['header'],'classification':'looping-native-sequence',
                      'actual_tracks_with_repeated_f6':item['tracks_with_repeated_f6'],
                      'rhythm_period_quanta':[track['period_quanta']for track in score['tracks']],
                      'common_rhythm_quanta':score['common_rhythm_quanta'],
                      'structure_joint_rhythm_tempo_frames':score['common_rhythm_tempo_frames'],
                      'tempo_changes':score['tempo_changes'],'raw_driver_recurrence':native,
                      'selected_event_recurrence':selected['selected_event_recurrence'],
                      'pcm_recurrence':selected['pcm_recurrence'],'exact_pcm_loop_verified':False,
                      'bounded_capture':bind(ROOT/item['capture']),
                      'limits':['Looping control flow proven; no accepted finite media loop',
                                'Structure-derived joint periods are not whole native recurrence claims where the captured window is shorter']})
    phases.append(('8d-observed-raw-boundary',STUDY/'raw-proof-8d/survey.json'))
    assert captured_steps==130480
    for label,path in phases:
        report=read(path)
        assert report['pass']and not report['published']and not report['release_ready']
        for song in report['songs']:
            for binding in song['files'].values():assert sha((ROOT/binding['path']).read_bytes())==binding['sha256']
    carrier=read(CARRIER/'e2e/result.json');observed=read(CARRIER/'e2e/observed.json');manifest=read(CARRIER/'fixture/manifest.json');capabilities=read(CARRIER/'capabilities.json');negatives=read(CARRIER/'negatives/validation.json')
    assert carrier['pass']and carrier['fragment_count']==12 and carrier['observed_boundaries']==11 and carrier['native_waiting_after_play']==0 and carrier['source_changes']==1
    assert manifest['roundtrip_pcm_equal']and manifest['fragment_reconstruction_pcm_equal']and manifest['stereo_frames']==529200
    assert sha((CARRIER/'fixture/roundtrip.pcm').read_bytes())==manifest['source_excerpt_pcm_sha256']
    assert sha((CARRIER/'fixture/manifest.json').read_bytes())==carrier['manifest_sha256']
    assert sha((CARRIER/'e2e/observed.json').read_bytes())==carrier['observed_sha256']
    assert sha((ROOT/'scripts/yuyuhakusho/check_music_native_carrier.py').read_bytes())==carrier['harness_sha256']
    for fragment in manifest['fragments']:assert sha((CARRIER/'fixture'/fragment['file']).read_bytes())==fragment['sha256']
    assert negatives['pass']and len(negatives['checks'])==4 and all(item['rejected']and item['exit_code']!=0 for item in negatives['checks'])
    for item in negatives['checks']:assert sha((ROOT/item['log']).read_bytes())==item['sha256']
    # Preserve small normative evidence in the repository; large wave/context
    # files stay private. These copies are exact, not rewritten expectations.
    small={
        'music-missing-red.txt':STUDY/'missing-red.txt',
        'music-structure-symbol-red.txt':STUDY/'structure-symbol-red.txt',
        'music-structure-tempo-red.txt':STUDY/'structure-tempo-red.txt',
        'music-clock-symbol-red.txt':STUDY/'clock-symbol-red.txt',
        'music-probe-validation.json':STUDY/'probe-validation.json',
        'music-structure-validation.json':STUDY/'structure-validation.json',
        'music-extended-validation.json':STUDY/'extended-validation.json',
        'music-plan-initial.md':STUDY/'AUDIO_MUSIC_PLAN-initial.md',
        'music-test-first.json':STUDY/'test-first.json',
        'music-producer-frozen.json':STUDY/'producer-frozen.json',
        'music-extended-producer-frozen.json':STUDY/'extended-producer-frozen.json',
        'music-8d-raw-boundary-summary.json':STUDY/'raw-proof-8d/survey.json',
        'music-8d-raw-boundary-capture.py.txt':STUDY/'capture-8d-raw-proof.py',
        'native-carrier-capabilities.json':CARRIER/'capabilities.json',
        'native-carrier-test-first.json':CARRIER/'test-first.json',
        'native-carrier-missing-red.txt':CARRIER/'missing-red.txt',
        'native-carrier-result.json':CARRIER/'e2e/result.json',
        'native-carrier-manifest.json':CARRIER/'fixture/manifest.json',
        'native-carrier-negatives.json':CARRIER/'negatives/validation.json',
    }
    evidence=[copy_evidence(path,name)for name,path in small.items()]+frozen
    for item in negatives['checks']:
        evidence.append(copy_evidence(ROOT/item['log'],'native-carrier-negative-'+item['mode']+'.txt'))
    raw=(json.dumps({'schema':1,'scope':'Research rhythmic control-flow model; not production notes/DSP data','runtime_asset_ready':False,'songs':structure['songs']},separators=(',',':'))+'\n').encode()
    score_path=DATA/'audio-music-score-structure.json.gz';score_path.write_bytes(gzip.compress(raw,mtime=0))
    observations_path=DATA/'audio-native-carrier-observed.json.gz';observations_path.write_bytes(gzip.compress((CARRIER/'e2e/observed.json').read_bytes(),mtime=0))
    source_names=['survey_music_sequences.py','validate_music_survey.py','music_score_structure.py','check_music_score_structure.py','capture_music_tracks.py','check_music_survey_full.py','probe_music_native_carrier.py','check_music_native_carrier.py','build_music_native_carrier.py','verify_music_carrier_negatives.py','summarize_music_research.py']
    sources={str(path.relative_to(ROOT)):sha(path.read_bytes())for path in [ROOT/'scripts/yuyuhakusho'/name for name in source_names]}
    sources.update({str(path.relative_to(ROOT)):sha(path.read_bytes())for path in [ROOT/'docs/research/md-yuyuhakusho/AUDIO_MUSIC_FAILURES.md',ROOT/'docs/research/md-yuyuhakusho/AUDIO_MUSIC_PLAN.md',ROOT/'docs/research/md-yuyuhakusho/AUDIO_MUSIC_NATIVE_CARRIER_FAILURES.md',ROOT/'docs/research/md-yuyuhakusho/AUDIO_MUSIC_RESULTS.md']})
    alignment=[]
    for period in (3840,11502):
        samples=Fraction(period*3420*262*44100,53693175);loops=samples.denominator;whole=samples*loops
        alignment.append({'sequence_period_video_frames':period,'stereo_samples_per_period':str(samples),'minimum_sequence_loops_same_video_and_sample_phase':loops,'seconds':str(whole/44100),'raw_stereo_s16_bytes':int(whole)*4,
                          'limit':'Alignment arithmetic only; not a proof excluding shorter waveform coincidence or additional chip-state factors'})
    summary={'schema':1,'pass':True,'scope':'Eight additional native music structures and bounded lossless native-carrier feasibility',
             'goal_complete':False,'release_ready':False,'published':False,'exact_pcm_loop_assets_ready':False,'independent_dsp_implemented':False,'platform_stream_integrated':False,
             'baseline_commit':'aa617e23','rom_sha256':probe['rom_sha256'],'new_music_codes':CODES,
             'coverage':{'tracks':72,'first_probe_steps':32768,'per_track_extended_steps':88377,'extra_8d_steps':9335,'original_hook_music_steps_compared':captured_steps,'first_probe_sequence_commands':7498,'extended_sequence_commands':21147,'all72_tracks_have_two_actual_f6_returns':True,'whole_raw_driver_periods_verified':[0x83,0x85,0x8d,0x95]},
             'songs':songs,'phases':[dict(label=label,**bind(path))for label,path in phases],
             'sampler_alignment_constraints':alignment,'score_structure':bind(score_path),'cleanup':bind(DATA/'audio-music-study-cleanup.json'),
             'native_carrier_index':'games/yuyuhakusho/data/audio-native-carrier-validation.json',
             'test_first':{'acceptance_sources_unchanged':True,'initial_missing_evidence_red_is_not_a_production_code_red':True,'plan_updated_after_probe_with_exact_initial_plan_preserved':True,'evidence':evidence},
             'source_sha256':sources,
             'limits':['Isolated music private input only; natural request dispatch and mixed SFX timing are separate owners',
                       'Long joint periods84/86/88/8a remain structure candidates, not captured whole-driver or PCM recurrence',
                       'Research model validates rhythm/command timing; complete pitched-note/instrument automation export and independent synthesis remain unimplemented',
                       'No fixed WAV loop is accepted; full native chip/PCM state can continue across sequence returns',
                       'Native carrier is finite12s Chromium evidence, not independentDSP or infinite music playback',
                       'No game/engine/platform production file, existing media or old producer modified']}
    carrier_summary={'schema':1,'pass':True,'scope':carrier['scope'],'goal_complete':False,'release_ready':False,'published':False,
                     'capabilities':capabilities,'actual_playback':carrier,'container':manifest,'observations':bind(observations_path),
                     'capture_file':bind(CARRIER/'e2e/native-capture.webm'),'capture_codec':'Opus48kHz; temporal evidence only',
                     'negative_validation':negatives,'test_first':carrier_first,'source_sha256':sources,'cleanup':bind(DATA/'audio-music-study-cleanup.json'),
                     'limits':carrier['limits']+['Prototype buffers90112 known samples before playback; dynamic SFX latency and buffer revision remain unmeasured','FutureGo/Wasm runtime needs its own lossless encoder/muxer; WebCodecsFLAC encoding is unsupported in this browser']}
    write(DATA/'audio-music-survey-validation.json',summary);write(DATA/'audio-native-carrier-validation.json',carrier_summary)
    print(json.dumps({'pass':True,'music_steps_compared':captured_steps,'tracks':72,'extended_sequence_commands':21147,'native_carrier_boundaries':11,'exact_pcm_loop_assets_ready':False},indent=2))

if __name__=='__main__':main()
