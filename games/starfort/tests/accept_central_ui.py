#!/usr/bin/env python3
"""Production browser evidence for fixed central routes and passive animation.
Failure inventory precedes implementation in EVOLUTION.md. Deterministic fixture
only sets starting combatants; actual shots, craters, repairs and HP use the server.
"""
import json
import time
import shutil
import sys
sys.argv.append("--central-case")
import accept_combat_controls_ui as c
w=c.w
w.OUT=w.OUT.parent/'central'
w.OUT.mkdir(parents=True,exist_ok=True)
w.PROFILE=w.OUT/'profiles'/w.SESSION
w.PROFILE.mkdir(parents=True,exist_ok=True)
w.ENV['AGENT_BROWSER_PROFILE']=str(w.PROFILE)
w.ENV['WIRE_BOARD_STARFORT_BROWSER_OUT']=str(w.OUT)
for key in ['TMPDIR','GOTMPDIR']:
    (w.OUT/'tmp').mkdir(exist_ok=True);w.ENV[key]=str(w.OUT/'tmp')

def run():
    try:
        w.REPORT['scope']='Central map 1–4 seats, production passive events and Canvas, PC and 320px touch viewport'
        w.REPORT['buildSHA256']=c.hashes((c.ROOT/'web/dist').rglob('*'))
        before=c.source_hashes()
        c.setup()
        w.PEERS.extend([w.Peer("中央防线同伴二"),w.Peer("中央防线同伴三")])
        for n in [1,2,3,4]:
            c.b('set','viewport','1366','900')
            w.scenario('central-passives',n)
            c.enter()
            state=w.data();m=state['map']
            c.verify(str(n)+' seats have fixed centered routes',len(m['routes'])==n and m['base']=={'x':m['width']/2,'y':m['height']/2},m)
            original=json.dumps(m,sort_keys=True)
            c.screenshot('central-'+str(n)+'-routes')
            c.leave()
            c.verify('view change preserves map',json.dumps(w.data()['map'],sort_keys=True)==original)
        for width,height in [(1366,900),(320,740)]:
            c.b('set','viewport',str(width),str(height));w.scenario('central-passives',2);c.enter()
            c.hud_geometry('central-'+str(width))
            target=w.data()['enemies'][0]
            point=c.js('''(()=>{const e=document.querySelector('[data-sf-minimap]'),p=e.createSVGPoint();p.x='''+str(target['x'])+';p.y='+str(target['y'])+''';const r=p.matrixTransform(e.getScreenCTM());return {x:r.x,y:r.y}})()''')
            w.mouse_click(point['x'],point['y']);c.stable()
            observed=set();workers=[];samples=[];deadline=time.monotonic()+24
            captured=False
            while time.monotonic()<deadline:
                state=w.data();observed.update(e['type'] for e in state.get('events',[]))
                active=[p for p in state.get('craters',[]) if p['remaining']>0]
                samples.append({'elapsedMS':state['elapsedMS'],'craters':state.get('craters',[]),'events':state.get('events',[])})
                if active:
                    workers.extend(len(p.get('workers',[])) for p in active)
                    if not captured and max(len(p.get('workers',[])) for p in active)>=2:
                        # Wait for buffered production playback to reach this impact.
                        time.sleep(1.3);c.screenshot('central-repair-'+str(width));captured=True
                if captured and {'crater_filled','frost_ring','chain'}.issubset(observed):break
                time.sleep(.2)
            w.artifact('central-events-'+str(width)+'.json',samples)
            c.verify(str(width)+' live passive and repair events',{'crater','repair_start','crater_repair','crater_filled','frost_ring','chain'}.issubset(observed),sorted(observed))
            c.verify(str(width)+' multiple living workers visible',captured and max(workers,default=0)>=2,workers)
            c.button_geometry('central-'+str(width));c.screenshot('central-combat-'+str(width));c.leave()
        c.verify('no browser errors',not c.b('errors')['errors'])
        c.verify('sources unchanged',before==c.source_hashes())
        w.REPORT['passed']=True
    except Exception as error:
        import traceback
        traceback.print_exc();w.REPORT['error']=repr(error)
        if w.OPENED:
            try:c.screenshot('central-failure')
            except Exception:pass
    finally:
        if w.OPENED:
            try:c.b('close')
            except Exception:pass
        if w.SERVER:
            (w.OUT/'stop').write_text('stop')
            try:w.SERVER.wait(timeout=15)
            except Exception:w.SERVER.terminate();w.SERVER.wait(timeout=10)
        w.artifact('central-result.json',w.REPORT)
        for path in [w.OUT/'profiles',w.OUT/'tmp']:
            if path.is_dir():shutil.rmtree(path)
        sys.exit(0 if w.REPORT.get('passed') else 1)
if __name__=='__main__':run()
