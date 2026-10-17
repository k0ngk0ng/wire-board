"""Browser-level logical controller acceptance, declared before implementation."""
import json,os,subprocess,threading,uuid,time
from pathlib import Path
from http.server import ThreadingHTTPServer,BaseHTTPRequestHandler

ROOT=Path(__file__).resolve().parents[2]
OUT=ROOT/'.local/basketball/input-browser';OUT.mkdir(parents=True,exist_ok=True)
env=dict(os.environ,AGENT_BROWSER_SESSION='bki-'+uuid.uuid4().hex[:6],AGENT_BROWSER_SOCKET_DIR=str(ROOT/'.local/bki-sockets'),TMPDIR=str(ROOT/'.local/tmp'))
Path(env['AGENT_BROWSER_SOCKET_DIR']).mkdir(exist_ok=True)
endpoint=None
page=None
inspector=None
def browser(*args):
    with (OUT/'commands.jsonl').open('a') as f:f.write(json.dumps(dict(at=time.time(),args=args))+'\n')
    r=subprocess.run(['agent-browser',*args,'--json'],env=env,capture_output=True,text=True,timeout=60)
    if r.returncode:raise RuntimeError(r.stdout+r.stderr)
    data=json.loads(r.stdout);assert data['success'],data
    return data['data']
def cdp(method,params):
    inspector.stdin.write(json.dumps(dict(method=method,params=params))+'\n');inspector.stdin.flush()
    result=json.loads(inspector.stdout.readline())
    if 'error' in result:raise RuntimeError(result['error'])
    return result['result']
def js(text):return cdp('Runtime.evaluate',dict(expression=text,returnByValue=True,awaitPromise=True,timeout=3000))['result'].get('value')
def native_key(key):
    if key=='Space':code,value,number='Space',' ',32
    elif key=='ArrowRight':code,value,number='ArrowRight','ArrowRight',39
    else:code,value,number='Key'+key.upper(),key,ord(key.upper())
    # Use native CDP edges; agent-browser 0.39 press/eval can stall after an
    # unbound key on this Chrome build. Never replace native input with a DOM
    # KeyboardEvent for the mapping acceptance itself.
    cdp('Input.dispatchKeyEvent',dict(type='keyDown',code=code,key=value,windowsVirtualKeyCode=number))
    cdp('Input.dispatchKeyEvent',dict(type='keyUp',code=code,key=value,windowsVirtualKeyCode=number))
class Handler(BaseHTTPRequestHandler):
    def log_message(self,*args):pass
    def do_GET(self):
        files={'/':ROOT/'scripts/basketball/input_test_page.html','/input-harness.js':OUT/'input-harness.js'}
        path=files.get(self.path)
        if path is None:self.send_error(404);return
        data=path.read_bytes();self.send_response(200);self.send_header('Content-Type','text/javascript' if path.suffix=='.js' else 'text/html');self.end_headers();self.wfile.write(data)

