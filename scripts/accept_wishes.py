#!/usr/bin/env python3
"""Real-browser acceptance for the game wish pool.

Starts a throw-away Wire Board with a fake OSS endpoint, submits a text +
pasted-screenshot wish, exercises AI clarification and player answers, lets the
agent start a pending wish, completes it in the admin UI, verifies statuses and
saves screenshots.
"""
from __future__ import annotations

import base64
import hashlib
import json
import os
import shutil
import socket
import struct
import zlib
import subprocess
import threading
import time
import urllib.error
import urllib.request
import uuid
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
RUN = uuid.uuid4().hex[:8]
OUT = ROOT / ".local/wishes-e2e" / RUN
OUT.mkdir(parents=True)

INVITE = "wish-accept-code"
ADMIN = "WishAdmin"
ADMIN_PASSWORD = "wish-admin-password-123"
AGENT_TOKEN = "wish-agent-token-acceptance-123"
TEXT = "想做一款 2v2 深海潜艇对战，有热泉、随机补给和可破坏的珊瑚礁。"
NOTE = "已排入下个制作周期"
NOTE_START = "本周开始制作"
NOTE_DONE = "已完成并部署"
def make_png(size: int = 64) -> bytes:
    def chunk(kind: bytes, payload: bytes) -> bytes:
        return (
            struct.pack(">I", len(payload))
            + kind
            + payload
            + struct.pack(">I", zlib.crc32(kind + payload) & 0xFFFFFFFF)
        )

    rows = b"".join(
        b"\x00" + bytes([36, 96, 126]) * size for _ in range(size)
    )
    header = struct.pack(">IIBBBBB", size, size, 8, 2, 0, 0, 0)
    return (
        b"\x89PNG\r\n\x1a\n"
        + chunk(b"IHDR", header)
        + chunk(b"IDAT", zlib.compress(rows, 9))
        + chunk(b"IEND", b"")
    )


PNG = make_png()
report = {"passed": False, "checks": [], "ossPuts": []}


def check(name: str, ok: bool, detail=None):
    report["checks"].append({"name": name, "ok": bool(ok), "detail": detail})
    if not ok:
        raise AssertionError(f"{name}: {detail}")


def free_port() -> int:
    with socket.socket() as sock:
        sock.bind(("127.0.0.1", 0))
        return sock.getsockname()[1]


class OSS:
    store: dict[str, tuple[bytes, str]] = {}

    class Handler(BaseHTTPRequestHandler):
        def _read(self) -> bytes:
            return self.rfile.read(int(self.headers.get("Content-Length", "0")))

        def do_PUT(self):
            data = self._read()
            OSS.store[self.path] = (data, self.headers.get("Content-Type", ""))
            report["ossPuts"].append({
                "path": self.path,
                "bytes": len(data),
                "sha256": hashlib.sha256(data).hexdigest(),
                "contentType": self.headers.get("Content-Type"),
                "forbid": self.headers.get("x-oss-forbid-overwrite"),
            })
            self.send_response(200)
            self.end_headers()

        def do_GET(self):
            if self.path not in OSS.store:
                self.send_response(404)
                self.end_headers()
                return
            data, mime = OSS.store[self.path]
            self.send_response(200)
            self.send_header("Content-Type", mime)
            self.send_header("Content-Length", str(len(data)))
            self.end_headers()
            self.wfile.write(data)

        def do_DELETE(self):
            OSS.store.pop(self.path, None)
            self.send_response(204)
            self.end_headers()

        def log_message(self, *_args):
            pass

    @classmethod
    def start(cls):
        server = ThreadingHTTPServer(("127.0.0.1", 0), cls.Handler)
        threading.Thread(target=server.serve_forever, daemon=True).start()
        return server


def wait_http(url: str, timeout=45):
    end = time.time() + timeout
    while time.time() < end:
        try:
            with urllib.request.urlopen(url, timeout=1) as response:
                if response.status == 200:
                    return
        except Exception:
            time.sleep(0.25)
    raise RuntimeError(f"server did not become ready: {url}")


