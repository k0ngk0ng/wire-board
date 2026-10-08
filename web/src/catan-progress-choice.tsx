import { CatanProgressArt } from "./catan-progress";
import { useState } from "react";
import type { Act, Room } from "./types";
import { Bundle, CatanResource, ResourcePicker } from "./catan-resources";
import { catanProgressNames } from "./catan-progress-names";
import {
  progressChoiceAction,
  progressChoiceDue,
  progressChoiceHand,
  treasonStrengths,
} from "./catan-progress-choice-state";

// The parent keys this panel by response/actor/deadline, so queued or repeated
// responses never inherit a previous player's selection.
export function CatanProgressChoice({
  room,
  act,
  busy,
  assets,
  chosen,
}: {
  room: Room;
  act: Act;
  busy: boolean;
  assets: string;
  chosen: { type: string; id: number } | null;
}) {
  const [bundle, setBundle] = useState<number[]>(Array(8).fill(0));
  const [color, setColor] = useState<number | null>(null);
  const [card, setCard] = useState<number | null>(null);
  const [skip, setSkip] = useState(false);
  const g = room.game!.catan!,
    q = g.citiesKnights?.pending;
  if (
    !q ||
    room.spectating ||
    room.you < 0 ||
    q.players[0] !== room.you ||
    room.game!.finished ||
    g.players[room.you]?.eliminated
  )
    return null;
  const hand = progressChoiceHand(g, room.you),
    due = progressChoiceDue(g, room.you);
  const count = bundle.reduce((s, n) => s + n, 0);
  const action = progressChoiceAction(room, {
    bundle,
    color,
    card,
    map: chosen,
    skip,
  });
  const routeName = q.ship ? "船只" : "道路";
  const targetName = room.seats[q.target]?.name || "对方";
  const optional = ["diplomacy", "espionage", "treason_place"].includes(q.kind);
  return (
    <div className="catan-progress-response">
      {["guild_dues", "wedding", "sabotage"].includes(q.kind) && (
        <>
          <p>
            {q.kind === "guild_dues"
              ? `${targetName}的手牌仅向你展示。请选择${due}张拿入自己手中。`
              : q.kind === "wedding"
                ? `向${targetName}赠送${due}张资源或商品。`
                : `弃置一半资源与商品，向下取整，共${due}张；城墙不减免本次弃牌。`}
          </p>
          {q.kind === "guild_dues" && <Bundle values={hand} assets={assets} />}
          <ResourcePicker
            label={
              q.kind === "guild_dues"
                ? "拿取"
                : q.kind === "wedding"
                  ? "赠送"
                  : "弃置"
            }
            values={bundle}
            onChange={setBundle}
            limits={hand.map((n, i) =>
              Math.min(n, bundle[i] + Math.max(0, due - count)),
            )}
            assets={assets}
            disabled={busy}
          />
          <p aria-live="polite">
            已选 {count} / {due} 张
          </p>
        </>
      )}
      {q.kind === "commercial_harbor" && (
        <>
          <p>
            {targetName}给你一张
            <CatanResource color={q.color} assets={assets} small />
            ，请选择一张自己持有的商品交换。
          </p>
          <div className="catan-city-choice-cards">
            {[5, 6, 7].map((n) => (
              <button
                key={n}
                disabled={busy || !hand[n]}
                aria-pressed={color === n}
                onClick={() => setColor(n)}
              >
                <CatanResource
                  color={n}
                  count={hand[n] || 0}
                  assets={assets}
                  small
                />
              </button>
            ))}
          </div>
        </>
      )}
      {q.kind === "espionage" && (
        <>
          <p>
            {targetName}
            的进步手牌仅向你展示。选择一张拿入手中，获得后按正常阶段规则即可使用。
          </p>
          <div className="catan-city-choice-cards">
            {(q.progress || []).map((n, i) => (
              <button
                key={i}
                disabled={busy || [9, 23].includes(n)}
                aria-pressed={!skip && card === n}
                onClick={() => {
                  setSkip(false);
                  setCard(n);
                }}
              >
                <CatanProgressArt card={n} assets={assets} />
                {catanProgressNames[n]}
              </button>
            ))}
          </div>
        </>
      )}
      {q.kind === "diplomacy" && (
        <p>
          在地图上选择亮起的{routeName}位置，免费重放刚移除的{routeName}
          ，也可以放弃重放。
        </p>
      )}
      {q.kind === "treason_remove" && (
        <p>
          {q.source === "two_neutral"
            ? "在地图上选择该中立势力最弱的骑士移除，随后可放置自己同级或更低级的骑士。"
            : "在地图上选择自己的一名骑士移除。对方随后可以放置同级或更低级骑士，并继承激活状态。"}
        </p>
      )}
      {q.kind === "treason_place" && (
        <>
          <p>
            被移除的是{q.knight?.strength}级
            {q.knight?.active ? "已激活" : "未激活"}
            骑士。选择有库存的等级，再在亮起的空交点放置。
            {q.knight?.active && "放置后可立即行动。"}
          </p>
          <div className="catan-city-choice-cards">
            {[1, 2, 3].map((n) => (
              <button
                key={n}
                disabled={busy || !treasonStrengths(g, room.you).includes(n)}
                aria-pressed={!skip && color === n}
                onClick={() => {
                  setSkip(false);
                  setColor(n);
                }}
              >
                {n}级骑士
              </button>
            ))}
          </div>
        </>
      )}
      {["diplomacy", "treason_remove", "treason_place"].includes(q.kind) && (
        <p aria-live="polite">
          {chosen?.type === q.kind
            ? `已选${q.kind === "diplomacy" ? routeName : "交点"} #${chosen.id + 1}`
            : "请在地图上选择目标"}
        </p>
      )}
      {optional && (
        <label className="catan-progress-skip">
          <input
            type="checkbox"
            checked={skip}
            disabled={busy}
            onChange={(e) => setSkip(e.target.checked)}
          />
          {q.kind === "espionage"
            ? "放弃取牌"
            : q.kind === "diplomacy"
              ? `放弃免费重放${routeName}`
              : "放弃免费放置骑士"}
        </label>
      )}
      {skip && <p>确认后将结束本次选择，不获得上述奖励。</p>}
      <button
        className="primary wide"
        disabled={busy || !action}
        onClick={() => {
          if (action) void act(action);
        }}
      >
        {skip ? "确认放弃" : "确认选择"}
      </button>
    </div>
  );
}
