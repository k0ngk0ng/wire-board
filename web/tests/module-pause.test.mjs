// Failure inventory before implementation: pause still announces a turn or a
// running clock; the shared turn shell/clock placeholder disappears; resume
// keeps the paused label. This targets actual React output, not source spelling.
import test from "node:test";
import assert from "node:assert/strict";
import { build } from "esbuild";
import { createRequire } from "node:module";
import { mkdirSync, writeFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import path from "node:path";
const root = fileURLToPath(new URL("../../", import.meta.url));
const out = path.join(root, ".local/module-pause");
mkdirSync(out, { recursive: true });
const result = await build({
  stdin: {
    contents: `
import React from 'react';
import {renderToStaticMarkup} from 'react-dom/server';
import {ModuleTurn} from './src/game-api/host.tsx';
export function render(room){return renderToStaticMarkup(React.createElement(ModuleTurn,{room,serverNow:1000,receivedAt:performance.now()}));}
`,
    resolveDir: path.join(root, "web"),
    sourcefile: "pause-fixture.tsx",
    loader: "tsx",
  },
  bundle: true,
  platform: "node",
  format: "cjs",
  jsx: "automatic",
  write: false,
  loader: { ".css": "empty" },
  plugins: [
    {
      name: "pause-fixture-imports",
      setup(b) {
        b.onResolve({ filter: /^\.\/registry$/ }, (args) =>
          args.importer.endsWith("/game-api/host.tsx")
            ? { path: "registry", namespace: "pause-fixture" }
            : null,
        );
        b.onLoad({ filter: /.*/, namespace: "pause-fixture" }, () => ({
          contents:
            'export const gameModule=()=>null;export const gameName=()=>"fixture";',
          loader: "js",
        }));
        b.onResolve({ filter: /^\.\.\/profiles$/ }, () => ({
          path: "profiles",
          namespace: "pause-profile",
        }));
        b.onLoad({ filter: /.*/, namespace: "pause-profile" }, () => ({
          contents: "export const PlayerName=({user})=>user.name;",
          loader: "js",
        }));
      },
    },
  ],
});
const compiled = path.join(out, "ui-fixture.cjs");
writeFileSync(compiled, result.outputFiles[0].contents);
const { render } = createRequire(import.meta.url)(compiled);
test("paused turn retains shell and silent clock placeholder, then resumes", () => {
  const room = {
    you: 0,
    turnDeadline: 91000,
    game: { finished: false, actors: [0, 1], attention: [0] },
  };
  const active = render(room);
  const paused = render({
    ...room,
    turnDeadline: 0,
    game: { ...room.game, paused: true, attention: [] },
  });
  assert.ok(active.includes("请行动"));
  assert.ok(paused.includes("对局已暂停"));
  assert.ok(!paused.includes("请行动"));
  assert.ok(paused.includes("turn-clock-placeholder"));
  assert.ok(paused.includes('aria-hidden="true"'));
  assert.ok(paused.includes('<section class="turn-card '));
  assert.equal(
    (active.match(/<h3>/g) || []).length,
    (paused.match(/<h3>/g) || []).length,
  );
  const resumed = render({ ...room, game: { ...room.game, paused: false } });
  assert.ok(resumed.includes("请行动"));
  assert.ok(!resumed.includes("对局已暂停"));
  // Even an inconsistent/stale deadline cannot expose a running pause clock.
  assert.ok(
    render({ ...room, game: { ...room.game, paused: true } }).includes(
      "turn-clock-placeholder",
    ),
  );
  writeFileSync(
    path.join(out, "ui-render.json"),
    JSON.stringify(
      {
        scope:
          "React server rendering; browser geometry covered by conquest E2E",
        passed: true,
        active,
        paused,
        resumed,
      },
      null,
      2,
    ) + "\n",
  );
});
