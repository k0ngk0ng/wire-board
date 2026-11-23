import type { Bomb, GameEvent, Player } from './types.ts';

export function actionProgress(player: Pick<Player, 'seat' | 'alive' | 'action' | 'actionSince' | 'defuser'>, bomb: Pick<Bomb, 'planted' | 'defuser' | 'defuseAt'>, frame: number) {
  if (!player.alive) return;
  if (player.action === 'plant' && !bomb.planted) return { kind: 'plant', value: Math.max(0, Math.min(1000, Math.floor((frame - (player.actionSince ?? frame)) * 1000 / 300))) };
  if (bomb.planted && bomb.defuser === player.seat && bomb.defuseAt > frame) {
    const duration = player.defuser ? 500 : 1000;
    return { kind: 'defuse', value: Math.max(0, Math.min(1000, Math.floor((duration - (bomb.defuseAt - frame)) * 1000 / duration))) };
  }
}

export function soundCues(event: Pick<GameEvent, 'type' | 'weapon' | 'value' | 'hitgroup'> & { id?: number }): { files: string[]; range: number; local?: boolean; volumeMilli?: number; layers?: { files: string[]; volumeMilli: number }[] } {
  const weapon = event.weapon === 'mp5navy' ? 'mp5' : event.weapon;
  if (event.type === 'buy') return { files: [event.weapon === 'nvgs' ? 'items/equip_nvg.wav' : event.weapon === 'primammo' || event.weapon === 'secammo' ? 'items/9mmclip1.wav' : event.weapon === 'vest' || event.weapon === 'vesthelm' || event.weapon === 'defuser' ? 'items/kevlar.wav' : 'items/gunpickup2.wav'], range: 0, local: true };
  if (event.type === 'nightvision') return { files: [event.value === 1 ? 'items/nvg_on.wav' : 'items/nvg_off.wav'], range: 0, local: true };
  if (event.type === 'pickup') return { files: [event.weapon === 'defuser' ? 'items/kevlar.wav' : 'items/gunpickup2.wav'], range: 1250 };
  if (event.type === 'knife_air') return { files: ['weapons/knife_slash1.wav', 'weapons/knife_slash2.wav'], range: 1250 };
  if (event.type === 'knife_hit') return { files: event.value === 1 ? ['weapons/knife_stab.wav'] : ['weapons/knife_hit1.wav', 'weapons/knife_hit2.wav', 'weapons/knife_hit3.wav', 'weapons/knife_hit4.wav'], range: 1250 };
  if (event.type === 'knife_wall') return { files: ['weapons/knife_hitwall1.wav'], range: 1250 };
  if (event.type === 'hit') return { files: [event.hitgroup === 1 ? 'player/headshot1.wav' : 'player/bhit_flesh-1.wav'], range: 1250 };
  if (event.type === 'shot' || event.type === 'fire') {
    if (weapon === 'knife' || weapon === 'c4') return { files: [], range: 0 };
    const silenced = event.value === 1 && (weapon === 'm4a1' || weapon === 'usp');
    return { files: [...(silenced ? [`weapons/${weapon}-1.wav`, `weapons/${weapon}1.wav`] : [`weapons/${weapon}_unsil-1.wav`]), `weapons/${weapon}-1.wav`, `weapons/${weapon}-2.wav`, `weapons/${weapon}_fire1.wav`, `weapons/${weapon}1.wav`], range: silenced ? 500 : 2500 };
  }
  if (event.type === 'step') return { files: [`player/pl_step${(event.value ?? 0) % 4 + 1}.wav`, 'player/pl_step1.wav'], range: 1250, volumeMilli: 500 };
  if (event.type === 'land') return { files: ['player/pl_jump1.wav', 'player/pl_step1.wav'], range: 512 };
  if (event.type === 'reload') return { files: [`weapons/${weapon}_clipout.wav`, `weapons/${weapon}_magout.wav`], range: 512 };
  if (event.type === 'flashlight') return { files: ['items/flashlight1.wav'], range: 256 };
  if (event.type === 'bomb_click') return { files: ['weapons/c4_click.wav'], range: 256 };
  if (event.type === 'bomb_beep') return { files: [`weapons/c4_beep${event.value ?? 1}.wav`], range: 1000 };
  if (event.type === 'bomb_defused') return { files: ['weapons/c4_disarm.wav'], range: 1000 };
  if (event.type === 'grenade_explode' || event.type === 'explode') {
    if (event.weapon === 'flashbang') return { files: ['weapons/flashbang-1.wav'], range: 2400 };
    if (event.weapon === 'smokegrenade') return { files: ['weapons/sg_explode.wav'], range: 2400 };
    if (event.weapon === 'hegrenade') {
      const variant = Math.abs(event.id ?? 0) % 3;
      return { files: [`weapons/explode${variant + 3}.wav`], range: 2400, layers: [{ files: [`weapons/explode${3 + (variant + 1) % 3}.wav`], volumeMilli: 1000 }, { files: [`weapons/debris${variant + 1}.wav`], volumeMilli: 550 }] };
    }
  }
  return { files: [], range: 0 };
}

export function decodeBombCue(code: number | undefined) {
  if (!Number.isSafeInteger(code) || !code || code < 8) return;
  const stage = code % 8, serial = Math.floor(code / 8);
  if (stage < 1 || stage > 5 || serial < 1) return;
  return { serial, stage };
}

export class BombAudioCursor {
  private identity = '';
  private previous = 0;
  private waiting = true;

  suspend() { this.waiting = true; }

  consume(identity: string, code: number | undefined, enabled: boolean, recovered = false) {
    const cue = decodeBombCue(code), serial = cue?.serial ?? 0;
    if (identity !== this.identity) { this.identity = identity; this.previous = serial; this.waiting = !enabled || recovered; return; }
    if (!enabled || recovered) { this.previous = serial; this.waiting = true; return; }
    if (this.waiting) { this.previous = serial; this.waiting = false; return; }
    if (!cue) { this.previous = 0; return; }
    if (serial <= this.previous) return;
    const consecutive = serial === this.previous + 1;
    this.previous = serial;
    return consecutive ? cue.stage : undefined;
  }
}

export function plantClick(frame: number, since: number) {
  const mdlFrame = Math.floor((frame - since) * 33 / 100);
  const events = [39, 47, 54, 62, 70, 78, 84];
  const index = events.filter(value => value <= mdlFrame).length - 1;
  return index >= 0 ? index : undefined;
}

export class CueCursor {
  private identity = '';
  private previous = '';

  consume(identity: string, key: string) {
    const changed = identity !== this.identity;
    const fresh = !!key && !changed && key !== this.previous;
    this.identity = identity;
    this.previous = key;
    return fresh;
  }
}
