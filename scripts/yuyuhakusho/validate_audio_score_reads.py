"""First acceptance contract for actual score parameter/header/read evidence."""
from pathlib import Path
import argparse,gzip,hashlib,json

ROOT=Path(__file__).resolve().parents[2]
MUSIC=[0x82,0x83,0x84,0x85,0x86,0x87,0x88,0x8a,0x8d,0x95,0x8f]
EFFECTS=list(range(0xa0,0xac))
ROM_SHA='c39d52b2e2298b3784ada3de70d64b26b67e172114c7d18494faad70fc4d7c06'

def sha(raw):return hashlib.sha256(raw).hexdigest()

def validate(index):
    report=json.loads(index.read_text());assert report['schema']==1 and report['pass']
    assert report['scope']=='native-score-read-oracle'and report['codes']==MUSIC+EFFECTS
    assert not report['production_ready']and not report['dsp_implemented']
    rom=(ROOT/'.local/md-yuyuhakusho/downloads/yuyuhakusho-jp.bin').read_bytes();assert sha(rom)==ROM_SHA==report['rom_sha256']
    assert report['source_sha256']['scripts/yuyuhakusho/validate_audio_score_reads.py']==sha(Path(__file__).read_bytes())
    voices=notes=patches=parameters=0;seen=set()
    for case in report['cases']:
        code=case['code'];assert code not in seen;seen.add(code)
        assert case['native_reference_equal']and case['pcm_samples_equal']and case['driver_steps']>0
        for binding in case['files'].values():
            path=(ROOT/binding['path']).resolve();assert path.is_relative_to(ROOT)and sha(path.read_bytes())==binding['sha256']
        trace=json.loads(gzip.decompress((ROOT/case['files']['events']['path']).read_bytes()))
        events=trace['events'];assert not trace['errors']and len([event for event in events if event['kind']=='tick-before'])==case['driver_steps']
        actual=[event for event in events if event['kind']=='voice-init']
        assert len(actual)==case['declared_voices']==(9 if code in MUSIC else case['declared_voices'])
        assert all(event['code']==code for event in actual)
        voices+=len(actual)
        assert any(event['kind']=='header'for event in events)
        for event in events:
            if 'read_address'in event:
                address=event['read_address'];raw=bytes.fromhex(event['read_hex'])
                assert 0<=address<=len(rom)-len(raw)and raw==rom[address:address+len(raw)]
            if event['kind']=='note-outcome':
                assert len(bytes.fromhex(event['row_hex']))==48
                assert 1<=event['effective_duration']<=256
                notes+=1
            elif event['kind']=='fm-patch-read':
                assert len(bytes.fromhex(event['read_hex']))==25
                patches+=1
            elif event['kind']in('pitch-envelope-read','volume-envelope-read','pan-pattern-read','e9-bank-read','fm3-offset-read'):parameters+=1
        frames=[]
        with gzip.open(ROOT/case['files']['frames']['path'],'rt')as stream:
            for line in stream:frames.append(json.loads(line))
        assert len(frames)==case['steps']and all(row['step']==i and len(bytes.fromhex(row['driver_hex']))==928 and row['stereo_frames']>0 for i,row in enumerate(frames))
        assert all(len(bytes.fromhex(event['driver_hex']))==928 for event in events if event['kind']=='tick-before')
    assert seen==set(MUSIC+EFFECTS)and voices==115 and notes>115 and patches>0 and parameters>0
    return {'pass':True,'definitions':23,'header_voices':voices,'actual_note_outcomes':notes,'actual_patch_reads':patches,'actual_parameter_reads':parameters,'dsp_implemented':False}

if __name__=='__main__':
    p=argparse.ArgumentParser(description=__doc__);p.add_argument('--capture',type=Path,required=True);a=p.parse_args();print(json.dumps(validate(a.capture.resolve()),indent=2))
