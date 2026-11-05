"""Read only resource dispatch fields and retained original observations."""
from pathlib import Path
import collections
import gzip
import hashlib
import json

ROOT = Path(__file__).resolve().parents[3]
OUT = ROOT / '.local/md-yuyuhakusho/audio-game-dispatch-resources'
ROM = 'c39d52b2e2298b3784ada3de70d64b26b67e172114c7d18494faad70fc4d7c06'
sources = {}


def digest(raw):
    return hashlib.sha256(raw).hexdigest()


def load(relative, expected=None):
    path = ROOT / relative
    raw = path.read_bytes()
    identity = digest(raw)
    if expected:
        assert identity == expected, relative
    decoded = gzip.decompress(raw) if path.suffix == '.gz' else raw
    sources[relative] = {'path': relative, 'sha256': identity, 'bytes': len(raw),
                         'decoded_json_sha256': digest(decoded)}
    return json.loads(decoded)


def pointer(parts):
    return '/' + '/'.join(str(x).replace('~', '~0').replace('/', '~1') for x in parts)


def grouped(ids):
    return {'low_01_7f': sorted(x for x in ids if 1 <= x <= 0x7f),
            'raw_80': sorted(x for x in ids if x == 0x80),
            'raw_81_9f': sorted(x for x in ids if 0x81 <= x <= 0x9f),
            'primary_a0_ab': sorted(x for x in ids if 0xa0 <= x <= 0xab),
            'group2_ac_ef': sorted(x for x in ids if 0xac <= x <= 0xef),
            'raw_f0_ff': sorted(x for x in ids if 0xf0 <= x <= 0xff)}


def resource_walk(value, parts, rows, counts):
    if isinstance(value, dict):
        for key, child in value.items():
            if key == 'sound':
                assert type(child) is int and 0 <= child <= 255
                counts['sound_fields'] += 1
                if child:
                    row = {'pointer': pointer(parts + [key]), 'id': child, 'hex': f'{child:02x}'}
                    if 'data' == (parts[-1] if parts else None):
                        row['trigger'] = 'SubmitAnimationRecord: selected record and actor.clock.ticks == 0'
                    elif len(parts) >= 2 and parts[-2] == 'types':
                        row['trigger'] = 'StepEffectMotion: selected type and stage == 0'
                    else:
                        row['trigger'] = 'typed sound field; consumer context retained in exact pointer'
                    rows.append(row)
                else:
                    counts['zero_no_request_fields'] += 1
            elif key in ('character_sounds', 'team_sounds'):
                assert isinstance(child, list)
                for index, command in enumerate(child):
                    assert type(command) is int and 0 < command <= 255
                    counts['sound_table_entries'] += 1
                    rows.append({'pointer': pointer(parts + [key, index]), 'id': command,
                                 'hex': f'{command:02x}',
                                 'trigger': 'Match overlay table index; corresponding callback and index must be reached'})
            elif key == 'sound_index':
                counts['excluded_lookup_indices'] += 1
            else:
                resource_walk(child, parts + [key], rows, counts)
    elif isinstance(value, list):
        for index, child in enumerate(value):
            resource_walk(child, parts + [index], rows, counts)


