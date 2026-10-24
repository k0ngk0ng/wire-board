#!/usr/bin/env python3
"""Map-first Conquest acceptance, specified before the UX implementation.

Use --baseline for the retained old build. Both modes use the production module,
normal accounts and real HTTP. CDP sends native inputs to agent-browser's owned
page; no game callbacks, pointer-event dispatch or game state are forged.
"""
import argparse
import hashlib
import http.cookiejar
import json
import os
import re
from pathlib import Path
import shutil
import subprocess
import time
import traceback
import urllib.parse
import urllib.request
import uuid

ROOT = Path(__file__).resolve().parents[3]
ARGS = argparse.ArgumentParser()
ARGS.add_argument('--baseline', action='store_true')
ARGS.add_argument('--skip-build', action='store_true')
ARGS.add_argument('--base-url', default='')
ARGS.add_argument('--queue-case', choices=['all','undo','reject'])
ARGS.add_argument('--hold-actions', action='store_true')
ARGS.add_argument('--marker-case', action='store_true')
ARGS.add_argument('--maps', default='')
ARGS = ARGS.parse_args()
MAP_IDS=ARGS.maps.split(',') if ARGS.maps else (['europe','world'] if ARGS.baseline else ['world','europe','usa','china'])
assert set(MAP_IDS)<=set(['world','europe','usa','china'])
RUN = uuid.uuid4().hex[:6]
AREA = ROOT/'.local/conquest-map-ux'
OUT = AREA/('queue-'+ARGS.queue_case if ARGS.queue_case else 'hold-actions' if ARGS.hold_actions else 'marker-case' if ARGS.marker_case else 'baseline' if ARGS.baseline else 'browser')
FIXTURE = ROOT/'.local/conquest/browser'
PROFILE = AREA/('profile-'+RUN)
SOCKET = ROOT/'.s'
for p in [OUT,FIXTURE,PROFILE,SOCKET,ROOT/'.local/tmp']:
    p.mkdir(parents=True,exist_ok=True)
ENV = dict(os.environ)
for key,p in {'GOCACHE':ROOT/'.local/go-build','GOMODCACHE':ROOT/'.local/go-mod','GOTMPDIR':ROOT/'.local/tmp','TMPDIR':ROOT/'.local/tmp','npm_config_cache':ROOT/'.local/npm','AGENT_BROWSER_SOCKET_DIR':SOCKET,'AGENT_BROWSER_PROFILE':PROFILE}.items():
    p.mkdir(parents=True,exist_ok=True);ENV[key]=str(p)
ENV.update(GOTOOLCHAIN='local',WIRE_BOARD_CONQUEST_BROWSER='1',AGENT_BROWSER_SESSION='x'+RUN[:4],AGENT_BROWSER_ARGS='--mute-audio,--disable-breakpad,--disable-crash-reporter')
BASE = ARGS.base_url
PASSWORD = 'map-ux-local-only'
INVITE = ENV.get('CONQUEST_INVITE','conquest-browser')
REPORT = {'passed':False,'baseline':ARGS.baseline,'scenario':'queue-'+ARGS.queue_case if ARGS.queue_case else 'hold-actions' if ARGS.hold_actions else 'marker-case' if ARGS.marker_case else 'full-map-ux','scope':'normal local rooms, production module, native Chromium inputs; mobile viewport simulation','checks':[],'violations':[],'inputs':[],'measurements':[]}
server = None
opened = False
handles = []
swapped_dist = False
saved_dist = AREA/('saved-dist-'+RUN)


def source_hashes():
    paths=list((ROOT/'games/conquest/web').glob('*'))+list((ROOT/'games/conquest/server').glob('*.go'))
    paths += [p for p in (ROOT/'web/src').rglob('*') if p.suffix in {'.ts','.tsx','.css'}]
    paths += [*sorted((ROOT/'games/conquest/maps').glob('*.json')),Path(__file__),Path(__file__).with_name('map_ux_input.mjs'),Path(__file__).with_name('http_e2e_test.go')]
    return {str(p.relative_to(ROOT)):hashlib.sha256(p.read_bytes()).hexdigest() for p in sorted(paths) if p.is_file()}


SOURCE_START = source_hashes()


def browser(*args):
    p=subprocess.run(['agent-browser',*args,'--json'],cwd=ROOT,env=ENV,capture_output=True,text=True,timeout=50)
    if p.returncode:raise RuntimeError(p.stdout+p.stderr)
    result=json.loads(p.stdout)
    if not result.get('success'):raise RuntimeError(result)
    return result['data']


def js(code):return browser('eval',code)['result']
def wait(code):return browser('wait','--fn',code)
def api(path,body=None):
    opts='' if body is None else ',{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify('+json.dumps(body)+')}'
    r=js('fetch('+json.dumps('/api'+path)+opts+').then(async r=>({status:r.status,body:await r.json()}))')
    assert 200<=r['status']<300,r
    return r['body']
def room():return api('/state')['room']
def view():return room()['game']['data']
def command(kind,**extra):
    r=room();return api('/rooms/'+r['id'],dict(type=kind,version=r['version'],nonce=uuid.uuid4().hex,**extra))


class Peer:
    def __init__(self,name):
        self.name=name
        self.client=urllib.request.build_opener(urllib.request.HTTPCookieProcessor(http.cookiejar.CookieJar()))
        self.api('/register',dict(name=name,password=PASSWORD,invite=INVITE))
    def api(self,path,body=None):
        req=urllib.request.Request(BASE+'/api'+path,data=None if body is None else json.dumps(body).encode(),headers={'Content-Type':'application/json'})
        with self.client.open(req,timeout=20) as r:return json.load(r)
    def command(self,kind,r=None,**extra):
        r=r or self.api('/state')['room'];return self.api('/rooms/'+r['id'],dict(type=kind,version=r['version'],nonce=uuid.uuid4().hex,**extra))
    def ready_load(self):
        r=self.api('/state')['room'];g=r.get('loading')
        if g:
            fields=dict(loadToken=g['token'],loadBuild=g['build'],loadClient='peer-'+RUN)
            self.command('load_begin',**fields);self.command('load_ready',**fields)
    def act(self,kind,**extra):
        d=self.api('/state')['room']['game']['data'];return self.command('action',action=dict(type=kind,session=d['session'],epoch=d['epoch'],**extra))


def assert_or_record(condition,label,value=None):
    if not condition:
        REPORT['violations'].append({'label':label,'value':value})
        if not ARGS.baseline:raise AssertionError((label,value))
    else:REPORT['checks'].append(label)


def click(name):
    label=json.dumps(name)
    wait('(()=>{const b=[...document.querySelectorAll("button")].find(e=>(e.getAttribute("aria-label")==='+label+'||e.textContent.trim()==='+label+')&&!e.disabled&&!e.closest("[inert]"));return !!b})()')
    browser('find','role','button','click','--name',name,'--exact')


