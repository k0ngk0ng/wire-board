"""Typed note/parameter catalog acceptance before the new extractor."""
from pathlib import Path
import argparse,gzip,hashlib,json

ROOT=Path(__file__).resolve().parents[2]
MUSIC=[0x82,0x83,0x84,0x85,0x86,0x87,0x88,0x8a,0x8d,0x95,0x8f]
CODES=MUSIC+list(range(0xa0,0xac))
KINDS={'note','transpose-add','level-add','detune','pan','pan-automation','tie','gate','tempo','enqueue-replace-first','fm-patch','volume-envelope','pitch-control','modulation','modulation-enable','psg-noise','track-end','all-key-off'}

def sha(raw):return hashlib.sha256(raw).hexdigest()
def load(path):return json.loads(gzip.decompress(path.read_bytes()))if path.suffix=='.gz'else json.loads(path.read_text())

def no_machine_data(value):
    if isinstance(value,dict):
        for key,item in value.items():
            assert not any(word in key.lower()for word in('address','opcode','bytecode','_hex','program_counter','raw_','rom_')),(key,'machine data in production candidate')
            no_machine_data(item)
    elif isinstance(value,list):
        for item in value:no_machine_data(item)

def validate(catalog,verification):
    library=load(catalog);proof=load(verification)
    assert library['schema']==1 and library['format']=='typed-audio-score-v1'
    assert not library['dsp_implemented']and not proof['dsp_implemented']and proof['pass']
    no_machine_data(library)
    assert [score['id']for score in library['scores']]==CODES
    patches={patch['id']:patch for patch in library['fm_patches']}
    assert patches and len(patches)==len(library['fm_patches'])
    for patch in patches.values():
        assert 0<=patch['algorithm']<=7 and 0<=patch['feedback']<=7 and len(patch['operators'])==4
        for operator in patch['operators']:
            assert 0<=operator['total_level']<=127 and isinstance(operator['volume_carry_bias'],bool)
    descriptors={descriptor['id']:descriptor for descriptor in library['dac_descriptors']}
    payloads={sample['id']:sample for sample in library['dac_payloads']}
    for descriptor in descriptors.values():assert descriptor['sample_id']in payloads and 0<=descriptor['native_rate']<=255 and 0<=descriptor['flags']<=255 and descriptor['routing']=='track-pan'
    assert len(library['pitch_envelopes'])==8 and len(library['psg_volume_envelopes'])==11 and len(library['pan_patterns'])==3
    voices=note_definitions=0;queue_intents=[]
    for score in library['scores']:
        assert score['kind']==('music'if score['id']in MUSIC else'effect-primary')
        assert len(score['voices'])==(9 if score['id']in MUSIC else len(score['voices']))
        assert score['start_phase']==('next-driver-tick'if score['id']in MUSIC else'current-driver-tick')
        for index,voice in enumerate(score['voices']):
            assert voice['id']==index and voice['channel']in{'dac','fm1','fm2','fm3','fm4','fm5','fm6','psg1','psg2','psg3','psg-noise'}
            assert -128<=voice['initial']['transpose']<=127 and 0<=voice['initial']['level']<=255
            assert 0<=voice['initial']['pitch_control']<=255 and 0<=voice['initial']['volume_envelope']<=11
            timeline=voice['timeline'];assert timeline['prefix_quanta']>=0 and timeline['cycle_quanta']>=0
            assert bool(timeline['cycle'])==(timeline['cycle_quanta']>0)
            for group in('prefix','cycle'):
                previous=-1
                for event in timeline[group]:
                    assert event['kind']in KINDS and event['at']>=previous;previous=event['at']
                    if group=='cycle':assert event['at']<=timeline['cycle_quanta']
                    if event['kind']=='note':
                        note_definitions+=1;assert 1<=event['duration']<=256
                        pitch=event['pitch'];assert pitch['kind']in{'chromatic','previous','rest','dac'}
                        if pitch['kind']=='chromatic':assert 0<=pitch['semitone']<=94
                        if pitch['kind']=='dac':assert pitch['descriptor_id']in descriptors
                    elif event['kind']=='fm-patch':assert event['patch_id']in patches
                    elif event['kind']=='enqueue-replace-first':queue_intents.append((score['id'],event['sound_id']))
            voices+=1
    assert voices==115 and note_definitions>115 and (0xab,0x7d)in queue_intents
    assert len(library['pitch_tables']['fm_fnums'])==12 and len(library['pitch_tables']['psg_dividers'])==70
    assert proof['catalog_sha256']==sha(catalog.read_bytes())
    assert proof['continuous_note_outcomes_compared']==127691
    assert proof['header_voices_compared']==115 and proof['definitions']==23
    assert proof['actual_read_parameters_checked']>=255109
    assert proof['machine_data_absent_from_catalog']
    for path,expected in proof['source_sha256'].items():assert sha((ROOT/path).read_bytes())==expected
    return {'pass':True,'definitions':23,'voices':115,'typed_note_definitions':note_definitions,'continuous_note_outcomes':127691,'fm_patches':len(patches),'dsp_implemented':False}

if __name__=='__main__':
    p=argparse.ArgumentParser(description=__doc__);p.add_argument('--catalog',type=Path,required=True);p.add_argument('--proof',type=Path,required=True);a=p.parse_args();print(json.dumps(validate(a.catalog.resolve(),a.proof.resolve()),indent=2))
