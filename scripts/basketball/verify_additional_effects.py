from pathlib import Path
import array,csv,hashlib,json,wave
from extract_audio import BASE,SOURCE_SHA,carrier

ROOT=Path(__file__).resolve().parents[2];OUT=ROOT/'.local/basketball/audio/export'


def main():
    manifest=json.loads((OUT/'additional-effects-manifest.json').read_text());assert {x['id'] for x in manifest['effects']}=={1,10,13,14,30,46,48}
    rom=(BASE/'basketball.nes').read_bytes();assert hashlib.sha256(rom).hexdigest()==manifest['rom_sha256']==SOURCE_SHA
    assert manifest['accepted_for_release'] is False
    playback=json.loads((ROOT/'games/basketball/assets/audio-playback.json').read_text())['effects']
    for entry in manifest['effects']:
        assert (OUT/f"sound-{entry['id']:03}.nsf").read_bytes()==carrier(rom,entry['id'])
        assert len(entry['captures'])==2
        for run in entry['captures']:
            info=json.loads((OUT/run['wav']).with_name('capture-info.json').read_text())
            assert info['id']==entry['id'] and info['frames']==600 and info['volume']==255
            assert info['nsf_sha256']==hashlib.sha256(carrier(rom,entry['id'])).hexdigest()
            assert info['files']==run
            assert all(hashlib.sha256((OUT/path).read_bytes()).hexdigest()==info['sha256'][key] for key,path in run.items())
            cursors=list(csv.DictReader((OUT/run['samples']).open()))
            assert len(cursors)==601
            assert len((OUT/run['driver']).read_bytes())==600*231
        assert not entry['accepted_for_release'] and not entry['listening_accepted']
        for key in entry['captures'][0]:
            a=(OUT/entry['captures'][0][key]).read_bytes();b=(OUT/entry['captures'][1][key]).read_bytes();assert a==b
            assert hashlib.sha256(a).hexdigest()==entry['capture_sha256'][key]
        calls=list(csv.DictReader((OUT/entry['captures'][0]['calls']).open()))
        assert [int(x['value']) for x in calls if x['kind']=='init']==[0,entry['id']]
        apu=csv.DictReader((OUT/entry['captures'][0]['apu']).open())
        assert not any(int(r['address'])==0x4015 and int(r['value'])&16 for r in apu)
        with wave.open(str(OUT/entry['captures'][0]['wav'])) as f:
            original=f.readframes(f.getnframes());assert f.getnchannels()==1 and f.getframerate()==44100 and f.getsampwidth()==2
        values=array.array('h',original);assert 500<max(abs(v) for v in values)<32767 and max(abs(v) for v in values[-4410:])<8
        with wave.open(str(OUT/entry['wav'])) as f:output=f.readframes(f.getnframes())
        start,end=entry['sample_range'];assert output==original[start*2:end*2]
        assert hashlib.sha256((OUT/entry['wav']).read_bytes()).hexdigest()==entry['sha256']==playback[str(entry['id'])]['sha256']
    result={'status':'passed','effects':[1,10,13,14,30,46,48],'accepted_for_release':False,'listening_accepted':False}
    (OUT/'additional-effects-verification.json').write_text(json.dumps(result,indent=2)+'\n');print(json.dumps(result))


if __name__=='__main__':main()
