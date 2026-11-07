"""Bind initialization analysis to the authenticated Japanese cartridge.

Failures considered before implementation: clearing unrelated zero-page state;
retaining transient actor/enemy/projectile coordinates; losing lives/weapons;
initializing the wrong map half/vertical cursor; palette fade called once instead
of twice for bases; copied CHR/pages becoming visible early; unconditional demo
music; duplicate routine advancement; and partial mutation on absent resources.
"""
import hashlib,json,zipfile
from pathlib import Path
ROOT=Path(__file__).resolve().parents[2]
BASE=ROOT/'.local/contra-campaign-initialize'
ROM=Path('/Users/jason/Documents/nes/roms/03/魂斗罗日版(完全版).zip')
SHA='6ba53139fa88b8de1ae527c438bda6f1541d1ee7df26d63dec5164a32d166bfe'
GROUPS={
 'fixed':(14*8192+1,16*8192,0xc001,[0xcc5f,0xcc7d,0xc328,0xc310,0xdad3,0xe09a,0xc776,0xc778,0xc910,0xc93a,0xf916,0xf946]),
 'header':(4*8192+1,6*8192,0x8001,[0xa816]),
}
def main():
 before=hashlib.sha256(ROM.read_bytes()).hexdigest()
 with zipfile.ZipFile(ROM) as archive:raw=archive.read('Contra (J).nes')
 assert hashlib.sha256(raw).hexdigest()==SHA
 BASE.mkdir(parents=True,exist_ok=True)
 for name,(start,end,entry,addresses) in GROUPS.items():
  path=BASE/(name+'.bin');path.write_bytes(raw[16+start:16+end])
  seeds=BASE/(name+'-seeds.tsv');seeds.write_text(''.join(f'{a:#x}\tfunction\tinitialize_{a:04x}\n' for a in addresses))
  spec={'target':str(path),'entry':hex(entry),'seeds':str(seeds),'output':str(BASE/(name+'-rea')),'addresses':[hex(a) for a in addresses],'file_sha256':hashlib.sha256(path.read_bytes()).hexdigest(),'rom_sha256':SHA,'prg_offset':start,'tmp':str(ROOT/'dist/cci-tmp'),'cache':str(BASE/'swift-cache')}
  (BASE/(name+'-spec.json')).write_text(json.dumps(spec,indent=2)+'\n')
 spec=json.loads((BASE/'fixed-spec.json').read_text())
 spec['addresses']=['0xc314','0xc912','0xdafa']
 spec['output']=str(BASE/'helpers-rea')
 seeds=BASE/'helpers-seeds.tsv'
 seeds.write_text(''.join(f'{a}\tfunction\tinitialize_helper_{a[2:]}\n' for a in spec['addresses']+['0xdb1c','0xe10a']))
 spec['seeds']=str(seeds)
 (BASE/'helpers-spec.json').write_text(json.dumps(spec,indent=2)+'\n')
 assert hashlib.sha256(ROM.read_bytes()).hexdigest()==before
 print(json.dumps({'passed':True,'groups':list(GROUPS)+['helpers'],'source_unchanged':True}))
if __name__=='__main__':main()
