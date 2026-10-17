"""Real native-media loop and adapter control acceptance in an isolated browser.

Seeks near the original long-loop boundary to exercise two real native wraps;
does not shorten, replace or accelerate the music. This is not a listening test.
"""
from pathlib import Path
from http.server import BaseHTTPRequestHandler,ThreadingHTTPServer
import hashlib
import json
import os
import re
import subprocess
import threading
import uuid
import wave
from urllib.parse import quote

ROOT=Path(__file__).resolve().parents[2]
TRACK=int(os.getenv('BASKETBALL_AUDIO_TEST_TRACK','66'))
assert TRACK in [59,60,61,62,63,64,65,66,75]
LOOP=TRACK!=75
PROBE=os.getenv('BASKETBALL_AUDIO_CODEC_PROBE','')
FALLBACK=os.getenv('BASKETBALL_AUDIO_CDN_FAILURE','')=='1'
assert not PROBE or re.fullmatch(r'[a-z0-9.-]+',PROBE)
MASTER=ROOT/'.local/basketball/audio/export'/f'music-{TRACK:03}{"-loop" if LOOP else ""}.wav'
PLAYBACK=json.loads((ROOT/'games/basketball/assets/audio-playback.json').read_text())['music'][str(TRACK)]
ASSET=(ROOT/'.local/basketball/audio/compression/probes'/PROBE) if PROBE else ROOT/'.local/basketball/audio/export'/PLAYBACK.get('file',PLAYBACK['wav'])
DIRECTORY=(('codec-probe-'+PROBE) if PROBE else (('encoded-loop-browser-'+str(TRACK)) if ASSET.suffix!='.wav' else ('loop-browser' if TRACK==66 else f'loop-browser-{TRACK:03}'))) + ('-cdn-fallback' if FALLBACK else '')
OUT=ROOT/'.local/basketball/audio'/DIRECTORY


