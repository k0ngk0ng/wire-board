"""Observe sound-request return carry against explicit original slot inputs.

Failure cases: constant carry for a sound ID; last target-slot priority ignored;
62/77 and 16-to-18 early returns; direct DPCM/noop returns; wrapper bank restore
mistaken for a flag mutation. The probe stages initial sound-code slots at a
request boundary, then executes unmodified F9BC and records its real return.
No returned flags or sound results are written by the probe.
"""
import gzip
import hashlib
import json
from pathlib import Path
import struct
import zipfile
from audio_export import ROOT,BASE,OUT,SHA,record


def main():
    with zipfile.ZipFile('/Users/jason/Documents/nes/roms/03/魂斗罗日版(完全版).zip')as z:rom=z.read('Contra (J).nes')
    assert hashlib.sha256(rom).hexdigest()==SHA
    prg=rom[16:131088];data=bytearray(prg[0x4000:0x8000]+prg[0x1c000:])
    init=bytes([0x20,0x0b,0xf9,0x20,0x19,0x87,0x60]);play=bytes([0x20,0x19,0x87,0xa9,1,0x20,0xbc,0xf9,0xea,0x60])
    assert data[0x3f00:0x3f00+len(play)]==bytes([255])*len(play)
    data[0x3f00:0x3f00+len(play)]=play;data[0x3fe0:0x3fe0+len(init)]=init
    header=bytearray(128);header[:8]=b'NESM\x1a\x01\x01\x01';struct.pack_into('<HHH',header,8,0x8000,0xbfe0,0xbf00);struct.pack_into('<H',header,110,16639)
    cases=[]
    for sid in range(1,256):
        for slots in [[0]*6,[sid]*6,[126]*6,[0,98,0,0,0,0],[0,119,0,0,22,119]]:
            cases.append({'id':sid,'slots':slots})
    base=BASE/'carry-oracle';base.mkdir(parents=True,exist_ok=True)
    carrier=base/'carry.nsf';carrier.write_bytes(header+data)
    values='{'+','.join('{id='+str(c['id'])+',slots={'+','.join(map(str,c['slots']))+'}}'for c in cases)+'}'
    lua=r'''
local out=assert(os.getenv('NES_AUDIO_OUT'));local f=assert(io.open(out..'/carry.jsonl','w'))
local cases=CASES;local index=0;local selected=nil
memory.registerexec(0xbf05,1,function()
 index=index+1;selected=cases[index];if not selected then return end
 for slot=0,5 do memory.writebyte(0x106+slot,selected.slots[slot+1]) end
 memory.setregister('a',selected.id)
end)
memory.registerexec(0xbf08,1,function()
 if not selected then return end
 local slots={};for slot=0,5 do slots[#slots+1]=tostring(memory.readbyte(0x106+slot))end
 f:write(string.format('{"case":%d,"carry":%d,"after":[%s]}\n',index,AND(memory.getregister('p'),1),table.concat(slots,',')))
end)
emu.speedmode('turbo')
for n=1,tonumber(assert(os.getenv('NES_AUDIO_FRAMES')))do emu.frameadvance() end
f:close();local marker=assert(io.open(out..'/complete','w'));marker:write('complete');marker:close()
while true do end
'''.replace('CASES',values)
    for p in [BASE/'tmp',BASE/'runtime']:p.mkdir(parents=True,exist_ok=True)
    result=base/'v1';record(carrier,result,OUT/'wave-bridge.dylib',len(cases)+4,lua)
    rows=[json.loads(line)for line in (result/'carry.jsonl').read_text().splitlines()]
    assert len(rows)==len(cases)
    fixture={'source_is_original_execution':True,'romSHA256':SHA,'scope':'Controlled initial sound-code slots and A input; original F9BC executes and returns flags. No source slot lifetimes or PCM are inferred from these staged inputs.',
             'carrierSHA256':hashlib.sha256((header+data)).hexdigest(),'cases':[c|r for c,r in zip(cases,rows,strict=True)]}
    target=ROOT/'docs/research/nes-contra/evidence/audio-carry-original.json.gz';target.write_bytes(gzip.compress(json.dumps(fixture,separators=(',',':')).encode(),mtime=0))
    print(json.dumps({'cases':len(rows),'sha256':hashlib.sha256(target.read_bytes()).hexdigest()}))


if __name__=='__main__':main()
