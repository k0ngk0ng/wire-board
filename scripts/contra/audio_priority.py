"""Probe same-tick request arbitration in the unmodified original sound driver.

Failures: globally picking the largest ID loses disjoint slots; playing every
request multiplies same-tick spread-gun volume; lower requests interrupt higher
ones; equality treated as a no-op rather than retrigger; stage-clear/heart special
guards omitted; pause cue confused with the separate game pause state.
The only carrier intervention is a bounded sequence of original API calls before
the first original audio update. No sound-slot or APU results are fabricated.
"""
import hashlib
import json
from pathlib import Path
import struct
import zipfile

from audio_export import ROOT,BASE,OUT,SHA,LUA,record

CASES=[
    ('spread-one',[0x28],[0,0,0,0,0x28,0x28]),
    ('spread-five',[0x28]*5,[0,0,0,0,0x28,0x28]),
    ('normal-machine',[0x1a,0x1c],[0,0,0,0,0x1c,0x1c]),
    ('machine-normal',[0x1c,0x1a],[0,0,0,0,0x1c,0x1c]),
    ('normal-explosion',[0x1a,0x2f],[0,0,0,0,0x2f,0x1a]),
    ('explosion-normal',[0x2f,0x1a],[0,0,0,0,0x2f,0x1a]),
    ('normal-pickup',[0x1a,0x38],[0,0,0,0,0x38,0x1a]),
    ('death-explosion',[0x72,0x2f],[0,0,0,0,0x72,0x72]),
    ('normal-twice',[0x1a,0x1a],[0,0,0,0,0x1a,0x1a]),
    ('jungle-normal',[0x46,0x1a],[0x46,0x46,0x46,0x46,0x1a,0x1a]),
    ('jungle-pause-cue',[0x46,0x74],[0x46,0x46,0x46,0x46,0x74,0]),
    ('clear-blocks-shot',[0x62,0x1a],[0x62,0x62,0x62,0x62,0,0]),
    ('clear-allows-landing',[0x62,0x0b],[0x62,0x62,0x62,0x62,0x0b,0x0b]),
    ('heart-blocks-clear',[0x77,0x62],[0,0x77,0,0x77,0x77,0x77]),
    ('jungle-death-percussion',[0x46,0x72],[0x46,0x46,0x46,0x46,0x72,0x72]),
    ('energy-death-percussion',[0x52,0x72],[0x52,0x52,0x52,0x52,0x72,0x72]),
]


def main():
    with zipfile.ZipFile('/Users/jason/Documents/nes/roms/03/魂斗罗日版(完全版).zip') as z:rom=z.read('Contra (J).nes')
    assert hashlib.sha256(rom).hexdigest()==SHA
    original=rom[16+0x4000:16+0x8000]+rom[16+0x1c000:16+0x20000]
    bridge=OUT/'wave-bridge.dylib';assert bridge.exists(),'run audio_export.py before this probe'
    special="""local firstUpdate=true
memory.registerexec(0x80d5,1,function()
 if firstUpdate then
  firstUpdate=false;local f=assert(io.open(out..'/before-first-update.bin','wb'));f:write(memory.readbyterange(0x100,130));f:close()
 end
 calls:write(string.format('%d,update,0\\n',emu.framecount()))
end)"""
    old="memory.registerexec(0x80d5,1,function()calls:write(string.format('%d,update,0\\n',emu.framecount()))end)"
    assert old in LUA
    lua=LUA.replace(old,special)
    results=[]
    for name,requests,expected in CASES:
        data=bytearray(original)
        wrapper=bytes([0x20,0x0b,0xf9,0x20,0x19,0x87])+b''.join(bytes([0xa9,i,0x20,0xa2,0x87]) for i in requests)+bytes([0x60])
        assert len(wrapper)<256 and data[0x3f00:0x3f00+len(wrapper)]==bytes([255])*len(wrapper)
        data[0x3f00:0x3f00+len(wrapper)]=wrapper
        header=bytearray(128);header[:8]=b'NESM\x1a\x01\x01\x01';struct.pack_into('<HHH',header,8,0x8000,0xbf00,0x80d5);struct.pack_into('<H',header,110,16639)
        carrier=OUT/f'priority-{name}.nsf';carrier.write_bytes(header+data)
        path=OUT/f'priority-{name}';record(carrier,path,bridge,180 if name.endswith('death-percussion') else 8,lua)
        state=(path/'before-first-update.bin').read_bytes();slots=list(state[6:12]);assert slots==expected,(name,slots,expected)
        results.append({'name':name,'requests':requests,'slots':slots,'driverBeforeUpdate':state.hex(),
                        'carrierSHA256':hashlib.sha256(carrier.read_bytes()).hexdigest(),
                        'apuSHA256':hashlib.sha256((path/'apu.csv').read_bytes()).hexdigest()})
    byname={r['name']:r for r in results}
    # Nonfunctional scratch pointer INIT_SOUND_CODE is the same for this pair;
    # all driver RAM has the same state before rendering any samples.
    assert byname['spread-one']['driverBeforeUpdate']==byname['spread-five']['driverBeforeUpdate']
    assert byname['normal-machine']['slots']==byname['machine-normal']['slots']
    import csv
    percussion=OUT/'priority-energy-death-percussion'
    calls=list(csv.DictReader((percussion/'calls.csv').open()))
    driver=(percussion/'driver.bin').read_bytes()
    live=[r for r in calls if r['kind']=='request' and int(r['value'])>=0x7f and driver[int(r['frame'])*130+11]==0x72]
    noise=[r for r in calls if r['kind']=='request' and int(r['value'])==2 and driver[int(r['frame'])*130+11]==0x72]
    apu=list(csv.DictReader((percussion/'apu.csv').open()))
    assert live and noise
    assert all(any(int(a['frame'])==int(r['frame']) and int(a['address'])==0x4015 and int(a['value'])&16 for a in apu) for r in live)
    out=ROOT/'docs/research/nes-contra/evidence/audio-priority.json'
    out.write_text(json.dumps({'passed':True,'romSHA256':SHA,'scope':'Explicit same-tick original sound-request API sequences, before the first update; not concurrent-media/browser equivalence.',
       'cases':results,'percussionDuringDeath':{'dpcmRequestsWhileSlot5Occupied':live,'rejectedNoiseRequestsWhileSlot5Occupied':noise},
       'rules':{'slotPriority':'Higher or equal sound ID replaces only the target slot; lower ID is ignored.',
       'sameTickDuplicates':'Repeated identical requests before a driver update yield identical driver state, not multiple voices.',
       'partialOverride':'A one-slot sound can replace pulse effect slot4 while retaining another sound in noise effect slot5.',
       'pause':'0x74 is only the pause cue. Game pause byte0x25 independently mutes/restores music in $809F.',
       'notProven':'Cross-tick same-ID retrigger timing and independent media channel mix are not deduplicated by this experiment.'}},indent=2)+'\n')
    print(json.dumps({'passed':True,'cases':len(results)}))


if __name__=='__main__':main()
