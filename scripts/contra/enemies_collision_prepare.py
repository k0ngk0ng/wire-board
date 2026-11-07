"""Prepare the source-bound fixed Japanese window for private REA collision work."""
from pathlib import Path
import hashlib,json,zipfile

ROOT=Path(__file__).resolve().parents[2]
BASE=ROOT/'.local/contra-enemies-collision'
def main():
    source=Path('/Users/jason/Documents/nes/roms/03/魂斗罗日版(完全版).zip');before=hashlib.sha256(source.read_bytes()).hexdigest()
    with zipfile.ZipFile(source)as z:r=z.read('Contra (J).nes')
    digest=hashlib.sha256(r).hexdigest();assert digest=='6ba53139fa88b8de1ae527c438bda6f1541d1ee7df26d63dec5164a32d166bfe'
    blob=r[16+0x1c001:16+0x20000];BASE.mkdir(parents=True,exist_ok=True);target=BASE/'fixed.bin';target.write_bytes(blob)
    addresses=[0xc33b,0xc343,0xc37e,0xe10a,0xe196,0xe1d0,0xe21d,0xe276,0xe2fe,0xe309,0xe342,0xe3e7,0xe8a6,0xe8d8,0xe8e1,0xe949,0xd79d]
    seeds=BASE/'all-seeds.tsv';seeds.write_text(''.join(f'{hex(a)}\tfunction\tcollision_{a:04x}\n'for a in addresses))
    spec={'target':str(target),'entry':'0xc001','seeds':str(seeds),'output':str(BASE/'all-rea'),'addresses':[hex(a)for a in addresses],'file_sha256':hashlib.sha256(blob).hexdigest()}
    (BASE/'all-spec.json').write_text(json.dumps(spec,indent=2)+'\n')
    identity={'romSHA256':digest,'archiveSHA256':before,'cpuStart':'0xc001','cpuEnd':'0xffff','prgOffset':0x1c001,'romOffset':16+0x1c001,'sha256':spec['file_sha256'],'note':'Fixed bank14/15 window. Tail entries C343/C37E and E8D8 are explicit original fallthrough/call targets, not unrelated cartridge banks.'}
    assert hashlib.sha256(source.read_bytes()).hexdigest()==before
    (BASE/'identity.json').write_text(json.dumps(identity,indent=2)+'\n')
    print(json.dumps({'spec':str(BASE/'all-spec.json'),'identity':identity}))

if __name__=='__main__':main()
