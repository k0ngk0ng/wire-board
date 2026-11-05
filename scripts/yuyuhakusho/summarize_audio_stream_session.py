"""Verify completed stream/PCM evidence without claiming a complete game synth."""
from pathlib import Path
import datetime,hashlib,json
ROOT=Path(__file__).resolve().parents[2]
OUT=ROOT/'games/yuyuhakusho/verification'
def sha(path):return hashlib.sha256(path.read_bytes()).hexdigest()
def bound(name):
 path=ROOT/name;value=json.loads(path.read_text());assert value['pass'],name
 return value,{'path':name,'sha256':sha(path)}
reports=[]
encoder,item=bound('games/yuyuhakusho/data/audio-stream-encoder-validation.json');reports.append(item)
assert encoder['native_wasm_output_bytes_identical'] and encoder['unit_tests_per_environment']==6
assert not encoder['independent_dsp_implemented']
for name,expected in encoder['source_sha256'].items():assert sha(ROOT/name)==expected,name
pcm,item=bound('games/yuyuhakusho/verification/audio-pcm-model/validation.json');reports.append(item)
assert pcm['arithmetic_complete'] and not pcm['independent_dac_cadence_complete'] and not pcm['ym2612_filter_resampler_complete']
for name,expected in pcm['source_sha256'].items():assert sha(ROOT/name)==expected,name
native,item=bound('games/yuyuhakusho/verification/audio-platform-stream/restored-final1/result.json');reports.append(item)
assert len(native['checks'])==5 and native['actual_go_fragments']==517 and len(native['crossings'])==516
assert not native['error'] and not native['sample_gapless_verified'] and not native['original_mix_equivalent']
fixture=ROOT/native['fixture_path'];fixture=fixture/'manifest.json'if fixture.is_dir()else fixture
assert sha(fixture)==native['fixture_manifest_sha256']
for name,expected in native['source_sha256'].items():assert sha(ROOT/name)==expected,name
for label,checks,overlap in [('stream-finite-compatible',8,True),('stream-legacy-compatible',10,False)]:
 value,item=bound('games/yuyuhakusho/verification/audio-platform-media-api/'+label+'/result.json');reports.append(item)
 assert len(value['checks'])==checks and value['finite_overlap_supported']==overlap
 for name,expected in value['source_sha256'].items():assert sha(ROOT/name)==expected,name
unit=OUT/'audio-platform-stream-root-all-unit.log';text=unit.read_text();assert 'tests 83' in text and 'pass 83' in text and 'fail 0' in text and 'skipped 0' in text
go_checks=[]
for environment in ('native','wasm'):
 path=OUT/('audio-stream-all-full-'+environment+'.jsonl');passed=[]
 for line in path.read_text().splitlines():
  event=json.loads(line)
  assert event['Action']!='fail',event
  if event.get('Test')and'/'not in event['Test']and event['Action']=='pass':passed.append((event['Package'],event['Test']))
 assert len(passed)==16,(environment,len(passed))
 go_checks.append({'environment':environment,'tests':16,'path':str(path.relative_to(ROOT)),'sha256':sha(path)})
assert not (OUT/'audio-platform-stream-root-tsc.log').read_text().strip()
freeze=json.loads((OUT/'audio-platform-stream-root-test-first.json').read_text())
for name,expected in freeze['sha256'].items():assert sha(ROOT/name)==expected,name
main=next(value for value in native['checks']if value['kind']=='real-low-buffer-incremental-playback')
assert main['final']['acceptedEndSample']==529200 and main['final']['nextSeq']==518
first=native['firstPlaying'];first_ahead=first['status']['buffered'][-1][1]-first['time'];assert first_ahead<=.5
assert all(item['kind']not in ('waiting','stalled','seeking','error')for item in native['events']if item['at']>first['at']and item.get('id')==first['id'])
assert len(native['removals'])>0 and all(row['end']<row['currentTime']for row in native['removals'])
final_urls=native['final_url_events']
for row in final_urls:
 if row['kind']=='create':assert sum(item['kind']=='revoke'and item['url']==row['url']for item in final_urls)==1
assert all(item['paused']and not item['src']and not item['connected']for item in native['final_media_state'])
source_names=['web/src/audio.ts','web/src/audio-stream.ts','web/src/game-api/module.ts','web/src/game-api/audio-stream.ts','web/tests/platform-audio-stream.test.mjs','scripts/yuyuhakusho/check_audio_stream.py','scripts/yuyuhakusho/audio_stream_harness.ts','scripts/yuyuhakusho/summarize_audio_stream_session.py']
source_names += [str(path.relative_to(ROOT))for directory in ['games/yuyuhakusho/audio/stream','games/yuyuhakusho/audio/pcm']for path in sorted((ROOT/directory).glob('*.go'))]
report={'verified_at_utc':datetime.datetime.now(datetime.timezone.utc).isoformat(),'pass':True,'baseline_commit':'0f3bea2f','stage':'PureGo lossless encoding, native platform stream lifecycle and DPCM arithmetic','goal_complete':False,'release_ready':False,'overall_progress_estimate_percent':81,'game_audio_playback_integrated':False,'independent_audio_synthesis_complete':False,'platform_unit_tests':83,'platform_native_stream_cases':5,'native_fragments':517,'native_crossings':516,'pcm_source_stereo_frames':529200,'native_first_playing_ahead_seconds':first_ahead,'encoder_unit_tests_per_runtime':6,'pcm_arithmetic_unit_tests_per_runtime':4,'go_package_checks':go_checks,'cleanup':{'path':'games/yuyuhakusho/verification/audio-stream-cleanup.json','sha256':sha(OUT/'audio-stream-cleanup.json')},'evidence':reports,'source_sha256':{name:sha(ROOT/name)for name in source_names},'limits':['Original DPCM values and8bit clamp are verified; independent event cadence/bank prefetch/YM/filter/resampler and world scheduling remain incomplete.','Native MSE startup needed9 small fragments (~209ms); original8-fragment pause fixture failure was retained and only its bootstrap supply was repaired. Main low-ahead acceptance was not relaxed.','Exact container roundtrip is separate from browser speaker samples. No sample-exact speaker output, Safari/mobile, input-to-sound latency, complete synthesis or infiniteBGM claim.','Encoder generic maximum segment size is larger than platform1MiB; caller must select a compatible fragment configuration.','All media remains local; no publication, upload, push or deployment.']}
(OUT/'audio-stream-session.json').write_text(json.dumps(report,ensure_ascii=False,indent=2)+'\n')
print('native stream, pureGo encoding and PCM arithmetic verified; actual game synth still pending')
