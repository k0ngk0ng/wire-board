"""Keep finite75 and original intro/loop76 separate; never loop the intro."""
from pathlib import Path
import array
import csv
import hashlib
import json
import sys
import wave

ROOT=Path(__file__).resolve().parents[2]
OUT=ROOT/'.local/basketball/audio/export'


def sha(path):return hashlib.sha256(path.read_bytes()).hexdigest()


def main():
    tracks=[]
    for ident in (75,76):
        captures=[json.loads((OUT/f'court-music-{ident:03}-20000-run-{n}/capture-info.json').read_text())['files'] for n in (1,2)]
        for kind in captures[0]:assert sha(OUT/captures[0][kind])==sha(OUT/captures[1][kind])
        driver=(OUT/captures[0]['driver']).read_bytes();active=False;end_step=None
        for i in range(20000):
            now=any(driver[i*231+0x15:i*231+0x19])
            if now:active=True
            elif active:end_step=i+1;break
        with wave.open(str(OUT/captures[0]['wav'])) as f:
            assert (f.getnchannels(),f.getsampwidth(),f.getframerate())==(1,2,44100)
            samples=array.array('h',f.readframes(f.getnframes()))
        if sys.byteorder!='little':samples.byteswap()
        start,end=0,len(samples)
        if ident==75:
            assert end_step==8470
            final=max(i for i,v in enumerate(samples) if abs(v)>=8)
            end=min(end,final+4411)
        else:assert end_step is None
        target=OUT/f'music-{ident:03}.wav';pcm=samples[start:end]
        if sys.byteorder!='little':pcm.byteswap()
        with wave.open(str(target),'wb') as f:
            f.setparams((1,2,44100,0,'NONE','not compressed'));f.writeframes(pcm.tobytes())
        jumps=[{k:int(v) for k,v in r.items()} for r in csv.DictReader((OUT/captures[0]['jumps']).open()) if r['channel']=='0']
        track={'id':ident,'wav':target.name,'sha256':sha(target),'sample_range':[start,end],'seconds':(end-start)/44100,
            'captures':captures,'capture_sha256':{kind:sha(OUT/path) for kind,path in captures[0].items()},
            'driver_end_step':end_step,'main_sequence_jumps':jumps,'loop':False,'preview_only':ident==76,
            'accepted_for_release':False,'listening_accepted':False,'render_volume':192,
            'source_rom_sha256':sha(ROOT/'.local/nes-basket/basketball.nes'),
            'processing':'source PCM prefix only; no waveform processing',
            'limitation':None if ident==75 else 'This is a333-second original timeline with a one-time tempo129 intro and tempo140 loop section; it is not an indefinite or intro-free loop and is not registered for runtime playback.'}
        tracks.append(track)
    manifest={'schema':1,'accepted_for_release':False,'license':{'source':'user-supplied original ROM','redistribution_permission':'not established'},'tracks':tracks}
    (OUT/'ending-music-manifest.json').write_text(json.dumps(manifest,indent=2)+'\n')
    path=ROOT/'games/basketball/assets/audio-playback.json';playback=json.loads(path.read_text());entry=tracks[0]
    playback['music']['75']={'wav':entry['wav'],'loop':False,'sha256':entry['sha256'],'source_manifest':'ending-music-manifest.json','processing':entry['processing'],'listening_accepted':False}
    path.write_text(json.dumps(playback,indent=2)+'\n')
    source=ROOT/'games/basketball/assets/court-music-source.json';ledger=json.loads(source.read_text())
    ledger['ending_music']={'scene':7,'dispatcher':'fixed:$CAE3 → $FC83 → bank3:$8015/$B995',
        'rea_evidence_id':'ev_33bb6f8ee44e775ab1856de49120b96ebace7f235894fef4fb2cd6684a2b56de',
        'tracks':[{'id':75,'request':'bank3:$BA6C','entry':'phase0 continuation $BA5A','mode':'finite','seconds':tracks[0]['seconds'],'runtime_mapping':True},
                  {'id':76,'request':'bank3:$BD68','entry':'phase8 $BD27','mode':'intro then loop','timeline_preview_seconds':tracks[1]['seconds'],'runtime_mapping':bool(playback['music'].get('76',{}).get('file'))}],
        'background_load_rea':'ev_f44a23d36985468ba8d23ffae716d6d847557e0a2960f8f3725844d06f3122ad',
        'background_load_limitation':'Ghidra stops at the interrupt wait; fixed DCB7/DCB9 resume255 confirmed by instructions and natural result captures.'}
    ledger.pop('separate_music_outside_scope',None)
    source.write_text(json.dumps(ledger,indent=2,ensure_ascii=False)+'\n')
    print(json.dumps({'finite75_seconds':tracks[0]['seconds'],'preview76_seconds':tracks[1]['seconds']}))


if __name__=='__main__':main()
