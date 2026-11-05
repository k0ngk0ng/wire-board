"""Test-first browser E2E for exact-byte schema2 local Wasm RPC and UI.

Run: PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/check_runtimebridge_schema2.py
All mutable files stay in the current worktree. This starts only loopback HTTP.
"""
from pathlib import Path
import hashlib,json,os,subprocess,threading
from http.server import ThreadingHTTPServer
from serve_local_player import build,PlayerHandler,ROOT

OUT=ROOT/'games/yuyuhakusho/verification/client-bridge-schema2'
TMP=ROOT/'.local/md-yuyuhakusho/client-bridge-browser'
for path in (OUT,TMP,ROOT/'.local/yycb'):path.mkdir(parents=True,exist_ok=True)
env=dict(os.environ,TMPDIR=str(TMP),AGENT_BROWSER_SOCKET_DIR=str(ROOT/'.local/yycb'),AGENT_BROWSER_SESSION='yybridge2')
class Handler(PlayerHandler):
    corrupt=False
    def do_GET(self):
        if self.corrupt and self.path=='/hud.json.gz':
            self.requested_paths.append(self.path)
            data=(ROOT/'games/yuyuhakusho/data/hud-assets.json.gz').read_bytes()+b'\x00'
            self.send_response(200);self.send_header('Content-Type','application/octet-stream');self.send_header('Content-Length',str(len(data)));self.end_headers();self.wfile.write(data)
        else:super().do_GET()
build()
server=ThreadingHTTPServer(('127.0.0.1',0),Handler)
thread=threading.Thread(target=server.serve_forever,daemon=True);thread.start()
url=f'http://127.0.0.1:{server.server_port}/'
def browser(*args):
    result=subprocess.run(['agent-browser',*args,'--json'],cwd=ROOT,env=env,capture_output=True,text=True,timeout=55)
    if result.returncode:raise RuntimeError(result.stdout+result.stderr)
    data=json.loads(result.stdout)
    if not data.get('success'):raise RuntimeError(data)
    return data.get('data')
def js(expression):return browser('eval','-b',__import__('base64').b64encode(expression.encode()).decode())['result']
def endpoint():
    data=browser('get','cdp-url');return data.get('url')or data.get('cdpUrl')or data.get('value')
def activate():
    r=subprocess.run(['node',str(ROOT/'scripts/yuyuhakusho/activate_audio_media_page.mjs'),endpoint(),url],cwd=ROOT,env=env,capture_output=True,text=True,timeout=25)
    if r.returncode:raise RuntimeError(r.stdout+r.stderr)
    return json.loads(r.stdout)
def native(actions):
    r=subprocess.run(['node',str(ROOT/'scripts/yuyuhakusho/native_player_input.mjs'),endpoint(),url,json.dumps(actions)],cwd=ROOT,env=env,capture_output=True,text=True,timeout=25)
    if r.returncode:raise RuntimeError(r.stdout+r.stderr)
    return json.loads(r.stdout)
