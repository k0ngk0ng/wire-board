
"""Upload a prepared artwork directory to OSS. Credentials never enter the bundle."""
import argparse
import base64
import hashlib
import hmac
from pathlib import Path
import sys
from email.utils import formatdate
import urllib.error
import urllib.parse
import urllib.request
sys.path.insert(0, str(Path(__file__).resolve().parent))
from resource_store import ObjectStore, publish, OWNERS


def collect_assets(directory, include_audio=False):
    """Preflight the complete bundle before performing any remote writes."""
    types = {'.webp': 'image/webp'}
    if include_audio:
        types.update({'.mp3': 'audio/mpeg', '.ogg': 'audio/ogg', '.wav': 'audio/wav'})
    result = []
    if directory.is_symlink():
        raise ValueError('asset directory must not be a symlink')
    for path in sorted(directory.rglob('*')):
        if path.is_symlink():
            raise ValueError(f'symlinks are not allowed in asset bundles: {path.name}')
        suffix = path.suffix.lower()
        if not path.is_file() or suffix not in types:
            continue
        with path.open('rb') as source:
            header = source.read(12)
        valid = len(header) == 12 and {
            '.webp': header[:4] == b'RIFF' and header[8:] == b'WEBP',
            '.wav': header[:4] == b'RIFF' and header[8:] == b'WAVE',
            '.ogg': header[:4] == b'OggS',
            '.mp3': header[:3] == b'ID3' or (header[:1] == b'\xff' and header[1] & 0xe0 == 0xe0),
        }[suffix]
        if not valid:
            raise ValueError(f'invalid or incomplete media: {path.name}')
        result.append((path, types[suffix]))
    return result


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('directory', type=Path)
    parser.add_argument('--bucket', required=True)
    parser.add_argument('--endpoint', required=True, help='HTTPS bucket hostname')
    parser.add_argument('--prefix', required=True, help='Versioned object prefix')
    parser.add_argument('--secrets', type=Path, default=Path('config/secrets'))
    parser.add_argument('--include-audio', action='store_true', help='Also upload MP3, Ogg and WAV sound files')
    args = parser.parse_args()
    key_id = (args.secrets / 'oss-access-key-id').read_text().strip()
    secret = (args.secrets / 'oss-access-key-secret').read_text().strip()
    endpoint = args.endpoint.removeprefix('https://').rstrip('/')
    if '/' in endpoint or not endpoint.endswith('.aliyuncs.com'):
        parser.error('endpoint must be an OSS bucket hostname')
    prefix = args.prefix.strip('/')
    parts = prefix.split('/')
    if len(parts) != 2 or parts[0] != 'bga' or parts[1] not in OWNERS:
        parser.error('expected bga/<game-id> or bga/_common')
    try:
        files = collect_assets(args.directory, args.include_audio)
    except ValueError as error:
        parser.error(str(error))
    if not files:
        parser.error('no supported media found')
    client = ObjectStore('https://' + endpoint, args.bucket, key_id, secret)
    for path, content_type in files:
        record = publish(client, parts[1], path.name, path.read_bytes(), mime=content_type)
        print(f'Verified {record["key"]} ({record["bytes"]} bytes)')


if __name__ == '__main__':
    main()
