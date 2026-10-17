"""Verify offline original-ROM sound exports (run after extract_audio.py)."""
from pathlib import Path
import array
import csv
import hashlib
import json
import sys
import wave

ROOT = Path(__file__).resolve().parents[2]
SOURCE_SHA = '0fa28f60abb8d2ccbbeee90e79e3fa160a5cf32feb90f47ebf1a89168cff569b'


def sha(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def verify(directory):
    manifest = json.loads((directory / 'manifest.json').read_text())
    rom = (ROOT / '.local/nes-basket/basketball.nes').read_bytes()
    assert hashlib.sha256(rom).hexdigest() == manifest['rom_sha256'] == SOURCE_SHA
    assert manifest['accepted_for_release'] is False
    catalog = json.loads((ROOT/'games/basketball/assets/audio-catalog.json').read_text())
    effect_ids = {3,4,5,6,8,9,11,15,17,19,21,23,27,31,32,33,35,36,37,38,40,41,43,45,50,51,55}
    assert {e['id'] for e in catalog['effects']} == effect_ids
    assert {e['id'] for e in catalog['controls']} == {0,253,255}
    assert all('wav' not in e for e in catalog['controls'])
    assert {e['id'] for e in manifest['sounds'] if e['kind']=='effect'} == effect_ids
    assert {e['id'] for e in manifest['sounds'] if e['kind']=='music'} == {66}
    assert manifest['catalog_sha256'] == sha(ROOT/'games/basketball/assets/audio-catalog.json')
    reports = []
    for entry in manifest['sounds']:
        sound_id = entry['id']
        definition=next(e for e in catalog['effects']+catalog['music'] if e['id']==sound_id)
        assert entry['render_volume']==definition.get('render_volume',255)
        nsf = (directory / entry['nsf']).read_bytes()
        # The only new instructions are a documented init wrapper in the final
        # vector area. The original sound bank and fixed bank remain identical.
        assert nsf[:8] == b'NESM\x1a\x01\x01\x01'
        assert len(nsf) == 128 + 32768
        assert nsf[128:128+16384] == rom[16:16+16384]
        assert nsf[128+16384:128+0x7ff0] == rom[16+7*16384:16+131072-16]
        assert nsf[128+0x7ff0:128+0x7ffb] == bytes([0xa9,0,0x20,0,0x80,0xa9,sound_id,0x20,0,0x80,0x60])
        assert nsf[128+0x7ffb:] == rom[16+131072-5:16+131072]
        for name, digest in entry['sha256'].items():
            assert sha(directory / name) == digest, name
        repeated = []
        for run in entry['runs']:
            calls = list(csv.DictReader((directory / run['calls']).open()))
            assert [int(c['value']) for c in calls if c['kind'] == 'init'] == [0, sound_id]
            ticks = [int(c['frame']) for c in calls if c['kind'] == 'play']
            assert len(ticks) >= entry['capture_frames']-10 and all(b == a + 1 for a, b in zip(ticks, ticks[1:]))
            writes = list(csv.DictReader((directory / run['apu']).open()))
            assert writes and all(0x4000 <= int(c['address']) <= 0x4017 for c in writes)
            # Every DPCM fetch must come from unchanged source bytes, outside
            # the init wrapper. The observed music uses an original drum sample.
            sample_address=sample_length=None
            regions=set()
            for write in writes:
                address,value=int(write['address']),int(write['value'])
                if address==0x4012:sample_address=0xc000+value*64
                if address==0x4013:sample_length=value*16+1
                if address==0x4015 and value&16:
                    assert sample_address is not None and sample_length is not None
                    end=sample_address+sample_length
                    assert 0xc000<=sample_address<end<=0xfff0, 'DPCM overlaps init wrapper'
                    regions.add((sample_address,sample_length))
            assert sorted(regions)==[tuple(r) for r in entry['dpcm_regions']]
            with wave.open(str(directory / run['wav'])) as recording:
                assert (recording.getnchannels(), recording.getsampwidth(), recording.getframerate()) == (1, 2, 44100)
                samples = array.array('h', recording.readframes(recording.getnframes()))
            if sys.byteorder != 'little':
                samples.byteswap()
            assert abs(len(samples)/44100 - entry['capture_frames']/60.0988) < .1
            assert 500 < max(abs(s) for s in samples) < 32767
            if entry['kind']=='effect':
                assert max(abs(s) for s in samples[-4410:]) < 8, f'unfinished effect {sound_id}'
            cursors=list(csv.DictReader((directory/run['samples']).open()))
            assert len(cursors)==entry['capture_frames']+1
            assert int(cursors[-1]['samples'])==len(samples)
            assert all(int(b['samples'])>=int(a['samples']) for a,b in zip(cursors,cursors[1:]))
            driver=(directory/run['driver']).read_bytes()
            assert len(driver)==entry['capture_frames']*231
            repeated.append(tuple(sha(directory/run[key]) for key in ['wav','apu','calls','samples','driver','jumps']))
        assert len(repeated) == 2 and repeated[0] == repeated[1], 'repeat export differs'
        with wave.open(str(directory / entry['wav'])) as output:
            clip = output.readframes(output.getnframes())
            assert 0.05 < output.getnframes() / 44100 <= entry['capture_frames']/60
        with wave.open(str(directory / entry['runs'][0]['wav'])) as full:
            raw = full.readframes(full.getnframes())
        assert clip == raw[entry['sample_range'][0]*2:entry['sample_range'][1]*2]
        if entry['kind']=='music':
            assert entry['loop']['status'] in ['driver-state-repeat','sequence-loop-candidate','unresolved']
            assert entry['loop']['pcm_seam_accepted'] is False
            if entry['loop']['status']=='driver-state-repeat':
                start,end=entry['loop']['driver_frames']
                driver=(directory/entry['runs'][0]['driver']).read_bytes()
                assert end>start and driver[(start-1)*231:start*231]==driver[(end-1)*231:end*231]
                cursors=list(csv.DictReader((directory/entry['runs'][0]['samples']).open()))
                assert entry['loop']['sample_range']==[int(cursors[start]['samples']),int(cursors[end]['samples'])]
            if entry['loop']['status']=='sequence-loop-candidate':
                markers=entry['loop']['markers']
                jumps=list(csv.DictReader((directory/entry['runs'][0]['jumps']).open()))
                assert all({k:int(v) for k,v in m.items()} in markers for m in jumps if int(m['channel'])==0)
                assert len(markers)>=2 and markers[0]['target']==markers[1]['target']
                start,end=markers[0]['step'],markers[1]['step']
                cursors=list(csv.DictReader((directory/entry['runs'][0]['samples']).open()))
                assert entry['loop']['sample_range']==[int(cursors[start]['samples']),int(cursors[end]['samples'])]
        reports.append({'id': sound_id, 'repeat_pcm_and_apu_identical': True, 'sha256': sha(directory / entry['wav'])})
    assert len(reports) == 28
    result = {'status': 'passed', 'scope': 'source identity, original driver execution, PCM/APU repeatability; not full-game audio fidelity', 'sounds': reports}
    (directory / 'verification.json').write_text(json.dumps(result, indent=2) + '\n')
    print(json.dumps(result))


if __name__ == '__main__':
    verify(Path(sys.argv[1]) if len(sys.argv) > 1 else ROOT / '.local/basketball/audio/export')
