"""Pin original campaign scene analysis before writing independent rules.

Failures considered: bank alias mistaken for original code; last boss death
standing in for a real exit; shared palette/queue duplicated; delayed queue
submission advancing too soon; entrance column counters wrapping incorrectly;
input cleared before autonomous exit movement; out/dead players revived; timer
underflow and odd-frame expiry; restore emitting a second stage transition;
partial scene mutation on unsupported stage/data/graphics capacity errors.
"""
import hashlib,json,zipfile
from pathlib import Path
ROOT=Path(__file__).resolve().parents[2]
BASE=ROOT/'.local/contra-campaign-scene'
ROM=Path('/Users/jason/Documents/nes/roms/03/魂斗罗日版(完全版).zip')
SHA='6ba53139fa88b8de1ae527c438bda6f1541d1ee7df26d63dec5164a32d166bfe'
GROUPS={
 'exit':(10*8192+0x88b,10*8192+0xa6a,0x888b,[0x888b,0x88a2,0x88cf,0x8910,0x8925,0x8952,0x8957,0x89a6,0x89df,0x89f6,0x89fa,0x89fe,0x8a0b,0x8a1b,0x8a45,0x8a57]),
 'entrance':(14*8192+0x1b1c,14*8192+0x2093,0xdb1c,[0xdb1c,0xdd4b,0xddef,0xde79,0xdec7,0xdf4e,0xe03d]),
 'intro_helpers_v2':(14*8192+1,16*8192,0xc001,[0xc93a,0xc9c9,0xca17,0xca2b,0xcac5,0xc778,0xc910,0xf946,0xc32e]),
 'interlude':(8*8192+0x1e22,10*8192,0x9e22,[0x9e22,0x9e7b,0x9f3f,0x9fc6,0x9ffe,0xa00d,0xa01c,0xa047,0xa04a,0xa060,0xa075,0xa080,0xa087,0xa08d,0xa099,0xa0a7,0xa0ac,0xa0bb,0xa150,0xa155,0xa15d,0xa165,0xa371,0xa37a,0xa3e6,0xa427,0xa430]),
 'graphics_mode':(14*8192+1,16*8192,0xc001,[0xcb30,0xcb37,0xcb9e,0xcbc0,0xcc7d]),
 'intro':(10*8192+0x2d7,10*8192+0x5d3,0x82d7,[0x82d7,0x8302,0x8316,0x8328,0x83a1,0x83ad,0x83e7,0x83ef,0x841c,0x842a,0x8463,0x8483,0x8507,0x8511,0x8527,0x854b])}
def main():
 before=hashlib.sha256(ROM.read_bytes()).hexdigest()
 raw=zipfile.ZipFile(ROM).read('Contra (J).nes');assert hashlib.sha256(raw).hexdigest()==SHA
 prg=raw[16:];BASE.mkdir(parents=True,exist_ok=True)
 for name,(start,end,entry,addresses) in GROUPS.items():
  path=BASE/(name+'.bin');path.write_bytes(prg[start:end]);seeds=BASE/(name+'-seeds.tsv');seeds.write_text(''.join(f'{a:#x}\tfunction\tscene_{a:04x}\n' for a in addresses))
  spec={'target':str(path),'entry':hex(entry),'seeds':str(seeds),'output':str(BASE/(name+'-rea')),'addresses':[hex(a) for a in addresses],'file_sha256':hashlib.sha256(path.read_bytes()).hexdigest(),'rom_sha256':SHA,'prg_offset':start}
  (BASE/(name+'-spec.json')).write_text(json.dumps(spec,indent=2)+'\n')
 assert hashlib.sha256(ROM.read_bytes()).hexdigest()==before
 print(json.dumps({'passed':True,'groups':list(GROUPS),'source_unchanged':True}))
if __name__=='__main__':main()
