import type { Command, Plan } from "./types";
export type Owner = "player" | "pet";
export type ChoicePopup = "skills" | "magic" | "items" | null;
export type BattleInput = {
  window: number;
  owner: Owner;
  popup: ChoicePopup;
  pending: Command | null;
  player: Command | null;
  pet: Command | null;
  details: number | null;
};
export function initialInput(window: number): BattleInput {
  return {
    window,
    owner: "player",
    popup: null,
    pending: null,
    player: null,
    pet: null,
    details: null,
  };
}
export function inspectActor(input: BattleInput, id: number): BattleInput {
  return { ...input, details: input.details === id ? null : id };
}
export function chooseCommand(
  input: BattleInput,
  owner: Owner,
  command: Command,
): BattleInput {
  return { ...input, owner, popup: null, pending: command, details: null };
}
export function confirmTarget(
  input: BattleInput,
  target: number,
  allowed: boolean,
): BattleInput {
  if (!input.pending || !allowed) return input;
  const command = { ...input.pending, target };
  return {
    ...input,
    [input.owner]: command,
    pending: null,
    owner: "pet",
    popup: input.owner === "player" ? "skills" : null,
  };
}
export function cancelInput(input: BattleInput): BattleInput {
  return { ...input, pending: null, popup: null, details: null };
}
export function readyPlan(
  input: BattleInput,
  petAlive: boolean,
  petID: number,
): Plan | null {
  if (!input.player) return null;
  if (!petAlive)
    return {
      player: input.player,
      pet: { kind: "wait", id: 0, target: petID },
    };
  return input.pet ? { player: input.player, pet: input.pet } : null;
}
