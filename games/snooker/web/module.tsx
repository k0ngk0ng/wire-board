import { createPlayback } from "./playback";
import { useEffect } from "react";
import {
  defineGame,
  type OptionsProps,
} from "../../../web/src/game-api/module";
import { Board } from "./Board";
import { Cover } from "./Cover";
import type { Options, View } from "./types";
function OptionsForm({
  value,
  onChange,
  capacity,
  disabled,
}: OptionsProps<Options>) {
  useEffect(() => {
    if (value.teams.length !== capacity)
      onChange({
        ...value,
        teams: Array.from({ length: capacity }, (_, p) => p % 2),
      });
  }, [capacity]);
  const teams =
    capacity === 2
      ? [0, 1]
      : value.teams.length === 4
        ? value.teams
        : [0, 1, 0, 1];
  return (
    <div className="snooker-options">
      <label>
        比赛局数
        <select
          aria-label="比赛局数"
          value={value.bestOf}
          disabled={disabled}
          onChange={(e) => onChange({ ...value, bestOf: +e.target.value })}
        >
          <option value={1}>单局决胜</option>
          <option value={3}>三局两胜</option>
          <option value={5}>五局三胜</option>
        </select>
      </label>
      <label>
        瞄准辅助
        <select
          aria-label="瞄准辅助"
          value={value.aimAssist}
          disabled={disabled}
          onChange={(e) =>
            onChange({
              ...value,
              aimAssist: e.target.value as Options["aimAssist"],
            })
          }
        >
          <option value="line">基础辅助线</option>
          <option value="none">关闭辅助线</option>
        </select>
      </label>
      {capacity === 4 && (
        <label>
          双打分队
          <select
            aria-label="双打分队"
            value={teams.join(",")}
            disabled={disabled}
            onChange={(e) =>
              onChange({
                ...value,
                teams: e.target.value.split(",").map(Number),
              })
            }
          >
            <option value="0,1,0,1">1／3 号 A 队，2／4 号 B 队</option>
            <option value="0,0,1,1">1／2 号 A 队，3／4 号 B 队</option>
            <option value="0,1,1,0">1／4 号 A 队，2／3 号 B 队</option>
          </select>
        </label>
      )}
      <p>
        标准十五红球 · 操作限时 120 秒，超时自动托管，不额外罚分。
        {capacity === 4
          ? "双打按整次上手轮换；连续进球由同一位球员继续，原位重打仍由犯规者击球。"
          : "红彩交替，最后按黄、绿、棕、蓝、粉、黑清彩。"}
      </p>
    </div>
  );
}
export default defineGame<View, Options>({
  id: "snooker",
  name: "斯诺克",
  description:
    "纯 2D 俯视球桌。控制力度与旋转，进球走位、做斯诺克；支持单打和双打。",
  category: "sports",
  minPlayers: 2,
  maxPlayers: 4,
  playerCounts: [2, 4],
  requiresFullTable: true,
  supportsBots: true,
  ownsSounds: true,
  createPresentation: createPlayback,
  defaultOptions: { bestOf: 1, teams: [0, 1], aimAssist: "line" },
  Board,
  Cover,
  Options: OptionsForm,
  PlayerSummary: ({ room, data: d, seat }) => (
    <div className="snooker-public-summary">
      <strong>
        {d.teams[seat] === 0 ? "A" : "B"} 队 · {d.scores[d.teams[seat]]} 分
      </strong>
      <span>已赢 {d.framesWon[d.teams[seat]]} 局</span>
      <span>
        {d.phase === "frame_end"
          ? d.ready[seat]
            ? "已准备"
            : "待准备"
          : d.turn === seat && d.phase !== "finished"
            ? d.phase === "aim" || d.phase === "rolling"
              ? "当前击球者"
              : "等待确认"
            : `最高单杆 ${d.bestBreak[seat]}`}
        {!room.spectating && seat === room.you ? " · 你" : ""}
      </span>
    </div>
  ),
  Rules: () => (
    <div className="snooker-rules">
      <p>
        采用 WPBSA 标准十五红球：红球 1 分，黄、绿、棕、蓝、粉、黑依次为 2～7
        分。合法进红球后打一颗自选彩球，彩球复位；最后一颗红球后仍可打一颗彩球，再按黄至黑清彩。
      </p>
      <p>
        未进球交接球权；犯规给对方加 4～7
        分，同杆多项犯规取最高适用罚分。白球在手时从 D
        区摆放。自由球须符合犯规后被斯诺克的条件，并明确提名。
      </p>
      <p>
        解球未中可要求原位重打，罚分保留。具备完整直线路径时连续原位未中，第二次后警告，警告后再未中判负本局。球都停稳并完成彩球复位后才可击下一杆。
      </p>
      <p>
        仅剩黑球时第一次进球或犯规通常结束本局；结算后平分则黑球复位、抽签选择先打方。领先超过台面剩余分数不会自动结束比赛。
      </p>
      <p>
        双打每队两人，共用比分；按本局固定顺序轮换整次上手。连续进球不换搭档；要求犯规者重打时仍由本人击球。新一局两队交替开球，可以调整队内顺序。按官方规则，搭档在击球者进入击球状态后到本次上手结束前，请勿给予战术指挥。
      </p>
      <p>
        拖动球桌平移、滚轮或双指缩放；点击台面瞄准，单独调整力度及击点，再点击「击球」。
      </p>
      <p>
        本局认输只在有人需要罚分才能取胜时提出，双打须由搭档共同确认；对手可接受或继续。弃赛本场会判负整场比赛。仅剩黑球且当前一方领先超过
        7 分，可宣告本局胜利。
      </p>
      <details>
        <summary>自动裁判与素材说明</summary>
        <p>
          解球未中的自动判定结合直接可达性、经物理模型验证的隔库路线和当前分差；未能证实可执行路线时按普通犯规处理，不擅自追加
          Miss。现实比赛中关于尽力义务和行为意图的裁判判断，在此按公开的几何与物理判据执行。
        </p>
        <p>
          二维图像为项目原创；碰撞音效使用 Kenney 的 CC0
          素材加工，提示音为项目合成。声音由平台静音开关统一控制。
        </p>
        <p>
          <a
            href="https://kenney.nl/assets/impact-sounds"
            target="_blank"
            rel="noreferrer"
          >
            Kenney 音效来源
          </a>{" "}
          ·{" "}
          <a href="/snooker/v1/manifest.json" target="_blank" rel="noreferrer">
            素材与许可清单
          </a>
        </p>
      </details>
      <p>
        <a href="https://www.wpbsa.com/rules/" target="_blank" rel="noreferrer">
          规则来源：WPBSA 官方规则书
        </a>
      </p>
    </div>
  ),
});
