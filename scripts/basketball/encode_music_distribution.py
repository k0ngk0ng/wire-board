"""Compress original-derived music masters for native web media, without reripping.

Completed source/settings-bound encodes are reused. Original masters are read
only. FFmpeg decodes every selected file while measuring quality and counting
all output samples, then a second encoding verifies byte reproducibility.
"""
from pathlib import Path
import array,csv,fcntl,hashlib,json,math,os,re,subprocess,wave

ROOT=Path(__file__).resolve().parents[2];OUT=ROOT/'.local/basketball/audio/export'
WORK=ROOT/'.local/basketball/audio/compression'
IDS=[60,66,75,64,61,65,59,63,62]


def sha(path):
    h=hashlib.sha256()
    with path.open('rb') as f:
        while chunk:=f.read(1024*1024):h.update(chunk)
    return h.hexdigest()


def save(path,data):
    p=path.with_name(path.name+'.tmp');p.write_text(json.dumps(data,indent=2)+'\n');p.replace(path)


def command(source,target,rate):
    return ['ffmpeg','-hide_banner','-loglevel','error','-y','-i',str(source),'-map_metadata','-1','-vn','-ac','1','-ar','44100',
        '-c:a','libmp3lame','-b:a',f'{rate}k','-write_xing','1','-id3v2_version','3','-f','mp3',str(target)]


def inspect(source,encoded,loop):
    probe=json.loads(subprocess.check_output(['ffprobe','-v','error','-show_entries','format=duration,size:stream=codec_name,sample_rate,channels','-of','json',str(encoded)],text=True))
    stream=probe['streams'][0];assert stream['codec_name']=='mp3' and stream['sample_rate']=='44100' and stream['channels']==1
    with wave.open(str(source)) as f:
        assert (f.getnchannels(),f.getsampwidth(),f.getframerate())==(1,2,44100)
        count=f.getnframes();source_head=array.array('h',f.readframes(32768));f.setpos(max(0,count-32768));source_tail=array.array('h',f.readframes(32768))
    log=WORK/(encoded.name+'.decode.log')
    graph='[0:a]asetpts=PTS-STARTPTS[ref];[1:a]asplit=2[dist][pcm0];[dist]asetpts=PTS-STARTPTS[aligned];[ref][aligned]asisdr[scored];[pcm0]astats=metadata=0:reset=0[pcm]'
    args=['ffmpeg','-hide_banner','-loglevel','info','-i',str(source),'-i',str(encoded),'-filter_complex',graph,
        '-map','[pcm]','-c:a','pcm_s16le','-f','s16le','pipe:1','-map','[scored]','-f','null',os.devnull]
    total=0;head=bytearray();tail=bytearray();digest=hashlib.sha256()
    with log.open('w') as stderr:
        p=subprocess.Popen(args,stdout=subprocess.PIPE,stderr=stderr)
        try:
            while chunk:=p.stdout.read(131072):
                total+=len(chunk);digest.update(chunk)
                if len(head)<65536:head.extend(chunk[:65536-len(head)])
                tail.extend(chunk)
                if len(tail)>65536:del tail[:-65536]
            assert p.wait()==0,'decoder failed'
        finally:
            if p.poll() is None:p.kill();p.wait()
    assert total==count*2,'encoder delay/padding changed decoded sample count'
    text=log.read_text();sdr=float(re.findall(r'SI-SDR ch0: ([-.0-9]+) dB',text)[-1]);peak=float(re.findall(r'Peak level dB: ([-.0-9]+)',text)[-1])
    a=array.array('h',head);b=array.array('h',tail)
    adjacent=max(abs(part[i]-part[i-1]) for part in [a[:2048],b[-2048:]] for i in range(1,len(part)))
    jump=abs(a[0]-b[-1]);levels=[]
    for actual,reference in [(a[:1411],source_head[:1411]),(b[-1411:],source_tail[-1411:])]:
        e=sum(x*x for x in reference)/len(reference);v=sum(x*x for x in actual)/len(actual)
        if e>256:levels.append(10*math.log10(v/e))
    return {'sample_rate':44100,'channels':1,'source_samples':count,'decoded_samples':total//2,'decoded_pcm_sha256':digest.hexdigest(),
        'container_duration':float(probe['format']['duration']),'si_sdr_db':sdr,'decoded_peak_db':peak,
        'seam':{'decoded_jump':jump,'adjacent_range_peak':adjacent,'within_decoded_adjacent_range':not loop or jump<=adjacent*1.05+2,
                'edge_rms_difference_db':levels,'loop':loop},'probe':probe,'decode_command':args,'decode_log_sha256':sha(log)}


