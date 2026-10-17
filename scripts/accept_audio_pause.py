"""Real HTML media acceptance: owner pause/resume, finite music and cancellation.
Run against a workspace Vite server; all browser state/results stay in workspace.
"""
import json,os,subprocess,sys,uuid
from pathlib import Path
root=Path(__file__).resolve().parents[1];out=root/'.local/basketball/audio-pause-browser';out.mkdir(parents=True,exist_ok=True)
env=dict(os.environ,AGENT_BROWSER_SESSION='ap-'+uuid.uuid4().hex[:6],AGENT_BROWSER_SOCKET_DIR=str(root/'.local/ap-sockets'),TMPDIR=str(root/'.local/tmp'))
Path(env['AGENT_BROWSER_SOCKET_DIR']).mkdir(exist_ok=True)
def browser(*args):
 p=subprocess.run(['agent-browser',*args,'--json'],env=env,cwd=root,capture_output=True,text=True,timeout=40)
 if p.returncode:raise RuntimeError(p.stdout+p.stderr)
 result=json.loads(p.stdout);assert result['success'],result;return result['data']
def js(text):return browser('eval',text)['result']
url=sys.argv[1] if len(sys.argv)>1 else 'http://127.0.0.1:5191'
assert url.startswith('http://127.0.0.1:')
try:
 browser('--profile',str(out/'profile'),'open',url)
 js('''(async()=>{
 const {GameAudio}=await import('/src/audio.ts');const a=new GameAudio(()=>{const m=new Audio();m.dataset.acceptAudio="";return m});a.mount();const p=a.channel('accept');
 const count=8000*8,buffer=new ArrayBuffer(44+count*2),v=new DataView(buffer);const text=(offset,s)=>{for(let i=0;i<s.length;i++)v.setUint8(offset+i,s.charCodeAt(i))};text(0,'RIFF');v.setUint32(4,36+count*2,true);text(8,'WAVEfmt ');v.setUint32(16,16,true);v.setUint16(20,1,true);v.setUint16(22,1,true);v.setUint32(24,8000,true);v.setUint32(28,16000,true);v.setUint16(32,2,true);v.setUint16(34,16,true);text(36,'data');v.setUint32(40,count*2,true);for(let i=0;i<count;i++)v.setInt16(44+i*2,Math.round(Math.sin(i*2*Math.PI*330/8000)*800),true);
 const src=URL.createObjectURL(new Blob([buffer],{type:'audio/wav'}));window.acceptAudio={a,p,src};
 const b=document.createElement('button');b.id='audio-accept';b.textContent='Start platform audio acceptance';b.onclick=async()=>{window.acceptAudio.started=await Promise.all([p.play([src]),p.music([src],{loop:false}),p.loop('accept-loop',[src])]);b.dataset.started='true'};document.body.append(b);return true})()''')
 browser('click','#audio-accept');browser('wait','#audio-accept[data-started="true"]');browser('wait','--fn',"[...document.querySelectorAll('audio[data-accept-audio]')].every(m=>m.currentTime>.1)")
 first=js('''(()=>{const t=window.acceptAudio;t.voices=[...document.querySelectorAll('audio[data-accept-audio]')];if(!t.started.every(Boolean)||t.voices.length!==3)throw Error('native media failed');t.p.pause();t.at=t.voices.map(m=>m.currentTime);return {times:t.at,paused:t.voices.map(m=>m.paused),musicLoop:t.voices.find(m=>m.hasAttribute('data-platform-music')).loop}})()''')
 assert all(first['paused']) and not first['musicLoop'],first
 paused=js('window.acceptAudio.voices.map(m=>m.currentTime)');assert paused==first['times'],(paused,first)
 resumed=js('''(async()=>{const t=window.acceptAudio;const ok=await t.p.resume();return {ok,times:t.voices.map(m=>m.currentTime),paused:t.voices.map(m=>m.paused)}})()''')
 assert resumed['ok'] and not any(resumed['paused']),resumed
 assert all(b>=a and b-a<.3 for a,b in zip(first['times'],resumed['times'])),(first,resumed)
 browser('wait','--fn',"window.acceptAudio.voices.every((m,i)=>m.currentTime>window.acceptAudio.at[i]+.05)");progress=js('window.acceptAudio.voices.map(m=>m.currentTime)');assert all(b>a for a,b in zip(resumed['times'],progress)),progress
 cancelled=js('''(async()=>{const t=window.acceptAudio;t.p.pause();t.a.setEnabled(false);t.a.setEnabled(true);const revived=await t.p.resume();return {revived,paused:t.voices.map(m=>m.paused),sources:t.voices.map(m=>m.getAttribute('src'))}})()''')
 assert not cancelled['revived'] and all(cancelled['paused']) and not any(cancelled['sources']),cancelled
 report=dict(native_media=True,first=first,paused=paused,resumed=resumed,progress=progress,cancelled=cancelled)
 (out/'result.json').write_text(json.dumps(report,indent=2)+'\n');browser('screenshot',str(out/'verified.png'));print(json.dumps(report))
finally:
 browser('close')
