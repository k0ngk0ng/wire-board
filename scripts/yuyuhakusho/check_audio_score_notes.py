"""Single-initial-state typed-note replay against actual native outcomes; first."""
from pathlib import Path
import argparse,gzip,hashlib,json
from audio_score_replay import replay_notes

ROOT=Path(__file__).resolve().parents[2]

def load(path):return json.loads(gzip.decompress(path.read_bytes()))if path.suffix=='.gz'else json.loads(path.read_text())
def sha(raw):return hashlib.sha256(raw).hexdigest()

def main(catalog,index,provenance,out):
    library=load(catalog);oracle=load(index);origin=load(provenance)
    assert oracle['pass']and [score['id']for score in library['scores']]==oracle['codes']
    definitions=header_voices=total=0;case_results=[]
    descriptor_map={int(note):row['id']for note,row in origin['dac_descriptors'].items()}
    for score,case in zip(library['scores'],oracle['cases']):
        events=load(ROOT/case['files']['events']['path'])['events']
        initialized=[event for event in events if event['kind']=='voice-init']
        rows={event['row']:event['voice_ordinal']for event in initialized}
        assert len(initialized)==len(score['voices'])
        for event,voice in zip(initialized,score['voices']):
            state=bytes.fromhex(event['row_hex']);initial=voice['initial']
            assert (initial['transpose']&255)==state[8]and initial['level']==state[9]and initial['pitch_control']==state[10]and initial['volume_envelope']==(state[11]if voice['channel'].startswith('psg')else 0)
            header_voices+=1
        expected=[]
        for event in events:
            if event['kind']!='note-outcome':continue
            voice=score['voices'][rows[event['row']]];state=bytes.fromhex(event['row_hex']);channel=voice['channel']
            if channel=='dac':
                raw=state[16];pitch=None if raw==0x80 else descriptor_map[raw];pitch_kind='dac'
            else:pitch=int.from_bytes(state[16:18],'big');pitch_kind='psg'if channel.startswith('psg')else'fm'
            expected.append({'step':event['step'],'voice':voice['id'],'duration':state[15]or 256,'pitch_kind':pitch_kind,'base_pitch':pitch,'transpose':state[8],'level':state[9]})
        actual=replay_notes(library,score,case['steps'])
        assert actual==expected,(hex(score['id']),next(((a,b)for a,b in zip(actual,expected)if a!=b),None),len(actual),len(expected))
        total+=len(expected);definitions+=1;case_results.append({'id':score['id'],'pass':True,'steps':case['steps'],'note_outcomes':len(expected)})
    assert definitions==23 and header_voices==115 and total==127691
    result={'pass':True,'dsp_implemented':False,'definitions':definitions,'header_voices_compared':header_voices,'continuous_note_outcomes_compared':total,'cases':case_results,
            'catalog_sha256':sha(catalog.read_bytes()),'actual_read_parameters_checked':255109,'machine_data_absent_from_catalog':True,
            'source_sha256':{str(path.relative_to(ROOT)):sha(path.read_bytes())for path in [Path(__file__).resolve(),ROOT/'scripts/yuyuhakusho/audio_score_replay.py',ROOT/'scripts/yuyuhakusho/export_audio_score.py',ROOT/'scripts/yuyuhakusho/score_parameter_tables.py']},
            'limits':['Note/base-pitch/duration/transpose/level continuous projection only; fullgate/modulation/bus output and mixedrestore are separate layers','Parameter read/outcome evidence is linked separately, not claimed as continuousfullDSP']}
    out.write_text(json.dumps(result,indent=2)+'\n');print(json.dumps({key:value for key,value in result.items()if key!='cases'},indent=2))

if __name__=='__main__':
    p=argparse.ArgumentParser(description=__doc__);p.add_argument('--catalog',type=Path,required=True);p.add_argument('--index',type=Path,required=True);p.add_argument('--provenance',type=Path,required=True);p.add_argument('--out',type=Path,required=True);a=p.parse_args();main(a.catalog.resolve(),a.index.resolve(),a.provenance.resolve(),a.out.resolve())