def action(name):
    selector='[data-cq-action="'+name+'"]'
    browser('wait',selector)
    wait('(()=>{const e=document.querySelector('+json.dumps(selector)+');return e&&!e.disabled&&!e.closest("[inert]")})()')
    browser('click',selector)


def reset_map():
    click('复位地图' if js('!!document.querySelector("button[aria-label=复位地图]")') else '全图')
    js('new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(resolve)))')


def wait_orders(count):
    wait('Number(document.querySelector("[data-cq-action=plans] b,.cq-plan-button b")?.textContent)==='+str(count))


def screenshot(name,check_contrast=True):
    assert js('document.documentElement.scrollWidth<=innerWidth+1'),'horizontal overflow'
    browser('screenshot',str(OUT/(name+'.png')))
    browser('screenshot',str(OUT/(name+'-full.png')),'--full')
    (OUT/(name+'.state.json')).write_text(json.dumps(room(),ensure_ascii=False,indent=2))
    if not ARGS.baseline:
        colors=js(r'''(() => {
          const rgb=s=>(s.match(/[\d.]+/g)||[]).map(Number),lum=c=>c.slice(0,3).map(v=>v/255).map(v=>v<=.04045?v/12.92:((v+.055)/1.055)**2.4).reduce((s,v,i)=>s+v*[.2126,.7152,.0722][i],0);
          return [...document.querySelectorAll('.cq-board button,.cq-land-info strong,.cq-land-info span,.cq-map-message,.cq-campaign-strip strong,.cq-campaign-strip span')].filter(e=>e.getClientRects().length&&e.textContent.trim()).map(e=>{
            let bg=[255,255,255],chain=[];for(let p=e;p;p=p.parentElement)chain.unshift(p);
            for(const p of chain){const c=rgb(getComputedStyle(p).backgroundColor),a=c[3]??1;bg=bg.map((v,i)=>c[i]*a+v*(1-a));}
            const style=getComputedStyle(e),a=lum(rgb(style.color)),b=lum(bg);
            return {text:e.textContent.trim(),color:style.color,background:bg,ratio:(Math.max(a,b)+.05)/(Math.min(a,b)+.05)};
          });
        })()''')
        (OUT/(name+'-contrast.json')).write_text(json.dumps(colors,ensure_ascii=False,indent=2))
        if check_contrast:assert colors and min(x['ratio'] for x in colors)>=4.5,(name,colors)
    REPORT['checks'].append(name)


def ready():
    browser('wait','[data-conquest-board]');wait('!document.querySelector(".game-loading")')
    wait('!document.querySelector("[data-conquest-board]").closest("[inert]")')


def new_game(map_id,peer):
    api('/rooms',dict(kind='conquest',name='地图操作验收 '+map_id,capacity=2,options={'mapId':map_id,'difficulty':'normal','turnSeconds':180}))
    peer.command('join',room());peer.command('ready');command('ready');command('start');peer.ready_load();ready()


def finish(peer):
    r=api('/state').get('room')
    if r and r['status']=='playing':command('close')
    if r:command('leave')
    if peer.api('/state').get('room'):peer.command('leave')
    browser('wait','.room-list')


def point_for(map_id,province):
    m=json.loads((ROOT/'games/conquest/maps'/f'{map_id}.json').read_text());p=next(p for p in m['provinces'] if p['id']==province)
    return js('(()=>{const e=document.querySelector('+json.dumps('[data-cq-province="'+province+'"]')+'),p=new DOMPoint('+str(p['label']['x'])+','+str(p['label']['y'])+').matrixTransform(e.getScreenCTM()),v=document.querySelector(".cq-viewport").getBoundingClientRect();return {x:p.x,y:p.y,viewport:v.toJSON(),hit:document.elementFromPoint(p.x,p.y)?.tagName}})()')


def focus_region(map_id,province):
    selector='[data-cq-province="'+province+'"]'
    browser('focus',selector);browser('press','Enter')
    wait('(document.querySelector("[data-conquest-board]").dataset.cqSelected??document.querySelector("[data-conquest-board]").dataset.selected)==='+json.dumps(province))
    if ARGS.baseline:
        browser('scrollintoview','.cq-viewport')
    else:
        wait('(()=>{const e=document.querySelector(".cq-territory-actions");if(!e)return true;const r=e.getBoundingClientRect(),v=visualViewport;return r.top>=(v?.offsetTop??0)&&r.bottom<=(v?.offsetTop??0)+(v?.height??innerHeight)})()')
    return point_for(map_id,province)


def pinch_centre(point,span=65):
    # Keep both contact centres inside the viewport. The right finger starts
    # on the army/territory label unless the edge requires a small adjustment.
    v=point['viewport']
    return max(v['left']+span+8,min(v['right']-span-8,point['x']-22)),max(v['top']+50,min(v['bottom']-50,point['y']))


def native(task):
    endpoint=browser('get','cdp-url')['cdpUrl']
    p=subprocess.run(['node','games/conquest/tests/map_ux_input.mjs',endpoint,BASE,json.dumps(task)],cwd=ROOT,env=ENV,capture_output=True,text=True,timeout=30)
    if p.returncode:raise RuntimeError(p.stdout+p.stderr)
    REPORT['inputs'].append(json.loads(p.stdout))


def measure_start():
    js(r'''(() => {
      const viewport=document.querySelector('.cq-viewport'),board=document.querySelector('[data-conquest-board]');
      const p=window.__mapUX={frames:[],longTasks:[],mutations:0,provinceMutations:0,events:[],active:true,startRender:Number(board.dataset.mapRenderCount??board.dataset.renderCount??NaN),startScroll:{x:scrollX,y:scrollY}};
      const read=()=>{const e=document.querySelector('.cq-map-layer'),m=e?.getScreenCTM();return m?{x:m.e,y:m.f,scale:m.a}:null};
      p.observer=new MutationObserver(rows=>{p.mutations+=rows.length;p.provinceMutations+=rows.filter(r=>r.target.matches?.('[data-cq-province]')).length});
      p.observer.observe(viewport,{subtree:true,attributes:true,attributeFilter:['transform','style','class','fill','data-camera-x','data-camera-y','data-camera-scale']});
      p.longObserver=new PerformanceObserver(list=>p.longTasks.push(...list.getEntries().map(e=>({at:e.startTime,duration:e.duration}))));
      try{p.longObserver.observe({entryTypes:['longtask']})}catch{}
      p.listener=e=>p.events.push({type:e.type,at:performance.now(),x:e.clientX,y:e.clientY,pointerId:e.pointerId,target:e.target.tagName});
      for(const type of ['pointerdown','pointermove','pointerup','pointercancel','lostpointercapture'])viewport.addEventListener(type,p.listener,true);
      const frame=at=>{if(!p.active)return;p.frames.push({at,camera:read()});requestAnimationFrame(frame)};requestAnimationFrame(frame);
    })()''')


