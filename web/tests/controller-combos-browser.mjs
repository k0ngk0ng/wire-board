import assert from "node:assert/strict";
import { promisify } from "node:util";
import { execFile } from "node:child_process";
import { mkdir, writeFile, readFile, rm } from "node:fs/promises";
import { createRequire } from "node:module";
import path from "node:path";
import { fileURLToPath } from "node:url";

const root = fileURLToPath(new URL("../../", import.meta.url));
const directories = {
  md: path.join(root, ".local/yuyuhakusho/controller-combos/contact-neon"),
  fc: path.join(root, ".local/contra/controller-combos/contact-neon"),
};
const out = directories.md;
const temporary = path.join(out, "temporary");
const sockets = path.join(root, ".local/yuyuhakusho/s");
for (const folder of [
  out,
  temporary,
  sockets,
  ...["profile", "cache", "config", "data"].map((name) =>
    path.join(temporary, name),
  ),
])
  await mkdir(folder, { recursive: true });
const env = {
  ...process.env,
  TMPDIR: temporary,
  XDG_CONFIG_HOME: path.join(temporary, "config"),
  XDG_DATA_HOME: path.join(temporary, "data"),
  XDG_CACHE_HOME: path.join(temporary, "cache"),
  AGENT_BROWSER_PROFILE: path.join(temporary, "profile"),
  AGENT_BROWSER_SOCKET_DIR: sockets,
  AGENT_BROWSER_NO_VERSION_CHECK: "1",
};
const execute = promisify(execFile);
const session = (
  await execute(
    "agent-browser",
    ["session", "id", "--scope", "worktree", "--prefix", "cb"],
    { cwd: root, env },
  )
).stdout.trim();
const browser = async (...args) => {
  const { stdout } = await execute(
    "agent-browser",
    [
      "--session",
      session,
      "--profile",
      path.join(temporary, "profile"),
      ...args,
      "--json",
    ],
    { cwd: root, env, timeout: 30000 },
  );
  const value = JSON.parse(stdout);
  assert.ok(value.success, stdout);
  return value.data;
};
const require = createRequire(new URL("../package.json", import.meta.url));
const { createServer } = require("vite");
const react = require("@vitejs/plugin-react").default;
await writeFile(
  path.join(temporary, "index.html"),
  `<meta name="viewport" content="width=device-width,initial-scale=1"><style>*{box-sizing:border-box}body{margin:0}header button{height:44px}</style><div id="root"></div><script type="module" src="/@fs/${path.join(root, "web/tests/ControllerComboFixture.tsx")}"></script>`,
);
const server = await createServer({
  configFile: false,
  root: temporary,
  cacheDir: path.join(temporary, "vite-cache"),
  publicDir: false,
  optimizeDeps: {
    include: ["react", "react-dom/client", "react/jsx-dev-runtime"],
  },
  plugins: [react()],
  resolve: {
    alias: {
      react: path.join(root, "web/node_modules/react"),
      "react-dom": path.join(root, "web/node_modules/react-dom"),
      "lucide-react": path.join(root, "web/node_modules/lucide-react"),
    },
    dedupe: ["react", "react-dom"],
  },
  server: { host: "127.0.0.1", port: 0, fs: { allow: [root] } },
});
await server.listen();
const reports = [];
let socket;
try {
  for (const kind of ["fc", "md"]) {
    const directory = directories[kind];
    await mkdir(directory, { recursive: true });
    const report = {
      kind,
      passed: false,
      source:
        kind === "md"
          ? "MDController and MDInput"
          : "FCController and Contra RuntimeInput",
      input:
        "Native CDP Touch events, not a physical device or full rule match",
      checks: [],
      events: [],
    };
    reports.push(report);
    const url = server.resolvedUrls.local[0] + "?kind=" + kind;
    await browser("open", url);
    await browser("wait", "--fn", "window.controllerComboProbe?.ready===true");
    const endpoint = await browser("get", "cdp-url");
    socket = new WebSocket(
      typeof endpoint === "string" ? endpoint : endpoint.cdpUrl,
    );
    await new Promise((resolve, reject) => {
      socket.onopen = resolve;
      socket.onerror = reject;
    });
    let serial = 0;
    const pending = new Map();
    socket.onmessage = ({ data }) => {
      const value = JSON.parse(data);
      const request = pending.get(value.id);
      if (!request) return;
      pending.delete(value.id);
      value.error
        ? request.reject(Error(JSON.stringify(value.error)))
        : request.resolve(value.result);
    };
    const call = (method, params = {}, sessionId) =>
      new Promise((resolve, reject) => {
        const id = ++serial;
        pending.set(id, { resolve, reject });
        socket.send(JSON.stringify({ id, method, params, sessionId }));
      });
    const { targetInfos } = await call("Target.getTargets");
    const { sessionId } = await call("Target.attachToTarget", {
      targetId: targetInfos.find(
        (value) => value.type === "page" && value.url === url,
      ).targetId,
      flatten: true,
    });
    const evaluate = async (expression) => {
      const value = await call(
        "Runtime.evaluate",
        { expression, returnByValue: true, awaitPromise: true },
        sessionId,
      );
      assert.ok(
        !value.exceptionDetails,
        JSON.stringify(value.exceptionDetails),
      );
      return value.result.value;
    };
    const frames = () =>
      evaluate(
        "new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(resolve)))",
      );
    const bits =
      kind === "md"
        ? {
            a: 64,
            b: 16,
            c: 32,
            up: 1,
            down: 2,
            left: 4,
            right: 8,
            x: 1024,
            y: 512,
            z: 256,
            mode: 2048,
            start: 128,
          }
        : {
            a: 128,
            b: 64,
            up: 8,
            down: 4,
            left: 2,
            right: 1,
            select: 32,
            start: 16,
          };
    const mask = (names) =>
      names.reduce((value, name) => value | bits[name], 0);
    const check = (name, actual, expected) => {
      report.checks.push({ name, actual, expected });
      assert.deepEqual(actual, expected, kind + " " + name);
    };
    const rectangle = (selector) =>
      evaluate(
        `document.querySelector(${JSON.stringify(selector)}).getBoundingClientRect().toJSON()`,
      );
    const button = (name) =>
      rectangle('[data-controller-button="' + name + '"]');
    const point = (rect, id = 1) => ({
      id,
      x: rect.x + rect.width / 2,
      y: rect.y + rect.height / 2,
      radiusX: 3,
      radiusY: 3,
      force: 1,
    });
    const sharedContact = (first, second, id = 1) => ({
      id,
      x: (first.x + first.width / 2 + second.x + second.width / 2) / 2,
      y: first.y + first.height / 2,
      radiusX:
        Math.abs(first.x + first.width / 2 - second.x - second.width / 2) / 2 +
        Math.min(first.width, second.width) * 0.4,
      radiusY: Math.min(first.height, second.height) * 0.8,
      force: 1,
    });
    const touch = async (type, points, names, name) => {
      await call(
        "Input.dispatchTouchEvent",
        { type, touchPoints: points },
        sessionId,
      );
      await frames();
      const actual = await evaluate("window.controllerComboProbe.mask");
      report.events.push({ name, type, points, actual, expected: mask(names) });
      check(name, actual, mask(names));
    };
    const pressed = () =>
      evaluate(
        "[...document.querySelectorAll('button[data-controller-button][aria-pressed=true]')].map(element=>element.dataset.controllerButton).sort()",
      );
    await call(
      "Emulation.setTouchEmulationEnabled",
      { enabled: true, maxTouchPoints: 5 },
      sessionId,
    );
    if (kind === "md") {
      const lateStyles = await readFile(
        path.join(root, "games/yuyuhakusho/web/network.css"),
        "utf8",
      );
      await evaluate(
        `(()=>{const style=document.createElement('style');style.dataset.lateGameStyles='true';style.textContent=${JSON.stringify(lateStyles)};document.head.appendChild(style);return true})()`,
      );
    }
    for (const [width, height] of [
      [320, 760],
      [844, 390],
      [1180, 820],
    ]) {
      await call(
        "Emulation.setDeviceMetricsOverride",
        {
          width,
          height,
          screenWidth: width,
          screenHeight: height,
          deviceScaleFactor: 1,
          mobile: true,
          screenOrientation: {
            type: width > height ? "landscapePrimary" : "portraitPrimary",
            angle: width > height ? 90 : 0,
          },
        },
        sessionId,
      );
      await frames();
      const a = point(await button("a"));
      const picturePadding = await evaluate(
        "(()=>{const style=getComputedStyle(document.querySelector('.touch-controller-picture'));return Math.round((parseFloat(style.paddingLeft)+parseFloat(style.paddingRight))*1000)})()",
      );
      check("overlay leaves full width for game picture", picturePadding, 0);
      const b = point(await button("b"));
      const ab = sharedContact(await button("a"), await button("b"));
      check(
        "no extra combo buttons",
        await evaluate(
          "document.querySelectorAll('[data-controller-combo],[data-controller-bridge],[data-controller-button=ac],[data-controller-button=abc]').length",
        ),
        0,
      );
      const colors = await evaluate(
        "[...document.querySelectorAll('[data-controller-button=a],[data-controller-button=b]')].map(element=>getComputedStyle(element).getPropertyValue('--control-neon').trim())",
      );
      check(
        "separate neon colors",
        colors.length === 2 && colors.every(Boolean) && colors[0] !== colors[1],
        true,
      );
      await touch(
        "touchStart",
        [ab],
        ["a", "b"],
        width + " broad contact covers AB",
      );
      check(width + " AB highlights", await pressed(), ["a", "b"]);
      await browser("screenshot", path.join(directory, width + "-ab.png"));
      await touch("touchEnd", [ab], [], "AB release");
      await touch("touchStart", [a], ["a"], "A center");
      await touch("touchMove", [ab], ["a", "b"], "A to AB");
      await touch("touchMove", [b], ["b"], "AB to B releases A");
      if (kind === "md") {
        const c = point(await button("c"));
        const bc = sharedContact(await button("b"), await button("c"));
        await touch("touchMove", [bc], ["b", "c"], "B to BC");
        check("BC highlights", await pressed(), ["b", "c"]);
        await touch("touchMove", [c], ["c"], "BC to C releases B");
      }
      await touch("touchEnd", [], [], "single finger slide ends");
      const broadA = {
        ...ab,
        x: (a.x * 2 + b.x) / 3,
        radiusX: Math.abs(a.x - b.x) * 1.1,
      };
      await touch(
        "touchStart",
        [broadA],
        ["a", "b"],
        "broad contact centered inside A reaches B",
      );
      await touch(
        "touchMove",
        [a],
        ["a"],
        "same finger narrows to A and releases B",
      );
      await touch("touchEnd", [], [], "narrow contact ends");
      await touch(
        "touchStart",
        [{ ...ab, radiusX: 3, radiusY: 3 }],
        [],
        "tiny contact in gap does not claim both buttons",
      );
      await touch("touchCancel", [], [], "tiny gap contact ends");
      await touch(
        "touchStart",
        [{ ...ab, radiusX: 0, radiusY: 0 }],
        ["a", "b"],
        "unknown contact size uses shared adjacent zone",
      );
      await touch("touchCancel", [], [], "unknown size contact ends");
      const pad = await rectangle("[data-controller-dpad]");
      const direction = (x, y, id = 2) => ({
        id,
        x: pad.x + (pad.width * (x + 1)) / 2,
        y: pad.y + (pad.height * (y + 1)) / 2,
        radiusX: 3,
        radiusY: 3,
        force: 1,
      });
      const directions = [
        [0, 0.8, ["down"]],
        [0.7, 0.7, ["down", "right"]],
        [0.8, 0, ["right"]],
        [0.7, -0.7, ["up", "right"]],
        [0, -0.8, ["up"]],
        [-0.7, -0.7, ["up", "left"]],
        [-0.8, 0, ["left"]],
        [-0.7, 0.7, ["down", "left"]],
        [0, 0.8, ["down"]],
        [0, 0, []],
      ];
      for (let index = 0; index < directions.length; index++) {
        const [x, y, names] = directions[index];
        await touch(
          index === 0 ? "touchStart" : "touchMove",
          [direction(x, y)],
          names,
          "eight-way slide " + index,
        );
      }
      await touch("touchEnd", [], [], "direction release");
      for (const [x, names] of [
        [0.2199, []],
        [0.22, ["right"]],
        [0.2201, ["right"]],
      ]) {
        await touch(
          "touchStart",
          [direction(x, 0)],
          names,
          "neutral boundary " + x,
        );
        await touch("touchEnd", [], [], "neutral boundary release");
      }
      await touch(
        "touchStart",
        [direction(0.7, 0.7)],
        ["down", "right"],
        "diagonal blank corner direct start",
      );
      await browser(
        "screenshot",
        path.join(directory, width + "-diagonal.png"),
      );
      await touch(
        "touchMove",
        [{ ...a, id: 2 }],
        [],
        "direction finger cannot become attack",
      );
      await touch("touchEnd", [], [], "direction ownership end");
      await touch("touchStart", [a], ["a"], "attack ownership starts");
      await touch(
        "touchMove",
        [{ ...direction(-0.8, 0), id: 1 }],
        [],
        "attack finger cannot become direction",
      );
      await touch(
        "touchMove",
        [ab],
        ["a", "b"],
        "attack finger returns to bridge",
      );
      await touch("touchEnd", [], [], "attack ownership ends");
      for (const [x, y, names] of [
        [0.9, 0.4196, ["right"]],
        [0.9, 0.4197, ["down", "right"]],
        [0.4197, 0.9, ["down", "right"]],
        [0.4196, 0.9, ["down"]],
      ]) {
        await touch(
          "touchStart",
          [direction(x, y)],
          names,
          "RetroArch 80 boundary " + x + ":" + y,
        );
        await touch("touchEnd", [], [], "boundary release");
      }
      const left = direction(-0.8, 0);
      if (kind === "md") {
        const c = point(await button("c"));
        const bc = sharedContact(await button("b"), await button("c"));
        await touch("touchStart", [bc], ["b", "c"], "direct BC start");
        await touch("touchEnd", [], [], "direct BC ends");
        const x = point(await button("x"));
        const y = point(await button("y"));
        await touch(
          "touchStart",
          [
            {
              ...x,
              radiusX: (await button("a")).width,
              radiusY: Math.abs(a.y - x.y) + (await button("a")).height,
            },
          ],
          ["x"],
          "broad X contact does not claim the lower primary row",
        );
        await touch("touchCancel", [], [], "broad X contact ends");
        await touch(
          "touchStart",
          [{ ...x, x: (x.x + y.x) / 2, y: (x.y + y.y) / 2 }],
          [],
          "XYZ gap has no ABC combo",
        );
        await touch("touchEnd", [], [], "XYZ gap ends");
      }
      const auxiliary = point(await button(kind === "md" ? "mode" : "select"));
      await touch(
        "touchStart",
        [auxiliary],
        [kind === "md" ? "mode" : "select"],
        "auxiliary starts",
      );
      await touch(
        "touchMove",
        [a],
        [],
        "auxiliary finger cannot become attack",
      );
      await touch("touchEnd", [], [], "auxiliary ends");
      await touch(
        "touchStart",
        [left, ab],
        ["left", "a", "b"],
        "movement plus combo",
      );
      await touch(
        "touchEnd",
        [ab],
        ["left"],
        "combo finger release retains direction",
      );
      await touch("touchEnd", [left], [], "other finger release");
      const anotherA = { ...a, id: 4 };
      await touch(
        "touchStart",
        [ab, anotherA],
        ["a", "b"],
        "AB and separate A finger",
      );
      await touch("touchEnd", [ab], ["a"], "AB release retains separate A");
      await touch("touchCancel", [], [], "cancel last finger");
      await call(
        "Input.dispatchKeyEvent",
        {
          type: "keyDown",
          key: kind === "md" ? "j" : "k",
          code: kind === "md" ? "KeyJ" : "KeyK",
        },
        sessionId,
      );
      await touch("touchStart", [ab], ["a", "b"], "keyboard A plus touch AB");
      await touch(
        "touchEnd",
        [ab],
        ["a"],
        "touch release preserves keyboard A",
      );
      await call(
        "Input.dispatchKeyEvent",
        {
          type: "keyUp",
          key: kind === "md" ? "j" : "k",
          code: kind === "md" ? "KeyJ" : "KeyK",
        },
        sessionId,
      );
      await frames();
      for (const id of ["disable", "profile", "configure", "unmount"]) {
        await touch(
          "touchStart",
          [left, ab],
          ["left", "a", "b"],
          "hold before " + id,
        );
        await evaluate(
          `document.getElementById(${JSON.stringify(id)}).click();true`,
        );
        await frames();
        check(
          id + " releases sources",
          await evaluate("window.controllerComboProbe.mask"),
          0,
        );
        if (id === "configure")
          check(
            "configuration excludes combo fields",
            await evaluate(
              "document.querySelectorAll('[data-controller-combo]').length",
            ),
            0,
          );
        await touch("touchEnd", [], [], id + " old touch ends");
        if (id !== "profile")
          await evaluate(
            `document.getElementById(${JSON.stringify(id)}).click();true`,
          );
        await frames();
      }
      await touch("touchStart", [ab], ["a", "b"], "hold before blur");
      await evaluate("window.dispatchEvent(new Event('blur'));true");
      await frames();
      check(
        "blur releases combo",
        await evaluate("window.controllerComboProbe.mask"),
        0,
      );
      await touch("touchEnd", [], [], "blur touch ends");
      await touch(
        "touchStart",
        [ab],
        ["a", "b"],
        "hold before pointer mode change",
      );
      await call(
        "Emulation.setTouchEmulationEnabled",
        { enabled: false },
        sessionId,
      );
      await frames();
      check(
        "fine pointer releases sources",
        await evaluate("window.controllerComboProbe.mask"),
        0,
      );
      check(
        "desktop controls hidden",
        await evaluate(
          "getComputedStyle(document.querySelector('.touch-controller-overlay')).display",
        ),
        "none",
      );
      await call(
        "Emulation.setTouchEmulationEnabled",
        { enabled: true, maxTouchPoints: 5 },
        sessionId,
      );
      await frames();
      await call(
        "Input.dispatchTouchEvent",
        { type: "touchCancel", touchPoints: [] },
        sessionId,
      ).catch((error) => {
        if (!error.message.includes("Must send a TouchStart first"))
          throw error;
      });
      await touch("touchStart", [ab], ["a", "b"], "hold before rotation");
      await call(
        "Emulation.setDeviceMetricsOverride",
        {
          width: height,
          height: width,
          screenWidth: height,
          screenHeight: width,
          deviceScaleFactor: 1,
          mobile: true,
        },
        sessionId,
      );
      await frames();
      check(
        "rotation releases sources",
        await evaluate("window.controllerComboProbe.mask"),
        0,
      );
      await touch("touchEnd", [], [], "rotation touch ends");
      await call(
        "Emulation.setDeviceMetricsOverride",
        {
          width,
          height,
          screenWidth: width,
          screenHeight: height,
          deviceScaleFactor: 1,
          mobile: true,
        },
        sessionId,
      );
      await frames();
      await evaluate(
        `window.originalRequestFullscreen??=Element.prototype.requestFullscreen;Element.prototype.requestFullscreen=${width === 1180 ? '()=>Promise.reject(Error("explicit browser test fallback"))' : "window.originalRequestFullscreen"};true`,
      );
      await browser("click", "#fullscreen");
      await browser("wait", "[data-room-fullscreen]");
      await frames();
      check(
        "fullscreen mode",
        await evaluate(
          "document.querySelector('[data-room-fullscreen]').dataset.roomFullscreen",
        ),
        width === 1180 ? "page" : "native",
      );
      const fullscreenWidth = await evaluate(
        "(()=>{const game=document.querySelector('.touch-controller-stage').getBoundingClientRect();const controller=document.querySelector('[data-controller-layout]').getBoundingClientRect();return {game:Math.round(game.width*1000),controller:Math.round(controller.width*1000)}})()",
      );
      check(
        "late game CSS keeps fullscreen controller width",
        fullscreenWidth.controller,
        fullscreenWidth.game,
      );
      const fa = point(await button("a"));
      const fb = point(await button("b"));
      const fab = sharedContact(await button("a"), await button("b"));
      await touch("touchStart", [fab], ["a", "b"], "fullscreen AB hit");
      await browser(
        "screenshot",
        path.join(directory, width + "-fullscreen-ab.png"),
      );
      await touch("touchCancel", [], [], "fullscreen combo ends");
      await browser("click", 'button[aria-label="退出全屏"]');
      await frames();
    }
    const events = await evaluate("window.controllerComboProbe.events");
    check(
      "trusted native touch events",
      events.length > 0 && events.every((event) => event.trusted),
      true,
    );
    report.trustedEvents = events;
    report.passed = true;
    await writeFile(
      path.join(directory, "result.json"),
      JSON.stringify(report, null, 2) + "\n",
    );
    socket.close();
    socket = undefined;
    await browser("close");
  }
} finally {
  await writeFile(
    path.join(out, "all-games.json"),
    JSON.stringify(
      {
        passed:
          reports.length === 2 && reports.every((report) => report.passed),
        reports,
      },
      null,
      2,
    ) + "\n",
  );
  socket?.close();
  await browser("close").catch(() => {});
  await server.close();
  await rm(temporary, { recursive: true, force: true });
}
