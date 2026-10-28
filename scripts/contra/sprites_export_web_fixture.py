"""Normalize recorded original sprite calls for the pure TypeScript data layer.

This exports expected output from the original emulator recordings, never from
the TypeScript implementation or the Python sprite composer.
"""
from pathlib import Path
import base64
import gzip
import hashlib
import json
from io import BytesIO
from PIL import Image

ROOT=Path(__file__).resolve().parents[2]
EVIDENCE=ROOT/'docs/research/nes-contra/evidence'


def main():
    original_path=EVIDENCE/'sprites-natural.json.gz'
    catalog_path=EVIDENCE/'sprites-catalog.json.gz'
    original=json.loads(gzip.decompress(original_path.read_bytes()))
    catalog=json.loads(gzip.decompress(catalog_path.read_bytes()))
    def read(name):
        entry=original['files'][name];data=base64.b64decode(entry['base64'],validate=True)
        assert hashlib.sha256(data).hexdigest()==entry['sha256']
        return data
    calls=[json.loads(s)for s in read('sprite-calls.jsonl').decode().splitlines()]
    raw=read('sprite-calls.bin')
    for i,call in enumerate(calls):
        call['before']=raw[i*512:i*512+256].hex()
        call['after']=raw[i*512+256:(i+1)*512].hex()
    displays=[json.loads(s)for s in read('display.jsonl').decode().splitlines()]
    dma=read('dma.bin');rgb=read('nes-colors.rgb')
    pages={};samples=[]
    for name in sorted(original['files']):
        if not name.startswith('display-')or not name.endswith('-ppu.bin'):continue
        frame=int(name.split('-')[1]);ppu=read(name);mapping=[]
        for slot in range(8):
            page=ppu[slot*1024:(slot+1)*1024];identity=hashlib.sha256(page).hexdigest()
            pages[identity]=page.hex();mapping.append(identity)
        image=Image.open(BytesIO(read(f'frame-{frame:05d}.png'))).convert('RGB')
        samples.append({'frame':frame,'control':displays[frame-1]['ctrl'],'mask':displays[frame-1]['mask'],
                        'oam':dma[(frame-1)*256:frame*256].hex(),'mapping':mapping,
                        'palette':ppu[0x3f00:0x3f20].hex(),
                        'expectedRGB':base64.b64encode(image.tobytes()).decode()})
    descriptor={'format':'contra-j-sprites-v1','sourceRomSHA256':catalog['rom_sha256'],
                'anchor':'original-sprite-position','frames':catalog['sprites']}
    fixture={'source':{'romSHA256':original['rom_sha256'],
                       'originalEvidenceSHA256':hashlib.sha256(original_path.read_bytes()).hexdigest(),
                       'catalogEvidenceSHA256':hashlib.sha256(catalog_path.read_bytes()).hexdigest(),
                       'expectedOutputs':'unmodified original FCEUX execution'},
             'catalog':descriptor,'calls':calls,'pages':pages,'rgb':rgb.hex(),'samples':samples}
    out=ROOT/'games/contra/tests/testdata';out.mkdir(parents=True,exist_ok=True)
    payload=gzip.compress(json.dumps(fixture,separators=(',',':')).encode(),mtime=0)
    (out/'sprites-original.json.gz').write_bytes(payload)
    source={**fixture['source'],'fixtureSHA256':hashlib.sha256(payload).hexdigest(),
            'calls':len(calls),'samples':len(samples),'parts':sum(len(f['parts'])for f in descriptor['frames']),
            'limitations':['Native 607-call fixture covers 14 sprite codes in outdoor stage one.',
                           'Other declared boundary tests are synthetic byte/format cases, not additional natural gameplay.']}
    (out/'sprites-original-source.json').write_text(json.dumps(source,indent=2)+'\n')
    assets=ROOT/'games/contra/assets';assets.mkdir(parents=True,exist_ok=True)
    asset_manifest={'format':descriptor['format'],'sourceRomSHA256':original['rom_sha256'],
                    'descriptorSource':'docs/research/nes-contra/evidence/sprites-catalog.json.gz',
                    'descriptorSourceSHA256':fixture['source']['catalogEvidenceSHA256'],
                    'rawROMRuntimeDependency':False,'publication':'not published',
                    'anchor':descriptor['anchor'],'frames':255,'parts':source['parts'],
                    'chrLocalResearchPath':'.local/contra-sprites/assets/chr',
                    'distribution':'A future immutable asset release must bind normalized descriptors and only required CHR pages to content hashes.'}
    (assets/'sprites-source.json').write_text(json.dumps(asset_manifest,indent=2)+'\n')
    print(json.dumps(source))


if __name__=='__main__':main()
