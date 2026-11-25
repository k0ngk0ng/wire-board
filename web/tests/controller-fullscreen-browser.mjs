import assert from "node:assert/strict";
import { mkdir, readFile, writeFile } from "node:fs/promises";
import { createHash } from "node:crypto";
import { fileURLToPath, pathToFileURL } from "node:url";
import path from "node:path";

const root = fileURLToPath(new URL("../../", import.meta.url));
const run = process.env.CONTROLLER_FULLSCREEN_RUN ?? "verification";
assert.match(run, /^[a-z0-9]+(?:-[a-z0-9]+)*$/);
const out = path.join(root, ".local/yuyuhakusho/fullscreen-layout", run);
await mkdir(out, { recursive: true });
const base = process.env.CONTROLLER_ROOM_URL;
assert.match(base ?? "", /^http:\/\/127\.0\.0\.1:\d+\/?$/);
const { webkit } = await import(
  process.env.CONTROLLER_BROWSER_MODULE
    ? pathToFileURL(process.env.CONTROLLER_BROWSER_MODULE).href
    : "playwright"
);
const browser = await webkit.launch({ headless: true });
const context = await browser.newContext({
  viewport: { width: 1174, height: 744 },
  deviceScaleFactor: 1,
  isMobile: true,
  hasTouch: true,
});
const page = await context.newPage();
await page.addInitScript(() => {
  const reject = async () => {
    throw Error("Page fullscreen regression acceptance");
  };
  HTMLElement.prototype.requestFullscreen = reject;
  HTMLElement.prototype.webkitRequestFullscreen = reject;
});
const report = {
  passed: false,
  engine: "WebKit",
  sourceStyles: process.env.CONTROLLER_SOURCE_STYLES === "1",
  scope:
    "Real rooms, compiled app and CDN runtimes; simulated mobile viewports",
  cases: [],
  errors: [],
};
page.on("pageerror", (error) => report.errors.push(String(error)));
const api = (route, body) =>
  page.evaluate(
    async ({ route, body }) => {
      const response = await fetch("/api" + route, {
        method: body ? "POST" : "GET",
        headers: { "Content-Type": "application/json" },
        body: body ? JSON.stringify(body) : undefined,
      });
      const value = await response.json();
      if (!response.ok) throw Error(JSON.stringify(value));
      return value;
    },
    { route, body },
  );
const command = async (type) => {
  const room = (await api("/state")).room;
  return api("/rooms/" + room.id, {
    type,
    version: room.version,
    nonce: crypto.randomUUID(),
  });
};
const settle = () =>
  page.evaluate(
    () =>
      new Promise((resolve) =>
        requestAnimationFrame(() =>
          requestAnimationFrame(() => requestAnimationFrame(resolve)),
        ),
      ),
  );
const measure = () =>
  page.evaluate(() => {
    const stage = document.querySelector(".touch-controller-stage");
    const overlay = stage.querySelector(".touch-controller-overlay");
    const controller = overlay.querySelector("[data-controller-layout]");
    const boxes = [
      stage,
      overlay,
      controller,
      controller.querySelector(".md-faceplate,.fc-faceplate"),
    ].map((element) => element.getBoundingClientRect().toJSON());
    const buttons = [
      ...overlay.querySelectorAll("button[data-controller-button]"),
    ].map((element) => {
      const box = element.getBoundingClientRect();
      const hit = document.elementFromPoint(
        box.x + box.width / 2,
        box.y + box.height / 2,
      );
      return {
        name: element.dataset.controllerButton,
        box: box.toJSON(),
        hit: hit === element || element.contains(hit),
      };
    });
    return {
      boxes,
      buttons,
      pad: overlay
        .querySelector("[data-controller-dpad]")
        .getBoundingClientRect()
        .toJSON(),
      viewport: {
        width: visualViewport.width,
        height: visualViewport.height,
        top: visualViewport.offsetTop,
        scrollY,
      },
      sameCanvas:
        stage.querySelector("canvas") === window.fullscreenRegressionCanvas,
      bodyFixed: getComputedStyle(document.body).position === "fixed",
    };
  });
