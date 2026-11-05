"""Native/Wasm pure-Go encoder E2E and independent ffmpeg roundtrip; written first."""
from pathlib import Path
from array import array
import argparse,hashlib,json,os,shutil,subprocess,sys,wave

ROOT=Path(__file__).resolve().parents[2]
PACKAGE='./games/yuyuhakusho/cmd/verify-audio-stream-encoder'

def sha(raw):return hashlib.sha256(raw).hexdigest()

def minimal_env():
    go=shutil.which('go');node=shutil.which('node');assert go and node
    tmp=ROOT/'.local/md-yuyuhakusho/tmp';tmp.mkdir(parents=True,exist_ok=True)
    env={key:os.environ[key]for key in ('HOME','LANG','LC_ALL','LC_CTYPE','TZ')if key in os.environ}
    env.update(PATH=str(Path(go).parent)+':'+str(Path(node).parent)+':/usr/bin:/bin',
               GOCACHE=str(ROOT/'.local/go-cache'),GOMODCACHE=str(ROOT/'.local/go-mod'),GOTMPDIR=str(tmp),TMPDIR=str(tmp),
               TEST_TELEMETRY_DIR=str(ROOT/'.local/go-telemetry'),GOTOOLCHAIN='local',GOENV='off')
    return go,env

def fixture_files(out):
    fixtures=out/'input';fixtures.mkdir(parents=True,exist_ok=True)
    seed=0xdeadbeef;values=array('h')
    for i in range(65537*2):
        seed=(seed*1664525+1013904223)&0xffffffff;value=seed>>16
        values.append(value-65536 if value>=32768 else value)
    values[:6]=array('h',[-32768,32767,0,-1,1,0])
    if sys.byteorder!='little':values.byteswap()
    synthetic=fixtures/'integer.pcm';synthetic.write_bytes(values.tobytes())
    catalog=json.loads((ROOT/'games/yuyuhakusho/data/audio-assets.json').read_text())
    source=next(row for row in catalog['diagnostics']if row['code']==0x82)
    path=ROOT/source['media_file'];assert sha(path.read_bytes())==source['wav_sha256']
    with wave.open(str(path),'rb')as reader:
        assert (reader.getnchannels(),reader.getsampwidth(),reader.getframerate())==(2,2,44100)
        pcm=reader.readframes(12*44100)
    assert len(pcm)==12*44100*4
    original=fixtures/'original12.pcm';original.write_bytes(pcm)
    return {'integer':synthetic,'original12':original}

def decode_check(directory,manifest,source,ffmpeg,ffprobe):
    expected=source.read_bytes();joined=directory/'joined.mp4'
    init=directory/manifest['init_file'];assert sha(init.read_bytes())==manifest['init_sha256']
    cursor=0;wire=bytearray(init.read_bytes())
    for index,fragment in enumerate(manifest['fragments']):
        raw=(directory/fragment['file']).read_bytes()
        assert fragment['index']==index and fragment['sequence']==index+1 and fragment['start_sample']==cursor
        assert sha(raw)==fragment['sha256']and len(raw)==fragment['bytes']
        cursor+=fragment['sample_frames'];wire.extend(raw)
    assert cursor==len(expected)//4==manifest['stereo_frames']
    joined.write_bytes(wire)
    decoded=directory/'decoded.pcm'
    command=[ffmpeg,'-nostdin','-v','error','-err_detect','crccheck+explode','-i',str(joined),'-map','0:a:0','-c:a','pcm_s16le','-f','s16le','-y',str(decoded)]
    result=subprocess.run(command,cwd=ROOT,text=True,capture_output=True)
    assert result.returncode==0,(command,result.stderr)
    assert decoded.read_bytes()==expected,('decoded PCM differs',directory)
    stream=json.loads(subprocess.check_output([ffprobe,'-v','error','-select_streams','a:0','-show_entries','stream=codec_name,codec_tag_string,sample_rate,channels,time_base:packet=pts,dts,duration','-of','json',str(joined)],cwd=ROOT,text=True))
    fmt=stream['streams'][0]
    assert fmt['codec_name']=='flac'and fmt['codec_tag_string']=='fLaC'and fmt['sample_rate']=='44100'and fmt['channels']==2 and fmt['time_base']=='1/44100'
    cursor=0
    for packet in stream['packets']:
        assert int(packet['pts'])==int(packet['dts'])==cursor
        assert int(packet['duration'])>0
        cursor+=int(packet['duration'])
    assert cursor==manifest['stereo_frames']
    return {'pass':True,'joined_sha256':sha(wire),'decoded_pcm_sha256':sha(expected),'packet_count':len(stream['packets']),'samples':cursor,'decode_command':command}

