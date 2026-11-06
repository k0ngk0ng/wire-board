"""Research logical-entry variants derived from the existing event cursor."""
from pathlib import Path
import argparse
import copy
import hashlib
import json


FIELDS = {
    'token-read': {'kind', 'form'}, 'control-dispatch': {'kind', 'action'},
    'jump': {'kind'}, 'call': {'kind'}, 'return': {'kind'},
    'loop': {'kind', 'first', 'taken'}, 'parameter-read': {'kind', 'role'},
    'pitch-resolve-site': {'kind', 'form', 'family'},
    'duration-probe': {'kind', 'outcome'}, 'duration-scale': {'kind', 'additions'},
    'note-commit-site': {'kind', 'phase'},
}
ROLES = {'instrument-selection', 'modulation-delay', 'modulation-rate', 'modulation-delta',
         'modulation-steps', 'stereo', 'transpose-delta', 'base-level-delta', 'volume-envelope',
         'noise-latch', 'gate-remaining-seed', 'gate-definition', 'detune', 'pan-mode',
         'pan-pattern', 'pan-start-step', 'pan-limit', 'pan-period', 'pan-elapsed-seed',
         'tempo-period', 'tempo-remaining-seed', 'queue-sound'}
ACTIONS = {'fm-patch', 'modulation', 'pan', 'transpose-add', 'level-add', 'tie',
           'volume-envelope', 'psg-noise', 'gate', 'detune', 'pan-automation',
           'modulation-enable', 'tempo', 'track-end', 'enqueue-replace-first',
           'jump', 'loop', 'call', 'return', 'fm-patch-select'}


def validate(work):
    if not isinstance(work, list) or len(work) > 12:
        raise ValueError('unknown Work sequence or profile operation bound')
    for step in work:
        if not isinstance(step, dict) or step.get('kind') not in FIELDS or set(step) != FIELDS[step['kind']]:
            raise ValueError('unknown Work or source-bearing extra fields')
        kind = step['kind']
        if kind == 'token-read' and step['form'] not in ('control', 'pitch', 'duration'):
            raise ValueError('unknown token form')
        if kind == 'control-dispatch' and step['action'] not in ACTIONS:
            raise ValueError('unknown dispatch family')
        if kind == 'loop' and (type(step['first']) is not bool or type(step['taken']) is not bool):
            raise ValueError('invalid finite loop branch')
        if kind == 'parameter-read' and step['role'] not in ROLES:
            raise ValueError('unknown parameter role')
        if kind == 'pitch-resolve-site' and (step['form'] not in ('rest', 'dac', 'chromatic') or step['family'] not in ('dac', 'psg', 'fm')):
            raise ValueError('unknown pitch site')
        if kind == 'duration-probe' and step['outcome'] not in ('take', 'reuse'):
            raise ValueError('unknown duration branch')
        if kind == 'duration-scale' and (type(step['additions']) is not int or step['additions'] not in (0, 1)):
            raise ValueError('duration repeats outside the bound formal profile')
        if kind == 'note-commit-site' and step['phase'] not in ('after-pitch', 'final'):
            raise ValueError('unknown note commit site')


def convert(data):
    result = copy.deepcopy(data)
    for record in result['records']:
        key = record['key']
        variants = {}
        used = {}
        for variant in record['variants']:
            work = variant['work']
            validate(work)
            identity = hashlib.sha256(json.dumps(work, sort_keys=True, separators=(',', ':')).encode()).hexdigest()
            target = variants.setdefault(identity, {'entries': [], 'work': work})
            for context in variant['contexts']:
                entry = 'continue'
                if key['section'] == 'cycle' and key['event_index'] == 0:
                    if context not in ('first-cycle', 'repeat-cycle'):
                        raise ValueError('unknown cycle-entry context')
                    entry = 'from-prefix' if context == 'first-cycle' else 'cycle-wrap'
                if entry in used and used[entry] != identity:
                    raise ValueError('existing logical entry cannot determine finite parser work')
                used[entry] = identity
                if entry not in target['entries']:
                    target['entries'].append(entry)
        record['variants'] = list(variants.values())
    result['logical_entry_contract'] = ['continue', 'from-prefix', 'cycle-wrap']
    result['entry_owner'] = 'Temporary nextEvent transition; never a saved cycle count or source stack.'
    return result


def select(record, entry):
    if entry not in ('continue', 'from-prefix', 'cycle-wrap'):
        raise ValueError('unknown logical entry')
    found = [variant for variant in record['variants'] if entry in variant['entries']]
    if len(found) != 1:
        raise ValueError('entry does not identify exactly one finite work sequence')
    validate(found[0]['work'])
    return copy.deepcopy(found[0]['work'])


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--input', type=Path, required=True)
    parser.add_argument('--out', type=Path, required=True)
    args = parser.parse_args()
    args.out.write_text(json.dumps(convert(json.loads(args.input.read_text())), separators=(',', ':')) + '\n')
