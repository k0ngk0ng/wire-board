"""Real HTMLAudio/MSE FLAC fragments E2E; written before fixture builder."""
from pathlib import Path
from http.server import BaseHTTPRequestHandler,ThreadingHTTPServer
import argparse,hashlib,json,os,subprocess,threading,time
from reference import ROOT

HTML='''<!doctype html><meta charset="utf-8"><title>Native lossless carrier E2E</title><button id="start">Start native carrier</button><audio id="audio"></audio><pre id="status">Ready</pre><script>
const audio=document.querySelector('#audio'),status=document.querySelector('#status');
const result={scope:'Research native carrier, not independent DSP or complete BGM',events:[],appends:[],clock:[],crossings:[],errors:[],audioContextCreated:false,sourceChanges:0};window.musicNativeResult=result;
function event(kind,extra={}){result.events.push({kind,wall:performance.now(),media:audio.currentTime,...extra})}
for(const name of ['playing','waiting','stalled','ended','error','pause','seeking','seeked'])audio.addEventListener(name,()=>event(name));
const delay=ms=>new Promise(resolve=>setTimeout(resolve,ms));let fragments=[],manifest,mediaSource,buffer,recording=[],recorder,stream,source;
async function append(path,index){const raw=await fetch(path).then(response=>{if(!response.ok)throw Error('segment HTTP failure');return response.arrayBuffer()});await new Promise((resolve,reject)=>{buffer.addEventListener('updateend',resolve,{once:true});buffer.addEventListener('error',()=>reject(Error('SourceBuffer error')),{once:true});buffer.appendBuffer(raw)});result.appends.push({index,media:audio.currentTime,paused:audio.paused,wall:performance.now(),bytes:raw.byteLength,ranges:Array.from({length:buffer.buffered.length},(_,i)=>[buffer.buffered.start(i),buffer.buffered.end(i)])})}
async function saveRecording(){if(recording.length){const blob=new Blob(recording,{type:recorder.mimeType});result.recordingBytes=blob.size;await fetch('/recording',{method:'POST',body:blob})}}
async function run(e){result.trusted=e.isTrusted;manifest=await fetch('/manifest').then(response=>response.json());fragments=manifest.fragments;
if(!MediaSource.isTypeSupported(manifest.mime))throw Error('native MSE codec unsupported');
mediaSource=new MediaSource();audio.src=URL.createObjectURL(mediaSource);result.sourceChanges++;source=audio.src;
await new Promise(resolve=>mediaSource.addEventListener('sourceopen',resolve,{once:true}));buffer=mediaSource.addSourceBuffer(manifest.mime);mediaSource.duration=manifest.stereo_frames/manifest.sample_rate;
await append('/init',-1);await append('/fragment/0',0);await append('/fragment/1',1);
let previous=0,lastWall=0;const loop=()=>{const wall=performance.now(),at=audio.currentTime;result.clock.push({wall,media:at,paused:audio.paused,ready:audio.readyState});for(let i=0;i<fragments.length-1;i++){const boundary=(fragments[i].start_sample+fragments[i].sample_frames)/manifest.sample_rate;if(previous<boundary&&at>=boundary)result.crossings.push({index:i,boundary,before:previous,after:at,wall,wallGap:lastWall?wall-lastWall:0})}previous=at;lastWall=wall;if(!audio.ended)requestAnimationFrame(loop)};
const ended=new Promise(resolve=>audio.addEventListener('ended',resolve,{once:true}));await audio.play();event('play-resolved');requestAnimationFrame(loop);
if(audio.captureStream&&typeof MediaRecorder!=='undefined'&&MediaRecorder.isTypeSupported('audio/webm;codecs=opus')){
 stream=audio.captureStream();result.captureTracks=stream.getAudioTracks().map(track=>({id:track.id,settings:track.getSettings()}));
 stream.addEventListener('inactive',()=>event('capture-inactive'));stream.addEventListener('removetrack',e=>event('capture-removetrack',{id:e.track.id}));
 recorder=new MediaRecorder(stream,{mimeType:'audio/webm;codecs=opus'});recorder.ondataavailable=e=>{if(e.data.size)recording.push(e.data)};recorder.onstop=()=>{event('recorder-stop');result.recordingStopped=true};recorder.start(100);event('recorder-start');
}
for(let index=2;index<fragments.length;index++){await delay(500);if(audio.src!==source)throw Error('unexpected native source replacement');await append('/fragment/'+index,index)}
mediaSource.endOfStream();event('source-eos');await ended;if(recorder&&recorder.state!=='inactive'){recorder.stop();await delay(150)}await saveRecording();result.final={media:audio.currentTime,duration:audio.duration,ended:audio.ended,paused:audio.paused,source:audio.src};result.manifest=manifest;result.done=true;status.textContent='Native carrier complete';window.musicNativeDone=true;
}
document.querySelector('#start').onclick=e=>{e.target.disabled=true;run(e).catch(error=>{result.errors.push(String(error));result.done=true;window.musicNativeDone=true;status.textContent=String(error)})};
</script>'''