def run(out,environment):
    go,env=minimal_env();fixtures=fixture_files(out)
    ffmpeg=shutil.which('ffmpeg');ffprobe=shutil.which('ffprobe');assert ffmpeg and ffprobe,'ffmpeg/ffprobe are acceptance tools, never codec runtime dependencies'
    build=out/'build';build.mkdir(exist_ok=True)
    binary=build/('encoder.wasm'if environment=='wasm'else'encoder')
    if environment=='wasm':env.update(GOOS='js',GOARCH='wasm')
    command=[go,'build','-o',str(binary),PACKAGE]
    built=subprocess.run(command,cwd=ROOT,env=env,text=True,capture_output=True)
    (out/f'{environment}-build.txt').write_text(built.stdout+built.stderr)
    assert built.returncode==0,(command,built.returncode,built.stderr)
    executable=[str(binary)]
    if environment=='wasm':
        goroot=subprocess.check_output([go,'env','GOROOT'],cwd=ROOT,env=env,text=True).strip()
        executable.insert(0,str(Path(goroot)/'lib/wasm/go_js_wasm_exec'))
    cases=[]
    for name,source in fixtures.items():
        for label,block,fragment in [('default',1024,4096),('small',256,1024),('wide',4096,8192)]:
            baseline=None
            for partition,chunks in [('bulk','65536'),('mixed','1,17,257,997,4096')]:
                directory=out/environment/name/(label+'-'+partition);directory.mkdir(parents=True,exist_ok=True)
                args=executable+['-pcm',str(source),'-output',str(directory),'-block',str(block),'-fragment',str(fragment),'-chunks',chunks]
                result=subprocess.run(args,cwd=ROOT,env=env,text=True,capture_output=True)
                (directory/'command.txt').write_text(json.dumps(args)+'\n'+result.stdout+result.stderr)
                assert result.returncode==0,(args,result.returncode,result.stderr)
                manifest=json.loads((directory/'manifest.json').read_text())
                assert manifest['pass']and manifest['scope']=='pure-go-pcm-flac-mp4'and manifest['encoder_runtime_external_dependencies']==[]
                assert manifest['source_pcm_sha256']==sha(source.read_bytes())and manifest['stereo_frames']==len(source.read_bytes())//4
                assert manifest['sample_rate']==44100 and manifest['channels']==2 and manifest['bits_per_sample']==16 and manifest['closed']
                checked=decode_check(directory,manifest,source,ffmpeg,ffprobe)
                signature=[manifest['init_sha256']]+[piece['sha256']for piece in manifest['fragments']]
                if baseline is None:baseline=signature
                else:assert signature==baseline,(name,label,'Append partition changed output bytes')
                if environment=='wasm':
                    native=json.loads((out/'native'/name/(label+'-'+partition)/'manifest.json').read_text())
                    assert signature==[native['init_sha256']]+[piece['sha256']for piece in native['fragments']],(name,label,partition,'native/Wasm bytes differ')
                    other=manifest.copy();prior=native.copy();other.pop('environment');prior.pop('environment');assert other==prior,'manifest fields differ across native/Wasm'
                cases.append({'name':name,'config':label,'partition':partition,'directory':str(directory.relative_to(ROOT)),'manifest_sha256':sha((directory/'manifest.json').read_bytes()),**checked})
                print(environment,name,label,partition,manifest['stereo_frames'],'samples pass',flush=True)
    report={'pass':True,'environment':environment,'cases':cases,'case_count':len(cases),
            'native_wasm_bytes_equal':environment=='wasm','codec_runtime_external_dependencies':[],
            'acceptance_decoder':subprocess.check_output([ffmpeg,'-version'],text=True).splitlines()[0],
            'source_sha256':sha(Path(__file__).read_bytes()),'limits':['Encoding/transport only; no independentDSP, infiniteBGM, dynamicmix latency or platform integration claim']}
    (out/f'{environment}-validation.json').write_text(json.dumps(report,indent=2)+'\n')
    return report

if __name__=='__main__':
    p=argparse.ArgumentParser(description=__doc__);p.add_argument('--out',type=Path,required=True);p.add_argument('environment',choices=['native','wasm','both']);a=p.parse_args();out=a.out.resolve();assert out.is_relative_to(ROOT/'.local');out.mkdir(parents=True,exist_ok=True)
    try:
        for environment in ('native','wasm')if a.environment=='both'else(a.environment,):run(out,environment)
    except Exception as error:
        (out/'failure.json').write_text(json.dumps({'error':repr(error),'producer_sha256':sha(Path(__file__).read_bytes())},indent=2)+'\n');raise
