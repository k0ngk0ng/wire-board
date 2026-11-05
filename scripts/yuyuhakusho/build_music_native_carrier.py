"""Verbatim original PCM -> lossless FLAC/fMP4 fragments, research only."""
from pathlib import Path
import argparse,hashlib,json,shutil,struct,subprocess,wave
from reference import ROOT,sha


def boxes(raw,start=0,end=None):
    end=len(raw)if end is None else end;at=start
    while at<end:
        if at+8>end:raise ValueError('truncated MP4 box')
        size,kind=struct.unpack_from('>I4s',raw,at);header=8
        if size==1:
            if at+16>end:raise ValueError('truncated64bit MP4 box')
            size=struct.unpack_from('>Q',raw,at+8)[0];header=16
        elif size==0:size=end-at
        if size<header or at+size>end:raise ValueError('invalid MP4 box bounds')
        yield kind.decode(),at,size,header
        at+=size
    assert at==end


def main(out):
    catalog_path=ROOT/'games/yuyuhakusho/data/audio-assets.json';catalog=json.loads(catalog_path.read_text())
    source=next(row for row in catalog['diagnostics']if row['code']==0x82)
    wav_path=ROOT/source['media_file'];assert sha(wav_path.read_bytes())==source['wav_sha256']
    frames=12*44100
    with wave.open(str(wav_path),'rb')as reader:
        assert (reader.getnchannels(),reader.getsampwidth(),reader.getframerate())==(2,2,44100)
        pcm=reader.readframes(frames);assert len(pcm)==frames*4
    excerpt=out/'source.wav'
    with wave.open(str(excerpt),'wb')as writer:
        writer.setnchannels(2);writer.setsampwidth(2);writer.setframerate(44100);writer.writeframes(pcm)
    ffmpeg=shutil.which('ffmpeg');ffprobe=shutil.which('ffprobe');assert ffmpeg and ffprobe
    encoded=out/'carrier.mp4';decoded=out/'roundtrip.pcm'
    commands=[[ffmpeg,'-nostdin','-v','error','-i',str(excerpt),'-map','0:a:0','-c:a','flac','-sample_fmt','s16','-strict','-2','-movflags','empty_moov+frag_keyframe+default_base_moof+skip_trailer','-frag_duration','1000000','-f','mp4',str(encoded)],
              [ffmpeg,'-nostdin','-v','error','-i',str(encoded),'-map','0:a:0','-c:a','pcm_s16le','-f','s16le',str(decoded)]]
    for command in commands:subprocess.run(command,cwd=ROOT,check=True,capture_output=True)
    assert decoded.read_bytes()==pcm,'Lossless-container roundtrip changed original PCM'
    metadata=json.loads(subprocess.check_output([ffprobe,'-v','error','-select_streams','a:0','-show_entries','stream=codec_name,codec_tag_string,sample_rate,channels,bits_per_raw_sample,time_base:packet=pts,dts,duration,pos','-of','json',str(encoded)],cwd=ROOT,text=True))
    stream=metadata['streams'][0];assert stream['codec_name']=='flac'and stream['codec_tag_string']=='fLaC'and stream['sample_rate']=='44100'and stream['channels']==2 and stream['time_base']=='1/44100'
    raw=encoded.read_bytes();top=list(boxes(raw));first=next(at for kind,at,size,header in top if kind=='moof');init=raw[:first];(out/'init.mp4').write_bytes(init)
    fragments=[];cursor=0
    for index,(kind,at,size,header)in enumerate(top):
        if kind!='moof':continue
        assert index+1<len(top)and top[index+1][0]=='mdat'
        mdat=top[index+1];end=mdat[1]+mdat[2];piece=raw[at:end]
        packets=[packet for packet in metadata['packets']if at<=int(packet['pos'])<end]
        assert packets and all(int(packet['pts'])==int(packet['dts'])for packet in packets)
        decode_start=None
        for typ,off,n,head in boxes(raw,at+header,at+size):
            if typ!='traf':continue
            for child,p,m,h in boxes(raw,off+head,off+n):
                if child=='tfdt':
                    version=raw[p+h];decode_start=int.from_bytes(raw[p+h+4:p+h+(12 if version else 8)],'big')
        assert decode_start==int(packets[0]['pts'])==cursor
        count=0
        for packet in packets:
            assert int(packet['pts'])==cursor+count
            count+=int(packet['duration'])
        name=f'fragment-{len(fragments):03d}.m4s';(out/name).write_bytes(piece)
        fragments.append({'index':len(fragments),'file':name,'sha256':sha(piece),'start_sample':cursor,'sample_frames':count,'packets':len(packets),'bytes':len(piece)})
        cursor+=count
    assert len(fragments)>=4 and cursor==frames
    joined=out/'reconstructed.mp4';joined.write_bytes(init+b''.join((out/row['file']).read_bytes()for row in fragments))
    reconstructed=out/'reconstructed.pcm'
    command=[ffmpeg,'-nostdin','-v','error','-i',str(joined),'-map','0:a:0','-c:a','pcm_s16le','-f','s16le',str(reconstructed)];subprocess.run(command,cwd=ROOT,check=True,capture_output=True);commands.append(command)
    assert reconstructed.read_bytes()==pcm
    version=subprocess.check_output([ffmpeg,'-version'],text=True).splitlines()[0]
    result={'schema':1,'scope':'verbatim-original-PCM-native-carrier','mime':'audio/mp4; codecs="fLaC"',
            'sample_rate':44100,'channels':2,'sample_frames':frames,'stereo_frames':frames,
            'source_code':130,'source_file':str(wav_path.relative_to(ROOT)),'source_wav_sha256':source['wav_sha256'],
            'source_catalog_sha256':sha(catalog_path.read_bytes()),'source_excerpt_pcm_sha256':sha(pcm),
            'decoder_pcm_sha256':sha(decoded.read_bytes()),'reconstructed_pcm_sha256':sha(reconstructed.read_bytes()),
            'roundtrip_pcm_equal':True,'fragment_reconstruction_pcm_equal':True,'init_sha256':sha(init),'fragments':fragments,
            'ffmpeg_version':version,'commands':commands,'published':False,'loop_asset':False,
            'source_sha256':{str(Path(__file__).relative_to(ROOT)):sha(Path(__file__).read_bytes())},
            'limits':['Finite12second carrier fixture from an existing original excerpt, not a BGM loop','Offline ffmpeg research encoder; no independent DSP/Go encoder is claimed']}
    (out/'manifest.json').write_text(json.dumps(result,indent=2)+'\n');print(json.dumps({key:value for key,value in result.items()if key not in ('commands','fragments')},indent=2))

if __name__=='__main__':
    parser=argparse.ArgumentParser(description=__doc__);parser.add_argument('--out',type=Path,required=True);args=parser.parse_args();out=args.out.resolve();assert out.is_relative_to(ROOT/'.local');out.mkdir(parents=True,exist_ok=True)
    assert not (out/'manifest.json').exists(),'Use a new research output directory; accepted fixture is immutable'
    main(out)
