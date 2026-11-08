#!/usr/bin/env python3
"""GROWTH.md failure cases, specified before UI implementation.
Production UI/HTTP/Canvas; native desktop mouse and 320px Chromium touch.
Repair and combat fixtures only establish labelled boundaries, not campaigns.
Build the matching frontend first; --skip-build skips the helper's rebuild.
"""
import json
import shutil
import sys
import time
from pathlib import Path
import accept_combat_controls_ui as c

w=c.w
w.OUT=c.ROOT/'.local/starfort-growth/browser'
w.OUT.mkdir(parents=True,exist_ok=True)
w.ENV.update(WIRE_BOARD_STARFORT_BROWSER_OUT=str(w.OUT),AGENT_BROWSER_PROFILE=str(w.OUT/'profile'))
checks=[]
passed=False

def mark(name,value):
    assert value,name
    checks.append(name)
    print('PASS',name,flush=True)

try:
    c.setup()
    source_start=c.source_hashes()
    for width,height in [(1366,900),(320,740)]:
        touch=width==320
        c.b('set','viewport',width,height)
        w.scenario('rebalance-growth-repair')
        c.enter()
        c.activate('[data-sf-open-tasks]',touch)
        mark('new rule visible '+str(width),w.data().get('growthRules')==1)
        mark('repair panel inside fullscreen '+str(width),c.visibility('[data-sf-repair-panel]')['inside'])
        quote=w.data()['controls']['repair']
        mark('affordable repair quote '+str(width),quote['maxAmount']==3 and quote['unitCost']==100)
        c.b('fill','[data-sf-repair-amount]','2')
        before=w.data()['players'][0]['gold']
        c.activate('[data-sf-repair]',touch)
        c.wait('document.querySelector("[data-sf-base]").textContent==="22"')
        after=w.data()
        mark('native paid repair '+str(width),after['baseHP']==22 and after['players'][0]['gold']==before-200)
        mark('shared quota updated '+str(width),after['controls']['repair']['remaining']==1)
        mark('repair controls align '+str(width),abs(c.rect('[data-sf-repair-amount]')['bottom']-c.rect('[data-sf-repair]')['bottom'])<1)
        c.screenshot('repair-'+str(width))
        c.b('reload')
        c.wait('document.querySelector(".sf-battlefield")?.dataset.assetsReady==="true"')
        mark('repair survives reload '+str(width),w.data()['baseHP']==22 and w.data()['controls']['repair']['remaining']==1)
        c.enter()
        c.activate('[data-sf-open-tasks]',touch)
        c.activate('[data-sf-repair]',touch)
        c.wait('document.querySelector("[data-sf-base]").textContent==="23"')
        mark('quota disables further repair '+str(width),c.js('document.querySelector("[data-sf-repair]").disabled'))
        c.activate('.sf-tasks-dialog [aria-label="关闭"]',touch)
        c.hud_geometry('hud-'+str(width))
        c.leave()
        for kind,branch,expected in [('cannon','power',['12%','102','22%']),('arrow','volley',['14%','80','3','52%']),('arcane','power',['×1.5','×1.1'])]:
            w.scenario('rebalance-growth-'+kind)
            c.enter()
            selector=c.choose_visible_tower(kind)
            c.activate(selector,touch)
            c.activate('[data-sf-upgrade='+branch+']',touch)
            c.wait('!!document.querySelector("[data-sf-skill]")')
            c.activate('[data-sf-skill]',touch)
            c.wait('!!document.querySelector("[data-sf-skill-tooltip]")')
            text=c.js('document.querySelector("[data-sf-skill-tooltip]").textContent')
            mark('current growth description '+kind+' '+str(width),all(v in text for v in expected))
            mark('growth description visible '+kind+' '+str(width),c.visibility('[data-sf-skill-tooltip]')['visible'])
            c.screenshot('skill-'+kind+'-'+str(width))
            w.artifact('skill-'+kind+'-'+str(width)+'.json',{'text':text,'state':w.data()})
            c.leave()
        w.scenario('rebalance-growth-refund')
        deadline=time.monotonic()+10
        while time.monotonic()<deadline and w.data()['world']['objectives'][0]['status']!='failed':
            time.sleep(.1)
        state=w.data()
        mark('real timer refunds failed task '+str(width),state['world']['objectives'][0].get('refunded')==21 and state['players'][0]['gold']==216)
        c.enter()
        c.activate('[data-sf-open-tasks]',touch)
        mark('failed-task refund shown '+str(width),'退还 21 金币' in c.js('document.querySelector(".sf-tasks-dialog").textContent'))
        c.screenshot('refund-'+str(width))
        c.activate('.sf-tasks-dialog [aria-label="关闭"]',touch)
        c.leave()
    mark('sources unchanged',source_start==c.source_hashes())
    mark('no browser errors',not c.b('errors')['errors'])
    passed=True
except Exception:
    import traceback
    traceback.print_exc()
    if w.OPENED:
        try:
            c.screenshot('failure')
            w.artifact('failure-state.json',{'state':w.data(),'text':c.js('document.body.innerText')})
        except Exception:pass
finally:
    if w.OPENED:
        try:c.b('close')
        except Exception:pass
    if w.SERVER:
        (w.OUT/'stop').write_text('stop')
        w.SERVER.wait(timeout=20)
    w.artifact('result.json',{'passed':passed,'checks':checks,'sources':c.source_hashes(),'buildSHA256':c.hashes([c.ROOT/'web/dist/index.html']),'scope':__doc__})
    for p in [w.OUT/'profile',w.OUT/'tmp',w.PROFILE]:
        if p.exists():shutil.rmtree(p)
    sys.exit(0 if passed else 1)
