import type {
  ClassicMotionState,
  ClassicRoadSurface,
} from "./classic-motion.ts";

export type ClassicRulePoint = { x: number; y: number };
export type ClassicRoadSample = {
  height: number;
  left: number;
  right: number;
  nx: number;
  ny: number;
  nz: number;
  fx: number;
  fy: number;
  fz: number;
  rx: number;
  ry: number;
  rz: number;
  surface: number;
  segment: number;
};
export type ClassicBarrier = {
  kind: string;
  a: ClassicRulePoint;
  b: ClassicRulePoint;
  bottom: number;
  top: number;
  segment: number;
};
export type ClassicRuleTrack = {
  version: number;
  id: string;
  name: string;
  geometry: string;
  width: number;
  height: number;
  roadWidth: number;
  length: number;
  points: ClassicRulePoint[];
  road: ClassicRoadSample[];
  barriers: ClassicBarrier[];
};
export type ClassicPropSnapshot = { catalog: string; broken: string };
export type ClassicPropCatalog = {
  version: number;
  unit: "metres" | "decimetres";
  geometry: string;
  catalogHash: string;
  props: Array<{ index: number; ruleEdges: number[] }>;
};
export type ClassicPropWorld = {
  catalog: string;
  geometry: string;
  byEdge: ReadonlyMap<number, number>;
  broken: Uint8Array;
};

/** Decode one confirmed race snapshot; static catalogs never own mutable bits. */
export function classicPropWorld(
  catalog: ClassicPropCatalog,
  state: ClassicPropSnapshot,
  track?: ClassicRuleTrack,
): ClassicPropWorld {
  if (
    catalog.version !== 1 ||
    !["metres", "decimetres"].includes(catalog.unit) ||
    !/^[a-f0-9]{64}$/.test(catalog.catalogHash) ||
    catalog.catalogHash !== state.catalog ||
    !/^[a-f0-9]{64}$/.test(catalog.geometry) ||
    (track && catalog.geometry !== track.geometry) ||
    !catalog.props.length || catalog.props.length > 108 ||
    typeof state.broken !== "string"
  ) throw new Error("赛道物件版本不一致");
  let decoded: string;
  try { decoded = atob(state.broken); } catch { throw new Error("赛道物件状态无效"); }
  if (btoa(decoded) !== state.broken || decoded.length !== Math.ceil(catalog.props.length / 8))
    throw new Error("赛道物件状态无效");
  const broken = Uint8Array.from(decoded, c => c.charCodeAt(0));
  const tail = catalog.props.length % 8;
  if (tail && broken[broken.length - 1] >> tail)
    throw new Error("赛道物件状态超出范围");
  const byEdge = new Map<number, number>();
  for (let index = 0; index < catalog.props.length; index++) {
    const prop = catalog.props[index];
    if (prop.index !== index || !Array.isArray(prop.ruleEdges) || !prop.ruleEdges.length)
      throw new Error("赛道物件编号无效");
    for (const edge of prop.ruleEdges) {
      if (!Number.isInteger(edge) || edge < 0 || byEdge.has(edge) || (track && track.barriers[edge]?.kind !== "object"))
        throw new Error("赛道物件碰撞引用无效");
      byEdge.set(edge, index);
    }
  }
  return { catalog: catalog.catalogHash, geometry: catalog.geometry, byEdge, broken };
}
export function classicPropBroken(world: ClassicPropWorld | undefined, index: number): boolean {
  return !!world && index >= 0 && index >> 3 < world.broken.length && !!(world.broken[index >> 3] & (1 << (index & 7)));
}
export function classicBarrierBroken(world: ClassicPropWorld | undefined, edge: number): boolean {
  return !!world && classicPropBroken(world, world.byEdge.get(edge) ?? -1);
}
export type ClassicRoadContact = {
  index: number;
  fraction: number;
  x: number;
  y: number;
  height: number;
  side: number;
  left: number;
  right: number;
  nx: number;
  ny: number;
  nz: number;
  rx: number;
  ry: number;
  rz: number;
  surface: number;
  segment: number;
};
export type NativeClassicRoute = {
  id: string;
  name: string;
  geometryHash: string;
  length: number;
  bounds: { max: number[] };
  points: Array<{
    x: number;
    y: number;
    z: number;
    leftWidth: number;
    rightWidth: number;
    nx: number;
    ny: number;
    nz: number;
    fx: number;
    fy: number;
    fz: number;
    rx: number;
    ry: number;
    rz: number;
    surface: number;
    segment: number;
  }>;
  barriers: Array<{
    kind: string;
    a?: number[];
    b?: number[];
    height?: number;
    segment?: number;
    collisionType?: number;
    vertices?: number[][];
  }>;
};

