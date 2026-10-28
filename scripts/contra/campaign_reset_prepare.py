"""Prepare source-bound REA reset/menu evidence without modifying the ROM."""
import hashlib,json,zipfile
from pathlib import Path
ROOT=Path(__file__).resolve().parents[2]
BASE=ROOT/'.local/contra-campaign-reset'
ROM=Path('/Users/jason/Documents/nes/roms/03/魂斗罗日版(完全版).zip')
SHA='6ba53139fa88b8de1ae527c438bda6f1541d1ee7df26d63dec5164a32d166bfe'
ADDRESSES=[0xcd1f,0xc309,0xc2cf,0xcdaf,0xcdf1,0xca2b,0xca62,0xc70f]
def main():
 before=hashlib.sha256(ROM.read_bytes()).hexdigest()
 with zipfile.ZipFile(ROM)as z:raw=z.read('Contra (J).nes')
 assert hashlib.sha256(raw).hexdigest()==SHA
 BASE.mkdir(parents=True,exist_ok=True)
 data=raw[16+14*8192+1:16+16*8192];path=BASE/'fixed.bin';path.write_bytes(data)
 seeds=BASE/'fixed-seeds.tsv';seeds.write_text(''.join(f'{a:#x}\tfunction\treset_{a:04x}\n'for a in ADDRESSES))
 spec={'target':str(path),'entry':'0xc001','seeds':str(seeds),'output':str(BASE/'fixed-rea'),'addresses':[hex(a)for a in ADDRESSES],'file_sha256':hashlib.sha256(data).hexdigest(),'tmp':str(ROOT/'dist/cr-tmp'),'cache':str(BASE/'swift-cache')}
 (BASE/'fixed-spec.json').write_text(json.dumps(spec,indent=2)+'\n')
 assert hashlib.sha256(ROM.read_bytes()).hexdigest()==before
 report={'rom_sha256':SHA,'archive_sha256':before,'source_unchanged':True,'raw_slice':{'rom_offset':16+14*8192+1,'cpu_start':'0xc001','bytes':len(data),'sha256':spec['file_sha256']},'cursor_y':{'cpu':'0xc0a4','rom_offset':16+14*8192+0xa4,'bytes':list(raw[16+14*8192+0xa4:16+14*8192+0xa6])},'functions':spec['addresses']}
 (ROOT/'docs/research/nes-contra/evidence/campaign-reset-source.json').write_text(json.dumps(report,indent=2)+'\n')
 print(json.dumps({'passed':True,'functions':len(ADDRESSES),'cursor_y':report['cursor_y']['bytes']}))
if __name__=='__main__':main()
