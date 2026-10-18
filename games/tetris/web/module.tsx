import {
  defineGame,
  type GameRoom,
  type OptionsProps,
} from "../../../web/src/game-api/module";
import { Board } from "./Board";
import { Cover } from "./Cover";
import { progressText } from "./format";
import type { Options, View } from "./types";

function OptionsForm({ value, onChange, capacity, disabled }: OptionsProps<Options>) {
  return (
    <div className="tetris-options">
      <label>
        对战模式
        <select
          aria-label="对战模式"
          disabled={disabled}
          value={value.mode}
          onChange={(event) =>
            onChange({ mode: event.target.value as Options["mode"] })
          }
        >
          <option value="survival">生存赛 · 消行攻击，最后存活</option>
          <option value="sprint">40 行冲刺 · 先清 40 行</option>
        </select>
      </label>
      <p>
        {capacity === 1
          ? "单人练习：成绩会保存，但不计积分。"
          : "2～4 人对战；电脑玩家参与的对局不计积分。"}
      </p>
    </div>
  );
}

function Rules() {
  return (
    <div className="tetris-rules">
      <p>
        1 人房间是单人练习，2～4 人房间是对战。生存赛通过消行向对手发送垃圾行，
        最后存活的玩家获胜；40 行冲刺各自清理 40 行，先完成者获胜，冲刺模式不发送垃圾行。
      </p>
      <p>
        普通消行 1／2／3／4 行分别发送 0／1／2／4 行垃圾；垃圾行会进入对手的待落队列，
        在对手下一次锁块后从底部升起。消行也可以抵消自己待落的垃圾行。
      </p>
      <p>
        计分：单行 100、双行 300、三行 500、四行 800，并乘以当前等级；
        软降每格 +1，硬降每格 +2。每 10 行提升一级，下落速度加快。
      </p>
      <p>
        键盘：←／→ 移动，↑／X 顺时针旋转，Z／Ctrl 逆时针旋转，↓ 软降，
        空格硬降，C／Shift 暂存。手机可使用下方按钮，按住移动或软降会连续触发。
      </p>
      <p>
        电脑会独立完成两种模式；自己超时或开启托管后由电脑接管本席。
        对局最长 10 分钟，刷新或重连不会重置棋盘、队列或期限。
      </p>
    </div>
  );
}

function PlayerSummary({
  room,
  data,
  seat,
}: {
  room: GameRoom;
  data?: View;
  seat: number;
  assets: string;
}) {
  const player = data?.players?.[seat];
  const mode = data?.options.mode;
  if (!player || !mode) return null;
  return (
    <div
      className="tetris-player-summary"
      data-tetris-seat={seat}
      title={`${room.seats[seat]?.name ?? `玩家 ${seat + 1}`}：${
        player.alive ? "存活" : "已出局"
      }`}
    >
      <strong>
        {player.alive ? "存活" : "已出局"}
        {room.you === seat ? " · 你" : ""}
      </strong>
      <span>{progressText(mode, player.lines)}</span>
      <span>{player.score} 分</span>
      <span>待落 {player.pendingCount}</span>
    </div>
  );
}

export default defineGame<View, Options>({
  id: "tetris",
  name: "俄罗斯方块",
  description:
    "经典落块对战：旋转与消行，向对手送去垃圾行，或抢先完成 40 行。",
  category: "puzzle",
  minPlayers: 1,
  maxPlayers: 4,
  playerCounts: [1, 2, 3, 4],
  supportsBots: true,
  ownsSounds: true,
  realtime: true,
  defaultOptions: { mode: "survival" },
  Board,
  Cover,
  Options: OptionsForm,
  Rules: Rules,
  PlayerSummary,
});
