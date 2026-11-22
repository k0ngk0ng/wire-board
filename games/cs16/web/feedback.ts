import type { GameInput } from './types.ts';

export type FeedbackKind = 'shot' | 'slash' | 'stab' | 'pin' | 'throw' | 'plant' | 'mode' | 'reload';

export function attackFeedbackBlocked(value: { kind: FeedbackKind; heldBefore: boolean; nextAttack: number; frame: number; reloadUntil: number }) {
  return value.reloadUntil > value.frame || value.nextAttack > value.frame || (value.kind === 'shot' || value.kind === 'slash') && value.heldBefore;
}

export function attackEventInCurrentLife(event: { seat: number; life?: number }, seat: number, life: number) {
  return event.seat !== seat || event.life === undefined || event.life === life;
}

export class AttackFeedbackQueue {
  private serial = 0;
  private pending: { kind: FeedbackKind; weapon: string; life: number; at: number; seq: number }[] = [];

  predict(kind: FeedbackKind, weapon: string, life: number, at: number, seq = 0) {
    const now = Math.round(at);
    this.pending = this.pending.filter(item => now - item.at <= 500).slice(-31);
    this.pending.push({ kind, weapon, life, at: now, seq });
    return ++this.serial;
  }

  reject(seq: number, life: number) {
    this.pending = this.pending.filter(item => item.seq !== seq || item.life !== life);
  }

  confirm(kind: FeedbackKind, weapon: string, life: number, at: number) {
    const now = Math.round(at);
    this.pending = this.pending.filter(item => now - item.at <= 500);
    const index = this.pending.findIndex(item => item.kind === kind && item.weapon === weapon && item.life === life && now >= item.at);
    if (index < 0) return false;
    this.pending.splice(index, 1);
    return true;
  }

  reset() { this.pending = []; }
}

export function announcementSound(event: { type: string; global?: boolean; value?: number }): string | undefined {
  if (!event.global) return;
  if (event.type === 'bomb_planted') return 'radio/bombpl.wav';
  if (event.type === 'bomb_defused') return 'radio/bombdef.wav';
  if (event.type === 'bomb_exploded') return 'weapons/c4_explode1.wav';
  if (event.type === 'hostage_rescued') return 'radio/rescued.wav';
  if (event.type === 'round_end') return event.value === 1 ? 'radio/terwin.wav' : event.value === 2 ? 'radio/ctwin.wav' : 'radio/rounddraw.wav';
}

export function localFeedback(weapon: string, input: GameInput, previous: GameInput, ammo: number): FeedbackKind | undefined {
  if (weapon === 'knife') {
    if (input.secondary && !previous.secondary) return 'stab';
    if (input.fire) return 'slash';
    return;
  }
  if (weapon === 'c4') return input.fire && !previous.fire ? 'plant' : undefined;
  if (['hegrenade', 'flashbang', 'smokegrenade'].includes(weapon)) {
    if (ammo <= 0) return;
    if (input.fire && !previous.fire) return 'pin';
    if (!input.fire && previous.fire) return 'throw';
    return;
  }
  if (input.reload && !previous.reload) return 'reload';
  if (input.secondary && !previous.secondary && ['m4a1', 'usp', 'glock18', 'famas', 'aug', 'sg552', 'scout', 'awp', 'g3sg1', 'sg550'].includes(weapon)) return 'mode';
  if (input.fire && ammo > 0) return 'shot';
}

export function eventFeedback(event: { type: string; weapon?: string; value?: number }): FeedbackKind | undefined {
  if (event.type === 'shot' || event.type === 'fire') return event.weapon === 'knife' ? event.value === 1 ? 'stab' : 'slash' : 'shot';
  if (event.type === 'grenade_pin') return 'pin';
  if (event.type === 'throw') return 'throw';
  if (event.type === 'plant_start') return 'plant';
  if (event.type === 'mode' || event.type === 'secondary') return 'mode';
  if (event.type === 'reload') return 'reload';
}
