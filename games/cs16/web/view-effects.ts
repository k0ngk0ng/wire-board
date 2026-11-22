export function scopeFov(zoom?: number) { return 2 * Math.atan(Math.tan((zoom || 90) * Math.PI / 360) * 3 / 4) * 180 / Math.PI; }

export function weaponAnimation(clips: readonly string[], weapon: string, kind: string, serial = 0, silenced = false, contact = 'air') {
  const find = (pattern: RegExp) => clips.filter(name => pattern.test(name));
  let candidates: string[] = [];
  if (kind === 'slash') candidates = find(/^midslash[12]$/i);
  else if (kind === 'stab') candidates = find(contact === 'air' ? /^stab_miss$/i : /^stab$/i);
  else if (kind === 'shot') {
    if (weapon === 'elite') candidates = find(serial % 2 ? /^shoot_right[1-5]$/i : /^shoot_left[1-5]$/i);
    else if ((weapon === 'm4a1' || weapon === 'usp') && !silenced) candidates = find(/^shoot[1-3]_unsil$/i);
    else candidates = find(/^(?:shoot[1-3]?|fire)$/i);
  } else if (kind === 'pin') candidates = find(/pullpin/i);
  else if (kind === 'throw') candidates = find(/^throw/i);
  else if (kind === 'plant') candidates = find(/pressbutton/i);
  else if (kind === 'mode') candidates = find(silenced ? /detach_silencer/i : /add_silencer|attach|burst|zoom/i);
  else if (kind === 'reload') candidates = find((weapon === 'm4a1' || weapon === 'usp') && !silenced ? /^reload_unsil$/i : /^reload$/i);
  return candidates.length ? candidates[serial % candidates.length] : undefined;
}

export function deathAnimation(clips: readonly string[], hitgroup: number, crouch: boolean) {
  const desired = crouch ? 'crouch_die' : hitgroup === 1 ? 'head' : hitgroup === 2 ? 'death1' : hitgroup === 3 ? 'gutshot' : hitgroup === 4 ? 'left' : hitgroup === 5 ? 'right' : 'death2';
  return clips.find(name => name.toLowerCase() === desired) ?? clips.find(name => /^(?:death|die)/i.test(name));
}

export function deathView(elapsedMS: number, persist = false) {
  const elapsed = Math.round(elapsedMS);
  if (elapsed <= 0 || elapsed >= 2000 && !persist) return;
  const fallen = Math.min(1, elapsed / 600);
  return { eye: 17 - 35 * fallen, roll: Math.PI / 3 * fallen };
}

export function corpseAge(frame: number, deathAt?: number) {
  if (!Number.isSafeInteger(deathAt) || (deathAt ?? 0) <= 0) return 3000;
  return Math.max(0, Math.round((frame - deathAt!) * 10));
}

export function corpseTravel(velocity: { x: number; y: number; z: number }, elapsedMS: number) {
  const seconds = Math.max(0, Math.min(3000, Math.round(elapsedMS))) / 1000;
  const speed = Math.hypot(velocity.x, velocity.y) / 256;
  const moving = Math.min(seconds, speed / 200);
  const distance = Math.min(1024, speed * moving - 100 * moving * moving);
  const scale = distance / Math.max(1, speed);
  return { x: velocity.x / 256 * scale, y: velocity.y / 256 * scale, z: Math.min(512, velocity.z / 256 * seconds - 400 * seconds * seconds) };
}

type Point = { x: number; y: number; z: number };

export class CorpseInterpolation {
  private at = -1;
  private from?: Point;
  private to?: Point;
  reset() { this.at = -1; this.from = this.to = undefined; }
  sample(ageMS: number, evaluate: (at: number) => Point): Point {
    const age = Math.max(0, Math.min(3000, Math.round(ageMS)));
    const at = Math.floor(age / 100) * 100;
    if (at !== this.at) {
      this.from = this.to && at === this.at + 100 ? this.to : evaluate(at);
      this.to = at === 3000 ? this.from : evaluate(at + 100);
      this.at = at;
    }
    const mix = (age - at) / 100;
    return { x: this.from!.x + (this.to!.x - this.from!.x) * mix, y: this.from!.y + (this.to!.y - this.from!.y) * mix, z: this.from!.z + (this.to!.z - this.from!.z) * mix };
  }
}

export class ViewImpulse {
  private shotAt = -10000;
  private landedAt = -10000;
  private landedRoll = 0;
  shot(now: number) { this.shotAt = Math.round(now); }
  land(speed: number, now: number) {
    if (Math.round(speed) < 350) return;
    this.landedAt = Math.round(now);
    this.landedRoll = Math.min(12, speed * 0.013) * Math.PI / 180;
  }
  reset() { this.shotAt = this.landedAt = -10000; this.landedRoll = 0; }
  sample(now: number) {
    const age = Math.round(now);
    const shot = Math.max(0, 1 - (age - this.shotAt) / 150);
    const landing = Math.max(0, 1 - (age - this.landedAt) / 450);
    return { weaponBack: shot * 0.6, weaponUp: shot * 0.15, roll: this.landedRoll * landing };
  }
}
