import copy
import hashlib
import json
from pathlib import Path
import re
import subprocess

ROOT = Path(__file__).resolve().parents[3]
INDEX = ROOT / '.local/pal/audio-media-e2e/index.json'
TARGET = ROOT / 'games/pal/server/audio_assets.go'
ARCHIVE = ROOT / 'games/pal/assets/audio-timelines.json'
GROUPS = [('sounds', 'Sounds', 363), ('music', 'Music', 87), ('cd', 'CD', 8)]


def validate_catalog(catalog):
    if not isinstance(catalog, dict) or set(catalog) != {group for group, _, _ in GROUPS}:
        raise ValueError('invalid original audio catalog groups')
    for group, _, count in GROUPS:
        clips = catalog[group]
        if not isinstance(clips, dict) or len(clips) != count:
            raise ValueError('unexpected original audio catalog length')
        for number, item in clips.items():
            if not isinstance(number, str) or not re.fullmatch(r'[1-9][0-9]{0,4}', number) or int(number) > 65535:
                raise ValueError('invalid original audio catalog number')
            if not isinstance(item, list) or len(item) != 2 or any(type(value) is not int for value in item) or not 0 < item[0] <= 1 << 40 or not 0 < item[1] <= 384000:
                raise ValueError('invalid original audio timing metadata')


def validate_archive(archive):
    if archive.get('version') != 1 or not isinstance(archive.get('catalogs'), dict):
        raise ValueError('invalid retained original audio catalogs')
    catalogs = archive['catalogs']
    if catalogs and archive.get('current') not in catalogs:
        raise ValueError('current original audio catalog is missing')
    for asset_id, catalog in catalogs.items():
        if not re.fullmatch(r'[a-f0-9]{64}', asset_id):
            raise ValueError('invalid original audio catalog identity')
        validate_catalog(catalog)


def extend(raw, existing):
    validate_archive(existing)
    index = json.loads(raw)
    if index['version'] != 1:
        raise ValueError('unexpected original audio index version')
    catalog = {group: {number: [item['samples'], item['sampleRate']] for number, item in index[group].items()} for group, _, _ in GROUPS}
    validate_catalog(catalog)
    asset_id = hashlib.sha256(raw).hexdigest()
    result = copy.deepcopy(existing)
    retained = result['catalogs'].get(asset_id)
    if retained is not None and retained != catalog:
        raise ValueError('immutable original audio timing metadata changed')
    result['catalogs'][asset_id] = catalog
    result['current'] = asset_id
    return result


def validate_release(archive, release):
    validate_archive(archive)
    if not archive['catalogs'] or release.get('version') != 1:
        raise ValueError('original audio publication metadata is missing')
    for asset_id in archive['catalogs']:
        publication = release.get('catalogs', {}).get(asset_id)
        if not publication or not publication.get('cdnVerified') or publication.get('assetID') != asset_id or publication.get('files', {}).get('index.json', {}).get('sha256') != asset_id:
            raise ValueError('original audio catalog has not passed CDN read-back')


def render(archive):
    validate_archive(archive)
    if not archive['catalogs']:
        raise ValueError('original audio catalog is empty')
    lines = ['package pal', '', 'const audioAssetID = "' + archive['current'] + '"', '', 'var audioCatalogs = map[string]audioCatalog{']
    for asset_id, catalog in sorted(archive['catalogs'].items()):
        lines.append('\t"' + asset_id + '": {')
        for group, name, _ in GROUPS:
            lines.append('\t\t' + name + ': map[int][2]int64{')
            for number, (samples, rate) in sorted(catalog[group].items(), key=lambda row: int(row[0])):
                lines.append(f'\t\t\t{number}: {{{samples}, {rate}}},')
            lines.append('\t\t},')
        lines.append('\t},')
    lines.extend(['}', ''])
    return subprocess.check_output(['gofmt'], input='\n'.join(lines), text=True, cwd=ROOT)


def guarded(path):
    if not path.resolve().is_relative_to(ROOT) or path.is_symlink() or any(parent.is_symlink() for parent in path.parents if parent.is_relative_to(ROOT)):
        raise ValueError('audio metadata path escapes workspace or uses symlinks')


def write(path, content):
    guarded(path)
    temporary = path.with_suffix(path.suffix + '.tmp')
    guarded(temporary)
    temporary.write_text(content)
    temporary.replace(path)


def main():
    guarded(INDEX)
    existing = json.loads(ARCHIVE.read_text()) if ARCHIVE.exists() else {'version': 1, 'catalogs': {}}
    archive = extend(INDEX.read_bytes(), existing)
    release = json.loads((ROOT / 'games/pal/assets/audio-release.json').read_text())
    validate_release(archive, release)
    code = render(archive)
    write(ARCHIVE, json.dumps(archive, indent=2, sort_keys=True) + '\n')
    write(TARGET, code)
    print(json.dumps({'assetID': archive['current'], 'catalogs': len(archive['catalogs']), 'target': str(TARGET.relative_to(ROOT)), 'archive': str(ARCHIVE.relative_to(ROOT))}))


if __name__ == '__main__':
    main()
