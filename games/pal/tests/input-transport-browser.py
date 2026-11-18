#!/usr/bin/env python3
"""Actual PAL room/native key E2E with initial HTTP input and a delayed ACK.

Requires a live Board fixture and board-browser.py's original test accounts.
The fixture holds the real socket handshake until the first HTTP input applies.
Fails if a 1-frame tap becomes a held level while its release waits for an ACK,
the original hero walks more than one step, or actual canvas pixels never move.
The interceptor changes only network delivery, never native/game state.
"""
import json
import argparse
import os
from pathlib import Path
import shutil
import signal
import subprocess
import time

ROOT=Path(__file__).resolve().parents[3]
parser=argparse.ArgumentParser(description=__doc__)
parser.add_argument('--base')
parser.add_argument('--user')
parser.add_argument('--password',default='pal-board-test-123')
parser.add_argument('--gate-dir',default='.local/pal/board-browser')
parser.add_argument('--dist',default='web/dist',help='the local app build served by the fixture')
args=parser.parse_args()
OUT=ROOT/'.local/pal/input-transport-browser';OUT.mkdir(parents=True,exist_ok=True)
BASE=args.base or (ROOT/'.local/pal/board-browser/url').read_text().strip()
USER=args.user or json.loads((ROOT/'.local/pal/board-browser/accounts.json').read_text())[0]
gate_dir=(ROOT/args.gate_dir).resolve()
assert gate_dir.is_relative_to(ROOT/'.local/pal'), 'test gate must stay in PAL workspace output'
GATE=gate_dir/'hold-ws'
render_probe=(ROOT/args.dist/'pal-renderer-probe.js').resolve()
assert render_probe.is_relative_to(ROOT), 'renderer verification output must stay in the workspace'
ENV=dict(os.environ,AGENT_BROWSER_SOCKET_DIR=str(ROOT/'.local/pab'),AGENT_BROWSER_PROFILE=str(OUT/'profile'),TMPDIR=str(ROOT/'.local/tmp'),AGENT_BROWSER_NO_VERSION_CHECK='1',AGENT_BROWSER_ARGS='--mute-audio,--disable-breakpad,--disable-crash-reporter')
Path(ENV['AGENT_BROWSER_PROFILE']).mkdir(exist_ok=True)
subprocess.run(['node','--input-type=module','-e','import {build} from "esbuild";await build({entryPoints:[process.argv[1]],outfile:process.argv[2],bundle:true,format:"esm",target:"es2022"});',str(ROOT/'games/pal/tests/field-render-probe.ts'),str(render_probe)],cwd=ROOT/'web',env=ENV,check=True)
SESSION=subprocess.check_output(['agent-browser','session','id','--scope','worktree','--prefix','pit'],cwd=ROOT,env=ENV,text=True).strip()
REPORT={'passed':False,'scope':'actual original PAL inn, authenticated platform Board/native keyboard, controlled pending WebSocket handshake and initial HTTP input with deliberately delayed 800ms ACK; correctness verification, not a latency benchmark'}
def browser(*args):
    p=subprocess.run(['agent-browser','--session',SESSION,*map(str,args),'--json'],cwd=ROOT,env=ENV,capture_output=True,text=True,timeout=45)
    assert p.returncode==0,p.stdout+p.stderr
    value=json.loads(p.stdout);assert value['success'],value;return value['data']
def js(source):return browser('eval',source)['result']
def state():
    return js('(async()=>{const d=(await(await fetch("/api/state")).json()).room.game.data;return {x:d.x,y:d.y,seq:d.seq,appliedSeq:d.appliedSeq,phase:d.phase}})()')
def pixels():
    return js('(async()=>{let c=document.querySelector("[data-pal-board] canvas"),p=c.getContext("2d").getImageData(0,0,320,200).data;return [...new Uint8Array(await crypto.subtle.digest("SHA-256",p))].map(x=>x.toString(16).padStart(2,"0")).join("")})()')
def wait(predicate,label,seconds=20):
    end=time.monotonic()+seconds
    while time.monotonic()<end:
        if predicate():return
        time.sleep(.08)
    raise AssertionError(label)
monitor=None
def trace_snapshot():
    monitor.send_signal(signal.SIGUSR2)
    return json.loads(monitor.stdout.readline())
