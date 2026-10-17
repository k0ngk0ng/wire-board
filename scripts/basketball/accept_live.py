"""Go/Wasm + original Canvas + native keyboard/touch browser acceptance."""
import argparse,hashlib,json,os,re,subprocess,threading,uuid,base64,time
from pathlib import Path
from http.server import ThreadingHTTPServer,BaseHTTPRequestHandler
from urllib.parse import unquote
from urllib.request import urlopen
from urllib.error import HTTPError

ROOT=Path(__file__).resolve().parents[2]
parser=argparse.ArgumentParser(description=__doc__)
parser.add_argument('--full-match',action='store_true',help='also render a cold complete match through the actual client')
parser.add_argument('--all-computer',action='store_true',help='run the natural complete-match case with all four original computer players')
parser.add_argument('--court',type=int,choices=[0,1,2,3,4,5,6],default=1,help='original recovered court selector; no world-state injection')
parser.add_argument('--carry',action='store_true',help='native pickup, carrying movement, jumping and A/B prop throws on Court3')
parser.add_argument('--running-carry',action='store_true',help='native role-specific running B prop attacks on Court3')
parser.add_argument('--fade',action='store_true',help='observe original opening fade through Go/Wasm and Canvas')
parser.add_argument('--gate',action='store_true',help='reach Court3 gate with native movement and jump input')
parser.add_argument('--spring',action='store_true',help='also reach Court2 spring using only native controller input')
ARGS=parser.parse_args()
if ARGS.all_computer and not ARGS.full_match:parser.error('--all-computer requires --full-match')
if ARGS.spring and ARGS.court!=2:parser.error('--spring requires --court 2')
if ARGS.gate and ARGS.court!=3:parser.error('--gate requires --court 3')
if ARGS.carry and ARGS.court!=3:parser.error('--carry requires --court 3')
if ARGS.running_carry and ARGS.court!=3:parser.error('--running-carry requires --court 3')
OUT=ROOT/('.local/basketball/all-computer-full-match-browser' if ARGS.all_computer else '.local/basketball/full-match-browser' if ARGS.full_match else '.local/basketball/live-browser')
if ARGS.court!=1:OUT=OUT.with_name(OUT.name+'-court-'+str(ARGS.court))
OUT.mkdir(parents=True,exist_ok=True)
env=dict(os.environ,AGENT_BROWSER_SESSION='bkl-'+uuid.uuid4().hex[:6],AGENT_BROWSER_SOCKET_DIR=str(ROOT/'.local/bkl-sockets'),TMPDIR=str(ROOT/'.local/tmp'),NODE_PATH=str(ROOT/'web/node_modules'))
Path(env['AGENT_BROWSER_SOCKET_DIR']).mkdir(exist_ok=True)
requests=[]
audio_catalog=json.loads((ROOT/'games/basketball/assets/audio-catalog.json').read_text())
audio_playback=json.loads((ROOT/'games/basketball/assets/audio-playback.json').read_text())
audio_files={e['wav'] for e in audio_catalog['effects']}|{e['wav'] for e in audio_playback['effects'].values()}|{e['file'] for e in audio_playback['music'].values() if e.get('mime_type')=='audio/mpeg'}
class Handler(BaseHTTPRequestHandler):
    def log_message(self,*args):pass
    def do_GET(self):
        route=unquote(self.path.split('?')[0]);requests.append(route)
        files={'/':ROOT/'scripts/basketball/live_test_page.html','/live.js':OUT/'live.js','/live.css':OUT/'live.css','/runtime/wasm_exec.js':OUT/'wasm_exec.js','/runtime/basketball.wasm':OUT/'basketball.wasm'}
        path=files.get(route)
        if route.startswith('/assets/'):
            rel=Path(route[8:])
            if rel.parts and '..' not in rel.parts and rel.parts[0] in ('court','courts','static-atlas','static-objects','raster','results') and rel.suffix in ('.json','.png'):
                allowed=(ROOT/'.local/basketball'/rel.parts[0]).resolve();candidate=(ROOT/'.local/basketball'/rel).resolve()
                if candidate.is_relative_to(allowed):path=candidate
        if route.startswith('/audio/') and route[7:] in audio_files:path=ROOT/'.local/basketball/audio/export'/route[7:]
        if path is None or not path.is_file():self.send_error(404);return
        content=path.read_bytes();mime={'.wasm':'application/wasm','.png':'image/png','.json':'application/json','.wav':'audio/wav','.mp3':'audio/mpeg','.html':'text/html','.css':'text/css'}.get(path.suffix,'text/javascript')
        match=re.fullmatch(r'bytes=(\d+)-(\d*)',self.headers.get('Range',''));start=0;end=len(content)-1
        if match:
            start=int(match[1]);end=min(end,int(match[2]) if match[2] else end)
            if start>end:self.send_error(416);return
        self.send_response(206 if match else 200);self.send_header('Content-Type',mime);self.send_header('Accept-Ranges','bytes')
        if match:self.send_header('Content-Range',f'bytes {start}-{end}/{len(content)}')
        self.send_header('Content-Length',str(end-start+1));self.end_headers()
        try:self.wfile.write(content[start:end+1])
        except (BrokenPipeError,ConnectionResetError):pass

def browser(*args):
    result=subprocess.run(['agent-browser',*args,'--json'],env=env,capture_output=True,text=True,timeout=60)
    if result.returncode:raise RuntimeError(result.stdout+result.stderr)
    value=json.loads(result.stdout);assert value['success'],value
    return value['data']

