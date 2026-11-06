#!/usr/bin/env python3
"""Production PVZ Board density acceptance; explicit opt-in initial snapshots."""
import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess
import time
import uuid

ROOT=Path(__file__).resolve().parents[3]
OUT=ROOT/'.local/pvz/full-density';OUT.mkdir(parents=True,exist_ok=True)
ENV=dict(os.environ)
for key,path in {'GOCACHE':'.local/go-build','GOMODCACHE':'.local/go-mod','GOTMPDIR':'.local/tmp','TMPDIR':'.local/tmp'}.items():
    ENV[key]=str(ROOT/path);Path(ENV[key]).mkdir(parents=True,exist_ok=True)
REPORT={'passed':False,'scope':'production Board / Module / clock / original native art / audio; controlled roofnight initial populations, not a natural whole match; 320px is viewport emulation', 'command':'python3 -B games/pvz/tests/full_density_browser.py','budget':{'seconds':15,'inputPixelP95MS':100,'visibleIntervalP95MS':35},'cases':[],'build':json.loads((ROOT/'web/dist/pvz/v1/runtime/build.json').read_text())}
REPORT['sources']={str(p.relative_to(ROOT)):hashlib.sha256(p.read_bytes()).hexdigest() for p in [ROOT/'games/pvz/tests/full_density_fixture_test.go',Path(__file__),*sorted((ROOT/'games/pvz/web').glob('*.ts*'))]}
REPORT['productionBundles']={str(p.relative_to(ROOT)):hashlib.sha256(p.read_bytes()).hexdigest() for p in (ROOT/'web/dist/assets').glob('*.js') if b'data-pvz-canvas' in p.read_bytes()}
MEASURE=r'''(()=>{
const parse=JSON.parse,stringify=JSON.stringify,NativeSocket=WebSocket,draw=CanvasRenderingContext2D.prototype.drawImage,clear=CanvasRenderingContext2D.prototype.clearRect;
const bytes=v=>typeof v==='string'?new TextEncoder().encode(v).length:v?.byteLength??v?.size??0;
let current=null,go;
window.__densityStart=()=>{current=window.__density={active:true,began:performance.now(),frames:[],inputs:[],wireIn:[],wireOut:[],wasm:[],json:[],longTasks:[],native:[],drawCalls:0,instrumentMS:0,lastSeq:null,hash:2166136261,drawAt:0};};
window.__densityStop=()=>{current.active=false;current.ended=performance.now();return current;};
Object.defineProperty(window,'pvzWasm',{configurable:true,get:()=>go,set:fn=>{go=input=>{const start=performance.now(),result=fn(input),end=performance.now();if(current?.active){const q=parse(input);current.wasm.push({op:q.op,compact:!!q.compact,ms:end-start,request:input.length,response:result.length});}return result;};}});
JSON.parse=function(...args){const start=performance.now(),result=Reflect.apply(parse,this,args);if(current?.active&&typeof args[0]==='string'&&args[0].startsWith('{"ok":'))current.json.push({op:'response-parse',ms:performance.now()-start});return result;};
JSON.stringify=function(...args){const start=performance.now(),result=Reflect.apply(stringify,this,args);if(current?.active&&args[0]?.id?.startsWith?.('pvz-')&&args[0]?.op)current.json.push({op:'request-encode',ms:performance.now()-start});return result;};
window.WebSocket=class extends NativeSocket{constructor(...args){super(...args);this.addEventListener('message',event=>{if(current?.active)current.wireIn.push({at:performance.now(),bytes:bytes(event.data),type:typeof event.data==='string'&&event.data.includes('"type":"game"')?'game':'other'});});}send(data){if(current?.active)current.wireOut.push({at:performance.now(),bytes:bytes(data)});return super.send(data);}};
document.addEventListener('pointerdown',event=>{if(current?.active&&event.isTrusted)current.native.push({at:performance.now(),target:event.target.tagName,text:event.target.textContent?.slice(0,60)});},true);
try{new PerformanceObserver(list=>{if(current?.active)for(const v of list.getEntries())current.longTasks.push({at:v.startTime,ms:v.duration});}).observe({type:'longtask',buffered:false});}catch{}
CanvasRenderingContext2D.prototype.clearRect=function(...args){if(current?.active&&this.canvas.hasAttribute?.('data-pvz-canvas')){current.hash=2166136261;current.drawAt=performance.now();}return Reflect.apply(clear,this,args);};
CanvasRenderingContext2D.prototype.drawImage=function(...args){const isMain=current?.active&&this.canvas.hasAttribute?.('data-pvz-canvas');if(isMain){const start=performance.now();const at=args.length===9?5:1,x=Number(args[at]),y=Number(args[at+1]),w=Number(args[at+2]??args[0].width??0),h=Number(args[at+3]??args[0].height??0);if(x+w>=0&&x<800&&y+h>=0&&y<600&&!(x===0&&y===0)){current.hash=Math.imul(current.hash^(Math.round(x*1024)|0),16777619);current.hash=Math.imul(current.hash^(Math.round(y*1024)|0),16777619);}current.drawCalls++;current.instrumentMS+=performance.now()-start;}return Reflect.apply(draw,this,args);};
window.__densityAttach=()=>{const canvas=document.querySelector('[data-pvz-canvas]');let frame=canvas.__pvzFrame;Object.defineProperty(canvas,'__pvzFrame',{configurable:true,get:()=>frame,set:value=>{frame=value;if(!current?.active)return;const now=performance.now(),row={at:value.renderAt,pixel:value.pixelAt,tick:value.tick,drawMS:now-current.drawAt,motion:current.hash,plants:value.plants,zombies:value.zombies,projectiles:value.projectiles,poses:value.poses?.map(p=>({kind:p.kind,clip:p.clip,index:p.index,x:p.x,y:p.y}))};current.frames.push(row);if(current.lastSeq!==null&&value.lastActionSeq!==current.lastSeq){const native=current.native.at(-1);if(native)current.inputs.push({seq:value.lastActionSeq,nativeAt:native.at,handlerAt:value.lastInputAt,pixelAt:value.pixelAt,ms:value.pixelAt-native.at,target:native.text,plants:value.plants});}current.lastSeq=value.lastActionSeq;}});};
return true;
})()'''

