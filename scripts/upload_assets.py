#!/usr/bin/env python3
"""Upload a prepared artwork directory to OSS. Credentials never enter the bundle."""
import argparse
import base64
import hashlib
import hmac
import mimetypes
from pathlib import Path
from email.utils import formatdate
import urllib.error
import urllib.parse
import urllib.request


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('directory', type=Path)
    parser.add_argument('--bucket', required=True)
    parser.add_argument('--endpoint', required=True, help='HTTPS bucket hostname')
    parser.add_argument('--prefix', required=True, help='Versioned object prefix')
    parser.add_argument('--secrets', type=Path, default=Path('config/secrets'))
    args = parser.parse_args()
    key_id = (args.secrets / 'oss-access-key-id').read_text().strip()
    secret = (args.secrets / 'oss-access-key-secret').read_text().strip()
    endpoint = args.endpoint.removeprefix('https://').rstrip('/')
    if '/' in endpoint or not endpoint.endswith('.aliyuncs.com'):
        parser.error('endpoint must be an OSS bucket hostname')
    prefix = args.prefix.strip('/')
    if not prefix or '..' in prefix.split('/'):
        parser.error('a nonempty object prefix is required')
    files = sorted(args.directory.rglob('*.webp'))
    if not files:
        parser.error('no WebP artwork found')
    for path in files:
        with path.open('rb') as source:
            header = source.read(12)
        if len(header) != 12 or header[:4] != b'RIFF' or header[8:] != b'WEBP':
            parser.error(f'invalid or incomplete WebP: {path.name}')
    for path in files:
        relative = path.relative_to(args.directory).as_posix()
        object_key = prefix + '/' + relative
        body = path.read_bytes()
        checksum = base64.b64encode(hashlib.md5(body).digest()).decode()
        content_type = mimetypes.guess_type(path.name)[0] or 'image/webp'
        date = formatdate(usegmt=True)
        canonical = f'PUT\n{checksum}\n{content_type}\n{date}\n/{args.bucket}/{object_key}'
        signature = base64.b64encode(hmac.new(secret.encode(), canonical.encode(), hashlib.sha1).digest()).decode()
        request = urllib.request.Request('https://' + endpoint + '/' + urllib.parse.quote(object_key, safe='/'),
            data=body, method='PUT', headers={
                'Content-MD5': checksum, 'Content-Type': content_type, 'Date': date,
                'Cache-Control': 'public, max-age=31536000, immutable',
                'Authorization': f'OSS {key_id}:{signature}',
            })
        try:
            with urllib.request.urlopen(request, timeout=45) as response:
                if response.status != 200:
                    raise RuntimeError(f'Upload failed: HTTP {response.status}')
        except urllib.error.HTTPError as error:
            raise SystemExit(f'Upload failed for {relative}: HTTP {error.code}; check OSS permissions and endpoint.') from None
        print(f'Uploaded {relative} ({len(body)} bytes)')
    print(f'Uploaded {len(files)} artwork files. Credentials were not included.')


if __name__ == '__main__':
    main()
