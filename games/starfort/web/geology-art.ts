import { pointOnPath } from "./presentation";
import type { Geology, MapData, Point } from "./types";

export function drawGeology(
  ctx: CanvasRenderingContext2D,
  geology: Geology | undefined,
  map: MapData,
  elapsedMS: number,
  reduced: boolean,
  previous?: Geology,
  fraction = 1,
) {
  if (!geology?.triggered || geology.rewarded) return;
  for (const raw of geology.zones ?? []) {
    let zone = raw;
    const prior =
      previous?.wave === geology.wave
        ? previous.zones?.find((z) => z.id === raw.id && z.kind === raw.kind)
        : undefined;
    if (
      raw.kind === "typhoon" &&
      raw.activated &&
      prior?.activated &&
      raw.progress !== undefined &&
      prior.progress !== undefined
    ) {
      const route = map.routes?.find((r) => r.id === raw.route);
      if (route) {
        const p = pointOnPath(
          route.points,
          prior.progress + (raw.progress - prior.progress) * fraction,
        );
        zone = { ...raw, ...p };
      }
    }
    if (
      elapsedMS < zone.createdMS ||
      zone.skipped ||
      (zone.expiresMS && elapsedMS >= zone.expiresMS) ||
      (zone.kind === "typhoon" && !zone.activated)
    )
      continue;
    ctx.save();
    ctx.lineWidth = 3;
    if (zone.kind === "earthquake") {
      ctx.fillStyle = "#79554888";
      ctx.strokeStyle = "#5b4032";
      ctx.beginPath();
      ctx.arc(zone.x, zone.y, zone.radius!, 0, Math.PI * 2);
      ctx.fill();
      ctx.stroke();
      // Jagged fissures distinguish ground cracks from cannon craters.
      for (let ray = 0; ray < 6; ray++) {
        const angle = (ray * Math.PI) / 3;
        ctx.beginPath();
        ctx.moveTo(zone.x, zone.y);
        for (let step = 1; step <= 4; step++) {
          const distance = (zone.radius! * step) / 4,
            a = angle + (step % 2 ? 0.13 : -0.13);
          ctx.lineTo(
            zone.x + Math.cos(a) * distance,
            zone.y + Math.sin(a) * distance,
          );
        }
        ctx.stroke();
      }
    } else if (zone.kind === "flood") {
      const fraction = Math.max(
        0,
        Math.min(1, (elapsedMS - zone.createdMS) / 4000),
      );
      const fromY = zone.fromBottom ? map.height : -zone.height!;
      const y = fromY + (zone.y - fromY) * fraction;
      let top = Math.max(0, y),
        bottom = Math.min(map.height, y + zone.height!);
      if (zone.sweep) {
        top = zone.y;
        bottom = Math.min(zone.y + zone.height!, map.height * fraction);
        if (zone.fromBottom) {
          top = Math.max(zone.y, map.height * (1 - fraction));
          bottom = zone.y + zone.height!;
        }
      }
      if (top >= bottom) {
        ctx.restore();
        continue;
      }
      ctx.fillStyle = "#2584b877";
      ctx.strokeStyle = "#1f5e86";
      ctx.beginPath();
      ctx.rect(zone.x, top, zone.width!, Math.max(0, bottom - top));
      ctx.fill();
      ctx.stroke();
      ctx.save();
      ctx.clip();
      ctx.strokeStyle = "#c3e7ee";
      ctx.lineWidth = 2;
      const drift = reduced ? 0 : (elapsedMS / 160) % 28;
      for (let yy = top + drift; yy < bottom; yy += 28) {
        ctx.beginPath();
        for (let x = 0; x <= zone.width!; x += 10) {
          const py = yy + Math.sin(x / 24) * 4;
          if (x === 0) ctx.moveTo(zone.x + x, py);
          else ctx.lineTo(zone.x + x, py);
        }
        ctx.stroke();
      }
      ctx.restore();
    } else {
      ctx.fillStyle = "#617d9477";
      ctx.strokeStyle = "#e4f3fa";
      ctx.beginPath();
      ctx.arc(zone.x, zone.y, zone.radius!, 0, Math.PI * 2);
      ctx.fill();
      ctx.stroke();
      const rotation = reduced ? 0 : elapsedMS / 400;
      for (let arm = 0; arm < 3; arm++) {
        ctx.beginPath();
        for (let i = 0; i <= 36; i++) {
          const t = i / 36,
            a = rotation + (arm * Math.PI * 2) / 3 + t * 5,
            r = zone.radius! * t;
          const x = zone.x + Math.cos(a) * r,
            y = zone.y + Math.sin(a) * r;
          if (i === 0) ctx.moveTo(x, y);
          else ctx.lineTo(x, y);
        }
        ctx.stroke();
      }
    }
    ctx.restore();
  }
}
export function geologyLabel(geology?: Geology): string | null {
  if (!geology?.triggered || geology.rewarded || geology.kind === "none")
    return null;
  return ({ earthquake: "地震", flood: "洪水", typhoon: "台风" } as const)[
    geology.kind
  ];
}
export function drawDisabledTower(
  ctx: CanvasRenderingContext2D,
  point: Point,
  zoom: number,
  elapsedMS = 0,
  reduced = false,
) {
  ctx.save();
  const y=point.y-(reduced?0:Math.sin(elapsedMS/450)*3/zoom);
  const text=elapsedMS ? "Zzz..." : "ZZZ";
  ctx.font = `bold ${Math.max(16, 11 / zoom)}px system-ui`;
  ctx.textAlign = "center";
  ctx.lineWidth = 4;
  ctx.strokeStyle = "#263c32";
  ctx.fillStyle = "#fff2b8";
  ctx.strokeText(text, point.x, y);
  ctx.fillText(text, point.x, y);
  ctx.restore();
}

