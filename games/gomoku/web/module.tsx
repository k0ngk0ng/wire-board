import catalog from "./catalog";
import { defineGame } from "../../../web/src/game-api/module";
import { Board } from "./Board";
import { Cover } from "./Cover";
import type { Options, View } from "./types";
import "./style.css";
export default defineGame<View, Options>({
  ...catalog,
  defaultOptions: {
    rules: "freestyle",
    difficulty: "standard",
    turnSeconds: 120,
  },
  Board,
  Cover,
  Options: ({ value, onChange, disabled }) => (
    <div className="gomoku-options">
      <label>
        五子棋规则
        <select
          aria-label="五子棋规则"
          disabled={disabled}
          value={value.rules}
          onChange={(e) =>
            onChange({ ...value, rules: e.target.value as Options["rules"] })
          }
        >
          <option value="freestyle">无禁手 · 连五或以上</option>
          <option value="exact-five">无禁手 · 恰好五子</option>
        </select>
      </label>
      <label>
        电脑难度
        <select
          aria-label="五子棋电脑难度"
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
        每步时限（秒）
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
    <div className="gomoku-player-summary">
      <strong>
        <i className={seat === data.black ? "black" : "white"} />
        {seat === data.black ? "执黑" : "执白"}
        {seat === room.you ? " · 你" : ""}
      </strong>
      <small>
        已落{" "}
        {data.board.filter((c) => c === (seat === data.black ? 1 : 2)).length}{" "}
        子
      </small>
    </div>
  ),
  Rules: () => (
    <div>
      <p>
        十五路棋盘，开局随机分配黑白，黑棋先行。选中空交叉点后确认落子，双方交替行动。
      </p>
      <p>
        「连五或以上」允许长连获胜；「恰好五子」只认恰好连续五子，六子以上不获胜。两种模式均无黑方禁手，不包含连珠或
        Swap2 交换开局。
      </p>
      <p>
        满盘仍无人获胜则和棋。第二手后可在自己的回合提议和棋，每手最多一次，不暂停计时；对方同意才结束，继续落子会撤回提议。真人和棋不加减积分，不计胜场。任一方可认输。
      </p>
      <p>
        超时由电脑接管，可从平台入口取消托管。手机可拖动棋盘或概览定位；鼠标滚轮直接缩放，方向键移动焦点，回车选点后仍须确认。
      </p>
    </div>
  ),
});
