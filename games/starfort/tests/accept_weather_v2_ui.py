#!/usr/bin/env python3
"""WEATHER_V2.md production HTTP/Canvas acceptance, written before implementation.
Milestone fixtures cover visible status and medical actions; natural runs are separate.
320px uses Chromium native touch emulation, not a physical phone.
"""
import json
import shutil
import sys
import accept_combat_controls_ui as c

w = c.w
w.OUT = c.ROOT / '.local/starfort-weather-v2/browser'
w.OUT.mkdir(parents=True, exist_ok=True)
(w.OUT/'tmp').mkdir(exist_ok=True)
w.ENV.update(WIRE_BOARD_STARFORT_BROWSER_OUT=str(w.OUT), AGENT_BROWSER_PROFILE=str(w.OUT/'profile'), GOTMPDIR=str(w.OUT/'tmp'), TMPDIR=str(w.OUT/'tmp'))
checks = []
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
            w.scenario('weather-v2-'+weather)
            check('schema 9 public weather '+weather+' '+str(width), w.data().get('weatherRules') == 2)
            check('weather notice before wave '+str(width), c.visibility('[data-sf-weather-notice]')['visible'])
            c.enter()
            geometry = c.js('(()=>{const e=document.querySelector("[data-sf-weather]");if(!e)return null;const r=e.getBoundingClientRect(),p=e.parentElement.getBoundingClientRect();return {id:e.dataset.sfWeather,top:r.top-p.top,height:r.height,field:p.height,pointer:getComputedStyle(e).pointerEvents}})()')
            check('weather top ten percent '+weather+' '+str(width), geometry and geometry['id']==weather and geometry['top']>=-1 and geometry['height']<=geometry['field']*.101+1 and geometry['pointer']=='none', geometry)
            c.screenshot('weather-'+weather+'-'+str(width))
            c.leave()
        w.scenario('weather-v2-sleep')
        c.enter()
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
        w.scenario('weather-v2-medical')
        c.enter()
        check('medical action fits fullscreen '+str(width), c.visibility('[data-sf-use-medical]')['inside'])
        before=w.data()
        c.activate('[data-sf-use-medical]',touch)
        c.wait('document.querySelector("[data-sf-base]")?.textContent==="23"')
        check('medical heals without spending gold '+str(width), w.data()['baseHP']==23 and w.data()['players'][0]['gold']==before['players'][0]['gold'])
        check('consumed medical action removed '+str(width), not c.js('!!document.querySelector("[data-sf-use-medical]")'))
        c.screenshot('medical-used-'+str(width))
        c.b('reload')
        c.wait('document.querySelector(".sf-battlefield")?.dataset.assetsReady==="true"')
        c.enter()
        check('medical consumption survives reload '+str(width), w.data()['baseHP']==23 and not c.js('!!document.querySelector("[data-sf-use-medical]")'))
        c.hud_geometry('weather-v2-hud-'+str(width))
        c.leave()
    check('sources unchanged', sources==c.source_hashes())
    check('no browser errors',not c.b('errors')['errors'])
    passed=True
except Exception as error:
    import traceback
    traceback.print_exc()
    w.artifact("failure.json", {"error":str(error)})
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
