"""Verify all recorded original loads/transitions and export normalized levels."""
from pathlib import Path
import argparse
import gzip
import hashlib
import json
import shutil

from levels_data import build,original

ROOT=Path(__file__).resolve().parents[2]
BASE=ROOT/'.local/contra-levels'
OUT=ROOT/'docs/research/nes-contra/evidence'


def sha(data):return hashlib.sha256(data).hexdigest()


def main():
    parser=argparse.ArgumentParser()
    parser.add_argument('--loader',type=Path,default=BASE/'runtime/loader-v1-observations.json.gz')
    parser.add_argument('--transitions',type=Path,default=BASE/'runtime/transitions-v1-observations.json.gz')
    args=parser.parse_args()
    data=build();stages={s['stage']:s for s in data['stages']}
    loader_path=args.loader
    transition_path=args.transitions
    loads=json.loads(gzip.decompress(loader_path.read_bytes()))['observations']
    transitions=json.loads(gzip.decompress(transition_path.read_bytes()))['observations']
    assert len(loads)==146 and len(transitions)==8
    seen=set();loaded_bytes=0
    for record in loads:
        stage=stages[record['stage']];index=record['requestedIndex'];loader=record['loader']
        key=(record['stage'],record['mode'],index);assert key not in seen;seen.add(key)
        maps=stage['maps']if record['mode']=='loader'else data['extraMapSets']['base-boss']
        expected=maps['entries'][index];length=maps['mapRows']*8;offset=loader['offset']
        before=bytes.fromhex(loader['before']);after=bytes.fromhex(loader['after']);header=bytes.fromhex(loader['header'])
        assert loader['input']==index and loader['pointer']==expected['pointer'] and loader['mapBank']==0x14
        assert int.from_bytes(header[2:4],'little')==maps['pointerTable']['cpu']
        assert (64 if header[1]else 56)==length and offset in (0,64)
        assert after[offset:offset+length]==bytes(expected['supertiles'])
        assert after[:offset]==before[:offset] and after[offset+length:]==before[offset+length:]
        loaded_bytes+=length
    verified_transitions=[]
    for record in transitions:
        stage=stages[record['stage']];alternate=stage['alternate']
        events=[e for e in record['events']if e['routine']=='alternate'];assert len(events)==1
        header=bytes.fromhex(events[0]['after']['header'])
        expected=alternate['collisionLimits']+alternate['cycleIndices']+alternate['paletteIndices']
        assert header[8]==255 and header[9:24]==bytes(expected)
        expected_preset=stage.get('baseBoss',{}).get('chrPreset',alternate['chrPreset'])
        chr_events=[e for e in record['events']if e['routine']=='chr']
        if expected_preset is None:assert not chr_events
        else:
            assert len(chr_events)==1 and chr_events[0]['inputA']==expected_preset
            pages=next(p['pages']for p in data['chrPresets']if p['id']==expected_preset)
            assert bytes.fromhex(chr_events[0]['after']['chr'])==bytes(pages)
        if 'baseBoss'in stage:
            boss=stage['baseBoss'];observed=record['loader'];entry=data['extraMapSets']['base-boss']['entries'][boss['mapIndex']]
            assert observed['input']==observed['beforeA']==boss['mapIndex']
            assert observed['pointer']==entry['pointer']
            assert bytes.fromhex(observed['header'])[2:8]==bytes.fromhex('e3908ab68abe')
            actual=bytes.fromhex(observed['after']);offset=observed['offset']
            assert actual[offset:offset+56]==bytes(entry['supertiles'])
            assert record['final']['sceneType']==128 and record['final']['screen']==stage['stopScrollScreen']
        verified_transitions.append({'stage':stage['stage'],'chrPreset':expected_preset,
                                     'paletteCollisionBytes':15,'baseBoss':'baseBoss'in stage})
    rom,_=original();chr=rom[16+131072:];entries=[]
    for stage in data['stages']:
        n=stage['stage'];entry=ROOT/'.local/contra-campaign/runs'/f'stage-entry-v2-{n}'
        header=(entry/'initialized-header.bin').read_bytes();assert header==bytes(stage['header']['bytes'])
        ram=(entry/'ram.bin').read_bytes()[1399*2048:1400*2048];ppu=(entry/'frame-01400-ppu.bin').read_bytes()
        rows=stage['maps']['mapRows'];matches=[]
        for offset in (0x600,0x640):
            matches.append([m['index']for m in stage['maps']['entries']if bytes(m['supertiles'])==ram[offset:offset+rows*8]])
        assert 0 in matches[0]
        if stage['sceneType']==0:assert 1 in matches[1]
        aliases=[]
        for slot in range(8):
            found=[n for n in range(128)if ppu[slot*1024:(slot+1)*1024]==chr[n*1024:(n+1)*1024]]
            assert found;aliases.append(found)
        entries.append({'stage':n,'source':json.loads((entry/'source.json').read_text()),
                        'intervention':json.loads((entry/'stage-intervention.json').read_text()),
                        'initializedHeaderMatches':True,'mapWindowMatches':matches,
                        'desiredCHR':list(ram[0x7f0:0x7f8]),'actualCHRPageAliases':aliases,
                        'ramSHA256':sha(ram),'ppuSHA256':sha(ppu)})
    OUT.mkdir(parents=True,exist_ok=True)
    packed=gzip.compress(json.dumps(data,separators=(',',':')).encode(),mtime=0)
    (OUT/'levels-catalog.json.gz').write_bytes(packed)
    shutil.copyfile(loader_path,OUT/'levels-loader-original.json.gz')
    shutil.copyfile(transition_path,OUT/'levels-transitions-original.json.gz')
    functions=[]
    for group in ['fixed','header']:
        raw=(BASE/f'{group}-rea/evidence.json').read_bytes()
        (OUT/f'levels-{group}-rea.json.gz').write_bytes(gzip.compress(raw,mtime=0))
        for p in sorted((BASE/f'{group}-rea').glob('function-*.json')):
            d=json.loads(p.read_text());functions.append({'entry':d['parameters']['procedure'],'evidence_id':d['evidence_id'],
                                                         'assembly':d['normalized_result']['assembly'],'limitations':d['limitations']})
    report={'passed':True,'romSHA256':data['romSHA256'],'stages':8,'normalMapEntries':144,'baseBossMapEntries':2,
            'actualOriginalMapLoads':len(loads),'actualLoadedSupertileBytes':loaded_bytes,
            'mapEntryCounts':[s['maps']['entryCount']for s in data['stages']],
            'tileSets':len(data['tileSets']),'verifiedTransitions':verified_transitions,
            'controlledStageEntries':entries,'functions':functions,
            'scope':'Original function and transition execution at declared controlled inputs, not natural campaign traversal.',
            'limitations':['Ordinary initialization CHR presets differ from later animated desired/applied pages; see separate animation recovery.',
                           'E09C Ghidra dossier includes a conservative fallthrough into E10A; admitted map streams terminate within E09C..E109.',
                           'C848..C857 are CHR-animation dispatch data, not graphics-queue opcodes or instructions.'],
            'files':{p.name:sha(p.read_bytes())for p in sorted(OUT.glob('levels-*.json.gz'))}}
    (OUT/'levels-verification.json').write_text(json.dumps(report,indent=2)+'\n')
    print(json.dumps({k:report[k]for k in ['passed','stages','normalMapEntries','baseBossMapEntries','actualOriginalMapLoads','actualLoadedSupertileBytes','tileSets']}))


if __name__=='__main__':main()
