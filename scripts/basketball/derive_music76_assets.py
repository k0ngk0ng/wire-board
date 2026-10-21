"""Create source-bound music76 compound WAV and verified MP3; no catalog writes."""
from pathlib import Path
import array, hashlib, json, math, os, re, shutil, struct, subprocess, sys, wave
from capture_music76_assets import ASSETS, ROOT, OLD, NAMES, sha, save
from verify_music76_sequence_anchors import RATE, BLEND, read_pcm, blend_tail, check_tail
from encode_music_distribution import command

def write_pcm(path,pcm):
    data=array.array('h',pcm)
    if sys.byteorder!='little':data.byteswap()
    with wave.open(str(path),'wb') as f:
        f.setparams((1,2,RATE,0,'NONE','not compressed'));f.writeframes(data.tobytes())

def mp3_metadata(path):
    data=path.read_bytes()[:4096]
    offset=0
    if data[:3]==b'ID3':
        offset=10+sum((data[6+i]&127)<<(7*(3-i)) for i in range(4))
    header=int.from_bytes(data[offset:offset+4],'big')
    assert header>>21==0x7ff and ((header>>19)&3)==3 and ((header>>17)&3)==1
    assert ((header>>10)&3)==0 and ((header>>6)&3)==3,'expected MPEG1 layer3 44100 mono'
    crc=0 if (header>>16)&1 else 2
    info=offset+4+crc+17;assert data[info:info+4] in [b'Info',b'Xing']
    flags=int.from_bytes(data[info+4:info+8],'big');cursor=info+8
    assert flags&1
    frames=int.from_bytes(data[cursor:cursor+4],'big');cursor+=4
    if flags&2:cursor+=4
    if flags&4:cursor+=100
    if flags&8:cursor+=4
    encoder=data[cursor:cursor+9].decode('ascii')
    packed=int.from_bytes(data[cursor+21:cursor+24],'big');delay=packed>>12;padding=packed&4095
    assert encoder.startswith(('LAME','Lavc')) and delay>0 and padding>0
    return {'marker':data[info:info+4].decode(),'mpeg_audio_frames':frames,'encoder':encoder,
            'encoder_delay_samples':delay,'encoder_padding_samples':padding,
            'metadata_decoded_samples':frames*1152-delay-padding,'metadata_offset':info}