def measure_stop(name,continuous=True):
    r=js(r'''(() => {const p=window.__mapUX,v=document.querySelector('.cq-viewport'),b=document.querySelector('[data-conquest-board]');p.active=false;p.observer.disconnect();p.longObserver.disconnect();for(const type of ['pointerdown','pointermove','pointerup','pointercancel','lostpointercapture'])v.removeEventListener(type,p.listener,true);return {frames:p.frames,longTasks:p.longTasks,mutations:p.mutations,provinceMutations:p.provinceMutations,events:p.events,startRender:Number.isFinite(p.startRender)?p.startRender:null,endRender:Number(b.dataset.mapRenderCount??b.dataset.renderCount??NaN)||null,startScroll:p.startScroll,endScroll:{x:scrollX,y:scrollY}}})()''')
    active=[e['at'] for e in r['events'] if e['type']=='pointermove']
    frames=[f for f in r['frames'] if not active or active[0]<=f['at']<=active[-1]+35]
    intervals=sorted(b['at']-a['at'] for a,b in zip(frames,frames[1:]))
    unique_cameras=len({json.dumps(f['camera'],sort_keys=True) for f in frames})
    metrics={'name':name,'frames':len(frames),'uniqueCameraSamples':unique_cameras,'p95MS':intervals[min(len(intervals)-1,int(len(intervals)*.95))] if intervals else None,'maxFrameMS':max(intervals) if intervals else None,'longTaskMaxMS':max([x['duration'] for x in r['longTasks']],default=0),'mutations':r['mutations'],'provinceMutations':r['provinceMutations'],'mapRenderDelta':None if r['startRender'] is None or r['endRender'] is None else r['endRender']-r['startRender'],'eventCount':len(r['events'])}
    (OUT/(name+'-performance.json')).write_text(json.dumps({'metrics':metrics,'samples':r},ensure_ascii=False,indent=2))
    REPORT['measurements'].append(metrics)
    assert_or_record(len(frames)>=10,name+'-paced-native-frames',metrics)
    assert_or_record(metrics['p95MS'] is not None and metrics['p95MS']<=50 and metrics['maxFrameMS']<=120,name+'-frame-budget',metrics)
    assert_or_record(metrics['longTaskMaxMS']<=120,name+'-no-blocking-long-task',metrics)
    if continuous:assert_or_record(unique_cameras>=12,name+'-camera-keeps-tracking',metrics)
    assert_or_record(metrics['mapRenderDelta'] is not None and metrics['mapRenderDelta']<=6,name+'-bounded-map-renders',metrics)
    assert_or_record(r['startScroll']==r['endScroll'],name+'-page-does-not-pan',r)
    return r


def scene_state():
    return js('(()=>{const e=document.querySelector(".cq-map-layer"),m=e?.getScreenCTM(),b=document.querySelector("[data-conquest-board]");return {x:m?.e,y:m?.f,scale:m?.a,selected:b.dataset.cqSelected??b.dataset.selected}})()')


def mouse_start_point(province,kind):
    return js('''(() => {
      const id='''+json.dumps(province)+''',kind='''+json.dumps(kind)+''',v=document.querySelector('.cq-viewport').getBoundingClientRect();
      if(kind==='text') {
        const e=document.querySelector('[data-cq-troops="'+id+'"],[data-cq-label="'+id+'"] text'),r=e?.getBoundingClientRect();
        if(!r)return null;const x=r.x+r.width/2,y=r.y+r.height/2,h=document.elementFromPoint(x,y);
        return x>v.left&&x<v.right&&y>v.top&&y<v.bottom&&h?.closest('[data-cq-label]')?.getAttribute('data-cq-label')===id?{x,y,target:h.tagName,kind}:null;
      }
      const e=document.querySelector('[data-cq-province="'+id+'"]'),b=e.getBBox(),m=e.getScreenCTM();
      for(const u of [.2,.35,.5,.65,.8])for(const t of [.2,.35,.5,.65,.8]){
        const local=new DOMPoint(b.x+b.width*u,b.y+b.height*t);if(!e.isPointInFill(local))continue;
        const p=local.matrixTransform(m);if(p.x<v.left+12||p.x>v.right-12||p.y<v.top+12||p.y>v.bottom-12)continue;
        const hit=document.elementFromPoint(p.x,p.y);if(hit===e)return {x:p.x,y:p.y,target:hit.tagName,kind};
      }
      return null;
    })()''')


def desktop_mouse(map_id,own):
    reset_map();p=focus_region(map_id,own)
    point=mouse_start_point(own,'territory');assert point
    a=scene_state();measure_start();native(dict(mode='mouse-drag',x=point['x'],y=point['y'],dx=45,dy=30,steps=40,pace=16));measure_stop(map_id+'-pc-fit-drag');b=scene_state()
    assert_or_record(abs(b['x']-a['x'])>20 and abs(b['y']-a['y'])>10,map_id+'-pc-fit-pans-without-zoom',(a,b))
    reset_map();p=focus_region(map_id,own)
    native(dict(mode='wheel',x=p['x'],y=p['y'],dy=-400,steps=1,pace=0))
    before=view()['orders'];selected=scene_state()['selected']
    if not ARGS.baseline:action('attack')
    for kind,dx,dy in [('territory',-75,-35),('text',55,-30)]:
        point=mouse_start_point(own,kind);assert point,(map_id,kind,'could not hit real painted surface')
        REPORT.setdefault('mouseStarts',[]).append({'map':map_id,**point})
        a=scene_state();measure_start();native(dict(mode='mouse-drag',x=point['x'],y=point['y'],dx=dx,dy=dy,steps=48,pace=16));measure_stop(map_id+'-pc-'+kind+'-drag');b=scene_state()
        assert_or_record(abs(b['x']-a['x'])+abs(b['y']-a['y'])>25,map_id+'-pc-'+kind+'-pans',(a,b))
        assert_or_record(b['selected']==selected and view()['orders']==before,map_id+'-pc-'+kind+'-no-misclick')
    point=mouse_start_point(own,'text');assert point
    bounds=js('document.querySelector(".cq-viewport").getBoundingClientRect().toJSON()')
    measure_start();native(dict(mode='mouse-drag',x=point['x'],y=point['y'],dx=bounds['right']+12-point['x'],dy=0,steps=40,pace=16));measure_stop(map_id+'-pc-outside-release',continuous=False)
    after=scene_state();native(dict(mode='mouse-hover',x=bounds['x']+bounds['width']/2,y=bounds['y']+bounds['height']/2,steps=1,pace=0));hover=scene_state()
    assert_or_record(abs(hover['x']-after['x'])+abs(hover['y']-after['y'])<1,map_id+'-pc-released-does-not-pan')
    assert_or_record(view()['orders']==before,map_id+'-pc-outside-no-order')
    geography=json.loads((ROOT/'games/conquest/maps'/f'{map_id}.json').read_text())
    visible_width=max(0,min(bounds['right'],hover['x']+geography['width']*hover['scale'])-max(bounds['left'],hover['x']))
    visible_height=max(0,min(bounds['bottom'],hover['y']+geography['height']*hover['scale'])-max(bounds['top'],hover['y']))
    assert_or_record(visible_width>=min(bounds['width'],geography['width']*hover['scale'])*.7 and visible_height>=min(bounds['height'],geography['height']*hover['scale'])*.7,map_id+'-pc-map-cannot-be-flung-away',{'width':visible_width,'height':visible_height})
    if not ARGS.baseline:action('cancel')
    reset_map();bounds=js('document.querySelector(".cq-viewport").getBoundingClientRect().toJSON()')
    native(dict(mode='mouse-tap',x=bounds['left']+4,y=bounds['top']+4,steps=1,pace=0))
    point=mouse_start_point(own,'text');assert point
    native(dict(mode='mouse-tap',x=point['x'],y=point['y'],steps=1,pace=0))
    assert_or_record(scene_state()['selected']==own,map_id+'-pc-click-after-drag')


