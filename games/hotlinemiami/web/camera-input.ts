import type { MotionPrediction } from "./prediction.ts";
import type { InputAction, Viewport } from "./types.ts";
import { cameraForPlayer, pointerWorldPoint } from "./renderer.ts";

export function aimAngle(x: number, y: number) {
  return (Math.round(Math.atan2(y, x) * 1024 / (Math.PI * 2)) + 1024) % 1024;
}

export function preparePredictedInput(prediction: MotionPrediction, action: InputAction, at: number, pointer: { x: number; y: number } | undefined, dimensions: { width: number; height: number }, viewport: Viewport, project?: (logical: ReturnType<MotionPrediction["sample"]>, at: number) => { x: number; y: number }) {
  prediction.input(action, at);
  if (!pointer) return;
  const logical = prediction.sample(at);
  if (!logical.alive) return;
  const shown = project ? project(logical, at) : logical;
  const point = pointerWorldPoint(pointer.x, pointer.y, cameraForPlayer(shown, dimensions.width, dimensions.height, viewport));
  action.aim = aimAngle(Math.round(point.x * 256) - shown.x, Math.round(point.y * 256) - shown.y);
  prediction.setAim(action.seq, action.aim);
}