def main():
    WORK.mkdir(parents=True,exist_ok=True)
    lock=(WORK/'encoding.lock').open('w');fcntl.flock(lock,fcntl.LOCK_EX|fcntl.LOCK_NB)
    playback_path=ROOT/'games/basketball/assets/audio-playback.json';playback=json.loads(playback_path.read_text())
    tool={'version':subprocess.check_output(['ffmpeg','-version'],text=True).splitlines()[0],
        'binary_sha256':sha(Path('/opt/homebrew/bin/ffmpeg')),'encoder':'libmp3lame','format':'audio/mpeg'}
    manifest={'schema':1,'accepted_for_release':False,'listening_accepted':False,'tool':tool,'tracks':[],
        'license':{'source':'user-supplied original ROM; processed original PCM masters','redistribution_permission':'not established'},
        'selection':'MP3 native-media baseline; CDN and local use compressed sources, no large-WAV network fallback'}
    for ident in IDS:
        old=playback['music'][str(ident)];source=OUT/old['wav'];source_hash=sha(source);assert source_hash==old['sha256']
        target=OUT/(source.stem+'.mp3');repeat=OUT/(source.stem+'-repeat.mp3');info=WORK/(source.stem+'.encoding.json')
        existing=json.loads(info.read_text()) if info.exists() else None
        if existing and existing['master_sha256']==source_hash and existing['tool']==tool and target.exists() and repeat.exists() and sha(target)==sha(repeat)==existing['sha256']:
            entry=existing;print('reused',ident,flush=True)
        else:
            entry=None
            for rate in [128,160,192]:
                temp=WORK/(source.stem+f'-{rate}.mp3');args=command(source,temp,rate)
                print('encoding',ident,rate,flush=True);subprocess.run(args,check=True)
                checked=inspect(source,temp,old['loop'])
                acceptable=checked['si_sdr_db']>=24 and checked['decoded_peak_db']<0 and checked['seam']['within_decoded_adjacent_range'] and all(abs(v)<=1.5 for v in checked['seam']['edge_rms_difference_db'])
                save(WORK/(source.stem+f'-{rate}-metrics.json'),checked)
                if not acceptable:print('candidate did not meet fixed criteria',ident,rate,checked['si_sdr_db'],checked['seam'],flush=True);continue
                subprocess.run(command(source,repeat,rate),check=True)
                assert sha(temp)==sha(repeat),'independent MP3 encoding differs'
                temp.replace(target)
                entry=dict(checked,id=ident,master=source.name,master_sha256=source_hash,file=target.name,sha256=sha(target),repeat_file=repeat.name,
                    bitrate_kbps=rate,encoded_bytes=target.stat().st_size,master_bytes=source.stat().st_size,tool=tool,encode_command=args,
                    accepted_for_release=False,listening_accepted=False)
                save(info,entry);break
            if entry is None:raise RuntimeError(f'No verified MP3 rendition for {ident}; existing catalog remains unchanged')
        manifest['tracks'].append(entry)
        save(WORK/'encoding-progress.json',manifest)
        print('verified',ident,entry['bitrate_kbps'],'kbps',entry['encoded_bytes'],'bytes','SI-SDR',entry['si_sdr_db'],flush=True)
    assert sum(r['encoded_bytes'] for r in manifest['tracks'])<sum(r['master_bytes'] for r in manifest['tracks'])*.3
    for entry in manifest['tracks']:
        playback['music'][str(entry['id'])].update(file=entry['file'],mime_type='audio/mpeg',distribution_sha256=entry['sha256'],encoding_manifest='music-distribution-manifest.json')
    save(OUT/'music-distribution-manifest.json',manifest)
    save(playback_path,playback)


if __name__=='__main__':main()