def key(letter,down,held):return {'method':'Input.dispatchKeyEvent','type':'keyDown'if down else'keyUp','key':letter,'code':'Key'+letter.upper(),'windowsVirtualKeyCode':ord(letter.upper()),'expect':held}
def rect(selector):return js('(()=>{const r=document.querySelector('+json.dumps(selector)+').getBoundingClientRect();return{x:r.x,y:r.y,w:r.width,h:r.height}})()')
def center(r):return {'x':r['x']+r['w']/2,'y':r['y']+r['h']/2}
opened=False;report={}
try:
    browser('--profile',str(TMP/'profile'),'open',url);opened=True
    browser('set','viewport','1120','900')
    report['native_activation']=activate()
    browser('wait','--fn','window.yyResearch?.assetsReady === true')
    browser('click','#start-match')
    browser('wait','--fn','window.yyResearch.view?.tick > 3')
    schema=js('JSON.parse(JSON.parse(window.yuyuhakushoWasm(JSON.stringify({Op:"save"}))).snapshot).schema')
    assert schema==2,('local page did not use exact-byte network assets',schema)
    report['initial_preview']=js('window.yyResearch.preview')
    assert report['initial_preview']['unchanged'] and report['initial_preview']['authorityFrame']=='0' and report['initial_preview']['predictedFrame']=='1'
    paths=list(Handler.requested_paths)
    assert '/render-meta.json.gz'in paths and'/resource-descriptor.json'in paths
    assert not any(path in paths for path in ['/arena-a.json.gz','/arena-a-aux.json.gz','/arena-2.json.gz','/arena-4.json.gz','/arena-6.json.gz','/arena-8.json.gz','/arena-c.json.gz','/arena-e.json.gz'])
    report['rpc']=js(r'''(()=>{
      const invoke=(request,...extra)=>JSON.parse(window.yuyuhakushoWasm(JSON.stringify(request),...extra));
      const call=(request,...extra)=>{const r=invoke(request,...extra);if(!r.ok)throw Error(r.error);return r};
      const ensure=(v,m)=>{if(!v)throw Error(m)};
      const pixels=new Uint8Array(320*224*4),sig=()=>{let n=2166136261;for(const b of pixels)n=Math.imul(n^b,16777619);return n>>>0};
      const options={Arena:0,Seed:0x12345678,Difficulty:6,Players:[{Slot:0,CharacterID:7,Color:0,HumanChannel:0},{Slot:3,CharacterID:9,Color:1,HumanChannel:1}]};
      call({Op:'prepare',Options:options});const before=call({Op:'save'}).snapshot;
      const preview=call({Op:'preview'},pixels);ensure(call({Op:'save'}).snapshot===before,'preview advanced authority');
      ensure(preview.view.frame==='0'&&preview.preview.frame==='1'&&sig()!==0,'preview lacks detached predicted frame');
      const saved=JSON.parse(before);ensure(saved.schema===2&&!('tile_slots'in saved)&&!('hud'in saved)&&saved.presentation,'legacy mutable presentation owner retained');
      call({Op:'ready'});for(let i=0;i<320;i++)call({Op:'step',Held:[]},pixels);
      const epoch='0123456789abcdef01234567',checkpoint=call({Op:'checkpoint',Epoch:epoch}).checkpoint;
      ensure(typeof checkpoint==='string'&&checkpoint.includes('18446744073709551615'),'actual MaxUint64 not exercised');
      const state=call({Op:'save'}).snapshot;call({Op:'restore-checkpoint',Checkpoint:checkpoint});
      ensure(call({Op:'checkpoint',Epoch:epoch}).checkpoint===checkpoint&&call({Op:'save'}).snapshot===state,'opaque restore lost integer bits');
      const trace=[];for(let i=0;i<30;i++){call({Op:'step',Held:[8]},pixels);trace.push([sig(),call({Op:'save'}).snapshot]);}
      call({Op:'restore-checkpoint',Checkpoint:checkpoint});for(let i=0;i<30;i++){call({Op:'step',Held:[8]},pixels);ensure(sig()===trace[i][0]&&call({Op:'save'}).snapshot===trace[i][1],'replay diverged at'+i);}
      const stable=call({Op:'save'}).snapshot;
      const lossy=invoke({Op:'restore-checkpoint',Checkpoint:JSON.stringify(JSON.parse(checkpoint))});
      ensure(!lossy.ok&&call({Op:'save'}).snapshot===stable,'lossy JS checkpoint accepted or mutated session');
      const wrongTarget=invoke({Op:'preview'},new Uint8Array(4));ensure(!wrongTarget.ok&&call({Op:'save'}).snapshot===stable,'bad preview target committed');
      const big=checkpoint.replace(/"frame":"\d+"/,'"frame":"9007199254741011"');
      ensure(call({Op:'restore-checkpoint',Checkpoint:big}).view.frame==='9007199254741011','high frame narrowed');
      ensure(call({Op:'step',Held:[]},pixels).view.frame==='9007199254741012','high frame did not advance exactly');
      call({Op:'restore-checkpoint',Checkpoint:checkpoint});
      return{schema:2,persistentOwners:Object.keys(saved),preview:{authorityFrame:preview.view.frame,predictedFrame:preview.preview.frame,unchanged:true},maxUint64Present:true,opaqueBytes:checkpoint.length,exactRoundtrip:true,replayedFrames:30,lossyRejected:lossy.error,highFrame:'9007199254741012',descriptor:saved.resource_descriptor_sha256};
    })()''')
    report['asset_atomic']=js(r'''(async()=>{
      const invoke=(r,...b)=>JSON.parse(window.yuyuhakushoWasm(JSON.stringify(r),...b));
      const unzip=async(path)=>new Uint8Array(await new Response(new Blob([await(await fetch(path)).arrayBuffer()]).stream().pipeThrough(new DecompressionStream('gzip'))).arrayBuffer());
      const values=await Promise.all(['/rules.json.gz','/sprites.json.gz','/arena.json.gz','/hud.json.gz'].map(unzip));
      values.push(new Uint8Array(),await unzip('/render-meta.json.gz'),new Uint8Array(await(await fetch('/resource-descriptor.json')).arrayBuffer()));
      const before=invoke({Op:'save'}).snapshot;
      values[0]=new TextEncoder().encode(JSON.stringify(JSON.parse(new TextDecoder().decode(values[0]))));
      const result=invoke({Op:'load-v2',Options:{Arena:0}},...values);
      if(result.ok||invoke({Op:'save'}).snapshot!==before)throw Error('normalized raw resources accepted or cleared live session');
      const count=invoke({Op:'load-v2',Options:{Arena:0}},values[0]);if(count.ok)throw Error('incomplete byte tuple accepted');
      return{normalizedRejected:result.error,arityRejected:count.error,sessionUnchanged:true};
    })()''')
    browser('click','#start-match');browser('wait','--fn','window.yyResearch.view?.tick > 260')
    before=js('({x:window.yyResearch.view.players[0].X,pixel:window.yyResearch.pixelSignature})')
    report['keyboard_down']=native([key('d',True,['right'])]);browser('wait','--fn','window.yyResearch.view.players[0].X > '+str(before['x']+8));native([key('d',False,[])])
    after=js('({x:window.yyResearch.view.players[0].X,pixel:window.yyResearch.pixelSignature})');assert after['x']>before['x'] and after['pixel']!=before['pixel']
    report['keyboard_movement']={'before':before,'after':after}
    native([key('u',True,['x']),key('o',True,['x','z']),key('u',False,['z']),key('o',False,[])])
    report['visibility_release']=native([key('j',True,['b']),{'method':'acceptance:hide','expect':[]},key('j',False,[])])
    browser('screenshot',str(OUT/'desktop.png'))
    browser('set','viewport','320','740');browser('scrollintoview','[data-controller-layout="md"]')
    assert not js('document.documentElement.scrollWidth > innerWidth')
    canvas,pad=rect('canvas'),rect('[data-controller-layout="md"]');assert pad['y']>=canvas['y']+canvas['h'],(canvas,pad)
    left,z,c=map(lambda s:center(rect(s)),['[data-controller-button="left"]','[data-controller-button="z"]','[data-controller-button="c"]'])
    report['mobile_touch']=native([{'type':'touchStart','touchPoints':[{'id':1,**left},{'id':2,**z},{'id':3,**c}],'expect':['c','left','z']},{'type':'touchCancel','touchPoints':[],'expect':[]}])
    assert js('window.yyResearch.trusted.length>0&&window.yyResearch.trusted.every(Boolean)')
    assert js('window.yyResearch.errors.length')==0,js('window.yyResearch.errors')
    browser('screenshot',str(OUT/'mobile-320.png'))
    browser('select','#arena-select','10');browser('click','#start-match');browser('wait','--fn','window.yyResearch.view?.arena === 10 && window.yyResearch.view?.tick > 3')
    assert '/arena-a.json.gz'in Handler.requested_paths and'/arena-a-aux.json.gz'in Handler.requested_paths
    assert not any(path in Handler.requested_paths for path in ['/arena-2.json.gz','/arena-4.json.gz','/arena-6.json.gz','/arena-8.json.gz','/arena-c.json.gz','/arena-e.json.gz'])
    report['on_demand_arenas']=[p for p in Handler.requested_paths if'/arena'in p]
    Handler.corrupt=True;browser('reload');activate();browser('wait','--fn','window.yyResearch?.errors.length>0')
    errors=js('window.yyResearch.errors');assert any('压缩资源校验失败'in error for error in errors),errors
    assert not js('window.yyResearch.assetsReady')and js('document.querySelector("#start-match").disabled')
    report['gzip_corruption_blocked']=errors
    report.update(pass_=True,scope='exact-byte schema2 local research client; no network latency or audio acceptance',native_inputs_trusted=True,requests=list(Handler.requested_paths))
    report['pass']=report.pop('pass_')
    sources=[ROOT/'scripts/yuyuhakusho/check_runtimebridge_schema2.py',ROOT/'scripts/yuyuhakusho/local_player.tsx',ROOT/'scripts/yuyuhakusho/serve_local_player.py',ROOT/'games/yuyuhakusho/cmd/local-player-wasm/main_js.go',ROOT/'games/yuyuhakusho/runtimebridge/session.go',ROOT/'games/yuyuhakusho/runtimebridge/serialization.go',ROOT/'games/yuyuhakusho/runtimebridge/network_assets.go']
    report['source_sha256']={str(p.relative_to(ROOT)):hashlib.sha256(p.read_bytes()).hexdigest()for p in sources}
    (OUT/'result.json').write_text(json.dumps(report,ensure_ascii=False,indent=2)+'\n');print(json.dumps(report,ensure_ascii=False))
except Exception as error:
    failure={'error':repr(error),'completed':report,'requests':list(Handler.requested_paths)}
    (TMP/'failure.json').write_text(json.dumps(failure,ensure_ascii=False,indent=2)+'\n')
    print(json.dumps(failure,ensure_ascii=False),flush=True)
    if opened:
        try:browser('screenshot',str(TMP/'failure.png'))
        except Exception:pass
    raise
finally:
    if opened:browser('close')
    server.shutdown();server.server_close();thread.join(timeout=5)
