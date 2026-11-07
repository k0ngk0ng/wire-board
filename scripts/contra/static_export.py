#!/usr/bin/env python3
"""Retain complete REA evidence and a readable address/assembly index."""
import argparse
import gzip
import hashlib
import json
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
BASE = ROOT / '.local/contra-static'
OUT = ROOT / 'docs/research/nes-contra/evidence'


def sha(raw):
    return hashlib.sha256(raw).hexdigest()


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--runtime-run', type=Path)
    args = parser.parse_args()
    OUT.mkdir(parents=True, exist_ok=True)
    manifest = json.loads((BASE/'bank-manifest.json').read_text())
    index = {'rom_sha256': manifest['rom_sha256'], 'bank_manifest': manifest,
             'functions': [], 'bundles': [], 'limitations': [
                 'Function boundaries and names are analysis seeds; fallthrough code and inline dispatch tables require separate recovery.',
                 'Static code does not demonstrate complete player simulation or gameplay timing.',
                 'Only recorded runtime windows are bound; no claim covers unrecorded mapper states.']}
    for group in ['fixed', 'physics', 'terrain', 'header', 'playerstep', 'lifecycle', 'modes', 'mode-sprites']:
        source = BASE/f'{group}-rea'
        if not (source/'evidence.json').exists():
            continue
        spec = json.loads((BASE/f'{group}-spec.json').read_text())
        bundle = (source/'evidence.json').read_bytes()
        data = json.loads(bundle)
        assert isinstance(data['records'], list) and data['records']
        compressed = gzip.compress(bundle, mtime=0)
        name = f'static-{group}-rea.json.gz'
        (OUT/name).write_bytes(compressed)
        index['bundles'].append({'file': name, 'sha256': sha(compressed),
                                 'uncompressed_sha256': sha(bundle), 'records': len(data['records'])})
        for path in sorted(source.glob('function-*.json')):
            evidence = json.loads(path.read_text())
            r = evidence['normalized_result']
            assert evidence['subject']['digest']['sha256'] == spec['file_sha256']
            index['functions'].append({'group': group, 'evidence_id': evidence['evidence_id'],
                                       'target_sha256': spec['file_sha256'],
                                       'procedure': r['procedure'], 'assembly': r['assembly'],
                                       'limitations': evidence['limitations'] + r.get('limitations', [])})
    if args.runtime_run:
        run = args.runtime_run.resolve()
        assert run.is_relative_to(ROOT), 'runtime results must remain in workspace'
        source = json.loads((run/'source.json').read_text())
        assert source['rom_sha256'] == manifest['rom_sha256'] and source['completed']
        bank_hashes = {b['sha256']: b['bank_8k'] for b in manifest['banks']}
        windows = []
        for path in sorted(run.glob('*-prg.bin')):
            raw = path.read_bytes()
            assert len(raw) == 32768
            mappings = []
            for offset in range(0, 32768, 8192):
                digest = sha(raw[offset:offset+8192])
                assert digest in bank_hashes, 'captured PRG does not match ROM'
                mappings.append({'cpu_start':hex(0x8000+offset), 'bank_8k':bank_hashes[digest], 'sha256':digest})
            assert mappings[2]['bank_8k'] == 14 and mappings[3]['bank_8k'] == 15
            windows.append({'file':path.name,'sha256':sha(raw),'mappings':mappings})
        assert windows
        index['runtime_windows'] = {'run':str(run.relative_to(ROOT)), 'source':source,
                                     'scenario':json.loads((run/'scenario.json').read_text()), 'captures':windows}
    (OUT/'static-index.json').write_text(json.dumps(index,ensure_ascii=False,indent=2)+'\n')
    print(json.dumps({'passed':True, 'functions':len(index['functions']),
                      'runtime_windows':len(index.get('runtime_windows',{}).get('captures',[])),
                      'output':str(OUT/'static-index.json')}))


if __name__ == '__main__':
    main()
