import argparse
from contextlib import contextmanager
import gzip
import hashlib
import json
from pathlib import Path, PurePosixPath
import re
import tarfile


SHA = re.compile(r'[a-f0-9]{64}')
IMAGE_DIGEST = re.compile(r'sha256:[a-f0-9]{64}')
SERVER_METADATA = {'web/dist/index.html', 'web/dist/game-load-manifest.json', 'web/dist/cdn-assets-manifest.json', 'web/dist/contra-delivery.json'}
OCI_INDEX = {'application/vnd.oci.image.index.v1+json', 'application/vnd.docker.distribution.manifest.list.v2+json'}
OCI_MANIFEST = {'application/vnd.oci.image.manifest.v1+json', 'application/vnd.docker.distribution.manifest.v2+json'}


def require(condition, message):
    if not condition:
        raise ValueError(message)


def digest(stream):
    return hashlib.file_digest(stream, 'sha256').hexdigest()


def file_digest(path):
    with path.open('rb') as stream:
        return digest(stream)


def package_files(package, proof):
    binary = package / 'wire-board'
    metadata = proof.get('embeddedFiles', [])
    required = SERVER_METADATA - {'web/dist/contra-delivery.json'}
    require(proof.get('passed') is True and proof.get('metadataOnly') is True and isinstance(metadata, list) and len(metadata) == len(set(metadata)) and required <= set(metadata) <= SERVER_METADATA and proof.get('binaryBytes') == binary.stat().st_size and proof.get('binarySHA256') == file_digest(binary), 'server package boundary differs from the runtime application binary')
    pal_root = package / 'pal'
    manifest = json.loads((pal_root / 'manifest.json').read_bytes())
    retained = manifest.get('retained')
    require(manifest.get('format') == 1 and isinstance(retained, list) and len(retained) <= 64, 'invalid PAL server rule manifest')
    cores = [manifest.get('current'), *retained]
    require(all(isinstance(core, str) and SHA.fullmatch(core) for core in cores) and len(cores) == len(set(cores)), 'invalid PAL current or retained rule identities')
    relative_rules = {'manifest.json', 'authority.zip', *(f'cores/{core}.wasm' for core in cores)}
    entries = list(pal_root.rglob('*'))
    require(pal_root.is_dir() and not pal_root.is_symlink() and not any(path.is_symlink() for path in entries), 'unexpected file type in PAL server package')
    require({p.relative_to(pal_root).as_posix() for p in entries if not p.is_dir()} == relative_rules, 'PAL server package differs from its rule manifest')
    sources = {'usr/local/bin/wire-board': binary}
    sources.update({'usr/local/bin/pal/' + name: pal_root / name for name in relative_rules})
    sources.update({'usr/local/share/licenses/wire-board/' + name: package / 'docs' / name for name in ['LICENSE.SDLPAL', 'THIRD_PARTY.md']})
    expected = {}
    for name, path in sources.items():
        require(path.is_file() and not path.is_symlink(), 'not a regular release package file: ' + name)
        expected[name] = {'bytes': path.stat().st_size, 'sha256': file_digest(path)}
    require(expected['usr/local/bin/pal/authority.zip']['sha256'] == manifest.get('authoritySHA256'), 'PAL authority rules differ from the server manifest')
    for core in cores:
        require(expected[f'usr/local/bin/pal/cores/{core}.wasm']['sha256'] == core, 'PAL core differs from the server manifest: ' + core)
    return expected, cores


def archive_member(archive, name):
    member = archive.getmember(name)
    require(member.isfile(), 'not a regular member of the image export: ' + name)
    return archive.extractfile(member)


def blob_path(descriptor):
    value = descriptor['digest']
    require(isinstance(value, str) and IMAGE_DIGEST.fullmatch(value) and type(descriptor.get('size')) is int and descriptor['size'] > 0, 'invalid OCI descriptor identity')
    return 'blobs/sha256/' + value.removeprefix('sha256:')


def oci_document(archive, descriptor):
    with archive_member(archive, blob_path(descriptor)) as stream:
        raw = stream.read()
    require(len(raw) == descriptor['size'] and 'sha256:' + hashlib.sha256(raw).hexdigest() == descriptor['digest'], 'OCI descriptor bytes differ from image identity')
    return json.loads(raw)


def oci_layers(archive, image_id, descriptor, raw_config):
    config_id = 'sha256:' + hashlib.sha256(raw_config).hexdigest()
    if config_id == image_id:
        return []
    require('index.json' in archive.getnames(), 'image config does not match the image selected for push')
    with archive_member(archive, 'index.json') as stream:
        index = json.load(stream)
    roots = index.get('manifests', [])
    require(len(roots) == 1 and roots[0]['digest'] == image_id, 'image config does not match the image selected for push')
    head = roots[0]
    if head.get('mediaType') in OCI_INDEX:
        index_payload = oci_document(archive, head)
        active = []
        for child in index_payload['manifests']:
            platform = child.get('platform', {})
            if platform.get('os') == 'unknown' and platform.get('architecture') == 'unknown' and child.get('annotations', {}).get('vnd.docker.reference.type') == 'attestation-manifest':
                continue
            require(platform.get('os') == 'linux' and platform.get('architecture') == 'amd64', 'additional runtime platform in selected image')
            active.append(child)
        require(len(active) == 1, 'expected exactly one OCI runtime manifest')
        head = active[0]
    require(head.get('mediaType') in OCI_MANIFEST, 'unsupported OCI runtime manifest format')
    manifest = oci_document(archive, head)
    require(manifest['config']['digest'] == config_id and manifest['config']['size'] == len(raw_config) and blob_path(manifest['config']) == descriptor['Config'], 'OCI config differs from exported runtime config')
    layers = manifest['layers']
    require([blob_path(layer) for layer in layers] == descriptor['Layers'], 'OCI layers differ from exported runtime layers')
    return layers


