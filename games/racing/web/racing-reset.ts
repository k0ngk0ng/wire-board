type ResetState = {
  active: boolean;
  canReset: boolean;
  physics?: number;
  phase: string;
  spectating: boolean;
  autoPlay: boolean;
  finished: boolean;
  busy: boolean;
};

export function racingResetSeconds(resetUntil: number, now: number, physics?: number): number {
  if (resetUntil <= 0) return 0;
  return Math.max(0, Math.ceil((resetUntil + (physics === 3 ? 0 : 3000) - now) / 1000));
}

/** Explain authority/ownership only; vehicle speed never gates a v3 request. */
export function racingResetFeedback(state: ResetState): string {
  if (state.spectating) return "观战时无法复位";
  if (state.finished || state.phase === "finished") return "已经完赛";
  if (state.autoPlay) return "恢复手动驾驶后可复位";
  if (state.phase === "countdown") return "发车后可复位";
  if (state.busy) return "操作正在提交，请稍候";
  if (!state.active) return "当前无法复位";
  if (!state.canReset)
    return state.physics === 3 ? "正在复位，请稍候" : "请减速或等待复位冷却";
  return "";
}
