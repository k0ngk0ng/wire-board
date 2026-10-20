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
        2／4／6／8 人分成红蓝两队，最多 4v4。水泡会连爆、破坏木箱并困住对手；
        触碰被困队友可以救援，触碰被困对手可将其击破。
      </p>
    </div>
  );
}

export default defineGame<View, Options>({
  id: "bubbleparty",
  name: "泡泡堂",
  description:
    "在小区草坪上放下水泡，连锁爆破、收集道具，救出被困队友，击破对手水泡。",
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
        2／4／6／8
        人的实时水泡对战。座位前半为红队、后半为蓝队；一方全员淘汰即获胜。
        时间到时双方仍有人存活则为和局。
      </p>
      <p>
        WASD／方向键移动，空格或 J 放水泡。水泡约 2.2 秒后沿十字方向爆开，
        可以连爆；木箱会挡住爆炸并被摧毁。放下水泡后可以离开，但不能返回穿过水泡。
      </p>
      <p>
        水柱命中会把角色困在水泡中；队友触碰可救援，对手触碰会击破淘汰。
        困泡倒计时结束仍未获救则淘汰，救援后有短暂保护。
        冲进尚未消退的水柱也会被困，被困时无法击破或救援。
      </p>
      <p>
        木箱中会掉落速度、水泡容量和射程道具。 新对局没有缩圈；可以在房间选择
        3、4 或 5 分钟时限。
      </p>
      <p>
        手机：左侧方向键拖动移动，右侧「放水泡」；按钮均不小于 44 px。
        刷新或断线会恢复同一局，不会重播旧爆炸。电脑玩家会寻路、放水泡和躲避爆炸，支持托管。
      </p>
      <p>
        小区10 按历史地图参考重建；其他三张地图为自定义场地。
        本项目为围桌内的玩法还原，与官方运营服务无关联。
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
          {teamNames[player.team]} ·{" "}
          {player.trappedUntil ? "困泡" : player.alive ? "存活" : "淘汰"}
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
