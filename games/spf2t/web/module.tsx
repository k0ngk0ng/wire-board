import {
  defineGame,
  type GameRoom,
  type OptionsProps,
} from "../../../web/src/game-api/module";
import { Board } from "./Board";
import { Cover } from "./Cover";
import { characterName, type Options, type View } from "./types";
import { warningClass } from "./gems";
import "./style.css";

function OptionsForm({ value, onChange, capacity, disabled }: OptionsProps<Options>) {
  return (
    <div className="spf2t-options">
      <label>
        赛制
        <select
          aria-label="赛制"
          disabled={disabled}
          value={value.format ?? "bestOf3"}
          onChange={(event) =>
            onChange({ format: event.target.value as Options["format"] })
          }
        >
          <option value="bestOf3">三局两胜 · 先赢 2 回合</option>
          <option value="single">单局决胜 · 快节奏</option>
        </select>
      </label>
      <p>
        {capacity === 2
          ? "固定 2 人对战；可添加电脑席位。电脑参与的对局不计积分，真人托管仍按真人计分。"
          : "街霸方块固定 2 人。"}
      </p>
    </div>
  );
}

function Rules() {
  return (
    <div className="spf2t-rules">
      <p>
        双方各有一块 6×12 棋盘。每回合落下一对宝石，移动、旋转、软降或硬降；
        同色宝石上下左右连通后不会自动消除，需要用同色碎裂宝石接触触发。
      </p>
      <p>
        同色普通宝石形成 2×2 或更大的实心矩形会合并为强化宝石，整体下落、
        整体被同色碎裂宝石清除，面积越大攻击越强。每第 25 对会出现一颗钻石：
        落在空地上无效，落在宝石上会清除全盘该颜色，攻击减半，送出的反击宝石计时为 3。
      </p>
      <p>
        消除会把反击宝石送入对手的待落队列；对手每落下一对，计时减 1，
        归零后变成普通宝石。消除可以抵消自己的待落反击宝石。
        每个角色有独立的 12×6 反击宝石图案，开局前双方选角，可选同一角色。
      </p>
      <p>
        连锁会提高攻击；同一步同时清除多组、清空棋盘都会获得额外奖励。
        三局两胜先赢 2 回合；单局决胜先赢 1 回合。每回合一方无法生成下一对即失败。
        攻击数值为平台平衡近似。
      </p>
      <p>
        键盘：←／→ 移动，↑／X 顺时针，Z／Ctrl 逆时针，↓ 软降，空格硬降。
        手机使用棋盘下方按钮；按住左右移动或软降会连续触发。
        刷新或重连不会重置棋盘、角色、回合、待落或期限。
      </p>
      <p>
        电脑会独立完成完整对局；主动开启托管后由电脑接管本席，空闲不会自动托管；
        取消托管不会重置棋盘、回合或其他席位。
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
  if (!player || !data) return null;
  const name = player.selected ? characterName(data, player.character) : "未选角";
  return (
    <div className="spf2t-summary" data-spf2t-summary={seat}>
      <strong>{name}</strong>
      <span>
        回合 {player.roundWins} / {data.roundsToWin}
      </span>
      {player.pendingCount > 0 ? (
        <span
          className={`spf2t-summary-warning ${warningClass(player.pendingCount)}`}
        >
          {player.warning} {player.pendingCount}
        </span>
      ) : null}
      <span>{room.seats[seat]?.name ?? `玩家 ${seat + 1}`}</span>
    </div>
  );
}

export default defineGame<View, Options>({
  id: "spf2t",
  name: "街霸方块",
  description:
    "双人对战消消乐：同色宝石连成方块，用碎裂宝石引爆连锁，把反击宝石送进对手场地。",
  category: "puzzle",
  minPlayers: 2,
  maxPlayers: 2,
  playerCounts: [2],
  requiresFullTable: true,
  supportsBots: true,
  ownsSounds: true,
  realtime: true,
  defaultOptions: { format: "bestOf3" },
  Board,
  Cover,
  Options: OptionsForm,
  Rules,
  PlayerSummary,
});
