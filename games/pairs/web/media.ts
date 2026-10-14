import type { GameAudioPort } from "../../../web/src/game-api/module";
export const localAsset = (path: string) => `/pairs/v1/${path}`;
export const asset = (base: string, path: string) =>
  `${base.replace(/\/$/, "")}${localAsset(path)}`;
export const sounds = {
  select: "sel.wav",
  match: "elec.wav",
  start: "start.wav",
  finish: "end.wav",
  hammer: "itemboom.wav",
  shuffle: "itemboom.wav",
  hint: "sel.wav",
} as const;
export function play(
  audio: GameAudioPort,
  base: string,
  kind: keyof typeof sounds,
) {
  const file = `audio/${sounds[kind]}`;
  return audio.playWithFallback([asset(base, file), localAsset(file)]);
}
