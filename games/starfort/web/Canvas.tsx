import { useEffect, useRef, useState } from "react";
import {
  assetURL,
  enemyPoint,
  freshEvents,
  spritePlacement,
  battlefieldTopInset,
} from "./presentation";
import type { EventCursor } from "./presentation";
import type { Enemy, Event, View } from "./types";
import atlasLayout from "../assets/public/starfort/v1/atlas.json";

type Atlas = {
  frames: Record<string, { x: number; y: number; w: number; h: number }>;
  meta: { image: string; width: number; height: number };
};
type Art = {
  atlas: Atlas | null;
  sheet: HTMLImageElement | null;
  map: HTMLImageElement | null;
};
type Effect = Event & { born: number };
const colors = ["#b34f2d", "#386da7", "#8a5196", "#4f7641"];
const towerNames: Record<string, string> = {
  arrow: "箭塔",
  cannon: "炮塔",
  frost: "冰塔",
  arcane: "法师塔",
};
const towerColors: Record<string, string> = {
  arrow: "#a07a39",
  cannon: "#925437",
  frost: "#6cb9d1",
  arcane: "#9a76c2",
};
function loadImage(
  src: string,
  fallback: string,
  signal: AbortSignal,
): Promise<HTMLImageElement> {
  return new Promise((resolve, reject) => {
    const image = new Image();
    const abort = () => {
      image.src = "";
      reject(new Error("aborted"));
    };
    signal.addEventListener("abort", abort, { once: true });
    const clean = () => signal.removeEventListener("abort", abort);
    image.onload = () => {
      clean();
      resolve(image);
    };
    image.onerror = () => {
      if (signal.aborted) {
        clean();
        return;
      }
      if (image.getAttribute("src") !== fallback) {
        image.src = fallback;
        return;
      }
      clean();
      reject(new Error("image unavailable"));
    };
    image.src = src;
  });
}
function sprite(
  ctx: CanvasRenderingContext2D,
  art: Art,
  name: string,
  x: number,
  y: number,
  width: number,
  height: number,
  anchor = "bottom",
): boolean {
  const frame = art.atlas?.frames[name];
  if (!art.sheet || !frame) return false;
  ctx.drawImage(
    art.sheet,
    frame.x,
    frame.y,
    frame.w,
    frame.h,
    x - width / 2,
    y - (anchor === "center" ? height / 2 : height),
    width,
    height,
  );
  return true;
}
function circle(
  ctx: CanvasRenderingContext2D,
  x: number,
  y: number,
  r: number,
  fill: string,
  stroke?: string,
) {
  ctx.beginPath();
  ctx.arc(x, y, r, 0, Math.PI * 2);
  ctx.fillStyle = fill;
  ctx.fill();
  if (stroke) {
    ctx.strokeStyle = stroke;
    ctx.lineWidth = 2;
    ctx.stroke();
  }
}
function linePath(ctx: CanvasRenderingContext2D, path: View["map"]["path"]) {
  ctx.beginPath();
  path.forEach((p, i) => (i ? ctx.lineTo(p.x, p.y) : ctx.moveTo(p.x, p.y)));
}
function fallbackTower(
  ctx: CanvasRenderingContext2D,
  x: number,
  y: number,
  size: number,
  kind: string,
) {
  ctx.fillStyle = "#766653";
  ctx.fillRect(x - size * 0.35, y - size * 0.34, size * 0.7, size * 0.35);
  ctx.fillStyle = towerColors[kind];
  ctx.fillRect(x - size * 0.25, y - size * 0.8, size * 0.5, size * 0.6);
  ctx.beginPath();
  ctx.moveTo(x - size * 0.4, y - size * 0.7);
  ctx.lineTo(x, y - size);
  ctx.lineTo(x + size * 0.4, y - size * 0.7);
  ctx.closePath();
  ctx.fill();
}
function fallbackEnemy(
  ctx: CanvasRenderingContext2D,
  x: number,
  y: number,
  size: number,
  kind: string,
) {
  circle(
    ctx,
    x,
    y - size * 0.4,
    size * 0.4,
    kind === "runner" ? "#bd783f" : kind === "boss" ? "#9d628b" : "#91b968",
    "#345444",
  );
  circle(ctx, x - size * 0.12, y - size * 0.45, size * 0.045, "#18332d");
  circle(ctx, x + size * 0.12, y - size * 0.45, size * 0.045, "#18332d");
}

