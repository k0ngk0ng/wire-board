import gzip
import hashlib
import io
import json
from pathlib import Path
import subprocess
import sys
import tarfile
import tempfile


ROOT = Path(__file__).resolve().parents[2]
SCRIPT = ROOT / 'scripts/ci/check-release-image.py'


def sha(data):
    return hashlib.sha256(data).hexdigest()


def content(value):
    return json.dumps(value, separators=(',', ':')).encode()


def tar_bytes(entries):
    output = io.BytesIO()
    with tarfile.open(fileobj=output, mode='w') as archive:
        for name, body in entries.items():
            entry = tarfile.TarInfo(name)
            if isinstance(body, tuple):
                entry.type, entry.linkname = body
                archive.addfile(entry)
            else:
                entry.size = len(body)
                entry.mode = 0o755 if name.endswith('/wire-board') else 0o644
                archive.addfile(entry, io.BytesIO(body))
    return output.getvalue()


def app_package(folder):
    path = folder / 'input'
    binary = b'\x7fELF' + b'verified compiler output' * 30
    old = b'\0asmretained-core'
    new = b'\0asmcurrent-core'
    rules = b'verified palette-free original rules'
    manifest = {'format': 1, 'resourceID': sha(b'original rules and assets'), 'authoritySHA256': sha(rules), 'current': sha(new), 'retained': [sha(old)]}
    files = {'wire-board': binary, 'pal/manifest.json': content(manifest), 'pal/authority.zip': rules, f'pal/cores/{sha(old)}.wasm': old, f'pal/cores/{sha(new)}.wasm': new, 'docs/LICENSE.SDLPAL': b'SDLPAL license', 'docs/THIRD_PARTY.md': b'License holders'}
    for name, raw in files.items():
        target = path / name
        target.parent.mkdir(parents=True, exist_ok=True)
        target.write_bytes(raw)
    meta = ['cdn-assets-manifest.json', 'contra-delivery.json', 'game-load-manifest.json', 'index.html']
    boundary = {'passed': True, 'metadataOnly': True, 'binarySHA256': sha(binary), 'binaryBytes': len(binary), 'embeddedFiles': ['web/dist/' + name for name in meta], 'cdnFileCount': 7587}
    audit = folder / 'boundary.json'
    audit.write_bytes(content(boundary))
    mapped = {'usr/local/share/licenses/wire-board/' + name.removeprefix('docs/'): raw for name, raw in files.items() if name.startswith('docs/')}
    mapped.update({'usr/local/bin/' + name: raw for name, raw in files.items() if not name.startswith('docs/')})
    return path, mapped, audit, manifest


