"""Prepare only Guardian/Heart REA inputs; never overwrite general alien tables."""
from pathlib import Path
import hashlib,json,zipfile
ROOT=Path(__file__).resolve().parents[2];BASE=ROOT/'.local/contra-enemies-aliens-boss'

def main():
    source=Path('/Users/jason/Documents/nes/roms/03/魂斗罗日版(完全版).zip')
    with zipfile.ZipFile(source)as archive:rom=archive.read('Contra (J).nes')
    digest=hashlib.sha256(rom).hexdigest();assert digest=='6ba53139fa88b8de1ae527c438bda6f1541d1ee7df26d63dec5164a32d166bfe'
    prg=rom[16:131088];window=prg[7:0x4000]+prg[0x1c000:];BASE.mkdir(parents=True,exist_ok=True);(BASE/'window.bin').write_bytes(window)
    groups={'main':[0xb463,0xb470,0xb4a1,0xb56a,0xb6c0,0xb6d7,0xb597,0xb5fd,0xb626,0xb648,0xb668,0xb69b,0xbd27,0xe73a,0xbcb7,0xbcc2,0xbcd9,0xbd71,0xbd10,0xbd1d],
            'helpers':[0xb46b,0xb486,0xb4ee,0xb50a,0xb522,0xb5ab,0xb686,0xb672,0xb6fd,0xbd8c,0xed5e],
            'draw':[0xb540,0xeda0,0xe989,0xea98]}
    for name,addresses in groups.items():
        seeds=BASE/(name+'-seeds.tsv');seeds.write_text(''.join(f'{hex(a)}\tfunction\talien_boss_{a:04x}\n'for a in addresses))
        spec={'target':str(BASE/'window.bin'),'entry':'0x8007','seeds':str(seeds),'output':str(BASE/(name+'-rea')),'addresses':[hex(a)for a in addresses],
              'file_sha256':hashlib.sha256(window).hexdigest(),'tmp':str(ROOT/'dist/contra-enemies-aliens-boss-tmp'),'cache':str(BASE/'swift-cache')}
        (BASE/(name+'-spec.json')).write_text(json.dumps(spec,indent=2)+'\n')
    tables=[]
    for name,address,size in [('boss-patterns',0xb5cd,48),('explosion-offsets',0xbd59,24)]:
        offset=16+address-0x8000;raw=rom[offset:offset+size];tables.append({'name':name,'address':hex(address),'romOffset':offset,'hex':raw.hex(),'sha256':hashlib.sha256(raw).hexdigest()})
    identity={'romSHA256':digest,'archiveSHA256':hashlib.sha256(source.read_bytes()).hexdigest(),'windowSHA256':hashlib.sha256(window).hexdigest(),'mapping':'bank0/1 at 8000-BFFF, fixed14/15 at C000-FFFF; raw file begins 8007','tables':tables,'groups':groups}
    (BASE/'identity.json').write_text(json.dumps(identity,indent=2)+'\n');print(json.dumps({'groups':list(groups),'windowSHA256':identity['windowSHA256']}))

if __name__=='__main__':main()
