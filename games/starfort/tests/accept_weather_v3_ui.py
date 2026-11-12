#!/usr/bin/env python3
"""WEATHER_V3.md production HTTP/Canvas acceptance, written before implementation.
Milestone fixtures cover visible status and medical actions; natural runs are separate.
320px uses Chromium native touch emulation, not a physical phone.
"""
import json
import shutil
import sys
import accept_combat_controls_ui as c

w = c.w
w.OUT = c.ROOT / '.local/starfort-weather-v3/browser'
w.OUT.mkdir(parents=True, exist_ok=True)
(w.OUT/'tmp').mkdir(exist_ok=True)
w.ENV.update(WIRE_BOARD_STARFORT_BROWSER_OUT=str(w.OUT), AGENT_BROWSER_PROFILE=str(w.OUT/'profile'), GOTMPDIR=str(w.OUT/'tmp'), TMPDIR=str(w.OUT/'tmp'))
checks = []
def scenario_ready(name):
    w.scenario(name)
    # HTTP actions can finish before the WebSocket replaces the previous room.
    # Anchor readiness to this room's visible heading, not a stale game canvas.
    c.wait('document.querySelector(".room-heading h1, .room-page h1")?.textContent.includes('+json.dumps(name)+')')
    c.wait('document.querySelector(".sf-battlefield")?.dataset.assetsReady==="true"')
    c.stable()

def enter_ready():
    if not c.js('!!document.querySelector("[data-fullscreen-enter]")'): c.b('click','.compact-room-tools')
    stable=c.js('new Promise(resolve=>{let last="",stable=0,n=0;function frame(){const e=document.querySelector("[data-fullscreen-enter]"),r=e.getBoundingClientRect(),value=JSON.stringify([r.x,r.y,r.width,r.height]);if(r.y<0||r.bottom>innerHeight)e.scrollIntoView({block:"center",behavior:"instant"});stable=r.y>=0&&r.bottom<=innerHeight&&last===value?stable+1:0;last=value;if(stable>=20||++n>=180)resolve(stable>=20);else requestAnimationFrame(frame)}frame()})')
    check('fullscreen entry position settles',stable)
    c.activate('[data-fullscreen-enter]',c.js('innerWidth')<=320)
    c.wait('!!document.querySelector("[data-room-fullscreen]")')
    c.stable()

passed = False

def check(name, ok, evidence=None):
    checks.append({'name': name, 'passed': bool(ok), 'evidence': evidence})
    assert ok, (name, evidence)
    print('PASS', name, flush=True)

