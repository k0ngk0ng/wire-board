#!/usr/bin/env python3
"""Native audio in a real HTTP/WebSocket cadence room and production board.
Uses actual media playback, source setters and load counters. No mocked audio.
Failure inventory: platform-audio-reuse.test.mjs and AUDIO_PERFORMANCE.md.
"""
import json
import shutil
import sys
import time
from pathlib import Path
import accept_combat_controls_ui as c
w=c.w
w.OUT=c.ROOT/'.local/starfort-audio-reuse/browser'
w.OUT.mkdir(parents=True,exist_ok=True)
(w.OUT/'tmp').mkdir(exist_ok=True)
w.ENV.update(WIRE_BOARD_STARFORT_BROWSER_OUT=str(w.OUT),AGENT_BROWSER_PROFILE=str(w.OUT/'profile'),GOTMPDIR=str(w.OUT/'tmp'),TMPDIR=str(w.OUT/'tmp'),AGENT_BROWSER_INIT_SCRIPTS=str(c.ROOT/'games/starfort/tests/audio-reuse-probe.js'))
c.BASELINE=True;c.BASELINE_COMBAT=False
checks=[];passed=False

def source_hashes():
 return {**c.source_hashes(),**c.hashes([Path(__file__),c.ROOT/"games/starfort/tests/audio-reuse-probe.js",c.ROOT/"web/src/audio.ts",c.ROOT/"web/src/game-api/module.ts"])}

def check(name,ok,evidence=None):
 checks.append({'name':name,'passed':bool(ok),'evidence':evidence});print(('PASS 'if ok else'FAIL ')+name,flush=True);assert ok,(name,evidence)
try:
 c.setup();sources=source_hashes();w.scenario('cadence');c.enter()
 c.wait('window.__sfAudio?.plays>=8')
 warm=c.js('window.__sfAudioSnapshot()');time.sleep(10)
 after=c.js('window.__sfAudioSnapshot()');w.artifact('native-media.json',{'warm':warm,'after':after})
 check('actual production combat continues playing native sound',after['plays']-warm['plays']>=10,after)
 check('warm short effects never unload or reset loaded sources',after['sources']==warm['sources'] and after['loads']==warm['loads'],{'warm':warm,'after':after})
 check('native players stay bounded and reused',after['players']==warm['players'] and after['players']<=14,{'warm':warm['players'],'after':after['players']})
 check('actual native play promises succeeded',after['resolved']>warm['resolved'] and after['rejected']==warm['rejected'],after)
 c.screenshot('combat-audio-reuse')
 c.leave();c.b('click','[data-sound-trigger]');c.b('click','[data-sound-mute]');c.wait('document.querySelector("[data-sound-mute]").getAttribute("aria-pressed")==="true"')
 stopped=c.js('window.__sfAudioSnapshot()');time.sleep(2)
 check('global mute stops future combat playback',c.js('window.__sfAudioSnapshot().plays')==stopped['plays'])
 check('global mute releases native sources',c.js('[...document.querySelectorAll("audio[data-platform-effect-lane]")].every(m=>m.paused&&!m.getAttribute("src"))'))
 check('sources unchanged during acceptance',sources==source_hashes())
 check('no browser errors',not c.b('errors')['errors']);passed=True
except Exception as error:
 import traceback;traceback.print_exc()
 w.artifact('failure.json',{'error':str(error)})
 try:c.screenshot('failure')
 except Exception:pass
finally:
 if w.OPENED:
  try:c.b('close')
  except Exception:pass
 if w.SERVER:(w.OUT/'stop').write_text('stop');w.SERVER.wait(timeout=20)
 w.artifact('result.json',{'passed':passed,'checks':checks,'sources':source_hashes(),'scope':__doc__})
 for path in [w.OUT/'profile',w.OUT/'tmp',w.PROFILE]:
  if path.exists():shutil.rmtree(path)
 sys.exit(0 if passed else 1)
