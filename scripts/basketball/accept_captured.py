"""Native keyboard and two-finger captured-player acceptance; four-human testhost only.

Failure inventory: route rewrites world state; two keyboard ports consume each
other's keys; A+B uses synthetic DOM events instead of native contacts; capture
counter decrements before landing; restoration changes visible pixels; pads
cover the court; touch buttons are outside the 320px viewport.
"""
import argparse, base64, hashlib, json, os, subprocess, threading, uuid
from pathlib import Path
from http.server import ThreadingHTTPServer, BaseHTTPRequestHandler
from urllib.parse import unquote

ROOT=Path(__file__).resolve().parents[2]
parser=argparse.ArgumentParser(description=__doc__)
parser.add_argument('--cooperative',action='store_true',help='same native sequence with two original AI opponents; do not force five escapes')
ARGS=parser.parse_args()
OUT=ROOT/('.local/basketball/captured-native-browser-cooperative' if ARGS.cooperative else '.local/basketball/captured-native-browser')
OUT.mkdir(parents=True,exist_ok=True)
env=dict(os.environ,AGENT_BROWSER_SESSION='bkc-'+uuid.uuid4().hex[:8],AGENT_BROWSER_SOCKET_DIR=str(ROOT/'.local/bkc-sockets'),TMPDIR=str(ROOT/'.local/tmp'),NODE_PATH=str(ROOT/'web/node_modules'))
Path(env['AGENT_BROWSER_SOCKET_DIR']).mkdir(exist_ok=True)
requests=[]
class Handler(BaseHTTPRequestHandler):
    def log_message(self,*args):pass
    def do_GET(self):
        route=unquote(self.path.split('?')[0]);requests.append(route)
        path={'/':ROOT/'scripts/basketball/captured_test_page.html','/capture.js':OUT/'capture.js','/capture.css':OUT/'capture.css','/runtime/wasm_exec.js':OUT/'wasm_exec.js','/runtime/basketball.wasm':OUT/'basketball.wasm'}.get(route)
        if route.startswith('/assets/'):
            rel=Path(route[8:])
            if rel.parts and '..' not in rel.parts and rel.parts[0] in ('court','courts','static-atlas','static-objects','raster','results') and rel.suffix in ('.json','.png'):
                allowed=(ROOT/'.local/basketball'/rel.parts[0]).resolve();candidate=(ROOT/'.local/basketball'/rel).resolve()
                if candidate.is_relative_to(allowed):path=candidate
        if path is None or not path.is_file():self.send_error(404);return
        content=path.read_bytes();self.send_response(200)
        self.send_header('Content-Type',{'.wasm':'application/wasm','.png':'image/png','.json':'application/json','.html':'text/html','.css':'text/css'}.get(path.suffix,'text/javascript'))
        self.send_header('Content-Length',str(len(content)));self.end_headers();self.wfile.write(content)

def browser(*args):
    r=subprocess.run(['agent-browser',*args,'--json'],env=env,capture_output=True,text=True,timeout=60)
    if r.returncode:raise RuntimeError(r.stdout+r.stderr)
    v=json.loads(r.stdout);assert v['success'],v;return v['data']

