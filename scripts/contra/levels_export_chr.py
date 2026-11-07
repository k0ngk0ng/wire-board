"""Retain original CHR animation descriptors, calls and presentation latency."""
import argparse,gzip,hashlib,json,re,shutil
from pathlib import Path
from levels_data import original

ROOT=Path(__file__).resolve().parents[2]
BASE=ROOT/'.local/contra-levels'
OUT=ROOT/'docs/research/nes-contra/evidence'


def sha(data):return hashlib.sha256(data).hexdigest()


def main():
    parser=argparse.ArgumentParser()
    parser.add_argument('--first-label',default='v1')
    parser.add_argument('--repeat-label',default='v2')
    parser.add_argument('--case-label',default='v1')
    args=parser.parse_args()
    assert all(re.fullmatch(r'[a-z0-9-]+',v)for v in [args.first_label,args.repeat_label,args.case_label])
    rom,_=original();prg=rom[16:16+131072]
    rows=[{'index':i,'backgroundSlot':4+prg[0x1c8df+i*5],
           'pages':list(prg[0x1c8e0+i*5:0x1c8e4+i*5]),
           'source':{'cpu':0xc8df+i*5,'romOffset':16+0x1c8df+i*5,'bytes':5,
                     'sha256':sha(prg[0x1c8df+i*5:0x1c8e4+i*5])}}for i in range(9)]
    descriptor={'schema':1,'romSHA256':sha(rom),'entry':0xc83b,
                'fields':{'stage':0x30,'sceneType':0x40,'transition':0x6a,'timer':0x5c,'phase':0x71,
                          'pace':0x5d,'alternate':0x48,'random':0x34,'screen':0x64,'counter':0x1a,'desired':0x7f0},
                'rows':rows,'secondaryLair':{'slot':7,'pages':list(prg[0x1c90c:0x1c910]),
                                           'sourceCPU':0xc90c,'disabledWhenAlternateHighBit':True},
                'transition':{'whenNonzero':True,'setTransition':128,'setTimer':0,'slot':7,'page':75},
                'countdown':{'operation':'decrement byte modulo 256; return when high bit is clear',
                             'phaseOperation':'decrement byte; reset to 3 when high bit is set'},
                'stageRules':[
                    {'stage':1,'row':0,'randomMask':7,'delayBase':4},
                    {'stage':2,'row':1,'delay':4,'disabledWhenSceneTypeHighBit':True},
                    {'stage':3,'row':2,'randomMask':1,'delayBase':6,'disabledWhenAlternateHighBit':True},
                    {'stage':4,'row':3,'delay':4,'disabledWhenSceneTypeHighBit':True},
                    {'stage':5,'row':4,'randomMask':3,'delayBase':1,'addPaceByteModulo256':True},
                    {'stage':6,'disabled':True},{'stage':7,'disabled':True},
                    {'stage':8,'row':7,'nonzeroPace':'increment pace modulo 256; if it remains nonzero, reload timer from old pace',
                     'screenDistanceBase':12,'distanceOperation':'12-screen modulo 256',
                     'zeroDistance':{'counterMask':7,'waitWhenMaskedCounterNonzero':True,'row':8,'keepDecrementedTimer':True}},
                ],
                'presentation':'Apply desired pages at the next original FACE/NMI presentation boundary; do not render the just-updated desired vector immediately.',
                'limits':['Rows 5 and 6 are retained source bytes but disabled by the eight-stage dispatch.',
                          'Row 8 is the lair screen-12 branch, not a ninth gameplay stage.',
                          'Reload timer value 4 does not mean a four-frame interval: decrement then high-bit gate also counts zero.']}
    current_path=BASE/'chr-runtime'/f'chr-{args.repeat_label}-observations.json.gz'
    old=json.loads(gzip.decompress((BASE/'chr-runtime'/f'chr-{args.first_label}-observations.json.gz').read_bytes()))
    observed=json.loads(gzip.decompress(current_path.read_bytes()))
    total=0;submitted=0;aliases=0
    for a,b in zip(old['stages'],observed['stages']):
        assert a['stage']==b['stage'] and a['frames']==b['frames'] and a['writes']==b['writes']
        n=b['stage'];r1=BASE/'chr-runtime/runs'/f'chr-stage-{n}-{args.first_label}'/'ram.bin';r2=BASE/'chr-runtime/runs'/f'chr-stage-{n}-{args.repeat_label}'/'ram.bin'
        assert r1.read_bytes()==r2.read_bytes()
        assert len(b['animationCalls'])==256
        total+=len(b['animationCalls'])
        for prior,following in zip(b['frames'],b['frames'][1:]):
            assert following['applied']==prior['desired'];submitted+=1
        for sample in b['samples']:
            assert len(sample['matchingPageAliases'])==8 and all(sample['matchingPageAliases']);aliases+=8
    assert total==2048 and submitted==2040 and aliases==256
    case_path=BASE/'chr-cases'/f'chr-cases-{args.case_label}.json.gz';cases=json.loads(gzip.decompress(case_path.read_bytes()))
    first={o['name']:o['calls'][0]for o in cases['observations']};assert len(first)==14
    assert first['transition']['after']['transition']==128 and first['transition']['after']['timer']==0
    assert first['transition']['after']['desired'][7]==75
    for name in ['countdown','base-boss','waterfall-alternate','lair-screen12-wait','energy-no-animation','hangar-no-animation']:
        assert first[name]['before']['desired']==first[name]['after']['desired'],name
    assert first['jungle-delay']['after']['timer']==11 and first['jungle-delay']['after']['phase']==3
    assert first['snow-pace-carry']['after']['timer']==3
    assert first['lair-pace']['after']['pace']==2 and first['lair-pace']['after']['timer']==1
    assert first['lair-pace-wrap']['after']['pace']==0 and first['lair-pace-wrap']['after']['timer']==1
    for name in ['lair-screen12-row8','lair-screen12-alternate']:
        a=first[name]['after'];assert a['timer']==255 and a['phase']==2 and a['desired'][6]==rows[8]['pages'][2]
    assert first['lair-screen12-row8']['after']['desired'][7]==descriptor['secondaryLair']['pages'][2]
    assert first['lair-screen12-alternate']['after']['desired'][7]==first['lair-screen12-alternate']['before']['desired'][7]
    assert first['lair-screen13-wrap']['after']['timer']==255
    OUT.mkdir(parents=True,exist_ok=True)
    (OUT/'levels-chr-animation.json').write_text(json.dumps(descriptor,indent=2)+'\n')
    shutil.copyfile(current_path,OUT/'levels-chr-original.json.gz')
    shutil.copyfile(case_path,OUT/'levels-chr-cases-original.json.gz')
    raw=(BASE/'animation-rea/evidence.json').read_bytes()
    (OUT/'levels-chr-rea.json.gz').write_bytes(gzip.compress(raw,mtime=0))
    report={'passed':True,'originalCalls':total,'controlledBoundaryCases':14,'controlledBoundaryCalls':28,
            'appliedEqualsPreviousDesiredPairs':submitted,'physicalCHRSlotsByteMatched':aliases,
            'repeatFullRAMIdenticalStages':8,'sourceRanges':descriptor['rows'],
            'scope':'Original ordinary stage-frame animation plus explicitly controlled function boundaries; not a campaign clear.',
            'files':{p.name:sha(p.read_bytes())for p in sorted(OUT.glob('levels-chr-*'))if p.name!='levels-chr-verification.json'}}
    (OUT/'levels-chr-verification.json').write_text(json.dumps(report,indent=2)+'\n')
    print(json.dumps({k:report[k]for k in ['passed','originalCalls','controlledBoundaryCases','appliedEqualsPreviousDesiredPairs','physicalCHRSlotsByteMatched']}))


if __name__=='__main__':main()
