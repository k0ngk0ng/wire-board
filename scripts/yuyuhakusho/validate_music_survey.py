"""Acceptance contract for the new bounded music survey; authored first."""
from pathlib import Path
import argparse, gzip, hashlib, json, math, wave

ROOT = Path(__file__).resolve().parents[2]
CODES = [0x83, 0x84, 0x85, 0x86, 0x88, 0x8a, 0x8d, 0x95]
ROM_SHA = 'c39d52b2e2298b3784ada3de70d64b26b67e172114c7d18494faad70fc4d7c06'

def sha(raw): return hashlib.sha256(raw).hexdigest()

def validate(path):
    report = json.loads(path.read_text())
    assert report['schema'] == 1 and report['scope'] == 'bounded-original-music-survey'
    assert report['rom_sha256'] == ROM_SHA
    assert report['codes'] == CODES and report['pass']
    assert not report['release_ready'] and not report['published']
    assert report['source_sha256']['scripts/yuyuhakusho/validate_music_survey.py'] == sha(Path(__file__).read_bytes())
    rom = (ROOT / '.local/md-yuyuhakusho/downloads/yuyuhakusho-jp.bin').read_bytes()
    assert sha(rom) == ROM_SHA
    assert [song['code'] for song in report['songs']] == CODES
    total = 0
    for song in report['songs']:
        code = song['code']; header = song['header']; sampling = song['sampling']
        entry = 0x3a492 + (code-0x81)*4
        assert header['table_entry'] == entry
        address = int.from_bytes(rom[entry:entry+4], 'big')
        assert header['address'] == address and header['fm_rows_including_dac'] == 6 and header['psg_rows'] == 3
        raw = rom[address:address+48]
        assert header['bytes'] == raw.hex() and header['sha256'] == sha(raw)
        assert len(header['voices']) == 9
        off = 6
        for index, voice in enumerate(header['voices']):
            width = 4 if index < 6 else 6
            assert voice['record'] == raw[off:off+width].hex()
            assert voice['sequence'] == address+int.from_bytes(raw[off:off+2], 'big')
            assert 0 <= voice['sequence'] < len(rom)
            off += width
        assert sampling['steps'] > 0 and sampling['steps'] <= sampling['limit'] == 4096
        assert sampling['original_ram_video_driver_pcm_equal'] and sampling['warmup_frames_checked'] == 2400
        assert sampling['world_clear_every_step'] and sampling['effect_rows_clear_every_step']
        assert sampling['sample_rate'] == 44100 and sampling['channels'] == 2
        assert song['exact_pcm_loop_verified'] is False
        assert song['classification'] in ('finite-native-music', 'bounded-loop-evidence', 'bounded-unresolved')
        for binding in song['files'].values():
            target = ROOT / binding['path']
            assert target.resolve().is_relative_to(ROOT)
            assert sha(target.read_bytes()) == binding['sha256']
        states_path = ROOT / song['files']['states']['path']
        events_path = ROOT / song['files']['events']['path']
        with gzip.open(states_path, 'rt') as stream:
            states = [json.loads(line) for line in stream]
        assert len(states) == sampling['steps']
        assert all(row['step'] == i and len(bytes.fromhex(row['driver_hex'])) == 928 for i,row in enumerate(states))
        assert all(row['stereo_frames'] > 0 and row['world_clear'] and row['effect_rows_clear'] for row in states)
        events = json.loads(gzip.decompress(events_path.read_bytes()))
        assert not events['errors'] and events['observer_equal']
        f6 = [event for event in events['events'] if event['kind'] == 'sequence' and event['command'] == 0xf6]
        groups = {(event['row'], event['source'], event['target']) for event in f6}
        assert groups == {(branch['row'],branch['source'],branch['target']) for branch in song['track_f6_observations']}
        assert len(groups) == len(song['track_f6_observations'])
        for branch in song['track_f6_observations']:
            assert rom[branch['source']] == 0xf6
            target = branch['source']+2+int.from_bytes(rom[branch['source']+1:branch['source']+3], 'big', signed=True)
            assert branch['target'] == target
            actual = [event['step'] for event in f6 if (event['row'],event['source'],event['target']) == (branch['row'],branch['source'],branch['target'])]
            assert actual == branch['steps'] and len(actual) > 0
            if 'period_frames' in branch:
                assert len(actual) >= 2 and {b-a for a,b in zip(actual,actual[1:])} == {branch['period_frames']}
        periods = song['track_periods']
        candidate = math.lcm(*periods) if periods else None
        assert song['candidate_period_lcm'] == candidate
        recurrent_rows = {branch['row'] for branch in song['track_f6_observations'] if 'period_frames' in branch}
        complete_rows = bool(sampling['active_music_rows_at_end']) and set(sampling['active_music_rows_at_end']) <= recurrent_rows
        assert song['all_active_rows_have_repeated_f6'] == complete_rows
        assert song['common_period_frames'] == (candidate if complete_rows else None)
        assert sampling['nonzero_sample_values'] > 0
        with wave.open(str(ROOT / song['files']['wav']['path']), 'rb') as wav:
            assert (wav.getnchannels(),wav.getsampwidth(),wav.getframerate()) == (2,2,44100)
            assert wav.getnframes() == sum(row['stereo_frames'] for row in states)
            for row in states:
                pcm = wav.readframes(row['stereo_frames'])
                assert sha(pcm) == row['pcm_sha256']
            assert not wav.readframes(1)
        if song['classification'] == 'finite-native-music':
            assert sampling['native_end_step'] is not None and sampling['idle_tail_steps'] >= 16 and sampling['exact_zero_tail_batches'] >= 8
        total += sampling['steps']
    return {'pass': True, 'songs': len(report['songs']), 'original_steps_compared': total, 'release_ready': False, 'exact_pcm_loop_verified': False}

if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--report', type=Path, required=True)
    args = parser.parse_args()
    print(json.dumps(validate(args.report.resolve()), indent=2))
