"""Real normal-app/auth/SQLite/socket/browser acceptance, with TCP write shaping."""
from pathlib import Path
import argparse, hashlib, json, os, subprocess, statistics, time

ROOT=Path(__file__).resolve().parents[2]
parser=argparse.ArgumentParser(description=__doc__)
parser.add_argument('--label',required=True)
parser.add_argument('--performance',action='store_true')
parser.add_argument('--layout-only',action='store_true')
args=parser.parse_args()
OUT=ROOT/'games/yuyuhakusho/verification/platform-app-browser'/args.label
TMP=ROOT/'.local/md-yuyuhakusho/platform-app'/('acceptance-'+args.label)
OUT.mkdir(parents=True,exist_ok=False)
FROZEN_SOURCE_PATHS=[ROOT/'scripts/yuyuhakusho/platform_app_host.go',ROOT/'scripts/yuyuhakusho/platform_app_observer.js',ROOT/'scripts/yuyuhakusho/native_app_input.mjs',Path(__file__)]
FROZEN_SOURCE_SHA256={str(p.relative_to(ROOT)):hashlib.sha256(p.read_bytes()).hexdigest() for p in FROZEN_SOURCE_PATHS}
(OUT/'check_platform_app.executed.py.txt').write_bytes(Path(__file__).read_bytes())
for path in [TMP/'tmp',ROOT/'.local/ya']:path.mkdir(parents=True,exist_ok=True)
env=dict(os.environ,TMPDIR=str(TMP/'tmp'),GOTMPDIR=str(TMP/'tmp'),GOCACHE=str(ROOT/'.local/go-cache'),GOMODCACHE=str(ROOT/'.local/go-mod'),TEST_TELEMETRY_DIR=str(ROOT/'.local/go-telemetry'),AGENT_BROWSER_SOCKET_DIR=str(ROOT/'.local/ya'))
report={'pass':False,'scope':'Normal Vite app + registered composition root + real authentication, SQLite, HTTP and WebSocket. Shaping delays real TCP writes and closes actual connections. Virtual Gamepad API only; no physical hardware claim.'}
report['appBuild']=json.loads((ROOT/'.local/md-yuyuhakusho/platform-app/dist/game-load-manifest.json').read_text())['build']
report['runtimeBuild']=json.loads((ROOT/'.local/md-yuyuhakusho/platform-app/dist/yuyuhakusho/v1/runtime/build.json').read_text())
sessions=[]
def browser(who,*items):
    task_env=dict(env,AGENT_BROWSER_SESSION='ua'+hashlib.sha256(args.label.encode()).hexdigest()[:6]+who[0])
    result=subprocess.run(['agent-browser',*items,'--json'],cwd=ROOT,env=task_env,text=True,capture_output=True,timeout=55)
    value=json.loads(result.stdout)
    if result.returncode or not value.get('success'):raise RuntimeError(result.stdout+result.stderr)
    return value['data']
def js(who,code):return browser(who,'eval',code)['result']
def wait(who,code):return browser(who,'wait','--fn',code)
def wait_input(who,seat,buttons):return wait(who,'yuyuAppObservation.latestView.inputs['+str(seat)+'].appliedButtons==='+str(buttons))
def click(who,name):return browser(who,'find','role','button','click','--name',name,'--exact')
def snapshot(who,name):
    data=browser(who,'snapshot','-i')
    (OUT/(name+'.json')).write_text(json.dumps(data,ensure_ascii=False,indent=2)+'\n')
    return data
def state(who):return js(who,'fetch("/api/state").then(r=>r.json())')
def native(who,actions):
    endpoint=browser(who,'get','cdp-url')['cdpUrl']
    return json.loads(subprocess.check_output(['node',str(ROOT/'scripts/yuyuhakusho/native_app_input.mjs'),endpoint,urls['direct' if who=='fast' else 'shaped'],json.dumps(actions)],cwd=ROOT,env=env,text=True))
def key(code,key,number,held=120,tail=300):
    return [{'type':'keyDown','key':key,'code':code,'windowsVirtualKeyCode':number,'wait':held},{'type':'keyUp','key':key,'code':code,'windowsVirtualKeyCode':number,'wait':tail}]