def make_image(folder, layers, baseline, name, compress, topology):
    diff = ['sha256:' + sha(layer) for layer in layers]
    rootfs = {'type': 'layers', 'diff_ids': diff}
    config = content({'architecture': 'amd64', 'os': 'linux', 'rootfs': rootfs})
    image_id = 'sha256:' + sha(config)
    names = []
    entries = {}
    for index, layer in enumerate(layers):
        packed = gzip.compress(layer, mtime=0) if compress else layer
        reference = ('blobs/sha256/' + sha(packed)) if compress else f'{"%064x" % (index + 1)}/layer.tar'
        names.append(reference)
        entries[reference] = packed
    cfg = 'blobs/sha256/' + sha(config) if compress else sha(config) + '.json'
    entries[cfg] = config
    entries['manifest.json'] = content([{'Config': cfg, 'RepoTags': ['wire-board-release:verified'], 'Layers': names}])
    if topology:
        def blob(raw, media_type):
            identity = sha(raw)
            entries['blobs/sha256/' + identity] = raw
            return {'mediaType': media_type, 'digest': 'sha256:' + identity, 'size': len(raw)}
        leaf_format = 'application/vnd.oci.image.manifest.v1+json'
        index_format = 'application/vnd.oci.image.index.v1+json'
        runtime = blob(content({'schemaVersion': 2, 'mediaType': leaf_format, 'config': blob(config, 'application/vnd.oci.image.config.v1+json'), 'layers': [blob(entries[layer], 'application/vnd.oci.image.layer.v1.tar+gzip') for layer in (names if topology != 'wrong-layers' else list(reversed(names)))]}), leaf_format)
        descriptors = [{**runtime, 'platform': {'os': 'linux', 'architecture': 'amd64'}}, {**runtime, 'platform': {'os': 'unknown', 'architecture': 'unknown'}, 'annotations': {'vnd.docker.reference.type': 'attestation-manifest'}}]
        if topology == 'extra-platform':
            descriptors.append({**runtime, 'platform': {'os': 'linux', 'architecture': 'arm64'}})
        pointer = blob(content({'schemaVersion': 2, 'mediaType': index_format, 'manifests': descriptors}), index_format)
        entries['index.json'] = content({'schemaVersion': 2, 'mediaType': index_format, 'manifests': [pointer]})
        image_id = pointer['digest']
    export = folder / f'{name}.tar'
    export.write_bytes(tar_bytes(entries))
    parent = folder / f'{name}-rootfs.json'
    parent.write_bytes(content(['sha256:' + sha(layer) for layer in baseline]))
    return export, parent, image_id


def case(folder, package, boundary, layers, baseline, name, expected_error='', compress=False, substitute_id=False, topology=''):
    image, parent, image_id = make_image(folder, layers, baseline, name, compress, topology)
    out = folder / f'{name}-result.json'
    passed = subprocess.run([sys.executable, '-B', SCRIPT, '--image', image, '--image-id', 'sha256:' + '0' * 64 if substitute_id else image_id, '--base-rootfs', parent, '--package', package, '--server-boundary', boundary, '--out', out], capture_output=True, text=True)
    if expected_error:
        assert passed.returncode != 0 and expected_error in passed.stderr and not out.exists(), (name, passed.returncode, passed.stderr)
    else:
        assert passed.returncode == 0, (name, passed.stderr)
        result = json.loads(out.read_bytes())
        assert result['passed'] and result['imageID'] == image_id and result['runtimeLayers'] == len(layers) - len(baseline)
    return {'case': name, 'rejected': bool(expected_error)}


