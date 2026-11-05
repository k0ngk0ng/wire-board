"""Bind completed audio components without claiming game audio is integrated."""
from pathlib import Path
import datetime, hashlib, json

ROOT = Path(__file__).resolve().parents[2]
OUT = ROOT / 'games/yuyuhakusho/verification'

def sha(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()

def read(relative, passed=True):
    path = ROOT / relative
    data = json.loads(path.read_text())
    if passed:
        assert data['pass'], relative
    return data, {'path': relative, 'sha256': sha(path)}

components = {}
for relative in [
    'games/yuyuhakusho/data/audio-finite-catalog-validation.json',
    'games/yuyuhakusho/engine/testdata/sound-scheduler-validation.json',
    'games/yuyuhakusho/data/audio-music-survey-validation.json',
    'games/yuyuhakusho/data/audio-native-carrier-validation.json',
    'games/yuyuhakusho/verification/audio-platform-media-api/visibility-setup-validation.json',
    'games/yuyuhakusho/verification/audio-dispatch/validation.json',
]:
    data, artifact = read(relative)
    components[relative] = artifact

finite, _ = read('games/yuyuhakusho/data/audio-finite-catalog-validation.json')
assert finite['coverage']['finite_clips'] == 134 and finite['coverage']['validated_wav_bytes'] == 26479688
assert finite['native_wasm_cli_facts_identical'] and not finite['overall_audio_complete']
for key in ('catalog', 'trusted_manifest'):
    item = finite[key]
    assert sha(ROOT / item['path']) == item['sha256']

media, media_artifact = read('games/yuyuhakusho/verification/audio-platform-media-api/finite-restored-final1/result.json')
legacy, legacy_artifact = read('games/yuyuhakusho/verification/audio-platform-media-api/finite-legacy-final1/result.json')
assert media['finite_overlap_supported'] and len(media['checks']) == 8
assert not legacy['finite_overlap_supported'] and len(legacy['checks']) == 10
for result in (media, legacy):
    assert not result['fake_media'] and not result['AudioContextCreated']
    assert not result['original_mix_equivalent'] and not result['sample_gapless_verified']
    for name, expected in result['source_sha256'].items():
        assert sha(ROOT / name) == expected, ('stale browser source', name)

freeze, _ = read('games/yuyuhakusho/verification/audio-platform-finite-root-test-first.json', False)
for name, expected in freeze['sha256'].items():
    path = ROOT / name
    if name.endswith('/check_audio_media_api.py'):
        path = OUT / 'audio-platform-media-api/checker-before-visibility-setup.py.txt'
    assert sha(path) == expected, ('changed frozen contract', name)
unit_log = OUT / 'audio-platform-finite-all-tests.log'
text = unit_log.read_text()
assert 'tests 75' in text and 'pass 75' in text and 'fail 0' in text and 'skipped 0' in text
assert not (OUT / 'audio-platform-finite-tsc.log').read_text().strip()

music, _ = read('games/yuyuhakusho/data/audio-music-survey-validation.json')
assert music['coverage']['original_hook_music_steps_compared'] == 130480
assert music['coverage']['tracks'] == 72 and not music['independent_dsp_implemented']
assert not music['platform_stream_integrated'] and not music['exact_pcm_loop_assets_ready']
carrier, _ = read('games/yuyuhakusho/data/audio-native-carrier-validation.json')
actual = carrier['actual_playback']
assert actual['pass'] and actual['source_pcm_roundtrip_exact'] and actual['stereo_frames'] == 529200
assert actual['observed_boundaries'] == 11 and actual['native_waiting_after_play'] == 0 and actual['source_changes'] == 1
assert not actual['audio_context_created']
arbitration, _ = read('games/yuyuhakusho/verification/audio-dispatch/validation.json')
assert arbitration['pcm_arbitration_helper_complete'] and not arbitration['full_audio_dispatch_complete']
assert arbitration['coverage']['private_original_calls'] == 13036
assert arbitration['coverage']['unknown_private_m68k_ram_writes'] == 0
assert arbitration['mix_counterexample']['unmodified_sum_different_sample_values'] == 161234

restoration, restoration_artifact = read('games/yuyuhakusho/verification/audio-tool-directory-restoration.json', False)
assert restoration['after'].split()[0] == 'local'
assert Path(restoration['path']).read_text().split()[0] == 'local'
assert not restoration['other_external_files_modified']

sources = [ROOT / name for name in [
    'web/src/audio.ts', 'web/src/game-api/module.ts', 'web/tests/platform-audio-finite.test.mjs',
    'games/yuyuhakusho/engine/sound_scheduler.go', 'games/yuyuhakusho/engine/sound_scheduler_test.go',
    'games/yuyuhakusho/engine/sound_pcm_arbitration.go', 'games/yuyuhakusho/engine/sound_pcm_arbitration_test.go',
    'scripts/yuyuhakusho/check_audio_media_api.py', 'scripts/yuyuhakusho/audio_finite_lanes_harness.ts',
    'scripts/yuyuhakusho/activate_audio_media_page.mjs', 'scripts/yuyuhakusho/summarize_audio_foundations.py',
]]
sources += [ROOT / 'games/yuyuhakusho/audio' / name for name in ('catalog.go', 'catalog_test.go', 'wav.go')]
report = {
    'verified_at_utc': datetime.datetime.now(datetime.timezone.utc).isoformat(),
    'baseline_commit': 'aa617e23',
    'stage': 'Finite media integrity, reentrant scheduler phases, native finite lanes and continuous lossless carrier evidence',
    'pass': True, 'goal_complete': False, 'release_ready': False,
    'overall_progress_estimate_percent': 81,
    'game_audio_playback_integrated': False,
    'independent_music_synthesis_implemented': False,
    'original_mixer_implemented': False,
    'platform_audio': {'unit_tests': 75, 'real_finite_lifecycle_checks': 8, 'real_legacy_lifecycle_checks': 10, 'finite_result': media_artifact, 'legacy_result': legacy_artifact, 'unit_log': {'path': str(unit_log.relative_to(ROOT)), 'sha256': sha(unit_log)}},
    'pcm_arbitration': {'private_calls': 13036, 'real_queue_decisions': 33, 'tests_per_runtime': 4, 'full_driver_complete': False},
    'components': list(components.values()),
    'tool_setting_restored_with_user_authorization': restoration_artifact,
    'cleanup': {'path': 'games/yuyuhakusho/verification/audio-foundations-cleanup.json', 'sha256': sha(OUT / 'audio-foundations-cleanup.json')},
    'limits': [
        'The actual game still has no integrated playback; these components do not supply a frame-end-only sound consumer.',
        'Natural callback-internal sound consumption and driver-originated requests require an independently derived causal schedule.',
        'Finite native lanes offer lifecycle concurrency, not original PCM/FM/PSG mixing equivalence.',
        'Native carrier proves Chromium lossless-container transport over11 boundaries, not independent synthesis, infinite BGM, exact speaker samples or Safari/mobile support.',
        'New music sequence recurrence does not establish repeatable PCM; remaining long common periods are structural candidates.',
        'No media publication, upload, push or deployment.'
    ],
    'source_sha256': {str(path.relative_to(ROOT)): sha(path) for path in sources},
}
(OUT / 'audio-foundations-session.json').write_text(json.dumps(report, ensure_ascii=False, indent=2) + '\n')
print('audio foundations verified; full game audio remains incomplete; Go setting restored with authorization')