def measure(who,name,code,key_name,number,seat,mask,direction,held=120,tail=300):
    browser(who,'focus','[data-yuyu-board]')
    observed=native(who,key(code,key_name,number,held,tail))
    (OUT/(name+'.json')).write_text(json.dumps(observed,indent=2)+'\n')
    event=next(k for k in observed['keys'] if k.get('code')==code and k['type']=='keydown' and k['trusted'])
    initial=float(observed['before']['board']['localX'])
    visible=next((p for p in observed['paints'] if p['at']>=event['at'] and (p.get('x',initial)-initial)*direction>0 and p['hash']!=observed['before']['paint']['hash']),None)
    assert visible is not None,{'case':name,'initial':initial,'data':observed}
    sent=next(i for i in observed['inputs'] if i['at']>=event['at'] and i['action'].get('buttons',0)&mask)
    ack=next((a for a in observed['acks'] if a.get('nonce')==sent['nonce']),None)
    applied=next((v for v in observed['views'] if v['view']['inputs'][seat]['appliedSequence']>=sent['action']['sequence'] and v['view']['inputs'][seat]['appliedButtons']&mask),None)
    intervals=[b['at']-a['at'] for a,b in zip(observed['paints'],observed['paints'][1:]) if b.get('frame')!=a.get('frame')]
    result={'nativeEventToFirstVisibleMs':visible['at']-event['at'],'inputAckMs':ack['at']-sent['at'] if ack else None,'nativeEventToAuthoritativeAppliedArrivalMs':applied['at']-event['at'] if applied else None,'firstVisibleBeforeAck':ack is not None and visible['at']<ack['at'],'renderFrameIntervalMedianMs':statistics.median(intervals) if intervals else None,'renderFrameIntervalMaxMs':max(intervals) if intervals else None,'initialX':initial,'firstVisibleX':visible['x'],'firstVisibleHash':visible['hash'],'beforeHash':observed['before']['paint']['hash'],'trusted':True}
    report.setdefault('measurements',{})[name]=result
    assert ack and ack['status']==200,{'case':name,'ack':ack}
    assert applied,{'case':name,'applied':applied}
    wait(who,'yuyuAppObservation.latestView.inputs['+str(seat)+'].appliedButtons===0')
    return result
def rtt(who):
    return js(who,'(async()=>{const values=[];for(let i=0;i<3;i++){const t=performance.now();await (await fetch("/api/state",{cache:"no-store"})).arrayBuffer();const entry=performance.getEntriesByType("resource").filter(e=>e.name.endsWith("/api/state")).at(-1);values.push({mainThreadObservedMs:performance.now()-t,networkResourceDurationMs:entry?.duration,responseTransferMs:entry?entry.responseEnd-entry.requestStart:null})}const state=await (await fetch("/api/state")).json();return {http:values,platformWebSocketRTT:state.network??null}})()')
def performance_check(who,name=None):
    name=name or who
    observed=native(who,[{'wait':2500}])
    (OUT/(name+'-render-performance.json')).write_text(json.dumps(observed,indent=2)+'\n')
    paints=[]
    for paint in observed['paints']:
        if not paints or paints[-1].get('frame')!=paint.get('frame'):paints.append(paint)
    intervals=sorted(b['at']-a['at'] for a,b in zip(paints,paints[1:]))
    elapsed=observed['now']-observed['start']
    result={'frames':len(paints),'elapsedMs':elapsed,'visibleUpdatesPerSecond':len(paints)*1000/elapsed,'medianFrameIntervalMs':statistics.median(intervals) if intervals else None,'p95FrameIntervalMs':intervals[min(len(intervals)-1,int(len(intervals)*.95))] if intervals else None,'budget':{'minimumVisibleUpdatesPerSecond':50,'maximumP95FrameIntervalMs':40},'hidden':observed['hidden']}
    report.setdefault('renderPerformance',{})[name]=result
    assert not observed['hidden'] and result['visibleUpdatesPerSecond']>=50 and result['p95FrameIntervalMs']<=40,result
