"""Retain source-bound sprite evidence and produce local indexed CHR page assets.

Export checks: all page bitplanes round-trip to original bytes, every sprite
pointer is bounded, evidence files carry SHA-256, and native bundles are kept
complete rather than replacing them with summary claims.
"""
from pathlib import Path
import base64
import gzip
import hashlib
import json
import sys
import zipfile

from PIL import Image, ImageDraw
from sprites_decode import SpriteBank

ROOT=Path(__file__).resolve().parents[2]
BASE=ROOT/'.local/contra-sprites'
OUT=ROOT/'docs/research/nes-contra/evidence'


def sha(raw):return hashlib.sha256(raw).hexdigest()


def main():
    run=Path(sys.argv[1]) if len(sys.argv)>1 else BASE/'runtime/runs/natural-sprites-animations'
    verification=json.loads((run/'verification.json').read_text())
    assert verification['passed']
    with zipfile.ZipFile('/Users/jason/Documents/nes/roms/03/魂斗罗日版(完全版).zip')as z:
        rom=z.read('Contra (J).nes')
    assert sha(rom)=='6ba53139fa88b8de1ae527c438bda6f1541d1ee7df26d63dec5164a32d166bfe'
    prg,chr=rom[16:16+131072],rom[16+131072:]
    bank=SpriteBank(prg)
    sprites=[bank.sprite(code)for code in range(1,256)]
    animation_tables={
        'stationary':{'cpu_address':0xa57e,'values':list(prg[0xa57e:0xa581])},
        'aim_sequence':{'cpu_address':0xa5ba,'values':list(prg[0xa5ba:0xa5c4])},
        'walking_sequences':{'cpu_address':0xa5ce,'values':[list(prg[a:a+6])for a in range(0xa5ce,0xa5ec,6)]},
        'death':{'cpu_address':0xa61b,'values':[list(prg[a:a+2])for a in range(0xa61b,0xa625,2)]},
        'jump':{'cpu_address':0xa673,'values':list(prg[0xa673:0xa677])},
    }
    pages=[];asset_dir=BASE/'assets/chr';asset_dir.mkdir(parents=True,exist_ok=True)
    for page in range(128):
        raw=chr[page*1024:(page+1)*1024];pixels=bytearray(4096);encoded=bytearray(1024)
        for tile in range(64):
            for y in range(8):
                lo,hi=raw[tile*16+y],raw[tile*16+y+8]
                for x in range(8):
                    value=((lo>>(7-x))&1)|(((hi>>(7-x))&1)<<1)
                    pixels[(tile//8*8+y)*64+tile%8*8+x]=value
                    encoded[tile*16+y]|=(value&1)<<(7-x)
                    encoded[tile*16+y+8]|=(value>>1)<<(7-x)
        assert bytes(encoded)==raw
        image=Image.frombytes('P',(64,64),bytes(pixels))
        image.putpalette([0,0,0,85,85,85,170,170,170,255,255,255]+[0]*756)
        path=asset_dir/f'page-{page:03d}.png';image.save(path)
        pages.append({'page_1k':page,'rom_offset':16+131072+page*1024,'source_sha256':sha(raw),
                      'png_sha256':sha(path.read_bytes()),'pixel_indices_sha256':sha(pixels),
                      'width':64,'height':64,'tiles':64,'tile_size':[8,8]})
    mappings=[]
    for p in sorted(run.glob('display-*-ppu.bin')):
        data=p.read_bytes();mapped=[]
        for slot in range(8):
            matches=[n for n in range(128)if data[slot*1024:(slot+1)*1024]==chr[n*1024:(n+1)*1024]]
            assert len(matches)==1,(p.name,slot,matches)
            mapped.append(matches[0])
        mappings.append({'frame':int(p.name.split('-')[1]),'chr_pages':mapped,'palette_codes':list(data[0x3f00:0x3f20])})
    OUT.mkdir(parents=True,exist_ok=True)
    native=(BASE/'sprite-rea/evidence.json').read_bytes()
    (OUT/'sprites-rea.json.gz').write_bytes(gzip.compress(native,mtime=0))
    functions=[]
    for path in sorted((BASE/'sprite-rea').glob('function-*.json')):
        d=json.loads(path.read_text())
        functions.append({'address':d['parameters']['procedure'],'evidence_id':d['evidence_id'],
                          'assembly':d['normalized_result']['assembly'],
                          'limitations':d['limitations']})
    selected=['sprite-calls.jsonl','sprite-calls.bin','dma.bin','display.jsonl','animation.jsonl',
              'nes-colors.rgb','sprite-source.json','scenario.json','frames.csv']
    selected += [p.name for p in sorted(run.glob('display-*-ppu.bin'))]
    selected += [p.name for p in sorted(run.glob('frame-*.png'))]
    files={}
    for name in selected:
        raw=(run/name).read_bytes();files[name]={'sha256':sha(raw),'base64':base64.b64encode(raw).decode()}
    bundle={'rom_sha256':sha(rom),'files':files,'verification':verification}
    (OUT/'sprites-natural.json.gz').write_bytes(gzip.compress(json.dumps(bundle,separators=(',',':')).encode(),mtime=0))
    catalog={'rom_sha256':sha(rom),'sprites':sprites,'animation_tables':animation_tables,'chr_pages':pages,
             'limitations':['Descriptor structure recovered for 255 codes; only recorded codes have natural runtime validation.',
                            'CHR PNGs encode palette indices, not final colors; runtime palette selection remains necessary.',
                            'No claim of complete animation/state-selection implementation.']}
    (OUT/'sprites-catalog.json.gz').write_bytes(gzip.compress(json.dumps(catalog,separators=(',',':')).encode(),mtime=0))
    index={'rom_sha256':sha(rom),'source':json.loads((BASE/'source-manifest.json').read_text()),
           'functions':functions,'verification':verification,'animation_tables':animation_tables,
           'animation_calls':len((run/'animation.jsonl').read_text().splitlines()),'display_mappings':mappings,
           'asset_decode':{'pages':128,'tiles':8192,'roundtrip_bytes':131072},
           'evidence_files':{p.name:sha(p.read_bytes())for p in sorted(OUT.glob('sprites-*.json.gz'))},
           'reference':{'repository':'https://github.com/vermiceli/nes-contra-us','tree':'687d651c021fd7020f10d05b970ccb62663c94bd',
                        'files':{p.name:sha(p.read_bytes())for p in sorted((BASE/'reference').glob('*.asm'))},
                        'role':'Candidate symbols only; Japanese addresses and format checked against local ROM and REA.'}}
    (OUT/'sprites-index.json').write_text(json.dumps(index,indent=2)+'\n')
    comparison=Image.new('RGB',(768,528),(42,42,42));draw=ImageDraw.Draw(comparison)
    for col,frame in enumerate((1,20,180)):
        draw.text((col*256,0),f'Original {frame}',fill='white')
        comparison.paste(Image.open(run/f'frame-{frame:05d}.png'),(col*256,24))
        draw.text((col*256,264),f'Recovered sprites {frame}',fill='white')
        layer=Image.open(run/f'recovered-sprites-{frame:05d}.png')
        comparison.paste(layer,(col*256,288),layer)
    comparison.save(OUT/'sprites-comparison.png')
    print(json.dumps({'sprites':len(sprites),'parts':sum(len(s['parts'])for s in sprites),
                      'chr_pages':len(pages),'animation_calls':index['animation_calls'],'native_functions':len(functions)}))


if __name__=='__main__':main()
