import { RailMapsContext, playerColors } from "./context";
export { RailMapsContext, playerColors } from "./context";
import {
  ArrowRight,
  Check,
  Eye,
  Flag,
  TrainFront,
  ZoomIn,
  ZoomOut,
} from "lucide-react";
import React, { useContext, useEffect, useRef, useState } from "react";
import { AnimatedSlot } from "../../../web/src/animated-slot";
import { PlayerName } from "../../../web/src/profiles";
import type {
  Catalog,
  RailMapSpec,
  RailPayment,
  RailPlayer,
  Room,
  Route,
  Ticket,
} from "./types";
import type { Act } from "../../../web/src/types";
import { AssetsContext, Modal, api } from "../../../web/src/ui/board-host";
import { useControllerCancel } from "../../../web/src/controllers/use-controller-cancel";
import { useMapControls } from "../../../web/src/ui/map-controls";
import { cityName } from "./cities";
import { HiddenDrawAnimation } from "./hidden-draw-animation";
import {
  RailPaymentEditor,
  paymentText,
  railPaymentValid,
  routePoints,
} from "./rail-expansions";

export const trainColors = [
  "#a35e91",
  "#eee6d1",
  "#357bac",
  "#e3bc38",
  "#dd8540",
  "#39434a",
  "#c84b43",
  "#568757",
  "#e4ba55",
];
export const trainNames = [
  "紫色",
  "白色",
  "蓝色",
  "黄色",
  "橙色",
  "黑色",
  "红色",
  "绿色",
  "万能",
];