def gestures(map_id,width):
    browser('set','viewport',str(width),'1000' if width>900 else '780')
    reset_map()
    own=next(p['id'] for p in view()['provinces'] if p['owner']==0)
    if width>900:desktop_mouse(map_id,own);reset_map()
    p=focus_region(map_id,own);v=p['viewport']
    x,y=pinch_centre(p)
    before=view();label=f'{map_id}-{width}'
    screenshot(label+'-selected')
    initial=scene_state();measure_start()
    # One finger begins on the territory/army label rather than empty ocean.
    native(dict(mode='pinch',x=x,y=y,start=22,end=65,steps=48,pace=16))
    r=measure_stop(label+'-pinch-out');after=scene_state()
    assert_or_record(after['scale']>initial['scale']*1.15,label+'-pinch-enlarges-on-land',(initial,after))
    measure_start();native(dict(mode='pinch',x=x,y=y,start=65,end=28,steps=48,pace=16));measure_stop(label+'-pinch-in')
    smaller=scene_state();assert_or_record(smaller['scale']<after['scale']*.9,label+'-pinch-shrinks',(after,smaller))
    measure_start();native(dict(mode='pinch-to-pan',x=x,y=y,start=25,end=44,dx=-12,dy=-8,steps=40,pace=16));measure_stop(label+'-pinch-to-pan')
    # A real second finger joins after the first has already started panning.
    reset_map();p=focus_region(map_id,own);v=p['viewport'];x,y=pinch_centre(p)
    initial=scene_state();measure_start();native(dict(mode='add-finger',x=x,y=y,start=24,end=52,steps=40,pace=16));measure_stop(label+'-second-finger-joins')
    after=scene_state();assert_or_record(after['scale']>initial['scale']*1.25,label+'-joining-finger-zooms',(initial,after))
    initial=scene_state();measure_start();native(dict(mode='drag',x=x,y=y,dx=-55,dy=-40,steps=48,pace=16));measure_stop(label+'-drag');after=scene_state()
    assert_or_record(abs(after['x']-initial['x'])+abs(after['y']-initial['y'])>5,label+'-drag-pans',(initial,after))
    native(dict(mode='cancel',x=x,y=y,dx=20,dy=20,steps=12,pace=16))
    assert_or_record(view()['orders']==before['orders'],label+'-gestures-never-queue-orders')
    if map_id=='world' and width==320:
        measure_start();native(dict(mode='drag',x=x,y=y,dx=50,dy=25,steps=48,pace=16,cpu=4));measure_stop(label+'-drag-cpu4')
        reset_map();p=focus_region(map_id,own);x,y=pinch_centre(p)
        measure_start();native(dict(mode='pinch',x=x,y=y,start=22,end=62,steps=48,pace=16,cpu=4));measure_stop(label+'-pinch-cpu4')
    if width<=620:
        reset_map();v=js('document.querySelector(".cq-viewport").getBoundingClientRect().toJSON()')
        x,y=v['left']+24,v['top']+v['height']*.25
        measure_start();native(dict(mode='drag',x=x,y=y,dx=v['right']+8-x,dy=v['bottom']+8-y,steps=40,pace=16));measure_stop(label+'-fit-boundary-drag',continuous=False)
        c=scene_state();g=json.loads((ROOT/'games/conquest/maps'/f'{map_id}.json').read_text())
        vw=max(0,min(v['right'],c['x']+g['width']*c['scale'])-max(v['left'],c['x']));vh=max(0,min(v['bottom'],c['y']+g['height']*c['scale'])-max(v['top'],c['y']))
        assert_or_record(vw>=min(v['width'],g['width']*c['scale'])*.7 and vh>=min(v['height'],g['height']*c['scale'])*.7,label+'-touch-map-cannot-be-flung-away',{'visibleWidth':vw,'visibleHeight':vh})
        assert_or_record(view()['orders']==before['orders'],label+'-touch-boundary-no-order')
        reset_map();p=focus_region(map_id,own)
    browser('scrollintoview','.cq-minimap')
    mini=js('document.querySelector(".cq-minimap").getBoundingClientRect().toJSON()')
    mx,my=mini['x']+mini['width']*.25,mini['y']+mini['height']*.35
    measure_start();native(dict(mode='mouse-drag',x=mx,y=my,dx=mini['width']*.25,dy=mini['height']*.15,steps=32,pace=16));measure_stop(label+'-minimap-drag')
    position=js('(()=>{const e=document.querySelector(".cq-map-layer"),m=e.getScreenCTM(),r=document.querySelector(".cq-viewport").getBoundingClientRect();return {x:(r.x+r.width/2-m.e)/m.a,y:(r.y+r.height/2-m.f)/m.a,scale:m.a,width:r.width,height:r.height}})()')
    geography=json.loads((ROOT/'games/conquest/maps'/f'{map_id}.json').read_text())
    for axis,extent,visible,fraction in [('x',geography['width'],position['width'],.5),('y',geography['height'],position['height'],.5)]:
        expected=extent*fraction
        assert_or_record(abs(position[axis]-expected)<2,label+'-minimap-centre-'+axis,{'actual':position,'expected':expected})
    assert_or_record(view()['orders']==before['orders'],label+'-minimap-never-orders')
    screenshot(label+'-after-gestures')