export function Battlefield({
  roomID,
  data,
  assets,
  selected,
  select,
}: {
  roomID: string;
  data: View;
  assets: string;
  selected: number | null;
  select: (pad: number) => void;
}) {
  const canvas = useRef<HTMLCanvasElement>(null),
    container = useRef<HTMLDivElement>(null);
  const art = useRef<Art>({ atlas: null, sheet: null, map: null });
  const state = useRef({
    data,
    previous: new Map<number, Enemy>(),
    at: performance.now(),
    duration: 1000,
  });
  const selectedRef = useRef(selected);
  selectedRef.current = selected;
  const cursor = useRef<EventCursor | null>(null),
    effects = useRef<Effect[]>([]);
  const [topInset, setTopInset] = useState(0);
  const camera = useRef({ topInset: 0 });
  const [loaded, setLoaded] = useState(false),
    [assetError, setAssetError] = useState(false);
  const [reduced, setReduced] = useState(
    () => window.matchMedia("(prefers-reduced-motion: reduce)").matches,
  );
  useEffect(() => {
    const query = window.matchMedia("(prefers-reduced-motion: reduce)");
    const change = () => setReduced(query.matches);
    query.addEventListener("change", change);
    return () => query.removeEventListener("change", change);
  }, []);
  useEffect(() => {
    const controller = new AbortController();
    setLoaded(false);
    setAssetError(false);
    art.current = { atlas: null, sheet: null, map: null };
    const load = async () => {
      // Geometry ships with this build; only artwork is served by the CDN.
      const atlas: Atlas = atlasLayout;
      const [sheet, map] = await Promise.all([
        loadImage(
          assetURL(assets, "atlas.webp"),
          assetURL("", "atlas.webp"),
          controller.signal,
        ),
        loadImage(
          assetURL(assets, "map.webp"),
          assetURL("", "map.webp"),
          controller.signal,
        ),
      ]);
      if (!controller.signal.aborted) {
        art.current = { atlas, sheet, map };
        setLoaded(true);
      }
    };
    void load().catch(() => {
      if (!controller.signal.aborted) setAssetError(true);
    });
    return () => controller.abort();
  }, [assets]);
  useEffect(() => {
    const at = performance.now(),
      prior = state.current;
    const contiguous =
      cursor.current?.room === roomID &&
      data.tick >= prior.data.tick &&
      at - prior.at < 4000;
    // Operations between simulation snapshots must not restart enemy motion.
    if (data.tick !== prior.data.tick || !contiguous) {
      state.current = {
        data,
        previous: contiguous
          ? new Map(prior.data.enemies.map((e) => [e.id, e]))
          : new Map(),
        at,
        duration: Math.min(
          1000,
          Math.max(200, (data.tick - prior.data.tick) * 200),
        ),
      };
    } else state.current.data = data;
    const [next, batch] = freshEvents(cursor.current, roomID, data, at);
    cursor.current = next;
    if (!contiguous || document.hidden || data.phase === "paused")
      effects.current = [];
    if (!document.hidden && data.phase !== "paused")
      effects.current.push(...batch.map((e) => ({ ...e, born: at })));
    effects.current = effects.current.slice(-60);
  }, [data, roomID]);
  useEffect(() => {
    const node = canvas.current,
      wrap = container.current;
    if (!node || !wrap) return;
    const ctx = node.getContext("2d");
    if (!ctx) return;
    let frame = 0,
      lastPaint = 0;
    const resize = () => {
      const width = wrap.clientWidth,
        dpr = Math.min(2, window.devicePixelRatio || 1);
      const inset = battlefieldTopInset(
        width,
        data.map.width,
        Math.min(...data.map.pads.map((p) => p.y)),
      );
      camera.current.topInset = inset;
      setTopInset(inset);
      node.width = Math.round(width * dpr);
      node.height = Math.round(
        ((width * (data.map.height + inset)) / data.map.width) * dpr,
      );
    };
    resize();
    const observer = new ResizeObserver(resize);
    observer.observe(wrap);
    const draw = (at: number) => {
      frame = requestAnimationFrame(draw);
      if (document.hidden || (reduced && at - lastPaint < 150)) return;
      lastPaint = at;
      if (!wrap.clientWidth || !node.width) return;
      const current = state.current,
        v = current.data,
        map = v.map,
        scale = node.width / map.width,
        pixelScale = wrap.clientWidth / map.width;
      const inset = camera.current.topInset;
      ctx.setTransform(scale, 0, 0, scale, 0, inset * scale);
      ctx.clearRect(0, -inset, map.width, map.height + inset);
      ctx.fillStyle = "#b9c997";
      ctx.fillRect(0, -inset, map.width, inset);
      if (art.current.map)
        ctx.drawImage(
          art.current.map,
          0,
          -inset,
          map.width,
          map.height + inset,
        );
      else {
        ctx.fillStyle = "#b9c997";
        ctx.fillRect(0, -inset, map.width, map.height + inset);
      }
      ctx.lineJoin = "round";
      ctx.lineCap = "round";
      linePath(ctx, map.path);
      ctx.lineWidth = 72;
      ctx.strokeStyle = "#82966e";
      ctx.stroke();
      linePath(ctx, map.path);
      ctx.lineWidth = 62;
      ctx.strokeStyle = "#d7bc87";
      ctx.stroke();
      linePath(ctx, map.path);
      ctx.lineWidth = 46;
      ctx.strokeStyle = "#e5cda0";
      ctx.stroke();
      // Small stones guide the eye from entry to the shared star core.
      ctx.save();
      ctx.setLineDash([1, 38]);
      linePath(ctx, map.path);
      ctx.lineWidth = 3;
      ctx.strokeStyle = "#b29a73";
      ctx.stroke();
      ctx.restore();
      const sel = map.pads.find((p) => p.id === selectedRef.current),
        tower = v.towers.find((t) => t.pad === selectedRef.current);
      if (sel && tower) {
        ctx.save();
        circle(
          ctx,
          sel.x,
          sel.y,
          tower.effectiveRange ??
            v.catalog.find((c) => c.kind === tower.kind)?.range ??
            120,
          "#f5edc024",
        );
        ctx.setLineDash([8, 8]);
        ctx.strokeStyle = "#625536";
        ctx.lineWidth = 2;
        ctx.stroke();
        ctx.restore();
      }
      for (const pad of map.pads) {
        const occupied = v.towers.find((t) => t.pad === pad.id);
        ctx.save();
        ctx.translate(pad.x, pad.y);
        ctx.scale(1, 0.45);
        circle(ctx, 0, 0, 34, occupied ? "#7b8864" : "#dfe6bd", "#788268");
        if (selectedRef.current === pad.id) {
          ctx.strokeStyle = "#fff5b0";
          ctx.lineWidth = 6;
          ctx.stroke();
        }
        ctx.restore();
      }
      const towerSize = Math.max(66, 42 / pixelScale);
      for (const t of v.towers) {
        const pad = map.pads.find((p) => p.id === t.pad);
        if (!pad) continue;
        const towerPoint = spritePlacement(
          { x: pad.x, y: pad.y + 14 },
          towerSize,
          (towerSize * 224) / 192,
          { ...map, top: -inset },
        );
        ctx.save();
        ctx.shadowColor = "#254b2a36";
        ctx.shadowBlur = 8;
        ctx.shadowOffsetY = 5;
        if (
          !sprite(
            ctx,
            art.current,
            `tower-${t.kind}-${Math.min(3, t.level)}`,
            towerPoint.x,
            towerPoint.y,
            towerSize,
            (towerSize * 224) / 192,
          )
        )
          fallbackTower(ctx, towerPoint.x, towerPoint.y, towerSize, t.kind);
        ctx.restore();
        // The aim marker rotates from server angles; it never picks a target itself.
        if (v.phase === "battle") {
          const pivot = { x: towerPoint.x, y: towerPoint.y - towerSize * 0.59 };
          ctx.save();
          ctx.translate(pivot.x, pivot.y);
          ctx.rotate(t.angle);
          ctx.strokeStyle =
            t.kind === "frost"
              ? "#d4f5ff"
              : t.kind === "arcane"
                ? "#f1d7ff"
                : "#ffebaa";
          ctx.lineWidth = 4;
          ctx.beginPath();
          ctx.moveTo(8, 0);
          ctx.lineTo(19, 0);
          ctx.stroke();
          ctx.restore();
        }
        if (t.level > 1) {
          ctx.font = `bold ${Math.max(13, 9 / pixelScale)}px system-ui`;
          ctx.textAlign = "center";
          ctx.lineWidth = 3;
          ctx.strokeStyle = "#263c32";
          ctx.strokeText(
            `★${t.level}`,
            towerPoint.x,
            towerPoint.y - towerSize * 0.93,
          );
          ctx.fillStyle = "#fff2b8";
          ctx.fillText(
            `★${t.level}`,
            towerPoint.x,
            towerPoint.y - towerSize * 0.93,
          );
        }
      }
      const endpoint = map.path[map.path.length - 1] ?? {
        x: map.width,
        y: map.height / 2,
      };
      const bx = Math.min(map.width - 55, endpoint.x - 65),
        by = endpoint.y + 35;
      const baseSize = Math.max(105, 38 / pixelScale);
      const damaged = v.baseHP < v.baseMaxHP * 0.5;
      if (
        !sprite(
          ctx,
          art.current,
          `base-${damaged ? "damaged" : "intact"}`,
          bx,
          by,
          baseSize,
          baseSize,
        )
      ) {
        ctx.fillStyle = damaged ? "#b19291" : "#a5bacc";
        ctx.fillRect(bx - 40, by - 76, 80, 76);
        circle(ctx, bx, by - 72, 18, "#f8e7a1");
      }
      if (!reduced && v.phase !== "lost") {
        const pulse = 0.3 + 0.12 * Math.sin(at / 700);
        circle(
          ctx,
          bx,
          by - baseSize * 0.62,
          baseSize * 0.13,
          `rgba(255,239,151,${pulse})`,
        );
      }
      ctx.font = `bold ${Math.max(15, 10 / pixelScale)}px system-ui`;
      ctx.textAlign = "center";
      ctx.strokeStyle = "#fff9e5";
      ctx.lineWidth = 4;
      ctx.strokeText("星核", bx, by + 15);
      ctx.fillStyle = "#3f5143";
      ctx.fillText("星核", bx, by + 15);
      const fraction =
        v.phase === "battle" ? (at - current.at) / current.duration : 1;
      for (const enemy of v.enemies) {
        const position = enemyPoint(
          map.path,
          current.previous.get(enemy.id),
          enemy,
          fraction,
          reduced,
        );
        const frozen = enemy.slowUntil > v.tick;
        const size = Math.max(
          enemy.kind === "boss" ? 64 : 42,
          (enemy.kind === "boss" ? 40 : 32) / pixelScale,
        );
        const moving = !reduced && v.phase === "battle";
        const pose = moving ? Math.floor(at / 160 + enemy.id) % 3 : 0;
        const bob = moving
          ? Math.sin(at / (enemy.kind === "runner" ? 90 : 140) + enemy.id) * 1.5
          : 0;
        const x = position.x,
          y = position.y + bob;
        ctx.save();
        ctx.translate(x, y);
        ctx.scale(1, 0.32);
        circle(ctx, 0, 3, size * 0.3, "#43593750");
        ctx.restore();
        if (frozen) {
          ctx.save();
          ctx.translate(x, y);
          ctx.scale(1, 0.38);
          circle(ctx, 0, 0, size * 0.5, "#bce9ff60", "#3e809e");
          ctx.restore();
        }
        if (
          !sprite(
            ctx,
            art.current,
            `enemy-${enemy.kind}-${pose}`,
            x,
            y + size * 0.2,
            size,
            size,
          )
        )
          fallbackEnemy(ctx, x, y + size * 0.15, size, enemy.kind);
        const barWidth = size * 0.66,
          barHeight = Math.max(4, 2.5 / pixelScale),
          barY = y - size * 0.75;
        ctx.fillStyle = "#283b34";
        ctx.fillRect(
          x - barWidth / 2 - 1,
          barY - 1,
          barWidth + 2,
          barHeight + 2,
        );
        ctx.fillStyle = "#add584";
        ctx.fillRect(
          x - barWidth / 2,
          barY,
          barWidth * Math.max(0, enemy.hp / Math.max(1, enemy.maxHP)),
          barHeight,
        );
        if (enemy.shield > 0) {
          ctx.fillStyle = "#cef0ff";
          ctx.fillRect(
            x - barWidth / 2,
            barY - barHeight - 2,
            barWidth * Math.min(1, enemy.shield / Math.max(1, enemy.maxHP)),
            barHeight,
          );
        }
      }
      effects.current = effects.current.filter((e) => at - e.born < 900);
      for (const e of effects.current) {
        const elapsed = at - e.born,
          t = elapsed / 900;
        if (reduced) {
          if (elapsed < 250 && (e.type === "base_hit" || e.type === "build"))
            circle(ctx, e.x, e.y, 16, "#fff4c880");
          continue;
        }
        ctx.save();
        if (e.type === "shot" && elapsed < 520) {
          const p = Math.min(1, elapsed / 420),
            tx = e.toX ?? e.x,
            ty = e.toY ?? e.y;
          const x = e.x + (tx - e.x) * p,
            y =
              e.y +
              (ty - e.y) * p -
              (e.kind === "cannon" ? Math.sin(p * Math.PI) * 45 : 0);
          ctx.translate(x, y);
          ctx.rotate(Math.atan2(ty - e.y, tx - e.x));
          if (
            !sprite(
              ctx,
              art.current,
              `projectile-${e.kind}`,
              0,
              0,
              24,
              24,
              "center",
            )
          )
            circle(ctx, 0, 0, 5, towerColors[e.kind ?? "arrow"] ?? "#f7e7a0");
        } else if (["hit", "death", "base_hit"].includes(e.type)) {
          ctx.globalAlpha = 1 - t;
          const name =
            e.type === "base_hit"
              ? "burst"
              : e.kind === "frost"
                ? "frost"
                : e.kind === "arcane"
                  ? "arcane"
                  : "burst";
          const size = (e.type === "death" ? 38 : 24) + t * 26;
          if (
            !sprite(
              ctx,
              art.current,
              `effect-${name}`,
              e.x,
              e.y,
              size,
              size,
              "center",
            )
          )
            circle(ctx, e.x, e.y, 8 + t * 15, "#ffefa870");
        } else if (["build", "upgrade", "sell", "gift"].includes(e.type)) {
          ctx.globalAlpha = 1 - t;
          ctx.lineWidth = 4;
          ctx.strokeStyle = e.type === "sell" ? "#fff0b0" : "#f6ffdf";
          ctx.beginPath();
          ctx.ellipse(e.x, e.y, 18 + t * 42, 8 + t * 18, 0, 0, Math.PI * 2);
          ctx.stroke();
          if (e.type === "upgrade") {
            ctx.font = "bold 22px system-ui";
            ctx.textAlign = "center";
            ctx.fillStyle = "#fff8c8";
            ctx.fillText("✦", e.x, e.y - 25 - t * 30);
          }
        }
        ctx.restore();
      }
    };
    frame = requestAnimationFrame(draw);
    return () => {
      cancelAnimationFrame(frame);
      observer.disconnect();
    };
  }, [data.map.width, data.map.height, reduced]);
  return (
    <div
      className="sf-battlefield"
      ref={container}
      data-assets-ready={loaded}
      style={{
        aspectRatio: `${data.map.width} / ${data.map.height + topInset}`,
      }}
    >
      <canvas
        ref={canvas}
        data-starfort-canvas
        data-enemies={data.enemies.length}
        role="img"
        aria-label={`星垒战场，第 ${data.wave} 波，${data.enemies.length} 名敌人，${data.towers.length} 座防御塔。下方建造点按钮可用键盘选择。`}
      />
      {data.map.pads.map((pad) => {
        const tower = data.towers.find((t) => t.pad === pad.id);
        return (
          <button
            key={pad.id}
            data-sf-pad={pad.id}
            data-pad={pad.id}
            className={`sf-pad ${tower ? "sf-pad-occupied" : ""} ${selected === pad.id ? "sf-pad-selected" : ""}`}
            style={{
              left: `${(pad.x / data.map.width) * 100}%`,
              top: `${((pad.y + topInset) / (data.map.height + topInset)) * 100}%`,
              ...(tower
                ? { "--sf-owner": colors[tower.owner % colors.length] }
                : {}),
            }}
            aria-label={`建造点 ${pad.id + 1}${tower ? `，${towerNames[tower.kind]} ${tower.level} 级，玩家 ${tower.owner + 1}` : "，空地"}`}
            aria-pressed={selected === pad.id}
            onClick={() => select(pad.id)}
          >
            <span>{pad.id + 1}</span>
          </button>
        );
      })}
      {!loaded && (
        <span className="sf-loading" role="status">
          {assetError ? "画面素材暂未加载，已使用简化图形。" : "正在点亮星垒…"}
        </span>
      )}
      {data.phase === "paused" && (
        <div className="sf-paused-overlay">
          <strong>守望暂停</strong>
          <span>全员同意后继续</span>
        </div>
      )}
    </div>
  );
}
