#!/usr/bin/env python3
"""Test-first original scene presentation: real canvas pixels vs original C.

The fixture only validates presentation; it does not claim a playable platform
game. All page, browser profile, sockets and screenshots remain in workspace.
"""
import hashlib
import gzip
import argparse
import base64
import http.server
import json
import os
from pathlib import Path
import shutil
import subprocess
import threading

ROOT=Path(__file__).resolve().parents[3]
parser=argparse.ArgumentParser(description=__doc__)
parser.add_argument('--dialogue',action='store_true')
parser.add_argument('--menus',action='store_true')
parser.add_argument('--opening',action='store_true')
parser.add_argument('--rng',action='store_true')
parser.add_argument('--scene-wave',action='store_true',help='original C map-wave frames, predicted movement and wave-bearing raster seed')
parser.add_argument('--controls',action='store_true')
parser.add_argument('--battle',choices=['attack','coop','wave'])
args=parser.parse_args()
OUT=ROOT/('.local/pal/controls-browser' if args.controls else '.local/pal/opening-palette-browser' if args.opening else '.local/pal/battle-'+args.battle+'-browser' if args.battle else '.local/pal/menus-browser' if args.menus else '.local/pal/dialogue-browser' if args.dialogue else '.local/pal/visual-browser')
if args.rng:OUT=ROOT/'.local/pal/rng-browser'
if args.scene_wave:OUT=ROOT/'.local/pal/scene-wave-browser'
ASSETS=ROOT/'.local/pal/visual'
OUT.mkdir(parents=True,exist_ok=True)
ENV=dict(os.environ,AGENT_BROWSER_SOCKET_DIR=str(ROOT/'.local/pal-ab'),AGENT_BROWSER_PROFILE=str(OUT/'profile'),AGENT_BROWSER_SCREENSHOT_DIR=str(OUT),TMPDIR=str(ROOT/'.local/tmp'),AGENT_BROWSER_NO_VERSION_CHECK='1',AGENT_BROWSER_ARGS='--mute-audio,--disable-breakpad,--disable-crash-reporter')
for key in ['AGENT_BROWSER_SOCKET_DIR','AGENT_BROWSER_PROFILE','TMPDIR']:Path(ENV[key]).mkdir(parents=True,exist_ok=True)
SESSION=subprocess.check_output(['agent-browser','session','id','--scope','worktree','--prefix','pv'],cwd=ROOT,env=ENV,text=True).strip()
REPORT={'passed':False,'scope':'original combat '+args.battle+' canvas pixel equivalence; isolated native battle, no campaign completion claim' if args.battle else 'original scene and dialogue canvas pixel equivalence' if args.dialogue else 'original scene canvas pixel equivalence; no gameplay completion claim','cases':[]}

def browser(*args):
    r=subprocess.run(['agent-browser','--session',SESSION,*map(str,args),'--json'],cwd=ROOT,env=ENV,text=True,capture_output=True,timeout=40)
    assert r.returncode==0,r.stdout+r.stderr
    data=json.loads(r.stdout);assert data['success'],data
    return data['data']

reference=json.loads((ROOT/('.local/pal/verification/scene-wave-reference.json' if args.scene_wave else '.local/pal/verification/original-rng-reference.json' if args.rng else '.local/pal/verification/original-opening-palette-reference.json' if args.opening else '.local/pal/verification/original-battle-'+args.battle+'-reference.json' if args.battle else '.local/pal/verification/original-menus-reference.json' if args.menus else '.local/pal/verification/original-inn-dialogue-reference.json' if args.dialogue else '.local/pal/verification/original-inn-reference.json')).read_text())
if args.scene_wave:
    assert len(reference['pages'])>=90 and any(p.get('predicted') for p in reference['pages'])
    REPORT['scope']='original C map-only wave pixels, actor/cover/text order, signed progression and phase wrapping, predicted movement and immutable scene seed; isolated fixture, not blood-pool campaign or Board acceptance'
