"""Verify finite75 and preserved preview76, separately from compound76 delivery."""
from pathlib import Path
import array
import hashlib
import json
import wave

ROOT=Path(__file__).resolve().parents[2]
OUT=ROOT/'.local/basketball/audio/export'


def data(path):
    with wave.open(str(path)) as f:
        assert (f.getnchannels(),f.getsampwidth(),f.getframerate())==(1,2,44100)
        return f.readframes(f.getnframes())


def main():
    manifest=json.loads((OUT/'ending-music-manifest.json').read_text());assert {e['id'] for e in manifest['tracks']}=={75,76}
    for entry in manifest['tracks']:
        assert entry['accepted_for_release'] is False and entry['listening_accepted'] is False and entry['loop'] is False
        original=data(OUT/entry['captures'][0]['wav']);again=data(OUT/entry['captures'][1]['wav']);assert original==again
        driver=(OUT/entry['captures'][0]['driver']).read_bytes();assert driver==(OUT/entry['captures'][1]['driver']).read_bytes()
        a,b=entry['sample_range'];output=data(OUT/entry['wav']);assert output==original[a*2:b*2]
        assert hashlib.sha256((OUT/entry['wav']).read_bytes()).hexdigest()==entry['sha256']
        if entry['id']==75:
            assert entry['driver_end_step']==8470 and not entry['preview_only']
            assert not any(driver[(8470-1)*231+0x15:(8470-1)*231+0x19])
            assert 140<len(output)/2/44100<143
            assert max(abs(v) for v in array.array('h',output[-8820:]))<8
        else:
            assert entry['preview_only'] is True and entry['driver_end_step'] is None
            assert a==0 and b*2==len(original) and 332<len(output)/2/44100<334
    playback=json.loads((ROOT/'games/basketball/assets/audio-playback.json').read_text())['music']
    assert playback['75']['wav']=='music-075.wav' and playback['75']['loop'] is False
    compound=playback['76'];preview=next(e for e in manifest['tracks'] if e['id']==76)
    delivery=json.loads((OUT/'music76-delivery-manifest.json').read_text())
    assert compound['file']==delivery['file']=='music-076-intro-loop.mp3' and compound['loop'] is True
    assert compound['wav']==delivery['master']=='music-076-intro-loop.wav' and compound['wav']!=preview['wav']
    assert compound['distribution_sha256']==delivery['sha256']!=preview['sha256']
    assert compound['sha256']==delivery['master_sha256'] and compound['source_manifest']=='music76-delivery-manifest.json'
    assert hashlib.sha256((OUT/compound['file']).read_bytes()).hexdigest()==compound['distribution_sha256']
    for key in ['sample_rate','loop_start_sample','loop_end_sample','loop_samples','decoded_samples']:assert compound[key]==delivery[key]
    assert compound['loop_start_sample']>0 and compound['loop_end_sample']==compound['decoded_samples']
    assert compound['loop_samples']==compound['loop_end_sample']-compound['loop_start_sample']
    result={'status':'passed','finite_music':[75],'preserved_timeline_preview':[76],
            'preview76_runtime_registered':False,'separate_compound76_runtime_registered':True,
            'scope':'finite75 and unchanged333-second preview76; distinct verified compound76 binding, not browser acceptance',
            'accepted_for_release':False,'listening_accepted':False}
    (OUT/'ending-music-verification.json').write_text(json.dumps(result,indent=2)+'\n');print(json.dumps(result))


if __name__=='__main__':main()
