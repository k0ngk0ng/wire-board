// Failure inventory before implementation: runtime manifests use the platform
// image root; nested room routes alter local paths; Vite's commit root is lost;
// standalone esbuild has no import.meta.env and throws instead of using /.
import test from "node:test";
import assert from "node:assert/strict";
import { build } from "../../../web/node_modules/esbuild/lib/main.js";

const runtimeID = "a".repeat(64);
const commitBase = `https://cdn.ichenj.com/bga/web/${"b".repeat(40)}/`;

async function resolver(base) {
  const built = await build({
    entryPoints: ["games/contra/web/runtime-url.ts"],
    bundle: true,
    write: false,
    format: "esm",
    define:
      base === undefined
        ? {}
        : { "import.meta.env.BASE_URL": JSON.stringify(base) },
  });
  return (
    await import(
      "data:text/javascript;base64," +
        Buffer.from(built.outputFiles[0].text).toString("base64")
    )
  ).runtimeManifestURL;
}

test("production manifest follows the compiled commit asset root", async () => {
  const url = await resolver(commitBase);
  assert.equal(
    url(runtimeID, "https://bga.ichenj.com/rooms/example"),
    `${commitBase}contra/runtime-${runtimeID}/manifest.json`,
  );
});

test("local platform root ignores nested room paths", async () => {
  const url = await resolver("/");
  assert.equal(
    url(runtimeID, "http://127.0.0.1:8080/rooms/example?tab=game"),
    `http://127.0.0.1:8080/contra/runtime-${runtimeID}/manifest.json`,
  );
});

test("standalone esbuild without Vite environment uses the application root", async () => {
  const url = await resolver();
  assert.equal(
    url(runtimeID, "http://127.0.0.1:8080/private/fixture"),
    `http://127.0.0.1:8080/contra/runtime-${runtimeID}/manifest.json`,
  );
});
