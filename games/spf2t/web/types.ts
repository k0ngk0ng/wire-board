export type Options = {
  format: "single" | "bestOf3";
};

export type Phase = "select" | "countdown" | "playing" | "round_break" | "finished";
export type Color = "r" | "y" | "g" | "b";
export type GemKind = "normal" | "crash" | "power" | "counter" | "diamond";

export type GemView = {
  id: number;
  kind: GemKind;
  color?: Color;
  x: number;
  y: number;
  w: number;
  h: number;
  timer?: number;
};

export type PairCell = {
  x: number;
  y: number;
  kind: GemKind;
  color?: Color;
};

export type PairView = {
  id: number;
  x: number;
  y: number;
  rot: number;
  cells: PairCell[];
  ghost?: { x: number; y: number }[];
};

export type PairSpec = {
  kind: GemKind;
  color?: Color;
};

export type PlayerView = {
  seat: number;
  character?: string;
  selected: boolean;
  roundWins: number;
  gems: GemView[];
  active?: PairView;
  next: { a: PairSpec; b: PairSpec };
  pendingCount: number;
  warning?: string;
  resolving: boolean;
  lost: boolean;
  pattern?: string[];
  gemsCleared: number;
  maxChain: number;
  seq?: number;
};

export type CharacterView = {
  id: string;
  name: string;
  pattern: string[];
};

export type GameEvent = {
  id: number;
  type:
    | "select"
    | "select_timeout"
    | "countdown"
    | "start"
    | "round_start"
    | "lock"
    | "clear"
    | "power_merge"
    | "diamond"
    | "attack"
    | "garbage"
    | "topout"
    | "round_end"
    | "resolve_end"
    | "finish";
  at: number;
  seat: number;
  target?: number;
  chain?: number;
  count?: number;
  attack?: number;
  kind?: GemKind;
  color?: Color;
  character?: string;
  cells?: { x: number; y: number }[];
};

export type View = {
  options: Options;
  phase: Phase;
  epoch: string;
  serverAt: number;
  startAt: number;
  matchEndsAt: number;
  selectUntil: number;
  breakUntil: number;
  round: number;
  roundsToWin: number;
  players: PlayerView[];
  events: GameEvent[];
  characters: CharacterView[];
  controls: { canPlay: boolean; canSelect: boolean };
};

export const formatLabel = (format: Options["format"]) =>
  format === "single" ? "单局决胜" : "三局两胜";

export const phaseLabel = (phase: Phase) => {
  switch (phase) {
    case "select":
      return "选择角色";
    case "countdown":
      return "准备开始";
    case "round_break":
      return "回合结束";
    case "finished":
      return "对局结束";
    default:
      return "对局进行中";
  }
};

export const characterName = (data: View, id?: string) =>
  data.characters.find((c) => c.id === id)?.name ?? "未选择";

export const scopeKey = (roomId: string, data: View, seat: number) =>
  `${roomId}:${data.epoch}:${seat}`;
