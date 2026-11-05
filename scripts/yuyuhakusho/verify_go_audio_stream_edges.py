"""Independent decoder edge/corruption acceptance for pure-Go stream output."""
from pathlib import Path
import argparse,hashlib,json,shutil,subprocess
from verify_go_audio_stream import ROOT,minimal_env,decode_check,sha


def boxes(raw,start=0,end=None):
    end=len(raw)if end is None else end;at=start
    while at<end:
        assert at+8<=end
        size=int.from_bytes(raw[at:at+4],'big');assert 8<=size<=end-at
        yield raw[at+4:at+8].decode(),at,size
        at+=size
    assert at==end


def run(out,base):
    go,env=minimal_env();ffmpeg=shutil.which('ffmpeg');ffprobe=shutil.which('ffprobe');assert ffmpeg and ffprobe
    source=base/'input/integer.pcm';assert source.exists()
    results=[]
    for environment in ('native','wasm'):
        command=[str(base/'build'/('encoder.wasm'if environment=='wasm'else'encoder'))]
        if environment=='wasm':
            goroot=subprocess.check_output([go,'env','GOROOT'],cwd=ROOT,env=env,text=True).strip();command.insert(0,str(Path(goroot)/'lib/wasm/go_js_wasm_exec'))
        for label,block,fragment in [('minimum',16,64),('maximum',65535,65535)]:
            directory=out/environment/label;directory.mkdir(parents=True,exist_ok=True)
            actual=subprocess.run(command+['-pcm',str(source),'-output',str(directory),'-block',str(block),'-fragment',str(fragment)],cwd=ROOT,env=env,text=True,capture_output=True)
            assert actual.returncode==0,actual.stderr
            manifest=json.loads((directory/'manifest.json').read_text())
            decoded=decode_check(directory,manifest,source,ffmpeg,ffprobe)
            if environment=='wasm':
                prior=json.loads((out/'native'/label/'manifest.json').read_text())
                assert manifest['init_sha256']==prior['init_sha256']and [p['sha256']for p in manifest['fragments']]==[p['sha256']for p in prior['fragments']]
            results.append({'environment':environment,'label':label,'manifest':str((directory/'manifest.json').relative_to(ROOT)),'manifest_sha256':sha((directory/'manifest.json').read_bytes()),**decoded})
    original=(base/'native/original12/default-bulk/joined.mp4').read_bytes()
    top=list(boxes(original));mdat=next((at,size)for typ,at,size in top if typ=='mdat');moof=next((at,size)for typ,at,size in top if typ=='moof')
    traf=next((at,size)for typ,at,size in boxes(original,moof[0]+8,moof[0]+moof[1])if typ=='traf')
    children=list(boxes(original,traf[0]+8,traf[0]+traf[1]));trun=next(at for typ,at,size in children if typ=='trun');tfdt=next(at for typ,at,size in children if typ=='tfdt')
    first_packet=int.from_bytes(original[trun+24:trun+28],'big')
    # TRUN fields: box8+flags4+count4+offset4+duration4+size4.
    assert first_packet>16
    corrupt=[]
    for mode in ('crc8','crc16','decode-time','duration'):
        raw=bytearray(original)
        if mode=='crc8':raw[mdat[0]+8+7]^=1
        elif mode=='crc16':raw[mdat[0]+8+first_packet-1]^=1
        elif mode=='decode-time':
            assert raw[tfdt+8]==1;at=tfdt+12;value=int.from_bytes(raw[at:at+8],'big');raw[at:at+8]=(value+1).to_bytes(8,'big')
        else:
            at=trun+20;value=int.from_bytes(raw[at:at+4],'big');raw[at:at+4]=(value+1).to_bytes(4,'big')
        path=out/(mode+'.mp4');path.write_bytes(raw)
        if mode.startswith('crc'):
            cmd=[ffmpeg,'-nostdin','-v','error','-xerror','-err_detect','crccheck+explode','-i',str(path),'-map','0:a:0','-f','s16le','-y',str(out/(mode+'.pcm'))]
            value=subprocess.run(cmd,cwd=ROOT,text=True,capture_output=True)
            assert value.returncode!=0 and ('crc'in value.stderr.lower()),(mode,value.returncode,value.stderr)
            log=out/(mode+'-rejection.txt');log.write_text(value.stdout+value.stderr)
            corrupt.append({'mode':mode,'rejected':True,'exit_code':value.returncode,'log':str(log.relative_to(ROOT)),'sha256':sha(log.read_bytes())})
        else:
            data=json.loads(subprocess.check_output([ffprobe,'-v','error','-select_streams','a:0','-show_entries','packet=pts,dts,duration','-of','json',str(path)],cwd=ROOT,text=True))
            cursor=0;bad=[]
            for index,packet in enumerate(data['packets']):
                pts=int(packet['pts']);duration=int(packet['duration'])
                if pts!=cursor:bad.append({'packet':index,'expected':cursor,'actual':pts})
                cursor=pts+duration
            if cursor!=529200:bad.append({'final_expected':529200,'final_actual':cursor})
            assert bad,(mode,'corrupted sample timeline accepted')
            corrupt.append({'mode':mode,'rejected':True,'timeline_mismatches':bad[:4]})
    report={'pass':True,'scope':'Minimum/maximum legal blocks and independent CRC/sample-timeline corruption rejection','roundtrip_cases':results,'negative_checks':corrupt,'source_sha256':sha(Path(__file__).read_bytes()),'no_runtime_external_encoder':True}
    (out/'validation.json').write_text(json.dumps(report,indent=2)+'\n');print(json.dumps({'pass':True,'edge_cases':len(results),'negative_checks':len(corrupt)},indent=2))

if __name__=='__main__':
    p=argparse.ArgumentParser(description=__doc__);p.add_argument('--out',type=Path,required=True);p.add_argument('--base',type=Path,required=True);a=p.parse_args();out=a.out.resolve();base=a.base.resolve();assert out.is_relative_to(ROOT/'.local')and base.is_relative_to(ROOT/'.local');out.mkdir(parents=True,exist_ok=True);run(out,base)
