import assert from "node:assert/strict";
import { spawn, execFile } from "node:child_process";
import { promisify } from "node:util";
import { mkdir, writeFile, access } from "node:fs/promises";
import { fileURLToPath } from "node:url";
import path from "node:path";

const root = fileURLToPath(new URL("../../../", import.meta.url));
assert.ok(process.argv[2], "Provide the prepared asset directory");
const out = path.join(root, ".local/yuyuhakusho/recovery-loop/results");
const temporary = path.join(
  root,
  ".local/yuyuhakusho/recovery-loop/fixture-temporary",
);
await mkdir(out, { recursive: true });
const report = { passed: false, exitCode: null, signal: null, childPIDs: [] };
const child = spawn(
  process.execPath,
  ["games/yuyuhakusho/tests/recovery-server.mjs", process.argv[2]],
  { cwd: root, stdio: ["ignore", "pipe", "pipe"] },
);
let output = "";
child.stdout.on("data", (data) => {
  output += data;
});
child.stderr.on("data", (data) => {
  output += data;
});
const ended = new Promise((resolve) =>
  child.once("exit", (code, signal) => {
    report.exitCode = code;
    report.signal = signal;
    resolve();
  }),
);
const deadline = Date.now() + 90000;
try {
  while (!/^http:\/\/127\.0\.0\.1:\d+\//m.test(output)) {
    if (
      report.exitCode !== null ||
      report.signal !== null ||
      Date.now() > deadline
    )
      throw Error("Fixture startup failed: " + output);
    await new Promise((resolve) => setTimeout(resolve, 100));
  }
  const { stdout } = await promisify(execFile)(
    "ps",
    ["-axo", "pid,ppid,command"],
    { cwd: root },
  );
  report.childPIDs = stdout.split("\n").flatMap((line) => {
    const row = /^\s*(\d+)\s+(\d+)\s+(.+)$/.exec(line);
    return row &&
      Number(row[2]) === child.pid &&
      row[3] === path.join(temporary, "fixture")
      ? [Number(row[1])]
      : [];
  });
  assert.equal(report.childPIDs.length, 1, "Fixture did not own one backend");
  child.kill("SIGTERM");
  await Promise.race([
    ended,
    new Promise((resolve, reject) => {
      const timer = setTimeout(
        () => reject(Error("Fixture shutdown timed out")),
        15000,
      );
      timer.unref();
    }),
  ]);
  assert.equal(
    report.exitCode,
    0,
    "Vite exited before fixture cleanup completed",
  );
  await assert.rejects(
    access(temporary),
    (error) => error.code === "ENOENT",
    "Fixture left its database or cache behind",
  );
  for (const pid of report.childPIDs)
    assert.throws(
      () => process.kill(pid, 0),
      (error) => error.code === "ESRCH",
      "Fixture backend survived shutdown",
    );
  report.passed = true;
} catch (error) {
  report.error = String(error);
  process.exitCode = 1;
} finally {
  if (report.exitCode === null && report.signal === null) child.kill("SIGKILL");
  for (const pid of report.childPIDs) {
    try {
      process.kill(pid, "SIGTERM");
    } catch {}
  }
  report.output = output;
  await writeFile(
    path.join(out, "lifecycle-" + (process.env.RUN ?? "run") + ".json"),
    JSON.stringify(report, null, 2) + "\n",
  );
  console.log(JSON.stringify(report));
}