try:
    browser('open',BASE);browser('set','viewport',1280,900)
    js('(async()=>{const r=await fetch("/api/login",{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify('+json.dumps({'name':USER,'password':args.password})+')});if(!r.ok)throw Error(await r.text());return true})()')
    monitor=subprocess.Popen(['node','games/pal/tests/input-transport-trace.mjs',browser('get','cdp-url')['cdpUrl'],BASE],cwd=ROOT,env=ENV,text=True,stdout=subprocess.PIPE,stdin=subprocess.PIPE)
    assert monitor.stdout.readline().strip()=='ready'
    GATE.touch()
    browser('reload');browser('wait','[data-pal-ready=true]')
    wait(lambda:js('!!document.querySelector("[data-pal-phase=field] [data-pal-direction=down]:not(:disabled)")'),'original field controller not ready')
    browser('click','[data-pal-board] canvas')
    initial=state();initial_pixels=pixels()
    assert initial['phase']=='field',initial
    browser('press','ArrowDown')
    expected=[initial['x']-16,initial['y']+8]
    def moved():
        current=state()
        return [current['x'],current['y']]!=[initial['x'],initial['y']]
    wait(moved,'native HTTP short direction was lost')
    GATE.unlink() # The live socket can recover while the HTTP ACK is delayed.
    wait(lambda:state()['appliedSeq']>=initial['seq']+2,'queued physical release did not reach original rules')
    # Check multiple original frames after release, including the delayed ACK.
    samples=[]
    for _ in range(6):
        current=state();samples.append(current)
        assert [current['x'],current['y']]==expected,{'message':'a released tap walked more than one original step','initial':initial,'expected':expected,'samples':samples}
        time.sleep(.12)
    wait(lambda:pixels()!=initial_pixels,'original movement never reached real canvas pixels')
    keys=js('window.palNativeKeyEvents')
    assert len(keys)==2 and keys[0]['kind']=='keydown' and keys[1]['kind']=='keyup',keys
    assert keys[1]['at']-keys[0]['at']<100,keys
    trace=trace_snapshot()
    inputs=[event for event in trace['events'] if 'action' in event]
    assert trace['delayed'] and len(inputs)==2 and inputs[0]['transport']=='Network.requestWillBeSent',trace
    assert inputs[0]['action']['direction']==4 and inputs[0]['action']['keys']==4 and inputs[1]['action']['direction']==4 and inputs[1]['action']['keys']==0,trace
    assert inputs[1]['at']-inputs[0]['at']>=750,trace
    assert not any('error' in event for event in trace['events']),trace
    # Continue through the recovered live connection with an actual held key.
    # Verify stopping after the release is both received and applied, without
    # treating this as a local-response latency measurement.
    held_before=state()
    browser('click','[data-pal-board] canvas')
    wait(lambda:js('!!document.querySelector("[data-pal-phase=field][data-pal-ready=true] [data-pal-direction=left]:not(:disabled)")'),'displayed held-key controller is not ready')
    monitor.stdin.write('hold-left\n');monitor.stdin.flush()
    wait(lambda:js('window.palNativeKeyEvents.some(event=>event.code==="ArrowLeft"&&event.kind==="keyup")'),'native held key was not released')
    def release_sent():
        sent=[event['action'] for event in trace_snapshot()['events'] if 'action' in event and event['action']['seq']>held_before['seq']]
        return sent and sent[-1]['direction']==4 and sent[-1]['keys']==0
    wait(release_sent,'held-key release was not submitted')
    sent=[event['action'] for event in trace_snapshot()['events'] if 'action' in event and event['action']['seq']>held_before['seq']]
    assert any(action['direction']==1 and action['keys']==0 for action in sent),'physical hold never renewed its original movement level'
    release_seq=sent[-1]['seq']
    wait(lambda:state()['appliedSeq']>=release_seq,'held-key release did not reach native rules')
    stopped=state();steps=(held_before['x']-stopped['x'])//16
    assert stopped['phase']=='field' and steps>=2 and stopped['x']==held_before['x']-steps*16 and stopped['y']==held_before['y']-steps*8,{'before':held_before,'after':stopped}
    held_samples=[]
    for _ in range(5):
        current=state();held_samples.append(current)
        assert [current['x'],current['y']]==[stopped['x'],stopped['y']],'original hero kept walking after the applied held-key release'
        time.sleep(.12)
    wait(lambda:js('(async()=>{const {authorityPixelsMatch}=await import("/pal-renderer-probe.js"),d=(await(await fetch("/api/state")).json()).room.game.data;return authorityPixelsMatch(d)})()'),'predicted Canvas did not converge to the actual authoritative original frame')
    keys=js('window.palNativeKeyEvents')
    assert len(keys)==4 and keys[2]['code']=='ArrowLeft' and keys[2]['kind']=='keydown' and keys[3]['kind']=='keyup',keys
    monitor.terminate();trace,_=monitor.communicate(timeout=5);monitor=None;trace=json.loads(trace)
    REPORT.update(initial=initial,expected=expected,samples=samples,nativeKeys=keys,trace=trace,canvasChanged=True,authorityPixelsMatchAfterRelease=True,heldMovement={'before':held_before,'after':stopped,'steps':steps,'releaseSeq':release_seq,'stoppedSamples':held_samples})
    browser('screenshot',OUT/'stopped-after-http-release.png','--full')
    REPORT['passed']=True
finally:
    GATE.unlink(missing_ok=True)
    if not REPORT['passed']:
        try:
            REPORT['debug']={'state':state(),'keys':js('window.palNativeKeyEvents'),'dom':js('({focus:document.hasFocus(),active:document.activeElement?.outerHTML,phase:document.querySelector("[data-pal-board]")?.dataset.palPhase,error:document.querySelector(".pal-board-error")?.textContent})'),'trace':trace_snapshot() if monitor else None}
        except Exception:pass
    if monitor:monitor.terminate();monitor.wait(timeout=5)
    if not REPORT['passed']:
        try:browser('screenshot',OUT/'failure.png','--full')
        except Exception:pass
    try:browser('close')
    except Exception:pass
    shutil.rmtree(OUT/'profile',ignore_errors=True)
    render_probe.unlink(missing_ok=True)
    (OUT/'result.json').write_text(json.dumps(REPORT,ensure_ascii=False,indent=2)+'\n')
    print(json.dumps(REPORT,ensure_ascii=False),flush=True)
