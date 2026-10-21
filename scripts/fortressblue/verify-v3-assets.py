#!/usr/bin/env python3
"""CI-safe v3 verifier. Uses only committed evidence/media; never imports local codecs."""
import argparse
import hashlib
import json
import struct
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
ASSETS = ROOT / 'games/fortressblue/assets'
DATA = ROOT / 'games/fortressblue/data'
ACTION_STATES = {'idle':0,'idle1':1,'idle2':2,'fire':3,'special':3,'afterFire':4,'damage':5,'item':6,'poison':7,'move':8,'moveReverse':9,'fall':10,'death':11,'dead':12}

def sha(data):
    return hashlib.sha256(data).hexdigest()

def webp_size(data):
    assert data[:4] == b'RIFF' and data[8:12] == b'WEBP'
    offset = 12
    while offset+8 <= len(data):
        kind, length = data[offset:offset+4], struct.unpack_from('<I',data,offset+4)[0]
        if kind == b'VP8L':
            assert data[offset+8] == 0x2f
            dims = struct.unpack_from('<I',data,offset+9)[0]
            return (dims & 0x3fff)+1, ((dims>>14)&0x3fff)+1
        if kind == b'VP8X':
            return int.from_bytes(data[offset+12:offset+15],'little')+1, int.from_bytes(data[offset+15:offset+18],'little')+1
        offset += 8+length+(length&1)
    raise AssertionError('lossless WebP dimensions absent')

def verify(dist=False):
    catalog = json.loads((DATA/'animations.json').read_text())
    manifest = json.loads((ASSETS/'manifest-v3.json').read_text())
    evidence = json.loads((ASSETS/'evidence/animation-native-evidence.json').read_text())
    assert catalog['version'] == 3 and catalog['pixelScale'] == 1
    assert catalog['actionMappingEvidence'] == 'native-wasm-static'
    assert catalog['nativeAnimationMappingConfirmed'] is True
    assert catalog['timingEvidence'] == 'reconstructed-30hz'
    assert evidence['verifiedCharacterIds'] == list(range(1,15))
    groups = {g['state']:g for g in evidence['actions']}
    assert len(groups) == 13
    dimensions = {}
    for entry in manifest['entries']:
        for base in ([ASSETS/'public',ROOT/'web/dist'] if dist else [ASSETS/'public']):
            path = base / entry['urlPath'].lstrip('/')
            data = path.read_bytes()
            assert len(data) == entry['bytes'] and sha(data) == entry['sha256'], path
            dimensions[entry['urlPath']] = webp_size(data)
            assert all(0 < value <= 2048 for value in dimensions[entry['urlPath']])
    assert len(manifest['entries']) == 14
    assert set(catalog['tanks']) == {str(i) for i in range(1,15)}
    for tid,tank in catalog['tanks'].items():
        assert len(tank['frames']) == 592 and set(tank['actions']) == set(ACTION_STATES)
        for i,frame in enumerate(tank['frames']):
            assert frame['sourceIndex'] == i//2 and frame['facing'] == ('left' if i%2 == 0 else 'right')
            width,height = dimensions[tank['atlases'][frame['atlas']]]
            assert frame['w'] > 0 and frame['h'] > 0 and frame['x'] >= 0 and frame['y'] >= 0
            assert frame['x']+frame['w'] <= width and frame['y']+frame['h'] <= height
        for name,state in ACTION_STATES.items():
            group,action = groups[state],tank['actions'][name]
            expected = [group['first']+n*group['slopes']+(2 if group['slopes']==5 else 0) for n in range(group['frames'])]
            assert action['nativeState'] == state and action['durationMs'] == round(group['frames']*1000/15)
            assert action['slopeVariants'] == group['slopes']
            for facing in ('left','right'):
                actual = [tank['frames'][i]['sourceIndex'] for i in action[facing]]
                assert actual == expected, (tid,name,actual,expected)
                assert all(tank['frames'][i]['facing']==facing for i in action[facing])
                if group['slopes'] == 5:
                    for index in action[facing]:
                        for slope in range(-2,3):
                            variant=tank['frames'][index+2*slope]
                            assert variant['facing']==facing and variant['sourceIndex']==tank['frames'][index]['sourceIndex']+slope
                            assert group['first'] <= variant['sourceIndex'] <= group['last']
        assert tank['actions']['fire']['projectileFrame'] == 5
        assert tank['actions']['fire']['projectileDelayMs'] == 334
        assert tank['actions']['fire']['nextAction'] == 'afterFire'
        assert tank['actions']['death']['nextAction'] == 'dead'
    source = (ASSETS/'evidence/character.txt').read_bytes()
    assert sha(source) == catalog['characterSha256']
    rows = source.decode().splitlines(); columns=rows[0].split('\t')
    tanks = json.loads((DATA/'tanks.json').read_text())['tanks']
    for tid in range(1,15):
        row=list(map(int,rows[tid].split('\t')));tank=tanks[str(tid)]
        assert tank['original']['values']==row and tank['original']['columns']==columns
        assert tank['original']['sourceSha256']==sha(source)
        for field,col in [('hp',3),('speed',4),('defense',6),('range',7),('primary',8),('special',10),('angle',1),('minAngle',2)]:
            assert tank[field]==row[col],(tid,field)
        assert tank['original']['roundMeaning']=='unverified'
    return {'passed':True,'version':3,'entries':len(manifest['entries']),'frames':8288,'nativeStates':13,'dist':dist}

if __name__ == '__main__':
    parser=argparse.ArgumentParser();parser.add_argument('--dist',action='store_true');args=parser.parse_args()
    result=verify(args.dist)
    output=ROOT/'.local/fortressblue/assets-v3-evidence';output.mkdir(parents=True,exist_ok=True)
    (output/'ci-verification.json').write_text(json.dumps(result,indent=2)+'\n')
    print(json.dumps(result))
