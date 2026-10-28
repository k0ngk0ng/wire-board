"""Verify real GameAudioPort.track suffix-loop boundaries with original stems.

This is native HTMLAudioElement state/clock evidence, not speaker recording,
sample-accurate multitrack synchronization or subjective listening acceptance.
"""
import argparse
import functools
import hashlib
import http.server
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import threading
import traceback
import urllib.parse
import uuid

ROOT=Path(__file__).resolve().parents[2]
BASE=ROOT/'.local/contra-audio-loops'


def main():
    parser=argparse.ArgumentParser();parser.add_argument('--label',default='v1');args=parser.parse_args()
    assert re.fullmatch(r'[a-z0-9-]+',args.label)
    out=BASE/f'browser-{args.label}';assert not out.exists(),'use a new label to retain evidence'
    temp=out/'temporary';temp.mkdir(parents=True)
    run='cl'+uuid.uuid4().hex[:6];sockets=ROOT/'.s'/run;sockets.mkdir(parents=True)
    env=dict(os.environ,AGENT_BROWSER_SESSION=run,AGENT_BROWSER_SOCKET_DIR=str(sockets),
             AGENT_BROWSER_PROFILE=str(temp/'profile'),AGENT_BROWSER_ARGS='--mute-audio,--disable-breakpad,--disable-crash-reporter',TMPDIR=str(temp/'tmp'))
    Path(env['TMPDIR']).mkdir()
    manifest_path=ROOT/'games/contra/assets/audio-loops.json';manifest=json.loads(manifest_path.read_text())
    assets={Path(stem['file']).name:ROOT/stem['file'] for song in manifest['songs'] for stem in song['stems']}
    fixture={'songs':[{'id':song['id'],'loopStart':song['loopStartSeconds'],'duration':song['decodedSamples']/44100,
                       'period':song['periodSeconds'],'stems':[{'channel':s['channel'],'url':'/media/'+Path(s['file']).name+'?sha='+s['sha256']}for s in song['stems']]}for song in manifest['songs']]}
    track_count=sum(len(s['stems'])for s in fixture['songs'])
    source_before={str(p.relative_to(ROOT)):hashlib.sha256(p.read_bytes()).hexdigest()for p in [ROOT/'web/src/audio.ts',manifest_path,Path(__file__)]}
    report={'passed':False,'scope':f'Real production GameAudioPort.track and Chromium HTMLAudioElement suffix-loop/position/pause/restore behavior with {track_count} original-derived WAV stems; no physical output/PCM capture or sample-accurate multi-track claim.'}
    opened=False;server=None
    def browser(*command):
        r=subprocess.run(['agent-browser',*command,'--json'],cwd=ROOT,env=env,capture_output=True,text=True,timeout=45)
        assert r.returncode==0,r.stdout+r.stderr
        value=json.loads(r.stdout);assert value['success'],value
        return value['data']
    def js(code):return browser('eval',code)['result']
    try:
        build=subprocess.run(['node','--input-type=module','-e',"import{build}from'./web/node_modules/esbuild/lib/main.js';await build({entryPoints:['web/src/audio.ts'],outdir:process.argv[1],bundle:true,format:'esm',platform:'browser'});",str(temp)],cwd=ROOT,env=env,capture_output=True,text=True)
        assert build.returncode==0,build.stderr
        (temp/'fixture.json').write_text(json.dumps(fixture))
        (temp/'index.html').write_text(r'''<!doctype html><html lang="zh-CN"><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>魂斗罗循环分轨验收</title><style>body{background:#f4f3ed;color:#172626;font:16px system-ui;padding:22px}button{font:inherit;padding:12px;color:#172626;background:white;border:1px solid #687878}pre{white-space:pre-wrap;overflow-wrap:anywhere}</style><h1>魂斗罗循环分轨验收</h1><button id="start">开始静默原生媒体检查</button><pre id="status">就绪</pre><script type="module">
import {GameAudio} from './audio.js';
const platform=new GameAudio();platform.mount();const port=platform.channel('contra-loop-verification');
const fixture=await fetch('./fixture.json').then(r=>r.json());
const state=window.loopProbe={ready:true,running:false,completed:false,error:'',reports:[],platform,port};
const status=document.querySelector('#status');
const until=async(fn,ms=6000)=>{const end=performance.now()+ms;while(!fn()){if(performance.now()>end)throw Error('media condition timed out');await new Promise(r=>setTimeout(r,10));}};
const media=()=>[...document.querySelectorAll('audio[data-platform-track]')];
function watch(m,song){const record={channel:m.dataset.platformTrack,ends:0,events:[],samples:[],maxBoundaryClockDebtMS:0};
 let last=null,raf;for(const name of ['ended','playing','pause','waiting','stalled','seeking','seeked','error'])m.addEventListener(name,()=>{if(name==='ended')record.ends++;record.events.push({event:name,at:performance.now(),time:m.currentTime,paused:m.paused,readyState:m.readyState});});
 function sample(){const row={at:performance.now(),time:m.currentTime,paused:m.paused,ended:m.ended,readyState:m.readyState};record.samples.push(row);if(last&&!last.paused&&!row.paused&&last.time>song.duration-.3&&row.time<song.loopStart+.5){const delta=(song.duration-last.time)+(row.time-song.loopStart);record.maxBoundaryClockDebtMS=Math.max(record.maxBoundaryClockDebtMS,0,(row.at-last.at)-delta*1000);}last=row;raf=requestAnimationFrame(sample);}sample();return {record,stop:()=>cancelAnimationFrame(raf)};}
document.querySelector('#start').onclick=async()=>{if(state.running)return;state.running=true;try{
 await platform.unlock();
 for(const song of fixture.songs){
  // Fetch to the private browser cache before timing the boundary. This is
  // test preparation, not an application policy to preload all game music.
  await Promise.all(song.stems.map(s=>fetch(s.url).then(r=>{if(!r.ok)throw Error('asset fetch failed');return r.arrayBuffer();})));
  port.stop();
  const started=await Promise.all(song.stems.map(s=>port.track(s.channel,[s.url],{loop:true,loopStartSeconds:song.loopStart,startOffset:song.duration-.65,playbackID:`loop-${song.id}-${s.channel}`})));
  if(!started.every(Boolean))throw Error('track start rejected '+song.id);
  const players=media();if(players.length!==song.stems.length)throw Error('wrong track count');
  if(players.some(m=>Math.abs(m.duration-song.duration)>1/44100+.00001||m.loop||m.paused))throw Error('native suffix-loop metadata differs');
  const watched=players.map(m=>watch(m,song));
  await until(()=>watched.every(w=>w.record.ends>=1)&&players.every(m=>!m.paused&&m.currentTime>song.loopStart+.10&&m.currentTime<song.loopStart+2));
  const firstTimes=players.map(m=>m.currentTime);
  for(const m of players)port.pauseTrack(m.dataset.platformTrack);
  const paused=players.map(m=>m.currentTime),at=performance.now();await until(()=>performance.now()-at>160);
  if(players.some((m,i)=>!m.paused||Math.abs(m.currentTime-paused[i])>.0001))throw Error('pause did not preserve media position');
  const resumed=await Promise.all(players.map(m=>port.resumeTrack(m.dataset.platformTrack)));if(!resumed.every(Boolean))throw Error('resume rejected');
  await until(()=>players.every((m,i)=>m.currentTime>paused[i]+.08));
  for(const m of players)m.currentTime=song.duration-.30;
  await until(()=>watched.every(w=>w.record.ends>=2)&&players.every(m=>!m.paused&&m.currentTime>song.loopStart+.10&&m.currentTime<song.loopStart+2));
  const secondTimes=players.map(m=>m.currentTime);
  const data={id:song.id,passed:true,loopStart:song.loopStart,duration:song.duration,firstTimes,secondTimes,
    crossTrackClockSpreadMS:Math.max(...firstTimes,...secondTimes)*0,tracks:watched.map(w=>w.record)};
  data.crossTrackClockSpreadMS=Math.max(Math.max(...firstTimes)-Math.min(...firstTimes),Math.max(...secondTimes)-Math.min(...secondTimes))*1000;
  if(data.tracks.some(r=>r.maxBoundaryClockDebtMS>150||r.events.some(e=>e.event==='error')))throw Error('stalled/error native boundary '+song.id);
  watched.forEach(w=>w.stop());
  // A fresh ID must preserve this group's actual prefix. Check all groups,
  // including the much shorter two-track Intro, instead of only the first BGM.
  port.stop();await Promise.all(song.stems.map(s=>port.track(s.channel,[s.url],{loop:true,loopStartSeconds:song.loopStart,playbackID:`prefix-${song.id}-${s.channel}`})));
  await until(()=>media().every(m=>m.currentTime>.03));
  data.prefix=media().map(m=>({key:m.dataset.platformTrack,time:m.currentTime}));
  if(data.prefix.some(m=>m.time>=song.loopStart))throw Error('intro was skipped '+song.id);
  port.stop();data.departure=media().every(m=>m.paused);if(!data.departure)throw Error('stop left active media');
  state.reports.push(data);status.textContent=state.reports.map(r=>`$${r.id.toString(16).toUpperCase()}：${r.tracks.length} 轨，通过；时钟散差 ${r.crossTrackClockSpreadMS.toFixed(3)} ms`).join('\n');
 }
 state.prefix=state.reports.map(r=>({id:r.id,positions:r.prefix}));state.departure=state.reports.every(r=>r.departure);
 state.completed=true;status.textContent=fixture.songs.reduce((n,s)=>n+s.stems.length,0)+' 个分轨完成两次真实边界、暂停恢复和开头检查。\n'+status.textContent;
 }catch(error){state.error=String(error);status.textContent=state.error;}finally{state.running=false;}};
window.addEventListener('beforeunload',()=>platform.dispose());
</script></html>''')
        class Handler(http.server.SimpleHTTPRequestHandler):
            def log_message(self,*_):pass
            def do_GET(self):
                path=urllib.parse.urlparse(self.path).path
                if path.startswith('/media/'):
                    name=path.removeprefix('/media/');target=assets.get(name)
                    if target is None:self.send_error(404);return
                    size=target.stat().st_size;start,end=0,size-1
                    match=re.fullmatch(r'bytes=(\d+)-(\d*)',self.headers.get('Range',''))
                    if match:
                        start=int(match[1]);end=min(size-1,int(match[2])if match[2]else size-1)
                        if start>end:self.send_error(416);return
                    self.send_response(206 if match else 200);self.send_header('Content-Type','audio/wav');self.send_header('Accept-Ranges','bytes')
                    self.send_header('Content-Length',str(end-start+1));self.send_header('Cache-Control','public,max-age=86400,immutable')
                    if match:self.send_header('Content-Range',f'bytes {start}-{end}/{size}')
                    self.end_headers()
                    try:
                        with target.open('rb')as f:
                            f.seek(start);remaining=end-start+1
                            while remaining:
                                block=f.read(min(262144,remaining));self.wfile.write(block);remaining-=len(block)
                    except (BrokenPipeError,ConnectionResetError):pass
                    return
                super().do_GET()
        server=http.server.ThreadingHTTPServer(('127.0.0.1',0),functools.partial(Handler,directory=temp));threading.Thread(target=server.serve_forever,daemon=True).start()
        browser('--profile',str(temp/'profile'),'open',f'http://127.0.0.1:{server.server_port}')
        opened=True;browser('wait','--fn','window.loopProbe?.ready===true');browser('snapshot','-i');browser('click','#start')
        browser('wait','--fn','loopProbe.completed||!!loopProbe.error')
        result=js('({completed:loopProbe.completed,error:loopProbe.error,reports:loopProbe.reports,prefix:loopProbe.prefix,departure:loopProbe.departure})')
        assert result['completed'] and not result['error'],result
        assert len(result['reports'])==len(fixture['songs'])
        report.update(result);report['passed']=True
        browser('set','viewport','1000','760');browser('screenshot',str(out/'native-loop.png'))
    except BaseException:
        report['error']=traceback.format_exc()
        if opened:
            try:
                report['browserErrors']=browser('errors')
                report['browserConsole']=browser('console')
                report['partial']=js('({completed:window.loopProbe?.completed,error:window.loopProbe?.error,reports:window.loopProbe?.reports})')
            except BaseException:pass
    finally:
        if opened:
            try:browser('close')
            except BaseException:report['passed']=False
        if server:server.shutdown();server.server_close()
        for p in (temp,sockets):
            if p.exists():shutil.rmtree(p)
        source_after={str(p.relative_to(ROOT)):hashlib.sha256(p.read_bytes()).hexdigest()for p in [ROOT/'web/src/audio.ts',manifest_path,Path(__file__)]}
        report['sourceSHA256']=source_before
        report['sourcesUnchangedDuringRun']=source_after==source_before
        if source_after!=source_before:report['passed']=False;report['sourceDrift']=source_after
        (out/'result.json').write_text(json.dumps(report,indent=2)+'\n')
        print(json.dumps({'passed':report['passed'],'reports':len(report.get('reports',[])),'output':str(out.relative_to(ROOT))}))
    raise SystemExit(0 if report['passed'] else 1)


if __name__=='__main__':main()
