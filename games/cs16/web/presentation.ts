export function eyeOffset(crouch: boolean) { return crouch ? 12 : 17; }
export function viewModelVerticalFov(aspect: number) { return 2 * Math.atan(Math.max(3 / 4, 1 / Math.max(0.1, aspect))) * 180 / Math.PI; }
export function viewModelHand(weapon: string) { return weapon === 'knife' ? 1 : -1; }
export function formatDuration(frames: number) {
  const total = Math.ceil(Math.max(0, frames) / 100);
  return `${Math.floor(total / 60)}:${String(total % 60).padStart(2, '0')}`;
}
export function viewAngles(authority: { yaw: number; pitch: number }, manual: { yaw: number; pitch: number }, automated: boolean) { return automated ? { yaw: authority.yaw, pitch: authority.pitch } : manual; }
export function visibleVelocity(previous: { x: number; y: number }, current: { x: number; y: number; vx?: number; vy?: number }, frames: number) {
  return { vx: current.vx ?? Math.round((current.x - previous.x) * 100 / Math.max(1, frames)), vy: current.vy ?? Math.round((current.y - previous.y) * 100 / Math.max(1, frames)) };
}
export function smokeRadius(smoke: { kind: string; active: boolean; expiresAt: number }, frame: number) {
  const age = frame - (smoke.expiresAt - 1800);
  return !smoke.active || smoke.kind !== 'smokegrenade' || age <= 0 || frame >= smoke.expiresAt ? 0 : Math.trunc(180 * 256 * Math.min(age, 100) / 100);
}

type DisplayPlayer = { x: number; y: number; z: number; alive: boolean; life: number; crouch: boolean };
export type SmoothTest = (from: { x: number; y: number; z: number }, to: { x: number; y: number; z: number }, crouch: boolean) => boolean;

export class DisplayCorrection {
  private offset = { x: 0, y: 0, z: 0 };
  private startedAt = 0;
  private endsAt = 0;
  private allowed: SmoothTest;
  constructor(allowed: SmoothTest) { this.allowed = allowed; }
  reset() { this.offset = { x: 0, y: 0, z: 0 }; this.startedAt = 0; this.endsAt = 0; }
  reconcile(from: DisplayPlayer, to: DisplayPlayer, at: number) {
    const offset = { x: from.x - to.x, y: from.y - to.y, z: from.z - to.z };
    const squared = offset.x * offset.x + offset.y * offset.y + offset.z * offset.z;
    if (!from.alive || !to.alive || from.life !== to.life || from.crouch !== to.crouch || squared === 0 || squared > 8 * 256 * 8 * 256 || !this.allowed(to, from, to.crouch)) { this.reset(); return; }
    this.endsAt = at < this.endsAt ? this.endsAt : at + 80;
    this.startedAt = at;
    this.offset = offset;
  }
  sample<T extends DisplayPlayer>(raw: T, at: number): T {
    if (!raw.alive || at >= this.endsAt || this.endsAt <= this.startedAt) { this.reset(); return raw; }
    const remaining = Math.max(0, Math.min(1, (this.endsAt - at) / (this.endsAt - this.startedAt)));
    const displayed = { ...raw, x: raw.x + Math.round(this.offset.x * remaining), y: raw.y + Math.round(this.offset.y * remaining), z: raw.z + Math.round(this.offset.z * remaining) };
    if (!this.allowed(raw, displayed, raw.crouch)) { this.reset(); return raw; }
    return displayed;
  }
}

export function observerCandidates<T extends { team: string; alive: boolean; visible?: boolean }>(players: T[], own: { team: string } | undefined, spectating: boolean): T[] {
  return players.filter(player => player.alive && player.visible !== false && (!own || spectating || player.team === own.team));
}
