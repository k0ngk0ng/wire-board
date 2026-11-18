#!/usr/bin/env python3
"""Inspect the real PAL room layout after the original Board acceptance reached it.

Requires the live TestPALBoardBrowserFixture and its test account evidence.
Tests full-width canvas, desktop without virtual buttons, native mobile touch
emulation and translucent controls wholly within the same canvas. Fullscreen
also checks maximum proportional size, reachable controls without overlapping
labels, native save/cancel, and the same Canvas after exit.
"""
import json
import argparse
import os
from pathlib import Path
import shutil
import subprocess

ROOT=Path(__file__).resolve().parents[3]
parser=argparse.ArgumentParser(description=__doc__)
parser.add_argument('--fullscreen',action='store_true')
parser.add_argument('--user')
parser.add_argument('--base',help='URL of an already running PAL room fixture')
parser.add_argument('--password',default='pal-board-test-123',help='test account password')
args=parser.parse_args()
OUT=ROOT/('.local/pal/fullscreen-room' if args.fullscreen else '.local/pal/layout-room');OUT.mkdir(parents=True,exist_ok=True)
BASE=args.base or (ROOT/'.local/pal/board-browser/url').read_text().strip()
user=args.user or json.loads((ROOT/'.local/pal/board-browser/accounts.json').read_text())[0]
ENV=dict(os.environ,AGENT_BROWSER_SOCKET_DIR=str(ROOT/'.local/pab'),AGENT_BROWSER_PROFILE=str(OUT/'profile'),AGENT_BROWSER_NO_VERSION_CHECK='1',AGENT_BROWSER_ARGS='--mute-audio,--disable-breakpad,--disable-crash-reporter')
Path(ENV['AGENT_BROWSER_PROFILE']).mkdir(parents=True,exist_ok=True)
SESSION=subprocess.check_output(['agent-browser','session','id','--scope','worktree','--prefix','po'],env=ENV,text=True).strip()
def browser(*args):
    r=subprocess.run(['agent-browser','--session',SESSION,*map(str,args),'--json'],env=ENV,cwd=ROOT,capture_output=True,text=True,timeout=45)
    assert r.returncode==0,r.stdout+r.stderr
    data=json.loads(r.stdout);assert data['success'],data;return data['data']
def js(code):return browser('eval',code)['result']
def tap(selector):
    x,y=js('(()=>{const e=document.querySelector('+json.dumps(selector)+'),r=e.getBoundingClientRect();return [r.x+r.width/2,r.y+r.height/2]})()')
    r=subprocess.run(['node','scripts/browser_touch.mjs',browser('get','cdp-url')['cdpUrl'],BASE,'tap',*map(str,[x,y,x,y])],cwd=ROOT,env=ENV,capture_output=True,text=True,timeout=25)
    assert r.returncode==0,r.stdout+r.stderr
