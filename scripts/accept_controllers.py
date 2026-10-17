"""Browser acceptance written before the settings UI.

Scenarios: native key capture and duplicate rejection; saving/reloading;
independent controller models and accounts; 320px visible controls/no overflow.
The FC faceplate uses non-overlapping physical hit regions; the separate FC
acceptance verifies its geometry and native gestures. Generic mapping rows keep
their former 40 px target check.
Uses only the fixture account and its own agent-browser session/profile.
"""
import argparse,json,os,re,subprocess,tempfile,time,uuid
from pathlib import Path

p=argparse.ArgumentParser();p.add_argument('url',nargs='?');args=p.parse_args()
root=Path.cwd();out=root/'.local/controllers/browser';out.mkdir(parents=True,exist_ok=True)
env=dict(os.environ,AGENT_BROWSER_SESSION='ctrl-'+uuid.uuid4().hex[:6],AGENT_BROWSER_SOCKET_DIR=str(root/'.local/controllers/sockets'),TMPDIR=str(root/'.local/tmp'))
Path(env['AGENT_BROWSER_SOCKET_DIR']).mkdir(parents=True,exist_ok=True)
def browser(*a):
    r=subprocess.run(['agent-browser',*a,'--json'],env=env,capture_output=True,text=True)
    if r.returncode:raise RuntimeError(r.stdout+r.stderr)
    d=json.loads(r.stdout);assert d.get('success'),d
    return d.get('data')
def js(source):return browser('eval',source)
def key_text():return js('document.querySelector('+json.dumps('[aria-label="修改 右 按键"]')+').textContent')
def auth(name):
    js('(async()=>{await fetch("/api/logout",{method:"POST"});const r=await fetch("/api/register",{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify('+json.dumps({'name':name,'password':'controller-browser-test','invite':'test-invite'})+')});if(!r.ok)throw Error(await r.text());return true})()')
    browser('reload');browser('wait','[aria-label="控制器设置"]')