def layouts():
    results=[]
    js('fast','window.layoutAcceptanceCanvas=document.querySelector("canvas");true')
    for label,width,height,fullscreen in [('desktop',1100,1000,False),('portrait-320',320,740,False),('landscape-740',740,360,True)]:
        browser('fast','set','viewport',str(width),str(height))
        if fullscreen:
            click('fast','全屏');wait('fast','!!document.querySelector("[data-room-fullscreen]")')
        browser('fast','focus','[data-yuyu-board]')
        native('fast',[{'wait':100}])
        geometry=js('fast','(()=>{const rect=s=>{const r=document.querySelector(s).getBoundingClientRect();return{x:r.x,y:r.y,left:r.left,right:r.right,top:r.top,bottom:r.bottom,width:r.width,height:r.height}};return{canvas:rect("canvas"),pad:rect("[data-controller-layout=md]"),c:rect("[data-controller-button=c]"),width:innerWidth,height:innerHeight,overflow:document.documentElement.scrollWidth>innerWidth,nativeFullscreen:!!document.fullscreenElement,frame:document.querySelector("[data-yuyu-board]").dataset.frame}})()')
        assert not geometry['overflow'] and geometry['pad']['top']>=geometry['canvas']['bottom'],geometry
        assert abs(geometry['canvas']['width']/geometry['canvas']['height']-320/224)<.01,geometry
        assert geometry['canvas']['top']>=0 and geometry['pad']['bottom']<=height+1,geometry
        c=geometry['c'];x,y=c['x']+c['width']/2,c['y']+c['height']/2
        observed=native('fast',[{'method':'Input.dispatchTouchEvent','type':'touchStart','touchPoints':[{'x':x,'y':y,'id':1}],'wait':120},{'method':'Input.dispatchTouchEvent','type':'touchEnd','touchPoints':[],'wait':200}])
        assert any(k['trusted'] and k['type']=='pointerdown' for k in observed['keys'])
        assert any(v['view']['inputs'][0]['appliedButtons']&32 for v in observed['views']),label
        wait_input('fast',0,0)
        browser('fast','screenshot',str(OUT/('layout-'+label+'.png')))
        (OUT/('layout-'+label+'-touch.json')).write_text(json.dumps(observed,indent=2)+'\n')
        if fullscreen:
            browser('fast','press','Escape');wait('fast','!document.querySelector("[data-room-fullscreen]")')
            assert js('fast','window.layoutAcceptanceCanvas===document.querySelector("canvas")')
        results.append({'label':label,'geometry':geometry,'nativeTouchC':True})
    report['layouts']=results
def network(delay=0,disconnect=False):
    host.stdin.write(json.dumps({'delayMs':delay,'disconnect':disconnect})+'\n');host.stdin.flush()
    return json.loads(host.stdout.readline())
def register(who,url):
    browser(who,'--profile',str(TMP/('profile-'+who)),'open',url);sessions.append(who)
    browser(who,'set','viewport','1100','1000')
    snapshot(who,who+'-login')
    js(who,(ROOT/'scripts/yuyuhakusho/platform_app_observer.js').read_text())
    click(who,'创建账号')
    for key,value in [('name','Yuyu'+who+args.label[-10:]),('password','LocalTestOnly01'),('invite','yuyu-local-e2e')]:browser(who,'fill','input[name='+key+']',value)
    click(who,'创建账号，入座')
    wait(who,'!document.querySelector("input[name=password]")')
    snapshot(who,who+'-lobby')

