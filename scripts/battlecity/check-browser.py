#!/usr/bin/env python3
"""Real Battle City browser acceptance.

Failure inventory: unpublished game leaks into the lobby; wrong capacity/map;
nonstandard create/waiting layout; 320px overflow or clipped team resources;
selection/countdown phases missing; keyboard or native touch not reaching the
server; autoplay leaving controls active; refresh replaying old state; result
overlay not closeable/reopenable; bot match rating changed.

Production registry, HTTP/SQLite and host timers are used. No debug actions or
state writes.
"""
import argparse
import json
import os
import subprocess
import time
import traceback
import uuid
from pathlib import Path

parser = argparse.ArgumentParser()
parser.add_argument("--build", action="store_true")
args = parser.parse_args()

root = Path(__file__).resolve().parents[2]
out = root / ".local/battlecity/browser"
out.mkdir(parents=True, exist_ok=True)
env = dict(os.environ)
session = "tank-" + uuid.uuid4().hex[:6]
for key, rel in {
    "AGENT_BROWSER_SOCKET_DIR": ".local/battlecity/browser/sockets",
    "AGENT_BROWSER_PROFILE": ".local/battlecity/browser/" + session,
    "TMPDIR": ".local/tmp",
    "GOTMPDIR": ".local/tmp",
    "GOCACHE": ".local/go-build",
    "GOMODCACHE": ".local/go-mod",
    "npm_config_cache": ".local/npm",
}.items():
    path = root / rel
    path.mkdir(parents=True, exist_ok=True)
    env[key] = str(path)
env.update(
    AGENT_BROWSER_SESSION=session,
    AGENT_BROWSER_ARGS="--mute-audio,--disable-breakpad,--disable-crash-reporter",
    WIRE_BOARD_BATTLECITY_BROWSER="1",
    GOTOOLCHAIN="local",
)
report = {
    "passed": False,
    "checks": [],
    "scope": "production registry/auth/SQLite and host clock; 6-seat 3v3 select/play; keyboard and native touch; autoplay; refresh; result",
    "command": ". scripts/env.sh && python3 -B scripts/battlecity/check-browser.py",
    "screenshots": [],
}
server = None
opened = False


def b(*arguments):
    result = subprocess.run(
        ["agent-browser", *arguments, "--json"],
        cwd=root,
        env=env,
        text=True,
        capture_output=True,
        timeout=90,
    )
    if result.returncode:
        raise RuntimeError(result.stdout + result.stderr)
    value = json.loads(result.stdout)
    assert value["success"], value
    return value["data"]


def js(code):
    return b("eval", code)["result"]


def api(path, body=None, status=200):
    options = (
        ""
        if body is None
        else ',{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify('
        + json.dumps(body)
        + ")}"
    )
    result = js(
        "fetch("
        + json.dumps("/api" + path)
        + options
        + ").then(async r=>({status:r.status,body:await r.json()}))"
    )
    assert result["status"] == status, result
    return result["body"]


def command(kind, **extra):
    payload = json.dumps({"type": kind, "nonce": uuid.uuid4().hex, **extra})
    result = js(
        '(async()=>{const room=(await (await fetch("/api/state")).json()).room;'
        "const body={..."
        + payload
        + ',version:room.version};const response=await fetch("/api/rooms/"+room.id,{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify(body)});return {status:response.status,body:await response.json()}})()'
    )
    assert result["status"] == 200, result
    return result["body"]


def check(name, **evidence):
    report["checks"].append({"name": name, **evidence})
    print("PASS", name, flush=True)


def wait_for(code, timeout=20):
    deadline = time.monotonic() + timeout
    while time.monotonic() < deadline:
        if js(code):
            return True
        time.sleep(0.1)
    raise TimeoutError(code)


