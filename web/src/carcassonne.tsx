import { useEffect, useRef, useState } from "react";
import { Check, Maximize, Minus, Plus, RotateCw } from "lucide-react";
import type { Act, CarDefinition, CarPlacement, CarTile, Room } from "./types";
import "./carcassonne.css";

export const carNames: Record<string, string> = {
  city: "城市",
  road: "道路",
  field: "田地",
  monastery: "修道院",
};
const colors = ["#337ab3", "#cf5347", "#42956b", "#d4a833", "#414753"];
const meepleArt = [0, 3, 1, 4, 2];
function rotated(x: number, y: number, r: number) {
  for (let i = 0; i < r; i++) {
    [x, y] = [1 - y, x];
  }
  return { x: x * 100, y: y * 100 };
}
function TileArt({
  art,
  definition,
  assets,
}: {
  art: number;
  definition: CarDefinition;
  assets: string;
}) {
  const [failed, setFailed] = useState(false);
  return assets && !failed ? (
    <image
      href={`${assets}/carcassonne/tile-${art}-v1.webp`}
      width="100"
      height="100"
      onError={() => setFailed(true)}
    />
  ) : (
    <>
      <rect width="100" height="100" fill="#9ab853" />
      {definition.features.map((f, i) =>
        f.kind === "city" ? (
          <g key={i}>
            <path
              d={(f.ports || [])
                .map((p) => {
                  const points = [
                    "0,0 100,0",
                    "100,0 100,100",
                    "100,100 0,100",
                    "0,100 0,0",
                  ];
                  return `M${points[Math.floor(p / 3)]} L${f.x * 100},${f.y * 100}Z`;
                })
                .join(" ")}
              fill="#be8966"
              stroke="#dfd7bb"
              strokeWidth="3"
            />
            {!!f.shields && (
              <text x={f.x * 100} y={f.y * 100} fill="#fff">
                ◆
              </text>
            )}
          </g>
        ) : f.kind === "road" ? (
          <path
            key={i}
            d={(f.ports || [])
              .map((p) => {
                const end = [
                  [50, 0],
                  [100, 50],
                  [50, 100],
                  [0, 50],
                ][Math.floor(p / 3)];
                return `M${end}L${f.x * 100},${f.y * 100}`;
              })
              .join(" ")}
            fill="none"
            stroke="#fbecd0"
            strokeWidth="8"
          />
        ) : f.kind === "monastery" ? (
          <g key={i}>
            <rect x="35" y="40" width="30" height="25" fill="#e5d9bb" />
            <path d="M30 40L50 20L70 40Z" fill="#a6453d" />
          </g>
        ) : null,
      )}
    </>
  );
}
function Meeple({
  player,
  assets,
  farmer = false,
}: {
  player: number;
  assets: string;
  farmer?: boolean;
}) {
  const [failed, setFailed] = useState(false);
  return (
    <g transform={farmer ? "rotate(90)" : ""}>
      {assets && !failed ? (
        <image
          href={`${assets}/carcassonne/meeple-${meepleArt[player]}-v1.webp`}
          x="-15"
          y="-15"
          width="30"
          height="30"
          onError={() => setFailed(true)}
        />
      ) : (
        <path
          d="M-5-13Q0-19 5-13L5-7L14 0L10 5L5 2L10 14H2L0 7L-2 14H-10L-5 2L-10 5L-14 0L-5-7Z"
          fill={colors[player]}
          stroke="#fff"
          strokeWidth="1.5"
        />
      )}
    </g>
  );
}
export function CarcassonneBoard({
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
  const state = room.game!,
    g = state.carcassonne!;
  const mine =
    !room.spectating &&
    room.you === state.turn &&
    !state.finished &&
    !g.players[room.you]?.eliminated;
  const tilePhase = state.phase === "car_tile";
  const [rotation, setRotation] = useState(0),
    [picked, setPicked] = useState<CarPlacement | null>(null),
    [feature, setFeature] = useState<number | null>(null);
  const [view, setView] = useState({ x: 50, y: 50, z: 1 }),
    [size, setSize] = useState({ w: 800, h: 560 });
  const viewport = useRef<HTMLDivElement>(null),
    svg = useRef<SVGSVGElement>(null),
    drag = useRef<{
      x: number;
      y: number;
      vx: number;
      vy: number;
      moved: boolean;
    } | null>(null),
    suppressClick = useRef(false);
  const latest = useRef(view);
  latest.current = view;
  const [dragging, setDragging] = useState(false);
  const last = g.tiles[g.last];
  const currentKind = g.catalog.findIndex((d) => d.arts.includes(g.current));
  const fit = () => {
    const xs = g.tiles.map((t) => t.x),
      ys = g.tiles.map((t) => t.y);
    const left = Math.min(...xs),
      right = Math.max(...xs),
      top = Math.min(...ys),
      bottom = Math.max(...ys);
    setView({
      x: (left + right + 1) * 50,
      y: (top + bottom + 1) * 50,
      z: Math.max(
        0.03,
        Math.min(
          1.3,
          size.w / ((right - left + 3) * 100),
          size.h / ((bottom - top + 3) * 100),
        ),
      ),
    });
  };
  useEffect(() => {
    const el = viewport.current;
    if (!el) return;
    const observer = new ResizeObserver(([e]) =>
      setSize({ w: e.contentRect.width, h: e.contentRect.height }),
    );
    observer.observe(el);
    return () => observer.disconnect();
  }, []);
  useEffect(() => {
    setPicked(null);
    setFeature(null);
    setRotation(0);
    if (mine && state.phase === "car_meeple" && last)
      setView((v) => ({
        ...v,
        x: (last.x + 0.5) * 100,
        y: (last.y + 0.5) * 100,
        z: Math.max(v.z, 1.15),
      }));
  }, [g.current, g.last, state.phase, state.turn]);
  const initiallyFit = useRef(false);
  useEffect(() => {
    if (size.w !== 800 && !initiallyFit.current) {
      initiallyFit.current = true;
      fit();
    }
  }, [size.w]);
  useEffect(() => {
    const el = viewport.current;
    if (!el) return;
    const wheel = (e: WheelEvent) => {
      e.preventDefault();
      const b = el.getBoundingClientRect(),
        v = latest.current;
      const z = Math.max(
        0.03,
        Math.min(2.4, v.z * Math.exp(-e.deltaY * 0.0015)),
      );
      const dx = e.clientX - b.left - b.width / 2,
        dy = e.clientY - b.top - b.height / 2;
      setView({ x: v.x + dx / v.z - dx / z, y: v.y + dy / v.z - dy / z, z });
    };
    el.addEventListener("wheel", wheel, { passive: false });
    return () => el.removeEventListener("wheel", wheel);
  }, []);
  const previewLegal =
    !!picked &&
    g.legal.some(
      (p) => p.x === picked.x && p.y === picked.y && p.rotation === rotation,
    );
  const cells = [...new Map(g.legal.map((p) => [`${p.x},${p.y}`, p])).values()];
  const choose = (p: CarPlacement) => {
    if (!mine || busy || suppressClick.current) return;
    setPicked(p);
    if (
      !g.legal.some(
        (l) => l.x === p.x && l.y === p.y && l.rotation === rotation,
      )
    )
      setRotation(p.rotation);
  };
  const submit = async (a: Record<string, unknown>) => {
    await act(a);
  };
  const displayTile =
    g.current >= 0 ? { art: g.current, kind: currentKind, rotation } : last;
  const label = state.finished
    ? "本局已结算"
    : mine
      ? tilePhase
        ? "选择地块位置"
        : "派遣随从，或跳过"
      : "等待其他玩家";
  const placeCell = (p: CarPlacement) => `${p.x},${p.y}`;
  const tile = (t: CarTile, i: number) => {
    const d = g.catalog[t.kind],
      f = t.feature >= 0 ? d.features[t.feature] : null,
      pos = f ? rotated(f.x, f.y, t.rotation) : null;
    return (
      <g
        key={i}
        transform={`translate(${t.x * 100} ${t.y * 100})`}
        className={i === g.last ? "car-new-tile" : ""}
      >
        <g transform={`rotate(${t.rotation * 90} 50 50)`}>
          <TileArt art={t.art} definition={d} assets={assets} />
        </g>
        <rect
          width="100"
          height="100"
          fill="none"
          stroke={i === g.last ? "#efd26c" : "#433c2560"}
          strokeWidth={i === g.last ? 2 : 1}
        />
        {pos && t.owner >= 0 && (
          <g transform={`translate(${pos.x} ${pos.y})`}>
            <title>
              {room.seats[t.owner].name} · {carNames[f!.kind]}
            </title>
            <Meeple
              player={t.owner}
              assets={assets}
              farmer={f!.kind === "field"}
            />
          </g>
        )}
        {i === g.last &&
          mine &&
          !tilePhase &&
          g.meepleChoices.map((fi) => {
            const mf = d.features[fi],
              p = rotated(mf.x, mf.y, t.rotation);
            return (
              <g
                key={fi}
                role="button"
                tabIndex={busy ? -1 : 0}
                aria-label={`派遣至${carNames[mf.kind]} ${fi + 1}`}
                className={`car-feature-target ${feature === fi ? "selected" : ""}`}
                transform={`translate(${p.x} ${p.y})`}
                onClick={() => {
                  if (!busy && !suppressClick.current) setFeature(fi);
                }}
                onKeyDown={(e) => {
                  if (!busy && (e.key === "Enter" || e.key === " ")) {
                    e.preventDefault();
                    setFeature(fi);
                  }
                }}
              >
                <circle r="14" />
                <text textAnchor="middle" dy="5">
                  {fi + 1}
                </text>
              </g>
            );
          })}
      </g>
    );
  };
  return (
    <div className="car-board">
      <section className="car-map-panel">
        <header className="car-map-tools">
          <span>
            <b>卡卡颂</b>
            <small>{g.tiles.length} / 72 块</small>
          </span>
          <div>
            <button
              aria-label="缩小地图"
              onClick={() =>
                setView((v) => ({ ...v, z: Math.max(0.03, v.z / 1.25) }))
              }
            >
              <Minus size={16} />
            </button>
            <button
              aria-label="放大地图"
              onClick={() =>
                setView((v) => ({ ...v, z: Math.min(2.4, v.z * 1.25) }))
              }
            >
              <Plus size={16} />
            </button>
            <button onClick={fit}>
              <Maximize size={16} />
              全图
            </button>
            {last && (
              <button
                onClick={() =>
                  setView((v) => ({
                    ...v,
                    x: (last.x + 0.5) * 100,
                    y: (last.y + 0.5) * 100,
                    z: Math.max(0.8, v.z),
                  }))
                }
              >
                最新
              </button>
            )}
          </div>
        </header>
        <div
          ref={viewport}
          className={`car-viewport ${dragging ? "dragging" : ""}`}
        >
          <svg
            ref={svg}
            role="group"
            aria-label="卡卡颂地图，可拖动和缩放"
            viewBox={`${view.x - size.w / (2 * view.z)} ${view.y - size.h / (2 * view.z)} ${size.w / view.z} ${size.h / view.z}`}
            onPointerDown={(e) => {
              if (e.button !== 0) return;
              suppressClick.current = false;
              drag.current = {
                x: e.clientX,
                y: e.clientY,
                vx: view.x,
                vy: view.y,
                moved: false,
              };
            }}
            onPointerMove={(e) => {
              const d = drag.current;
              if (!d) return;
              const dx = e.clientX - d.x,
                dy = e.clientY - d.y;
              if (!d.moved && Math.hypot(dx, dy) < 5) return;
              if (!d.moved) {
                e.currentTarget.setPointerCapture(e.pointerId);
                d.moved = true;
                setDragging(true);
              }
              suppressClick.current = true;
              setView((v) => ({
                ...v,
                x: d.vx - dx / v.z,
                y: d.vy - dy / v.z,
              }));
            }}
            onPointerUp={(e) => {
              drag.current = null;
              setDragging(false);
              if (e.currentTarget.hasPointerCapture(e.pointerId))
                e.currentTarget.releasePointerCapture(e.pointerId);
            }}
            onPointerCancel={() => {
              drag.current = null;
              setDragging(false);
            }}
          >
            {mine &&
              tilePhase &&
              cells.map((p) => (
                <g
                  key={placeCell(p)}
                  transform={`translate(${p.x * 100} ${p.y * 100})`}
                  role="button"
                  tabIndex={busy ? -1 : 0}
                  aria-label={`放置位置 ${p.x},${p.y}`}
                  className="car-empty-cell"
                  onClick={() => choose(p)}
                  onKeyDown={(e) => {
                    if (e.key === "Enter" || e.key === " ") {
                      e.preventDefault();
                      choose(p);
                    }
                  }}
                >
                  <rect x="3" y="3" width="94" height="94" rx="5" />
                  <text x="50" y="57" textAnchor="middle">
                    +
                  </text>
                </g>
              ))}
            {g.tiles.map(tile)}
            {picked && tilePhase && mine && currentKind >= 0 && (
              <g
                transform={`translate(${picked.x * 100} ${picked.y * 100})`}
                pointerEvents="none"
                className="car-preview"
              >
                <g transform={`rotate(${rotation * 90} 50 50)`}>
                  <TileArt
                    art={g.current}
                    definition={g.catalog[currentKind]}
                    assets={assets}
                  />
                </g>
                <rect
                  x="2"
                  y="2"
                  width="96"
                  height="96"
                  fill="none"
                  stroke={previewLegal ? "#ffd34b" : "#d74338"}
                  strokeWidth="4"
                />
              </g>
            )}
          </svg>
        </div>
        <footer>拖动地图 · 滚轮缩放 · 点击虚线格预览</footer>
      </section>
      <aside className="car-actions" aria-label="卡卡颂行动">
        <div className="car-draw">
          <svg
            viewBox="0 0 100 100"
            aria-label={tilePhase ? "本回合地块" : "刚放置的地块"}
          >
            {displayTile && displayTile.kind >= 0 && (
              <g transform={`rotate(${displayTile.rotation * 90} 50 50)`}>
                <TileArt
                  art={displayTile.art}
                  definition={g.catalog[displayTile.kind]}
                  assets={assets}
                />
              </g>
            )}
          </svg>
          <div>
            <span className="eyebrow">
              {tilePhase ? "本回合地块" : "刚刚放置"}
            </span>
            <h3>{label}</h3>
            <small>
              牌堆剩余 <b>{g.remaining}</b> 张
            </small>
            {g.discarded.length > 0 && (
              <small>无法放置：{g.discarded.length} 张</small>
            )}
          </div>
        </div>
        {mine && tilePhase && (
          <div className="car-place-actions">
            <button
              className="outline"
              disabled={busy}
              onClick={() => setRotation((r) => (r + 1) % 4)}
            >
              <RotateCw size={17} />
              旋转 {rotation * 90}°
            </button>
            <button
              className="primary"
              disabled={busy || !previewLegal}
              onClick={() =>
                void submit({
                  type: "car_place",
                  x: picked!.x,
                  y: picked!.y,
                  rotation,
                })
              }
            >
              <Check size={17} />
              确认放置
            </button>
            <small>
              {picked
                ? previewLegal
                  ? `位置 (${picked.x}, ${picked.y}) · 确认后不能撤回`
                  : "接边不匹配，请旋转地块"
                : "点击地图上的虚线格，先预览再确认。"}
            </small>
          </div>
        )}
        {mine && !tilePhase && last && (
          <section className="car-deploy">
            <p>
              可用随从 <b>{g.players[room.you].meeples} / 7</b>
            </p>
            <div className="car-feature-list">
              {g.meepleChoices.map((fi) => {
                const f = g.catalog[last.kind].features[fi];
                return (
                  <button
                    key={fi}
                    disabled={busy}
                    className={feature === fi ? "selected" : ""}
                    onClick={() => setFeature(fi)}
                  >
                    <span>{fi + 1}</span>
                    {f.kind === "field" ? "农民 · " : ""}
                    {carNames[f.kind]}
                  </button>
                );
              })}
            </div>
            {feature !== null &&
              g.catalog[last.kind].features[feature].kind === "field" && (
                <p className="car-farmer-note">
                  农民会留在田地直到终局，每座相邻完整城市计 3 分。
                </p>
              )}
            <div className="car-deploy-buttons">
              <button
                className="outline"
                disabled={busy}
                onClick={() => void submit({ type: "car_meeple", feature: -1 })}
              >
                不派遣
              </button>
              <button
                className="primary"
                disabled={busy || feature === null}
                onClick={() => void submit({ type: "car_meeple", feature })}
              >
                确认派遣
              </button>
            </div>
            {g.meepleChoices.length === 0 && (
              <small>没有可用随从或空闲区域，点击“不派遣”继续。</small>
            )}
          </section>
        )}
        <details className="car-scoring">
          <summary>计分速查</summary>
          <dl>
            <dt>道路</dt>
            <dd>每块 1 分</dd>
            <dt>完整城市</dt>
            <dd>每块 2 分，每盾 2 分</dd>
            <dt>未完成城市</dt>
            <dd>终局每块、每盾各 1 分</dd>
            <dt>修道院</dt>
            <dd>自身与八邻格各 1 分</dd>
            <dt>田地</dt>
            <dd>终局每座完整城市 3 分</dd>
          </dl>
          <p>同一区域随从最多者得分；并列各得全部分数。农民仅在终局结算。</p>
        </details>
      </aside>
    </div>
  );
}
