import { isMeleeWeapon } from "./content.ts";
import type { Player } from "./types.ts";

export type HudPlayer = Pick<Player, "weapon" | "ammo" | "alive" | "downUntil" | "protectedUntil">;
export type BattleHint = { kind: "empty" | "unarmed" | "status" | "controls"; text: string };

export function ammoLabel(weapon: string, ammo: number) {
  return isMeleeWeapon(weapon) ? "近战" : `${ammo} 发`;
}

export function battleHint(player: HudPlayer | undefined, frame: number, finished: boolean): BattleHint {
  if (finished) return { kind: "status", text: "对局结束" };
  if (!player) return { kind: "status", text: "观战中" };
  if (!player.alive) return { kind: "status", text: "复活时恢复起始装备" };
  if (player.downUntil > frame) return { kind: "status", text: "倒地中" };
  if (player.protectedUntil > frame) return { kind: "status", text: "出生保护 · 攻击后解除" };
  if (!player.weapon) return { kind: "unarmed", text: "徒手：近身击倒，或拾取武器" };
  if (!isMeleeWeapon(player.weapon) && player.ammo === 0) return { kind: "empty", text: "弹药用尽：投掷空枪，拾取其他武器" };
  return { kind: "controls", text: "" };
}