def shot(name, selectors=None):
    time.sleep(0.35)
    assert js("document.documentElement.scrollWidth<=innerWidth+2"), (
        "horizontal overflow",
        name,
        js("[document.documentElement.scrollWidth,innerWidth]"),
    )
    errors = b("errors")["errors"]
    assert not errors, (name, errors)
    scope = selectors or ".battlecity-board *, .battlecity-options *, .battlecity-rules *, .battlecity-player-summary *"
    colors = js(
        r'''(()=>{const rgb=s=>(s.match(/[\d.]+/g)||[]).map(Number),lum=c=>c.slice(0,3).map(v=>v/255).map(v=>v<=.04045?v/12.92:((v+.055)/1.055)**2.4).reduce((a,v,i)=>a+v*[.2126,.7152,.0722][i],0);const bg=e=>{let b=[255,255,255],chain=[];for(let n=e;n;n=n.parentElement)chain.unshift(n);for(const n of chain){const c=rgb(getComputedStyle(n).backgroundColor),a=c[3]??1;b=b.map((v,i)=>v*(1-a)+(c[i]??0)*a)}return b};return [...document.querySelectorAll('''
        + json.dumps(scope)
        + r''')].filter(e=>e.getClientRects().length&&[...e.childNodes].some(n=>n.nodeType===3&&n.textContent.trim())).map(e=>{const s=getComputedStyle(e),a=lum(rgb(s.color)),z=lum(bg(e));return {text:e.textContent.trim().slice(0,75),class:String(e.className),foreground:s.color,background:bg(e),ratio:(Math.max(a,z)+.05)/(Math.min(a,z)+.05),minimum:parseFloat(s.fontSize)>=24||(parseFloat(s.fontSize)>=18.66&&+s.fontWeight>=700)?3:4.5}})})()'''
    )
    bad = [item for item in colors if item["ratio"] < item["minimum"] - 0.01]
    assert not bad, (name, bad)
    (out / (name + "-contrast.json")).write_text(json.dumps(colors, ensure_ascii=False, indent=2))
    borders = js(
        r'''(()=>{const rgb=s=>(s.match(/[\d.]+/g)||[]).map(Number),lum=c=>c.slice(0,3).map(v=>v/255).map(v=>v<=.04045?v/12.92:((v+.055)/1.055)**2.4).reduce((a,v,i)=>a+v*[.2126,.7152,.0722][i],0),ratio=(a,b)=>(Math.max(lum(a),lum(b))+.05)/(Math.min(lum(a),lum(b))+.05),bg=e=>{let b=[255,255,255],chain=[];for(let n=e;n;n=n.parentElement)chain.unshift(n);for(const n of chain){const c=rgb(getComputedStyle(n).backgroundColor),a=c[3]??1;b=b.map((v,i)=>v*(1-a)+(c[i]??0)*a)}return b};return [...document.querySelectorAll('.bc-fire:not(:disabled),.bc-dpad,.bc-tank-card:not(:disabled),.bc-option-row select:not(:disabled)')].filter(e=>e.getClientRects().length).map(e=>({text:e.getAttribute('aria-label')||e.textContent.trim(),ratio:Math.max(ratio(bg(e),bg(e.parentElement)),ratio(rgb(getComputedStyle(e).borderTopColor),bg(e.parentElement)))}))})()'''
    )
    assert all(item["ratio"] >= 2.99 for item in borders), (name, borders)
    (out / (name + "-controls-contrast.json")).write_text(json.dumps(borders, ensure_ascii=False, indent=2))
    path = out / (name + ".png")
    b("screenshot", str(path), "--full")
    report["screenshots"].append(str(path.relative_to(root)))
    check(name, contrast=len(colors), controls=len(borders))


def tap_at(selector, fx=0.5, fy=0.5):
    b("scrollintoview", selector)
    rect = js(
        "(()=>{const r=document.querySelector("
        + json.dumps(selector)
        + ").getBoundingClientRect();return {x:r.left+r.width*"
        + str(fx)
        + ",y:r.top+r.height*"
        + str(fy)
        + "}})()"
    )
    endpoint = b("get", "cdp-url")["cdpUrl"]
    url = b("get", "url")["url"]
    result = subprocess.run(
        ["node", "scripts/browser_touch.mjs", endpoint, url, "tap", str(rect["x"]), str(rect["y"]), str(rect["x"]), str(rect["y"])],
        cwd=root,
        env=env,
        text=True,
        capture_output=True,
        timeout=40,
    )
    if result.returncode:
        raise RuntimeError(result.stdout + result.stderr)


def seq_of(seat=0):
    selector = f'.bc-player[data-battlecity-seat="{seat}"]'
    return int(
        js(
            "Number(document.querySelector("
            + json.dumps(selector)
            + ")?.dataset.battlecitySeq||0)"
        )
    )


def leave_room():
    command("leave")
    wait_for('!!document.querySelector(".room-list")', 10)


