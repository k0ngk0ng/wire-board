import type { Options } from "./types";
export type Art = Record<string, HTMLImageElement>;
const pending = new Map<string, Promise<Art>>();
export function loadArt(
  scenario: Options["scenario"],
  players: number,
  progress: (n: number) => void,
) {
  const key = scenario + "/" + players;
  if (!pending.has(key))
    pending.set(
      key,
      (async () => {
        const names = [
          "characters",
          "devices",
          ...Array.from({ length: players }, (_, i) => "walk-" + i),
          ...(scenario === "station"
            ? ["station-atrium", "station-service"]
            : ["archive-hall", "archive-vault"]),
        ];
        let loaded = 0;
        const entries = await Promise.all(
          names.map(async (name) => {
            const image = new Image();
            image.decoding = "async";
            image.src = "/escape/v1/" + name + ".webp";
            await image.decode();
            if (!image.naturalWidth) throw Error("素材为空");
            progress((++loaded / names.length) * 100);
            return [name, image] as const;
          }),
        );
        return Object.fromEntries(entries);
      })().catch((e) => {
        pending.delete(key);
        throw e;
      }),
    );
  return pending.get(key)!;
}
export const deviceFrames: Record<string, number> = {
  power: 0,
  pump: 1,
  signal: 2,
  supply: 3,
  medical: 4,
  relay: 5,
  briefing: 6,
  "clue-power": 6,
  "clue-pump": 6,
  "clue-signal": 6,
  battery: 7,
  document: 10,
  extract: 14,
};
