import { SPRINT_TARGET, type Options } from "./types";

export function timeText(ms: number): string {
  const seconds = Math.max(0, Math.floor(ms / 1000));
  return `${String(Math.floor(seconds / 60)).padStart(2, "0")}:${String(
    seconds % 60,
  ).padStart(2, "0")}`;
}

export function progressText(mode: Options["mode"], lines: number): string {
  return mode === "sprint"
    ? `${Math.min(lines, SPRINT_TARGET)} / ${SPRINT_TARGET} 行`
    : `${lines} 行`;
}

export function prizeText(mode: Options["mode"]): string {
  return mode === "sprint" ? "先清 40 行" : "最后存活";
}
