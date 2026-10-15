import { defineGame } from "../../../web/src/game-api/module";
import { Board } from "./Board";
import { coverSVG } from "../assets/art";
import { createPlayback } from "./playback";
import type { Options, View } from "./types";
import "./style.css";
export default defineGame<View, Options>({
  id: "chess",
  name: "国际象棋",
  category: "board",
  description:
    "王、后、车、象、马与兵各展所长。通过将杀取胜，支持易位、吃过路兵、四种升变与规则和棋，可与朋友或电脑对弈。",
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
      className="chess-cover"
      aria-hidden="true"
      dangerouslySetInnerHTML={{ __html: coverSVG }}
    />
  ),
  Options: ({ value, onChange, disabled }) => (
    <div className="chess-options">
      <label>
        电脑难度
        <select
          aria-label="国际象棋电脑难度"
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
    <div className="chess-player-summary">
      <strong>
        {seat === data.white ? "⚪ 执白" : "⚫ 执黑"}
        {room.you === seat ? " · 你" : ""}
      </strong>
      <small>吃掉 {data.captured[seat]?.length ?? 0} 子</small>
    </div>
  ),
  Rules: () => (
    <div>
      <p>
        标准 8 × 8
        棋盘，随机分配黑白，白方先走。以将杀对方的王获胜，不能让自己的王受攻击。支持王车易位、吃过路兵及升变为后、车、象或马。
      </p>
      <p>
        无合法走法但未被将军是逼和；双方无法将杀的死局也和棋。三次重复局面或双方连续
        50
        回合不动兵、不吃子可申请和棋，支持先选择一步预告着法；预告申请成立时不实际走子。五次重复或连续
        75 回合自动和棋，将杀优先。
      </p>
      <p>
        可随走子提议和棋，对方同意才结束；提议和回应不暂停或重置计时。真人和棋双方
        0 积分，不计胜场。可随时认输；对手无法通过任何合法走子将杀时按和棋处理。
      </p>
      <p>
        采用休闲每步时限，不是比赛总时钟。超时由电脑接管，可通过平台取消托管。电脑遵守相同规则，无需额外服务。
      </p>
    </div>
  ),
});
