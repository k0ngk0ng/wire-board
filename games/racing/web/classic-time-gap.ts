export type GapCarInput = {
  seat: number;
  rank: number;
  gapMs?: number;
  gapLaps?: number;
  finishAt: number;
};

export type OpponentGapRow = {
  seat: number;
  rank: number;
  self: boolean;
  deltaMs: number;
  deltaLaps: number;
  leaderGapMs: number;
  leaderGapLaps: number;
  relativeText: string;
  leaderText: string;
};

function formatDurationMs(absMs: number): string {
  const clamped = Math.max(0, Math.round(Number.isFinite(absMs) ? absMs : 0));
  const totalHundredths = Math.round(clamped / 10);
  const hundredths = totalHundredths % 100;
  const totalSeconds = Math.floor(totalHundredths / 100);
  const seconds = totalSeconds % 60;
  const minutes = Math.floor(totalSeconds / 60);
  if (minutes > 0) {
    return `${minutes}:${String(seconds).padStart(2, "0")}.${String(hundredths).padStart(2, "0")}`;
  }
  return `${seconds}.${String(hundredths).padStart(2, "0")}s`;
}

export function formatLeaderGap(gapMs: number, gapLaps: number): string {
  const laps = Math.trunc(Number.isFinite(gapLaps) ? gapLaps : 0);
  if (laps >= 1) return `+${laps} 圈`;
  const ms = Math.max(0, Math.round(Number.isFinite(gapMs) ? gapMs : 0));
  if (ms === 0) return "领先";
  return `+${formatDurationMs(ms)}`;
}

export function formatGapDelta(deltaMs: number, deltaLaps: number): string {
  const laps = Math.trunc(Number.isFinite(deltaLaps) ? deltaLaps : 0);
  if (laps > 0) return `+${laps} 圈`;
  if (laps < 0) return `${laps} 圈`;
  const ms = Math.round(Number.isFinite(deltaMs) ? deltaMs : 0);
  if (ms === 0) return "—";
  return `${ms > 0 ? "+" : "-"}${formatDurationMs(Math.abs(ms))}`;
}

export function computeOpponentGaps(
  cars: readonly GapCarInput[],
  referenceSeat: number,
): OpponentGapRow[] {
  const ordered = [...cars].sort(
    (a, b) => a.rank - b.rank || a.seat - b.seat,
  );
  const leader = ordered[0];
  const reference =
    ordered.find((c) => c.seat === referenceSeat) ?? leader;
  const leaderGapOf = (car: GapCarInput) => {
    if (!leader || car.seat === leader.seat) return { ms: 0, laps: 0 };
    if (leader.finishAt > 0 && car.finishAt > 0) {
      return { ms: Math.max(0, Math.round(car.finishAt - leader.finishAt)), laps: 0 };
    }
    const laps = Math.max(0, Math.trunc(car.gapLaps ?? 0));
    const ms = Math.max(0, Math.round(car.gapMs ?? 0));
    return { ms, laps };
  };
  const refGap = reference ? leaderGapOf(reference) : { ms: 0, laps: 0 };
  return ordered.map((car) => {
    const gap = leaderGapOf(car);
    const self = !!reference && car.seat === reference.seat;
    let deltaMs = 0;
    let deltaLaps = 0;
    if (!self && reference) {
      if (car.finishAt > 0 && reference.finishAt > 0) {
        deltaMs = Math.round(car.finishAt - reference.finishAt);
      } else if (gap.laps !== refGap.laps) {
        deltaLaps = gap.laps - refGap.laps;
      } else {
        deltaMs = gap.ms - refGap.ms;
      }
    }
    return {
      seat: car.seat,
      rank: car.rank,
      self,
      deltaMs,
      deltaLaps,
      leaderGapMs: gap.ms,
      leaderGapLaps: gap.laps,
      relativeText: self ? "—" : formatGapDelta(deltaMs, deltaLaps),
      leaderText: formatLeaderGap(gap.ms, gap.laps),
    };
  });
}
