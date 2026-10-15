import { defineGame } from "../../../web/src/game-api/module";
import { Board } from "./Board";
import { coverSVG } from "../assets/art";
import { createPlayback } from "./playback";
import type { Options, View } from "./types";
import "./style.css";
export default defineGame<View, Options>({
  id: "xiangqi",
  name: "中国象棋",
  category: "board",
  description:
    "楚河汉界，两军对弈。车马炮协同将杀，支持长将、长捉与循环提示，可与朋友或电脑对弈。",
  minPlayers: 2,
  maxPlayers: 2,
  requiresFullTable: true,
  supportsBots: true,
  ownsSounds: true,
  defaultOptions: { difficulty: "standard", turnSeconds: 120 },
  createPresentation: () => createPlayback(),
  Board,
  Cover: () => (
    <span
      className="xiangqi-cover"
      aria-hidden="true"
      dangerouslySetInnerHTML={{ __html: coverSVG }}
    />
  ),
  Options: ({ value, onChange, disabled }) => (
    <div className="xiangqi-options">
      <label>
        电脑难度
        <select
          aria-label="中国象棋电脑难度"
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
  PlayerSummary: ({ room, data, seat }) => (
    <div className="xiangqi-player-summary">
      <strong className={seat === data.red ? "red" : "black"}>
        {seat === data.red ? "● 执红" : "● 执黑"}
        {room.you === seat ? " · 你" : ""}
      </strong>
      <small>吃掉 {data.captured[seat]?.length ?? 0} 子</small>
    </div>
  ),
  Rules: () => (
    <div>
      <p>
        九路十行棋盘，随机分配红黑，红先。帅将与仕士留在九宫，相象不过河，马有蹩腿、炮隔一子吃子；兵卒过河后可以横走，始终不能后退。不能将帅照面或送将。
      </p>
      <p>
        将杀或困毙对方获胜。循环按长将、长捉及许可重复区别裁定：第三次相同局面提示有责方变着，第四次仍重复时裁定；长将责任高于长捉，双方同责或均不犯例则和棋。将帅、兵卒、未过河被捉兵卒、真根、同类互吃及马炮捉有根车按对应例外处理。
      </p>
      <p>
        连续 50 回合、双方共 100
        着未吃子自动和棋，动兵不清零；将杀和困毙优先。可随走子提议和棋，不改变行动时限；对方同意才结束。真人和棋双方
        0 积分，不计胜场，也可认输结束。
      </p>
      <p>
        采用休闲每步时限，不是比赛总时钟。超时由平台电脑托管，可随时取消托管。支持鼠标、键盘方向键与回车、手机触屏，选点后确认才走子。
      </p>
    </div>
  ),
});