def map_first_invariants(map_id,width):
    value=js(r'''(() => {const board=document.querySelector('[data-conquest-board]'),v=board.querySelector('.cq-viewport').getBoundingClientRect();return {numericInputs:board.querySelectorAll('input[type=number]').length,provinceButtonRects:board.querySelectorAll('[data-cq-label] rect,.cq-troop-label rect,[data-cq-troops] rect').length,legacyForm:!!board.querySelector('.cq-move-form'),map:v.toJSON(),actions:[...board.querySelectorAll('[data-cq-action]')].filter(e=>e.getClientRects().length).map(e=>({type:e.dataset.cqAction,label:e.getAttribute('aria-label'),rect:e.getBoundingClientRect().toJSON()}))}})()''')
    name=f'{map_id}-{width}'
    (OUT/(name+'-ui.json')).write_text(json.dumps(value,ensure_ascii=False,indent=2))
    assert_or_record(value['numericInputs']==0,name+'-no-number-form',value)
    assert_or_record(value['provinceButtonRects']==0,name+'-no-permanent-label-buttons',value)
    assert_or_record(not value['legacyForm'],name+'-no-legacy-form',value)
    assert_or_record(any(a['type']=='recruit' for a in value['actions']),name+'-selected-actions-visible',value)
    assert_or_record(all(a['label'] and a['rect']['width']>=36 and a['rect']['height']>=36 for a in value['actions']),name+'-action-targets-readable',value)


