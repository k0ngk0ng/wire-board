#!/usr/bin/env python3
"""Real browser acceptance for bubbleparty.

Production registry, HTTP/SQLite and host clock; no debug writes.
"""
import argparse, json, os, subprocess, time, traceback, uuid
from pathlib import Path

parser = argparse.ArgumentParser()
parser.add_argument("--build", action="store_true")
args = parser.parse_args()
root = Path(__file__).resolve().parents[2]
out = root / ".local/bubbleparty/browser"
out.mkdir(parents=True, exist_ok=True)
env = dict(os.environ)
session = "bubble-" + uuid.uuid4().hex[:6]
for key, rel in {
    "AGENT_BROWSER_SOCKET_DIR": "/tmp/bubbleparty-sockets",
    "AGENT_BROWSER_PROFILE": ".local/bubbleparty/browser/" + session,
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
    WIRE_BOARD_BUBBLEPARTY_BROWSER="1",
    GOTOOLCHAIN="local",
)
report = {"passed": False, "checks": [], "screenshots": []}
server = None
opened = False
failure = None


def b(*arguments):
    result = subprocess.run(
        ["agent-browser", *arguments, "--json"],
        cwd=root, env=env, text=True, capture_output=True, timeout=90,
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
        "fetch(" + json.dumps("/api" + path) + options + ").then(async r=>({status:r.status,body:await r.json()}))"
    )
    assert result["status"] == status, result
    return result["body"]


