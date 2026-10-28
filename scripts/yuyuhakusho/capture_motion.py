#!/usr/bin/env python3
"""Passive original-ROM captures of motion, projection and RNG stages."""
import argparse
from collections import Counter
import ctypes as C
import json
from pathlib import Path
from reference import ROOT, ReferenceSession, buttons_at, sha


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument('--out', required=True, type=Path)
    args = ap.parse_args()
    out = args.out.resolve()
    assert out.is_relative_to(ROOT)
    out.mkdir(parents=True, exist_ok=True)
    events = json.loads((ROOT/'docs/research/md-yuyuhakusho/inputs.json').read_text())
    expected = json.loads((ROOT/'docs/research/md-yuyuhakusho/frame-hashes.json').read_text())
    rom = (ROOT/'.local/md-yuyuhakusho/downloads/yuyuhakusho-jp.bin').read_bytes()
    core = ROOT/'.local/md-yuyuhakusho/tools/Genesis-Plus-GX-60bee4c724859710ce05672239e5b601ad01382e/genesis_plus_gx_libretro.dylib'
    bridge_path = ROOT/'.local/md-yuyuhakusho/tools/hook_filter.dylib'
    bridge = C.CDLL(str(bridge_path))
    cbtype = C.CFUNCTYPE(None, C.c_int, C.c_int, C.c_uint, C.c_uint)
    bridge.yyh_set_observer.argtypes = [cbtype, C.POINTER(C.c_uint), C.c_size_t]
    bridge.yyh_set_observer.restype = C.c_int
    entries = {0x3982:'rng', 0x156f0:'projection', 0x19da2:'walk', 0x19dc6:'air'}
    exits = {0x39aa:'rng', 0x1574e:'projection', 0x19dc2:'walk', 0x19df2:'air', 0x19df6:'air'}
    addresses = list(entries) + list(exits)
    pcs = (C.c_uint * len(addresses))(*addresses)
    rows = {kind: [] for kind in entries.values()}
    pending, errors, side_effects = {}, [], Counter()

    def integer(data, offset, size, signed=False):
        return int.from_bytes(data[offset:offset+size], 'big', signed=signed)

    def motion(obj):
        return {name: integer(obj, offset, 4, True) for name, offset in
                [('x',0x40),('y',0x44),('vx',0x10),('vy',0x14),('gravity',0x4c),('ground',0x9c)]} | {'facing': obj[0x61]}

    def projection(obj, ram):
        return {'world_x':integer(obj,0x40,4,True),'world_y':integer(obj,0x44,4,True),
                'screen_x':integer(obj,8,2,True),'screen_y':integer(obj,12,2,True),
                'offset_x':integer(obj,0x54,2,True),'offset_y':integer(obj,0x56,2,True),
                'camera_x':integer(ram,0xa014,2,True),'camera_y':integer(ram,0xa016,2,True),
                'shake':integer(ram,0xf566,2,True)}

    with ReferenceSession(ROOT,out,core=core) as ref:
        @cbtype
        def observe(kind, width, pc, value):
            try:
                ram = ref.ram_cpu()
                what = entries.get(pc) or exits[pc]
                objaddr = 0 if what == 'rng' else ref.get_register(13) & 0xffffff
                key = (what,objaddr)
                obj = ram[objaddr & 0xffff:(objaddr & 0xffff)+256]
                if pc in entries:
                    if key in pending:
                        raise ValueError(f'Overlapping {key}')
                    record = {'frame':ref.frame,'entry':hex(pc),'object':objaddr}
                    if what == 'rng':
                        record['seed'] = integer(ram,0xff18,4)
                    elif what == 'projection':
                        record['before'] = projection(obj,ram)
                    else:
                        record['before'] = motion(obj)
                        if what == 'walk':
                            pointer = integer(obj,0x90,4) & 0xffffff
                            if pointer+4 > len(rom): raise ValueError('Walk parameters not in ROM')
                            record['speed'] = integer(rom,pointer,4,True)
                            record['speed_source'] = hex(pointer)
                    pending[key] = (record,obj)
                    return
                record, before = pending.pop(key)
                record['exit'], record['exit_frame'] = hex(pc),ref.frame
                if what == 'rng':
                    record['next_seed'] = integer(ram,0xff18,4)
                    record['returned'] = ref.get_register(0)
                else:
                    record['after'] = projection(obj,ram) if what == 'projection' else motion(obj)
                    if what == 'air': record['landing'] = pc == 0x19df6
                    allowed = {'projection':set(range(8,10))|set(range(12,14))|set(range(0x54,0x58)),
                               'walk':set(range(0x40,0x48)),
                               'air':set(range(0x40,0x48))|set(range(0x14,0x18))}[what]
                    changed = [i for i,(a,b) in enumerate(zip(before,obj)) if a != b]
                    unexpected = [i for i in changed if i not in allowed]
                    if unexpected: raise ValueError(f'Unknown {what} object effects: {unexpected}')
                    side_effects[what] += len(changed)
                rows[what].append(record)
            except Exception as exc:
                errors.append(str(exc))
        assert bridge.yyh_set_observer(observe,pcs,len(pcs))
        ref.api('set_cpu_hook',None,C.c_void_p)(C.cast(bridge.yyh_cpu_hook,C.c_void_p))
        frames = [ref.step(f,buttons_at(events,f))['reference'] for f in range(1,3601)]
        ref.api('set_cpu_hook',None,C.c_void_p)(None)
        report = {'scope':'Natural stage captures; air exits before landing constructor and shared action tail',
                  'identity':ref.identity,'filter_sha256':sha(bridge_path.read_bytes()),
                  'capture_sha256':sha(Path(__file__).read_bytes()),'inputs_sha256':sha((ROOT/'docs/research/md-yuyuhakusho/inputs.json').read_bytes()),
                  'frames':3600,'frames_match_original_core':frames == expected,
                  'counts':{k:len(v) for k,v in rows.items()},'air_landing_branches':sum(x['landing'] for x in rows['air']),
                  'object_changed_bytes_audited':dict(side_effects),'incomplete_calls':[x[0] for x in pending.values()],
                  'errors':errors,'pass':frames == expected and not pending and not errors and all(rows.values())}
    fixture = out/'motion-oracle.json'
    fixture.write_text(json.dumps(rows,separators=(',',':'))+'\n')
    report['fixture_sha256'] = sha(fixture.read_bytes())
    (out/'report.json').write_text(json.dumps(report,indent=2)+'\n')
    print(json.dumps(report,indent=2))
    assert report['pass'], 'Motion capture incomplete or changed execution'


if __name__ == '__main__':
    main()
