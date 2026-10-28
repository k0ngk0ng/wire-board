#!/usr/bin/env python3
"""Repeatable native-media seek acceptance. No game rules or fake media player."""
import argparse,functools,hashlib,http.server,json,math,os,re,shutil,struct,subprocess,threading,traceback,uuid,wave
from pathlib import Path

parser=argparse.ArgumentParser();parser.add_argument('--out',required=True);args=parser.parse_args()
root=Path(__file__).resolve().parents[2];out=(root/args.out).resolve()
assert out.is_relative_to(root)and not out.exists();out.mkdir(parents=True)
temp=out/'temporary';temp.mkdir();(temp/'tmp').mkdir()
session='ks'+uuid.uuid4().hex[:6];sockets=root/'.s'/session;sockets.mkdir(parents=True)
env=dict(os.environ,TMPDIR=str(temp/'tmp'),AGENT_BROWSER_SESSION=session,AGENT_BROWSER_SOCKET_DIR=str(sockets),AGENT_BROWSER_PROFILE=str(temp/'profile'),AGENT_BROWSER_ARGS='--mute-audio,--disable-breakpad,--disable-crash-reporter')
files=[root/'web/src/audio.ts',root/'web/src/game-api/module.ts',Path(__file__)]
sources={str(p.relative_to(root)):hashlib.sha256(p.read_bytes()).hexdigest()for p in files}
shutil.copyfile(__file__,out/'harness-at-run.py')
report={'passed':False,'scope':'Native Chromium HTMLAudioElement, production GameAudio; no game integration, physical iOS or audio-output capture.','sources':sources,'checks':[]}
server=None;opened=False;gates={name:threading.Event()for name in ['delayed','cancelled']};requested={name:threading.Event()for name in gates}

def browser(*args):
    p=subprocess.run(['agent-browser',*args,'--json'],cwd=root,env=env,capture_output=True,text=True,timeout=45)
    assert p.returncode==0,p.stdout+p.stderr
    j=json.loads(p.stdout);assert j['success'],j;return j['data']
def js(code):return browser('eval',code)['result']
def check(name,code):
    result=js(code);assert result['passed'],(name,result)
    report['checks'].append({'name':name,**result})

