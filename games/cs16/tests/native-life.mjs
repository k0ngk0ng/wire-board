export function nativeLifeInterrupted(authority, identity) {
  if (!authority || !identity || authority.partial !== false || typeof identity.epoch !== 'string' || !identity.epoch || authority.epoch !== identity.epoch || !Number.isSafeInteger(identity.life) || identity.life < 1 || !Number.isSafeInteger(authority.life) || authority.life < 1 || !Number.isSafeInteger(identity.seat) || identity.seat < 0 || authority.player?.seat !== identity.seat || typeof authority.player.alive !== 'boolean') return false;
  return authority.player.alive === false || authority.life !== identity.life;
}
