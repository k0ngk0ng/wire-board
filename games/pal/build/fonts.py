#!/usr/bin/env python3
"""Add lossless, hashed original font assets to the local visual export."""
import gzip
import hashlib
import json
from pathlib import Path
from font_data import glyph_data
ROOT=Path(__file__).resolve().parents[3]

def main():
    vendor=ROOT/'games/pal/native/upstream'
    manifest=json.loads((vendor.parent/'upstream-manifest.json').read_text())
    sources=['fontglyph.h','fontglyph_cn.h','ascii.h']
    for name in sources:
        expected=next(item for item in manifest['files'] if item['path']==name)
        data=(vendor/name).read_bytes()
        if len(data)!=expected['size'] or hashlib.sha256(data).hexdigest()!=expected['sha256']:raise ValueError('font source identity differs')
    base=ROOT/'.local/pal/visual/pal/v1'
    index=json.loads((base/'index.json').read_text())
    if index['version']!=1:raise ValueError('unsupported visual asset version')
    def write(name,data,mime):
        target=base/name;target.parent.mkdir(parents=True,exist_ok=True);target.write_bytes(data)
        index['files'][name]={'size':len(data),'sha256':hashlib.sha256(data).hexdigest(),'mime':mime}
    glyphs,widths,small=glyph_data(vendor)
    for name,data in [('glyphs',glyphs),('widths',widths),('small',small)]:write('fonts/'+name+'.bin.gz',gzip.compress(data,mtime=0),'application/octet-stream')
    meta={'version':1,'height':16,'count':65536,'stride':32,'glyphs':'fonts/glyphs.bin.gz','widths':'fonts/widths.bin.gz','small':'fonts/small.bin.gz','license':'fonts/license.txt'}
    write('fonts.json',(json.dumps(meta,separators=(',',':'))+'\n').encode(),'application/json')
    license='\n\n'.join((vendor/name).read_text().split('#ifndef')[0] for name in sources)+'\n\n'+(vendor/'LICENSE').read_text()
    write('fonts/license.txt',license.encode(),'text/plain')
    for name in sources:index['sources']['font/'+name]=hashlib.sha256((vendor/name).read_bytes()).hexdigest()
    (base/'index.json').write_text(json.dumps(index,separators=(',',':'))+'\n')
    print(json.dumps({'passed':True,'glyphs':65536,'fontBytes':sum(index['files'][path]['size'] for path in index['files'] if path.startswith('fonts/'))}))

if __name__=='__main__':main()