try:
    for name,duration in [('long',8),('short',.2)]:
        with wave.open(str(temp/(name+'.wav')),'wb')as f:
            f.setparams((1,2,8000,0,'NONE','not compressed'))
            f.writeframes(b''.join(struct.pack('<h',round(math.sin(i*2*math.pi*330/8000)*500))for i in range(round(8000*duration))))
    build=subprocess.run(['node','--input-type=module','-e',"import{build}from'./web/node_modules/esbuild/lib/main.js';await build({entryPoints:['web/src/audio.ts'],outfile:process.argv[1],bundle:true,format:'esm',platform:'browser'});",str(temp/'audio.js')],cwd=root,env=env,text=True,capture_output=True)
    assert build.returncode==0,build.stderr
    (temp/'index.html').write_text('''<!doctype html><html lang="zh-CN"><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>原生音频位置校准验收</title><style>body{font:16px system-ui;color:#152828;background:#f4f3ed;padding:24px}button{padding:12px;font:inherit;color:#152828;background:#fff;border:1px solid #777}pre{white-space:pre-wrap;overflow-wrap:anywhere}</style><h1>原生音频位置校准验收</h1><button id="start">播放原生媒体</button><pre id="status">就绪</pre><script type="module">
import {GameAudio} from './audio.js';const voices=[];const a=new GameAudio(()=>{const m=new Audio();m.seekProof={playCalls:0};const play=m.play.bind(m);m.play=()=>{m.seekProof.playCalls++;return play()};voices.push(m);return m});a.mount();const p=a.channel('room'),q=a.channel('other');window.seekProof={a,p,q,voices};
document.querySelector('#start').onclick=async()=>{try{await a.unlock();await Promise.all([p.track('bg',['/long.wav'],{loop:true,loopStartSeconds:2,playbackID:'song'}),q.track('bg',['/long.wav'],{loop:true,playbackID:'other'})]);seekProof.bg=voices.find(m=>m.dataset.platformTrack==='bg');seekProof.other=voices.filter(m=>m.dataset.platformTrack==='bg')[1];seekProof.started=true;document.querySelector('#status').textContent='媒体已就绪'}catch(e){seekProof.error=String(e)}};
seekProof.ready=true;
</script></html>''')
    class Handler(http.server.SimpleHTTPRequestHandler):
        def log_message(self,*_):pass
        def do_GET(self):
            key=self.path.removeprefix('/').removesuffix('.wav')
            if key in gates:
                requested[key].set();gates[key].wait(20)
                self.path='/long.wav'
            # Like the immutable asset server, expose actual byte-range seeking.
            # SimpleHTTPRequestHandler alone does not implement Range.
            if self.path in ('/long.wav','/short.wav'):
                data=(temp/self.path[1:]).read_bytes();start,end=0,len(data)-1
                header=self.headers.get('Range');partial=False
                if header:
                    match=re.fullmatch(r'bytes=(\d+)-(\d*)',header)
                    if not match:self.send_error(416);return
                    start=int(match[1]);end=min(end,int(match[2])if match[2]else end);partial=True
                    if start>end:self.send_error(416);return
                self.send_response(206 if partial else 200);self.send_header('Content-Type','audio/wav');self.send_header('Accept-Ranges','bytes');self.send_header('Content-Length',str(end-start+1))
                if partial:self.send_header('Content-Range',f'bytes {start}-{end}/{len(data)}')
                self.end_headers()
                try:self.wfile.write(data[start:end+1])
                except(BrokenPipeError,ConnectionResetError):pass
                return
            try:super().do_GET()
            except(BrokenPipeError,ConnectionResetError):pass
    server=http.server.ThreadingHTTPServer(('127.0.0.1',0),functools.partial(Handler,directory=temp));threading.Thread(target=server.serve_forever,daemon=True).start()
    base=f'http://127.0.0.1:{server.server_port}'
    browser('open',base);opened=True;browser('wait','--fn','window.seekProof?.ready===true');browser('snapshot','-i');browser('click','#start');browser('wait','--fn','seekProof.started||seekProof.error')
    browser('wait','--fn','seekProof.bg.seekable.length>0&&seekProof.bg.seekable.end(0)>=8')
    report['seekRequest']=js("(()=>{const t=seekProof,m=t.bg;t.seekSource=m.currentSrc;t.seekCalls=m.seekProof.playCalls;t.seekAt=performance.now();t.seekAccepted=t.p.seekTrack('bg',15);return {accepted:t.seekAccepted,position:m.currentTime,seeking:m.seeking,readyState:m.readyState,seekable:Array.from({length:m.seekable.length},(_,i)=>[m.seekable.start(i),m.seekable.end(i)])}})()")
    browser('wait','--fn','!seekProof.bg.seeking&&seekProof.bg.currentTime>=3&&seekProof.bg.currentTime<3.5')
    check('active suffix seek keeps source, element and native play count', '''(()=>{const t=seekProof,m=t.bg,elapsed=performance.now()-t.seekAt;return {passed:t.seekAccepted&&elapsed<1000&&!m.paused&&t.seekSource===m.currentSrc&&t.seekCalls===m.seekProof.playCalls,position:m.currentTime,duration:m.duration,playCalls:m.seekProof.playCalls,elapsedMS:elapsed}})()''')
    js("(()=>{const t=seekProof,m=t.bg;t.p.muteTrack('bg',true);t.p.pauseTrack('bg');t.pauseSeekCalls=m.seekProof.playCalls;t.pauseSeekAccepted=t.p.seekTrack('bg',4.25);return true})()")
    browser('wait','--fn','!seekProof.bg.seeking&&Math.abs(seekProof.bg.currentTime-4.25)<.01')
    check('paused soft-muted seek remains silent and does not resume', '''(()=>{const t=seekProof,m=t.bg;return {passed:t.pauseSeekAccepted&&m.paused&&m.muted&&Math.abs(m.currentTime-4.25)<.01&&t.pauseSeekCalls===m.seekProof.playCalls,position:m.currentTime,paused:m.paused,muted:m.muted}})()''')
    check('owner isolation and stable playback identity', '''(async()=>{const t=seekProof,m=t.bg,calls=m.seekProof.playCalls,count=t.voices.length;await t.p.track('bg',['/long.wav'],{loop:true,loopStartSeconds:2,playbackID:'song',startOffset:0});const ok=t.q.seekTrack('bg',1.5);return {passed:ok&&t.voices.length===count&&m.currentTime===4.25&&m.paused&&m.seekProof.playCalls===calls&&Math.abs(t.other.currentTime-1.5)<.1,source:m.currentSrc,voices:count}})()''')
    js("(()=>{const t=seekProof;t.pending=t.p.track('delayed',['/delayed.wav'],{loop:true,loopStartSeconds:2,playbackID:'delayed'}).then(ok=>{t.pendingDone=ok});t.delayed=t.voices.at(-1);return true})()")
    assert requested['delayed'].wait(10),'native delayed request did not start'
    check('unready native metadata accepts only the latest queued seek', '''(()=>{const t=seekProof,m=t.delayed;return {passed:m.readyState===0&&t.p.seekTrack('delayed',12)&&t.p.seekTrack('delayed',15),readyState:m.readyState}})()''')
    gates['delayed'].set();browser('wait','--fn','seekProof.pendingDone!==undefined&&seekProof.delayed.readyState>=1')
    check('metadata applies elapsed suffix seek without another play', '''(()=>{const t=seekProof,m=t.delayed;return {passed:t.pendingDone&&m.currentTime>=3&&m.currentTime<3.5&&m.seekProof.playCalls===1,position:m.currentTime,playCalls:m.seekProof.playCalls}})()''')
    js("(()=>{const t=seekProof;t.p.track('cancelled',['/cancelled.wav'],{loop:true,playbackID:'cancelled'}).then(ok=>t.cancelledDone=ok);t.cancelled=t.voices.at(-1);return true})()")
    assert requested['cancelled'].wait(10)
    check('stopping a pending seek cannot recreate the lane', '''(()=>{const t=seekProof,m=t.cancelled,accepted=t.p.seekTrack('cancelled',6);t.p.stopTrack('cancelled');return {passed:accepted&&!t.p.seekTrack('cancelled',2)&&m.paused&&!m.isConnected,playCalls:m.seekProof.playCalls}})()''')
    gates['cancelled'].set();browser('wait','--fn','seekProof.cancelledDone!==undefined')
    check('late native cancellation stays stopped', '''(()=>{const t=seekProof,m=t.cancelled;return {passed:t.cancelledDone===false&&m.paused&&!m.isConnected&&!m.getAttribute('src'),playCalls:m.seekProof.playCalls}})()''')
    check('invalid seek leaves active media unchanged', '''(()=>{const t=seekProof,m=t.bg,calls=m.seekProof.playCalls,position=m.currentTime;const results=[t.p.seekTrack('bg',-1),t.p.seekTrack('bg',Infinity),t.p.seekTrack('missing',3)];return {passed:results.every(v=>!v)&&m.currentTime===position&&m.seekProof.playCalls===calls,results}})()''')
    js("seekProof.p.track('finite',['/short.wav'],{playbackID:'finite'}).then(ok=>seekProof.finiteReady=ok)")
    browser('wait','--fn',"seekProof.finiteReady&&seekProof.voices.find(m=>m.dataset.platformTrack==='finite').ended")
    check('completed one-shot cannot be revived by seek', '''(()=>{const t=seekProof,m=t.voices.find(m=>m.dataset.platformTrack==='finite'),calls=m.seekProof.playCalls;return {passed:!t.p.seekTrack('finite',0)&&m.paused&&m.seekProof.playCalls===calls,ended:m.ended}})()''')
    check('global mute and disposal reject seeks without unmuting or recreating media', '''(()=>{const t=seekProof;t.a.setEnabled(false);const count=t.voices.length,muted=t.p.seekTrack('bg',1);t.a.setEnabled(true);const stopped=t.p.seekTrack('bg',1);t.a.dispose();const disposed=t.p.seekTrack('bg',1);return {passed:!muted&&!stopped&&!disposed&&count===t.voices.length&&t.voices.every(m=>m.paused),muted,stopped,disposed}})()''')
    js("document.querySelector('#status').textContent='校准验收通过：活跃轨定位、暂停静音保持、延迟元数据、取消隔离、已结束音效不复活';true")
    browser('set','viewport','1100','780');browser('screenshot',str(out/'desktop.png'));browser('set','viewport','320','740');browser('screenshot',str(out/'mobile-320.png'))
    assert sources=={str(p.relative_to(root)):hashlib.sha256(p.read_bytes()).hexdigest()for p in files},'source changed during native acceptance'
    report['passed']=True
except BaseException:
    report['error']=traceback.format_exc()
    if opened:
        try:report['browserErrors']=browser('errors');report['page']=js('({url:location.href,text:document.body.innerText})')
        except BaseException:pass
finally:
    for gate in gates.values():gate.set()
    if opened:
        try:browser('close')
        except BaseException:report['passed']=False
    if server:server.shutdown();server.server_close()
    for path in [temp,sockets]:
        if path.exists():shutil.rmtree(path)
    (out/'result.json').write_text(json.dumps(report,ensure_ascii=False,indent=2)+'\n');print(json.dumps(report,ensure_ascii=False,indent=2))
raise SystemExit(0 if report['passed']else 1)
