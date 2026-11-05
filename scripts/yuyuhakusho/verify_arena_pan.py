#!/usr/bin/env python3
"""Seven-arena physical Held -> source7 15094 -> dynamic-pan acceptance."""
import argparse
from concurrent.futures import ThreadPoolExecutor, as_completed
import hashlib
import json
import os
from pathlib import Path
import subprocess

ROOT = Path(__file__).resolve().parents[2]
LOCAL = ROOT/'.local/md-yuyuhakusho'
OUT = ROOT/'games/yuyuhakusho/verification/additional-arena-pan'
ARENAS = (2, 4, 6, 8, 10, 12, 14)


def sha(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def write(path, value):
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(value, indent=2)+'\n')


def require_case(value, arena):
    assert value['pass'] and value['arena'] == arena
    assert value['players'] == 2 and value['characters'] == [7, 9]
    assert not value['reference_state_loaded'] and value['human_auto_takeovers'] == 0
    assert 0 < value['pan_child_first_tick'] < value['first_pan_tick'] < value['ticks']
    for field in ('pan_frames', 'max_abs_pan', 'feedback_frames', 'restored_pan_frames',
                  'restored_pixel_frames', 'restored_state_frames', 'pause_pan_frames',
                  'sprite_frames', 'pan_save_tick'):
        assert value[field] > 0, (arena, field, value)
    assert value['hud_window_complete'] and value['rgba_frames_complete']
    assert value['rules_match_runtime'] and value['feedback_stream_gaps_preserved']
    assert value['pan_returned_to_zero']
    assert value['pan_child_cleared'] and value['pan_save_tick'] == value['first_pan_tick']
    assert 0 < value['damage_gate_tick'] < value['command_tick'] < value['pan_child_first_tick']
    assert 0 < value['damage_gate_health'] <= 111 and 0 < value['command_health'] <= 111
    assert value['command_energy'] >= 200
    if arena == 10:
        assert value['arena_sprite_requests'] >= 6*value['ticks']
    for field in ('held_trace_sha256', 'rules_trace_sha256', 'state_trace_sha256',
                  'pixel_trace_sha256', 'final_state_sha256'):
        assert len(value[field]) == 64, (arena, field)
    assert set(value['artifacts']) == {'held-inputs.json', 'trace.jsonl.gz', 'first-pan.png', 'pan-save.json.gz'}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('environment', choices=['native', 'wasm'])
    parser.add_argument('--red', action='store_true')
    parser.add_argument('--workers', type=int, default=2)
    parser.add_argument('--arena', type=lambda value: int(value, 0), choices=ARENAS)
    args = parser.parse_args()
    assert 1 <= args.workers <= 3
    for path in (LOCAL/'tmp', LOCAL/'build', OUT/args.environment):
        path.mkdir(parents=True, exist_ok=True)
    env = dict(os.environ, GOCACHE=str(ROOT/'.local/go-cache'), GOMODCACHE=str(ROOT/'.local/go-mod'),
               GOTMPDIR=str(LOCAL/'tmp'), TMPDIR=str(LOCAL/'tmp'), GOTELEMETRY='off')
    if args.environment == 'wasm':
        # The Wasm wrapper receives no inherited Node/Go behavior flags.
        env = {key: env[key] for key in ('PATH', 'GOCACHE', 'GOMODCACHE', 'GOTMPDIR', 'TMPDIR', 'GOTELEMETRY')}
        env.update(GOOS='js', GOARCH='wasm')
    binary = LOCAL/'build'/('verify-arena-pan'+('.wasm' if args.environment == 'wasm' else ''))
    build = ['go', 'build', '-o', str(binary), './games/yuyuhakusho/cmd/verify-arena-pan']
    result = subprocess.run(build, cwd=ROOT, env=env, text=True, capture_output=True)
    if args.red:
        report = {'test_first': True, 'command': build, 'exit_code': result.returncode,
                  'stdout': result.stdout, 'stderr': result.stderr,
                  'expected_missing_harness': result.returncode != 0 and 'verify-arena-pan' in result.stderr,
                  'script_sha256': sha(Path(__file__).resolve())}
        write(OUT/'missing-harness-red.json', report)
        assert report['expected_missing_harness']
        print('Missing independent dynamic-pan harness red retained.', flush=True)
        return
    if result.returncode:
        raise RuntimeError(result.stderr)
    command = [str(binary)]
    if args.environment == 'wasm':
        goroot = subprocess.check_output(['go', 'env', 'GOROOT'], cwd=ROOT, env=env, text=True).strip()
        command.insert(0, str(Path(goroot)/'lib/wasm/go_js_wasm_exec'))

    def run_one(arena):
        label = f'arena-{arena:x}'
        output = OUT/args.environment/label
        input_path = OUT/'native'/label/'held-inputs.json'
        discovery = None
        if args.environment == 'native':
            discovery_out = LOCAL/'arena-pan-discovery'/label
            adaptive_command = command+['-arena', str(arena), '-output', str(discovery_out)]
            found = subprocess.run(adaptive_command, cwd=ROOT, env=env, text=True, capture_output=True)
            if found.returncode:
                failure = {'command': adaptive_command, 'exit_code': found.returncode, 'stdout': found.stdout, 'stderr': found.stderr}
                write(OUT/'native'/(label+'-discovery-failure.json'), failure)
                raise RuntimeError(found.stderr[-3000:])
            discovery = json.loads(found.stdout)
            require_case(discovery, arena)
            input_path = discovery_out/'held-inputs.json'
        assert input_path.is_file(), 'Wasm requires the successful fixed native input fixture'
        replay_command = command+['-arena', str(arena), '-inputs', str(input_path), '-output', str(output)]
        replay = subprocess.run(replay_command, cwd=ROOT, env=env, text=True, capture_output=True)
        if replay.returncode:
            failure = {'command': replay_command, 'exit_code': replay.returncode, 'stdout': replay.stdout, 'stderr': replay.stderr}
            write(OUT/args.environment/(label+'-replay-failure.json'), failure)
            raise RuntimeError(replay.stderr[-3000:])
        value = json.loads(replay.stdout)
        require_case(value, arena)
        if discovery is not None:
            assert discovery == value, (arena, 'fixed cold replay differs from discovery')
        else:
            native = json.loads((OUT/'native'/(label+'.json')).read_text())
            native.pop('environment')
            comparable = value.copy()
            comparable.pop('environment')
            assert native == comparable, (arena, 'native/Wasm complete legal-input trajectory differs')
        for name, digest in value['artifacts'].items():
            assert sha(output/name) == digest, (arena, name, 'artifact digest mismatch')
        path = OUT/args.environment/(label+'.json')
        write(path, value)
        return {'arena': arena, 'path': str(path.relative_to(ROOT)), 'sha256': sha(path),
                'ticks': value['ticks'], 'pan_frames': value['pan_frames'],
                'feedback_frames': value['feedback_frames'], 'restored_pan_frames': value['restored_pan_frames'],
                'restored_pixel_frames': value['restored_pixel_frames'], 'pass': True}

    selected = [args.arena] if args.arena is not None else ARENAS
    checks = []
    with ThreadPoolExecutor(max_workers=args.workers) as pool:
        futures = {pool.submit(run_one, arena): arena for arena in selected}
        for future in as_completed(futures):
            check = future.result()
            checks.append(check)
            print(f"{args.environment} arena {check['arena']:x}: {check['pan_frames']} pan frames, "
                  f"{check['feedback_frames']} feedback, {check['restored_pixel_frames']} restored pixels", flush=True)
    if len(checks) == 7:
        sources = sorted((ROOT/'games/yuyuhakusho/engine').glob('*.go'))
        sources += sorted((ROOT/'games/yuyuhakusho/runtimebridge').glob('*.go'))
        sources += sorted((ROOT/'games/yuyuhakusho/presentation').glob('*.go'))
        sources += [ROOT/'games/yuyuhakusho/cmd/verify-arena-pan/main.go', Path(__file__).resolve()]
        sources += [ROOT/'docs/research/md-yuyuhakusho/source7-special-discovery.json', ROOT/'docs/research/md-yuyuhakusho/source7-special-discovery-inputs.json']
        report = {'pass': True, 'environment': args.environment, 'scenarios': 7,
                  'checks': sorted(checks, key=lambda row: row['arena']),
                  'every_arena_has_legal_dynamic_pan': True, 'reference_state_loaded': False,
                  'fixed_held_replay_verified': True, 'native_wasm_complete_traces_equal': args.environment == 'wasm',
                  'source_sha256': {str(path.relative_to(ROOT)): sha(path) for path in sources}}
        write(OUT/args.environment/'validation.json', report)


if __name__ == '__main__':
    main()
