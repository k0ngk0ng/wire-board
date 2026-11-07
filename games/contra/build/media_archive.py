"""Deterministic lossless source audio package for clean, offline builds."""
import argparse
import io
import json
import os
from pathlib import Path
import re
import tarfile
import tempfile
from media import sha, verify_pack

ROOT=Path(__file__).resolve().parents[3]
FORMAT='contra-audio-source-pack-v1'

def inside(root,path):
    path=Path(path).resolve()
    if not path.is_relative_to(root.resolve()):raise ValueError('media package path outside workspace')
    return path

def pack_media(root,media,archive):
    media=inside(root,media);archive=inside(root,archive)
    manifest=verify_pack(root,media)
    if archive.exists():raise ValueError('source archive already exists')
    archive.parent.mkdir(parents=True,exist_ok=True)
    names=sorted({'manifest.json',*(item['file'] for item in manifest['assets'].values())})
    with tarfile.open(archive,'w:xz',format=tarfile.USTAR_FORMAT,preset=6) as tar:
        for name in names:
            raw=(media/name).read_bytes();entry=tarfile.TarInfo(name)
            entry.size=len(raw);entry.mode=0o644;entry.mtime=0
            tar.addfile(entry,io.BytesIO(raw))
    raw=archive.read_bytes()
    return {'format':FORMAT,'bytes':len(raw),'sha256':sha(raw),
            'manifestSHA256':sha((media/'manifest.json').read_bytes())}

def unpack_media(root,archive,pin,out):
    archive=inside(root,archive);out=inside(root,out)
    raw=archive.read_bytes()
    if pin.get('format')!=FORMAT or len(raw)!=pin.get('bytes') or sha(raw)!=pin.get('sha256'):
        raise ValueError('source audio archive identity differs')
    if out.exists():
        verify_pack(root,out)
        if sha((out/'manifest.json').read_bytes())!=pin['manifestSHA256']:
            raise ValueError('cached audio package differs')
        return out
    out.parent.mkdir(parents=True,exist_ok=True)
    with tarfile.open(fileobj=io.BytesIO(raw),mode='r:xz') as tar:
        members=tar.getmembers();names=[item.name for item in members]
        if len(names)!=len(set(names)) or any(not item.isfile() or not re.fullmatch(r'(?:manifest\.json|[a-f0-9]{64}\.wav)',item.name) for item in members):
            raise ValueError('unsupported source audio archive member')
        if 'manifest.json' not in names:raise ValueError('audio manifest missing')
        manifest_raw=tar.extractfile('manifest.json').read()
        if sha(manifest_raw)!=pin['manifestSHA256']:raise ValueError('audio manifest differs from package pin')
        manifest=json.loads(manifest_raw)
        expected={'manifest.json',*(item['file'] for item in manifest['assets'].values())}
        if set(names)!=expected:raise ValueError('source audio archive inventory differs')
        with tempfile.TemporaryDirectory(dir=out.parent,prefix='unpack-') as temporary:
            target=Path(temporary)/'media';target.mkdir()
            for member in members:
                (target/member.name).write_bytes(tar.extractfile(member).read())
            verify_pack(root,target)
            os.rename(target,out)
    return out

def prepare_source_media(root):
    assets=root/'games/contra/assets'
    pin=json.loads((assets/'audio-pack.json').read_text())
    return unpack_media(root,assets/'audio-pack-v1.tar.xz',pin,
                        root/'.local/contra-media'/('source-'+pin['sha256']))

def main():
    p=argparse.ArgumentParser(description=__doc__)
    p.add_argument('action',choices=['pack','unpack'])
    p.add_argument('--media-dir',type=Path)
    p.add_argument('--archive',type=Path,default=ROOT/'games/contra/assets/audio-pack-v1.tar.xz')
    p.add_argument('--pin',type=Path,default=ROOT/'games/contra/assets/audio-pack.json')
    p.add_argument('--out',type=Path)
    args=p.parse_args();pin_path=inside(ROOT,args.pin)
    if args.action=='pack':
        if args.media_dir is None:p.error('--media-dir is required to pack')
        if pin_path.exists():p.error('source package pin already exists; use a new version')
        pin=pack_media(ROOT,args.media_dir,args.archive)
        pin_path.write_text(json.dumps(pin,sort_keys=True,indent=2)+'\n')
        print(json.dumps(pin))
    else:
        pin=json.loads(pin_path.read_text())
        out=args.out or ROOT/'.local/contra-media'/('source-'+pin['sha256'])
        print(unpack_media(ROOT,args.archive,pin,out))

if __name__=='__main__':main()
