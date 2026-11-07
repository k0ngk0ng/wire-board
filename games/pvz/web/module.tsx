import { useEffect } from "react";
import { defineGame, type OptionsProps } from "../../../web/src/game-api/module";
import catalog from "./catalog";
import { Board } from "./Board";
import type { Options, View } from "./types";
import { SCENES } from "./scene";
import { ResultDetails } from "./ResultDetails";
import "./style.css";

function PvzOptions({ value, onChange, capacity, disabled }: OptionsProps<Options>) {
  useEffect(() => {
    const mode = capacity === 4 ? "team" : "duel";
    if (value.teams.length !== capacity || value.mode !== mode)
      onChange({ ...value, mode, teams: Array.from({ length: capacity }, (_, i) => i % 2) });
  }, [capacity, value, onChange]);
  return <div className="pvz-options">
    <label>经典场景<select aria-label="植物大战僵尸场景" disabled={disabled} value={value.scene ?? "day"} onChange={event => onChange({ ...value, scene: event.target.value as Options["scene"] })}>{SCENES.map(([id, name]) => <option key={id} value={id}>{name}</option>)}</select></label>
    <label>每回合时限<select aria-label="植物大战僵尸每回合时限" disabled={disabled} value={value.roundSeconds} onChange={e => onChange({ ...value, roundSeconds: Number(e.target.value) })}>
      <option value={180}>3 分钟</option><option value={360}>6 分钟</option><option value={600}>10 分钟</option>
    </select></label>
    <label>布阵时间<select aria-label="植物大战僵尸布阵时间" disabled={disabled} value={value.prepSeconds} onChange={e => onChange({ ...value, prepSeconds: Number(e.target.value) })}>
      <option value={10}>10 秒</option><option value={20}>20 秒</option><option value={30}>30 秒</option>
    </select></label>
    <label>首回合阵营<select aria-label="植物大战僵尸分队" disabled={disabled} value={value.teams.join(",")} onChange={e => onChange({ ...value, teams: e.target.value.split(",").map(Number) })}>
      {capacity === 4 ? <><option value="0,1,0,1">座位 1、3 植物，座位 2、4 僵尸</option><option value="0,0,1,1">座位 1、2 植物，座位 3、4 僵尸</option><option value="0,1,1,0">座位 1、4 植物，座位 2、3 僵尸</option></> : <><option value="0,1">座位 1 植物，座位 2 僵尸</option><option value="1,0">座位 2 植物，座位 1 僵尸</option></>}
    </select></label>
  </div>;
}

export default defineGame<View, Options>({
  ...catalog,
  asyncBoard: true,
  assetLoading: "board",
  // The Board loads the selected scene and public decks from this build's CDN
  // root. The shared legacy-media prefix does not own PvZ's new assets.
  resources: () => [],
  Board,
  ResultDetails,
  Options: PvzOptions,
  defaultOptions: { mode: "duel", teams: [0, 1], roundSeconds: 360, prepSeconds: 20, scene: "day" },
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
    <p>植物队初始阳光：白天草坪 500、泳池 600、屋顶与夜间屋顶 700、夜晚草坪 300、浓雾 400；2v2 均分到各自钱包。基础补给每次 25 阳光：草坪／泳池每 4 秒、屋顶每 3 秒、夜晚草坪／浓雾每 8 秒。向日葵和阳光菇种下后 3～6 秒首次产出，此后每 18～20 秒产出。</p>
    <p>僵尸队每次部署至少间隔 3 秒，各单位更长的冷却仍生效。特殊僵尸在开战后分批解锁：45 秒开放橄榄球等强化单位，60 秒开放潜水、海豚、跳跳等单位，90 秒开放气球、矿工、蹦极和车辆，120 秒开放巨人与僵王；换边后重新计时。默认植物牌组含对空手段，三叶草可吹走气球。</p>
    <p>布阵时通过卡册选择最多 10 张卡，模仿者须指定目标。泳池水面使用睡莲承载陆生植物，屋顶使用花盆；花盆或睡莲、本体和南瓜可以同格。点击植物可分别管理自己的层，玉米加农炮选择瞄准后再指定目标。白天休眠的蘑菇可用咖啡豆唤醒。</p>
    <p>白天、夜晚、泳池、浓雾、屋顶与夜间屋顶共用对战规则。浓雾隐藏未被路灯或三叶草揭示的敌方单位；水生、飞行、潜水与地底单位遵循各自规则。升级植物需要自己的前置植物。</p>
    <p>割草机每行仅能使用一次。僵尸突破房屋即攻破，植物守到回合结束即守住。双方各进攻一次：只有一方攻破时该队获胜；均攻破时比较突破时间，均守住或时间相同则和局。2v2 共用草坪、共同胜负。</p>
    <p>鼠标或触屏先选卡，再选择格子或行入口。键盘与手柄通过平台控制器导航、确认和取消；切换操作组可在卡牌与草坪之间移动。添加电脑和主动托管沿用房间入口，断线不会自动开启托管。</p>
  </div>,
});
