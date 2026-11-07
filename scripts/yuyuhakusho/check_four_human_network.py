"""Four independent authenticated browsers, real input replay and slow-peer QA.

Frozen budgets reuse the accepted realtime scope: >=50 visible updates/s and
p95 frame interval <=40ms. Native input response and ACK are measured separately.
No game state, input window, render result or transport packet is substituted.
"""
from pathlib import Path
import argparse, concurrent.futures, hashlib, json, os, statistics, subprocess

ROOT = Path(__file__).resolve().parents[2]
p = argparse.ArgumentParser(description=__doc__)
p.add_argument('--dist', type=Path, required=True)
p.add_argument('--label', required=True)
args = p.parse_args()
DIST = args.dist.resolve()
assert DIST.is_relative_to(ROOT/'.local')
OUT = ROOT/'games/yuyuhakusho/verification/four-human-network'/args.label
TMP = ROOT/'.local/md-yuyuhakusho/four-human-network'/args.label
OUT.mkdir(parents=True, exist_ok=False)
TMP.mkdir(parents=True, exist_ok=False)
socket = (ROOT.parents[1] if ROOT.parent.name == 'worktrees' else ROOT)/'.local/au'
env = dict(os.environ, AGENT_BROWSER_SOCKET_DIR=str(socket))
suffix = hashlib.sha256(args.label.encode()).hexdigest()[:4]
sessions = []
report = {'pass':False, 'scope':'Four distinct signed-in human seats in independent Chromium browsers on this Mac; actual HTTP/SQLite/WebSocket and Go/Wasm. One real TCP connection is then delayed120ms per write. This does not claim four physical devices or arbitrary public networks.', 'budget':{'minimum_visible_fps':50,'maximum_p95_ms':40,'maximum_input_visible_ms':75}, 'stages':{}}
def browser(who, *items):
    e = dict(env, AGENT_BROWSER_SESSION='q'+suffix+str(who))
    r = subprocess.run(['agent-browser', *items, '--json'], cwd=ROOT, env=e, text=True, capture_output=True, timeout=55)
    if r.returncode: raise RuntimeError(r.stdout+r.stderr)
    value = json.loads(r.stdout)
    assert value['success'], value
    return value['data']
def js(who, code): return browser(who, 'eval', code)['result']
def wait(who, code): return browser(who, 'wait', '--fn', code)
def click(who, name): return browser(who, 'find','role','button','click','--name',name,'--exact')
def state(who): return js(who,'fetch("/api/state").then(r=>r.json())')
def native(who, actions):
    endpoint = browser(who,'get','cdp-url')['cdpUrl']
    url = urls['shaped' if who==3 else 'direct']
    return json.loads(subprocess.check_output(['node',str(ROOT/'scripts/yuyuhakusho/native_app_input.mjs'),endpoint,url,json.dumps(actions)],cwd=ROOT,env=env,text=True))
def register(who):
    url = urls['shaped' if who==3 else 'direct']
    browser(who,'--profile',str(TMP/('browser-'+str(who))),'open',url)
    sessions.append(who)
    browser(who,'set','viewport','1100','1000')
    wait(who,'!!document.querySelector("input[name=password]")')
    js(who,(ROOT/'scripts/yuyuhakusho/platform_app_observer.js').read_text())
    click(who,'创建账号')
    for key,value in [('name','Four'+suffix+str(who)),('password','LocalTestOnly01'),('invite','yuyu-local-e2e')]:
        browser(who,'fill','input[name='+key+']',value)
    click(who,'创建账号，入座')
    wait(who,'!document.querySelector("input[name=password]")')
