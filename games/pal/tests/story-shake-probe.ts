import { SceneRenderer, type SceneFrame } from "../web/scene-renderer";
import { visualPublication } from "../web/visual-assets";

type View = SceneFrame & { scene: number; controlAt: number };
const renderer = new SceneRenderer("/unavailable/pal/v1", {
  publication: visualPublication(),
});
const logical = document.createElement("canvas");
const zero = document.createElement("canvas");
let active = false;
let busy = false;
let generation = 0;
let tag = "";
const seen = new Set<string>();
export const results: Record<string, unknown>[] = [];
export const diagnostics = { attempted: 0, mismatched: 0, errors: [] as string[] };

export function currentView(): View | undefined {
  const board = document.querySelector("[data-pal-board]");
  if (!board) return;
  const key = Object.keys(board).find((key) => key.startsWith("__reactFiber"));
  for (let fiber = (board as any)[key!]; fiber; fiber = fiber.return) {
    for (let hook = fiber.memoizedState; hook && hook.next !== undefined; hook = hook.next) {
      const value = hook.memoizedState?.current?.data;
      if (value?.party && value?.controlAt) return value;
    }
  }
}

export function describe(frame: View) {
  const graph = frame.raster;
  if (!graph) return;
  const fill = graph.nodes[graph.root - 1];
  if (fill?.[0] !== 4 || fill[5] !== 0 || fill[7] !== 320 || fill[9] !== 0) return;
  const copy = graph.nodes[fill[1] - 1];
  if (copy?.[0] !== 8 || copy[3] !== 320 || copy[4] !== 200 || copy[5] !== 0 || copy[7] !== 320 || copy[9] !== 0 || copy[11] !== 320) return;
  const shift = copy[10] - copy[6];
  const level = Math.max(copy[6], copy[10]);
  if (!shift || level >= 200 || copy[8] !== 200-level || copy[12] !== 200-level || Math.min(copy[6], copy[10]) !== 0 || fill[8] !== level || fill[6] !== (shift < 0 ? 200-level : 0)) return;
  return { shift, level, logicalRoot: copy[1] };
}

export async function prime() {
  const frame = currentView();
  if (!frame) throw Error("Actual platform PAL view is unavailable");
  await renderer.prepare({ ...frame, draws: frame.draws ?? [] });
}

export function start(label: string) {
  generation++;
  tag = label;
  seen.clear();
  results.length = 0;
  diagnostics.attempted = diagnostics.mismatched = 0;
  diagnostics.errors.length = 0;
  active = true;
}

export function stop() {
  active = false;
  generation++;
}

async function inspect() {
  const frame = currentView();
  if (!frame) return;
  const display = describe(frame);
  if (!display || results.filter((row) => row.shift === display.shift).length >= 4) return;
  const key = `${frame.controlAt}:${display.shift}`;
  if (seen.has(key)) return;
  seen.add(key);
  const canvas = document.querySelector<HTMLCanvasElement>("[data-pal-board] canvas");
  if (!canvas || canvas.width !== 320 || canvas.height !== 200) return;
  const actual = canvas.getContext("2d")!.getImageData(0, 0, 320, 200);
  const rect = canvas.getBoundingClientRect();
  const run = generation;
  diagnostics.attempted++;
  const graph = frame.raster!;
  await renderer.paint(logical, { ...frame, draws: frame.draws ?? [], raster: { ...graph, root: display.logicalRoot } });
  await renderer.paint(zero, { ...frame, draws: [], raster: { version: 1, root: 1, nodes: [[0, 0, 0, 320, 200, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0]] } });
  const plain = logical.getContext("2d")!.getImageData(0, 0, 320, 200).data;
  const expected = zero.getContext("2d")!.getImageData(0, 0, 320, 200).data;
  const rowBytes = 320 * 4;
  for (let y = 0; y < 200; y++) {
    const sourceY = y - display.shift;
    if (sourceY >= 0 && sourceY < 200)
      expected.set(plain.subarray(sourceY * rowBytes, (sourceY + 1) * rowBytes), y * rowBytes);
  }
  if (run !== generation || !active) return;
  if (!expected.every((value, index) => value === actual.data[index])) {
    diagnostics.mismatched++;
    return;
  }
  let changedPixels = 0;
  for (let i = 0; i < plain.length; i += 4)
    if (plain[i] !== expected[i] || plain[i + 1] !== expected[i + 1] || plain[i + 2] !== expected[i + 2]) changedPixels++;
  if (changedPixels < 3200) throw Error("Actual story shake did not change enough original scene pixels");
  const captured = document.createElement("canvas");
  captured.width = 320;
  captured.height = 200;
  captured.getContext("2d")!.putImageData(actual, 0, 0);
  const digest = await crypto.subtle.digest("SHA-256", actual.data);
  results.push({ label: tag, scene: frame.scene, controlAt: frame.controlAt, ...display, changedPixels, pixelSHA256: [...new Uint8Array(digest)].map((value) => value.toString(16).padStart(2, "0")).join(""), canvas: [rect.width, rect.height], viewport: [innerWidth, innerHeight], canvasPNG: captured.toDataURL("image/png"), view: frame });
}

function tick() {
  if (active && !busy) {
    busy = true;
    void inspect().catch((error) => diagnostics.errors.push(String(error))).finally(() => { busy = false; });
  }
  requestAnimationFrame(tick);
}
requestAnimationFrame(tick);