def quantile(values,q=.95):
    values=sorted(values);return values[min(len(values)-1,max(0,__import__('math').ceil(len(values)*q)-1))] if values else None
def stats(values):return {'count':len(values),'sum':sum(values),'mean':sum(values)/len(values) if values else None,'p50':quantile(values,.5),'p95':quantile(values),'max':max(values) if values else None}
def summarize(raw):
    frames=raw['frames'];intervals=[b['pixel']-a['pixel'] for a,b in zip(frames,frames[1:])]
    movement=[v for i,v in enumerate(frames) if i==0 or v['motion']!=frames[i-1]['motion']]
    moving=[b['pixel']-a['pixel'] for a,b in zip(movement,movement[1:])]
    span=(raw['ended']-raw['began'])/1000
    groups={}
    for item in raw['wasm']:
        key=item['op']+('/compact' if item['compact'] else '/full');groups.setdefault(key,[]).append(item['ms'])
    result={'seconds':span,'frames':len(frames),'visibleIntervalsMS':stats(intervals),'motionUpdates':len(movement),'motionUpdatesPerSecond':len(movement)/span,'motionIntervalsMS':stats(moving),'inputPixelMS':stats([v['ms'] for v in raw['inputs']]),'paintMS':stats([v['drawMS'] for v in frames]),'wasmOperationsMS':{k:stats(v) for k,v in groups.items()},'wasmMainThreadPercent':sum(v['ms'] for v in raw['wasm'])/(span*10),'jsonMS':stats([v['ms'] for v in raw['json']]),'longTasksMS':stats([v['ms'] for v in raw['longTasks']]),'wireInBytes':stats([v['bytes'] for v in raw['wireIn']]),'gameMessagesBytes':stats([v['bytes'] for v in raw['wireIn'] if v['type']=='game']),'wireOutBytes':stats([v['bytes'] for v in raw['wireOut']]),'wireInBytesPerSecond':sum(v['bytes'] for v in raw['wireIn'])/span,'instrumentMS':raw['instrumentMS'],'drawCalls':raw['drawCalls'],'populations':{k:[min(v[k] for v in frames),max(v[k] for v in frames)] for k in ['plants','zombies','projectiles']},'costMeaning':'Wasm operation durations include Go request parsing + rule execution/copying + response encoding; full operations also checksum. fork isolates copy + compact encoding; no unsupported pure-rule/clone/hash split is claimed. Movement signature hashes visible draw destination coordinates, excluding sprite image/frame identities; original sprite clip rates are unchanged.'}
    result['passed']=span>=15 and len(raw['inputs'])>=6 and result['inputPixelMS']['p95']<=100 and result['visibleIntervalsMS']['p95']<=35 and result['motionIntervalsMS']['p95']<=35 and result['gameMessagesBytes']['count']>=100
    return result