def main():
    checks=[];inspector=None;opened=False
    (OUT/'result.json').write_text(json.dumps({'status':'running'})+'\n')
    subprocess.run(['go','build','-o',str(OUT/'basketball.wasm'),'./games/basketball/cmd/browser-wasm'],cwd=ROOT,env=dict(env,GOOS='js',GOARCH='wasm'),check=True)
    goroot=subprocess.check_output(['go','env','GOROOT'],cwd=ROOT,env=env,text=True).strip();(OUT/'wasm_exec.js').write_bytes((Path(goroot)/'lib/wasm/wasm_exec.js').read_bytes())
    subprocess.run([str(ROOT/'web/node_modules/.bin/esbuild'),str(ROOT/'scripts/basketball/captured_browser_harness.ts'),'--bundle','--format=esm','--jsx=automatic','--loader:.svg=dataurl','--target=es2022','--outfile='+str(OUT/'capture.js')],cwd=ROOT,env=env,check=True)
    server=ThreadingHTTPServer(('127.0.0.1',0),Handler);threading.Thread(target=server.serve_forever,daemon=True).start()
    def cdp(method,params={}):
        inspector.stdin.write(json.dumps({'method':method,'params':params})+'\n');inspector.stdin.flush();r=json.loads(inspector.stdout.readline())
        if 'error' in r:raise RuntimeError(r['error'])
        return r['result']
    def js(expression):
        r=cdp('Runtime.evaluate',{'expression':expression,'returnByValue':True,'awaitPromise':True,'timeout':5000})
        if r.get('exceptionDetails'):raise RuntimeError(r['exceptionDetails'])
        return r['result'].get('value')
    def expect(name,condition):
        assert condition,name
        checks.append(name);(OUT/'checks.json').write_text(json.dumps(checks,ensure_ascii=False,indent=2)+'\n')
    keys={'W':('KeyW','w',87),'A':('KeyA','a',65),'D':('KeyD','d',68),'K':('KeyK','k',75),'Up':('ArrowUp','ArrowUp',38),'Right':('ArrowRight','ArrowRight',39),',':('Comma',',',188),'.':('Period','.',190)}
    held=set()
    def set_keys(names):
        names=set(names)
        for kind,items in [('keyUp',held-names),('keyDown',names-held)]:
            for name in sorted(items):
                code,key,num=keys[name];cdp('Input.dispatchKeyEvent',{'type':kind,'code':code,'key':key,'windowsVirtualKeyCode':num})
        held.clear();held.update(names)
    def save(name):
        snap=js('captureHost.save()');(OUT/(name+'.json')).write_text(json.dumps(snap,indent=2)+'\n')
        browser('screenshot',str(OUT/(name+'.png')))
        image=js('document.querySelector("canvas").toDataURL()');(OUT/(name+'-canvas.png')).write_bytes(base64.b64decode(image.split(',')[1]));return snap,image
    try:
        page='http://127.0.0.1:'+str(server.server_port)+'/'+('?cooperative=1' if ARGS.cooperative else '')
        browser('--profile',str(OUT/'profile'),'open',page);opened=True
        endpoint=browser('get','cdp-url')['cdpUrl'];inspector=subprocess.Popen(['node',str(ROOT/'scripts/basketball/browser_cdp_session.mjs'),endpoint,page],env=env,stdin=subprocess.PIPE,stdout=subprocess.PIPE,stderr=(OUT/'cdp.stderr').open('w'),text=True);assert json.loads(inspector.stdout.readline())['ready']
        browser('wait','--fn','!!window.captureHost')
        for mobile in (False,True):
            label='mobile-320' if mobile else 'desktop'
            browser('set','viewport',*(['320','760'] if mobile else ['1000','950']))
            cdp('Emulation.setTouchEmulationEnabled',{'enabled':mobile,'maxTouchPoints':2})
            browser('click','#start');js('captureHost.advance(114)')
            expect(label+' cold startup reaches original frame114',js('captureHost.view.state.videoFrames===114 && !!captureHost.view.state.playing'))
            # All movement/pickup/throw comes from actual DOM key events, sampled
            # by two unmodified production BasketballInput instances.
            stages=[(21,['W','Up']),(54,['A','Up']),(15,['A']),(6,['K']),(15,[]),(6,['D','K']),(60,[])]
            segments=[]
            for frames,names in stages:
                set_keys(names);segments.append({'frames':frames,'native_keys':names,'result':js(f'captureHost.advance({frames})')})
            set_keys([])
            row=js('captureHost.summary()')
            expect(label+' original thrown Kind10 captured player2',row['holder']==1 and row['capture']==4 and row['players'][1]['action']==80)
            expect(label+' captured actor is actually on camera',1 in row['display']['drawOrder'] and any(o['id']==1 and 16<=o['projected']['x']<240 and 32<=o['projected']['y']<184 for o in row['display']['objects']))
            checkpoint,pixels=save(label+'-captured')
            before=js('JSON.stringify(captureHost.view.state)')
            js('captureHost.restore('+json.dumps(checkpoint)+')')
            expect(label+' captured checkpoint restores exact state and pixels',js('JSON.stringify(captureHost.view.state)')==before and js('document.querySelector("canvas").toDataURL()')==pixels)
            layout=js('(()=>{const box=s=>{const r=document.querySelector(s).getBoundingClientRect();return{x:r.x,y:r.y,width:r.width,height:r.height,bottom:r.bottom,right:r.right}};return{canvas:box("canvas"),p0:box("#port-0"),p1:box("#port-1"),overflow:document.documentElement.scrollWidth>innerWidth,viewport:innerHeight}})()')
            expect(label+' controllers below canvas and inside viewport',layout['p0']['y']>=layout['canvas']['bottom'] and layout['p1']['y']>=layout['p0']['bottom'] and layout['p1']['bottom']<=layout['viewport'] and not layout['overflow'])
            for jump in range(5):
                if mobile:
                    points=js('(()=>{return ["a","b"].map((name,i)=>{const e=document.querySelector(`#port-1 [data-controller-button=${name}]`),r=e.getBoundingClientRect();return{x:r.x+r.width/2,y:r.y+r.height/2,id:i+1}})})()')
                    expect(f'{label} jump{jump+1} touch targets visible',all(0<p['x']<320 and 0<p['y']<760 for p in points))
                    cdp('Input.dispatchTouchEvent',{'type':'touchStart','touchPoints':points})
                else:set_keys([',','.'])
                row=js('captureHost.advance(6)')
                if not ARGS.cooperative or jump==0:expect(f'{label} jump{jump+1} fresh chord starts original54',row['players'][1]['action']==84 and row['capture']==4-jump)
                else:expect(f'{label} chord{jump+1} reaches runtime after AI intervention',js('captureHost.trace.at(-1).held[1]')==192)
                if jump==0:save(label+'-jump')
                if mobile:cdp('Input.dispatchTouchEvent',{'type':'touchEnd','touchPoints':[]})
                else:set_keys([])
                if ARGS.cooperative and jump==0:
                    row=js('captureHost.advance(60)');save(label+'-ai-interruption')
                    expect(label+' original AI interrupts capture at357 after one escape',row['video']==357 and row['holder']==-1 and row['capture']==3 and row['players'][1]['action']==72)
                    row=js('captureHost.advance(60)')
                else:row=js('captureHost.advance(120)')
                if not ARGS.cooperative:expect(f'{label} jump{jump+1} one landing decrements once',row['capture']==(3-jump)%256)
                if jump==3:save(label+('-continued-ai-play' if ARGS.cooperative else '-one-remaining'))
            if not ARGS.cooperative:expect(label+' fifth landing releases captured player',row['holder']==-1 and row['players'][1]['action']==0)
            else:expect(label+' AI intervention preserved incomplete escape count',row['holder']==-1 and row['capture']==3)
            save(label+'-escaped')
            x=row['players'][1]['body']['position']['x'];set_keys(['Right']);row=js('captureHost.advance(18)');set_keys([])
            if not ARGS.cooperative:expect(label+' released player walks normally',row['players'][1]['action']==1 and row['players'][1]['body']['position']['x']>x)
            else:
                x=row['players'][1]['body']['position']['x'];set_keys([])
                code,key,num=('ArrowLeft','ArrowLeft',37);cdp('Input.dispatchKeyEvent',{'type':'keyDown','code':code,'key':key,'windowsVirtualKeyCode':num})
                row=js('captureHost.advance(90)');cdp('Input.dispatchKeyEvent',{'type':'keyUp','code':code,'key':key,'windowsVirtualKeyCode':num})
                expect(label+' human directional input still moves after original AI contact and riding',row['video']==1029 and row['players'][1]['body']['position']['x']<x)
                save(label+'-continued-human-input')
            events=js('captureHost.events');trace=[]
            for offset in range(0,js('captureHost.trace.length'),30):trace.extend(js(f'captureHost.trace.slice({offset},{offset+30})'))
            expect(label+' every dispatched input event is native trusted',len(events)>20 and all(e['trusted'] for e in events))
            if mobile:expect(label+' five real two-finger chords reached shared pad',sum(e['type']=='pointerdown' and e.get('pointerType')=='touch' for e in events)==10)
            else:expect(label+' independent keyboard profiles both consumed input',any(t['held'][:2]==[8,8] for t in trace) and any(t['held'][:2]==[0,192] for t in trace))
            captures=[t['video'] for prev,t in zip(trace,trace[1:]) if prev.get('holder')==-1 and t.get('holder')==1]
            landings=[t['video'] for prev,t in zip(trace,trace[1:]) if prev.get('holder')==1 and t.get('capture')!=prev.get('capture')]
            expect(label+' actual native route matches verified runtime boundaries',captures==[246] and landings==([336] if ARGS.cooperative else [336,462,588,714,840]))
            (OUT/(label+'-trace.json')).write_text(json.dumps({'scope':'two-human cooperative testhost, original AI slots2/3; no network room' if ARGS.cooperative else 'four-human testhost; slots2/3 idle; no network room','mobile_scope':'keyboard navigation and throw on both ports, port1 native two-finger escape' if mobile else 'native keyboard on two independent profiles','segments':segments,'layout':layout,'events':events,'trace':trace,'world_injected':False},indent=2)+'\n')
        expect('no ROM or engine oracle fetched',not any('testdata' in r or 'fixture' in r or r.endswith('.nes') for r in requests))
        assets={p.name:hashlib.sha256(p.read_bytes()).hexdigest() for p in OUT.iterdir() if p.is_file() and p.suffix in ('.png','.json','.wasm') and p.name!='result.json'}
        (OUT/'result.json').write_text(json.dumps({'status':'passed','checks':checks,'scope':'two-human cooperative development host with original AI; not network or complete phone multiplayer' if ARGS.cooperative else 'two native inputs on four-human development host; not four-player platform support, network, or complete phone multiplayer','seed':'0x12345678','world_injected':False,'sha256':assets},ensure_ascii=False,indent=2)+'\n');print(json.dumps({'passed':len(checks),'output':str(OUT)}))
    except Exception as error:
        (OUT/'result.json').write_text(json.dumps({'status':'failed','error':str(error),'checks':checks},indent=2)+'\n');raise
    finally:
        (OUT/'requests.json').write_text(json.dumps(requests,indent=2)+'\n')
        if inspector:inspector.stdin.close();inspector.wait(timeout=10)
        if opened:browser('close')
        server.shutdown();server.server_close()
if __name__=='__main__':main()