oss = OSS.start()
app_port = free_port()
base = f"http://127.0.0.1:{app_port}"
oss_base = f"http://127.0.0.1:{oss.server_address[1]}"
binary = OUT / "wire-board"
data = OUT / "data"
png = OUT / "reference.png"
png.write_bytes(PNG)
env = dict(
    os.environ,
    INVITE_CODE=INVITE,
    ADDR=f"127.0.0.1:{app_port}",
    DATA_DIR=str(data),
    ADMIN_USERNAME=ADMIN,
    ADMIN_PASSWORD=ADMIN_PASSWORD,
    OSS_ENDPOINT=oss_base,
    OSS_BUCKET="test-bucket",
    OSS_ACCESS_KEY_ID="acceptance-key",
    OSS_ACCESS_KEY_SECRET="acceptance-secret",
    WISH_IMAGE_BASE_URL=oss_base,
    WISH_IMAGE_PREFIX="bga/wishes",
    WISH_AGENT_TOKEN=AGENT_TOKEN,
    WIRE_BOARD_VERSION="v-wishes-e2e",
    GOTOOLCHAIN="local",
)
app = None
socket_dir = ROOT / ".local/ab3"
if len(str(socket_dir)) > 70:
    socket_dir = Path("/tmp/wb-ab3")
browser_env = dict(
    os.environ,
    AGENT_BROWSER_SOCKET_DIR=str(socket_dir),
    AGENT_BROWSER_SESSION="wishes-" + RUN,
    AGENT_BROWSER_PROFILE=str(OUT / "profile"),
    AGENT_BROWSER_ARGS="--mute-audio,--disable-breakpad,--disable-crash-reporter",
    TMPDIR=str(ROOT / ".local/tmp"),
)


def b(*args):
    result = subprocess.run(
        ["agent-browser", *args, "--json"],
        env=browser_env, text=True, capture_output=True, timeout=60,
    )
    if result.returncode:
        raise RuntimeError(result.stdout + result.stderr)
    value = json.loads(result.stdout)
    if not value.get("success"):
        raise RuntimeError(str(value))
    return value["data"]


def js(code: str):
    return b("eval", code)["result"]


def wait_js(code: str, timeout=15):
    end = time.time() + timeout
    last = None
    while time.time() < end:
        try:
            if js(code):
                return
        except Exception as error:
            last = error
        time.sleep(0.2)
    raise AssertionError(f"timed out waiting for {code}: {last}")


def contrast_check(selectors):
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


def fetch_json(url: str, headers=None, body=None, method="GET"):
    raw = json.dumps(body).encode() if body is not None else None
    request = urllib.request.Request(url, data=raw, method=method)
    if body is not None:
        request.add_header("Content-Type", "application/json")
    for name, value in (headers or {}).items():
        request.add_header(name, value)
    try:
        with urllib.request.urlopen(request, timeout=10) as response:
            return response.status, json.load(response)
    except urllib.error.HTTPError as error:
        return error.code, json.load(error)


def browser_login(name: str, password: str):
    ok = js(
        'fetch("/api/login",{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify('
        + json.dumps({"name": name, "password": password})
        + ")}).then(r=>r.ok)"
    )
    check(f"login {name}", ok is True, ok)


def fetch_bytes(url: str) -> tuple[int, bytes, str]:
    with urllib.request.urlopen(url, timeout=10) as response:
        return response.status, response.read(), response.headers.get_content_type()


