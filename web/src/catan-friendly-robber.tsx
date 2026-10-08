import { isPublicCatanSea } from "./catan-scenario-setup";
import { supportsExtendedBaseVariants } from "./catan-base-layout";
import { ShieldCheck } from "lucide-react";
import type { CatanState, Room } from "./types";
import "./catan-friendly-robber.css";
import { catanRuleContext, type CatanRuleContext } from "./catan-rule-context";

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
  const eligible =
    supportsExtendedBaseVariants(room) ||
    (room.capacity >= 3 &&
      room.capacity <= 6 &&
      room.capacity > 4 === !!room.catanOptions?.fiveSix &&
      !room.catanTwoRules &&
      !room.catanFishing &&
      (!room.catanScenario ||
        room.catanScenario === "cities-knights" ||
        !!room.catanSeafarers));
  if (room.catanFishing || (!setup && !eligible)) return null;
  const availability = room.catanFriendlyRobberAvailability;
  const reason = !eligible
    ? "此变体需要基础或已核验的航海地图，请检查地图与人数配置。"
    : !availability?.allowed
      ? availability?.reason || "该组合尚未开放。"
      : "";
  return (
    <CatanFriendlyRobberChoice
      value={!!setup?.enabled}
      onChange={(enabled) =>
        command("catan_friendly_robber", { catanFriendlyRobber: { enabled } })
      }
      disabled={disabled}
      reason={reason}
      scenario={catanRuleContext(room).scenario}
      knights={catanRuleContext(room).citiesKnights}
      needed={
        setup?.enabled
          ? Math.max(0, (availability?.minPlayers || 3) - room.seats.length)
          : 0
      }
    />
  );
}

export const catanFriendlyKnightsNote =
  "本站组合规则：首次蛮族入侵前强盗与海盗仍休眠；征税和骑士驱逐遵守友善保护。城市失守后按最新公开分数判断保护，保护不免除弃牌或城市劫掠。";

export const catanFriendlySeaFallbackNote =
  "本站补充规则：没有合法陆地且没有符合剧本限制的沙漠时，强盗退到场外且不偷牌；海盗已在外海且没有合法海洋时可留在外海。";

export const supportsPublicCatanFriendly = (scenario?: string) =>
  !scenario || scenario === "cities-knights" || isPublicCatanSea(scenario);

export function CatanFriendlyRobberChoice({
  value,
  onChange,
  disabled = false,
  reason = "",
  scenario = "",
  needed = 0,
  knights = false,
}: {
  value: boolean;
  onChange: (enabled: boolean) => void;
  disabled?: boolean;
  reason?: string;
  scenario?: string;
  needed?: number;
  knights?: boolean;
}) {
  return (
    <fieldset
      className="catan-helper-options catan-friendly-options"
      disabled={disabled}
    >
      <legend>商人与蛮族 · 友善强盗</legend>
      <label>
        <input
          type="checkbox"
          checked={value}
          disabled={disabled || (!value && !!reason)}
          onChange={(e) => onChange(e.target.checked)}
        />{" "}
        启用友善强盗
      </label>
      <p>
        {["cloth", "pirate_islands"].includes(scenario)
          ? "本剧本完成起始建设后，每人至少有3点建筑分，正常行动中不会触发友善保护。"
          : "公开分数不足3分的玩家受到保护，强盗不能放到其建筑旁。隐藏胜利点不计入判断。"}
      </p>
      <small>
        {reason || "不改变获胜门槛、弃牌规则或回合时间。更改后需要重新准备。"}
      </small>
      {value && isPublicCatanSea(scenario) && (
        <small>{catanFriendlySeaFallbackNote}</small>
      )}
      {value && (knights || scenario === "cities-knights") && (
        <small>{catanFriendlyKnightsNote}</small>
      )}
      {needed > 0 && (
        <strong>还需 {needed} 位玩家才能开局，可添加电脑。</strong>
      )}
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
      title="公开分数不足3分：强盗不能放到你的建筑旁，海盗不能放到你的船只旁，也不能偷取你的资源。隐藏胜利点不影响保护；强盗无合法地块时允许退回沙漠。"
    >
      <ShieldCheck size={14} aria-hidden="true" />{" "}
      {game.seafarers && !game.seafarers.wonders ? "强盗/海盗保护" : "强盗保护"}
    </span>
  );
}