def inspect(source,encoded,start):
    probe_cmd=['ffprobe','-v','error','-show_entries','format=duration,size:stream=codec_name,sample_rate,channels,duration,start_time','-of','json',str(encoded)]
    probe=json.loads(subprocess.check_output(probe_cmd,text=True));stream=probe['streams'][0]
    assert stream['codec_name']=='mp3' and stream['sample_rate']=='44100' and stream['channels']==1
    with wave.open(str(source)) as f:
        count=f.getnframes();source_head=array.array('h',f.readframes(32768))
        f.setpos(start);source_loop=array.array('h',f.readframes(32768))
        f.setpos(count-32768);source_tail=array.array('h',f.readframes(32768))
    log=encoded.with_suffix('.decode.log')
    graph='[0:a]asetpts=PTS-STARTPTS[ref];[1:a]asplit=2[dist][pcm0];[dist]asetpts=PTS-STARTPTS[aligned];[ref][aligned]asisdr[scored];[pcm0]astats=metadata=0:reset=0[pcm]'
    args=['ffmpeg','-hide_banner','-loglevel','info','-i',str(source),'-i',str(encoded),'-filter_complex',graph,
          '-map','[pcm]','-c:a','pcm_s16le','-f','s16le','pipe:1','-map','[scored]','-f','null',os.devnull]
    total=0;head=bytearray();loop=bytearray();tail=bytearray();digest=hashlib.sha256()
    with log.open('w') as stderr:
        p=subprocess.Popen(args,stdout=subprocess.PIPE,stderr=stderr)
        try:
            while chunk:=p.stdout.read(131072):
                oldtotal=total;total+=len(chunk);digest.update(chunk)
                if len(head)<65536:head.extend(chunk[:65536-len(head)])
                lo=max(oldtotal,start*2);hi=min(total,start*2+65536)
                if hi>lo:loop.extend(chunk[lo-oldtotal:hi-oldtotal])
                tail.extend(chunk)
                if len(tail)>65536:del tail[:-65536]
            assert p.wait()==0,'decoder failed'
        finally:
            if p.poll() is None:p.kill();p.wait()
    assert total==count*2 and len(loop)==65536,'decoded count or loop-start coverage mismatch'
    text=log.read_text();sdr=float(re.findall(r'SI-SDR ch0: ([-.0-9]+) dB',text)[-1])
    peak=float(re.findall(r'Peak level dB: ([-.0-9]+)',text)[-1])
    a,b,c=array.array('h',head),array.array('h',loop),array.array('h',tail)
    if sys.byteorder!='little':
        for item in (a,b,c,source_head,source_loop,source_tail):item.byteswap()
    adjacent=max(abs(part[i]-part[i-1]) for part in [b[:2048],c[-2048:]] for i in range(1,len(part)))
    jump=abs(b[0]-c[-1]);levels=[]
    for location,actual,reference in [('prefix_start',a[:BLEND],source_head[:BLEND]),
                                      ('loop_start',b[:BLEND],source_loop[:BLEND]),
                                      ('eof',c[-BLEND:],source_tail[-BLEND:])]:
        e=sum(x*x for x in reference)/len(reference);v=sum(x*x for x in actual)/len(actual)
        levels.append({'location':location,'reference_energy':e,'decoded_energy':v,
                       'difference_db':10*math.log10(v/e) if e>256 and v else None,
                       'criterion_applies':e>256})
    metadata=mp3_metadata(encoded);assert metadata['metadata_decoded_samples']==count
    checks={'exact_decoded_samples':total//2==count,'xing_lame_exact_samples':metadata['metadata_decoded_samples']==count,
            'si_sdr_ge_24_db':sdr>=24,'no_clipping':peak<0,
            'decoded_eof_to_loop_start_seam':jump<=adjacent*1.05+2,
            'edge_rms_within_1_5_db':all(not r['criterion_applies'] or r['difference_db'] is not None and abs(r['difference_db'])<=1.5 for r in levels)}
    return {'passed':all(checks.values()),'checks':checks,'sample_rate':RATE,'channels':1,
            'source_samples':count,'decoded_samples':total//2,'decoded_pcm_sha256':digest.hexdigest(),
            'si_sdr_db':sdr,'decoded_peak_db':peak,'seam':{'from_sample':count-1,'to_sample':start,
            'decoded_jump':jump,'adjacent_range_peak':adjacent,'limit':adjacent*1.05+2,
            'edge_rms':levels,'comparison':'decoded EOF to decoded loop_start_sample, never EOF to sample zero'},
            'mp3_metadata':metadata,'container_duration':float(probe['format']['duration']),
            'probe':probe,'probe_command':probe_cmd,'decode_command':args,'decode_log':log.name,'decode_log_sha256':sha(log)}