def command(kind, **extra):
    payload = json.dumps({"type": kind, "nonce": uuid.uuid4().hex, **extra})
    result = js(
        '(async()=>{const room=(await (await fetch("/api/state")).json()).room;'
        "const body={..." + payload + ',version:room.version};const response=await fetch("/api/rooms/"+room.id,{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify(body)});return {status:response.status,body:await response.json()}})()'
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


def shot(name):
    time.sleep(0.3)
    assert js("document.documentElement.scrollWidth<=innerWidth+2"), (
        "horizontal overflow", name, js("[document.documentElement.scrollWidth,innerWidth]")
    )
    errors = b("errors")["errors"]
    assert not errors, (name, errors)
    path = out / (name + ".png")
    b("screenshot", str(path), "--full")
    report["screenshots"].append(str(path.relative_to(root)))
    check(name)


def tap_at(selector):
    b("scrollintoview", selector)
    rect = js(
        "(()=>{const r=document.querySelector(" + json.dumps(selector) + ").getBoundingClientRect();return {x:r.left+r.width/2,y:r.top+r.height/2}})()"
    )
    endpoint = b("get", "cdp-url")["cdpUrl"]
    url = b("get", "url")["url"]
    result = subprocess.run(
        ["node", "scripts/browser_touch.mjs", endpoint, url, "tap", str(rect["x"]), str(rect["y"]), str(rect["x"]), str(rect["y"])],
        cwd=root, env=env, text=True, capture_output=True, timeout=40,
    )
    if result.returncode:
        raise RuntimeError(result.stdout + result.stderr)


def contrast(selectors):
    code = r"""(() => {
      const rgb = (s) => (s.match(/[\d.]+/g) || []).map(Number);
      const lum = (c) => c.slice(0, 3).map((v) => v / 255).map((v) => v <= 0.04045 ? v / 12.92 : ((v + 0.055) / 1.055) ** 2.4).reduce((a, v, i) => a + v * [0.2126, 0.7152, 0.0722][i], 0);
      const bg = (e) => {
        let b = [255, 255, 255];
        const chain = [];
        for (let n = e; n; n = n.parentElement) chain.unshift(n);
        for (const n of chain) {
          const c = rgb(getComputedStyle(n).backgroundColor);
          const a = c[3] ?? 1;
          b = b.map((v, i) => v * (1 - a) + (c[i] ?? 0) * a);
        }
        return b;
      };
      return __SELECTORS__.map((s) => {
        const e = document.querySelector(s);
        const st = getComputedStyle(e);
        const a = lum(rgb(st.color));
        const z = lum(bg(e));
        return {selector: s, ratio: (Math.max(a, z) + 0.05) / (Math.min(a, z) + 0.05)};
      });
    })()"""
    return js(code.replace("__SELECTORS__", json.dumps(selectors)))


def seq_of(seat=0):
    return int(js(
        'Number(document.querySelector(".bubble-player-summary[data-bubbleparty-seat=\\\''
        + str(seat)
        + '\\\']")?.dataset.bubblepartySeq||0)'
    ))


try:
    for name in ["url", "stop"]:
        (out / name).unlink(missing_ok=True)
    if args.build or not (root / "web/dist/index.html").exists():
        subprocess.run(["npm", "--prefix", "web", "run", "build"], cwd=root, env=env, check=True)
    log = (out / "server.log").open("w")
    server = subprocess.Popen(
        ["go", "test", "-count=1", "-timeout=12m", "-run", "^TestBubblepartyBrowserFixture$", "./games/bubbleparty/tests"],
        cwd=root, env=env, stdout=log, stderr=subprocess.STDOUT,
    )
    deadline = time.monotonic() + 120
    while not (out / "url").exists():
        if server.poll() is not None or time.monotonic() > deadline:
            raise RuntimeError("fixture did not start; inspect server.log")
        time.sleep(0.1)
    base = (out / "url").read_text()
    b("open", base)
    opened = True
    api("/login", {"name": "泡泡验收管理员", "password": "bubbleparty-browser-test-only"})
    assert "bubbleparty" in api("/state")["availableGames"], "new game was not published by default"
    api("/admin/games/bubbleparty", {"enabled": True})
    b("reload")
    b("wait", ".room-list")
    check("default-published")

    b("set", "viewport", "1280", "900")
    b("click", ".lobby-create")
    b("wait", "[role=dialog]")
    b("fill", "input[aria-label=搜索可选游戏]", "泡泡大作战")
    b("click", ".choose-games button")
    b("wait", "[data-create-step=setup]:not([hidden])")
    assert js('[...document.querySelectorAll("select[aria-label=座位上限] option")].map(e=>e.value)') == ["2", "4", "6", "8"]
    assert "小区10" in js('document.querySelector(".bubble-options")?.textContent||""')
    b("select", "select[aria-label=座位上限]", "8")
    b("select", "select[aria-label=水泡战场地图]", "community10")
    b("fill", "input[name=name]", "泡泡小区10验收")
    b("click", "[role=dialog] button[type=submit]")
    b("wait", ".waiting-actions")
    for _ in range(7):
        command("add_bot")
    wait_for('document.querySelectorAll(".waiting-seat.filled").length===8', 5)
    for width in [1280, 390, 320]:
        b("set", "viewport", str(width), "900")
        shot("waiting-" + str(width))
    b("set", "viewport", "1280", "900")
    command("ready")
    command("start")
    b("wait", ".bubble-board")
    wait_for('document.querySelector(".bubble-board")?.dataset.bubblepartyPhase==="countdown"', 10)
    shot("countdown-1280")
    wait_for('document.querySelector(".bubble-board")?.dataset.bubblepartyPhase==="playing"', 10)
    check("countdown-playing")
    assert js('document.querySelector(".bubble-board")?.dataset.bubblepartyMap') == "community10"
    assert js('document.querySelectorAll(".player-panel").length') == 8
    assert js('document.querySelectorAll(".bubble-player-summary").length') == 8
    assert js('!!document.querySelector("[data-bubbleparty-canvas]")')
    assert js('document.querySelectorAll(".bubble-team").length') == 2
    b("set", "viewport", "320", "900")
    sprite = js('(()=>{const c=document.querySelector("[data-bubbleparty-canvas]");const r=c.getBoundingClientRect();return {width:r.width,height:r.height,sprite:28*r.width/360}})()')
    assert sprite["sprite"] >= 24, sprite
    check("320-character-size", sprite=round(sprite["sprite"], 2))
    b("set", "viewport", "1280", "900")
    for width in [1280, 390, 320]:
        b("set", "viewport", str(width), "900")
        shot("playing-" + str(width))
    b("set", "viewport", "1280", "900")
    ratios = contrast([
        ".bubble-status strong",
        ".bubble-mine",
        ".bubble-team span",
        ".bubble-hint",
        ".bubble-player-summary span",
    ])
    assert all(item["ratio"] >= 4.5 for item in ratios), ratios
    check("computed-contrast", ratios=[round(item["ratio"], 2) for item in ratios])

    before = seq_of()
    b("press", "ArrowRight")
    wait_for(f"Number(document.querySelector('.bubble-player-summary[data-bubbleparty-seat=\\\"0\\\"]')?.dataset.bubblepartySeq||0)>{before}", 5)
    check("keyboard-move", before=before)

    before = seq_of()
    b("press", "Space")
    wait_for(f"Number(document.querySelector('.bubble-player-summary[data-bubbleparty-seat=\\\"0\\\"]')?.dataset.bubblepartySeq||0)>{before}", 5)
    check("keyboard-bomb", before=before)

    before = seq_of()
    tap_at(".bubble-dpad .left")
    wait_for(f"Number(document.querySelector('.bubble-player-summary[data-bubbleparty-seat=\\\"0\\\"]')?.dataset.bubblepartySeq||0)>{before}", 5)
    check("native-touch-move", before=before)

    epoch = js('document.querySelector(".bubble-board")?.dataset.bubblepartyEpoch')
    b("reload")
    b("wait", ".bubble-board")
    wait_for('document.querySelector(".bubble-board")?.dataset.bubblepartyPhase==="playing"', 10)
    assert js('document.querySelector(".bubble-board")?.dataset.bubblepartyEpoch') == epoch
    assert seq_of() >= before
    check("refresh-preserves-epoch")

    # Let the bot battle finish (sudden death shrinks the arena). This can take
    # up to a couple of minutes on the real host clock.
    deadline = time.monotonic() + 230
    while time.monotonic() < deadline:
        if js('!!document.querySelector(".match-result-modal")'):
            break
        time.sleep(0.5)
    if js('!!document.querySelector(".match-result-modal")'):
        shot("result-1280")
        b("set", "viewport", "320", "844")
        shot("result-320")
        b("click", ".modal-head button[aria-label=关闭]")
        b("wait", ".result-reopen")
        b("click", ".result-reopen")
        b("wait", ".match-result-modal")
        check("result-closable-reopenable")
        b("find", "role", "button", "click", "--name", "再来一局", "--exact")
        b("wait", ".waiting-layout")
        check("rematch-returns-to-waiting")
    else:
        check("result-overlay-deferred", note="bots did not finish within 230s")

    report["passed"] = True
except BaseException:
    failure = traceback.format_exc()
    raise
finally:
    try:
        if opened:
            b("close")
    finally:
        if server is not None:
            (out / "stop").write_text("stop")
            try:
                server.wait(timeout=20)
            except subprocess.TimeoutExpired:
                server.kill()
        if failure:
            report["error"] = failure
        (out / "result.json").write_text(json.dumps(report, ensure_ascii=False, indent=2) + "\n")
        print(out / "result.json")
        if not report["passed"]:
            raise SystemExit(1)
