"""Prepare Japanese lifecycle evidence inputs without modifying the source ROM."""
import hashlib,json,zipfile
from pathlib import Path
ROOT=Path(__file__).resolve().parents[2]
BASE=ROOT/'.local/contra-campaign-rules'
ROM=Path('/Users/jason/Documents/nes/roms/03/魂斗罗日版(完全版).zip')
FIXED=[0xc153,0xc162,0xc16f,0xc177,0xc2c1,0xc2cf,0xc307,0xc309,0xca2b,0xcc42,0xcc5f,0xcc76,0xcc7d,0xccc0,0xcccc,0xccdc,0xcce8,0xcd03,0xcd16,0xcd18,0xcd1f,0xcdaf,0xcdf1,0xcdf8,0xce3a,0xce4f,0xce94,0xfa5a,0xfa6a,0xfaa6,0xfaae,0xfac6]
ENDING=[0x8151,0x816a,0x8170,0x817b,0x819f,0x82bc,0x8372,0x8498,0x84bb,0x84da,0x84e8,0x9e22,0xa67c]
SHA='6ba53139fa88b8de1ae527c438bda6f1541d1ee7df26d63dec5164a32d166bfe'

def sha(data):return hashlib.sha256(data).hexdigest()

def main():
    before=sha(ROM.read_bytes())
    with zipfile.ZipFile(ROM) as z:
        assert z.testzip() is None
        raw=z.read('Contra (J).nes')
    assert sha(raw)==SHA
    prg=raw[16:131088];BASE.mkdir(parents=True,exist_ok=True)
    samples=[]
    for name,offset,end,entry,addresses in [('fixed',14*8192+1,16*8192,0xc001,FIXED),('ending',8*8192+0x151,10*8192,0x8151,ENDING)]:
        sample=prg[offset:end];path=BASE/(name+'-prepared.bin');path.write_bytes(sample)
        seeds=BASE/(name+'-prepared-seeds.tsv')
        seeds.write_text(''.join(f'{a:#x}\tfunction\t'+('sound_test_not_campaign' if name=='ending' and a==0xa67c else f'{name}_{a:04x}')+'\n' for a in addresses))
        spec={'target':str(path),'entry':hex(entry),'seeds':str(seeds),'output':str(BASE/(name+'-prepared-rea')),'addresses':[hex(a) for a in addresses],'file_sha256':sha(sample)}
        (BASE/(name+'-prepared-spec.json')).write_text(json.dumps(spec,indent=2)+'\n')
        samples.append({'name':name,'rom_offset':16+offset,'prg_offset':offset,'cpu_start':hex(entry),'bytes':len(sample),'sha256':sha(sample)})
    assert sha(ROM.read_bytes())==before
    facts={'rom_sha256':SHA,'archive_sha256':before,'source_unchanged':True,'slices':samples,
           'routine_table':{'cpu':'0xcc47','values':[hex(int.from_bytes(prg[0x1cc47+i*2:0x1cc49+i*2],'little')) for i in range(12)]},
           'initial_continues':{'cpu':'0xc2c6','bytes':prg[0x1c2c6:0x1c2ca].hex(),'value':3},
           'standard_reserve_lives':{'cpu':'0xc2e9','bytes':prg[0x1c2e9:0x1c2eb].hex(),'value':2},
           'stage_scroll_and_music':{'cpu':'0xccb0','values':list(prg[0x1ccb0:0x1ccc0])},
           'clear_tunes':{'cpu':'0xce32','values':list(prg[0x1ce32:0x1ce3a])}}
    (ROOT/'docs/research/nes-contra/evidence/campaign-source.json').write_text(json.dumps(facts,indent=2)+'\n')
    print(json.dumps({'passed':True,'functions':len(FIXED)+len(ENDING),'slices':samples}))

if __name__=='__main__':main()
