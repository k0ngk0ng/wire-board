#!/usr/bin/env python3
"""Unique observer-free cold original3602 controls for physical stage selection.

The separate --verify process replays published hashes without creating another
reference array. No save/restore, CPU hook, direct call or emulated-RAM write is
used by either path.
"""
import argparse
import json
from pathlib import Path
import subprocess
import sys

from reference import ROOT, ReferenceSession, buttons_at, sha

CORE_SHA = '1d08b724cf975de17af146f9ba4292946a609b26d03da3d0d9414c90983095da'
CALLBACKS = {2: 0x20ae0, 4: 0x20cb2, 6: 0x20e28, 8: 0x20fb4,
             10: 0x21126, 12: 0x212f8, 14: 0x21412}
DOCS = ROOT/'docs/research/md-yuyuhakusho'
DATA = ROOT/'games/yuyuhakusho/engine/testdata'
MENU_FRAME = 2110
MATCH_FRAME = 2800
SAMPLE_FRAMES = {1650, MENU_FRAME, 2121, 2400, 2650, MATCH_FRAME, 3200, 3600, 3602}


def n(raw, offset, size=2):
    return int.from_bytes(raw[offset:offset+size], 'big')


def paths(arena):
    label = f'{arena:x}'
    return {'input': DOCS/f'arena-{label}-inputs.json',
            'selection': DOCS/f'arena-{label}-selection.json',
            'menu_image': DOCS/f'arena-{label}-selection.png',
            'match_image': DOCS/f'arena-{label}-original.png',
            'output': DATA/f'arena-{label}-reference-frames.json',
            'manifest': DATA/f'arena-{label}-reference-manifest.json',
            'proof': DOCS/f'arena-{label}-coldproof.json'}


def binding(path):
    return {'path': str(path.relative_to(ROOT)), 'sha256': sha(path.read_bytes())}


def write_json(path, value, compact=False):
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(value, separators=(',', ':') if compact else None,
                               indent=None if compact else 2)+'\n')


def inputs(arena):
    # The shared prefix was physically observed from cold boot through ENTRY
    # and CHARACTER SELECT. MD libretro: B=0, A=1, Start=3, U/D/L/R=4/5/6/7.
    events = [
        (180, 185, [3]), (360, 365, [3]), (650, 655, [3]),
        (800, 805, [0]), (910, 912, [5]), (950, 955, [0]),
        (1000, 1060, [7]), (1100, 1104, [1]), (1150, 1154, [0]),
        (1250, 1254, [1]), (1300, 1304, [3]), (1600, 1604, [0]),
    ]
    distance = arena//2 if arena <= 8 else (16-arena)//2
    direction = 7 if arena <= 8 else 6
    events += [(1720+step*100, 1722+step*100, [direction])
               for step in range(distance)]
    events += [
        (2120, 2124, [0]),
        (2750, 2840, [7]), (2900, 2904, [1]), (2950, 2954, [0]),
        (3000, 3004, [8]), (3050, 3054, [4]), (3100, 3190, [6]),
        (3250, 3254, [1]), (3300, 3304, [0]), (3350, 3354, [8]),
        (3400, 3404, [4]), (3450, 3500, [7]), (3550, 3554, [0]),
    ]
    return [{'start': start, 'end': end, 'buttons': buttons, 'port': 0}
            for start, end, buttons in events]


def state(ram):
    return {'main_behavior': n(ram, 0x8000, 4), 'main_stage': n(ram, 0x8006),
            'manager_behavior': n(ram, 0x8040, 4), 'manager_stage': n(ram, 0x8046),
            'menu_input_behavior': n(ram, 0x8300, 4), 'menu_input_stage': n(ram, 0x8306),
            'arena': n(ram, 0xa060), 'arena_behavior': n(ram, 0x8180, 4),
            'arena_stage': n(ram, 0x8186), 'resource_busy': ram[0xf54e]}


def snapshot(ref, frame):
    ram = ref.ram_cpu()
    return {'frame': frame, **state(ram), 'main_container': ram[0x8000:0x8040].hex(),
            'manager_container': ram[0x8040:0x8080].hex(),
            'arena_container': ram[0x8180:0x8200].hex(),
            'header': ram[0xa000:0xa200].hex(),
            'match_characters': list(ram[0xa100:0xa104]),
            'controllers': list(ram[0xa104:0xa108]),
            'colors': list(ram[0xa108:0xa10c]),
            'actors': [{'address': address, 'behavior': n(ram, address, 4),
                        'stage': n(ram, address+6), 'source_tag': ram[address+0x58],
                        'bytes': ram[address:address+0x400].hex()}
                       for address in (0x8400, 0x8800, 0x8c00, 0x9000)],
            'reference': {'frame': frame, 'ram': sha(ref.ram_native()),
                          'video': sha(ref.image), 'dimensions': [ref.width, ref.height]}}


