#!/usr/bin/env python3
"""Audit saved arena A raw sprites and tile ownership without running a core."""
import argparse
from collections import defaultdict
import gzip
import hashlib
import json
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
ROM_SHA = 'c39d52b2e2298b3784ada3de70d64b26b67e172114c7d18494faad70fc4d7c06'
CORE_SHA = 'e87739d0804e62423fda890377fa46a8932fe1a6fa0132b1ad7f13b8b3af7306'
AUX_SHA = '43993f1d01e867cbd11750e7526ec6ce33e8d395f3445ff9b0643d47ebbb9343'


def require(condition, message):
    if not condition:
        raise ValueError(message)


def digest(raw):
    return hashlib.sha256(raw).hexdigest()


def inside(path):
    path = Path(path).resolve()
    require(path.is_relative_to(ROOT), 'audit path must stay inside the worktree')
    return path


def load_source(path, manifest_path, gzip_key, payload_key):
    path, manifest_path = inside(path), inside(manifest_path)
    zipped, manifest_raw = path.read_bytes(), manifest_path.read_bytes()
    manifest = json.loads(manifest_raw)
    raw = gzip.decompress(zipped)
    require(manifest['pass'], 'source manifest is not passing')
    require(digest(zipped) == manifest[gzip_key], 'source gzip digest mismatch')
    require(digest(raw) == manifest[payload_key], 'source payload digest mismatch')
    return json.loads(raw), manifest, {
        'path': str(path.relative_to(ROOT)), 'gzip_sha256': digest(zipped), 'payload_sha256': digest(raw),
        'manifest_path': str(manifest_path.relative_to(ROOT)), 'manifest_sha256': digest(manifest_raw)}


def word(raw, offset):
    return int.from_bytes(raw[offset:offset+2], 'big')


def ranges(slots):
    result = []
    for slot in sorted(slots):
        if result and result[-1][1] == slot:
            result[-1][1] += 1
        else:
            result.append([slot, slot+1])
    return result