def finish_room():
    try:
        command("close")
    except Exception:
        try:
            command("leave")
        except Exception:
            pass
    try:
        b("find", "role", "button", "click", "--name", "离开房间", "--exact")
    except Exception:
        try:
            b("find", "role", "link", "click", "--name", "围桌首页", "--exact")
        except Exception:
            pass
    wait_for('!!document.querySelector(".room-list")', 10)


def layout_room(capacity, label):
    b("click", ".lobby-create")
    b("wait", "[role=dialog]")
    b("wait", "[data-create-step=choose]:not([hidden])")
    b("fill", "input[aria-label=搜索可选游戏]", "坦克大战")
    b("click", ".choose-games button")
    b("wait", "[data-create-step=setup]:not([hidden])")
    b("select", "select[aria-label=座位上限]", str(capacity))
    b("select", "select[aria-label=对战地图]", "classic-01")
    b("fill", "input[name=name]", f"布局{capacity}")
    b("click", "[role=dialog] button[type=submit]")
    b("wait", ".waiting-actions")
    for _ in range(capacity - 1):
        command("add_bot")
    command("ready")
    command("start")
    b("wait", ".battlecity-board")
    wait_for('document.querySelector("[data-battlecity-phase]")?.dataset.battlecityPhase==="select"', 10)
    b("click", '.bc-tank-card[data-battlecity-tank="balanced"]')
    wait_for('document.querySelector("[data-battlecity-phase]")?.dataset.battlecityPhase==="playing"', 15)
    assert js('document.querySelector("[data-battlecity-team-size]")?.dataset.battlecityTeamSize') == str(capacity // 2)
    assert js('document.querySelectorAll(".bc-player").length') == capacity
    assert js('document.querySelectorAll(".bc-team").length') == 2
    assert js('document.querySelectorAll(".bc-team-top .bc-player").length') == capacity // 2
    assert js('document.querySelectorAll(".bc-team-bottom .bc-player").length') == capacity // 2
    b("set", "viewport", "320", "740")
    shot(label + "-320")
    finish_room()


try:
    for name in ["url", "stop"]:
        (out / name).unlink(missing_ok=True)
    if args.build:
        with (out / "build.log").open("w") as log:
            subprocess.run(
                ["npm", "--prefix", "web", "run", "build"],
                cwd=root,
                env=env,
                stdout=log,
                stderr=subprocess.STDOUT,
                check=True,
            )
    assert (root / "web/dist/index.html").exists(), "build frontend first"
    log = (out / "server.log").open("w")
    server = subprocess.Popen(
        ["go", "test", "-count=1", "-timeout=25m", "-run", "^TestBattlecityBrowserFixture$", "./games/battlecity/tests"],
        cwd=root,
        env=env,
        stdout=log,
        stderr=subprocess.STDOUT,
    )
    deadline = time.monotonic() + 120
    while not (out / "url").exists():
        if server.poll() is not None or time.monotonic() > deadline:
            raise RuntimeError("fixture did not start; inspect server.log")
        time.sleep(0.1)
    base = (out / "url").read_text()
    b("open", base)
    opened = True
    api("/login", {"name": "坦克验收管理员", "password": "battlecity-browser-test-only"})
    assert "battlecity" not in api("/state")["availableGames"], "new game was automatically listed"
    api("/rooms", {"kind": "battlecity", "name": "未上架", "capacity": 2}, 403)
    api("/admin/games/battlecity", {"enabled": True})
    b("reload")
    b("wait", ".room-list")
    check("default-unlisted-admin-only")

    b("set", "viewport", "1280", "900")
    b("click", ".lobby-create")
    b("wait", "[role=dialog]")
    b("wait", "[data-create-step=choose]:not([hidden])")
    b("fill", "input[aria-label=搜索可选游戏]", "坦克大战")
    b("click", ".choose-games button")
    b("wait", "[data-create-step=setup]:not([hidden])")
    assert js('[...document.querySelectorAll("select[aria-label=座位上限] option")].map(e=>e.value)') == ["2", "4", "6"]
    assert js('[...document.querySelectorAll("select[aria-label=对战地图] option")].length') == 13
    b("select", "select[aria-label=座位上限]", "6")
    b("select", "select[aria-label=对战地图]", "classic-01")
    assert "3v3" in js('document.querySelector(".battlecity-options").textContent')
    for width in [1280, 390, 320]:
        b("set", "viewport", str(width), "900")
        shot("create-" + str(width))
    b("set", "viewport", "1280", "900")
    b("fill", "input[name=name]", "坦克大战真实验收")
    b("click", "[role=dialog] button[type=submit]")
    b("wait", ".waiting-actions")
    for _ in range(5):
        command("add_bot")
    wait_for('document.querySelectorAll(".waiting-seat.filled").length===6', 5)
    for width in [1280, 320]:
        b("set", "viewport", str(width), "900")
        shot("waiting-" + str(width))
    b("set", "viewport", "1280", "900")
    command("ready")
    command("start")
    b("wait", ".battlecity-board")
    wait_for('document.querySelector("[data-battlecity-phase]")?.dataset.battlecityPhase==="select"', 10)
    asset_expected = bool(os.environ.get("WIRE_BOARD_BATTLECITY_ASSETS")) or (
        root / "games/battlecity/assets/public/battlecity/v1/atlas.webp"
    ).exists() or (root / "web/dist/battlecity/v1/atlas.webp").exists()
    if asset_expected:
        wait_for('document.querySelector("[data-battlecity-atlas]")?.dataset.battlecityAtlas==="1"', 10)
        check("atlas-loaded")
    else:
        check("atlas-fallback")
    asset_base = os.environ.get("WIRE_BOARD_BATTLECITY_ASSETS", "").rstrip("/")
    if asset_base:
        audio_url = asset_base + "/battlecity/v1/audio/bgm-battle.mp3"
        audio = js(
            "new Promise(resolve=>{const a=new Audio("
            + json.dumps(audio_url)
            + ");a.preload='metadata';a.onloadedmetadata=()=>resolve({ok:true,duration:a.duration});a.onerror=()=>resolve({ok:false});setTimeout(()=>resolve({ok:false}),8000)})"
        )
        assert audio.get("ok") and audio.get("duration", 0) > 5, audio
        check("cdn-audio-metadata", duration=audio["duration"])
    assert js('document.querySelectorAll(".player-panel").length') == 6
    assert js('document.querySelectorAll(".battlecity-player-summary").length') == 6
    assert js('document.querySelectorAll(".bc-player").length') == 6
    assert js('document.querySelectorAll(".bc-team").length') == 2
    for width in [1280, 320]:
        b("set", "viewport", str(width), "900")
        shot("select-" + str(width))
    b("set", "viewport", "1280", "900")
    b("click", '.bc-tank-card[data-battlecity-tank="balanced"]')
    wait_for('document.querySelector("[data-battlecity-phase]")?.dataset.battlecityPhase==="playing"', 15)
    check("select-countdown-playing")
    assert js('document.querySelector("[data-battlecity-team-size]")?.dataset.battlecityTeamSize') == "3"

    b("set", "viewport", "1280", "900")
    js("document.activeElement?.blur()")
    before_seq = seq_of()
    b("press", "ArrowDown")
    wait_for(
        'Number(document.querySelector(".bc-player[data-battlecity-seat=\'0\']")?.dataset.battlecitySeq||0)>'
        + str(before_seq),
        5,
    )
    check("keyboard-move", before=before_seq)

    before_fire = seq_of()
    b("press", "Space")
    wait_for(
        'Number(document.querySelector(".bc-player[data-battlecity-seat=\'0\']")?.dataset.battlecitySeq||0)>'
        + str(before_fire),
        5,
    )
    check("keyboard-fire", before=before_fire)

    b("set", "viewport", "320", "740")
    time.sleep(0.5)
    before_touch = seq_of()
    tap_at("[data-battlecity-dpad]", 0.82, 0.5)
    wait_for(
        'Number(document.querySelector(".bc-player[data-battlecity-seat=\'0\']")?.dataset.battlecitySeq||0)>'
        + str(before_touch),
        5,
    )
    check("native-touch-move", before=before_touch)
    before_touch_fire = seq_of()
    tap_at("[data-battlecity-fire]")
    wait_for(
        'Number(document.querySelector(".bc-player[data-battlecity-seat=\'0\']")?.dataset.battlecitySeq||0)>'
        + str(before_touch_fire),
        5,
    )
    check("native-touch-fire", before=before_touch_fire)

    for width in [1280, 390, 320]:
        b("set", "viewport", str(width), "740")
        shot("game-" + str(width))
        cards = js(
            '[...document.querySelectorAll(".bc-player")].map(e=>{const r=e.getBoundingClientRect();return {left:r.left,right:r.right,width:r.width}})'
        )
        assert len(cards) == 6, cards
        assert all(item["left"] >= 0 and item["right"] <= width + 1 and item["width"] > 40 for item in cards), (width, cards)
        canvas = js(
            '(()=>{const r=document.querySelector("[data-battlecity-canvas]").getBoundingClientRect();return {width:r.width,height:r.height}})()'
        )
        assert abs(canvas["height"] - canvas["width"]) < 2, canvas
        assert canvas["width"] / 13 >= 22, (width, canvas)
        check("resources-" + str(width), canvas=canvas)

    b("set", "viewport", "1280", "900")
    b("wait", "--fn", '!document.querySelector(".autoplay-control")?.disabled')
    b("find", "role", "button", "click", "--name", "开启托管", "--exact")
    wait_for(
        'document.querySelector("[data-battlecity-fire]")?.disabled===true',
        5,
    )
    check("autoplay-disables-controls")
    if js('document.querySelector("[data-battlecity-phase]")?.dataset.battlecityPhase!=="finished"'):
        b("find", "role", "button", "click", "--name", "取消托管", "--exact")
        wait_for(
            'document.querySelector("[data-battlecity-fire]")?.disabled===false',
            5,
        )
        check("autoplay-returned-control")
    else:
        check("autoplay-match-finished")

    epoch = js('document.querySelector("[data-battlecity-epoch]")?.getAttribute("data-battlecity-epoch")')
    seq = seq_of()
    b("reload")
    b("wait", ".battlecity-board")
    wait_for(
        '["playing","finished"].includes(document.querySelector("[data-battlecity-phase]")?.dataset.battlecityPhase)',
        15,
    )
    assert js('document.querySelector("[data-battlecity-epoch]")?.getAttribute("data-battlecity-epoch")') == epoch, "refresh changed epoch"
    assert seq_of() >= seq, "refresh lost input sequence"
    check("refresh-without-replay", epoch=epoch, seq=seq)

    b("set", "viewport", "1280", "900")
    if js('document.querySelector("[data-battlecity-phase]")?.dataset.battlecityPhase!=="finished"'):
        b("wait", "--fn", '!document.querySelector(".autoplay-control")?.disabled')
        b("find", "role", "button", "click", "--name", "开启托管", "--exact")
    deadline = time.monotonic() + 300
    while time.monotonic() < deadline:
        if js('!!document.querySelector(".match-result-modal")'):
            break
        if js('document.querySelector("[data-battlecity-phase]")?.dataset.battlecityPhase==="finished"'):
            if js('!!document.querySelector(".result-reopen")'):
                b("click", ".result-reopen")
        time.sleep(1)
    assert js('!!document.querySelector(".match-result-modal")'), "bot match did not finish"
    result = api("/state")["room"]["result"]
    assert result["rated"] is False, result
    text = js('document.querySelector(".match-result-modal").textContent')
    assert "不计积分" in text, text
    shot("result-1280")
    b("set", "viewport", "320", "740")
    shot("result-320")
    b("click", ".modal-head button[aria-label=关闭]")
    b("wait", ".result-reopen")
    b("click", ".result-reopen")
    b("wait", ".match-result-modal")
    check("result-closable-reopenable")
    b("find", "role", "button", "click", "--name", "再来一局", "--exact")
    b("wait", ".waiting-layout")
    check("rematch-returns-to-waiting")
    leave_room()
    layout_room(2, "layout-1v1")
    layout_room(4, "layout-2v2")
    report["passed"] = True
except Exception as error:
    report["error"] = repr(error)
    report["trace"] = traceback.format_exc()
    if opened:
        try:
            report["snapshot"] = b("snapshot", "-i")
            b("screenshot", str(out / "failure.png"), "--full")
        except Exception:
            pass
    raise
finally:
    try:
        if opened:
            b("close")
    except Exception:
        pass
    if server:
        try:
            (out / "stop").write_text("stop")
        except Exception:
            pass
        server.terminate()
        try:
            server.wait(timeout=15)
        except Exception:
            server.kill()
    (out / "result.json").write_text(json.dumps(report, ensure_ascii=False, indent=2) + "\n")
