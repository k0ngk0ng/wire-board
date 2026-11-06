import { Battery, Heart, Package } from "lucide-react";
import {
  defineGame,
  type OptionsProps,
} from "../../../web/src/game-api/module";
import catalog from "./catalog";
import { Board } from "./Board";
import { scenarioNames, roleNames, type Options, type View } from "./types";
import "./style.css";
function OptionsForm({ value, onChange, disabled }: OptionsProps<Options>) {
  return (
    <div className="escape-options">
      <fieldset disabled={disabled}>
        <legend>选择密室</legend>
        <div className="escape-scenarios">
          {(["station", "archive"] as const).map((scene) => (
            <label
              className={value.scenario === scene ? "selected" : ""}
              key={scene}
            >
              <input
                type="radio"
                name="escape-scenario"
                value={scene}
                checked={value.scenario === scene}
                onChange={() => onChange({ ...value, scenario: scene })}
              />
              <img
                src={`/escape/v1/${scene === "station" ? "station-atrium" : "archive-hall"}.webp`}
                alt=""
              />
              <span>
                <strong>{scenarioNames[scene]}</strong>
                <small>
                  {scene === "station"
                    ? "最后一班 · 供电与远端联锁"
                    : "月下旧馆 · 排水与档案护送"}
                </small>
              </span>
            </label>
          ))}
        </div>
      </fieldset>
      <div className="escape-option-fields">
        <label>
          难度
          <select
            aria-label="密室难度"
            disabled={disabled}
            value={value.difficulty}
            onChange={(e) =>
              onChange({
                ...value,
                difficulty: e.target.value as Options["difficulty"],
              })
            }
          >
            <option value="easy">探索 · 25 分钟</option>
            <option value="normal">标准 · 20 分钟</option>
            <option value="hard">险境 · 15 分钟</option>
          </select>
        </label>
        <label>
          地图编号（留空随机）
          <input
            disabled={disabled}
            aria-label="密室地图编号"
            value={value.seed ?? ""}
            maxLength={32}
            placeholder="每次重组"
            pattern="[a-zA-Z0-9_-]*"
            onChange={(e) => onChange({ ...value, seed: e.target.value })}
          />
        </label>
      </div>
    </div>
  );
}
export default defineGame<View, Options>({
  ...catalog,
  asyncBoard: true,
  defaultOptions: { scenario: "station", difficulty: "normal", seed: "" },
  Board,
  Options: OptionsForm,
  resources: (room, media) =>
    Object.keys(media).filter(
      (p) =>
        p.startsWith("/escape/v1/") &&
        (!p.includes("/walk-") ||
          Number(p.match(/walk-(\d)/)?.[1]) < room.seats.length) &&
        ((!p.includes("station-") && !p.includes("archive-")) ||
          p.includes((room.game?.data as View)?.options.scenario + "-")),
    ),
  isInputRelease: (a) => a.type === "input" && !(a.dx || a.dy),
  PlayerSummary: ({ room, data, seat }) => {
    const p = data.players[seat];
    if (!p) return null;
    return (
      <div className="escape-player-summary" data-escape-health={seat}>
        <span>
          <Heart size={13} />
          <strong>{Math.ceil(p.hp)}</strong>
          <small>
            {roleNames[seat]}
            {room.you === seat && !room.spectating ? " · 你" : ""}
          </small>
        </span>
        <span>
          {p.hp <= 0
            ? "倒地 · 等待救援"
            : p.ready
              ? "撤离就绪"
              : p.carry
                ? "携带密封档案"
                : "正在探索"}
        </span>
      </div>
    );
  },
  Rules: () => (
    <div>
      <p>
        2～4
        个角色合作逃脱，支持真人、电脑伙伴与主动托管。WASD／方向键移动，点击物件走近调查，E
        就近交互，M 查看地图，J
        查看调查笔记。手机可点击地面移动，也可使用方向键。
      </p>
      <p>
        封锁站可以恢复供电并完成双端联锁，或修复排水与信号后走检修通道。沉没档案馆可以排水并运回两份档案乘艇离开，也可解开天文锁盘并启动穹顶联锁。旋钮按
        1～4 循环，线索由全队共享。联锁必须由不同房间的两名队友同时维持。
      </p>
      <p>
        急救包和电池全队共享，每个补给只可领取一次。倒地队员可由同房队友靠近救援；全队倒地或超过时限则失败。危险区域会提前预警。携带档案时移动较慢，一次只能携带一份；到水闸点击撤离装置交付。
      </p>
      <p>
        全队到撤离室选择可用路线并确认后共同获胜。支持投票暂停、存档重连；离线不会自动托管。地图编号可重现布局，留空则每局重新生成。合作局记录成绩，不改变竞争积分。
      </p>
    </div>
  ),
  ResultDetails: ({ data }) => (
    <div className="escape-result">
      <strong>
        {scenarioNames[data.options.scenario]} · {data.reason}
      </strong>
      <p>
        <Package size={16} /> 调查 {data.stats.explored} 个房间 · 救援{" "}
        {data.stats.rescues} 次 · 校准失误 {data.stats.mistakes} 次
      </p>
      <p>
        <Battery size={16} /> 剩余电力 {Math.ceil(data.power)}% · 档案{" "}
        {data.supplies.documents} 份
      </p>
      <small>地图编号：{data.seed ?? data.options.seed}</small>
    </div>
  ),
});
