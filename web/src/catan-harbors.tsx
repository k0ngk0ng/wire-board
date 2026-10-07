import { supportsExtendedBaseVariants } from "./catan-base-layout";
import { Anchor, Award } from "lucide-react";
import type { CatanState, Room } from "./types";
import { catanRuleContext } from "./catan-rule-context";
import "./catan-harbors.css";
import { isPublicCatanSea } from "./catan-scenario-setup";

export const supportsPublicCatanHarbors = (scenario?: string) =>
  !scenario || scenario === "cities-knights" || isPublicCatanSea(scenario);

export function CatanHarborsPicker({
  room,
  disabled,
  command,
}: {
  room: Room;
  disabled: boolean;
  command: (type: string, extra?: Record<string, unknown>) => void;
}) {
  const setup = room.catanHarbors;
  const available =
    supportsExtendedBaseVariants(room) ||
    (room.capacity >= 3 &&
      room.capacity <= 4 &&
      !room.catanFishing &&
      !room.catanTwoRules &&
      !room.catanOptions?.fiveSix &&
      supportsPublicCatanHarbors(room.catanScenario));
  if (room.catanFishing || (!setup && !available)) return null;
  return (
    <CatanHarborsChoice
      value={!!setup?.enabled}
      disabled={disabled || (!available && !setup?.enabled)}
      helpers={!!room.catanOptions?.helpers}
      target={catanRuleContext(room).target}
      onChange={(enabled) =>
        command("catan_harbors", { catanHarbors: { enabled } })
      }
    />
  );
}

export function CatanHarborsChoice({
  value,
  onChange,
  disabled = false,
  helpers = false,
  target,
}: {
  value: boolean;
  onChange: (enabled: boolean) => void;
  disabled?: boolean;
  helpers?: boolean;
  target: number;
}) {
  return (
    <fieldset
      className="catan-helper-options catan-harbors-options"
      disabled={disabled}
    >
      <legend>商人与蛮族 · 港口霸主</legend>
      <label>
        <input
          type="checkbox"
          checked={value}
          disabled={disabled || helpers}
          onChange={(e) => onChange(e.target.checked)}
        />{" "}
        启用港口霸主
      </label>
      <p>
        港口村庄计1点、城市计2点。率先达到3点获得2分奖励；只有超过持有者，才能夺取奖励。
      </p>
      {value && <strong>本局分数门槛：{target}分</strong>}
      <small>
        {helpers
          ? "请先关闭 Helpers，再启用港口霸主。"
          : "获胜门槛增加1分，剧本的其他结束条件保留。更改后需要重新准备。"}
      </small>
    </fieldset>
  );
}

export function CatanHarborsSeat({
  game,
  seat,
}: {
  game: CatanState;
  seat: number;
}) {
  const h = game.harbors;
  if (!h) return null;
  const owner = h.owner === seat;
  return (
    <span
      className={`catan-harbors-seat ${owner ? "holder" : ""}`}
      title="港口村庄1点，城市和大都会2点；奖励牌值2胜利分。"
    >
      <Anchor size={13} aria-hidden="true" /> 港口 {h.points[seat] || 0}点
      {owner && (
        <b>
          <Award size={13} aria-hidden="true" />
          霸主 +2分
        </b>
      )}
    </span>
  );
}

export function CatanHarborsStatus({ room }: { room: Room }) {
  const h = room.game?.catan?.harbors;
  if (!h) return null;
  return (
    <div className="catan-harbors-status">
      <Anchor size={16} aria-hidden="true" />
      <strong>港口霸主</strong>
      <span>
        {h.owner >= 0
          ? `${room.seats[h.owner]?.name || "玩家"}持有 · ${h.points[h.owner]}港口点 · 奖励2分`
          : "暂无持有者 · 率先达到3港口点获奖"}
      </span>
    </div>
  );
}

export function CatanHarborsRules() {
  return (
    <section className="catan-harbors-rules">
      <h4>港口霸主</h4>
      <p>
        已安放港口旁的村庄计1港口点，城市和大都会计2港口点。率先达到3点，获得价值2胜利分的奖励牌；超过持有者即可夺取，同分保留原持有者。港口点本身不直接计入胜利分。
      </p>
      <p>
        遗忘部落领取但尚未安放的港口不计点。城市被劫掠或玩家离场后会重新计算归属。本局获胜门槛已增加1分，奇迹4级、布匹耗尽和夺回要塞等条件不变。
      </p>
    </section>
  );
}
