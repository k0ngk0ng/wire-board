import type { Impact } from "./types";
export function soundForImpact(
  event: Impact,
  stroke: number,
): string | undefined {
  if (event.kind === "ball" || event.kind === "cushion") {
    const speed = event.speed ?? 0,
      tier = speed < 0.35 ? "soft" : speed < 1 ? "medium" : "hard";
    return `${event.kind}-${tier}-${Math.abs(stroke) % 3}`;
  }
  return (
    { cue: "cue", pot: "pocket", off: "pocket" } as Record<string, string>
  )[event.kind];
}