def measure(who, stage):
    browser(who,'focus','[data-yuyu-board]')
    left = who>=2
    code,key,vk,mask,direction = ('KeyA','a',65,4,-1) if left else ('KeyD','d',68,8,1)
    actions=[{'type':'keyDown','key':key,'code':code,'windowsVirtualKeyCode':vk,'wait':140}]
    if stage.startswith('busy'):
        for _ in range(4):
            actions += [{'type':'keyDown','key':'k','code':'KeyK','windowsVirtualKeyCode':75,'wait':55},{'type':'keyDown','key':'j','code':'KeyJ','windowsVirtualKeyCode':74,'wait':55},{'type':'keyUp','key':'k','code':'KeyK','windowsVirtualKeyCode':75,'wait':55},{'type':'keyUp','key':'j','code':'KeyJ','windowsVirtualKeyCode':74,'wait':55}]
        actions += [{'type':'keyUp','key':key,'code':code,'windowsVirtualKeyCode':vk,'wait':1980}]
    else: actions += [{'type':'keyUp','key':key,'code':code,'windowsVirtualKeyCode':vk,'wait':2860}]
    data = native(who,actions)
    (OUT/(stage+'-seat'+str(who)+'.json')).write_text(json.dumps(data,indent=2)+'\n')
    paints=[]
    for row in data['paints']:
        if not paints or paints[-1].get('frame')!=row.get('frame'): paints.append(row)
    elapsed=data['now']-data['start']
    spans=sorted(b['at']-a['at'] for a,b in zip(paints,paints[1:]))
    event=next(k for k in data['keys'] if k.get('code')==code and k['type']=='keydown' and k['trusted'])
    initial=float(data['before']['board']['localX'])
    response=next((r for r in paints if r['at']>=event['at'] and (r.get('x',initial)-initial)*direction>0 and r['hash']!=data['before']['paint']['hash']),None)
    sent=next((r for r in data['inputs'] if r['at']>=event['at'] and r['action'].get('buttons',0)&mask),None)
    ack=next((r for r in data['acks'] if sent and r.get('nonce')==sent.get('nonce')),None)
    applied=next((r for r in data['views'] if r['view']['inputs'][who]['appliedButtons']&mask),None)
    view_bytes=[r['bytes'] for r in data['views']]
    result={'seat':who,'hidden':data['hidden'],'visible_frames':len(paints),'elapsed_ms':elapsed,'visible_fps':len(paints)*1000/elapsed,'p95_ms':spans[int(.95*(len(spans)-1))]if spans else None,
        'native_input_to_visible_ms':response['at']-event['at']if response else None,'input_ack_ms':ack['at']-sent['at']if ack and sent else None,'native_input_to_authority_ms':applied['at']-event['at']if applied else None,
        'local_before_ack':bool(response and ack and response['at']<ack['at']),'view_messages':len(view_bytes),'mean_view_bytes':statistics.mean(view_bytes)if view_bytes else None,'max_view_bytes':max(view_bytes)if view_bytes else None,
        'incoming_view_bytes_per_second':sum(view_bytes)*1000/elapsed,'outgoing_input_bytes':sum(len(json.dumps(r,separators=(',',':')).encode())for r in data['inputs']),'wire_checkpoints':sum('checkpoint'in r['view'] or 'snapshot'in r['view'] for r in data['views']), 'recoveries':js(who,'yuyuAppObservation.recoveries??[]')}
    assert sent and ack and applied, result
    if not stage.startswith('busy'):
        assert response and result['native_input_to_visible_ms']<=75, result
    assert not result['hidden'] and result['visible_fps']>=50 and result['p95_ms']<=40, result
    assert result['wire_checkpoints']==0 and all('replay'in r['view']for r in data['views']),result
    wait(who,'yuyuAppObservation.latestView.inputs['+str(who)+'].appliedButtons===0')
    return result
