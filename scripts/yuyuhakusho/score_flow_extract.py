"""Research-only finite parser work; never loaded by the production driver."""
from pathlib import Path
import argparse
import copy
import gzip
import hashlib
import json

from reference import ROOT
from music_score_structure import ARG_BYTES

JSON_SHA = '735d012d3d3140e76f8f756f0ca757bdc4b8deeb5a66750dd4610e41e06af830'
FLOW = {0xf6: 'jump', 0xf7: 'loop', 0xf8: 'call', 0xf9: 'return'}
CONTROL = {0xe0: 'pan', 0xe1: 'detune', 0xe4: 'pan-automation', 0xe5: 'level-add',
           0xe6: 'level-add', 0xe7: 'tie', 0xe8: 'gate', 0xea: 'tempo',
           0xeb: 'enqueue-replace-first', 0xec: 'level-add', 0xef: 'fm-patch',
           0xf0: 'modulation', 0xf2: 'track-end', 0xf3: 'psg-noise',
           0xf4: 'pitch-control', 0xf5: 'volume-envelope', 0xfb: 'transpose-add',
           0xfc: 'modulation-enable', 0xfd: 'modulation-enable'}


def sha(raw):
    return hashlib.sha256(raw).hexdigest()


def canonical(value):
    return json.dumps(value, sort_keys=True, separators=(',', ':')).encode()


def parameter_roles(token, channel, first):
    roles = {0xe0: ['stereo'], 0xe1: ['detune'], 0xe6: ['base-level-delta'],
             0xe8: ['gate-remaining-seed', 'gate-definition'],
             0xea: ['tempo-period', 'tempo-remaining-seed'], 0xeb: ['queue-sound'],
             0xec: ['base-level-delta'], 0xef: ['instrument-selection'],
             0xf0: ['modulation-delay', 'modulation-rate', 'modulation-delta', 'modulation-steps'],
             0xf3: ['noise-latch'], 0xf4: ['pitch-control'],
             0xf5: ['volume-envelope' if channel.startswith('psg') else 'instrument-selection'],
             0xfb: ['transpose-delta']}
    if token == 0xe4:
        return ['pan-mode'] + (['pan-pattern', 'pan-start-step', 'pan-limit',
                               'pan-period', 'pan-elapsed-seed'] if first else [])
    if token == 0xe5:
        return ['psg-level-delta'] if channel.startswith('psg') else ['discarded-psg-level', 'fm-level-delta']
    return roles.get(token, [])


