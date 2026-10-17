"""Derive each court's own complete driver-phase loop from retained captures.

Source PCM and captures are immutable inputs. Missing captures are reported;
this command never launches or restarts an emulator, and never rewrites NY66.
"""
from pathlib import Path
import array
import collections
import csv
import hashlib
import json
import math
import sys
import wave

ROOT=Path(__file__).resolve().parents[2]
OUT=ROOT/'.local/basketball/audio/export'
FRAMES={59:213000,60:20000,61:69000,62:250000,63:224000,64:59000,65:86000}
BLEND=1411


def sha(path):
    h=hashlib.sha256()
    with path.open('rb') as f:
        while chunk:=f.read(1024*1024):h.update(chunk)
    return h.hexdigest()


def samples(path):
    with wave.open(str(path)) as f:
        assert (f.getnchannels(),f.getsampwidth(),f.getframerate())==(1,2,44100)
        a=array.array('h',f.readframes(f.getnframes()))
    if sys.byteorder!='little':a.byteswap()
    return a


def save_json(path,value):
    temporary=path.with_name(path.name+'.tmp')
    temporary.write_text(json.dumps(value,indent=2)+'\n')
    temporary.replace(path)


def transition(a,start,end):
    incoming=a[end-BLEND:end];target=a[start-BLEND:start]
    mixed=array.array('h',(round((1-w)*x+w*y) for i,(x,y) in enumerate(zip(incoming,target)) for w in [(1-math.cos(math.pi*i/(BLEND-1)))/2]))
    if abs((a[start]-mixed[-1])-(a[start]-a[start-1]))>1:return None
    if max(abs(x-y) for x,y in zip(mixed[-16:],a[start-16:start]))>2:return None
    levels=[]
    for k in range(0,BLEND-353+1,88):
        x=incoming[k:k+353];y=target[k:k+353];z=mixed[k:k+353]
        w=(1-math.cos(math.pi*(k+176)/(BLEND-1)))/2
        expected=((1-w)*sum(v*v for v in x)+w*sum(v*v for v in y))/353
        actual=sum(v*v for v in z)/353
        db=10*math.log10(actual/expected) if expected and actual else 0
        if abs(db)>2:return None
        levels.append(db)
    old_step=max(abs(x[i]-x[i-1]) for x in [a[end-BLEND-1:end],a[start-BLEND-1:start]] for i in range(1,BLEND+1))
    new_step=max(abs(mixed[i]-mixed[i-1]) for i in range(1,BLEND))
    if new_step>old_step*1.05+2:return None
    return mixed,levels,new_step,old_step


def derive(ident,run,output,selection=None):
    driver=(OUT/run['driver']).read_bytes()
    jumps=[{k:int(v) for k,v in row.items()} for row in csv.DictReader((OUT/run['jumps']).open()) if row['channel']=='0']
    assert len(jumps)>=2 and jumps[0]['target']==jumps[1]['target']
    tempo_counts=collections.Counter(driver[i+5] for i in range(0,len(driver),231))
    tempo=tempo_counts.most_common(1)[0][0]
    assert set(tempo_counts)<=set([255,tempo]),'variable-tempo source needs separate analysis'
    first,second=jumps[0]['step'],jumps[1]['step']
    phase_a,phase_b=driver[(first-1)*231+6],driver[(second-1)*231+6]
    tick_numerator=(second-first)*tempo+phase_a-phase_b
    assert tick_numerator%256==0
    ticks=tick_numerator//256
    period=ticks*256//math.gcd(ticks*256,tempo)
    loops=period*tempo//(ticks*256)
    cursor=[int(row['samples']) for row in csv.DictReader((OUT/run['samples']).open())]
    source=samples(OUT/run['wav'])
    assert max(abs(v) for v in source)<32767,'clipped original render'
    chosen=None
    offsets=[selection] if selection is not None else [-1,0,-2,1,-3,2,-4,3]
    for offset in offsets:
        start_frame=first+offset;end_frame=start_frame+period
        assert end_frame<len(cursor),'capture does not span full driver-phase period'
        assert driver[(start_frame-1)*231:start_frame*231]==driver[(end_frame-1)*231:end_frame*231],(ident,start_frame,end_frame,'driver differs')
        start,end=cursor[start_frame],cursor[end_frame]
        candidate=transition(source,start,end)
        if candidate is None:continue
        mixed,levels,new_step,old_step=candidate
        chosen=(start_frame,end_frame,start,end,offset,mixed,levels,new_step,old_step)
        break
    if chosen is None:raise RuntimeError(f'{ident}: no near-loop-boundary32ms transition meets the fixed seam/RMS criteria')
    sf,ef,start,end,offset,mixed,levels,new_step,old_step=chosen
    if sys.byteorder!='little':source.byteswap();mixed.byteswap()
    with wave.open(str(output),'wb') as f:
        f.setparams((1,2,44100,end-start,'NONE','not compressed'))
        f.writeframesraw(memoryview(source).cast('B')[start*2:(end-BLEND)*2])
        f.writeframesraw(mixed.tobytes())
    return {'id':ident,'driver_frame_range':[sf,ef],'sample_range':[start,end],'loop_jump':jumps[0],
        'loop_anchor_offset_frames':offset,'tempo':tempo,'music_ticks_per_sequence_loop':ticks,'sequence_loops':loops,
        'period_frames':period,'seconds':(end-start)/44100,'blend_samples':BLEND,
        'processing':'original PCM; only final1411samples use raised-cosine convex blend with corresponding pre-start original passage; unchanged complete driver-phase period',
        'local_rms_db':levels,'max_transition_step':new_step,'max_source_step':old_step,
        'listening_accepted':False,'accepted_for_release':False}


