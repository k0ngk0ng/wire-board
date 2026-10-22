import catalog from "./catalog";
import {
  defineGame,
  type GameRoom,
  type OptionsProps,
} from "../../../web/src/game-api/module";
import { Board } from "./Board";
import { Cover } from "./Cover";
import { MapPreview } from "./MapPreview";
import {
  MAPS,
  TANK_NAMES,
  TEAM_NAMES,
  phaseLabel,
  type Options,
  type View,
} from "./types";

function OptionsForm({
  value,
  onChange,
  capacity,
  disabled,
}: OptionsProps<Options>) {
  const selected =
    value?.map &&
    (value.map === "random" || MAPS.some((m) => m.id === value.map))
      ? value.map
      : "classic-01";
  const preview = selected === "random" ? "classic-01" : selected;
  const map = MAPS.find((m) => m.id === preview);
  const teamSize = Math.max(1, capacity / 2);
  return (
    <div className="battlecity-options">
      <div className="bc-option-row">
        <label>
          对战地图
          <select
            aria-label="对战地图"
            disabled={disabled}
            value={selected}
            onChange={(event) => onChange({ map: event.target.value })}
          >
            {MAPS.map((m) => (
              <option key={m.id} value={m.id}>
                {m.name}
              </option>
            ))}
            <option value="random">随机地图</option>
          </select>
        </label>
        <div className="bc-option-preview">
          <MapPreview mapId={preview} small />
          <span>{selected === "random" ? "开局随机" : map?.name}</span>
        </div>
      </div>
      <p className="muted small">
        {teamSize}v{teamSize}，两队各 {teamSize} 人；座位 1～{teamSize}{" "}
        为上方队伍， 其余为下方队伍。电脑玩家参与的对局不计积分。
      </p>
    </div>
  );
}

function Rules() {
  return (
    <div className="battlecity-rules">
      <p>
        1v1、2v2 或 3v3：两队各有一个大本营，先摧毁对方大本营的队伍获胜。
        大本营被任意敌方炮弹命中一次即毁；坦克阵亡 4 秒后无限重生。
      </p>
      <p>
        开局 15 秒选择坦克：均衡、基础、快速、火力和装甲。每种坦克的移动、装甲、
        弹速不同；双方可以重复选择。击毁敌方坦克约 30% 概率掉落随机道具。
      </p>
      <p>
        星提升科技（弹速、双发、破钢）；手雷摧毁敌方坦克；头盔 8 秒无敌；
        铲子把己方基地保护墙变成钢铁 20 秒；备用坦克让下一次阵亡立即重生；
        时钟冻结敌方坦克 8 秒。
      </p>
      <p>
        键盘：WASD／方向键移动，空格或 J
        开火。手机使用画布下方的方向键和开火键。 对局上限 10
        分钟；双方基地都存活时按平台显式和局处理。
      </p>
    </div>
  );
}

function PlayerSummary({
  room,
  data,
  seat,
}: {
  room: GameRoom;
  data?: View;
  seat: number;
  assets: string;
}) {
  const tank = data?.tanks?.[seat];
  if (!tank) return null;
  const team = tank.team === 0 ? TEAM_NAMES[0] : TEAM_NAMES[1];
  const cls = tank.class ? TANK_NAMES[tank.class] : "未选择";
  const alive = tank.alive;
  const respawn =
    !alive && tank.respawnAt > 0
      ? Math.max(1, Math.ceil((tank.respawnAt - (data?.serverAt ?? 0)) / 1000))
      : 0;
  return (
    <div
      className={`battlecity-player-summary ${tank.team === 0 ? "is-top" : "is-bottom"} ${
        room.you === seat ? "is-you" : ""
      }`}
      data-battlecity-seat={seat}
      data-battlecity-team={tank.team}
      title={`${room.seats[seat]?.name ?? `玩家 ${seat + 1}`} · ${team}`}
    >
      <strong>
        {cls}
        {room.you === seat ? " · 你" : ""}
      </strong>
      <span>{team}</span>
      <span>
        {alive ? `装甲 ${tank.hp}/${tank.maxHp}` : `重生 ${respawn}s`}
      </span>
      <span>击杀 {tank.kills}</span>
    </div>
  );
}

export default defineGame<View, Options>({
  ...catalog,
  asyncBoard: true,
  defaultOptions: { map: "classic-01" },
  Board,
  Cover,
  Options: OptionsForm,
  Rules,
  PlayerSummary,
});

// Keep the phase labels reachable from tests and future summaries.
export { phaseLabel };