def main():
    OUT.mkdir(parents=True,exist_ok=True)
    env=dict(os.environ,AGENT_BROWSER_SESSION='baq-'+uuid.uuid4().hex[:6],
        AGENT_BROWSER_SOCKET_DIR=str(ROOT/'.local/baq-sockets'),TMPDIR=str(ROOT/'.local/tmp'))
    Path(env['AGENT_BROWSER_SOCKET_DIR']).mkdir(exist_ok=True)
    subprocess.run([str(ROOT/'web/node_modules/.bin/esbuild'),str(ROOT/'scripts/basketball/audio_loop_browser.ts'),
        '--bundle','--format=esm','--target=es2022','--outfile='+str(OUT/'loop.js')],env=env,cwd=ROOT,check=True)
    requests=[]
    with wave.open(str(MASTER)) as source:expected_seconds=source.getnframes()/source.getframerate()

    class Handler(BaseHTTPRequestHandler):
        def log_message(self,*args):pass
        def do_GET(self):
            route=self.path.split('?')[0]
            requests.append({'path':route,'range':self.headers.get('Range')})
            file=None
            if route=='/':
                content=b'<!doctype html><meta charset="utf-8"><title>Basketball native audio loop acceptance</title><button id="start">Start original music verification</button><pre id="status">Ready</pre><script type="module" src="/loop.js"></script>'
                mime='text/html'
            elif route=='/loop.js':content=(OUT/'loop.js').read_bytes();mime='text/javascript'
            elif route==f'/audio/{ASSET.name}':file=ASSET;content=b'';mime={'.wav':'audio/wav','.mp3':'audio/mpeg','.m4a':'audio/mp4','.ogg':'audio/ogg','.flac':'audio/flac'}[ASSET.suffix]
            else:self.send_error(404);return
            size=file.stat().st_size if file else len(content)
            match=re.fullmatch(r'bytes=(\d+)-(\d*)',self.headers.get('Range',''));start=0;end=size-1
            if match:
                start=int(match[1]);end=min(end,int(match[2]) if match[2] else end)
                if start>end:self.send_error(416);return
            self.send_response(206 if match else 200);self.send_header('Content-Type',mime);self.send_header('Accept-Ranges','bytes')
            if match:self.send_header('Content-Range',f'bytes {start}-{end}/{size}')
            self.send_header('Content-Length',str(end-start+1));self.end_headers()
            try:
                if file:
                    with file.open('rb') as source:
                        source.seek(start);remaining=end-start+1
                        while remaining:
                            chunk=source.read(min(remaining,128*1024))
                            if not chunk:break
                            self.wfile.write(chunk);remaining-=len(chunk)
                else:self.wfile.write(content[start:end+1])
            except (BrokenPipeError,ConnectionResetError):pass

    def browser(*args):
        p=subprocess.run(['agent-browser',*args,'--json'],env=env,cwd=ROOT,capture_output=True,text=True,timeout=45)
        if p.returncode:raise RuntimeError(p.stdout+p.stderr)
        result=json.loads(p.stdout);assert result['success'],result;return result['data']
    def js(text):return browser('eval',text)['result']
    server=ThreadingHTTPServer(('127.0.0.1',0),Handler)
    threading.Thread(target=server.serve_forever,daemon=True).start()
    opened=False
    try:
        browser('--profile',str(OUT/'profile'),'open',f'http://127.0.0.1:{server.server_port}/?track={TRACK}&probe={quote(PROBE)}'+('&fallback=1' if FALLBACK else ''));opened=True
        support=js('(()=>{const m=document.querySelector("audio");return Object.fromEntries(["audio/mpeg","audio/mp4; codecs=mp4a.40.2","audio/ogg; codecs=opus","audio/flac"].map(t=>[t,m.canPlayType(t)]))})()')
        browser('click','#start')
        browser('wait','--fn','window.basketballLoopAcceptance.started || !!window.basketballLoopAcceptance.error')
        assert js('basketballLoopAcceptance.error')==''
        browser('wait','--fn','basketballLoopAcceptance.media?.duration>100 && basketballLoopAcceptance.media.currentTime>.1')
        initial=js('(()=>{const m=basketballLoopAcceptance.media;return{src:m.currentSrc,duration:m.duration,time:m.currentTime,loop:m.loop,paused:m.paused,mediaCount:document.querySelectorAll("audio").length}})()')
        assert initial['loop']==LOOP and not initial['paused'] and abs(initial['duration']-expected_seconds)<.001
        assert initial['src'].endswith(f'/audio/{ASSET.name}')
        wraps=[]
        for n in ((1,2) if LOOP else ()):
            js('basketballLoopAcceptance.media.currentTime=basketballLoopAcceptance.media.duration-.4')
            browser('wait','--fn',f'basketballLoopAcceptance.loops>={n} && basketballLoopAcceptance.media.currentTime>.1 && basketballLoopAcceptance.media.currentTime<3')
            state=js('(()=>{const h=basketballLoopAcceptance,m=h.media;return{loops:h.loops,time:m.currentTime,paused:m.paused,ended:m.ended,src:m.currentSrc,mediaCount:document.querySelectorAll("audio").length}})()')
            assert not state['paused'] and not state['ended'] and state['src']==initial['src'] and state['mediaCount']==initial['mediaCount']
            wraps.append(state)
        paused=js('(async()=>{const h=basketballLoopAcceptance;await h.adapter.consume([253],2);return{time:h.media.currentTime,paused:h.media.paused,src:h.media.currentSrc}})()')
        assert paused['paused']
        resumed=js('(async()=>{const h=basketballLoopAcceptance,r=await h.adapter.consume([255],3);return{result:r,time:h.media.currentTime,paused:h.media.paused,src:h.media.currentSrc}})()')
        assert not resumed['paused'] and not resumed['result']['failed'] and resumed['src']==paused['src']
        assert resumed['time']>=paused['time'] and resumed['time']-paused['time']<.2
        finite_ended=None
        if not LOOP:
            js('basketballLoopAcceptance.media.currentTime=basketballLoopAcceptance.media.duration-.4')
            browser('wait','--fn','basketballLoopAcceptance.media.ended && basketballLoopAcceptance.media.paused')
            finite_ended=js('({time:basketballLoopAcceptance.media.currentTime,ended:basketballLoopAcceptance.media.ended,paused:basketballLoopAcceptance.media.paused})')
            assert finite_ended['ended'] and finite_ended['paused']
        before_dispose=js('basketballLoopAcceptance.samples')
        disposed=js('(()=>{const h=basketballLoopAcceptance;h.adapter.dispose();return{paused:h.media.paused,src:h.media.getAttribute("src")}})()')
        (OUT/'lifecycle-observation.json').write_text(json.dumps({'initial':initial,'finite_ended':finite_ended,'paused':paused,'resumed':resumed,'disposed':disposed},indent=2)+'\n')
        assert disposed['paused'] and disposed['src'] is None,disposed
        if ASSET.suffix!='.wav':assert not any(r['path'].endswith('.wav') for r in requests),'WAV master fetched on distribution path'
        if FALLBACK:assert any(r['path'].startswith('/cdn-unavailable/') and r['path'].endswith('.mp3') for r in requests)
        result={'status':'passed','music_id':TRACK,'scope':'real HTML media looping and platform adapter controls; not listening or complete gameplay acceptance',
            'initial':initial,'wraps':wraps,'finite_ended':finite_ended,'paused':paused,'resumed':resumed,'disposed':disposed,'timeupdates':before_dispose,
            'requests':requests,'wav_sha256':hashlib.file_digest(MASTER.open('rb'),'sha256').hexdigest(),
            'asset_sha256':hashlib.file_digest(ASSET.open('rb'),'sha256').hexdigest(),'codec_support':support,'asset_file':ASSET.name,
            'bundle_sha256':hashlib.file_digest((OUT/'loop.js').open('rb'),'sha256').hexdigest(),'cdn_failure_exercised':FALLBACK,
            'listening_accepted':False,'accepted_for_release':False}
        (OUT/'result.json').write_text(json.dumps(result,indent=2)+'\n')
        browser('screenshot',str(OUT/'verified.png'))
        print(json.dumps({'status':'passed','wraps':len(wraps),'music_seconds':initial['duration']}))
    finally:
        if opened:browser('close')
        server.shutdown();server.server_close()


if __name__=='__main__':main()
