import type { MotionPrediction } from "./prediction.ts";
import type { InputAction } from "./types.ts";
import { cameraForPlayer, pointerWorldPoint } from "./renderer.ts";

export function aimAngle(x: number, y: number) {
  return (Math.round(Math.atan2(y, x) * 1024 / (Math.PI * 2)) + 1024) % 1024;
}

export function preparePredictedInput(prediction: MotionPrediction, action: InputAction, at: number, pointer?: { x: number; y: number }, dimensions?: { width: number; height: number }) {
  prediction.input(action, at);
  if (!pointer || !dimensions) return;
  const shown = prediction.sample(at);
  if (!shown.alive) return;
  const point = pointerWorldPoint(pointer.x, pointer.y, cameraForPlayer(shown, dimensions.width, dimensions.height));
  action.aim = aimAngle(Math.round(point.x * 256) - shown.x, Math.round(point.y * 256) - shown.y);
  prediction.setAim(action.seq, action.aim);
}
