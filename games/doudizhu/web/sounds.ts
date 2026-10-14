import type { PokerEvent } from "../../../web/src/poker/audio-events";
const voice = (name: string) => `/poker/v3/audio/${name}.mp3`;
export function soundFor(e: PokerEvent, huanle = true): string | undefined {
  if (e.type === "deal") return "/poker/v1/audio/card-shuffle.ogg";
  if (e.type === "bid")
    return voice(
      !e.value ? "bid-pass" : `${huanle ? "tier" : "bid"}-${e.value}`,
    );
  if (e.type === "pass") return voice("pass");
  if (e.type === "report")
    return voice(e.value === 2 ? "double-report" : "report");
  if (e.type === "open_hand") return voice("open-hand");
  if (e.type !== "play") return;
  if (e.kind === "single" || e.kind === "pair") {
    const b = (e.cards?.[0] ?? 0) % 54;
    const rank = e.value || (b >= 52 ? b - 36 : (b % 13) + 2);
    return voice(`${e.kind}-${rank}`);
  }
  const kinds: Record<string, string> = {
    triple: "triple",
    trio_single: "trio-single",
    trio_pair: "trio-pair",
    straight: "straight",
    pairs: "pairs",
    triples: "triples",
    plane_single: "plane-single",
    plane_pair: "plane-pair",
    four_single: "four-single",
    four_pair: "four-pair",
    bomb: "bomb",
    rocket: "rocket",
  };
  return kinds[e.kind ?? ""]
    ? voice(kinds[e.kind!])
    : "/poker/v1/audio/card-place-1.ogg";
}
