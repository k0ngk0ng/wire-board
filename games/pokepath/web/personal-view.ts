import type { View } from "./types.ts";

export function personalView(value: View, actor: number): View {
  const trainer = value.trainers?.[actor];
  if (!trainer || trainer.actor !== actor) return value;
  return { ...value, ...trainer, battleTeam: value.battleTeam ?? value.team };
}
