import { defineGame } from "../../../web/src/game-api/module";
import { Board } from "./Board";
import { coverSVG, colors, armyNames } from "../assets/art";
import { createPlayback } from "./playback";
import type { Options, View } from "./types";
import "./style.css";
export default defineGame<View, Options>({
  id: "junqi",
  name: "军棋",
  category: "board",
  description:
    "布阵藏锋，沿铁路调兵夺旗。支持两人陆战棋和四国对家搭档，暗子只向本人展示，可与朋友或电脑对战。",
  minPlayers: 2,
  maxPlayers: 4,
  playerCounts: [2, 4],
  requiresFullTable: true,
  supportsBots: true,
  ownsSounds: true,
  defaultOptions: {
    difficulty: "standard",
    turnSeconds: 120,
    setupSeconds: 180,
  },
  createPresentation: () => createPlayback(),
  Board,
  Cover: () => (
    <span
      className="junqi-cover"
      aria-hidden="true"
      dangerouslySetInnerHTML={{ __html: coverSVG }}
    />
  ),
  Options: ({ value, onChange, disabled, capacity }) => (
    <div className="junqi-options">
      <p>
        {capacity === 4
          ? "四国军棋：红黄对家，蓝绿对家。"
          : "两人陆战棋：两军对战，使用独立的南北棋盘。"}
      </p>
      <label>
        电脑难度
        <select
          aria-label="军棋电脑难度"
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
          <option value="standard">标准 · 更重视护旗</option>
        </select>
      </label>
      <label>
        布阵时限（秒）
        <input
          type="number"
          min={30}
          max={300}
          step={10}
          disabled={disabled}
          value={value.setupSeconds}
          onChange={(e) =>
            onChange({ ...value, setupSeconds: Number(e.target.value) })
          }
        />
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
  PlayerSummary: ({ room, data, seat }) => {
    const p = data.players[seat];
    const status =
      data.phase === "deploy"
        ? p.ready
          ? "已确认"
          : "待布阵"
        : p.flagRevealed
          ? "旗已公开"
          : data.mode === "teams"
            ? ""
            : "军旗未公开";
    return (
      <div className="junqi-player-summary">
        <strong>
          <i style={{ background: colors[seat] }} />
          <span>
            {armyNames[seat]}
            {room.you === seat ? " · 你" : ""}
          </span>
        </strong>
        <b>
          剩余 {p.remaining} 枚{p.alive ? "" : " · 已出局"}
        </b>
        <small>
          {data.mode === "teams"
            ? `对家${armyNames[(seat + 2) % 4]}${status ? " · " : ""}`
            : ""}
          {status}
        </small>
      </div>
    );
  },
  Rules: () => (
    <div>
      <p>
        每方 25
        枚棋子，暗子只向本人显示，队友和观众也看不到。军旗在后排大本营，地雷只能放最后两排，炸弹不能在前排，行营开局为空；可以交换己方棋子或随机布阵，全部确认后开始。
      </p>
      <p>
        普通道路走相邻一步，铁路可沿连续直道走多个空点，不能越子。四国四角弧线按连续边线处理；工兵还可在铁路交叉点转线，实际转弯后身份公开。行营内敌子不可攻击，任何棋子进入大本营后不能再走；地雷和军旗不移动。
      </p>
      <p>
        司令、军长、师长、旅长、团长、营长、连长、排长、工兵依次变小，大吃小，同级同归。工兵排雷并存活，其他普通子碰雷阵亡；炸弹与任何棋子同归，炸弹撞军旗也算夺旗。司令阵亡后公开军旗。
      </p>
      <p>
        军旗被夺、认输或轮到时无合法移动都会使该方出局并清除棋子。四国按对家分队，队友出局后仍可继续；仅剩一队即全队获胜。不能攻击或穿过盟友棋子。
      </p>
      <p>
        连续 200
        着无战损或局面与手番四次重复自动和棋，真人与电脑采用同一规则，正常胜负优先。存活玩家也可一致同意和棋，提议不暂停计时；真人和棋不加减积分，不计胜场。
      </p>
      <p>
        布阵和行棋采用休闲时限，超时托管，可从平台取消托管。鼠标左键拖动、滚轮缩放，手机拖动与双指缩放；概览可连续拖动定位，方向键移动焦点，回车选点后确认走子。
      </p>
    </div>
  ),
});