REPORT={'passed':False,'scope':'actual platform PAL room, full-width proportional surface, desktop keyboard layout and native mobile overlay geometry','fullscreen':args.fullscreen,'cases':[]}
mobile_process=None
try:
    browser('open',BASE)
    js('(async()=>{const r=await fetch("/api/login",{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify('+json.dumps({'name':user,'password':args.password})+')});if(!r.ok)throw Error(await r.text());return true})()')
    browser('reload');browser('wait','[data-pal-ready="true"]');browser('wait','--fn','!document.querySelector("[data-game-loading]")')
    js('window.originalPalCanvas=document.querySelector("[data-pal-board] canvas");true')
    for label,width,height in [('desktop',1280,1000),('phone',320,820),('landscape',820,320)]:
        if mobile_process:mobile_process.terminate();mobile_process.wait(timeout=5);mobile_process=None
        browser('set','viewport',width,height)
        if label!='desktop':
            mobile_process=subprocess.Popen(['node','scripts/browser_device.mjs',browser('get','cdp-url')['cdpUrl'],BASE,str(width),str(height),'--keep'],cwd=ROOT,env=ENV,text=True,stdout=subprocess.PIPE)
            assert mobile_process.stdout.readline().strip()=='ready'
        if args.fullscreen and label=='desktop':
            browser('click','[data-fullscreen-enter]');browser('wait','[data-room-fullscreen]')
            REPORT['fullscreenMode']=js('document.querySelector("[data-room-fullscreen]").dataset.roomFullscreen')
        data=js('(async()=>{await new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(()=>requestAnimationFrame(r))));const c=document.querySelector("[data-pal-board] canvas"),s=document.querySelector("[data-pal-stage]"),p=document.querySelector("[data-pal-controls]"),a=c.getBoundingClientRect(),v=s.getBoundingClientRect(),buttons=[...p.querySelectorAll("button")].filter(e=>e.getClientRects().length);return {canvas:{x:a.x,y:a.y,w:a.width,h:a.height},stage:{w:v.width,h:v.height},same:c===window.originalPalCanvas,touch:matchMedia("(pointer:coarse)").matches,controls:!!p.getClientRects().length,inside:buttons.every(e=>{const r=e.getBoundingClientRect();return r.left>=a.left-1&&r.right<=a.right+1&&r.top>=a.top-1&&r.bottom<=a.bottom+1}),background:buttons.length?getComputedStyle(buttons[0]).backgroundColor:null}})()')
        assert data['same'] and abs(data['canvas']['w']/data['canvas']['h']-1.6)<.01,data
        if args.fullscreen:
            assert abs(data['canvas']['w']-min(width,height*1.6))<2,{'message':'fullscreen toolbar/padding reduced game surface',**data}
            reachable=js('(()=>{const controls=[...document.querySelectorAll("[data-room-fullscreen] .game-fullscreen-toolbar button,[data-pal-panel-action],[data-pal-controls] button")].filter(e=>e.getClientRects().length&&!e.disabled);return controls.every(e=>{const r=e.getBoundingClientRect(),x=r.x+r.width/2,y=r.y+r.height/2;return x>=0&&x<innerWidth&&y>=0&&y<innerHeight&&e.contains(document.elementFromPoint(x,y))})})()')
            assert reachable,{'message':'fullscreen toolbar obscures playable controls',**data}
            assert js('(()=>{const title=document.querySelector(".pal-play-status > span"),t=title?.getBoundingClientRect();return !t||[...document.querySelectorAll("[data-pal-controls] button")].filter(e=>e.getClientRects().length).every(e=>{const b=e.getBoundingClientRect();return b.right<=t.left||b.left>=t.right||b.bottom<=t.top||b.top>=t.bottom})})()'),{'message':'player label overlaps virtual controls',**data}
        else:assert abs(data['canvas']['w']-data['stage']['w'])<2,data
        assert data['controls']==(label!='desktop'),data
        if label!='desktop':assert data['touch'] and data['inside'] and 'rgba' in data['background'],data
        if args.fullscreen:browser('screenshot',OUT/(label+'.png'))
        else:browser('screenshot',OUT/(label+'.png'),'--full')
        REPORT['cases'].append({'label':label,**data})
        if args.fullscreen and label=='phone':
            tap('[data-room-fullscreen] [data-save-game]')
            browser('wait','[data-room-fullscreen] [data-cancel-save]')
            assert js('document.querySelector("[data-pal-board] canvas")===window.originalPalCanvas')
            assert abs(js('document.querySelector("[data-pal-board] canvas").getBoundingClientRect().width')-width)<2
            browser('screenshot',OUT/'phone-save.png')
            tap('[data-room-fullscreen] [data-cancel-save]')
            browser('wait','[data-room-fullscreen] [data-save-game]')
            REPORT['cases'][-1]['nativeSaveCancelWithoutCanvasChange']=True
    if args.fullscreen:
        tap('[data-room-fullscreen] .room-fullscreen-toggle')
        browser('wait','--fn','!document.querySelector("[data-room-fullscreen]")')
        assert js('document.querySelector("[data-pal-board] canvas")===window.originalPalCanvas&&!document.querySelector("header[inert]")')
        REPORT['sameCanvasAfterExit']=True
    REPORT['passed']=True
finally:
    if mobile_process:mobile_process.terminate();mobile_process.wait(timeout=5)
    try:browser('close')
    except Exception:pass
    shutil.rmtree(OUT/'profile',ignore_errors=True)
    (OUT/'result.json').write_text(json.dumps(REPORT,ensure_ascii=False,indent=2)+'\n')
    print(json.dumps(REPORT,ensure_ascii=False))