@contextmanager
def uncompressed_layer(archive, name):
    with archive_member(archive, name) as raw:
        magic = raw.read(4)
        raw.seek(0)
        if magic.startswith(b'\x1f\x8b'):
            with gzip.GzipFile(fileobj=raw, mode='rb') as stream:
                yield stream
        else:
            yield raw


def inspect(image, image_id, base, expected):
    require(IMAGE_DIGEST.fullmatch(image_id) and isinstance(base, list) and bool(base) and all(isinstance(layer, str) and IMAGE_DIGEST.fullmatch(layer) for layer in base), 'invalid requested image or runtime base identity')
    directories = {parent.as_posix() for file in expected for parent in PurePosixPath(file).parents}
    seen = set()
    with tarfile.open(image, 'r:*') as archive:
        names = archive.getnames()
        require(len(names) == len(set(names)), 'duplicate path in image export')
        with archive_member(archive, 'manifest.json') as stream:
            inventory = json.load(stream)
        require(isinstance(inventory, list) and len(inventory) == 1, 'expected an export of exactly one image')
        descriptor = inventory[0]
        with archive_member(archive, descriptor['Config']) as stream:
            raw = stream.read()
        published_layers = oci_layers(archive, image_id, descriptor, raw)
        config = json.loads(raw)
        require(config.get('os') == 'linux' and config.get('architecture') == 'amd64' and config.get('rootfs', {}).get('type') == 'layers', 'expected a Linux amd64 runtime image')
        layers = descriptor['Layers']
        identities = config['rootfs']['diff_ids']
        require(len(layers) == len(identities) and len(layers) > len(base), 'runtime image has an incomplete layer chain')
        require(identities[:len(base)] == base, 'runtime base layer chain differs from the built base')
        for index, (layer, identity) in enumerate(zip(layers, identities)):
            if published_layers:
                published = published_layers[index]
                with archive_member(archive, layer) as stream:
                    require(archive.getmember(layer).size == published['size'] and 'sha256:' + digest(stream) == published['digest'], 'OCI layer blob bytes differ from image manifest')
            with uncompressed_layer(archive, layer) as stream:
                require('sha256:' + digest(stream) == identity, 'exported layer bytes differ from image config')
            if index < len(base):
                continue
            with uncompressed_layer(archive, layer) as stream, tarfile.open(fileobj=stream, mode='r|') as files:
                for member in files:
                    name = member.name.removeprefix('./').rstrip('/')
                    if member.isdir() and name in directories:
                        continue
                    require(name in expected, 'unexpected runtime layer member: ' + name)
                    require(member.isfile() and not member.issparse(), 'not a regular image file: ' + name)
                    require(name not in seen, 'duplicate runtime image file: ' + name)
                    if name == 'usr/local/bin/wire-board':
                        require(member.mode & 0o111, 'runtime application binary is not executable')
                    with files.extractfile(member) as stream:
                        require(member.size == expected[name]['bytes'] and digest(stream) == expected[name]['sha256'], 'runtime image bytes differ: ' + name)
                    seen.add(name)
    require(seen == set(expected), 'missing runtime image files: ' + ', '.join(sorted(set(expected) - seen)))
    return len(layers) - len(base)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--image', type=Path, required=True)
    parser.add_argument('--image-id', required=True)
    parser.add_argument('--base-rootfs', type=Path, required=True)
    parser.add_argument('--package', type=Path, required=True)
    parser.add_argument('--server-boundary', type=Path, required=True)
    parser.add_argument('--out', type=Path, required=True)
    args = parser.parse_args()
    try:
        base = json.loads(args.base_rootfs.read_bytes())
        proof = json.loads(args.server_boundary.read_bytes())
        expected, cores = package_files(args.package, proof)
        layers = inspect(args.image, args.image_id, base, expected)
    except (OSError, ValueError, KeyError, TypeError, EOFError, tarfile.TarError) as error:
        parser.exit(1, f'{error}\n')
    report = {'passed': True, 'imageID': args.image_id, 'runtimeBaseLayers': base, 'runtimeLayers': layers, 'imageFiles': expected, 'palCurrentCore': cores[0], 'palRetainedCores': cores[1:], 'binaryBoundarySHA256': file_digest(args.server_boundary)}
    args.out.write_text(json.dumps(report, indent=2, sort_keys=True) + '\n')
    print(json.dumps(report, sort_keys=True))


if __name__ == '__main__':
    main()
