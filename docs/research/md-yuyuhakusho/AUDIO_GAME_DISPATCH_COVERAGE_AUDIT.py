"""Read-only sound request closure audit for the frozen platform fight flow."""
from pathlib import Path
import argparse
import collections
import gzip
import hashlib
import json

ROOT = Path(__file__).resolve().parents[3]
ROM_SHA = 'c39d52b2e2298b3784ada3de70d64b26b67e172114c7d18494faad70fc4d7c06'


def sha(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def bind(path):
    return {'path': str(path.relative_to(ROOT)), 'sha256': sha(path), 'bytes': path.stat().st_size}


def load(path):
    raw = path.read_bytes()
    return json.loads(gzip.decompress(raw) if path.suffix == '.gz' else raw)


def source_entry(path, anchor, ids, reason):
    full = ROOT / path
    lines = full.read_text().splitlines()
    positions = [i + 1 for i, line in enumerate(lines) if anchor in line]
    assert positions, (path, anchor)
    return {'source': bind(full), 'line': positions[0], 'anchor': anchor,
            'request_ids': ids, 'request_ids_hex': [f'{code:02x}' for code in ids],
            'reason': reason}


def main(out):
    assert out.is_relative_to(ROOT)
    resources_path = ROOT / 'docs/research/md-yuyuhakusho/AUDIO_GAME_DISPATCH_RESOURCES_INDEX.json'
    resources = load(resources_path)
    assert resources['pass'] and resources['resource_files'] == 28 and len(resources['resource_ids']) == 99
    original_path = ROOT / 'docs/research/md-yuyuhakusho/AUDIO_GAME_DISPATCH_ORIGINAL_IDS.json'
    original = load(original_path)
    assert len(original['dispatch']) == 256 and original['music']['distinct_headers'] == 22
    assert original['music']['walk_errors'] == original['primary']['walk_errors'] == []
    assert original['secondary']['normal_dispatch_initialization_ids'] == []
    assert original['special_controls']['hardware_lfo_music_ids'] == [0x96]
    assert all(not original['special_controls'][name] for name in ('fm3_control_music_ids', 'pitch_envelope_music_ids',
               'hardware_lfo_primary_ids', 'fm3_control_primary_ids', 'pitch_envelope_primary_ids', 'direct_ym_music', 'direct_ym_primary', 'initial_music_pitch_nonzero'))
    rom_path = ROOT / '.local/md-yuyuhakusho/downloads/yuyuhakusho-jp.bin'
    rom = rom_path.read_bytes()
    assert sha(rom_path) == ROM_SHA
    catalog_path = ROOT / 'games/yuyuhakusho/data/audio-score.json.gz'
    catalog = load(catalog_path)
    assets_path = ROOT / 'games/yuyuhakusho/data/audio-assets.json'
    assets = load(assets_path)
    ordinary = {row['code']: row for row in assets['assets'] if row['code'] < 128}
    assert sorted(ordinary) == list(range(6, 127))
    score_ids = {row['id'] for row in catalog['scores']}
    music_ids = {row['id'] for row in catalog['scores'] if row['kind'] == 'music'}
    primary_ids = score_ids - music_ids
    assert primary_ids == set(range(0xa0, 0xac))
    entries = [
        source_entry('games/yuyuhakusho/engine/match_start.go', 'startSound(&state, 4)', [4], 'Battle scene initialization clears the sound driver.'),
        source_entry('games/yuyuhakusho/engine/match_start.go', 'music := [8]uint8', [0x82, 0x86, 0x83, 0x95, 0x85, 0x8a, 0x88, 0x8d], 'Eight validated arena indices 0..7.'),
        source_entry('games/yuyuhakusho/engine/match_start.go', 'music = 0x87', [0x87], 'Rule0 and original character slot4=10 override.'),
        source_entry('games/yuyuhakusho/engine/match_start.go', 'music = 0x84', [0x84], 'Rule0 and original character slot4=7 override.'),
        source_entry('games/yuyuhakusho/engine/match_lifecycle.go', 'queueMatchSound(&state, 1)', [1], 'Abort uses original fade control.'),
        source_entry('games/yuyuhakusho/engine/match_lifecycle.go', 'queueMatchSound(&state, 5)', [5], 'KO transition uses original fade control.'),
        source_entry('games/yuyuhakusho/engine/match_lifecycle.go', 'queueMatchSound(state, 0x8f)', [0x8f], 'Bounded character result/taunt trigger.'),
        source_entry('games/yuyuhakusho/engine/match_overlays.go', 'sound, first :=', [0x58, 0x59], 'Intro branch selects one of two requests.'),
        source_entry('games/yuyuhakusho/engine/match_overlays.go', 'sound, x, parameter :=', [0x5c, 0x5d], 'Result branch selects one of two requests.'),
        source_entry('games/yuyuhakusho/engine/match_overlays.go', 'parameter, sound, timer :=', [0x5a, 0x5b], 'Result branch selects one of two requests.'),
        source_entry('games/yuyuhakusho/engine/match_overlays.go', 'overlaySound(&state.SoundQueue, 0x6e)', [0x6d, 0x6e, 0x66], 'Result overlay count/team branches.'),
        source_entry('games/yuyuhakusho/engine/match_overlays.go', 'sounds := [11]uint8', [0x6b, 0x69, 0x68, 0x67, 0x6a, 0x71, 0x70, 0x72, 0x6c, 0x6f, 0x6c], 'Validated character IDs1..11.'),
        source_entry('games/yuyuhakusho/engine/match_overlays.go', 'uint8(0x5e+title-1)', list(range(0x5e, 0x66)), 'Validated team title IDs1..8.'),
        source_entry('games/yuyuhakusho/engine/attack_contacts.go', 'effect, sound := strength', [0x7a, 0x7b, 0x7c, 0x78], 'Strength is attackFlags&3 and this branch requires strength<3.'),
        source_entry('games/yuyuhakusho/engine/attack_contacts.go', 'state.PendingSound = [...]uint8', [0x29, 0x19, 0x13, 0x0b, 0x21, 0x39, 0x55, 0x4d, 0x32, 0x3f, 0x48], 'KO voices indexed by validated character0..10.'),
        source_entry('games/yuyuhakusho/engine/clash.go', 'state.PendingSound = 0x77', [0x77], 'Clash/guard branch.'),
        source_entry('games/yuyuhakusho/engine/reaction_motion.go', '= 0x7d,', [0x7d], 'Landing/reaction branches.'),
        source_entry('games/yuyuhakusho/engine/fighter0_motion.go', 'state.PendingSound = 0xa6', [0xa6], 'Yusuke charge clock&3 ==0.'),
        source_entry('games/yuyuhakusho/engine/fighter4_projectile.go', 'v.PendingSound = 0xf0', [0xf0], 'Hiei dragon head SoundClock==0; counter wraps modulo256.'),
    ]
    fight_music = set(entries[1]['request_ids'] + entries[2]['request_ids'] + entries[3]['request_ids'] + entries[6]['request_ids'])
    assert fight_music == music_ids
    nested = [{'score_id': score['id'], 'voice_id': voice['id'], 'event': event}
              for score in catalog['scores'] for voice in score['voices']
              for part in ('prefix', 'cycle') for event in voice['timeline'][part]
              if event['kind'] == 'enqueue-replace-first']
    assert len(nested) == 1 and nested[0]['score_id'] == 0xab and nested[0]['event']['sound_id'] == 0x7d
    engine_ids = {code for row in entries for code in row['request_ids']}
    fight_ids = set(resources['resource_ids']) | engine_ids | {0x7d}
    controls = fight_ids & set(range(6))
    assert controls == {1, 4, 5}
    gaps = fight_ids - set(ordinary) - score_ids - controls
    assert gaps == {0xf0}
    assert not fight_ids & set(range(0xac, 0xf0))

    natural_path = ROOT / 'games/yuyuhakusho/data/audio-natural-events.json.gz'
    natural = load(natural_path)
    request_count = collections.Counter(row['code'] for row in natural['requests'])
    dispatch_count = collections.Counter(row['code'] for row in natural['selected'])
    menu_only_observed = sorted(set(dispatch_count) & set(range(0x81, 0xa0)) - fight_music)
    assert menu_only_observed == [0x8e, 0x90, 0x92, 0x93, 0x96]
    menu_examples = {f'{code:02x}': {'request': [row for row in natural['requests'] if row['code'] == code],
                                    'dispatch': [row for row in natural['selected'] if row['code'] == code]}
                     for code in menu_only_observed}

    dragon_path = ROOT / 'games/yuyuhakusho/engine/testdata/fighter4-projectile-oracle.json.gz'
    dragon_manifest_path = ROOT / 'games/yuyuhakusho/engine/testdata/fighter4-projectile-oracle-manifest.json'
    dragon_manifest = load(dragon_manifest_path)
    assert dragon_manifest['pass'] and dragon_manifest['frames_match_original_core'] and dragon_manifest['gzip_sha256'] == sha(dragon_path)
    dragon_raw = gzip.decompress(dragon_path.read_bytes())
    assert hashlib.sha256(dragon_raw).hexdigest() == dragon_manifest['fixture_sha256']
    dragon = json.loads(dragon_raw)
    dragon_hits = []
    for index, call in enumerate(dragon['calls']):
        if call['kind'] == 'head' and call['before']['sound_queue'] == [0, 0, 0, 0] and call['after']['sound_queue'] == [0xf0, 0, 0, 0]:
            dragon_hits.append({'json_pointer': f'/calls/{index}', 'frame': call['frame'], 'scenario': call['scenario'],
                                'object_id': call['object_id'], 'before_queue': call['before']['sound_queue'], 'after_queue': call['after']['sound_queue']})
    assert len(dragon_hits) == 2 and sorted(row['frame'] for row in dragon_hits) == [3479, 3481]
    descriptor_address = 0x380a8 + 0x7a * 8
    descriptor = rom[descriptor_address:descriptor_address + 8]
    assert descriptor.hex() == '0b80008014814000'
    bank = descriptor[0] << 16 | (descriptor[1] & 128) << 8
    header = bank + (int.from_bytes(descriptor[2:4], 'little') & 0x7fff)
    sample = bank + (int.from_bytes(rom[header:header + 2], 'little') & 0x7fff)
    size = int.from_bytes(rom[header + 2:header + 4], 'little')
    sample_hash = hashlib.sha256(rom[sample:sample + size]).hexdigest()
    assert size == 6670 and sample_hash == 'c6409ebd28c594205970069fbc3233241cf15dc08ce8b847a5fa118b3e1a5382'
    known_pcm = {row['encoded_sha256'] for row in ordinary.values()}
    music_payloads = {row['id'] for row in catalog['dac_payloads']}
    assert not known_pcm & music_payloads and sample_hash not in known_pcm | music_payloads

    frozen_path = ROOT / 'games/yuyuhakusho/verification/audio-score-driver/frozen-sources.json'
    frozen = load(frozen_path)
    for path, expected in frozen['bindings'].items():
        assert sha(ROOT / path) == expected, path
    all_engine = sorted(path for path in (ROOT / 'games/yuyuhakusho/engine').glob('*.go') if not path.name.endswith('_test.go'))
    paths = [catalog_path, assets_path, natural_path, dragon_path, dragon_manifest_path, resources_path, original_path, frozen_path,
             ROOT / 'docs/research/md-yuyuhakusho/AUDIO_GAME_DISPATCH_ORIGINAL_EVIDENCE.json',
             ROOT / 'docs/research/md-yuyuhakusho/AUDIO_GAME_DISPATCH_ORIGINAL_RESULTS.md',
             ROOT / 'docs/research/md-yuyuhakusho/AUDIO_GAME_DISPATCH_RESOURCES_RESULTS.md',
             ROOT / 'games/yuyuhakusho/engine/world_clock.go', ROOT / 'games/yuyuhakusho/engine/sound_scheduler.go',
             ROOT / 'docs/research/md-yuyuhakusho/static/audio-dispatch.txt', ROOT / 'docs/research/md-yuyuhakusho/static/audio-control.txt']
    result = {'schema': 1, 'scope': 'Read-only request/path audit; frozen revision92f27271 unchanged',
              'pass': True, 'runtime_coverage_complete': False, 'rom_sha256': ROM_SHA,
              'resource_index': bind(resources_path), 'resource_nonzero_ids': resources['resource_ids'],
              'original_driver_index': bind(original_path), 'original_dispatch_id_count': 256,
              'original_distinct_music_headers': 22, 'original_control_path_audit': original['special_controls'],
              'group2_normal_dispatch_initialization_ids': original['secondary']['normal_dispatch_initialization_ids'],
              'engine_bindings': entries, 'engine_ids': sorted(engine_ids),
              'platform_fight_candidate_ids': sorted(fight_ids), 'platform_fight_candidate_ids_hex': [f'{x:02x}' for x in sorted(fight_ids)],
              'fight_music_ids': sorted(fight_music), 'fight_music_catalog_closed': True,
              'score_nested_enqueue': nested, 'score_nested_queue_introduces_uncovered_music': False,
              'game_control_ids': sorted(controls), 'controls_are_not_pcm_or_score_assets': True,
              'coverage_gaps_against_121_pcm_plus_23_score': [0xf0],
              'f0': {'descriptor_index': 122, 'descriptor_address': descriptor_address, 'descriptor_hex': descriptor.hex(),
                     'sample_address': sample, 'encoded_bytes': size, 'encoded_sha256': sample_hash,
                     'rate': descriptor[4], 'flags': descriptor[5], 'routing': descriptor[6], 'natural_original_queue_writes': dragon_hits,
                     'normal_game_reachability': 'Original head callbacks from declared legal physical menu/game inputs, plus source branch.',
                     'not_equivalent_to_ordinary_7a': True},
              'payload_owner': {'ordinary_non_diagnostic': 121, 'music_payloads': 6, 'ordinary_music_overlap': [],
                                'additional_legal_f0_payload': 1, 'required_non_diagnostic_distinct_minimum': 128,
                                'frozen_score_gzip_sha256': sha(catalog_path), 'frozen_score_json_sha256': hashlib.sha256(gzip.decompress(catalog_path.read_bytes())).hexdigest(),
                                'migration': 'New pcmassets imports frozen schema2 music payloads; no schema2 edit in this audit.'},
              'original_menu_observed_uncovered_music_ids': menu_only_observed,
              'original_menu_examples': menu_examples,
              'request_and_dispatch_counts': {'frames': natural['report']['frames'], 'requests': sum(request_count.values()), 'dispatches': sum(dispatch_count.values())},
              'source_bindings': [bind(path) for path in dict.fromkeys(paths)], 'all_engine_file_bindings': [bind(path) for path in all_engine],
              'audit_script': bind(Path(__file__).resolve()),
              'limits': ['Resource sound fields are candidate requests, not a claim that every entry is naturally reached.',
                         'Queue request, dispatcher selection and actual chip output are different observation layers.',
                         'Original menus are skipped by the platform preparation/battle flow; this does not make their five observed music IDs diagnostic.',
                         'Group2 guard, all22-header controls and bypass-candidate scope are supplied by the separate original-driver audit.',
                         'This audit does not implement controls/fade, payload migration, master-cycle scheduling or DSP.',
                         'No production source, frozen catalog, portable golden or prior proof is modified.']}
    report_path = ROOT / 'docs/research/md-yuyuhakusho/AUDIO_GAME_DISPATCH_COVERAGE.md'
    if report_path.exists():
        result['report'] = bind(report_path)
    for row in result['source_bindings'] + result['all_engine_file_bindings'] + [result['audit_script']]:
        assert sha(ROOT / row['path']) == row['sha256'], row['path']
    result['audit_verified_sources_unchanged'] = True
    out.parent.mkdir(parents=True, exist_ok=True)
    out.write_text(json.dumps(result, indent=2) + '\n')
    print(json.dumps({'pass': True, 'fight_candidates': len(fight_ids), 'fight_music': [f'{x:02x}' for x in sorted(fight_music)],
                      'gaps': ['f0'], 'menu_observed': [f'{x:02x}' for x in menu_only_observed], 'manifest': str(out.relative_to(ROOT))}, indent=2))


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--out', type=Path, required=True)
    args = parser.parse_args()
    main(args.out.resolve())
