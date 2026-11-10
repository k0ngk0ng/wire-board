#!/usr/bin/env python3
"""WEATHER.md browser acceptance specified before UI implementation.
Production HTTP/Canvas; native mouse, 320px touch emulation and fullscreen.
Fixtures establish weather/task/shop boundaries, never natural campaign proof.
"""
import json
import shutil
import sys
from pathlib import Path
import accept_combat_controls_ui as c

w = c.w
w.OUT = c.ROOT / '.local/starfort-weather/browser'
w.OUT.mkdir(parents=True, exist_ok=True)
(w.OUT/'tmp').mkdir(parents=True, exist_ok=True)
w.ENV.update(WIRE_BOARD_STARFORT_BROWSER_OUT=str(w.OUT), AGENT_BROWSER_PROFILE=str(w.OUT/'profile'), GOTMPDIR=str(w.OUT/'tmp'), TMPDIR=str(w.OUT/'tmp'))
checks = []
passed = False

def enter(touch):
    if c.js('!!document.querySelector(".sf-shop-dialog")'):
        c.activate('.sf-shop-dialog [aria-label="关闭"]', touch)
    c.enter()

def mark(name, value):
    assert value, name
    checks.append(name)
    print('PASS', name, flush=True)

try:
    c.setup()
    sources = c.source_hashes()
    for width, height in [(1366, 900), (320, 740)]:
        touch = width == 320
        c.b('set', 'viewport', width, height)
        for weather in ['calm', 'fog', 'frost', 'starlight']:
            w.scenario('weather-'+weather)
            mark('weather rule '+weather+' '+str(width), w.data().get('weatherRules') == 1)
            mark('wave weather announced before battle '+weather+' '+str(width), c.visibility('[data-sf-weather-notice]')['visible'])
            enter(touch)
            selector = '[data-sf-weather="'+weather+'"]'
            c.wait('!!document.querySelector('+json.dumps(selector)+')')
            mark('weather visible '+weather+' '+str(width), c.visibility(selector)['visible'])
            geometry = c.js('(()=>{const w=document.querySelector("[data-sf-weather]");const a=w.getBoundingClientRect(),b=w.parentElement.getBoundingClientRect();return {height:a.height,field:b.height,top:a.top-b.top,pointer:getComputedStyle(w).pointerEvents,reduced:matchMedia("(prefers-reduced-motion: reduce)").matches}})()')
            mark('weather top ten percent '+weather+' '+str(width), geometry['top'] >= -1 and geometry['height'] <= geometry['field']*.101+1 and geometry['pointer'] == 'none')
            if weather != 'calm':
                motion=c.js('new Promise(resolve=>{const values=[];let n=0;function frame(){values.push(getComputedStyle(document.querySelector("[data-sf-weather] i")).transform);if(++n<20)requestAnimationFrame(frame);else resolve([...new Set(values)])}frame()})')
                mark('weather animates '+weather+' '+str(width),len(motion)>1)
                c.b('set','media','light','reduced-motion')
                mark('reduced weather motion '+weather+' '+str(width),c.js('Array.from(document.querySelectorAll("[data-sf-weather] i")).every(e=>getComputedStyle(e).animationName==="none")'))
                c.b('set','media','light')
            c.activate('[data-sf-ready]',touch)
            c.wait('document.querySelector(".starfort-board")?.dataset.phase==="battle"')
            mark('weather continues during battle '+weather+' '+str(width),c.visibility(selector)['visible'])
            c.screenshot(weather+'-'+str(width))
            c.leave()
        w.scenario('weather-fog')
        enter(touch)
        selector = c.choose_visible_tower('arrow')
        c.activate(selector, touch)
        if touch:
            c.activate('[data-sf-skill]', touch)
        else:
            c.b('hover', '[data-sf-skill]')
        c.wait('!!document.querySelector("[data-sf-skill-tooltip]")')
        skill=c.js('document.querySelector("[data-sf-skill-tooltip]").textContent')
        mark('new weather skill values '+str(width),all(part in skill for part in ['12%','85','33%','50','8 秒']))
        mark('new skill tooltip visible '+str(width),c.visibility('[data-sf-skill-tooltip]')['visible'])
        frames=c.js('new Promise(resolve=>{const out=[];let n=0;function frame(){const e=document.querySelector("[data-sf-skill]");const r=e.getBoundingClientRect();const p=document.querySelector("[data-sf-skill-tooltip]");out.push({open:!!p,rect:[r.x,r.y,r.width,r.height]});if(++n<60)requestAnimationFrame(frame);else resolve(out)}frame()})')
        mark('stationary skill tooltip stays open '+str(width),all(f['open'] for f in frames))
        mark('stationary skill trigger stays fixed '+str(width),len(set(tuple(f['rect']) for f in frames))==1)
        w.artifact('skill-stability-'+str(width)+'.json',{'text':skill,'frames':frames})
        c.screenshot('weather-skill-'+str(width))
        if touch:
            c.activate('[data-sf-skill]',touch)
        else:
            c.b('mouse','move',5,200)
        before = w.data()
        c.activate('[data-sf-sell]', touch)
        c.wait('!!document.querySelector("[data-sf-sell-confirm]")')
        mark('sell opens confirmation '+str(width), len(w.data()['towers']) == len(before['towers']))
        mark('sale dialog visible '+str(width), c.visibility('[data-sf-sell-confirm]')['inside'])
        c.activate('[data-sf-sell-cancel]', touch)
        mark('cancel preserves tower and gold '+str(width), len(w.data()['towers']) == len(before['towers']) and w.data()['players'][0]['gold'] == before['players'][0]['gold'])
        c.activate('[data-sf-sell]', touch)
        c.activate('[data-sf-sell-accept]', touch)
        c.wait('!document.querySelector("[data-sf-sell-confirm]")')
        mark('confirmed sale exactly once '+str(width), len(w.data()['towers']) == len(before['towers'])-1 and w.data()['players'][0]['gold'] > before['players'][0]['gold'])
        c.leave()
        w.scenario('weather-shop')
        enter(touch)
        c.activate('[data-sf-open-shop]', touch)
        mark('shop visible '+str(width), c.visibility('[data-sf-shop]')['inside'])
        before = w.data()
        for choice in ['repair', 'damage', 'range', 'shield']:
            c.b('scrollintoview', '[data-sf-shop-buy="'+choice+'"]')
            c.stable()
            c.activate('[data-sf-shop-buy="'+choice+'"]', touch)
            c.wait('!document.querySelector("[data-sf-shop-buy='+choice+']") || document.querySelector("[data-sf-shop-buy='+choice+']").disabled')
        mark('all merchant purchases '+str(width), w.data()['baseHP'] == before['baseHP']+1 and w.data()['players'][0]['gold'] == before['players'][0]['gold']-1300)
        c.screenshot('shop-purchased-'+str(width))
        c.b('reload')
        c.wait('document.querySelector(".sf-battlefield")?.dataset.assetsReady==="true"')
        enter(touch)
        c.activate('[data-sf-open-shop]', touch)
        mark('shop quotas persist reload '+str(width), c.js('Array.from(document.querySelectorAll("[data-sf-shop-buy]")).every(b=>b.disabled)'))
        c.screenshot('shop-restored-'+str(width))
        c.activate('.sf-shop-dialog [aria-label="关闭"]', touch)
        c.hud_geometry('weather-hud-'+str(width))
        c.leave()
    mark('sources unchanged', sources == c.source_hashes())
    mark('no browser errors', not c.b('errors')['errors'])
    passed = True
except Exception:
    import traceback
    traceback.print_exc()
    if w.OPENED:
        try:
            c.screenshot('failure')
            w.artifact('failure-state.json', {'state': w.data(), 'text': c.js('document.body.innerText')})
        except Exception:
            pass
finally:
    if w.OPENED:
        try: c.b('close')
        except Exception: pass
    if w.SERVER:
        (w.OUT/'stop').write_text('stop')
        w.SERVER.wait(timeout=20)
    w.artifact('result.json', {'passed': passed, 'checks': checks, 'sources': c.source_hashes(), 'buildSHA256': c.hashes([c.ROOT/'web/dist/index.html']), 'scope': __doc__})
    for path in [w.OUT/'profile', w.OUT/'tmp', w.PROFILE]:
        if path.exists(): shutil.rmtree(path)
    sys.exit(0 if passed else 1)
