import { resources } from "./loading";
import catalog from "./catalog";
import {
  defineGame,
  type OptionsProps,
} from "../../../web/src/game-api/module";
import { Board, FallbackImage } from "./Board";
import { Cover } from "./Cover";
import {
  defaultMap,
  mapById,
  maps,
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
  const random = value.map === "random";
  const selected = random ? null : (mapById.get(value.map) ?? maps[0]);
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
              {map.stage}. {map.name}
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
          onChange={(event) =>
            onChange({ ...value, turnSeconds: Number(event.target.value) })
          }
        >
          {[15, 20, 25, 30, 40, 60].map((seconds) => (
            <option key={seconds} value={seconds}>
              {seconds} 秒
            </option>
          ))}
        </select>
      </label>
      <label>
        电脑难度
        <select
          aria-label="疯狂坦克电脑难度"
          disabled={disabled}
          value={value.botDifficulty ?? "normal"}
          onChange={(event) =>
            onChange({
              ...value,
              botDifficulty: event.target.value as Options["botDifficulty"],
            })
          }
        >
          <option value="easy">简单</option>
          <option value="normal">普通</option>
          <option value="hard">困难</option>
        </select>
      </label>
      <div className={`fb-map-preview ${random ? "random" : ""}`}>
        {random ? (
          <div className="fb-map-collage" aria-hidden="true">
            {maps.slice(0, 4).map((map) => (
              <img key={map.id} src={map.thumb} alt="" />
            ))}
          </div>
        ) : (
          <img src={selected?.thumb} alt="" />
        )}
        <div>
          <strong>{random ? "随机地图" : selected?.name}</strong>
          <span>
            {random
              ? `开局从 ${maps.length} 张原版地图中随机选择`
              : "可破坏地形 · 自由观察战场"}
          </span>
          <small>2～8 人 · 4v4 团队炮战 · 原版可破坏地形</small>
        </div>
      </div>
      <p>
        红蓝两队轮流行动。利用角度、力度和风向击中对手；爆炸会永久改变地形，
        坦克落地后才结算坠落伤害。电脑与托管使用所选难度；三档仅改变操作精度。按住空格或开火键蓄力，松开或满力时发射。
      </p>
    </div>
  );
}

export default defineGame<View, Options>({
  ...catalog,
  resources,
  asyncBoard: true,
  defaultOptions: { map: defaultMap, turnSeconds: 60, botDifficulty: "normal" },
  Board,
  Cover,
  Options: OptionsForm,
  PlayerSummary: ({ data, seat, room, assets }) => {
    const tank = data.tanks?.[seat];
    if (!tank) return null;
    const info = tankById.get(tank.tankId);
    const portrait = tankPortraits[String(tank.tankId)];
    const current =
      data.current === seat &&
      data.phase === "playing" &&
      !data.resolution &&
      data.deadline > 0;
    const pct = Math.max(
      0,
      Math.min(100, (tank.hp / Math.max(1, tank.maxHp)) * 100),
    );
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
          <FallbackImage
            key={`${assets}:${tank.tankId}`}
            base={assets}
            path={portrait?.url ?? info?.sprites.idle?.url ?? ""}
            alt=""
          />
          <strong>
            <i
              aria-hidden="true"
              style={{ background: teamColors[tank.team] }}
            />
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
                生命 {tank.hp}/{tank.maxHp}
                {!tank.specialUnlimited ? ` · 2号弹 ${tank.special}` : ""}
                {tank.statuses?.length
                  ? ` · ${tank.statuses.map(statusText).join(" ")}`
                  : ""}
                {current ? " · 行动中" : ""}
              </>
            )
          ) : (
            `生命 0/${tank.maxHp} · 已被击毁`
          )}
        </span>
      </div>
    );
  },
  Rules: () => (
    <div className="fortressblue-rules">
      <p>
        2／4／6／8
        人，红蓝两队各占一半。每回合可以先用移动力左右移动，再调整角度、力度、
        朝向与弹种开火。炮弹飞行、爆炸与落地全部完成后，下一辆坦克才获得完整行动时间。爆炸会破坏地形，坦克可能因坠落受伤或掉出地图。
      </p>
      <p>
        14 种坦克拥有各自的生命、防御、移动力和炮角范围；新局可反复切换 1
        号弹、2 号弹。
        道具在自己的回合开火前使用：补丸恢复生命，防护罩抵挡一次攻击，能量补充弹
        将下一发改为落点附近敌我双方都能获得恢复的炮弹。火焰和毒气留在落点区域，离开范围可避开持续伤害。
        每回合有时限；本平台团队规则在 8 分钟后加强伤害，12
        分钟按剩余生命判定，相等为和局。
      </p>
      <p>
        PC：←／→ 或 A／D 持续移动，↑／↓ 或 W／S 调角度，Q／E 切换弹种；
        按住空格蓄力，松开或充满时发射。点击战场只调整瞄准，拖动平移，滚轮以指针位置为中心缩放。
        手机：拖动地图或双指缩放，使用角度滑杆、移动键和按住蓄力键；手势取消不会发射。
        飞行中刷新会继续当前进度，已完成的炮弹不会重播。
      </p>
      <details>
        <summary>素材与许可</summary>
        <p>
          坦克、地图、炮弹和爆炸动画来自用户提供的官方《Fortress 2
          BLUE》网页客户端资源； 版权归 CCR /
          原权利方所有，仅按本私有平台的授权范围使用。
          角色使用原始尺寸、朝向和动作帧；地图保持可见地形与碰撞一致。封面为本项目原创手绘矢量插画。
        </p>
      </details>
    </div>
  ),
});
