import {
  defineGame,
  type OptionsProps,
} from "../../../web/src/game-api/module";
import { Board } from "./Board";
import { Cover } from "./Cover";
import {
  defaultMap,
  mapById,
  maps,
  sourceList,
  statusText,
  tankById,
  tankName,
  tankPortraits,
  teamColors,
  teamNames,
  type Options,
  type View,
} from "./types";
import "./style.css";

function OptionsForm({ value, onChange, disabled }: OptionsProps<Options>) {
  const selected = mapById.get(value.map) ?? maps[0];
  return (
    <div className="fortressblue-options">
      <label>
        地图
        <select
          aria-label="疯狂坦克地图"
          disabled={disabled}
          value={value.map}
          onChange={(event) => onChange({ ...value, map: event.target.value })}
        >
          <option value="random">随机地图</option>
          {maps.map((map) => (
            <option key={map.id} value={map.id}>
              {map.name}
            </option>
          ))}
        </select>
      </label>
      <label>
        每回合时限
        <select
          aria-label="每回合时限"
          disabled={disabled}
          value={value.turnSeconds}
          onChange={(event) => onChange({ ...value, turnSeconds: Number(event.target.value) })}
        >
          {[15, 20, 25, 30, 40].map((seconds) => (
            <option key={seconds} value={seconds}>
              {seconds} 秒
            </option>
          ))}
        </select>
      </label>
      <div className="fb-map-preview">
        <img src={selected.thumb} alt="" />
        <div>
          <strong>{selected.name}</strong>
          <span>{selected.nameEn}</span>
          <small>2～8 人 · 4v4 团队炮战 · 原版可破坏地形</small>
        </div>
      </div>
      <p>
        红蓝两队轮流行动。利用角度、力度和风向击中对手；爆炸会永久改变地形，
        落石与坠落同样会造成伤害。
      </p>
    </div>
  );
}

export default defineGame<View, Options>({
  id: "fortressblue",
  name: "疯狂坦克",
  category: "sports",
  description:
    "原版《Fortress 2 BLUE》回合制坦克炮战：17 张地图、14 种坦克、可破坏地形与风向。",
  minPlayers: 2,
  maxPlayers: 8,
  playerCounts: [2, 4, 6, 8],
  requiresFullTable: true,
  supportsBots: true,
  ownsSounds: true,
  defaultOptions: { map: defaultMap, turnSeconds: 30 },
  Board,
  Cover,
  Options: OptionsForm,
  PlayerSummary: ({ data, seat, room, assets }) => {
    const tank = data.tanks?.[seat];
    if (!tank) return null;
    const info = tankById.get(tank.tankId);
    const portrait = tankPortraits[String(tank.tankId)];
    const current = data.current === seat && data.phase === "playing";
    const pct = Math.max(0, Math.min(100, (tank.hp / Math.max(1, tank.maxHp)) * 100));
    return (
      <div
        className={`fortressblue-player ${current ? "current" : ""} ${tank.alive ? "" : "dead"}`}
        data-fortressblue-seat={seat}
        data-hp={tank.hp}
        data-x={tank.x}
        data-y={tank.y}
        data-team={tank.team}
        data-alive={tank.alive ? "1" : "0"}
      >
        <div className="fb-player-head">
          <img src={sourceList(assets, portrait?.url ?? info?.sprites.idle?.url ?? "")[0]} alt="" />
          <strong>
            <i aria-hidden="true" style={{ background: teamColors[tank.team] }} />
            {tank.selected ? tankName(tank.tankId) : "选坦克中"}
            <em>{teamNames[tank.team]}</em>
            {room.you === seat && !room.spectating ? " · 你" : ""}
          </strong>
        </div>
        <div className="fb-player-hp">
          <i style={{ width: `${pct}%`, background: teamColors[tank.team] }} />
        </div>
        <span>
          {tank.alive ? (
            data.phase === "select" ? (
              "待开局"
            ) : (
            <>
              HP {tank.hp}/{tank.maxHp} · 特殊弹 {tank.special}
              {tank.statuses?.length ? ` · ${tank.statuses.map(statusText).join(" ")}` : ""}
              {current ? " · 行动中" : ""}
            </>
            )
          ) : (
            "已被击毁"
          )}
        </span>
      </div>
    );
  },
  Rules: () => (
    <div className="fortressblue-rules">
      <p>
        2／4／6／8 人，红蓝两队各占一半。每回合可以先用移动力左右移动，再调整角度、力度、
        朝向与弹种开火；开火后轮到下一辆存活坦克。爆炸会破坏地形，坦克可能因坠落受伤或掉出地图。
      </p>
      <p>
        14 种原版坦克各有 HP、防御、移动力、主炮和特殊弹。特殊弹每局 2 发，附带爆炸、毒、火或雷
        效果；没有技能的坦克使用高伤害特殊弹。每回合有时限，8 分钟后进入突然死亡，12
        分钟按剩余总生命判定。
      </p>
      <p>
        PC：W／S 调角度，←／→ 调力度，A／D 移动，Q／E 切换弹种，空格开火。手机：拖动角度与力度滑杆，
        使用左右移动、朝向、弹种与开火按钮；所有结果由服务器判定，刷新或重连不会重播旧炮弹。
      </p>
      <details>
        <summary>素材与许可</summary>
        <p>
          坦克、地图、炮弹和爆炸动画来自用户提供的官方《Fortress 2 BLUE》网页客户端资源；
          版权归 CCR / 原权利方所有，仅经用户授权在本私有平台使用，原始媒体不进入 Git。
          素材经 LH5 解压和逐行 span 解码恢复，地图碰撞由原版前景层生成。封面为本项目原创手绘矢量插画。
        </p>
      </details>
    </div>
  ),
});
