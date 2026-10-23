import {
  useEffect,
  useId,
  useMemo,
  useRef,
  useState,
  type CSSProperties,
} from "react";
import {
  ArrowRight,
  Check,
  ChevronDown,
  Crown,
  Flag,
  Minus,
  Plus,
  RotateCcw,
  ScrollText,
  Shield,
  Ship,
  X,
} from "lucide-react";
import type { BoardProps } from "../../../web/src/game-api/module";
import { Modal } from "../../../web/src/ui/board-host";
import { mapPoint, viewPoint } from "./camera";
import { useCamera } from "./useCamera";
import { recruitLimit, moveLimit, wholeAmount } from "./action-limits";
import { createCursor, consumeEvents } from "./events";
import type {
  ConquestMap,
  Event,
  MapProvince,
  Order,
  Province,
  View,
} from "./types";

const loaders: Record<string, () => Promise<{ default: ConquestMap }>> = {
  world: () => import("../maps/world.json"),
  europe: () => import("../maps/europe.json"),
};
const number = (n: number) => (n < 0 ? "?" : n.toLocaleString("zh-CN"));
const short = (n: number) =>
  n < 0
    ? "?"
    : n >= 100000
      ? `${(n / 10000).toFixed(0)}万`
      : n >= 10000
        ? `${(n / 10000).toFixed(1)}万`
        : String(n);
const fallbackColors = [
  "#aa6553",
  "#4f889c",
  "#b99b51",
  "#79916b",
  "#8a719b",
  "#ab7e59",
  "#58968b",
  "#a56c86",
];
type Tab = "move" | "recruit" | "build" | "state";

export function Board(props: BoardProps<View>) {
  const { data, onLoadError, onLoadProgress } = props;
  const [map, setMap] = useState<ConquestMap | null>(null),
    [error, setError] = useState("");
  useEffect(() => {
    let cancelled = false;
    setMap(null);
    setError("");
    onLoadProgress?.(5, "加载地图");
    const load = loaders[data.mapId];
    if (!load) {
      const message = "未找到这张地图";
      setError(message);
      onLoadError?.(message);
      return;
    }
    void load()
      .then(({ default: next }) => {
        if (cancelled) return;
        if (next.version !== data.mapVersion || next.sha256 !== data.mapSHA256)
          throw new Error("地图版本与对局不一致，请刷新后重试");
        onLoadProgress?.(75, "绘制地区与兵力");
        setMap(next);
      })
      .catch((e: unknown) => {
        if (cancelled) return;
        const message = e instanceof Error ? e.message : "地图加载失败";
        setError(message);
        onLoadError?.(message);
      });
    return () => {
      cancelled = true;
    };
  }, [
    data.mapId,
    data.mapVersion,
    data.mapSHA256,
    onLoadError,
    onLoadProgress,
  ]);
  return map ? (
    <MapBoard
      key={`${props.room.id}:${data.session}:${map.sha256}`}
      {...props}
      map={map}
    />
  ) : (
    <div className="cq-loading" role="status">
      {error || "正在绘制地图…"}
    </div>
  );
}

