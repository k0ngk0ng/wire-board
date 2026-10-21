import art from "../data/supplies.json" with { type: "json" };
import type { Supply } from "./types";
export const supplyPaths = [art.crate.path, art.helicopter.path];
export function supplyPose(supply: Supply, now: number) {
  if (now < supply.at) return null;
  const approaching = now < supply.dropAt;
  const leaving = Math.max(0, (now - supply.dropAt) / 650);
  const helicopter =
    leaving >= 1
      ? null
      : {
          x: supply.fromX,
          y: approaching
            ? 50 * Math.min(1, (now - supply.at) / 700)
            : 50 * (1 - leaving),
        };
  const progress = Math.max(
    0,
    Math.min(
      1,
      (now - supply.dropAt) / Math.max(1, supply.landAt - supply.dropAt),
    ),
  );
  const crate = approaching
    ? null
    : {
        x: supply.fromX + (supply.x - supply.fromX) * progress,
        y: 45 + (supply.y - 45) * progress * progress,
        airborne: now < supply.landAt,
      };
  return { helicopter, crate };
}
export function supplySpriteFrame(
  kind: "crate" | "helicopter",
  now: number,
  reduced: boolean,
  airborne = false,
) {
  const index = reduced
    ? 0
    : kind === "helicopter"
      ? Math.floor(Math.max(0, now) / 90) % 10
      : airborne
        ? 1 + (Math.floor(Math.max(0, now) / 90) % 8)
        : 0;
  return { ...art[kind].frames[index], path: art[kind].path, index };
}
export function drawSupplies(
  ctx: CanvasRenderingContext2D,
  supplies: Supply[],
  images: Record<string, HTMLImageElement | null | undefined>,
  now: number,
  reduced: boolean,
) {
  const drawn = [];
  for (const supply of supplies) {
    const pose = supplyPose(supply, now);
    if (!pose) continue;
    let artReady = false;
    for (const kind of ["helicopter", "crate"] as const) {
      const point = pose[kind];
      if (!point) continue;
      const frame = supplySpriteFrame(
        kind,
        now - supply.at,
        reduced,
        kind === "crate" && !!pose.crate?.airborne,
      );
      const image = images[frame.path];
      if (image) {
        ctx.drawImage(
          image,
          frame.x,
          frame.y,
          frame.w,
          frame.h,
          point.x - frame.ax,
          point.y - frame.ay,
          frame.w,
          frame.h,
        );
        if (kind === "crate") artReady = true;
      } else if (kind === "crate") {
        ctx.fillStyle = "#be914f";
        ctx.fillRect(point.x - 10, point.y - 18, 20, 18);
        ctx.strokeStyle = "#4a351e";
        ctx.lineWidth = 2;
        ctx.strokeRect(point.x - 10, point.y - 18, 20, 18);
        ctx.beginPath();
        ctx.moveTo(point.x - 8, point.y - 16);
        ctx.lineTo(point.x + 8, point.y - 2);
        ctx.moveTo(point.x + 8, point.y - 16);
        ctx.lineTo(point.x - 8, point.y - 2);
        ctx.stroke();
      } else {
        ctx.fillStyle = "#536d78";
        ctx.fillRect(point.x - 25, point.y - 18, 45, 16);
        ctx.fillRect(point.x + 18, point.y - 16, 20, 4);
        ctx.strokeStyle = "#1b313c";
        ctx.lineWidth = 3;
        ctx.beginPath();
        ctx.moveTo(point.x - 34, point.y - 24);
        ctx.lineTo(point.x + 30, point.y - 24);
        ctx.moveTo(point.x, point.y - 24);
        ctx.lineTo(point.x, point.y - 17);
        ctx.stroke();
      }
    }
    if (pose.crate) {
      const p = pose.crate;
      ctx.strokeStyle = "#ffe395";
      ctx.lineWidth = 1.5;
      ctx.strokeRect(p.x - 13, p.y - 21, 26, 23);
      if (!p.airborne) {
        ctx.fillStyle = "#122d3b";
        ctx.fillRect(p.x - 17, p.y - 38, 34, 15);
        ctx.fillStyle = "#fff0b7";
        ctx.font = "bold 11px sans-serif";
        ctx.textAlign = "center";
        ctx.fillText("补给", p.x, p.y - 27);
      }
      drawn.push({
        id: supply.id,
        x: p.x,
        y: p.y,
        airborne: p.airborne,
        artReady,
        helicopter: !!pose.helicopter,
      });
    }
  }
  return drawn;
}
