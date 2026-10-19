#!/usr/bin/env python3
"""Real-browser acceptance for fortressblue (疯狂坦克).

Starts a disposable production-like server, creates a 10-seat room with nine
bots, verifies the shared select/board/control layout at desktop and 320px,
fires a shot, and proves refresh does not replay the old animation.
"""
from __future__ import annotations

import json
import os
import shutil
import socket
import subprocess
import time
import uuid
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
RUN = uuid.uuid4().hex[:8]
OUT = ROOT / ".local/fortressblue/browser" / RUN
OUT.mkdir(parents=True)
INVITE = "fortress-accept-code"
ADMIN = "FortressAdmin"
PASSWORD = "fortress-admin-password-123"
report = {"passed": False, "checks": []}


def check(name, ok, detail=None):
    report["checks"].append({"name": name, "ok": bool(ok), "detail": detail})
    if not ok:
        raise AssertionError(f"{name}: {detail}")


def free_port():
    with socket.socket() as sock:
        sock.bind(("127.0.0.1", 0))
        return sock.getsockname()[1]


def wait_http(url, timeout=45):
    import urllib.request

    end = time.time() + timeout
    while time.time() < end:
        try:
            with urllib.request.urlopen(url, timeout=1) as response:
                if response.status == 200:
                    return
        except Exception:
            time.sleep(0.25)
    raise RuntimeError(f"server not ready: {url}")


port = free_port()
base = f"http://127.0.0.1:{port}"
binary = OUT / "wire-board"
data = OUT / "data"
env = dict(
    os.environ,
    INVITE_CODE=INVITE,
    ADDR=f"127.0.0.1:{port}",
    DATA_DIR=str(data),
    ADMIN_USERNAME=ADMIN,
    ADMIN_PASSWORD=PASSWORD,
    GOTOOLCHAIN="local",
)
browser_env = dict(
    os.environ,
    AGENT_BROWSER_SOCKET_DIR="/tmp/wbfb-ab3",
    AGENT_BROWSER_SESSION="fb" + RUN,
    AGENT_BROWSER_PROFILE=str(OUT / "profile"),
    AGENT_BROWSER_ARGS="--mute-audio,--disable-breakpad,--disable-crash-reporter",
    TMPDIR="/tmp",
)
app = None


def b(*args):
    result = subprocess.run(
        ["agent-browser", *args, "--json"],
        env=browser_env, text=True, capture_output=True, timeout=90,
    )
    if result.returncode:
        raise RuntimeError(result.stdout + result.stderr)
    value = json.loads(result.stdout)
    if not value.get("success"):
        raise RuntimeError(str(value))
    return value["data"]


def js(code):
    return b("eval", code)["result"]


def wait_js(code, timeout=20):
    end = time.time() + timeout
    last = None
    while time.time() < end:
        try:
            if js(code):
                return
        except Exception as error:
            last = error
        time.sleep(0.25)
    raise AssertionError(f"timed out waiting for {code}: {last}")


def contrast(selectors):
    code = r"""
    (()=>{
      const rgb=s=>{const m=s.match(/[\d.]+/g);return m?m.slice(0,3).map(Number):[0,0,0]};
      const lum=c=>{const v=c.map(x=>{x/=255;return x<=.03928?x/12.92:((x+.055)/1.055)**2.4});return .2126*v[0]+.7152*v[1]+.0722*v[2]};
      const ratio=(a,b)=>{const x=lum(a),y=lum(b);return (Math.max(x,y)+.05)/(Math.min(x,y)+.05)};
      return %s.map(([fgSel,bgSel])=>{
        const fg=document.querySelector(fgSel),bg=document.querySelector(bgSel);
        if(!fg||!bg)return {fgSel,bgSel,ratio:0,missing:true};
        return {fgSel,bgSel,ratio:ratio(rgb(getComputedStyle(fg).color),rgb(getComputedStyle(bg).backgroundColor))};
      });
    })()
    """ % json.dumps(selectors)
    return json.loads(js("JSON.stringify(" + code + ")"))