report['app_build']=json.loads((DIST/'game-load-manifest.json').read_bytes())['build']
report['runtime']=json.loads((DIST/'yuyuhakusho/v1/runtime/build.json').read_bytes())
host=None
try:
    host=subprocess.Popen([str(ROOT/'.local/md-yuyuhakusho/review-polish-final/host'),'-build',str(DIST),'-data',str(TMP/'database')],cwd=ROOT,env=env,stdin=subprocess.PIPE,stdout=subprocess.PIPE,stderr=(OUT/'host.log').open('w'),text=True)
    urls=json.loads(host.stdout.readline());report['urls']=urls
    register(0)
    click(0,'创建房间');browser(0,'find','role','button','click','--name','魔强统一战')
    wait(0,'!!document.querySelector("[aria-label=房间设置] input[name=name]")')
    browser(0,'select','[aria-label=座位上限]','4')
    for arena in [2,0]:
        browser(0,'click','[data-arena-picker] summary');wait(0,'document.querySelectorAll("[data-arena-option]").length===8')
        browser(0,'click','[data-arena-option="'+str(arena)+'"]')
        wait(0,'document.querySelector("[data-arena-picker]").dataset.arena==="'+str(arena)+'"')
    browser(0,'fill','input[name=name]','四真人输入同步验收')
    browser(0,'click','[aria-label=房间设置] form button.primary')
    wait(0,'!!document.querySelector("[aria-label=比赛准备]")')
    for who in [1,2,3]:
        register(who);wait(who,'[...document.querySelectorAll("button")].some(b=>b.textContent.includes("加入牌桌"))');click(who,'加入牌桌');wait(who,'!!document.querySelector("[aria-label=比赛准备]")')
    room=state(0)['room']
    assert room['capacity']==4 and len(room['seats'])==4 and not any(s.get('bot')or s.get('autoPlay') for s in room['seats']),room
    report['human_seats']=4;report['characters']=room['options']['characters']
    for who in [1,2,3,0]:click(who,'我准备好了')
    wait(0,'[...document.querySelectorAll("button")].some(b=>b.textContent.trim()==="开始游戏"&&!b.disabled)')
    click(0,'开始游戏')
    for who in range(4):wait(who,'document.querySelector("[data-yuyu-board]")?.dataset.connection==="connected"&&BigInt(document.querySelector("[data-yuyu-board]").dataset.authorityFrame)>280n&&yuyuAppObservation.latestView?.phase==="playing"')
    for who in range(4):
        installed = js(who, """(()=>{const el=document.querySelector('[data-yuyu-board]');let f=el[Object.keys(el).find(k=>k.startsWith('__reactFiber$'))];for(;f;f=f.return){for(let h=f.memoizedState,n=0;h&&n<100;h=h.next,n++){const c=h.memoizedState?.current;if(c&&typeof c.recover==='function'&&c.scene){const original=c.recover;yuyuAppObservation.recoveries=[];c.recover=function(reason){yuyuAppObservation.recoveries.push({at:performance.now(),reason:String(reason),view:this.view?.frame,confirmed:String(this.scene?.authorityFrame),transportConnected:this.transport?.connected,pending:this.transport?.pendingCount});return Reflect.apply(original,this,[reason]);};return true;}}}return false})()""")
        assert installed, 'passive recovery observer missing'
    checkpoint=js(0,'fetch("/api/rooms/'+room['id']+'/sync").then(r=>r.json())')
    (OUT/'initial-checkpoint.json').write_text(json.dumps(checkpoint,ensure_ascii=False)+'\n')
    print('Four real human Boards ready',flush=True)
    for stage,delay in [('normal',0),('one-slow',120),('busy-normal',0),('busy-one-slow',120)]:
        host.stdin.write(json.dumps({'delayMs':delay})+'\n');host.stdin.flush();assert json.loads(host.stdout.readline())['delayMs']==delay
        with concurrent.futures.ThreadPoolExecutor(max_workers=4) as pool:
            futures=[pool.submit(measure,who,stage)for who in range(4)]
            results=[f.result()for f in futures]
        report['stages'][stage]=results
        if delay and not stage.startswith('busy'):assert results[3]['local_before_ack'],results[3]
        print(json.dumps({'stage':stage,'fps':[round(r['visible_fps'],2)for r in results],'native_ms':[round(r['native_input_to_visible_ms'],2)if r['native_input_to_visible_ms']is not None else None for r in results]}),flush=True)
    for who in range(4):
        browser(who,'screenshot',str(OUT/('seat'+str(who)+'.png')))
        report.setdefault('checkpoints',[]).append({'seat':who,'requests':js(who,'yuyuAppObservation.fetches.filter(r=>r.url.endsWith("/sync")).length'),'recoveries':js(who,'yuyuAppObservation.recoveries??[]')})
    current=state(0)['room']
    assert not any(s.get('bot')or s.get('autoPlay') for s in current['seats'])
    report['pass']=True
except Exception as error:
    report['error']=repr(error)
    for who in sessions:
        try:
            report.setdefault('diagnostics',{})[str(who)]=js(who,'({board:document.querySelector("[data-yuyu-board]")?{...document.querySelector("[data-yuyu-board]").dataset}:null,text:document.body.innerText.slice(-1400),recoveries:yuyuAppObservation.recoveries,fetches:yuyuAppObservation.fetches.slice(-20)})')
            browser(who,'screenshot',str(OUT/('failure-seat'+str(who)+'.png')))
        except Exception:pass
    raise
finally:
    report['sources']={str(p.relative_to(ROOT)):hashlib.sha256(p.read_bytes()).hexdigest()for p in [Path(__file__),ROOT/'scripts/yuyuhakusho/platform_app_observer.js',ROOT/'scripts/yuyuhakusho/native_app_input.mjs',ROOT/'games/yuyuhakusho/web/network-scene.ts',ROOT/'games/yuyuhakusho/web/network-client.ts',ROOT/'games/yuyuhakusho/web/types.ts']}
    (OUT/'result.json').write_text(json.dumps(report,ensure_ascii=False,indent=2)+'\n')
    for who in sessions:
        try:browser(who,'close')
        except Exception:pass
    if host:host.terminate();host.wait(timeout=10)
print('Four-human realtime acceptance passed')