def main():
    anchors=json.loads((ASSETS/'sequence-anchor-verification.json').read_text())
    assert anchors['status']=='passed';chosen=anchors['selected'];assert chosen['source_frames']==[7449,44313]
    rejected=ASSETS/'music76-asset-manifest.json'
    assert sha(rejected)==anchors['rejected4304_manifest_sha256']=='3a1f0ff79cb17d675dd4eb8593b95e40968eace0ef504ec0059289b1e968b524'
    capture=json.loads((ASSETS/'capture-verification.json').read_text());start,end=chosen['loop_start_sample'],chosen['loop_end_sample']
    runs=[ASSETS/f'capture-45000-run-{n}' for n in (1,2)];outputs=[];derivation_checks=[]
    for i,run in enumerate(runs,1):
        assert all(sha(run/k)==capture['files'][k] for k in NAMES)
        original=read_pcm(run/'full.wav');tail=blend_tail(original,start,end)
        checks=check_tail(original,start,end,tail);assert checks['passed'],'anchor failed; stop before derivative'
        result=original[:end];result[-BLEND:]=tail
        assert len(result)==end and result[:end-BLEND]==original[:end-BLEND]
        assert result[:start]==original[:start],'original prefix changed'
        target=ASSETS/f'music-076-intro-loop-derived-{i}.wav';write_pcm(target,result)
        actual=read_pcm(target);assert actual==result and len(actual)==end
        outputs.append(target);derivation_checks.append(checks)
        print('derived',i,'samples',len(actual),'sha256',sha(target),flush=True)
    assert sha(outputs[0])==sha(outputs[1]);master=ASSETS/'music-076-intro-loop.wav'
    shutil.copyfile(outputs[0],master)
    tool={'version':subprocess.check_output(['ffmpeg','-version'],text=True).splitlines()[0],
          'binary_sha256':sha(Path('/opt/homebrew/bin/ffmpeg')),'encoder':'libmp3lame','format':'audio/mpeg'}
    assert tool==json.loads((OLD/'music-distribution-manifest.json').read_text())['tool'],'pinned existing encoder differs'
    candidates=[];accepted=None
    for rate in [128,160,192]:
        candidate=ASSETS/f'music-076-intro-loop-{rate}.mp3';args=command(master,candidate,rate)
        print('encoding',rate,'kbps',flush=True);subprocess.run(args,check=True)
        metrics=inspect(master,candidate,start);save(ASSETS/f'music76-{rate}-metrics.json',metrics)
        candidates.append({'bitrate_kbps':rate,'file':candidate.name,'sha256':sha(candidate),'metrics':metrics,'encode_command':args})
        print('quality',rate,metrics['checks'],'SI-SDR',metrics['si_sdr_db'],flush=True)
        if not metrics['passed']:continue
        repeat=ASSETS/f'music-076-intro-loop-{rate}-repeat.mp3';repeat_args=command(master,repeat,rate)
        subprocess.run(repeat_args,check=True);assert sha(repeat)==sha(candidate),'independent encode mismatch'
        repeated=inspect(master,repeat,start);assert repeated['passed'] and repeated['decoded_pcm_sha256']==metrics['decoded_pcm_sha256']
        final=ASSETS/'music-076-intro-loop.mp3';shutil.copyfile(candidate,final)
        accepted=dict(candidates[-1],file=final.name,repeat_file=repeat.name,repeat_encode_command=repeat_args,repeat_metrics=repeated,
                      encoded_bytes=final.stat().st_size,master_bytes=master.stat().st_size)
        break
    if accepted is None:
        save(ASSETS/'music76-encoding-failure.json',{'status':'no-passing-mp3','candidates':candidates})
        raise RuntimeError('No MP3 met unchanged quality limits; no delivery registration allowed')
    assert accepted['encoded_bytes']<accepted['master_bytes']*.3
    manifest={'schema':1,'status':'offline-asset-verified','id':76,'file':accepted['file'],'mime_type':'audio/mpeg',
              'sha256':accepted['sha256'],'distribution_sha256':accepted['sha256'],
              'master':master.name,'master_sha256':sha(master),'source_sha256':capture['files']['full.wav'],
              'sample_rate':RATE,'loop_start_sample':start,'loop_end_sample':end,'loop_samples':end-start,
              'source_samples':end,'decoded_samples':accepted['metrics']['decoded_samples'],
              'prefix_seconds':start/RATE,'loop_seconds':(end-start)/RATE,'seconds':end/RATE,
              'source_frames':chosen['source_frames'],'period_frames':36864,'musical_loops':35,
              'loop':True,'loopStartSeconds':start/RATE,
              'processing':{'method':'unchanged original prefix and full original period; final1411samples raised-cosine convex blend with corresponding source pre-anchor passage',
                            'blend_samples':BLEND,'blend_seconds':BLEND/RATE,'source_tail_range':[end-BLEND,end],
                            'source_target_range':[start-BLEND,start],'unchanged_range':[0,end-BLEND],
                            'formula':'round((1-w)*tail[i]+w*pre_start[i]), w=(1-cos(pi*i/(1411-1)))/2'},
              'source':capture,'anchor_selection':anchors,'derivation_checks':derivation_checks,
              'independent_masters':[{'file':p.name,'sha256':sha(p)} for p in outputs],
              'tool':tool,'encoding':accepted,'encoding_candidates':candidates,
              'commands':['PYTHONDONTWRITEBYTECODE=1 .local/board-art-env/bin/python scripts/basketball/capture_music76_assets.py',
                          'PYTHONDONTWRITEBYTECODE=1 .local/board-art-env/bin/python scripts/basketball/verify_music76_sequence_anchors.py',
                          'PYTHONDONTWRITEBYTECODE=1 .local/board-art-env/bin/python scripts/basketball/derive_music76_assets.py'],
              'scripts_sha256':{p.name:sha(p) for p in [Path(__file__),ROOT/'scripts/basketball/verify_music76_sequence_anchors.py',ROOT/'scripts/basketball/capture_music76_assets.py']},
              'accepted_for_release':False,'listening_accepted':False,'browser_accepted':False,
              'license':{'source':'user-supplied original ROM','redistribution_permission':'not established'},
              'limitations':['Isolated original sound driver capture; not earned campaign evidence.',
                             'Processed original PCM derivative; final1411samples are not sample-identical source.',
                             'Offline MP3 checks do not establish gapless browser playback.',
                             'Human listening, native browser boundary timing and real-device acceptance remain pending.',
                             'No playback registration, package installation or upload performed by these scripts.']}
    save(ASSETS/'music76-delivery-manifest.json',manifest)
    print(json.dumps({k:manifest[k] for k in ['status','file','sha256','loop_start_sample','loop_end_sample','loop_samples','decoded_samples','seconds']}),flush=True)

if __name__=='__main__':main()