def main():
    p = argparse.ArgumentParser()
    p.add_argument('--reference', type=Path, default=ROOT/'.local/md-yuyuhakusho/additional-arena-assets/references/arena-assets-a-reference.json.gz')
    p.add_argument('--out', type=Path, default=ROOT/'games/yuyuhakusho/data/arena-a-aux-order-audit.json')
    args = p.parse_args()
    data, tests = ROOT/'games/yuyuhakusho/data', ROOT/'games/yuyuhakusho/engine/testdata'
    reference, rm, rs = load_source(args.reference, data/'arena-assets-a-reference-manifest.json', 'gzip_sha256', 'fixture_sha256')
    natural, nm, ns = load_source(tests/'arena-a-natural.json.gz', tests/'arena-a-natural-manifest.json', 'gzip_sha256', 'fixture_sha256')
    aux, am, ass = load_source(data/'arena-a-aux-assets.json.gz', data/'arena-a-aux-assets-manifest.json', 'gzip_sha256', 'catalog_sha256')
    sprites, sm, ss = load_source(data/'world-sprites.json.gz', data/'world-sprites-manifest.json', 'catalog_gzip_sha256', 'catalog_sha256')
    oracle, _, os = load_source(data/'world-sprites-oracle.json.gz', data/'world-sprites-manifest.json', 'oracle_gzip_sha256', 'oracle_sha256')
    sources = {'arena_reference': rs, 'raw_natural': ns, 'aux_catalog': ass, 'sprite_catalog': ss, 'sprite_oracle': os}
    for manifest in [rm, nm, sm]:
        require(manifest['identity']['rom_sha256'] == ROM_SHA and manifest['identity']['core_sha256'] == CORE_SHA, 'reference target identity differs')
    require(aux['rom_sha256'] == sprites['rom_sha256'] == oracle['rom_sha256'] == ROM_SHA, 'asset target differs')
    require((aux['arena_offset'], aux['scene_behavior'], aux['callback'], aux['resource_source']) == (10, 0x21126, 0x21258, 0x297056), 'aux identity differs')
    require(rm['original_control']['output_sha256'] == nm['control_sha256'] and rm['original_control']['input_sha256'] == nm['inputs_sha256'], 'arena references use different controls')
    require(am['reference_sha256'] == rs['payload_sha256'] and am['reference_gzip_sha256'] == rs['gzip_sha256'], 'aux reference binding differs')
    tiles = aux['packed_4bpp_tiles']
    require(len(tiles) == 48 and all(len(bytes.fromhex(t)) == 32 for t in tiles), 'aux is not 48 packed tiles')
    bank = b''.join(bytes.fromhex(t) for t in tiles)
    require(digest(bank) == am['decoded_sha256'] == AUX_SHA, 'aux decoded bytes differ')

    children = [c for c in natural['calls'] if c['kind'] == 'child']
    require(len(children) == 1203 and len({c['frame'] for c in children}) == len(children), 'missing or duplicate natural child calls')
    by_frame = {c['frame']: c for c in children}
    require(all(c['object_id'] == 0x9e00 and len(c['draws']) == 6 for c in children), 'raw call identity differs')
    expected_tiles = [0x5d0, 0x5d0, 0x5e0, 0x5e0, 0x5f0, 0x5f0]
    for c in children:
        require(c['draws'][0]['layer_tail'] == 0xf402, 'FC bucket was not initially empty')
        for i, d in enumerate(c['draws']):
            b = bytes.fromhex(d['descriptor'])
            require(len(b) == 10 and word(b, 2) == 0x0ffc and word(b, 4) == expected_tiles[i] and word(b, 8) == 0xffff, 'raw descriptor identity differs')
            require(d['accepted'], 'natural raw submission was rejected')

    samples = reference['frames']
    require(len(samples) == 20 and len({s['frame'] for s in samples}) == 20, 'missing or duplicate VDP samples')
    frame_results = []
    for s in samples:
        frame = s['frame']
        require(s['arena_offset'] == 10 and s['video_next_frame'] == frame+1 and frame in by_frame, 'sample time or arena identity differs')
        require(s['width'] == 320 and s['height'] == 224 and s['pixel_format'] == 2, 'sample video format differs')
        vram, active = bytes.fromhex(s['vram']), bytes.fromhex(s['active_vram'])
        require(len(vram) == len(active) == 65536, 'incomplete VRAM sample')
        require(vram[0xba00:0xc000] == active[0xba00:0xc000] == bank, 'aux bytes changed in a VRAM sample')
        register5 = s['registers'][5]
        require(all(row['registers'][5] == register5 for row in s['active_lines'] if row['draw']), 'SAT base changed during actual drawing')
        sat = (register5&0x7e) << 9
        index, chain, seen = 0, [], set()
        for _ in range(80):
            require(index < 80 and index not in seen, 'invalid or cyclic actual SAT chain')
            seen.add(index)
            entry = active[sat+index*8:sat+(index+1)*8]
            chain.append((index, entry))
            index = entry[3]&127
            if index == 0:
                break
        require(index == 0, 'actual SAT chain did not terminate')
        raw_parts = [(i, e) for i, e in chain if 0x5d0 <= (word(e, 4)&2047) < 0x600]
        require(len(raw_parts) == 6 and chain[-6:] == raw_parts, 'raw sprites are not exactly the six final chain entries')
        for (index, entry), draw in zip(raw_parts, by_frame[frame]['draws']):
            descriptor = bytes.fromhex(draw['descriptor'])
            require(index == draw['count'] and entry[:3] == descriptor[:3] and entry[4:] == descriptor[4:8], 'actual SAT differs from immediate raw descriptor')
        ram = bytes.fromhex(s['ram'])
        later = [offset for offset in range(0x9e40, 0xa000, 64) if int.from_bytes(ram[offset:offset+4], 'big')]
        require(not later, 'active object follows raw child in this sample')
        frame_results.append({'frame': frame, 'video_frame': frame+1, 'first_raw_index': raw_parts[0][0],
                              'last_raw_index': raw_parts[-1][0], 'last_link': raw_parts[-1][1][3]&127})

    lookup = {(f['key']['graphics_set'], f['key']['parameter']): f for f in sprites['frames']}
    ordinary, sky = set(), defaultdict(set)
    descriptors, preloaded, excluded = 0, 0, defaultdict(int)
    require(len(oracle['natural']) == 83, 'sprite oracle scope differs')
    for frame in oracle['natural']:
        for draw in frame['draws']:
            b = bytes.fromhex(draw['actor'])
            key = (int.from_bytes(b[0x1c:0x20], 'big'), word(b, 0x3c))
            definition = lookup[key]
            for transfer in definition['transfers']:
                stamp = transfer['stamp']
                start = ((word(b, 0x24)&2047)+((stamp>>8)&255))&2047
                targets = {(start+i)&2047 for i in range(stamp&255)}
                if draw['owner'] in [0x9c80, 0x9d00]:
                    sky[draw['owner']] |= targets
                    excluded[draw['owner']] += 1
                elif definition['preloaded_only']:
                    preloaded += 1
                else:
                    ordinary |= targets
                    descriptors += 1
    overlap = ordinary&set(range(0x5d0, 0x600))
    require(ordinary and not overlap, 'ordinary observed transfer descriptors overlap arena aux slots')
    static_paths = ['arena-a-draw.txt', 'arena-a-consumers.txt', 'sprite-submit.txt']
    result = {'schema': 1, 'pass': True, 'scope': 'Read-only audit of saved arena A SAT order, aux bytes and observed ordinary sprite transfer targets',
              'producer_sha256': digest(Path(__file__).read_bytes()), 'rom_sha256': ROM_SHA, 'core_sha256': CORE_SHA,
              'sources': sources, 'static_sha256': {str((ROOT/'docs/research/md-yuyuhakusho/static'/name).relative_to(ROOT)): digest((ROOT/'docs/research/md-yuyuhakusho/static'/name).read_bytes()) for name in static_paths},
              'raw_natural': {'child_calls': len(children), 'submissions': len(children)*6, 'all_accepted': True,
                              'first_count_min': min(c['draws'][0]['count'] for c in children), 'first_count_max': max(c['draws'][0]['count'] for c in children),
                              'fc_bucket_initial_tail': 0xf402, 'local_tile_starts': [0, 0, 16, 16, 32, 32]},
              'sat_audit': {'samples': 20, 'matched_descriptors': 120, 'all_are_final_six': True, 'all_last_links_zero': True,
                            'alignment': 'CPU call frame f matches active_vram SAT used by video_next frame f+1', 'frames': frame_results},
              'aux_vram': {'snapshots': 40, 'frame_end_matches': 20, 'actual_draw_matches': 20, 'bytes_per_snapshot': len(bank), 'decoded_sha256': AUX_SHA},
              'ordinary_transfer_targets': {'natural_frames': 83, 'descriptors': descriptors, 'preloaded_only_descriptors_excluded': preloaded,
                                             'method': 'Conservative union of non-preloaded transfer descriptors, including cache hits and SkipTiles cases; not an actual-write count',
                                             'slot_count': len(ordinary), 'slot_ranges_half_open': ranges(ordinary), 'aux_overlap_slots': sorted(overlap),
                                             'excluded_default_sky': [{'owner': owner, 'descriptors': excluded[owner], 'slot_ranges_half_open': ranges(slots)} for owner, slots in sorted(sky.items())]},
              'limits': ['These 20 arena A samples and 83 existing sprite-reference frames do not prove all characters and times cannot overwrite the aux region.',
                         'This audit checks saved SAT descriptors and bank bytes; final combined sprite/background pixels are a separate acceptance scope.',
                         'No ROM, emulator, original control or shared presentation code is executed or modified.']}
    out = inside(args.out)
    out.parent.mkdir(parents=True, exist_ok=True)
    encoded = (json.dumps(result, indent=2)+'\n').encode()
    out.write_bytes(encoded)
    print(json.dumps({'pass': True, 'output': str(out.relative_to(ROOT)), 'sha256': digest(encoded),
                      'sat_samples': 20, 'vram_snapshots': 40, 'ordinary_slot_ranges_half_open': ranges(ordinary)}))


if __name__ == '__main__':
    main()