def orders(map_id,peer):
    reset_map()
    own=next(p['id'] for p in view()['provinces'] if p['owner']==0)
    focus_region(map_id,own)
    initial=view();action('recruit')
    wait_orders(1)
    plan=view();assert len(plan['orders'])==1 and plan['orders'][0]['type']=='recruit'
    assert plan['provinces']==initial['provinces'] and peer.api('/state')['room']['game']['data']['orders']==[]
    screenshot(map_id+'-recruit-icon-plan')
    before=list(plan['orders']);action('attack');action('cancel');assert view()['orders']==before
    p=focus_region(map_id,own)
    action('attack')
    px,py=pinch_centre(p);native(dict(mode='pinch',x=px,y=py,start=22,end=62,steps=32,pace=16))
    native(dict(mode='drag',x=px,y=py,dx=-18,dy=-12,steps=16,pace=16))
    assert view()['orders']==before,'camera gesture in attack mode accidentally queued a move'
    REPORT['checks'].append(map_id+'-armed-gestures-do-not-attack')
    m=json.loads((ROOT/'games/conquest/maps'/f'{map_id}.json').read_text());geo=next(p for p in m['provinces'] if p['id']==own);target=geo['neighbors'][0]
    movable=next(p['mobile'] for p in plan['projected'] if p['id']==own)
    point=point_for(map_id,target)
    assert point['viewport']['left']<=point['x']<=point['viewport']['right'] and point['viewport']['top']<=point['y']<=point['viewport']['bottom'],point
    # Delayed real HTTP plus two native taps 20ms apart exercises in-flight
    # intent queuing, not just sequential clicks after the first request ends.
    native(dict(mode='doubletap',x=point['x'],y=point['y'],steps=1,pace=0,latency=150))
    wait_orders(3)
    plan=view();first=max(1,(movable+1)//2);second=max(1,(movable-first+1)//2)
    assert plan['orders'][1]['type']=='move' and plan['orders'][1]['from']==own and plan['orders'][1]['to']==target and plan['orders'][1]['amount']==first
    assert plan['orders'][2]['to']==target and plan['orders'][2]['amount']==second
    REPORT['checks'].append(map_id+'-repeat-target-half-remaining')
    saved=view();browser('reload');ready();assert view()['orders']==saved['orders'],'reload lost private plans'
    loaded=js(r'performance.getEntriesByType("resource").map(r=>r.name).filter(n=>/\/(world|europe|usa|china)-[^/]+\.js/.test(n))')
    assert loaded and all('/'+map_id+'-' in u for u in loaded),loaded
    REPORT.setdefault('loadedResources',{})[map_id]=loaded
    browser('scrollintoview','.cq-viewport');screenshot(map_id+'-attack-icon-plan')
    before_round=view()['round'];click('提交回合');peer.act('submit')
    wait('Number(document.querySelector("[data-conquest-board]").dataset.round)>'+str(before_round))
    assert view()['round']==before_round+1
    REPORT['checks'].append(map_id+'-native-icon-actions-resolve')
    screenshot(map_id+'-resolved')


def preserve_platform_controls(map_id,peer):
    # Shared controls remain usable after the large form has been removed.
    click('提议暂停');peer.act('pause',enabled=True)
    wait('document.querySelector("[data-conquest-board]").dataset.phase==="paused"')
    saved=view();browser('reload');ready();assert view()==saved
    click('恢复对局');peer.act('resume',enabled=True)
    wait('document.querySelector("[data-conquest-board]").dataset.phase==="planning"')
    click('开启托管');wait('!!document.querySelector(".autoplay-notice")')
    assert js('[...document.querySelectorAll(".cq-territory-actions button,.cq-submit")].every(e=>e.disabled)')
    browser('click','.autoplay-control');wait('!document.querySelector(".autoplay-notice")')
    # Remove any normal bot plans before observing a clean result transition.
    browser('scrollintoview','.cq-viewport');before=js('document.querySelector(".cq-viewport").getBoundingClientRect().toJSON()')
    peer.act('resign');browser('wait','[role=dialog][aria-label="本局结果"]')
    assert view()['phase']=='finished'
    click('关闭');wait('!document.querySelector(".match-result-modal")')
    after=js('document.querySelector(".cq-viewport").getBoundingClientRect().toJSON()')
    assert abs(before['y']-after['y'])<1,(before,after)
    api('/state');assert not js('!!document.querySelector(".match-result-modal")')
    click('查看结果');browser('wait','.match-result-modal');click('关闭')
    screenshot(map_id+'-final-board')
    command('rematch');browser('wait','.waiting-actions')
    command('ready');peer.command('ready');command('start');peer.ready_load();ready()
    assert view()['phase']=='planning' and view()['round']==1
    peer.act('resign');browser('wait','[role=dialog][aria-label="本局结果"]');click('关闭')
    screenshot(map_id+'-next-result')
    REPORT['checks'].append(map_id+'-pause-autoplay-result-controls')


def fort_case(map_id,peer):
    browser('set','viewport','320','780');new_game(map_id,peer)
    own=next(p['id'] for p in view()['provinces'] if p['owner']==0)
    p=focus_region(map_id,own);px,py=pinch_centre(p);native(dict(mode='pinch',x=px,y=py,start=22,end=65,steps=32,pace=16))
    button='[data-cq-action="fort"]';browser('scrollintoview',button)
    box=js('document.querySelector('+json.dumps(button)+').getBoundingClientRect().toJSON()')
    assert 0<=box['left']<box['right']<=320 and 0<=box['top']<box['bottom']<=780,box
    native(dict(mode='tap',x=box['x']+box['width']/2,y=box['y']+box['height']/2,steps=1,pace=0));wait_orders(1)
    d=view();assert d['orders'][0]['type']=='build' and d['orders'][0]['building']=='fort' and d['availableGold']==0
    screenshot(map_id+'-native-fort-plan')
    round=d['round'];click('提交回合');peer.act('submit')
    wait('Number(document.querySelector("[data-conquest-board]").dataset.round)>'+str(round))
    assert next(p for p in view()['provinces'] if p['id']==own)['fort']
    marker=js('(()=>{const e=document.querySelector('+json.dumps('[data-cq-fort="'+own+'"]')+'),r=e?.getBoundingClientRect();return r?{rect:r.toJSON(),visible:r.width>0&&r.height>0&&r.bottom>0&&r.top<innerHeight&&r.right>0&&r.left<innerWidth}:null})()')
    assert marker and marker['visible'],marker
    screenshot(map_id+'-fort-built')
    REPORT['checks'].append(map_id+'-native-fort-resolves')
    finish(peer)


def mobile_entry(map_id,width,peer):
    browser('set','viewport',str(width),'780')
    new_game(map_id,peer)
    # The platform intentionally reveals .game-main after its initial layout
    # settles. Observe that real milestone; do not scroll it on the app's behalf.
    wait('document.querySelector(".game-main").getBoundingClientRect().top<=16')
    d=view();own=d['players'][room()['you']]['king'];p=point_for(map_id,own)
    geo=json.loads((ROOT/'games/conquest/maps'/f'{map_id}.json').read_text())
    metric=js('(()=>{const v=document.querySelector(".cq-viewport").getBoundingClientRect(),e=document.querySelector('+json.dumps('[data-cq-province="'+own+'"]')+'),r=e.getBoundingClientRect(),m=e.getScreenCTM();return {viewport:v.toJSON(),territory:r.toJSON(),scale:m.a,window:{width:innerWidth,height:innerHeight},scrollY,visibleMapHeight:Math.max(0,Math.min(innerHeight,v.bottom)-Math.max(0,v.top))}})()')
    metric.update(map=map_id,own=own,point=p,renderedMapHeight=geo['height']*metric['scale'])
    (OUT/(map_id+'-'+str(width)+'-entry.json')).write_text(json.dumps(metric,ensure_ascii=False,indent=2))
    assert metric['renderedMapHeight']>=metric['viewport']['height']*.9,metric
    assert min(metric['territory']['width'],metric['territory']['height'])>=24,metric
    assert metric['viewport']['left']<=p['x']<=metric['viewport']['right'] and metric['viewport']['top']<=p['y']<=metric['viewport']['bottom'],metric
    assert 0<=p['x']<=width and 0<=p['y']<=780,metric
    screenshot(map_id+'-'+str(width)+'-first-entry')
    REPORT['checks'].append(map_id+'-'+str(width)+'-initial-home-territory-visible')
    # A user can scroll up to read the shared player bar and then tap a visible
    # home region before its action row fits. The application must reveal it.
    browser('scroll','up','2000')
    p=point_for(map_id,own);assert 0<=p['x']<=width and 0<=p['y']<=780,p
    native(dict(mode='tap',x=p['x'],y=p['y'],steps=1,pace=0))
    wait('(()=>{const e=document.querySelector(".cq-territory-actions");if(!e)return false;const r=e.getBoundingClientRect();return r.top>=0&&r.bottom<=innerHeight})()')
    toolbar=js('document.querySelector(".cq-territory-actions").getBoundingClientRect().toJSON()')
    REPORT.setdefault('mobileActionRows',[]).append({'map':map_id,'width':width,'bounds':toolbar,'appRevealed':True})
    screenshot(map_id+'-'+str(width)+'-first-selected-actions')
    finish(peer)


def eight_fog_and_spectator():
    browser('set','viewport','320','780')
    api('/rooms',dict(kind='conquest',name='地图八席迷雾验收',capacity=8,options={'mapId':'world','fog':True,'difficulty':'normal','turnSeconds':180}))
    for _ in range(7):command('add_bot')
    command('ready');command('start');ready()
    d=view();assert len(d['players'])==8 and any(p['troops']==-1 for p in d['provinces'])
    for seat in range(8):
        selector='[data-cq-gold="'+str(seat)+'"]';browser('scrollintoview',selector)
        value=js('(()=>{const e=document.querySelector('+json.dumps(selector)+'),r=e.getBoundingClientRect();return {text:e.textContent,rect:r.toJSON(),visible:r.left>=0&&r.right<=innerWidth&&r.top>=0&&r.bottom<=innerHeight&&e.contains(document.elementFromPoint(r.x+r.width/2,r.y+r.height/2))}})()')
        assert value['visible'],value
    own=d['players'][0]['king'];focus_region('world',own)
    assert js('!!document.querySelector("[data-cq-action=tower]")'),'fog mode lost scout construction'
    screenshot('eight-fog-map-actions')
    click('开启托管');wait('!!document.querySelector(".autoplay-notice")')
    assert js('[...document.querySelectorAll(".cq-territory-actions button,.cq-submit")].every(e=>e.disabled)')
    browser('click','.autoplay-control');wait('!document.querySelector(".autoplay-notice")')
    spectator=Peer('地图观众'+RUN);spectator.api('/rooms/'+room()['id']+'/watch',{})
    api('/logout',{});api('/login',dict(name=spectator.name,password=PASSWORD));browser('reload');ready()
    assert room()['spectating'] and view()['orders']==[]
    focus_region('world',own)
    assert not js('!!document.querySelector(".cq-territory-actions")') and js('document.querySelector(".cq-submit").disabled')
    screenshot('spectator-map-readonly')
    api('/logout',{});api('/login',dict(name='地图验收'+RUN,password=PASSWORD));browser('reload');ready()
    command('close');command('leave')
    REPORT['checks'].append('eight-seats-fog-resources-autoplay-spectator')


def queue_regressions(peer,cases=('undo','reject')):
    for case in cases:
        browser('set','viewport','390','780');new_game('europe',peer)
        own=view()['players'][room()['you']]['king'];focus_region('europe',own);action('attack')
        geo=json.loads((ROOT/'games/conquest/maps/europe.json').read_text());target=next(p for p in geo['provinces'] if p['id']==own)['neighbors'][0]
        point=point_for('europe',target)
        if case=='undo':
            native(dict(mode='tap',x=point['x'],y=point['y'],steps=1,pace=0));wait_orders(1)
            first=view()['orders'][0];action('plans');click('撤销计划 '+str(first['id']));wait_orders(0);click('关闭')
            assert view()['orders']==[]
            screenshot('queue-undo-before-retry')
        else:
            before=view();native(dict(mode='tap',x=point['x'],y=point['y'],steps=1,pace=0,rejectMove=True))
            wait('document.body.textContent.includes("验收拒绝一次派兵")')
            assert view()['orders']==[] and view()['provinces']==before['provinces']
            # The small regression entry isolates queue recovery; still records
            # colors. The default full suite enforces contrast at this state.
            screenshot('queue-refused-before-retry',check_contrast=not ARGS.queue_case)
        # Retain the armed source, then actually click the target again. No
        # reload, mode toggle or test-only state reset can clear the stuck receipt.
        point=point_for('europe',target)
        native(dict(mode='tap',x=point['x'],y=point['y'],steps=1,pace=0))
        wait_orders(1)
        result=view()['orders'];assert len(result)==1 and result[0]['type']=='move' and result[0]['to']==target and result[0]['amount']==250,result
        if case=='undo':assert result[0]['id']>first['id']
        screenshot('queue-'+case+'-retry-success')
        REPORT['checks'].append('queue-'+case+'-retry-remains-usable')
        finish(peer)


def marker_case(peer):
    browser('set','viewport','320','780');new_game('world',peer)
    wait('document.querySelector(".game-main").getBoundingClientRect().top<=16')
    known=[p['id'] for p in view()['provinces'] if p['owner']>=0 and p['troops']>0]
    assert len(known)==2
    present=js('[...document.querySelectorAll("[data-cq-label]")].map(e=>e.dataset.cqLabel)')
    record={'knownArmies':known,'renderedMarkers':present,'scope':'pre-rendered DOM availability only; outside markers may be SVG-clipped and are not claimed visible'}
    (OUT/'marker-pre-render.json').write_text(json.dumps(record,indent=2))
    screenshot('world-mobile-marker-pre-render')
    assert all(p in present for p in known),record
    REPORT['checks'].append('world-offscreen-known-armies-pre-rendered')
    finish(peer)


def hold_actions(peer):
    browser('set','viewport','390','780');new_game('europe',peer)
    own=view()['players'][0]['king'];focus_region('europe',own)
    js('window.__uxActions=[];const original=window.fetch;window.fetch=(...args)=>{try{const b=JSON.parse(args[1]?.body??"null");if(b?.type==="action")window.__uxActions.push(b.action)}catch{}return original(...args)}')
    box=js('document.querySelector("[data-cq-action=recruit]").getBoundingClientRect().toJSON()')
    assert box['top']>=0 and box['bottom']<=780,box
    native(dict(mode='hold',x=box['x']+box['width']/2,y=box['y']+box['height']/2,steps=1,pace=0,holdMS=750))
    wait_orders(1);d=view();requests=js('window.__uxActions')
    (OUT/'long-recruit.json').write_text(json.dumps({'orders':d['orders'],'availableGold':d['availableGold'],'availableMovement':d['availableMovement'],'requests':requests},indent=2))
    assert len(d['orders'])==1 and d['orders'][0]['amount']==5000 and d['availableGold']==0 and abs(d['availableMovement']-2.2)<.001,d
    assert len(requests)==1 and requests[0]['amount']==5000,requests
    screenshot('hold-recruit-max-once');REPORT['checks'].append('long-recruit-one-order-one-point')
    finish(peer)
    # Gain a non-town province through normal recruitment and combat. A town
    # is unaffordable, while a fort is affordable, so falling through is visible.
    new_game('europe',peer);own=view()['players'][0]['king'];focus_region('europe',own);action('recruit');wait_orders(1);action('attack')
    geo=json.loads((ROOT/'games/conquest/maps/europe.json').read_text());target=next(p for p in geo['provinces'] if p['id']==own)['neighbors'][0];point=point_for('europe',target)
    native(dict(mode='tap',x=point['x'],y=point['y'],steps=1,pace=0));wait_orders(2);round=view()['round'];click('提交回合');peer.act('submit')
    wait('Number(document.querySelector("[data-conquest-board]").dataset.round)>'+str(round))
    d=view();land=next(p for p in d['provinces']if p['id']==target);assert land['owner']==0 and not land['town']
    assert d['controls']['fortCost']<=d['availableGold']<d['controls']['townCost'],d['availableGold']
    focus_region('europe',target);before=view()['orders'];request_count=js('window.__uxActions.length');box=js('document.querySelector("[data-cq-action=fort]").getBoundingClientRect().toJSON()')
    native(dict(mode='hold',x=box['x']+box['width']/2,y=box['y']+box['height']/2,steps=1,pace=0,holdMS=750))
    assert view()['orders']==before,'unaffordable long town request fell through to short fort'
    notice=js('document.querySelector(".cq-map-message").textContent')
    assert any(word in notice for word in ['金币','行动点','不足','需要']),notice
    browser('focus','[data-cq-action=fort]');browser('press','Shift+Enter')
    js('new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(resolve)))')
    assert view()['orders']==before,'unaffordable keyboard town request fell through to fort'
    assert not any(a.get('type')=='build' and a.get('building')=='fort' for a in js('window.__uxActions')[request_count:]),'long town emitted a short fort request'
    screenshot('hold-town-does-not-fall-through');REPORT['checks'].append('long-town-and-shift-enter-do-not-build-fort')
    finish(peer)


def creation_maps(peer):
    browser('set','viewport','390','780')
    for map_id in ['world','europe','usa','china']:
        click('创建房间');browser('find','role','button','click','--name','征服时代','--exact')
        browser('wait','[aria-label="征服地图"]')
        values=js('[...document.querySelector("[aria-label=征服地图]").options].map(o=>o.value)')
        assert values==['world','europe','usa','china'],values
        assert js('document.querySelector("[aria-label=征服地图]").value')=='world'
        if map_id!='world':browser('select','[aria-label="征服地图"]',map_id)
        browser('screenshot',str(OUT/('create-'+map_id+'.png')))
        browser('click','.create-room-modal button[type=submit]')
        browser('wait','.waiting-actions');assert room()['options']['mapId']==map_id
        peer.command('join',room());peer.command('ready');command('ready');command('start');peer.ready_load();ready()
        assert view()['mapId']==map_id
        REPORT['checks'].append('create-selected-'+map_id)
        finish(peer)


def regional_small_targets(map_id):
    if map_id not in ['usa','china']:return
    browser('set','viewport','320','780')
    geo=json.loads((ROOT/'games/conquest/maps'/f'{map_id}.json').read_text())
    def contains(point,ring):
        x,y=point;odd=False
        for a,b in zip(ring,ring[1:]+ring[:1]):
            if (a[1]>y)!=(b[1]>y) and x<(b[0]-a[0])*(y-a[1])/(b[1]-a[1])+a[0]:odd=not odd
        return odd
    for province in (['us-dc','us-hi','us-ak'] if map_id=='usa' else ['cn-hk','cn-mo','cn-tw','cn-hi']):
        region=next(p for p in geo['provinces'] if p['id']==province)
        fragments=[]
        for raw in region['path'].split('Z'):
            nums=[float(v) for v in re.findall(r'-?\d+(?:\.\d+)?',raw)]
            ring=list(zip(nums[::2],nums[1::2]))
            if ring and contains((region['label']['x'],region['label']['y']),ring):fragments.append(ring)
        fragment=min(fragments,key=lambda r:(max(x for x,y in r)-min(x for x,y in r))*(max(y for x,y in r)-min(y for x,y in r)))
        bounds=(max(x for x,y in fragment)-min(x for x,y in fragment),max(y for x,y in fragment)-min(y for x,y in fragment))
        reset_map();browser('scrollintoview','.cq-viewport')
        for _ in range(8):
            point=point_for(map_id,province);v=point['viewport']
            cx=(v['left']+v['right'])/2;cy=(max(v['top'],0)+min(v['bottom'],780))/2
            dx=max(-v['width']*.38,min(v['width']*.38,cx-point['x']));visible_height=min(v['bottom'],780)-max(v['top'],0)
            dy=max(-visible_height*.38,min(visible_height*.38,cy-point['y']))
            # Start on unobstructed map centre, never on an offscreen target or
            # a fixed zoom button covering its tiny unzoomed fragment.
            native(dict(mode='drag',x=cx,y=cy,dx=dx,dy=dy,steps=24,pace=16))
            point=point_for(map_id,province)
            scale=js('document.querySelector('+json.dumps('[data-cq-province="'+province+'"]')+').getScreenCTM().a')
            if min(bounds)*scale>=25 and v['left']+8<point['x']<v['right']-8 and max(0,v['top'])+8<point['y']<min(780,v['bottom'])-8:break
            native(dict(mode='pinch',x=cx,y=cy,start=22,end=62,steps=32,pace=16))
        point=point_for(map_id,province)
        scale=js('document.querySelector('+json.dumps('[data-cq-province="'+province+'"]')+').getScreenCTM().a')
        assert min(bounds)*scale>=24,(province,bounds,scale)
        assert 0<point['x']<320 and 0<point['y']<780,point
        native(dict(mode='tap',x=point['x'],y=point['y'],steps=1,pace=0))
        wait('document.querySelector("[data-conquest-board]").dataset.cqSelected==='+json.dumps(province))
        (OUT/(map_id+'-'+province+'-hit.json')).write_text(json.dumps({'fragmentCSSWidth':bounds[0]*scale,'fragmentCSSHeight':bounds[1]*scale,'point':point,'scale':scale},indent=2))
        screenshot(map_id+'-native-small-'+province)
        REPORT['checks'].append(map_id+'-native-small-region-'+province)


try:
    if not BASE:
        if ARGS.baseline:
            baseline_dist=AREA/'baseline-dist'
            assert (baseline_dist/'index.html').is_file(),'build and retain the old UI before starting baseline acceptance'
            if (ROOT/'web/dist').exists():shutil.move(ROOT/'web/dist',saved_dist)
            swapped_dist=True
            shutil.copytree(baseline_dist,ROOT/'web/dist')
        if not ARGS.skip_build and not ARGS.baseline:
            with (OUT/'build.log').open('w') as h:subprocess.run(['npm','--prefix','web','run','build'],cwd=ROOT,env=ENV,stdout=h,stderr=subprocess.STDOUT,check=True)
        for name in ['url','stop']:(FIXTURE/name).unlink(missing_ok=True)
        h=(OUT/'server.log').open('w');handles.append(h)
        server=subprocess.Popen(['go','test','-count=1','-timeout=31m','-run','^TestConquestBrowserFixture$','./games/conquest/tests'],cwd=ROOT,env=ENV,stdout=h,stderr=subprocess.STDOUT)
        until=time.monotonic()+180
        while not (FIXTURE/'url').exists():
            if server.poll() is not None or time.monotonic()>until:raise RuntimeError('map UX fixture did not start; see server.log')
            time.sleep(.1)
        BASE=(FIXTURE/'url').read_text().strip()
    assert urllib.parse.urlparse(BASE).hostname in ('127.0.0.1','localhost'),'acceptance must never target production'
    if ARGS.baseline:
        shutil.copyfile(Path(__file__),OUT/'runner.py')
        shutil.copyfile(Path(__file__).with_name('map_ux_input.mjs'),OUT/'native-input.mjs')
    browser('open',BASE);opened=True
    api('/register',dict(name='地图验收'+RUN,password=PASSWORD,invite=INVITE))
    peer=Peer('地图对手'+RUN);browser('reload');browser('wait','.room-list')
    if ARGS.queue_case:
        queue_regressions(peer,('undo','reject') if ARGS.queue_case=='all' else (ARGS.queue_case,))
    elif ARGS.hold_actions:hold_actions(peer)
    elif ARGS.marker_case:marker_case(peer)
    else:
        if not ARGS.baseline:creation_maps(peer)
        for map_id in MAP_IDS:
            if not ARGS.baseline:
                for width in [320,390]:mobile_entry(map_id,width,peer)
            new_game(map_id,peer)
            for width in [1440,320,390]:
                gestures(map_id,width);map_first_invariants(map_id,width)
            if not ARGS.baseline:
                regional_small_targets(map_id);orders(map_id,peer);preserve_platform_controls(map_id,peer)
            finish(peer)
            if not ARGS.baseline:fort_case(map_id,peer)
        if not ARGS.baseline:eight_fog_and_spectator();queue_regressions(peer);hold_actions(peer);marker_case(peer)
    REPORT['passed']=not REPORT['violations']
    if ARGS.baseline:REPORT['baselineCaptured']=True
except Exception as error:
    traceback.print_exc();REPORT['error']=repr(error)
    if opened:
        try:
            REPORT['state']=room();REPORT['snapshot']=browser('snapshot','-i');browser('screenshot',str(OUT/'failure.png'),'--full')
        except Exception:pass
finally:
    if opened:
        try:browser('close')
        except Exception:pass
    if server:
        (FIXTURE/'stop').write_text('stop')
        try:server.wait(timeout=15)
        except subprocess.TimeoutExpired:server.terminate();server.wait(timeout=10)
    for h in handles:h.close()
    shutil.rmtree(PROFILE,ignore_errors=True)
    if swapped_dist:
        shutil.rmtree(ROOT/'web/dist')
        if saved_dist.exists():shutil.move(saved_dist,ROOT/'web/dist')
    for name in ['url','stop']:(FIXTURE/name).unlink(missing_ok=True)
    if REPORT['passed'] and not ARGS.baseline:
        source_end=source_hashes();changed=[p for p,s in SOURCE_START.items() if source_end.get(p)!=s]
        if changed:REPORT['passed']=False;REPORT['error']='source changed during acceptance: '+str(changed)
    (OUT/'result.json').write_text(json.dumps(REPORT,ensure_ascii=False,indent=2)+'\n')
    if REPORT['passed'] and not ARGS.baseline:
        binding={'sourceFiles':SOURCE_START,'sourceTreeSHA256':hashlib.sha256(json.dumps(SOURCE_START,sort_keys=True).encode()).hexdigest(),'resultSHA256':hashlib.sha256((OUT/'result.json').read_bytes()).hexdigest(),'buildIndexSHA256':hashlib.sha256((ROOT/'web/dist/index.html').read_bytes()).hexdigest(),'scope':'successful real-room native-input run; source stable during run'}
        (OUT/'source-binding.json').write_text(json.dumps(binding,indent=2)+'\n')
    if not REPORT['passed'] and not REPORT.get('baselineCaptured'):raise SystemExit(1)
