import {
  defineGame,
  type OptionsProps,
} from "../../../web/src/game-api/module";
import { Board } from "./Board";
import { Cover } from "./Cover";
import {
  classInfo,
  mapNames,
  teamColors,
  teamNames,
  type Options,
  type View,
} from "./types";
import "./style.css";

function OptionsForm({ value, onChange, disabled }: OptionsProps<Options>) {
  return (
    <div className="fortressblue-options">
      <label>
        地图
        <select
          aria-label="疯狂坦克地图"
          disabled={disabled}
          value={value.map}
          onChange={(event) =>
            onChange({ ...value, map: event.target.value as Options["map"] })
          }
        >
          {Object.entries(mapNames).map(([id, name]) => (
            <option key={id} value={id}>
              {name}
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
          {[15, 20, 25, 30, 40].map((seconds) => (
            <option key={seconds} value={seconds}>
              {seconds} 秒
            </option>
          ))}
        </select>
      </label>
      <p>
        2～10 人，红蓝两队各占一半；每人选择一种坦克，轮流计算角度、力度和风向开火。
        地形可被炸毁，坦克可能被击落或沉入低地。
      </p>
    </div>
  );
}

export default defineGame<View, Options>({
  id: "fortressblue",
  name: "疯狂坦克",
  category: "sports",
  description:
    "红蓝两队驾驶坦克轮流开火，计算角度、力度与风力，炸开地形，把对手打进深谷。",
  minPlayers: 2,
  maxPlayers: 10,
  playerCounts: [2, 4, 6, 8, 10],
  requiresFullTable: true,
  supportsBots: true,
  ownsSounds: true,
  defaultOptions: { map: "hills", turnSeconds: 25 },
  Board,
  Cover,
  Options: OptionsForm,
  PlayerSummary: ({ data, seat, room }) => {
    const tank = data.tanks?.[seat];
    if (!tank) return null;
    const info = classInfo[tank.class];
    const current = data.current === seat && data.phase === "playing";
    return (
      <div
        className={`fortressblue-player ${current ? "current" : ""} ${tank.alive ? "" : "dead"}`}
        data-fortressblue-seat={seat}
        data-hp={tank.hp}
        data-alive={tank.alive ? "1" : "0"}
      >
        <strong>
          <i
            aria-hidden="true"
            style={{ background: teamColors[tank.team] }}
          />
          {tank.selected ? info?.name ?? tank.class : "选坦克中"}
          <em>{teamNames[tank.team]}</em>
          {room.you === seat && !room.spectating ? " · 你" : ""}
        </strong>
        <span>
          {tank.alive ? (
            <>
              HP {tank.hp}/{tank.maxHp}
              {tank.special > 0 ? ` · 特殊弹 ${tank.special}` : ""}
              {current ? " · 行动中" : ""}
            </>
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
        2～10 人的回合制坦克炮战，红蓝两队各占一半席位。选坦克后红队先手，双方按
        「红蓝交替」行动。每回合可以移动一次或多次（消耗移动力），再调整角度、力度和弹种开火；开火后轮到下一辆存活坦克。
      </p>
      <p>
        炮弹受重力和风力影响，命中坦克或地形后爆炸。爆炸会削掉地形；坦克落到新地面会受伤，落入深水或低洼地带可能直接沉没。每局 8
        分钟后进入突然死亡，伤害翻倍、风力增强；12 分钟时按双方剩余总生命判定，相同则和局。
      </p>
      <p>
        五种坦克：均衡（双联炮）、重装（重锤）、侦察（三连发）、狙击（穿甲弹）、支援（维修弹）。特殊弹数量有限；支援弹会治疗爆炸范围内的队友，不造成伤害。
      </p>
      <p>
        PC：W／S 调整角度，A／D 移动，←／→ 调整力度，空格开火，Q／E
        切换弹种。手机：拖动角度和力度滑杆，使用左右移动与开火按钮。所有结果由服务器判定；刷新或重连不会重播旧炮弹。
      </p>
      <details>
        <summary>素材与许可</summary>
        <p>
          全部坦克、地形、封面与音效由本项目原创制作：封面为手绘式矢量
          SVG，坦克和地形由 Canvas 实时绘制，音效与战斗音乐由
          <code>scripts/fortressblue/prepare-audio.mjs</code>
          离线合成，无外部采样。没有使用参考游戏的商业图片、音频或名称。
        </p>
      </details>
    </div>
  ),
});
