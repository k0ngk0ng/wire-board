"""Verify finite ending75 and bounded original timeline76 without a fake loop."""
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
    assert '76' not in playback
    result={'status':'passed','finite_music':[75],'timeline_preview_only':[76],'accepted_for_release':False,'listening_accepted':False}
    (OUT/'ending-music-verification.json').write_text(json.dumps(result,indent=2)+'\n');print(json.dumps(result))


if __name__=='__main__':main()