try:
    build = subprocess.run(
        ["go", "build", "-o", str(binary), "."],
        cwd=ROOT, env=env, text=True, capture_output=True, timeout=300,
    )
    check("build server", build.returncode == 0, (build.stdout + build.stderr)[-1200:])
    app_log = open(OUT / "server.log", "w")
    app = subprocess.Popen([str(binary)], cwd=ROOT, env=env, stdout=app_log, stderr=subprocess.STDOUT)
    wait_http(base + "/healthz")
    check("healthz", True)

    b("open", base)
    registered = js(
        'fetch("/api/register",{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify('
        + json.dumps({"name": "许愿玩家" + RUN, "password": "wish-player-password", "invite": INVITE})
        + ')}).then(r=>r.ok)'
    )
    check("register player", registered is True, registered)
    b("reload")
    b("wait", ".wish-nav-button")
    check(
        "version shown in lobby",
        js('document.querySelector(".app-version")?.textContent.trim()') == "v-wishes-e2e"
        and js('document.querySelector(".lobby-version")?.textContent.trim()')
        == "v-wishes-e2e",
    )
    b("click", ".wish-nav-button")
    b("wait", ".wish-modal")
    check("wish dialog opens", js('!!document.querySelector(".wish-modal textarea")'))

    b("fill", ".wish-modal textarea", TEXT)
    check("upload control present", js('!!document.querySelector(\'.wish-modal input[type="file"]\')'))
    pasted = js(
        "(()=>{const raw=atob("
        + json.dumps(base64.b64encode(PNG).decode())
        + ");const bytes=new Uint8Array(raw.length);for(let i=0;i<raw.length;i++)bytes[i]=raw.charCodeAt(i);"
        "const file=new File([bytes],'screenshot.png',{type:'image/png'});"
        "const dt=new DataTransfer();dt.items.add(file);"
        "const ev=new ClipboardEvent('paste',{bubbles:true,cancelable:true});"
        "Object.defineProperty(ev,'clipboardData',{value:dt});"
        "return document.querySelector('.wish-modal textarea').dispatchEvent(ev)})()"
    )
    check("paste event dispatched", pasted is True or pasted is False, pasted)
    wait_js('!!document.querySelector(".wish-preview img")')
    preview = js(
        '(()=>{const i=document.querySelector(".wish-preview img");'
        'return {src:i.src.slice(0,5),loaded:!!i.complete&&i.naturalWidth>0,w:i.naturalWidth}})()'
    )
    check("image preview", preview["src"] == "data:" and preview["loaded"], preview)
    b("set", "viewport", "1280", "900")
    b("screenshot", str(OUT / "desktop-wish-dialog.png"))
    ratios = contrast_check([
        [".wish-modal .modal-head h2", ".wish-modal"],
        [".wish-text-label", ".wish-modal"],
        [".wish-upload small", ".wish-modal"],
        [".wish-submit .primary", ".wish-submit .primary"],
    ])
    check(
        "wish dialog contrast",
        all(item.get("ratio", 0) >= 4.5 for item in ratios),
        ratios,
    )
    b("click", ".wish-submit .primary")
    wait_js('!!document.querySelector(".wish-item")')
    first_id = js(
        'fetch("/api/wishes").then(r=>r.json()).then(d=>d.wishes[0].id)'
    )
    check("wish id available", bool(first_id), first_id)
    check("wish submitted", True)
    check("wish pending", js('!!document.querySelector(".wish-item .wish-status.pending")'))

    waits = json.loads(js('JSON.stringify([...document.querySelectorAll(".wish-item p")].map(e=>e.textContent))'))
    check("wish text shown", waits and TEXT in waits[0], waits)
    puts = report["ossPuts"]
    check("one OSS upload", len(puts) == 1, puts)
    put = puts[0]
    check("OSS forbids overwrite", put["forbid"] == "true", put)
    check("OSS stores PNG", put["contentType"] == "image/png", put)
    check("OSS stored exact bytes", put["sha256"] == hashlib.sha256(PNG).hexdigest(), put)
    image_url = js('document.querySelector(".wish-item img")?.src || ""')
    check("image URL is CDN-style", image_url.startswith(oss_base + "/bga/wishes/"), image_url)
    rendered = js(
        '(()=>{const i=document.querySelector(".wish-item img");'
        'return {loaded:!!i&&i.complete&&i.naturalWidth>0,w:i?.naturalWidth||0}})()'
    )
    check("player image renders", rendered["loaded"], rendered)
    status, image, mime = fetch_bytes(image_url)
    check(
        "image readable",
        status == 200 and image == PNG and mime == "image/png",
        {"status": status, "bytes": len(image), "mime": mime},
    )

    agent_headers = {"Authorization": "Bearer " + AGENT_TOKEN}
    status, asked = fetch_json(
        base + "/api/agent/wishes/" + first_id + "/questions",
        headers=agent_headers,
        body={"question": "更希望单人还是多人？需要剧情吗？"},
        method="POST",
    )
    check("agent asks clarification", status == 200 and asked.get("questionCount") == 1, asked)
    status, feed = fetch_json(
        base + "/api/agent/wishes?status=pending", headers=agent_headers
    )
    check(
        "agent sees unanswered question",
        status == 200 and feed["wishes"][0]["needsAnswer"] is True,
        feed,
    )
    b("click", '.wish-modal .icon-button[aria-label="关闭许愿池"]')
    b("click", ".wish-nav-button")
    wait_js('!!document.querySelector(".wish-question")')
    question_text = js('document.querySelector(".wish-question p")?.textContent || ""')
    check("player sees AI question", "单人还是多人" in question_text, question_text)
    b("fill", ".wish-answer textarea", "希望 2 人合作，加一点轻松剧情")
    b("scrollintoview", ".wish-answer button")
    b("click", ".wish-answer button")
    try:
        wait_js('!!document.querySelector(".wish-answer-text")')
    except Exception:
        try:
            b("screenshot", str(OUT / "answer-debug.png"))
        except Exception:
            pass
        report["debug"] = js(
            '(()=>({error:document.querySelector(".wish-answer .admin-message")?.textContent||"",'
            'value:document.querySelector(".wish-answer textarea")?.value||"",'
            'disabled:document.querySelector(".wish-answer button")?.disabled,'
            'html:document.querySelector(".wish-answer")?.outerHTML||"",'
            'wishItem:document.querySelector(".wish-item")?.outerHTML||"",'
            'body:document.body.innerText.slice(0,1500)}))()'
        )
        try:
            report["debug"]["feed"] = js('fetch("/api/wishes").then(r=>r.json())')
        except Exception as error:
            report["debug"]["feedError"] = str(error)
        try:
            report["debug"]["requests"] = b("network", "requests")
        except Exception as error:
            report["debug"]["requestError"] = str(error)
        raise
    check("player answer shown", True)
    status, feed = fetch_json(
        base + "/api/agent/wishes?status=pending", headers=agent_headers
    )
    check(
        "agent sees answer",
        status == 200
        and feed["wishes"][0]["needsAnswer"] is False
        and "2 人合作" in feed["wishes"][0]["questions"][0]["answer"],
        feed,
    )
    status, started = fetch_json(
        base + "/api/agent/wishes/" + first_id,
        headers=agent_headers,
        body={"action": "start", "note": "追问已答复，开始实现"},
        method="POST",
    )
    check(
        "agent starts pending wish",
        status == 200 and started.get("status") == "in_progress",
        started,
    )
    b("click", '.wish-modal .icon-button[aria-label="关闭许愿池"]')
    b("click", ".wish-nav-button")
    wait_js('!!document.querySelector(".wish-status.in-progress")')
    check("player sees in-progress after pending start", True)

    b("set", "viewport", "320", "844")
    b("reload")
    b("wait", ".wish-nav-button")
    b("click", ".wish-nav-button")
    b("wait", ".wish-modal")
    layout = js(
        '(()=>{const m=document.querySelector(".wish-modal").getBoundingClientRect();'
        'return {vw:innerWidth,scroll:document.documentElement.scrollWidth,left:m.left,right:m.right,'
        'textarea:!!document.querySelector(".wish-modal textarea"),submit:!!document.querySelector(".wish-submit .primary")}})()'
    )
    check("320 dialog fits", layout["left"] >= 0 and layout["right"] <= layout["vw"] + 1, layout)
    check("320 no horizontal overflow", layout["scroll"] <= layout["vw"] + 1, layout)
    check("320 controls visible", layout["textarea"] and layout["submit"], layout)
    b("screenshot", str(OUT / "mobile-320-wish.png"))
    b("click", '.wish-modal .icon-button[aria-label="关闭许愿池"]')

    js('fetch("/api/logout",{method:"POST",headers:{"Content-Type":"application/json"},body:"{}"}).then(()=>true)')
    browser_login(ADMIN, ADMIN_PASSWORD)
    b("open", base + "/?admin=1")
    b("wait", ".admin-tabs")
    time.sleep(0.5)
    for label in ["游戏上架", "许愿审核", "用户与在线", "牌桌管理", "操作记录"]:
        clicked = js(
            '(()=>{const x=[...document.querySelectorAll(".admin-tabs button")].find(b=>b.textContent.includes('
            + json.dumps(label)
            + '));if(!x)return false;x.click();return true})()'
        )
        check("admin tab " + label, clicked is True)
        wait_js('!!document.querySelector(".admin-list-refresh")')
        check("admin refresh " + label, True)
        if label == "游戏上架":
            check(
                "admin game cards stay concise",
                js('document.querySelectorAll(".admin-game-card p").length') == 0,
            )
    clicked = js(
        '(()=>{const x=[...document.querySelectorAll(".admin-tabs button")].find(b=>b.textContent.includes("许愿审核"));if(!x)return false;x.click();return true})()'
    )
    check("admin wishes tab", clicked is True)
    b("wait", ".admin-wish-card")
    b("click", ".admin-list-refresh")
    time.sleep(0.3)
    b("wait", ".admin-wish-card")
    admin_text = js('document.querySelector(".admin-wish-text")?.textContent || ""')
    check("admin sees wish", admin_text == TEXT, admin_text)
    check(
        "admin sees image",
        js(
            '(()=>{const i=document.querySelector(".admin-wish-image img");'
            'return !!i&&i.complete&&i.naturalWidth>0})()'
        ),
    )
    check(
        "admin sees Q&A",
        js(
            '(()=>{const q=document.querySelector(".admin-wish-question");'
            'return !!q&&q.textContent.includes("更希望单人还是多人")&&q.textContent.includes("2 人合作")})()'
        ),
    )
    wait_js('!!document.querySelector(".admin-wish-card .wish-status.in-progress")')
    check("admin sees pending AI work as in-progress", True)
    b("set", "viewport", "1280", "900")
    b("screenshot", str(OUT / "admin-wish-in-progress.png"))
    ratios = contrast_check([
        [".admin-wish-text", ".admin-wish-card"],
        [".admin-wish-card header small", ".admin-wish-card"],
        [".wish-status.in-progress", ".wish-status.in-progress"],
        [".admin-wish-question p", ".admin-wish-qa"],
        [".admin-wish-actions .primary", ".admin-wish-actions .primary"],
    ])
    check(
        "admin wish contrast",
        all(item.get("ratio", 0) >= 4.5 for item in ratios),
        ratios,
    )
    status, feed = fetch_json(
        base + "/api/agent/wishes?status=in_progress",
        headers={"Authorization": "Bearer " + AGENT_TOKEN},
    )
    check("agent sees in-progress wish", status == 200 and len(feed.get("wishes", [])) == 1 and feed["wishes"][0]["status"] == "in_progress", feed)

    b("click", ".admin-wish-actions .primary")
    b("wait", ".admin-confirm textarea")
    b("fill", '.admin-confirm textarea[name="note"]', NOTE_DONE)
    b("click", ".admin-confirm button.primary")
    wait_js('!!document.querySelector(".admin-wish-card .wish-status.implemented")')
    check("admin marked implemented", True)
    b("screenshot", str(OUT / "admin-wish-implemented.png"))
    ratios = contrast_check([[".wish-status.implemented", ".wish-status.implemented"]])
    check("implemented contrast", ratios[0]["ratio"] >= 4.5, ratios)

    js('fetch("/api/logout",{method:"POST",headers:{"Content-Type":"application/json"},body:"{}"}).then(()=>true)')
    browser_login("许愿玩家" + RUN, "wish-player-password")
    b("reload")
    b("wait", ".wish-nav-button")
    b("click", ".wish-nav-button")
    b("wait", ".wish-item")
    check("player sees implemented", js('!!document.querySelector(".wish-item .wish-status.implemented")'))
    note = js('document.querySelector(".wish-item .wish-note")?.textContent || ""')
    check("player sees final note", NOTE_DONE in note, note)
    ratios = contrast_check([
        [".wish-status.implemented", ".wish-status.implemented"],
        [".wish-note", ".wish-note"],
    ])
    check(
        "implemented wish contrast",
        all(item.get("ratio", 0) >= 4.5 for item in ratios),
        ratios,
    )
    b("screenshot", str(OUT / "player-wish-implemented.png"))

    status, feed = fetch_json(
        base + "/api/agent/wishes?status=approved",
        headers={"Authorization": "Bearer " + AGENT_TOKEN},
    )
    check("agent no longer sees approved", status == 200 and len(feed.get("wishes", [])) == 0, feed)
    status, feed = fetch_json(
        base + "/api/agent/wishes",
        headers={"Authorization": "Bearer " + AGENT_TOKEN},
    )
    check("agent feed status", status == 200, feed)
    items = feed.get("wishes", [])
    check("agent sees implemented wish", len(items) == 1 and items[0]["text"] == TEXT, items)
    check("agent gets final status", items[0]["status"] == "implemented", items[0])
    check("agent gets note", items[0]["note"] == NOTE_DONE, items[0])
    check("agent gets image", items[0]["imageUrl"] == image_url, items[0])
    status, feed = fetch_json(
        base + "/api/agent/wishes?status=in_progress",
        headers={"Authorization": "Bearer " + AGENT_TOKEN},
    )
    check("agent no longer sees in-progress", status == 200 and len(feed.get("wishes", [])) == 0, feed)
    status, _ = fetch_json(base + "/api/agent/wishes", headers={"Authorization": "Bearer wrong-token"})
    check("agent rejects wrong token", status == 401, status)

    report["passed"] = True
except Exception as error:
    report["error"] = f"{type(error).__name__}: {error}"
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
    oss.shutdown()
    shutil.rmtree(OUT / "profile", ignore_errors=True)
    report["serverLog"] = str(OUT / "server.log")
    (OUT / "result.json").write_text(json.dumps(report, ensure_ascii=False, indent=2) + "\n")
    print(OUT / "result.json")
    if not report["passed"]:
        raise SystemExit(1)