scratch = ROOT / '.local/ci'
scratch.mkdir(parents=True, exist_ok=True)
with tempfile.TemporaryDirectory(prefix='release-image-e2e-', dir=scratch) as temporary:
    folder = Path(temporary)
    package, runtime, boundary, manifest = app_package(folder)
    baseline = [tar_bytes({'bin/busybox': b'alpine'}), tar_bytes({'usr/bin/sqlite3': b'sqlite'})]
    clean = tar_bytes(runtime)
    old = 'usr/local/bin/pal/cores/' + manifest['retained'][0] + '.wasm'
    binary = 'usr/local/bin/wire-board'
    result = [
        case(folder, package, boundary, [*baseline, clean], baseline, 'docker-layer-tar'),
        case(folder, package, boundary, [*baseline, clean], baseline, 'oci-layer-gzip', compress=True),
        case(folder, package, boundary, [*baseline, clean], baseline, 'containerd-inspect-index', compress=True, topology='index'),
        case(folder, package, boundary, [*baseline, clean], baseline, 'different-oci-layer-chain', 'OCI layers differ', compress=True, topology='wrong-layers'),
        case(folder, package, boundary, [*baseline, clean], baseline, 'unaudited-oci-platform', 'additional runtime platform', compress=True, topology='extra-platform'),
        case(folder, package, boundary, [*baseline, tar_bytes({**runtime, 'opt/data.bin': b'client payload'})], baseline, 'renamed-byte-payload', 'unexpected runtime layer member'),
        case(folder, package, boundary, [*baseline, tar_bytes({**runtime, 'data/map.json': b'client payload'}), tar_bytes({'data/.wh.map.json': b''})], baseline, 'deleted-from-rootfs', 'unexpected runtime layer member'),
        case(folder, package, boundary, [*baseline, tar_bytes({k: v for k, v in runtime.items() if k != old})], baseline, 'missing-retained-core', 'missing runtime image files'),
        case(folder, package, boundary, [*baseline, tar_bytes({**runtime, old: b'worse retained core'})], baseline, 'changed-retained-core', 'runtime image bytes differ'),
        case(folder, package, boundary, [*baseline, tar_bytes({**runtime, binary: (tarfile.SYMTYPE, '/opt/binary')})], baseline, 'symlink-executable', 'not a regular image file'),
        case(folder, package, boundary, [*baseline, clean, tar_bytes({binary: runtime[binary]})], baseline, 'duplicate-application-file', 'duplicate runtime image file'),
        case(folder, package, boundary, [*reversed(baseline), clean], baseline, 'different-base-chain', 'runtime base layer chain differs'),
        case(folder, package, boundary, [*baseline, clean], baseline, 'unrelated-image-id', 'image config does not match', substitute_id=True),
    ]
    original = boundary.read_bytes()
    proof = json.loads(original)
    boundary.write_bytes(content({**proof, 'binarySHA256': 'f' * 64}))
    result.append(case(folder, package, boundary, [*baseline, clean], baseline, 'different-compiler-output', 'server package boundary differs'))
    boundary.write_bytes(content({**proof, 'embeddedFiles': [*proof['embeddedFiles'], 'web/dist/pal/music.flac']}))
    result.append(case(folder, package, boundary, [*baseline, clean], baseline, 'reported-embedded-payload', 'server package boundary differs'))
    assert len(result) == 15
    workflow = (ROOT / '.github/workflows/publish-verified.yml').read_text()
    embedded = workflow.split("          python3 - <<'PY'\n", 1)[1].split('          PY\n', 1)[0]
    renderer = '\n'.join(line[10:] for line in embedded.splitlines())
    original = (ROOT / 'Dockerfile').read_text()
    marker = 'FROM runtime-base AS runtime'
    assert original.count(marker) == 1
    (folder / 'image').mkdir()
    (folder / 'image/image').mkdir()
    pin = folder / 'games/pal/assets/server-release.json'
    pin.parent.mkdir(parents=True)
    released = content(manifest)
    identity = sha(released)
    pin.write_bytes(content({'current': identity, 'catalogs': {identity: {'cdnVerified': True, 'assetID': identity}}}))
    image_pal = folder / 'image/image/pal'
    image_pal.mkdir()
    cases = [
        ('runtime-source', original, released, ''),
        ('payload-copy-into-base', original.replace(marker, 'COPY client-data /opt/data\n' + marker), released, 'Unexpected runtime base inputs'),
        ('payload-installed-into-base', original.replace(marker, 'RUN unpack-assets /opt\n' + marker), released, 'Unexpected runtime base inputs'),
        ('wrong-published-rule-bundle', original, content({**manifest, 'retained': []}), 'PAL source publication differs'),
    ]
    for label, text, packaged, error in cases:
        (folder / 'Dockerfile').write_text(text)
        (image_pal / 'manifest.json').write_bytes(packaged)
        model = subprocess.run([sys.executable, '-B', '-'], cwd=folder / 'image', input=renderer, capture_output=True, text=True)
        assert (model.returncode == 0) == (not error), (label, model.stderr)
        if not error:
            rendered = (folder / 'image/image/Dockerfile').read_text()
            assert marker in rendered and 'COPY pal /usr/local/bin/pal' in rendered and '--from=backend' not in rendered
        else:
            assert error in model.stderr
        result.append({'case': label, 'rejected': bool(error)})
print(json.dumps({'passed': True, 'accepted': 4, 'rejected': 15, 'cases': result}))
