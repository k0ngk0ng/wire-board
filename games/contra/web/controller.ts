/** Use /controllers profiles["classic-2"] and the shared FCController faceplate.
 * Keyboard bindings, pointer ownership and Turbo sampling belong to the platform.
 * This adapter only translates its sampled logical names to Contra's latch bits.
 */
export const CONTRA_CONTROLLER_MODEL = "classic-2";

const logicalBits = new Map<string, number>([
  ["right", 1], ["left", 2], ["down", 4], ["up", 8],
  ["start", 16], ["select", 32], ["b", 64], ["a", 128],
]);

export function contraControllerMask(buttons: Iterable<string>): number {
  let mask = 0;
  for (const button of buttons) mask |= logicalBits.get(button) ?? 0;
  return mask;
}