host=None
try:
    subprocess.run(['go','build','-o',str(TMP/'host'),'scripts/yuyuhakusho/platform_app_host.go'],cwd=ROOT,env=env,check=True)
    stderr=(OUT/'host.log').open('w')
    host=subprocess.Popen([str(TMP/'host'),'-build',str(ROOT/'.local/md-yuyuhakusho/platform-app/dist'),'-data',str(TMP/'database')],cwd=ROOT,env=env,stdin=subprocess.PIPE,stdout=subprocess.PIPE,stderr=stderr,text=True)
    urls=json.loads(host.stdout.readline());report['urls']=urls
    print('Real platform host started',flush=True)
    register('fast',urls['direct']);print('Fast user registered through normal UI',flush=True)
    click('fast','创建房间');snapshot('fast','create-game-picker')
    browser('fast','find','role','button','click','--name','魔强统一战')
    wait('fast','!!document.querySelector("[aria-label=房间设置] input[name=name]")')
    snapshot('fast','create-options')
    browser('fast','fill','input[name=name]','魔强统一战验收')
    browser('fast','click','[aria-label=房间设置] form button.primary')
    wait('fast','!!document.querySelector("[aria-label=比赛准备]")')
    snapshot('fast','preparation-first')
    room=state('fast')['room'];report['roomID']=room['id']
    browser('fast','click','[data-arena-picker] summary');browser('fast','wait','--fn','document.querySelectorAll("[data-arena-option]").length===8');browser('fast','click','[data-arena-option="10"]')
    browser('fast','click','[data-character-seat="0"] summary');browser('fast','wait','--fn','document.querySelectorAll("[data-character-option]").length===11');browser('fast','click','[data-character-option="8"]')
    register('slow',urls['shaped'])
    snapshot('slow','lobby-room')
    browser('slow','find','role','button','click','--name','加入')
    wait('slow','!!document.querySelector("[aria-label=比赛准备]")')
    snapshot('slow','preparation-second')
    report['preparation']=state('fast')['room']
    print('Normal create/join/preparation passed',flush=True)
    click('slow','我准备好了');click('fast','我准备好了')
    wait('fast','[...document.querySelectorAll("button")].some(b=>b.textContent.trim()==="开始游戏"&&!b.disabled)')
    click('fast','开始游戏')
    for who in ['fast','slow']:
        wait(who,'document.querySelector("[data-yuyu-board]")?.dataset.connection==="connected" && Number(document.querySelector("[data-yuyu-board]").dataset.authorityFrame)>250')
        snapshot(who,who+'-playing')
    live=state('fast')['room'];report['epoch']=live['game']['data']['epoch']
    report['stage']='playing'
    print('Both normal Boards initialized and real clock advanced',flush=True)
    if args.layout_only:
        layouts();report['stage']='layout-complete';report['pass']=True
        print('Actual app desktop/portrait/landscape layout and native touch passed',flush=True)
        raise SystemExit(0)
    if args.performance:
        for who in ['fast','slow']:performance_check(who)
        print('Both normal visible Boards meet the frozen smoothness budget',flush=True)
    browser('fast','focus','[data-yuyu-board]')
    measure('fast','normal-native-input','KeyD','d',68,0,8,1)
    report['normalTransportMeasurements']=rtt('fast')
    browser('fast','screenshot',str(OUT/'desktop.png'))
    network(120)
    report['shapedTransportMeasurements']=rtt('slow')
    if args.performance:performance_check('slow','slow-tcp-delayed')
    baseline=state('fast')['room']['game']['data']['frame'];started=time.monotonic()
    delayed=measure('slow','tcp-delayed-native-input','KeyA','a',65,1,4,-1,held=120,tail=800)
    assert delayed['firstVisibleBeforeAck'],delayed
    fast=measure('fast','fast-peer-while-other-delayed','KeyD','d',68,0,8,1,held=100,tail=300)
    current=state('fast')['room']['game']['data']
    report['fastClockWhileSlow']={'before':baseline,'after':current['frame'],'elapsedMs':(time.monotonic()-started)*1000}
    assert int(current['frame'])>int(baseline)+30
    assert all(not value['automated'] for value in current['inputs'])
    network()
    report['stage']='tcp-delay'
    print('Trusted input/Canvas response and real TCP-delayed fast/slow peers passed',flush=True)
    # Disconnect while holding a real key, then recover the same running match.
    native('slow',[{'type':'keyDown','key':'a','code':'KeyA','windowsVirtualKeyCode':65,'wait':100}])
    disconnectedAt=state('fast')['room']['game']['data']['frame']
    network(disconnect=True)
    wait('fast','yuyuAppObservation.latestView.inputs[1].appliedButtons===0')
    wait('slow','document.querySelector("[data-yuyu-board]")?.dataset.connection==="connected" && BigInt(document.querySelector("[data-yuyu-board]").dataset.authorityFrame)>BigInt('+json.dumps(disconnectedAt)+')')
    native('slow',[{'type':'keyUp','key':'a','code':'KeyA','windowsVirtualKeyCode':65,'wait':200}])
    restored=state('slow')['room']
    assert restored.get('loading') is None and restored['game']['data']['epoch']==report['epoch']
    assert all(not value['automated'] for value in restored['game']['data']['inputs'])
    report['recovery']={'beforeFrame':disconnectedAt,'afterFrame':restored['game']['data']['frame'],'epoch':restored['game']['data']['epoch'],'sockets':js('slow','yuyuAppObservation.sockets')}
    assert sum(s['event']=='open' for s in report['recovery']['sockets'])>=2
    measure('slow','reconnected-native-input','KeyA','a',65,1,4,-1)
    print('Actual socket close, release and same-match recovery passed',flush=True)
    report['stage']='recovery'
    # The production main entry point must select this game's public settings.
    click('fast','控制器设置')
    wait('fast','document.querySelector("[data-controller-model]")?.value==="game:yuyuhakusho"')
    options=js('fast','[...document.querySelector("[data-controller-model]").options].map(x=>x.value)')
    assert 'md-6' in options and 'classic-2' in options and not any('sfc' in x.lower() for x in options)
    browser('fast','click','[data-controller-button=right]');browser('fast','press','p')
    browser('fast','click','[data-controller-save]')
    wait('fast','document.querySelector(".controller-notice")?.textContent.includes("已保存")')
    browser('fast','click','[aria-label=关闭]')
    browser('fast','focus','[data-yuyu-board]')
    old=native('fast',key('KeyD','d',68))
    assert not any(i['action'].get('buttons',0)&8 for i in old['inputs'])
    measure('fast','saved-remap-native-input','KeyP','p',80,0,8,1)
    click('fast','控制器设置')
    wait('fast','!!document.querySelector("[data-controller-save]")')
    click('fast','恢复默认键位');browser('fast','click','[data-controller-save]')
    wait('fast','document.querySelector(".controller-notice")?.textContent.includes("已保存")')
    browser('fast','screenshot',str(OUT/'desktop-controller-settings.png'))
    browser('fast','click','[aria-label=关闭]')
    report['settings']={'currentGame':'game:yuyuhakusho','models':options,'trustedRemap':True,'oldKeyBypass':False}
    # Explicitly virtual Gamepad API snapshots; actual logical input and host.
    js('fast','window.virtualYuyuPad={id:"Acceptance virtual MD standard",index:3,mapping:"standard",connected:true,axes:[0,0],buttons:Array.from({length:17},()=>({pressed:false,value:0}))};Object.defineProperty(navigator,"getGamepads",{configurable:true,value:()=>[null,null,null,virtualYuyuPad]})')
    browser('fast','focus','[data-yuyu-board]');native('fast',[{'wait':100}])
    native('fast',[{'type':'keyDown','key':'d','code':'KeyD','windowsVirtualKeyCode':68,'wait':70}])
    js('fast','virtualYuyuPad.buttons[0]={pressed:true,value:1}')
    wait_input('fast',0,24)
    js('fast','virtualYuyuPad.connected=false');wait_input('fast',0,8)
    native('fast',[{'type':'keyUp','key':'d','code':'KeyD','windowsVirtualKeyCode':68,'wait':100}]);wait_input('fast',0,0)
    js('fast','virtualYuyuPad.connected=true');native('fast',[{'wait':150}]);assert js('fast','yuyuAppObservation.latestView.inputs[0].appliedButtons')==0
    js('fast','virtualYuyuPad.buttons[0]={pressed:false,value:0}');native('fast',[{'wait':100}])
    js('fast','virtualYuyuPad.buttons[5]={pressed:true,value:1}');wait_input('fast',0,256)
    js('fast','virtualYuyuPad.buttons[5]={pressed:false,value:0}');wait_input('fast',0,0)
    report['virtualGamepad']={'hardware':False,'standardSparseIndex':3,'mixedKeyboardDisconnect':True,'reconnectNeutral':True,'ZPosition5':True}
    click('fast','全屏');wait('fast','!!document.querySelector("[data-room-fullscreen]")')
    assert js('fast','!!document.querySelector(".topbar") && !document.querySelector("[data-room-fullscreen]").contains(document.querySelector(".topbar"))')
    browser('fast','screenshot',str(OUT/'desktop-fullscreen.png'))
    browser('fast','press','Escape');wait('fast','!document.querySelector("[data-room-fullscreen]")')
    browser('fast','set','viewport','320','740')
    assert not js('fast','document.documentElement.scrollWidth>innerWidth')
    js('fast','window.scrollTo(0,0)')
    summaries=js('fast','[...document.querySelectorAll("[data-player-seat]")].map(e=>{const r=e.getBoundingClientRect(),h=e.querySelector(".yuyu-player-summary strong");return{seat:e.dataset.playerSeat,left:r.left,right:r.right,top:r.top,bottom:r.bottom,health:h?.textContent,healthRects:h?.getClientRects().length}})')
    assert len(summaries)==2 and all(s['left']>=0 and s['right']<=320 and s['healthRects'] and s['health'].startswith('体力') for s in summaries),summaries
    browser('fast','screenshot',str(OUT/'mobile-320-public-resources.png'))
    browser('fast','focus','[data-yuyu-board]')
    geometry=js('fast','(()=>{const a=document.querySelector("canvas").getBoundingClientRect(),b=document.querySelector("[data-controller-layout=md]").getBoundingClientRect(),c=document.querySelector("[data-controller-button=c]").getBoundingClientRect();return {canvasBottom:a.bottom,padTop:b.top,c:{x:c.x+c.width/2,y:c.y+c.height/2},width:innerWidth}})()')
    assert geometry['padTop']>=geometry['canvasBottom'],geometry
    x,y=geometry['c']['x'],geometry['c']['y']
    touch=native('fast',[{'method':'Input.dispatchTouchEvent','type':'touchStart','touchPoints':[{'x':x,'y':y,'id':1}],'wait':120},{'method':'Input.dispatchTouchEvent','type':'touchEnd','touchPoints':[],'wait':200}])
    assert any(k['trusted'] and k['type']=='pointerdown' for k in touch['keys'])
    assert any(v['view']['inputs'][0]['appliedButtons']&32 for v in touch['views']),touch
    wait_input('fast',0,0)
    (OUT/'mobile-native-touch.json').write_text(json.dumps(touch,indent=2)+'\n')
    browser('fast','screenshot',str(OUT/'mobile-320-game.png'))
    report['mobile']={'width':320,'noHorizontalOverflow':True,'geometry':geometry,'publicResources':summaries,'nativeTouchC':True}
    print('Production settings, virtual pad, fullscreen and 320px native touch passed',flush=True)
    report['stage']='complete';report['pass']=True
except Exception as error:
    report['error']=repr(error)
    for who in sessions:
        try:
            report[who+'FailureState']=state(who)
            (OUT/(who+'-failure-observation.json')).write_text(json.dumps(js(who,'window.yuyuAppObservation'),ensure_ascii=False,indent=2)+'\n')
            snapshot(who,who+'-failure')
            browser(who,'screenshot',str(OUT/(who+'-failure.png')))
        except Exception:pass
    raise
finally:
    report['sourceSHA256']=FROZEN_SOURCE_SHA256
    (OUT/'result.json').write_text(json.dumps(report,ensure_ascii=False,indent=2)+'\n')
    for who in sessions:
        try:browser(who,'close')
        except Exception:pass
    if host:
        host.stdin.close()
        try:host.wait(timeout=15)
        except subprocess.TimeoutExpired:host.terminate();host.wait(timeout=10)
    print(json.dumps({'pass':report['pass'],'stage':report.get('stage'),'error':report.get('error')}),flush=True)
