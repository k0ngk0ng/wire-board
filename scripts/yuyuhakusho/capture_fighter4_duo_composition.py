#!/usr/bin/env python3
"""Read-only complete fighter4 APIs in the fixed two-human 5502-frame scene."""
import argparse
import ctypes as C
import gzip
import hashlib
import json
from pathlib import Path
from reference import ROOT, ReferenceSession, buttons_at, sha
from capture_fighter4_composition import Sink, n
from capture_chu_action import snapshot as action_snapshot
from capture_chu_actor import snapshot as actor_snapshot, view as actor_view
from capture_chu_input import snapshot as input_snapshot, route as input_route
from capture_animation_records import selected_data

INPUT_SHA = 'b303ca8622f90747052902f81b2b7f2bdde39f59e47feb6f36a6bf8287a6ac34'
CONTROL_SHA = '9f282f53a016d184d45052b30eaf8c482d4b568132f7979f7eebff4a7faa5917'


class DuoSink(Sink):
    def __init__(self, path, kind):
        super().__init__(path, kind)
        self.ai, self.record = 0, 0
        self.first_boost, self.last_boost, self.recovered = 0, 0, 0
        self.boost_rows, self.transitions = 0, []
        self.first_recovery = None

    def add(self, row):
        super().add(row)
        self.ai += row['ai_calls']
        self.record += row['record_calls']
        snapshots = [row['before'], row['after']]
        if self.kind == 'actor':
            snapshots = [s['action'] for s in snapshots]
        actors = [bytes.fromhex(s['ai']['actor'] if self.kind == 'input' else s['tail']['actor_bytes']) for s in snapshots]
        old, new = [n(b, 0x90, 4) for b in actors]
        if old != new:
            self.transitions.append({'frame': row['frame'], 'before': hex(old), 'after': hex(new), 'action_before': actors[0][0x5d], 'action_after': actors[1][0x5d]})
        if 0x15946 in [old, new]:
            self.first_boost = self.first_boost or row['frame']
            self.last_boost = row['frame']
            self.boost_rows += 1
        if self.kind != 'input' and self.first_boost and row['frame'] > self.last_boost and old == new == 0x12578 and n(actors[1], 0xb8, 4, True) < 0 and actors[1][0x5d] == 0:
            child = bytes.fromhex(snapshots[1]['children'])
            if n(child, 4) == 0:
                self.recovered += 1
                self.first_recovery = self.first_recovery or row['frame']


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--out', type=Path, required=True)
    args = parser.parse_args()
    out = args.out.resolve()
    assert out.is_relative_to(ROOT)
    out.mkdir(parents=True, exist_ok=True)
    input_path = ROOT / 'docs/research/md-yuyuhakusho/source4-special-discovery-duo-inputs.json'
    reference_path = ROOT / 'games/yuyuhakusho/engine/testdata/fighter4-attacks-duo-reference-frames.json'
    manifest_path = ROOT / 'games/yuyuhakusho/engine/testdata/fighter4-attacks-duo-reference-manifest.json'
    inputs, control = input_path.read_bytes(), reference_path.read_bytes()
    baseline_manifest = json.loads(manifest_path.read_text())
    assert sha(inputs) == INPUT_SHA == baseline_manifest['input']['sha256']
    assert sha(control) == CONTROL_SHA == baseline_manifest['output']['sha256']
    assert baseline_manifest['observer_attached'] is False
    assert baseline_manifest['identity']['core_sha256'] == '1d08b724cf975de17af146f9ba4292946a609b26d03da3d0d9414c90983095da'
    expected, events = json.loads(control), json.loads(inputs)
    assert len(expected) == baseline_manifest['frames'] == 5502
    entry_limit = baseline_manifest['entry_frame_limit']
    assert entry_limit == 5500 and baseline_manifest['drain_frames'] == len(expected) - entry_limit == 2
    core = ROOT / '.local/md-yuyuhakusho/tools/Genesis-Plus-GX-60bee4c724859710ce05672239e5b601ad01382e/genesis_plus_gx_libretro.dylib'
    bridge_path = ROOT / '.local/md-yuyuhakusho/tools/hook_filter.dylib'
    rom = (ROOT / '.local/md-yuyuhakusho/downloads/yuyuhakusho-jp.bin').read_bytes()
    bridge = C.CDLL(str(bridge_path))
    callback_type = C.CFUNCTYPE(None, C.c_int, C.c_int, C.c_uint, C.c_uint)
    bridge.yyh_set_observer.argtypes = [callback_type, C.POINTER(C.c_uint), C.c_size_t]
    bridge.yyh_set_observer.restype = C.c_int
    points = [0x12682, 0x126ba, 0x15540, 0x1269a, 0x15a18, 0x15b78, 0x15b82, 0x3982,
              0x1cc8a, 0x15594, 0x14922, 0x156f0, 0x208d8, 0x15772, 0x37c56, 0x37cac, 0x1e3fe]
    pcs = (C.c_uint * len(points))(*points)
    sinks = {kind: DuoSink(out / ('duo-' + kind + '.jsonl'), kind) for kind in ['action', 'input', 'actor']}
    pending, library, errors = {}, {}, []

    def snapshot(kind, ram, address):
        if kind == 'input':
            return input_snapshot(ram, address, rom, stream_base=0xff6000)
        if kind == 'action':
            value = action_snapshot(ram, address, rom)
            value['global_a038'] = n(ram, 0xa038)
        else:
            value = actor_snapshot(ram, address, rom)
            value['action']['global_a038'] = n(ram, 0xa038)
        return value

    with ReferenceSession(ROOT, out / 'observed', core=core) as ref:
        def finish(kind, ram, address):
            row = pending.pop(kind)
            assert address == 0xff9000
            row.update(after=snapshot(kind, ram, address), exit_frame=ref.frame)
            if kind == 'action':
                assert row['rng_calls'] == 1 and 'animation_key' in row
            assert row['ai_calls'] == 0 and row['record_calls'] == 0
            sinks[kind].add(row)

        @callback_type
        def observe(hook_kind, width, pc, value):
            try:
                address, sp = ref.get_register(13) & 0xffffff, ref.get_register(15)
                if pc in [0x37c56, 0x37cac]:
                    a = ref.get_register(9) & 65535
                    for row in pending.values():
                        row['external_queue_clears'].append({'pc': hex(pc), 'address': hex(a), 'before': ref.ram_cpu()[a], 'sr': ref.get_register(17), 'sp': sp})
                    return
                if pc in [0x3982, 0x1e3fe, 0x1cc8a, 0x15594]:
                    field = {0x3982: 'rng_calls', 0x1e3fe: 'threat_branches', 0x1cc8a: 'ai_calls', 0x15594: 'record_calls'}[pc]
                    for row in pending.values():
                        row[field] += 1
                if 'actor' in pending and pc in [0x14922, 0x15540, 0x15a18, 0x156f0, 0x208d8, 0x15772]:
                    stages = pending['actor']['stages']
                    stages[hex(pc)] = stages.get(hex(pc), 0) + 1
                ram = ref.ram_cpu()
                if pc in [0x12682, 0x15540, 0x15a18] and ram[(address & 65535) + 0x58] == 4 and ref.frame <= entry_limit:
                    assert address == 0xff9000 and ref.get_register(13) == 0xffff9000
                    kind = {0x12682: 'actor', 0x15540: 'input', 0x15a18: 'action'}[pc]
                    assert kind not in pending
                    row = {'object_id': 0x9000, 'source_register': hex(ref.get_register(13)), 'entry_sp': sp,
                           'frame': ref.frame, 'scenario': 'real-menu-source4-two-human-stage32', 'name': 'duo-' + str(ref.frame),
                           'before': snapshot(kind, ram, address), 'external_queue_clears': [], 'rng_calls': 0,
                           'ai_calls': 0, 'record_calls': 0, 'threat_branches': 0}
                    if kind == 'actor':
                        row['view'], row['stages'] = actor_view(ram), {}
                    elif kind == 'input':
                        row['route'] = input_route(row)
                    pending[kind] = row
                elif pc == 0x15b78 and 'action' in pending:
                    assert address == 0xff9000
                    key, data, source = selected_data(rom, ram, ram[0x9000:0x9100])
                    key_id = json.dumps(key, sort_keys=True)
                    assert key_id not in library or library[key_id]['data'] == data
                    library[key_id] = {'key': key, 'data': data, 'source': source}
                    pending['action']['animation_key'] = key
                elif pc == 0x15b82 and 'action' in pending:
                    assert sp == pending['action']['entry_sp']
                    finish('action', ram, address)
                elif pc == 0x1269a and 'input' in pending:
                    assert sp == pending['input']['entry_sp'] + 4
                    finish('input', ram, address)
                elif pc == 0x126ba and 'actor' in pending:
                    assert sp == pending['actor']['entry_sp']
                    finish('actor', ram, address)
            except Exception as error:
                errors.append(repr(error))

        assert bridge.yyh_set_observer(observe, pcs, len(points))
        hook = ref.api('set_cpu_hook', None, C.c_void_p)
        hook(observer := C.cast(bridge.yyh_cpu_hook, C.c_void_p))
        frames, binding = [], None
        for frame in range(1, len(expected) + 1):
            frames.append(ref.step(frame, buttons_at(events, frame))['reference'])
            if frame == 2400:
                ram = ref.ram_cpu()
                assert ram[0x9058] == 4 and ram[0x9076] == 1 and ram[0x90c9] == 0
                assert ram[0x8458] == 3 and ram[0x8476] == 0 and ram[0x84c9] == 0
                binding = {'source_tag': ram[0x9058], 'source_object': '0x9000', 'input_channel': ram[0x9076], 'input_mode': ram[0x90c9],
                           'other_source_tag': ram[0x8458], 'other_source_object': '0x8400', 'other_input_channel': ram[0x8476], 'other_input_mode': ram[0x84c9],
                           'checkpoint_video_label': frame, 'animation_set': n(ram, 0x9050, 4), 'shape_set': n(ram, 0x908c, 4),
                           'parameter_pointer': n(ram, 0x9090, 4), 'source_actor_sha256': sha(ram[0x9000:0x9100])}
        hook(None)
        assert frames == expected and not errors and not pending and binding, (errors[:5], list(pending))
        identity = ref.identity
    assert sha(input_path.read_bytes()) == INPUT_SHA and sha(reference_path.read_bytes()) == CONTROL_SHA
    for kind, sink in sinks.items():
        sink.close()
        assert sink.count > 2000 and sink.boost_rows > 1
        if kind != 'input':
            assert sink.transitions and sink.recovered > 0
        name = 'fighter4-composition-' + kind + '-duo'
        raw_hash = hashlib.sha256()
        path = out / (name + '.json.gz')
        with path.open('wb') as file, gzip.GzipFile(filename='', mode='wb', fileobj=file, mtime=0) as z:
            def write(raw):
                raw_hash.update(raw)
                z.write(raw)
            write(b'{"library":' + json.dumps(list(library.values()), separators=(',', ':')).encode() + b',"calls":[')
            with sink.path.open('rb') as rows:
                for index, row in enumerate(rows):
                    if index:
                        write(b',')
                    write(row.rstrip(b'\n'))
            write(b']}\n')
        statistics = {'calls': sink.count, 'external': sink.external, 'threat_branches': sink.threat, 'ai_calls': sink.ai, 'record_calls': sink.record,
                      'actions': dict(sink.actions), 'routes': dict(sink.routes), 'rng_counts': dict(sink.rng), 'sources': dict(sink.sources)}
        lifecycle = {'first_boost_frame': sink.first_boost, 'last_boost_frame': sink.last_boost, 'boost_rows': sink.boost_rows,
                     'source_transition_rows': sink.transitions, 'first_default_negative_energy_idle_child_empty_frame': sink.first_recovery, 'recovered_rows': sink.recovered}
        report = {'pass': True, 'scope': kind + ' natural source4 two-human complete original API return; excludes world callbacks',
                  'identity': identity, 'capture_sha256': sha(Path(__file__).read_bytes()), 'filter_sha256': sha(bridge_path.read_bytes()),
                  'records': sink.count, 'calls': sink.count, 'frames_match_original_core': frames == expected, 'video_steps': len(expected),
                  'external_queue_calls': sink.external, 'fixture_sha256': raw_hash.hexdigest(), 'gzip_sha256': sha(path.read_bytes()),
                  'binding': binding, 'library_frames': len(library), 'baseline_path': str(reference_path.relative_to(ROOT)), 'baseline_sha256': CONTROL_SHA,
                  'baseline_manifest_sha256': sha(manifest_path.read_bytes()), 'input_path': str(input_path.relative_to(ROOT)), 'input_sha256': INPUT_SHA,
                  'accepted_entry_video_limit': entry_limit, 'drain_video_steps': len(expected) - entry_limit, 'canonical_source_registers': True,
                  'natural_only': True, 'no_ram_writes': True, 'incomplete_calls': 0, 'errors': errors, 'statistics': statistics, 'natural_lifecycle': lifecycle,
                  'limits': ['Two human controls; no natural source4 AI.', 'Rows are independent complete fighter API observations; child/world scheduling is external.',
                             'Only observed audio consumer PC/address markers permit narrow queue exclusions.']}
        (out / (name + '-manifest.json')).write_text(json.dumps(report, indent=2) + '\n')
        print(json.dumps({'name': name, 'records': sink.count, 'statistics': statistics, 'natural_lifecycle': lifecycle, 'pass': True}))


if __name__ == '__main__':
    main()
