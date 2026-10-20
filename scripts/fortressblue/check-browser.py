#!/usr/bin/env python3
"""Real-browser acceptance for the rebuilt fortressblue game.

Starts a disposable production-like server, creates an 8-seat room with seven
bots, selects an original tank through the shared UI, moves, fires, verifies
the server-driven shot animation, refresh behavior, local original media and
the 320 px mobile layout. Screenshots and result.json are retained.
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
ASSETS_BASE = os.environ.get("CHECK_ASSETS_BASE", "").strip()
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
env = dict(os.environ, INVITE_CODE=INVITE, ADDR=f"127.0.0.1:{port}", DATA_DIR=str(data),
           ADMIN_USERNAME=ADMIN, ADMIN_PASSWORD=PASSWORD, GOTOOLCHAIN="local")
if ASSETS_BASE:
    env["ASSETS_BASE_URL"] = ASSETS_BASE
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
    result = subprocess.run(["agent-browser", *args, "--json"], env=browser_env, text=True,
                            capture_output=True, timeout=120)
    if result.returncode:
        raise RuntimeError(result.stdout + result.stderr)
    value = json.loads(result.stdout)
    if not value.get("success"):
        raise RuntimeError(str(value))
    return value["data"]


def js(code):
    return b("eval", code)["result"]


def wait_js(code, timeout=25):
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
        const bgc=getComputedStyle(bg).backgroundColor;
        return {fgSel,bgSel,ratio:ratio(rgb(getComputedStyle(fg).color),rgb(bgc)),bg:bgc};
      });
    })()
    """ % json.dumps(selectors)
    return json.loads(js("JSON.stringify(" + code + ")"))


def api_state():
    return json.loads(js("fetch('/api/state').then(r=>r.json()).then(x=>JSON.stringify(x))"))


setup = r"""
(async()=>{
  const post=(url,body)=>fetch(url,{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(body)}).then(async r=>({status:r.status,data:await r.json()}));
  let r=await post('/api/login',{name:%s,password:%s});
  if(r.status!==200)return {error:'login '+r.status};
  r=await post('/api/admin/games/fortressblue',{enabled:true});
  if(r.status!==200)return {error:'publish '+r.status};
  r=await post('/api/rooms',{kind:'fortressblue',name:'坦克原版验收房',capacity:8,options:{map:'stage-16',turnSeconds:30}});
  if(r.status!==201)return {error:'create '+r.status+' '+JSON.stringify(r.data)};
  const id=r.data.id;
  for(let i=0;i<7;i++){
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
  return {ok:true,id};
})()
""" % (json.dumps(ADMIN), json.dumps(PASSWORD))

