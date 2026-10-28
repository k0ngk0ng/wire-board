"""Retain source/priority/timing evidence and publish a local research catalog.

This writes metadata only. Audio remains inside .local/contra-audio, and the
catalog explicitly refuses release/seam/multi-media-mix acceptance claims.
"""
import array
import csv
import gzip
import hashlib
import json
from pathlib import Path
import shutil
import wave
import zipfile

ROOT=Path(__file__).resolve().parents[2]
BASE=ROOT/'.local/contra-audio'
EVIDENCE=ROOT/'docs/research/nes-contra/evidence'


def sha(p):return hashlib.sha256(p.read_bytes()).hexdigest()
def pcm(p):
    with wave.open(str(p))as f:return array.array('h',f.readframes(f.getnframes()))


def main():
    full=json.loads((BASE/'export/manifest.json').read_text())
    verification=json.loads((BASE/'export/verification.json').read_text());assert verification['passed']
    validation_by_id={s['id']:s for s in verification['sounds']}
    stems=json.loads((BASE/'stems/manifest.json').read_text());stem_verify=json.loads((BASE/'stems/verification.json').read_text());assert stem_verify['passed']
    assert {s['id'] for s in full['sounds']} == {s['id'] for s in stems['sounds']}
    assert len(full['sounds']) == len(stems['sounds'])
    assert set(validation_by_id) == {s['id'] for s in full['sounds']}
    stem_by_id={s['id']:s for s in stems['sounds']}
    native=json.loads((EVIDENCE/'audio-requests.json').read_text())
    priority=json.loads((EVIDENCE/'audio-priority.json').read_text());assert priority['passed']
    with zipfile.ZipFile('/Users/jason/Documents/nes/roms/03/魂斗罗日版(完全版).zip')as z:rom=z.read('Contra (J).nes')
    prg=rom[16:131088];bank=prg[0x4000:0x8000];table=int.from_bytes(bank[0x717:0x719],'little')
    entries=[];timelines=[]
    for sound in full['sounds']:
        sid=sound['id'];run=BASE/'export'/sound['runs'][0]
        driver=(run/'driver.bin').read_bytes();calls=list(csv.DictReader((run/'calls.csv').open()))
        cursors=list(csv.DictReader((run/'samples.csv').open()))
        definition=[]
        if sid<0x7f:
            offset=table-0x8000+sid*3;count=((bank[offset]>>3)&3)+1
            for i in range(count):
                at=offset+i*3;definition.append({'slot':bank[at]&7,'control':bank[at],'sequenceAddress':hex(int.from_bytes(bank[at+1:at+3],'little'))})
        source_start=sound['requestFrame']
        lifetimes=[]
        for definition_row in definition:
            slot=definition_row['slot'];active=[f for f in range(source_start,sound['captureFrames']) if driver[f*130+6+slot]==sid]
            if active:
                lifetimes.append({'slot':slot,'firstUpdateTick':active[0]-source_start,
                                  'releaseUpdateTick':active[-1]+1-source_start if active[-1]+1<sound['captureFrames'] else None})
        entry={k:sound[k] for k in ['id','label','kind','file','sha256','durationSeconds','sampleRange','requestFrame','requestSampleOrigin','warmupFrames']}
        entry.update(hexID=f'0x{sid:02X}',researchDirectory='.local/contra-audio/export',sourceTable=hex(table+sid*3) if sid<0x7f else None,
                     slotDefinitions=definition,slotLifetimes=lifetimes,dpcmRegions=validation_by_id[sid]['dpcmRegions'],
                     releaseMeaning='Original slot ownership expiry is independent of the WAV filter tail; do not use media end as the priority timer.')
        entry['stems']=[{k:v for k,v in s.items() if k!='runs'}|{'researchDirectory':'.local/contra-audio/stems'} for s in stem_by_id[sid]['stems']]
        if sound['kind']=='music':
            loop=sound['loop'];first,last=loop['driverFrames'];period=last-first
            events=[{'frame':int(r['frame']),'id':int(r['value']),'ordinal':i} for i,r in enumerate(calls)
                    if r['kind']=='request' and int(r['value']) in (2,127,128,129)]
            one=[(r['frame']-first,r['id'])for r in events if first<=r['frame']<last]
            two=[(r['frame']-last,r['id'])for r in events if last<=r['frame']<last+period]
            assert one==two and one
            timeline={'id':sid,'requestFrame':source_start,'loopStartTick':first-source_start,'loopTicks':period,
                      'repeatEventsMatch':True,'events':[]}
            for r in events:
                f=r['frame'];timeline['events'].append({'audioTick':f-source_start,'id':r['id'],'ordinal':r['ordinal'],
                    'sampleWindow':[int(cursors[f]['samples'])-sound['sampleRange'][0],int(cursors[f+1]['samples'])-sound['sampleRange'][0]]})
            timelines.append(timeline)
            samples=pcm(run/'full.wav');a,b=loop['sourceSampleRange']
            entry['loop']=loop|{'seconds':(b-a)/44100,'rawBoundaryStep':abs(samples[a]-samples[b-1]),'originalAdjacentStep':abs(samples[a]-samples[a-1]),
                               'eventRepeatCount':len(one),'seamReadiness':'Unprocessed candidate; no seamless-loop or listening acceptance.'}
        entries.append(entry)
    catalog={'format':'contra-j-audio-source-v1','romSHA256':full['romSHA256'],'acceptedForRelease':False,
             'tool':full['tool'],
             'sampleRate':44100,'channels':1,'sampleBits':16,'renderVolume':192,
             'stageMusic':[r['bgm']['id']for r in native['runs'] if r['kind']=='stage-initialization'],
             'stageClearTable':list(prg[0x1ce32:0x1ce3a]),
             'stageClearTableMeaning':'CE27 reads CE32 by stage; zero defers/no immediate request. Do not broadcast 0x62 for every boss defeat.',
             'baseBossMusic':{'id':0x5e,'source':'fixed DD28 after sceneType=0x80'},
             'gameOverCue':{'id':0x6e,'source':'fixed CD96'},
             'sounds':entries,'percussionTimeline':'docs/research/nes-contra/evidence/audio-percussion-timeline.json.gz',
             'priorityEvidence':'docs/research/nes-contra/evidence/audio-priority.json',
             'platformContract':{'api':'BoardProps.audio.track/stopTrack/muteTrack/pauseTrack/resumeTrack',
               'gameRules':'Per-slot acceptance, retransmit/epoch deduplication, music event timeline and pause state belong to Contra.',
               'musicPulseOverride':'Effect slot4 overrides physical pulse1 while the underlying music slot0 advances. Background pulse1 must not be paused for an effect.',
               'noiseOverride':'Music noise request0x02 and game effects share slot5; rejected noise requests are discarded, not queued.',
               'dpcm':'0x7F..81 directly start DPCM and bypass normal slot priority; DPCM is a separate physical channel.',
               'sameTickDuplicates':'Keep at most the last accepted request per slot within a tick; equal IDs in later ticks are genuine retriggers.',
               'audioPhase':'Game F9BC requests are serviced at the following original audio update; music-internal percussion is generated in the current audio update.'},
             'limitations':['No upload has been performed. The local controller/adapter is verified separately.',
               'Original whole clips cannot be blindly overlaid; partial slot replacement needs stems/timeline rules.',
               'Raw continuous captures are not loop-ready; verified loop derivatives are catalogued separately. Multi-track sync and complete gameplay mixing remain unaccepted.',
               'Separate channel rendering does not recreate the original nonlinear pulse/TND mix or all oscillator-history effects.',
               'Source media are local research candidates. Distribution rights are not established by ROM/content identity.']}
    assets=ROOT/'games/contra/assets';assets.mkdir(parents=True,exist_ok=True)
    (assets/'audio-source.json').write_text(json.dumps(catalog,ensure_ascii=False,indent=2)+'\n')
    (EVIDENCE/'audio-percussion-timeline.json.gz').write_bytes(gzip.compress(json.dumps({'source_is_original_execution':True,'timelines':timelines},separators=(',',':')).encode(),mtime=0))
    bundles=[];functions=[]
    for name in ['rea','extra-rea','transition-rea']:
        bundles.append({'name':name,'bundle':json.loads((BASE/name/'evidence.json').read_text())})
        for path in sorted((BASE/name).glob('function-*.json')):
            d=json.loads(path.read_text());functions.append({'address':d['parameters']['procedure'],'evidenceID':d['evidence_id'],'assembly':d['normalized_result']['assembly'],'limitations':d['limitations']})
    (EVIDENCE/'audio-rea.json.gz').write_bytes(gzip.compress(json.dumps(bundles,separators=(',',':')).encode(),mtime=0))
    (EVIDENCE/'audio-index.json').write_text(json.dumps({'functions':functions,'source':json.loads((BASE/'source.json').read_text()),
                'soundTable':hex(table),'catalogSHA256':sha(assets/'audio-source.json')},indent=2)+'\n')
    sourcefiles=[ROOT/f'scripts/contra/{name}' for name in ['audio_prepare.py','audio_trace.py','audio_export.py','audio_verify.py','audio_stems.py','audio_stems_verify.py','audio_priority.py','audio_catalog.py','audio_wave_bridge.c']]
    report={'passed':True,'scope':'Original requests, slot-priority matrix, quiet-warmed offline PCM/channel isolation and numerical repeat checks; no browser, listening, loop-seam or complete-game claim.',
            'resources':len(entries),'effects':sum(s['kind']=='effect' for s in entries),'cues':sum(s['kind']=='cue' for s in entries),
            'music':len(timelines),'loopEffects':sum(s['kind']=='loop-effect' for s in entries),'percussion':sum(s['kind']=='percussion' for s in entries),'stems':stem_verify['stemCount'],
            'priorityCases':len(priority['cases']),'stageInitializersRAMIdentical':8,
            'verification':verification,'stemsVerification':stem_verify,
            'sourceSHA256':{str(p.relative_to(ROOT)):sha(p) for p in sourcefiles},
            'catalogSHA256':sha(assets/'audio-source.json')}
    (EVIDENCE/'audio-verification.json').write_text(json.dumps(report,indent=2)+'\n')
    for source,name in [(BASE/'checks/export-red.log','audio-export-red.log'),(BASE/'checks/warmup-red.log','audio-warmup-red.log'),(BASE/'checks/stems-red.log','audio-stems-red.log')]:shutil.copyfile(source,EVIDENCE/name)
    print(json.dumps({k:report[k] for k in ['passed','resources','effects','cues','music','percussion','stems','priorityCases']}))


if __name__=='__main__':main()
