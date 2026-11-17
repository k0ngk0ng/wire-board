import test from "node:test";
import { execFileSync } from "node:child_process";
import { fileURLToPath } from "node:url";

test("racing dynamics lint checks registered files and rejects invalid examples", () => {
  const root = fileURLToPath(new URL("../../../", import.meta.url));
  execFileSync(process.execPath, ["games/racing/build/lint.mjs", "--self-test"], { cwd: root, stdio: "pipe" });
});
