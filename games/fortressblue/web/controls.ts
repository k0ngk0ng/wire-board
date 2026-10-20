/** Native evidence confirms one charge unit per update and a Range tick cap.
 * The original update frequency is unconfirmed: 30 Hz is this project's clock. */
export const CHARGE_HZ = 30;
export type Charge = {
  start: number | null;
  token: string;
  source: string;
  maxTicks?: number;
};
export function chargePower(start: number, now: number, maxTicks = 45) {
  const ticks = Math.max(0, Math.floor(((now - start) * CHARGE_HZ) / 1000));
  return Math.floor(Math.min(1, ticks / Math.max(1, maxTicks)) * 100);
}
export function beginCharge(
  state: Charge,
  now: number,
  token: string,
  source: string,
  maxTicks = 45,
) {
  if (state.start !== null) return false;
  state.start = now;
  state.token = token;
  state.source = source;
  state.maxTicks = Math.max(1, maxTicks);
  return true;
}
export function cancelCharge(state: Charge) {
  state.start = null;
  state.token = "";
  state.source = "";
}
export function releaseCharge(
  state: Charge,
  now: number,
  token: string,
  source: string,
) {
  if (state.start === null || state.source !== source) return null;
  const power =
    state.token === token
      ? chargePower(state.start, now, state.maxTicks)
      : null;
  cancelCharge(state);
  return power;
}
export function finishFullCharge(state: Charge, now: number, token: string) {
  if (state.start === null) return null;
  if (state.token !== token) {
    cancelCharge(state);
    return null;
  }
  if (chargePower(state.start, now, state.maxTicks) < 100) return null;
  return releaseCharge(state, now, token, state.source);
}
export function acceptsGameKeys(
  target: {
    tagName?: string;
    isContentEditable?: boolean;
    closest?: (selector: string) => unknown;
  } | null,
) {
  if (!target) return true;
  return (
    !target.isContentEditable &&
    !["INPUT", "TEXTAREA", "SELECT", "BUTTON"].includes(target.tagName ?? "") &&
    !target.closest?.('[role="dialog"], [contenteditable="true"]')
  );
}
