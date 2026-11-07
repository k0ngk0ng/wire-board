#!/usr/bin/env python3
"""Prepare Japanese terrain streaming entries for private, source-bound REA."""
import hashlib
import json
from pathlib import Path
import zipfile

ROOT=Path(__file__).resolve().parents[2]
BASE=ROOT/'.local/contra-terrain-scroll'
ENTRIES={'0xdb81':'terrain_scroll','0xdc0d':'vertical_scroll','0xdc93':'base_scroll',
         '0xdd4b':'tile_column','0xdd97':'expand_tile_column','0xddef':'tile_row',
         '0xde79':'attribute_column','0xdec7':'attribute_row','0xdf30':'write_collision',
         '0xe09c':'load_map','0xc76e':'alternate_chr','0xcec6':'alternate_header',
         '0xcee3':'load_palette','0xdb1c':'initial_background_progress'}


def main():
    path=Path('/Users/jason/Documents/nes/roms/03/魂斗罗日版(完全版).zip')
    before=hashlib.sha256(path.read_bytes()).hexdigest()
    with zipfile.ZipFile(path) as archive:raw=archive.read('Contra (J).nes')
    assert hashlib.sha256(raw).hexdigest()=='6ba53139fa88b8de1ae527c438bda6f1541d1ee7df26d63dec5164a32d166bfe'
    BASE.mkdir(parents=True,exist_ok=True)
    data=raw[16+14*8192+1:16+131072];target=BASE/'fixed-c001.bin';target.write_bytes(data)
    seeds=BASE/'seeds.tsv';seeds.write_text(''.join(f'{a}\tfunction\t{n}\n' for a,n in ENTRIES.items()))
    spec={'target':str(target),'entry':'0xc001','file_sha256':hashlib.sha256(data).hexdigest(),
          'seeds':str(seeds),'addresses':list(ENTRIES),'output':str(BASE/'rea'),
          'tmp':str(ROOT/'dist/cts-tmp'),'cache':str(BASE/'swift-cache')}
    (BASE/'spec.json').write_text(json.dumps(spec,indent=2)+'\n')
    assert hashlib.sha256(path.read_bytes()).hexdigest()==before
    print(json.dumps({'source_unchanged':True,'functions':len(ENTRIES),'spec':str(BASE/'spec.json')}))


if __name__=='__main__':main()
