import { useControllerCancel } from "../../../web/src/controllers/use-controller-cancel";
import { useEffect, useRef, useState } from "react";
import type { CSSProperties, PointerEvent as ReactPointerEvent } from "react";
import type { BoardProps } from "../../../web/src/game-api/module";
import { Modal } from "../../../web/src/ui/board-host";
import { useMapControls } from "../../../web/src/ui/map-controls";
import type { View } from "./types";
const local = (path: string) => `/weiqi/v1/${path}`;
const source = (base: string, path: string) =>
  `${base.replace(/\/$/, "")}${local(path)}`;
function Stone({ color, assets }: { color: number; assets: string }) {
  const [failed, setFailed] = useState(false),
    path = `images/stone-${color === 1 ? "black" : "white"}.webp`;
  useEffect(() => setFailed(false), [assets, path]);
  return failed ? (
    <span className={`weiqi-stone-fallback colour-${color}`} />
  ) : (
    <img
      draggable={false}
      alt=""
      src={source(assets, path)}
      onError={(e) => {
        if (e.currentTarget.getAttribute("src") !== local(path))
          e.currentTarget.src = local(path);
        else setFailed(true);
      }}
    />
  );
}
export function Board({
  room,
  data,
  act,
  busy,
  assets,
  audio,
  clock,
}: BoardProps<View>) {
  const n = data.options.size,
    controls = useMapControls({ aspect: 1, fitMobile: true });
  const controllerRoot = useRef<HTMLElement>(null);
  const [selected, setSelected] = useState<number | null>(null),
    [focus, setFocus] = useState((n * n) >> 1),
    [resign, setResign] = useState(false),
    [windowRect, setWindowRect] = useState({ x: 0, y: 0, w: 600, h: 600 });
  const pinch = useRef<{ distance: number; zoom: number } | null>(null);
  useControllerCancel(controllerRoot, () => {
    setSelected(null);
  });
  const cells = useRef<(HTMLButtonElement | null)[]>([]),
    scope = `${room.id}:${data.round}:${room.you}:${clock.epoch ?? 0}`,
    seen = useRef({ scope, id: data.last.id });
  const overview = (controls.mapStyle.width / (n + 1)) * 0.8 < 28,
    enabled =
      !busy &&
      !room.spectating &&
      !room.seats[room.you]?.autoPlay &&
      !data.motion,
    canPlay = data.canPlay && enabled,
    canScore = data.canScore && enabled;
  const motion = data.motion?.event,
    dead = new Set(data.dead),
    removed = new Set(motion?.removed ?? []),
    label = (p: number) =>
      `${String.fromCharCode(65 + (p % n))}${n - Math.floor(p / n)}`,
    send = (type: string, extra: Record<string, unknown> = {}) =>
      act({
        type,
        round: data.round,
        ply: data.ply,
        revision: data.revision,
        ...extra,
      });
  useEffect(() => {
    setSelected(null);
    setResign(false);
  }, [scope, data.ply, data.revision, data.phase]);
  useEffect(() => {
    if (!canPlay && !canScore) setSelected(null);
  }, [canPlay, canScore]);
  useEffect(() => {
    const prev = seen.current;
    seen.current = { scope, id: data.last.id };
    if (prev.scope !== scope) {
      audio.stop();
      return;
    }
    if (data.last.id > prev.id) {
      const cue =
        data.phase === "finished"
          ? data.winner < 0
            ? "draw"
            : "win"
          : data.last.type === "place"
            ? data.last.removed?.length
              ? "capture"
              : "place"
            : data.last.type === "pass"
              ? "pass"
              : null;
      if (cue)
        void audio.playWithFallback([
          source(assets, `audio/${cue}.wav`),
          local(`audio/${cue}.wav`),
        ]);
    }
  }, [scope, data.last.id, data.phase, audio, assets]);
  useEffect(() => () => audio.stop(), [audio]);
  useEffect(() => {
    const el = controls.viewport.current;
    if (!el) return;
    const start = (e: TouchEvent) => {
      if (e.touches.length === 2) {
        e.preventDefault();
        setSelected(null);
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
  const track = () => {
    const e = controls.viewport.current;
    if (e)
      setWindowRect({
        x: e.scrollLeft - controls.mapStyle.left,
        y: e.scrollTop - controls.mapStyle.top,
        w: e.clientWidth,
        h: e.clientHeight,
      });
  };
  useEffect(track, [controls.mapStyle.width, controls.mapStyle.height]);
  const reset = () => {
    setSelected(null);
    controls.zoomAt(1);
    requestAnimationFrame(() => {
      const e = controls.viewport.current;
      if (e) {
        e.scrollLeft = 0;
        e.scrollTop = 0;
        track();
      }
    });
  };
  const pick = (p: number) => {
    if (overview) {
      const e = controls.viewport.current;
      if (!e) return;
      controls.zoomAt(
        (controls.zoom * 30 * (n + 1)) / (0.8 * controls.mapStyle.width),
        controls.mapStyle.left +
          (((p % n) + 1) / (n + 1)) * controls.mapStyle.width -
          e.scrollLeft,
        controls.mapStyle.top +
          ((Math.floor(p / n) + 1) / (n + 1)) * controls.mapStyle.height -
          e.scrollTop,
      );
      return;
    }
    if ((canPlay && data.legal.includes(p)) || (canScore && data.board[p]))
      setSelected(selected === p ? null : p);
  };
  const locate = (event: ReactPointerEvent<SVGSVGElement>) => {
    const e = controls.viewport.current;
    if (!e) return;
    const r = event.currentTarget.getBoundingClientRect(),
      u = Math.max(0, Math.min(1, (event.clientX - r.left) / r.width)),
      v = Math.max(0, Math.min(1, (event.clientY - r.top) / r.height));
    e.scrollLeft =
      controls.mapStyle.left + u * controls.mapStyle.width - e.clientWidth / 2;
    e.scrollTop =
      controls.mapStyle.top + v * controls.mapStyle.height - e.clientHeight / 2;
    track();
  };
  const step = 600 / (n + 1),
    point = (p: number) => ({
      x: ((p % n) + 1) * step,
      y: (Math.floor(p / n) + 1) * step,
    }),
    grid = Array.from({ length: n }, (_, i) => (
      <path
        key={i}
        d={`M${step} ${(i + 1) * step}H${n * step}M${(i + 1) * step} ${step}V${n * step}`}
      />
    )),
    star = n === 9 ? [2, 4, 6] : n === 13 ? [3, 6, 9] : [3, 9, 15];
  return (
    <section
      ref={controllerRoot}
      className="weiqi-board"
      data-ply={data.ply}
      data-phase={data.phase}
      data-revision={data.revision}
      data-zoom={controls.zoom}
      data-overview={overview}
      data-selected={selected ?? ""}
      data-moving={motion?.id ?? ""}
      style={{ "--weiqi-cell": `${100 / (n + 1)}%` } as CSSProperties}
    >
      <div className="weiqi-heading">
        <div>
          <h2>{n} 路围棋</h2>
          <p>
            面积计分 · 禁自杀 · 全局同形 ·{" "}
            {data.options.komi ? `白贴 ${data.options.komi} 目` : "不贴目"}
          </p>
        </div>
        <strong>
          {data.phase === "finished"
            ? "本局已结束"
            : data.phase === "scoring"
              ? "确认死子与计分"
              : `第 ${data.ply + 1} 手 · ${data.turn === data.black ? "黑方" : "白方"}行棋`}
        </strong>
      </div>
      <div className="weiqi-workspace">
        <div className="weiqi-play-area">
          <div
            className="weiqi-map-tools"
            role="group"
            aria-label="围棋棋盘视图"
          >
            <button
              type="button"
              aria-label="缩小棋盘"
              disabled={controls.zoom <= 1}
              onClick={() => controls.zoomAt(controls.zoom / 1.2)}
            >
              −
            </button>
            <span>{Math.round(controls.zoom * 100)}%</span>
            <button
              type="button"
              aria-label="放大棋盘"
              disabled={controls.zoom >= 3}
              onClick={() => controls.zoomAt(controls.zoom * 1.2)}
            >
              ＋
            </button>
            <button type="button" onClick={reset}>
              全图
            </button>
            <span>
              {overview ? "点击区域放大操作" : "拖动查看 · 双指／滚轮缩放"}
            </span>
          </div>
          <div
            className={`weiqi-viewport ${controls.dragging ? "dragging" : ""}`}
            ref={controls.viewport}
            {...controls.handlers}
            onScroll={track}
          >
            <div style={controls.canvasStyle}>
              <div
                data-controller-group="board"
                className="weiqi-map"
                style={controls.mapStyle}
                role="group"
                aria-label={`${n} 路围棋棋盘`}
              >
                <svg
                  viewBox="0 0 600 600"
                  className="weiqi-grid"
                  aria-hidden="true"
                >
                  <rect width="600" height="600" fill="#dfc493" />
                  <image
                    href={source(assets, "images/board.webp")}
                    width="600"
                    height="600"
                    onError={(e) => {
                      if (
                        e.currentTarget.getAttribute("href") !==
                        local("images/board.webp")
                      )
                        e.currentTarget.setAttribute(
                          "href",
                          local("images/board.webp"),
                        );
                      else e.currentTarget.style.display = "none";
                    }}
                  />
                  <g stroke="#6b5839" strokeWidth="1.3" fill="none">
                    {grid}
                  </g>
                  {star.flatMap((y) =>
                    star.map((x) => (
                      <circle
                        key={`${x}-${y}`}
                        cx={(x + 1) * step}
                        cy={(y + 1) * step}
                        r={2.7}
                        fill="#655037"
                      />
                    )),
                  )}
                  {Array.from({ length: n }, (_, i) => (
                    <g
                      key={i}
                      fill="#493c2d"
                      fontSize={Math.min(12, step * 0.38)}
                      textAnchor="middle"
                    >
                      <text x={(i + 1) * step} y={step * 0.55}>
                        {String.fromCharCode(65 + i)}
                      </text>
                      <text x={step * 0.4} y={(i + 1) * step + 4}>
                        {n - i}
                      </text>
                    </g>
                  ))}
                </svg>
                {data.board.map((c, p) => {
                  const q = point(p),
                    arriving = motion?.type === "place" && motion.pos === p,
                    color = arriving ? (motion.seat === data.black ? 1 : 2) : c;
                  return (
                    <button
                      type="button"
                      key={p}
                      ref={(e) => {
                        cells.current[p] = e;
                      }}
                      data-weiqi-cell={p}
                      className="weiqi-cell"
                      style={{ left: `${q.x / 6}%`, top: `${q.y / 6}%` }}
                      aria-label={`${label(p)}，${c === 1 ? "黑棋" : c === 2 ? "白棋" : "空点"}${dead.has(p) ? "，标记死子" : ""}`}
                      aria-disabled={
                        !overview &&
                        (!canPlay || !data.legal.includes(p)) &&
                        (!canScore || !c)
                      }
                      aria-pressed={selected === p}
                      tabIndex={focus === p ? 0 : -1}
                      onFocus={() => setFocus(p)}
                      onClick={() => pick(p)}
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
                          const next =
                            Math.max(
                              0,
                              Math.min(n - 1, Math.floor(p / n) + dy),
                            ) *
                              n +
                            Math.max(0, Math.min(n - 1, (p % n) + dx));
                          setFocus(next);
                          cells.current[next]?.focus();
                          cells.current[next]?.scrollIntoView({
                            block: "nearest",
                            inline: "nearest",
                          });
                        }
                      }}
                    >
                      {!!color && (
                        <span
                          className={`weiqi-stone colour-${color} ${arriving ? "arriving" : ""} ${removed.has(p) ? "removing" : ""} ${dead.has(p) ? "dead" : ""} ${data.last.type === "place" && data.last.pos === p ? "last" : ""}`}
                        >
                          <Stone color={color} assets={assets} />
                        </span>
                      )}
                      {!color && selected === p && (
                        <span
                          className={`weiqi-preview colour-${data.turn === data.black ? 1 : 2}`}
                        />
                      )}
                      <span
                        hidden={
                          data.phase === "playing" ||
                          !!color ||
                          !data.score.ownership[p]
                        }
                        className={`weiqi-territory ${data.score.ownership[p] === 1 ? "black" : "white"}`}
                      />
                    </button>
                  );
                })}
              </div>
            </div>
          </div>
        </div>
        <aside data-controller-group="actions" className="weiqi-actions">
          <div className="weiqi-minimap-row">
            <div>
              <strong>整盘概览</strong>
              <p>
                拖动小棋盘定位。
                {overview
                  ? "全图用于观察，点区域放大后操作。"
                  : "选点后确认才落子。"}
              </p>
            </div>
            <svg
              className="weiqi-minimap"
              viewBox="0 0 600 600"
              role="img"
              aria-label="拖动围棋概览定位"
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
              <rect width="600" height="600" fill="#dfc493" />
              <g fill="none" stroke="#998360" strokeWidth="1.8">
                {grid}
              </g>
              {data.board.map((c, p) =>
                c ? (
                  <circle
                    key={p}
                    cx={point(p).x}
                    cy={point(p).y}
                    r={step * 0.36}
                    fill={c === 1 ? "#263633" : "#fffdf1"}
                    stroke="#53624d"
                  />
                ) : null,
              )}
              <rect
                x={
                  (Math.max(0, windowRect.x) /
                    Math.max(1, controls.mapStyle.width)) *
                  600
                }
                y={
                  (Math.max(0, windowRect.y) /
                    Math.max(1, controls.mapStyle.height)) *
                  600
                }
                width={Math.min(
                  600,
                  (windowRect.w / Math.max(1, controls.mapStyle.width)) * 600,
                )}
                height={Math.min(
                  600,
                  (windowRect.h / Math.max(1, controls.mapStyle.height)) * 600,
                )}
                fill="#2d725315"
                stroke="#285e47"
                strokeWidth="7"
              />
            </svg>
          </div>
          {(data.phase === "scoring" ||
            (data.phase === "finished" && data.reason === "score")) && (
            <div className="weiqi-score-totals">
              <strong>黑 {data.score.black}</strong>
              <strong>白 {data.score.white}</strong>
              <span>白含贴目 · 中立 {data.score.neutral}</span>
            </div>
          )}
          {data.phase === "scoring" ? (
            <div className="weiqi-scoring">
              <h3>确认死子与计分</h3>
              <p>
                点选棋块后切换死子标记。仅在双方确认后移除；有争议可继续对弈。
              </p>
              <p>
                {[0, 1]
                  .map(
                    (seat) =>
                      `${seat === data.black ? "黑方" : "白方"}${data.confirmed[seat] ? "已确认" : "待确认"}`,
                  )
                  .join(" · ")}
              </p>
              <button
                type="button"
                data-weiqi-mark-dead
                disabled={!canScore || selected === null}
                onClick={() => {
                  if (selected !== null)
                    void send("mark_dead", { pos: selected });
                }}
              >
                切换整块死子标记
              </button>
              <button
                type="button"
                className="primary"
                disabled={!canScore || data.confirmed[room.you]}
                data-weiqi-score
                onClick={() => void send("confirm_score")}
              >
                确认计分
              </button>
              <button
                type="button"
                disabled={!canScore}
                data-weiqi-resume
                onClick={() => void send("resume")}
              >
                继续对弈
              </button>
            </div>
          ) : data.phase === "playing" ? (
            <>
              <h3>
                {room.spectating
                  ? "正在观战"
                  : canPlay
                    ? "轮到你行棋"
                    : "等待对方行棋"}
              </h3>
              <p role="status">
                {selected === null
                  ? "选择空点，或用方向键移动后按回车选点。"
                  : `待落点：${label(selected)}，尚未提交。`}
              </p>
              <button
                type="button"
                className="primary"
                data-weiqi-confirm
                disabled={!canPlay || selected === null}
                onClick={() => {
                  if (selected !== null) void send("place", { pos: selected });
                }}
              >
                确认落子{selected === null ? "" : ` · ${label(selected)}`}
              </button>
              {selected !== null && (
                <button type="button" onClick={() => setSelected(null)}>
                  取消选点
                </button>
              )}
              <button
                type="button"
                disabled={!canPlay}
                data-weiqi-pass
                onClick={() => void send("pass")}
              >
                停一手
              </button>
              {data.passes === 1 && <p>对方已停着。再停一手将进入计分确认。</p>}
            </>
          ) : (
            <p>
              {data.reason === "resign" ? "认输结束。" : "双方已确认计分。"}
              {data.winner < 0
                ? "本局和棋。"
                : `${data.winner === data.black ? "黑方" : "白方"}获胜。`}
            </p>
          )}
          {data.phase !== "finished" && (
            <button
              type="button"
              disabled={!data.canResign || !enabled}
              data-weiqi-resign
              onClick={() => setResign(true)}
            >
              认输
            </button>
          )}
          <p>提子数不额外加分。真人和局 0 分、不计胜场。</p>
        </aside>
      </div>
      {resign && (
        <Modal title="确认认输" onClose={() => setResign(false)}>
          <p>认输后本局结束，对方获胜。</p>
          <div className="weiqi-secondary">
            <button type="button" onClick={() => setResign(false)}>
              继续下棋
            </button>
            <button
              type="button"
              disabled={!data.canResign || !enabled}
              onClick={() => void send("resign")}
            >
              确认认输
            </button>
          </div>
        </Modal>
      )}
    </section>
  );
}
