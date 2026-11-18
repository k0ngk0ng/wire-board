// Test-only independent authority rendering. This never modifies the Board,
// controls, rules or transport; it verifies prediction has converged in pixels.
import { SceneRenderer, type SceneFrame } from "../web/scene-renderer";
const renderer = new SceneRenderer("/pal/v1");
export async function authorityPixelsMatch(frame: SceneFrame) {
  const reference = document.createElement("canvas");
  await renderer.paint(reference, frame);
  const actual = document.querySelector<HTMLCanvasElement>(
    "[data-pal-board] canvas",
  );
  if (!actual) return false;
  const a = actual.getContext("2d")!.getImageData(0, 0, 320, 200).data;
  const b = reference.getContext("2d")!.getImageData(0, 0, 320, 200).data;
  return a.every((value, i) => value === b[i]);
}

// A real Board must match the wave-bearing authority and differ from the same
// current map/actors rendered without that original wave stage.
export async function authorityWaveVisible(frame: SceneFrame) {
  if (!frame.field || !frame.draws.some((d) => d.kind === 2 && d.a === 2))
    return { matches: false, differsFromUnwaved: false };
  const reference = document.createElement("canvas");
  await renderer.paint(reference, frame);
  const expected = reference
    .getContext("2d")!
    .getImageData(0, 0, 320, 200).data;
  const actual = document.querySelector<HTMLCanvasElement>(
    "[data-pal-board] canvas",
  );
  if (!actual) return { matches: false, differsFromUnwaved: false };
  const shown = actual.getContext("2d")!.getImageData(0, 0, 320, 200).data;
  await renderer.paint(reference, {
    ...frame,
    draws: frame.draws.filter((d) => d.kind !== 2),
  });
  const plain = reference.getContext("2d")!.getImageData(0, 0, 320, 200).data;
  return {
    matches: expected.every((value, i) => value === shown[i]),
    differsFromUnwaved: expected.some((value, i) => value !== plain[i]),
  };
}
