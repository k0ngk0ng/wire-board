import type { SoundKey } from "./events";

export const SOUND_FILES: Record<SoundKey, string> = {
  select: "audio/sfx-select.mp3",
  countdown: "audio/sfx-countdown.mp3",
  lock: "audio/sfx-lock.mp3",
  crash: "audio/sfx-crash.mp3",
  chain: "audio/sfx-chain.mp3",
  power: "audio/sfx-power.mp3",
  diamond: "audio/sfx-diamond.mp3",
  garbage: "audio/sfx-garbage.mp3",
  topout: "audio/sfx-topout.mp3",
  win: "audio/sfx-win.mp3",
  lose: "audio/sfx-lose.mp3",
};

export const BGM_FILES = {
  select: "audio/bgm-select.mp3",
  battle: "audio/bgm-battle.mp3",
} as const;

export const soundURL = (base: string, file: string) =>
  `${base.replace(/\/$/, "")}/spf2t/v1/${file}`;

export const soundSources = (base: string, file: string) => {
  const local = `/spf2t/v1/${file}`;
  return base ? [soundURL(base, file), local] : [local];
};