class Browser:
    def __init__(self,base,label):
        self.base=base;self.env=dict(ENV,AGENT_BROWSER_SESSION='d'+uuid.uuid4().hex[:5],AGENT_BROWSER_SOCKET_DIR=str(ROOT/'.local/ab'),AGENT_BROWSER_PROFILE=str(OUT/('profile-'+label)),AGENT_BROWSER_ARGS='--mute-audio,--disable-breakpad,--disable-crash-reporter');Path(self.env['AGENT_BROWSER_SOCKET_DIR']).mkdir(parents=True,exist_ok=True)
        script=OUT/'measure-init.js';script.write_text(MEASURE)
        self.b('open',base,'--init-script',str(script));self.api('/register',{'name':'密集验收'+uuid.uuid4().hex[:7],'password':'pvz-density-local','invite':'pvz-density'});self.b('reload');self.b('wait','--text','创建房间')
    def b(self,*args):
        p=subprocess.run(['agent-browser',*map(str,args),'--json'],cwd=ROOT,env=self.env,text=True,capture_output=True,timeout=60);assert p.returncode==0,p.stdout+p.stderr;r=json.loads(p.stdout);assert r['success'],r;return r['data']
    def js(self,code):return self.b('eval',code)['result']
    def api(self,path,body=None):
        options='' if body is None else ',{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify('+json.dumps(body)+')}'
        r=self.js('(async()=>{const r=await fetch('+json.dumps('/api'+path)+options+');return {status:r.status,body:await r.json()}})()');assert 200<=r['status']<300,r;return r['body']
    def room(self):return self.api('/state')['room']
    def command(self,kind,**extra):
        r=self.room();return self.api('/rooms/'+r['id'],dict(type=kind,version=r['version'],nonce=uuid.uuid4().hex,**extra))
    def close(self):
        try:self.b('close')
        finally:
            shutil.rmtree(self.env['AGENT_BROWSER_PROFILE'],ignore_errors=True)
            for suffix in ['config','target']:(Path(self.env['AGENT_BROWSER_SOCKET_DIR'])/(self.env['AGENT_BROWSER_SESSION']+'.'+suffix)).unlink(missing_ok=True)