const verify = (geometry, fullscreen) => {
  const [stage, ...layers] = geometry.boxes;
  const px = (value) => Math.round(value);
  for (const box of layers)
    for (const edge of ["top", "left", "bottom", "right"])
      assert.equal(
        px(box[edge]),
        px(stage[edge]),
        "controller layer " + edge + " differs from game stage",
      );
  assert.ok(geometry.sameCanvas, "fullscreen replaced the actual game canvas");
  assert.equal(geometry.bodyFixed, fullscreen);
  if (fullscreen) {
    assert.equal(px(stage.height), px(geometry.viewport.height));
    assert.equal(px(stage.top), 0);
  }
  if (!fullscreen) return;
  const main = geometry.buttons.filter((button) =>
    ["a", "b", "c"].includes(button.name),
  );
  const center = px(geometry.pad.top + geometry.pad.height / 2);
  assert.ok(
    center >= px(stage.top + stage.height / 2),
    "direction pad moved into the upper half",
  );
  for (const button of main)
    assert.equal(
      px(button.box.top + button.box.height / 2),
      center,
      "primary row is not aligned with direction pad",
    );
  for (const { name, box, hit } of geometry.buttons) {
    assert.ok(
      px(box.width) >= 44 && px(box.height) >= 44,
      name + " touch target is too small",
    );
    assert.ok(
      px(box.top) >= px(stage.top) && px(box.bottom) <= px(stage.bottom),
      name + " is clipped vertically",
    );
    assert.ok(
      px(box.left) >= px(stage.left) && px(box.right) <= px(stage.right),
      name + " is clipped horizontally",
    );
    if (fullscreen) assert.ok(hit, name + " is covered at its center");
  }
};
const capture = async (kind, label, fullscreen) => {
  await settle();
  const geometry = await measure();
  const entry = { kind, label, fullscreen, geometry, passed: false };
  report.cases.push(entry);
  await page.screenshot({ path: path.join(out, kind + "-" + label + ".png") });
  try {
    verify(geometry, fullscreen);
    entry.passed = true;
  } catch (error) {
    entry.error = error.message;
  }
  console.log(
    kind + " " + label + ": " + (entry.passed ? "passed" : entry.error),
  );
};
try {
  await page.goto(base);
  await api("/register", {
    name: "全屏验收" + Date.now().toString().slice(-8),
    password: "fullscreen-local-test-only",
    invite: "gamepad-browser",
  });
  await page.evaluate(() => localStorage.setItem("wb_sound", "off"));
  for (const kind of ["yuyuhakusho", "contra"]) {
    const old = (await api("/state")).room;
    if (old) {
      if (old.status === "playing") await command("close");
      await command("leave");
    }
    await api("/rooms", {
      kind,
      capacity: kind === "contra" ? 1 : 4,
      options:
        kind === "contra"
          ? {}
          : { arena: 2, characters: [3, 5, 3, 2], teamBattle: false },
      name: "控制器全屏验收",
    });
    if (kind === "yuyuhakusho")
      for (let count = 0; count < 3; count++) await command("add_bot");
    await page.reload();
    await page.getByRole("button", { name: "我准备好了", exact: true }).click();
    await page.getByRole("button", { name: "开始游戏", exact: true }).click();
    await page.waitForSelector("[data-controller-button=a]:not(:disabled)", {
      timeout: 90000,
    });
    if (report.sourceStyles)
      await page.addStyleTag({
        content: await readFile(
          path.join(root, "games/yuyuhakusho/web/network.css"),
          "utf8",
        ),
      });
    await page.evaluate(() => {
      window.fullscreenRegressionCanvas = document.querySelector(
        ".touch-controller-stage canvas",
      );
    });
    const roomID = (await api("/state")).room.id;
    for (const [width, height] of [
      [1174, 744],
      [844, 390],
      [678, 466],
      [390, 844],
      [1180, 820],
      [820, 1180],
    ]) {
      const label = width + "x" + height;
      await page.setViewportSize({ width, height });
      await settle();
      await page
        .getByRole("button", { name: "全屏", exact: true })
        .scrollIntoViewIfNeeded();
      const savedScroll = await page.evaluate(() => Math.round(scrollY));
      await page.getByRole("button", { name: "全屏", exact: true }).click();
      await page.waitForSelector("[data-room-fullscreen=page]");
      await capture(kind, label + "-enter", true);
      await page.setViewportSize({ width: height, height: width });
      await capture(kind, label + "-rotate", true);
      await page.setViewportSize({ width, height: height - 80 });
      await capture(kind, label + "-chrome", true);
      await page.setViewportSize({ width, height });
      await capture(kind, label + "-restore", true);
      await page.getByRole("button", { name: "退出全屏", exact: true }).click();
      await page.waitForSelector(
        "[data-game-fullscreen-surface]:not([data-room-fullscreen])",
      );
      await capture(kind, label + "-exit", false);
      assert.equal(
        await page.evaluate(() => Math.round(scrollY)),
        savedScroll,
        "exit did not restore page scroll",
      );
      assert.equal(
        (await api("/state")).room.id,
        roomID,
        "fullscreen changed the room",
      );
    }
  }
  report.passed =
    report.errors.length === 0 && report.cases.every((entry) => entry.passed);
  assert.ok(
    report.passed,
    report.cases.filter((entry) => !entry.passed).length +
      " fullscreen cases failed",
  );
} catch (error) {
  report.error = error.stack;
  process.exitCode = 1;
} finally {
  report.sources = Object.fromEntries(
    await Promise.all(
      [
        "games/yuyuhakusho/web/network.css",
        "web/src/controllers/pointer-controller.ts",
        "web/tests/controller-fullscreen-browser.mjs",
      ].map(async (name) => [
        name,
        createHash("sha256")
          .update(await readFile(path.join(root, name)))
          .digest("hex"),
      ]),
    ),
  );
  await writeFile(
    path.join(out, "result.json"),
    JSON.stringify(report, null, 2) + "\n",
  );
  await browser.close();
}