export function CatanFriendlyRobberStatus({ room }: { room: Room }) {
  const g = room.game?.catan;
  if (!g?.friendlyRobber) return null;
  const outside =
    room.game?.phase === "catan_robber" && g.legal.robber?.includes(-1);
  const fallback =
    room.game?.phase === "catan_robber" && g.legal.robber?.includes(g.robber);
  return (
    <div className="catan-friendly-status">
      <ShieldCheck size={16} aria-hidden="true" />
      <strong>友善强盗</strong>
      <span>
        {outside
          ? "无合法陆地或沙漠，可点击“强盗退到场外”继续，不偷牌。"
          : fallback
            ? "没有其他合法地块，点击当前沙漠完成强盗处理。"
            : ["cloth", "pirate_islands"].includes(g.seafarers?.scenario || "")
              ? "三座村庄起步，正常行动不触发友善保护。"
              : g.seafarers && !g.seafarers.wonders
                ? "强盗与海盗均保护公开不足3分的玩家；隐藏胜利点不计。"
                : "公开不足3分受保护；隐藏胜利点不计。"}
      </span>
    </div>
  );
}

export function CatanFriendlyRobberRules({ info }: { info: CatanRuleContext }) {
  if (["cloth", "pirate_islands"].includes(info.scenario))
    return (
      <section>
        <h4>友善强盗</h4>
        {info.friendlyKnights && <p>{catanFriendlyKnightsNote}</p>}
        <p>
          本剧本从三座村庄起步，每人至少有3点建筑分，正常行动中不会触发“不足3分”的友善保护。即使失去布匹或港口奖励，建筑分仍至少为3。
        </p>
        <p>
          {info.scenario === "pirate_islands"
            ? "自动舰队巡航、袭击、掷7偷牌及骑士升级战舰，均按海盗群岛原规则处理。"
            : "强盗的主岛限制、建立贸易后才能移动海盗、资源或布匹偷取，均按卡坦布匹原规则处理。"}
        </p>
      </section>
    );

  return (
    <section>
      <h4>友善强盗</h4>
      {info.friendlyKnights && <p>{catanFriendlyKnightsNote}</p>}
      {info.helpers && (
        <p>
          友善保护只限制强盗与海盗，不限制助手交易或取牌。迪古尔按助手效果返回沙漠，即使旁边有受保护玩家；不偷牌。无沙漠时不可使用，不能改为场外驱逐。卡娅在强盗位于场外时不可使用。
        </p>
      )}
      <p>
        公开分数不足3分的玩家受到保护，包括行动玩家自己。强盗不能放到受保护玩家的村庄或城市旁，也不能向其偷取资源；隐藏胜利点不影响保护。公开分数达到3分后，保护立即消失。
      </p>
      <p>
        没有其他合法陆地时，强盗退回沙漠，即使沙漠旁有受保护者；只能向邻接且公开至少3分的对手偷牌。如果唯一退路就是当前沙漠，点击该沙漠即可完成处理。骑士牌仍可以用于移动强盗，即使没有可偷取的对手。
      </p>
      {info.scenario && info.scenario !== "wonders" && (
        <p>
          海盗同样受保护规则约束：不能放到公开不足3分玩家的船只旁。岸边只有村庄或城市、没有船只，不会阻止海盗进入该海洋。
        </p>
      )}
      {info.friendlySeaFallback && <p>{catanFriendlySeaFallbackNote}</p>}
      <p>保护不会免除掷出7后的弃牌，也不改变获胜门槛和回合时间。</p>
    </section>
  );
}
