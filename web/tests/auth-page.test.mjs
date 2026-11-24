import assert from "node:assert/strict";
import test from "node:test";
import { createRequire } from "node:module";
import { mkdir, readFile, rm } from "node:fs/promises";
import { fileURLToPath } from "node:url";
import path from "node:path";

const root = fileURLToPath(new URL("../../", import.meta.url));
const require = createRequire(new URL("../package.json", import.meta.url));

test("login artwork has one public resource identity and the platform uses the new page", async () => {
  const resources = JSON.parse(
    await readFile(path.join(root, "assets/resources.json"), "utf8"),
  );
  const external = JSON.parse(
    await readFile(path.join(root, "assets/external-resources.json"), "utf8"),
  );
  const record = resources.files["web/public/_common/login/hero.webp"];
  assert.equal(record.mime, "image/webp");
  assert.equal(record.key, `bga/_common/${record.sha256}.webp`);
  assert.deepEqual(external.files["/_common/login/hero.webp"], record);
  const main = await readFile(path.join(root, "web/src/main.tsx"), "utf8");
  assert.match(main, /import\s*\{\s*Auth\s*\}\s*from\s*["']\.\/ui\/auth["']/);
  assert.doesNotMatch(main, /function Auth\(/);
});

test("login preserves invitation, credentials, errors, pending state and release entry", async () => {
  const temporary = path.join(
    root,
    ".local/_common/login/verification/ssr-cache",
  );
  await mkdir(temporary, { recursive: true });
  const server = await require("vite").createServer({
    configFile: false,
    define: {
      "import.meta.env.VITE_CDN_ORIGIN": JSON.stringify(
        "https://assets.example.test",
      ),
    },
    root: path.join(root, "web"),
    cacheDir: temporary,
    publicDir: false,
    esbuild: { jsx: "automatic" },
    optimizeDeps: { noDiscovery: true },
    server: { middlewareMode: true, hmr: false, fs: { allow: [root] } },
  });
  try {
    const { Auth } = await server.ssrLoadModule("/src/ui/auth.tsx");
    const { createElement } = require("react");
    const { renderToStaticMarkup } = require("react-dom/server");
    const render = (props) =>
      renderToStaticMarkup(
        createElement(Auth, {
          busy: false,
          error: "",
          invited: false,
          submit: () => {},
          ...props,
        }),
      );
    const markup = render({});
    const external = JSON.parse(
      await readFile(path.join(root, "assets/external-resources.json"), "utf8"),
    ).files;
    assert.ok(
      markup.includes(
        `src="https://assets.example.test/${external["/_common/login/hero.webp"].key}"`,
      ),
    );
    assert.match(markup, /name="name"/);
    assert.match(markup, /name="password"/);
    assert.match(markup, /autoComplete="current-password"/i);
    assert.match(markup, /href="\/releases"/);
    assert.doesNotMatch(markup, /https:\/\/.*(?:fonts|feishu)/);
    assert.match(render({ invited: true }), /登录后将自动进入邀请的房间/);
    assert.match(render({ error: "密码错误" }), /role="alert"[^>]*>密码错误/);
    assert.match(render({ busy: true }), /disabled=""/);
    assert.match(render({ busy: true }), /请稍候/);
  } finally {
    await server.close();
    await rm(temporary, { recursive: true, force: true });
  }
});
