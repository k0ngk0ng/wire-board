#!/usr/bin/env python3
"""Reconstruct captured native voices using measured gain; never synthesize engines.

Requires ffmpeg and numpy. All inputs/outputs must remain in this repository.
Media capture already contains native pitch/rate changes; never time-stretch again.
"""
import argparse
import hashlib
import json
import os
from pathlib import Path
import subprocess
import wave
import numpy as np

ROOT=Path(__file__).resolve().parents[2]
RATE=48000
VEHICLES=('p911','fm50','ldia','mcf1','ccor')

def sha(path):return hashlib.sha256(path.read_bytes()).hexdigest()
def local(value):
    p=(ROOT/value).resolve()
    if not p.is_relative_to(ROOT):raise ValueError('audio evidence must remain in repository')
    return p

def longest(mask,step_ms):
    best=run=0
    for value in mask:
        run=run+1 if value else 0;best=max(best,run)
    return best*step_ms

def statistics(pcm,start,end):
    part=pcm[max(0,int(start*RATE)):min(len(pcm),int(end*RATE))]
    step=96;part=part[:len(part)//step*step]
    if not len(part):return {}
    rms=np.sqrt(np.mean(part.reshape(-1,step)**2,axis=1));median=float(np.median(rms))
    silent=rms<median*.05;runs=[];begin=None
    for i,value in enumerate(np.append(silent,False)):
        if value and begin is None:begin=i
        if not value and begin is not None:
            if(i-begin)*2>=20:runs.append({'startSeconds':start+begin*.002,'durationMS':(i-begin)*2})
            begin=None
    return {'seconds':len(part)/RATE,'rmsMedian':median,'rmsP01':float(np.percentile(rms,1)),
        'peak':float(np.max(np.abs(part))),'longestBelow1e_5MS':longest(rms<1e-5,2),
        'longestBelow1e_4MS':longest(rms<1e-4,2),'longestBelow5pctMedianMS':longest(silent,2),'nearSilenceRunsAtLeast20MS':runs,
        'thresholdMeaning':'descriptive 2ms-window output statistics, not an asserted listening-quality pass'}

def reconstruct(directory,vehicle,label,out):
    source=directory/('audio-native-output-'+vehicle+'.json');d=json.loads(source.read_text());origin=d['startedAt'];duration=(d['stoppedAt']-origin)/1000
    mix=np.zeros(int(np.ceil(duration*RATE)),dtype=np.float64);timeline=d['mediaTimeline'];clips=[];decoded={}
    env=dict(os.environ,TMPDIR=str(ROOT/'.local/racing-native-audio-analysis/tmp'))
    for clip in d['audioArtifacts']:
        path=(directory/clip['file']).resolve()
        if not path.is_relative_to(directory.resolve()):raise ValueError('invalid recorded filename')
        result=subprocess.run(['ffmpeg','-v','error','-i',str(path),'-ac','1','-ar',str(RATE),'-f','f32le','pipe:1'],capture_output=True,check=True,env=env)
        pcm=np.frombuffer(result.stdout,dtype='<f4').astype(np.float64);decoded[clip['id']]=pcm
        points=[]
        for row in timeline:
            voice=next((v for v in row['voices']if v.get('id')==clip['id']),None)
            if voice:
                gain=voice['volume'] if not voice.get('paused')and voice.get('connected')and not voice.get('muted',False)and voice.get('ready',0)>=3 else 0
                points.append(((row['at']-origin)/1000,gain))
        offset=max(0,int(round((clip['startedAt']-origin)/1000*RATE)));count=min(len(pcm),len(mix)-offset)
        if points and count>0:
            t=offset/RATE+np.arange(count)/RATE
            p=np.asarray(points);gain=np.interp(t,p[:,0],p[:,1],left=p[0,1],right=0)
            mix[offset:offset+count]+=pcm[:count]*gain
        clips.append({'file':str(path.relative_to(ROOT)),'sha256':sha(path),'key':clip['key'],'recordId':clip['id'],
            'startSeconds':(clip['startedAt']-origin)/1000,'decodedSeconds':len(pcm)/RATE,'gainPoints':len(points)})
    dest=out/(label+'-'+vehicle+'-native-voice-envelope-mix.wav')
    peak=float(np.max(np.abs(mix))) if len(mix)else 0
    with wave.open(str(dest),'wb')as f:
        f.setnchannels(1);f.setsampwidth(2);f.setframerate(RATE);f.writeframes(np.rint(np.clip(mix,-1,1)*32767).astype('<i2').tobytes())
    native_ids={v.get('nativeId',v.get('id'))for row in timeline for v in row['voices']if v.get('nativeId',v.get('id'))is not None}
    audible=[v for row in timeline for v in row['voices']if v['volume']>.005 and not v['paused']]
    missing=sum(v.get('id')is None for v in audible)
    edges=[]
    for event in d['mediaEvents']:
        if event['type']!='seeking':continue
        row=min(timeline,key=lambda r:abs(r['at']-event['at']));voice=next((v for v in row['voices']if v['key']==event['key']),None)
        if not voice or voice['volume']<=.005 or voice.get('id')not in decoded:continue
        clip=next(c for c in d['audioArtifacts']if c['id']==voice['id']);pcm=decoded[voice['id']];at=(event['at']-clip['startedAt'])/1000;n=int(at*RATE)
        if n<960 or n+960>=len(pcm):continue
        delta=np.abs(np.diff(pcm[max(0,n-960):min(len(pcm),n+960)]))
        nearby=np.abs(np.diff(pcm[max(0,n-9600):min(len(pcm),n+9600)]));p99=float(np.percentile(nearby,99))
        edges.append({'key':event['key'],'atSeconds':(event['at']-origin)/1000,'gain':voice['volume'],
            'maxStep20ms':float(np.max(delta)),'neighbourStepP99':p99,'ratio':float(np.max(delta))/p99 if p99 else None})
    phases={'idle':statistics(mix,.25,min(7.75,duration-.1)),'held_load':statistics(mix,8.3,min(15.7,duration-.1)),
        'acceleration':statistics(mix,16,min(23,duration-.1)),'brake_and_idle':statistics(mix,23,duration-.1)}
    return {'source':str(source.relative_to(ROOT)),'sourceSHA256':sha(source),'wav':str(dest.relative_to(ROOT)),
        'wavSHA256':sha(dest),'durationSeconds':duration,'peakBeforePCMClamping':peak,'clippedSamples':int(np.count_nonzero(np.abs(mix)>1)),
        'nativeInstancesObserved':len(native_ids),'nativeInstanceMethod':'native WeakMap element ID'if any('nativeId'in v for r in timeline for v in r['voices'])else'one capture ID per observed native element; lower bound',
        'recordIntervals':len(clips),'audibleRowsWithoutCaptureId':missing,'audibleRows':len(audible),
        'mutedFieldMissingRows':sum('muted'not in v for r in timeline for v in r['voices']),
        'beforeAfterNativeMuted':[[{'key':v['key'],'muted':v['muted']}for v in d[side]['audio']]for side in('before','after')],
        'captureErrors':d['captureErrors'],'phases':phases,'audibleSeekingEdges':edges,'clips':clips}

def continuous(directory,vehicle,out):
    source=directory/('audio-continuous-output-'+vehicle+'.json');d=json.loads(source.read_text())
    if len(d['audioArtifacts'])!=1:raise ValueError('continuous evidence must contain exactly one actual mixed capture')
    clip=d['audioArtifacts'][0];path=(directory/clip['file']).resolve()
    if not path.is_relative_to(directory.resolve()):raise ValueError('invalid mixed recording filename')
    env=dict(os.environ,TMPDIR=str(ROOT/'.local/racing-native-audio-analysis/tmp'))
    result=subprocess.run(['ffmpeg','-v','error','-i',str(path),'-ac','1','-ar',str(RATE),'-f','f32le','pipe:1'],capture_output=True,check=True,env=env)
    pcm=np.frombuffer(result.stdout,dtype='<f4').astype(np.float64);origin=d['startedAt'];timeline=d['mediaTimeline']
    points=np.asarray([((r['at']-origin)/1000,r['master']if r['connected']and not r['paused']and not r['muted']and r['ready']>=3 else 0)for r in timeline])
    t=np.arange(len(pcm))/RATE;gain=np.interp(t,points[:,0],points[:,1],left=points[0,1],right=points[-1,1]);mix=pcm*gain
    dest=out/(vehicle+'-actual-continuous-mix-native-master.wav')
    with wave.open(str(dest),'wb')as f:
        f.setnchannels(1);f.setsampwidth(2);f.setframerate(RATE);f.writeframes(np.rint(np.clip(mix,-1,1)*32767).astype('<i2').tobytes())
    duration=len(pcm)/RATE
    phases={'idle':statistics(mix,.5,min(7.5,duration-.1)),'held_load':statistics(mix,8.5,min(15.5,duration-.1)),
            'acceleration':statistics(mix,16.5,min(22.8,duration-.1)),'brake_and_idle':statistics(mix,23.3,duration-.1),
            'whole_capture':statistics(mix,0,duration)}
    if d.get('captureOnly'):phases={'idle':statistics(mix,.5,duration-.5),'whole_capture':statistics(mix,0,duration)}
    keys=sorted({v['key']for r in timeline for v in r['voices']})
    return {'source':str(source.relative_to(ROOT)),'sourceSHA256':sha(source),'recording':str(path.relative_to(ROOT)),
            'recordingSHA256':sha(path),'wav':str(dest.relative_to(ROOT)),'wavSHA256':sha(dest),'durationSeconds':duration,
            'captureMethod':d['captureMethod'],'captureErrors':d['captureErrors'],'nativeSinkCounts':sorted({r['nativeSinks']for r in timeline}),
            'nativePlaybackRates':sorted({r['rate']for r in timeline}),'sourceTrackIds':sorted({t['id']for r in timeline for t in r['sourceTracks']}),
            'voiceKeys':keys,'peakBeforePCMClamping':float(np.max(np.abs(mix))),'clippedSamples':int(np.count_nonzero(np.abs(mix)>1)),
            'phases':phases,'nativeMediaEvents':d['mediaEvents'],
            'continuityChecks':{phase:{'nonSilent':phases[phase].get('rmsMedian',0)>1e-5,
                                     'longestNearSilenceMS':phases[phase].get('longestBelow5pctMedianMS'),
                                     'passed':phases[phase].get('rmsMedian',0)>1e-5 and phases[phase].get('longestBelow5pctMedianMS',999)<=20}
                                for phase in (('idle',)if d.get('captureOnly')else('idle','held_load'))},
            'controlProtocol':d.get('controlProtocol'),'captureOnly':d.get('captureOnly',False),
            'excludedStableWindowEdgesSeconds':[[0,.5],[duration-.5,duration]]if d.get('captureOnly')else[[0,.5],[7.5,8.5],[15.5,16.5]],
            'gainHandling':'actual mixed stream already contains every voice gain/pitch; multiply native master exactly once, with muted/paused/disconnected rows zero; never re-mix per voice or time-stretch',
            'normalization':'none; captured mixed PCM with measured native master; mono channel conversion and 16-bit WAV quantization only'}

def main():
    parser=argparse.ArgumentParser();parser.add_argument('--a');parser.add_argument('--b');parser.add_argument('--continuous');parser.add_argument('--vehicle',choices=VEHICLES);parser.add_argument('--out',required=True);args=parser.parse_args()
    if bool(args.continuous)==bool(args.a or args.b):parser.error('choose --continuous or both --a and --b')
    out=local(args.out)
    if out.exists():raise ValueError('use a new dedicated output directory')
    out.mkdir(parents=True)
    if args.continuous:
        directory=local(args.continuous);recordings={v:continuous(directory,v,out)for v in ((args.vehicle,)if args.vehicle else VEHICLES)}
        passed=all(check['passed']for recording in recordings.values()for check in recording['continuityChecks'].values())
        report={'passed':passed,'method':'actual unique native HTMLAudio mixed-stream sampling plus measured native master envelope; not per-voice synthesis or speaker loopback',
                'frozenEngineeringContinuityBudget':{'windowMS':2,'nearSilenceThreshold':'<5% of each stable window RMS median','maximumContinuousNearSilenceMS':20,'scope':'stable idle and stationary load only; every transition and whole capture also retained; not a subjective listening score'},
                'sampleRate':RATE,'limitations':['40ms native master timeline and MediaRecorder timestamp alignment are approximate, not sample-exact system loopback.',
                                               'Opus may smooth single-sample edges; original browser recording retained.',
                                               'Near-silence RMS windows are descriptive evidence and do not alone establish subjective audio quality.'],
                'recordings':recordings}
        (out/'actual-continuous-audio.json').write_text(json.dumps(report,ensure_ascii=False,indent=2)+'\n')
        print(json.dumps({v:{'wav':r['wav'],'duration':r['durationSeconds'],'checks':r['continuityChecks'],'idle':r['phases']['idle'],'held_load':r['phases'].get('held_load')}for v,r in recordings.items()},ensure_ascii=False,indent=2))
        if not passed:raise SystemExit(1)
        return
    if not(args.a and args.b):parser.error('both --a and --b are required for old native voice A/B')
    a,b=local(args.a),local(args.b)
    rows={label:{v:reconstruct(directory,v,label,out)for v in VEHICLES}for label,directory in(('A',a),('B',b))}
    report={'method':'actual native voice capture + measured volume envelope reconstruction; not speaker output or offline engine synthesis',
        'sampleRate':RATE,'fixedPlaybackScale':1,'rateHandling':'captured audio already includes native playbackRate; no second time stretch',
        'gainHandling':'timeline.volume already includes native master level; multiply once; paused/disconnected/muted weights are zero',
        'mixNormalization':'none; plain PCM sum for every A/B recording',
        'limitations':['MediaRecorder Opus may smooth single-sample peaks; original captures retained.',
            '40ms measured envelopes and browser recording start timestamps are approximate alignment, not sample-exact system loopback.',
            'B capture intervals use audible gain hysteresis .005/.003 to avoid Chromium zero-gain capture/EOS stalls; tiny excluded tails are not represented.',
            'Original run schema lacks per-row muted; before/after muted flags and absence of mute commands are retained, not claimed as a mute-transition waveform test.'],
        'recordings':rows}
    (out/'native-audio-ab.json').write_text(json.dumps(report,ensure_ascii=False,indent=2)+'\n')
    print(json.dumps({label:{v:{'wav':r['wav'],'nativeInstances':r['nativeInstancesObserved'],'intervals':r['recordIntervals'],'missingAudibleRows':r['audibleRowsWithoutCaptureId'],'peak':r['peakBeforePCMClamping'],'idleSilentMS':r['phases']['idle'].get('longestBelow1e_5MS')}for v,r in values.items()}for label,values in rows.items()},ensure_ascii=False,indent=2))
if __name__=='__main__':main()
