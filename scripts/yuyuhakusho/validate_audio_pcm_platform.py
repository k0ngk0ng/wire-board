"""Verify final platform PCM evidence; does not execute or replace tests."""
from pathlib import Path
import datetime,hashlib,json
ROOT=Path(__file__).resolve().parents[2];OUT=ROOT/'games/yuyuhakusho/verification/audio-platform-pcm';sha=lambda p:hashlib.sha256(p.read_bytes()).hexdigest()
first=json.loads((OUT/'test-first.json').read_text());review=json.loads((OUT/'review-test-first.json').read_text());reapplied=json.loads((OUT/'review-final-reapplication.json').read_text())
for name,h in first['sha256'].items():
 path=OUT/'harness-before-meaningful-native-coverage.ts.txt'if name.endswith('audio_pcm_platform_harness.ts')else ROOT/name
 assert sha(path)==h,(name,str(path))
for name,h in review['sha256'].items():assert sha(ROOT/name)==h,name
assert sha(ROOT/'web/src/audio-pcm-stream.ts')==reapplied['final_source_sha256']
assert sha(OUT/'controller-before-review-regression.ts.txt')==review['tested_bad_source_sha256']
assert 'ℹ fail 6' in (OUT/'review-actual-red.log').read_text()and 'ℹ pass 6' in (OUT/'review-final-green.log').read_text()
assert 'ℹ tests 105'in(OUT/'all-unit-tdd-final.log').read_text()and 'ℹ fail 0'in(OUT/'all-unit-tdd-final.log').read_text()
assert not (OUT/'tsc-tdd-final.log').read_text().strip()
m=json.loads((OUT/'native-matrix-tdd.json').read_text());assert m['pass']and len(m['checks'])==9
for name,h in m['source_sha256'].items():assert sha(ROOT/name)==h,name
checks=[];metrics=[]
for entry in m['checks']:
 path=ROOT/entry['result'];assert sha(path)==entry['sha256'];r=json.loads(path.read_text());assert r['pass']and not r['speaker_bit_exact']and not r['original_mix_equivalent']
 assert r['status']['nativePlayedSamples']is None and r['status']['nativeDroppedSamples']is None
 assert r['after']['framesCreated']==r['after']['framesClosed']and all(t['state']=='ended'and not t['locked']for t in r['after']['tracks'])and all(t['srcObjectCleared']and t['paused']and t['removed']for t in r['after']['media'])
 for name,h in r['source_sha256'].items():assert sha(ROOT/name)==h,name
 for row in r['clocks']:assert row['pendingBytes']<=4096 and row['nativePlayedSamples']is None and row['nativeDroppedSamples']is None
 accepted=sum(p['frames']for p in r['packets']if p['accepted']);assert accepted==r['status']['acceptedEndSample']
 if entry['scenario']in['smoke','contract','jitter','pause']:assert accepted==r['manifest']['sampleFrames']and r['inputEnded']['status']['phase']=='input-ended'and not r['inputEnded']['released']and r['inputEnded']['status']['lastRejection']=='input-closed'
 if entry['scenario']=='late':assert r['status']['phase']=='failed'and r['status']['error']=='source-too-late'and r['status']['sourceLateSamples']>4410
 firstPacket=r['packets'][0];fp=r['firstPlaying'];metrics.append({'label':entry['label'],'scenario':entry['scenario'],'accepted_samples':accepted,'click_handler_to_playing_ms':fp['gestureDelayMs'],'first_append_to_playing_ms':fp['at']-(firstPacket['at']-firstPacket['waitMs']),'max_ahead_samples':max(p['status']['acceptedEndSample']-p['status']['sourceClockSamples']for p in r['packets']if p['accepted']),'native_clock_at_input_end':r.get('inputEnded',{}).get('status',{}).get('nativeCurrentTimeSeconds'),'source_lateness_on_close_samples':r['status']['sourceLateSamples']})
 checks.append({'result':str(path.relative_to(ROOT)),'sha256':sha(path),'scenario':entry['scenario'],'pass':True})
