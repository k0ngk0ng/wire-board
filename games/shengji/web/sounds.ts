import type { PokerEvent } from "../../../web/src/poker/audio-events";
const voice = (name: string) => `/poker/v3/audio/${name}.mp3`;
export function soundFor(e: PokerEvent): string | undefined {
  if (e.type === "deal") return "/poker/v1/audio/card-shuffle.ogg";
  if (e.type === "declare") return "/poker/v1/audio/card-fan-1.ogg";
  if (e.type === "bury") return "/poker/v1/audio/card-slide-1.ogg";
  if (e.type !== "play") return;
  if (e.kind === "single" || e.kind === "pair") {
    const b = (e.cards?.[0] ?? 0) % 54;
    const rank = e.value || (b >= 52 ? b - 36 : (b % 13) + 2);
    return voice(`${e.kind}-${rank}`);
  }
  const kinds: Record<string, string> = {
    tractor: "tractor",
    throw: "throw",
    four_suits: "four-suits",
    follow: "follow",
  };
  return voice(kinds[e.kind ?? ""] ?? "follow");
}