try:
    c.setup()
    sources = c.source_hashes()
    for width, height in [(1366, 900), (320, 740)]:
        touch = width == 320
        c.b('set', 'viewport', width, height)
        for weather in ['calm', 'fog', 'frost', 'starlight']:
            scenario_ready('weather-v3-'+weather)
            check('schema 10 public weather '+weather+' '+str(width), w.data().get('weatherRules') == 3)
            check('weather notice before wave '+weather+' '+str(width), c.visibility('[data-sf-weather-notice]').get('visible',False) == (weather != 'calm'))
            for fullscreen in [False, True]:
                if fullscreen: enter_ready()
                geometry = c.js('(()=>{const e=document.querySelector("[data-sf-weather]");if(!e)return null;const r=e.getBoundingClientRect(),parent=e.parentElement,p=parent.getBoundingClientRect();return {id:e.dataset.sfWeather,top:r.top-p.top-parent.clientTop,height:r.height,field:parent.clientHeight,pointer:getComputedStyle(e).pointerEvents}})()')
                check('calm has no weather overlay '+str(width), geometry is None) if weather=='calm' else check('weather covers full battlefield '+weather+' '+str(width), geometry and geometry['id']==weather and geometry['top']>=-1 and abs(geometry['height']-geometry['field'])<=1 and geometry['pointer']=='none', geometry)
                c.screenshot('weather-'+weather+'-'+str(width)+'-'+str(fullscreen))
                if fullscreen: c.leave()
        scenario_ready('weather-v3-fog')
        enter_ready()
        for kind, values in [('arrow',['15%','250','35%','70','8 秒']), ('cannon',['20%','18.0 秒','5 的倍数']), ('frost',['20%','3.0 秒','8.0 秒']), ('arcane',['20%','240','35%','5 秒','8 秒'])]:
            selector=c.choose_visible_tower(kind)
            point=c.painted_tower_point(selector)
            if touch: c.native('tap',point['x'],point['y'],0,0)
            else: w.mouse_click(point['x'],point['y'])
            if touch: c.activate('[data-sf-skill]',True)
            else: c.b('hover','[data-sf-skill]')
            c.wait('!!document.querySelector("[data-sf-skill-tooltip]")')
            text=c.js('document.querySelector("[data-sf-skill-tooltip]").textContent')
            check('actual skill values '+kind+' '+str(width),all(value in text for value in values),text)
            frames=c.js('new Promise(resolve=>{const frames=[];let n=0;function frame(){const e=document.querySelector("[data-sf-skill]"),r=e.getBoundingClientRect();frames.push({open:!!document.querySelector("[data-sf-skill-tooltip]"),rect:[r.x,r.y,r.width,r.height]});if(++n<60)requestAnimationFrame(frame);else resolve(frames)}frame()})')
            check('skill stable '+kind+' '+str(width),all(f['open'] for f in frames) and len(set(tuple(f['rect']) for f in frames))==1)
            check('tooltip inside viewport '+kind+' '+str(width),c.visibility('[data-sf-skill-tooltip]')['inside'] and c.visibility('[data-sf-skill-tooltip]')['visible'])
            w.artifact('skill-'+kind+'-'+str(width)+'.json',{'text':text,'frames':frames})
            c.screenshot('skill-'+kind+'-'+str(width))
            if touch: c.activate('[data-sf-skill]',True)
            else: c.b('mouse','move',5,200)
            c.wait('!document.querySelector("[data-sf-skill-tooltip]")')
            c.activate('[data-sf-tower-close]',touch)
        c.leave()
        scenario_ready('weather-v3-sleep')
        enter_ready()
        target=w.data()['enemies'][0]
        point=c.js('(()=>{const e=document.querySelector("[data-sf-minimap]"),p=e.createSVGPoint();p.x='+str(target['x'])+';p.y='+str(target['y'])+';const r=p.matrixTransform(e.getScreenCTM());return {x:r.x,y:r.y}})()')
        if touch:c.native('tap',point['x'],point['y'],0,0)
        else:w.mouse_click(point['x'],point['y'])
        c.stable()
        c.js('window.__sfSleepDraws=[];if(!window.__sfOldFillText){window.__sfOldFillText=CanvasRenderingContext2D.prototype.fillText;CanvasRenderingContext2D.prototype.fillText=function(text,x,y,...rest){if(String(text).includes("Z")&&this.canvas.hasAttribute("data-starfort-canvas")){const p=new DOMPoint(x,y).matrixTransform(this.getTransform());window.__sfSleepDraws.push({text,x:p.x,y:p.y,width:this.canvas.width,height:this.canvas.height,at:performance.now()});if(window.__sfSleepDraws.length>300)window.__sfSleepDraws.shift()}return window.__sfOldFillText.call(this,text,x,y,...rest)}}')
        c.wait('window.__sfSleepDraws.length>10')
        draws = c.js('window.__sfSleepDraws')
        check('sleep ZZZ actually drawn '+str(width), any(d['text']=='ZZZ' and 0<=d['x']<=d['width'] and 0<=d['y']<=d['height'] for d in draws), draws[-5:])
        c.b('set','media','light','reduced-motion')
        c.js('window.__sfSleepDraws=[]')
        c.wait('window.__sfSleepDraws.length>5')
        check('sleep remains visible with reduced motion '+str(width), bool(c.js('window.__sfSleepDraws.length')))
        c.screenshot('sleep-'+str(width))
        c.b('set','media','light')
        w.artifact('sleep-draws-'+str(width)+'.json',draws)
        c.leave()
    check('sources unchanged', sources==c.source_hashes())
    check('no browser errors',not c.b('errors')['errors'])
    passed=True
except Exception as error:
    import traceback
    traceback.print_exc()
    try:
        c.screenshot('failure')
        ui=c.js('({text:document.body.innerText,fullscreen:document.fullscreenElement?.tagName,button:document.querySelector("[data-fullscreen-enter]")?.outerHTML})')
    except Exception: ui=None
    w.artifact("failure.json", {"error":str(error),"ui":ui})
finally:
    if w.OPENED:
        try: c.b('close')
        except Exception: pass
    if w.SERVER:
        (w.OUT/'stop').write_text('stop')
        w.SERVER.wait(timeout=20)
    w.artifact('result.json',{'passed':passed,'checks':checks,'sources':c.source_hashes(),'scope':__doc__})
    for path in [w.OUT/'profile',w.OUT/'tmp',w.PROFILE]:
        if path.exists():shutil.rmtree(path)
    sys.exit(0 if passed else 1)