setup = r"""
(async()=>{
  const post=(url,body)=>fetch(url,{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(body)}).then(async r=>({status:r.status,data:await r.json()}));
  let r=await post('/api/login',{name:%s,password:%s});
  if(r.status!==200)return {error:'login '+r.status};
  r=await post('/api/admin/games/fortressblue',{enabled:true});
  if(r.status!==200)return {error:'publish '+r.status};
  r=await post('/api/rooms',{kind:'fortressblue',name:'坦克验收房',capacity:10,options:{map:'hills',turnSeconds:20}});
  if(r.status!==201)return {error:'create '+r.status+' '+JSON.stringify(r.data)};
  const id=r.data.id;
  for(let i=0;i<9;i++){
    const state=await fetch('/api/state').then(x=>x.json());
    const room=state.room;
    if(!room)return {error:'missing room'};
    r=await post('/api/rooms/'+id,{type:'add_bot',version:room.version,nonce:'fb-'+i+'-'+Date.now()});
    if(r.status!==200)return {error:'add_bot '+r.status+' '+JSON.stringify(r.data)};
  }
  let state=await fetch('/api/state').then(x=>x.json());
  let room=state.room;
  r=await post('/api/rooms/'+id,{type:'ready',version:room.version,nonce:'fb-ready-'+Date.now()});
  if(r.status!==200)return {error:'ready '+r.status+' '+JSON.stringify(r.data)};
  state=await fetch('/api/state').then(x=>x.json());
  room=state.room;
  r=await post('/api/rooms/'+id,{type:'start',version:room.version,nonce:'fb-start-'+Date.now()});
  if(r.status!==200)return {error:'start '+r.status+' '+JSON.stringify(r.data)};
  return {ok:true,id};
})()
""" % (json.dumps(ADMIN), json.dumps(PASSWORD))

