"""Prepare source-bound Japanese weapon slices for private REA analysis."""
import hashlib,json,zipfile
from pathlib import Path
ROOT=Path(__file__).resolve().parents[2]
OUT=ROOT/'.local/contra-weapons'
ROM=Path('/Users/jason/Documents/nes/roms/03/魂斗罗日版(完全版).zip')
ROM_SHA='6ba53139fa88b8de1ae527c438bda6f1541d1ee7df26d63dec5164a32d166bfe'
ENTRIES=[0x8001,0x8043,0x808f,0x8094,0x809a,0x809f,0x80a8,0x80b0,0x80b6,0x80e8,0x80f3,0x8104,0x811d,0x8142,0x8176,0x819f,0x81a1,0x81a3,0x81b6,0x81eb,0x81fe,0x828c,0x82d8,0x83e3,0x8411,0x8421,0x8475,0x848a,0x8584,0x859b,0x861a,0x861d,0x8669,0x866c,0x867c,0x8680,0x8689,0x869a,0x86b6,0x86bc,0x86c5,0x86e1,0x86e7,0x8774,0x878c,0x87a4,0x87b7,0x87cb,0x87e3,0x87f7,0x880c,0x8822,0x8858,0x886a,0x88d8,0x8927,0x8957]

def digest(raw):return hashlib.sha256(raw).hexdigest()

def main():
    source_digest=digest(ROM.read_bytes())
    with zipfile.ZipFile(ROM) as archive:
        assert archive.testzip() is None
        raw=archive.read('Contra (J).nes')
    assert digest(raw)==ROM_SHA
    prg=raw[16:16+131072]
    assert prg[12*8192]==0x1c
    OUT.mkdir(parents=True,exist_ok=True)
    reports=[]
    for name,offset,entry,entries in [('weapon',12*8192+1,0x8001,ENTRIES),('fixed',14*8192+1,0xc001,[0xd80b,0xe2fe])]:
        end=14*8192 if name=='weapon' else 16*8192
        sample=prg[offset:end];target=OUT/(name+'-slice.bin');target.write_bytes(sample)
        seeds=OUT/(name+'-seeds.tsv');seeds.write_text(''.join(f'{a:#x}\tfunction\tweapon_{a:04x}\n' for a in entries))
        spec={'target':str(target),'entry':hex(entry),'seeds':str(seeds),'output':str(OUT/(name+'-rea')),'addresses':[hex(a) for a in entries],'file_sha256':digest(sample)}
        (OUT/(name+'-spec.json')).write_text(json.dumps(spec,indent=2)+'\n')
        reports.append({'name':name,'rom_offset':16+offset,'prg_offset':offset,'cpu_start':hex(entry),'bytes':len(sample),'sha256':digest(sample)})
    assert digest(ROM.read_bytes())==source_digest
    (OUT/'source.json').write_text(json.dumps({'rom_sha256':ROM_SHA,'archive_sha256':source_digest,'slices':reports,'source_unchanged':True},indent=2)+'\n')
    print(json.dumps({'passed':True,'slices':reports}))

if __name__=='__main__':main()