server=None;data=None;opened=False
try:
    if not args.url:
        data=tempfile.TemporaryDirectory(dir=out,prefix='data-')
        server_env=dict(env,ADDR='127.0.0.1:0',DATA_DIR=data.name,INVITE_CODE='test-invite',PUBLIC_ORIGIN='',ASSETS_BASE_URL='',COOKIE_SECURE='false',ADMIN_USERNAME='ControllerRoot',ADMIN_PASSWORD='controller-root-only',ADMIN_EXISTING_USERNAME='')
        with (out/'server.log').open('w') as log:
            server=subprocess.Popen([str(root/'.local/controllers/wire-board')],cwd=root,env=server_env,stdout=log,stderr=subprocess.STDOUT)
        for _ in range(200):
            match=re.search(r'listening on (127\.0\.0\.1:\d+)',(out/'server.log').read_text())
            if match:args.url='http://'+match[1];break
            assert server.poll() is None,'fixture server exited'
            time.sleep(.05)
        assert args.url,'fixture failed to start'
    assert args.url.startswith('http://127.0.0.1:'),'Use only a local fixture server'
    browser('--profile',str(out/'profile'),'open',args.url)
    opened=True
    auth('ControllerBrowserA')
    browser('click','[aria-label="控制器设置"]');browser('wait','[aria-label="修改 右 按键"]')
    browser('click','[aria-label="修改 右 按键"]');browser('press','w')
    browser('wait','--text','每个按键只能对应一个操作')
    browser('press','ArrowRight');browser('click','[data-controller-save]');browser('wait','--text','控制器设置已保存')
    browser('select','[aria-label="控制器型号"]','classic-4')
    result=key_text()
    assert result.get('result')=='D',result
    browser('select','[aria-label="控制器型号"]','classic-2')
    browser('reload');browser('wait','[aria-label="控制器设置"]');browser('click','[aria-label="控制器设置"]');browser('wait','[aria-label="修改 右 按键"]')
    result=key_text();assert result.get('result')=='→',result
    browser('find','role','button','click','--name','恢复此型号默认键位','--exact')
    browser('click','[data-controller-save]');browser('wait','--text','控制器设置已保存')
    result=key_text();assert result.get('result')=='D',result
    browser('scrollintoview','[aria-label="控制器型号"]')
    browser('screenshot',str(out/'desktop.png'))
    browser('set','viewport','320','720');browser('screenshot',str(out/'mobile-320.png'))
    result=js('({overflow:document.documentElement.scrollWidth>innerWidth,visible:[...document.querySelectorAll(".controller-key")].every(e=>{const r=e.getBoundingClientRect();const minimum=e.closest("[data-controller-layout=fc]")?18:40;return r.width>=minimum&&r.height>=minimum&&r.left>=0&&r.right<=innerWidth})})')
    assert result['result']=={'overflow':False,'visible':True},result
    for key in ['上','下','左','右','选择（Select）','开始（Start）','B','A','Turbo B','Turbo A']:
        selector='[aria-label="修改 '+key+' 按键"]'
        browser('scrollintoview',selector)
        result=js('(()=>{const e=document.querySelector('+json.dumps(selector)+');const r=e.getBoundingClientRect();const hit=document.elementFromPoint(r.x+r.width/2,r.y+r.height/2);const minimum=e.closest("[data-controller-layout=fc]")?18:40;return r.width>=minimum&&r.height>=minimum&&!!hit&&(e===hit||e.contains(hit))})()')
        assert result['result'],f'Key is not visibly reachable: {key}'
    browser('scrollintoview','[data-controller-save]')
    browser('screenshot',str(out/'mobile-320-bottom.png'))
    contrasts=js(r'''(()=>{const rgb=s=>(s.match(/[\d.]+/g)||[]).map(Number),lum=c=>c.slice(0,3).map(v=>v/255).map(v=>v<=.04045?v/12.92:((v+.055)/1.055)**2.4).reduce((s,v,i)=>s+v*[.2126,.7152,.0722][i],0);return [...document.querySelectorAll('.controller-key:not(.fc-control),.fc-control kbd,.controller-model select,.controller-intro,.controller-help,.controller-notice')].map(e=>{let bg=[255,255,255],chain=[];for(let n=e;n;n=n.parentElement)chain.unshift(n);for(const n of chain){const c=rgb(getComputedStyle(n).backgroundColor),a=c[3]??1;bg=bg.map((v,i)=>c[i]*a+v*(1-a))};const a=lum(rgb(getComputedStyle(e).color)),b=lum(bg);const border=lum(rgb(getComputedStyle(e).borderTopColor));return {text:e.textContent.trim(),ratio:(Math.max(a,b)+.05)/(Math.min(a,b)+.05),borderRatio:e.matches('button,select')?(Math.max(border,b)+.05)/(Math.min(border,b)+.05):null}})})()''')
    assert all(c['ratio']>=4.5 for c in contrasts['result']),contrasts
    assert all(c['borderRatio'] is None or c['borderRatio']>=3 for c in contrasts['result']),contrasts
    (out/'contrast.json').write_text(json.dumps(contrasts['result'],ensure_ascii=False,indent=2)+'\n')
    auth('ControllerBrowserB');browser('click','[aria-label="控制器设置"]');browser('wait','[aria-label="修改 右 按键"]')
    result=key_text();assert result.get('result')=='D',result
    (out/'result.json').write_text(json.dumps({'status':'passed','native_key_capture':True,'duplicates_rejected':True,'reload_persistence':True,'restore_defaults':True,'default_right':'KeyD','model_isolation':True,'account_isolation':True,'mobile_width':320,'all_keys_reachable':True,'text_contrast_at_least':4.5},indent=2)+'\n')
except Exception as error:
    (out/'result.json').write_text(json.dumps({'status':'failed','error':repr(error)},indent=2)+'\n')
    if opened:
        try:
            browser('screenshot',str(out/'failure.png'))
            (out/'failure-snapshot.json').write_text(json.dumps(browser('snapshot','-i'),ensure_ascii=False,indent=2))
        except Exception:pass
    raise
finally:
    if opened:browser('close')
    if server:
        server.terminate();server.wait(timeout=10)
    if data:data.cleanup()