services=[];clients=[]
try:
    for mode in ['ordinary','dense']:
        area=OUT/mode;area.mkdir(exist_ok=True);(area/'url').unlink(missing_ok=True);(area/'stop').unlink(missing_ok=True)
        log=(area/'server.log').open('w');env=dict(ENV,WIRE_BOARD_PVZ_DENSITY='1',PVZ_DENSITY_MODE=mode,PVZ_DENSITY_OUT=str(area))
        process=subprocess.Popen(['go','test','./games/pvz/tests','-run','^TestPvZFullDensityBrowserFixture$','-count=1','-timeout','15m'],cwd=ROOT,env=env,stdout=log,stderr=subprocess.STDOUT);services.append((process,area,log))
        until=time.monotonic()+90
        while not (area/'url').exists() and time.monotonic()<until and process.poll() is None:time.sleep(.2)
        assert (area/'url').exists(),(area/'server.log').read_text()
        client=Browser((area/'url').read_text().strip(),mode);clients.append(client)
        for width,height in [(1440,960),(320,760)]:
            label=f'{mode}-{width}';client.b('set','viewport',width,height)
            client.api('/rooms',{'kind':'pvz','name':'受控初始密度验收 '+mode,'capacity':2,'options':{'mode':'duel','teams':[0,1],'scene':'roofnight','roundSeconds':600,'prepSeconds':20}})
            client.command('add_bot');client.command('ready');client.command('start');client.b('wait','[data-pvz-board][data-ready="true"]')
            until=time.monotonic()+60
            while client.room().get('loading') and time.monotonic()<until:time.sleep(.1)
            client.b('click','button[aria-label="全屏"]');client.b('wait','[data-room-fullscreen]');time.sleep(1)
            initial=client.room();assert initial['game']['data']['map']['id']=='roofnight'
            client.js('window.__densityAttach();window.__densityStart();true')
            began=time.monotonic()
            # Open an actual stack, then native-click its shell/body removal.
            # Reuse ordinary rear cells; no script-dispatched gameplay events.
            for row,col in [(0,0),(1,0),(2,0),(3,0),(4,0),(0,1),(1,1),(2,1)]:
                client.b('scrollintoview',f'[data-pvz-cell="{row}:{col}"]')
                rect=client.js(f'document.querySelector(\'[data-pvz-cell="{row}:{col}"]\').getBoundingClientRect().toJSON()')
                client.b('mouse','move',round(rect['left']+rect['width']/2),round(rect['top']+rect['height']/2));client.b('mouse','down');client.b('mouse','up')
                client.b('wait','.pvz-layer-list')
                client.b('click','.pvz-layer-list > div:last-child > button')
                client.b('wait','--fn','!document.querySelector(".pvz-layer-list")')
                time.sleep(.2)
            while time.monotonic()-began<15.2:time.sleep(.2)
            raw=client.js('window.__densityStop()');summary=summarize(raw)
            geometry=client.js('({overflow:document.documentElement.scrollWidth>innerWidth+1,canvas:document.querySelector("[data-pvz-canvas]").getBoundingClientRect().toJSON(),audio:[...document.querySelectorAll("audio")].filter(v=>!v.paused).map(v=>({src:v.currentSrc,time:v.currentTime})),errors:[...document.querySelectorAll(".pvz-action-error")].map(v=>v.textContent)})')
            summary.update(mode=mode,viewport=[width,height],fullscreen=True,geometry=geometry,initial={'plants':len(initial['game']['data']['plants']),'zombies':len(initial['game']['data']['zombies'])})
            summary['passed']=summary['passed'] and not geometry['overflow'] and bool(geometry['audio'])
            (OUT/(label+'-raw.json')).write_text(json.dumps(raw,separators=(',',':'))+'\n');(OUT/(label+'.json')).write_text(json.dumps(summary,ensure_ascii=False,indent=2)+'\n')
            client.b('screenshot',str(OUT/(label+'.png')));REPORT['cases'].append(summary);(OUT/'result.json').write_text(json.dumps(REPORT,ensure_ascii=False,indent=2)+'\n')
            print(json.dumps(summary,ensure_ascii=False),flush=True)
            client.command('close');client.command('leave')
        client.close();clients.remove(client);(area/'stop').touch();process.wait(timeout=15);log.close()
    REPORT['passed']=all(c['passed'] for c in REPORT['cases']) and len(REPORT['cases'])==4
finally:
    for client in clients:
        try:client.b('screenshot',str(OUT/'failure.png'));REPORT['failurePage']=client.js('document.body.innerText');client.close()
        except Exception:pass
    for process,area,log in services:
        (area/'stop').touch()
        try:process.wait(timeout=10)
        except subprocess.TimeoutExpired:process.terminate()
        log.close()
    (OUT/'result.json').write_text(json.dumps(REPORT,ensure_ascii=False,indent=2)+'\n')
assert REPORT['passed'],'production density budget failed; inspect .local/pvz/full-density/result.json'
