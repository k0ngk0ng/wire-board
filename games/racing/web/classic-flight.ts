import type { ClassicMotionState } from "./classic-motion.ts";

export type ClassicFlightState = {
  height: number;
  verticalSpeed: number;
  pitch: number;
  roll: number;
  airborne: boolean;
  impact: number;
};

export type ClassicFlightGround = {
  height: number;
  verticalSpeed: number;
  pitch: number;
  roll: number;
  supported: boolean;
};

export function stepClassicFlight(
  state: ClassicFlightState,
  ground: ClassicFlightGround,
  speed: number,
  dt: number,
): ClassicFlightState {
  const s = { ...state, impact: 0 };
  dt = Math.max(0, Math.min(.1, Number.isFinite(dt) ? dt : 0));
  if (!dt) return s;
  const nextHeight = s.height + s.verticalSpeed * dt - 49.05 * dt * dt;
  const nextSpeed = s.verticalSpeed - 98.1 * dt;
  if (s.airborne || !ground.supported || (speed > 80 && nextHeight > ground.height + .08)) {
    s.height = nextHeight;
    s.verticalSpeed = nextSpeed;
    s.airborne = true;
    s.pitch += (Math.atan2(nextSpeed, Math.max(40, speed)) - s.pitch) * (1 - Math.exp(-2 * dt));
    s.roll *= Math.exp(-dt);
    if (!ground.supported || s.height > ground.height) return s;
    s.impact = Math.max(0, ground.verticalSpeed - nextSpeed);
  }
  s.height = ground.height;
  s.verticalSpeed = ground.verticalSpeed;
  s.pitch = ground.pitch;
  s.roll = ground.roll;
  s.airborne = false;
  return s;
}

export function stepClassicAirMotion(state: ClassicMotionState, dt: number): ClassicMotionState {
  const s = { ...state };
  dt = Math.max(0, Math.min(.1, Number.isFinite(dt) ? dt : 0));
  const drag = Math.exp(-.000035 * Math.hypot(s.vx, s.vz) * dt);
  s.vx *= drag;
  s.vz *= drag;
  s.x += s.vx * dt;
  s.z += s.vz * dt;
  s.angle = Math.atan2(Math.sin(s.angle + s.yawRate * dt), Math.cos(s.angle + s.yawRate * dt));
  s.yawRate *= Math.exp(-.15 * dt);
  s.longAccel = 0;
  s.lateralAccel = 0;
  return s;
}
