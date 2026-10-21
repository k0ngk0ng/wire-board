import { CAR_MODELS, bodySurface } from "./car-definitions";
import {
  chasePose,
  projectPoint,
  clipPolygon,
  type Vec3,
} from "./chase-camera";
import { themeFor } from "./scene-theme";
import { seatColors, type Track } from "./types";
import type { SceneFrame } from "./chase-renderer";
// Same chase perspective when WebGL is unavailable; never changes simulation.
export class ChaseFallback {
  readonly mode = "canvas";
  private ctx: CanvasRenderingContext2D;
  private width = 1;
  private height = 1;
  constructor(
    private canvas: HTMLCanvasElement,
    private track: Track,
  ) {
    this.ctx = canvas.getContext("2d")!;
    if (!this.ctx) throw Error("无法初始化赛车画面");
    this.resize();
  }
  resize() {
    const r = this.canvas.getBoundingClientRect();
    this.width = Math.max(1, r.width);
    this.height = Math.max(1, r.height);
    this.canvas.width = Math.round(this.width);
    this.canvas.height = Math.round(this.height);
  }
  render(frame: SceneFrame) {
    const car =
      frame.cars.find((c) => c.seat === frame.follow) ?? frame.cars[0];
    if (!car) return null;
    const pose = chasePose(
        frame.reduced ? { ...car, speed: 0 } : car,
        frame.rear,
        this.width / this.height,
      ),
      c = this.ctx,
      t = themeFor(this.track.id),
      w = this.width,
      h = this.height;
    const sky = c.createLinearGradient(0, 0, 0, h * 0.58);
    sky.addColorStop(0, t.sky);
    sky.addColorStop(1, t.horizon);
    c.fillStyle = sky;
    c.fillRect(0, 0, w, h);
    c.fillStyle = t.ground;
    c.fillRect(0, h * 0.51, w, h * 0.49);
    const faces: { points: Vec3[]; color: string; depth: number }[] = [];
    const face = (raw: Vec3[], color: string) => {
      const points = clipPolygon(raw, pose);
      if (points.length < 3) return;
      const projected = points.map((p) => projectPoint(p, pose, w, h));
      if (projected.some((p) => !p)) return;
      faces.push({
        points,
        color,
        depth: projected.reduce((n, p) => n + p!.depth, 0) / points.length,
      });
    };
    const pts = this.track.points;
    for (let i = 0; i < pts.length; i++) {
      const a = pts[i],
        b = pts[(i + 1) % pts.length],
        len = Math.hypot(b.x - a.x, b.y - a.y),
        nx = -(b.y - a.y) / len,
        nz = (b.x - a.x) / len,
        r = this.track.roadWidth / 2;
      face(
        [
          { x: a.x + nx * (r + 10), y: 0, z: a.y + nz * (r + 10) },
          { x: b.x + nx * (r + 10), y: 0, z: b.y + nz * (r + 10) },
          { x: b.x - nx * (r + 10), y: 0, z: b.y - nz * (r + 10) },
          { x: a.x - nx * (r + 10), y: 0, z: a.y - nz * (r + 10) },
        ],
        t.edge,
      );
      face(
        [
          { x: a.x + nx * r, y: 0.4, z: a.y + nz * r },
          { x: b.x + nx * r, y: 0.4, z: b.y + nz * r },
          { x: b.x - nx * r, y: 0.4, z: b.y - nz * r },
          { x: a.x - nx * r, y: 0.4, z: a.y - nz * r },
        ],
        t.road,
      );
      if (i % 2 === 0)
        face(
          [
            { x: a.x + nx, y: 0.8, z: a.y + nz },
            { x: b.x + nx, y: 0.8, z: b.y + nz },
            { x: b.x - nx, y: 0.8, z: b.y - nz },
            { x: a.x - nx, y: 0.8, z: a.y - nz },
          ],
          "#eee5ca",
        );
    }
    for (const prop of this.track.props ?? []) {
      const r = prop.radius * 0.7,
        h = prop.height * 0.7,
        p = (x: number, y: number, z: number) => ({
          x: prop.x + x,
          y,
          z: prop.y + z,
        });
      const color =
        prop.kind === "building"
          ? "#384a5b"
          : prop.kind === "container"
            ? "#526d7c"
            : prop.kind === "rock"
              ? "#b1805a"
              : "#456b56";
      face([p(-r, 0, -r), p(r, 0, -r), p(r, h, -r), p(-r, h, -r)], color);
      face([p(-r, 0, r), p(r, 0, r), p(r, h, r), p(-r, h, r)], color);
      face([p(-r, 0, -r), p(-r, 0, r), p(-r, h, r), p(-r, h, -r)], color);
      face([p(r, 0, -r), p(r, 0, r), p(r, h, r), p(r, h, -r)], color);
      face([p(-r, h, -r), p(r, h, -r), p(r, h, r), p(-r, h, r)], color);
    }
    for (const vehicle of frame.cars) {
      const cs = Math.cos(vehicle.angle),
        sn = Math.sin(vehicle.angle),
        p = (x: number, y: number, z: number) => ({
          x: vehicle.x + x * cs - z * sn,
          y,
          z: vehicle.y + x * sn + z * cs,
        });
      const color = seatColors[vehicle.seat % seatColors.length];
      const model = CAR_MODELS[vehicle.skin] ?? CAR_MODELS[0],
        body = bodySurface(model);
      for (let k = 0; k < body.indices.length; k += 3) {
        const poly = body.indices
          .slice(k, k + 3)
          .map((index) =>
            p(
              body.vertices[index * 3],
              body.vertices[index * 3 + 1],
              body.vertices[index * 3 + 2],
            ),
          );
        face(poly, color);
      }

      const half = model.roofWidth / 2;
      face(
        model.roof.map(([x, y]) => p(x, y, -half)),
        "#263947",
      );
      face(
        model.roof.map(([x, y]) => p(x, y, half)),
        "#263947",
      );
      for (let i = 0; i < model.roof.length - 1; i++) {
        const a = model.roof[i],
          b = model.roof[i + 1];
        face(
          [
            p(a[0], a[1], -half),
            p(b[0], b[1], -half),
            p(b[0], b[1], half),
            p(a[0], a[1], half),
          ],
          Math.abs(a[1] - b[1]) < 1 ? color : "#263947",
        );
      }
      for (const side of [-1, 1])
        face(
          [
            p(-23.2, 6, side * 7 - 3),
            p(-23.2, 6, side * 7 + 3),
            p(-23.2, 8.5, side * 7 + 3),
            p(-23.2, 8.5, side * 7 - 3),
          ],
          vehicle.brake ? "#ff5461" : "#d82c42",
        );
    }
    faces.sort((a, b) => b.depth - a.depth);
    for (const f of faces) {
      c.beginPath();
      f.points.forEach((p, i) => {
        const q = projectPoint(p, pose, w, h)!;
        if (i) c.lineTo(q.x, q.y);
        else c.moveTo(q.x, q.y);
      });
      c.closePath();
      c.fillStyle = f.color;
      c.fill();
    }
    return pose;
  }
  get stats() {
    return { calls: 0, triangles: 0 };
  }
  dispose() {}
}