def main():
    (OUT/'result.json').write_text(json.dumps(dict(status='running',natural_full_match=ARGS.full_match))+'\n')
    subprocess.run(['go','build','-o',str(OUT/'basketball.wasm'),'./games/basketball/cmd/browser-wasm'],cwd=ROOT,env=dict(env,GOOS='js',GOARCH='wasm'),check=True)
    goroot=subprocess.check_output(['go','env','GOROOT'],cwd=ROOT,env=env,text=True).strip();(OUT/'wasm_exec.js').write_bytes((Path(goroot)/'lib/wasm/wasm_exec.js').read_bytes())
    subprocess.run([str(ROOT/'web/node_modules/.bin/esbuild'),str(ROOT/'scripts/basketball/live_browser_harness.ts'),'--bundle','--format=esm','--jsx=automatic','--loader:.svg=dataurl','--target=es2022','--outfile='+str(OUT/'live.js')],cwd=ROOT,env=env,check=True)
    server=ThreadingHTTPServer(('127.0.0.1',0),Handler);thread=threading.Thread(target=server.serve_forever,daemon=True);thread.start();opened=False;inspector=None;checks=[]
    def cdp(method,params={}):
        inspector.stdin.write(json.dumps(dict(method=method,params=params))+'\n');inspector.stdin.flush();result=json.loads(inspector.stdout.readline())
        if 'error'in result:raise RuntimeError(result['error'])
        return result['result']
    def js(expression):
        result=cdp('Runtime.evaluate',dict(expression=expression,returnByValue=True,awaitPromise=True,timeout=3000))
        if result.get('exceptionDetails'):raise RuntimeError(result['exceptionDetails'])
        return result['result'].get('value')
    def key(kind,code,key,number):cdp('Input.dispatchKeyEvent',dict(type=kind,code=code,key=key,windowsVirtualKeyCode=number))
    def expect(name,value):
        assert value,name
        checks.append(name);(OUT/'checks.json').write_text(json.dumps(checks,indent=2)+'\n')
    def position():return js('live.view.state.playing.world.Preparation.Actions.actors[0].player.body.position.x')
    try:
        origin='http://127.0.0.1:'+str(server.server_port)
        route_checks=[]
        for route,want in [('/assets/courts/manifest.json',200),('/assets/courts/0/court-tiles-1.png',200),('/assets/renderer/browser/cases.json',404),('/assets/courts/%2e%2e/%2e%2e/renderer/browser/cases.json',404),('/audio/music-061-loop.mp3',200),('/audio/music-061-loop.wav',404)]:
            try:
                with urlopen(origin+route,timeout=5) as response:status=response.status
            except HTTPError as error:status=error.code
            route_checks.append(dict(route=route,expected=want,actual=status))
        (OUT/'asset-routes.json').write_text(json.dumps(route_checks,indent=2)+'\n')
        expect('court assets served within permitted directories',all(r['actual']==r['expected'] for r in route_checks))
        page='http://127.0.0.1:'+str(server.server_port)+'/?manual=1&court='+str(ARGS.court)
        browser('--profile',str(OUT/'profile'),'open',page);opened=True
        endpoint=browser('get','cdp-url')['cdpUrl'];inspector=subprocess.Popen(['node',str(ROOT/'scripts/basketball/browser_cdp_session.mjs'),endpoint,page],env=env,stdin=subprocess.PIPE,stdout=subprocess.PIPE,stderr=(OUT/'cdp.stderr').open('w'),text=True);assert json.loads(inspector.stdout.readline())['ready']
        browser('click','#start');browser('wait','--fn','!!window.live || !!document.querySelector("#error").textContent')
        expect('created from configuration',js('!!window.live'))
        expect('requested original court is selected',js('live.view.state.config.match.Court')==ARGS.court)
        expect('FC background and layout are shared',js('!!document.querySelector("[data-controller-layout=fc] img")'))
        layout=js('(()=>{const rect=id=>{const r=document.querySelector(`[data-controller-button="${id}"]`).getBoundingClientRect();return{x:r.x+r.width/2,y:r.y+r.height/2}};return{left:rect("left"),select:rect("select"),start:rect("start"),b:rect("b"),a:rect("a")}})()')
        expect('FC D-pad Select Start B A ordering',layout['left']['x']<layout['select']['x']<layout['start']['x']<layout['b']['x']<layout['a']['x'])
        expect('play controller does not display keyboard mappings',js('document.querySelectorAll(".fc-play kbd").length===0 && [...document.querySelectorAll(".fc-play [data-controller-button]")].every(e=>!/[（(](?:W|S|D|F|J|K|U|I|Space|空格)[）)]| · /.test(e.getAttribute("aria-label")||"") && !/ · /.test(e.title))'))
        expect('turbo buttons sit above their matching buttons inside faceplate',js('(()=>{const p=document.querySelector(".fc-faceplate").getBoundingClientRect();return [["turboB","b"],["turboA","a"]].every(([t,b])=>{const a=document.querySelector(`[data-controller-button="${t}"]`).getBoundingClientRect(),n=document.querySelector(`[data-controller-button="${b}"]`).getBoundingClientRect();return Math.abs(a.x+a.width/2-n.x-n.width/2)<1 && a.bottom<=n.top+1 && a.left>=p.left && a.right<=p.right && a.top>=p.top && a.bottom<=p.bottom})})()'))
        if ARGS.fade:
            trace=[];pictures={};last=None
            for tick in range(50):
                js('live.advance(1)')
                row=js('(()=>{const s=live.view.state,d=s.display,c=document.querySelector("canvas"),p=c.getContext("2d").getImageData(0,0,256,240).data;const colors=new Set();for(let i=0;i<p.length;i+=4)colors.add([p[i],p[i+1],p[i+2],p[i+3]].join(","));return{frame:s.videoFrames,logic:s.logicSteps,visible:d.HasVisible,level:d.Visible.paletteLevel??4,fade:d.PaletteFade??null,colors:colors.size}})()')
                trace.append(row)
                if row['visible'] and row['level'] not in pictures:
                    pictures[row['level']]=js('document.querySelector("canvas").toDataURL()')
                    (OUT/('opening-fade-level-'+str(row['level'])+'.png')).write_bytes(base64.b64decode(pictures[row['level']].split(',')[1]))
                if row['fade'] and row['fade']['Frame']==4:
                    checkpoint=js('live.save()');before_image=js('document.querySelector("canvas").toDataURL()')
                    js('live.restore('+json.dumps(checkpoint)+')')
                    expect('restoring mid-fade retains the actual darkened pixels',js('document.querySelector("canvas").toDataURL()')==before_image)
                if row['visible'] and row['level']==4 and not row['fade']:break
            (OUT/'opening-fade-trace.json').write_text(json.dumps(trace,indent=2)+'\n')
            expect('cold opening does not flash full colors before fade',all(row['colors']==1 for row in trace if row['visible'] and row['level']==0))
            start=next(i for i,row in enumerate(trace) if row['level']==1 and row['visible'])
            expect('Go opening commits original four levels on video boundaries',[row['level'] for row in trace[start:start+8]]==[1,1,2,2,3,3,4,4])
            expect('Canvas presents five distinct original palette brightness levels',len(pictures)==5 and len(set(pictures.values()))==5)
            expect('fade suspends logical actor updates for all eight videos',len({row['logic'] for row in trace[start:start+8]})==1)
        js('live.advance(120);live.advance(90)');expect('Go opening reaches playing',js('!!live.view.state.playing && live.view.state.display.HasVisible'))
        before=position();key('keyDown','KeyD','d',68);js('live.advance(18)');key('keyUp','KeyD','d',68)
        expect('native held D moves Go player',position()>before)
        js('live.advance(3)');key('keyDown','Space',' ',32);key('keyUp','Space',' ',32);js('live.advance(3)')
        expect('short Start pauses original match',js('live.view.state.playing.match.Phase===2'))
        paused=position();clock=js('live.view.state.playing.match.Clock');js('live.advance(30)')
        expect('pause freezes world and clock',position()==paused and js('live.view.state.playing.match.Clock')==clock)
        checkpoint=js('live.save()');png=js('document.querySelector("canvas").toDataURL()')
        js('live.restore('+json.dumps(checkpoint)+')');expect('restore keeps exact picture',js('document.querySelector("canvas").toDataURL()')==png)
        # Explicit display-state contract on the real client. These patches are
        # synthetic renderer inputs, not evidence that Go emitted court animation.
        patch_contract=js('(()=>{const c=live.client,checkpoint=live.save(),canvas=document.querySelector("canvas"),before=canvas.toDataURL(),visible=JSON.stringify(c.view.state.display.Visible);c.view.state.display.CourtPatches=[{plane:1,row:20,column:0,tiles:Array(32).fill(0)}];c.view.state.display.CourtSerial++;c.present();const patched=canvas.toDataURL(),unchangedVisible=visible===JSON.stringify(c.view.state.display.Visible);c.view.state.display.CourtSerial++;c.present();const stable=patched===canvas.toDataURL();live.restore(checkpoint);return{changed:patched!==before,unchangedVisible,stable,restored:canvas.toDataURL()===before,scope:"synthetic patch-only display state through actual client"}})()')
        (OUT/'patch-contract.json').write_text(json.dumps(patch_contract,indent=2)+'\n')
        expect('patch-only display commits redraw without changing visible snapshot',patch_contract['changed'] and patch_contract['unchangedVisible'])
        expect('compacted patch replay is stable and restore clears it',patch_contract['stable'] and patch_contract['restored'])
        key('keyDown','Space',' ',32);key('keyUp','Space',' ',32);js('live.advance(3)');expect('Start resumes',js('live.view.state.playing.match.Phase===1'))
        key('keyDown','KeyK','k',75);js('live.advance(1)');key('keyUp','KeyK','k',75);js('window.dispatchEvent(new Event("blur"))')
        expect('blur cancels unconsumed Go input',js('live.view.state.held[0]===0 && live.view.state.pendingPressed[0]===0'))
        browser('set','viewport','320','720');cdp('Emulation.setTouchEmulationEnabled',dict(enabled=True,maxTouchPoints=2))
        point=js('(()=>{const r=document.querySelector("[data-controller-button=right]").getBoundingClientRect();return{x:r.x+r.width/2,y:r.y+r.height/2}})()')
        before=position();cdp('Input.dispatchTouchEvent',dict(type='touchStart',touchPoints=[dict(x=point['x'],y=point['y'],id=1)]));js('live.advance(12)');cdp('Input.dispatchTouchEvent',dict(type='touchCancel',touchPoints=[]));js('live.advance(3)')
        expect('native touch moves Go player',position()>before)
        expect('touch cancel releases controller',js('live.view.state.held[0]===0'))
        point=js('(()=>{const r=document.querySelector("[data-controller-button=a]").getBoundingClientRect();return{x:r.x+r.width/2,y:r.y+r.height/2}})()')
        key('keyDown','KeyD','d',68);cdp('Input.dispatchTouchEvent',dict(type='touchStart',touchPoints=[dict(x=point['x'],y=point['y'],id=2)]));js('live.advance(1)');cdp('Input.dispatchTouchEvent',dict(type='touchCancel',touchPoints=[]))
        expect('touch cancel clears only its Go input bit',js('live.view.state.held[0]===1 && (live.view.state.pendingPressed[0]&128)===0'))
        key('keyUp','KeyD','d',68);js('live.advance(3)')
        box=js('(()=>{const c=document.querySelector("canvas").getBoundingClientRect(),p=document.querySelector("#controls").getBoundingClientRect();return{width:c.width,height:c.height,bottom:p.bottom,overflow:document.documentElement.scrollWidth>innerWidth}})()')
        expect('320px game and controls fit viewport',not box['overflow'] and box['bottom']<=720 and box['width']>=250)
        pixels=js('(()=>{const d=document.querySelector("canvas").getContext("2d").getImageData(0,0,256,240).data;let count=0;for(let i=0;i<d.length;i+=4)if(d[i]+d[i+1]+d[i+2]>40)count++;return count})()')
        expect('original-art canvas is visible',pixels>20000)
        browser('screenshot',str(OUT/'mobile.png'));data=js('document.querySelector("canvas").toDataURL()');(OUT/'canvas.png').write_bytes(base64.b64decode(data.split(',')[1]))
        browser('set','viewport','1280','900');browser('screenshot',str(OUT/'desktop.png'))
        desktop=js('(()=>{const main=document.querySelector("main").getBoundingClientRect(),pad=document.querySelector(".fc-faceplate").getBoundingClientRect(),start=document.querySelector("[data-controller-button=start]").getBoundingClientRect();return{bottom:main.bottom,viewport:innerHeight,pad:{top:pad.top,bottom:pad.bottom},start:{top:start.top,bottom:start.bottom}}})()')
        (OUT/'desktop-layout.json').write_text(json.dumps(desktop,indent=2)+'\n')
        expect('desktop whole game fits viewport',desktop['bottom']<=desktop['viewport']+1 and desktop['start']['top']>=desktop['pad']['top'] and desktop['start']['bottom']<=desktop['pad']['bottom'])
        if ARGS.court==4:
            palette_trace=js('(()=>{const rows=[];for(let i=0;i<48;i++){live.advance(1);const d=live.view.state.display;rows.push({frame:live.view.state.videoFrames,palette:d.Visible.paletteID??5,pending:d.PendingPaletteID??0,hasVisible:d.HasVisible})}return rows})()')
            (OUT/'palette-input-trace.json').write_text(json.dumps(palette_trace,indent=2)+'\n')
            expect('Court4 original palettes cycle through committed visible frames',set(row['palette'] for row in palette_trace)=={5,9,10})
            browser('screenshot',str(OUT/'palette-cycle.png'))
        if ARGS.spring:
            # Independent physical-input scenario: four human controllers,
            # three idle. This does not replace either original-AI cold gate.
            js('(()=>{const call=v=>{const r=JSON.parse(basketballWasm(JSON.stringify(v)));if(!r.ok)throw Error(JSON.stringify(r.error));return r.data};const config=call({op:"defaults"});config.match.Court=2;config.match.Mode=128;config.match.Seed=0x12345678;config.match.ControllerAssignments=[0,1,2,3];const fresh=call({op:"create",config});live.restore({version:1,state:fresh.state});call({op:"dispose",session:fresh.session});live.advance(120);live.advance(90)})()')
            key('keyDown','KeyD','d',68)
            walked=js('(()=>{for(let n=0;n<600;n++){live.advance(1);const p=live.view.state.playing.world.Preparation.Actions.actors[0].player;if(p.body.position.x>=278*256)return{ticks:n+1,position:p.body.position};}throw Error("native walk never reached the spring column")})()')
            key('keyUp','KeyD','d',68);js('live.advance(9)')
            key('keyDown','KeyW','w',87);key('keyDown','KeyJ','j',74);key('keyDown','KeyK','k',75);js('live.advance(3)');key('keyUp','KeyJ','j',74);key('keyUp','KeyK','k',75)
            trace=[];triggered=False
            for _ in range(300):
                row=js('(()=>{live.advance(1);const s=live.view.state,a=s.playing.world.Preparation.Actions;return{frame:s.videoFrames,logic:s.logicSteps,player:a.actors[0].player,left:a.actors[1].player.newYorkPassage??0,right:a.actors[2].player.newYorkPassage??0,patches:s.display.PendingPatches??[]}})()');trace.append(row)
                if row['player']['action']==62:triggered=True;break
            key('keyUp','KeyW','w',87)
            (OUT/'spring-input-trace.json').write_text(json.dumps(dict(walk=walked,trace=trace,worldFieldsRewritten=False,otherControllers='three idle human seats'),indent=2)+'\n')
            expect('native direction and jump input activates Court2 spring',triggered)
            (OUT/'spring-before-action.json').write_text(json.dumps(js('live.save()'),indent=2)+'\n');browser('screenshot',str(OUT/'spring-trigger.png'))
            js('live.advance(9)');browser('screenshot',str(OUT/'spring-launch.png'));js('live.advance(51)')
            expect('original spring action executes after the device request',js('live.view.state.playing.world.Preparation.Actions.actors[0].player.action!==62 || live.view.state.playing.world.Preparation.Actions.actors[0].player.entered'))
            browser('screenshot',str(OUT/'spring-flight.png'))
            (OUT/'spring-after-action.json').write_text(json.dumps(js('live.save()'),indent=2)+'\n')
        if ARGS.gate:
            # Distinct native input scenario; choose four human controllers at
            # cold creation, with the other three idle. No position injection.
            js('(()=>{const call=v=>{const r=JSON.parse(basketballWasm(JSON.stringify(v)));if(!r.ok)throw Error(JSON.stringify(r.error));return r.data};const config=call({op:"defaults"});config.match.Court=3;config.match.Mode=128;config.match.Seed=0x12345678;config.match.ControllerAssignments=[0,1,2,3];const fresh=call({op:"create",config});live.restore({version:1,state:fresh.state});call({op:"dispose",session:fresh.session});live.advance(120);live.advance(90)})()')
            key('keyDown','KeyA','a',65)
            walked=js('(()=>{for(let n=0;n<600;n++){live.advance(1);const p=live.view.state.playing.world.Preparation.Actions.actors[0].player;if(p.body.position.x<=150*256)return{ticks:n+1,position:p.body.position};}throw Error("native walk never reached the gate column")})()')
            key('keyUp','KeyA','a',65);js('live.advance(3)')
            (OUT/'gate-before.json').write_text(json.dumps(js('live.save()'),indent=2)+'\n')
            browser('screenshot',str(OUT/'gate-before.png'))
            key('keyDown','KeyW','w',87);key('keyDown','KeyJ','j',74);key('keyDown','KeyK','k',75);js('live.advance(3)');key('keyUp','KeyJ','j',74);key('keyUp','KeyK','k',75)
            trace=[];triggered=False
            for tick in range(300):
                if tick==18:
                    key('keyDown','KeyJ','j',74);key('keyDown','KeyK','k',75)
                if tick==21:
                    key('keyUp','KeyJ','j',74);key('keyUp','KeyK','k',75)
                row=js('(()=>{live.advance(1);const s=live.view.state,a=s.playing.world.Preparation.Actions;return{frame:s.videoFrames,logic:s.logicSteps,player:a.actors[0].player,gateFlags:a.actors[0].player.newYorkPassage??0,gateTicks:a.actors[1].player.newYorkPassage??0,patches:s.display.PendingPatches??[]}})()');trace.append(row)
                if row['player']['action']==62:triggered=True;break
            key('keyUp','KeyW','w',87);key('keyUp','KeyJ','j',74);key('keyUp','KeyK','k',75)
            (OUT/'gate-input-trace.json').write_text(json.dumps(dict(walk=walked,trace=trace,worldFieldsRewritten=False,otherControllers='three idle human seats'),indent=2)+'\n')
            expect('native input activates Court3 gate and original launch',triggered)
            (OUT/'gate-trigger.json').write_text(json.dumps(js('live.save()'),indent=2)+'\n')
            js('live.advance(9)');browser('screenshot',str(OUT/'gate-launch.png'))
            expect('gate trigger reaches the original background animation',js('(live.view.state.playing.world.Preparation.Actions.actors[0].player.newYorkPassage&1)!==0 && (live.view.state.display.CourtPatches??[]).length>0'))
            js('live.advance(60)')
            (OUT/'gate-after.json').write_text(json.dumps(js('live.save()'),indent=2)+'\n')
        if ARGS.carry:
            carry_records=[]
            def carried():return js('(()=>{const s=live.view.state,a=s.playing.world.Preparation.Actions;return{frame:s.videoFrames,player:a.actors[0].player,aux:a.auxiliary[0],held:s.held}})()')
            def capture_carry(label):
                row=dict(label=label,**carried());carry_records.append(row)
                (OUT/'carry-native-input.json').write_text(json.dumps(dict(nativeInput=True,worldFieldsRewritten=False,configuration='four original human controllers, other three idle',trace=carry_records),indent=2)+'\n')
                return row
            def pickup():
                js('(()=>{const call=v=>{const r=JSON.parse(basketballWasm(JSON.stringify(v)));if(!r.ok)throw Error(JSON.stringify(r.error));return r.data};const config=call({op:"defaults"});config.match.Court=3;config.match.Mode=128;config.match.Seed=0x12345678;config.match.ControllerAssignments=[0,1,2,3];const fresh=call({op:"create",config});live.restore({version:1,state:fresh.state});call({op:"dispose",session:fresh.session});live.advance(120);live.advance(90);document.querySelector("#game").focus()})()')
                key('keyDown','KeyA','a',65)
                js('(()=>{for(let n=0;n<600;n++){live.advance(1);if(live.view.state.playing.world.Preparation.Actions.actors[0].player.body.position.x<=60*256)return;}throw Error("native walk never reached pickup zone")})()')
                key('keyUp','KeyA','a',65);js('live.advance(6)');capture_carry('walked beside original prop')
                key('keyDown','KeyK','k',75);js('live.advance(6)');key('keyUp','KeyK','k',75)
                js('(()=>{for(let n=0;n<180;n++){live.advance(1);const a=live.view.state.playing.world.Preparation.Actions;if(a.actors[0].player.action===32&&a.auxiliary[0].Owner===0)return;}throw Error("native A pickup did not reach carried idle")})()')
                capture_carry('picked up with native A')
            pickup()
            before=carried()['player']['body']['position']['x']
            key('keyDown','KeyD','d',68);js('live.advance(90)');key('keyUp','KeyD','d',68);js('live.advance(6)')
            state=capture_carry('walked carrying original prop')
            expect('native movement keeps the picked-up prop attached',state['player']['body']['position']['x']>before and state['aux']['Owner']==0)
            expect('carried player and prop are inside the visible camera and draw order',js('(()=>{const v=live.view.state.display.Visible;return [0,4].every(id=>v.drawOrder.includes(id)&&v.objects[id].pose!==127&&v.objects[id].projected.x>=0&&v.objects[id].projected.x<240&&v.objects[id].projected.y>=0&&v.objects[id].projected.y<192)})()'))
            browser('screenshot',str(OUT/'carry-picked-up.png'))
            key('keyDown','KeyJ','j',74);key('keyDown','KeyK','k',75);js('live.advance(6)');key('keyUp','KeyJ','j',74);key('keyUp','KeyK','k',75);js('live.advance(12)')
            state=capture_carry('jumped with carried prop')
            expect('native A+B jumps while carrying the prop',state['player']['body']['position']['height']>0 and state['aux']['Owner']==0)
            key('keyDown','KeyK','k',75);js('live.advance(6)');key('keyUp','KeyK','k',75);js('live.advance(90)')
            state=capture_carry('airborne A throw completed')
            expect('native airborne A throw releases the original prop',state['aux']['Owner']==-1)
            browser('screenshot',str(OUT/'carry-air-throw.png'))
            pickup()
            key('keyDown','KeyD','d',68);js('live.advance(90)');key('keyUp','KeyD','d',68);js('live.advance(60)')
            key('keyDown','KeyD','d',68);key('keyDown','KeyJ','j',74);js('live.advance(12)');key('keyUp','KeyJ','j',74);js('live.advance(6)');key('keyUp','KeyD','d',68);js('live.advance(90)')
            state=capture_carry('directional B lob completed')
            expect('native horizontal B and release launches original high throw',state['aux']['Owner']==-1 and state['aux']['Body']['position']['x']>60*256)
            browser('screenshot',str(OUT/'carry-high-throw.png'))
            pickup()
            key('keyDown','KeyD','d',68);js('live.advance(90)');key('keyUp','KeyD','d',68);js('live.advance(60)')
            key('keyDown','KeyJ','j',74);js('live.advance(6)');key('keyUp','KeyJ','j',74)
            state=capture_carry('native B set-down begins')
            expect('native nondirectional B selects set-down instead of throwing',state['player']['action']==79)
            js('live.advance(90)');state=capture_carry('native B set-down complete')
            expect('native B puts the original prop on the floor and releases input',state['aux']['Owner']==-1 and state['aux']['Action']==0 and state['aux']['Body']['position']['height']//256==0 and state['held'][0]==0)
            browser('screenshot',str(OUT/'carry-set-down.png'))
            # A separate phone scenario uses native touch only: the same
            # logical controller path must carry, jump and throw on mobile.
            browser('set','viewport','320','720');cdp('Emulation.setTouchEmulationEnabled',dict(enabled=True,maxTouchPoints=3))
            js('(()=>{const call=v=>{const r=JSON.parse(basketballWasm(JSON.stringify(v)));if(!r.ok)throw Error(JSON.stringify(r.error));return r.data};const config=call({op:"defaults"});config.match.Court=3;config.match.Mode=128;config.match.Seed=0x12345678;config.match.ControllerAssignments=[0,1,2,3];const fresh=call({op:"create",config});live.restore({version:1,state:fresh.state});call({op:"dispose",session:fresh.session});live.advance(120);live.advance(90)})()')
            def touch_point(selector,x=.5,y=.5):
                return js('(()=>{const r=document.querySelector('+json.dumps(selector)+').getBoundingClientRect();return{x:r.x+r.width*'+str(x)+',y:r.y+r.height*'+str(y)+'}})()')
            left=touch_point('[data-controller-dpad]',.15,.5);right=touch_point('[data-controller-dpad]',.85,.5)
            ta=touch_point('[data-controller-button=a]');tb=touch_point('[data-controller-button=b]')
            cdp('Input.dispatchTouchEvent',dict(type='touchStart',touchPoints=[dict(id=1,**left)]))
            js('(()=>{for(let n=0;n<600;n++){live.advance(1);if(live.view.state.playing.world.Preparation.Actions.actors[0].player.body.position.x<=60*256)return;}throw Error("touch left did not reach prop")})()')
            cdp('Input.dispatchTouchEvent',dict(type='touchEnd',touchPoints=[]));js('live.advance(6)')
            cdp('Input.dispatchTouchEvent',dict(type='touchStart',touchPoints=[dict(id=2,**ta)]));js('live.advance(6)');cdp('Input.dispatchTouchEvent',dict(type='touchEnd',touchPoints=[]));js('live.advance(60)')
            state=capture_carry('phone touch pickup')
            expect('native phone A touch picks up original prop',state['player']['action']==32 and state['aux']['Owner']==0)
            cdp('Input.dispatchTouchEvent',dict(type='touchStart',touchPoints=[dict(id=1,**right)]));js('live.advance(90)');cdp('Input.dispatchTouchEvent',dict(type='touchEnd',touchPoints=[]));js('live.advance(6)')
            state=capture_carry('phone touch carrying movement')
            expect('native phone D-pad moves with the held prop',state['player']['body']['position']['x']>150*256 and state['aux']['Owner']==0)
            browser('screenshot',str(OUT/'carry-mobile-320.png'))
            js('live.advance(60)')
            cdp('Input.dispatchTouchEvent',dict(type='touchStart',touchPoints=[dict(id=3,**tb)]));js('live.advance(6)');cdp('Input.dispatchTouchEvent',dict(type='touchEnd',touchPoints=[]))
            state=capture_carry('phone B set-down begins')
            expect('phone B selects original set-down without a direction',state['player']['action']==79)
            js('live.advance(90)');state=capture_carry('phone B set-down complete')
            expect('phone set-down returns prop to floor with no stale held buttons',state['aux']['Owner']==-1 and state['aux']['Action']==0 and state['aux']['Body']['position']['height']//256==0 and state['held'][0]==0)
            browser('screenshot',str(OUT/'carry-set-down-mobile-320.png'))
            cdp('Input.dispatchTouchEvent',dict(type='touchStart',touchPoints=[dict(id=2,**ta)]));js('live.advance(6)');cdp('Input.dispatchTouchEvent',dict(type='touchEnd',touchPoints=[]));js('live.advance(60)')
            state=capture_carry('phone picks up set-down prop again')
            expect('phone can pick up the same prop after setting it down',state['player']['action']==32 and state['aux']['Owner']==0)
            cdp('Input.dispatchTouchEvent',dict(type='touchStart',touchPoints=[dict(id=2,**ta),dict(id=3,**tb)]));js('live.advance(6)');cdp('Input.dispatchTouchEvent',dict(type='touchEnd',touchPoints=[]));js('live.advance(12)')
            state=capture_carry('phone native two-finger jump')
            expect('phone simultaneous A+B touch jumps while carrying',state['player']['body']['position']['height']>0 and state['aux']['Owner']==0)
            cdp('Input.dispatchTouchEvent',dict(type='touchStart',touchPoints=[dict(id=2,**ta)]));js('live.advance(6)');cdp('Input.dispatchTouchEvent',dict(type='touchEnd',touchPoints=[]));js('live.advance(90)')
            state=capture_carry('phone airborne A throw')
            expect('native phone A touch throws without retaining stale input',state['aux']['Owner']==-1 and state['held'][0]==0)
            browser('set','viewport','1280','900')
        if ARGS.running_carry:
            running_results=[]
            for team,mobile in [(0,False),(2,False),(7,False),(0,True),(7,True)]:
                browser('set','viewport',*(['320','720'] if mobile else ['1280','900']))
                if mobile:cdp('Emulation.setTouchEmulationEnabled',dict(enabled=True,maxTouchPoints=3))
                js('(()=>{const call=v=>{const r=JSON.parse(basketballWasm(JSON.stringify(v)));if(!r.ok)throw Error(JSON.stringify(r.error));return r.data};const config=call({op:"defaults"});config.match.Court=3;config.match.Mode=128;config.match.Teams[0]='+str(team)+';config.match.Seed=0x12345678;config.match.ControllerAssignments=[0,1,2,3];const fresh=call({op:"create",config});live.restore({version:1,state:fresh.state});call({op:"dispose",session:fresh.session});live.advance(120);live.advance(90);document.querySelector("#game").focus();window.runningTrace=[];window.runningStep=()=>{live.advance(1);const s=live.view.state,a=s.playing.world.Preparation.Actions,p=a.actors[0].player,o=a.auxiliary[0];const r={video:s.videoFrames,action:p.action,request:p.request,auxAction:o.Action,owner:o.Owner,position:p.body.position,auxPosition:o.Body.position,held:s.held[0]};runningTrace.push(r);return r;}})()')
                held_touch={}
                mapping={'left':('KeyA','a',65),'right':('KeyD','d',68),'a':('KeyK','k',75),'b':('KeyJ','j',74)}
                ids={'left':1,'right':2,'a':3,'b':4}
                def running_press(button):
                    if not mobile:key('keyDown',*mapping[button]);return
                    if button in ('left','right'):
                        x=.15 if button=='left' else .85
                        point=js('(()=>{const r=document.querySelector("[data-controller-dpad]").getBoundingClientRect();return{x:r.x+r.width*'+str(x)+',y:r.y+r.height*.5}})()')
                    else:
                        point=js('(()=>{const r=document.querySelector("[data-controller-button='+button+']").getBoundingClientRect();return{x:r.x+r.width*.5,y:r.y+r.height*.5}})()')
                    held_touch[button]=dict(id=ids[button],**point)
                    cdp('Input.dispatchTouchEvent',dict(type='touchStart',touchPoints=list(held_touch.values())))
                def running_release(button):
                    if not mobile:key('keyUp',*mapping[button]);return
                    held_touch.pop(button,None);cdp('Input.dispatchTouchEvent',dict(type='touchEnd',touchPoints=list(held_touch.values())))
                def running_ticks(n):return js('(()=>{let r;for(let i=0;i<'+str(n)+';i++)r=runningStep();return r})()')
                def running_until(condition):return js('(()=>{for(let i=0;i<600;i++){const r=runningStep();if('+condition+')return r;}throw Error("running carry route failed: '+condition+'")})()')
                label=('touch' if mobile else 'keyboard')+' team'+str(team)
                running_press('left');running_until('r.position.x<=60*256');running_release('left');running_ticks(6)
                running_press('a');running_ticks(6);running_release('a');running_until('r.action===32&&r.owner===0')
                running_press('right');running_until('r.position.x>=160*256');running_release('right');running_ticks(60)
                running_press('right');running_ticks(6);running_release('right');running_ticks(3);running_press('right');row=running_ticks(12)
                expect('native double-tap runs while holding prop '+label,row['action']==34 and row['owner']==0)
                running_release('right');running_press('b');running_ticks(6);running_release('b')
                if team==7:
                    row=running_ticks(36)
                    expect('native running B lifts prop for original A release '+label,row['auxAction']==9 and row['owner']==0)
                    browser('screenshot',str(OUT/('running-lift-'+label.replace(' ','-')+'.png')))
                    running_press('a');running_ticks(6);running_release('a');running_ticks(18)
                    expect('native A releases lifted prop along original attack '+label,js('runningTrace.some(r=>r.auxAction===11&&r.owner===-1)'))
                running_ticks(120)
                rows=js('runningTrace');running_results.append(dict(team=team,input='touch' if mobile else 'keyboard',trace=rows))
                (OUT/'running-carry-native.json').write_text(json.dumps(dict(worldFieldsRewritten=False,scope='cold original configurations, other three humans idle, native keyboard/touch',scenarios=running_results),indent=2)+'\n')
                # Input and action dispatch share a logic update; the original
                # request is already consumed when the video view is published.
                expect('native running B preserves original request '+label,any(r['request']['action']==104 for r in rows))
                if team==0:expect('original role spins the held prop '+label,any(r['action']==104 and r['auxAction']==10 for r in rows))
                if team==2:expect('original fallback brakes without invented prop attack '+label,any(r['action']==35 for r in rows) and not any(r['auxAction']in[9,10,11] for r in rows))
                expect('running attack leaves no stuck native input '+label,rows[-1]['held']==0)
                browser('screenshot',str(OUT/('running-final-'+label.replace(' ','-')+'.png')))
            browser('set','viewport','1280','900')
        if ARGS.full_match:
            # Actual cold creation and original clock. No scores, positions,
            # RNG or clock fields are rewritten. All Go frames reach Canvas.
            controller_setup='config.match.Court='+str(ARGS.court)+';'+('config.match.ControllerAssignments=[-1,-1,-1,-1];' if ARGS.all_computer else '')
            js('(()=>{const call=v=>{const r=JSON.parse(window.basketballWasm(JSON.stringify(v)));if(!r.ok)throw Error(JSON.stringify(r.error));return r.data};const config=call({op:"defaults"});config.match.Seed=0x12345678;'+controller_setup+'const fresh=call({op:"create",config});live.restore({version:1,state:fresh.state});call({op:"dispose",session:fresh.session});return true})()')
            if ARGS.all_computer:expect('all four original computer slots selected',js('live.view.state.config.match.ControllerAssignments.every(slot=>slot===-1)'))
            observe_fade=''
            if ARGS.fade:
                js('window.naturalFades=[]')
                observe_fade='const f=s.display.PaletteFade;if(f&&f.Frame%2===1){const p=document.querySelector("canvas").getContext("2d").getImageData(0,0,256,240).data,colors=new Set();let hash=2166136261;for(let j=0;j<p.length;j+=4){colors.add((p[j]<<16)|(p[j+1]<<8)|p[j+2]);for(let k=0;k<3;k++)hash=Math.imul(hash^p[j+k],16777619)>>>0;}naturalFades.push({video:s.videoFrames,out:f.Out,cursor:f.Frame,level:s.display.Visible.paletteLevel??4,result:!!s.display.Visible.result,palette:s.display.Visible.paletteID,colors:colors.size,pixelHash:hash});}'
            if ARGS.court==6:
                js('window.naturalSlots=[];window.lastSlotKey=""')
                observe_fade+='const p=s.playing?.world.Preparation;if(p){const a=p.Actions,row={frame:s.videoFrames,logic:s.logicSteps,progress:p.CourtScoreFlags,outcome:p.CourtOutcomeFlags??0,scores:p.Scores,palette:s.display.Visible.paletteID,reels:[...a.actors.map(a=>a.player.newYorkPassage??0),...a.auxiliary.map(a=>a.NewYorkPassage??0)]},key=JSON.stringify([row.progress,row.outcome,row.scores,row.palette,row.reels]);if(key!==lastSlotKey){lastSlotKey=key;naturalSlots.push(row);}}'
            began=time.monotonic();trace=[];intermissions=0
            for batch in range(500):
                state=js('(()=>{let s=live.view.state;for(let i=0;i<120&&!s.outcome?.confirmed;i++){if(s.awaitingNextPeriod&&s.waitVideoFrames===0)break;live.advance(1);s=live.view.state;'+observe_fade+'}return{videoFrames:s.videoFrames,logicSteps:s.logicSteps,awaiting:s.awaitingNextPeriod,wait:s.waitVideoFrames,period:s.playing?.match.Period,outcome:s.outcome}})()')
                trace.append(state)
                (OUT/'natural-progress.json').write_text(json.dumps(dict(elapsedSeconds=time.monotonic()-began,latest=state,batches=len(trace)),indent=2)+'\n')
                if state.get('outcome',{}).get('confirmed'):break
                assert state['videoFrames']<48000,'natural full browser match exceeded unchanged video limit'
                if state['awaiting'] and state['wait']==0:
                    expect('intermission gives a visible Start prompt',js('(()=>{const e=document.querySelector("#status"),r=e.getBoundingClientRect();return e.textContent.includes("中场休息")&&e.textContent.includes("START")&&r.top>=0&&r.bottom<=innerHeight})()'))
                    browser('screenshot',str(OUT/'natural-intermission.png'))
                    key('keyDown','Space',' ',32);js('live.advance(1)');key('keyUp','Space',' ',32);intermissions+=1
            expect('cold browser match reaches original confirmed outcome',state.get('outcome',{}).get('confirmed') and intermissions==1 and state['videoFrames']>=21600)
            if ARGS.court==6:
                slots=js('naturalSlots');(OUT/'natural-slot-events.json').write_text(json.dumps(slots,indent=2)+'\n')
                expect('natural Court6 goals activate original slot progress',any(any(r['progress']) for r in slots))
                expect('natural Court6 match reaches the original slot award phase',any(r['outcome']&128 for r in slots))
                expect('original slot award commits changing palettes on Canvas',len({r['palette'] for r in slots if r['outcome']&128})>1)
            if ARGS.fade:
                fades=js('naturalFades');(OUT/'natural-fades.json').write_text(json.dumps(fades,indent=2)+'\n')
                expect('natural complete match renders both openings, both exits and result fade',[r['level'] for r in fades]==[1,2,3,4,3,2,1,0,1,2,3,4,3,2,1,0,1,2,3,4])
                expect('all natural fades commit two videos apart with distinct pixel output',all(len({r['pixelHash'] for r in fades[i:i+4]})==4 and all(fades[i+j]['video']-fades[i]['video']==2*j for j in range(4)) for i in range(0,20,4)))
                expect('natural exits visibly reach black and result fades its own background',all(r['colors']==1 for r in fades if r['level']==0) and all(r['result'] for r in fades[-4:]))
            natural=js('live.save()');natural_image=js('document.querySelector("canvas").toDataURL()')
            js('live.advance(120)')
            expect('natural final state remains stable',js('live.save()')==natural and js('document.querySelector("canvas").toDataURL()')==natural_image)
            js('live.restore('+json.dumps(natural)+')')
            expect('natural final image survives restore',js('document.querySelector("canvas").toDataURL()')==natural_image)
            (OUT/'natural-final-state.json').write_text(json.dumps(natural,indent=2)+'\n')
            (OUT/'natural-final-canvas.png').write_bytes(base64.b64decode(natural_image.split(',')[1]))
            (OUT/'natural-result.json').write_text(json.dumps(dict(status='passed',coldSeed=0x12345678,allComputer=ARGS.all_computer,court=ARGS.court,rewrittenWorldFields=False,intermissions=intermissions,elapsedSeconds=time.monotonic()-began,final=state,trace=trace),indent=2)+'\n')
            browser('screenshot',str(OUT/'natural-final-desktop.png'));browser('set','viewport','320','720');browser('screenshot',str(OUT/'natural-final-mobile.png'));browser('set','viewport','1280','900')
        # Explicit final-scene boundary; this does not stand in for natural
        # full-match acceptance in verify_video_runtime.py --full-match.
        js('(()=>{const call=v=>{const r=JSON.parse(window.basketballWasm(JSON.stringify(v)));if(!r.ok)throw Error(JSON.stringify(r.error));return r.data};const config=call({op:"defaults"});config.match.Court='+str(ARGS.court)+';const fresh=call({op:"create",config});const s=fresh.state,o=s.opening,previous=live.save().state;s.display=previous.display;const camera=previous.display.Visible.camera;o.Transition.CameraScroll=[camera.x,camera.y,camera.x,camera.y];o.Transition.Match.Phase=6;o.Transition.Match.Period=2;o.Transition.Scores=[0,12];o.Transition.Objects.forEach(object=>object.Pose={frame:127,mirrored:false});o.Transition.Match.DisplayedScores=[[0,0,0],[0,1,2]];o.Projection.drawOrder.fill(255,0,12);o.Projection.drawCount=0;delete s.opening;s.transition={scene:o};s.logicSteps=1;call({op:"dispose",session:fresh.session});live.restore({version:1,state:s});for(let i=0;i<5;i++)live.advance(120);return true})()')
        expect('Go result is confirmed and rendered',js('live.view.state.outcome?.confirmed===true && live.view.state.display.Visible.result?.winningTeam===1'))
        terminal=js('live.save()');terminal_image=js('document.querySelector("canvas").toDataURL()')
        js('live.advance(120)')
        expect('completed match keeps final state and picture',js('live.save()')==terminal and js('document.querySelector("canvas").toDataURL()')==terminal_image)
        js('live.restore('+json.dumps(terminal)+')')
        expect('completed match restore keeps final picture',js('document.querySelector("canvas").toDataURL()')==terminal_image)
        browser('screenshot',str(OUT/'result-desktop.png'));browser('set','viewport','320','720');browser('screenshot',str(OUT/'result-mobile.png'))
        expect('320px result and controls fit viewport',js('document.documentElement.scrollWidth<=innerWidth && document.querySelector("#controls").getBoundingClientRect().bottom<=innerHeight'))
        expect('no original executable or replay fixture fetched',not any('.nes'in p or'testdata'in p or'cases.json'in p for p in requests if p not in {unquote(r['route']) for r in route_checks if r['expected']==404}))
        (OUT/'final-state.json').write_text(json.dumps(js('live.save()'),indent=2)+'\n')
        (OUT/'result.json').write_text(json.dumps(dict(status='passed',checks=checks,scope='actual Go/Wasm interactive browser segment plus explicit result boundary; full-match result is separate when requested, platform integration remains incomplete',natural_full_match=ARGS.full_match,all_computer=ARGS.all_computer,court=ARGS.court,result_boundary_is_synthetic=True,wasm_sha256=hashlib.sha256((OUT/'basketball.wasm').read_bytes()).hexdigest(),responsive=box),indent=2)+'\n')
        print(json.dumps(dict(status='passed',checks=len(checks))))
    finally:
        (OUT/'requests.json').write_text(json.dumps(requests,indent=2)+'\n')
        if inspector:
            try:
                image=js('document.querySelector("canvas").toDataURL()');(OUT/'last-canvas.png').write_bytes(base64.b64decode(image.split(',')[1]))
                (OUT/'last-state.json').write_text(json.dumps(js('window.live?.client.view??null'),indent=2)+'\n')
            except Exception as error:(OUT/'capture-error.log').write_text(str(error))
        if inspector:inspector.stdin.close();inspector.wait(timeout=10)
        if opened:browser('close')
        server.shutdown();server.server_close();thread.join(timeout=5)

if __name__=='__main__':
    try:main()
    except Exception as error:
        (OUT/'result.json').write_text(json.dumps(dict(status='failed',natural_full_match=ARGS.full_match,error=repr(error)),indent=2)+'\n')
        raise