try:
    build = subprocess.run(
        ["go", "build", "-o", str(binary), "."],
        cwd=ROOT, env=env, text=True, capture_output=True, timeout=300,
    )
    check("build server", build.returncode == 0, (build.stdout + build.stderr)[-1200:])
    log = open(OUT / "server.log", "w")
    app = subprocess.Popen([str(binary)], cwd=ROOT, env=env, stdout=log, stderr=subprocess.STDOUT)
    wait_http(base + "/healthz")
    check("healthz", True)

    b("open", base)
    result = js("(" + setup + ").then(r=>JSON.stringify(r))")
    setup_result = json.loads(result)
    check("10-seat room setup", setup_result.get("ok") is True, setup_result)
    b("reload")
    b("wait", ".fortressblue-board")
    check("board opens", js('document.querySelector(".fortressblue-board").dataset.fortressbluePhase === "select"'))
    check("ten seats in player bar", js('document.querySelectorAll("[data-player-seat]").length') >= 10)
    b("set", "viewport", "1280", "900")
    b("screenshot", str(OUT / "desktop-select.png"))
    select_ratios = contrast([
        [".fb-select-head h3", ".fb-select-overlay"],
        [".fb-class-card", ".fb-class-card"],
        [".fb-class-card small", ".fb-class-card"],
    ])
    check("select contrast", all(item.get("ratio", 0) >= 4.5 for item in select_ratios), select_ratios)

    # Pick a class through the shared game UI.
    clicked = js('(()=>{const b=[...document.querySelectorAll(".fb-class-card")].find(x=>x.textContent.includes("重装"));if(!b)return false;b.click();return true})()')
    check("class card click", clicked is True)
    wait_js('document.querySelector(".fortressblue-board").dataset.fortressbluePhase === "playing"', 30)
    check("playing phase", True)
    check("canvas present", js('!!document.querySelector("[data-fortressblue-canvas]")'))
    timer = js('document.querySelector("[data-fortressblue-timer]").textContent')
    check("game timer is live", "0:00" not in timer, timer)
    js('document.querySelector("[data-fortressblue-canvas]").scrollIntoView({block:"center"})')
    time.sleep(0.3)
    b("screenshot", str(OUT / "desktop-board.png"))

    ratios = contrast([
        [".fb-head h2", ".fortressblue-board"],
        [".fb-team h3", ".fb-team"],
        [".fb-tank-row", ".fb-team"],
        [".fb-turn-note", ".fortressblue-board"],
        [".fb-fire", ".fb-fire"],
    ])
    check("board contrast", all(item.get("ratio", 0) >= 4.5 for item in ratios), ratios)

    # Move to the right and verify the server-authoritative position changes.
    before = js('document.querySelector(\'.fb-tank-row[data-fortressblue-seat="0"]\').dataset.x')
    b("click", '[aria-label="向左移动"]')
    wait_js('document.querySelector(\'.fb-tank-row[data-fortressblue-seat="0"]\').dataset.x !== %s' % json.dumps(before), 10)
    after = js('document.querySelector(\'.fb-tank-row[data-fortressblue-seat="0"]\').dataset.x')
    check("move changes position", float(after) < float(before), {"before": before, "after": after})

    # Set a legal aim, fire, and wait for the returned shot animation to end.
    js('(()=>{const a=document.querySelector(\'input[aria-label="射角"]\');a.value="55";a.dispatchEvent(new Event("input",{bubbles:true}));const p=document.querySelector(\'input[aria-label="力度"]\');p.value="72";p.dispatchEvent(new Event("input",{bubbles:true}));return true})()')
    b("click", ".fb-fire")
    wait_js('Number(document.querySelector(".fortressblue-board").dataset.fortressblueShot) > 0', 15)
    check("shot recorded", True)
    wait_js('document.querySelector(".fortressblue-board").dataset.fortressblueAnimating === "0"', 20)
    check("shot animation ends", True)
    b("screenshot", str(OUT / "desktop-after-shot.png"))

    # Reload must not replay the old shot.
    b("reload")
    time.sleep(0.8)
    b("wait", ".fortressblue-board")
    wait_js('document.querySelector(".fortressblue-board")?.dataset.fortressblueAnimating === "0"', 10)
    check("refresh does not replay", True)

    b("set", "viewport", "320", "844")
    b("reload")
    b("wait", ".fortressblue-board")
    layout = js(
        '(()=>{const c=document.querySelector("[data-fortressblue-canvas]").getBoundingClientRect();'
        'return {vw:innerWidth,scroll:document.documentElement.scrollWidth,w:c.width,h:c.height,'
        'rows:document.querySelectorAll(".fb-tank-row").length,fire:!!document.querySelector(".fb-fire")}})()'
    )
    check("320 no horizontal overflow", layout["scroll"] <= layout["vw"] + 1, layout)
    check("320 canvas usable", layout["w"] >= 300 and layout["rows"] == 10, layout)
    check("320 controls visible", layout["fire"], layout)
    js('document.querySelector("[data-fortressblue-canvas]").scrollIntoView({block:"center"})')
    time.sleep(0.3)
    b("screenshot", str(OUT / "mobile-320.png"))

    report["passed"] = True
except Exception as error:
    report["error"] = repr(error)
    print("ERROR:", repr(error), flush=True)
    raise
finally:
    try:
        b("close")
    except Exception:
        pass
    if app is not None:
        app.terminate()
        try:
            app.wait(timeout=10)
        except subprocess.TimeoutExpired:
            app.kill()
    shutil.rmtree(OUT / "profile", ignore_errors=True)
    report["serverLog"] = str(OUT / "server.log")
    (OUT / "result.json").write_text(json.dumps(report, ensure_ascii=False, indent=2) + "\n")
    print(OUT / "result.json")
    if not report["passed"]:
        raise SystemExit(1)
