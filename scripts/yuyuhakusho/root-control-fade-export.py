"""Extract native fade counter decisions; never computes golden counters.

All before/after values come from canonical-v2 original observations. Actions
are named from the observed exit boundary. Fixed write ordering is bound to
the original source templates, not to a Python/Go fade rule implementation.
"""
import argparse
import gzip
import hashlib
import json
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
RAW = ROOT / '.local/md-yuyuhakusho/root-control-canonical-v2'
FIELDS = ['Priority', 'Selected', 'FadeSteps', 'FadeRemaining', 'ChoiceBits']
EXITS = {0x3894c: 'none', 0x389a6: 'fade-music-step', 0x38a2c: 'request-full-clear'}
STARTS = {0x38916: 1, 0x38928: 5}

def sha(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()

def load(path):
    raw = path.read_bytes()
    return json.loads(gzip.decompress(raw) if path.suffix == '.gz' else raw)

def root5(root):
    return {key: root[key] for key in FIELDS}

def fade(root):
    return {'steps': root['FadeSteps'], 'remaining': root['FadeRemaining']}

def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--out', type=Path, required=True)
    parser.add_argument('--provenance', type=Path, required=True)
    args = parser.parse_args()
    target, provenance_path = args.out.resolve(), args.provenance.resolve()
    assert target.parent == ROOT / 'games/yuyuhakusho/audio/rootcontrol/testdata' and target.name.startswith('fade-')
    assert provenance_path.is_relative_to(ROOT / 'games/yuyuhakusho/verification/root-control-fade')
    capture_path = RAW / 'capture.json'
    capture = load(capture_path)
    assert capture['pass'] and len(capture['cases']) == 5
    rompath = ROOT / '.local/md-yuyuhakusho/downloads/yuyuhakusho-jp.bin'
    rom = rompath.read_bytes()
    templates = {'01': (0x38916, '11fc0003f80611fc0028f8044238f8404e75'),
                 '05': (0x38928, '11fc0020f80611fc0032f8044238f8404e75'),
                 'advance': (0x3893a, '70001038f804670a1038f80667065338f8064e755338f804670000d811fc0003f806')}
    for _, (address, expected) in templates.items():
        assert rom[address:address + len(bytes.fromhex(expected))].hex() == expected
    streams, lineage, bindings = [], [], []
    decisions = {'advance': 0, 'start': 0}
    actions = {'none': 0, 'stop-dac-flags': 0, 'fade-music-step': 0, 'request-full-clear': 0}
    for source in capture['cases']:
        observed_path = ROOT / source['artifact']['path']
        control_path = RAW / (source['id'] + '-control.json.gz')
        assert sha(observed_path) == source['artifact']['sha256'] and sha(control_path) == source['control_sha256']
        observed, control = load(observed_path), load(control_path)
        assert not observed['observer_errors'] and observed['frames'] == control['frames']
        events, frames = observed['events'], observed['frames']
        output_frames = [{'step': row['step'], 'operations': [], 'fade_after': fade(row['root'])} for row in frames]
        assert all(row['step'] == index for index, row in enumerate(frames))
        for index, event in enumerate(events):
            pc = event['pc']
            if pc != 0x3893a and pc not in STARTS:
                continue
            before = root5(event['root'])
            if pc in STARTS:
                after = root5(frames[event['step']]['root'])
                assert not any(later['pc'] == 0x3893a and later['step'] == event['step'] for later in events[index + 1:])
                action = 'stop-dac-flags'
                writes = [{'counter':'remaining','value':after['FadeRemaining']}, {'counter':'steps','value':after['FadeSteps']}]
                operation = {'kind':'start','command':STARTS[pc],'before':before,'after':after,'writes':writes,'action':action,
                             'DAC_before_flags':bytes.fromhex(event['driver_hex'])[0x40], 'DAC_after_flags':bytes.fromhex(frames[event['step']]['driver_hex'])[0x40]}
                after_reference = {'frame':event['step'],'json_pointer':f"/frames/{event['step']}/root"}
            else:
                found = next((offset for offset in range(index + 1, len(events)) if events[offset]['pc'] in EXITS or events[offset]['pc'] == 0x3893a), None)
                assert found is not None and events[found]['pc'] in EXITS, (source['id'],index)
                exit = events[found]
                assert exit['step'] == event['step'] and exit['tick'] == event['tick']
                after = root5(exit['root'])
                action = EXITS[exit['pc']]
                writes = [{'counter':counter,'value':after[field]} for counter,field in [('steps','FadeSteps'),('remaining','FadeRemaining')] if before[field] != after[field]]
                operation = {'kind':'advance','before':before,'after':after,'writes':writes,'action':action}
                after_reference = {'event':found,'pc':exit['pc'],'json_pointer':f'/events/{found}/root'}
            assert all(before[key] == after[key] for key in ['Priority','Selected','ChoiceBits']), (source['id'],index)
            step_ops = output_frames[event['step']]['operations']
            lineage.append({'case':source['id'],'step':event['step'],'operation':len(step_ops),'before_event':index,'before_pc':pc,'after':after_reference})
            step_ops.append(operation)
            decisions[operation['kind']] += 1
            actions[action] += 1
        streams.append({'id':source['id'],'initial':fade(events[0]['root'] if 'root' in events[0] else frames[0]['root']),'frames':output_frames})
        bindings += [{'path':str(path.relative_to(ROOT)),'sha256':sha(path)} for path in [observed_path,control_path]]
    assert sum(len(stream['frames']) for stream in streams) == 785
    assert decisions == {'advance':744,'start':2}
    assert actions == {'none':654,'stop-dac-flags':2,'fade-music-step':88,'request-full-clear':2}
    target.parent.mkdir(parents=True, exist_ok=True)
    raw = (json.dumps({'schema':1,'format':'native-fade-counter-streams-v1','scope':'Pure two-counter decisions and ordered semantic writes; no score mutation, full reset or runtime orchestration','streams':streams},separators=(',',':'))+'\n').encode()
    target.write_bytes(gzip.compress(raw,mtime=0))
    provenance_path.parent.mkdir(parents=True, exist_ok=True)
    provenance = {'pass':True,'no_new_CPU_execution':True,'native_identity':capture['identity'],'native_frames_original_observed_equal':785,
                  'streams':5,'frames':785,'decisions':decisions,'actions':actions,'lineage':lineage,'sources':bindings,
                  'capture':{'path':str(capture_path.relative_to(ROOT)),'sha256':sha(capture_path)},
                  'source_templates':{name:{'address':at,'bytes':raw} for name,(at,raw) in templates.items()},
                  'source_sha256':{str(path.relative_to(ROOT)):sha(path) for path in [Path(__file__).resolve(),rompath]},
                  'golden_values_generated_by_fade_model':False,'terminal_expected_is_reset_entry_before_full_clear':True,
                  'output':{'path':str(target.relative_to(ROOT)),'gzip_sha256':sha(target),'json_sha256':hashlib.sha256(raw).hexdigest()},
                  'runtime_fixture_contains_source_PC_or_RAM':False,
                  'limits':['This pure-counter fixture does not verify score.ApplyControl, full04 reset, pause payload, chip IO or whole driver orchestration.','Pause stream contains idle fade state only; active-fade pause orchestration is outside this profile.','Native PC and frame references are retained only in this extraction provenance.']}
    provenance_path.write_text(json.dumps(provenance,indent=2)+'\n')
    manifest = {'source':{'path':str(provenance_path.relative_to(ROOT)),'sha256':sha(provenance_path)},'output':provenance['output'],'streams':5,'frames':785,'decisions':decisions,'actions':actions,'producer_sha256':sha(Path(__file__).resolve())}
    target.with_name('fade-native-manifest.json').write_text(json.dumps(manifest,indent=2)+'\n')
    print(json.dumps({'pass':True,'streams':5,'frames':785,'decisions':decisions,'actions':actions,'fixture_sha256':sha(target)},indent=2))

if __name__ == '__main__':
    main()
