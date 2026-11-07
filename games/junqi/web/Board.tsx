import { useControllerCancel } from "../../../web/src/controllers/use-controller-cancel";
import { useEffect, useRef, useState, type CSSProperties } from "react";
import type { BoardProps } from "../../../web/src/game-api/module";
import { Modal } from "../../../web/src/ui/board-host";
import { useMapControls } from "../../../web/src/ui/map-controls";
import { colors, names, armyNames } from "../assets/art";
import type { View, Piece, Node, Move } from "./types";
const local = (file: string) => "/junqi/v1/" + file;
const source = (base: string, file: string) =>
  base.replace(/\/$/, "") + local(file);
const asset = (p: Piece) =>
  `images/${p.seat}-${p.kind === "unknown" ? "back" : p.kind}.webp`;
const nameOf = (p: Piece) => (p.kind === "unknown" ? "暗子" : names[p.kind]);
function Token({ piece, assets }: { piece: Piece; assets: string }) {
  const [failed, setFailed] = useState(false),
    file = asset(piece);
  useEffect(() => setFailed(false), [assets, file]);
  return (
    <span
      className="junqi-piece"
      style={{ "--army-color": colors[piece.seat] } as CSSProperties}
    >
      {failed ? (
        <span
          className={
            "junqi-piece-fallback " + (piece.kind === "unknown" ? "back" : "")
          }
        >
          {piece.kind === "unknown" ? "★" : names[piece.kind]}
        </span>
      ) : (
        <img
          draggable={false}
          alt=""
          src={source(assets, file)}
          onError={(e) => {
            if (e.currentTarget.getAttribute("src") !== local(file))
              e.currentTarget.src = local(file);
            else setFailed(true);
          }}
        />
      )}
    </span>
  );
}
const reasons: Record<string, string> = {
  last_team: "仅剩一个阵营，胜负已定",
  agreement: "存活玩家一致同意和棋",
  no_casualty: "连续 200 着无战损，和棋",
  repetition: "局面与手番四次重复，和棋",
};
export function Board({
  room,
  data,
  act,
  busy,
  assets,
  audio,
  clock,
}: BoardProps<View>) {
  const turns = room.spectating
    ? 0
    : Math.max(0, room.you) * (data.mode === "duel" ? 2 : 1);
  const rotate = (x: number, y: number) => {
    for (let k = 0; k < turns; k++) [x, y] = [y, -x];
    return { x, y };
  };
  const rawPoints = data.graph.nodes.map((n) => rotate(n.x, n.y));
  const minX = Math.min(...rawPoints.map((p) => p.x)) - 1,
    minY = Math.min(...rawPoints.map((p) => p.y)) - 1,
    W = Math.max(...rawPoints.map((p) => p.x)) - minX + 1,
    H = Math.max(...rawPoints.map((p) => p.y)) - minY + 1;
  const point = (id: number) => ({
    x: rawPoints[id].x - minX,
    y: rawPoints[id].y - minY,
  });
  const controls = useMapControls({
    aspect: W / H,
    fitMobile: true,
  });
  const overview =
    controls.mobile &&
    (controls.zoom <= 1.05 || (controls.mapStyle.width / W) * 0.9 < 28);
  const [selected, setSelected] = useState<string | null>(null),
    [other, setOther] = useState<string | null>(null),
    [target, setTarget] = useState<number | null>(null),
    [focus, setFocus] = useState(0),
    [resign, setResign] = useState(false),
    [windowRect, setWindowRect] = useState({ x: 0, y: 0, w: 1, h: 1 });
  const nodes = useRef<(HTMLButtonElement | null)[]>([]),
    pinch = useRef<{ distance: number; zoom: number } | null>(null);
  const controllerRoot = useRef<HTMLElement>(null);
  useControllerCancel(controllerRoot, () => clear());
  const scope = `${room.id}:${data.round}:${room.you}:${clock.epoch ?? 0}`;
  const seen = useRef({
    scope,
    id: data.last.id,
    phase: data.phase,
    pieces: data.pieces,
  });
  const canAct = !busy && !room.spectating && !room.seats[room.you]?.autoPlay;
  const canArrange = canAct && data.canArrange,
    canMove = canAct && data.canMove;
  const send = (type: string, extra: Record<string, unknown> = {}) =>
    act({
      type,
      round: data.round,
      ply: data.ply,
      revision: data.revision,
      ...extra,
    });
  const clear = () => {
    setSelected(null);
    setOther(null);
    setTarget(null);
  };
  useEffect(() => {
    clear();
    setResign(false);
  }, [scope, data.ply, data.revision, data.phase]);
  useEffect(() => {
    const old = seen.current;
    seen.current = {
      scope,
      id: data.last.id,
      phase: data.phase,
      pieces: data.pieces,
    };
    if (old.scope !== scope) {
      audio.stop();
      return;
    }
    let cue = "";
    if (data.phase === "finished" && old.phase !== "finished")
      cue = data.winner < 0 ? "draw" : "win";
    else if (data.last.id > old.id) {
      const e = data.last;
      if (e.type === "move") {
        cue = e.flagCaptured
          ? "flag"
          : e.commanders.length || e.moves.some((m) => m.turned)
            ? "reveal"
            : e.removed.length
              ? "capture"
              : "move";
        if (
          old.pieces.some((p) => p.kind === "bomb" && e.removed.includes(p.id))
        )
          cue = "bomb";
      } else if (
        e.type === "begin" ||
        (e.seat === room.you && (e.type === "shuffle" || e.type === "swap"))
      )
        cue = "ready";
    }
    if (cue)
      void audio.playWithFallback([
        source(assets, "audio/" + cue + ".wav"),
        local("audio/" + cue + ".wav"),
      ]);
  }, [scope, data.last.id, data.phase, assets, audio]);
  useEffect(() => () => audio.stop(), [audio]);
  useEffect(() => {
    const el = controls.viewport.current;
    if (!el) return;
    const start = (e: TouchEvent) => {
      if (e.touches.length === 2) {
        e.preventDefault();
        clear();
        pinch.current = {
          distance: Math.hypot(
            e.touches[0].clientX - e.touches[1].clientX,
            e.touches[0].clientY - e.touches[1].clientY,
          ),
          zoom: controls.zoom,
        };
      }
    };
    const move = (e: TouchEvent) => {
      const p = pinch.current;
      if (!p || e.touches.length !== 2) return;
      e.preventDefault();
      const a = e.touches[0],
        b = e.touches[1],
        r = el.getBoundingClientRect(),
        d = Math.hypot(a.clientX - b.clientX, a.clientY - b.clientY);
      controls.zoomAt(
        (p.zoom * d) / Math.max(1, p.distance),
        (a.clientX + b.clientX) / 2 - r.left,
        (a.clientY + b.clientY) / 2 - r.top,
      );
    };
    const end = (e: TouchEvent) => {
      if (e.touches.length < 2) pinch.current = null;
    };
    el.addEventListener("touchstart", start, { passive: false });
    el.addEventListener("touchmove", move, { passive: false });
    el.addEventListener("touchend", end);
    el.addEventListener("touchcancel", end);
    return () => {
      el.removeEventListener("touchstart", start);
      el.removeEventListener("touchmove", move);
      el.removeEventListener("touchend", end);
      el.removeEventListener("touchcancel", end);
    };
  }, [controls.zoomAt, controls.zoom]);
  const byPos = new Map(data.pieces.map((p) => [p.pos, p])),
    byID = new Map(data.pieces.map((p) => [p.id, p]));
  const legal = selected ? data.legal.filter((m) => m.unit === selected) : [],
    choice = legal.find((m) => m.to === target);
  const selectedPiece = selected ? byID.get(selected) : undefined,
    otherPiece = other ? byID.get(other) : undefined;
  const fits = (p: Piece, n: Node) =>
    !n.camp &&
    n.seat === p.seat &&
    (p.kind !== "flag" || n.hq) &&
    (p.kind !== "mine" || n.row >= 4) &&
    (p.kind !== "bomb" || n.row > 0);
  const swapOK =
    !!selectedPiece &&
    !!otherPiece &&
    fits(selectedPiece, data.graph.nodes[otherPiece.pos]) &&
    fits(otherPiece, data.graph.nodes[selectedPiece.pos]);
  const inspectArea = (id: number) => {
    const el = controls.viewport.current;
    if (!el || !controls.mapStyle.width) return;
    const p = point(id),
      base = controls.mapStyle.width / controls.zoom;
    const target = Math.min(
      3,
      Math.max(1.5, (data.mode === "teams" ? 720 : 330) / Math.max(1, base)),
    );
    controls.zoomAt(
      target,
      controls.mapStyle.left +
        (p.x / W) * controls.mapStyle.width -
        el.scrollLeft,
      controls.mapStyle.top +
        (p.y / H) * controls.mapStyle.height -
        el.scrollTop,
    );
  };
  const pick = (id: number) => {
    if (overview) {
      inspectArea(id);
      return;
    }
    const p = byPos.get(id);
    if (canArrange) {
      if (p?.seat !== room.you) return;
      if (!selected || selected === p.id) {
        setSelected(selected === p.id ? null : p.id);
        setOther(null);
      } else setOther(p.id);
      return;
    }
    if (!canMove) return;
    if (legal.some((m) => m.to === id)) {
      setTarget(id);
      return;
    }
    if (p?.seat === room.you && data.legal.some((m) => m.unit === p.id)) {
      setSelected(p.id);
      setTarget(null);
    } else clear();
  };
  const track = () => {
    const el = controls.viewport.current;
    if (el)
      setWindowRect({
        x: el.scrollLeft - controls.mapStyle.left,
        y: el.scrollTop - controls.mapStyle.top,
        w: el.clientWidth,
        h: el.clientHeight,
      });
  };
  useEffect(track, [controls.mapStyle.width, controls.mapStyle.height]);
  const reset = () => {
    controls.zoomAt(1);
    requestAnimationFrame(() => {
      const el = controls.viewport.current;
      if (el) {
        el.scrollLeft = Math.max(0, (el.scrollWidth - el.clientWidth) / 2);
        el.scrollTop = Math.max(0, (el.scrollHeight - el.clientHeight) / 2);
        track();
      }
    });
  };
  const locate = (e: React.PointerEvent<SVGSVGElement>) => {
    const el = controls.viewport.current;
    if (!el) return;
    const r = e.currentTarget.getBoundingClientRect(),
      x = Math.max(0, Math.min(1, (e.clientX - r.left) / r.width)),
      y = Math.max(0, Math.min(1, (e.clientY - r.top) / r.height));
    el.scrollLeft =
      controls.mapStyle.left + x * controls.mapStyle.width - el.clientWidth / 2;
    el.scrollTop =
      controls.mapStyle.top +
      y * controls.mapStyle.height -
      el.clientHeight / 2;
    track();
  };
  const curves = new Set(
    data.graph.edges
      .filter((e) => e.curve)
      .map((e) => [e.a, e.b].sort((a, b) => a - b).join("-")),
  );
  const segment = (a: number, b: number) => {
    const q = point(b);
    if (curves.has([a, b].sort((a, b) => a - b).join("-"))) {
      const first = data.graph.nodes[a],
        second = data.graph.nodes[b],
        c = rotate(
          Math.sign(first.x + second.x) * 2,
          Math.sign(first.y + second.y) * 2,
        );
      return `Q${c.x - minX} ${c.y - minY} ${q.x} ${q.y}`;
    }
    return `L${q.x} ${q.y}`;
  };
  const path = (list: number[]) =>
    list
      .map((id, i) =>
        i ? segment(list[i - 1], id) : `M${point(id).x} ${point(id).y}`,
      )
      .join(" ");
  const motion = data.motion,
    event = motion?.event,
    movingIDs = new Set(event?.moves.map((m) => m.unit) ?? []);
  const label = (n: Node) =>
    n.seat >= 0
      ? `${armyNames[n.seat]}阵地，第 ${n.row + 1} 排第 ${n.col + 1} 列`
      : `中央铁路 ${n.row + 1} 行 ${n.col + 1} 列`;
  const ownPlayer = data.players[room.you];
  return (
    <section
      ref={controllerRoot}
      className={"junqi-board " + data.mode + (overview ? " overview" : "")}
      data-overview={overview}
      data-ply={data.ply}
      data-phase={data.phase}
      data-revision={data.revision}
      data-moving={event?.id ?? ""}
      data-zoom={controls.zoom}
    >
      <div className="junqi-heading">
        <div>
          <h2>{data.mode === "duel" ? "两人陆战棋" : "四国军棋 · 对家搭档"}</h2>
          <p>暗棋 · 行营内免受攻击 · 进入大本营后不能移动</p>
        </div>
        <strong>
          {data.phase === "deploy"
            ? `布阵确认 ${data.players.filter((p) => p.ready).length}/${data.players.length}`
            : data.phase === "finished"
              ? "本局已结束"
              : `第 ${data.ply + 1} 着 · ${armyNames[data.turn]}行动`}
        </strong>
      </div>
      <div className="junqi-workspace">
        <div className="junqi-play-area">
          <div
            className="junqi-map-tools"
            role="group"
            aria-label="军棋地图视图"
          >
            <button
              aria-label="缩小棋盘"
              disabled={controls.zoom <= 1}
              onClick={() => controls.zoomAt(controls.zoom / 1.2)}
            >
              −
            </button>
            <span>{Math.round(controls.zoom * 100)}%</span>
            <button
              aria-label="放大棋盘"
              disabled={controls.zoom >= 3}
              onClick={() => controls.zoomAt(controls.zoom * 1.2)}
            >
              ＋
            </button>
            <button onClick={reset}>全图</button>
            <span>
              {overview ? "点击阵地放大操作" : "拖动查看 · 双指／滚轮缩放"}
            </span>
          </div>
          <p className="junqi-orientation">
            {room.spectating
              ? "正在观战，暗子保持隐藏"
              : `你是${armyNames[room.you]}，己方在下方`}
            {data.mode === "teams" && !room.spectating
              ? ` · 对家是${armyNames[(room.you + 2) % 4]}`
              : ""}
          </p>
          <div
            className={
              "junqi-viewport " + (controls.dragging ? "dragging" : "")
            }
            ref={controls.viewport}
            {...controls.handlers}
            onScroll={track}
          >
            <div style={controls.canvasStyle}>
              <div
                data-controller-group="board"
                className="junqi-map"
                style={controls.mapStyle}
                role="group"
                aria-label="军棋棋盘"
              >
                <svg
                  className="junqi-grid"
                  viewBox={`0 0 ${W} ${H}`}
                  aria-hidden="true"
                >
                  <rect width={W} height={H} rx=".35" fill="#e5e1c7" />
                  <image
                    href={source(assets, `images/board-${data.mode}.webp`)}
                    width={W}
                    height={H}
                    onError={(e) => {
                      const file = local(`images/board-${data.mode}.webp`);
                      if (e.currentTarget.getAttribute("href") !== file)
                        e.currentTarget.setAttribute("href", file);
                      else e.currentTarget.style.display = "none";
                    }}
                  />
                  {data.players.map((p) => {
                    const points = data.graph.nodes
                        .filter((n) => n.seat === p.seat)
                        .map((n) => point(n.id)),
                      x = Math.min(...points.map((p) => p.x)) - 0.5,
                      y = Math.min(...points.map((p) => p.y)) - 0.5,
                      w = Math.max(...points.map((p) => p.x)) - x + 0.5,
                      h = Math.max(...points.map((p) => p.y)) - y + 0.5;
                    return (
                      <rect
                        key={p.seat}
                        x={x}
                        y={y}
                        width={w}
                        height={h}
                        rx=".25"
                        fill={colors[p.seat]}
                        fillOpacity=".09"
                        stroke={colors[p.seat]}
                        strokeWidth=".025"
                      />
                    );
                  })}
                  {data.graph.edges.map((e) => (
                    <path
                      key={`${e.a}-${e.b}`}
                      d={path([e.a, e.b])}
                      fill="none"
                      stroke="#65715c"
                      strokeWidth={e.lines.length ? 0.13 : 0.04}
                      strokeLinecap="round"
                    />
                  ))}
                  {data.graph.edges
                    .filter((e) => e.lines.length)
                    .map((e) => (
                      <path
                        key={`${e.a}-${e.b}`}
                        d={path([e.a, e.b])}
                        fill="none"
                        stroke="#f0ebd5"
                        strokeWidth=".045"
                      />
                    ))}
                  {data.graph.nodes.map((n) => {
                    const p = point(n.id);
                    return (
                      <g key={n.id}>
                        {n.camp ? (
                          <>
                            <circle
                              cx={p.x}
                              cy={p.y}
                              r=".48"
                              fill="#dce9d0"
                              stroke="#41613e"
                              strokeWidth=".055"
                            />
                            <text
                              x={p.x}
                              y={p.y + 0.1}
                              textAnchor="middle"
                              fontSize=".28"
                              fill="#355331"
                            >
                              营
                            </text>
                          </>
                        ) : n.hq ? (
                          <>
                            <rect
                              x={p.x - 0.43}
                              y={p.y - 0.36}
                              width=".86"
                              height=".72"
                              rx=".1"
                              fill="#eee0b5"
                              stroke="#705b30"
                              strokeWidth=".06"
                            />
                            <text
                              x={p.x}
                              y={p.y + 0.1}
                              textAnchor="middle"
                              fontSize=".24"
                              fill="#5e4824"
                            >
                              总部
                            </text>
                          </>
                        ) : (
                          !byPos.has(n.id) && (
                            <circle cx={p.x} cy={p.y} r=".09" fill="#68735d" />
                          )
                        )}
                      </g>
                    );
                  })}
                </svg>
                {data.graph.nodes.map((n) => {
                  const p = point(n.id),
                    unit = byPos.get(n.id),
                    destination = legal.some((m) => m.to === n.id);
                  return (
                    <button
                      type="button"
                      key={n.id}
                      ref={(el) => {
                        nodes.current[n.id] = el;
                      }}
                      data-junqi-node={n.id}
                      data-piece-seat={unit?.seat}
                      className={
                        "junqi-node" +
                        (unit && (unit.id === selected || unit.id === other)
                          ? " selected"
                          : "") +
                        (n.id === target ? " target" : "")
                      }
                      style={{
                        left: (p.x / W) * 100 + "%",
                        top: (p.y / H) * 100 + "%",
                        width: (0.9 / W) * 100 + "%",
                        height: (0.82 / H) * 100 + "%",
                      }}
                      aria-label={`${label(n)}，${unit ? `${armyNames[unit.seat]} ${nameOf(unit)}` : n.camp ? "空行营" : n.hq ? "空大本营" : "空点"}${destination ? "，可到达" : ""}`}
                      aria-pressed={
                        unit?.id === selected ||
                        unit?.id === other ||
                        target === n.id
                      }
                      aria-disabled={!overview && !canArrange && !canMove}
                      tabIndex={focus === n.id ? 0 : -1}
                      onFocus={() => setFocus(n.id)}
                      onClick={() => pick(n.id)}
                      onKeyDown={(e) => {
                        const dx =
                            e.key === "ArrowLeft"
                              ? -1
                              : e.key === "ArrowRight"
                                ? 1
                                : 0,
                          dy =
                            e.key === "ArrowUp"
                              ? -1
                              : e.key === "ArrowDown"
                                ? 1
                                : 0;
                        if (dx || dy) {
                          e.preventDefault();
                          const next = data.graph.nodes
                            .filter((a) => {
                              const q = point(a.id);
                              return dx
                                ? (q.x - p.x) * dx > 0.01
                                : (q.y - p.y) * dy > 0.01;
                            })
                            .sort((a, b) => {
                              const score = (id: number) => {
                                const q = point(id);
                                return (
                                  Math.abs(q.x - p.x) * (dy ? 4 : 1) +
                                  Math.abs(q.y - p.y) * (dx ? 4 : 1)
                                );
                              };
                              return score(a.id) - score(b.id);
                            })[0];
                          if (next) {
                            setFocus(next.id);
                            nodes.current[next.id]?.focus();
                            nodes.current[next.id]?.scrollIntoView({
                              block: "nearest",
                              inline: "nearest",
                            });
                          }
                        }
                      }}
                    >
                      {unit && !movingIDs.has(unit.id) && (
                        <Token piece={unit} assets={assets} />
                      )}{" "}
                      {destination && (
                        <span
                          className={
                            "junqi-destination " + (unit ? "capture" : "")
                          }
                        />
                      )}
                    </button>
                  );
                })}
                {event && (
                  <svg
                    className="junqi-motion-layer"
                    viewBox={`0 0 ${W} ${H}`}
                    aria-hidden="true"
                  >
                    {event.moves.map((m: Move) => {
                      const p = byID.get(m.unit);
                      if (!p) return null;
                      const file = asset(p);
                      return (
                        <g
                          key={event.id + "-" + m.unit}
                          className="junqi-moving"
                        >
                          <rect
                            x="-.42"
                            y="-.34"
                            width=".84"
                            height=".68"
                            rx=".09"
                            fill="#fff2d2"
                            stroke={colors[p.seat]}
                            strokeWidth=".055"
                          />
                          <text
                            y=".1"
                            textAnchor="middle"
                            fill={colors[p.seat]}
                            fontWeight="700"
                            fontSize=".28"
                          >
                            {p.kind === "unknown" ? "★" : names[p.kind]}
                          </text>
                          <image
                            x="-.45"
                            y="-.41"
                            width=".9"
                            height=".82"
                            href={source(assets, file)}
                            onError={(e) => {
                              if (
                                e.currentTarget.getAttribute("href") !==
                                local(file)
                              )
                                e.currentTarget.setAttribute(
                                  "href",
                                  local(file),
                                );
                              else e.currentTarget.style.display = "none";
                            }}
                          />
                          <animateMotion
                            path={path(m.path)}
                            dur={`${motion!.duration}ms`}
                            fill="freeze"
                            calcMode="paced"
                          />
                        </g>
                      );
                    })}
                  </svg>
                )}
              </div>
            </div>
          </div>
        </div>
        <aside data-controller-group="actions" className="junqi-actions">
          <div className="junqi-overview">
            <div>
              <strong>整盘概览</strong>
              <p>拖动概览定位。手机支持双指缩放。</p>
            </div>
            <svg
              className="junqi-minimap"
              viewBox={`0 0 ${W} ${H}`}
              role="img"
              aria-label="拖动军棋概览定位棋盘"
              onPointerDown={(e) => {
                e.currentTarget.setPointerCapture(e.pointerId);
                locate(e);
              }}
              onPointerMove={(e) => {
                if (e.currentTarget.hasPointerCapture(e.pointerId)) locate(e);
              }}
              onPointerUp={(e) => {
                if (e.currentTarget.hasPointerCapture(e.pointerId))
                  e.currentTarget.releasePointerCapture(e.pointerId);
              }}
              onPointerCancel={(e) => {
                if (e.currentTarget.hasPointerCapture(e.pointerId))
                  e.currentTarget.releasePointerCapture(e.pointerId);
              }}
            >
              <rect width={W} height={H} fill="#e5e1c7" />
              {data.graph.edges.map((e) => (
                <path
                  key={e.a + "-" + e.b}
                  d={path([e.a, e.b])}
                  fill="none"
                  stroke="#8c967e"
                  strokeWidth={e.lines.length ? 0.12 : 0.05}
                />
              ))}
              {data.pieces.map((p) => {
                const q = point(p.pos);
                return (
                  <rect
                    key={p.id}
                    x={q.x - 0.32}
                    y={q.y - 0.25}
                    width=".64"
                    height=".5"
                    rx=".08"
                    fill={colors[p.seat]}
                  />
                );
              })}
              <rect
                x={
                  (Math.max(0, windowRect.x) /
                    Math.max(1, controls.mapStyle.width)) *
                  W
                }
                y={
                  (Math.max(0, windowRect.y) /
                    Math.max(1, controls.mapStyle.height)) *
                  H
                }
                width={Math.min(
                  W,
                  (windowRect.w / Math.max(1, controls.mapStyle.width)) * W,
                )}
                height={Math.min(
                  H,
                  (windowRect.h / Math.max(1, controls.mapStyle.height)) * H,
                )}
                fill="#24553e12"
                stroke="#24553e"
                strokeWidth=".17"
              />
            </svg>
          </div>
          <h3>
            {data.phase === "finished"
              ? reasons[data.reason]
              : room.spectating
                ? "正在观战"
                : ownPlayer && !ownPlayer.alive
                  ? "已出局，可继续观战"
                  : data.phase === "deploy"
                    ? ownPlayer?.ready
                      ? "等待其他玩家确认"
                      : "调整己方布阵"
                    : event
                      ? "正在走子"
                      : canMove
                        ? "轮到你行动"
                        : "等待其他玩家行动"}
          </h3>
          <p role="status">
            {overview
              ? "当前为完整棋盘概览。点选阵地放大后，再选择棋子与操作。"
              : data.phase === "deploy"
                ? otherPiece && selectedPiece
                  ? `待交换：${nameOf(selectedPiece)}与${nameOf(otherPiece)}，尚未提交。`
                  : selectedPiece
                    ? `已选${nameOf(selectedPiece)}，再选一枚己方棋子。`
                    : "选择两枚己方棋子交换；军旗须在大本营，地雷在最后两排，炸弹不能在前排。"
                : selectedPiece
                  ? target === null
                    ? `已选${nameOf(selectedPiece)}，请选择标记的合法目标。`
                    : "目标已选择，确认才走子。"
                  : "棋子越大不一定越安全，留意地雷、炸弹与军旗。"}
          </p>
          {data.phase === "deploy" ? (
            <>
              <button
                data-controller-action="swap"
                disabled={overview || !canArrange || !swapOK}
                onClick={() => {
                  if (selected && other)
                    void send("swap", { unit: selected, other });
                }}
              >
                交换位置
              </button>
              {otherPiece && !swapOK && <p>该交换不符合布阵限制。</p>}
              <button
                disabled={!canArrange}
                onClick={() => void send("shuffle")}
              >
                随机布阵
              </button>
              {data.canUnready ? (
                <button
                  disabled={!canAct}
                  onClick={() => void send("deploy_ready", { ready: false })}
                >
                  继续调整
                </button>
              ) : (
                <button
                  className="primary"
                  disabled={!canAct || !data.canConfirm}
                  data-controller-action="deploy-ready"
                  onClick={() => void send("deploy_ready", { ready: true })}
                >
                  完成布阵
                </button>
              )}
            </>
          ) : (
            <>
              <button
                className="primary"
                data-junqi-confirm
                disabled={overview || !canMove || !choice}
                onClick={() => {
                  if (choice)
                    void send("move", { unit: choice.unit, to: choice.to });
                }}
              >
                确认走子
              </button>
              <button
                disabled={!canAct || !data.canOffer}
                onClick={() => void send("offer_draw")}
              >
                提议和棋
              </button>
              <button
                disabled={!canAct || !data.canResign}
                data-controller-action="resign"
                onClick={() => setResign(true)}
              >
                认输
              </button>
            </>
          )}
          {data.offer && (
            <div className="junqi-draw-offer">
              <p>
                {armyNames[data.offer.by]}
                提议和棋，需要所有存活玩家同意。计时继续。
              </p>
              {data.canVote ? (
                <>
                  <button
                    disabled={!canAct}
                    onClick={() =>
                      void send("vote_draw", {
                        offerID: data.offer!.id,
                        accept: true,
                      })
                    }
                  >
                    同意和棋
                  </button>
                  <button
                    disabled={!canAct}
                    onClick={() =>
                      void send("vote_draw", {
                        offerID: data.offer!.id,
                        accept: false,
                      })
                    }
                  >
                    继续对局
                  </button>
                </>
              ) : (
                <p>已同意，等待其他玩家回应。</p>
              )}
            </div>
          )}
          {data.phase !== "deploy" && (
            <p>已连续 {data.noCasualty} 着无战损，200 着自动和棋。</p>
          )}
          <p>暗子不向队友或观众公开。真人和棋双方 0 积分，不计胜场。</p>
        </aside>
      </div>
      {resign && (
        <Modal title="确认认输" onClose={() => setResign(false)}>
          <p>
            {data.mode === "teams"
              ? "认输会清除你的棋子，对家仍可继续对局。"
              : "认输会结束本局。"}
          </p>
          <div className="button-row">
            <button onClick={() => setResign(false)}>继续对局</button>
            <button
              className="danger"
              disabled={!canAct}
              onClick={() => {
                setResign(false);
                void send("resign");
              }}
            >
              确认认输
            </button>
          </div>
        </Modal>
      )}
    </section>
  );
}
