import { useEffect, useRef, useState } from "react";
import type { BoardProps } from "../../../web/src/game-api/module";
import { Modal } from "../../../web/src/ui/board-host";
import { useMapControls } from "../../../web/src/ui/map-controls";
import type { View } from "./types";

const label = (index: number) =>
  `${String.fromCharCode(65 + (index % 15))}${15 - Math.floor(index / 15)}`;
const local = (path: string) => `/gomoku/v1/${path}`;
const source = (base: string, path: string) =>
  `${base.replace(/\/$/, "")}${local(path)}`;
function Stone({ color, assets }: { color: number; assets: string }) {
  const [failed, setFailed] = useState(false);
  const path = `images/stone-${color === 1 ? "black" : "white"}.webp`;
  useEffect(() => setFailed(false), [assets, path]);
  return failed ? (
    <span className={`gomoku-stone-fallback colour-${color}`} />
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
  const controls = useMapControls({ aspect: 1, minMobileWidth: 480 });
  const [selected, setSelected] = useState<number | null>(null),
    [focus, setFocus] = useState(112),
    [resign, setResign] = useState(false),
    [arriving, setArriving] = useState(-1);
  const [windowRect, setWindowRect] = useState({ x: 0, y: 0, w: 480, h: 480 });
  const cells = useRef<(HTMLButtonElement | null)[]>([]);
  const scope = `${room.id}:${data.round}:${room.you}:${clock.epoch ?? 0}`;
  const seen = useRef({ scope, id: data.last.id, phase: data.phase });
  const canPlace =
    data.canPlace &&
    !busy &&
    !room.spectating &&
    !room.seats[room.you]?.autoPlay;
  useEffect(() => {
    setSelected(null);
    setResign(false);
  }, [scope, data.ply, data.phase]);
  useEffect(() => {
    if (!canPlace) setSelected(null);
  }, [canPlace]);
  useEffect(() => {
    const before = seen.current;
    seen.current = { scope, id: data.last.id, phase: data.phase };
    if (before.scope !== scope) {
      setArriving(-1);
      audio.stop();
      return;
    }
    if (data.last.id > before.id) {
      const reduced = matchMedia("(prefers-reduced-motion: reduce)").matches;
      if (!reduced) setArriving(data.last.y * 15 + data.last.x);
      const cue =
        data.phase === "finished"
          ? data.winner < 0
            ? "draw"
            : "win"
          : "place";
      void audio.playWithFallback([
        source(assets, `audio/${cue}.wav`),
        local(`audio/${cue}.wav`),
      ]);
      const timer = setTimeout(() => setArriving(-1), 180);
      return () => clearTimeout(timer);
    }
    if (data.phase === "finished" && before.phase !== "finished") {
      const cue = data.winner < 0 ? "draw" : "win";
      void audio.playWithFallback([
        source(assets, `audio/${cue}.wav`),
        local(`audio/${cue}.wav`),
      ]);
    }
  }, [scope, data.last.id, data.phase, assets, audio]);
  useEffect(() => () => audio.stop(), [audio]);
  const send = (type: string, extra: Record<string, unknown> = {}) =>
    act({ type, round: data.round, ply: data.ply, ...extra });
  const trackView = () => {
    const e = controls.viewport.current;
    if (e)
      setWindowRect({
        x: e.scrollLeft - controls.mapStyle.left,
        y: e.scrollTop - controls.mapStyle.top,
        w: e.clientWidth,
        h: e.clientHeight,
      });
  };
  useEffect(trackView, [controls.mapStyle.width, controls.mapStyle.height]);
  const reset = () => {
    controls.zoomAt(1);
    requestAnimationFrame(() => {
      const e = controls.viewport.current;
      if (e) {
        e.scrollLeft = (e.scrollWidth - e.clientWidth) / 2;
        e.scrollTop = (e.scrollHeight - e.clientHeight) / 2;
        trackView();
      }
    });
  };
  const locate = (event: React.PointerEvent<SVGSVGElement>) => {
    const e = controls.viewport.current;
    if (!e) return;
    const r = event.currentTarget.getBoundingClientRect(),
      u = Math.max(0, Math.min(1, (event.clientX - r.left) / r.width)),
      v = Math.max(0, Math.min(1, (event.clientY - r.top) / r.height));
    e.scrollLeft =
      controls.mapStyle.left + u * controls.mapStyle.width - e.clientWidth / 2;
    e.scrollTop =
      controls.mapStyle.top + v * controls.mapStyle.height - e.clientHeight / 2;
    trackView();
  };
  const grid = Array.from({ length: 15 }, (_, i) => (
    <path key={i} d={`M30 ${30 + i * 30}H450M${30 + i * 30} 30V450`} />
  ));
  const first = data.line?.[0],
    last = data.line?.at(-1);
  return (
    <section
      className="gomoku-board"
      data-ply={data.ply}
      data-zoom={controls.zoom}
      data-selected={selected ?? ""}
    >
      <div className="gomoku-heading">
        <div>
          <h2>五子棋</h2>
          <p>
            {data.options.rules === "freestyle"
              ? "无禁手 · 连成五子或以上获胜"
              : "无禁手 · 恰好五子获胜，长连不胜"}
          </p>
        </div>
        <strong>
          {data.phase === "finished"
            ? "本局已结束"
            : `第 ${data.ply + 1} 手 · ${data.turn === data.black ? "黑棋" : "白棋"}回合`}
        </strong>
      </div>
      <div className="gomoku-workspace">
        <div className="gomoku-play-area">
          <div className="gomoku-map-tools" role="group" aria-label="棋盘视图">
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
              复位视图
            </button>
            <span>滚轮缩放 · 拖动查看</span>
          </div>
          <div
            className={`gomoku-viewport ${controls.dragging ? "dragging" : ""}`}
            ref={controls.viewport}
            {...controls.handlers}
            onScroll={trackView}
          >
            <div style={controls.canvasStyle}>
              <div
                className="gomoku-map"
                data-controller-group="grid"
                style={controls.mapStyle}
                role="group"
                aria-label="十五路五子棋棋盘"
              >
                <svg
                  viewBox="0 0 480 480"
                  className="gomoku-grid"
                  aria-hidden="true"
                >
                  <rect width="480" height="480" rx="10" fill="#e4cca0" />
                  <image
                    href={source(assets, "images/board.webp")}
                    width="480"
                    height="480"
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
                  <g stroke="#655037" strokeWidth="1.2" fill="none">
                    {grid}
                  </g>
                  {[
                    [3, 3],
                    [11, 3],
                    [7, 7],
                    [3, 11],
                    [11, 11],
                  ].map(([x, y]) => (
                    <circle
                      key={`${x}-${y}`}
                      cx={(x + 1) * 30}
                      cy={(y + 1) * 30}
                      r="3"
                      fill="#655037"
                    />
                  ))}
                  {Array.from({ length: 15 }, (_, i) => (
                    <g key={i} fill="#493c2d" fontSize="10" textAnchor="middle">
                      <text x={(i + 1) * 30} y="16">
                        {String.fromCharCode(65 + i)}
                      </text>
                      <text x="14" y={(i + 1) * 30 + 3}>
                        {15 - i}
                      </text>
                    </g>
                  ))}
                </svg>
                {data.board.map((color, index) => (
                  <button
                    type="button"
                    key={index}
                    ref={(e) => {
                      cells.current[index] = e;
                    }}
                    data-gomoku-cell={index}
                    className="gomoku-cell"
                    style={{
                      left: `${((index % 15) + 1) * 6.25}%`,
                      top: `${(Math.floor(index / 15) + 1) * 6.25}%`,
                    }}
                    aria-label={`${label(index)}，${color === 1 ? "黑棋" : color === 2 ? "白棋" : "空点"}`}
                    aria-disabled={!canPlace || color !== 0}
                    aria-pressed={selected === index}
                    tabIndex={focus === index ? 0 : -1}
                    onFocus={() => setFocus(index)}
                    onClick={() => {
                      if (canPlace && !color) setSelected(index);
                    }}
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
                        const x = Math.max(0, Math.min(14, (index % 15) + dx)),
                          y = Math.max(
                            0,
                            Math.min(14, Math.floor(index / 15) + dy),
                          );
                        const n = y * 15 + x;
                        setFocus(n);
                        cells.current[n]?.focus();
                        cells.current[n]?.scrollIntoView({
                          block: "nearest",
                          inline: "nearest",
                        });
                      }
                    }}
                  >
                    {!!color && (
                      <span
                        className={`gomoku-stone colour-${color} ${arriving === index ? "arriving" : ""} ${data.last.id && data.last.x === index % 15 && data.last.y === Math.floor(index / 15) ? "last" : ""}`}
                      >
                        <Stone color={color} assets={assets} />
                      </span>
                    )}
                    {!color && selected === index && (
                      <span
                        className={`gomoku-preview colour-${data.turn === data.black ? 1 : 2}`}
                      />
                    )}
                  </button>
                ))}
                {first !== undefined && last !== undefined && (
                  <svg
                    className="gomoku-win-line"
                    viewBox="0 0 480 480"
                    aria-hidden="true"
                  >
                    <line
                      x1={((first % 15) + 1) * 30}
                      y1={(Math.floor(first / 15) + 1) * 30}
                      x2={((last % 15) + 1) * 30}
                      y2={(Math.floor(last / 15) + 1) * 30}
                      stroke="#c65437"
                      strokeWidth="4"
                    />
                  </svg>
                )}
              </div>
            </div>
          </div>
        </div>
        <aside className="gomoku-actions">
          <div className="gomoku-minimap-row">
            <div>
              <strong>整盘概览</strong>
              <p>拖动小棋盘定位。放大后选点，确认才落子。</p>
            </div>
            <svg
              className="gomoku-minimap"
              viewBox="0 0 480 480"
              role="img"
              aria-label="拖动整盘概览定位棋盘"
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
              <rect width="480" height="480" fill="#e4cca0" />
              <g stroke="#998360" strokeWidth="1.5" fill="none">
                {grid}
              </g>
              {data.board.map((c, i) =>
                c ? (
                  <circle
                    key={i}
                    cx={((i % 15) + 1) * 30}
                    cy={(Math.floor(i / 15) + 1) * 30}
                    r="11"
                    fill={c === 1 ? "#20322e" : "#fffdf1"}
                    stroke="#45554b"
                  />
                ) : null,
              )}
              <rect
                x={
                  (Math.max(0, windowRect.x) /
                    Math.max(1, controls.mapStyle.width)) *
                  480
                }
                y={
                  (Math.max(0, windowRect.y) /
                    Math.max(1, controls.mapStyle.height)) *
                  480
                }
                width={Math.min(
                  480,
                  (windowRect.w / Math.max(1, controls.mapStyle.width)) * 480,
                )}
                height={Math.min(
                  480,
                  (windowRect.h / Math.max(1, controls.mapStyle.height)) * 480,
                )}
                fill="#2d725315"
                stroke="#285e47"
                strokeWidth="9"
              />
            </svg>
          </div>
          <h3>
            {room.spectating
              ? "正在观战"
              : data.phase === "finished"
                ? "对局已结束"
                : canPlace
                  ? "请选择落点"
                  : "等待对方落子"}
          </h3>
          <p role="status">
            {selected === null
              ? "鼠标点选，或用方向键移动后按回车选择。"
              : `待落点：${label(selected)}，尚未提交。`}
          </p>
          <button
            type="button"
            className="primary"
            data-gomoku-confirm
            disabled={!canPlace || selected === null}
            onClick={() => {
              if (selected !== null)
                void send("place", {
                  x: selected % 15,
                  y: Math.floor(selected / 15),
                });
            }}
          >
            确认落子{selected === null ? "" : ` · ${label(selected)}`}
          </button>
          {selected !== null && (
            <button
              type="button"
              data-controller-cancel
              onClick={() => setSelected(null)}
            >
              取消选点
            </button>
          )}
          {data.canRespond && !room.spectating && (
            <div className="gomoku-draw-offer">
              <strong>对方提议和棋</strong>
              <p>双方同意后结束，积分不变。</p>
              <button
                type="button"
                disabled={busy}
                onClick={() => void send("respond_draw", { accept: true })}
              >
                同意和棋
              </button>
              <button
                type="button"
                disabled={busy}
                onClick={() => void send("respond_draw", { accept: false })}
              >
                继续对弈
              </button>
            </div>
          )}
          {data.offer === room.you && data.phase === "playing" && (
            <p>已提议和棋。可以继续落子，计时不会暂停。</p>
          )}
          {!room.spectating && data.phase === "playing" && (
            <div className="gomoku-secondary">
              <button
                type="button"
                disabled={
                  busy || !data.canOffer || room.seats[room.you]?.autoPlay
                }
                onClick={() => void send("offer_draw")}
              >
                提议和棋
              </button>
              <button
                type="button"
                disabled={
                  busy || !data.canResign || room.seats[room.you]?.autoPlay
                }
                onClick={() => setResign(true)}
              >
                认输
              </button>
            </div>
          )}
        </aside>
      </div>
      {resign && (
        <Modal title="确认认输" onClose={() => setResign(false)}>
          <p>认输后本局结束，对方获胜。</p>
          <button
            type="button"
            className="primary"
            disabled={busy}
            onClick={() => {
              setResign(false);
              void send("resign");
            }}
          >
            确认认输
          </button>
        </Modal>
      )}
    </section>
  );
}
