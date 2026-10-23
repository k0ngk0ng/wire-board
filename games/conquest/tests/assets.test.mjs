// Wire the pre-implementation audio acceptance into the existing per-game CI.
import test from "node:test";
import { execFileSync } from "node:child_process";
import { fileURLToPath } from "node:url";

test("conquest original audio passes its repeatable contract", () => {
  const root = fileURLToPath(new URL("../../../", import.meta.url));
  execFileSync(
    "python3",
    ["-B", "games/conquest/tests/audio_contract_test.py"],
    {
      cwd: root,
      stdio: "pipe",
    },
  );
});
