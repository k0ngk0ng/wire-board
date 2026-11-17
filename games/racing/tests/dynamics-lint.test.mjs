import test from "node:test";
import { execFileSync } from "node:child_process";
import { fileURLToPath } from "node:url";
import { readFileSync } from "node:fs";
import assert from "node:assert/strict";

test("racing dynamics lint checks registered files and rejects invalid examples", () => {
  const root = fileURLToPath(new URL("../../../", import.meta.url));
  execFileSync(process.execPath, ["games/racing/build/lint.mjs", "--self-test"], { cwd: root, stdio: "pipe" });
  const report = JSON.parse(readFileSync(new URL("../../../.local/racing-dynamics/evidence/lint.json", import.meta.url), "utf8"));
  assert(report.languages.includes("python") && report.languages.includes("css"));
});
