"""Original-event E2E for the research-only rhythmic score parser, written first."""
from pathlib import Path
import gzip, hashlib, json
from music_score_structure import derive_song, project_commands, walk_track

ROOT = Path(__file__).resolve().parents[2]
ROM = ROOT / '.local/md-yuyuhakusho/downloads/yuyuhakusho-jp.bin'
PROBE = ROOT / '.local/md-yuyuhakusho/audio-music-survey/probe'
CODES = [0x83,0x84,0x85,0x86,0x88,0x8a,0x8d,0x95]

def main():
    rom=ROM.read_bytes();reports=[];total=0
    for code in CODES:
        observed=json.loads(gzip.decompress((PROBE/f'{code:02x}-events.json.gz').read_bytes()))
        assert observed['observer_equal']and not observed['errors']
        score=derive_song(rom,code)
        for track in score['tracks']:
            expected=[{key:event[key]for key in ('step','row','source','command')}
                      for event in observed['events']if event['kind']=='sequence'and event['row']==track['row']]
            actual=project_commands(track,score['tempo_byte'],4096)
            assert actual==expected,(hex(code),hex(track['row']),next(((a,b)for a,b in zip(actual,expected)if a!=b),None),len(actual),len(expected))
            total+=len(expected)
        assert len(score['tracks'])==9 and all(track['period_quanta']>0 or track['stopped']for track in score['tracks'])
        reports.append(score)
    # Isolated malformed inputs are explicitly rejected before use.
    for raw, record in [
        (bytes([0xf6,0xff,0xfe]),{'sequence':0,'row':0xf840,'duration_multiplier':1}),
        (bytes([0xf9]),{'sequence':0,'row':0xf840,'duration_multiplier':1}),
        (bytes([0xf7,8,2,0xff,0xfc]),{'sequence':0,'row':0xf840,'duration_multiplier':1}),
        (bytes([0xff,0]),{'sequence':0,'row':0xf840,'duration_multiplier':1}),
        (bytes([0x80]),{'sequence':3,'row':0xf840,'duration_multiplier':1}),
    ]:
        try:walk_track(raw,record)
        except ValueError:pass
        else:raise AssertionError(('accepted malformed/unowned control flow',raw.hex(),record))
    result={'pass':True,'scope':'All captured sequence commands for eight music codes plus malformed-flow rejection',
            'codes':CODES,'tracks':72,'original_sequence_events_compared':total,
            'source_sha256':{str(path.relative_to(ROOT)):hashlib.sha256(path.read_bytes()).hexdigest()for path in [Path(__file__).resolve(),ROOT/'scripts/yuyuhakusho/music_score_structure.py']},
            'songs':reports,'limits':['Rhythmic control-flow periods only; raw driver, synthesis and PCM recurrence require separate native proof']}
    out=ROOT/'.local/md-yuyuhakusho/audio-music-survey/structure-validation.json'
    out.write_text(json.dumps(result,indent=2)+'\n');print(json.dumps({key:value for key,value in result.items()if key!='songs'},indent=2))

if __name__=='__main__':main()
