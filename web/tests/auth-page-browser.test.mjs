import assert from "node:assert/strict";
import test from "node:test";
import { execFile } from "node:child_process";
import { promisify } from "node:util";
import { createRequire } from "node:module";
import { mkdir, readFile, rm, writeFile } from "node:fs/promises";
import { fileURLToPath } from "node:url";
import path from "node:path";

test(
  "login reference layout and credential flows work on desktop and phone",
  { skip: process.env.AUTH_PAGE_BROWSER !== "1", timeout: 180000 },
  async () => {
    const root = fileURLToPath(new URL("../../", import.meta.url));
    const output = path.join(root, ".local/_common/login/verification");
    const temporary = path.join(output, "browser-temporary");
    const environment = {
      ...process.env,
      TMPDIR: path.join(temporary, "tmp"),
      AGENT_BROWSER_PROFILE: path.join(temporary, "profile"),
      AGENT_BROWSER_SOCKET_DIR: path.join(root, ".local/_common/login/s"),
      AGENT_BROWSER_NO_VERSION_CHECK: "1",
      AGENT_BROWSER_SCREENSHOT_DIR: output,
    };
    for (const directory of [
      output,
      temporary,
      environment.TMPDIR,
      environment.AGENT_BROWSER_PROFILE,
      environment.AGENT_BROWSER_SOCKET_DIR,
    ])
      await mkdir(directory, { recursive: true });
    const require = createRequire(new URL("../package.json", import.meta.url));
    const execute = promisify(execFile);
    const session = (
      await execute(
        "agent-browser",
        ["session", "id", "--scope", "worktree", "--prefix", "au"],
        { cwd: root, env: environment },
      )
    ).stdout.trim();
    const browser = async (...args) => {
      const response = await execute(
        "agent-browser",
        ["--session", session, ...args, "--json"],
        { cwd: root, env: environment, timeout: 30000, maxBuffer: 4 << 20 },
      );
      const value = JSON.parse(response.stdout);
      assert.equal(value.success, true, response.stdout);
      return value.data;
    };
    const evaluate = async (expression) =>
      (await browser("eval", expression)).result;
    const fixture = path.join(temporary, "fixture.tsx");
    await writeFile(
      fixture,
      `import {createRoot} from "react-dom/client";import {Auth} from "/src/ui/auth";import "/src/style.css";const q=new URLSearchParams(location.search);createRoot(document.getElementById("root")!).render(<Auth busy={q.has("busy")} error={q.has("error")?"密码错误":""} invited={q.has("invited")} submit={(body,register)=>{document.getElementById("submission")!.textContent=JSON.stringify({body,register})}}/>);`,
    );
    const html =
      '<!doctype html><html lang="zh-CN"><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>登录页验收</title><style>body{margin:0}</style><div id="root"></div><output id="submission" hidden></output><script type="module" src="/@fs/' +
      fixture +
      '"></script></html>';
    const publication = JSON.parse(
      await readFile(
        path.join(root, ".local/_common/login/publication.json"),
        "utf8",
      ),
    );
    const hero = publication.files.find((row) => row.mime === "image/webp").url;
    const server = await require("vite").createServer({
      configFile: false,
      define: {
        "import.meta.env.VITE_CDN_ORIGIN": JSON.stringify(new URL(hero).origin),
      },
      root: path.join(root, "web"),
      cacheDir: path.join(temporary, "vite"),
      publicDir: false,
      esbuild: { jsx: "automatic" },
      optimizeDeps: {
        noDiscovery: true,
        include: [
          "react",
          "react-dom",
          "react-dom/client",
          "react/jsx-runtime",
          "react/jsx-dev-runtime",
          "lucide-react",
        ],
      },
      resolve: { dedupe: ["react", "react-dom", "lucide-react"] },
      server: { host: "127.0.0.1", port: 0, hmr: false, fs: { allow: [root] } },
      plugins: [
        {
          name: "auth-verification",
          configureServer(server) {
            server.middlewares.use((request, response, next) => {
              if (request.url.startsWith("/_common/login/hero.webp")) {
                response.writeHead(302, { Location: hero });
                response.end();
              } else if (request.url.startsWith("/?") || request.url === "/") {
                response.setHeader("Content-Type", "text/html; charset=utf-8");
                response.end(html);
              } else next();
            });
          },
        },
      ],
    });
    const report = {
      passed: false,
      scope:
        "Actual Auth component with a local submission recorder and verified CDN image; no production account mutations",
      checks: [],
    };
    try {
      await server.listen();
      const base = "http://127.0.0.1:" + server.httpServer.address().port;
      await browser("set", "viewport", "1440", "900");
      await browser("open", base);
      await browser(
        "wait",
        "--fn",
        'document.querySelector(".login-hero img")?.complete && document.querySelector(".login-hero img")?.naturalWidth===700',
      );
      assert.equal(
        await evaluate(
          'document.querySelector(".login-hero img").getAttribute("src")',
        ),
        hero,
      );
      await browser("snapshot", "-i");
      await browser("find", "label", "玩家昵称", "fill", "layout-player");
      await browser("find", "label", "密码", "fill", "local-test-password");
      await browser(
        "find",
        "role",
        "button",
        "click",
        "--name",
        "进入游戏大厅",
      );
      assert.deepEqual(
        JSON.parse(
          await evaluate('document.getElementById("submission").textContent'),
        ),
        {
          body: { name: "layout-player", password: "local-test-password" },
          register: false,
        },
      );
      report.checks.push("login fields submit unchanged");
      await browser("screenshot", path.join(output, "desktop.png"));
      await browser("find", "role", "button", "click", "--name", "创建账号");
      await browser("snapshot", "-i");
      await browser("find", "label", "确认密码", "fill", "different-password");
      await browser("find", "label", "邀请码", "fill", "local-invite");
      await browser(
        "find",
        "role",
        "button",
        "click",
        "--name",
        "创建账号，入座",
      );
      assert.match(
        await evaluate('document.querySelector("[role=alert]").textContent'),
        /两次输入的密码不一致/,
      );
      assert.equal(
        JSON.parse(
          await evaluate('document.getElementById("submission").textContent'),
        ).register,
        false,
      );
      await browser("find", "label", "确认密码", "fill", "local-test-password");
      await browser(
        "find",
        "role",
        "button",
        "click",
        "--name",
        "创建账号，入座",
      );
      assert.deepEqual(
        JSON.parse(
          await evaluate('document.getElementById("submission").textContent'),
        ),
        {
          body: {
            name: "layout-player",
            password: "local-test-password",
            invite: "local-invite",
          },
          register: true,
        },
      );
      report.checks.push(
        "registration validates confirmation and retains invite",
      );
      await browser("set", "viewport", "320", "800");
      const phone = await evaluate(
        '({width:innerWidth,scrollWidth:document.documentElement.scrollWidth,controls:[...document.querySelectorAll(".login-form input,.login-form button")].map(e=>({height:Math.round(e.getBoundingClientRect().height),width:Math.round(e.getBoundingClientRect().width)})),release:document.querySelector("[data-release-entry]")?.getAttribute("href")})',
      );
      assert.ok(phone.scrollWidth <= phone.width);
      assert.ok(
        phone.controls.every(
          (control) => control.height >= 44 && control.width >= 44,
        ),
      );
      assert.equal(phone.release, "/releases");
      report.phone = phone;
      await browser(
        "screenshot",
        "--full",
        path.join(output, "phone-registration.png"),
      );
      await browser("open", base + "/?invited&error&busy");
      await browser("wait", "--text", "登录后将自动进入邀请的房间");
      assert.equal(
        await evaluate('document.querySelector(".login-submit").disabled'),
        true,
      );
      assert.equal(
        await evaluate('document.querySelector("[role=alert]").textContent'),
        "密码错误",
      );
      await browser(
        "screenshot",
        "--full",
        path.join(output, "phone-pending-error.png"),
      );
      report.checks.push(
        "320px controls, invitation, pending state, accessible error and release entry",
      );
      const contrast = await evaluate(`(() => {
        const linear = value => { const channel = value / 255; return channel <= 0.04045 ? channel / 12.92 : ((channel + 0.055) / 1.055) ** 2.4; };
        const luminance = color => { const channels = color.match(/[0-9.]+/g).slice(0, 3).map(Number).map(linear); return channels[0] * 0.2126 + channels[1] * 0.7152 + channels[2] * 0.0722; };
        const ratio = (foreground, background) => { const a = luminance(foreground), b = luminance(background); return Math.round((Math.max(a, b) + 0.05) / (Math.min(a, b) + 0.05) * 100); };
        const page = getComputedStyle(document.querySelector('.login-page')).backgroundColor;
        return ['.login-subtitle', '.login-field', '.login-footer', '.login-footer a', '.login-error', '.login-submit', '.login-input input'].map(selector => { const node = document.querySelector(selector), style = getComputedStyle(node); return { selector, ratio100: ratio(style.color, ['.login-submit', '.login-input input'].includes(selector) ? style.backgroundColor : page) }; });
      })()`);
      assert.ok(
        contrast.every((row) => row.ratio100 >= 450),
        JSON.stringify(contrast),
      );
      report.contrast = contrast;
      const beforeFailure = await evaluate(
        'Math.round(document.querySelector(".login-hero").getBoundingClientRect().height)',
      );
      await evaluate(
        'document.querySelector(".login-hero img").src="/missing-login-artwork-test.webp"',
      );
      await browser(
        "wait",
        "--fn",
        'document.querySelector(".login-hero img").complete',
      );
      assert.equal(
        await evaluate(
          'Math.round(document.querySelector(".login-hero").getBoundingClientRect().height)',
        ),
        beforeFailure,
      );
      report.checks.push(
        "computed text contrast and stable missing-image layout with platform CSS",
      );
      report.passed = true;
    } catch (error) {
      report.error = String(error);
      throw error;
    } finally {
      await writeFile(
        path.join(output, "browser-result.json"),
        JSON.stringify(report, null, 2) + "\n",
      );
      await browser("close").catch(() => {});
      await server.close();
      await rm(temporary, { recursive: true, force: true });
    }
  },
);
