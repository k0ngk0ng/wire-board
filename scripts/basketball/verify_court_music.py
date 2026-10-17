"""Verify each court's original capture and explicitly processed playback loop."""
from pathlib import Path
import array
import csv
import hashlib
import json
import math
import sys
import wave

ROOT=Path(__file__).resolve().parents[2]
OUT=ROOT/'.local/basketball/audio/export'


def sha(p):
    h=hashlib.sha256()
    with p.open('rb') as f:
        while chunk:=f.read(1024*1024):h.update(chunk)
    return h.hexdigest()


def pcm(p):
    with wave.open(str(p)) as f:
        assert (f.getnchannels(),f.getsampwidth(),f.getframerate())==(1,2,44100)
        a=array.array('h',f.readframes(f.getnframes()))
    if sys.byteorder!='little':a.byteswap()
    return a


def main():
    manifest=json.loads((OUT/'court-music-manifest.json').read_text())
    source=json.loads((ROOT/'games/basketball/assets/court-music-source.json').read_text())
    playback=json.loads((ROOT/'games/basketball/assets/audio-playback.json').read_text())
    assert source['court_table']['ids']==[59,66,61,64,60,62,63,65]
    assert {x['id'] for x in manifest['tracks']}==set(source['court_table']['ids'])
    assert manifest['accepted_for_release'] is False
    rom=(ROOT/'.local/nes-basket/basketball.nes').read_bytes()
    assert hashlib.sha256(rom).hexdigest()==source['rom_sha256']=='0fa28f60abb8d2ccbbeee90e79e3fa160a5cf32feb90f47ebf1a89168cff569b'
    reports=[]
    for track in manifest['tracks']:
        sound_id=track['id'];entry=playback['music'][str(sound_id)]
        assert entry['loop'] is True and entry['wav']==f'music-{sound_id:03}-loop.wav'
        if sound_id==66:
            assert entry['sha256']=='d06fe1429fdca09c0e9fd5b77687b7603bff21812e58630c58b805f12404ff87'
            assert sha(OUT/entry['wav'])==entry['sha256'];reports.append({'id':66,'reused':True});continue
        runs=track['captures'];assert len(runs)==2
        nsf=(OUT/f'sound-{sound_id:03}.nsf').read_bytes()
        assert nsf[128:128+16384]==rom[16:16+16384]
        assert nsf[128+16384:128+0x7ff0]==rom[16+7*16384:16+131072-16]
        assert nsf[128+0x7ff0:128+0x7ffb]==bytes([0xa9,0,0x20,0,0x80,0xa9,sound_id,0x20,0,0x80,0x60])
        for kind in ['wav','driver','samples','jumps','calls','apu']:
            assert sha(OUT/runs[0][kind])==sha(OUT/runs[1][kind])==track['capture_sha256'][kind]
        address=length=None;dpcm=set()
        for row in csv.DictReader((OUT/runs[0]['apu']).open()):
            register,value=int(row['address']),int(row['value'])
            if register==0x4012:address=0xc000+value*64
            if register==0x4013:length=value*16+1
            if register==0x4015 and value&16:
                assert address is not None and length is not None and address+length<=0xfff0
                dpcm.add((address,length))
        states=(OUT/runs[0]['driver']).read_bytes()
        frame_a,frame_b=track['driver_frame_range']
        assert frame_b>frame_a and states[(frame_a-1)*231:frame_a*231]==states[(frame_b-1)*231:frame_b*231]
        cursors=list(csv.DictReader((OUT/runs[0]['samples']).open()))
        start,end=track['sample_range'];assert [start,end]==[int(cursors[frame_a]['samples']),int(cursors[frame_b]['samples'])]
        original=pcm(OUT/runs[0]['wav']);output=pcm(OUT/entry['wav']);blend=track['blend_samples']
        assert len(output)==end-start and blend==1411
        for offset in range(0,len(output)-blend,65536):
            stop=min(offset+65536,len(output)-blend)
            assert output[offset:stop]==original[start+offset:start+stop]
        assert abs((output[0]-output[-1])-(original[start]-original[start-1]))<=1
        assert max(abs(a-b) for a,b in zip(output[-16:],original[start-16:start]))<=2
        assert max(abs(x) for x in output)<32767
        levels=[]
        for k in range(0,blend-353+1,88):
            a=original[end-blend+k:end-blend+k+353];b=original[start-blend+k:start-blend+k+353];v=output[-blend+k:-blend+k+353]
            pos=(k+176)/(blend-1);w=(1-math.cos(math.pi*pos))/2
            expected=((1-w)*sum(x*x for x in a)+w*sum(x*x for x in b))/353
            actual=sum(x*x for x in v)/353
            db=10*math.log10(actual/expected) if expected and actual else 0
            assert abs(db)<=2,(sound_id,k,db);levels.append(db)
        assert sha(OUT/entry['wav'])==entry['sha256']==track['sha256']
        assert track['listening_accepted'] is False and track['accepted_for_release'] is False
        reports.append({'id':sound_id,'seconds':len(output)/44100,'rms_max_abs_db':max(abs(x) for x in levels),'sha256':track['sha256'],'dpcm_source_ranges':sorted(dpcm)})
    result={'status':'passed','scope':'source-bound independent court loops; listening and publication unaccepted','tracks':reports,'accepted_for_release':False}
    (OUT/'court-music-verification.json').write_text(json.dumps(result,indent=2)+'\n');print(json.dumps(result))


if __name__=='__main__':main()
