import { useEffect } from "react";
import {
  defineGame,
  type OptionsProps,
} from "../../../web/src/game-api/module";
import { Board } from "./Board";
import { Cover } from "./Cover";
import { Preparation } from "./Preparation";
import type { BasketballNetworkView } from "./network-scene";

export type BasketballOptions = {
  court: "new-york" | "court-0" | "court-2" | "court-3" | "court-4";
  teams?: [number, number];
  characters?: [number, number, number, number];
  mode: "solo" | "cooperative" | "versus";
};
export function OptionsForm({
  value,
  onChange,
  capacity,
  disabled,
}: OptionsProps<BasketballOptions>) {
  const mode =
    capacity === 1
      ? "solo"
      : value.mode === "versus"
        ? "versus"
        : "cooperative";
  useEffect(() => {
    if (mode !== value.mode) onChange({ ...value, mode });
  }, [mode, value, onChange]);
  return (
    <div className="basketball-options">
      {capacity === 2 && (
        <label>
          比赛模式
          <select
            aria-label="比赛模式"
            value={mode}
            disabled={disabled}
            onChange={(e) =>
              onChange({
                ...value,
                mode: e.target.value as BasketballOptions["mode"],
              })
            }
          >
            <option value="cooperative">双人合作</option>
            <option value="versus">双人对战</option>
          </select>
        </label>
      )}
      <p>
        {mode === "solo"
          ? "你与一名原版电脑队友，对阵两名电脑球员。"
          : mode === "cooperative"
            ? "两位玩家同属 A 队，对阵电脑 B 队。"
            : "第一席为 A 队，第二席为 B 队，各有一名原版电脑队友。"}
      </p>
      <p>上下半场 · 含电脑的比赛不计竞技积分。场地和球员在准备阶段选择。</p>
    </div>
  );
}

// The frontend discovers this metadata automatically. The server registry
// remains gated by gameplay, asset distribution and release acceptance.
export default defineGame<BasketballNetworkView, BasketballOptions>({
  id: "basketball",
  name: "热血篮球",
  description: "街头双人篮球，争抢、传球与扣篮。支持单人、双人合作和双人对战。",
  category: "sports",
  minPlayers: 1,
  maxPlayers: 2,
  playerCounts: [1, 2],
  requiresFullTable: true,
  supportsBots: true,
  realtime: true,
  ownsSounds: true,
  defaultOptions: {
    court: "new-york",
    mode: "solo",
    teams: [0, 1],
    characters: [0, 1, 0, 1],
  },
  Board,
  Cover,
  Options: OptionsForm,
  Preparation,
  PlayerSummary: ({ data, seat }) => {
    const actor = data.seatActors?.[seat] ?? seat;
    const team = Math.floor(actor / 2);
    return (
      <div>
        <strong>
          {team === 0 ? "A 队" : "B 队"} · {data.scores[team]} 分
        </strong>
      </div>
    );
  },
});