if args.rng:
    assert len(reference['pages'])==410
    REPORT['scope']='all 410 original C sword lesson movie frames in desktop and mobile Canvas; isolated playback, not full platform/campaign'
if args.opening:assert all(p['view'].get('paletteEffect') for p in reference['pages']),'original palette fade metadata is missing'
if args.battle:assert len(reference['pages'])>=20 and all(p['view'].get('raster') for p in reference['pages']),'original offscreen combat graph is missing'
if args.dialogue:assert len(reference['pages'])==23,'all original inn pages need C raster references'
if args.menus:assert len(reference['pages'])==8,'all menu/restore cases need C raster references'
# Source imported below does not exist before this test is introduced.
entry=OUT/'entry.ts'
entry.write_text('''import {SceneRenderer} from "../../../games/pal/web/scene-renderer";
import React,{createRef} from "react";
import {createRoot} from "react-dom/client";
import {flushSync} from "react-dom";
import {PalStage} from "../../../games/pal/web/Stage";
import {PalControls} from "../../../games/pal/web/Controls";
import "../../../games/pal/web/board.css";
const canvasRef=createRef<HTMLCanvasElement>();
const root=createRoot(document.getElementById('root'));
const profile={revision:0,keyboard:{},turboHz:10};
const sink={pressPointer(){},releasePointer(){},pulse(){}};
const controls=CONTROL_TEST?React.createElement(PalControls,{profile,input:sink,disabled:false,read:false,alreadyRead:false}):undefined;
flushSync(()=>root.render(React.createElement('div',{className:'pal-board',style:{height:'100%'}},React.createElement(PalStage,{canvasRef,label:"仙剑原版场景",controls}))));
const canvas=canvasRef.current;
const reference=await(await fetch('/reference.json')).json();
const renderer=new SceneRenderer('/pal/v1');
const pages=reference.pages??[reference],hashes=[];
window.paintReference=async(index)=>{await renderer.paint(canvas,pages[index].view,()=>true,pages[index].predicted);};
for(let i=0;i<pages.length;i++){
try{await window.paintReference(i);}catch(error){window.result={error:String(error),index:i};break;}
const pixels=canvas.getContext('2d').getImageData(0,0,320,200).data;
const hash=[...new Uint8Array(await crypto.subtle.digest('SHA-256',pixels))].map(x=>x.toString(16).padStart(2,'0')).join('');
hashes.push(hash);window.result={hash,hashes,index:i};if(hash!==pages[i].rgbaSHA256)break;
}
document.body.dataset.ready='true';
'''.replace('CONTROL_TEST','true' if args.controls else 'false'))
subprocess.run(['node','--input-type=module','-e',"import {build} from 'esbuild';await build({entryPoints:[process.argv[1]],outfile:process.argv[2],bundle:true,format:'esm',target:'es2022',jsx:'automatic',nodePaths:[process.cwd()+'/node_modules']});",str(entry),str(OUT/'entry.js')],cwd=ROOT/'web',env=ENV,check=True)
(OUT/'reference.json').write_text(json.dumps(reference))
(OUT/'index.html').write_text('<!doctype html><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><link rel="stylesheet" href="/entry.css"><style>html,body,#root{margin:0;width:100%;height:100%;background:#101510;color:#eff4ec}#root{height:100dvh}</style><div id="root"></div><script type="module" src="/entry.js"></script>')

class Handler(http.server.SimpleHTTPRequestHandler):
    def translate_path(self,name):
        if name.startswith('/pal/v1/'):
            return str(ASSETS/'pal/v1'/name.removeprefix('/pal/v1/').split('?')[0])
        return super().translate_path(name)
    def log_message(self,*args):pass

