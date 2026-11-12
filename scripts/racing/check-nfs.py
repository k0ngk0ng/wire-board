#!/usr/bin/env python3
"""Production-page acceptance for the NFS4 reference remake.

Failure inventory and frozen budgets precede the implementation in
docs/research/racing-nfs/IMPLEMENTATION-PLAN.md. This harness sends ordinary
authenticated commands and native CDP input. It never writes race snapshots,
accelerates the server clock, fakes renderer statistics, or injects results.
"""
import argparse
import base64
import hashlib
import json
import os
import re
from pathlib import Path
import shutil
import signal
import subprocess
import time
import traceback
import uuid


ROOT = Path(__file__).resolve().parents[2]
TRACKS = ("france", "coastal", "uk", "park")
VEHICLES = ("p911", "fm50", "ldia", "mcf1", "ccor")
BUDGET = {"desktopFPS": 50, "desktopP95MS": 35, "touchFPS": 30,
          "touchP95MS": 50, "inputPixelP95MS": 80, "dynamicBytes": 8192}


def require(condition, message, detail=None):
    if not condition:
        raise AssertionError(message + ("\n" + json.dumps(detail, ensure_ascii=False) if detail is not None else ""))


class Acceptance:
    def __init__(self, args):
        self.args = args
        self.out = (ROOT / args.out).resolve()
        require(self.out.is_relative_to(ROOT) and self.out != ROOT and not self.out.exists(),
                "output must be a new dedicated directory inside this repository")
        self.out.mkdir(parents=True)
        self.temp = self.out / "temporary"
        self.env = dict(os.environ)
        self.run = "nfs" + uuid.uuid4().hex[:8]
        for key, value in {
            "TMPDIR": self.temp / "tmp", "GOTMPDIR": self.temp / "tmp",
            "GOCACHE": ROOT / ".local/go-build", "GOMODCACHE": ROOT / ".local/go-mod",
            "npm_config_cache": ROOT / ".local/npm",
            "AGENT_BROWSER_SOCKET_DIR": ROOT / ".s" / self.run,
            "XDG_CACHE_HOME": self.temp / "cache", "XDG_DATA_HOME": self.temp / "data",
            "XDG_CONFIG_HOME": self.temp / "config",
            "AGENT_BROWSER_SCREENSHOT_DIR": self.out,
        }.items():
            value.mkdir(parents=True, exist_ok=True)
            self.env[key] = str(value)
        self.config = self.temp / "agent-browser.json"
        self.config.write_text("{}\n")
        native_probe = self.temp / "native-event-observer.js"
        native_probe.write_text("""(() => {
          const events=[];
          const listener=e=>{events.push({key:e.key,type:e.type,label:e.target instanceof Element?e.target.closest('button')?.getAttribute('aria-label'):null,trusted:e.isTrusted,at:performance.timeOrigin+e.timeStamp,received:performance.timeOrigin+performance.now()});if(events.length>512)events.splice(0,events.length-512)};
          for(const type of ['keydown','keyup','pointerdown','pointerup','pointercancel'])addEventListener(type,listener,true);
          const releaseFault=e=>{const p=window.__nfsSafetyProbe;if(!p?.blockPointerRelease)return;p.events.push({type:e.type,trusted:e.isTrusted,blocked:true,pointerId:e.pointerId,label:e.target instanceof Element?e.target.closest('[aria-label]')?.getAttribute('aria-label'):null,at:performance.timeOrigin+e.timeStamp});e.stopImmediatePropagation()};
          for(const type of ['pointerup','pointercancel','lostpointercapture'])addEventListener(type,releaseFault,true);
          window.__nfsNativeProbe={events,early:true,stop:()=>{for(const type of ['keydown','keyup','pointerdown','pointerup','pointercancel'])removeEventListener(type,listener,true)}};
        })();\n""")
        self.env.update(AGENT_BROWSER_CONFIG=str(self.config), GOTOOLCHAIN="local",
                        AGENT_BROWSER_INIT_SCRIPTS=str(native_probe),
                        AGENT_BROWSER_ARGS="--mute-audio,--disable-breakpad,--disable-crash-reporter,--enable-unsafe-swiftshader",
                        WIRE_BOARD_RACING_BROWSER="1", WIRE_BOARD_RACING_BROWSER_OUT=str(self.out),
                        WIRE_BOARD_REALTIME_NETWORK_CONTROL="1")
        self.sessions = {}
        self.server = None
        self.log = None
        self.base = args.base
        self.report = {"passed": False, "phase": args.phase, "budgets": BUDGET,
                       "scope": "production registry/auth/HTTP/SQLite/clock; native CDP keyboard/touch and compositor PNG frames; no gameplay injection",
                       "screenshots": [], "checks": [], "limitations": [
                           "320px Chromium touch emulation is not physical-phone GPU measurement.",
                           "Standard Gamepad API input is simulated only at navigator.getGamepads; production mapping/sampling/HTTP are used, and no physical controller is claimed tested.",
                           "Ordered TCP delay and interrupted sockets are measured; interruption is not described as IP packet loss.",
                           "Chrome output is muted by launch flag; native HTMLAudio decode/playback/rate/time are verified, not physical loudspeaker audibility."],
                       "command": "python3 -B scripts/racing/check-nfs.py " + " ".join(os.sys.argv[1:])}
        self.report["harnessSHA256"] = {
            path: hashlib.sha256((ROOT / path).read_bytes()).hexdigest()
            for path in ("scripts/racing/check-nfs.py", "scripts/racing/nfs-browser-input.mjs", "games/racing/tests/browser_fixture_test.go")
            if (ROOT / path).exists()
        }
        self.report["sourceSHA256"] = {
            str(path.relative_to(ROOT)): hashlib.sha256(path.read_bytes()).hexdigest()
            for directory in (ROOT / "games/racing/web", ROOT / "games/racing/server")
            for path in sorted(directory.rglob("*")) if path.is_file()
        }
        self.session("main")

    def record(self, name, value):
        (self.out / (name + ".json")).write_text(json.dumps(value, ensure_ascii=False, indent=2) + "\n")

    def check(self, name, **detail):
        self.report["checks"].append(dict(name=name, **detail))
        print("PASS", name, flush=True)

    def reuse_report(self, relative, required=None):
        path = ROOT / relative
        prior = json.loads(path.read_text())
        require(prior["distSHA256"] == self.report["distSHA256"] and prior["sourceSHA256"] == self.report["sourceSHA256"], "cannot reuse checks across different production source/build")
        checks = [check for check in prior["checks"] if required is None or check["name"] in required]
        if required:
            require(required.issubset({check["name"] for check in checks}), "prior actual evidence does not cover reusable gates")
        self.report.setdefault("reusedEvidence", []).append({"report":relative,"sha256":hashlib.sha256(path.read_bytes()).hexdigest(),"sameDistSHA256":prior["distSHA256"],"sameSourceSHA256":True,"checks":checks,"runOverallPassed":prior["passed"]})
        return checks

    def session(self, name):
        if name not in self.sessions:
            profile = self.temp / (name + "-profile")
            profile.mkdir(parents=True)
            env = dict(self.env, AGENT_BROWSER_PROFILE=str(profile))
            result = subprocess.run(["agent-browser", "session", "id", "--scope", "worktree", "--prefix", self.run + "-" + name[:3]],
                                    cwd=ROOT, env=env, text=True, capture_output=True, timeout=15)
            require(result.returncode == 0, "session allocation failed", result.stdout + result.stderr)
            self.sessions[name] = {"id": result.stdout.strip(), "env": env, "opened": False}
        return self.sessions[name]

    def browser(self, *args, name="main", code=None):
        session = self.session(name)
        command = ["agent-browser", "--session", session["id"], *args, "--json"]
        result = subprocess.run(command, cwd=ROOT, env=session["env"], input=code,
                                text=True, capture_output=True, timeout=60)
        require(result.returncode == 0, "browser command failed: " + " ".join(args[:2]), result.stdout + result.stderr)
        try:
            data = json.loads(result.stdout)
        except json.JSONDecodeError as error:
            raise AssertionError("invalid browser JSON: " + result.stdout[:1000]) from error
        require(data.get("success"), "browser command rejected", data)
        return data.get("data", {})

    def js(self, code, name="main"):
        return self.browser("eval", "--stdin", name=name, code=code)["result"]

    def request(self, path, body=None, name="main"):
        options = "" if body is None else ",{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(" + json.dumps(body) + ")}"
        return self.js("(async()=>{const r=await fetch(" + json.dumps("/api" + path) + options + ");return {status:r.status,data:await r.json()}})()", name)

    def api(self, path, body=None, name="main"):
        data = self.request(path, body, name)
        require(data["status"] in (200, 201), "API request rejected: " + path, data)
        return data["data"]

    def room(self, name="main"):
        return self.api("/state", name=name).get("room")

    def command(self, kind, name="main", **extra):
        # A timer may advance after GET /state. A 409 means this command did
        # not apply; retry with a fresh revision and nonce, never change intent.
        for _ in range(12):
            room = self.room(name)
            result = self.request("/rooms/" + room["id"], dict(type=kind, version=room["version"], nonce=uuid.uuid4().hex, **extra), name)
            if result["status"] == 409:
                time.sleep(.02)
                continue
            require(result["status"] == 200, "room command rejected: " + kind, result)
            return result["data"]
        raise AssertionError("room command could not reach a current revision: " + kind)

    def wait(self, callback, seconds=30, description="condition"):
        end = time.monotonic() + seconds
        while time.monotonic() < end:
            if self.server and self.server.poll() is not None:
                raise AssertionError("browser fixture stopped: " + (self.out / "server.log").read_text()[-3000:])
            value = callback()
            if value:
                return value
            time.sleep(.12)
        raise AssertionError(description + " timed out")

    def screenshot(self, name, session="main"):
        path = self.out / (name + ".png")
        self.browser("screenshot", str(path), "--full", name=session)
        require(path.exists() and path.stat().st_size > 2000, "empty page screenshot: " + name)
        self.report["screenshots"].append(name + ".png")

    def native(self, name, actions=None, mode="sequence", session="main", **options):
        endpoint = self.browser("get", "cdp-url", name=session)["cdpUrl"]
        spec = dict(mode=mode, actions=actions or [], **options)
        result = subprocess.run(["node", "scripts/racing/nfs-browser-input.mjs", endpoint, self.base, json.dumps(spec)],
                                cwd=ROOT, env=self.session(session)["env"], text=True, capture_output=True, timeout=100)
        require(result.returncode == 0, "native probe failed: " + name, result.stdout[-2000:] + result.stderr[-3000:])
        value = json.loads(result.stdout)
        for image in value.pop("screenshots", []):
            image_name = name + "-" + image["name"] + ".png"
            (self.out / image_name).write_bytes(base64.b64decode(image["data"]))
            self.report["screenshots"].append(image_name)
        for index, frame in enumerate(value.get("pixelFrames", [])):
            filename = f"{name}-pixel-{index:03}.png"
            (self.out / filename).write_bytes(base64.b64decode(frame.pop("data")))
            frame["file"] = filename
        self.record(name, value)
        require(not value.get("exceptions"), "uncaught browser runtime exception during " + name, value.get("exceptions"))
        return value

    def start_fixture(self):
        if not self.base:
            self.log = (self.out / "server.log").open("w")
            self.server = subprocess.Popen(["go", "test", "-count=1", "-timeout=26m", "-run", "^TestRacingBrowserFixture$", "./games/racing/tests"],
                                           cwd=ROOT, env=self.env, stdout=self.log, stderr=subprocess.STDOUT, start_new_session=True)
            self.wait(lambda: (self.out / "url").exists(), 180, "fixture URL")
            self.base = (self.out / "url").read_text().strip()
        require(self.base.startswith("http://127.0.0.1:"), "only an explicitly owned local fixture is allowed")
        self.browser("open", self.base)
        self.session("main")["opened"] = True
        self.browser("set", "viewport", "1280", "1000")
        self.api("/login", {"name": "赛车验收管理员", "password": "racing-browser-test-only"})
        self.browser("reload")
        self.browser("wait", ".room-list")
        self.report["base"] = self.base
        self.report["distSHA256"] = hashlib.sha256((ROOT / "web/dist/index.html").read_bytes()).hexdigest()
        self.report["browser"] = self.js("({userAgent:navigator.userAgent,viewport:{width:innerWidth,height:innerHeight}})")

    def create_ui(self, track="france", night=False, vehicle="p911", players=6, bots=None):
        self.browser("click", ".lobby-create")
        self.browser("wait", "[role=dialog]")
        self.browser("fill", 'input[aria-label="搜索可选游戏"]', "极品飞车")
        self.browser("snapshot", "-i")
        self.browser("click", ".choose-games button")
        self.browser("wait", 'select[aria-label="赛道"]')
        actual = self.js('[...document.querySelectorAll("select[aria-label=赛道] option")].map(e=>e.value)')
        require(set(actual) == set(TRACKS), "creation must expose exactly four original routes", actual)
        periods = self.js('[...document.querySelectorAll("select[aria-label=时段] option")].map(e=>e.value)')
        require(set(periods) == {"day", "night"}, "original day/night selection missing", periods)
        self.browser("select", 'select[aria-label="赛道"]', track)
        self.browser("select", 'select[aria-label="时段"]', "night" if night else "day")
        self.browser("select", 'select[aria-label="座位上限"]', str(players))
        self.browser("select", 'select[aria-label="比赛圈数"]', "1")
        self.browser("fill", "input[name=name]", "极品飞车原版参考验收")
        self.browser("snapshot", "-i")
        self.screenshot("create-" + track + ("-night" if night else ""))
        self.browser("click", "[role=dialog] button[type=submit]")
        self.browser("wait", ".waiting-layout")
        for _ in range(players - 1 if bots is None else bots):
            self.command("add_bot")
        self.browser("wait", '.racing-preparation select[aria-label="车型"]')
        ids = self.js('[...document.querySelectorAll(".racing-preparation select[aria-label=车型] option")].map(e=>e.value)')
        require(set(ids) == set(VEHICLES), "waiting room must expose five genuine vehicles", ids)
        if self.js('document.querySelector(".racing-preparation select[aria-label=车型]").value') == vehicle:
            other = next(value for value in VEHICLES if value != vehicle)
            self.browser("select", '.racing-preparation select[aria-label="车型"]', other)
            self.wait(lambda: self.room()["options"].get("vehicles", [None])[0] == other, description="changed prepared vehicle")
        self.browser("select", '.racing-preparation select[aria-label="车型"]', vehicle)
        self.wait(lambda: self.room()["options"].get("vehicles", [None])[0] == vehicle, description="prepared vehicle")
        self.screenshot("waiting-" + vehicle)
        self.check("shared creation and per-seat vehicle preparation", track=track, night=night, vehicle=vehicle)

    def observe_loading(self):
        self.browser("console", "--clear")
        self.browser("errors", "--clear")
        self.js("""(()=>{const rows=[];let last='';window.__nfsLoadingProbe={rows};
          const take=()=>{const b=document.querySelector('[data-game-loading] [data-loaded-bytes]'),p=document.querySelector('[data-game-loading] progress');
          if(!b)return;const row={at:performance.now(),loaded:Number(b.dataset.loadedBytes),total:Number(b.dataset.totalBytes),percent:Number(p?.value),text:b.textContent};
          const key=JSON.stringify(row).replace(/\"at\":.*?,/,'');if(key!==last){rows.push(row);last=key;}};
          const observer=new MutationObserver(take);observer.observe(document.body,{subtree:true,childList:true,characterData:true,attributes:true});
          window.__nfsLoadingProbe.stop=()=>observer.disconnect();take();performance.clearResourceTimings();return true})()""")

    def enter(self, track="france", night=False, vehicle="p911"):
        self.observe_loading()
        self.command("ready")
        self.command("start")
        self.browser("wait", ".racing-board")
        self.wait(lambda: self.js('document.querySelector(".racing-chase")?.dataset.classic==="3"&&document.querySelector(".racing-chase")?.dataset.renderer==="webgl"&&!document.querySelector("[data-game-loading]")'),
                  100, "initialized classic WebGL scene")
        self.wait(lambda: self.room()["game"]["data"]["phase"] == "racing", 15, "real starting lights")
        data = self.js("""(()=>{window.__nfsLoadingProbe?.stop();const c=document.querySelector('.racing-chase');return {
          board:{...document.querySelector('.racing-board').dataset},canvas:{...c.dataset},loading:window.__nfsLoadingProbe?.rows??[],
          requests:performance.getEntriesByType('resource').filter(e=>e.name.includes('/racing/v3/')).map(e=>({url:new URL(e.name).pathname,transfer:e.transferSize,decoded:e.decodedBodySize}))}})()""")
        self.record("loading-" + track + ("-night" if night else "") + "-" + vehicle, data)
        console = self.browser("console")
        errors = self.browser("errors")
        self.record("console-" + track + ("-night" if night else "") + "-" + vehicle, {"console": console, "errors": errors})
        require(not re.search(r"couldn't load texture|webglprogram: shader error|failed to load.*texture", json.dumps(console).lower()), "original texture/shader load failed despite renderer initialization", console)
        require(data["board"].get("physics") == "3", "board did not mount v3 physics", data["board"])
        require(data["canvas"].get("vehicle") == vehicle, "renderer ignored prepared original model", data["canvas"])
        require(len(data["canvas"].get("geometry", "")) == 64, "renderer omitted exact geometry identity")
        require(data["canvas"]["geometry"] == self.room()["game"]["data"]["track"]["geometry"], "renderer and authority use different road geometry")
        require(int(data["canvas"].get("decodedImages", 0)) > 0, "original embedded textures were not decoded", data["canvas"])
        pose = self.js("""(async()=>{const room=(await(await fetch('/api/state')).json()).room,d=room.game.data,c=d.cars.find(c=>c.seat===room.you),route=await(await fetch('/racing/v3/tracks/'+d.track.id+'.route.json')).json();
          const i=c.routeIndex??0,a=route.points[i],b=route.points[(i+1)%route.points.length],x=c.x/10,z=c.y/10,dx=b.x-a.x,dz=b.z-a.z,
          u=Math.max(0,Math.min(1,((x-a.x)*dx+(z-a.z)*dz)/(dx*dx+dz*dz))),cx=a.x+dx*u,cz=a.z+dz*u,
          nx=a.nx+(b.nx-a.nx)*u,ny=a.ny+(b.ny-a.ny)*u,nz=a.nz+(b.nz-a.nz)*u,ground=a.y+(b.y-a.y)*u-(nx*(x-cx)+nz*(z-cz))/ny;
          return {height:c.height/10,nativeRoadHeight:ground,difference:c.height/10-ground,routeIndex:i,speed:c.speed,roadMinimum:Math.min(...route.points.map(p=>p.y))}})()""")
        require(abs(pose["difference"]) <= 1.2, "starting vehicle does not sit on the original elevated road", pose)
        self.record("road-pose-" + track + ("-night" if night else "") + "-" + vehicle, pose)
        require(float(data["canvas"].get("triangles", 0)) > 2000, "scene lacks original geometry", data["canvas"])
        rows = data["loading"]
        require(rows and all(0 <= r["loaded"] <= r["total"] and r["total"] > 0 for r in rows), "load progress has no real byte accounting", rows)
        require(max(r["loaded"] for r in rows) == max(r["total"] for r in rows), "renderer became ready before all selected bytes completed", rows)
        for request in data["requests"]:
            path = request["url"]
            if "/tracks/" in path:
                basename = path.rsplit("/", 1)[-1]
                require(basename.startswith(track + ".") or basename.startswith(track + "-"), "unselected route preloaded", data["requests"])
                require(night or "-night" not in basename, "unselected night scene preloaded", data["requests"])
            if "/racing/v3/audio/" in path:
                audio_owner = path.split("/racing/v3/audio/", 1)[1].split("/", 1)[0]
                require(audio_owner not in VEHICLES or audio_owner == vehicle, "unfollowed vehicle engine audio preloaded", data["requests"])
        if self.js("innerWidth") == 320:
            self.browser("scrollintoview", ".racing-chase")
            self.visible_controls("first-touch-entry")
        self.browser("scrollintoview", ".racing-chase")
        self.check("selected original geometry and byte loading", track=track, night=night, bytes=max(r["total"] for r in rows), triangles=data["canvas"]["triangles"])

    def close_room(self):
        tracked = self.js("""(()=>{const c=document.querySelector('.racing-chase[data-classic="3"]');if(!c)return false;window.__nfsTeardownProbe={canvas:c};return true})()""")
        r = self.room()
        if r:
            if r["status"] != "finished":
                self.command("close")
            self.command("leave")
        self.browser("wait", ".room-list")
        if tracked:
            result = self.js("""(async()=>{const c=window.__nfsTeardownProbe.canvas,first=Number(c.dataset.frameCounter);await new Promise(r=>setTimeout(r,220));
              const out={detached:!c.isConnected,first,later:Number(c.dataset.frameCounter),remainingScenes:document.querySelectorAll('.racing-chase').length};delete window.__nfsTeardownProbe;return out})()""")
            require(result["detached"] and result["remainingScenes"] == 0 and result["first"] == result["later"], "departed classic scene kept rendering", result)
            self.check("departed scene detaches and stops observable drawing", **result)

    def smoke(self, skip_controls=False):
        for index, track in enumerate(TRACKS):
            self.create_ui(track, vehicle=VEHICLES[index], players=6)
            if self.args.phase == "contract":
                return
            self.enter(track, vehicle=VEHICLES[index])
            self.screenshot(track + "-desktop")
            if index == 0 and not skip_controls:
                self.revised_controls_contract()
                self.contrast("desktop-contrast")
                self.controls()
            self.close_room()
        for index, track in enumerate(TRACKS if self.args.phase in ("full", "remaining") else ("france",)):
            vehicle = VEHICLES[(index + 4) % len(VEHICLES)]
            self.create_ui(track, night=True, vehicle=vehicle, players=2)
            self.enter(track, night=True, vehicle=vehicle)
            self.screenshot(track + "-night-desktop")
            self.close_room()

    def driving_layout(self, name, touch=False, observed=None):
        layout = observed or self.js("""(()=>{const visible=e=>{if(!e)return false;const r=e.getBoundingClientRect(),s=getComputedStyle(e);return r.width>0&&r.height>0&&s.display!=='none'&&s.visibility!=='hidden'},box=e=>e?e.getBoundingClientRect().toJSON():null;
          const canvas=document.querySelector('.racing-chase'),stage=document.querySelector('.racing-stage'),board=document.querySelector('.racing-board');
          const controls=[...document.querySelectorAll('.racing-board button,.racing-wheel,.racing-skins,.racing-skin-picker,.racing-settings')].filter(visible).map(e=>({label:e.getAttribute('aria-label')||e.textContent.trim(),class:e.className,box:box(e),withinStage:!!stage?.contains(e)}));
          const touch=[...document.querySelectorAll('.racing-steering button,.racing-wheel,.racing-pedals button')].filter(visible).map(e=>({label:e.getAttribute('aria-label'),box:box(e)}));
          const panel=document.querySelector('.racing-controls'),panelStyle=panel?getComputedStyle(panel):null;
          return {width:innerWidth,height:innerHeight,scrollY,canvas:box(canvas),stage:box(stage),board:box(board),controls,touch,
            panel:panel?{withinStage:!!stage?.contains(panel),background:panelStyle.backgroundColor,backgroundImage:panelStyle.backgroundImage,position:panelStyle.position}:null,
            paragraphs:[...document.querySelectorAll('.racing-board p,.racing-view-controls span,.racing-wheel small')].filter(visible).map(e=>e.textContent.trim())}})()""")
        self.record(name, layout)
        c = layout["canvas"]
        require(c and c["width"] >= (280 if touch else 900) and c["height"] >= (380 if touch else 620), "actual driving viewport is too small", layout)
        if touch:
            wanted = {"向左转向", "向右转向", "手刹", "刹车", "油门"}
            require(wanted.issubset({p["label"] for p in layout["touch"]}), "necessary mobile driving controls disappeared", layout)
            panel = layout["panel"]
            alpha = re.findall(r"[\d.]+", panel["background"]) if panel else []
            require(panel and panel["withinStage"] and (len(alpha) == 4 and float(alpha[3]) < 1), "mobile controls are not a transparent overlay inside the stage", panel)
            require(all(p["box"]["left"] >= c["left"] - 1 and p["box"]["right"] <= c["right"] + 1 and p["box"]["top"] >= c["top"] - 1 and p["box"]["bottom"] <= c["bottom"] + 1 for p in layout["touch"]), "touch controls consume space outside the real driving canvas", layout)
            require(all(p["box"]["width"] >= 40 and p["box"]["height"] >= 44 for p in layout["touch"] if p["label"] != "方向盘"), "touch pedal hit area is too small", layout)
        else:
            forbidden = [p for p in layout["controls"] if p["label"] in {"方向盘", "手刹", "刹车", "油门", "向后看", "重置赛车"} or "racing-skin" in p["class"] or "racing-settings" in p["class"]]
            require(not forbidden, "desktop retains driving panels requested to be removed", forbidden)
            require(not layout["paragraphs"], "desktop repeats driving instructions outside the gameplay quick reference", layout["paragraphs"])
        self.check(name, layout=layout)
        return layout

    def revised_controls_contract(self):
        # New failure inventory, written before the revised product build:
        # below-canvas panels, tiny/cropped game view, touch overlay escaping the
        # canvas, C repeat/edge errors, B restoring the wrong view, silent/stuck
        # H, rejected authority horn, stale-input expiry, blur, gamepad analog
        # loss, unarmed reconnect, disconnected holds, and default-key bypass
        # after a public settings remap. No gameplay clock/state is injected.
        failures = []
        def attempt(name, callback):
            try:
                callback()
            except AssertionError as error:
                failures.append({"name": name, "error": str(error)})
        self.browser("set", "viewport", "1280", "1000")
        self.browser("scrollintoview", ".racing-chase")
        self.screenshot("revised-desktop-layout")
        attempt("desktop layout", lambda: self.driving_layout("revised-desktop-layout"))
        attempt("C and B", self.camera_keys)
        attempt("traditional display keys", self.display_keys)
        attempt("H native audio and authority", self.horn_keys)
        attempt("standard Gamepad API and public remapping", self.gamepad_controls)
        self.browser("set", "viewport", "320", "844")
        self.browser("scrollintoview", ".racing-chase")
        touch = self.native("touch-overlay-arm", [{"wait": 150}, {"shot": "layout"}], width=320, height=844, touch=True)
        attempt("touch layout", lambda: self.driving_layout("revised-touch-layout", touch=True, observed=touch["samples"][-1]["data"]["layout"]))
        self.record("revised-controls-failures", failures)
        require(not failures, "revised controls acceptance failed", failures)
        self.browser("set", "viewport", "1280", "1000")

    def thumb_controls(self):
        # MOBILE-TOUCH-PLAN failure inventory was written before this change.
        self.create_ui("france", vehicle="p911", players=2)
        self.enter("france", vehicle="p911")
        self.browser("set", "viewport", "320", "844")
        self.browser("scrollintoview", ".racing-chase")
        self.browser("focus", ".racing-chase")
        initial = self.native("thumb-layout", [{"wait":180},{"scrollTo":".racing-chase"},{"wait":150},{"read":"layout"},{"shot":"portrait"}], width=320,height=844,touch=True)
        row = next(r["data"] for r in initial["samples"] if r["name"]=="layout")
        layout = row["layout"]
        wanted = {"向左转向","向右转向","刹车","油门","手刹"}
        controls = {r["label"]:r["box"] for r in layout["controls"]}
        require("方向盘" not in controls and wanted <= controls.keys(), "replace the mobile wheel with independent left/right thumb buttons", layout)
        c=layout["canvas"];middle=(c["left"]+c["right"])/2
        for label in wanted:
            r=controls[label]; minimum=44 if label=="手刹" else 56
            require(r["width"]>=minimum and r["height"]>=(44 if label=="手刹" else 64),"thumb target too small",{label:r})
            require(r["left"]>=c["left"] and r["right"]<=c["right"] and r["top"]>=c["top"] and r["bottom"]<=c["bottom"],"thumb control outside real canvas",{label:r})
        require(all(controls[k]["right"]<=middle for k in ("向左转向","向右转向")) and all(controls[k]["left"]>=middle for k in ("油门","刹车")),"left steering and right pedals must belong to separate thumbs",controls)
        handbrake=controls["手刹"]
        require(handbrake["width"]<=48 and handbrake["height"]<=48 and handbrake["top"]>=controls["油门"]["bottom"]+4 and c["right"]-handbrake["right"]<=16,"infrequent handbrake must be a small button at the lower-right edge",handbrake)
        self.check("320px wheel-free separated thumb layout",layout=layout)
        left={"selector":'button[aria-label="向左转向"]',"id":1,"x":.9}
        right={"selector":'button[aria-label="向右转向"]',"id":1,"x":.1}
        gas={"selector":'button[aria-label="油门"]',"id":2}
        brake={"selector":'button[aria-label="刹车"]',"id":3}
        v=self.native("thumb-native",[
            {"touch":"touchStart","points":[left,gas]},{"wait":250},{"read":"left-gas"},
            {"touch":"touchMove","points":[right,gas]},{"wait":180},{"read":"slide-right"},
            {"touch":"touchStart","points":[right,gas,brake]},{"wait":180},{"read":"brake"},
            {"touch":"touchCancel","points":[]},{"wait":160},{"read":"cancelled"},
            {"key":"a","down":True},{"touch":"touchStart","points":[right,gas]},{"wait":160},{"read":"opposed"},
            {"touch":"touchCancel","points":[]},{"wait":140},{"read":"keyboard-remains"},{"key":"a","down":False},
            {"touch":"touchStart","points":[left,gas]},{"wait":160},{"blur":True,"milliseconds":200},{"wait":180},{"read":"blur-released"},
            {"touch":"touchCancel","points":[]},{"wait":120},{"shot":"portrait-controls"}],width=320,height=844,touch=True)
        rows={r["name"]:r["data"] for r in v["samples"]}
        for name,steer,throttle in [("left-gas",-1,1),("slide-right",1,1),("brake",1,0),("cancelled",0,0),("opposed",0,1),("keyboard-remains",-1,0),("blur-released",0,0)]:
            r=rows[name]["controls"]
            require(float(r["steer"])==steer and float(r["throttle"])==throttle,"thumb input/composition/release incorrect at "+name,r)
        require(rows["brake"]["controls"]["brake"]=="true","thumb service brake must override throttle",rows["brake"]["controls"])
        require(v["ackMetrics"]["count"]>0 and all(int(k)<400 for k in v["ackMetrics"]["statuses"]),"thumb requests rejected by real authority",v["ackMetrics"])
        self.check("native two-thumb steering, slide, brake priority, mixed keyboard and blur release",ack=v["ackMetrics"])
        rotation=self.native("thumb-orientation",[{"wait":180},{"touch":"touchStart","points":[left,gas]},{"wait":150},{"read":"held"},
            {"viewport":{"width":844,"height":390}},{"wait":250},{"read":"rotated"},{"touch":"touchCancel","points":[]},
            {"touch":"touchStart","points":[right,gas]},{"wait":160},{"read":"fresh"},{"touch":"touchCancel","points":[]},{"wait":100}],width=320,height=844,touch=True)
        rotated={r["name"]:r["data"] for r in rotation["samples"]}
        require(rotated["rotated"]["controls"]["steer"]=="0" and rotated["rotated"]["controls"]["throttle"]=="0","orientation change retained held thumb input",rotated["rotated"]["controls"])
        require(float(rotated["fresh"]["controls"]["steer"])==1 and float(rotated["fresh"]["controls"]["throttle"])==1,"fresh thumb gesture failed after rotation",rotated["fresh"]["controls"])
        self.check("orientation releases held thumbs and accepts fresh gesture")
        secondary=self.native("thumb-keyboard-cancel",[{"focus":'button[aria-label="向左转向"]'},{"key":" ","down":True},{"wait":130},{"read":"held"},
            {"blur":True,"milliseconds":180},{"focus":'button[aria-label="向左转向"]'},{"key":" ","down":True,"repeat":True},{"wait":130},{"read":"repeat-blocked"},
            {"key":" ","down":False},{"key":" ","down":True},{"wait":130},{"read":"fresh"},{"key":" ","down":False},{"wait":100}],width=320,height=844,touch=True)
        keys={r["name"]:r["data"] for r in secondary["samples"]}
        require(keys["held"]["controls"]["steer"]=="-1" and keys["repeat-blocked"]["controls"]["steer"]=="0" and keys["fresh"]["controls"]["steer"]=="-1","direction button resumed stale held keyboard input",{k:v["controls"]for k,v in keys.items()})
        self.check("direction button keyboard focus loss blocks repeat until new press")
        self.browser("focus",".racing-chase")
        # The updated layout must remain reachable after rotation, including
        # actual native fullscreen with the platform's toolbar and exit path.
        self.browser("set","viewport","844","390")
        self.browser("scrollintoview","[data-fullscreen-enter]")
        self.browser("click","[data-fullscreen-enter]")
        self.wait(lambda:self.js("!!document.querySelector('[data-room-fullscreen]')"),description="landscape fullscreen")
        landscape=self.native("thumb-landscape",[{"wait":180},{"scrollTo":".racing-chase"},{"wait":150},{"read":"layout"},{"shot":"landscape"}],width=844,height=390,touch=True)
        row=next(r["data"] for r in landscape["samples"] if r["name"]=="layout")
        allbuttons=[r for r in row["layout"]["controls"] if r["withinStage"]]
        for control in allbuttons:
            r=control["box"]
            require(r["left"]>=0 and r["right"]<=844 and r["top"]>=0 and r["bottom"]<=390,"landscape touch button clipped",control)
        overlap=lambda a,b:max(0,min(a["right"],b["right"])-max(a["left"],b["left"]))*max(0,min(a["bottom"],b["bottom"])-max(a["top"],b["top"]))
        require(all(overlap(a["box"],b["box"])<=1 for i,a in enumerate(allbuttons) for b in allbuttons[i+1:]),"landscape game buttons overlap",allbuttons)
        chat=self.js("document.querySelector('.chat-toggle')?.getBoundingClientRect().toJSON()")
        require(not chat or all(overlap(r["box"],chat)<=1 for r in allbuttons),"fullscreen chat overlaps mobile driving buttons",{"chat":chat,"controls":allbuttons})
        self.check("landscape fullscreen keeps separate visible thumb controls",layout=row["layout"])
        self.browser("click",'[data-game-fullscreen-surface] button[aria-label="退出全屏"]')
        self.browser("set","viewport","320","844")
        self.browser("scrollintoview",".racing-chase")
        self.browser("focus",".racing-chase")
        if self.args.phase=="thumb-layouts":
            self.close_room()
            return
        # Normal input and the same actual two-way TCP delay as the original
        # acceptance, measured with native pointer events and vehicle pixels.
        for label in ("normal","rtt150"):
            if label=="rtt150":
                self.native("thumb-slow-transport",[{"headers":{"X-Acceptance-RTT":"150"}},{"reload":True},{"wait":2200}])
                self.browser("wait",'.racing-chase[data-classic="3"]')
                self.wait(lambda:(self.api("/state").get("network") or {}).get("rttMs",0)>=110,20,"actual 150ms transport RTT")
            self.browser("focus",".racing-chase")
            self.native("thumb-rest-"+label,[{"wait":1100}])
            self.wait(lambda:self.room()["game"]["data"]["cars"][0]["speed"]<=2,10,"actual protected car at rest")
            pixels=self.native("thumb-pixels-"+label,mode="pixels",width=320,height=844,touch=True,touchPixels=True,trials=8)
            require(pixels["metrics"]["visibleTrials"]==8 and pixels["metrics"]["p95MS"]<=BUDGET["inputPixelP95MS"],"native thumb-to-vehicle pixels missed response budget",pixels["metrics"])
            require(all(r["nativeEvent"].get("trusted") and r["nativeEvent"]["type"]=="pointerdown" for r in pixels["trials"]),"thumb measurement substituted keyboard or synthetic input",pixels["trials"])
            self.check("native thumb-to-vehicle pixels "+label,metrics=pixels["metrics"],ack=pixels["ackMetrics"])
        self.close_room()

    def safety_controls(self):
        # Safety failure inventory precedes the Board fix: a lifted steering
        # finger sticks if WebKit/menu routing loses pointerup/cancel/capture;
        # ending it must preserve the other finger, keyboard and Gamepad.
        # HUD/canvas text must not enter selection/callout, while ordinary
        # platform editors retain editing. Faults only suppress native pointer
        # releases at window capture; native touchend/touches remain untouched.
        self.report["limitations"] += [
            "Safari/menu release loss is simulated by a pre-app window-capture blocker for pointerup/pointercancel/lostpointercapture only; touch events remain native CDP events.",
            "Cancelable contextmenu/selectstart guard-boundary probes are explicitly synthetic and are separate from native long-press/drag/right-click evidence.",
            "This is Chromium touch emulation, not physical iOS/Safari testing; unsupported -webkit-touch-callout is checked as a published CSS declaration, not claimed as a Safari computed result."]
        failures=[]
        def expect(name,condition,detail=None):
            if condition:self.check(name)
            else:failures.append({"name":name,"detail":detail})
        def rows(value):return {r["name"]:r["data"] for r in value["samples"]}
        def ctrl(row,steer,gas):return float(row["controls"]["steer"])==steer and float(row["controls"]["throttle"])==gas
        self.create_ui("france",vehicle="p911",players=2)
        self.enter("france",vehicle="p911")
        self.browser("set","viewport","320","844")
        self.browser("scrollintoview",".racing-chase")
        left={"selector":'button[aria-label="向左转向"]',"id":1}
        right={"selector":'button[aria-label="向右转向"]',"id":1}
        gas={"selector":'button[aria-label="油门"]',"id":2}
        def run(name,actions):
            return self.native(name,[{"safetyStart":True},{"wait":150},{"scrollTo":".racing-chase"},{"wait":100},*actions],width=320,height=844,touch=True)
        normal=run("safety-normal-two-finger",[{"safetyDOM":True},{"touch":"touchStart","points":[left,gas]},{"wait":220},{"read":"held"},
            {"touch":"touchEnd","points":[left]},{"wait":180},{"read":"gas-only"},{"touch":"touchEnd","points":[]},{"wait":150},{"read":"released"}])
        r=rows(normal)
        expect("native two-finger lift clears left and preserves gas",ctrl(r["held"],-1,1) and ctrl(r["gas-only"],0,1) and ctrl(r["released"],0,0),{k:v["controls"]for k,v in r.items()})
        dom=normal["safetyDOM"][0]
        expect("all held controls opt in to touch menu guard",len(dom["held"])>=7 and all(x["marked"]for x in dom["held"]),dom["held"])
        held_starts=[e for e in normal["safetyEvents"]if e["type"]=="touchstart"and e["trusted"]and e["label"]in("向左转向","油门")]
        expect("native held touchstart prevents callout defaults",bool(held_starts)and all(e["defaultPrevented"]for e in held_starts),held_starts)
        expect("ordinary click controls do not suppress compatibility click",len(dom["clickOnly"])>=3 and all(not x["marked"]for x in dom["clickOnly"]),dom["clickOnly"])
        expect("board and descendants disable text selection and callout",not dom["violations"] and bool(dom["rootProtectionCSS"]),dom)
        for name,ended,steer,throttle in (("left-lift",[left],0,1),("gas-lift",[gas],-1,0)):
            value=run("safety-menu-loss-"+name,[{"touch":"touchStart","points":[left,gas]},{"wait":220},{"read":"held"},{"dropPointerReleases":True},
                {"touch":"touchEnd","points":ended},{"wait":200},{"read":"one-remains"},{"shot":"stuck-check"},
                {"touch":"touchEnd","points":[]},{"wait":160},{"read":"released"},{"dropPointerReleases":False},
                {"touch":"touchStart","points":[right,gas]},{"wait":200},{"read":"fresh"},{"touch":"touchCancel","points":[]},{"blur":True,"milliseconds":100}])
            r=rows(value);events=value["safetyEvents"]
            expect("native release-loss simulation is exercised "+name,any(e.get("blocked")and e.get("trusted")for e in events)and any(e["type"]=="touchend"and e["trusted"]and len(e["activeTouches"])==1 for e in events),events)
            expect("touch list releases only departed finger "+name,ctrl(r["one-remains"],steer,throttle),{"controls":r["one-remains"]["controls"],"activeTouches":r["one-remains"]["safety"]})
            expect("all lifted touches clear held state "+name,ctrl(r["released"],0,0),r["released"]["controls"])
            expect("fresh gesture works after interrupted release "+name,ctrl(r["fresh"],1,1),r["fresh"]["controls"])
        drag=run("safety-drag-out",[{"touch":"touchStart","points":[left,gas]},{"wait":150},
            {"touch":"touchMove","points":[{"selector":".racing-chase","id":1,"x":.5,"y":.5},gas]},{"wait":180},{"read":"outside"},
            {"touch":"touchEnd","points":[{"selector":".racing-chase","id":1,"x":.5,"y":.5}]},{"wait":100},{"touch":"touchEnd","points":[]},{"wait":120},{"read":"released"}])
        r=rows(drag)
        expect("dragging steering outside releases it without cancelling gas",ctrl(r["outside"],0,1)and ctrl(r["released"],0,0),{k:v["controls"]for k,v in r.items()})
        for key in (" ","Enter"):
            value=run("safety-button-focus-"+("space"if key==" "else"enter"),[{"focus":left["selector"]},{"key":key,"down":True},{"wait":100},{"read":"held"},
                {"focus":".racing-chase"},{"wait":100},{"focus":left["selector"]},{"key":key,"down":True,"repeat":True},{"wait":100},{"read":"repeat"},
                {"key":key,"down":False},{"key":key,"down":True},{"wait":100},{"read":"fresh"},{"key":key,"down":False},{"wait":100}])
            r=rows(value)
            expect("button refocus blocks held key repeat "+repr(key),ctrl(r["held"],-1,0)and ctrl(r["repeat"],0,0)and ctrl(r["fresh"],-1,0),{k:v["controls"]for k,v in r.items()})
        model=self.api("/controllers?game=racing")["models"][0]
        brake=next(b["index"]for b in model["standardGamepad"]["brake"]if b["type"]=="button")
        mixed=run("safety-mixed-sources",[{"focus":".racing-chase"},{"gamepad":{}},{"wait":180},{"key":"a","down":True},{"touch":"touchStart","points":[gas]},{"wait":180},{"read":"keyboard-gas"},
            {"gamepad":{"buttons":{str(brake):.45}}},{"wait":180},{"read":"brake"},{"touch":"touchEnd","points":[]},{"wait":150},{"read":"touch-gone"},
            {"gamepad":{"connected":False}},{"wait":150},{"read":"keyboard-remains"},{"key":"a","down":False},{"gamepadStop":True},{"wait":120}])
        r=rows(mixed)
        expect("touch release preserves keyboard and Gamepad sources",ctrl(r["keyboard-gas"],-1,1)and ctrl(r["brake"],-1,0)and r["touch-gone"]["controls"]["brake"]=="true"and ctrl(r["keyboard-remains"],-1,0)and r["keyboard-remains"]["controls"]["brake"]=="false",{k:v["controls"]for k,v in r.items()})
        click=run("safety-ordinary-touch-click",[{"touch":"touchStart","points":[{"selector":'button[aria-label="切换视角"]',"id":1}]},{"touch":"touchEnd","points":[]},{"wait":180},{"read":"camera"},
            {"touch":"touchStart","points":[{"selector":'button[aria-label="查看赛道地图"]',"id":1}]},{"touch":"touchEnd","points":[]},{"wait":180},{"read":"map"},
            {"touch":"touchStart","points":[{"selector":'.racing-map-modal button[aria-label="关闭"]',"id":1}]},{"touch":"touchEnd","points":[]},{"wait":150},{"read":"closed"}])
        r=rows(click)
        expect("ordinary touch click still changes camera and opens map",r["camera"]["canvas"]["cameraMode"]!=r["before"]["canvas"]["cameraMode"]and r["map"].get("modal")=="赛道地图"and not r["closed"].get("modal"),{k:{"camera":v["canvas"]["cameraMode"],"modal":v.get("modal")}for k,v in r.items()})
        hud=".racing-hud .racing-track-title strong"
        targets=[hud,".racing-chase",left["selector"]]
        actions=[]
        for i,target in enumerate(targets):
            actions += [{"nativeClick":".racing-chase"},{"touch":"touchStart","points":[{"selector":target,"id":1,"allowUnderlay":target==hud}]},{"wait":850},{"read":"long-"+str(i)},{"touch":"touchEnd","points":[]},{"wait":100}]
        actions += [{"nativeClick":".racing-chase"},{"drag":hud,"dx":65,"dy":0},{"wait":100},{"read":"hud-drag"},{"shot":"selection-check"},{"guardProbe":targets}]
        for target in targets:actions += [{"contextMenu":{"selector":target,"allowUnderlay":target==hud}},{"wait":100},{"key":"Escape","down":True},{"key":"Escape","down":False}]
        selection=run("safety-selection-and-menu",actions);r=rows(selection)
        expect("native HUD canvas button long press and HUD drag do not select",all(not r[k]["selected"]for k in ("long-0","long-1","long-2","hud-drag")),{k:r[k]["selected"]for k in ("long-0","long-1","long-2","hud-drag")})
        expect("board contextmenu and selectstart guard defaults",len(selection["guardProbes"])==6 and all(e["defaultPrevented"]for e in selection["guardProbes"]),selection["guardProbes"])
        native_menus=[e for e in selection["safetyEvents"]if e["type"]=="contextmenu"and e["trusted"]]
        expect("native board contextmenu is prevented",len(native_menus)>=3 and all(e["defaultPrevented"]for e in native_menus),native_menus)
        self.browser("click",".chat-toggle")
        self.browser("wait",'textarea[aria-label="聊天消息"]')
        self.browser("fill",'textarea[aria-label="聊天消息"]',"手机编辑选择验收")
        self.browser("focus",'textarea[aria-label="聊天消息"]')
        self.native("safety-editor-native-select",[{"focus":'textarea[aria-label="聊天消息"]'},{"selectEditorAll":True},{"wait":100}])
        editor=self.js("""(()=>{const e=document.querySelector('textarea[aria-label="聊天消息"]'),s=getComputedStyle(e);return {length:e.value.length,start:e.selectionStart,end:e.selectionEnd,userSelect:s.userSelect,callout:s.getPropertyValue('-webkit-touch-callout'),insideBoard:!!e.closest('.racing-board')}})()""")
        expect("platform chat retains native editable selection",not editor["insideBoard"]and editor["start"]==0 and editor["end"]==editor["length"]and editor["userSelect"]!="none",editor)
        editprobe=self.native("safety-editor-guards",[{"safetyStart":True},{"guardProbe":['textarea[aria-label="聊天消息"]']}])
        expect("platform editor contextmenu and selection defaults remain allowed",all(not e["defaultPrevented"]for e in editprobe["guardProbes"]),editprobe["guardProbes"])
        self.record("safety-failures",failures)
        self.report["safety"]={"passed":not failures,"failures":failures,"simulation":"native Chromium CDP touch; only pointer release delivery intentionally blocked; no physical iOS claim"}
        require(not failures,"mobile safety regression gates failed",failures)
        self.close_room()

    def camera_keys(self):
        self.browser("focus", ".racing-chase")
        actions = [{"key": "b", "down": True}, {"wait": 160}, {"read": "B-initial"}, {"shot": "B-initial"}, {"key": "b", "down": False}, {"wait": 160},
            {"key": "c", "down": True}, {"wait": 180}, {"read": "C-first"},
            {"key": "c", "down": True, "repeat": True}, {"wait": 70}, {"key": "c", "down": True, "repeat": True}, {"wait": 70}, {"read": "C-repeat"}, {"key": "c", "down": False},
            {"key": "b", "down": True}, {"wait": 200}, {"read": "B-held"}, {"shot": "B-held"}, {"key": "b", "down": False}, {"wait": 200}, {"read": "B-released"}]
        for index in range(1, 6):
            actions.extend([{"key": "c", "down": True}, {"key": "c", "down": False}, {"wait": 180}, {"read": "C-" + str(index)}, {"shot": "C-" + str(index)},
                {"key": "b", "down": True}, {"wait": 160}, {"read": "B-mode-" + str(index)}, {"shot": "B-mode-" + str(index)}, {"key": "b", "down": False}, {"wait": 160}, {"read": "B-restored-" + str(index)}])
        value = self.native("traditional-camera-keys", actions)
        rows = {row["name"]: row["data"] for row in value["samples"]}
        mode = lambda name: rows[name]["canvas"].get("cameraMode")
        require(mode("before") is not None and mode("C-first") != mode("before"), "C did not change the renderer camera position", rows)
        require(mode("C-repeat") == mode("C-first"), "repeated native keydown cycled camera more than once", rows)
        require(rows["B-held"]["canvas"]["rear"] == "true" and rows["B-released"]["canvas"]["rear"] == "false" and mode("B-released") == mode("C-first"), "B release did not restore the selected camera mode", rows)
        for index in range(1, 6):
            held, restored = rows["B-mode-" + str(index)]["canvas"], rows["B-restored-" + str(index)]["canvas"]
            require(held["rear"] == "true" and restored["rear"] == "false" and held.get("cameraMode") == restored.get("cameraMode") == mode("C-" + str(index)), "B changed or failed to restore camera mode " + str(index), {"held": held, "restored": restored})
        cycle = [mode("before"), mode("C-first"), *(mode("C-" + str(index)) for index in range(1, 6))]
        require(set(cycle) == {"0", "1", "2", "3"} and cycle.count(cycle[0]) >= 2, "C did not cycle through all four actual camera positions", cycle)
        require(rows["C-first"]["canvas"].get("camera") != rows["before"]["canvas"].get("camera"), "camera mode changed without moving the actual camera")
        self.check("native C edges cycle camera and B release restores selected view", modes=cycle)

    def horn_keys(self):
        self.set_sound(True)
        self.browser("focus", ".racing-chase")
        self.js("""(()=>{const events=[],listener=e=>{const a=e.target;if(a instanceof HTMLMediaElement&&a.dataset.platformLoop?.startsWith('racing-horn'))events.push({type:e.type,trusted:e.isTrusted,key:a.dataset.platformLoop,time:a.currentTime,rate:a.playbackRate,readyState:a.readyState,paused:a.paused,error:a.error?.code??null})};for(const type of ['playing','pause','error'])addEventListener(type,listener,true);window.__nfsHornProbe={events,stop:()=>{for(const type of ['playing','pause','error'])removeEventListener(type,listener,true)}}})()""")
        try:
            value = self.native("traditional-H-horn", [{"key": "h", "down": True}, {"wait": 450}, {"read": "held"}, {"wait": 230}, {"read": "advancing"}, {"key": "h", "down": False}, {"wait": 250}, {"read": "released"},
                {"key": "h", "down": True}, {"wait": 200}, {"blur": True, "milliseconds": 350}, {"key": "h", "down": False}, {"wait": 200}, {"read": "blur-released"}])
            rows = {row["name"]: row["data"] for row in value["samples"]}
            own = lambda row: row["room"]["game"]["data"]["cars"][row["room"]["you"]]
            voices = lambda row: [voice for voice in row["audio"] if voice["key"].startswith("racing-horn")]
            require(rows["held"]["controls"].get("horn") == "true" and own(rows["held"]).get("horn") is True, "H did not reach local and authoritative held-horn state", rows)
            initial, later = voices(rows["held"]), voices(rows["advancing"])
            horn_metadata = json.loads((ROOT / "games/racing/assets/public/racing/v3/audio/horn/horn.json").read_text())
            require(initial and all(voice.get("sourceSHA256") == horn_metadata["sha256"] for voice in initial), "native horn source differs from the verified self-composed FLAC", initial)
            require(any(not b["paused"] and b["readyState"] >= 3 and b["volume"] > .001 and b["error"] is None and abs(a["time"] - b["time"]) > .002 for a in initial for b in later if a["key"] == b["key"]), "held H did not decode and advance native horn media", {"before": initial, "after": later})
            events = self.js("window.__nfsHornProbe.events")
            self.record("native-horn-events", events)
            require(any(row["type"] == "playing" and row["trusted"] for row in events), "H produced no trusted native media playing event", events)
            for name in ("released", "blur-released"):
                require(rows[name]["controls"].get("horn") == "false" and own(rows[name]).get("horn", False) is False and not any(not v["paused"] for v in voices(rows[name])), "horn retained after " + name, rows[name])
            require(not rows["blurred"]["focus"] and not any(not v["paused"] for v in voices(rows["blurred"])), "focus loss kept horn media playing", rows["blurred"])
            self.check("native H playback, authority hold/release and blur")
        finally:
            self.js("window.__nfsHornProbe?.stop()")
            self.set_sound(False)

    def horn_expiry(self):
        # The CDP hard-freeze operation leaves this Chrome headless tab hidden
        # even after active. Keep this fault as the last independent gate so no
        # later gameplay or frame measurement runs in an invisible renderer.
        self.browser("set", "viewport", "1280", "1000")
        self.create_ui("france", vehicle="p911", players=2)
        self.enter("france", vehicle="p911")
        self.browser("focus", ".racing-chase")
        value = self.native("horn-real-input-expiry", [{"key": "h", "down": True}, {"wait": 250}, {"read": "held"}, {"freeze": True, "milliseconds": 1250}, {"key": "h", "down": False}])
        held = next(row["data"] for row in value["samples"] if row["name"] == "held")
        frozen = next(row["data"] for row in value["samples"] if row["name"] == "frozen-authority")
        room = frozen["state"]["room"]
        require(held["room"]["game"]["data"]["cars"][held["room"]["you"]].get("horn") is True, "expiry fault started without an authoritative held horn", held["controls"])
        require(frozen["status"] == 200 and room["game"]["data"]["cars"][room["you"]].get("horn", False) is False, "authority retained horn after actual input heartbeat stopped for >750ms", frozen)
        self.check("authority expires H after actual input heartbeat stops for 1250ms")

    def display_keys(self):
        self.browser("focus", ".racing-chase")
        value = self.native("traditional-display-keys", [
            {"key": "F3", "down": True}, {"wait": 160}, {"read": "distance-first"}, {"key": "F3", "down": True, "repeat": True}, {"wait": 120}, {"read": "distance-repeat"}, {"key": "F3", "down": False},
            {"key": "F3", "down": True}, {"key": "F3", "down": False}, {"wait": 160}, {"read": "distance-second"},
            {"key": "F3", "down": True}, {"key": "F3", "down": False}, {"wait": 160}, {"read": "distance-third"},
            {"key": "F5", "down": True}, {"wait": 160}, {"read": "hud-off"}, {"shot": "hud-off"}, {"key": "F5", "down": True, "repeat": True}, {"wait": 120}, {"read": "hud-repeat"}, {"key": "F5", "down": False},
            {"key": "F5", "down": True}, {"key": "F5", "down": False}, {"wait": 160}, {"read": "hud-restored"},
            {"key": "F7", "down": True}, {"wait": 300}, {"read": "mirror-on"}, {"shot": "mirror-on"}, {"key": "F7", "down": True, "repeat": True}, {"wait": 160}, {"read": "mirror-repeat"}, {"key": "F7", "down": False},
            {"key": "F7", "down": True}, {"key": "F7", "down": False}, {"wait": 160}, {"read": "mirror-off"},
            {"key": "F11", "down": True}, {"wait": 160}, {"read": "view-smaller"}, {"shot": "view-smaller"}, {"key": "F11", "down": True, "repeat": True}, {"wait": 120}, {"read": "size-repeat"}, {"key": "F11", "down": False},
            {"key": "F12", "down": True}, {"key": "F12", "down": False}, {"wait": 200}, {"read": "view-restored"}, {"shot": "view-restored"}])
        rows = {row["name"]: row["data"] for row in value["samples"]}
        errors = []
        distance = [rows[name]["canvas"].get("drawDistance") for name in ("before", "distance-first", "distance-second", "distance-third")]
        if distance[0] is None or len(set(distance)) < 3 or rows["distance-first"]["canvas"].get("drawDistance") != rows["distance-repeat"]["canvas"].get("drawDistance"):
            errors.append("F3 must cycle at least three draw-distance settings once per physical edge")
        hud_visible = lambda row: bool(row["hud"] and row["hud"]["display"] != "none" and row["hud"]["visibility"] != "hidden" and row["hud"]["rect"]["height"] > 0)
        if not hud_visible(rows["before"]) or hud_visible(rows["hud-off"]) or hud_visible(rows["hud-repeat"]) or not hud_visible(rows["hud-restored"]):
            errors.append("F5 must hide and restore the actual HUD; repeats must not toggle it")
        mirror_on, mirror_repeat, mirror_off = (rows[name]["canvas"] for name in ("mirror-on", "mirror-repeat", "mirror-off"))
        if mirror_on.get("mirrorVisible") != "true" or mirror_repeat.get("mirrorVisible") != "true" or mirror_off.get("mirrorVisible") != "false":
            errors.append("F7 must toggle the real rear mirror once per physical edge")
        mirror_first = json.loads(mirror_on.get("mirrorFrame", "null"))
        mirror_later = json.loads(mirror_repeat.get("mirrorFrame", "null"))
        if not mirror_first or not mirror_later or not mirror_first.get("live") or not mirror_first.get("rect") or mirror_later.get("frame", 0) <= mirror_first.get("frame", 0) or mirror_first.get("drawCalls", 0) <= 0:
            errors.append("mirror must expose its actual scissor rectangle and advancing rear-scene draw counter")
        smaller, repeated, restored, initial = (rows[name]["canvas"] for name in ("view-smaller", "size-repeat", "view-restored", "before"))
        if smaller.get("viewScale") is None or smaller.get("viewScale") == initial.get("viewScale") or repeated.get("viewScale") != smaller.get("viewScale") or restored.get("viewScale") != initial.get("viewScale"):
            errors.append("F11/F12 must change and restore game-view scale without repeat jumps")
        rect_area = lambda canvas: canvas["rect"]["width"] * canvas["rect"]["height"]
        if rect_area(smaller) >= rect_area(initial) - 100 or abs(rect_area(restored) - rect_area(initial)) > 100:
            errors.append("F11/F12 must change the visible game viewport area and restore it")
        summary = {name: {"canvas": row["canvas"], "hud": row["hud"], "board": row["board"]} for name, row in rows.items()}
        self.record("traditional-display-summary", {"distance": distance, "errors": errors, "observations": summary})
        require(not errors, "traditional display controls failed", errors)
        self.check("native F3/F5/F7/F11/F12 edges change drawing, HUD, real mirror and viewport", distance=distance)

    def gamepad_controls(self):
        data = self.api("/controllers?game=racing")
        model = next(model for model in data["models"] if model["gameId"] == "racing")
        mapping = model.get("standardGamepad", {})
        require(all(mapping.get(action) for action in ("throttle", "right", "brake", "drift", "camera", "rear", "horn")), "racing standard preset omits a required driving action", model)
        def physical(**values):
            buttons, axes = {}, [0, 0, 0, 0]
            for action, value in values.items():
                binding = next((b for b in mapping[action] if b["type"] == ("axis" if action == "right" else "button")), mapping[action][0])
                if binding["type"] == "button":
                    buttons[str(binding["index"])] = value
                else:
                    axes[binding["index"]] = value * binding["direction"]
            return {"buttons": buttons, "axes": axes}
        def own(row):
            return row["room"]["game"]["data"]["cars"][row["room"]["you"]]
        self.set_sound(True)
        self.browser("scrollintoview", ".racing-chase")
        self.browser("focus", ".racing-chase")
        try:
            value = self.native("standard-gamepad-production-drive", [
                {"gamepad": {}}, {"wait": 200}, {"gamepad": physical(throttle=.63, right=.72)}, {"wait": 400}, {"read": "analog"},
                {"gamepad": physical(brake=.42, drift=1)}, {"wait": 200}, {"read": "braking"},
                {"gamepad": {}}, {"wait": 100}, {"gamepad": physical(camera=1)}, {"wait": 180}, {"read": "camera-edge"}, {"wait": 180}, {"read": "camera-held"},
                {"gamepad": physical(rear=1, horn=1)}, {"wait": 350}, {"read": "rear-horn"}, {"gamepad": {"connected": False}}, {"wait": 200}, {"read": "disconnected"},
                {"gamepad": physical(throttle=1, rear=1, horn=1)}, {"wait": 200}, {"read": "unarmed-reconnect"}, {"gamepad": {}}, {"wait": 150},
                {"gamepad": physical(throttle=.6, rear=1, horn=1)}, {"wait": 250}, {"read": "rearmed"}, {"gamepad": {}}, {"wait": 180}, {"read": "released"}])
            rows = {row["name"]: row["data"] for row in value["samples"]}
            analog = rows["analog"]
            require(.4 < float(analog["controls"]["throttle"]) < .85 and .2 < float(analog["controls"]["steer"]) < .95, "platform Gamepad API mapping lost analog trigger/stick values", analog)
            require(own(analog)["speed"] > own(rows["before"])["speed"] + 1 and abs(float(analog["canvas"]["renderX"]) - float(rows["before"]["canvas"]["renderX"])) + abs(float(analog["canvas"]["renderY"]) - float(rows["before"]["canvas"]["renderY"])) > .1, "simulated gamepad did not move the authoritative and rendered vehicle", rows)
            braking = rows["braking"]["controls"]
            require(braking["throttle"] == "0" and braking["brake"] == "true" and .1 < float(braking["brakeAmount"]) < .8 and braking["drift"] == "true", "gamepad analog brake priority or handbrake failed", braking)
            require(rows["camera-edge"]["canvas"].get("cameraMode") != rows["before"]["canvas"].get("cameraMode") and rows["camera-edge"]["canvas"].get("cameraMode") == rows["camera-held"]["canvas"].get("cameraMode"), "held gamepad camera button did not produce one camera edge", rows)
            require(rows["rear-horn"]["canvas"]["rear"] == "true" and rows["rear-horn"]["controls"].get("horn") == "true" and own(rows["rear-horn"]).get("horn") is True and any(v["key"].startswith("racing-horn") and not v["paused"] and v["readyState"] >= 3 and v["volume"] > .001 for v in rows["rear-horn"]["audio"]), "gamepad rear/horn did not reach camera, native audio and authority", rows["rear-horn"])
            for name in ("disconnected", "unarmed-reconnect", "released"):
                row = rows[name]
                require(row["controls"]["throttle"] == "0" and row["controls"].get("horn") == "false" and row["canvas"]["rear"] == "false" and own(row).get("horn", False) is False, "gamepad retained or replayed held input at " + name, row)
            require(rows["rearmed"]["controls"].get("horn") == "true" and float(rows["rearmed"]["controls"]["throttle"]) > .3, "neutral reconnect did not rearm gamepad", rows["rearmed"])
            require(value["ackMetrics"]["count"] > 0 and all(int(status) < 400 for status in value["ackMetrics"]["statuses"]), "simulated gamepad inputs were rejected by production authority", value["ackMetrics"])
            self.check("standard Gamepad API simulation uses production analog mapping, camera/horn and neutral disconnect lifecycle", simulation="navigator.getGamepads hardware boundary only; no physical controller tested", ack=value["ackMetrics"])
            self.remap_controls(model, mapping, physical)
        finally:
            if self.js("!!document.querySelector('.controller-settings')"):
                self.browser("press", "Escape")
                self.browser("press", "Escape")
            self.native("gamepad-simulation-cleanup", [{"gamepadStop": True}, {"wait": 120}])
            self.set_sound(False)

    def remap_controls(self, model, mapping, physical):
        labels = {button["id"]: button["label"] for button in model["buttons"]}
        replacements = {"camera": "v", "rear": "n", "horn": "g"}
        occupied = {binding["index"] for bindings in mapping.values() for binding in bindings if binding["type"] == "button"}
        replacement_button = next(index for index in range(23, 15, -1) if index not in occupied)
        self.browser("click", 'button[aria-label="控制器设置"]')
        self.browser("wait", ".controller-settings [data-controller-model]")
        self.browser("select", "[data-controller-model]", model["id"])
        for action, key in replacements.items():
            self.browser("click", 'button[aria-label=' + json.dumps("修改 " + labels[action] + " 按键", ensure_ascii=False) + ']')
            self.native("keyboard-remap-" + action, [{"key": key, "down": True}, {"key": key, "down": False}, {"wait": 90}])
        self.browser("scrollintoview", "[data-controller-save]")
        self.browser("click", "[data-controller-save]")
        self.wait(lambda: all(self.api("/controllers?game=racing")["profiles"][model["id"]]["keyboard"].get(action) == "Key" + key.upper() for action, key in replacements.items()), description="public keyboard mapping saved")
        self.browser("select", 'select[aria-label="输入来源"]', "gamepad")
        self.browser("wait", '[data-gamepad-binding="horn"] button.controller-key')
        self.browser("click", '[data-gamepad-binding="horn"] button.controller-key')
        self.native("gamepad-remap-horn", [{"gamepad": {"buttons": {str(replacement_button): 1}}}, {"wait": 180}, {"gamepad": {}}, {"wait": 120}])
        self.browser("scrollintoview", "[data-controller-save]")
        self.browser("click", "[data-controller-save]")
        pad_key = "standard:NFS acceptance standard Gamepad API simulation"
        self.wait(lambda: self.api("/controllers?game=racing")["profiles"][model["id"]].get("gamepads", {}).get(pad_key, {}).get("bindings", {}).get("horn") == [{"type": "button", "index": replacement_button}], description="public gamepad horn mapping saved")
        saved = self.api("/controllers?game=racing")
        self.record("public-controller-remapped", saved)
        self.browser("press", "Escape")
        self.browser("wait", ".racing-chase")
        self.browser("scrollintoview", ".racing-chase")
        self.browser("focus", ".racing-chase")
        value = self.native("remapped-old-default-bypass", [{"gamepad": {}}, {"wait": 180},
            {"key": "c", "down": True}, {"key": "b", "down": True}, {"key": "h", "down": True}, {"wait": 200}, {"read": "old-keys"},
            {"key": "c", "down": False}, {"key": "b", "down": False}, {"key": "h", "down": False},
            {"key": "v", "down": True}, {"key": "n", "down": True}, {"key": "g", "down": True}, {"wait": 250}, {"read": "new-keys"},
            {"key": "v", "down": False}, {"key": "n", "down": False}, {"key": "g", "down": False}, {"wait": 150},
            {"gamepad": physical(horn=1)}, {"wait": 180}, {"read": "old-pad"}, {"gamepad": {}}, {"wait": 100},
            {"gamepad": {"buttons": {str(replacement_button): 1}}}, {"wait": 230}, {"read": "new-pad"}, {"gamepad": {}}, {"wait": 180}, {"read": "released"}])
        rows = {row["name"]: row["data"] for row in value["samples"]}
        require(rows["old-keys"]["canvas"].get("cameraMode") == rows["before"]["canvas"].get("cameraMode") and rows["old-keys"]["canvas"]["rear"] == "false" and rows["old-keys"]["controls"].get("horn") == "false", "old C/B/H bypassed the saved keyboard mapping", rows["old-keys"])
        require(rows["new-keys"]["canvas"].get("cameraMode") != rows["before"]["canvas"].get("cameraMode") and rows["new-keys"]["canvas"]["rear"] == "true" and rows["new-keys"]["controls"].get("horn") == "true", "saved V/N/G did not reach game actions", rows["new-keys"])
        require(rows["old-pad"]["controls"].get("horn") == "false" and rows["new-pad"]["controls"].get("horn") == "true" and rows["released"]["controls"].get("horn") == "false", "saved physical button mapping retained old default or failed release", rows)
        self.screenshot("controller-remapped-play")
        self.check("public keyboard and gamepad remapping saves and disables the old defaults")
        # Restore through the same public UI so later canonical C/B/H and
        # front-wheel latency stages test the default platform profile.
        self.browser("click", 'button[aria-label="控制器设置"]')
        self.browser("wait", ".controller-settings [data-controller-model]")
        self.browser("select", "[data-controller-model]", model["id"])
        self.browser("find", "role", "button", "click", "--name", "恢复默认键位", "--exact")
        self.browser("scrollintoview", "[data-controller-save]")
        self.browser("click", "[data-controller-save]")
        self.wait(lambda: self.api("/controllers?game=racing")["profiles"][model["id"]]["keyboard"] == model["defaults"]["keyboard"], description="keyboard defaults restored")
        self.browser("select", 'select[aria-label="输入来源"]', "gamepad")
        self.browser("find", "role", "button", "click", "--name", "恢复此手柄默认映射", "--exact")
        self.browser("scrollintoview", "[data-controller-save]")
        self.browser("click", "[data-controller-save]")
        self.wait(lambda: self.api("/controllers?game=racing")["profiles"][model["id"]].get("gamepads", {}).get(pad_key, {}).get("bindings") == mapping, description="gamepad defaults restored")
        self.browser("press", "Escape")

    def controls(self):
        self.browser("focus", ".racing-chase")
        value = self.native("keyboard", [{"key": "w", "down": True}, {"wait": 850}, {"read": "gas"},
            {"key": "d", "down": True}, {"wait": 120}, {"key": "d", "down": False},
            {"key": "b", "down": True}, {"wait": 250}, {"read": "rear"}, {"shot": "front-model"},
            {"key": "b", "down": False}, {"key": "w", "down": False}, {"wait": 200}, {"read": "released"}])
        sample = {r["name"]: r["data"] for r in value["samples"]}
        require(value["ackMetrics"]["count"] > 0 and all(int(status) < 400 for status in value["ackMetrics"]["statuses"]), "keyboard prediction hid rejected authoritative inputs", value["ackMetrics"])
        require(sample["gas"]["controls"]["throttle"] == "1" and sample["rear"]["canvas"]["rear"] == "true", "native keyboard input did not drive/rear-view", sample)
        car = sample["gas"]["room"]["game"]["data"]["cars"][0]
        require(car.get("rpm", 0) >= 600 and car.get("gear", 0) >= 1, "authoritative engine/transmission telemetry missing", car)
        require(sample["released"]["controls"]["throttle"] == "0" and sample["released"]["canvas"]["rear"] == "false", "key release retained old input", sample)
        self.check("native keyboard acceleration, steering, rear view and release")
        reset = self.native("reset", [{"key": "s", "down": True}, {"waitFor": "resettable"},
            {"key": "r", "down": True}, {"key": "r", "down": False}, {"key": "s", "down": False}, {"wait": 180}, {"read": "reset"}])
        reset_state = next(r["data"]["room"]["game"]["data"] for r in reset["samples"] if r["name"] == "reset")
        require(reset_state["cars"][0]["resetUntil"] > reset_state["serverAt"], "native reset did not enter the authoritative restart wait", reset_state["cars"][0])
        self.check("native R uses authoritative low-speed reset")
        self.wait(lambda: (lambda d: d["serverAt"] >= d["cars"][0]["resetUntil"])(self.room()["game"]["data"]), 10, "actual two-second reset wait")
        self.browser("set", "viewport", "320", "844")
        self.browser("scrollintoview", ".racing-chase")
        value = self.native("touch", [{"touch": "touchStart", "points": [{"selector": 'button[aria-label="油门"]', "id": 1}, {"selector": 'button[aria-label="向右转向"]', "id": 2, "x": .72}]},
            {"wait": 900}, {"read": "held"}, {"touch": "touchStart", "points": [{"selector": 'button[aria-label="油门"]', "id": 1}, {"selector": 'button[aria-label="向右转向"]', "id": 2, "x": .72}, {"selector": 'button[aria-label="刹车"]', "id": 3}]},
            {"wait": 150}, {"read": "brake"}, {"touch": "touchCancel", "points": []}, {"wait": 250}, {"read": "released"}, {"shot": "320-controls"}], width=320, height=844, touch=True)
        sample = {r["name"]: r["data"] for r in value["samples"]}
        require(sample["held"]["controls"]["throttle"] == "1" and float(sample["held"]["controls"]["steer"]) > .1, "native multi-touch did not combine throttle and steering", sample)
        require(sample["brake"]["controls"]["throttle"] == "0" and sample["brake"]["controls"]["brake"] == "true", "touch brake priority failed", sample)
        require(sample["released"]["controls"]["throttle"] == "0" and sample["released"]["controls"]["brake"] == "false", "touch cancellation retained input", sample)
        require(value["ackMetrics"]["count"] > 0 and sample["held"]["room"]["game"]["data"]["cars"][0]["speed"] > sample["before"]["room"]["game"]["data"]["cars"][0]["speed"] + 1, "native touch did not accelerate the actual authoritative car", value["ackMetrics"])
        self.check("320px native simultaneous throttle/steering, brake priority and cancellation")
        self.contrast("touch-contrast")
        self.browser("set", "viewport", "1280", "1000")
        self.browser("click", 'button[aria-label="查看赛道地图"]')
        self.browser("wait", '[role="dialog"]')
        self.screenshot("france-route-map")
        value = self.native("map-zoom", [{"wheel": 'canvas[aria-label="赛车赛道"]', "delta": -180}, {"wait": 150}, {"read": "zoomed"},
            {"drag": 'canvas[aria-label="赛车赛道"]', "dx": 48, "dy": 24}, {"wait": 150}, {"read": "dragged"}])
        require(value["samples"][-1]["data"]["modal"] == "赛道地图", "map gesture dismissed the map")
        zoom = {r["name"]: r["data"]["map"] for r in value["samples"]}
        require(float(zoom["zoomed"]["cameraScale"]) > float(zoom["before"]["cameraScale"]), "native wheel did not zoom route map", zoom)
        require(zoom["dragged"]["cameraX"] != zoom["zoomed"]["cameraX"] or zoom["dragged"]["cameraY"] != zoom["zoomed"]["cameraY"], "native drag did not pan route map", zoom)
        self.browser("press", "Escape")
        self.check("shared route map opens, accepts native zoom and closes")
        before = self.room()["game"]["data"]
        self.browser("reload")
        self.wait(lambda: self.js('document.querySelector(".racing-chase")?.dataset.classic==="3"&&!document.querySelector("[data-game-loading]")'), 90, "same-race refresh")
        after = self.room()["game"]["data"]
        require(after["epoch"] == before["epoch"] and after["maxRaceAt"] == before["maxRaceAt"] and after["cars"][0]["vehicle"] == before["cars"][0]["vehicle"], "refresh reset the original race")
        self.check("refresh preserves race identity, vehicle and deadline")

    def contrast(self, name):
        rows = self.js(r"""(()=>{
          const rgb=s=>(s.match(/[\d.]+/g)||[]).map(Number),mix=(base,paint)=>base.map((v,i)=>v*(1-(paint[3]??1))+(paint[i]??0)*(paint[3]??1));
          const luminance=c=>c.slice(0,3).map(v=>v/255).map(v=>v<=.04045?v/12.92:((v+.055)/1.055)**2.4).reduce((n,v,i)=>n+v*[.2126,.7152,.0722][i],0);
          const backgrounds=e=>{let colors=[[255,255,255]],chain=[];for(let n=e;n;n=n.parentElement)chain.unshift(n);for(const n of chain){const s=getComputedStyle(n),solid=rgb(s.backgroundColor);colors=colors.map(c=>mix(c,solid));
          const stops=[...s.backgroundImage.matchAll(/rgba?\([^)]*\)/g)].map(m=>rgb(m[0]));if(stops.length)colors=colors.flatMap(c=>stops.map(p=>mix(c,p))).slice(-32);}return colors};
          return [...document.querySelectorAll('.racing-hud span,.racing-hud strong,.racing-status,.racing-view-controls button,.racing-view-controls span,.racing-controls button,.racing-controls small,.racing-wheel span')]
            .filter(e=>e.getClientRects().length&&!e.matches(':disabled')&&getComputedStyle(e).visibility!=='hidden').map(e=>{const color=rgb(getComputedStyle(e).color),bg=backgrounds(e),a=luminance(color),ratio=Math.min(...bg.map(c=>{const b=luminance(c);return (Math.max(a,b)+.05)/(Math.min(a,b)+.05)}));return {text:e.textContent.trim(),foreground:getComputedStyle(e).color,backgrounds:bg,ratio}})
        })()""")
        self.record(name, rows)
        require(rows and all(row["ratio"] >= 4.5 for row in rows), "visible driving text contrast below 4.5:1", rows)
        self.check(name)

    def visible_controls(self, name):
        observed = self.native(name + "-native", [{"wait": 150}, {"shot": "layout"}], width=320, height=844, touch=True)
        layout = self.driving_layout(name, touch=True, observed=observed["samples"][-1]["data"]["layout"])
        require(all(p["box"]["left"] >= -1 and p["box"]["right"] <= layout["width"] + 1 and p["box"]["top"] >= -1 and p["box"]["bottom"] <= layout["height"] + 1 for p in layout["touch"]), "mobile viewport crops in-canvas controls", layout)

    def performance(self, name, session="main", touch=False, mirror=False):
        width, height = (320, 844) if touch else (1280, 1000)
        self.browser("set", "viewport", str(width), str(height), name=session)
        self.browser("scrollintoview", ".racing-chase", name=session)
        value = self.native(name, mode="performance", session=session, width=width, height=height, touch=touch, milliseconds=6000, screenshot="scene")
        m = value["metrics"]
        require(value["before"]["canvas"].get("mirrorVisible") == str(mirror).lower(), "performance scene did not use its stated mirror setting", value["before"]["canvas"])
        if mirror:
            first = json.loads(value["before"]["canvas"].get("mirrorFrame", "null"))
            last = json.loads(value["after"]["canvas"].get("mirrorFrame", "null"))
            require(first and last and last["frame"] > first["frame"] and last.get("drawCalls", 0) > 0, "moving mirror performance omitted actual rear-world draws", {"first": first, "last": last})
        require(m["actualDrawUpdates"] > 0 and m["moved"] > 1, "performance sample was static, not a moving race", m)
        require(m["fps"] >= BUDGET["touchFPS" if touch else "desktopFPS"] and m["p95MS"] <= BUDGET["touchP95MS" if touch else "desktopP95MS"], "frozen frame budget failed", m)
        require(value["states"], "no authoritative wire samples captured")
        require(all(s["bytes"] <= BUDGET["dynamicBytes"] and not any(s["staticCounts"].values()) for s in value["states"]), "dynamic frames repeat static geometry or exceed budget", value["states"][:2])
        if mirror and touch:
            c = value["after"]["canvas"]["rect"]
            r = json.loads(value["after"]["canvas"]["mirrorFrame"])["rect"]
            layout = {"mirror": {"left": c["left"] + r["x"], "top": c["top"] + r["y"], "right": c["left"] + r["x"] + r["width"], "bottom": c["top"] + r["y"] + r["height"]}, "parts": value["after"]["layout"]["overlays"]}
            self.record(name + "-layout", layout)
            overlap = lambda r: max(0, min(r["right"], layout["mirror"]["right"]) - max(r["left"], layout["mirror"]["left"])) * max(0, min(r["bottom"], layout["mirror"]["bottom"]) - max(r["top"], layout["mirror"]["top"]))
            require(all(overlap(part["rect"]) <= 1 for part in layout["parts"]), "320px live mirror overlaps necessary HUD or driving controls", layout)
        self.check(name, metrics=m, maxDynamicBytes=max(s["bytes"] for s in value["states"]), ack=value["ackMetrics"])
        return value

    def models_and_complete_race(self, skip_presentation=False):
        if not skip_presentation:
            self.presentation_permissions()
        for vehicle in VEHICLES:
            self.create_ui("france", vehicle=vehicle, players=2)
            with_audio = vehicle in ("p911", "fm50")
            if with_audio:
                self.set_sound(True)
            self.enter("france", vehicle=vehicle)
            self.browser("scrollintoview", ".racing-chase")
            self.browser("focus", ".racing-chase")
            self.screenshot(vehicle + "-rear")
            self.native(vehicle + "-front", [{"key": "b", "down": True}, {"wait": 350}, {"shot": "front"}, {"key": "b", "down": False}])
            actions = []
            for _ in range(3):
                actions.extend([{"key": "c", "down": True}, {"key": "c", "down": False}, {"wait": 120}])
            actions.extend([{"read": "cockpit"}, {"shot": "cockpit"}, {"key": "b", "down": True}, {"wait": 160}, {"key": "b", "down": False}, {"wait": 160}, {"read": "cockpit-restored"}])
            cockpit = self.native(vehicle + "-original-cockpit", actions)
            cockpit_rows = {row["name"]: row["data"] for row in cockpit["samples"]}
            require(cockpit_rows["cockpit"]["canvas"].get("cameraMode") == "3" and cockpit_rows["cockpit-restored"]["canvas"].get("cameraMode") == "3" and cockpit_rows["cockpit-restored"]["canvas"].get("rear") == "false", "original vehicle cockpit did not render/restore after rear look " + vehicle, {name: row["canvas"] for name, row in cockpit_rows.items()})
            self.check("original selected-vehicle cockpit and rear release " + vehicle)
            if with_audio:
                self.original_engine_audio(vehicle)
            self.close_room()
            if with_audio:
                require(self.js("document.querySelectorAll('audio[data-platform-loop^=\"racing-engine-\"]').length") == 0, "departed room retained engine media elements")
                self.check("native original-engine owner cleanup " + vehicle)
                self.set_sound(False)
        self.browser("set", "viewport", "320", "844")
        self.create_ui("park", vehicle="mcf1", players=6)
        self.enter("park", vehicle="mcf1")
        self.screenshot("park-mobile-first-entry")
        self.close_room()
        self.complete_race()

    def set_sound(self, enabled, session="main"):
        self.browser("click", '[data-sound-trigger]', name=session)
        self.browser("wait", '[data-sound-panel]', name=session)
        muted = self.js("document.querySelector('[data-sound-mute]').getAttribute('aria-pressed')==='true'", session)
        if muted == enabled:
            self.browser("click", '[data-sound-mute]', name=session)
        self.wait(lambda: self.js("document.querySelector('[data-sound-mute]').getAttribute('aria-pressed')", session) == ("false" if enabled else "true"), description="public sound preference")
        self.browser("press", "Escape", name=session)

    def rear_button_repeat(self):
        # Fresh Space/Enter may hold rear view. Losing element focus cancels;
        # repeated native keydown after refocus must stay cancelled until up.
        self.browser("set", "viewport", "320", "844")
        self.native("rear-button-touch-mode", [{"wait": 100}], width=320, height=844, touch=True)
        self.browser("scrollintoview", ".racing-chase")
        target = 'button[aria-label="向后看"]'
        for key in (" ", "Enter"):
            value = self.native("rear-button-repeat-" + ("space" if key == " " else "enter"), [{"focus": target}, {"key": key, "down": True}, {"wait": 120}, {"read": "held"},
                {"focus": ".racing-chase"}, {"wait": 100}, {"read": "blurred-element"}, {"focus": target}, {"key": key, "down": True, "repeat": True}, {"wait": 120}, {"read": "repeat"},
                {"key": key, "down": False}, {"key": key, "down": True}, {"wait": 120}, {"read": "fresh"}, {"key": key, "down": False}, {"wait": 100}, {"read": "released"}], width=320, height=844, touch=True)
            rows = {row["name"]: row["data"] for row in value["samples"]}
            require(rows["held"]["canvas"]["rear"] == rows["fresh"]["canvas"]["rear"] == "true" and all(rows[name]["canvas"]["rear"] == "false" for name in ("blurred-element", "repeat", "released")), "rear button refocus replayed a held " + repr(key), {name: row["canvas"] for name, row in rows.items()})
        self.check("rear button Space/Enter focus loss blocks repeat until fresh press")

    def readonly_presentation(self, name, session="main"):
        self.browser("scrollintoview", ".racing-chase", name=session)
        self.browser("focus", ".racing-chase", name=session)
        value = self.native("readonly-presentation-" + name, [{"key": "c", "down": True}, {"key": "c", "down": False}, {"key": "F3", "down": True}, {"key": "F3", "down": False},
            {"key": "F5", "down": True}, {"key": "F5", "down": False}, {"key": "F7", "down": True}, {"key": "F7", "down": False}, {"key": "F11", "down": True}, {"key": "F11", "down": False},
            {"key": "b", "down": True}, {"key": "w", "down": True}, {"key": "h", "down": True}, {"wait": 650}, {"read": "held"}, {"shot": "presentation"},
            {"key": "w", "down": False}, {"key": "h", "down": False}, {"key": "b", "down": False}, {"key": "F12", "down": True}, {"key": "F12", "down": False},
            {"key": "F5", "down": True}, {"key": "F5", "down": False}, {"key": "F7", "down": True}, {"key": "F7", "down": False}, {"wait": 850}, {"read": "restored"}], session=session)
        rows = {row["name"]: row["data"] for row in value["samples"]}
        first, held, restored = (rows[key] for key in ("before", "held", "restored"))
        require(held["canvas"]["cameraMode"] != first["canvas"]["cameraMode"] and held["canvas"]["drawDistance"] != first["canvas"]["drawDistance"] and held["canvas"]["mirrorVisible"] == "true" and held["canvas"]["viewScale"] != first["canvas"]["viewScale"], "presentation keys were disabled during " + name, {key: row["canvas"] for key, row in rows.items()})
        require(held["canvas"]["rear"] == "true" and restored["canvas"]["rear"] == "false" and restored["canvas"]["cameraMode"] == held["canvas"]["cameraMode"] and restored["canvas"]["viewScale"] == first["canvas"]["viewScale"], "rear/view-size restoration failed during " + name)
        require(not held["hud"] or held["hud"]["display"] == "none" or held["hud"]["visibility"] == "hidden", "F5 failed during " + name, held["hud"])
        require(not any(packet["type"] == "input" for packet in value["packets"]), "read-only presenter sent gameplay input during " + name, value["packets"])
        require(not held["controls"] or held["controls"].get("horn") == "false" and held["controls"]["throttle"] == "0", "read-only presenter retained local driving input during " + name, held["controls"])
        voices = lambda row: [voice for voice in row["audio"] if voice["key"].startswith("racing-engine-") and not voice["paused"] and voice["readyState"] >= 3 and voice["volume"] > .001]
        require(any(abs(a["time"] - b["time"]) > .002 for a in voices(held) for b in voices(restored) if a["key"] == b["key"]), "engine stayed suspended during " + name, {"held": held["audio"], "restored": restored["audio"]})
        self.check("readonly presentation and advancing native engine during " + name)

    def presentation_permissions(self):
        self.browser("set", "viewport", "1280", "1000")
        self.create_ui("france", vehicle="p911", players=2)
        self.set_sound(True)
        self.enter("france", vehicle="p911")
        self.rear_button_repeat()
        self.browser("set", "viewport", "1280", "1000")
        self.browser("find", "role", "button", "click", "--name", "开启托管", "--exact")
        self.wait(lambda: self.room()["seats"][0].get("autoPlay"), description="presentation autoplay")
        self.readonly_presentation("autoplay")
        self.browser("open", self.base, name="watcher")
        self.session("watcher")["opened"] = True
        self.browser("set", "viewport", "1280", "1000", name="watcher")
        self.api("/register", {"name": "赛车观战验收" + self.run[-4:], "password": "racing-browser-test-only", "invite": "racing-browser"}, name="watcher")
        self.browser("reload", name="watcher")
        self.browser("wait", ".room-list", name="watcher")
        self.set_sound(True, "watcher")
        self.api("/rooms/" + self.room()["id"] + "/watch", {}, name="watcher")
        self.browser("reload", name="watcher")
        self.wait(lambda: self.js('document.querySelector(".racing-chase")?.dataset.classic==="3"&&!document.querySelector("[data-game-loading]")', "watcher"), 90, "read-only spectator scene")
        require(self.room("watcher")["spectating"] and not self.room("watcher")["game"]["data"]["controls"]["canDrive"], "separate viewer was not read-only")
        self.readonly_presentation("spectator", "watcher")
        self.browser("find", "role", "button", "click", "--name", "离开观战", "--exact", name="watcher")
        self.browser("wait", ".room-list", name="watcher")
        self.close_room()
        self.set_sound(False)

    def audio_csp_diagnostic(self):
        self.create_ui("france", vehicle="p911", players=2)
        self.set_sound(True)
        self.js("""(()=>{const rows=[];addEventListener('securitypolicyviolation',e=>rows.push({directive:e.effectiveDirective,blocked:e.blockedURI,disposition:e.disposition,source:e.sourceFile,line:e.lineNumber}));window.__nfsCSP=rows})()""")
        self.enter("france", vehicle="p911")
        self.browser("focus", ".racing-chase")
        self.native("audio-csp-native-H", [{"key":"h","down":True},{"wait":550},{"read":"held"},{"key":"h","down":False}])
        proof = {"csp":self.js("window.__nfsCSP"),"console":self.browser("console"),"errors":self.browser("errors"),
                 "foreground":self.js("({focus:document.hasFocus(),visibility:document.visibilityState,volume:localStorage.getItem('wire-board.volume'),audio:[...document.querySelectorAll('audio')].map(a=>({key:a.dataset.platformLoop,ready:a.readyState,error:a.error?.code,src:a.currentSrc.slice(0,100)}))})")}
        self.record("audio-csp-proof",proof)
        require(any(row["directive"] == "media-src" and row["blocked"].startswith("blob") for row in proof["csp"]),"no actual media CSP violation captured",proof)
        self.check("actual verified-byte blob media rejected by production CSP", violations=len(proof["csp"]))

    def fullscreen_validation(self):
        self.create_ui("france", vehicle="p911", players=2)
        self.enter("france", vehicle="p911")
        initial = self.room()["game"]["data"]
        for touch in (False, True):
            label = "320" if touch else "desktop"
            self.browser("set", "viewport", "320" if touch else "1280", "844" if touch else "1000")
            self.browser("scrollintoview", "[data-fullscreen-enter]")
            baseline = self.js("""(()=>{window.__nfsFullscreenCanvas=document.querySelector('.racing-chase');return {inert:[...document.querySelectorAll('[inert]')].filter(e=>e.inert).map(e=>e.id||e.className),overflow:document.documentElement.style.overflow}})()""")
            self.browser("click", "[data-fullscreen-enter]")
            self.wait(lambda: self.js("!!document.querySelector('[data-game-fullscreen-surface][data-room-fullscreen]')"), description="public fullscreen entry")
            mode = self.js("""(()=>{const root=document.querySelector('[data-game-fullscreen-surface]');return {mode:root.dataset.roomFullscreen,nativeIsSurface:document.fullscreenElement===root,nativeIsPage:document.fullscreenElement===document.body||document.fullscreenElement===document.documentElement,containsTopbar:!!root.querySelector('.topbar'),sameCanvas:root.contains(window.__nfsFullscreenCanvas)}})()""")
            require(mode["mode"] in ("native", "page") and not mode["nativeIsPage"] and not mode["containsTopbar"] and mode["sameCanvas"] and (mode["mode"] != "native" or mode["nativeIsSurface"]), "fullscreen included the outer page or remounted the game", mode)
            if touch:
                value = self.native("fullscreen-320-controls", [{"wait":150},{"touch":"touchStart","points":[{"selector":'button[aria-label="油门"]',"id":1},{"selector":'button[aria-label="向右转向"]',"id":2,"x":.72}]},{"wait":450},{"read":"drive"},{"touch":"touchCancel","points":[]},
                    {"touch":"touchStart","points":[{"selector":'button[aria-label="查看赛道地图"]',"id":3}]},{"touch":"touchEnd","points":[]},{"wait":200},{"read":"map"},{"shot":"map"}],width=320,height=844,touch=True)
                rows = {row["name"]:row["data"] for row in value["samples"]}
                require(rows["drive"]["controls"]["throttle"] == "1" and float(rows["drive"]["controls"]["steer"]) > .1 and value["ackMetrics"]["count"] > 0 and rows["map"].get("modal") == "赛道地图", "320px fullscreen lost touch driving or its map", rows["drive"]["controls"])
                self.driving_layout("fullscreen-320-driving-layout", touch=True, observed=rows["drive"]["layout"])
            else:
                self.browser("focus", ".racing-chase")
                value = self.native("fullscreen-desktop-drive", [{"key":"w","down":True},{"wait":350},{"read":"drive"},{"key":"w","down":False}])
                require(next(row["data"] for row in value["samples"] if row["name"] == "drive")["controls"]["throttle"] == "1" and value["ackMetrics"]["count"] > 0, "desktop fullscreen lost keyboard driving")
                self.browser("click", 'button[aria-label="查看赛道地图"]')
                self.browser("wait", ".racing-map-modal")
                self.screenshot("fullscreen-desktop-map")
            containment = self.js("""(()=>{const root=document.querySelector('[data-game-fullscreen-surface]'),map=document.querySelector('.racing-map-modal'),r=map?.getBoundingClientRect();return {mapWithinSurface:!!map&&root.contains(map),sameCanvas:root.contains(window.__nfsFullscreenCanvas),rect:r?.toJSON(),width:innerWidth,height:innerHeight}})()""")
            require(containment["mapWithinSurface"] and containment["sameCanvas"] and containment["rect"]["left"] >= -1 and containment["rect"]["right"] <= containment["width"] + 1, "fullscreen map escaped or cropped the game surface", containment)
            if touch:
                self.browser("click", '.racing-map-modal button[aria-label="关闭"]')
                self.browser("click", '[data-game-fullscreen-surface] button[aria-label="退出全屏"]')
            else:
                self.browser("press", "Escape")
            self.wait(lambda: self.js("!document.fullscreenElement&&!document.querySelector('[data-room-fullscreen]')"), description="fullscreen exit")
            cleanup = self.js("""({inert:[...document.querySelectorAll('[inert]')].filter(e=>e.inert).map(e=>e.id||e.className),overflow:document.documentElement.style.overflow,sameCanvas:document.querySelector('.racing-chase')===window.__nfsFullscreenCanvas})""")
            after = self.room()["game"]["data"]
            require(cleanup["inert"] == baseline["inert"] and cleanup["overflow"] == baseline["overflow"] and cleanup["sameCanvas"] and after["epoch"] == initial["epoch"] and after["maxRaceAt"] == initial["maxRaceAt"], "fullscreen exit retained inert/scroll state or restarted the race", cleanup)
            self.record("fullscreen-" + label, {"mode":mode,"containment":containment,"baseline":baseline,"cleanup":cleanup,"epoch":after["epoch"],"maxRaceAt":after["maxRaceAt"]})
            self.check("public fullscreen gameplay/map and clean exit " + label, mode=mode["mode"], exit="button" if touch else "Escape")
        self.close_room()

    def engine_media(self):
        return self.js("""Promise.all([...document.querySelectorAll('audio[data-platform-loop^="racing-engine-"]')].map(async a=>{const source=a.currentSrc;let sourceSHA256=null;if(source.startsWith('data:audio/flac;base64,')){const bytes=Uint8Array.from(atob(source.split(',')[1]),c=>c.charCodeAt(0));sourceSHA256=[...new Uint8Array(await crypto.subtle.digest('SHA-256',bytes))].map(n=>n.toString(16).padStart(2,'0')).join('')};return {key:a.dataset.platformLoop,src:source.slice(0,100),sourceLength:source.length,sourceSHA256,paused:a.paused,readyState:a.readyState,time:a.currentTime,rate:a.playbackRate,volume:a.volume,muted:a.muted,error:a.error?.code??null}}))""")

    def original_engine_audio(self, vehicle):
        self.js("""(()=>{window.__nfsMediaProbe?.stop?.();const events=[],types=['playing','pause','ratechange','error'];const listener=e=>{const a=e.target;if(!(a instanceof HTMLMediaElement)||!a.dataset.platformLoop?.startsWith('racing-engine-'))return;
          events.push({at:Date.now(),type:e.type,trusted:e.isTrusted,key:a.dataset.platformLoop,time:a.currentTime,rate:a.playbackRate,volume:a.volume,readyState:a.readyState,paused:a.paused,error:a.error?.code??null});if(events.length>3000)events.splice(0,events.length-3000)};
          for(const type of types)addEventListener(type,listener,true);window.__nfsMediaProbe={events,stop:()=>{for(const type of types)removeEventListener(type,listener,true)}};return true})()""")
        self.browser("scrollintoview", ".racing-chase")
        self.browser("focus", ".racing-chase")
        self.native("audio-wake-" + vehicle, [{"key": "ArrowLeft", "down": True}, {"wait": 40}, {"key": "ArrowLeft", "down": False}])
        try:
            self.wait(lambda: any(not row["paused"] and row["readyState"] >= 3 and row["time"] > 0 for row in self.engine_media()), 15, "original engine native playback " + vehicle)
        finally:
            self.record("audio-initial-" + vehicle, {"voices": self.engine_media(), "events": self.js("window.__nfsMediaProbe.events"), "console": self.browser("console")})
        before = self.engine_media()
        time.sleep(.233)
        after = self.engine_media()
        advancing = [(a, b) for a in before for b in after if a["key"] == b["key"] and not b["paused"] and b["readyState"] >= 3 and abs(b["time"] - a["time"]) > .002]
        metadata = json.loads((ROOT / "games/racing/assets/public/racing/v3/audio" / vehicle / "engine.json").read_text())
        expected_sources = {f"racing-engine-{vehicle}-{sample['patch']}": sample["sha256"] for sample in metadata["samples"]}
        require(advancing and all(row["key"] in expected_sources and row["src"].startswith("data:audio/flac;base64,") and row.get("sourceSHA256") == expected_sources[row["key"]] and row["error"] is None for row in after), "verified original FLAC did not decode and advance in native media", {"before": before, "after": after})
        events = self.js("window.__nfsMediaProbe.events")
        require(any(event["type"] == "playing" and event["trusted"] for event in events), "no trusted native playing event for original engine", events)
        self.record("audio-idle-proof-" + vehicle, {"before": before, "after": after, "events": events})
        self.check("original FLAC native decode and advancing playback " + vehicle)
        self.browser("find", "role", "button", "click", "--name", "开启托管", "--exact")
        self.wait(lambda: self.room()["seats"][0].get("autoPlay"), description="audio telemetry autoplay")
        observation = self.native("audio-load-and-shift-" + vehicle, mode="audio", milliseconds=16000)
        rows = observation["rows"]
        audible = [row for row in rows if any(not voice["paused"] and voice["readyState"] >= 3 and voice["volume"] > .001 for voice in row["voices"])]
        require(len(audible) >= len(rows) * .9, "following the autoplay car stopped its engine audio", {"rows": len(rows), "audibleRows": len(audible)})
        rates = {}
        for row in rows:
            for voice in row["voices"]:
                if not voice["paused"] and voice["volume"] > .002:
                    rates.setdefault(voice["key"], []).append(voice["rate"])
        require(any(max(values) - min(values) > .04 for values in rates.values()), "RPM/load changes did not tune native engine rate", rates)
        frames = [frame for frame in observation["states"] if frame["cars"]]
        shifts = []
        for index, frame in enumerate(frames[1:], 1):
            previous = frames[index - 1]["cars"][0]
            current = frame["cars"][0]
            if current.get("gear", 0) > previous.get("gear", 0) >= 1:
                upcoming = frames[index:index + 5]
                rpm_after = min(row["cars"][0]["rpm"] for row in upcoming)
                nearby_before = [row for row in rows if frame["at"] - 250 <= row["at"] < frame["at"]]
                nearby_after = [row for row in rows if frame["at"] <= row["at"] <= frame["at"] + 500]
                rate_drop = max((a["rate"] - b["rate"] for left in nearby_before for right in nearby_after for a in left["voices"] for b in right["voices"] if a["key"] == b["key"] and a["volume"] > .002 and b["volume"] > .002), default=0)
                shifts.append({"at": frame["at"], "from": previous["gear"], "to": current["gear"], "rpmBefore": previous["rpm"], "rpmAfter": rpm_after, "nativeRateDrop": rate_drop})
        self.record("audio-shift-proof-" + vehicle, {"shifts": shifts, "rateRanges": {key: [min(value), max(value)] for key, value in rates.items()}})
        require(any(shift["rpmAfter"] < shift["rpmBefore"] * .97 and shift["nativeRateDrop"] > .03 for shift in shifts), "no authoritative upshift RPM drop and corresponding native audio-rate drop", shifts)
        self.check("original native engine follows RPM/load and upshift " + vehicle, shifts=shifts)
        self.browser("find", "role", "button", "click", "--name", "取消托管", "--exact")
        self.wait(lambda: not self.room()["seats"][0].get("autoPlay"), description="audio manual reclaim")
        self.set_sound(False)
        self.wait(lambda: not any(not voice["paused"] for voice in self.engine_media()), description="native engine mute")
        self.record("audio-muted-" + vehicle, self.engine_media())
        self.check("public mute stops original engine channels " + vehicle)
        self.set_sound(True)
        self.browser("scrollintoview", ".racing-chase")
        self.browser("focus", ".racing-chase")
        self.native("audio-resume-" + vehicle, [{"key": "ArrowLeft", "down": True}, {"wait": 40}, {"key": "ArrowLeft", "down": False}])
        self.wait(lambda: any(not row["paused"] and row["readyState"] >= 3 for row in self.engine_media()), 10, "current engine resumes after unmute")
        blurred = self.native("audio-blur-cleanup-" + vehicle, [{"blur": True, "milliseconds": 350}])
        sample = next(row["data"] for row in blurred["samples"] if row["name"] == "blurred")
        require(not sample["focus"] and not any(not voice["paused"] for voice in sample["audio"] if voice["key"].startswith("racing-engine-")), "blurred room kept an original engine playing", sample)
        self.check("actual focus loss stops original engine channels " + vehicle, visibility=sample["visibility"])
        self.browser("focus", ".racing-chase")
        self.native("audio-fresh-gesture-" + vehicle, [{"key": "ArrowLeft", "down": True}, {"wait": 40}, {"key": "ArrowLeft", "down": False}])
        self.wait(lambda: any(not row["paused"] and row["readyState"] >= 3 for row in self.engine_media()), 10, "fresh gesture resumes current engine")
        self.js("window.__nfsMediaProbe.stop()")

    def complete_race(self):
        self.browser("set", "viewport", "1280", "1000")
        self.create_ui("france", vehicle="p911", players=6)
        self.enter("france", vehicle="p911")
        self.browser("find", "role", "button", "click", "--name", "开启托管", "--exact")
        self.wait(lambda: self.room()["seats"][0].get("autoPlay"), description="explicit autoplay")
        self.performance("desktop-six-car-performance")
        self.browser("focus", ".racing-chase")
        self.native("moving-mirror-on", [{"key": "F7", "down": True}, {"key": "F7", "down": False}, {"wait": 200}])
        self.performance("desktop-six-car-mirror-performance", mirror=True)
        self.performance("touch-six-car-mirror-performance", touch=True, mirror=True)
        self.browser("focus", ".racing-chase")
        self.native("moving-mirror-off", [{"key": "F7", "down": True}, {"key": "F7", "down": False}, {"wait": 200}], width=320, height=844, touch=True)
        self.performance("touch-six-car-performance", touch=True)
        self.browser("find", "role", "button", "click", "--name", "取消托管", "--exact")
        self.wait(lambda: not self.room()["seats"][0].get("autoPlay"), description="manual reclaim")
        self.check("manual reclaim preserves the running race")
        self.browser("find", "role", "button", "click", "--name", "开启托管", "--exact")
        started = self.room()["game"]["data"]
        self.wait(lambda: self.room()["status"] == "finished", 900, "one real-clock six-car complete race")
        final = self.room()["game"]["data"]
        require(final["epoch"] == started["epoch"] and final["serverAt"] < final["maxRaceAt"], "race used a restart/absolute timeout")
        require(all(c["finishAt"] > 0 and c["lap"] == 1 and c.get("lastReset", 0) == 0 for c in final["cars"]), "real-page computers did not naturally complete", final["cars"])
        self.record("real-complete-race", {"initial": started, "final": final})
        self.screenshot("real-complete-race")
        self.check("real-clock six-car one-lap race completes naturally")
        self.close_room()

    def network_and_pixels(self):
        self.browser("set", "viewport", "1280", "1000")
        # Keep both cars at rest for causal front-wheel pixel measurement.
        # Six-car moving load is separately checked in the complete-race gate.
        self.create_ui("france", vehicle="p911", players=2, bots=0)
        self.browser("open", self.base, name="peer")
        self.session("peer")["opened"] = True
        self.browser("set", "viewport", "1280", "1000", name="peer")
        self.api("/register", {"name": "慢连接车手" + self.run[-4:], "password": "racing-browser-test-only", "invite": "racing-browser"}, name="peer")
        self.browser("reload", name="peer")
        r = self.room()
        self.api("/rooms/" + r["id"], {"type": "join", "version": r["version"], "nonce": uuid.uuid4().hex}, name="peer")
        self.browser("wait", '.racing-preparation select[aria-label="车型"]', name="peer")
        self.browser("select", '.racing-preparation select[aria-label="车型"]', "fm50", name="peer")
        self.wait(lambda: self.room("peer")["options"].get("vehicles", [])[1] == "fm50", description="peer vehicle")
        self.native("peer-delay-setup", [{"headers": {"X-Acceptance-RTT": "150"}}, {"reload": True}, {"wait": 2000}], session="peer")
        self.command("ready", name="peer")
        self.enter("france", vehicle="p911")
        self.wait(lambda: self.js('document.querySelector(".racing-chase")?.dataset.classic==="3"&&!document.querySelector("[data-game-loading]")', "peer"), 90, "slow peer scene")
        self.wait(lambda: ((self.api("/state", name="peer").get("network") or {}).get("rttMs") or 0) >= 110, 20, "measured delayed WebSocket RTT")
        slow = self.api("/state", name="peer")["network"]
        fast = self.api("/state")["network"]
        require(fast["rttMs"] < 80 and slow["rttMs"] >= 110, "TCP delay did not isolate the slow peer", {"fast": fast, "slow": slow})
        self.record("paired-rtt", {"fast": fast, "slow": slow})
        for name, session in (("normal", "main"), ("rtt150", "peer")):
            self.browser("scrollintoview", ".racing-chase", name=session)
            self.browser("focus", ".racing-chase", name=session)
            value = self.native("pixel-response-" + name, mode="pixels", session=session, trials=8)
            m = value["metrics"]
            require(value["rectSource"] == "renderer-projected", "remake pixel acceptance cannot use a guessed/manual crop")
            require(m["visibleTrials"] == m["trials"] == 8 and m["p95MS"] <= BUDGET["inputPixelP95MS"], "native input-to-pixel budget failed", m)
            self.check("native input to real vehicle pixels " + name, metrics=m, connectionRTT=fast if session == "main" else slow, ack=value["ackMetrics"])
        self.browser("find", "role", "button", "click", "--name", "开启托管", "--exact")
        self.wait(lambda: self.room()["seats"][0].get("autoPlay"), description="healthy peer autoplay")
        healthy = self.performance("healthy-peer-alongside-rtt150")
        require(healthy["after"]["room"]["game"]["data"]["serverAt"] - healthy["before"]["room"]["game"]["data"]["serverAt"] >= 5000, "slow peer stalled authority")
        before = self.room("peer")["game"]["data"]
        value = self.native("reconnect-releases-stale-input", [{"key": "w", "down": True}, {"key": "h", "down": True}, {"wait": 150}, {"disconnect": True}, {"wait": 2500}, {"read": "reconnected"},
            {"key": "w", "down": True, "repeat": True}, {"key": "h", "down": True, "repeat": True}, {"wait": 180}, {"read": "stale-repeat"}, {"key": "w", "down": False}, {"key": "h", "down": False}], session="peer")
        samples = {r["name"]: r["data"] for r in value["samples"]}
        after = samples["reconnected"]["room"]["game"]["data"]
        require(after["epoch"] == before["epoch"] and after["maxRaceAt"] == before["maxRaceAt"], "reconnection restarted the race")
        require(samples["stale-repeat"]["controls"]["throttle"] == "0", "replayed held key resumed acceleration after reconnect", samples["stale-repeat"]["controls"])
        require(samples["stale-repeat"]["controls"].get("horn") == "false" and after["cars"][samples["reconnected"]["room"]["you"]].get("horn", False) is False, "reconnection retained or replayed held horn", samples["stale-repeat"]["controls"])
        self.check("actual socket interruption preserves race and releases held input")
        self.close_room()
        if self.room("peer"):
            self.command("leave", name="peer")

    def finish(self):
        for name, session in self.sessions.items():
            if session["opened"]:
                try:
                    self.browser("close", name=name)
                except Exception as error:
                    self.report.setdefault("cleanupErrors", []).append(str(error))
        if self.server:
            (self.out / "stop").touch()
            try:
                self.server.wait(timeout=8)
            except subprocess.TimeoutExpired:
                os.killpg(self.server.pid, signal.SIGTERM)
                self.server.wait(timeout=5)
        if self.log:
            self.log.close()
        self.record("report", self.report)
        # The report and screenshots are retained; only this run's disposable
        # profile/caches/socket directory is removed after its browser closes.
        if not self.report.get("cleanupErrors"):
            shutil.rmtree(self.temp, ignore_errors=True)
            shutil.rmtree(Path(self.env["AGENT_BROWSER_SOCKET_DIR"]), ignore_errors=True)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--out", required=True)
    parser.add_argument("--phase", choices=("contract", "controls", "display", "presenter", "expiry", "audio-diagnostic", "safety", "thumbs", "thumb-layouts", "fullscreen", "smoke", "full", "remaining", "experience", "vehicles", "latency"), default="smoke")
    parser.add_argument("--base", help="reuse an explicitly owned local Racing fixture URL")
    args = parser.parse_args()
    job = Acceptance(args)
    try:
        job.start_fixture()
        if args.phase == "safety":
            job.safety_controls()
        elif args.phase in ("thumbs", "thumb-layouts"):
            job.thumb_controls()
        elif args.phase == "fullscreen":
            job.fullscreen_validation()
        elif args.phase in ("remaining", "experience", "vehicles"):
            required = {"revised-desktop-layout", "native C edges cycle camera and B release restores selected view", "native F3/F5/F7/F11/F12 edges change drawing, HUD, real mirror and viewport", "native H playback, authority hold/release and blur", "standard Gamepad API simulation uses production analog mapping, camera/horn and neutral disconnect lifecycle", "public keyboard and gamepad remapping saves and disables the old defaults", "revised-touch-layout"}
            job.reuse_report(".local/racing-nfs-browser/full-final-02/report.json", required)
            if args.phase in ("experience", "vehicles"):
                checks = job.reuse_report(".local/racing-nfs-browser/remaining-final-02/report.json")
                scenes = {(check["track"],check["night"]) for check in checks if check["name"] == "selected original geometry and byte loading"}
                require(scenes == {(track,night) for track in TRACKS for night in (False,True)}, "reused evidence must cover all four day and four night scenes")
            else:
                job.create_ui("france", vehicle="p911", players=6)
                job.enter("france", vehicle="p911")
                job.controls()
                job.close_room()
                job.smoke(skip_controls=True)
            if args.phase == "vehicles":
                job.reuse_report(".local/racing-nfs-browser/experience-final-02/report.json", {"rear button Space/Enter focus loss blocks repeat until fresh press", "readonly presentation and advancing native engine during autoplay", "readonly presentation and advancing native engine during spectator"})
            job.models_and_complete_race(skip_presentation=args.phase == "vehicles")
            job.network_and_pixels()
            job.horn_expiry()
        elif args.phase == "expiry":
            job.horn_expiry()
        elif args.phase == "audio-diagnostic":
            job.audio_csp_diagnostic()
        elif args.phase == "presenter":
            job.presentation_permissions()
        elif args.phase in ("controls", "display"):
            job.create_ui("france", vehicle="p911", players=2)
            job.enter("france", vehicle="p911")
            if args.phase == "controls":
                job.revised_controls_contract()
                job.controls()
            else:
                job.display_keys()
            job.close_room()
        elif args.phase != "latency":
            job.smoke()
        if args.phase == "full":
            job.models_and_complete_race()
        if args.phase in ("full", "latency"):
            job.network_and_pixels()
        if args.phase == "full":
            job.horn_expiry()
        job.report["passed"] = True
    except Exception as error:
        job.report["error"] = str(error)
        job.report["traceback"] = traceback.format_exc()
        print(traceback.format_exc(), flush=True)
        if job.sessions.get("main", {}).get("opened"):
            try:
                job.screenshot("failure")
            except Exception as capture_error:
                job.report["failureCaptureError"] = str(capture_error)
    finally:
        job.finish()
    return 0 if job.report["passed"] else 1


if __name__ == "__main__":
    raise SystemExit(main())
