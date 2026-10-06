import { useEffect, useState } from "react";
import type { CSSProperties, KeyboardEvent } from "react";
import { Anchor, Dices, Minus, Plus, RotateCcw, Ship } from "lucide-react";
import type { Act, Room } from "./types";
import { Bundle, ResourcePicker } from "./catan-resources";
import {
  ExplorerPiece,
  ExplorerEffects,
  useExplorerMotion,
} from "./catan-explorer-effects";
import {
  catanColorIndex,
  catanPieceColors,
  catanSeatColor,
} from "./catan-player-colors";
import { useRailMapControls } from "./rail-map-controls";
import {
  explorerActionNames,
  explorerActionDescription,
  explorerActionKey,
  explorerChoices,
  explorerCanRespond,
  explorerSelectedAction,
  explorerDiscardAction,
  explorerContents,
  explorerShipPosition,
  explorerTarget,
  explorerResources,
} from "./catan-explorer-state";
import type { ExplorerAction, ExplorerPick } from "./catan-explorer-state";
import "./catan-explorer.css";

const empty = () => [0, 0, 0, 0, 0];
const total = (n: number[]) => n.reduce((sum, x) => sum + x, 0);
const terrainColors = [
  "#436c39",
  "#b36b41",
  "#9baf55",
  "#d6b763",
  "#909496",
  "#d9c390",
  "#4f9eb8",
  "#d5b047",
  "#dbe8e9",
];
const terrainNames = [
  ...explorerResources,
  "沙漠",
  "海洋",
  "金矿",
  "未探索区域",
];
const primaryTypes = ["catan_roll", "catan_explorer_begin_move", "catan_end"];
const buttonKeys = (event: KeyboardEvent, click: () => void) => {
  if (event.key === "Enter" || event.key === " ") {
    event.preventDefault();
    click();
  }
};