server=http.server.ThreadingHTTPServer(('127.0.0.1',0),lambda *args,**kwargs:Handler(*args,directory=str(OUT),**kwargs))
mobile_process=None
threading.Thread(target=server.serve_forever,daemon=True).start()
try:
    browser('open',f'http://127.0.0.1:{server.server_port}')
    browser('wait','--fn','document.body.dataset.ready==="true"')
    result=browser('eval','window.result')['result']
    assert not result.get('error'),result
    pages=reference.get('pages',[reference])
    expected_hash=pages[result['index']]['rgbaSHA256']
    encoded=browser('eval',"(()=>{const p=document.querySelector('canvas').getContext('2d').getImageData(0,0,320,200).data;let s='';for(let i=0;i<p.length;i+=8192)s+=String.fromCharCode(...p.subarray(i,i+8192));return btoa(s)})()")['result']
    actual=base64.b64decode(encoded);(OUT/'canvas.rgba').write_bytes(actual)
    if result['hash']!=expected_hash:
        raw_path=(ROOT/('.local/pal/verification/original-scene-wave-pages.rgba' if args.scene_wave else '.local/pal/verification/original-rng-pages.rgba' if args.rng else '.local/pal/verification/original-opening-palette-pages.rgba' if args.opening else '.local/pal/verification/original-battle-'+args.battle+'-pages.rgba' if args.battle else '.local/pal/verification/original-menu-pages.rgba' if args.menus else '.local/pal/verification/original-inn-dialogue-pages.rgba' if args.dialogue else '.local/pal/verification/original-inn.rgba'))
        raw=raw_path.read_bytes() if raw_path.exists() else gzip.decompress(raw_path.with_suffix(raw_path.suffix+'.gz').read_bytes())
        expected=raw[result['index']*256000:(result['index']+1)*256000] if args.dialogue or args.menus or args.battle or args.opening or args.rng or args.scene_wave else raw
        changed=[i for i in range(320*200) if actual[i*4:i*4+4]!=expected[i*4:i*4+4]]
        print(json.dumps({'differentPixels':len(changed),'bounds':[min(i%320 for i in changed),min(i//320 for i in changed),max(i%320 for i in changed),max(i//320 for i in changed)],'samples':[[i%320,i//320,list(actual[i*4:i*4+4]),list(expected[i*4:i*4+4])] for i in changed[:8]]}))
        browser('screenshot',OUT/'mismatch.png','--full')
    assert result['hash']==expected_hash and len(result['hashes'])==len(pages),{'canvas':result,'reference':expected_hash}
    shot_index=55 if args.rng else 20 if args.scene_wave else 2 if args.menus else 0
    browser('eval','window.paintReference('+str(shot_index)+')')
    browser('eval','(()=>{window.originalCanvas=document.querySelector("canvas");return true})()')
    for width,height in ([(1280,900),(320,820),(820,320)] if args.controls else [(320,820),(820,320),(1280,900)]):
        if mobile_process:mobile_process.terminate();mobile_process.wait(timeout=5);mobile_process=None
        if (args.controls or args.rng or args.scene_wave) and width==320:browser('set','device','iPhone 14')
        browser('set','viewport',width,height)
        if (args.controls or args.rng or args.scene_wave) and width!=1280:
            mobile_process=subprocess.Popen(['node','scripts/browser_device.mjs',browser('get','cdp-url')['cdpUrl'],f'http://127.0.0.1:{server.server_port}',str(width),str(height),'--keep'],cwd=ROOT,env=ENV,text=True,stdout=subprocess.PIPE)
            assert mobile_process.stdout.readline().strip()=='ready'
        if args.rng or args.scene_wave:
            check=browser('eval',"""(async()=>{const pages=(await(await fetch('/reference.json')).json()).pages,c=document.querySelector('canvas');for(let i=0;i<pages.length;i++){await window.paintReference(i);const pixels=c.getContext('2d').getImageData(0,0,320,200).data,hash=[...new Uint8Array(await crypto.subtle.digest('SHA-256',pixels))].map(x=>x.toString(16).padStart(2,'0')).join('');if(hash!==pages[i].rgbaSHA256)return {passed:false,frame:i,hash};}return {passed:true,frames:pages.length,touch:matchMedia('(pointer:coarse)').matches};})()""")['result']
            assert check['passed'] and check['frames']==len(pages),check
            assert check['touch']==(width!=1280),check
            browser('eval','window.paintReference('+str(shot_index)+')')
        assert browser('eval','document.documentElement.scrollWidth<=innerWidth')['result']
        placement=browser('eval','(async()=>{await new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r)));const c=document.querySelector("canvas"),b=c.getBoundingClientRect();return {x:b.x,y:b.y,width:b.width,height:b.height,centerX:Math.abs(b.x+b.width/2-innerWidth/2),centerY:Math.abs(b.y+b.height/2-innerHeight/2),sameCanvas:c===window.originalCanvas}})()')['result']
        assert placement['centerX']<=1 and placement['centerY']<=1,{'viewport':[width,height],'placement':placement}
        assert abs(placement['width']/placement['height']-1.6)<.01 and placement['sameCanvas']
        if args.controls:
            expected_width=min(width,height*1.6)
            assert abs(placement['width']-expected_width)<2,{'message':'controls reduced the playable canvas','viewport':[width,height],'placement':placement,'expectedWidth':expected_width}
            if width==1280:assert browser('eval','[...document.querySelectorAll("[data-pal-controls]")].every(e=>!e.getClientRects().length)')['result'],'desktop virtual controls must be hidden'
            else:
                overlay=browser('eval','(()=>{const c=document.querySelector("canvas").getBoundingClientRect(),p=document.querySelector("[data-pal-controls]"),b=[...p.querySelectorAll("button")].filter(e=>e.getClientRects().length);return {touch:matchMedia("(pointer:coarse)").matches,visible:!!p.getClientRects().length,inside:b.every(e=>{const r=e.getBoundingClientRect();return r.left>=c.left-1&&r.right<=c.right+1&&r.top>=c.top-1&&r.bottom<=c.bottom+1}),background:getComputedStyle(b[0]).backgroundColor}})()')['result']
                assert overlay['touch'] and overlay['visible'] and overlay['inside'] and 'rgba' in overlay['background'],overlay
                point=browser('eval','(()=>{const r=document.querySelector("[data-pal-action=confirm]").getBoundingClientRect();return [r.x+r.width/2,r.y+r.height/2]})()')['result']
                subprocess.run(['node','scripts/browser_touch.mjs',browser('get','cdp-url')['cdpUrl'],f'http://127.0.0.1:{server.server_port}','tap',*map(str,point+point)],cwd=ROOT,env=ENV,check=True)
                assert browser('eval','matchMedia("(pointer:coarse)").matches && !!document.querySelector("[data-pal-controls]").getClientRects().length')['result'],'native touch helper cleared the mobile controller context'
        browser('screenshot',OUT/f'inn-{width}.png','--full')
        REPORT['cases'].append({'movieFramesCompared':410 if args.rng else None,'sceneWaveFramesCompared':len(pages) if args.scene_wave else None,'viewport':width,'height':height,'rgbaSHA256':pages[shot_index]['rgbaSHA256'],'originalMatch':True,'placement':placement})
    REPORT['pageHashes']=result['hashes']
    REPORT['passed']=True
finally:
    if mobile_process:mobile_process.terminate();mobile_process.wait(timeout=5)
    try:browser('close')
    except Exception:pass
    server.shutdown();server.server_close()
    shutil.rmtree(ENV['AGENT_BROWSER_PROFILE'],ignore_errors=True)
    (OUT/'result.json').write_text(json.dumps(REPORT,ensure_ascii=False,indent=2)+'\n')
    print(json.dumps({'passed':REPORT['passed'],'result':str(OUT/'result.json')}))
