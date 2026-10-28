"""Prepare a source-bound, portable audio pack without uploading anything."""
import argparse
import hashlib
import json
from pathlib import Path

ROOT = Path(__file__).resolve().parents[3]
DESCRIPTORS = ('audio-source.json', 'audio-loops.json')

def sha(data):
    return hashlib.sha256(data).hexdigest()

def source_name(stem):
    return str(Path(stem.get('researchDirectory', '')) / stem['file'])

def collect_media(source, loops):
    continuous = {sound['id'] for sound in loops['songs']}
    entries = {}
    for sound in [s for s in source['sounds'] if s['id'] not in continuous] + loops['songs']:
        for stem in sound['stems'] + ([sound] if sound['id'] == 110 else []):
            name = source_name(stem)
            if name in entries and entries[name]['sha256'] != stem['sha256']:
                raise ValueError('conflicting media source: ' + name)
            entries[name] = stem
    return entries

def read_media(root, stem, pack=None):
    digest = stem['sha256']
    if len(digest) != 64 or any(c not in '0123456789abcdef' for c in digest):
        raise ValueError('invalid media digest')
    base = (pack if pack is not None else root).resolve()
    path = (base / (digest + '.wav' if pack is not None else source_name(stem))).resolve()
    if not path.is_relative_to(base):
        raise ValueError('media path escapes source root')
    data = path.read_bytes()
    if sha(data) != digest:
        raise ValueError('media content mismatch: ' + str(path))
    return data

def inventory(root, pack=None):
    raw = {name:(root/'games/contra/assets'/name).read_bytes() for name in DESCRIPTORS}
    entries = collect_media(*(json.loads(raw[name]) for name in DESCRIPTORS))
    assets = {}
    for name, stem in entries.items():
        data = read_media(root, stem, pack)
        assets[name] = {'file':stem['sha256']+'.wav', 'sha256':sha(data), 'bytes':len(data)}
    return {'format':'contra-portable-media-v1',
            'descriptors':{name:sha(data) for name,data in raw.items()}, 'assets':assets}

def verify_pack(root, pack):
    expected = json.loads((pack/'manifest.json').read_text())
    actual = inventory(root, pack)
    if actual != expected:
        raise ValueError('media pack descriptor or inventory mismatch')
    return actual

def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--out', required=True, type=Path)
    parser.add_argument('--verify', action='store_true')
    args = parser.parse_args()
    out = args.out.resolve()
    if not out.is_relative_to(ROOT):
        parser.error('media pack must stay in workspace')
    if args.verify:
        result = verify_pack(ROOT, out)
    else:
        result = inventory(ROOT)
        out.mkdir(parents=True, exist_ok=False)
        for name, asset in result['assets'].items():
            (out/asset['file']).write_bytes(read_media(ROOT, {'file':name,'sha256':asset['sha256']}))
        (out/'manifest.json').write_text(json.dumps(result, sort_keys=True, indent=2)+'\n')
        verify_pack(ROOT, out)
    print(json.dumps({'path':str(out), 'references':len(result['assets']),
                      'manifestSHA256':sha((out/'manifest.json').read_bytes())}))

if __name__ == '__main__':
    main()
