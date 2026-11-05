"""Validate retained pure-Go stream encoder evidence; no implicit execution."""
from pathlib import Path
import argparse,gzip,hashlib,json

ROOT=Path(__file__).resolve().parents[2]
BASE=ROOT/'.local/md-yuyuhakusho/audio-encoder-plan'
DATA=ROOT/'games/yuyuhakusho/data'
EVIDENCE=ROOT/'games/yuyuhakusho/verification/audio-stream-encoder'
PACKAGE='github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/stream'

def sha(raw):return hashlib.sha256(raw).hexdigest()
def bind(path):return {'path':str(path.relative_to(ROOT)),'sha256':sha(path.read_bytes()),'bytes':path.stat().st_size}
def read(path):return json.loads(path.read_text())
def copy(path,name):
    target=EVIDENCE/name;target.write_bytes(path.read_bytes());return bind(target)

def go_tests(path):
    passed=[]
    for line in path.read_text().splitlines():
        event=json.loads(line)
        assert event['Action']not in('fail','build-fail'),path
        if event['Action']=='pass'and event.get('Test')and'/'not in event['Test']:passed.append(event['Test'])
    assert len(passed)==6
    return sorted(passed)

def validated_platform(path):
    result=read(path)
    assert result['pass'] and result['complete'] and result['error']==''
    fixture_path=ROOT/result['fixture_path'];fixture=read(fixture_path/'manifest.json')
    expected_fixture=BASE/'e2e/native/original12/small-bulk'
    assert fixture_path.resolve()==expected_fixture.resolve()
    assert sha((fixture_path/'manifest.json').read_bytes())==result['fixture_manifest_sha256']
    assert result['source_pcm_sha256']==fixture['source_pcm_sha256']=='7801d4c5f7ae515e7819c69637f3974ed2feedfc0362df2370ec28f5b8d55f4c'
    assert fixture['block_frames']==256 and fixture['fragment_frames']==1024
    assert result['actual_go_fragments']==len(fixture['fragments'])==517 and len(result['crossings'])==516
    prefixes={0:1};cursor=0
    for index,piece in enumerate(fixture['fragments']):
        assert piece['sequence']==index+1 and piece['start_sample']==cursor
        assert sha((fixture_path/piece['file']).read_bytes())==piece['sha256']
        assert piece['bytes']<=1<<20
        cursor+=piece['sample_frames'];prefixes[cursor]=index+2
    assert cursor==529200
    appended=[row['bytes'] for row in result['appends'][:len(fixture['fragments'])+1]]
    assert appended==[(fixture_path/fixture['init_file']).stat().st_size]+[piece['bytes']for piece in fixture['fragments']]
    prior_sample=0;prior_time=0
    for state in result['clocks']:
        assert state['acceptedEndSample']in prefixes and state['nextSeq']==prefixes[state['acceptedEndSample']]
        assert state['acceptedEndSample']>=prior_sample and state['currentTime']>=prior_time-0.000001
        assert state['maxForwardSamples']==22050 and state['pendingBytes']<=1<<20
        prior_sample,prior_time=state['acceptedEndSample'],state['currentTime']
    for index,crossing in enumerate(result['crossings']):
        piece=fixture['fragments'][index];edge=(piece['start_sample']+piece['sample_frames'])/44100
        assert crossing['index']==index and abs(crossing['boundary']-edge)<1e-10
        assert crossing['before']<edge<=crossing['after']
    required={'opening-replacement-and-old-epoch','copied-bytes-sequence-and-single-flight','native-stream-pause-resume','stop-owner-and-mute-cleanup','real-low-buffer-incremental-playback'}
    assert {case['kind']for case in result['checks']}==required
    main=next(case for case in result['checks']if case['kind']=='real-low-buffer-incremental-playback')
    final=main['final'];assert final['acceptedEndSample']==529200 and final['nextSeq']==518 and final['phase']=='ended' and abs(final['currentTime']-12)<0.1
    playing=result['firstPlaying'];identity=playing['id']
    events=[event for event in result['events']if event['id']==identity]
    assert len({event['src']for event in events if event['src']})==1
    assert len([event for event in events if event['kind']=='ended'])==1
    assert not [event for event in events if event['at']>playing['at'] and event['kind']in('waiting','stalled','seeking','error')]
    assert result['removals'] and all(row['end']<row['currentTime']for row in result['removals'])
    assert result['hidden_cleanup']['status']['phase']=='stopped' and all(row['paused']and row['src']is None for row in result['hidden_cleanup']['media'])
    assert all(row['paused']and row['src']is None and not row['connected']for row in result['final_media_state'])
    assert not result['sample_gapless_verified'] and not result['speaker_output_verified'] and not result['original_mix_equivalent']
    for name,expected in result['source_sha256'].items():assert sha((ROOT/name).read_bytes())==expected,name
    platform_index=path.parent.parent/'validation.json';overview=read(platform_index)
    assert overview['pass'] and overview['fixture']['manifest_sha256']==result['fixture_manifest_sha256']
    assert overview['native_checks']['groups']==5 and overview['native_checks']['actual_boundaries']==516
    assert overview['native_checks']['waiting_stalled_seeking_error_during_main']==0
    assert any(item['path']==str(path.relative_to(ROOT))and item['sha256']==sha(path.read_bytes())for item in overview['native_runs'])
    return {'report':bind(path),'platform_index':bind(platform_index),'scope':'Current platform-owned nativeChromium playback of517 pure-Go small fragments; bounded feed and lifecycle, not DSP/speaker/sample-gapless proof',
            'fixture_manifest_sha256':result['fixture_manifest_sha256'],'source_pcm_sha256':result['source_pcm_sha256'],
            'cases':5,'actual_go_fragments':517,'actual_sample_boundaries':516,'accepted_end_sample':529200,
            'first_playing_buffer_seconds':overview['native_checks']['first_playing_buffer_seconds'],
            'maximum_forward_seconds':overview['native_checks']['maximum_forward_seconds'],
            'safe_back_removes':len(result['removals']),'main_source_changes':1,'waiting_after_play':0,
            'source_sha256':result['source_sha256'],'sample_gapless_verified':False,'speaker_output_verified':False,'original_mix_equivalent':False}

