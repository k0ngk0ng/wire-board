import { useEffect } from "react";
import { defineGame, type OptionsProps } from "../../../web/src/game-api/module";
import catalog from "./catalog";
import { Board } from "./Board";
import type { Options, View } from "./types";
import "./style.css";

function PvzOptions({ value, onChange, capacity, disabled }: OptionsProps<Options>) {
  useEffect(() => {
    const mode = capacity === 4 ? "team" : "duel";
    if (value.teams.length !== capacity || value.mode !== mode)
      onChange({ ...value, mode, teams: Array.from({ length: capacity }, (_, i) => i % 2) });
  }, [capacity, value, onChange]);
  return <div className="pvz-options">
    <label>每回合时限<select aria-label="植物大战僵尸每回合时限" disabled={disabled} value={value.roundSeconds} onChange={e => onChange({ ...value, roundSeconds: Number(e.target.value) })}>
      <option value={180}>3 分钟</option><option value={360}>6 分钟</option><option value={600}>10 分钟</option>
    </select></label>
    <label>布阵时间<select aria-label="植物大战僵尸布阵时间" disabled={disabled} value={value.prepSeconds} onChange={e => onChange({ ...value, prepSeconds: Number(e.target.value) })}>
      <option value={10}>10 秒</option><option value={20}>20 秒</option><option value={30}>30 秒</option>
    </select></label>
    <label>首回合阵营<select aria-label="植物大战僵尸分队" disabled={disabled} value={value.teams.join(",")} onChange={e => onChange({ ...value, teams: e.target.value.split(",").map(Number) })}>
      {capacity === 4 ? <><option value="0,1,0,1">座位 1、3 防守，座位 2、4 进攻</option><option value="0,0,1,1">座位 1、2 防守，座位 3、4 进攻</option><option value="0,1,1,0">座位 1、4 防守，座位 2、3 进攻</option></> : <><option value="0,1">座位 1 防守，座位 2 进攻</option><option value="1,0">座位 2 防守，座位 1 进攻</option></>}
    </select></label>
  </div>;
}

export default defineGame<View, Options>({
  ...catalog,
  asyncBoard: true,
  Board,
  Options: PvzOptions,
  defaultOptions: { mode: "duel", teams: [0, 1], roundSeconds: 360, prepSeconds: 20 },
  PlayerSummary: ({ room, data, seat }) => {
    const player = data.players?.[seat];
    if (!player) return null;
    return <div className={`pvz-player-summary pvz-side-${player.side}`}>
      <span>{player.side === "plants" ? "植物" : "僵尸"} · {player.team === 0 ? "A" : "B"} 队</span>
      <strong data-pvz-money={seat} aria-label={`${room.seats[seat]?.name ?? "玩家"}的${player.side === "plants" ? "阳光" : "脑力"}`}>{player.money}<small>{player.side === "plants" ? "阳光" : "脑力"}</small></strong>
    </div>;
  },
  Rules: () => <div className="pvz-rules">
    <p>植物方选择卡牌后点击草坪格种植，僵尸方选择卡牌后点击右侧行入口出兵。所有单位自动战斗。每人独立持有阳光或脑力、卡牌冷却和单位；铲子只清除自己的植物。</p>
    <p>割草机每行仅能使用一次。僵尸突破房屋即攻破，植物守到回合结束即守住。双方各进攻一次：只有一方攻破时该队获胜；均攻破时比较突破时间，均守住或时间相同则和局。2v2 共用草坪、共同胜负。</p>
    <p>鼠标或触屏先选卡，再选择格子或行入口。键盘与手柄通过平台控制器导航、确认和取消；切换操作组可在卡牌与草坪之间移动。添加电脑和主动托管沿用房间入口，断线不会自动开启托管。</p>
  </div>,
});
