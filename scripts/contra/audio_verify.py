"""Verify retained original-APU audio captures before accepting export output.

Failure inventory: modified driver/data, wrong region, missing/duplicate sounds,
non-deterministic PCM/APU writes, silent or clipped files, truncated effect tails,
unproven music loop represented as seamless, and audio claims exceeding capture.
"""
import array
import argparse
import csv
import hashlib
import json
from pathlib import Path
import sys
import wave
import zipfile

ROOT=Path(__file__).resolve().parents[2]


def sha(p):return hashlib.sha256(p.read_bytes()).hexdigest()
def pcm(p):
    with wave.open(str(p)) as wav:
        assert (wav.getnchannels(),wav.getsampwidth(),wav.getframerate())==(1,2,44100)
        out=array.array('h',wav.readframes(wav.getnframes()))
    if sys.byteorder!='little':out.byteswap()
    return out


def main():
    parser=argparse.ArgumentParser();parser.add_argument('--manifest',type=Path,default=ROOT/'.local/contra-audio/export/manifest.json');parser.add_argument('--ids');args=parser.parse_args()
    path=args.manifest.resolve();assert path.is_relative_to(ROOT)
    manifest=json.loads(path.read_text());base=path.parent
    with zipfile.ZipFile('/Users/jason/Documents/nes/roms/03/魂斗罗日版(完全版).zip') as z:rom=z.read('Contra (J).nes')
    assert hashlib.sha256(rom).hexdigest()==manifest['romSHA256']=='6ba53139fa88b8de1ae527c438bda6f1541d1ee7df26d63dec5164a32d166bfe'
    assert manifest['acceptedForRelease'] is False
    prg=rom[16:131088];ids=[];reports=[]
    for sound in manifest['sounds']:
        if args.ids and sound['id'] not in [int(n,0)for n in args.ids.split(',')]:continue
        ids.append(sound['id'])
        assert sound.get('warmupFrames',0)>=120,'Cold-start export retains unrelated APU startup output'
        nsf=(base/sound['carrier']).read_bytes();data=nsf[128:]
        assert data[:0x3f80]==prg[0x4000:0x7f80]
        assert data[0x3f9a:0x3fe0]==prg[0x7f9a:0x7fe0]
        assert data[0x3fef:0x4000]==prg[0x7fef:0x8000]
        assert data[0x4000:]==prg[0x1c000:0x20000]
        assert prg[0x7fe0:0x7fef]==bytes([255])*15
        assert prg[0x7f80:0x7f9a]==bytes([255])*26
        assert data[0x3fe0:0x3fef]==bytes([0x20,0x0b,0xf9,0x20,0x19,0x87,0xa9,0,0x8d,0xf0,5,0x8d,0xf1,5,0x60])
        assert data[0x3f80:0x3f9a]==bytes([0xad,0xf1,5,0xd0,0x12,0xee,0xf0,5,0xad,0xf0,5,0xc9,120,0xd0,8,0xee,0xf1,5,0xa9,sound['id'],0x20,0xa2,0x87,0x4c,0xd5,0x80])
        assert len(sound['runs'])==2
        for name in ['full.wav','apu.csv','driver.bin','samples.csv','calls.csv']:
            a,b=[base/r/name for r in sound['runs']]
            assert a.read_bytes()==b.read_bytes(),(sound['id'],name,'not deterministic')
        samples=pcm(base/sound['runs'][0]/'full.wav')
        origin=sound['requestSampleOrigin']
        assert origin>4410 and max(abs(v)for v in samples[origin-4410:origin])<8,'APU did not settle before request'
        assert sound['sampleRange'][0]==origin,'The original request-to-first-note delay must be preserved'
        assert samples and max(abs(v) for v in samples)>8,(sound['id'],'silent')
        assert all(-32768<v<32767 for v in samples),(sound['id'],'clipping')
        result=pcm(base/sound['file']);first,last=sound['sampleRange']
        assert result==samples[first:last] and sha(base/sound['file'])==sound['sha256']
        if sound['kind'] in ('effect','cue','percussion'):
            assert max(abs(v) for v in samples[-4410:])<8,(sound['id'],'tail not finished')
            assert last-first>0
        else:
            assert sound['loop']['pcmSeamAccepted'] is False
            assert sound['loop']['status']=='driver-state-repeat','Loop classification requires an observed active driver period'
            final_driver=(base/sound['runs'][0]/'driver.bin').read_bytes()
            assert sound['id'] in final_driver[-124:-118],'A finished sound is not a persistent loop'
            if sound['loop']['status']=='driver-state-repeat':
                start,end=sound['loop']['driverFrames']
                driver=(base/sound['runs'][0]/'driver.bin').read_bytes()
                assert driver[(start-1)*130:start*130]==driver[(end-1)*130:end*130]
                cursor=list(csv.DictReader((base/sound['runs'][0]/'samples.csv').open()))
                assert sound['loop']['sourceSampleRange']==[int(cursor[start]['samples']),int(cursor[end]['samples'])]
                assert sound['loop']['sampleRange']==[n-first for n in sound['loop']['sourceSampleRange']]
        address=length=None;regions=set()
        for row in csv.DictReader((base/sound['runs'][0]/'apu.csv').open()):
            register,value=int(row['address']),int(row['value'])
            if register==0x4012:address=0xc000+value*64
            if register==0x4013:length=value*16+1
            if register==0x4015 and value&16:
                assert address is not None and length is not None
                # Every fetched DPCM byte is from the unchanged fixed bank;
                # the init wrapper is below $C000 in verified unused padding.
                assert 0xc000<=address and address+length<=0x10000
                regions.add((address,length))
        reports.append({'id':sound['id'],'seconds':len(result)/44100,'peak':max(abs(v) for v in result),'sha256':sound['sha256'],'dpcmRegions':sorted(regions)})
    assert len(ids)==len(set(ids)) and ids
    result={'passed':True,'scope':'Byte-bound offline original APU rendering and repeated numerical checks; no browser/listening/seam or public-release acceptance.',
            'manifestSHA256':sha(path),'sounds':reports}
    (base/'verification.json').write_text(json.dumps(result,indent=2)+'\n')
    print(json.dumps({'passed':True,'sounds':len(reports)}))


if __name__=='__main__':main()
