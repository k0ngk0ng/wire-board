import { useEffect, useLayoutEffect, useRef, useState } from "react";
import type { CatanState, Room } from "./types";
import {
  catanColorIndex,
  catanPieceColors,
  catanSeatColor,
} from "./catan-player-colors";
import {
  explorerContents,
  explorerFishContents,
  explorerFishFlight,
  explorerCargoPoint,
  explorerMotionBetween,
  explorerMotionPath,
  explorerPathPoint,
  explorerShipPosition,
} from "./catan-explorer-state";
import type { ExplorerMotion } from "./catan-explorer-state";

export function ExplorerPiece({
  g,
  assets,
  player,
  kind,
  width = 38,
}: {
  g: CatanState;
  assets: string;
  player: number;
  kind: "ship" | "harbor" | "settler" | "crew" | "pirate";
  width?: number;
}) {
  const height =
    width *
    (kind === "crew"
      ? 2
      : kind === "pirate"
        ? 70 / 105
        : kind === "ship"
          ? 61 / 103
          : kind === "harbor"
            ? 78 / 83
            : 66 / 55);
  return assets ? (
    <image
      href={`${assets}/catan/explorer/${kind}-${catanPieceColors[catanColorIndex(g, player)]}-v1.webp`}
      x={-width / 2}
      y={-height / 2}
      width={width}
      height={height}
    />
  ) : (
    <g>
      <circle r={width / 2} fill={catanSeatColor(g, player)} stroke="#453b29" />
      <text y="4" fontSize="11" fill="#171717">
        {
          {
            ship: "船",
            harbor: "港",
            settler: "移民",
            crew: "船员",
            pirate: "海盗",
          }[kind]
        }
      </text>
    </g>
  );
}

export function ExplorerFishPiece({
  assets,
  width = 27,
}: {
  assets: string;
  width?: number;
}) {
  return assets ? (
    <image
      href={`${assets}/catan/explorer/fish-v1.webp`}
      x={-width / 2}
      y={(-width * 61) / 68 / 2}
      width={width}
      height={(width * 61) / 68}
    />
  ) : (
    <text textAnchor="middle" y="4" fontSize="12" fill="#17394b">
      鱼群
    </text>
  );
}

export function ExplorerCargoPieces({
  g,
  assets,
  units,
  arriving = [],
}: {
  g: CatanState;
  assets: string;
  units: number[];
  arriving?: number[];
}) {
  return (
    <>
      {units.map((id, i) => (
        <g
          key={id}
          className={
            arriving.includes(id) ? "explorer-cargo-arriving" : undefined
          }
          transform={`translate(${(i - (units.length - 1) / 2) * 16},0)`}
        >
          <ExplorerPiece
            g={g}
            assets={assets}
            player={Math.floor(id / 11)}
            kind={id % 11 < 2 ? "settler" : "crew"}
            width={id % 11 < 2 ? 18 : 11}
          />
        </g>
      ))}
    </>
  );
}

type Active = {
  event: ExplorerMotion;
  before: CatanState;
  after: CatanState;
  version: number;
};
export function useExplorerMotion(room: Room) {
  const previous = useRef(room);
  const [active, setActive] = useState<Active | null>(null);
  useLayoutEffect(() => {
    const before = previous.current;
    previous.current = room;
    if (before === room) return;
    const event = explorerMotionBetween(before, room);
    if (
      event &&
      !document.hidden &&
      !matchMedia("(prefers-reduced-motion: reduce)").matches
    ) {
      setActive({
        event,
        before: before.game!.catan!,
        after: room.game!.catan!,
        version: room.version,
      });
    } else if (
      before.id !== room.id ||
      before.you !== room.you ||
      !!before.spectating !== !!room.spectating ||
      before.game?.catan?.explorer?.actionId !==
        room.game?.catan?.explorer?.actionId ||
      room.version < before.version ||
      room.version > before.version + 1 ||
      !["playing", "finished"].includes(room.status)
    ) {
      setActive(null);
    }
  }, [room]);
  useEffect(() => {
    if (!active) return;
    const timer = setTimeout(() => setActive(null), 1500);
    return () => clearTimeout(timer);
  }, [active]);
  useEffect(() => {
    const clear = () => setActive(null),
      media = matchMedia("(prefers-reduced-motion: reduce)");
    document.addEventListener("visibilitychange", clear);
    media.addEventListener("change", clear);
    return () => {
      document.removeEventListener("visibilitychange", clear);
      media.removeEventListener("change", clear);
    };
  }, []);
  return active;
}