const clamp = (n: number, lo: number, hi: number) =>
  Math.max(lo, Math.min(hi, n));
const numeric = (n: number, scale = 1) => {
  if (!Number.isFinite(n)) throw new Error("赛道坐标无效");
  return Math.round(n * scale * 1e8) / 1e8;
};
function hull(vertices: number[][]): [number, number][] {
  const values = new Map<string, [number, number]>();
  for (const v of vertices) {
    const p: [number, number] = [numeric(v[0], 10), numeric(v[2], 10)];
    values.set(`${p[0]},${p[1]}`, p);
  }
  const points = [...values.values()].sort(
    (a, b) => a[0] - b[0] || a[1] - b[1],
  );
  if (points.length < 3) return points;
  const cross = (a: number[], b: number[], c: number[]) =>
    (b[0] - a[0]) * (c[1] - a[1]) - (b[1] - a[1]) * (c[0] - a[0]);
  const lower: [number, number][] = [],
    upper: [number, number][] = [];
  for (const p of points) {
    while (lower.length > 1 && cross(lower.at(-2)!, lower.at(-1)!, p) <= 1e-10)
      lower.pop();
    lower.push(p);
  }
  for (const p of [...points].reverse()) {
    while (upper.length > 1 && cross(upper.at(-2)!, upper.at(-1)!, p) <= 1e-10)
      upper.pop();
    upper.push(p);
  }
  return [...lower.slice(0, -1), ...upper.slice(0, -1)];
}

/** The same native-to-rule transform as scripts/racing/nfs-rules.py. */
export function classicTrackFromRoute(
  route: NativeClassicRoute,
): ClassicRuleTrack {
  if (!/^[a-f0-9]{64}$/.test(route.geometryHash))
    throw new Error("赛道版本无效");
  const points = route.points.map((p) => ({
    x: numeric(p.x, 10),
    y: numeric(p.z, 10),
  }));
  const road: ClassicRoadSample[] = route.points.map((p) => ({
    height: numeric(p.y, 10),
    left: numeric(p.leftWidth, 10),
    right: numeric(p.rightWidth, 10),
    nx: numeric(p.nx),
    ny: numeric(p.ny),
    nz: numeric(p.nz),
    fx: numeric(p.fx),
    fy: numeric(p.fy),
    fz: numeric(p.fz),
    rx: numeric(p.rx),
    ry: numeric(p.ry),
    rz: numeric(p.rz),
    surface: p.surface,
    segment: p.segment,
  }));
  if (road.some((r) => r.left <= 0 || r.right <= 0))
    throw new Error("赛道宽度无效");
  const barriers: ClassicBarrier[] = [];
  for (const source of route.barriers) {
    if (source.kind === "wall") {
      const a = source.a!,
        b = source.b!,
        bottom = Math.min(a[1], b[1]);
      barriers.push({
        kind: "wall",
        a: { x: numeric(a[0], 10), y: numeric(a[2], 10) },
        b: { x: numeric(b[0], 10), y: numeric(b[2], 10) },
        bottom: numeric(bottom, 10),
        top: numeric(bottom + source.height!, 10),
        segment: source.segment!,
      });
    } else if (source.kind === "object" && source.collisionType !== 120) {
      const vertices = source.vertices ?? [],
        polygon = hull(vertices);
      if (!polygon.length) continue;
      const bottom = Math.min(...vertices.map((v) => v[1])),
        top = Math.max(...vertices.map((v) => v[1]));
      for (let i = 0; i < polygon.length; i++) {
        const a = polygon[i],
          b = polygon[(i + 1) % polygon.length];
        barriers.push({
          kind: "object",
          a: { x: a[0], y: a[1] },
          b: { x: b[0], y: b[1] },
          bottom: numeric(bottom, 10),
          top: numeric(top, 10),
          segment: source.segment ?? -1,
        });
      }
    }
  }
  return {
    version: 3,
    id: route.id,
    name: route.name,
    geometry: route.geometryHash,
    width: numeric(route.bounds.max[0] + 30, 10),
    height: numeric(route.bounds.max[2] + 30, 10),
    roadWidth: numeric(
      road.reduce((a, r) => a + r.left + r.right, 0) / road.length,
    ),
    length: numeric(route.length, 10),
    points,
    road,
    barriers,
  };
}

const BODIES: Record<
  string,
  { width: number; height: number; length: number }
