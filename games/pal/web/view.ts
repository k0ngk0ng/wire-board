import type { SceneFrame } from "./scene-renderer";
import type { FieldState } from "./field-rules";
import type { PalAudioState } from "./audio-types";
import type { LocalMeta } from "./local-types";
export type PalPartyMember = {
  role: number;
  name: string;
  controller: number;
  hp: number;
  maxHP: number;
  mp: number;
  maxMP: number;
};
export type PalView = SceneFrame & {
  local?: LocalMeta;
  audio?: PalAudioState;
  gameOver?: { stage: number; slot: number };
  field?: FieldState;
  epoch: number;
  seq: number;
  appliedSeq: number;
  controlAt: number;
  discardedSeq?: number;
  phase:
    | "script"
    | "dialogue"
    | "field"
    | "menu"
    | "system"
    | "battle"
    | "cooperation"
    | "ending"
    | "finished"
    | "gameover";
  leader: number;
  party: PalPartyMember[];
  scene: number;
  x: number;
  y: number;
  cash: number;
  menu: {
    kind: number;
    depth: number;
    selected: number;
    role: number;
    items: {
      value: number;
      word: number;
      enabled: boolean;
      x: number;
      y: number;
      label: string;
    }[];
  };
  dialogue?: {
    id: number;
    text: string;
    face: number;
    position: number;
    read: boolean[];
  };
  battle?: {
    phase: number;
    state: number;
    menu: number;
    current: number;
    selected: number;
    action: number;
    object: number;
    target: number;
    players: {
      role: number;
      hp: number;
      maxHP: number;
      mp: number;
      maxMP: number;
      level: number;
      state: number;
      action: number;
      object: number;
      target: number;
      frame: number;
      defending: boolean;
    }[];
    enemies: {
      object: number;
      frame: number;
      x: number;
      y: number;
      colorShift: number;
    }[];
  };
  cooperation?: {
    id: number;
    initiator: number;
    magic: number;
    name: string;
    target: number;
    affectedRoles: number[];
    contributors: number[];
    hpCost: number;
    required: number[];
    approved: boolean[];
  };
};
