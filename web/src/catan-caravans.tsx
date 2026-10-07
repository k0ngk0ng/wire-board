import { useEffect, useState } from "react";
import type { CatanState, CatanWagon, Room } from "./types";
import { CatanResource } from "./catan-resources";
import { catanSeatColor } from "./catan-player-colors";
import {
  caravanBonus,
  caravanPlacementStep,
  caravanCanRespond,
  caravanGeometry,
  caravanPick,
  caravanSelected,
} from "./catan-caravans-state";
import type { CaravanSelection } from "./catan-caravans-state";
import { useCaravanMotion } from "./catan-caravans-motion";
import "./catan-caravans.css";

const art = (assets: string, name: string) =>
  `${assets}/catan/caravans/${name}-v1.webp`;
const votes = (bid: number[] | null) =>
  (bid || []).reduce((sum, n) => sum + n, 0);

function Wagon({
  g,
  wagon,
  assets,
  preview = false,
}: {
  g: CatanState;
  wagon: CatanWagon;
  assets: string;
  preview?: boolean;
}) {
  const geometry = caravanGeometry(g, wagon);
  if (!geometry) return null;
  const scale = (g.hexSize || 62) / 62;
  return (
    <g
      className={`caravan-wagon ${preview ? "preview" : ""}`}
      pointerEvents="none"
      transform={`translate(${geometry.x},${geometry.y}) rotate(${geometry.rotation}) scale(${scale})`}
    >
      <g data-caravan-piece={preview ? undefined : wagon.edge}>
        {preview && (
          <rect
            x={-14}
            y={-22}
            width={28}
            height={44}
            rx={7}
            fill="#ffeb90"
            stroke="#724417"
            strokeWidth={2}
          />
        )}
        {assets ? (
          <image
            href={art(assets, "wagon-south")}
            x={-11}
            y={-18}
            width={22}
            height={35}
            preserveAspectRatio="xMidYMid meet"
          />
        ) : (
          <path
            d="M-8-15H8V9H4V15H-4V9H-8Z"
            fill="#92968e"
            stroke="#3c423c"
            strokeWidth={2}
          />
        )}
        <path
          d="M-4 20L0 24L4 20"
          fill="none"
          stroke={preview ? "#714110" : "#fff5da"}
          strokeWidth={2.6}
          strokeLinecap="round"
        />
      </g>
    </g>
  );
}

export function CatanCaravanMap({
  room,
  assets,
  busy,
  selected,
  onSelect,
}: {
  room: Room;
  assets: string;
  busy: boolean;
  selected: CaravanSelection | null;
  onSelect: (s: CaravanSelection | null) => void;
}) {
  const g = room.game!.catan!,
    c = g.caravans;
  const root = useCaravanMotion(room);
  if (!c) return null;
  const selecting =
    caravanCanRespond(room) && c.pending?.kind !== "bid" && !busy;
  const scale = (g.hexSize || 62) / 62;
  const chosen = caravanSelected(g, selected);
  const edges = [...new Set((c.choices || []).map((w) => w.edge))];
  return (
    <g ref={root} className="caravan-map">
      {!c.pending &&
        c.map.starts
          .filter(
            (w) =>
              !c.wagons.some(
                (placed) => placed.edge === w.edge || placed.from === w.from,
              ),
          )
          .map((w) => {
            const p = caravanGeometry(g, w);
            return (
              p && (
                <g
                  key={w.edge}
                  pointerEvents="none"
                  transform={`translate(${p.from.x + p.dx * 12 * scale},${p.from.y + p.dy * 12 * scale}) rotate(${p.rotation}) scale(${scale})`}
                >
                  <path d="M-4-3L0 3L4-3" className="caravan-start" />
                  <title>商队起点：向水源外出发</title>
                </g>
              )
            );
          })}
      {c.wagons.map((w) => (
        <Wagon key={w.edge} g={g} wagon={w} assets={assets} />
      ))}
      {selecting &&
        edges.map((id) => {
          const e = g.edges[id],
            a = g.vertices[e.a],
            b = g.vertices[e.b];
          const pick = () => onSelect(caravanPick(g, id));
          return (
            <g
              key={id}
              className={`caravan-candidate ${selected?.edge === id ? "picked" : ""}`}
              role="button"
              tabIndex={0}
              aria-label={`选择商队路线 ${id + 1}`}
              onClick={pick}
              onKeyDown={(event) => {
                if (event.key === "Enter" || event.key === " ") {
                  event.preventDefault();
                  pick();
                }
              }}
            >
              <line
                x1={a.x + (b.x - a.x) * 0.22}
                y1={a.y + (b.y - a.y) * 0.22}
                x2={a.x + (b.x - a.x) * 0.78}
                y2={a.y + (b.y - a.y) * 0.78}
              />
              <circle
                cx={(a.x + b.x) / 2}
                cy={(a.y + b.y) / 2}
                r={18 * scale}
                fill="transparent"
                stroke="none"
              />
            </g>
          );
        })}
      {selecting && chosen && (
        <>
          <Wagon g={g} wagon={chosen} assets={assets} preview />
          <g
            className="caravan-origin"
            pointerEvents="none"
            transform={`translate(${g.vertices[chosen.from].x},${g.vertices[chosen.from].y}) scale(${scale})`}
          >
            <circle r={10} />
            <text textAnchor="middle" dominantBaseline="central">
              起
            </text>
          </g>
        </>
      )}
    </g>
  );
}

