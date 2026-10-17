"""Export supplementary original requests; reuse verified captures and never rerender the original 27."""
from pathlib import Path
import array,fcntl,hashlib,json,wave
from extract_audio import BASE,OUT,ROOT,SOURCE_SHA,carrier,record

SOURCES={1:'captured_player.go ActionCapturedRun (original captured-player source oracle)',48:'world_auxiliary_running.go selected.Handler8 A release; AUXILIARY_RUNNING_B.md / auxiliary-running-controller source chains',10:'shot_team_special.go (original BB6D)',13:'court_animation.go',14:'court_animation.go',30:'special_ball.go',46:'court_animation.go'}


def sha(p):return hashlib.sha256(p.read_bytes()).hexdigest()


def main():
    lock=(OUT/'court-capture.lock').open('w');fcntl.flock(lock,fcntl.LOCK_EX|fcntl.LOCK_NB)
    rom=(BASE/'basketball.nes').read_bytes();assert hashlib.sha256(rom).hexdigest()==SOURCE_SHA
    manifest={'schema':1,'rom_sha256':SOURCE_SHA,'accepted_for_release':False,'license':{'source':'user-supplied original ROM','redistribution_permission':'not established'},'effects':[]}
    playback_path=ROOT/'games/basketball/assets/audio-playback.json';playback=json.loads(playback_path.read_text());playback.setdefault('effects',{})
    for ident,source in SOURCES.items():
        nsf=OUT/f'sound-{ident:03}.nsf';payload=carrier(rom,ident)
        if nsf.exists():assert nsf.read_bytes()==payload
        else:nsf.write_bytes(payload)
        captures=[]
        for n in (1,2):
            folder=OUT/f'additional-sound-{ident:03}-run-{n}';info=folder/'capture-info.json'
            if info.exists():
                existing=json.loads(info.read_text());assert existing['id']==ident and existing['nsf_sha256']==sha(nsf)
                assert all(sha(OUT/path)==existing['sha256'][key] for key,path in existing['files'].items());captured=existing['files']
            else:
                captured=record(nsf,folder,OUT/'wave_bridge.dylib',600,255)
                info.write_text(json.dumps({'id':ident,'frames':600,'volume':255,'nsf_sha256':sha(nsf),'files':captured,'sha256':{key:sha(OUT/path) for key,path in captured.items()}},indent=2)+'\n')
            captures.append(captured)
        with wave.open(str(OUT/captures[0]['wav'])) as f:values=array.array('h',f.readframes(f.getnframes()))
        assert max(abs(v) for v in values[-4410:])<8 and max(abs(v) for v in values)<32767
        active=[i for i,v in enumerate(values) if abs(v)>=8];start=max(0,active[0]-882);end=min(len(values),active[-1]+4411)
        output=OUT/f'sound-{ident:03}.wav'
        with wave.open(str(output),'wb') as f:f.setparams((1,2,44100,0,'NONE','not compressed'));f.writeframes(values[start:end].tobytes())
        entry={'id':ident,'wav':output.name,'sha256':sha(output),'sample_range':[start,end],'seconds':(end-start)/44100,'captures':captures,
            'capture_sha256':{key:sha(OUT/path) for key,path in captures[0].items()},'source':source,'accepted_for_release':False,'listening_accepted':False}
        manifest['effects'].append(entry);playback['effects'][str(ident)]={'wav':output.name,'sha256':entry['sha256'],'source_manifest':'additional-effects-manifest.json','listening_accepted':False}
        print('exported',ident,entry['seconds'],flush=True)
    (OUT/'additional-effects-manifest.json').write_text(json.dumps(manifest,indent=2)+'\n')
    playback_path.write_text(json.dumps(playback,indent=2)+'\n')


if __name__=='__main__':main()
