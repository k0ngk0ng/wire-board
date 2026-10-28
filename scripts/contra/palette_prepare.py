"""Bind palette tables and needed additional colors to the Japanese source."""
from pathlib import Path
import argparse,hashlib,json,zipfile

ROOT=Path(__file__).resolve().parents[2]
BASE=ROOT/'.local/contra-palette'
OUT=ROOT/'docs/research/nes-contra/evidence'
ROM_SHA='6ba53139fa88b8de1ae527c438bda6f1541d1ee7df26d63dec5164a32d166bfe'


def main():
 parser=argparse.ArgumentParser();parser.add_argument('--colors-only',action='store_true');args=parser.parse_args()
 archive=Path('/Users/jason/Documents/nes/roms/03/魂斗罗日版(完全版).zip')
 source_hash=hashlib.sha256(archive.read_bytes()).hexdigest()
 with zipfile.ZipFile(archive)as z:rom=z.read('Contra (J).nes')
 assert hashlib.sha256(rom).hexdigest()==ROM_SHA
 prg=rom[16:16+131072];BASE.mkdir(parents=True,exist_ok=True);OUT.mkdir(parents=True,exist_ok=True)
 tables={}
 for name,address,size in [('fade',0xcf64,9),('cycleCounts',0xd03d,9),('baseBossPalette2',0xd046,8),
                           ('flash',0xd04e,4),('ordinaryPalette2',0xd052,4),('endingPalette2',0xd056,4),
                           ('snowProgram',0xce87,13)]:
  raw=prg[address+0x10000:address+0x10000+size]
  tables[name]={'cpu':address,'romOffset':16+address+0x10000,'bytes':list(raw),'sha256':hashlib.sha256(raw).hexdigest()}
 # Snow tank initialization and HP-dependent palettes (A435/A6A5), plus
 # energy-zone giant HP palettes (bank0/1 ACB8..ACD4; IDs are hex 51/52).
 runtime_requirements=[
  {'name':'snow-enemies','stage':5,'paletteIndices':[0x3f,0x41,0x5f,0x60,0x61]},
  {'name':'energy-giant-hp','stage':6,'paletteIndices':[0x51,0x52],'paletteSlot':7,
   'consumerSource':{'cpuStart':0xacb8,'cpuEndInclusive':0xacd4,'prgOffset':0x2cb8,'romOffset':16+0x2cb8,
                     'bank8KiB':1,'sha256':hashlib.sha256(prg[0x2cb8:0x2cd5]).hexdigest()}},
 ]
 needed=sorted(set(tables['baseBossPalette2']['bytes']+tables['ordinaryPalette2']['bytes']+tables['endingPalette2']['bytes']+
                   [i for requirement in runtime_requirements for i in requirement['paletteIndices']]))
 color_base=int.from_bytes(prg[0x1d0e1:0x1d0e3],'little');assert color_base==0xd0e3
 colors=[]
 for index in needed:
  address=color_base+index*3;raw=prg[0x10000+address:0x10000+address+3];assert len(raw)==3 and max(raw)<64
  colors.append({'id':index,'colors':list(raw),'source':{'cpu':address,'romOffset':16+0x10000+address,'sha256':hashlib.sha256(raw).hexdigest()}})
 assert hashlib.sha256(archive.read_bytes()).hexdigest()==source_hash
 (OUT/'palette-additional-colors.json').write_text(json.dumps({'romSHA256':ROM_SHA,'sourceArchiveSHA256':source_hash,
  'sourceArchiveUnchanged':True,'palettePointerBase':color_base,'runtimePaletteRequirements':runtime_requirements,'colors':colors,'tables':tables},indent=2)+'\n')
 if args.colors_only:
  print(json.dumps({'colors':needed,'sourceUnchanged':True}));return
 raw=prg[0x1c001:];target=BASE/'fixed.bin';target.write_bytes(raw)
 seeds={'0xc001':'frame_entry','0xcc02':'commit_palette','0xcee3':'load_palette_colors',
        '0xcf6d':'palette_cycle_and_flash','0xce59':'snow_visibility_program'}
 seed=BASE/'seeds.tsv';seed.write_text(''.join(f'{a}\tfunction\t{n}\n'for a,n in seeds.items()))
 spec={'target':str(target),'entry':'0xc001','seeds':str(seed),'output':str(BASE/'rea'),
       'addresses':list(seeds),'file_sha256':hashlib.sha256(raw).hexdigest()}
 (BASE/'spec.json').write_text(json.dumps(spec,indent=2)+'\n')
 print(json.dumps({'colors':needed,'spec':str(BASE/'spec.json')}))


if __name__=='__main__':main()
