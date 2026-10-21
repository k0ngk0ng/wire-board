"""Real platform-native MP3 intro/suffix E2E; no source shortening or speedup.

Records native ended/seeked/playing event latency and optional MediaRecorder
output. Event latency is not a proof of sample-gapless speaker output.
"""
import base64,hashlib,json,os,re,subprocess,threading,uuid
from http.server import BaseHTTPRequestHandler,ThreadingHTTPServer
from pathlib import Path
from urllib.parse import urlparse
ROOT=Path(__file__).resolve().parents[2];OUT=ROOT/'.local/music76-source/browser';ASSETS=ROOT/'.local/basketball/audio/export'
ENTRY=json.load(open(ROOT/'games/basketball/assets/audio-playback.json'))['music']['76'];I=ENTRY['loop_start_sample']/ENTRY['sample_rate'];D=ENTRY['decoded_samples']/ENTRY['sample_rate'];L=D-I

def main():
 OUT.mkdir(parents=True,exist_ok=True);env=dict(os.environ,AGENT_BROWSER_SESSION='m'+uuid.uuid4().hex[:4],AGENT_BROWSER_SOCKET_DIR=str(ROOT/'.local/s'),TMPDIR=str(ROOT/'.local/tmp'))
 subprocess.run([str(ROOT/'web/node_modules/.bin/esbuild'),str(ROOT/'scripts/basketball/music76_browser.ts'),'--bundle','--format=esm','--target=es2022','--outfile='+str(OUT/'harness.js')],env=env,cwd=ROOT,check=True)
 requests=[]
 class Handler(BaseHTTPRequestHandler):
  def log_message(self,*args):pass
  def do_GET(self):
   route=urlparse(self.path).path;requests.append(dict(path=route,range=self.headers.get('Range')));file=None
   if route=='/':data=b'<!doctype html><meta charset="utf-8"><title>Music76 native media verification</title><button id="start">Start original music76 verification</button><pre>Ready</pre><script type="module" src="/harness.js"></script>';mime='text/html'
   elif route=='/harness.js':data=(OUT/'harness.js').read_bytes();mime='text/javascript'
   elif route=='/blank':data=b'<!doctype html><title>Visibility boundary</title>Music76 visibility verification';mime='text/html'
   elif route in ['/audio/'+ENTRY['file'],'/audio/music-075.mp3']:file=ASSETS/route.rsplit('/',1)[1];data=b'';mime='audio/mpeg'
   else:self.send_error(404);return
   size=file.stat().st_size if file else len(data);start=0;end=size-1;match=re.fullmatch(r'bytes=(\d+)-(\d*)',self.headers.get('Range',''))
   if match:start=int(match[1]);end=min(end,int(match[2])if match[2]else end)
   if start>end:self.send_error(416);return
   self.send_response(206 if match else 200);self.send_header('Content-Type',mime);self.send_header('Accept-Ranges','bytes');self.send_header('Content-Length',str(end-start+1))
   if match:self.send_header('Content-Range',f'bytes {start}-{end}/{size}')
   self.end_headers()
   try:
    if file:
     with file.open('rb')as f:
      f.seek(start);remaining=end-start+1
      while remaining:
       chunk=f.read(min(remaining,131072))
       if not chunk:break
       self.wfile.write(chunk);remaining-=len(chunk)
    else:self.wfile.write(data[start:end+1])
   except(BrokenPipeError,ConnectionResetError):pass
 server=ThreadingHTTPServer(('127.0.0.1',0),Handler);threading.Thread(target=server.serve_forever,daemon=True).start();url=f'http://127.0.0.1:{server.server_port}/'
 def browser(*args):
  p=subprocess.run(['agent-browser',*args,'--json'],env=env,cwd=ROOT,capture_output=True,text=True,timeout=45)
  if p.returncode:raise RuntimeError(p.stdout+p.stderr)
  r=json.loads(p.stdout);assert r['success'],r;return r['data']
 def js(code):return browser('eval',code)['result']
 def wait(code):return browser('wait','--fn',code)
 checks=[];recordings=[];opened=False
 try:
  browser('--profile',str(OUT/('profile-'+env['AGENT_BROWSER_SESSION'])),'open',url);opened=True;browser('snapshot','-i');browser('click','#start');wait('music76.started || !!music76.error');assert js('music76.error')==''
  wait('music76.media.duration>700 && music76.media.currentTime>.1');initial=js('music76.snapshot()');assert abs(initial['duration']-D)<1/44100 and not initial['loop'] and initial['rate']==1
  assert initial['src'].endswith('/'+ENTRY['file']);checks.append(dict(kind='native-initial',state=initial))
  js('music76.events=[];music76.media.currentTime=music76.loopStart-.3');wait('!music76.media.seeking && music76.media.currentTime>music76.loopStart+.25');prefix=js('({state:music76.snapshot(),events:music76.events})');assert not any(e['kind']=='ended'for e in prefix['events']);assert prefix['state']['src']==initial['src'];checks.append(dict(kind='continuous-prefix-to-body',**prefix))
  gaps=[]
  for n in [1,2]:
   capture=js('music76.events=[];music76.recordStart()');js('music76.media.currentTime=music76.media.duration-.5');wait('music76.events.some(e=>e.kind==="ended") && music76.media.currentTime>music76.loopStart+.25 && music76.media.currentTime<music76.loopStart+4 && !music76.media.paused')
   sample=js('({state:music76.snapshot(),events:music76.events})');events=sample['events'];ended=next(e for e in events if e['kind']=='ended');playing=next(e for e in events if e['kind']=='playing'and e['at']>=ended['at']);seeked=next(e for e in events if e['kind']=='seeked'and e['at']>=ended['at'])
   assert sample['state']['src']==initial['src'] and sample['state']['mediaCount']==initial['mediaCount'];gaps.append(dict(loop=n,endedToPlayingMS=playing['at']-ended['at'],endedToSeekedMS=seeked['at']-ended['at'],**sample))
   recording=js('music76.recordStop()')if capture else None
   if recording:
    data=base64.b64decode(recording.pop('data'));file=OUT/f'wrap-{n}.webm';file.write_bytes(data);recordings.append(dict(file=file.name,sha256=hashlib.sha256(data).hexdigest(),**recording))
  checks.append(dict(kind='two-native-suffix-wraps',gaps=gaps))
  paused=js('(async()=>{await music76.command([253]);return music76.snapshot()})()');assert paused['paused'];js('music76.pauseObservedAt=performance.now()');wait('performance.now()-music76.pauseObservedAt>250');still=js('music76.snapshot()');assert abs(still['time']-paused['time'])<.001
  resumed=js('(async()=>{await music76.command([255]);return music76.snapshot()})()');assert not resumed['paused']and abs(resumed['time']-paused['time'])<.25;checks.append(dict(kind='source-pause-resume',paused=paused,resumed=resumed))
  elapsed=(I+2*L+3)*1000;assert js(f'music76.restore({elapsed},false,500)')is True;wait('!music76.media.paused && music76.media.currentTime>music76.loopStart+3.4');restored=js('music76.snapshot()');assert I+3.5<=restored['time']<I+5;checks.append(dict(kind='wall-clock-restore',elapsedMS=elapsed,serverDeltaMS=500,state=restored))
  js('music76.restore((music76.loopStart-.5)*1000,true,999999)');wait('music76.media.readyState>0 && music76.media.paused');frozen=js('music76.snapshot()');assert abs(frozen['time']-(I-.5))<.001;checks.append(dict(kind='paused-snapshot-restore',state=frozen));js('music76.command([255])')
  muted=js('(()=>{music76.staleEnded=music76.media.onended;music76.platform.setEnabled(false);music76.staleEnded?.();return music76.snapshot()})()');assert muted['attribute']is None and muted['paused'];enabled=js('music76.platform.setEnabled(true);music76.snapshot()');assert enabled['attribute']is None;checks.append(dict(kind='mute-clears-no-stale-restart',muted=muted,enabled=enabled))
  assert js(f'music76.restore({elapsed},false,1000)')is True
  # Exercise native tab visibility and the same stop-on-hidden policy as main.tsx.
  tabs=browser('tab');(OUT/'tabs-before.json').write_text(json.dumps(tabs,indent=2)+'\n')
  original_tab=next(t['tabId']for t in tabs['tabs']if t['active']);browser('tab','new',url+'blank');browser('tab',original_tab);visibility=js('({events:music76.visibility,state:music76.snapshot()})')
  assert any(v['hidden']for v in visibility['events']),'native hidden transition not observed';assert visibility['state']['attribute']is None;checks.append(dict(kind='native-hidden-clears',**visibility))
  assert js(f'music76.restore({elapsed},false,1500)')is True
  newer=js('(async()=>{music76.staleEnded=music76.media.onended;await music76.platform.channel("other-owner").music(["/audio/music-075.mp3"],{loop:false,startOffset:3});music76.staleEnded?.();return music76.snapshot()})()');assert newer['src'].endswith('/music-075.mp3')and newer['time']>=3 and not newer['paused'];checks.append(dict(kind='owner-replacement',state=newer))
  disposed=js('music76.platform.dispose();music76.snapshot()');assert disposed['attribute']is None and disposed['paused'];checks.append(dict(kind='dispose',state=disposed))
  browser('open',url+'?fallback=1');browser('snapshot','-i');browser('click','#start');wait('music76.started || !!music76.error');assert js('music76.error')=='';wait('music76.media.currentTime>.1');fallback=js('music76.snapshot()');assert fallback['src'].endswith('/audio/'+ENTRY['file']);assert any(r['path'].startswith('/cdn-unavailable/')for r in requests);checks.append(dict(kind='real-source-fallback',state=fallback));js('music76.platform.dispose()')
  assert not any(r['path'].endswith('.wav')for r in requests)
  result=dict(status='passed',scope='real Chromium native HTML media on verified complete MP3; explicit boundary seeks at1x, not full-duration listening/device or sample-gapless acceptance',loopStartSeconds=I,loopSeconds=L,durationSeconds=D,assetFile=ENTRY['file'],assetSHA256=hashlib.file_digest((ASSETS/ENTRY['file']).open('rb'),'sha256').hexdigest(),bundleSHA256=hashlib.file_digest((OUT/'harness.js').open('rb'),'sha256').hexdigest(),checks=checks,recordings=recordings,requests=requests,listening_accepted=False,accepted_for_release=False,gapScope='native ended-to-playing event latency; not speaker-output or sample-gaplessness proof')
  (OUT/'result.json').write_text(json.dumps(result,indent=2)+'\n');browser('screenshot',str(OUT/'verified.png'));print(json.dumps(dict(status='passed',checks=len(checks),gapsMS=[g['endedToPlayingMS']for g in gaps],recordings=len(recordings))))
 except Exception:
  try:(OUT/'failure.json').write_text(json.dumps(dict(checks=checks,requests=requests,state=js('typeof music76!=="undefined"?{state:music76.media?music76.snapshot():null,events:music76.events,error:music76.error,hidden:document.hidden}:null')),indent=2)+'\n')
  except Exception:pass
  raise
 finally:
  if opened:browser('close')
  server.shutdown();server.server_close()
if __name__=='__main__':main()
