import { ShieldCheck } from "lucide-react";
import type { CatanState, Room } from "./types";
import "./catan-friendly-robber.css";

export function CatanFriendlyRobberPicker({
  room,
  disabled,
  command,
}: {
  room: Room;
  disabled: boolean;
  command: (type: string, extra?: Record<string, unknown>) => void;
}) {
  const setup = room.catanFriendlyRobber;
  if (!setup) return null;
  const incompatible = !!(
    room.catanSeafarers ||
    room.catanNewWorldMap ||
    room.catanCitiesKnights ||
    room.catanOptions?.helpers ||
    room.catanOptions?.allHelpers
  );
  return (
    <fieldset
      className="catan-helper-options catan-friendly-options"
      disabled={disabled}
    >
      <legend>商人与蛮族 · 友善强盗</legend>
      <label>
        <input
          type="checkbox"
          checked={setup.enabled}
          disabled={disabled || (!setup.enabled && incompatible)}
          onChange={(e) =>
            command("catan_friendly_robber", {
              catanFriendlyRobber: { enabled: e.target.checked },
            })
          }
        />{" "}
        启用友善强盗
      </label>
      <p>
        公开分数不足3分的玩家受到保护，强盗不能放到其建筑旁。隐藏胜利点不计入判断。
      </p>
      <small>
        {incompatible
          ? "目前仅支持基础版，可叠加港口霸主。请先关闭其他扩展。"
          : "不改变获胜门槛、弃牌规则或回合时间。更改后需要重新准备。"}
      </small>
    </fieldset>
  );
}

export function CatanFriendlyRobberSeat({
  game,
  seat,
}: {
  game: CatanState;
  seat: number;
}) {
  if (!game.friendlyRobber?.protectedPlayers.includes(seat)) return null;
  return (
    <span
      className="catan-friendly-seat"
      title="公开分数不足3分：强盗不能放到你的建筑旁，也不能偷取你的资源。隐藏胜利点不影响保护；无合法地块时允许退回沙漠。"
    >
      <ShieldCheck size={14} aria-hidden="true" /> 强盗保护
    </span>
  );
}

export function CatanFriendlyRobberStatus({ room }: { room: Room }) {
  const g = room.game?.catan;
  if (!g?.friendlyRobber) return null;
  const fallback =
    room.game?.phase === "catan_robber" && g.legal.robber?.includes(g.robber);
  return (
    <div className="catan-friendly-status">
      <ShieldCheck size={16} aria-hidden="true" />
      <strong>友善强盗</strong>
      <span>
        {fallback
          ? "没有其他合法地块，点击当前沙漠完成强盗处理。"
          : "公开不足3分受保护；隐藏胜利点不计。"}
      </span>
    </div>
  );
}

export function CatanFriendlyRobberRules() {
  return (
    <section>
      <h4>友善强盗</h4>
      <p>
        公开分数不足3分的玩家受到保护，包括行动玩家自己。强盗不能放到受保护玩家的村庄或城市旁，也不能向其偷取资源；隐藏胜利点不影响保护。公开分数达到3分后，保护立即消失。
      </p>
      <p>
        没有其他合法陆地时，强盗退回沙漠，即使沙漠旁有受保护者；只能向邻接且公开至少3分的对手偷牌。如果唯一退路就是当前沙漠，点击该沙漠即可完成处理。骑士牌仍可以用于移动强盗，即使没有可偷取的对手。
      </p>
      <p>保护不会免除掷出7后的弃牌，也不改变获胜门槛和回合时间。</p>
    </section>
  );
}