// Update only SVG attributes per frame; no React render or screen-coordinate math.
export function ExplorerEffects({
  active,
  assets,
}: {
  active: Active | null;
  assets: string;
}) {
  const group = useRef<SVGGElement>(null);
  useLayoutEffect(() => {
    if (!active || !group.current) return;
    const moving = [
      ...group.current.querySelectorAll<SVGGElement>("[data-flight]"),
    ].map((node) => ({
      node,
      points: JSON.parse(node.dataset.flight!) as { x: number; y: number }[],
      duration: Number(node.dataset.duration),
    }));
    const fading = [
      ...group.current.querySelectorAll<SVGElement>(
        "[data-fade], [data-appear]",
      ),
    ];
    let frame = 0;
    const start = performance.now();
    const tick = (now: number) => {
      const elapsed = now - start;
      for (const { node, points, duration } of moving) {
        const p = explorerPathPoint(points, Math.min(1, elapsed / duration));
        node.setAttribute("transform", `translate(${p.x},${p.y})`);
      }
      for (const node of fading) {
        const [delay, duration] = (node.dataset.fade ?? node.dataset.appear!)
          .split(",")
          .map(Number);
        const fraction = Math.max(0, Math.min(1, (elapsed - delay) / duration));
        node.setAttribute(
          "opacity",
          String(
            node.dataset.appear
              ? fraction
              : node.dataset.pulse
                ? Math.sin(fraction * Math.PI)
                : 1 - fraction,
          ),
        );
      }
      if (elapsed < 1450) frame = requestAnimationFrame(tick);
    };
    tick(start);
    return () => cancelAnimationFrame(frame);
  }, [active]);
  if (!active) return null;
  const { event: e, before, after } = active,
    path = explorerMotionPath(before, after, e);
  const sailing = path.length > 1;
  return (
    <g
      ref={group}
      key={e.id}
      pointerEvents="none"
      aria-hidden="true"
      data-explorer-motion={e.id}
    >
      {sailing && (
        <g data-flight={JSON.stringify(path)} data-duration="900">
          <ExplorerPiece
            g={after}
            assets={assets}
            player={e.player}
            kind="ship"
          />
          {explorerFishContents(before, "ship", e.ship).length > 0 && (
            <g transform="translate(0,-12)">
              <ExplorerFishPiece assets={assets} />
            </g>
          )}
          {explorerContents(before, "ship", e.ship).length > 0 && (
            <g transform="translate(0,-12)">
              <ExplorerCargoPieces
                g={after}
                assets={assets}
                units={explorerContents(before, "ship", e.ship)}
              />
            </g>
          )}
          <text y="30" className="explorer-ship-label">
            {(e.ship % 3) + 1}
          </text>
        </g>
      )}
      {e.revealed?.map((id) => {
        const t = after.tiles[id];
        return (
          <polygon
            key={id}
            points={t.vertices
              .map((v) => `${after.vertices[v].x},${after.vertices[v].y}`)
              .join(" ")}
            fill="#dbe8e9"
            stroke="#f7dd83"
            strokeWidth="3"
            data-fade={`${sailing ? 900 : 0},450`}
          />
        );
      })}
      {e.pirate &&
        (() => {
          const p = e.pirate,
            old = before.tiles[p.fromTile],
            next = after.tiles[p.toTile];
          if (!next) return null;
          const same = p.fromOwner >= 0 && p.fromOwner === p.toOwner && old;
          return (
            <>
              {!same && p.fromOwner >= 0 && old && (
                <g transform={`translate(${old.x},${old.y})`} data-fade="0,500">
                  <ExplorerPiece
                    g={before}
                    assets={assets}
                    player={p.fromOwner}
                    kind="pirate"
                    width={44}
                  />
                </g>
              )}
              <g
                data-flight={JSON.stringify(
                  same
                    ? [
                        { x: old.x, y: old.y },
                        { x: next.x, y: next.y },
                      ]
                    : [
                        { x: next.x, y: next.y - 20 },
                        { x: next.x, y: next.y },
                      ],
                )}
                data-duration="650"
                data-fade="650,200"
              >
                <ExplorerPiece
                  g={after}
                  assets={assets}
                  player={p.toOwner}
                  kind="pirate"
                  width={44}
                />
              </g>
            </>
          );
        })()}
      {e.chase &&
        (() => {
          const p = explorerShipPosition(after, e.chase.ship);
          return (
            p && (
              <g
                transform={`translate(${p.x},${p.y - 36})`}
                data-fade="850,450"
                className="explorer-motion-label"
              >
                <rect x="-38" y="-14" width="76" height="24" rx="8" />
                <text y="3">
                  {e.chase.die} · {e.chase.success ? "驱赶成功" : "未成功"}
                </text>
              </g>
            )
          );
        })()}
      {e.lair &&
        (() => {
          const l = e.lair,
            t = after.tiles[l.tile];
          if (!t) return null;
          const text = l.resolved
            ? "金矿已解放"
            : l.ready
              ? "巢穴已攻陷"
              : l.dice?.length
                ? l.dice.filter(Boolean).join(" · ")
                : e.kind === "catan_explorer_land"
                  ? "船员已登陆"
                  : "结算战果";
          return (
            <g transform={`translate(${t.x},${t.y})`} data-fade="750,650">
              <circle
                r="33"
                fill="none"
                stroke="#ffe987"
                strokeWidth="4"
                data-pulse="true"
                data-fade="0,1400"
              />
              <g transform="translate(0,-38)" className="explorer-motion-label">
                <rect x="-48" y="-14" width="96" height="24" rx="8" />
                <text y="3">{text}</text>
              </g>
            </g>
          );
        })()}
      {e.fish?.map((c) => {
        const flight = explorerFishFlight(before, after, e, c.fish);
        if (!flight) return null;
        const end = flight.points[1];
        return (
          <g key={`fish-${c.fish}`} data-fish-motion={c.fish}>
            <g
              data-flight={JSON.stringify(flight.points)}
              data-duration="650"
              data-appear={flight.appear ? "0,250" : undefined}
              data-fade={flight.retire ? "650,350" : undefined}
            >
              <ExplorerFishPiece assets={assets} width={flight.width} />
            </g>
            {flight.delivered && (
              <g
                transform={`translate(${end.x},${end.y - 30})`}
                data-fade="900,450"
                className="explorer-motion-label"
              >
                <rect x="-40" y="-14" width="80" height="24" rx="8" />
                <text y="3">鱼群已交付</text>
              </g>
            )}
          </g>
        );
      })}
      {e.cargo?.map((c) => {
        const from = explorerCargoPoint(before, c.unit, c.from),
          to =
            explorerCargoPoint(after, c.unit, c.to) ??
            (e.kind === "catan_explorer_settle"
              ? after.vertices[e.vertex]
              : null);
        const point = from ?? to;
        if (!point) return null;
        return (
          <g
            key={c.unit}
            data-flight={JSON.stringify([
              point,
              to ?? { x: point.x, y: point.y - 20 },
            ])}
            data-duration="650"
            data-fade="0,900"
            data-pulse={to ? "true" : undefined}
          >
            <ExplorerPiece
              g={after}
              assets={assets}
              player={Math.floor(c.unit / 11)}
              kind={c.unit % 11 < 2 ? "settler" : "crew"}
              width={c.unit % 11 < 2 ? 24 : 12}
            />
          </g>
        );
      })}
      {e.kind === "catan_explorer_settle" &&
        (() => {
          const p = explorerShipPosition(before, e.ship);
          return (
            p && (
              <g transform={`translate(${p.x},${p.y})`} data-fade="0,750">
                <ExplorerPiece
                  g={before}
                  assets={assets}
                  player={e.player}
                  kind="ship"
                />
              </g>
            )
          );
        })()}
      {e.vertex >= 0 && e.kind !== "catan_explorer_transfer" && (
        <circle
          cx={after.vertices[e.vertex].x}
          cy={after.vertices[e.vertex].y}
          r="26"
          fill="none"
          stroke="#ffe387"
          strokeWidth="3"
          data-fade="300,700"
        />
      )}
    </g>
  );
}