class Parser:
    """Source reader for proof only; all mutable source state stays private."""
    def __init__(self, rom, sequence, multiplier, channel):
        self.rom, self.position, self.multiplier, self.channel = rom, sequence, multiplier, channel
        self.duration, self.words, self.stack = 0, [0, 0], 48
        self.pending = []
        self.control_trace, self.read_trace, self.emissions = [], [], []
        self.ended = False

    def source_read(self, address, role):
        if not 0 <= address < len(self.rom):
            raise ValueError('source read outside ROM')
        value = self.rom[address]
        self.read_trace.append({'address': address, 'value': value, 'role': role})
        return value

    def counter(self, index):
        return self.words[index // 4] >> ((3 - index % 4) * 8) & 255

    def store_counter(self, index, value):
        shift = (3 - index % 4) * 8
        self.words[index // 4] = self.words[index // 4] & ~(255 << shift) | (value & 255) << shift

    def next(self):
        if self.ended:
            raise StopIteration
        for _ in range(100000):
            source = self.position
            token = self.source_read(source, 'sequence')
            self.position += 1
            self.pending.append({'kind': 'token-read', 'form': 'control' if token >= 224 else 'pitch' if token >= 128 else 'duration'})
            if token in FLOW:
                action = FLOW[token]
                self.pending.append({'kind': 'control-dispatch', 'action': action})
                before = {'words': list(self.words), 'stack': self.stack}
                work = {'kind': action}
                if token in (0xf6, 0xf8):
                    if token == 0xf8:
                        self.stack -= 4
                        if self.stack < 40:
                            raise ValueError('source shared call words exceeded')
                        self.words[(self.stack - 40) // 4] = source + 1
                    self.position = source + 2 + int.from_bytes(self.rom[source + 1:source + 3], 'big', signed=True)
                elif token == 0xf7:
                    index, count = self.rom[source + 1:source + 3]
                    if index >= 8:
                        raise ValueError('source shared counter leaves declared words')
                    first = self.counter(index) == 0
                    remaining = ((count if first else self.counter(index)) - 1) & 255
                    self.store_counter(index, remaining)
                    taken = remaining != 0
                    work.update(first=first, taken=taken)
                    self.position = source + 4 + int.from_bytes(self.rom[source + 3:source + 5], 'big', signed=True) if taken else source + 5
                else:
                    if self.stack >= 48:
                        raise ValueError('source shared return underflow')
                    self.position = self.words[(self.stack - 40) // 4] + 2
                    self.stack += 4
                self.pending.append(work)
                self.control_trace.append({'address': source, 'token': token, 'next_address': self.position,
                                           'before': before, 'work': copy.deepcopy(work)})
                continue
            if token < 224:
                if token >= 128:
                    form = 'rest' if token == 128 else 'dac' if self.channel == 'dac' else 'chromatic'
                    family = 'dac' if self.channel == 'dac' else 'psg' if self.channel.startswith('psg') else 'fm'
                    self.pending.append({'kind': 'pitch-resolve-site', 'form': form, 'family': family})
                    if self.channel.startswith('psg') or self.channel.startswith('fm') and token == 128:
                        self.pending.append({'kind': 'note-commit-site', 'phase': 'after-pitch'})
                    probe = self.source_read(self.position, 'duration-lookahead')
                    take = probe < 128
                    self.pending.append({'kind': 'duration-probe', 'outcome': 'take' if take else 'reuse'})
                    if take:
                        self.position += 1
                        self.duration = probe * self.multiplier & 255
                        self.pending.append({'kind': 'duration-scale', 'additions': (self.multiplier - 1) & 255})
                else:
                    self.duration = token * self.multiplier & 255
                    self.pending.append({'kind': 'duration-scale', 'additions': (self.multiplier - 1) & 255})
                self.pending.append({'kind': 'note-commit-site', 'phase': 'final'})
                work, self.pending = self.pending, []
                result = {'event_kind': 'note', 'work': work, 'source_address': source,
                          'effective_duration': self.duration or 256}
                self.emissions.append(result)
                return result
            if token == 0xfa:
                self.pending += [{'kind': 'control-dispatch', 'action': 'duration-scale-definition'},
                                 {'kind': 'parameter-read', 'role': 'duration-scale-definition'}]
                self.multiplier = self.rom[self.position]
                self.position += 1
                continue
            if token not in CONTROL:
                raise ValueError(('unrestored retained parser token', token))
            event_kind = CONTROL[token]
            if token == 0xf5 and self.channel.startswith('fm'):
                event_kind = 'fm-patch-select'
            first = self.rom[self.position]
            size = 5 if token == 0xe4 and first else 1 if token == 0xe4 else ARG_BYTES[token]
            self.pending.append({'kind': 'control-dispatch', 'action': event_kind})
            self.pending += [{'kind': 'parameter-read', 'role': role} for role in parameter_roles(token, self.channel, first)]
            if token == 0xe5 and self.channel.startswith('psg'):
                self.pending.append({'kind': 'parameter-skip', 'count': 1})
            self.position += size
            self.control_trace.append({'address': source, 'token': token, 'next_address': self.position,
                                       'before': {'words': list(self.words), 'stack': self.stack}})
            if token == 0xf2:
                self.ended = True
            work, self.pending = self.pending, []
            result = {'event_kind': event_kind, 'work': work, 'source_address': source}
            self.emissions.append(result)
            return result
        raise ValueError('same-parser-advance work budget exceeded')


def extract(out):
    catalog_path = ROOT / 'games/yuyuhakusho/data/audio-score.json.gz'
    raw = gzip.decompress(catalog_path.read_bytes())
    if sha(raw) != JSON_SHA:
        raise ValueError('formal catalog JSON identity changed')
    catalog = json.loads(raw)
    provenance_path = ROOT / '.local/md-yuyuhakusho/audio-score-driver/official-with-fallback/provenance.json'
    origins = json.loads(provenance_path.read_text())
    rom_path = ROOT / '.local/md-yuyuhakusho/downloads/yuyuhakusho-jp.bin'
    rom = rom_path.read_bytes()
    if sha(rom) != 'c39d52b2e2298b3784ada3de70d64b26b67e172114c7d18494faad70fc4d7c06':
        raise ValueError('ROM identity changed')
    records, private, conflicts = [], [], []
    for score in catalog['scores']:
        origin = origins[str(score['id'])]
        multiplier = rom[origin['header_address'] + (4 if score['kind'] == 'music' else 2)]
        for voice, source_voice in zip(score['voices'], origin['voices']):
            parser = Parser(rom, source_voice['sequence'], multiplier, voice['channel'])
            prefix, cycle = voice['timeline']['prefix'], voice['timeline']['cycle']
            total = len(prefix) + len(cycle) * 3
            grouped = {}
            for ordinal in range(total):
                section = 'prefix' if ordinal < len(prefix) else 'cycle'
                index = ordinal if section == 'prefix' else (ordinal - len(prefix)) % len(cycle)
                event = voice['timeline'][section][index]
                actual = parser.next()
                if actual['event_kind'] != event['kind']:
                    raise AssertionError((score['id'], voice['id'], section, index, actual['event_kind'], event['kind']))
                if event['kind'] == 'note' and actual['effective_duration'] != event['duration']:
                    raise AssertionError('resolved duration differs from formal event')
                key = (section, index)
                context = 'prefix' if section == 'prefix' else 'first-cycle' if ordinal < len(prefix) + len(cycle) else 'repeat-cycle'
                variants = grouped.setdefault(key, {})
                identity = sha(canonical(actual['work']))
                if identity not in variants:
                    variants[identity] = {'contexts': [], 'work': actual['work']}
                if context not in variants[identity]['contexts']:
                    variants[identity]['contexts'].append(context)
            for (section, index), values in grouped.items():
                key = {'score_id': score['id'], 'voice_id': voice['id'], 'section': section, 'event_index': index}
                record = {'key': key, 'variants': list(values.values())}
                records.append(record)
                if len(values) > 1:
                    conflicts.append({'key': key, 'variant_count': len(values),
                                      'contexts': [value['contexts'] for value in values.values()]})
            private.append({'score_id': score['id'], 'voice_id': voice['id'],
                            'controls': parser.control_trace, 'reads': parser.read_trace,
                            'emissions': [{'source_address': item['source_address'], 'event_kind': item['event_kind']}
                                          for item in parser.emissions]})
    sidecar = {'schema': 1, 'production_resource': False, 'catalog_json_sha256': JSON_SHA,
               'event_key_unambiguous': not conflicts, 'records': records,
               'unpriced_work': True, 'limits': ['Research variants only; occurrence-sensitive prelude must be resolved before production.',
                'Dynamic claim/tie/gate/parameter/IO bodies and instrument seek costs are outside pure lexical work.']}
    out.mkdir(parents=True, exist_ok=False)
    (out / 'research-sidecar.json').write_text(json.dumps(sidecar, separators=(',', ':')) + '\n')
    (out / 'provenance.json.gz').write_bytes(gzip.compress(canonical(private), mtime=0))
    report = {'pass': True, 'production_resource': False, 'catalog_json_sha256': JSON_SHA,
              'records': len(records), 'voice_tracks': len(private), 'event_key_conflicts': conflicts,
              'source_sha256': {str(path.relative_to(ROOT)): sha(path.read_bytes()) for path in
                               [Path(__file__).resolve(), catalog_path, provenance_path, rom_path]}}
    (out / 'extraction.json').write_text(json.dumps(report, indent=2) + '\n')
    print(json.dumps({'pass': True, 'records': len(records), 'voice_tracks': len(private),
                      'event_key_conflicts': len(conflicts), 'first_conflicts': conflicts[:8]}, indent=2))


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--out', type=Path, required=True)
    args = parser.parse_args()
    target = args.out.resolve()
    if not target.is_relative_to(ROOT / '.local/md-yuyuhakusho/audio-score-flow'):
        raise ValueError('output outside owned flow research scope')
    extract(target)