regress=[]
for name,expected in [('audio-platform-stream/pcm-tdd-regression-mse/result.json',5),('audio-platform-media-api/pcm-tdd-regression-finite/result.json',8),('audio-platform-media-api/pcm-tdd-regression-legacy/result.json',10)]:
 path=ROOT/'games/yuyuhakusho/verification'/name;r=json.loads(path.read_text());assert r['pass']and len(r['checks'])==expected
 for source,h in r['source_sha256'].items():assert sha(ROOT/source)==h,source
 if expected==5:assert r['actual_go_fragments']==517 and len(r['crossings'])==516
 if expected==10:assert not r['finite_overlap_supported']
 regress.append({'path':str(path.relative_to(ROOT)),'sha256':sha(path),'checks':expected,'pass':True})
sources=['web/src/audio-pcm-stream.ts','web/src/game-api/audio-pcm-stream.ts','web/src/audio.ts','web/src/game-api/module.ts','web/tests/platform-audio-pcm.test.mjs','web/tests/platform-audio-pcm-review.test.mjs','scripts/yuyuhakusho/audio_pcm_platform_harness.ts','scripts/yuyuhakusho/activate_public_pcm.mjs','scripts/yuyuhakusho/check_audio_pcm_platform.py','scripts/yuyuhakusho/verify_audio_pcm_platform.py','scripts/yuyuhakusho/validate_audio_pcm_platform.py','docs/research/md-yuyuhakusho/AUDIO_PLATFORM_PCM_FAILURES.md','docs/research/md-yuyuhakusho/AUDIO_PLATFORM_PCM_REVIEW_FAILURES.md','docs/research/md-yuyuhakusho/AUDIO_PLATFORM_PCM.md']
report={'created_at_utc':datetime.datetime.now(datetime.timezone.utc).isoformat(),'pass':True,'scope':'Production platform native generated PCM input, lifecycle/admission and native playback activity; not full original audio/DSP or speaker completion','source_format':{'sample_rate':44100,'channels':2,'format':'s16le','packet_max_frames':1024,'max_ahead_samples':2048,'late_budget_samples':4410},'tests':{'original_controlled_pcm_cases':16,'new_review_regressions':6,'existing_platform_cases':83,'all_platform_cases':105,'unit_log':'games/yuyuhakusho/verification/audio-platform-pcm/all-unit-tdd-final.log','unit_log_sha256':sha(OUT/'all-unit-tdd-final.log'),'tsc_log':'games/yuyuhakusho/verification/audio-platform-pcm/tsc-tdd-final.log','tsc_log_sha256':sha(OUT/'tsc-tdd-final.log')},'native_cases':checks,'metrics':metrics,'regressions':regress,'test_first':{'initial':'test-first.json','initial_unit_red':'unit-final-red.log','initial_native_red':'native-missing-final-red.log','review':'review-test-first.json','review_bad_source':'controller-before-review-regression.ts.txt','review_actual_red':'review-actual-red.log','final_reapplication':'review-final-reapplication.json','honest_premature_review_history':'review-fix-rewind.json','native_coverage_strengthening':'native-coverage-refinement.json'},'native_played_samples':None,'native_dropped_samples':None,'input_end_means_native_drain':False,'speaker_bit_exact':False,'original_mix_equivalent':False,'safari_mobile_verified':False,'host_explicit_stop_required':True,'publication':False,'source_sha256':{name:sha(ROOT/name)for name in sources},'evidence':[{'path':str(path.relative_to(ROOT)),'sha256':sha(path)}for path in sorted(OUT.rglob('*'))if path.is_file()and path.name!='validation.json']}
(OUT/'validation.json').write_text(json.dumps(report,ensure_ascii=False,indent=2)+'\n');print('PCM public contract verified:105 unit cases,9 native scenarios,MSE517+finite8+legacy10')