def main():
    selected=[int(x) for x in sys.argv[1:]] or list(FRAMES)
    assert set(selected)<=set(FRAMES)
    playback_path=ROOT/'games/basketball/assets/audio-playback.json'
    playback=json.loads(playback_path.read_text())
    manifest_path=OUT/'court-music-manifest.json'
    manifest=json.loads(manifest_path.read_text()) if manifest_path.exists() else {
        'schema':1,'accepted_for_release':False,'license':{'source':'user-supplied original ROM','redistribution_permission':'not established'},
        'tracks':[{'id':66,'reused':True,'source_manifest':'music-066-loop-source.json'}]}
    assert playback['music']['66']['sha256']=='d06fe1429fdca09c0e9fd5b77687b7603bff21812e58630c58b805f12404ff87'
    for ident in selected:
        frames=FRAMES[ident]
        runs=[]
        for n in (1,2):
            info=OUT/f'court-music-{ident:03}-{frames}-run-{n}/capture-info.json'
            if not info.exists():raise RuntimeError(f'missing completed source {info}')
            data=json.loads(info.read_text());assert data['settings']['id']==ident and data['settings']['frames']==frames
            runs.append(data['files'])
        hashes={kind:sha(OUT/path) for kind,path in runs[0].items()}
        assert all(sha(OUT/path)==hashes[kind] for kind,path in runs[1].items())
        existing=next((x for x in manifest['tracks'] if x['id']==ident),None)
        if existing and existing.get('capture_sha256')==hashes and sha(OUT/playback['music'][str(ident)]['wav'])==existing['sha256']:
            print('reused verified derivative',ident,flush=True);continue
        outputs=[OUT/f'music-{ident:03}-loop-derived-{n}.wav' for n in (1,2)]
        first=derive(ident,runs[0],outputs[0]);second=derive(ident,runs[1],outputs[1],first['loop_anchor_offset_frames'])
        assert first==second and sha(outputs[0])==sha(outputs[1])
        output=OUT/f'music-{ident:03}-loop.wav'
        if output.exists():output.unlink()
        # Same immutable bytes, avoiding a third large duplicate on disk.
        output.hardlink_to(outputs[0])
        track=dict(first,captures=runs,capture_sha256=hashes,sha256=sha(output),
            repeated_outputs=[p.name for p in outputs],source_rom_sha256=json.loads((ROOT/'games/basketball/assets/court-music-source.json').read_text())['rom_sha256'],
            derivation_script_sha256=sha(Path(__file__).resolve()))
        save_json(OUT/f'music-{ident:03}-loop-source.json',track)
        manifest['tracks']=[x for x in manifest['tracks'] if x['id']!=ident]+[track]
        playback['music'][str(ident)]={'wav':output.name,'loop':True,'sha256':track['sha256'],'source_manifest':f'music-{ident:03}-loop-source.json','processing':track['processing'],'listening_accepted':False}
        save_json(manifest_path,manifest);save_json(playback_path,playback)
        print('derived',ident,'seconds',track['seconds'],'sequence loops',track['sequence_loops'],'rms max',max(abs(x) for x in track['local_rms_db']),flush=True)


if __name__=='__main__':main()