function MapBoard({
  room,
  data,
  map,
  act,
  busy,
  clock,
  audio,
  assets,
  sound,
  onReady,
  onLoadProgress,
}: BoardProps<View> & { map: ConquestMap }) {
  const camera = useCamera(map),
    uid = useId().replaceAll(":", "");
  const [selected, setSelected] = useState(""),
    [target, setTarget] = useState(""),
    [tab, setTab] = useState<Tab>("move");
  const [amount, setAmount] = useState(1),
    [recruits, setRecruits] = useState(100),
    [king, setKing] = useState(false),
    [tax, setTax] = useState(20);
  const [notice, setNotice] = useState(""),
    [pending, setPending] = useState(false),
    [ordersOpen, setOrdersOpen] = useState(false),
    [resignOpen, setResignOpen] = useState(false);
  const [activeEvents, setActiveEvents] = useState<Event[]>([]),
    [frame, setFrame] = useState(0);
  const eventCursor = useRef(
    createCursor({ events: data.events ?? [], round: data.round }, clock.epoch),
  );
  const latestClock = useRef(clock);
  latestClock.current = clock;
  const loaded = useRef(false);
  const provinceById = useMemo(
    () => new Map(map.provinces.map((p) => [p.id, p])),
    [map],
  );
  const stateById = useMemo(
    () => new Map(data.provinces.map((p) => [p.id, p])),
    [data.provinces],
  );
  const projectedById = useMemo(
    () => new Map((data.projected ?? []).map((p) => [p.id, p])),
    [data.projected],
  );
  const mine = data.players[room.you],
    plan = data.orders ?? [],
    selectedGeo = provinceById.get(selected);
  const actual = stateById.get(selected),
    projected = projectedById.get(selected) ?? actual;
  const own = !!actual && actual.owner === room.you && !room.spectating;
  const canPlan =
    data.controls.canPlan &&
    !room.spectating &&
    !room.seats[room.you]?.autoPlay &&
    !busy &&
    !pending &&
    room.status === "playing";
  const mobile = own ? Math.max(0, projected?.mobile ?? 0) : 0;
  const maxRecruit = own
    ? recruitLimit(
        data.availableGold,
        data.controls.recruitCost,
        projected?.population ?? 0,
        data.availableMovement,
        data.controls.recruitMovement,
      )
    : 0;
  const reachable = useMemo(
    () =>
      selectedGeo
        ? [...selectedGeo.neighbors, ...selectedGeo.seaNeighbors]
            .map((id) => provinceById.get(id))
            .filter((p): p is MapProvince => !!p)
        : [],
    [selectedGeo, provinceById],
  );
  const sea = !!selectedGeo?.seaNeighbors.includes(target);
  const maxMove = moveLimit(
    mobile,
    data.availableMovement,
    data.controls.moveCost,
    sea,
    data.availableGold,
    data.controls.seaCost ?? 0,
  );
  const ownerName = (owner: number) =>
    owner < 0 ? "中立" : (room.seats[owner]?.name ?? "未知势力");
  const color = (owner: number) =>
    owner < 0
      ? "#d3d1bb"
      : data.players[owner]?.color ||
        fallbackColors[owner % fallbackColors.length];
  const name = (id?: string) => (id ? (provinceById.get(id)?.name ?? id) : "");
  const serverNow = () =>
    latestClock.current.serverNow +
    Math.max(0, performance.now() - latestClock.current.receivedAt);

  useEffect(() => {
    if (loaded.current || camera.size.width < 2) return;
    let first = 0,
      second = 0,
      alive = true;
    void document.fonts.ready.then(() => {
      if (!alive) return;
      first = requestAnimationFrame(() => {
        second = requestAnimationFrame(() => {
          if (!alive) return;
          loaded.current = true;
          onLoadProgress?.(100, "地图已就绪");
          onReady?.();
        });
      });
    });
    return () => {
      alive = false;
      cancelAnimationFrame(first);
      cancelAnimationFrame(second);
    };
  }, [camera.size.width, onReady, onLoadProgress]);
  useEffect(() => {
    const next = consumeEvents(
      eventCursor.current,
      { events: data.events ?? [], round: data.round },
      clock.epoch,
      serverNow(),
      sound && !document.hidden,
      !document.hidden &&
        !matchMedia("(prefers-reduced-motion: reduce)").matches,
    );
    if (next.reset) {
      setActiveEvents([]);
      audio.stop();
      return;
    }
    if (next.events.length)
      setActiveEvents((previous) =>
        [...previous, ...next.events].filter((e) => e.until > serverNow()),
      );
    for (const clip of next.clips) {
      const local = `/conquest/v1/audio/${clip}.wav`;
      void audio.playWithFallback(
        assets ? [assets.replace(/\/$/, "") + local, local] : [local],
      );
    }
  }, [data.events, data.round, clock.epoch, audio, assets, sound]);
  useEffect(() => () => audio.stop(), [audio]);
  useEffect(() => {
    if (!activeEvents.length) return;
    let handle = 0,
      stopped = false;
    const tick = () => {
      if (stopped) return;
      const now = serverNow();
      setFrame(now);
      if (activeEvents.every((e) => e.until <= now)) {
        setActiveEvents([]);
        return;
      }
      handle = requestAnimationFrame(tick);
    };
    handle = requestAnimationFrame(tick);
    return () => {
      stopped = true;
      cancelAnimationFrame(handle);
    };
  }, [activeEvents]);
  useEffect(() => {
    setAmount((old) => Math.max(1, Math.min(old, Math.max(1, maxMove))));
  }, [maxMove]);
  useEffect(() => {
    setRecruits((old) => Math.max(1, Math.min(old, Math.max(1, maxRecruit))));
  }, [maxRecruit]);
  useEffect(() => {
    if (!reachable.some((p) => p.id === target)) setTarget("");
  }, [reachable, target]);
  useEffect(() => {
    if (mine?.tax !== undefined) setTax(mine.tax);
  }, [mine?.tax]);
  useEffect(() => {
    if (!own || mine?.king !== selected) setKing(false);
  }, [own, mine?.king, selected]);

  async function send(type: string, extra: Record<string, unknown> = {}) {
    setNotice("");
    setPending(true);
    try {
      await act({ type, session: data.session, epoch: data.epoch, ...extra });
    } catch (e) {
      setNotice(e instanceof Error ? e.message : "操作未能完成，请重试");
    } finally {
      setPending(false);
    }
  }
  function choose(id: string, locate = false) {
    const geography = provinceById.get(id);
    if (!geography) return;
    if (
      own &&
      id !== selected &&
      reachable.some((p) => p.id === id) &&
      tab === "move" &&
      !locate
    ) {
      setTarget(id);
      return;
    }
    setSelected(id);
    setTarget("");
    setKing(false);
    setNotice("");
    const province = projectedById.get(id) ?? stateById.get(id);
    setAmount(Math.max(1, Math.floor((province?.mobile ?? 1) / 2)));
    if (locate) camera.focus(geography.label);
  }
  const labels = useMemo(() => {
    const priority = (p: MapProvince) =>
      p.id === selected || p.id === target
        ? 0
        : stateById.get(p.id)?.owner === room.you
          ? 1
          : 2;
    const result: {
      province: MapProvince;
      point: { x: number; y: number };
      width: number;
    }[] = [];
    const ordered = [...map.provinces].sort(
      (a, b) => priority(a) - priority(b),
    );
    for (const province of ordered) {
      const point = viewPoint(camera.camera, province.label);
      if (
        point.x < 8 ||
        point.x > camera.size.width - 8 ||
        point.y < 8 ||
        point.y > camera.size.height - 8
      )
        continue;
      const width = Math.max(
        29,
        short(stateById.get(province.id)?.troops ?? 0).length * 7.4 + 14,
      );
      if (
        priority(province) !== 0 &&
        result.some(
          (item) =>
            Math.abs(item.point.x - point.x) < (item.width + width) / 2 + 3 &&
            Math.abs(item.point.y - point.y) < 25,
        )
      )
        continue;
      result.push({ province, point, width });
    }
    return result;
  }, [map, camera.camera, camera.size, selected, target, stateById, room.you]);
  const viewportTop = mapPoint(camera.camera, { x: 0, y: 0 });
  const status = room.spectating
    ? "正在观战"
    : mine?.eliminated
      ? "你的势力已退出"
      : data.phase === "paused"
        ? "对局已暂停"
        : data.phase === "resolving"
          ? "正在执行各方计划"
          : data.phase === "finished"
            ? "本局已结束"
            : mine?.submitted
              ? "已提交，等待其他势力"
              : "规划本轮行动";
  const orderDescription = (order: Order) =>
    order.type === "move"
      ? `${name(order.from)} → ${name(order.to)} · ${number(order.amount ?? 0)}${order.king ? " · 君主随军" : ""}`
      : order.type === "tax"
        ? `税率调整为 ${order.value}%`
        : `${name(order.province)} · ${order.type === "recruit" ? "征募 " + number(order.amount ?? 0) : order.type === "disband" ? "解散 " + number(order.amount ?? 0) : order.building === "fort" ? "修建要塞" : order.building === "tower" ? "修建瞭望塔" : "修建城镇"}`;
  function miniLocate(e: React.PointerEvent<SVGSVGElement>) {
    const r = e.currentTarget.getBoundingClientRect();
    camera.locate({
      x: ((e.clientX - r.left) / r.width) * map.width,
      y: ((e.clientY - r.top) / r.height) * map.height,
    });
  }
  const miniRelease = (e: React.PointerEvent<SVGSVGElement>) => {
    e.stopPropagation();
    if (e.currentTarget.hasPointerCapture(e.pointerId))
      e.currentTarget.releasePointerCapture(e.pointerId);
  };
  const mapStyle = { "--cq-player-color": color(room.you) } as CSSProperties;
  return (
    <section
      className="cq-board"
      style={mapStyle}
      data-conquest-board
      data-map={map.id}
      data-round={data.round}
      data-phase={data.phase}
      data-selected={selected}
      data-target={target}
      data-orders={plan.length}
      data-submitted={!!mine?.submitted}
      data-zoom={camera.ratio}
    >
      <div className="cq-heading">
        <div>
          <span className="cq-map-name">{map.name}</span>
          <strong>第 {data.round} 轮</strong>
        </div>
        <span className="cq-phase" role="status">
          {status}
        </span>
      </div>
      <div className="cq-map-tools">
        <label className="cq-region-select">
          <span className="cq-sr-only">选择地区</span>
          <select
            aria-label="选择地区"
            value={selected}
            onChange={(e) => choose(e.target.value, true)}
          >
            <option value="">查找地区</option>
            {map.provinces.map((p) => (
              <option key={p.id} value={p.id}>
                {p.name}
                {stateById.get(p.id)?.owner === room.you && !room.spectating
                  ? " · 己方"
                  : ""}
              </option>
            ))}
          </select>
        </label>
        <div className="cq-zoom-controls" role="group" aria-label="地图视图">
          <button
            type="button"
            aria-label="缩小地图"
            disabled={camera.ratio <= 1.001}
            onClick={() => camera.zoom(1 / 1.4)}
          >
            <Minus size={16} />
          </button>
          <button
            type="button"
            aria-label="放大地图"
            disabled={camera.ratio >= 7.99}
            onClick={() => camera.zoom(1.4)}
          >
            <Plus size={16} />
          </button>
          <button type="button" onClick={camera.reset}>
            <RotateCcw size={14} />
            全图
          </button>
        </div>
      </div>
      <div className="cq-viewport" ref={camera.viewport} {...camera.handlers}>
        <svg
          className="cq-map"
          width="100%"
          height="100%"
          role="group"
          aria-label={`${map.name}战场，选择地区查看与规划`}
        >
          <defs>
            <pattern
              id={`${uid}-water`}
              width="60"
              height="40"
              patternUnits="userSpaceOnUse"
            >
              <path
                d="M4 18q9-5 18 0t18 0M34 35q7-4 14 0"
                fill="none"
                stroke="#d7e6de"
                strokeWidth="1"
                opacity=".55"
              />
            </pattern>
            <pattern
              id={`${uid}-land`}
              width="18"
              height="18"
              patternUnits="userSpaceOnUse"
            >
              <path
                d="m4 7 2-3 2 3m4 7 2-3 2 3"
                fill="none"
                stroke="#5d6752"
                strokeWidth=".5"
                opacity=".16"
              />
            </pattern>
            <marker
              id={`${uid}-arrow`}
              viewBox="0 0 10 10"
              refX="8"
              refY="5"
              markerWidth="5"
              markerHeight="5"
              orient="auto-start-reverse"
            >
              <path d="M0 0 10 5 0 10 3 5Z" fill="#234d46" />
            </marker>
          </defs>
          <rect width="100%" height="100%" fill="#a8c6c8" />
          <rect width="100%" height="100%" fill={`url(#${uid}-water)`} />
          <g
            className="cq-map-layer"
            transform={`translate(${camera.camera.x} ${camera.camera.y}) scale(${camera.camera.scale})`}
          >
            <rect
              width={map.width}
              height={map.height}
              fill="none"
              stroke="#6e9c9d"
              strokeWidth="1"
              vectorEffect="non-scaling-stroke"
            />
            {map.seaRoutes.map((route, i) => (
              <path
                key={i}
                d={route.path}
                className={`cq-sea-route ${selected === route.from || selected === route.to ? "active" : ""}`}
                fill="none"
                vectorEffect="non-scaling-stroke"
              />
            ))}
            {map.provinces.map((p) => {
              const state = stateById.get(p.id),
                isSelected = p.id === selected,
                isTarget = p.id === target;
              const eligible = own && reachable.some((n) => n.id === p.id);
              return (
                <path
                  key={p.id}
                  data-cq-province={p.id}
                  data-owner={state?.owner ?? -1}
                  data-troops={state?.troops ?? -1}
                  d={p.path}
                  fill={color(state?.owner ?? -1)}
                  fillRule="evenodd"
                  vectorEffect="non-scaling-stroke"
                  className={`cq-province ${isSelected ? "selected" : ""} ${isTarget ? "target" : ""} ${eligible ? "reachable" : ""}`}
                  role="button"
                  tabIndex={0}
                  aria-label={`${p.name}，${ownerName(state?.owner ?? -1)}，兵力 ${number(state?.troops ?? 0)}`}
                  aria-pressed={isSelected || isTarget}
                  onClick={(e) => {
                    if (camera.canClick(e.detail)) choose(p.id);
                  }}
                  onKeyDown={(e) => {
                    if (e.key === "Enter" || e.key === " ") {
                      e.preventDefault();
                      choose(p.id);
                      camera.focus(p.label);
                    }
                  }}
                  onFocus={() => {
                    /* Focus ring remains on geographic region. */
                  }}
                >
                  <title>
                    {p.name} · {ownerName(state?.owner ?? -1)} ·{" "}
                    {number(state?.troops ?? 0)}
                  </title>
                </path>
              );
            })}
            {map.provinces.map((p) => (
              <path
                key={p.id}
                d={p.path}
                fill={`url(#${uid}-land)`}
                fillRule="evenodd"
                pointerEvents="none"
              />
            ))}
            {plan
              .filter((o) => o.type === "move")
              .map((order) => {
                const from = provinceById.get(order.from ?? "")?.label,
                  to = provinceById.get(order.to ?? "")?.label;
                if (!from || !to) return null;
                return (
                  <path
                    key={order.id}
                    className="cq-planned-arrow"
                    d={`M${from.x},${from.y}L${to.x},${to.y}`}
                    fill="none"
                    stroke="#234d46"
                    strokeWidth="2.5"
                    strokeDasharray="5 4"
                    vectorEffect="non-scaling-stroke"
                    markerEnd={`url(#${uid}-arrow)`}
                    pointerEvents="none"
                  />
                );
              })}
            {selectedGeo && target && provinceById.has(target) && (
              <path
                d={`M${selectedGeo.label.x},${selectedGeo.label.y}L${provinceById.get(target)!.label.x},${provinceById.get(target)!.label.y}`}
                fill="none"
                stroke="#fff6ce"
                strokeWidth="3"
                strokeDasharray="6 4"
                vectorEffect="non-scaling-stroke"
                pointerEvents="none"
              />
            )}
          </g>
          {labels.map(({ province: p, point, width }) => {
            const state = stateById.get(p.id),
              ruler = data.players.find(
                (player) => !player.eliminated && player.king === p.id,
              );
            return (
              <g
                key={p.id}
                data-cq-label={p.id}
                transform={`translate(${point.x} ${point.y})`}
                className={`cq-troop-label ${p.id === selected ? "selected" : ""} ${p.id === target ? "target" : ""}`}
                role="button"
                tabIndex={-1}
                aria-label={`${p.name}，兵力 ${number(state?.troops ?? 0)}`}
                onClick={(e) => {
                  if (camera.canClick(e.detail)) choose(p.id);
                }}
              >
                <rect
                  x={-width / 2}
                  y="-11"
                  width={width}
                  height="22"
                  rx="5"
                  fill="#fff9e8"
                  stroke={
                    p.id === selected || p.id === target ? "#294b40" : "#647267"
                  }
                  strokeWidth={p.id === selected ? 2 : 1}
                />
                <text
                  textAnchor="middle"
                  dominantBaseline="central"
                  fill="#20372e"
                  fontSize="12"
                  fontWeight="750"
                >
                  {short(state?.troops ?? 0)}
                </text>
                {ruler && (
                  <g
                    transform={`translate(${-width / 2 - 10} -7)`}
                    aria-label="君主所在地区"
                  >
                    <path
                      d="M0 1 3 5 6 0 9 5 12 1 10 10H2Z"
                      fill="#eed187"
                      stroke="#665127"
                      strokeWidth="1.4"
                    />
                  </g>
                )}
              </g>
            );
          })}
          {activeEvents.map((event) => {
            const from = provinceById.get(event.from ?? "")?.label,
              to = provinceById.get(event.to ?? event.province ?? "")?.label;
            if (!to || event.until <= Math.max(frame, serverNow())) return null;
            const end = viewPoint(camera.camera, to),
              start = viewPoint(camera.camera, from ?? to),
              progress = Math.max(
                0,
                Math.min(
                  1,
                  ((frame || serverNow()) - event.at) /
                    Math.max(1, event.until - event.at),
                ),
              );
            return (
              <g key={event.id} data-cq-event={event.id} pointerEvents="none">
                <path
                  d={`M${start.x},${start.y}L${end.x},${end.y}`}
                  stroke="#fff9db"
                  strokeWidth="3"
                  strokeDasharray="6 4"
                />
                <circle
                  cx={start.x + (end.x - start.x) * progress}
                  cy={start.y + (end.y - start.y) * progress}
                  r="7"
                  fill={color(event.seat)}
                  stroke="#fff9db"
                  strokeWidth="2"
                />
              </g>
            );
          })}
        </svg>
        <div className="cq-minimap-wrap">
          <svg
            className="cq-minimap"
            viewBox={`0 0 ${map.width} ${map.height}`}
            preserveAspectRatio="none"
            role="img"
            aria-label="拖动小地图定位"
            onPointerDown={(e) => {
              e.stopPropagation();
              e.currentTarget.setPointerCapture(e.pointerId);
              miniLocate(e);
            }}
            onPointerMove={(e) => {
              e.stopPropagation();
              if (e.currentTarget.hasPointerCapture(e.pointerId)) miniLocate(e);
            }}
            onPointerUp={miniRelease}
            onPointerCancel={miniRelease}
          >
            <rect width={map.width} height={map.height} fill="#a8c6c8" />
            {map.provinces.map((p) => (
              <path
                key={p.id}
                d={p.path}
                fill={color(stateById.get(p.id)?.owner ?? -1)}
                fillRule="evenodd"
                stroke="#667565"
                strokeWidth=".5"
              />
            ))}
            <rect
              x={Math.max(0, viewportTop.x)}
              y={Math.max(0, viewportTop.y)}
              width={Math.min(
                map.width,
                camera.size.width / camera.camera.scale,
              )}
              height={Math.min(
                map.height,
                camera.size.height / camera.camera.scale,
              )}
              fill="#fff9db20"
              stroke="#fff9db"
              strokeWidth={map.width / 85}
            />
          </svg>
        </div>
        <div className="cq-map-scale" aria-hidden="true">
          {Math.round(camera.ratio * 100)}%
        </div>
      </div>
      <div className="cq-actions">
        <div className="cq-action-heading">
          <div>
            <strong>{selectedGeo?.name || "选择一个地区"}</strong>
            <span className="cq-selected-owner">
              {actual ? (
                <>
                  <i style={{ backgroundColor: color(actual.owner) }} />
                  <span>{ownerName(actual.owner)}</span>
                </>
              ) : (
                "点击地图，或从地区列表定位"
              )}
            </span>
            <span className="cq-selected-detail">
              {actual ? (
                <>
                  <span>兵力 {number(actual.troops)}</span>
                  {actual.fort && <Shield size={13} aria-label="要塞" />}
                  {actual.tower && <Flag size={13} aria-label="瞭望塔" />}
                </>
              ) : (
                "\u00a0"
              )}
            </span>
          </div>
          <button
            type="button"
            className="cq-plan-button"
            aria-label="查看行动计划"
            onClick={() => setOrdersOpen(true)}
          >
            <ScrollText size={17} />
            计划 <b>{plan.length}</b>
          </button>
        </div>
        <div className="cq-action-tabs" role="tablist" aria-label="地区操作">
          {(
            [
              ["move", "行军"],
              ["recruit", "征募"],
              ["build", "建设"],
              ["state", "国政"],
            ] as const
          ).map(([value, label]) => (
            <button
              key={value}
              type="button"
              role="tab"
              aria-selected={tab === value}
              className={tab === value ? "active" : ""}
              onClick={() => setTab(value)}
            >
              {label}
            </button>
          ))}
        </div>
        <div className="cq-action-content" role="tabpanel">
          {tab === "move" && (
            <div className="cq-move-form">
              <label>
                目标地区
                <select
                  aria-label="目标地区"
                  value={target}
                  disabled={!own || !canPlan}
                  onChange={(e) => setTarget(e.target.value)}
                >
                  <option value="">选择相邻地区</option>
                  {reachable.map((p) => (
                    <option key={p.id} value={p.id}>
                      {p.name} · {number(stateById.get(p.id)?.troops ?? 0)}
                      {selectedGeo?.seaNeighbors.includes(p.id)
                        ? " · 海路"
                        : ""}
                    </option>
                  ))}
                </select>
              </label>
              <div className="cq-amount">
                <label>
                  出兵数量
                  <input
                    aria-label="出兵数量"
                    inputMode="numeric"
                    type="number"
                    min="1"
                    max={maxMove}
                    disabled={!own || !canPlan || maxMove < 1}
                    value={amount}
                    onChange={(e) => setAmount(wholeAmount(e.target.value))}
                  />
                </label>
                <div className="cq-proportions">
                  {[0.25, 0.5, 1].map((fraction, i) => (
                    <button
                      type="button"
                      key={fraction}
                      disabled={!own || !canPlan || maxMove < 1}
                      onClick={() =>
                        setAmount(Math.max(1, Math.floor(maxMove * fraction)))
                      }
                    >
                      {["¼", "½", "全部"][i]}
                    </button>
                  ))}
                </div>
              </div>
              <div className="cq-move-bottom">
                <span>
                  {own
                    ? `可调动 ${number(mobile)} · ${data.controls.moveCost} 行动点`
                    : "选择己方地区后规划行军"}
                  {sea && (
                    <span className="cq-sea-caption">
                      <Ship size={13} />
                      海运 {number(amount * (data.controls.seaCost ?? 0))} 金币
                    </span>
                  )}
                </span>
                {own && mine?.king === selected && (
                  <label className="cq-checkbox">
                    <input
                      type="checkbox"
                      checked={king}
                      disabled={!canPlan}
                      onChange={(e) => setKing(e.target.checked)}
                    />
                    君主随军
                  </label>
                )}
              </div>
              <button
                type="button"
                className="cq-primary"
                disabled={
                  !canPlan || !own || !target || amount < 1 || amount > maxMove
                }
                onClick={() =>
                  void send("move", {
                    from: selected,
                    to: target,
                    amount,
                    king,
                  })
                }
              >
                加入行军计划
                <ArrowRight size={16} />
              </button>
            </div>
          )}
          {tab === "recruit" && (
            <div className="cq-recruit-form">
              <label>
                征募数量
                <input
                  type="number"
                  inputMode="numeric"
                  aria-label="征募数量"
                  min="1"
                  max={maxRecruit}
                  value={recruits}
                  disabled={!own || !canPlan}
                  onChange={(e) => setRecruits(wholeAmount(e.target.value))}
                />
              </label>
              <div className="cq-estimate">
                <span>
                  金币 <b>{number(recruits * data.controls.recruitCost)}</b>
                </span>
                <span>
                  行动点 <b>{data.controls.recruitMovement}</b>
                </span>
              </div>
              <button
                type="button"
                className="cq-primary"
                disabled={
                  !own || !canPlan || recruits < 1 || recruits > maxRecruit
                }
                onClick={() =>
                  void send("recruit", { province: selected, amount: recruits })
                }
              >
                加入征募计划
                <Plus size={16} />
              </button>
            </div>
          )}
          {tab === "build" && (
            <div className="cq-build-form">
              {(
                [
                  [
                    "fort",
                    "要塞",
                    data.controls.fortCost,
                    data.controls.fortMovement,
                  ],
                  [
                    "tower",
                    "瞭望塔",
                    data.controls.towerCost,
                    data.controls.towerMovement,
                  ],
                  [
                    "town",
                    "城镇",
                    data.controls.townCost ?? 0,
                    data.controls.townMovement ?? 0,
                  ],
                ] as const
              )
                .filter(
                  ([building]) =>
                    (building !== "tower" || data.options.fog) &&
                    (building !== "town" ||
                      data.controls.townCost !== undefined),
                )
                .map(([building, label, cost, movement]) => (
                  <button
                    key={building}
                    type="button"
                    disabled={
                      !own ||
                      !canPlan ||
                      !!projected?.[building] ||
                      data.availableGold < cost ||
                      data.availableMovement < movement
                    }
                    aria-label={`修建${label}`}
                    onClick={() =>
                      void send("build", { province: selected, building })
                    }
                  >
                    <Shield size={20} />
                    <strong>{label}</strong>
                    <span>
                      {projected?.[building]
                        ? "已修建或已规划"
                        : `${number(cost)} 金币 · ${movement} 行动点`}
                    </span>
                  </button>
                ))}
            </div>
          )}
          {tab === "state" && (
            <div className="cq-state-form">
              <div className="cq-estimate">
                <span>
                  预计收入 <b>{number(mine?.income ?? 0)}</b>
                </span>
                <span>
                  军队维护 <b>{number(mine?.upkeep ?? 0)}</b>
                </span>
                <span>
                  经济支出 <b>{number(mine?.economySpending ?? 0)}</b>
                </span>
              </div>
              <label className="cq-tax">
                税率{" "}
                <input
                  type="range"
                  aria-label="税率"
                  min="0"
                  max="100"
                  step="10"
                  value={tax}
                  disabled={!canPlan}
                  onChange={(e) => setTax(Number(e.target.value))}
                />
                <strong>{tax}%</strong>
              </label>
              <button
                type="button"
                disabled={!canPlan}
                onClick={() => void send("tax", { value: tax })}
              >
                调整税率
              </button>
              {own && (
                <button
                  type="button"
                  disabled={
                    !canPlan || mobile < 1 || data.availableMovement < 0.1
                  }
                  onClick={() =>
                    void send("disband", {
                      province: selected,
                      amount: Math.min(amount, mobile),
                    })
                  }
                >
                  解散选定兵力 · {number(Math.min(amount, mobile))}
                </button>
              )}
            </div>
          )}
        </div>
        <div className="cq-notice" role="status">
          {notice || "\u00a0"}
        </div>
        <div className="cq-budget">
          <span>
            可用金币{" "}
            <strong>
              {room.spectating
                ? "?"
                : data.availableGold.toLocaleString("zh-CN")}
            </strong>
          </span>
          <span>
            行动点 <strong>{data.availableMovement}</strong>
          </span>
        </div>
        <div className="cq-turn-actions">
          <button
            type="button"
            className="cq-primary"
            disabled={
              busy ||
              pending ||
              room.spectating ||
              !(data.controls.canSubmit || data.controls.canUnsubmit)
            }
            onClick={() =>
              void send(data.controls.canUnsubmit ? "unsubmit" : "submit")
            }
          >
            <Check size={17} />
            {data.controls.canUnsubmit ? "撤回提交" : "提交回合"}
          </button>
          {data.phase === "paused" ? (
            <button
              type="button"
              disabled={
                !data.controls.canResume || busy || pending || room.spectating
              }
              onClick={() =>
                void send("resume", { enabled: !data.resumeVotes[room.you] })
              }
            >
              {data.resumeVotes[room.you] ? "撤回恢复" : "同意恢复"}
            </button>
          ) : (
            <button
              type="button"
              disabled={
                !data.controls.canPause || busy || pending || room.spectating
              }
              onClick={() =>
                void send("pause", { enabled: !data.pauseVotes[room.you] })
              }
            >
              {data.pauseVotes[room.you] ? "撤回暂停" : "提议暂停"}
            </button>
          )}
        </div>
        {(data.pauseVotes.some(Boolean) ||
          data.resumeVotes.some(Boolean) ||
          data.phase === "paused") && (
          <div className="cq-votes">
            {data.phase === "paused" ? "恢复" : "暂停"}确认：
            {room.seats
              .filter(
                (_, i) =>
                  (data.phase === "paused"
                    ? data.resumeVotes[i]
                    : data.pauseVotes[i]) && !data.players[i]?.eliminated,
              )
              .map((s) => s.name)
              .join("、") || "等待同桌确认"}
          </div>
        )}
      </div>
      <details className="cq-journal">
        <summary>
          <span>战报</span>
          <ChevronDown size={15} />
        </summary>
        <ol>
          {(data.log ?? []).slice(-12).map((line, i) => (
            <li key={i}>{line}</li>
          ))}
        </ol>
        {data.controls.canResign && !room.spectating && (
          <button
            type="button"
            disabled={
              busy ||
              pending ||
              !!room.seats[room.you]?.autoPlay ||
              room.status !== "playing"
            }
            onClick={() => setResignOpen(true)}
          >
            退出争霸
          </button>
        )}
      </details>
      {ordersOpen && (
        <Modal
          title="本轮行动计划"
          onClose={() => setOrdersOpen(false)}
          className="cq-orders-modal"
        >
          <div className="cq-order-list">
            {plan.length ? (
              plan.map((order) => (
                <div key={order.id} className="cq-order">
                  <span>{orderDescription(order)}</span>
                  <button
                    type="button"
                    aria-label={`撤销指令 ${order.id}`}
                    disabled={!canPlan}
                    onClick={() => void send("undo", { id: order.id })}
                  >
                    <X size={16} />
                  </button>
                </div>
              ))
            ) : (
              <p>本轮还没有计划。</p>
            )}
          </div>
          <p className="cq-order-budget">
            预留后剩余：
            {room.spectating
              ? "?"
              : data.availableGold.toLocaleString("zh-CN")}{" "}
            金币 · {data.availableMovement} 行动点
          </p>
          {notice && <p role="alert">{notice}</p>}
        </Modal>
      )}
      {resignOpen && (
        <Modal title="退出本局争霸" onClose={() => setResignOpen(false)}>
          <p>你的领土将转为中立，其他势力继续争夺。退出后仍可观战。</p>
          <button
            type="button"
            className="primary"
            disabled={busy || pending}
            onClick={() => {
              setResignOpen(false);
              void send("resign");
            }}
          >
            确认退出争霸
          </button>
        </Modal>
      )}
    </section>
  );
}
