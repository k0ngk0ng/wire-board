"""Read actual native HTML media/MSE codec declarations in a private browser."""
from pathlib import Path
from http.server import BaseHTTPRequestHandler,ThreadingHTTPServer
import argparse,hashlib,json,os,subprocess,threading
from reference import ROOT

HTML='''<!doctype html><meta charset="utf-8"><title>Native music carrier probe</title><p>Native media codec declarations</p><script>
(async()=>{const types=['audio/wav','audio/wav; codecs="1"','audio/flac','audio/mp4; codecs="fLaC"','audio/mp4; codecs="alac"','audio/mp4; codecs="mp4a.40.2"','audio/webm; codecs="opus"','audio/ogg; codecs="flac"'];
const audio=document.createElement('audio');const rows=[];
for(const type of types){let mse=false,error=null;try{mse=typeof MediaSource!=='undefined'&&MediaSource.isTypeSupported(type)}catch(e){error=e.name+': '+e.message}rows.push({type,htmlAudio:audio.canPlayType(type),mediaSource:mse,error})}
const encoders=[];for(const codec of ['flac','alac','opus','mp4a.40.2','pcm-s16']){try{const value=await AudioEncoder.isConfigSupported({codec,sampleRate:44100,numberOfChannels:2});encoders.push({codec,supported:value.supported,config:value.config})}catch(e){encoders.push({codec,supported:false,error:e.name+': '+e.message})}}
window.musicCarrierProbe={scope:'Capability declarations only; no playback or continuity proof',userAgent:navigator.userAgent,secureContext:isSecureContext,mediaSource:typeof MediaSource,managedMediaSource:typeof ManagedMediaSource,audioEncoder:typeof AudioEncoder,audioData:typeof AudioData,mediaStreamTrackGenerator:typeof MediaStreamTrackGenerator,rows,encoders,audioContextCreated:false};window.musicCarrierProbeDone=true;
})();</script>'''

def main(out):
    tmp=out/'tmp';tmp.mkdir(parents=True,exist_ok=True)
    sock=ROOT/'.local/ym';sock.mkdir(parents=True,exist_ok=True)
    env=dict(os.environ,TMPDIR=str(tmp),AGENT_BROWSER_SOCKET_DIR=str(sock))
    seed=subprocess.check_output(['agent-browser','session','id','--scope','worktree','--prefix','music-carrier'],cwd=ROOT,env=env,text=True).strip()
    env['AGENT_BROWSER_SESSION']='mu'+seed.rsplit('-',1)[-1][:5]
    class Handler(BaseHTTPRequestHandler):
        def log_message(self,*items):pass
        def do_GET(self):
            raw=HTML.encode();self.send_response(200);self.send_header('Content-Type','text/html; charset=utf-8');self.send_header('Content-Length',str(len(raw)));self.end_headers();self.wfile.write(raw)
    server=ThreadingHTTPServer(('127.0.0.1',0),Handler);thread=threading.Thread(target=server.serve_forever,daemon=True);thread.start()
    def browser(*args):
        value=subprocess.run(['agent-browser',*args,'--json'],cwd=ROOT,env=env,text=True,capture_output=True,timeout=55)
        if value.returncode:raise RuntimeError(value.stdout+value.stderr)
        result=json.loads(value.stdout);assert result['success'],result
        return result['data']
    opened=False
    try:
        url='http://127.0.0.1:'+str(server.server_port)+'/'
        browser('--profile',str(out/'profile'),'open',url);opened=True
        browser('wait','--fn','window.musicCarrierProbeDone === true')
        result=browser('eval','window.musicCarrierProbe')['result']
        result['source_sha256']=hashlib.sha256(Path(__file__).read_bytes()).hexdigest()
        (out/'capabilities.json').write_text(json.dumps(result,indent=2)+'\n')
        print(json.dumps(result,indent=2))
    finally:
        if opened:browser('close')
        server.shutdown();server.server_close();thread.join(timeout=2)

if __name__=='__main__':
    p=argparse.ArgumentParser(description=__doc__);p.add_argument('--out',type=Path,required=True);a=p.parse_args();out=a.out.resolve();assert out.is_relative_to(ROOT/'.local');out.mkdir(parents=True,exist_ok=True);main(out)