> = {
  p911: { width: 1.89999998, height: 1.29999995, length: 4.57179165 },
  fm50: { width: 2.0018816, height: 1.26999998, length: 4.54811954 },
  ldia: { width: 1.99999988, height: 1.25818837, length: 4.46999979 },
  mcf1: { width: 1.89999998, height: 1.28496265, length: 4.92000055 },
  ccor: { width: 2, height: 1.28402138, length: 4.55999994 },
};
export function classicVehicleBody(id: string) {
  return Object.hasOwn(BODIES, id) ? BODIES[id] : BODIES.p911;
}

type Index = { grid: Map<string, number[]> };
const indices = new Map<string, Index>();
function indexFor(t: ClassicRuleTrack): Index {
  const known = indices.get(t.geometry);
  if (known) return known;
  const grid = new Map<string, number[]>();
  for (let i = 0; i < t.barriers.length; i++) {
    const b = t.barriers[i],
      left = Math.floor((Math.min(b.a.x, b.b.x) - 30) / 100),
      right = Math.floor((Math.max(b.a.x, b.b.x) + 30) / 100),
      top = Math.floor((Math.min(b.a.y, b.b.y) - 30) / 100),
      bottom = Math.floor((Math.max(b.a.y, b.b.y) + 30) / 100);
    for (let x = left; x <= right; x++)
      for (let y = top; y <= bottom; y++) {
        const key = `${x},${y}`;
        const rows = grid.get(key) ?? [];
        rows.push(i);
        grid.set(key, rows);
      }
  }
  const index = { grid };
  indices.set(t.geometry, index);
  return index;
}
export function releaseClassicRoad(geometry: string) {
  indices.delete(geometry);
}

export function classicRoadAt(
  t: ClassicRuleTrack,
  x: number,
  y: number,
  height: number,
  hint: number,
): ClassicRoadContact {
  const n = t.points.length;
  const empty: ClassicRoadContact = {
    index: 0,
    fraction: 0,
    x: 0,
    y: 0,
    height: 0,
    side: 0,
    left: t.roadWidth / 2,
    right: t.roadWidth / 2,
    nx: 0,
    ny: 1,
    nz: 0,
    rx: 0,
    ry: 0,
    rz: 0,
    surface: 0,
    segment: 0,
  };
  if (!n || t.road.length !== n) return empty;
  let best = Infinity,
    bestHorizontal = Infinity,
    out = empty;
  function consider(i: number) {
    const j = (i + 1) % n,
      a = t.points[i],
      b = t.points[j],
      dx = b.x - a.x,
      dy = b.y - a.y,
      length = dx * dx + dy * dy;
    const f =
      length > 1e-12
        ? clamp(((x - a.x) * dx + (y - a.y) * dy) / length, 0, 1)
        : 0;
    const qx = a.x + dx * f,
      qy = a.y + dy * f,
      r = t.road[i],
      rr = t.road[j],
      lerp = (a: number, b: number) => a + (b - a) * f;
    const nx = lerp(r.nx, rr.nx),
      nz = lerp(r.nz, rr.nz);
    let ny = lerp(r.ny, rr.ny);
    if (Math.abs(ny) < 0.1) ny = 1;
    const h = lerp(r.height, rr.height) - (nx * (x - qx) + nz * (y - qy)) / ny;
    const horizontal = (x - qx) * (x - qx) + (y - qy) * (y - qy),
      vertical = height - h,
      score = horizontal + vertical * vertical * 4;
    if (score >= best) return;
    best = score;
    bestHorizontal = horizontal;
    let rx = lerp(r.rx, rr.rx),
      ry = lerp(r.ry, rr.ry),
      rz = lerp(r.rz, rr.rz),
      rl = Math.hypot(rx, rz);
    if (rl < 1e-8) {
      rx = -dy;
      rz = dx;
      rl = Math.hypot(rx, rz);
    }
    const side = rl > 1e-8 ? ((x - qx) * rx + (y - qy) * rz) / rl : 0;
    out = {
      index: i,
      fraction: f,
      x: qx,
      y: qy,
      height: h,
      side,
      left: lerp(r.left, rr.left),
      right: lerp(r.right, rr.right),
      nx,
      ny,
      nz,
      rx,
      ry,
      rz,
      surface: r.surface,
      segment: r.segment,
    };
  }
  if (hint >= 0 && hint < n)
    for (let k = -40; k <= 40; k++) consider((hint + k + n) % n);
  else for (let i = 0; i < n; i++) consider(i);
  if (bestHorizontal > 1200 * 1200) for (let i = 0; i < n; i++) consider(i);
  return out;
}

