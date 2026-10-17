"""Actual browser-Wasm/native protocol parity and session/restore acceptance."""
import json,os,subprocess,threading,uuid,hashlib
from pathlib import Path
from http.server import ThreadingHTTPServer,BaseHTTPRequestHandler

ROOT=Path(__file__).resolve().parents[2]
OUT=ROOT/'.local/basketball/wasm-runtime';OUT.mkdir(parents=True,exist_ok=True)
env=dict(os.environ,AGENT_BROWSER_SESSION='bkw-'+uuid.uuid4().hex[:6],AGENT_BROWSER_SOCKET_DIR=str(ROOT/'.local/bkw-sockets'),TMPDIR=str(ROOT/'.local/tmp'))
Path(env['AGENT_BROWSER_SOCKET_DIR']).mkdir(exist_ok=True)
requests=[]
def browser(*args):
    r=subprocess.run(['agent-browser',*args,'--json'],env=env,capture_output=True,text=True,timeout=60)
    if r.returncode:raise RuntimeError(r.stdout+r.stderr)
    data=json.loads(r.stdout);assert data['success'],data
    return data['data']
def js(source):return browser('eval',source)['result']
class Handler(BaseHTTPRequestHandler):
    def log_message(self,*args):pass
    def do_GET(self):
        requests.append(self.path)
        files={'/':ROOT/'scripts/basketball/runtime_wasm_test_page.html','/harness.js':OUT/'harness.js','/runtime/wasm_exec.js':OUT/'wasm_exec.js','/runtime/basketball.wasm':OUT/'basketball.wasm'}
        path=files.get(self.path)
        if path is None:self.send_error(404);return
        data=path.read_bytes();self.send_response(200);self.send_header('Content-Type','application/wasm' if path.suffix=='.wasm' else 'text/html' if path.suffix=='.html' else 'text/javascript');self.send_header('Content-Length',str(len(data)));self.end_headers();self.wfile.write(data)