def main():
    global endpoint,page,inspector
    subprocess.run([str(ROOT/'web/node_modules/.bin/esbuild'),str(ROOT/'scripts/basketball/input_browser_harness.ts'),'--bundle','--format=esm','--target=es2022','--outfile='+str(OUT/'input-harness.js')],env=env,check=True)
    server=ThreadingHTTPServer(('127.0.0.1',0),Handler);thread=threading.Thread(target=server.serve_forever,daemon=True);thread.start();opened=False
    checks=[]
    def expect(name,value):
        assert value,name
        checks.append(name)
        (OUT/'checks.json').write_text(json.dumps(checks,indent=2)+'\n')
    try:
        page='http://127.0.0.1:'+str(server.server_port)+'/'
        browser('--profile',str(OUT/'profile'),'open',page);opened=True
        endpoint=browser('get','cdp-url')['cdpUrl']
        inspector=subprocess.Popen(['node',str(ROOT/'scripts/basketball/browser_cdp_session.mjs'),endpoint,page],env=env,stdin=subprocess.PIPE,stdout=subprocess.PIPE,stderr=(OUT/'cdp.stderr').open('w'),text=True)
        assert json.loads(inspector.stdout.readline())['ready']
        browser('wait','--fn','!!window.inputAcceptance')
        for key,mask in [('w',8),('s',4),('a',2),('d',1),('f',32),('Space',16),('j',64),('k',128)]:
            native_key(key);expect('native '+key,js('inputAcceptance.sample()')==mask);expect('released '+key,js('inputAcceptance.sample()')==0)
        browser('focus','#text');native_key('d');expect('editable blocked',js('inputAcceptance.sample()')==0)
        browser('focus','#game');js('document.querySelector("#settings").showModal()');native_key('k');expect('modal blocked',js('inputAcceptance.sample()')==0);js('document.querySelector("#settings").close()')
        browser('focus','#game');js('inputAcceptance.configureRight()');native_key('d');expect('old key removed',js('inputAcceptance.sample()')==0);native_key('ArrowRight');expect('personal mapping applied',js('inputAcceptance.sample()')==1)
        mixed=js('''(()=>{const passed=[];const check=(name,value)=>{if(!value)throw Error(name);passed.push(name)};
          inputAcceptance.reset();inputAcceptance.key("keydown","KeyD");inputAcceptance.pointer(7,["right"]);inputAcceptance.sample(0);inputAcceptance.key("keyup","KeyD");
          check("mixed source retains held button",inputAcceptance.sample(1)===1);
          inputAcceptance.releasePointer(7);check("last source releases",inputAcceptance.sample(2)===0);
          inputAcceptance.pointer(9,["a"]);inputAcceptance.releasePointer(9,true);check("cancelled touch does not fire",inputAcceptance.sample(3)===0);
          inputAcceptance.clearCancellations();inputAcceptance.key("keydown","KeyD");inputAcceptance.pointer(10,["a"]);inputAcceptance.sample(4);inputAcceptance.releasePointer(10,true);
          check("pointer cancellation identifies only its own logical bit",JSON.stringify(inputAcceptance.cancellations())==="[128]" && inputAcceptance.sample(5)===1);
          inputAcceptance.pointer(11,["right"]);inputAcceptance.clearCancellations();inputAcceptance.releasePointer(11,true);check("shared key is not cancelled",inputAcceptance.cancellations().length===0 && inputAcceptance.sample(6)===1);inputAcceptance.key("keyup","KeyD");
          inputAcceptance.pointer(12,["turboA"],0);check("pointer turbo begins immediately",inputAcceptance.sample(0)===128);check("pointer turbo has original profile gap",inputAcceptance.sample(50)===0);check("pointer turbo repeats",inputAcceptance.sample(100)===128);inputAcceptance.releasePointer(12,true);
          inputAcceptance.key("keydown","KeyU");inputAcceptance.sample();window.dispatchEvent(new Event("blur"));check("blur stops turbo",inputAcceptance.sample()===0);
          inputAcceptance.key("keydown","KeyD");inputAcceptance.enable(false);inputAcceptance.enable(true);inputAcceptance.key("keydown","KeyD",true);check("disable requires fresh press",inputAcceptance.sample()===0);
          return passed;})()''')
        for name in mixed:expect(name,True)
        js('inputAcceptance.key("keydown","KeyD");document.querySelector("#text").focus()');expect('focus into an editor cancels already held input',js('inputAcceptance.sample()')==0);browser('focus','#game')
        js('inputAcceptance.key("keyup","KeyD")');browser('click','#a');expect('native pointer short tap',js('inputAcceptance.sample()')==128);expect('native pointer release',js('inputAcceptance.sample()')==0)
        js('inputAcceptance.pointer(8,["right"]);inputAcceptance.configureRight()');expect('profile switch clears every source',js('inputAcceptance.sample()')==0)
        js('inputAcceptance.dispose()');browser('focus','#game');native_key('ArrowRight');expect('disposed listener removed',js('inputAcceptance.sample()')==0)
        browser('set','viewport','320','720');browser('screenshot',str(OUT/'mobile.png'))
        expect('320px no overflow',js('document.documentElement.scrollWidth<=innerWidth'))
        (OUT/'result.json').write_text(json.dumps(dict(status='passed',checks=checks,scope='real browser input boundary; not full game acceptance'),indent=2)+'\n')
        print(json.dumps(dict(status='passed',checks=len(checks))))
    finally:
        if inspector:inspector.stdin.close();inspector.wait(timeout=10)
        if opened:browser('close')
        server.shutdown();server.server_close();thread.join(timeout=5)

if __name__=='__main__':main()