def main():
    OUT.mkdir(parents=True, exist_ok=True)
    manifest_path = 'games/yuyuhakusho/data/chu-world-rules-manifest.json'
    manifest = load(manifest_path)
    assert manifest['rom_sha256'] == ROM
    resources = [('games/yuyuhakusho/data/chu-world-rules.json.gz', manifest['gzip_sha256'])]
    resources.extend((entry['path'], entry['sha256']) for entry in manifest['sources'])
    resources.append(('games/yuyuhakusho/engine/testdata/match-overlays-data.json', None))
    records, resource_rows, resource_ids = [], [], set()
    seen = set()
    for relative, expected in resources:
        if relative in seen:
            continue
        seen.add(relative)
        value = load(relative, expected)
        rows, counts = [], collections.Counter()
        resource_walk(value, [], rows, counts)
        ids = sorted({row['id'] for row in rows})
        resource_ids.update(ids)
        for row in rows:
            row['source'] = relative
        resource_rows.extend(rows)
        records.append({'source': sources[relative], 'counts': dict(counts),
                        'ids': ids, 'ids_hex': [f'{x:02x}' for x in ids],
                        'groups': grouped(ids)})
    resources_report = {'schema': 1, 'level': 'typed-resource-request-capability',
                        'files': records, 'nonzero_fields': resource_rows,
                        'ids': sorted(resource_ids), 'groups': grouped(resource_ids),
                        'limits': ['Availability of a selected record/type, not normal gameplay reachability of every selector.',
                                   'source.sound_index is a lookup index and is excluded.',
                                   'No whole-game negative reachability conclusion.']}

    natural_path = 'games/yuyuhakusho/data/audio-natural-events.json.gz'
    natural = load(natural_path)
    report = natural['report']
    assert report['pass'] and report['ram_video_match_original'] and report['pcm_frames_match_original']
    assert report['identity']['rom_sha256'] == ROM
    observed = []
    request_counts, dispatch_counts = collections.Counter(), collections.Counter()
    for field, kind, target in (('requests', 'natural-request-entry', request_counts),
                                ('selected', 'natural-dispatch-entry', dispatch_counts)):
        for index, entry in enumerate(natural[field]):
            command = entry['code']
            assert type(command) is int and 0 <= command <= 255
            target[command] += 1
            observed.append({'source': natural_path, 'pointer': f'/{field}/{index}/code',
                             'id': command, 'hex': f'{command:02x}', 'kind': kind,
                             'frame': entry['frame'], 'pc': entry['pc'], 'caller': entry.get('caller')})
    assert dict(request_counts) == {int(key): value for key, value in report['request_codes'].items()}
    assert dict(dispatch_counts) == {int(key): value for key, value in report['selected_codes'].items()}
    assert len(natural['requests']) == report['request_count'] == 109
    assert len(natural['selected']) == report['selected_count'] == 107

    animation_path = 'games/yuyuhakusho/data/animation-catalog.json.gz'
    catalog = load(animation_path)
    original_path = 'games/yuyuhakusho/data/animation-catalog-oracle.json.gz'
    original = load(original_path)
    assert original['catalog_sha256'] == sources[animation_path]['decoded_json_sha256']
    catalog_validation = load('games/yuyuhakusho/data/animation-catalog-validation.json')
    for relative, expected in catalog_validation['source_sha256'].items():
        assert digest((ROOT / relative).read_bytes()) == expected, relative
    assert catalog_validation['pass'] and catalog_validation['all_extracted_records_verified']
    keys = {tuple(sorted(frame['key'].items())): frame for frame in catalog['frames']}
    isolated_counts, selected_frames = collections.Counter(), 0
    for index, call in enumerate(original['calls']):
        if call['variant'] != 0:
            continue
        selected_frames += 1
        assert call['before']['actor']['clock']['ticks'] == 0
        assert call['before']['pending_sound'] == 0 and call['before']['sound_queue'] == [0, 0, 0, 0]
        frame = keys[tuple(sorted(call['key'].items()))]
        command = frame['data']['sound']
        assert call['after']['pending_sound'] == 0
        assert call['after']['sound_queue'] == [command, 0, 0, 0]
        if command:
            isolated_counts[command] += 1
            observed.append({'source': original_path, 'pointer': f'/calls/{index}/after/sound_queue/0',
                             'id': command, 'hex': f'{command:02x}',
                             'kind': 'original-isolated-animation-queue-write',
                             'character_id': frame['character_id'], 'key': call['key']})
    assert selected_frames == len(catalog['frames']) == 4732

    phases_path = 'games/yuyuhakusho/engine/testdata/audio-timing-natural-phases.json.gz'
    phases_manifest = load('games/yuyuhakusho/engine/testdata/audio-timing-natural-phases-manifest.json')
    phases = load(phases_path, phases_manifest['fixture_gzip_sha256'])
    assert phases_manifest['pass']
    consumer_counts, empty_consumers = collections.Counter(), 0
    for index, entry in enumerate(phases['consumers']):
        command = entry['after']['selected']
        if command == 255:
            empty_consumers += 1
            continue
        consumer_counts[command] += 1
        observed.append({'source': phases_path, 'pointer': f'/consumers/{index}/after/selected',
                         'id': command, 'hex': f'{command:02x}',
                         'kind': 'natural-queue-consumer-selection',
                         'scenario': entry['scenario'], 'name': entry['name']})
    observations_report = {'schema': 1, 'level': 'retained-original-runtime-observation',
                           'rows': observed,
                           'natural_window': {'frames': report['frames'], 'requests': 109, 'dispatches': 107},
                           'original_isolated_animation_frames': selected_frames,
                           'counts': {'natural_request': dict(sorted(request_counts.items())),
                                      'natural_dispatch': dict(sorted(dispatch_counts.items())),
                                      'original_isolated_animation_queue_write': dict(sorted(isolated_counts.items())),
                                      'natural_consumer_selection': dict(sorted(consumer_counts.items())),
                                      'empty_consumer_ff_sentinel': empty_consumers},
                           'groups': {'natural_request': grouped(request_counts),
                                      'natural_dispatch': grouped(dispatch_counts),
                                      'original_isolated_animation_queue_write': grouped(isolated_counts),
                                      'natural_consumer_selection': grouped(consumer_counts)},
                           'limits': ['Isolated animation selection is not proof of normal player reachability.',
                                      'Consumer selection is not automatically a chip output or completed audio.',
                                      'Raw IDs are not normalized to aliases or presumed distinct music tracks.']}
    # Prior private command injection is runtime evidence but cannot establish
    # that any normal game rule requests that command. Keep it in its own layer.
    dispatch_validation = load('games/yuyuhakusho/verification/audio-dispatch/validation.json')
    study_binding = dispatch_validation['original_paired_study']
    study = load(study_binding['path'], study_binding['sha256'])
    assert dispatch_validation['pass'] and study['pass'] and study['all_original_observed_frames_equal']
    f0_case = study['cases'][18]
    assert f0_case['name'] == 'special-f0' and f0_case['commands'] == [[0, 240]] and f0_case['frames_match']
    f0_files = {Path(entry['path']).name: entry for entry in f0_case['files']}
    observed_binding = f0_files['special-f0-observed.json.gz']
    original_binding = f0_files['special-f0-original.json.gz']
    f0_observed = load(observed_binding['path'], observed_binding['sha256'])
    f0_original = load(original_binding['path'], original_binding['sha256'])
    assert len(f0_observed['rows']) == len(f0_original['rows']) == 190
    for original_row, observed_row in zip(f0_original['rows'], f0_observed['rows']):
        for key in ('step', 'reference', 'stereo_frames', 'pcm_sha256'):
            assert original_row[key] == observed_row[key]
    assert len(f0_observed['dispatches']) == 1 and f0_observed['dispatches'][0]['code'] == 240
    assert f0_observed['dispatches'][0]['after']['slot_busy'] == [0, 128]
    assert f0_observed['rows'][0]['state']['slot_busy'] == [0, 64]
    private_control = [{'id': 240, 'hex': 'f0', 'kind': 'original-private-injected-command',
                        'source': observed_binding['path'], 'pointer': '/dispatches/0/code',
                        'paired_original_source': original_binding['path'], 'frames': 190,
                        'accepted_slot': 'B',
                        'pending_pointer': '/dispatches/0/after/slot_busy/1',
                        'active_pointer': '/rows/0/state/slot_busy/1',
                        'normal_game_request_reachability': 'not established'}]
    observations_report['prior_private_controls'] = private_control
    by_id = collections.defaultdict(list)
    for entry in resource_rows:
        by_id[entry['id']].append({'source': entry['source'], 'pointer': entry['pointer']})
    universe = set(resource_ids) | set(request_counts) | set(dispatch_counts) | set(isolated_counts) | set(consumer_counts)
    universe.update((0x81, 0x89, 0x8b))
    universe.update(range(0xac, 0x100))
    matrix = [{'id': command, 'hex': f'{command:02x}',
               'resource_fields': by_id[command],
               'natural_request_entries': request_counts[command],
               'natural_dispatch_entries': dispatch_counts[command],
               'original_isolated_animation_queue_writes': isolated_counts[command],
               'natural_consumer_selections': consumer_counts[command],
               'prior_private_injected_command_observations': 1 if command == 240 else 0}
              for command in sorted(universe)]
    summary = {'schema': 1, 'scope': 'Read-only declared game resources and bounded retained original observations',
               'pass': True, 'rom_sha256': ROM,
               'resource_files': len(records), 'resource_ids': sorted(resource_ids),
               'resource_ids_hex': [f'{x:02x}' for x in sorted(resource_ids)],
               'resource_groups': grouped(resource_ids),
               'observation_groups': observations_report['groups'],
               'natural_request_count': sum(request_counts.values()),
               'natural_dispatch_count': sum(dispatch_counts.values()),
               'original_isolated_animation_queue_write_count': sum(isolated_counts.values()),
               'prior_private_control_observations': private_control,
               'target_matrix': matrix,
               'sources': list(sources.values()),
               'audit_sha256': digest(Path(__file__).read_bytes()),
               'unresolved': ['Group2 absence is bounded to these resource fields and retained windows, not proof of whole-game unreachability.',
                              '81/89/8b and raw f0..ff have no resource/normal observation in this scope; dispatcher aliases and additional producers require parent code audit.',
                              'Raw natural 8e/90/92/93/96 are not assumed distinct music definitions.',
                              'All catalog animation poses are not proven reachable by normal gameplay.',
                              'Private arbitrary command injection and queue boundary stimuli are deliberately excluded from gameplay reachability.']}
    for name, value in [('resources.json', resources_report), ('observations.json', observations_report), ('summary.json', summary)]:
        (OUT / name).write_text(json.dumps(value, indent=2) + '\n')
    print(json.dumps({'pass': True, 'resource_files': len(records),
                      'resource_ids': summary['resource_ids_hex'],
                      'resource_groups': summary['resource_groups'],
                      'observation_groups': summary['observation_groups'],
                      'isolated_queue_writes': sum(isolated_counts.values())}, indent=2))


if __name__ == '__main__':
    main()
