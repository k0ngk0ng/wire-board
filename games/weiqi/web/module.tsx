import { defineGame } from "../../../web/src/game-api/module";
import { Board } from "./Board";
import { Cover } from "./Cover";
import { createPlayback } from "./playback";
import type { Options, View } from "./types";
import "./style.css";
export default defineGame<View, Options>({
  id: "weiqi",
  name: "围棋",
  category: "board",
  description:
    "围地、提子，黑白交替落子。支持九路、十三路与十九路，双方确认死子后按面积计分，可与朋友或电脑对弈。",
  minPlayers: 2,
  maxPlayers: 2,
  requiresFullTable: true,
  supportsBots: true,
  ownsSounds: true,
  defaultOptions: {
    size: 19,
    komi: 7.5,
    difficulty: "standard",
    turnSeconds: 120,
  },
  Board,
  Cover,
  createPresentation: () => createPlayback(),
  Options: ({ value, onChange, disabled }) => (
    <div className="weiqi-options">
      <label>
        棋盘路数
        <select
          aria-label="围棋路数"
          disabled={disabled}
          value={value.size}
          onChange={(e) => onChange({ ...value, size: Number(e.target.value) })}
        >
          <option value={9}>九路</option>
          <option value={13}>十三路</option>
          <option value={19}>十九路</option>
        </select>
      </label>
      <label>
        白方贴目
        <select
          aria-label="围棋贴目"
          disabled={disabled}
          value={value.komi}
          onChange={(e) => onChange({ ...value, komi: Number(e.target.value) })}
        >
          <option value={7.5}>白贴 7.5 目</option>
          <option value={0}>不贴目</option>
        </select>
      </label>
      <label>
        电脑难度
        <select
          aria-label="围棋电脑难度"
          disabled={disabled}
          value={value.difficulty}
          onChange={(e) =>
            onChange({
              ...value,
              difficulty: e.target.value as Options["difficulty"],
            })
          }
        >
          <option value="easy">入门</option>
          <option value="standard">标准</option>
        </select>
      </label>
      <label>
        每步／确认时限（秒）
        <input
          type="number"
          min={30}
          max={300}
          step={10}
          disabled={disabled}
          value={value.turnSeconds}
          onChange={(e) =>
            onChange({ ...value, turnSeconds: Number(e.target.value) })
          }
        />
      </label>
    </div>
  ),
  PlayerSummary: ({ data, seat, room }) => (
    <div className="weiqi-player-summary">
      <strong>
        <i className={seat === data.black ? "black" : "white"} />
        {seat === data.black ? "黑方" : "白方"}
        {seat === room.you ? " · 你" : ""}
      </strong>
      <small>
        已提 {data.captured[seat]} 子
        {data.phase === "scoring"
          ? data.confirmed[seat]
            ? " · 已确认"
            : " · 待确认"
          : ""}
      </small>
    </div>
  ),
  Rules: () => (
    <div>
      <p>
        两人对弈，随机分配黑白，黑先。可选 9／13／19 路，默认白贴 7.5
        目，也可不贴目。采用面积计分、禁自杀、全局同形禁着，无让子。
      </p>
      <p>
        同色棋子沿上下左右相连，邻接空点为气。落子先提走无气的敌方棋块；己方仍无气时不能落子。落子后的整个黑白局面不得与本局历史重复，停着和继续对弈不清除重复历史。
      </p>
      <p>
        连续两次停着后，双方点选整块棋子标记死子，再确认同一份计分提案。修改标记会清除旧确认，不延长时限；有争议可继续对弈，由第二次停着后的应行动方继续。程序不自动判死活。
      </p>
      <p>
        双方确认后移除死子，按存活棋子数加仅邻接己方的空点数计分，白方再加贴目；邻接两色的公共空域不计分，提子数不额外加分。分数相同为和局，真人
        0 分、不计胜场。任何阶段都可以认输。
      </p>
      <p>
        超时由平台电脑托管，可随时取消。电脑提供无需额外服务的基本策略。手机先看全图，放大后选点并确认；支持鼠标滚轮、拖动、触屏双指及键盘方向键／回车。
      </p>
    </div>
  ),
});
