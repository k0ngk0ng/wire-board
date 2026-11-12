#!/usr/bin/env python3
"""Isolated native-media mechanism diagnostic, NOT production game acceptance.

Uses one HTMLAudioElement with the platform loop/rate/pitch settings. Compare
with/without capture and original FLAC/identical-PCM WAV/marked playback copy.
No game state, private product player, AudioContext or application build changes.
"""
import argparse,base64,hashlib,json,os
from pathlib import Path
import shutil,subprocess,threading,time,uuid,wave
from http.server import BaseHTTPRequestHandler,ThreadingHTTPServer

ROOT=Path(__file__).resolve().parents[2]
HTML='''<!doctype html><meta charset="utf-8"><title>Native loop container diagnostic</title>
<button id="start">Start diagnostic</button><pre id="status">Isolated media diagnostic; not the production game.</pre><audio id="player" hidden></audio>
<script>
const cases=CASES;const player=document.querySelector('#player');player.preload='none';player.loop=true;player.preservesPitch=false;player.mozPreservesPitch=false;player.webkitPreservesPitch=false;
const sleep=ms=>new Promise(r=>setTimeout(r,ms));const post=(path,data)=>fetch(path,{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(data)});
async function hash(data){return [...new Uint8Array(await crypto.subtle.digest('SHA-256',data))].map(x=>x.toString(16).padStart(2,'0')).join('')}
document.querySelector('#start').onclick=async()=>{document.querySelector('#start').disabled=true;const results=[];
 try{if(navigator.audioSession)navigator.audioSession.type='playback';for(const c of cases){
  document.querySelector('#status').textContent=c.name;await post('/progress',{stage:c.name,at:Date.now()});
  const bytes=Uint8Array.from(atob(c.source.split(',')[1]),x=>x.charCodeAt(0));if(await hash(bytes)!==c.sha256)throw Error('source digest mismatch');
  player.pause();player.defaultPlaybackRate=c.rate;player.src=c.source;player.volume=.1;player.playbackRate=c.rate;await player.play();await sleep(300);
  const events=[],startedAt=Date.now();const listen=e=>events.push({at:Date.now(),type:e.type,time:player.currentTime,duration:player.duration,rate:player.playbackRate,gain:player.volume,trusted:e.isTrusted});
  for(const name of ['seeking','seeked','waiting','playing','pause','ratechange','error'])player.addEventListener(name,listen);
  let stream,recorder,chunks=[],done;
  if(c.capture){stream=player.captureStream();if(!stream.getAudioTracks().length)await Promise.race([new Promise(r=>stream.addEventListener('addtrack',r,{once:true})),sleep(1000)]);
   if(!stream.getAudioTracks().length)throw Error('no capture audio track');recorder=new MediaRecorder(stream,{mimeType:'audio/webm;codecs=opus',audioBitsPerSecond:128000});done=new Promise((resolve,reject)=>{recorder.onstop=resolve;recorder.onerror=e=>reject(Error(e.error?.message??'recorder error'))});recorder.ondataavailable=e=>{if(e.data.size)chunks.push(e.data)};recorder.start(100);}
  await sleep(12000);const stoppedAt=Date.now();let artifact;
  if(recorder){recorder.stop();await Promise.race([done,sleep(3000).then(()=>{throw Error('recorder stop timeout')})]);const data=new Uint8Array(await new Blob(chunks).arrayBuffer());let binary='';for(let i=0;i<data.length;i+=16384)binary+=String.fromCharCode(...data.subarray(i,i+16384));artifact=btoa(binary);for(const track of stream.getTracks())track.stop();}
  const row={name:c.name,capture:c.capture,sourceSha256:c.sha256,expectedDuration:c.frames/22050,nativeDuration:player.duration,rate:player.playbackRate,gain:player.volume,startedAt,stoppedAt,events,artifact};
  for(const name of ['seeking','seeked','waiting','playing','pause','ratechange','error'])player.removeEventListener(name,listen);player.pause();results.push({...row,artifact:undefined});await post('/result',row);await sleep(100);
 }await post('/done',{passed:true,results,method:'isolated same native HTMLAudioElement; fixed rate/gain; with/without capture; not production game or hardware recording'});}
 catch(error){await post('/done',{passed:false,error:String(error),results});}finally{player.pause();player.removeAttribute('src');player.load();document.querySelector('#status').textContent='Finished';}}
</script>'''

