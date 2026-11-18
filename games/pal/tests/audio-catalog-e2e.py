import copy
import hashlib
import importlib.util
import json
from pathlib import Path
import tempfile

ROOT = Path(__file__).resolve().parents[3]
spec = importlib.util.spec_from_file_location('audio_metadata', ROOT / 'games/pal/build/audio-metadata.py')
metadata = importlib.util.module_from_spec(spec)
spec.loader.exec_module(metadata)
raw = (ROOT / '.local/pal/audio-media-e2e/index.json').read_bytes()
first = hashlib.sha256(raw).hexdigest()
empty = {'version': 1, 'catalogs': {}}
original = copy.deepcopy(empty)
retained = metadata.extend(raw, empty)
assert empty == original
first_copy = copy.deepcopy(retained['catalogs'][first])
next_index = json.loads(raw)
next_index['music']['31']['samples'] = next_index['music']['31']['sampleRate'] * 2
next_raw = (json.dumps(next_index, sort_keys=True) + '\n').encode()
second = hashlib.sha256(next_raw).hexdigest()
updated = metadata.extend(next_raw, retained)
assert updated['current'] == second and set(updated['catalogs']) == {first, second}
assert updated['catalogs'][first] == retained['catalogs'][first] == first_copy
assert updated['catalogs'][second]['music']['31'][0] == next_index['music']['31']['sampleRate'] * 2
assert metadata.extend(raw, retained) == retained
rendered = metadata.render(updated)
assert first in rendered and second in rendered and 'const audioAssetID = "' + second + '"' in rendered
assert rendered == metadata.render(copy.deepcopy(updated))
rejected = []


def reject(label, operation):
    try:
        operation()
    except (ValueError, KeyError, TypeError):
        rejected.append(label)
        return
    raise AssertionError('invalid audio catalog accepted: ' + label)


for label, value in [('negative', -1), ('fraction', 1.5), ('boolean', True), ('excessive', 1 << 53)]:
    bad = json.loads(raw)
    bad['music']['31']['samples'] = value
    reject(label, lambda bad=bad: metadata.extend(json.dumps(bad).encode(), retained))
bad = json.loads(raw)
bad['sounds']['098'] = bad['sounds'].pop('98')
reject('noncanonical-id', lambda: metadata.extend(json.dumps(bad).encode(), retained))
bad = copy.deepcopy(retained)
bad['catalogs'][first]['music']['31'][0] += 1
reject('same-version-timing-overwrite', lambda: metadata.extend(raw, bad))
bad = copy.deepcopy(retained)
bad['current'] = '0' * 64
reject('missing-current-catalog', lambda: metadata.render(bad))
reject('unverified-release', lambda: metadata.validate_release(retained, {'version': 1, 'catalogs': {}}))
release = json.loads((ROOT / 'games/pal/assets/audio-release.json').read_text())
metadata.validate_release(retained, release)
bad = copy.deepcopy(release)
bad['catalogs'][first]['cdnVerified'] = False
reject('unverified-current-publication', lambda: metadata.validate_release(retained, bad))
bad = json.loads(raw)
bad['music']['31']['sampleRate'] = 0
reject('invalid-sample-rate', lambda: metadata.extend(json.dumps(bad).encode(), retained))
reject('outside-workspace-target', lambda: metadata.guarded(ROOT.parent / 'audio-metadata-forbidden.go'))
with tempfile.TemporaryDirectory(prefix='audio-catalog-', dir=ROOT / '.local/pal') as temporary:
    link = Path(temporary) / 'protected.go'
    link.symlink_to(ROOT / 'rules/common.md')
    reject('symlink-target', lambda: metadata.guarded(link))
report = {'passed': True, 'scope': 'offline genuine original index, independent changed timing catalog, append-only metadata retention, exact repeat generation and invalid catalog/release rejection; no variant upload or actual second published-version claim', 'original': first, 'variant': second, 'retainedVersions': len(updated['catalogs']), 'rejected': rejected}
(ROOT / '.local/pal/verification/audio-catalog-generation.json').write_text(json.dumps(report, indent=2) + '\n')
print(json.dumps(report))