def run(arena, verify=False):
    names = paths(arena)
    if verify:
        expected = json.loads(names['output'].read_text())
        events = json.loads(names['input'].read_text())
        selection = json.loads(names['selection'].read_text())
        wanted_samples = {row['frame']: row for row in selection['samples']}
    else:
        for key in ('input', 'output', 'manifest', 'selection', 'proof'):
            if names[key].exists():
                raise ValueError(f'Refusing to replace existing unique control: {names[key]}')
        events = inputs(arena)
        write_json(names['input'], events)
        expected = None
    core = ROOT/'.local/md-yuyuhakusho/tools/genesis_plus_gx_libretro.dylib'
    assert sha(core.read_bytes()) == CORE_SHA
    rows, samples, transitions = [], [], []
    previous = None
    output = ROOT/'.local/md-yuyuhakusho/menu-controls'/f'arena-{arena:x}-{"verify" if verify else "capture"}'
    with ReferenceSession(ROOT, output, core=core) as ref:
        for frame in range(1, 3603):
            row = ref.step(frame, buttons_at(events, frame))['reference']
            if verify:
                assert row == expected[frame-1], ('cold frame mismatch', frame, row, expected[frame-1])
            else:
                rows.append(row)
            ram = ref.ram_cpu()
            current = state(ram)
            if current != previous:
                if not verify:
                    transitions.append({'frame': frame, **current})
                previous = current
            if frame in SAMPLE_FRAMES:
                sample = snapshot(ref, frame)
                if verify:
                    assert sample == wanted_samples[frame], ('cold sample mismatch', frame)
                else:
                    samples.append(sample)
            if frame == MENU_FRAME:
                assert current['arena'] == arena, ('menu arena', current)
                assert current['manager_behavior'] == 0xdb68, ('stage menu absent', current)
                assert current['menu_input_behavior'] == 0xdcc8, ('stage input absent', current)
                if not verify:
                    ref.screenshot(names['menu_image'])
            if frame == MATCH_FRAME:
                assert current['arena'] == arena, ('match arena', current)
                assert (current['main_behavior'], current['main_stage']) == (0xa01e, 32), current
                assert current['manager_behavior'] == 0xf2a4, current
                assert current['arena_behavior'] == CALLBACKS[arena], current
                assert n(ram, 0x8400, 4) and n(ram, 0x9000, 4), 'Missing match fighters'
                if not verify:
                    ref.screenshot(names['match_image'])
        identity = ref.identity
    if verify:
        proof = {'scope': 'Fresh process cold original replay compared to the unique published control',
                 'identity': identity, 'input': binding(names['input']),
                 'output': binding(names['output']), 'frames_compared': 3602,
                 'ram_hashes_equal': True, 'video_hashes_equal': True,
                 'sample_states_equal': len(wanted_samples), 'observer_attached': False,
                 'checkpoint_restore': False, 'emulated_ram_writes': False,
                 'process_isolation': 'Separate Python process; retro_init/load_game from reset',
                 'pass': True}
        write_json(names['proof'], proof)
        print(json.dumps({'arena': arena, 'verified': True, 'frames': 3602}), flush=True)
        return
    write_json(names['output'], rows, compact=True)
    selection = {'scope': 'Cold original physical training-mode stage selection and match; no independent-engine acceptance',
                 'arena': arena, 'expected_arena_callback': CALLBACKS[arena],
                 'identity': identity, 'input': binding(names['input']),
                 'controller_registry': {'teamplayer': False, 'ports': [513, 513],
                                         'physical_input_port': 0},
                 'menu_frame': MENU_FRAME, 'match_frame': MATCH_FRAME,
                 'observer_attached': False, 'checkpoint_restore': False,
                 'emulated_ram_writes': False, 'direct_cpu_calls': False,
                 'samples': samples, 'transitions': transitions,
                 'screenshots': [binding(names['menu_image']), binding(names['match_image'])]}
    write_json(names['selection'], selection)
    subprocess.run([sys.executable, '-B', str(Path(__file__).resolve()), '--arena', f'{arena:x}', '--verify'],
                   cwd=ROOT, check=True)
    proof = json.loads(names['proof'].read_text())
    assert proof['pass'] and proof['frames_compared'] == 3602
    manifest = {'scope': f'Unique unobserved original arena {arena:x} real-menu3602 control',
                'identity': identity, 'input': binding(names['input']),
                'output': binding(names['output']), 'frames': len(rows),
                'entry_frame_limit': 3600, 'drain_frames': 2, 'observer_attached': False,
                'capture': binding(Path(__file__).resolve()),
                'capture_sha256': sha(Path(__file__).read_bytes()),
                'selection_discovery': binding(names['selection']),
                'coldproof': binding(names['proof']), 'cold_replay_verified': True}
    write_json(names['manifest'], manifest)
    print(json.dumps({'arena': arena, 'input': binding(names['input']),
                      'output': binding(names['output']), 'manifest': binding(names['manifest'])}), flush=True)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--arena', required=True, type=lambda value: int(value, 16), choices=CALLBACKS)
    parser.add_argument('--verify', action='store_true')
    args = parser.parse_args()
    run(args.arena, args.verify)


if __name__ == '__main__':
    main()
