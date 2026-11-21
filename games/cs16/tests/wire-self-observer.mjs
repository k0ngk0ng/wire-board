const safe = key => !['__proto__', 'prototype', 'constructor'].includes(key);
function patchObject(previous, delta) {
  const result = previous && typeof previous === 'object' && !Array.isArray(previous) ? { ...previous } : {};
  for (const [key, value] of Object.entries(delta.values ?? {})) if (safe(key)) result[key] = value;
  for (const key of delta.remove ?? []) if (safe(key)) delete result[key];
  for (const [key, value] of Object.entries(delta.objects ?? {})) if (safe(key)) result[key] = patchObject(result[key], value);
  return result;
}

export class WireSelfObserver {
  seat;
  epoch;
  life;
  player;
  partial = true;
  bind(context) { this.seat = context.seat; this.epoch = context.epoch; this.life = context.life; this.player = undefined; this.partial = true; }
  observe(data) {
    let appliedObserved = false;
    if (data.epoch !== this.epoch) { this.epoch = data.epoch; this.life = undefined; this.player = undefined; this.partial = true; }
    if (data.schema === 1) {
      if (!Number.isInteger(this.seat) && Number.isInteger(data.you)) this.seat = data.you;
      const player = data.players?.find(value => value.seat === this.seat);
      if (player) { this.player = { ...player }; this.life = player.life; this.partial = false; appliedObserved = Number.isSafeInteger(player.appliedSeq); }
    } else if (data.schema === 2) {
      const changes = data.collections?.players;
      if (changes?.remove?.includes(this.seat)) { this.player = undefined; this.partial = true; }
      const update = changes?.upsert?.find(value => value.id === this.seat);
      if (update?.full) { this.player = { ...update.full }; this.life = update.full.life; this.partial = false; appliedObserved = Number.isSafeInteger(update.full.appliedSeq); }
      else if (update?.patch) { this.player = patchObject(this.player, update.patch); appliedObserved = Number.isSafeInteger(update.patch.values?.appliedSeq); }
    }
    return { player: this.player, partial: this.partial, epoch: this.epoch, life: this.life, frame: data.frame, appliedObserved };
  }
}
