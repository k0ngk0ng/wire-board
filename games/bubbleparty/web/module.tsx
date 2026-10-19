import {
  defineGame,
  type OptionsProps,
} from "../../../web/src/game-api/module";
import { Board } from "./Board";
import { Cover } from "./Cover";
import { mapNames, teamNames, type Options, type View } from "./types";

function OptionsForm({ value, onChange, disabled }: OptionsProps<Options>) {
  return (
    <div className="bubble-options">
      <label>
        地图
        <select
          aria-label="水泡战场地图"
          disabled={disabled}
          value={value.map}
          onChange={(event) => onChange({ ...value, map: event.target.value })}
        >
          {Object.entries(mapNames).map(([id, name]) => (
            <option key={id} value={id}>
              {name}
            </option>
          ))}
        </select>
      </label>
      <label>
        单局时限
        <select
          aria-label="单局时限"
          disabled={disabled}
          value={value.roundSeconds}
          onChange={(event) =>
            onChange({ ...value, roundSeconds: Number(event.target.value) })
          }
        >
          <option value={180}>3 分钟 · 快速局</option>
          <option value={240}>4 分钟 · 标准局</option>
          <option value={300}>5 分钟 · 完整局</option>
        </select>
      </label>
      <p>
        2／4／6／8 人分成红蓝两队，最多 4v4。水泡会连爆、破坏木箱并淘汰对手；
        3 分钟后安全区开始收缩。
      </p>
    </div>
  );
}

export default defineGame<View, Options>({
  id: "bubbleparty",
  name: "泡泡大作战",
  description:
    "在小区草坪上放下水泡，连锁爆破、收集道具，和队友一起把对手全部淘汰。",
  category: "realtime",
  minPlayers: 2,
  maxPlayers: 8,
  playerCounts: [2, 4, 6, 8],
  requiresFullTable: true,
  supportsBots: true,
  ownsSounds: true,
  realtime: true,
  defaultOptions: { map: "community10", roundSeconds: 300 },
  Board,
  Cover,
  Options: OptionsForm,
  Rules: () => (
    <div className="bubble-rules">
      <p>
        2／4／6／8 人的实时水泡对战。座位前半为红队、后半为蓝队；一方全员淘汰即获胜。
        时间到时按存活人数判定，人数相同为和局。
      </p>
      <p>
        WASD／方向键移动，空格或 J 放水泡。水泡约 2.2 秒后沿十字方向爆开，
        可以连爆；木箱会挡住爆炸并被摧毁。离开水泡所在格前它也会挡住移动。
      </p>
      <p>
        木箱中会掉落速度、炸弹容量、射程和护盾。护盾抵挡一次爆炸。
        3 分钟后进入突然死亡，安全区每 3 秒向内收缩一圈；停留在安全区外会被淘汰。
      </p>
      <p>
        手机：左侧方向键拖动移动，右侧「放水泡」；按钮均不小于 44 px。
        刷新或断线会恢复同一局，不会重播旧爆炸。电脑玩家会寻路、放水泡和躲避爆炸，支持托管。
      </p>
      <p>
        素材说明：泡泡堂玩法为灵感来源；本作的角色、地图、封面和音效均为项目原创，
        不使用商业游戏素材。小区10 是原创住宅小区主题地图。
      </p>
    </div>
  ),
  PlayerSummary: ({ room, data, seat }) => {
    const player = data.players.find((p) => p.seat === seat);
    if (!player) return null;
    return (
      <div
        className={`bubble-player-summary team-${player.team}`}
        data-bubbleparty-seat={seat}
        data-bubbleparty-seq={player.seq}
        data-bubbleparty-team={player.team}
        data-bubbleparty-alive={player.alive ? "1" : "0"}
      >
        <strong>
          <i aria-hidden="true" />
          {teamNames[player.team]} · {player.alive ? "存活" : "淘汰"}
          {room.you === seat && !room.spectating ? " · 你" : ""}
        </strong>
        <span>
          {player.alive
            ? `水泡 ${player.maxBombs} · 射程 ${player.range}${player.shield ? " · 护盾" : ""}`
            : "观战中"}
          {player.eliminations ? ` · 淘汰 ${player.eliminations}` : ""}
        </span>
      </div>
    );
  },
});