try:
    build = subprocess.run(["go", "build", "-o", str(binary), "."], cwd=ROOT, env=env,
                           text=True, capture_output=True, timeout=300)
    check("build server", build.returncode == 0, (build.stdout + build.stderr)[-1600:])
    log = open(OUT / "server.log", "w")
    app = subprocess.Popen([str(binary)], cwd=ROOT, env=env, stdout=log, stderr=subprocess.STDOUT)
    wait_http(base + "/healthz")
    check("healthz", True)

    b("open", base)
    result = json.loads(js("(" + setup + ").then(r=>JSON.stringify(r))"))
    check("8-seat room setup", result.get("ok") is True, result)
    b("reload")
    wait_js('[...document.querySelectorAll("button")].some(b=>b.textContent.trim()==="开始游戏")', 15)
    started = js('(()=>{const b=[...document.querySelectorAll("button")].find(x=>x.textContent.trim()==="开始游戏");if(!b)return false;b.click();return true})()')
    check("start from waiting room", started is True)
    wait_js('document.querySelector(".fortressblue-board")?.dataset.fortressbluePhase === "select"', 20)
    check("select phase opens", True)
    check("eight seats in player bar", js('document.querySelectorAll("[data-player-seat]").length') >= 8)
    check("original map selected", js('document.querySelector(".fb-mapname strong")?.textContent === "热血之都"'))
    check("14 original tank cards", js('document.querySelectorAll("[data-fortressblue-tank]").length') == 14)
    wait_js('(()=>{const i=document.querySelector("[data-fortressblue-tank=\\"1\\"] img");return !!i&&i.complete&&i.naturalWidth>0})()', 20)
    check("tank portrait loaded", True)

    b("set", "viewport", "1280", "900")
    b("screenshot", str(OUT / "desktop-select.png"))
    select_ratios = contrast([
        [".fb-draft-head h2", ".fb-draft"],
        [".fb-draft-head p", ".fb-draft"],
        [".fb-tank-card .fb-card-name", ".fb-tank-card"],
        [".fb-tank-card .fb-card-stats", ".fb-tank-card"],
    ])
    check("select contrast", all(item.get("ratio", 0) >= 4.5 for item in select_ratios), select_ratios)

    clicked = js('(()=>{const b=document.querySelector("[data-fortressblue-tank=\\"1\\"]");if(!b)return false;b.click();return true})()')
    check("original tank selected", clicked is True)
    wait_js('document.querySelector(".fortressblue-board").dataset.fortressbluePhase === "playing"', 30)
    check("playing phase", True)
    check("canvas present", js('!!document.querySelector("[data-fortressblue-canvas]")'))
    prefix = json.dumps(ASSETS_BASE)
    check("map foreground loaded", js(f'performance.getEntriesByType("resource").some(e=>e.name.startsWith({prefix}) && e.name.includes("stage-16-fg.webp"))' if ASSETS_BASE else 'performance.getEntriesByType("resource").some(e=>e.name.includes("stage-16-fg.webp"))'))
    check("map background loaded", js(f'performance.getEntriesByType("resource").some(e=>e.name.startsWith({prefix}) && e.name.includes("stage-16-bg.webp"))' if ASSETS_BASE else 'performance.getEntriesByType("resource").some(e=>e.name.includes("stage-16-bg.webp"))'))
    check("explosion sheet loaded", js(f'performance.getEntriesByType("resource").some(e=>e.name.startsWith({prefix}) && e.name.includes("effects/explosion.webp"))' if ASSETS_BASE else 'performance.getEntriesByType("resource").some(e=>e.name.includes("effects/explosion.webp"))'))
    check("tank sprite loaded", js(f'performance.getEntriesByType("resource").some(e=>e.name.startsWith({prefix}) && e.name.includes("/tanks/1-idle.webp"))' if ASSETS_BASE else 'performance.getEntriesByType("resource").some(e=>e.name.includes("/tanks/1-idle.webp"))'))
    if ASSETS_BASE:
        # CDN images are intentionally loaded without crossOrigin so production
        # display does not depend on a per-origin CORS allowlist. Canvas
        # readback is therefore forbidden; the resource checks above and the
        # screenshot prove that original art was requested from the CDN.
        check("CDN original art requested", True)
    else:
        pixels = js(
            '(()=>{const c=document.querySelector("[data-fortressblue-canvas]");const x=c.getContext("2d");const d=x.getImageData(0,0,c.width,c.height).data;'
            'let n=0,colors=new Set();for(let i=0;i<d.length;i+=4*97){if(d[i+3])n++;colors.add(d[i]>>4<<8|d[i+1]>>4<<4|d[i+2]>>4)}return {n,colors:colors.size,w:c.width,h:c.height}})()'
        )
        check("canvas actually renders original art", pixels["n"] > 500 and pixels["colors"] > 12, pixels)
    timer = js('document.querySelector("[data-fortressblue-timer]")?.textContent')
    check("game timer is live", bool(timer) and "0:00" not in timer, timer)
    js('document.querySelector("[data-fortressblue-canvas]").scrollIntoView({block:"center"})')
    time.sleep(0.3)
    b("screenshot", str(OUT / "desktop-board.png"))

    board_ratios = contrast([
        [".fb-mapname strong", ".fb-mapname"],
        [".fb-active", ".fb-active"],
        [".fb-tip", ".fb-controls"],
        [".fb-actions .fb-fire", ".fb-actions .fb-fire"],
    ])
    check("board contrast", all(item.get("ratio", 0) >= 4.5 for item in board_ratios), board_ratios)

    summary = js(
        '(()=>{const p=document.querySelector(".fortressblue-player[data-fortressblue-seat=\\"0\\"]");if(!p)return null;'
        'const r=p.getBoundingClientRect();return {text:p.textContent,w:r.width,h:r.height}})()'
    )
    check("player summary shows readable HP", summary and summary["w"] > 0 and "HP" in summary["text"], summary)

    # Move on the server; the summary's data-x is authoritative state.
    before = js('document.querySelector(".fortressblue-player[data-fortressblue-seat=\\"0\\"]").dataset.x')
    b("click", '[aria-label="向右移动"]')
    wait_js('document.querySelector(".fortressblue-player[data-fortressblue-seat=\\"0\\"]").dataset.x !== %s' % json.dumps(before), 10)
    after = js('document.querySelector(".fortressblue-player[data-fortressblue-seat=\\"0\\"]").dataset.x')
    check("move changes server position", float(after) > float(before), {"before": before, "after": after})

    # Keyboard angle control is available during the human turn.
    angle_before = js('document.querySelector(\'input[aria-label="角度"]\').value')
    b("press", "w")
    b("press", "w")
    angle_after = js('document.querySelector(\'input[aria-label="角度"]\').value')
    check("keyboard angle control", float(angle_after) > float(angle_before), {"before": angle_before, "after": angle_after})

    # Aim and fire; the returned server shot drives the animation.
    js('(()=>{const a=document.querySelector(\'input[aria-label="角度"]\');a.value="55";a.dispatchEvent(new Event("input",{bubbles:true}));const p=document.querySelector(\'input[aria-label="力度"]\');p.value="72";p.dispatchEvent(new Event("input",{bubbles:true}));return true})()')
    b("click", ".fb-fire")
    wait_js('Number(document.querySelector(".fortressblue-board").dataset.fortressblueShot) > 0', 20)
    check("shot recorded", True)
    wait_js('document.querySelector(".fortressblue-board").dataset.fortressblueAnimating === "0"', 25)
    check("shot animation ends", True)
    craters = int(js('document.querySelector(".fortressblue-board").dataset.fortressblueCraters || "0"'))
    check("explosion carved original terrain", craters > 0, craters)
    b("screenshot", str(OUT / "desktop-after-shot.png"))

    # Reload must not replay the old shot.
    b("reload")
    time.sleep(0.9)
    b("wait", ".fortressblue-board")
    wait_js('document.querySelector(".fortressblue-board")?.dataset.fortressblueAnimating === "0"', 10)
    check("refresh does not replay", True)

    b("set", "viewport", "320", "844")
    b("reload")
    b("wait", ".fortressblue-board")
    layout = js(
        '(()=>{const c=document.querySelector("[data-fortressblue-canvas]").getBoundingClientRect();'
        'const f=document.querySelector(".fb-fire")?.getBoundingClientRect();'
        'return {vw:innerWidth,scroll:document.documentElement.scrollWidth,w:c.width,h:c.height,'
        'cards:document.querySelectorAll("[data-fortressblue-tank]").length,fire:!!f,fireH:f&&f.height,'
        'seats:document.querySelectorAll("[data-player-seat]").length}})()'
    )
    check("320 no horizontal overflow", layout["scroll"] <= layout["vw"] + 1, layout)
    check("320 canvas and controls usable", layout["w"] >= 280 and layout["fire"] and layout["fireH"] >= 44, layout)
    check("320 seats present", layout["seats"] >= 8 and (layout["cards"] == 0 or layout["cards"] >= 14), layout)
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
