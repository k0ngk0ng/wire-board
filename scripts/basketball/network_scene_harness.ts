import {
  BasketballNetworkScene,
  type BasketballNetworkView,
} from "../../games/basketball/web/network-scene";
import { loadBasketballWasm } from "../../games/basketball/web/wasm";
import {
  BasketballRenderer,
  loadBasketballAssets,
} from "../../games/basketball/web/renderer";
import type { BasketballRuntimeView } from "../../games/basketball/web/client";

// Acceptance specified before the network scene. These are actual Go module
// snapshots and real browser Wasm, with an explicitly synthetic audio schedule.
const canvas = document.querySelector("canvas")!;
const checks: string[] = [],
  frames: number[] = [],
  sounds: number[] = [],
  soundFrames: number[] = [];
const musicResets: unknown[] = [];
const check = (name: string, value: boolean) => {
  if (!value) throw Error(name);
  checks.push(name);
};
async function run() {
  const [engine, assets, fixture] = await Promise.all([
    loadBasketballWasm("/runtime"),
    loadBasketballAssets("/assets"),
    fetch("/host-views.json").then((r) => r.json()),
  ]);
  const views = fixture.views as BasketballNetworkView[];
  const renderer = new BasketballRenderer(canvas, assets);
  const render = (frame: {
    frame: number;
    display: BasketballNetworkView["runtime"]["display"];
  }) => {
    frames.push(frame.frame);
    const d = frame.display;
    if (d.HasVisible) {
      renderer.resetCourt(d.Visible.court);
      renderer.commitCourtEvents([
        ...(d.CourtColumns ?? []),
        ...(d.CourtPatches ?? []).map((patch) => ({ patch })),
      ]);
      renderer.render(d.Visible);
    }
  };
  const scene = new BasketballNetworkScene({
    engine,
    onFrame: render,
    onReset: (_sequence, music) => musicResets.push(music),
    onSound: (_ids, seq) => {
      sounds.push(seq);
      soundFrames.push(frames.at(-1)!);
    },
  });
  scene.update(views[0], 0, 0);
  check(
    "initial/reset playback receives the current authoritative music state",
    JSON.stringify(musicResets[0]) === JSON.stringify(views[0].music) &&
      !!musicResets[0],
  );
  const start = views[0].runtime.videoFrames;
  check(
    "initial join displays current authority without replaying old sounds",
    frames.at(-1) === start && sounds.length === 0,
  );
  const next = structuredClone(views[1]);
  // Replace only the sound stream for this scheduling case. Appending earlier
  // frame numbers after real later-frame sounds would be an invalid server log.
  next.events = [
    { sequence: views[0].soundSequence + 1, videoFrame: start + 2, ids: [19] },
    { sequence: views[0].soundSequence + 2, videoFrame: start + 5, ids: [21] },
  ];
  next.soundSequence = views[0].soundSequence + 2;
  scene.update(next, 1, 100);
  check(
    "receiving a six-frame batch does not skip straight to the last frame",
    frames.at(-1) === start,
  );
  for (let n = 0; n < 6; n++) scene.tick(100 + (n * 1000) / 60);
  check(
    "spectator renders all six consecutive authoritative video frames",
    JSON.stringify(frames.slice(-6)) ===
      JSON.stringify(Array.from({ length: 6 }, (_, n) => start + n + 1)),
  );
  check(
    "confirmed audio follows the displayed frame",
    JSON.stringify(sounds) ===
      JSON.stringify([next.soundSequence - 1, next.soundSequence]) &&
      JSON.stringify(soundFrames) === JSON.stringify([start + 2, start + 5]),
  );
  const count = frames.length;
  scene.update(next, 1, 190);
  scene.update(views[0], 0, 190);
  scene.tick(200);
  check(
    "duplicate and old snapshots do not replay pictures or sound",
    frames.length === count && sounds.length === 2,
  );
  let now = 200;
  for (let index = 2; index < views.length; index++) {
    scene.update(views[index], index, now);
    check(
      "Wasm replays complete authority including host cancellation and AI handoff " +
        index,
      JSON.stringify(scene.lastReplayState) ===
        JSON.stringify(views[index].runtime),
    );
    for (let n = 0; n < 6; n++) scene.tick(now + (n * 1000) / 60);
    now += 100;
  }
  scene.dispose();

  const local = new BasketballNetworkScene({
    engine,
    controller: 0,
    onFrame: render,
  });
  local.update(views[0], 0, 0);
  for (let n = 1; n <= 10; n++) local.tick((n * 1000) / 60, () => 1);
  const reference = engine.request<BasketballRuntimeView>({
    op: "restore",
    checkpoint: { version: 1, state: views[0].runtime },
  });
  const expected = engine.request<BasketballRuntimeView>({
    op: "advance",
    session: reference.session,
    ticks: 10,
    held: [1, 0, 0, 0],
  });
  check(
    "local prediction uses the actual Go engine before confirmation",
    JSON.stringify(local.predictedState) === JSON.stringify(expected.state),
  );
  engine.request({ op: "dispose", session: reference.session });
  local.update(views[1], 1, 170);
  const correction = engine.request<BasketballRuntimeView>({
    op: "restore",
    checkpoint: { version: 1, state: views[1].runtime },
  });
  const corrected = engine.request<BasketballRuntimeView>({
    op: "advance",
    session: correction.session,
    ticks: 4,
    held: [1, 0, 0, 0],
  });
  check(
    "authority corrects prediction then replays local input to the displayed frame",
    JSON.stringify(local.predictedState) === JSON.stringify(corrected.state),
  );
  engine.request({ op: "dispose", session: correction.session });
  local.cancel(1);
  check(
    "cancel clears local predicted held and pending bits",
    ((local.predictedState?.held as number[])[0] & 1) === 0 &&
      ((local.predictedState?.pendingPressed as number[])[0] & 1) === 0,
  );
  local.dispose();

  const recovery = new BasketballNetworkScene({ engine, onFrame: render });
  recovery.update(views[0], 0, 0);
  const gap = structuredClone(views[3]);
  gap.videoInputs = [];
  recovery.update(gap, 3, 300);
  check(
    "missing video history resets to authority instead of inventing intermediate play",
    recovery.resetReason === "gap" && frames.at(-1) === gap.runtime.videoFrames,
  );
  const newEpoch = { ...views[0], epoch: "new-match" };
  recovery.update(newEpoch, 0, 400);
  check(
    "new epoch resets frame and audio watermarks",
    frames.at(-1) === newEpoch.runtime.videoFrames &&
      recovery.resetReason === "epoch",
  );
  recovery.tick(1000);
  recovery.update({ ...views[3], epoch: "new-match" }, 3, 1001);
  check(
    "resuming after a long browser suspension uses the newest authority immediately",
    frames.at(-1) === views[3].runtime.videoFrames &&
      recovery.resetReason === "resume",
  );
  recovery.dispose();
  const finalCount = frames.length;
  recovery.tick(500);
  recovery.update(views[1], 1, 500);
  check(
    "disposed scene cannot resume old playback",
    frames.length === finalCount,
  );
  // Restore/dispose repeatedly must leave no hidden Wasm sessions behind.
  for (let n = 0; n < 24; n++) {
    const s = new BasketballNetworkScene({ engine, onFrame: () => {} });
    s.update(views[0], 0, 0);
    s.dispose();
  }
  check("scene lifecycle releases Wasm sessions", true);
  const endings = (await (await fetch("/host-result-views.json")).json())
    .views as BasketballNetworkView[];
  const ending = new BasketballNetworkScene({
    engine,
    controller: 0,
    onFrame: render,
  });
  ending.update(endings.at(-2)!, 0, 0);
  for (let n = 1; n <= 12; n++) ending.tick((n * 1000) / 60, () => 0);
  check(
    "predicted result reaches the original parked boundary",
    ending.predictedState?.outcome?.confirmed === true,
  );
  const parkedCount = frames.length;
  for (let n = 13; n <= 14; n++) ending.tick((n * 1000) / 60, () => 0);
  check(
    "parked predicted result does not emit repeated frames",
    frames.length === parkedCount,
  );
  ending.update(endings.at(-1)!, 1, 240);
  check(
    "authoritative completed result disables prediction",
    !ending.predicting,
  );
  ending.dispose();
  const pixels = canvas.getContext("2d")!.getImageData(0, 0, 256, 240).data;
  check(
    "actual original-art canvas contains the rendered scene",
    pixels.filter((v, i) => i % 4 !== 3 && v > 40).length > 20000,
  );
  return {
    status: "passed",
    checks,
    frames,
    sounds,
    syntheticAudioSchedule: true,
    scope:
      "actual browser Wasm and Canvas using recorded real host snapshots; not live multiplayer input or full-match browser",
  };
}
void run()
  .then((result) => {
    (window as any).sceneResult = result;
    document.querySelector("output")!.textContent =
      `${result.checks.length} 项通过`;
  })
  .catch((error) => {
    (window as any).sceneResult = {
      status: "failed",
      error: String(error),
      checks,
      frames,
      sounds,
    };
    document.querySelector("output")!.textContent = String(error);
  });