def main(platform):
    EVIDENCE.mkdir(parents=True,exist_ok=True)
    frozen=read(BASE/'test-first.json')
    assert all(sha((ROOT/path).read_bytes())==expected for path,expected in frozen['files'].items())
    first_e2e=read(BASE/'e2e-test-first.json');assert sha((ROOT/first_e2e['path']).read_bytes())==first_e2e['sha256']
    tests={}
    for environment in('native','wasm'):
        path=BASE/('stream-'+environment+'.jsonl');tests[environment]=go_tests(path)
    assert tests['native']==tests['wasm']
    wasm_command=read(BASE/'wasm-command.json');assert wasm_command['exit_code']==0
    cases={}
    for environment in('native','wasm'):
        report=read(BASE/'e2e'/(environment+'-validation.json'))
        assert report['pass']and report['case_count']==12 and report['source_sha256']==first_e2e['sha256']
        if environment=='wasm':assert report['native_wasm_bytes_equal']
        for case in report['cases']:
            directory=ROOT/case['directory'];manifest=read(directory/'manifest.json')
            assert sha((directory/'manifest.json').read_bytes())==case['manifest_sha256']
            assert manifest['pass']and manifest['encoder_runtime_external_dependencies']==[]
            assert sha((directory/'init.mp4').read_bytes())==manifest['init_sha256']
            for fragment in manifest['fragments']:assert sha((directory/fragment['file']).read_bytes())==fragment['sha256']
            assert sha((directory/'decoded.pcm').read_bytes())==case['decoded_pcm_sha256']
        cases[environment]={'report':bind(BASE/'e2e'/(environment+'-validation.json')),'case_count':12,'all_pcm_exact':True}
    edge=read(BASE/'edges/validation.json');assert edge['pass']and len(edge['roundtrip_cases'])==4 and len(edge['negative_checks'])==4
    for case in edge['roundtrip_cases']:
        manifest=ROOT/case['manifest'];assert sha(manifest.read_bytes())==case['manifest_sha256']
    for case in edge['negative_checks']:
        assert case['rejected']
        if'log'in case:assert sha((ROOT/case['log']).read_bytes())==case['sha256']
    mse=read(BASE/'mse-go-e2e/result.json');fixture=read(BASE/'mse-go-fixture/manifest.json')
    assert mse['pass']and mse['observed_boundaries']==11 and mse['native_waiting_after_play']==0 and mse['source_changes']==1 and not mse['audio_context_created']
    assert fixture['pure_go_encoder']and fixture['logical_mp4_fragments']==130
    small_mse=read(BASE/'mse-small-e2e/result.json');small_fixture=read(BASE/'mse-small-fixture/manifest.json')
    assert small_mse['pass']and small_mse['observed_boundaries']==11 and small_mse['native_waiting_after_play']==0 and small_mse['source_changes']==1
    assert small_fixture['pure_go_encoder']and small_fixture['logical_mp4_fragments']==517
    assert sha((ROOT/small_fixture['source_manifest']).read_bytes())==small_fixture['source_manifest_sha256']
    assert sha((ROOT/fixture['source_manifest']).read_bytes())==fixture['source_manifest_sha256']
    assert sha((BASE/'mse-go-fixture/manifest.json').read_bytes())==mse['manifest_sha256']
    assert (BASE/'vet.txt').read_text()==''
    deps=[line for line in(BASE/'dependencies.txt').read_text().splitlines()if line.strip()]
    assert deps==[PACKAGE]
    artifacts=[]
    mapping={
        'test-first.json':BASE/'test-first.json','e2e-test-first.json':BASE/'e2e-test-first.json',
        'symbol-red.jsonl':BASE/'stream-symbol-red.jsonl','e2e-symbol-red.txt':BASE/'e2e-symbol-red.txt',
        'native.jsonl':BASE/'stream-native.jsonl','wasm.jsonl':BASE/'stream-wasm.jsonl','wasm-command.json':BASE/'wasm-command.json',
        'native-e2e.json':BASE/'e2e/native-validation.json','wasm-e2e.json':BASE/'e2e/wasm-validation.json',
        'edges.json':BASE/'edges/validation.json','ffmpeg-soft-error-red.txt':BASE/'edges-unstrict-ffmpeg-red.txt',
        'mse-go-result.json':BASE/'mse-go-e2e/result.json','mse-go-fixture.json':BASE/'mse-go-fixture/manifest.json',
        'mse-small-result.json':BASE/'mse-small-e2e/result.json','mse-small-fixture.json':BASE/'mse-small-fixture/manifest.json',
        'vet.txt':BASE/'vet.txt','dependencies.txt':BASE/'dependencies.txt'}
    for name,path in mapping.items():artifacts.append(copy(path,name))
    observations=EVIDENCE/'mse-go-observed.json.gz';observations.write_bytes(gzip.compress((BASE/'mse-go-e2e/observed.json').read_bytes(),mtime=0));artifacts.append(bind(observations))
    source_paths=list((ROOT/'games/yuyuhakusho/audio/stream').glob('*.go'))+[ROOT/'games/yuyuhakusho/audio/stream/FAILURES.md',ROOT/'games/yuyuhakusho/audio/stream/README.md',ROOT/'games/yuyuhakusho/audio/stream/VERIFICATION.md',ROOT/'games/yuyuhakusho/cmd/verify-audio-stream-encoder/main.go',ROOT/'scripts/yuyuhakusho/verify_go_audio_stream.py',ROOT/'scripts/yuyuhakusho/verify_go_audio_stream_edges.py',Path(__file__).resolve()]
    platform_binding=None
    if platform:
        platform_binding=validated_platform(platform)
    summary={'schema':1,'pass':True,'scope':'PureGo integer PCM-to-verbatimFLAC/fMP4 encoding, independentdecode and nativeMSE codec acceptance',
             'baseline_commit':'0f3bea2f','goal_complete':False,'release_ready':False,'independent_dsp_implemented':False,
             'runtime_external_dependencies':[],'runtime_imports':['encoding/binary','errors'],
             'test_first':{'frozen_sources_unchanged':True,'initial_missing_implementation_and_CLI_red_retained':True},
             'unit_tests':tests,'unit_tests_per_environment':6,'complete_roundtrip_cases':cases,
             'additional_edge_cases':4,'negative_checks':edge['negative_checks'],'native_wasm_output_bytes_identical':True,
             'default_format':{'sample_rate':44100,'channels':2,'bits_per_sample':16,'block_frames':1024,'fragment_frames':4096,'max_input_frames':65536},
             'latency_scope':{'default_fragment_span_ms':4096/44100*1000,'small_fragment_span_ms':1024/44100*1000,'actual_input_to_sound_latency_verified':False,'limit':'Encoding aggregation only; inputbatch/MSE/device/bufferlatency is not included'},
             'mse_codec_acceptance':{'report':bind(BASE/'mse-go-e2e/result.json'),'logical_mp4_fragments':130,'transport_append_chunks':12,'actual_chunk_boundaries':11,'waiting_after_play':0,'source_changes':1,'limit':'Frozen older researchharness groups11 unchangedGo fragments perHTTP append; individual93ms platformappend is a separate obligation'},
             'small_mse_codec_acceptance':{'report':bind(BASE/'mse-small-e2e/result.json'),'logical_mp4_fragments':517,'transport_append_chunks':12,'actual_chunk_boundaries':11,'waiting_after_play':0,'source_changes':1,'limit':'Groups44 unchangedGo256sampleFLAC fragments perappend; this excludes a small-codec syntax failure but does not establish startup with only8192 samples'},
             'platform_stream_verification':platform_binding,'platform_stream_verified':platform is not None,
             'platform_segment_capacity':{'maximum_bytes':1<<20,'default_upper_bound_bytes':96+4*4096+26*4,'small_upper_bound_bytes':96+4*1024+26*4,'wide_upper_bound_bytes':96+4*8192+26*2,'general_encoder_config_automatically_supported':False,'adapter_requirement':'Before consuming PCM/opening a platformstream, reject configs where96+4*FragmentFrames+26*ceil(FragmentFrames/BlockFrames)>1MiB'},
             'capacity_bound':{'maximum_input_frames':1<<20,'maximum_segment_bytes_upper_bound':96+4*(1<<20)+26*((1<<20)//16),'maximum_moof_relative_data_offset':96+8*((1<<20)//16),'maximum_flac_sample_start_exclusive':1<<36,'maximum_fragment_sequence':(1<<32)-1},
             'cleanup':bind(DATA/'audio-stream-encoder-cleanup.json'),'evidence':artifacts,'source_sha256':{str(path.relative_to(ROOT)):sha(path.read_bytes())for path in source_paths},
             'limits':['Encoder is not DSP, infiniteBGM, gameaudio integration or dynamicmix fidelity','Platform scoped lifecycle/517fragment transport is verified only when platform_stream_verified=true; lowlatency dynamicmix, Safari/mobile and game recovery remain separate obligations','FFmpeg is an independent acceptance decoder only; no FFmpeg/core/ROM runtime is used','Original12secondPCM is a transport fixture, not a new loopasset']}
    (DATA/'audio-stream-encoder-validation.json').write_text(json.dumps(summary,ensure_ascii=False,indent=2)+'\n')
    print(json.dumps({'pass':True,'unit_tests_per_environment':6,'primary_roundtrip_cases':24,'edge_roundtrip_cases':4,'native_wasm_bytes_equal':True,'platform_stream_verified':platform is not None},indent=2))

if __name__=='__main__':
    p=argparse.ArgumentParser(description=__doc__);p.add_argument('--platform-report',type=Path,default=ROOT/'games/yuyuhakusho/verification/audio-platform-stream/restored-final1/result.json');a=p.parse_args();platform=a.platform_report.resolve()if a.platform_report else None
    if platform:assert platform.is_relative_to(ROOT)
    main(platform)
