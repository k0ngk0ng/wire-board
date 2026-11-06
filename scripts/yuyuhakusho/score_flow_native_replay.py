"""Reuse saved native parser reads and controls without executing any CPU."""
from pathlib import Path
import argparse
import collections
import gzip
import json

from reference import ROOT
from score_flow_extract import Parser, JSON_SHA, sha


def load(path):
    raw = path.read_bytes()
    return json.loads(gzip.decompress(raw) if path.suffix == '.gz' else raw)


def run(out):
    catalog_path = ROOT / 'games/yuyuhakusho/data/audio-score.json.gz'
    raw = gzip.decompress(catalog_path.read_bytes())
    assert sha(raw) == JSON_SHA
    library = json.loads(raw)
    origin_path = ROOT / '.local/md-yuyuhakusho/audio-score-driver/official-with-fallback/provenance.json'
    origins = load(origin_path)
    index_path = ROOT / '.local/md-yuyuhakusho/audio-score-source/full/index.json'
    index = load(index_path)
    rom_path = ROOT / '.local/md-yuyuhakusho/downloads/yuyuhakusho-jp.bin'
    rom = rom_path.read_bytes()
    assert sha(rom) == index['rom_sha256']
    counts, flows, loop_branches = collections.Counter(), collections.Counter(), collections.Counter()
    cases, suffixes = [], []
    for score, case in zip(library['scores'], index['cases']):
        assert score['id'] == case['code']
        path = ROOT / case['files']['events']['path']
        assert sha(path.read_bytes()) == case['files']['events']['sha256']
        events = load(path)['events']
        initialized = [event for event in events if event['kind'] == 'voice-init']
        assert len(initialized) == len(score['voices'])
        row_to_voice = {event['row']: event['voice_ordinal'] for event in initialized}
        groups = {voice['id']: collections.defaultdict(list) for voice in score['voices']}
        for event in events:
            if event['kind'] == 'tick-before':
                counts['native_driver_ticks'] += 1
            if event['kind'] in ('token-read', 'control-before', 'control-after', 'note-outcome'):
                groups[row_to_voice[event['row']]][event['kind']].append(event)
        origin = origins[str(score['id'])]
        multiplier = rom[origin['header_address'] + (4 if score['kind'] == 'music' else 2)]
        for voice, source in zip(score['voices'], origin['voices']):
            group = groups[voice['id']]
            reads = group['token-read']
            parser = Parser(rom, source['sequence'], multiplier, voice['channel'])
            while len(parser.read_trace) < len(reads):
                parser.next()
            expected = parser.read_trace[:len(reads)]
            actual = [{'address': event['read_address'], 'value': bytes.fromhex(event['read_hex'])[0],
                       'role': event['role']} for event in reads]
            assert actual == expected, (score['id'], voice['id'], 'native lexical read mismatch')
            if len(parser.read_trace) != len(reads):
                suffixes.append({'score_id': score['id'], 'voice_id': voice['id'], 'kind': 'read suffix'})
            before = group['control-before']
            assert len(parser.control_trace) == len(before), (score['id'], voice['id'], 'control boundary count')
            expected_after = []
            for control, native in zip(parser.control_trace, before):
                assert control['address'] == native['command_address'] and control['token'] == native['command']
                state = bytes.fromhex(native['row_hex'])
                assert control['before']['words'] == [int.from_bytes(state[40:44], 'big'), int.from_bytes(state[44:48], 'big')]
                assert control['before']['stack'] == state[13]
                counts['shared_words_and_stack_comparisons'] += 1
                work = control.get('work')
                if work:
                    flows[work['kind']] += 1
                    if work['kind'] == 'loop':
                        selection, initial = bytes.fromhex(native['read_hex'])[:2]
                        first = state[40 + selection] == 0
                        remaining = ((initial if first else state[40 + selection]) - 1) & 255
                        assert work['first'] == first and work['taken'] == (remaining != 0)
                        loop_branches[('first-' if first else 'repeat-') + ('taken' if remaining else 'exit')] += 1
                if control['token'] != 0xf2:
                    expected_after.append(control['next_address'])
            assert [event['next_read_address'] for event in group['control-after']] == expected_after
            commit_sites = sum(step['kind'] == 'note-commit-site' for event in parser.emissions for step in event['work'])
            assert commit_sites == len(group['note-outcome']), (score['id'], voice['id'], 'native note cleanup/commit count')
            counts['native_token_reads'] += len(reads)
            counts['native_control_before'] += len(before)
            counts['native_control_after'] += len(group['control-after'])
            counts['native_note_commits'] += len(group['note-outcome'])
            counts['voice_tracks'] += 1
        counts['definitions'] += 1
        cases.append({'id': score['id'], 'event_input_sha256': case['files']['events']['sha256'], 'pass': True})
        print(json.dumps({'native_reused': score['id'], 'tracks': len(score['voices'])}), flush=True)
    report = {'pass': True, 'new_cpu_capture': False, 'catalog_json_sha256': JSON_SHA,
              **dict(counts), 'flow_counts': dict(flows), 'loop_branch_counts': dict(loop_branches),
              'partial_native_suffixes': suffixes, 'cases': cases,
              'source_sha256': {str(path.relative_to(ROOT)): sha(path.read_bytes()) for path in
                               [Path(__file__).resolve(), ROOT / 'scripts/yuyuhakusho/score_flow_extract.py',
                                catalog_path, origin_path, index_path, rom_path]},
              'limits': ['Parameter-read roles/duplicate reads are static-instruction evidence, not new per-argument native observations.',
                         'Native equality is ordered lexical reads, flow state/branches, next parser positions and commit sites; no priced timing claim.']}
    out.write_text(json.dumps(report, indent=2) + '\n')
    print(json.dumps({key: value for key, value in report.items() if key not in ('cases', 'source_sha256')}, indent=2))


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--out', type=Path, required=True)
    args = parser.parse_args()
    target = args.out.resolve()
    if not target.is_relative_to(ROOT / '.local/md-yuyuhakusho/audio-score-flow'):
        raise ValueError('output outside owned flow research')
    run(target)