export function TrainCard({
  color,
  count,
  onClick,
  disabled = false,
  label,
}: {
  color: number;
  count?: number;
  onClick?: () => void;
  disabled?: boolean;
  label?: string;
}) {
  const previousCount = useRef(count);
  const [gained, setGained] = useState(0);
  useEffect(() => {
    if (
      count !== undefined &&
      previousCount.current !== undefined &&
      count > previousCount.current
    )
      setGained((n) => n + 1);
    previousCount.current = count;
  }, [count]);
  useEffect(() => {
    if (!gained) return;
    const timer = setTimeout(() => setGained(0), 900);
    return () => clearTimeout(timer);
  }, [gained]);
  return (
    <button
      key={gained}
      data-hand-color={count !== undefined ? color : undefined}
      className={`train-card train-${color} ${gained ? "just-gained" : ""}`}
      onAnimationEnd={() => setGained(0)}
      style={
        {
          "--train": trainColors[color],
          backgroundPosition: `${(color === 8 ? 0 : color + 1) * 12.5}% 0`,
        } as React.CSSProperties
      }
      onClick={onClick}
      disabled={disabled}
      aria-label={
        label ||
        `${trainNames[color]}列车牌${count !== undefined ? ` ${count} 张` : ""}`
      }
    >
      <TrainFront size={28} />
      <div className="train-stripes" />
      {count !== undefined && <b>{count}</b>}
    </button>
  );
}
export function TicketCard({
  ticket,
  catalog,
  selected,
  onClick,
  showResult = false,
  showProgress = false,
}: {
  ticket: Ticket;
  catalog: Catalog;
  selected?: boolean;
  onClick?: () => void;
  showResult?: boolean;
  showProgress?: boolean;
}) {
  const assets = useContext(AssetsContext);
  const a = catalog.cities[ticket.a],
    b = catalog.cities[ticket.b];
  return (
    <button
      style={
        assets
          ? {
              backgroundImage: `url("${assets}/${catalog.map.art}/tickets/${ticket.id}.webp")`,
            }
          : undefined
      }
      className={`ticket-card ${catalog.map.height > catalog.map.width ? "portrait-ticket" : ""} ${ticket.options ? "country-ticket" : ""} ${selected ? "selected" : ""} ${showResult ? (ticket.complete ? "completed" : "incomplete") : showProgress && ticket.complete ? "completed" : ""}`}
      onClick={onClick}
    >
      <div className="ticket-label">
        DESTINATION TICKET{" "}
        <span>
          {showResult
            ? ticket.complete
              ? "已完成"
              : "未完成"
            : selected
              ? "✓ 保留"
              : "目的地"}
        </span>
      </div>
      <div className="ticket-route">
        <strong>{cityName(a.name)}</strong>
        <ArrowRight size={16} />
        <strong>{ticket.options ? "多个目的地" : cityName(b.name)}</strong>
      </div>
      <small>
        {a.name} — {b.name}
      </small>
      {ticket.options && (
        <div className="ticket-options">
          {ticket.options.map((o) => (
            <span key={o.to}>
              {cityName(catalog.cities[o.to].name)} · {o.points} 分
            </span>
          ))}
        </div>
      )}
      {ticket.mandala && <span className="ticket-mandala">✓ 环游达成</span>}
      {showProgress && (
        <span className={`ticket-progress ${ticket.complete ? "done" : ""}`}>
          {ticket.complete ? (
            <>
              <Check size={12} />
              已完成
            </>
          ) : (
            "未完成"
          )}
        </span>
      )}
      <div
        className={`ticket-bottom ${assets && catalog.map.id !== "usa" && !showResult && !ticket.options ? "sr-only" : ""}`}
      >
        <svg viewBox="0 0 110 35">
          <path
            d="m4 25 19-12 19 11 27-15 17 14 19-17"
            fill="none"
            stroke="#a38e62"
            strokeDasharray="3 3"
          />
          <circle cx="4" cy="25" r="3" fill="#174e43" />
          <circle cx="105" cy="6" r="3" fill="#c65342" />
        </svg>
        <b>
          {showResult ? (ticket.complete ? "+" : "−") : ""}
          {showResult ? Math.abs(ticket.value ?? ticket.points) : ticket.points}
          <small>{ticket.options && !showResult ? "分起" : "分"}</small>
        </b>
      </div>
    </button>
  );
}
export const wagonColors = ["blue", "red", "green", "yellow", "black"];
export function RailMap({
  catalog,
  owners,
  selected,
  onSelect,
  highlight,
  players,
  stationMode,
  onStation,
}: {
  catalog: Catalog;
  players: RailPlayer[];
  stationMode: boolean;
  onStation: (city: number) => void;
  owners: Record<string, number>;
  selected?: Route;
  onSelect: (r: Route) => void;
  highlight?: Ticket;
}) {
  const { viewport, zoom, zoomAt, dragging, canvasStyle, mapStyle, handlers } =
    useMapControls({
      aspect: catalog.map.width / catalog.map.height,
      minMobileWidth: catalog.map.width < catalog.map.height ? 360 : 620,
      fillMobileHeight: true,
    });
  const [showCities, setShowCities] = useState(
    () => localStorage.getItem("wb_rail_city_names") !== "off",
  );
  const assets = useContext(AssetsContext);
  const [wagonReady, setWagonReady] = useState<boolean[]>([]);
  useEffect(() => {
    let cancelled = false;
    setWagonReady([]);
    if (assets)
      wagonColors.forEach((color, i) => {
        const image = new Image();
        image.onload = () => {
          if (!cancelled)
            setWagonReady((ready) => {
              const next = [...ready];
              next[i] = true;
              return next;
            });
        };
        image.src = `${assets}/rail/wagon-${color}-v2.webp`;
      });
    return () => {
      cancelled = true;
    };
  }, [assets]);
  return (
    <div className="map-frame">
      <div className="map-controls" data-controller-group="camera">
        <button
          className="city-toggle"
          aria-pressed={showCities}
          onClick={() => {
            setShowCities(!showCities);
            localStorage.setItem(
              "wb_rail_city_names",
              showCities ? "off" : "on",
            );
          }}
        >
          <Eye size={15} />
          城市名{showCities ? "：开" : "：关"}
        </button>
        <div>
          <button
            className="icon-button"
            aria-label="缩小地图"
            disabled={zoom <= 1}
            onClick={() => zoomAt(zoom - 0.25)}
          >
            <ZoomOut size={17} />
          </button>
          <span>{Math.round(zoom * 100)}%</span>
          <button
            className="icon-button"
            aria-label="放大地图"
            disabled={zoom >= 3}
            onClick={() => zoomAt(zoom + 0.25)}
          >
            <ZoomIn size={17} />
          </button>
        </div>
      </div>
      <div
        ref={viewport}
        className={`map-scroll ${dragging ? "is-dragging" : ""}`}
        {...handlers}
      >
        <div className="map-canvas" style={canvasStyle}>
          <svg
            className="rail-map"
            data-controller-group="board"
            viewBox={`0 0 ${catalog.map.width} ${catalog.map.height}`}
            style={mapStyle}
            role="group"
            aria-label={`${catalog.map.name}铁路地图，点击路线选择占领`}
          >
            <rect
              width={catalog.map.width}
              height={catalog.map.height}
              fill="#ece8d8"
            />
            {assets && (
              <image
                href={`${assets}/${catalog.map.art}/${catalog.map.id === "usa" ? "map-unlabeled-v4.webp" : "map.webp"}`}
                width={catalog.map.width}
                height={catalog.map.height}
              />
            )}
            {catalog.routes.map((r) => {
              const a = catalog.cities[r.a],
                b = catalog.cities[r.b];
              const owner = owners[r.id],
                claimed = owner !== undefined;
              const active = selected?.id === r.id;
              return (
                <g
                  key={r.id}
                  className="map-route"
                  data-rail-route={r.id}
                  role="button"
                  tabIndex={0}
                  aria-label={`${cityName(a.name)} 到 ${cityName(b.name)}，${r.length} 节，${r.color < 0 ? "任意单色" : trainNames[r.color]}${claimed ? "，已占领" : ""}`}
                  onClick={() => onSelect(r)}
                  onKeyDown={(e) => {
                    if (e.key === "Enter" || e.key === " ") {
                      e.preventDefault();
                      onSelect(r);
                    }
                  }}
                >
                  <title>
                    {cityName(a.name)} → {cityName(b.name)} · {r.length} 节
                    {r.tunnel ? " · 隧道" : ""}
                    {r.ferry ? ` · 万能×${r.ferry}` : ""}
                    {r.mountain ? ` · 山地×${r.mountain}` : ""}
                    {claimed ? ` · 玩家 ${owner + 1}` : ""}
                  </title>
                  {r.segments?.map((segment, i) => (
                    <g
                      key={i}
                      transform={`translate(${segment.x},${segment.y}) rotate(${segment.angle})`}
                    >
                      <rect
                        x="-31"
                        y="-13"
                        width="62"
                        height="26"
                        rx="4"
                        fill="transparent"
                      />
                      {active && (
                        <rect
                          x="-31"
                          y="-15"
                          width="62"
                          height="30"
                          rx="6"
                          fill="#f5cf4740"
                          stroke="#f5cf47"
                          strokeWidth="3"
                        />
                      )}
                      {claimed ? (
                        wagonReady[owner] ? (
                          <image
                            className="claimed-wagon"
                            href={`${assets}/rail/wagon-${wagonColors[owner]}-v2.webp`}
                            x="-33"
                            y="-14"
                            width="66"
                            height="28"
                            preserveAspectRatio="none"
                            pointerEvents="none"
                          />
                        ) : (
                          <g
                            fill={playerColors[owner]}
                            stroke="#17252b"
                            strokeWidth="1.1"
                            pointerEvents="none"
                          >
                            <path d="M-27-6L-22-11H22L27-6V7H-27Z" />
                            <path
                              d="M-22-10H22L25-6H-25Z"
                              fill="#fff"
                              opacity=".28"
                              stroke="none"
                            />
                            <path
                              d="M-27 7H27V10H-27ZM-20 10V13H-13V10M13 10V13H20V10"
                              fill="#253039"
                            />
                            <path
                              d="M-18-4V5M-9-4V5M0-4V5M9-4V5M18-4V5"
                              stroke="#000"
                              opacity=".25"
                            />
                          </g>
                        )
                      ) : (
                        <rect
                          x="-26"
                          y="-8"
                          width="52"
                          height="16"
                          rx="3"
                          fill={
                            assets
                              ? "transparent"
                              : r.color < 0
                                ? "#bbb4a1"
                                : trainColors[r.color]
                          }
                        />
                      )}
                    </g>
                  ))}
                </g>
              );
            })}
            {catalog.cities.map((c) => {
              const countryLit =
                !!highlight &&
                c.kind === "border" &&
                [
                  highlight.a,
                  highlight.b,
                  ...(highlight.options?.map((o) => o.to) || []),
                ].some(
                  (id) =>
                    catalog.cities[id].kind === "country" &&
                    catalog.cities[id].country === c.country,
                );
              if (c.kind === "border" && !countryLit) return null;
              const lit =
                countryLit ||
                (highlight &&
                  (c.id === highlight.a ||
                    c.id === highlight.b ||
                    highlight.options?.some((o) => o.to === c.id)));
              const stationOwner = players.findIndex((p) =>
                p.stations?.includes(c.id),
              );
              const [lx, ly, lw, lh] = c.label || [c.x + 12, c.y - 30, 100, 30];
              return (
                <g
                  key={c.id}
                  data-rail-city={c.id}
                  className={`map-city ${stationMode ? "station-city" : ""}`}
                  role={stationMode ? "button" : undefined}
                  tabIndex={stationMode ? 0 : undefined}
                  aria-label={
                    stationMode ? `在${cityName(c.name)}建造车站` : undefined
                  }
                  onClick={() => {
                    if (stationMode) onStation(c.id);
                  }}
                  onKeyDown={(e) => {
                    if (stationMode && (e.key === "Enter" || e.key === " ")) {
                      e.preventDefault();
                      onStation(c.id);
                    }
                  }}
                >
                  {stationMode && (
                    <circle
                      cx={c.x}
                      cy={c.y}
                      r={23}
                      fill="#f2ce6488"
                      stroke="#8e5c21"
                      strokeWidth={3}
                    />
                  )}
                  {lit && (
                    <circle
                      cx={c.x}
                      cy={c.y}
                      r="23"
                      fill="#f1b52f"
                      opacity=".7"
                    />
                  )}
                  {c.kind !== "country" && (
                    <circle
                      cx={c.x}
                      cy={c.y}
                      r={lit ? 9 : 5}
                      fill={lit ? "#b53427" : "#f6f0dc"}
                      stroke="#4d503c"
                      strokeWidth="2"
                    />
                  )}
                  {stationOwner >= 0 && (
                    <g
                      className="map-station"
                      transform={`translate(${c.x},${c.y})`}
                    >
                      {assets ? (
                        <image
                          href={`${assets}/${catalog.map.art}/station-${wagonColors[stationOwner]}.webp`}
                          x={-20}
                          y={-34}
                          width={40}
                          height={52}
                        />
                      ) : (
                        <path
                          d="M-17 9V-8L0-22L17-8V9ZM-5 9V-3H5V9"
                          fill={playerColors[stationOwner]}
                          stroke="#fff6d4"
                          strokeWidth={3}
                        />
                      )}
                      <title>玩家 {stationOwner + 1} 的车站</title>
                    </g>
                  )}
                  {showCities && c.kind !== "border" && (
                    <rect
                      x={lx}
                      y={ly}
                      width={lw}
                      height={lh}
                      rx="5"
                      fill="#f4f0e3"
                    />
                  )}
                  {showCities && c.kind !== "border" && (
                    <text
                      x={lx + lw / 2}
                      y={ly + lh / 2}
                      textAnchor="middle"
                      style={{
                        fontSize: Math.min(
                          22,
                          (lw - 4) / cityName(c.name).length,
                        ),
                      }}
                      dominantBaseline="central"
                    >
                      {cityName(c.name)}
                    </text>
                  )}
                </g>
              );
            })}
          </svg>
        </div>
      </div>
      <div className="map-caption">
        <span>滚轮缩放 · 按住拖动 · 点击路线查看费用</span>
        <span>灰色路线可使用任意一种颜色</span>
      </div>
    </div>
  );
}
export function RailBoard({
  room,
  act,
  busy,
}: {
  room: Room;
  act: Act;
  busy: boolean;
}) {
  const root = useRef<HTMLDivElement>(null);
  useControllerCancel(root, () => {
    setKeep([]);
    setHighlight(undefined);
    setStationMode(false);
    setStationCity(undefined);
  });
  const [catalog, setCatalog] = useState<Catalog>();
  const [loadError, setLoadError] = useState("");
  const [selected, setSelected] = useState<Route>();
  const [color, setColor] = useState(0);
  const [wild, setWild] = useState(0);
  const [keep, setKeep] = useState<number[]>([]);
  const [ticketChoiceOpen, setTicketChoiceOpen] = useState(true);
  const [confirmTickets, setConfirmTickets] = useState(false);
  const [highlight, setHighlight] = useState<Ticket>();
  const [stationMode, setStationMode] = useState(false);
  const [stationCity, setStationCity] = useState<number>();
  const [payment, setPayment] = useState<RailPayment>({
    color: 0,
    tokens: Array(9).fill(0),
  });
  const [tunnelOpen, setTunnelOpen] = useState(true);
  const [tunnelWild, setTunnelWild] = useState(-1);
  const g = room.game!,
    r = g.rail!,
    p = r.players[room.you] ?? {
      hand: Array(9).fill(0),
      handCount: 0,
      tickets: [],
      ticketCount: 0,
      trains: 0,
      stations: [],
      stationRoutes: [],
    },
    mine =
      !room.spectating &&
      (r.setup ? !r.setupReady?.[room.you] : g.turn === room.you) &&
      !g.finished,
    turn = mine && !r.setup && g.phase === "turn",
    drawing = mine && !r.setup && (g.phase === "turn" || g.phase === "draw");
  useEffect(() => {
    let alive = true;
    setCatalog(undefined);
    setLoadError("");
    setHighlight(undefined);
    api(`/catalog?map=${encodeURIComponent(r.map || "usa")}`)
      .then((value) => {
        if (alive) setCatalog(value);
      })
      .catch((e) => {
        if (alive) setLoadError(e.message);
      });
    return () => {
      alive = false;
    };
  }, [r.map]);
  useEffect(() => {
    setSelected(undefined);
    setConfirmTickets(false);
    setStationMode(false);
    setStationCity(undefined);
    setTunnelOpen(true);
    setTunnelWild(-1);
  }, [g.turn, g.phase, g.round]);
  useEffect(() => {
    setKeep([]);
    setTicketChoiceOpen(true);
  }, [r.pending?.map((t) => t.id).join(",")]);
  if (loadError)
    return (
      <div ref={root}>
        <p role="alert">地图加载失败：{loadError}。请刷新重试。</p>
      </div>
    );
  if (!catalog)
    return (
      <div ref={root} className="loading">
        正在展开地图…
      </div>
    );
  const city = (id: number) => cityName(catalog.cities[id].name);
  const selectRoute = (route: Route) => {
    setSelected(route);
    setStationMode(false);
    const suggested = r.payments?.[route.id]?.[0];
    setPayment(
      suggested
        ? { color: suggested.color, tokens: [...suggested.tokens] }
        : { color: Math.max(0, route.color), tokens: Array(9).fill(0) },
    );
    const c =
      route.color >= 0
        ? route.color
        : Math.max(
            0,
            p.hand!.findIndex((n) => n >= route.length),
          );
    setColor(c > 7 ? 0 : c);
    setWild(
      Math.min(
        route.length,
        Math.max(0, route.length - p.hand![c > 7 ? 0 : c]),
      ),
    );
  };
  const parallelBlocked =
    selected &&
    catalog.routes.some(
      (x) =>
        x.id !== selected.id &&
        ((x.a === selected.a && x.b === selected.b) ||
          (x.a === selected.b && x.b === selected.a)) &&
        r.owners[x.id] !== undefined &&
        (room.seats.length < catalog.map.doubleMin ||
          r.owners[x.id] === room.you),
    );
  return (
    <div
      ref={root}
      className={`rail-board ${catalog.map.height > catalog.map.width ? "portrait-board" : ""}`}
    >
      <h2 className="sr-only">铁路环游 · {catalog.map.name}</h2>
      <div className="rail-map-legend">
        <span className="tag">{catalog.map.name}地图</span>
        <span>
          {
            {
              usa: "经典路线",
              europe: "隧道 · 渡轮 · 车站",
              india: "渡轮 · 环游奖励",
              switzerland: "隧道 · 国家任务",
              nordiccountries: "隧道 · 渡轮 · 任务奖励",
              legendaryasia: "山地 · 渡轮 · 网络奖励",
            }[catalog.map.id]
          }
        </span>
        {!!catalog.map.stations && !room.spectating && (
          <button
            className="outline"
            disabled={
              !turn || busy || (p.stations?.length || 0) >= catalog.map.stations
            }
            onClick={() => {
              setSelected(undefined);
              setStationMode(!stationMode);
            }}
          >
            {stationMode ? "取消建站" : "建造车站"} · 剩余{" "}
            {catalog.map.stations - (p.stations?.length || 0)}
          </button>
        )}
      </div>
      {stationMode && (
        <p className="rail-special-note">
          点击地图上的城市建站。第 {(p.stations?.length || 0) + 1} 座车站需{" "}
          {(p.stations?.length || 0) + 1} 张同色牌。
        </p>
      )}
      {r.tunnel && (
        <section
          className={`ticket-choice ${tunnelOpen ? "" : "is-collapsed"}`}
          aria-label="隧道追加费用"
        >
          <header>
            <h3>
              {city(catalog.routes.find((x) => x.id === r.tunnel!.route)!.a)} →{" "}
              {city(catalog.routes.find((x) => x.id === r.tunnel!.route)!.b)} ·
              隧道检查
            </h3>
            <button
              className="subtle"
              onClick={() => setTunnelOpen(!tunnelOpen)}
            >
              {tunnelOpen ? "收起看地图" : "展开检查牌"}
            </button>
          </header>
          {tunnelOpen && (
            <div>
              <div className="tunnel-revealed">
                {r.tunnel.revealed.map((c, i) => (
                  <TrainCard key={i} color={c} />
                ))}
              </div>
              <p>
                原支付：{paymentText(r.tunnel.base)}。需追加 {r.tunnel.extra} 张
                {r.tunnel.wildOnly
                  ? "万能"
                  : `${trainNames[r.tunnel.color]}或万能`}
                牌。
              </p>
              {mine ? (
                <>
                  <p className="muted small">
                    放弃会退回原支付牌，并结束本回合。
                  </p>
                  {!r.tunnel.wildOnly && (
                    <label>
                      追加费用中的万能牌
                      <select
                        aria-label="隧道追加万能牌"
                        value={
                          tunnelWild < 0
                            ? Math.max(
                                0,
                                r.tunnel.extra - p.hand![r.tunnel.color],
                              )
                            : tunnelWild
                        }
                        onChange={(e) => setTunnelWild(Number(e.target.value))}
                      >
                        {Array.from({ length: r.tunnel.extra + 1 }, (_, i) => (
                          <option
                            key={i}
                            value={i}
                            disabled={
                              i > p.hand![8] ||
                              r.tunnel!.extra - i > p.hand![r.tunnel!.color]
                            }
                          >
                            {i} 张万能 + {r.tunnel!.extra - i} 张
                            {trainNames[r.tunnel!.color]}牌
                          </option>
                        ))}
                      </select>
                    </label>
                  )}
                  <div className="modal-actions">
                    <button
                      className="outline"
                      disabled={busy}
                      onClick={() => void act({ type: "tunnel_cancel" })}
                    >
                      放弃并退牌
                    </button>
                    <button
                      className="primary"
                      disabled={
                        busy ||
                        (r.tunnel.wildOnly
                          ? p.hand![8]
                          : p.hand![r.tunnel.color] + p.hand![8]) <
                          r.tunnel.extra
                      }
                      onClick={() =>
                        void act({
                          type: "tunnel_pay",
                          wild: r.tunnel!.wildOnly
                            ? r.tunnel!.extra
                            : tunnelWild >= 0
                              ? tunnelWild
                              : Math.max(
                                  0,
                                  r.tunnel!.extra - p.hand![r.tunnel!.color],
                                ),
                        })
                      }
                    >
                      追加支付并铺路
                    </button>
                  </div>
                </>
              ) : (
                <p>等待 {room.seats[g.turn].name} 决定追加支付或放弃。</p>
              )}
            </div>
          )}
        </section>
      )}
      {mine && g.phase === "tickets" && (
        <section
          className={`ticket-choice ${ticketChoiceOpen ? "" : "is-collapsed"}`}
          aria-label="选择目的地任务"
        >
          <header>
            <h3>
              {ticketChoiceOpen
                ? r.setup
                  ? "选择你的第一段旅程"
                  : "新的目的地，新的机会"
                : `目的地 · 已选 ${keep.length} 张`}
            </h3>
            <button
              className="subtle"
              aria-expanded={ticketChoiceOpen}
              aria-controls="ticket-choice-controls"
              onClick={() => setTicketChoiceOpen(!ticketChoiceOpen)}
            >
              {ticketChoiceOpen ? "收起看地图" : "继续选择"}
            </button>
          </header>
          {ticketChoiceOpen && (
            <div id="ticket-choice-controls">
              <p>
                从这些任务中至少保留 <strong>{r.setup ? 2 : 1}</strong>{" "}
                张。未完成的任务会在结算时扣分。
                {r.setup && "所有人同时选择；120 秒后由电脑选牌并开启托管。"}
              </p>
              {catalog.map.height > catalog.map.width && (
                <small className="portrait-ticket-hint">
                  左右滑动车票查看全部任务，点击选择保留。
                </small>
              )}
              <div
                className="pending-tickets"
                data-controller-group="tickets"
                data-controller-cards=""
              >
                {r.pending?.map((t) => (
                  <TicketCard
                    key={t.id}
                    ticket={t}
                    catalog={catalog}
                    selected={keep.includes(t.id)}
                    onClick={() => {
                      setHighlight(t);
                      setKeep(
                        keep.includes(t.id)
                          ? keep.filter((x) => x !== t.id)
                          : [...keep, t.id],
                      );
                    }}
                  />
                ))}
              </div>
              <button
                className="primary wide"
                disabled={busy || keep.length < (r.setup ? 2 : 1)}
                onClick={() => void act({ type: "keep", keep })}
              >
                保留 {keep.length} 张目的地任务
                <Check size={17} />
              </button>
            </div>
          )}
        </section>
      )}

      {confirmTickets && (
        <Modal
          title="领取新的目的地任务"
          onClose={() => setConfirmTickets(false)}
        >
          <p>
            将抽取最多 3 张任务，至少保留 1
            张，并消耗本回合。未完成的任务会在结算时扣分。
          </p>
          <div className="modal-actions">
            <button
              className="outline"
              onClick={() => setConfirmTickets(false)}
            >
              取消
            </button>
            <button
              className="primary"
              disabled={!turn || busy || r.ticketsRemaining === 0}
              onClick={() => {
                setConfirmTickets(false);
                void act({ type: "tickets" });
              }}
            >
              确认领取
            </button>
          </div>
        </Modal>
      )}
      <HiddenDrawAnimation room={room} />
      <div
        className={`rail-table ${catalog.map.height > catalog.map.width ? "portrait-map" : ""}`}
      >
        <RailMap
          key={catalog.map.id}
          catalog={catalog}
          players={r.players}
          stationMode={stationMode}
          onStation={(id) => {
            setStationCity(id);
            const cost = (p.stations?.length || 0) + 1;
            const hand = p.hand!.slice(0, 8);
            const enough = hand.findIndex((n) => n >= cost);
            const c = enough >= 0 ? enough : hand.indexOf(Math.max(...hand));
            setColor(c);
            setWild(Math.max(0, cost - hand[c]));
          }}
          owners={r.owners}
          selected={selected}
          onSelect={selectRoute}
          highlight={highlight}
        />
        <div className="train-market">
          <div className="section-line">
            <h3>列车牌市场</h3>
            <small>
              {catalog.map.wildSingle
                ? g.phase === "draw" && mine
                  ? "还可摸 1 张；可拿万能牌"
                  : "每回合摸两张；公开万能牌计一张"
                : g.phase === "draw" && mine
                  ? "还可以摸 1 张（不能拿公开万能牌）"
                  : "每回合摸两张；公开万能牌占用整个行动"}
            </small>
          </div>
          <div className="train-market-cards" data-controller-group="market">
            <button
              className="train-deck"
              aria-label="摸一张暗牌"
              disabled={!drawing || busy}
              onClick={() => void act({ type: "draw", slot: -1 })}
            >
              <TrainFront size={28} />
              <strong>
                <span className="train-deck-label-full">摸一张暗牌</span>
                <span className="train-deck-label-short" aria-hidden="true">
                  暗牌
                </span>
              </strong>
              <small>可摸 {r.remaining} 张</small>
            </button>
            {r.face.map((c, i) => (
              <AnimatedSlot
                key={i}
                identity={`${c}:${r.faceVersion?.[i] ?? 0}`}
                marker={i}
              >
                {c < 0 ? (
                  <div className="train-card empty">牌堆已空</div>
                ) : (
                  <TrainCard
                    color={c}
                    disabled={
                      !drawing ||
                      busy ||
                      (g.phase === "draw" && c === 8 && !catalog.map.wildSingle)
                    }
                    onClick={() => void act({ type: "draw", slot: i })}
                    label={`拿取公开${trainNames[c]}列车牌，第 ${i + 1} 张`}
                  />
                )}
              </AnimatedSlot>
            ))}
          </div>
          <button
            className="ticket-deck"
            disabled={!turn || busy || r.ticketsRemaining === 0}
            onClick={() => setConfirmTickets(true)}
          >
            <Flag size={25} />
            <strong>领取目的地</strong>
            <small>抽 3 留至少 1 · 余 {r.ticketsRemaining}</small>
          </button>
        </div>
      </div>
      {!room.spectating && (
        <div className="personal-area rail-personal">
          <div className="section-line">
            <h3>
              你的列车手牌 <span>{p.handCount}</span>
            </h3>
            <span className="tag">
              剩余车厢 {p.trains} / {catalog.map.trains}
            </span>
          </div>
          <div className="train-hand">
            {p.hand!.map((n, c) => (
              <TrainCard key={c} color={c} count={n} disabled={n === 0} />
            ))}
          </div>
          {!!catalog.map.stations && (
            <p className="muted small">
              车站按总得分最高的借路方案结算，各任务共用同一方案。当前借路：
              {p.stationRoutes
                ?.filter(Boolean)
                .map((id) => {
                  const x = catalog.routes.find((r) => r.id === id)!;
                  return `${city(x.a)} → ${city(x.b)}`;
                })
                .join("、") || "暂无"}
              （仅你可见）
            </p>
          )}
          <div className="section-line ticket-heading">
            <h3>
              你的目的地{" "}
              <span>
                已完成 {p.tickets?.filter((t) => t.complete).length || 0} /{" "}
                {p.tickets?.length || 0}
              </span>
            </h3>
            <small>点击车票，在地图上标记起终点</small>
          </div>
          <div className="tickets">
            {p.tickets?.map((t) => (
              <TicketCard
                key={t.id}
                ticket={t}
                catalog={catalog}
                selected={highlight?.id === t.id}
                onClick={() =>
                  setHighlight(highlight?.id === t.id ? undefined : t)
                }
                showResult={g.finished}
                showProgress
              />
            ))}
          </div>
        </div>
      )}
      {g.finished && (
        <div className="personal-area">
          <h3>所有玩家的目的地结算</h3>
          {room.seats.map((seat, i) => (
            <details key={seat.id}>
              <summary>
                <PlayerName user={seat} /> · 完成 {r.players[i].completed} 张 ·
                净得分 {r.players[i].ticketScore}
              </summary>
              <div className="tickets">
                {r.players[i].tickets?.map((t) => (
                  <TicketCard
                    key={t.id}
                    ticket={t}
                    catalog={catalog}
                    showResult
                    onClick={() => setHighlight(t)}
                  />
                ))}
              </div>
            </details>
          ))}
        </div>
      )}
      {selected && (
        <Modal
          title={`${city(selected.a)} → ${city(selected.b)}`}
          onClose={() => setSelected(undefined)}
        >
          <div className="route-preview">
            <TrainFront size={28} />
            <strong>{selected.length} 节车厢</strong>
            <span>铺设得 {routePoints(selected)} 分</span>
          </div>
          {r.owners[selected.id] !== undefined ? (
            <p>
              这条路线已由{" "}
              <strong>{room.seats[r.owners[selected.id]].name}</strong> 占领。
            </p>
          ) : parallelBlocked ? (
            <p>
              这条双线目前不可用。少于 {catalog.map.doubleMin}{" "}
              人只能使用其中一条，同一玩家也不能同时占领双线。
            </p>
          ) : room.spectating ? (
            <p>
              这条路线尚未铺设，需要 {selected.length} 张
              {selected.color >= 0
                ? trainNames[selected.color]
                : "同一种颜色的"}
              列车牌，可使用万能牌替代。
            </p>
          ) : (
            <>
              {(selected.tunnel ||
                selected.ferry ||
                selected.mountain ||
                selected.substitute ||
                catalog.map.wildRule !== "any") && (
                <p className="rail-special-note">
                  {selected.tunnel &&
                    "隧道：翻开三张检查牌后，可能需要追加支付；可放弃并退回原支付牌。 "}
                  {!!selected.ferry &&
                    `渡轮：至少 ${selected.ferry} 张万能牌。 `}
                  {!!selected.mountain &&
                    `山地：另消耗 ${selected.mountain} 节车厢，奖励已计入上方得分。 `}
                  {selected.substitute === 3 &&
                    "每张必需万能牌可用任意三张牌替代。"}
                  {selected.substitute === 4 &&
                    "每张同色牌可用任意四张牌替代。"}
                  {catalog.map.wildRule !== "any" && "本地图万能牌只能用于隧道"}
                  {catalog.map.wildRule === "special" && "和渡轮"}
                </p>
              )}
              <label>
                路线主颜色
                <select
                  aria-label="路线主颜色"
                  value={payment.color}
                  disabled={selected.color >= 0}
                  onChange={(e) =>
                    setPayment({ ...payment, color: Number(e.target.value) })
                  }
                >
                  {trainNames.slice(0, 8).map((n, i) => (
                    <option key={i} value={i}>
                      {n}
                    </option>
                  ))}
                </select>
              </label>
              <RailPaymentEditor
                hand={p.hand!}
                value={payment}
                onChange={setPayment}
                suggestions={r.payments?.[selected.id]}
              />
              {!r.payments?.[selected.id]?.length && (
                <p className="muted small">
                  目前手牌或车厢不足以铺设这条路线。
                </p>
              )}
              <button
                className="primary wide"
                disabled={
                  !turn ||
                  busy ||
                  p.trains < selected.length + (selected.mountain || 0) ||
                  !railPaymentValid(
                    selected,
                    catalog.map,
                    p.hand!,
                    payment.color,
                    payment.tokens,
                  )
                }
                onClick={() =>
                  void act({
                    type: "claim",
                    route: selected.id,
                    color: payment.color,
                    tokens: payment.tokens,
                  })
                }
              >
                {selected.tunnel ? "尝试铺设隧道" : "铺设这条铁路"}
                <TrainFront size={18} />
              </button>
            </>
          )}
        </Modal>
      )}
      {stationCity !== undefined && (
        <Modal
          title={`在${city(stationCity)}建造车站`}
          onClose={() => setStationCity(undefined)}
        >
          <p>
            第 {(p.stations?.length || 0) + 1}{" "}
            座车站需相同数量的同色牌，可以万能牌补足。每个城市只能有一座车站。未建车站结算时每座得
            4 分。
          </p>
          <label>
            支付颜色
            <select
              value={color}
              onChange={(e) => {
                setColor(Number(e.target.value));
                setWild(
                  Math.max(
                    0,
                    (p.stations?.length || 0) +
                      1 -
                      p.hand![Number(e.target.value)],
                  ),
                );
              }}
            >
              {trainNames.slice(0, 8).map((n, i) => (
                <option key={i} value={i}>
                  {n} · {p.hand![i]} 张
                </option>
              ))}
            </select>
          </label>
          <label>
            使用万能牌
            <select
              value={wild}
              onChange={(e) => setWild(Number(e.target.value))}
            >
              {Array.from({ length: (p.stations?.length || 0) + 2 }, (_, i) => (
                <option key={i} value={i}>
                  {i} 张
                </option>
              ))}
            </select>
          </label>
          <button
            className="primary wide"
            disabled={
              !turn ||
              busy ||
              r.players.some((x) => x.stations?.includes(stationCity)) ||
              p.hand![color] < (p.stations?.length || 0) + 1 - wild ||
              p.hand![8] < wild
            }
            onClick={() =>
              void act({ type: "station", vertex: stationCity, color, wild })
            }
          >
            确认建造车站
          </button>
        </Modal>
      )}
      {turn && (
        <button
          className="pass-button"
          disabled={busy}
          onClick={() => void act({ type: "pass" })}
        >
          无合法行动时跳过回合
        </button>
      )}
    </div>
  );
}
