#!/usr/bin/env python3
"""Real tetris browser acceptance.

Failure inventory: published without admin approval; nonstandard create/waiting
layout; 320px overflow or clipped opponent resources; missing zero values;
keyboard/touch input not reaching the server; autoplay not returning control;
refresh replaying old animation; solo practice receiving rating points; result
overlay not closeable/reopenable. Production registry, HTTP, SQLite and host
timers are used. No debug actions or state writes.
"""
import argparse
import json
import os
import select
import subprocess
import time
import traceback
import urllib.error
import urllib.request
import uuid
from pathlib import Path

parser = argparse.ArgumentParser()
parser.add_argument("--build", action="store_true")
args = parser.parse_args()

root = Path(__file__).resolve().parents[2]
out = root / ".local/tetris/browser"
out.mkdir(parents=True, exist_ok=True)
env = dict(os.environ)
session = "tt-" + uuid.uuid4().hex[:6]
for key, rel in {
    "AGENT_BROWSER_SOCKET_DIR": ".local/tetris/browser/sockets",
    "AGENT_BROWSER_PROFILE": ".local/tetris/browser/" + session,
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
    WIRE_BOARD_TETRIS_BROWSER="1",
    GOTOOLCHAIN="local",
)
report = {
    "passed": False,
    "checks": [],
    "scope": "production registry/auth/SQLite and host clock; native keyboard and touch; natural bot sprint",
    "command": ". scripts/env.sh && python3 -B scripts/tetris/check-browser.py",
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
        timeout=60,
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

def state():
    return api("/state")["room"]

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

def shot(name):
    time.sleep(0.35)
    assert js("document.documentElement.scrollWidth<=innerWidth+2"), (
        "horizontal overflow",
        name,
        js("[document.documentElement.scrollWidth,innerWidth]"),
    )
    errors = b("errors")["errors"]
    assert not errors, (name, errors)
    colors = js(
        r'''(()=>{const rgb=s=>(s.match(/[\d.]+/g)||[]).map(Number),lum=c=>c.slice(0,3).map(v=>v/255).map(v=>v<=.04045?v/12.92:((v+.055)/1.055)**2.4).reduce((a,v,i)=>a+v*[.2126,.7152,.0722][i],0);const bg=e=>{let b=[255,255,255],chain=[];for(let n=e;n;n=n.parentElement)chain.unshift(n);for(const n of chain){const c=rgb(getComputedStyle(n).backgroundColor),a=c[3]??1;b=b.map((v,i)=>v*(1-a)+(c[i]??0)*a)}return b};return [...document.querySelectorAll('.tetris-board *, .tetris-options *, .tetris-player-summary *')].filter(e=>e.getClientRects().length&&[...e.childNodes].some(n=>n.nodeType===3&&n.textContent.trim())).map(e=>{const s=getComputedStyle(e),a=lum(rgb(s.color)),z=lum(bg(e));return {text:e.textContent.trim().slice(0,75),class:String(e.className),foreground:s.color,background:bg(e),ratio:(Math.max(a,z)+.05)/(Math.min(a,z)+.05),minimum:parseFloat(s.fontSize)>=24||(parseFloat(s.fontSize)>=18.66&&+s.fontWeight>=700)?3:4.5}})})()'''
    )
    bad = [item for item in colors if item["ratio"] < item["minimum"] - 0.01]
    assert not bad, (name, bad)
    (out / (name + "-contrast.json")).write_text(
        json.dumps(colors, ensure_ascii=False, indent=2)
    )
    borders = js(
        r'''(()=>{const rgb=s=>(s.match(/[\d.]+/g)||[]).map(Number),lum=c=>c.slice(0,3).map(v=>v/255).map(v=>v<=.04045?v/12.92:((v+.055)/1.055)**2.4).reduce((a,v,i)=>a+v*[.2126,.7152,.0722][i],0),ratio=(a,b)=>(Math.max(lum(a),lum(b))+.05)/(Math.min(lum(a),lum(b))+.05),bg=e=>{let b=[255,255,255],chain=[];for(let n=e;n;n=n.parentElement)chain.unshift(n);for(const n of chain){const c=rgb(getComputedStyle(n).backgroundColor),a=c[3]??1;b=b.map((v,i)=>v*(1-a)+(c[i]??0)*a)}return b};return [...document.querySelectorAll('.tetris-controls button:not(:disabled),.tetris-status button,.tetris-options select:not(:disabled)')].filter(e=>e.getClientRects().length).map(e=>({text:e.getAttribute('aria-label')||e.textContent.trim(),ratio:Math.max(ratio(bg(e),bg(e.parentElement)),ratio(rgb(getComputedStyle(e).borderTopColor),bg(e.parentElement)))}))})()'''
    )
    assert all(item["ratio"] >= 2.99 for item in borders), (name, borders)
    (out / (name + "-controls-contrast.json")).write_text(
        json.dumps(borders, ensure_ascii=False, indent=2)
    )
    path = out / (name + ".png")
    b("screenshot", str(path), "--full")
    report["screenshots"].append(str(path.relative_to(root)))
    check(name, contrast=len(colors), controls=len(borders))

def tap(selector):
    b("scrollintoview", selector)
    rect = js(
        "(()=>{const r=document.querySelector("
        + json.dumps(selector)
        + ").getBoundingClientRect();return {x:(r.left+r.right)/2,y:(r.top+r.bottom)/2}})()"
    )
    endpoint = b("get", "cdp-url")["cdpUrl"]
    url = b("get", "url")["url"]
    result = subprocess.run(
        [
            "node",
            "scripts/browser_touch.mjs",
            endpoint,
            url,
            "tap",
            str(rect["x"]),
            str(rect["y"]),
            str(rect["x"]),
            str(rect["y"]),
        ],
        cwd=root,
        env=env,
        text=True,
        capture_output=True,
        timeout=40,
    )
    if result.returncode:
        raise RuntimeError(result.stdout + result.stderr)

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
        [
            "go",
            "test",
            "-count=1",
            "-timeout=25m",
            "-run",
            "^TestTetrisBrowserFixture$",
            "./games/tetris/tests",
        ],
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
    api("/login", {"name": "方块验收管理员", "password": "tetris-browser-test-only"})
    assert "tetris" not in api("/state")["availableGames"], "new game was automatically listed"
    api("/rooms", {"kind": "tetris", "name": "未上架", "capacity": 1}, 403)
    api("/admin/games/tetris", {"enabled": True})
    b("reload")
    b("wait", ".room-list")
    check("default-unlisted-admin-only")

    b("set", "viewport", "1280", "900")
    b("click", ".lobby-create")
    b("wait", "[role=dialog]")
    b("wait", "[data-create-step=choose]:not([hidden])")
    b("fill", "input[aria-label=搜索可选游戏]", "俄罗斯方块")
    b("click", ".choose-games button")
    b("wait", "[data-create-step=setup]:not([hidden])")
    assert js(
        '[...document.querySelectorAll("select[aria-label=座位上限] option")].map(e=>e.value)'
    ) == ["1", "2", "3", "4"]
    assert js(
        '[...document.querySelectorAll("select[aria-label=对战模式] option")].map(e=>e.value)'
    ) == ["survival", "sprint"]
    assert "单人练习" in js(
        'document.querySelector(".tetris-options").textContent'
    )
    for width in [1280, 390, 320]:
        b("set", "viewport", str(width), "900")
        shot("create-" + str(width))
    b("set", "viewport", "1280", "940")
    b("select", "select[aria-label=座位上限]", "4")
    b("fill", "input[name=name]", "俄罗斯方块真实验收")
    b("select", "select[aria-label=对战模式]", "survival")
    b("click", "[role=dialog] button[type=submit]")
    b("wait", ".waiting-actions")
    for _ in range(3):
        command("add_bot")
    assert len(state()["seats"]) == 4
    for width in [1280, 320]:
        b("set", "viewport", str(width), "900")
        shot("waiting-four-" + str(width))
    command("ready")
    command("start")
    b("wait", ".tetris-board")
    wait_for('document.querySelector(".tetris-board")?.dataset.tetrisPhase==="playing"', 15)
    b("set", "viewport", "1280", "900")
    js("document.activeElement?.blur()")
    before = js(
        'Number(document.querySelector(".tetris-board").dataset.tetrisActiveX)'
    )
    b("press", "ArrowRight")
    wait_for(
        'Number(document.querySelector(".tetris-board").dataset.tetrisActiveX)!='
        + str(before),
        3,
    )
    check("keyboard-move", before=before)
    rotated = False
    for _ in range(7):
        kind = js(
            'Number(document.querySelector(".tetris-board").dataset.tetrisActiveKind)'
        )
        rotation = js(
            'Number(document.querySelector(".tetris-board").dataset.tetrisActiveRotation)'
        )
        piece = js(
            'Number(document.querySelector(".tetris-board").dataset.tetrisPieceId)'
        )
        if kind == 4:
            b("press", "Space")
            wait_for(
                'Number(document.querySelector(".tetris-board").dataset.tetrisPieceId)!='
                + str(piece),
                4,
            )
            continue
        b("press", "ArrowUp")
        try:
            wait_for(
                'Number(document.querySelector(".tetris-board").dataset.tetrisActiveRotation)!='
                + str(rotation),
                3,
            )
            rotated = True
            break
        except TimeoutError:
            b("press", "Space")
            wait_for(
                'Number(document.querySelector(".tetris-board").dataset.tetrisPieceId)!='
                + str(piece),
                4,
            )
    assert rotated, "could not rotate a non-O piece"
    check("keyboard-rotate")
    piece = js(
        'Number(document.querySelector(".tetris-board").dataset.tetrisPieceId)'
    )
    score = js('Number(document.querySelector(".tetris-board").dataset.tetrisScore)')
    b("press", "Space")
    wait_for(
        'Number(document.querySelector(".tetris-board").dataset.tetrisPieceId)!='
        + str(piece),
        4,
    )
    wait_for(
        'Number(document.querySelector(".tetris-board").dataset.tetrisScore)>'
        + str(score),
        4,
    )
    check("keyboard-hard-drop", piece=piece, score=score)

    b("set", "viewport", "320", "740")
    x_before = js(
        'Number(document.querySelector(".tetris-board").dataset.tetrisActiveX)'
    )
    tap('button[aria-label="向右移动"]')
    wait_for(
        'Number(document.querySelector(".tetris-board").dataset.tetrisActiveX)!='
        + str(x_before),
        4,
    )
    check("native-touch-move", before=x_before)

    b("wait", "--fn", '!document.querySelector(".autoplay-control")?.disabled')
    b("find", "role", "button", "click", "--name", "开启托管", "--exact")
    wait_for(
        'document.querySelector(".tetris-auto-note")?.textContent.includes("托管")',
        5,
    )
    assert js(
        '[...document.querySelectorAll(".tetris-controls button")].every(b=>b.disabled)'
    ), "autoplay controls remained enabled"
    check("autoplay-disables-controls")
    b("find", "role", "button", "click", "--name", "取消托管", "--exact")
    wait_for(
        '!document.querySelector(".tetris-controls button")?.disabled',
        5,
    )
    check("autoplay-returned-control")

    b("set", "viewport", "1280", "900")
    shot("game-1280")
    for width in [390, 320]:
        b("set", "viewport", str(width), "740")
        shot("game-" + str(width))
        summaries = js(
            '[...document.querySelectorAll(".tetris-player-summary")].map(e=>{const r=e.getBoundingClientRect();return {text:e.textContent.trim(),left:r.left,right:r.right,width:r.width}})'
        )
        assert len(summaries) == 4 and all(
            ("存活" in item["text"] or "已出局" in item["text"])
            and "行" in item["text"]
            and "分" in item["text"]
            and "待落" in item["text"]
            and item["left"] >= 0
            and item["right"] <= width + 1
            and item["width"] > 20
            for item in summaries
        ), summaries
        rivals = js(
            '[...document.querySelectorAll(".tetris-rival")].map(e=>{const r=e.getBoundingClientRect();return {text:e.textContent.trim(),right:r.right}})'
        )
        assert len(rivals) == 3 and all(item["right"] <= width + 1 for item in rivals), rivals
        check("resources-" + str(width), summaries=summaries)
    grid = js(
        '(()=>{const r=document.querySelector(".tetris-grid[data-tetris-grid=self]").getBoundingClientRect();return {width:r.width,height:r.height,cells:document.querySelectorAll(".tetris-grid[data-tetris-grid=self] .tetris-cell").length}})()'
    )
    assert grid["cells"] == 200 and abs(grid["height"] / grid["width"] - 2) < 0.08, grid
    assert grid["width"] >= 180, grid
    check("self-board-geometry", grid=grid)

    b("reload")
    b("wait", ".tetris-board")
    wait_for('document.querySelector(".tetris-board")?.dataset.tetrisPhase==="playing"', 15)
    view = api("/state")["room"]["game"]["data"]
    last = max([event["id"] for event in view["events"]] or [0])
    assert js(
        'Number(document.querySelector(".tetris-board").dataset.lastEventId)>='
        + str(last)
    ), "refresh did not catch up to the server event id"
    assert js(
        '!document.querySelector(".tetris-clear-row")&&!document.querySelector(".tetris-grid.is-clear")'
    ), "refresh replayed an old clear animation"
    check("refresh-without-animation-replay", lastEventId=last)

    # Finish the four-player room and return to the lobby before the practice run.
    command("close")
    command("leave")
    b("reload")
    b("wait", ".lobby-create")
    admin = api("/state")["user"]
    before_rating = api("/players/" + admin["id"])["rating"]

    api(
        "/rooms",
        {
            "kind": "tetris",
            "name": "单人冲刺验收",
            "capacity": 1,
            "options": {"mode": "sprint"},
        },
        201,
    )
    b("reload")
    b("wait", ".waiting-actions")
    command("ready")
    command("start")
    b("wait", ".tetris-board")
    wait_for('document.querySelector(".tetris-board")?.dataset.tetrisPhase==="playing"', 15)
    b("set", "viewport", "1280", "900")
    b("wait", "--fn", '!document.querySelector(".autoplay-control")?.disabled')
    b("find", "role", "button", "click", "--name", "开启托管", "--exact")
    deadline = time.monotonic() + 150
    while time.monotonic() < deadline:
        if js('!!document.querySelector(".match-result-modal")'):
            break
        time.sleep(1)
    assert js('!!document.querySelector(".match-result-modal")'), "practice sprint did not finish"
    result = api("/state")["room"]["result"]
    assert result["unrated"] is True and result["rated"] is False, result
    text = js('document.querySelector(".match-result-modal").textContent')
    assert "练习结束" in text and "不计积分" in text, text
    after_rating = api("/players/" + admin["id"])["rating"]
    assert after_rating["played"] == before_rating["played"], (
        before_rating,
        after_rating,
    )
    assert after_rating["points"] == before_rating["points"], (
        before_rating,
        after_rating,
    )
    shot("practice-result-1280")
    b("set", "viewport", "320", "740")
    shot("practice-result-320")
    b("click", ".modal-head button[aria-label=关闭]")
    b("wait", ".result-reopen")
    b("click", ".result-reopen")
    b("wait", ".match-result-modal")
    check("practice-unrated-and-reopen", result=result, rating=after_rating)

    close_button = js(
        '[...document.querySelectorAll(".match-result-modal button")].find(b=>/再来一局/.test(b.textContent))?.textContent||""'
    )
    assert "再来一局" in close_button
    b("find", "role", "button", "click", "--name", "再来一局", "--exact")
    b("wait", ".waiting-layout")
    check("rematch-returns-to-waiting")
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
    (out / "result.json").write_text(
        json.dumps(report, ensure_ascii=False, indent=2) + "\n"
    )
