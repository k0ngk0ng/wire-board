import { useEffect, useState } from "react";
import type { Act, Room } from "./types";
import { CatanResource } from "./catan-resources";
import {
  catanCardEventAction,
  catanCardEventActor,
  catanEventNames,
} from "./catan-card-event-state";
import "./catan-card-event.css";

export function CatanCardEventChoice({
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
  const [collapsed, setCollapsed] = useState(false);
  const [color, setColor] = useState<number | null>(null);
  const [target, setTarget] = useState<number | null>(null);
  useEffect(() => {
    if (chosen) setCollapsed(false);
  }, [chosen?.type, chosen?.id]);
  const g = room.game?.catan,
    q = g?.cardEvent;
  if (!g || !q || room.game?.finished) return null;
  const mine = catanCardEventActor(room),
    name = catanEventNames[q.kind] || "事件牌";
  const giver = q.kind === "good_neighbors" || q.kind === "helpful_neighbor";
  const resource = ["plentiful_year", "calm_seas", "tournament"].includes(
    q.kind,
  );
  const thief = q.kind === "conflict" || q.kind === "trade_advantage";
  const mapChoice = q.kind === "earthquake" || q.kind === "robber_flees";
  const action = catanCardEventAction(room, { color, target, map: chosen });
  const own = q.ownGift;
  const recipient = own
    ? room.seats[own.to]?.name || `玩家${own.to + 1}`
    : "左邻";
  const explanation =
    q.kind === "earthquake"
      ? "在地图上选择一条自己的完好道路。受损道路需修复后才能继续修建新路。"
      : q.kind === "robber_flees"
        ? "选择地图上亮起的沙漠。强盗返回沙漠，不偷牌。"
        : q.kind === "good_neighbors"
          ? `选一张牌交给${recipient}，所有人选完后统一转交。`
          : q.kind === "helpful_neighbor"
            ? "选一位分数较低的玩家，交给他自己的一张牌。"
            : thief
              ? "选择一位对手，随机偷取一张牌；牌的颜色不向其他玩家公开。"
              : resource
                ? "从银行选择一张普通资源。所有获奖者选择完毕后再生产。"
                : "先结算事件，再按牌上点数生产。";
  const colors = giver
    ? g.legal.eventGifts || []
    : resource
      ? g.legal.eventResources || []
      : [];
  return (
    <section
      className={`catan-card-event ${collapsed ? "collapsed" : ""}`}
      aria-label="事件牌回应"
    >
      <header>
        <strong>
          {name}
          <small>生产点数 {q.production}</small>
        </strong>
        <button
          type="button"
          onClick={() => setCollapsed(!collapsed)}
          aria-expanded={!collapsed}
          aria-label={collapsed ? "展开事件牌" : "收起事件牌"}
        >
          {collapsed ? "展开" : "收起"}
        </button>
      </header>
      {!collapsed && (
        <div className="catan-card-event-body">
          <div className="catan-card-event-summary">
            {assets && catanEventNames[q.kind] && (
              <img
                src={`${assets}/catan/events/${q.kind}-v1.webp`}
                alt={`${name}原版插画`}
                onError={(e) => {
                  e.currentTarget.hidden = true;
                }}
              />
            )}
            <p>
              {mine
                ? explanation
                : `等待${room.seats[q.players[0]]?.name || "玩家"}完成选择。`}
            </p>
          </div>
          {!mine && own && own.color >= 0 && (
            <p>
              你已选择 <CatanResource color={own.color} assets={assets} small />{" "}
              交给{recipient}，等待统一转交。
            </p>
          )}
          {mine && (
            <>
              {mapChoice && (
                <p className="catan-event-map-hint">
                  {action && chosen
                    ? `已选${q.kind === "earthquake" ? "道路" : "沙漠"} #${chosen.id + 1}`
                    : "点击地图上亮起的位置，再确认。可以收起本面板查看完整地图。"}
                </p>
              )}
              {!!colors.length && (
                <div
                  className="catan-event-resources"
                  role="group"
                  aria-label={giver ? "选择交出的牌" : "选择银行资源"}
                >
                  {colors.map((c) => (
                    <button
                      key={c}
                      type="button"
                      disabled={busy}
                      aria-pressed={color === c}
                      onClick={() => setColor(c)}
                    >
                      <CatanResource
                        color={c}
                        assets={assets}
                        count={
                          giver
                            ? g.players[room.you]?.resources?.[c] || 0
                            : g.bank[c]
                        }
                        small
                      />
                    </button>
                  ))}
                </div>
              )}
              {(thief || q.kind === "helpful_neighbor") && (
                <div
                  className="catan-event-targets"
                  role="group"
                  aria-label={thief ? "选择偷牌对手" : "选择接收玩家"}
                >
                  {(g.legal.eventTargets || []).map((p) => (
                    <button
                      key={p}
                      type="button"
                      disabled={busy}
                      aria-pressed={target === p}
                      onClick={() => setTarget(p)}
                    >
                      <strong>{room.seats[p]?.name || `玩家${p + 1}`}</strong>
                      <small>
                        {g.players[p].publicScore} 分 · 手牌{" "}
                        {g.players[p].resourceCount} 张
                      </small>
                    </button>
                  ))}
                </div>
              )}
              <footer>
                {q.canSkip && (
                  <button
                    disabled={busy}
                    onClick={() => {
                      const skip = catanCardEventAction(room, {
                        color,
                        target,
                        map: chosen,
                        skip: true,
                      });
                      if (skip) void act(skip);
                    }}
                  >
                    放弃偷牌
                  </button>
                )}
                <button
                  className="primary"
                  disabled={busy || !action}
                  onClick={() => {
                    if (action) void act(action);
                  }}
                >
                  确认
                  {giver ? "交牌" : thief ? "偷牌" : resource ? "领取" : "选择"}
                </button>
              </footer>
            </>
          )}
        </div>
      )}
    </section>
  );
}
