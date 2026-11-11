type XYZ = Readonly<{ x: number; y: number; z: number }>;
export type ClassicCameraBody = { seat: number; min: XYZ; max: XYZ };
export type ClassicCameraNeighbor = {
  seat: number;
  roof: number;
  footprintDistance: number;
  influence: number;
};
export type ClassicCameraClearance = {
  targetLift: number;
  minimumLift: number;
  neighbors: ClassicCameraNeighbor[];
};

const clamp = (n: number, a: number, b: number) => Math.max(a, Math.min(b, n));

/** World-space decimetres, using actual rendered car bounds rather than seats. */
export function classicCameraClearance(
  eye: XYZ,
  follow: number,
  bodies: readonly ClassicCameraBody[],
): ClassicCameraClearance {
  const out: ClassicCameraClearance = { targetLift: 0, minimumLift: 0, neighbors: [] };
  for (const body of bodies) {
    if (body.seat === follow || ![
      body.min.x, body.min.y, body.min.z, body.max.x, body.max.y, body.max.z,
    ].every(Number.isFinite)) continue;
    // A car on an upper bridge is not an obstacle to a lower camera.
    if (eye.y < body.min.y - 1) continue;
    const dx = Math.max(body.min.x-eye.x, 0, eye.x-body.max.x);
    const dz = Math.max(body.min.z-eye.z, 0, eye.z-body.max.z);
    const distance = Math.hypot(dx,dz);
    if (distance >= 4) continue;
    const edge = clamp(distance/4,0,1);
    const influence = 1-edge*edge*(3-2*edge);
    out.neighbors.push({ seat:body.seat,roof:body.max.y,footprintDistance:distance,influence });
    out.targetLift = Math.max(out.targetLift,Math.max(0,body.max.y+22-eye.y)*influence);
    out.minimumLift = Math.max(out.minimumLift,Math.max(0,body.max.y+15-eye.y)*influence);
  }
  out.targetLift = clamp(out.targetLift,0,24);
  out.minimumLift = clamp(out.minimumLift,0,out.targetLift);
  return out;
}

/** Fast safe rise, slower continuous release; view switches remain immediate. */
export function advanceClassicCameraLift(
  current: number,
  clearance: ClassicCameraClearance,
  elapsed: number,
  snap: boolean,
): number {
  if (snap) return clearance.targetLift;
  const dt = clamp(Number.isFinite(elapsed) ? elapsed : 0,0,.05);
  const rate = clearance.targetLift > current ? 16 : 4;
  const value = current+(clearance.targetLift-current)*(1-Math.exp(-dt*rate));
  return clamp(Math.max(clearance.minimumLift,value),0,24);
}
