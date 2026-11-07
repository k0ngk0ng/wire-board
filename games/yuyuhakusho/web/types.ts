import type { GameClock } from "../../../web/src/game-api/module";
export type GameOptions = {
  arena: number;
  teamBattle: boolean;
  characters: number[];
};
export type PlayerSummary = {
  seat: number;
  slot: number;
  characterID: number;
  computer: boolean;
  active: boolean;
  health: number;
  energy: number;
  x: number;
  y: number;
};
export type WireInput = {
  sequence: number;
  appliedSequence: number;
  buttons: number;
  appliedButtons: number;
  automated: boolean;
};
export type InputWindow = {
  version: 1;
  firstFrame: string;
  frameCount: number;
  data: string;
};
export type NetworkView = {
  protocol: 1;
  epoch: string;
  frame: string;
  phase: "loading" | "opening" | "playing" | "paused" | "result";
  options: GameOptions;
  seatSlots: number[];
  localSeat: number;
  players: PlayerSummary[];
  inputs: WireInput[];
  loading?: { ready: boolean[] };
  digest: string;
  presentationDigest: string;
  replay: InputWindow;
  serverAt: number;
  resourceDescriptorSHA256: string;
};
export type CheckpointResponse = {
  roomId?: string;
  version: number;
  serverInstance: string;
  serverNow: number;
  data: { view: NetworkView; checkpoint: string };
};
export type InputAction = {
  type: "input" | "release";
  epoch: string;
  sequence: number;
  buttons?: number;
};
export type LocalEvent = {
  sequence: number;
  buttons: number;
  at: number;
  frame?: bigint;
};
export type Progress = {
  percent: number;
  label: string;
  loaded?: number;
  total?: number;
};
export const FPS = 59.92274340431231;
export const FRAME_MS = 1000 / FPS;
// Bounded speculative lead covers the verified delayed four-player profile.
export const MAX_PREDICTION = 20;
export function frameNumber(value: string): bigint {
  if (!/^(0|[1-9][0-9]*)$/.test(value)) throw Error("无效帧号");
  const number = BigInt(value);
  if (number > 0xffffffffffffffffn) throw Error("帧号越界");
  return number;
}
export function estimatedServerTime(
  clock: GameClock | undefined,
  now = performance.now(),
) {
  return clock
    ? clock.serverNow + Math.max(0, now - clock.receivedAt)
    : Date.now();
}