// The footprint follows the same world transform as the tower. Fade on the
// authoritative simulation clock, so pause/reconnect cannot restart the debuff.
export function drawFloodAura(
  ctx: CanvasRenderingContext2D,
  point: { x: number; y: number },
  size: number,
  ageMS: number,
) {
  if (ageMS < 0 || ageMS >= 10000) return;
  ctx.save();
  ctx.translate(point.x, point.y + 10);
  ctx.scale(1, 0.48);
  ctx.globalAlpha = 0.85 * (1 - ageMS / 10000);
  ctx.strokeStyle = "#b45de8";
  ctx.lineWidth = 3;
  for (const radius of [size * 0.56, size * 0.7]) {
    ctx.beginPath();
    ctx.arc(0, 0, radius, 0, Math.PI * 2);
    ctx.stroke();
  }
  for (let i = 0; i < 8; i++) {
    const angle = i * Math.PI / 4;
    const r = size * 0.63;
    ctx.save();
    ctx.translate(Math.cos(angle) * r, Math.sin(angle) * r);
    ctx.rotate(angle + Math.PI / 4);
    ctx.strokeRect(-3, -3, 6, 6);
    ctx.restore();
  }
  ctx.restore();
}

// Cyan rays echo the wish reference. The thin ellipse leaves roads readable.
export function drawAttackSlowAura(ctx: CanvasRenderingContext2D, point: Point, size: number, elapsedMS: number, reduced: boolean) {
  ctx.save();
  ctx.translate(point.x, point.y + 10);
  ctx.scale(1, .48);
  ctx.strokeStyle = "#72eced";
  ctx.lineWidth = 3;
  const pulse = reduced ? 1 : .9 + .1 * Math.sin(elapsedMS / 380);
  ctx.globalAlpha = .85;
  ctx.beginPath(); ctx.arc(0, 0, size * .65 * pulse, 0, Math.PI * 2); ctx.stroke();
  for (let i = 0; i < 12; i++) {
    const a = i * Math.PI / 6 + (reduced ? 0 : elapsedMS / 5000);
    ctx.beginPath();ctx.moveTo(Math.cos(a)*size*.55,Math.sin(a)*size*.55);
    ctx.lineTo(Math.cos(a)*size*.75,Math.sin(a)*size*.75);ctx.stroke();
  }
  ctx.restore();
}

// Keep annotations off every road, including crossings and narrow zoom views.
export function towerLabelPoint(ctx: CanvasRenderingContext2D, map: MapData, point: Point, preferredY: number, text: string, size: number): Point {
  const width = ctx.measureText(text).width / 2 + 3;
  const height = parseFloat(ctx.font.match(/[\d.]+px/)?.[0] ?? "12") + 3;
  const candidates = [{x:point.x,y:preferredY}, {x:point.x,y:preferredY-height*2}, {x:point.x,y:preferredY-height*4},
    {x:point.x+size*.7,y:point.y-size*.5},{x:point.x-size*.7,y:point.y-size*.5},
    {x:point.x,y:point.y+18},{x:point.x,y:point.y}];
  const distance = (p: Point) => {
    let best = Infinity;
    for (const path of (map.routes?.map(r=>r.points) ?? [map.path])) for(let i=1;i<path.length;i++) {
      const a=path[i-1],b=path[i],dx=b.x-a.x,dy=b.y-a.y;
      const t=Math.max(0,Math.min(1,((p.x-a.x)*dx+(p.y-a.y)*dy)/(dx*dx+dy*dy||1)));
      best=Math.min(best,Math.hypot(p.x-a.x-t*dx,p.y-a.y-t*dy));
    }
    return best;
  };
  let fallback=candidates[0], best=-Infinity;
  for(const p of candidates) {
    const clear=Math.min(...[-width,width].flatMap(x=>[0,-height].map(y=>distance({x:p.x+x,y:p.y+y}))));
    if(clear>=36 && p.x-width>=0 && p.x+width<=map.width && p.y-height>=0 && p.y<=map.height)return p;
    if(clear>best){best=clear;fallback=p;}
  }
  return fallback;
}