export function classicRoadSurface(
  road: ClassicRoadContact,
  heading: number,
): ClassicRoadSurface {
  const excess = Math.max(
      0,
      Math.max(road.side - road.right, -road.side - road.left),
    ),
    shoulder = clamp(excess / 30, 0, 1);
  const ny = Math.abs(road.ny) < 0.1 ? 1 : road.ny;
  const grade = Math.atan(
    -(road.nx * Math.cos(heading) + road.nz * Math.sin(heading)) / ny,
  );
  return {
    grip: 1 - 0.6 * shoulder,
    resistance: 5 * shoulder,
    normalY: Math.abs(ny),
    grade,
  };
}

function segmentClosest(
  a: ClassicRulePoint,
  b: ClassicRulePoint,
  c: ClassicRulePoint,
  d: ClassicRulePoint,
): [ClassicRulePoint, ClassicRulePoint] {
  const ux = b.x - a.x,
    uy = b.y - a.y,
    vx = d.x - c.x,
    vy = d.y - c.y,
    wx = a.x - c.x,
    wy = a.y - c.y;
  const aa = ux * ux + uy * uy,
    bb = vx * vx + vy * vy,
    ab = ux * vx + uy * vy,
    aw = ux * wx + uy * wy,
    bw = vx * wx + vy * wy;
  let s = 0,
    q = 0;
  if (aa < 1e-10 && bb < 1e-10) return [a, c];
  if (aa < 1e-10) q = clamp(bw / bb, 0, 1);
  else if (bb < 1e-10) s = clamp(-aw / aa, 0, 1);
  else {
    const den = aa * bb - ab * ab;
    if (den > 1e-10) s = clamp((ab * bw - aw * bb) / den, 0, 1);
    q = (ab * s + bw) / bb;
    if (q < 0) {
      q = 0;
      s = clamp(-aw / aa, 0, 1);
    } else if (q > 1) {
      q = 1;
      s = clamp((ab - aw) / aa, 0, 1);
    }
  }
  return [
    { x: a.x + ux * s, y: a.y + uy * s },
    { x: c.x + vx * q, y: c.y + vy * q },
  ];
}

export function collideClassicRoad(
  t: ClassicRuleTrack,
  motion: ClassicMotionState,
  vehicle: string,
  height: number,
  props?: ClassicPropWorld,
  onBreak?: (index: number, vx: number, vz: number, closing: number) => void,
): ClassicMotionState {
  if (props && props.geometry !== t.geometry) throw new Error("赛道物件版本不一致");
  const s = { ...motion },
    info = indexFor(t),
    body = classicVehicleBody(vehicle);
  let x = s.x * 10,
    y = s.z * 10;
  for (let pass = 0; pass < 2; pass++) {
    const candidates =
      info.grid.get(`${Math.floor(x / 100)},${Math.floor(y / 100)}`) ?? [];
    for (const idx of candidates) {
      const prop = props?.byEdge.get(idx) ?? -1;
      if (classicPropBroken(props, prop)) continue;
      const wall = t.barriers[idx];
      if (
        height + body.height * 10 < wall.bottom + 0.05 ||
        height > wall.top - 0.05
      )
        continue;
      const radius = body.width * 5,
        half = (body.length - body.width) * 5,
        ax = Math.cos(s.angle) * half,
        ay = Math.sin(s.angle) * half;
      const [p, q] = segmentClosest(
          { x: x - ax, y: y - ay },
          { x: x + ax, y: y + ay },
          wall.a,
          wall.b,
        ),
        dx = p.x - q.x,
        dy = p.y - q.y,
        distance = Math.hypot(dx, dy);
      if (distance >= radius) continue;
      let nx = dx,
        ny = dy;
      if (distance > 1e-7) {
        nx /= distance;
        ny /= distance;
      } else {
        nx = -(wall.b.y - wall.a.y);
        ny = wall.b.x - wall.a.x;
        const l = Math.hypot(nx, ny);
        if (l < 1e-8) {
          nx = -Math.cos(s.angle);
          ny = -Math.sin(s.angle);
        } else {
          nx /= l;
          ny /= l;
        }
        if (
          (x - (wall.a.x + wall.b.x) * 0.5) * nx +
            (y - (wall.a.y + wall.b.y) * 0.5) * ny <
          0
        ) {
          nx = -nx;
          ny = -ny;
        }
      }
      const inward = s.vx * nx + s.vz * ny;
      if (props && prop >= 0 && inward < -1e-3) {
        props.broken[prop >> 3] |= 1 << (prop & 7);
        onBreak?.(prop, s.vx, s.vz, -inward);
        continue;
      }
      const depth = radius - distance + 0.001;
      x += nx * depth;
      y += ny * depth;
      if (inward < 0) {
        s.vx -= inward * nx * 1.1;
        s.vz -= inward * ny * 1.1;
      }
    }
  }
  s.x = x / 10;
  s.z = y / 10;
  return s;
}