export function CatanCaravanBonuses({ g }: { g: CatanState }) {
  if (!g.caravans) return null;
  const scale = (g.hexSize || 62) / 62;
  return (
    <g className="caravan-building-bonuses" pointerEvents="none">
      {g.vertices
        .filter((v) => v.owner >= 0 && v.level > 0 && caravanBonus(g, v.id))
        .map((v) => (
          <g
            key={v.id}
            data-caravan-bonus={v.id}
            transform={`translate(${v.x + 14 * scale},${v.y + 13 * scale}) scale(${scale})`}
          >
            <circle r={10} />
            <text textAnchor="middle" dominantBaseline="central">
              +1
            </text>
            <title>商队建筑额外1分</title>
          </g>
        ))}
    </g>
  );
}

export function CatanCaravanPanel({
  room,
  assets,
  busy,
  act,
  selected,
  onSelect,
}: {
  room: Room;
  assets: string;
  busy: boolean;
  act: (a: Record<string, unknown>) => Promise<unknown>;
  selected: CaravanSelection | null;
  onSelect: (s: CaravanSelection | null) => void;
}) {
  const g = room.game!.catan!,
    c = g.caravans,
    q = c?.pending;
  const step = caravanPlacementStep(g);
  const [collapsed, setCollapsed] = useState(false);
  const [bid, setBid] = useState([0, 0, 0, 0, 0]);
  useEffect(() => {
    setCollapsed(false);
    setBid([0, 0, 0, 0, 0]);
  }, [
    room.id,
    c?.sequence,
    c?.actor,
    q?.kind,
    step,
    room.you,
    room.spectating,
  ]);
  useEffect(() => {
    if (selected) setCollapsed(false);
  }, [selected]);
  if (!c) return null;
  const mine = caravanCanRespond(room);
  const choice = caravanSelected(g, selected);
  const alternatives = (c.choices || []).filter(
    (w) => w.edge === selected?.edge,
  );
  const hand = g.players[room.you]?.resources || [0, 0, 0, 0, 0];
  const validBid = bid.every((n, i) => n >= 0 && n <= hand[i]);
  return (
    <>
      <section className="caravan-stock" aria-label="商队马车供给">
        {assets && <img src={art(assets, "wagon")} alt="" />}
        <div>
          <strong>商队 · 还剩 {c.remaining} 辆马车</strong>
          <small>
            {c.built
              ? "本次已建建筑，结束行动后投票放车"
              : "建造建筑后投票放车 · 12分获胜"}
          </small>
        </div>
      </section>
      {q && room.status === "playing" && !room.game!.finished && (
        <section
          className="catan-gold-choice caravan-choice"
          aria-label="商队投票"
        >
          <header>
            <strong>
              {mine
                ? q.kind === "bid"
                  ? "商队：出价"
                  : q.kind === "vote"
                    ? "商队：分配选票"
                    : step
                      ? `商队：第${step}辆马车`
                      : "商队：决定位置"
                : `${room.seats[c.actor]?.name} 正在${q.kind === "bid" ? "出价" : step ? `放置第${step}辆马车` : "选位"}`}
            </strong>
            <button
              aria-expanded={!collapsed}
              onClick={() => setCollapsed(!collapsed)}
            >
              {collapsed ? "展开" : "收起"}
            </button>
          </header>
          {!collapsed && (
            <div className="catan-gold-body">
              <p>
                {q.kind === "bid"
                  ? "每张羊毛或粮食算1票，可不出价。确认后无法更改，放车结束后归还银行。"
                  : q.kind === "vote"
                    ? "可在聊天中协商，再把全部选票投给一个位置；确认后无法更改。"
                    : mine
                      ? "由你决定马车位置与前进方向。"
                      : `等待${room.seats[c.actor]?.name}决定马车位置与前进方向。`}{" "}
                每次响应120秒，超时自动处理。
              </p>
              {q.two && (
                <>
                  <p>
                    {q.kind === "bid"
                      ? "双方各出价一次，尽量放满两辆马车。票多者分别延伸不同商队；平票时各放一辆，本回合玩家先放。"
                      : c.placementLimit === 1
                        ? "由当前决定者放置本轮唯一能放的马车。"
                        : votes(q.bids[0]) === votes(q.bids[1])
                          ? `双方平票，各放一辆；${room.seats[q.active]?.name}先放。`
                          : "票多者连续决定两辆马车，第二辆须延伸另一支商队。"}
                    会合后的商队视为同一支。出价在本轮放车结束后一起归还银行。
                  </p>
                  <p>
                    本站补充规则：最多放两辆；只有无法放满时才少放。
                    {c.placementLimit === 1
                      ? "本轮只能放一辆，确认后结束放车。"
                      : "若能放满，第一辆只可选择能继续放第二辆的位置。"}
                  </p>
                  {step && (
                    <ol className="caravan-steps" aria-label="本轮马车进度">
                      {(c.placementLimit === 1 ? [1] : [1, 2]).map((n) => (
                        <li
                          key={n}
                          className={
                            n < step ? "done" : n === step ? "current" : ""
                          }
                          aria-current={n === step ? "step" : undefined}
                        >
                          <b>{n < step ? "✓" : n}</b>
                          <span>
                            第{n}辆
                            {n < step
                              ? "已放置"
                              : n === step
                                ? "待确认"
                                : "待放置"}
                          </span>
                        </li>
                      ))}
                    </ol>
                  )}
                  {q.two.first && (
                    <small>
                      第一辆：路线 #{q.two.first.edge + 1} ·{" "}
                      {caravanGeometry(g, q.two.first)?.direction}
                    </small>
                  )}
                </>
              )}
              <div className="caravan-bids">
                {q.order.map((p) => (
                  <div key={p} className={c.actor === p ? "responding" : ""}>
                    <b>
                      <i style={{ background: catanSeatColor(g, p) }} />
                      {room.seats[p]?.name}
                    </b>
                    <span>
                      {q.bids[p] === null ? "未出价" : `${votes(q.bids[p])}票`}
                    </span>
                    {q.bids[p] && votes(q.bids[p]) > 0 && (
                      <div className="caravan-bid-cards">
                        {[2, 3]
                          .filter((color) => q.bids[p]![color] > 0)
                          .map((color) => (
                            <CatanResource
                              key={color}
                              color={color}
                              count={q.bids[p]![color]}
                              assets={assets}
                              small
                            />
                          ))}
                      </div>
                    )}
                    {q.votes[p] && (
                      <small>
                        路线 #{q.votes[p]!.edge + 1} ·{" "}
                        {caravanGeometry(g, q.votes[p]!)?.direction}
                      </small>
                    )}
                  </div>
                ))}
              </div>
              {mine && q.kind === "bid" && (
                <>
                  <div className="caravan-bid-picker">
                    {[2, 3].map((color) => (
                      <div key={color}>
                        <CatanResource color={color} assets={assets} small />
                        <small>持有 {hand[color]}</small>
                        <div className="catan-stepper">
                          <button
                            aria-label={`出价减少${color === 2 ? "羊毛" : "粮食"}`}
                            disabled={busy || bid[color] === 0}
                            onClick={() =>
                              setBid(
                                bid.map((n, i) => (i === color ? n - 1 : n)),
                              )
                            }
                          >
                            −
                          </button>
                          <output>{bid[color]}</output>
                          <button
                            aria-label={`出价增加${color === 2 ? "羊毛" : "粮食"}`}
                            disabled={busy || bid[color] >= hand[color]}
                            onClick={() =>
                              setBid(
                                bid.map((n, i) => (i === color ? n + 1 : n)),
                              )
                            }
                          >
                            ＋
                          </button>
                        </div>
                      </div>
                    ))}
                  </div>
                  <button
                    className="primary caravan-submit"
                    disabled={busy || !validBid}
                    onClick={() =>
                      act({ type: "catan_caravan_bid", tokens: bid })
                    }
                  >
                    {votes(bid) ? `确认出价 · ${votes(bid)}票` : "不出价，继续"}
                  </button>
                </>
              )}
              {mine && q.kind !== "bid" && (
                <>
                  <p>
                    {selected
                      ? `已选择路线 #${selected.edge + 1}${choice ? ` · ${caravanGeometry(g, choice)?.direction}` : "，请选择前进方向"}`
                      : "收起面板，点击地图上亮起的路线，再确认。"}
                  </p>
                  {alternatives.length > 1 && (
                    <div className="caravan-directions">
                      {alternatives.map((w) => (
                        <button
                          key={w.from}
                          className={
                            selected?.from === w.from ? "selected" : ""
                          }
                          disabled={busy}
                          onClick={() => onSelect(w)}
                        >
                          {caravanGeometry(g, w)?.direction}
                        </button>
                      ))}
                    </div>
                  )}
                  <div className="cloth-confirm-actions">
                    {selected && (
                      <button disabled={busy} onClick={() => onSelect(null)}>
                        重选位置
                      </button>
                    )}
                    <button
                      className="primary"
                      disabled={busy || !choice}
                      onClick={async () => {
                        if (!choice) return;
                        await act({
                          type: `catan_caravan_${q.kind}`,
                          edge: choice.edge,
                          vertex: choice.from,
                        });
                        onSelect(null);
                      }}
                    >
                      {q.kind === "vote"
                        ? `确认投出全部 ${votes(q.bids[room.you])} 票`
                        : step
                          ? `确认放置第${step}辆马车`
                          : "确认放置马车"}
                    </button>
                  </div>
                </>
              )}
            </div>
          )}
        </section>
      )}
    </>
  );
}
