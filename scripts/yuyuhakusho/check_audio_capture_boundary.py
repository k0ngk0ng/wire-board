"""Observe native captureStream/MediaRecorder lifetime across actual suffix ends."""
from pathlib import Path
from http.server import BaseHTTPRequestHandler,ThreadingHTTPServer
from urllib.parse import urlsplit
import base64,hashlib,json,os,re,subprocess,threading,wave
ROOT=Path(__file__).resolve().parents[2]
OUT=ROOT/'games/yuyuhakusho/verification/audio-platform-media-api/capture-boundary'
TMP=ROOT/'.local/md-yuyuhakusho/audio-capture-boundary'
for p in [OUT,TMP,TMP/'tmp',TMP/'baseline',ROOT/'.local/ym']:p.mkdir(parents=True,exist_ok=True)
env=dict(os.environ,TMPDIR=str(TMP/'tmp'),AGENT_BROWSER_SOCKET_DIR=str(ROOT/'.local/ym'))
name=subprocess.check_output(['agent-browser','session','id','--scope','worktree','--prefix','yy-capture'],cwd=ROOT,env=env,text=True).strip();env['AGENT_BROWSER_SESSION']='ac'+name.rsplit('-',1)[-1][:6]
bindings={}
for name in ['audio.ts','audio-tones.ts']:
 raw=subprocess.check_output(['git','show','aa617e23:web/src/'+name],cwd=ROOT)
 (TMP/'baseline'/name).write_bytes(raw);bindings['aa617e23:web/src/'+name]=hashlib.sha256(raw).hexdigest()
esbuild=str(ROOT/'web/node_modules/.bin/esbuild')
subprocess.run([esbuild,str(TMP/'baseline/audio.ts'),'--bundle','--format=esm','--target=es2022','--outfile='+str(TMP/'baseline-audio.js')],cwd=ROOT,env=env,check=True)
subprocess.run([esbuild,'scripts/yuyuhakusho/audio_capture_boundary_harness.ts','--bundle','--format=esm','--target=es2022','--external:/baseline-audio.js','--outfile='+str(TMP/'harness.js')],cwd=ROOT,env=env,check=True)
diagnostic=ROOT/'.local/md-yuyuhakusho/audio-media-api/diagnostic-loop.wav';assert diagnostic.is_file()
class Handler(BaseHTTPRequestHandler):
 def log_message(self,*args):pass
 def do_GET(self):
  route=urlsplit(self.path).path
  if route=='/':data=b'<!doctype html><meta charset="utf-8"><title>Native capture boundary</title><h1>Native capture boundary</h1><button id="start">Observe two native boundaries</button><pre>Ready</pre><script type="module" src="/harness.js"></script>';mime='text/html'
  elif route in ['/harness.js','/baseline-audio.js']:data=(TMP/route[1:]).read_bytes();mime='text/javascript'
  elif route=='/diagnostic-loop.wav':data=diagnostic.read_bytes();mime='audio/wav'
  else:self.send_error(404);return
  size=len(data);first=0;last=size-1;match=re.fullmatch(r'bytes=(\d+)-(\d*)',self.headers.get('Range',''))
  if match:first=int(match[1]);last=min(last,int(match[2])if match[2]else last)
  if first>last:self.send_error(416);return
  self.send_response(206 if match else 200);self.send_header('Content-Type',mime);self.send_header('Accept-Ranges','bytes');self.send_header('Content-Length',str(last-first+1))
  if match:self.send_header('Content-Range',f'bytes {first}-{last}/{size}')
  self.end_headers()
  try:self.wfile.write(data[first:last+1])
  except(BrokenPipeError,ConnectionResetError):pass
server=ThreadingHTTPServer(('127.0.0.1',0),Handler);thread=threading.Thread(target=server.serve_forever,daemon=True);thread.start();url=f'http://127.0.0.1:{server.server_port}/'
def browser(*args):
 p=subprocess.run(['agent-browser',*args,'--json'],cwd=ROOT,env=env,text=True,capture_output=True,timeout=45)
 if p.returncode:raise RuntimeError(p.stdout+p.stderr)
 r=json.loads(p.stdout);assert r['success'],r
 return r['data']
def js(code):return browser('eval',code)['result']
opened=False
try:
 browser('--profile',str(TMP/'profile'),'open',url);opened=True;browser('snapshot','-i');browser('click','#start');browser('wait','--fn','nativeCaptureBoundary.complete || !!nativeCaptureBoundary.error')
 assert not js('nativeCaptureBoundary.error'),js('nativeCaptureBoundary.error')
 r=js('({complete:nativeCaptureBoundary.complete,rows:nativeCaptureBoundary.rows,supported:nativeCaptureBoundary.supported,userAgent:navigator.userAgent})')
 capture=js('nativeCaptureBoundary.capture');raw=base64.b64decode(capture.pop('data'));path=OUT/'native-recording.webm';path.write_bytes(raw);r['capture']={**capture,'path':str(path.relative_to(ROOT)),'sha256':hashlib.sha256(raw).hexdigest()}
 decoded=TMP/'decoded.wav';subprocess.run(['ffmpeg','-hide_banner','-v','error','-i',str(path),'-map','0:a:0','-acodec','pcm_s16le','-y',str(decoded)],cwd=ROOT,env=env,check=True)
 with wave.open(str(decoded),'rb')as audio:
  samples=audio.getnframes();rate=audio.getframerate();pcm=audio.readframes(samples)
 r['decoded']={'sample_rate':rate,'stereo_frames':samples,'seconds':samples/rate,'pcm_sha256':hashlib.sha256(pcm).hexdigest()}
 ends=[x for x in r['rows']if x['kind']=='media-ended'];stops=[x for x in r['rows']if x['kind']=='recorder-stop'];explicit=[x for x in r['rows']if x['kind']=='explicit-recorder-stop'];assert len(ends)>=2 and stops
 r.update(pass_=True,baseline_source_sha256=bindings,diagnostic_sha256=hashlib.sha256(diagnostic.read_bytes()).hexdigest(),actual_media_boundaries=len(ends),recording_span_seconds=samples/rate,media_boundary_span_seconds=(ends[1]['at']-next(x['at']for x in r['rows']if x['kind']=='before-record-start'))/1000,recorder_stopped_before_explicit_request=bool(not explicit or stops[0]['at']<explicit[0]['at']),cross_boundary_capture_verified=samples/rate>4.5,sample_gapless_verified=False,speaker_output_verified=False,scope='Committed GameAudio baseline; real native captureStream/MediaRecorder observation across two actual suffix endings, no output-device timing proof')
 r['pass']=r.pop('pass_');r['source_sha256']={name:hashlib.sha256((ROOT/name).read_bytes()).hexdigest()for name in ['scripts/yuyuhakusho/audio_capture_boundary_harness.ts','scripts/yuyuhakusho/check_audio_capture_boundary.py']}
 (OUT/'result.json').write_text(json.dumps(r,ensure_ascii=False,indent=2)+'\n');browser('screenshot',str(OUT/'verified.png'));print(json.dumps({k:r[k]for k in ['pass','actual_media_boundaries','recording_span_seconds','media_boundary_span_seconds','recorder_stopped_before_explicit_request','cross_boundary_capture_verified']}))
except Exception as e:
 (OUT/'failure.json').write_text(json.dumps({'error':repr(e)},indent=2)+'\n');raise
finally:
 if opened:browser('close')
 server.shutdown();server.server_close();thread.join(timeout=5)