def main():
    subprocess.run(['go','build','-o',str(OUT/'runtime-stdio'),'./games/basketball/cmd/runtime-stdio'],cwd=ROOT,env=env,check=True)
    subprocess.run(['go','build','-o',str(OUT/'basketball.wasm'),'./games/basketball/cmd/browser-wasm'],cwd=ROOT,env=dict(env,GOOS='js',GOARCH='wasm'),check=True)
    goroot=subprocess.check_output(['go','env','GOROOT'],cwd=ROOT,env=env,text=True).strip()
    (OUT/'wasm_exec.js').write_bytes((Path(goroot)/'lib/wasm/wasm_exec.js').read_bytes())
    subprocess.run([str(ROOT/'web/node_modules/.bin/esbuild'),str(ROOT/'scripts/basketball/runtime_wasm_harness.ts'),'--bundle','--format=esm','--target=es2022','--outfile='+str(OUT/'harness.js')],env=env,check=True)
    native=subprocess.Popen([str(OUT/'runtime-stdio')],cwd=ROOT,env=env,text=True,stdin=subprocess.PIPE,stdout=subprocess.PIPE,stderr=(OUT/'native.stderr').open('w'))
    server=ThreadingHTTPServer(('127.0.0.1',0),Handler);thread=threading.Thread(target=server.serve_forever,daemon=True);thread.start();opened=False
    transcript=[]
    def call(command):
        raw=json.dumps(command,separators=(',',':'));native.stdin.write(raw+'\n');native.stdin.flush()
        expected=json.loads(native.stdout.readline())
        observed=js('JSON.parse(window.basketballWasm('+json.dumps(raw)+'))')
        assert observed==expected,'native/Wasm divergence for '+command['op']
        transcript.append(dict(command=command,response=observed));return observed
    try:
        browser('--profile',str(OUT/'profile'),'open','http://127.0.0.1:'+str(server.server_port));opened=True
        browser('wait','--fn','!!window.runtimeEngine')
        defaults=call(dict(op='defaults'));assert defaults['ok']
        config=defaults['data'];config['match']['Seed']=0x12345678
        a=call(dict(op='create',config=config));b=call(dict(op='create',config=config));assert a['ok'] and b['ok'] and a['data']['session']!=b['data']['session']
        aid,bid=a['data']['session'],b['data']['session']
        step=call(dict(op='advance',session=aid,ticks=210,held=[0,0,0,0]));assert step['ok'],step
        sounds=step['data'].get('soundIDs')
        assert isinstance(sounds,list) and sounds[:3]==[253,255,66] and 55 in sounds,'sound IDs must be numeric events, never a byte-string encoding'
        assert step['data']['state']['playing'] is not None
        unchanged=call(dict(op='snapshot',session=bid));assert unchanged['data']['checkpoint']['state']['videoFrames']==0
        checkpoint=call(dict(op='snapshot',session=aid))['data']['checkpoint']
        rejected=call(dict(op='advance',session=aid,ticks=-1,held=[0,0,0,0]));assert not rejected['ok']
        assert call(dict(op='snapshot',session=aid))['data']['checkpoint']==checkpoint
        bad=json.loads(json.dumps(checkpoint));bad['state']['playing']['match']['Clock']['Tens']=9
        assert not call(dict(op='restore',checkpoint=bad))['ok']
        restored=call(dict(op='restore',checkpoint=checkpoint));assert restored['ok'];rid=restored['data']['session'];assert not restored['data'].get('soundIDs')
        left=call(dict(op='advance',session=aid,ticks=18,held=[1,0,0,0]));right=call(dict(op='advance',session=rid,ticks=18,held=[1,0,0,0]));assert left['ok'] and right['ok']
        assert left['data']['state']==right['data']['state']
        latched=call(dict(op='advance',session=aid,ticks=1,held=[64,128,0,0]));assert latched['ok']
        cancel=call(dict(op='cancelInput',session=aid,controller=0));assert cancel['ok'],cancel
        state=cancel['data']['state'];prior=latched['data']['state']
        assert state['held'][0]==state['pendingPressed'][0]==0
        assert state['held'][1]==prior['held'][1] and state['pendingPressed'][1]==prior['pendingPressed'][1]
        assert state['videoFrames']==prior['videoFrames'] and state['playing']==prior['playing']
        assert not call(dict(op='cancelInput',session=aid,controller=4))['ok']
        latched=call(dict(op='advance',session=aid,ticks=1,held=[65,128,0,0]));assert latched['ok']
        partial=call(dict(op='cancelInput',session=aid,controller=0,buttons=64));assert partial['ok']
        assert partial['data']['state']['held'][0]==1 and partial['data']['state']['pendingPressed'][0]&64==0
        # Explicit result-scene boundary fixture, not proof of a whole match.
        # A large batch must return the confirmed match instead of rolling it
        # back after trying to enter an unsupported campaign/menu destination.
        result_state=json.loads(json.dumps(b['data']['state']))
        scene=result_state.pop('opening');transition=scene['Transition']
        result_state['display']=json.loads(json.dumps(checkpoint['state']['display']))
        camera=result_state['display']['Visible']['camera'];transition['CameraScroll']=[camera['x'],camera['y'],camera['x'],camera['y']]
        transition['Match']['Phase']=6;transition['Match']['Period']=2
        transition['Scores']=[0,12];transition['Match']['DisplayedScores']=[[0,0,0],[0,1,2]]
        for obj in transition['Objects']:obj['Pose']={'frame':127,'mirrored':False}
        scene['Projection']['drawOrder'][:12]=[255]*12;scene['Projection']['drawCount']=0
        result_state['transition']={'scene':scene};result_state['logicSteps']=1
        boundary=call(dict(op='restore',checkpoint=dict(version=1,state=result_state)));assert boundary['ok'],boundary
        result_id=boundary['data']['session']
        terminal=call(dict(op='advance',session=result_id,ticks=600,held=[0,0,0,0]));assert terminal['ok'],terminal
        final=terminal['data']['state'];outcome=final['outcome']
        assert outcome['confirmed'] and outcome['winningTeam']==1 and outcome['scores']==[0,12]
        assert outcome['nextScene']==7 and outcome['nextPhase']==0
        assert 0<final['videoFrames']<600 and terminal['data']['logicUpdates']>0
        assert terminal['data'].get('soundIDs'), 'completion batch discarded result audio'
        expected_display=json.loads(json.dumps(final['display']['Submitted']))
        expected_display['hud']=final['display']['HUD'];expected_display['ballCHR']=final['display']['BallCHR']
        assert final['display']['Visible']==expected_display,'last submitted result was not made visible'
        again=call(dict(op='advance',session=result_id,ticks=600,held=[255,0,0,0]));assert again['ok'],again
        assert again['data']['state']==final and again['data']['logicUpdates']==0 and not again['data'].get('soundIDs')
        terminal_checkpoint=call(dict(op='snapshot',session=result_id))['data']['checkpoint']
        terminal_restore=call(dict(op='restore',checkpoint=terminal_checkpoint));assert terminal_restore['ok'],terminal_restore
        final_id=terminal_restore['data']['session']
        stable=call(dict(op='advance',session=final_id,ticks=600));assert stable['ok'] and stable['data']['state']==final
        assert not call(dict(op='advance',session=final_id,ticks=601))['ok'], 'terminal states still validate requests'
        for sid in (aid,bid,rid,result_id,final_id):
            assert call(dict(op='dispose',session=sid))['ok']
            assert not call(dict(op='advance',session=sid))['ok']
        assert not call(dict(op='unknown'))['ok']
        assert not js('JSON.parse(window.basketballWasm("{"))')['ok']
        assert not any('.nes' in path or 'fixture' in path or 'testdata' in path for path in requests)
        (OUT/'result.json').write_text(json.dumps(dict(status='passed',requests=len(transcript),native_and_browser_wasm_identical=True,independent_sessions=True,restore_continues_same_state=True,confirmed_match_boundary=True,result_boundary_is_synthetic=True,fixture_or_rom_fetches=False,wasm_sha256=hashlib.sha256((OUT/'basketball.wasm').read_bytes()).hexdigest()),indent=2)+'\n')
        browser('screenshot',str(OUT/'browser.png'))
        print((OUT/'result.json').read_text())
    finally:
        (OUT/'transcript.json').write_text(json.dumps(transcript,indent=2)+'\n');(OUT/'requests.json').write_text(json.dumps(requests,indent=2)+'\n')
        if opened:browser('close')
        native.stdin.close();native.wait(timeout=10)
        server.shutdown();server.server_close();thread.join(timeout=5)

if __name__=='__main__':main()
