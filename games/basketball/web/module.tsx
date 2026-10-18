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
  campaign?: import("./campaign-types").CampaignOptions;
  court:
    | "new-york"
    | "court-0"
    | "court-2"
    | "court-3"
    | "court-4"
    | "court-5"
    | "court-6"
    | "court-7";
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
  const entry: "single" | "new" | "password" = value.campaign
    ? (value.campaign.entry ??
      (Object.hasOwn(value.campaign, "password") ? "password" : "new"))
    : "single";
  const mode =
    capacity === 1
      ? "solo"
      : value.mode === "versus"
        ? "versus"
        : "cooperative";
  useEffect(() => {
    if (mode !== value.mode || (mode === "versus" && value.campaign))
      onChange({
        ...value,
        mode,
        campaign:
          mode !== "versus" && value.campaign
            ? {
                ...value.campaign,
                preparation: undefined,
                availableOpponents: undefined,
                opponent: 0,
              }
            : undefined,
      });
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
            onChange={(e) => {
              const next = e.target.value as BasketballOptions["mode"];
              onChange({
                ...value,
                mode: next,
                campaign:
                  next === "versus"
                    ? undefined
                    : value.campaign
                      ? {
                          ...value.campaign,
                          preparation: undefined,
                          availableOpponents: undefined,
                          opponent: 0,
                        }
                      : undefined,
              });
            }}
          >
            <option value="cooperative">双人合作</option>
            <option value="versus">双人对战</option>
          </select>
        </label>
      )}
      {mode !== "versus" && (
        <label>
          比赛形式
          <select
            aria-label="比赛形式"
            value={entry}
            disabled={disabled}
            onChange={(e) => {
              const next = e.target.value as typeof entry;
              onChange({
                ...value,
                campaign:
                  next === "single"
                    ? undefined
                    : {
                        entry: next,
                        opponent: 0,
                        tactic: 4,
                        ...(next === "password" ? { password: "" } : {}),
                      },
              });
            }}
          >
            <option value="single">单场比赛</option>
            <option value="new">新战役</option>
            <option value="password">原密码继续</option>
          </select>
        </label>
      )}
      {entry === "password" && mode !== "versus" && (
        <label>
          原版密码
          <input
            aria-label="原版密码"
            name="campaignPassword"
            value={value.campaign?.password ?? ""}
            required
            pattern="[0-9A-Fa-f]{6}"
            maxLength={6}
            autoComplete="off"
            spellCheck={false}
            disabled={disabled}
            onChange={(e) =>
              onChange({
                ...value,
                campaign: {
                  entry: "password",
                  password: e.target.value.trim().toUpperCase(),
                  opponent: 0,
                  tactic: 4,
                },
              })
            }
          />
          <small>输入 6 位原版密码，人数须与当前房间一致。</small>
        </label>
      )}
      {entry !== "single" && mode !== "versus" && (
        <p>
          热血队挑战七支球队，各进行一场主场和客场比赛。对手、球员和装备在准备阶段选择。
        </p>
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