def digest(path):return hashlib.sha256(path.read_bytes()).hexdigest()

def main(out,fixture):
    manifest=json.loads((fixture/'manifest.json').read_text())
    assert manifest['scope']=='verbatim-original-PCM-native-carrier'
    assert manifest['roundtrip_pcm_equal']and manifest['sample_rate']==44100 and manifest['channels']==2
    assert manifest['sample_frames']==manifest['stereo_frames']==12*44100 and len(manifest['fragments'])>=4
    assert manifest['decoder_pcm_sha256']==manifest['source_excerpt_pcm_sha256']
    assert digest(fixture/'roundtrip.pcm')==manifest['source_excerpt_pcm_sha256']
    cursor=0
    for fragment in manifest['fragments']:
        assert fragment['start_sample']==cursor and fragment['sample_frames']>0
        assert digest(fixture/fragment['file'])==fragment['sha256']
        cursor+=fragment['sample_frames']
    assert cursor==manifest['stereo_frames']
    assert digest(fixture/'init.mp4')==manifest['init_sha256']
    tmp=out/'tmp';tmp.mkdir(parents=True,exist_ok=True);sock=ROOT/'.local/ym';sock.mkdir(parents=True,exist_ok=True)
    env=dict(os.environ,TMPDIR=str(tmp),AGENT_BROWSER_SOCKET_DIR=str(sock))
    seed=subprocess.check_output(['agent-browser','session','id','--scope','worktree','--prefix','music-native'],cwd=ROOT,env=env,text=True).strip();env['AGENT_BROWSER_SESSION']='mn'+seed.rsplit('-',1)[-1][:5]
    requests=[]
    class Handler(BaseHTTPRequestHandler):
        def log_message(self,*items):pass
        def do_GET(self):
            requests.append(self.path)
            if self.path=='/':raw=HTML.encode();mime='text/html; charset=utf-8'
            elif self.path=='/manifest':raw=json.dumps(manifest).encode();mime='application/json'
            elif self.path=='/init':raw=(fixture/'init.mp4').read_bytes();mime='audio/mp4'
            elif self.path.startswith('/fragment/'):
                index=int(self.path.rsplit('/',1)[-1]);raw=(fixture/manifest['fragments'][index]['file']).read_bytes();mime='audio/mp4'
            else:self.send_error(404);return
            self.send_response(200);self.send_header('Content-Type',mime);self.send_header('Content-Length',str(len(raw)));self.end_headers();self.wfile.write(raw)
        def do_POST(self):
            if self.path!='/recording':self.send_error(404);return
            raw=self.rfile.read(int(self.headers['Content-Length']));(out/'native-capture.webm').write_bytes(raw);self.send_response(204);self.end_headers()
    server=ThreadingHTTPServer(('127.0.0.1',0),Handler);thread=threading.Thread(target=server.serve_forever,daemon=True);thread.start()
    def browser(*items):
        result=subprocess.run(['agent-browser',*items,'--json'],cwd=ROOT,env=env,text=True,capture_output=True,timeout=55)
        if result.returncode:raise RuntimeError(result.stdout+result.stderr)
        value=json.loads(result.stdout);assert value['success'],value
        return value['data']
    opened=False;result={}
    try:
        browser('--profile',str(out/'profile'),'open','http://127.0.0.1:'+str(server.server_port)+'/');opened=True
        browser('click','#start');browser('wait','--fn','window.musicNativeDone === true')
        result=browser('eval','window.musicNativeResult')['result']
        (out/'observed.json').write_text(json.dumps(result,indent=2)+'\n')
        assert not result['errors']and result['done']and result['trusted']and not result['audioContextCreated'],result
        assert result['sourceChanges']==1 and len(result['appends'])==len(manifest['fragments'])+1
        assert result['final']['ended']and abs(result['final']['duration']-12)<0.001 and abs(result['final']['media']-12)<0.03
        assert len(result['crossings'])>=2
        start=next(event['wall']for event in result['events']if event['kind']=='play-resolved')
        unwanted=[event for event in result['events']if event['wall']>start and event['kind']in('waiting','stalled','error','seeking')]
        assert not unwanted,unwanted
        assert sum(not append['paused']and append['media']>0 for append in result['appends'])>=2
        assert all(len(append['ranges'])==1 for append in result['appends']if append['index']>=0)
        assert all(row['after']>=row['boundary']and row['before']<row['boundary']for row in result['crossings'])
        end=next(event['wall']for event in result['events']if event['kind']=='ended')
        assert len([event for event in result['events']if event['kind']=='ended'])==1
        assert not [event for event in result['events']if event['kind']in('capture-inactive','capture-removetrack','recorder-stop')and event['wall']<end-50]
        report={'pass':True,'scope':'Native Chromium HTMLAudio/MSE lossless-container fragment feasibility; not independent DSP or infinite BGM',
                'user_agent':browser('eval','navigator.userAgent')['result'],'source_pcm_roundtrip_exact':True,
                'source_pcm_sha256':manifest['source_excerpt_pcm_sha256'],'stereo_frames':manifest['stereo_frames'],
                'fragment_count':len(manifest['fragments']),'observed_boundaries':len(result['crossings']),
                'native_waiting_after_play':len(unwanted),'source_changes':result['sourceChanges'],'audio_context_created':False,
                'capture_ended_before_final_boundary':False,'requests':requests,'manifest_sha256':digest(fixture/'manifest.json'),
                'observed_sha256':digest(out/'observed.json'),'harness_sha256':digest(Path(__file__).resolve()),
                'limits':['Container decoded PCM roundtrip is exact; native speaker output is not separately captured sample-for-sample',
                          'Any native WebM capture is lossy48kHz timing evidence, not original44.1kHz PCM proof',
                          'Only this Chromium build is tested; Safari/mobile remain unverified','Source is a finite original excerpt to test transport; no BGM looping or independent synthesis is claimed']}
        (out/'result.json').write_text(json.dumps(report,indent=2)+'\n');print(json.dumps(report,indent=2))
    except Exception as error:
        (out/'failure.json').write_text(json.dumps({'error':repr(error),'partial':result},indent=2)+'\n');raise
    finally:
        if opened:browser('close')
        server.shutdown();server.server_close();thread.join(timeout=2)

if __name__=='__main__':
    parser=argparse.ArgumentParser(description=__doc__);parser.add_argument('--out',type=Path,required=True);parser.add_argument('--fixture',type=Path,required=True)
    args=parser.parse_args();out=args.out.resolve();fixture=args.fixture.resolve();assert out.is_relative_to(ROOT/'.local')and fixture.is_relative_to(ROOT/'.local');out.mkdir(parents=True,exist_ok=True);main(out,fixture)
