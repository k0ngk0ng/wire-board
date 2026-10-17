"""Bind the existing Go oracle to FCEUX recordings and retained alias traces.

This verifies source artifacts. Run the package tests separately; a retained
test log is not proof that a later source change was tested.
"""
import base64
import gzip
import hashlib
import json
from pathlib import Path
ROOT=Path(__file__).resolve().parents[2]


def digest(path):return hashlib.sha256(path.read_bytes()).hexdigest()


def main():
    package=ROOT/'games/basketball/audiostate'
    fixture=package/'testdata/original-driver.json.gz'
    metadata=json.loads((package/'testdata/original-driver-source.json').read_text())
    assert digest(fixture)==metadata['oracle_sha256']
    data=json.loads(gzip.decompress(fixture.read_bytes()))
    source_metadata=json.loads((package/'source-data-source.json').read_text())
    assert digest(package/'source-data.json')==source_metadata['sha256']
    rows=[]
    for ident,name in [(30,'additional-sound-030-run-1'),(61,'court-music-061-20000-run-1')]:
        case=next(x for x in data['cases'] if x['name']==f'sound{ident}-alone')
        states=[base64.b64decode(x['driver']) for x in case['steps'] if x['kind']=='tick']
        path=ROOT/'.local/basketball/audio/export'/name/'driver-ram.bin'
        raw=path.read_bytes();original=[raw[i:i+231] for i in range(0,len(raw),231)]
        assert states[:597]==original[3:600]
        rows.append({'sound':ident,'fixture_case':case['name'],'matched_frames':597,'source':str(path.relative_to(ROOT)),'sha256':digest(path)})
    reference=ROOT/'.local/basketball/rea/special-turn-alias/music-shot-work.json'
    expected=json.loads(reference.read_text());actual=[]
    for music in range(59,67):
        case=next(x for x in data['cases'] if x['name']==f'music{music}-shot30')
        shot=next(i for i,x in enumerate(case['steps']) if x['kind']=='request' and x['id']==30)
        actual += [{'music':music,'frameAfterShot':i+1,'soundWork':x['work']} for i,x in enumerate(case['steps'][shot+1:])]
    assert actual==expected
    report={'status':'passed','scope':'Go oracle bound to retained FCEUX driver states and alias work-byte traces; not world/IRQ integration',
        'oracle_sha256':digest(fixture),'cases':len(data['cases']),'steps':sum(len(case['steps']) for case in data['cases']),
        'fceux':rows,'alias_steps':len(actual),'alias_sha256':digest(reference),
        'parsed_data_sha256':digest(package/'source-data.json'),
        'current_sources':{str(path.relative_to(ROOT)):digest(path) for path in sorted(package.glob('*.go'))}}
    (ROOT/'.local/basketball/audio/state/source-binding.json').write_text(json.dumps(report,indent=2)+'\n')
    print(json.dumps({'status':'passed','cases':report['cases'],'steps':report['steps'],'alias_steps':len(actual),'fceux_states':1194}))


if __name__=='__main__':main()
