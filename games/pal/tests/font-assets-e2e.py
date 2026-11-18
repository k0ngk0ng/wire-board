#!/usr/bin/env python3
"""Before font export: generated glyphs differ from original C, CN overrides
lose to ISO setup, glyph bit order flips, font pixels enter the rule module,
font hashes/lengths are missing, or decompression silently accepts bad sizes.
The final raster is separately compared in visual-browser.py --dialogue.
"""
import gzip
import hashlib
import json
from pathlib import Path
import subprocess
import sys
ROOT=Path(__file__).resolve().parents[3]
subprocess.run([sys.executable,'-B','games/pal/build/fonts.py'],cwd=ROOT,check=True)
base=ROOT/'.local/pal/visual/pal/v1'
index=json.loads((base/'index.json').read_text())
for path,size in [('fonts/glyphs.bin.gz',65536*32),('fonts/widths.bin.gz',65536),('fonts/small.bin.gz',256*8)]:
    data=(base/path).read_bytes();entry=index['files'][path]
    assert entry['size']==len(data) and entry['sha256']==hashlib.sha256(data).hexdigest()
    raw=gzip.decompress(data);assert len(raw)==size
widths=gzip.decompress((base/'fonts/widths.bin.gz').read_bytes())
assert widths[ord('A')]==16 and widths[0xb7]==32 and widths[ord('李')]==32
assert 'fonts.json' in index['files']
print(json.dumps({'passed':True,'scope':'original font/ISO/CN export, exact sizes and registered hashes; browser C raster comparison is separate'}))