export function CatanExplorerBoard({
  room,
  act,
  busy,
  assets,
}: {
  room: Room;
  act: Act;
  busy: boolean;
  assets: string;
}) {
  const game = room.game!,
    g = game.catan!,
    x = g.explorer!,
    you = room.you;
  const motion = useExplorerMotion(room);
  const hand = g.players[you]?.resources || empty();
  const [pick, setPick] = useState<ExplorerPick | null>(null);
  const [collapsed, setCollapsed] = useState(false);
  const [mode, setMode] = useState("");
  const [ship, setShip] = useState(-1);
  const [discard, setDiscard] = useState(empty);
  const [give, setGive] = useState(empty),
    [take, setTake] = useState(empty);
  const [goldGive, setGoldGive] = useState(0),
    [goldTake, setGoldTake] = useState(0);
  const [error, setError] = useState("");
  useEffect(() => {
    setPick(null);
    setMode("");
    setShip(-1);
    setDiscard(empty());
    setGive(empty());
    setTake(empty());
    setGoldGive(0);
    setGoldTake(0);
    setError("");
  }, [room.id, x.sequence, game.phase, you]);
  const can = explorerCanRespond(room),
    choices = explorerChoices(room);
  const kinds = [...new Set(choices.map((a) => a.type))].filter(
    (type) => !primaryTypes.includes(type),
  );
  const effective = kinds.includes(mode)
    ? mode
    : kinds.includes("catan_explorer_sail")
      ? "catan_explorer_sail"
      : kinds[0] || "";
  const options = choices.filter(
    (a) =>
      a.type === effective &&
      (ship < 0 || a.slot === undefined || a.slot === ship),
  );
  const selected = explorerSelectedAction(room, pick);
  const select = (a: ExplorerAction) => {
    setPick({ room: room.id, action: a });
    setCollapsed(false);
    setError("");
  };
  const send = async (a: Record<string, unknown>) => {
    if (busy) return;
    setError("");
    try {
      await act(a);
      setPick(null);
    } catch (e) {
      setError(e instanceof Error ? e.message : "操作未成功，请重试");
    }
  };
  const size = g.hexSize || 62;
  const minX = Math.min(...g.vertices.map((v) => v.x)) - 30,
    minY = Math.min(...g.vertices.map((v) => v.y)) - 30;
  const width = Math.max(...g.vertices.map((v) => v.x)) - minX + 30,
    height = Math.max(...g.vertices.map((v) => v.y)) - minY + 30;
  const controls = useRailMapControls({
    aspect: width / height,
    minMobileWidth: 0,
    fillMobileHeight: false,
  });
  const targets = new Map<
    string,
    { kind: "edge" | "vertex"; id: number; actions: ExplorerAction[] }
  >();
  for (const a of options) {
    const target = explorerTarget(g, a);
    if (!target) continue;
    const key = `${target.kind}-${target.id}`;
    const item = targets.get(key) || { ...target, actions: [] };
    item.actions.push(a);
    targets.set(key, item);
  }
  const selectedTarget = selected ? explorerTarget(g, selected) : null;
  const alternatives = selectedTarget
    ? options.filter((a) => {
        const p = explorerTarget(g, a);
        return p?.kind === selectedTarget.kind && p.id === selectedTarget.id;
      })
    : [];
  const discardAction = explorerDiscardAction(room, discard);
  const due = g.discardDue[you] || 0;
  const trade = g.trade;
  const mayOffer = can && game.turn === you && game.phase === "catan_turn";
  const gold = x.economy.gold[you] || 0;
  const offerValid =
    mayOffer &&
    total(give) + goldGive > 0 &&
    total(take) + goldTake > 0 &&
    goldGive <= gold &&
    [goldGive, goldTake].every(
      (n) => Number.isInteger(n) && n >= 0 && n <= 148,
    ) &&
    give.every((n, i) => n <= hand[i]);
  const phase = game.finished
    ? "本局已结束"
    : game.phase === "catan_roll"
      ? "掷骰生产"
      : game.phase === "catan_discard"
        ? "所有人同时弃牌"
        : game.phase === "catan_turn"
          ? "交易与建设"
          : "船只航行";
  const path =
    selected?.type === "catan_explorer_sail"
      ? [x.fleet.positions[selected.slot!], ...selected.targets!]
          .map((id) => {
            const e = g.edges[id],
              a = g.vertices[e.a],
              b = g.vertices[e.b];
            return `${(a.x + b.x) / 2},${(a.y + b.y) / 2}`;
          })
          .join(" ")
      : "";
  return (
    <section className="explorer-board" aria-label="探索者与海盗初航">
      <div className="explorer-map-column">
        <header className="explorer-heading">
          <div>
            <strong>探索者与海盗 · 初航</strong>
            <span>{phase} · 目标8分</span>
          </div>
          <div
            className="explorer-dice"
            role="img"
            aria-label={`骰子${g.dice.join("、")}`}
          >
            <Dices size={20} />
            {g.dice.join(" + ")}
          </div>
        </header>
        <div className="explorer-map-tools">
          <span>
            {options.length && targets.size
              ? `选择高亮位置：${explorerActionNames[effective]}`
              : "滚轮缩放 · 拖动地图"}
          </span>
          <button
            aria-label="缩小地图"
            disabled={controls.zoom <= 1}
            onClick={() => controls.zoomAt(controls.zoom / 1.2)}
          >
            <Minus size={16} />
          </button>
          <button
            aria-label="放大地图"
            disabled={controls.zoom >= 3}
            onClick={() => controls.zoomAt(controls.zoom * 1.2)}
          >
            <Plus size={16} />
          </button>
          <button aria-label="重置地图" onClick={() => controls.zoomAt(1)}>
            <RotateCcw size={16} />
          </button>
        </div>
        <div
          ref={controls.viewport}
          className={`map-scroll explorer-map-scroll ${controls.dragging ? "is-dragging" : ""}`}
          style={{ "--explorer-map-aspect": width / height } as CSSProperties}
          {...controls.handlers}
        >
          <div className="map-canvas" style={controls.canvasStyle}>
            <svg
              className="explorer-map"
              viewBox={`${minX} ${minY} ${width} ${height}`}
              style={controls.mapStyle}
              role="group"
              aria-label="初航地图"
            >
              <defs>
                {g.tiles.map((t) => (
                  <clipPath id={`explorer-hex-${t.id}`} key={t.id}>
                    <polygon
                      points={t.vertices
                        .map((v) => `${g.vertices[v].x},${g.vertices[v].y}`)
                        .join(" ")}
                    />
                  </clipPath>
                ))}
              </defs>
              {g.tiles.map((t) => (
                <g key={t.id}>
                  <polygon
                    points={t.vertices
                      .map((v) => `${g.vertices[v].x},${g.vertices[v].y}`)
                      .join(" ")}
                    fill={terrainColors[t.resource]}
                    stroke="#e3d5b4"
                    strokeWidth="1.4"
                  />
                  {assets && t.resource !== 8 && (
                    <image
                      href={`${assets}/catan/${t.resource < 6 ? `terrain-${["wood", "brick", "wool", "grain", "ore", "desert"][t.resource]}` : `seafarers/terrain-${t.resource === 6 ? "sea" : "gold"}`}-v1.webp`}
                      x={t.x - (size * Math.sqrt(3)) / 2}
                      y={t.y - size}
                      width={size * Math.sqrt(3)}
                      height={size * 2}
                      clipPath={`url(#explorer-hex-${t.id})`}
                      preserveAspectRatio="xMidYMid slice"
                    />
                  )}
                  {t.resource === 8 && (
                    <text x={t.x} y={t.y + 8} className="explorer-fog">
                      ?
                    </text>
                  )}
                  {t.number > 0 && (
                    <g
                      className={`explorer-number ${[6, 8].includes(t.number) ? "hot" : ""}`}
                    >
                      <circle cx={t.x} cy={t.y} r={size * 0.22} />
                      <text x={t.x} y={t.y + size * 0.095}>
                        {t.number}
                      </text>
                    </g>
                  )}
                  <title>
                    {terrainNames[t.resource]}
                    {t.number > 0 ? ` · ${t.number}` : ""}
                  </title>
                </g>
              ))}
              {g.edges
                .filter((e) => e.owner !== -1)
                .map((e) => {
                  const a = g.vertices[e.a],
                    b = g.vertices[e.b];
                  return (
                    <line
                      key={e.id}
                      x1={a.x * 0.8 + b.x * 0.2}
                      y1={a.y * 0.8 + b.y * 0.2}
                      x2={b.x * 0.8 + a.x * 0.2}
                      y2={b.y * 0.8 + a.y * 0.2}
                      stroke={catanSeatColor(g, e.owner)}
                      strokeWidth="7"
                      strokeLinecap="round"
                    />
                  );
                })}
              {g.vertices
                .filter((v) => v.level > 0)
                .map((v) => (
                  <g
                    key={v.id}
                    transform={`translate(${v.x} ${v.y})`}
                    pointerEvents="none"
                  >
                    {v.level === 2 ? (
                      <ExplorerPiece
                        g={g}
                        assets={assets}
                        player={v.owner}
                        kind="harbor"
                        width={34}
                      />
                    ) : assets ? (
                      <image
                        href={`${assets}/catan/${v.level === 2 ? "city" : "settlement"}-${catanPieceColors[catanColorIndex(g, v.owner)]}-v1.webp`}
                        x="-17"
                        y="-24"
                        width="34"
                        height="32"
                      />
                    ) : (
                      <path
                        d={
                          v.level === 2
                            ? "M-15 8V-8H-5V-18H6V-8H15V8Z"
                            : "M-12 8V-6L0-17L12-6V8Z"
                        }
                        fill={catanSeatColor(g, v.owner)}
                        stroke="#433624"
                        strokeWidth="2"
                      />
                    )}
                    {explorerContents(g, "harbor", v.id).length > 0 && (
                      <g
                        className={
                          motion?.event.cargo?.some(
                            (c) =>
                              c.to.kind === "harbor" && c.to.index === v.id,
                          )
                            ? "explorer-cargo-arriving"
                            : undefined
                        }
                        transform="translate(0,20)"
                      >
                        <ExplorerPiece
                          g={g}
                          assets={assets}
                          player={v.owner}
                          kind="settler"
                          width={18}
                        />
                      </g>
                    )}
                    <title>
                      {v.owner < 0 ? "中立" : room.seats[v.owner]?.name} ·{" "}
                      {v.level === 2 ? "港口" : "村庄"} · 位置{v.id + 1}
                    </title>
                  </g>
                ))}
              {path && (
                <polyline
                  points={path}
                  className="explorer-voyage-preview"
                  pointerEvents="none"
                />
              )}
              {x.fleet.positions.map((at, id) => {
                const p = explorerShipPosition(g, id);
                if (
                  at < 0 ||
                  !p ||
                  (motion?.event.kind === "catan_explorer_sail" &&
                    motion.event.ship === id)
                )
                  return null;
                return (
                  <g
                    key={id}
                    transform={`translate(${p.x} ${p.y})`}
                    className="explorer-vessel"
                    pointerEvents="none"
                  >
                    <ExplorerPiece
                      g={g}
                      assets={assets}
                      player={Math.floor(id / 3)}
                      kind="ship"
                    />
                    {explorerContents(g, "ship", id).length > 0 && (
                      <g
                        className={
                          motion?.event.cargo?.some(
                            (c) => c.to.kind === "ship" && c.to.index === id,
                          )
                            ? "explorer-cargo-arriving"
                            : undefined
                        }
                        transform="translate(0,-12)"
                      >
                        <ExplorerPiece
                          g={g}
                          assets={assets}
                          player={Math.floor(id / 3)}
                          kind="settler"
                          width={18}
                        />
                      </g>
                    )}
                    <text y="30" className="explorer-ship-label">
                      {(id % 3) + 1}
                    </text>
                    <title>
                      {room.seats[Math.floor(id / 3)]?.name} · 船{(id % 3) + 1}
                    </title>
                  </g>
                );
              })}
              <ExplorerEffects active={motion} assets={assets} />
              {!busy &&
                [...targets.entries()].map(([key, item]) => {
                  const chosen =
                    selectedTarget?.kind === item.kind &&
                    selectedTarget.id === item.id;
                  const click = () => select(item.actions[0]);
                  const props = {
                    role: "button",
                    tabIndex: 0,
                    "aria-label": `${explorerActionNames[effective]}${item.kind === "edge" ? (effective === "catan_road" ? "道路" : "海边") : "位置"}${item.id + 1}`,
                    onClick: click,
                    onKeyDown: (e: KeyboardEvent) => buttonKeys(e, click),
                  };
                  if (item.kind === "vertex") {
                    const v = g.vertices[item.id];
                    return (
                      <circle
                        key={key}
                        className={`explorer-target ${chosen ? "picked" : ""}`}
                        cx={v.x}
                        cy={v.y}
                        r="14"
                        {...props}
                      />
                    );
                  }
                  const e = g.edges[item.id],
                    a = g.vertices[e.a],
                    b = g.vertices[e.b];
                  return (
                    <g
                      key={key}
                      className={`explorer-target-edge ${chosen ? "picked" : ""}`}
                      {...props}
                    >
                      <line x1={a.x} y1={a.y} x2={b.x} y2={b.y} />
                      <line
                        className="hit"
                        x1={a.x}
                        y1={a.y}
                        x2={b.x}
                        y2={b.y}
                      />
                    </g>
                  );
                })}
            </svg>
          </div>
        </div>
        <div className="explorer-map-caption">
          未探索：鹦鹉区{x.board.unexplored[0]}块 · 鹅区{x.board.unexplored[1]}
          块 <span>港口2分，村庄1分 · 港口每格仍产1资源</span>
        </div>
      </div>
      <aside className="explorer-panel">
        {room.spectating ? (
          <p className="explorer-notice">
            正在观战 · 船只与货物公开，手牌仅本人可见
          </p>
        ) : (
          <>
            <div className="explorer-hand">
              <strong>你的资源</strong>
              <b className="explorer-gold">金币 {gold}</b>
              <Bundle values={hand} assets={assets} showZero />
            </div>
            {room.seats[you]?.autoPlay && (
              <p className="explorer-notice">电脑正在托管，请先收回操作权。</p>
            )}
          </>
        )}
        {game.phase === "catan_discard" && can && due > 0 && (
          <section className="explorer-discard">
            <ResourcePicker
              label={`归还资源（共${due}张）`}
              values={discard}
              onChange={setDiscard}
              limits={hand}
              assets={assets}
              disabled={busy}
            />
            <button
              disabled={busy || !discardAction}
              onClick={() => discardAction && void send(discardAction)}
            >
              确认归还 {total(discard)}/{due}
            </button>
          </section>
        )}
        {choices.length > 0 && (
          <>
            <div className="explorer-primary">
              {choices
                .filter((a) => primaryTypes.includes(a.type))
                .map((a) => (
                  <button
                    key={a.type}
                    disabled={busy}
                    onClick={() =>
                      a.type === "catan_roll" ? void send(a) : select(a)
                    }
                  >
                    {explorerActionNames[a.type]}
                  </button>
                ))}
            </div>
            <div className="explorer-modes" role="group" aria-label="探险操作">
              {kinds.map((type) => (
                <button
                  key={type}
                  disabled={busy}
                  aria-pressed={effective === type}
                  onClick={() => {
                    setMode(type);
                    setPick(null);
                    setShip(-1);
                  }}
                >
                  {explorerActionNames[type]}
                </button>
              ))}
            </div>
            {game.phase === "catan_explorer_move" && (
              <div
                className="explorer-fleet"
                role="group"
                aria-label="选择船只"
              >
                <button
                  aria-pressed={ship < 0}
                  onClick={() => {
                    setShip(-1);
                    setPick(null);
                  }}
                >
                  全部船只
                </button>
                {x.fleet.positions.map((at, id) =>
                  at >= 0 && Math.floor(id / 3) === you ? (
                    <button
                      key={id}
                      aria-pressed={ship === id}
                      onClick={() => {
                        setShip(id);
                        setPick(null);
                      }}
                    >
                      <Ship size={16} />船{(id % 3) + 1} ·{" "}
                      {x.fleet.turn?.ships[id].closed
                        ? "已停止"
                        : `${x.fleet.turn?.ships[id].remaining ?? 0}步`}
                      {explorerContents(g, "ship", id).length
                        ? " · 移民"
                        : " · 空舱"}
                    </button>
                  ) : null,
                )}
              </div>
            )}
            {targets.size > 0 && (
              <p className="explorer-hint">
                点击地图高亮位置，查看费用与操作后确认。
              </p>
            )}
            <div className="explorer-option-list">
              {options
                .filter((a) => !explorerTarget(g, a))
                .map((a) => (
                  <button
                    key={explorerActionKey(a)}
                    disabled={busy}
                    onClick={() => select(a)}
                  >
                    {explorerActionDescription(g, a)}
                  </button>
                ))}
            </div>
          </>
        )}
        {selected && (
          <section
            className="explorer-confirm"
            role="dialog"
            aria-label="确认探险操作"
            onKeyDown={(e) => {
              if (e.key === "Escape") setPick(null);
            }}
          >
            <header className="explorer-confirm-heading">
              <strong>{explorerActionNames[selected.type]}</strong>
              <button
                aria-label={collapsed ? "展开操作确认" : "收起操作确认"}
                aria-expanded={!collapsed}
                onClick={() => setCollapsed(!collapsed)}
              >
                {collapsed ? "展开" : "收起"}
              </button>
            </header>
            <div hidden={collapsed}>
              {alternatives.length > 1 && (
                <label>
                  选择船只或舱位
                  <select
                    value={explorerActionKey(selected)}
                    onChange={(e) => {
                      const a = alternatives.find(
                        (a) => explorerActionKey(a) === e.target.value,
                      );
                      if (a) select(a);
                    }}
                  >
                    {alternatives.map((a) => (
                      <option
                        key={explorerActionKey(a)}
                        value={explorerActionKey(a)}
                      >
                        {a.type === "catan_explorer_unit"
                          ? `移民${(a.card! % 11) + 1} · ${a.choice === "ship" ? `船${(a.target! % 3) + 1}` : `港口${a.target! + 1}`}${a.cards?.length ? "（替换原移民）" : ""}`
                          : `船${(a.slot! % 3) + 1}${a.give?.length && a.take?.length ? " · 交换" : a.give?.length ? " · 装载" : a.take?.length ? " · 卸载" : ""}`}
                      </option>
                    ))}
                  </select>
                </label>
              )}
              <p>{explorerActionDescription(g, selected)}</p>
              <div className="explorer-confirm-actions">
                <button disabled={busy} onClick={() => setPick(null)}>
                  取消
                </button>
                <button
                  className="primary"
                  disabled={busy}
                  onClick={() => {
                    const fresh = explorerSelectedAction(room, pick);
                    if (fresh) void send(fresh);
                  }}
                >
                  确认{explorerActionNames[selected.type]}
                </button>
              </div>
            </div>
          </section>
        )}
        {error && (
          <p role="alert" className="explorer-error">
            {error}
          </p>
        )}
        {trade && (
          <section className="explorer-trade">
            <strong>{room.seats[trade.from]?.name}提出交易</strong>
            <p>付出</p>
            <Bundle values={trade.give} assets={assets} />
            {!!trade.goldGive && <span>金币 {trade.goldGive}</span>}
            <p>希望收到</p>
            <Bundle values={trade.take} assets={assets} />
            {!!trade.goldTake && <span>金币 {trade.goldTake}</span>}
            {can && game.phase === "catan_turn" && (
              <div className="explorer-trade-buttons">
                {trade.from === you ? (
                  <>
                    <button
                      disabled={busy}
                      onClick={() =>
                        void send({
                          type: "catan_trade_cancel",
                          offer: trade.id,
                          prompt: x.sequence,
                        })
                      }
                    >
                      取消报价
                    </button>
                    {trade.responses.map((r, p) =>
                      p !== you && r === 1 ? (
                        <button
                          key={p}
                          disabled={busy}
                          onClick={() =>
                            void send({
                              type: "catan_trade_complete",
                              offer: trade.id,
                              target: p,
                              prompt: x.sequence,
                            })
                          }
                        >
                          与{room.seats[p]?.name}成交
                        </button>
                      ) : null,
                    )}
                  </>
                ) : (
                  <>
                    <button
                      disabled={
                        busy ||
                        trade.responses[you] === 1 ||
                        trade.take.some((n, i) => n > hand[i]) ||
                        (trade.goldTake || 0) > gold
                      }
                      onClick={() =>
                        void send({
                          type: "catan_trade_accept",
                          offer: trade.id,
                          prompt: x.sequence,
                        })
                      }
                    >
                      {trade.responses[you] === 1
                        ? "已同意，等待成交"
                        : "同意报价"}
                    </button>
                    <button
                      disabled={busy || trade.responses[you] === -1}
                      onClick={() =>
                        void send({
                          type: "catan_trade_reject",
                          offer: trade.id,
                          prompt: x.sequence,
                        })
                      }
                    >
                      拒绝
                    </button>
                  </>
                )}
              </div>
            )}
          </section>
        )}
        {mayOffer && (
          <details className="explorer-trade">
            <summary>向玩家提出交易</summary>
            <ResourcePicker
              label="我付出"
              values={give}
              onChange={setGive}
              limits={hand}
              assets={assets}
              disabled={busy}
            />
            <label>
              付出金币
              <input
                type="number"
                min="0"
                max={gold}
                value={goldGive}
                disabled={busy}
                onChange={(e) => setGoldGive(Number(e.target.value))}
              />
            </label>
            <ResourcePicker
              label="我需要"
              values={take}
              onChange={setTake}
              limits={[19, 19, 19, 19, 19]}
              assets={assets}
              disabled={busy}
            />
            <label>
              需要金币
              <input
                type="number"
                min="0"
                max="148"
                value={goldTake}
                disabled={busy}
                onChange={(e) => setGoldTake(Number(e.target.value))}
              />
            </label>
            <button
              disabled={busy || !offerValid}
              onClick={() =>
                void send({
                  type: "catan_trade_offer",
                  prompt: x.sequence,
                  give,
                  take,
                  goldGive,
                  goldTake,
                })
              }
            >
              提出报价
            </button>
          </details>
        )}
        {!choices.length && !due && !trade && room.status === "playing" && (
          <p className="explorer-notice">
            等待{room.seats[game.turn]?.name}完成{phase}。
          </p>
        )}
        <details className="explorer-rules">
          <summary>
            <Anchor size={16} /> 初航规则与银行
          </summary>
          <p>
            每船4步，可付1羊毛加2步。切换船只后不能回到上一艘；发现地块会停止本船。船端移民可在合法陆地点定居，船和移民回供应。
          </p>
          <p>
            每个港口和船舱有2格，一枚移民占2格。只能通过实际停靠的己方港口装卸。
          </p>
          <p>
            非7点未获资源的玩家获1金币补偿。3同类资源可换1其他资源或金币；2金币可买1资源，每回合最多2次。无发展卡、强盗、最长道路或最大军队。
          </p>
          <Bundle values={g.bank} assets={assets} showZero />
          <p>
            金币银行 {x.economy.goldBank} · 本回合金币购资源 {x.economy.bought}
            /2
          </p>
        </details>
      </aside>
    </section>
  );
}
