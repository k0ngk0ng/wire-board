#!/usr/bin/env python3
"""Verify shipped v2 files, immutable source hashes and sprite dimensions."""
import hashlib, json, pathlib, struct, wave
ROOT=pathlib.Path(__file__).resolve().parents[2]
manifest=json.loads((ROOT/'games/bubbleparty/assets/manifest-v2.json').read_text())
assert manifest['imageSource']['sourceCommit']=='7070051995a5feab0ca83eabc708cdecda1e0e74'
assert manifest['licenseStatus']=='Original asset redistribution permission unverified'
assert len(manifest['entries'])>=30
for entry in manifest['entries']:
    p=ROOT/entry['output']; b=p.read_bytes()
    assert entry['urlPath'].startswith('/bubbleparty/v2/')
    assert len(b)==entry['bytes'],p
    assert hashlib.sha256(b).hexdigest()==entry['sha256']==entry['sourceSha256'],p
    assert entry['conversion']=='byte-for-byte copy',p
    if '/audio/' in entry['urlPath']:
        assert entry['sourceType'].startswith('2002 official client'),p
        assert 'sourceCommit' not in entry,p
    else:
        assert entry['sourceCommit']==manifest['imageSource']['sourceCommit'],p
    if p.suffix=='.png':
        assert b[:8]==b'\x89PNG\r\n\x1a\n',p
        assert list(struct.unpack('>II',b[16:24]))==entry['dimensions'],p
    elif p.suffix=='.mp3':
        assert b[:3]==b'ID3' or (b[0]==255 and (b[1]&224)==224),p
        assert entry['archiveMember']=='Fx.idd' and entry['indexOffset']>=0,p
    elif p.suffix=='.wav':
        with wave.open(str(p)) as wav:
            assert wav.getnframes()>0 and wav.getframerate()>0,p
print(json.dumps({'status':'passed','files':len(manifest['entries']),'version':'bubbleparty/v2'},ensure_ascii=False))

# Native selectable characters are a separate immutable version.
import subprocess, sys
subprocess.run([sys.executable, "-B", str(ROOT / "scripts/bubbleparty/check-avatar-assets.py")], cwd=ROOT, check=True)