def main():
 parser=argparse.ArgumentParser(description=__doc__);parser.add_argument('--out',type=Path,required=True);args=parser.parse_args();out=args.out.resolve()
 if not out.is_relative_to(ROOT)or out.exists():parser.error('output must be a new directory inside the repository')
 out.mkdir(parents=True);temporary=out/'temporary';temporary.mkdir();run='container'+uuid.uuid4().hex[:7];socket=ROOT/'.s'/run;socket.mkdir(parents=True)
 original=ROOT/'games/racing/assets/public/racing/v3/audio/ldia';wave_path=out/'ldia-41-original-pcm.wav';copy=ROOT/'.local/racing-nfs-reference/engine-playback-e2e/output/audio/ldia/engine-42-playback.flac'
 raw=subprocess.run(['ffmpeg','-nostdin','-v','error','-i',str(original/'engine-41.flac'),'-f','s16le','-acodec','pcm_s16le','pipe:1'],capture_output=True,check=True).stdout
 sample41=next(s for s in json.loads((original/'engine.json').read_text())['samples']if s['patch']==41)
 if hashlib.sha256(raw).hexdigest()!=sample41['pcmSha256']:raise ValueError('original41 PCM mismatch')
 with wave.open(str(wave_path),'wb')as wav:wav.setnchannels(1);wav.setsampwidth(2);wav.setframerate(22050);wav.writeframes(raw)
 if not copy.exists():
  derived=out/'derived-fixture';subprocess.run([os.sys.executable,'-B',str(ROOT/'scripts/racing/nfs-engine-playback.py'),'--out',str(derived),'--model','ldia','--patch','42'],cwd=ROOT,capture_output=True,check=True)
  copy=derived/'audio/ldia/engine-42-playback.flac'
 def sample(name,path,frames,rate,capture,mime):
  data=path.read_bytes();return {'name':name,'sha256':hashlib.sha256(data).hexdigest(),'frames':frames,'rate':rate,'capture':capture,'source':'data:'+mime+';base64,'+base64.b64encode(data).decode()}
 cases=[sample('41-flac-no-capture',original/'engine-41.flac',21744,.6902674649296876,False,'audio/flac'),sample('41-flac-capture',original/'engine-41.flac',21744,.6902674649296876,True,'audio/flac'),
 sample('41-wav-no-capture',wave_path,21744,.6902674649296876,False,'audio/wav'),sample('41-wav-capture',wave_path,21744,.6902674649296876,True,'audio/wav'),
 sample('42-original-flac-capture',original/'engine-42.flac',15505,.5,True,'audio/flac'),sample('42-playback-flac-capture',copy,15395,.5,True,'audio/flac')]
 page=HTML.replace('CASES',json.dumps(cases,separators=(',',':'))).encode();finished=threading.Event()
 provenance={'scriptSha256':hashlib.sha256(Path(__file__).read_bytes()).hexdigest(),'pageSha256':hashlib.sha256(page).hexdigest(),
             'hardwareMutedByLaunchFlag':True,'sources':[{k:v for k,v in c.items()if k!='source'}for c in cases]}
 class Handler(BaseHTTPRequestHandler):
  def log_message(self,*args):pass
  def do_GET(self):
   self.send_response(200);self.send_header('Content-Type','text/html;charset=utf-8');self.send_header('Content-Security-Policy',"default-src 'self'; script-src 'self' 'unsafe-inline'; media-src 'self' data:; connect-src 'self'; object-src 'none'");self.end_headers();self.wfile.write(page)
  def do_POST(self):
   value=json.loads(self.rfile.read(int(self.headers['Content-Length'])))
   if self.path=='/progress':print(json.dumps(value),flush=True)
   elif self.path=='/result':
    artifact=value.pop('artifact',None)
    if artifact:
     data=base64.b64decode(artifact);file=value['name']+'.webm';(out/file).write_bytes(data);value['artifact']={'file':file,'bytes':len(data),'sha256':hashlib.sha256(data).hexdigest()}
    (out/(value['name']+'.json')).write_text(json.dumps(value,indent=2)+'\n');print(json.dumps({'completed':value['name'],'nativeDuration':value['nativeDuration'],'events':len(value['events'])}),flush=True)
   elif self.path=='/done':(out/'report.json').write_text(json.dumps(value,indent=2)+'\n');finished.set()
   self.send_response(200);self.end_headers();self.wfile.write(b'{}')
 server=ThreadingHTTPServer(('127.0.0.1',0),Handler);threading.Thread(target=server.serve_forever,daemon=True).start();env=os.environ.copy()
 for key,folder in [('TMPDIR','tmp'),('XDG_CACHE_HOME','cache'),('XDG_CONFIG_HOME','config'),('XDG_DATA_HOME','data'),('AGENT_BROWSER_PROFILE','profile')]:
  path=temporary/folder;path.mkdir();env[key]=str(path)
 config=temporary/'agent-browser.json';config.write_text('{}\n');env.update(AGENT_BROWSER_SOCKET_DIR=str(socket),AGENT_BROWSER_CONFIG=str(config),AGENT_BROWSER_ARGS='--mute-audio,--disable-breakpad,--disable-crash-reporter')
 def browser(*cmd):return subprocess.run(['agent-browser','--session',run,*cmd,'--json'],cwd=ROOT,env=env,capture_output=True,text=True,timeout=40)
 try:
  result=browser('open',f'http://127.0.0.1:{server.server_port}/');print(result.stdout[:1000],flush=True)
  if result.returncode:raise RuntimeError(result.stderr)
  result=browser('find','role','button','click','--name','Start diagnostic');print(result.stdout[:1000],flush=True)
  if result.returncode:raise RuntimeError(result.stderr)
  deadline=time.monotonic()+115
  while not finished.wait(1):
   if time.monotonic()>deadline:raise TimeoutError('isolated native diagnostic timed out')
  report=json.loads((out/'report.json').read_text());report.update(provenance);(out/'report.json').write_text(json.dumps(report,indent=2)+'\n');print(json.dumps({'passed':report['passed'],'output':str(out.relative_to(ROOT))}),flush=True)
 finally:
  try:
   closed=browser('close')
   if closed.returncode==0:shutil.rmtree(temporary,ignore_errors=True)
  finally:server.shutdown();server.server_close();shutil.rmtree(socket,ignore_errors=True)

if __name__=='__main__':main()
