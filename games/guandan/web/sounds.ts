import type { PokerEvent } from "../../../web/src/poker/audio-events";
const voice = (name: string) => `/poker/v3/audio/${name}.mp3`;
export function soundFor(e: PokerEvent): string | undefined {
  if (e.type === "deal") return "/poker/v1/audio/card-shuffle.ogg";
  const actions: Record<string, string> = {
    pass: "pass",
    tribute: "tribute",
    return: "return",
    anti_tribute: "anti-tribute",
    partner_lead: "partner-lead",
  };
  if (actions[e.type]) return voice(actions[e.type]);
  if (e.type !== "play") return;
  if (e.kind === "single" || e.kind === "pair") {
    const b = (e.cards?.[0] ?? 0) % 54;
    const rank = e.value || (b >= 52 ? b - 36 : (b % 13) + 2);
    return voice(`${e.kind}-${rank}`);
  }
  const kinds: Record<string, string> = {
    triple: "triple",
    full_house: "trio-pair",
    straight: "straight",
    tube: "tube",
    plate: "plate",
    straight_flush: "straight-flush",
    bomb: "bomb",
    rocket: "rocket",
  };
  return kinds[e.kind ?? ""]
    ? voice(kinds[e.kind!])
    : "/poker/v1/audio/card-place-1.ogg";
}
