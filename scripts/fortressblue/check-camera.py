#!/usr/bin/env python3
"""Real legal high-arc camera acceptance; see CAMERA_BOUNDARY_SCENARIOS.md.

Reuse the existing fixture/input helpers without running its other acceptance
suites. No game state, clocks or projectile paths are replaced.
"""
import ast
from pathlib import Path

helper = Path(__file__).with_name('check-refactor.py')
module = ast.parse(helper.read_text(), filename=str(helper))
# The helper defines its utilities, then enters one top-level try/finally suite.
# Execute only the definitions/setup prefix; --out is its shared CLI option.
first_suite = next(i for i, node in enumerate(module.body) if isinstance(node, ast.Try))
exec(compile(ast.Module(body=module.body[:first_suite], type_ignores=[]), str(helper), 'exec'), globals())
import math
report.update(selection='camera', command='python3 -B scripts/fortressblue/check-camera.py --out '+str(out.relative_to(root)), scope='production rules, normal opening and legal shots; live original-map camera coordinates; native mouse/touch')


def bounds(frame, dimensions):
    c, rect = frame['canvas'], frame['board']
    x, y, width = (float(c[key]) for key in ['cameraX','cameraY','worldViewWidth'])
    height = width * rect['h'] / rect['w']
    assert x >= -.02 and y >= -.02, (x,y)
    assert x+width <= dimensions[0]+.1 and y+height <= dimensions[1]+.1, (x,y,width,height,dimensions)
    return {'x':x,'y':y,'width':width,'height':height}


try:
    assert (root/'web/dist/index.html').exists(),'Parent must build frontend first'
    (out/'scenario').write_text('normal')
    log=(out/'server.log').open('w')
    server=subprocess.Popen(['go','test','-count=1','-timeout=12m','-run','^TestFortressBlueBrowserFixture$','./games/fortressblue/tests'],cwd=root,env=env,stdout=log,stderr=subprocess.STDOUT)
    wait(lambda:(out/'url').exists(),timeout=180,label='fixture URL')
    base=(out/'url').read_text();browser('open',base);opened=True
    api('/login',{'name':'坦克验收管理员','password':'fortress-browser-test-only'})
    other('/register',{'name':'坦克静止对手','password':'fortress-browser-test-only','invite':'fortress-browser'})
    browser('reload');browser('wait','.room-list')
    for screen_width in [1280,320]:
        browser('set','viewport',str(screen_width),'1000' if screen_width>320 else '844')
        start(tank_id=1);ensure_own_turn();browser('focus','[data-fortressblue-canvas]')
        dimensions=js('(()=>{const t=document.querySelector(".fb-terrain");return [t.width,t.height]})()')
        # Public position/wind determine a legal near-vertical shot with an
        # apex just above the image and compensated horizontal wind drift.
        v=view();tank=next(t for t in v['tanks'] if t['seat']==0)
        vy=math.sqrt(2*260*(tank['y']-26+80))
        vx=-v['wind']*14*vy/260
        power=(math.hypot(vx,vy)/.9-120)/7
        assert 0<power<=100,{'power':power,'position':tank,'wind':v['wind']}
        fire={'type':'fire','epoch':v['epoch'],'turn':v['turn'],'angle':math.degrees(math.atan2(vy,abs(vx))),'power':power,'facing':-1 if vx<0 else 1,'weapon':'primary'}
        action(fire)
        result=native([{'sample':True},{'wait':1600,'capture':'high-arc-'+str(screen_width)},{'wait':6500,'read':'after-return'}])
        frames=[];above=[];returned=[];saw_above=False
        for f in result['frames']:
            if not f.get('canvas'):continue
            b=bounds(f,dimensions)
            projectiles=json.loads(f['canvas'].get('renderedProjectiles','[]'))
            for p in projectiles:
                if p['y']<0:saw_above=True;above.append({'camera':b,'shell':p})
                elif saw_above:returned.append({'camera':b,'shell':p})
            frames.append({'at':f['at'],'camera':b,'shells':projectiles,'renderedAt':f['canvas'].get('renderedAt')})
        record('high-arc-'+str(screen_width),{'action':fire,'mapSize':dimensions,'frames':frames,'lastShot':view().get('lastShot')})
        assert len(above)>=2,'legal trajectory did not provide above-map samples'
        assert len(returned)>=2,'shell did not return from above map'
        assert all(row['camera']['y']<=1 for row in above),above[:3]
        assert any(row['camera']['y']>10 for row in returned),'camera did not resume downward tracking'
        check('legal high arc stays physically above map while camera clamps and resumes '+str(screen_width),aboveFrames=len(above),returnFrames=len(returned))
        wait(lambda:not view().get('resolution'),timeout=15)
        r=rect('[data-fortressblue-canvas]');p={'x':r['x']+r['w']*.5,'y':r['y']+r['h']*.45}
        if screen_width>320:
            gestures=[{'mouse':'mousePressed',**p},{'mouse':'mouseMoved','x':p['x'],'y':p['y']+r['h']*2},{'mouse':'mouseReleased','x':p['x'],'y':p['y']+r['h']*2},{'wait':100,'read':'pan-top'},{'mouse':'mouseWheel',**p,'dy':10000},{'wait':200,'read':'zoom-out','capture':'bounded-camera-desktop'}]
        else:
            a=[{'id':1,'x':p['x']-60,'y':p['y']},{'id':2,'x':p['x']+60,'y':p['y']}]
            b=[{'id':1,'x':p['x']-8,'y':p['y']},{'id':2,'x':p['x']+8,'y':p['y']}]
            gestures=[{'touch':'touchStart',**p},{'touch':'touchMove','x':p['x'],'y':p['y']+r['h']*2},{'touch':'touchEnd'},{'wait':100,'read':'pan-top'},{'touch':'touchStart','points':a},{'touch':'touchMove','points':b},{'touch':'touchEnd'},{'wait':200,'read':'zoom-out','capture':'bounded-camera-320'}]
        samples=native(gestures)
        record('manual-bounds-'+str(screen_width),[bounds(f,dimensions) for f in samples['frames'] if f.get('canvas')])
        check('native manual pan and zoom remain inside map '+str(screen_width))
    report['passed']=True
except Exception:
    report['error']=traceback.format_exc();print(report['error'],flush=True)
finally:
    if opened:
        try:browser('close')
        except Exception:report['passed']=False
    (out/'stop').write_text('stop')
    if server:
        try:server.wait(timeout=20)
        except subprocess.TimeoutExpired:server.terminate();server.wait(timeout=10);report['passed']=False
        if server.returncode:report['passed']=False
    if log:log.close()
    for path in [temp,sockets,out/'data']:
        if path.exists():shutil.rmtree(path)
    for name in ['url','stop','scenario']:(out/name).unlink(missing_ok=True)
    report['cleanup']=['owned browser profile, socket directory, temporary SQLite and build files']
    record('report',report);print(out/'report.json')
raise SystemExit(0 if report['passed'] else 1)
